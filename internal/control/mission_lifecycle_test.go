// pattern: Imperative Shell
package control

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"polis/internal/kernel"
	"polis/internal/provider"
)

func TestPauseResumeAreFormalAndIdempotentAcrossLaterTransitions(t *testing.T) {
	dsn := os.Getenv("POLIS_TEST_DSN")
	if dsn == "" {
		t.Skip("dedicated PostgreSQL required")
	}
	ctx := context.Background()
	runtime, err := kernel.Open(ctx, dsn, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Close()
	companyID := "lifecycle-control-" + fmt.Sprint(time.Now().UnixNano())
	scope, err := runtime.TXCreateCompany(ctx, companyID)
	if err != nil {
		t.Fatal(err)
	}
	mission, err := runtime.TXCreateMissionGoal(ctx, scope, "Pause/resume", "verify user-controlled formal pause and resume", "lifecycle-control-create")
	if err != nil {
		t.Fatal(err)
	}
	worker := &recordingWorkerAdapter{}
	service := NewService(runtime, worker)
	startRequest := MissionCommandRequest{MissionID: mission.ID, RequestID: "lifecycle-control-start"}
	if _, err = service.StartMission(ctx, companyID, startRequest); err != nil {
		t.Fatal(err)
	}
	if replay, replayErr := service.StartMission(ctx, companyID, startRequest); replayErr != nil || replay.ResultingState != "active" || len(worker.started) != 1 {
		t.Fatalf("replayed StartMission must return the prior receipt without starting another Worker: receipt=%+v starts=%v err=%v", replay, worker.started, replayErr)
	}
	pauseRequest := MissionCommandRequest{MissionID: mission.ID, RequestID: "lifecycle-control-pause-1"}
	paused, err := service.PauseMission(ctx, companyID, pauseRequest)
	if err != nil || paused.ResultingState != "paused" || len(worker.stopped) != 1 {
		t.Fatalf("pause receipt=%+v stopped=%v err=%v", paused, worker.stopped, err)
	}
	if _, err = service.ResumeMission(ctx, companyID, MissionCommandRequest{MissionID: mission.ID, RequestID: "lifecycle-control-resume-1"}); err != nil {
		t.Fatal(err)
	}
	if len(worker.started) != 2 {
		t.Fatalf("resume did not start a new WorkerSession: %v", worker.started)
	}
	if replay, err := service.PauseMission(ctx, companyID, pauseRequest); err != nil || replay.ResultingState != "paused" || len(worker.stopped) != 1 {
		t.Fatalf("replayed old pause changed current lifecycle: receipt=%+v stopped=%v err=%v", replay, worker.stopped, err)
	}
	secondPause, err := service.PauseMission(ctx, companyID, MissionCommandRequest{MissionID: mission.ID, RequestID: "lifecycle-control-pause-2"})
	if err != nil || secondPause.ResultingState != "paused" || len(worker.stopped) != 2 {
		t.Fatalf("second pause receipt=%+v stopped=%v err=%v", secondPause, worker.stopped, err)
	}
	if replay, err := service.ResumeMission(ctx, companyID, MissionCommandRequest{MissionID: mission.ID, RequestID: "lifecycle-control-resume-1"}); err != nil || replay.ResultingState != "active" || len(worker.started) != 2 {
		t.Fatalf("replayed old resume changed current lifecycle: receipt=%+v started=%v err=%v", replay, worker.started, err)
	}
	details, err := runtime.MissionDetails(ctx, scope, mission.ID)
	if err != nil || details.State != "paused" {
		t.Fatalf("current Mission state = %q, err=%v", details.State, err)
	}
}

func TestStartMissionFinishesWorkerStartBeforeConcurrentCancellation(t *testing.T) {
	dsn := os.Getenv("POLIS_TEST_DSN")
	if dsn == "" {
		t.Skip("dedicated PostgreSQL required")
	}
	ctx := context.Background()
	runtime, err := kernel.Open(ctx, dsn, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Close()
	companyID := "lifecycle-start-cancel-" + fmt.Sprint(time.Now().UnixNano())
	scope, err := runtime.TXCreateCompany(ctx, companyID)
	if err != nil {
		t.Fatal(err)
	}
	mission, err := runtime.TXCreateMissionGoal(ctx, scope, "Start/cancel ordering", "verify lifecycle serialization", "lifecycle-start-cancel-create")
	if err != nil {
		t.Fatal(err)
	}
	worker := &blockingStartLifecycleWorker{
		startEntered:               make(chan struct{}),
		releaseStart:               make(chan struct{}),
		startReturned:              make(chan struct{}),
		stopBeforeStartWasReturned: make(chan struct{}, 1),
	}
	service := NewService(runtime, worker)
	startDone := make(chan error, 1)
	go func() {
		_, startErr := service.StartMission(ctx, companyID, MissionCommandRequest{MissionID: mission.ID, RequestID: "lifecycle-start-cancel-start"})
		startDone <- startErr
	}()
	select {
	case <-worker.startEntered:
	case <-time.After(2 * time.Second):
		t.Fatal("WorkerAdapter.Start was not reached")
	}
	cancelStarted := make(chan struct{})
	cancelDone := make(chan error, 1)
	go func() {
		close(cancelStarted)
		_, cancelErr := service.CancelMission(ctx, companyID, MissionCommandRequest{MissionID: mission.ID, RequestID: "lifecycle-start-cancel-cancel"})
		cancelDone <- cancelErr
	}()
	<-cancelStarted
	select {
	case cancelErr := <-cancelDone:
		t.Fatalf("CancelMission passed the blocked WorkerAdapter.Start boundary: %v", cancelErr)
	case <-time.After(100 * time.Millisecond):
	}
	close(worker.releaseStart)
	if err = <-startDone; err != nil {
		t.Fatalf("StartMission: %v", err)
	}
	if err = <-cancelDone; err != nil {
		t.Fatalf("CancelMission after WorkerAdapter.Start returned: %v", err)
	}
	select {
	case <-worker.stopBeforeStartWasReturned:
		t.Fatal("WorkerAdapter.Stop ran before WorkerAdapter.Start returned")
	default:
	}
	details, err := runtime.MissionDetails(ctx, scope, mission.ID)
	if err != nil || details.State != "cancelled" {
		t.Fatalf("Mission state after serialized Start/Cancel = %q error=%v, want cancelled", details.State, err)
	}
}

func TestFailedWorkerStartReplayRequiresLifecycleRecovery(t *testing.T) {
	dsn := os.Getenv("POLIS_TEST_DSN")
	if dsn == "" {
		t.Skip("dedicated PostgreSQL required")
	}
	ctx := context.Background()
	runtime, err := kernel.Open(ctx, dsn, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Close()
	companyID := "lifecycle-start-failed-" + fmt.Sprint(time.Now().UnixNano())
	scope, err := runtime.TXCreateCompany(ctx, companyID)
	if err != nil {
		t.Fatal(err)
	}
	mission, err := runtime.TXCreateMissionGoal(ctx, scope, "Failed Worker start", "ensure start replay is fenced", "lifecycle-start-failed-create")
	if err != nil {
		t.Fatal(err)
	}
	worker := &failedStartLifecycleWorker{}
	service := NewService(runtime, worker)
	request := MissionCommandRequest{MissionID: mission.ID, RequestID: "lifecycle-start-failed-request"}
	if _, err = service.StartMission(ctx, companyID, request); err == nil {
		t.Fatal("StartMission succeeded after the WorkerAdapter start failed")
	}
	if _, err = service.StartMission(ctx, companyID, request); err == nil || worker.startCalls != 1 {
		t.Fatalf("failed StartMission replay should require pause/resume recovery without starting again: starts=%d err=%v", worker.startCalls, err)
	}
}

func TestStartMissionReplayKeepsConfirmedStartAfterWorkerFinishes(t *testing.T) {
	dsn := os.Getenv("POLIS_TEST_DSN")
	if dsn == "" {
		t.Skip("dedicated PostgreSQL required")
	}
	ctx := context.Background()
	runtime, err := kernel.Open(ctx, dsn, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Close()
	companyID := "lifecycle-start-finished-" + fmt.Sprint(time.Now().UnixNano())
	scope, err := runtime.TXCreateCompany(ctx, companyID)
	if err != nil {
		t.Fatal(err)
	}
	mission, err := runtime.TXCreateMissionGoal(ctx, scope, "Completed Worker start", "preserve the confirmed start receipt", "lifecycle-start-finished-create")
	if err != nil {
		t.Fatal(err)
	}
	worker := &recordingWorkerAdapter{}
	service := NewService(runtime, worker)
	request := MissionCommandRequest{MissionID: mission.ID, RequestID: "lifecycle-start-finished-request"}
	if _, err = service.StartMission(ctx, companyID, request); err != nil {
		t.Fatal(err)
	}
	worker.stopped = append(worker.stopped, companyID+"/"+mission.ID)
	replayed, err := service.StartMission(ctx, companyID, request)
	if err != nil || replayed.ResultingState != "active" || len(worker.started) != 1 {
		t.Fatalf("confirmed start replay after Worker completion = %+v starts=%v err=%v", replayed, worker.started, err)
	}
}

func TestStartMissionReplayAfterRuntimeRestartRequiresSessionReconciliation(t *testing.T) {
	dsn := os.Getenv("POLIS_TEST_DSN")
	if dsn == "" {
		t.Skip("dedicated PostgreSQL required")
	}
	ctx := context.Background()
	root := t.TempDir()
	runtime, err := kernel.Open(ctx, dsn, root)
	if err != nil {
		t.Fatal(err)
	}
	companyID := "lifecycle-start-restart-" + fmt.Sprint(time.Now().UnixNano())
	scope, err := runtime.TXCreateCompany(ctx, companyID)
	if err != nil {
		runtime.Close()
		t.Fatal(err)
	}
	mission, err := runtime.TXCreateMissionGoal(ctx, scope, "Restarted Worker start", "require persisted WorkerSession reconciliation", "lifecycle-start-restart-create")
	if err != nil {
		runtime.Close()
		t.Fatal(err)
	}
	startRequest := MissionCommandRequest{MissionID: mission.ID, RequestID: "lifecycle-start-restart-request"}
	firstWorker := &recordingWorkerAdapter{}
	firstService := NewService(runtime, firstWorker)
	if _, err = firstService.StartMission(ctx, companyID, startRequest); err != nil {
		firstService.Close()
		runtime.Close()
		t.Fatal(err)
	}
	if err = firstService.Close(); err != nil {
		runtime.Close()
		t.Fatal(err)
	}
	runtime.Close()

	restarted, err := kernel.Open(ctx, dsn, root)
	if err != nil {
		t.Fatal(err)
	}
	defer restarted.Close()
	restartedWorker := &recordingWorkerAdapter{}
	restartedService := NewService(restarted, restartedWorker)
	defer restartedService.Close()
	if _, err = restartedService.StartMission(ctx, companyID, startRequest); err == nil || !strings.Contains(err.Error(), "Host-side reconciliation must confirm") || !strings.Contains(err.Error(), "Workbench cannot verify or clear it") || len(restartedWorker.started) != 0 {
		t.Fatalf("Start replay after restart must identify reconciliation and avoid another Worker start: starts=%v err=%v", restartedWorker.started, err)
	}
}

type blockingStartLifecycleWorker struct {
	startEntered               chan struct{}
	releaseStart               chan struct{}
	startReturned              chan struct{}
	stopBeforeStartWasReturned chan struct{}
}

func (*blockingStartLifecycleWorker) Mode() string                    { return "test" }
func (*blockingStartLifecycleWorker) Readiness(context.Context) error { return nil }
func (*blockingStartLifecycleWorker) ToolSurface() provider.ToolSurface {
	return provider.ToolSurface{}
}
func (worker *blockingStartLifecycleWorker) Start(context.Context, string, string) error {
	close(worker.startEntered)
	<-worker.releaseStart
	close(worker.startReturned)
	return nil
}
func (worker *blockingStartLifecycleWorker) Stop(context.Context, string, string) error {
	select {
	case <-worker.startReturned:
	default:
		worker.stopBeforeStartWasReturned <- struct{}{}
	}
	return nil
}
func (*blockingStartLifecycleWorker) Close() {}

type failedStartLifecycleWorker struct{ startCalls int }

func (*failedStartLifecycleWorker) Mode() string                      { return "test" }
func (*failedStartLifecycleWorker) Readiness(context.Context) error   { return nil }
func (*failedStartLifecycleWorker) ToolSurface() provider.ToolSurface { return provider.ToolSurface{} }
func (worker *failedStartLifecycleWorker) Start(context.Context, string, string) error {
	worker.startCalls++
	return errors.New("fixture worker start failed")
}
func (*failedStartLifecycleWorker) Stop(context.Context, string, string) error { return nil }
func (*failedStartLifecycleWorker) Close()                                     {}

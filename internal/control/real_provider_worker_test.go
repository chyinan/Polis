// pattern: Imperative Shell
// pattern: Imperative Shell
package control

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"polis/internal/codex"
	"polis/internal/core"
	"polis/internal/kernel"
	"polis/internal/provider"
	"polis/internal/runner"
	"polis/internal/taskvalidation"
)

func TestRealProviderWorkerAdapterUsesOfflineRuntimeAndFinalizesWorkerLifecycle(t *testing.T) {
	dsn := os.Getenv("POLIS_R05B1_TEST_DSN")
	if dsn == "" {
		t.Skip("dedicated R0.5B1 PostgreSQL required")
	}
	ctx := context.Background()
	runtime := provider.NewFakeRuntime(provider.FakeRuntimeConfig{TurnDelay: 5 * time.Millisecond})
	k, err := kernel.Open(ctx, dsn, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer k.Close()
	company := "r05b1-provider-company"
	if _, err = k.TXCreateCompany(ctx, company); err != nil {
		t.Fatal(err)
	}
	adapter, err := NewRealProviderWorkerAdapter(k, runtime)
	if err != nil {
		t.Fatal(err)
	}
	productSurface := provider.ProductToolSurface()
	adapterSurface := adapter.ToolSurface()
	if adapterSurface.ToolCount != productSurface.ToolCount || adapterSurface.ManifestDigest != productSurface.ManifestDigest || adapterSurface.AggregateSchemaBytes != productSurface.AggregateSchemaBytes || adapterSurface.AggregateSchemaDigest != productSurface.AggregateSchemaDigest {
		t.Fatalf("production adapter surface differs from generic product registry: adapter=%+v product=%+v", adapterSurface, productSurface)
	}
	service := NewService(k, adapter)
	created, err := service.CreateMission(ctx, company, CreateMissionRequest{Title: "offline provider bridge", Goal: "persist one deterministic product artifact", RequestID: "bridge-create", AcceptanceContract: &taskvalidation.AcceptanceContract{Revision: taskvalidation.AcceptanceContractRevision, RequiredText: []string{"Mission ID: {{mission_id}}", "Task ID: {{task_id}}", "Acknowledgement:", "Task summary:"}}})
	if err != nil {
		t.Fatal(err)
	}
	started, err := service.StartMission(ctx, company, MissionCommandRequest{MissionID: created.TargetID, RequestID: "bridge-start"})
	if err != nil {
		t.Fatal(err)
	}
	if started.ResultingState != "active" || runtime.Stats().ProviderEgress != 0 {
		t.Fatalf("unexpected start/runtime state: %+v/%+v", started, runtime.Stats())
	}

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	var sessionState, taskState string
	var artifacts, terminalEvents int
	waitForProviderTerminal(t, ctx, pool, company, &sessionState, &taskState, &artifacts, &terminalEvents)
	if sessionState != "stopped" || taskState != "candidate" || artifacts != 1 || terminalEvents != 1 {
		t.Fatalf("unexpected product bridge state: session=%s task=%s artifacts=%d terminal_events=%d", sessionState, taskState, artifacts, terminalEvents)
	}
	if runtime.Stats().Reservations != 0 || runtime.Stats().ProviderEgress != 0 {
		t.Fatalf("offline bridge used provider: %+v", runtime.Stats())
	}
	var bindingDigest, runnerKind, runnerRevision string
	if err = pool.QueryRow(ctx, `SELECT v.configuration_digest,v.runner_kind,v.runner_revision
FROM task_validation_bindings v JOIN tasks t ON t.company_id=v.company_id AND t.id=v.task_id
WHERE v.company_id=$1 AND t.mission_id=$2`, company, created.TargetID).Scan(&bindingDigest, &runnerKind, &runnerRevision); err != nil {
		t.Fatal(err)
	}
	if len(bindingDigest) != 64 || runnerKind != taskvalidation.TextContainsAllRunnerKind || runnerRevision != taskvalidation.TextContainsAllRunnerRevision {
		t.Fatalf("unexpected task validation binding %s/%s/%s", bindingDigest, runnerKind, runnerRevision)
	}
	var checks, passedChecks, qualifications int
	if err = pool.QueryRow(ctx, "SELECT count(*),count(*) FILTER (WHERE passed) FROM worker_checks WHERE company_id=$1 AND phase='product'", company).Scan(&checks, &passedChecks); err != nil {
		t.Fatal(err)
	}
	if err = pool.QueryRow(ctx, "SELECT count(*) FROM task_validation_artifact_qualifications WHERE company_id=$1", company).Scan(&qualifications); err != nil {
		t.Fatal(err)
	}
	if checks != 1 || passedChecks != 1 || qualifications != 1 {
		t.Fatalf("product validation evidence checks=%d passed=%d artifact_qualifications=%d", checks, passedChecks, qualifications)
	}
	var checkReportRaw []byte
	if err = pool.QueryRow(ctx, "SELECT report FROM worker_checks WHERE company_id=$1 AND phase='product'", company).Scan(&checkReportRaw); err != nil {
		t.Fatal(err)
	}
	var checkReport taskvalidation.Result
	if json.Unmarshal(checkReportRaw, &checkReport) != nil || checkReport.Status != taskvalidation.StatusPass || checkReport.ConfigurationDigest != bindingDigest || checkReport.WorkspaceRevision != 2 {
		t.Fatalf("product check report is not bound to current Task: %+v", checkReport)
	}
	if _, err = service.StartMission(ctx, company, MissionCommandRequest{MissionID: created.TargetID, RequestID: "bridge-start-duplicate"}); !errors.Is(err, core.Conflict) {
		t.Fatalf("duplicate start error=%v, want %s", err, core.Conflict)
	}
	service.Close()
}

func TestLaunchQualificationAdapterAcceptsExplicitOfflineControlledMCPSurface(t *testing.T) {
	runtime := provider.NewFakeRuntime(provider.FakeRuntimeConfig{ControlledMCPToolSurface: true})
	adapter, err := NewRealProviderWorkerLaunchQualificationAdapter(runtime)
	if err != nil {
		t.Fatal(err)
	}
	if err = adapter.Readiness(context.Background()); err == nil {
		t.Fatal("controlled MCP readiness passed without an AppContainer owner factory")
	}
	order := []string{}
	adapter.mcpFactory = &controlledMCPFactoryFixture{order: &order, process: &controlledMCPProcessFixture{order: &order, digest: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}}
	if err = adapter.Readiness(context.Background()); err != nil {
		t.Fatalf("explicit fake controlled-MCP surface readiness: %v", err)
	}
	if adapter.ToolSurface().ManifestDigest != provider.ProductControlledMCPToolSurface().ManifestDigest {
		t.Fatalf("adapter surface=%+v, want isolated controlled-MCP surface", adapter.ToolSurface())
	}
}

func TestLaunchQualificationAdapterAcceptsExplicitOfflineDirectMessagingSurface(t *testing.T) {
	runtime := provider.NewFakeRuntime(provider.FakeRuntimeConfig{DirectMessagingSurface: true})
	profile := runtime.ExecutionProfile()
	adapter, err := NewRealProviderWorkerLaunchQualificationAdapter(runtime)
	if err != nil {
		t.Fatal(err)
	}
	if err = adapter.Readiness(context.Background()); err != nil {
		t.Fatalf("explicit fake direct-message surface readiness: %v", err)
	}
	if adapter.ToolSurface().ManifestDigest != provider.ProductDirectMessagingManifestDigest ||
		profile.ToolSurfaceQualification != provider.ProductDirectMessagingToolSurfaceQualification {
		t.Fatalf("adapter surface/profile=%+v/%+v, want the unqualified direct-message @7 profile", adapter.ToolSurface(), profile)
	}
}

func TestRealProviderWorkerAdapterMissingBindingCannotQualifyOrSubmit(t *testing.T) {
	dsn := os.Getenv("POLIS_R05B3_TEST_DSN")
	if dsn == "" {
		t.Skip("dedicated offline product PostgreSQL required")
	}
	ctx := context.Background()
	runtime := provider.NewFakeRuntime(provider.FakeRuntimeConfig{TurnDelay: time.Millisecond})
	k, err := kernel.Open(ctx, dsn, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer k.Close()
	company := "r05b3-no-validation-company"
	if _, err = k.TXCreateCompany(ctx, company); err != nil {
		t.Fatal(err)
	}
	adapter, err := NewRealProviderWorkerAdapter(k, runtime)
	if err != nil {
		t.Fatal(err)
	}
	service := NewService(k, adapter)
	created, err := service.CreateMission(ctx, company, CreateMissionRequest{Title: "exploratory task", Goal: "explore without a public acceptance contract", RequestID: "no-validation-create"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = service.StartMission(ctx, company, MissionCommandRequest{MissionID: created.TargetID, RequestID: "no-validation-start"}); err != nil {
		t.Fatal(err)
	}
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		var sessionState, taskState string
		var artifacts, checkpoints, checks int
		err = pool.QueryRow(ctx, "SELECT state FROM worker_sessions WHERE company_id=$1 ORDER BY id DESC LIMIT 1", company).Scan(&sessionState)
		if err == nil {
			err = pool.QueryRow(ctx, "SELECT state FROM tasks WHERE company_id=$1 AND kind='compat' ORDER BY id DESC LIMIT 1", company).Scan(&taskState)
		}
		if err == nil {
			err = pool.QueryRow(ctx, "SELECT count(*) FROM artifacts WHERE company_id=$1", company).Scan(&artifacts)
		}
		if err == nil {
			err = pool.QueryRow(ctx, "SELECT count(*) FROM worker_checkpoints c JOIN worker_sessions s ON s.company_id=c.company_id AND s.id=c.session_id WHERE c.company_id=$1", company).Scan(&checkpoints)
		}
		if err == nil {
			err = pool.QueryRow(ctx, "SELECT count(*) FROM worker_checks WHERE company_id=$1 AND phase='product' AND NOT passed AND report->>'status'='VALIDATION_NOT_CONFIGURED'", company).Scan(&checks)
		}
		if err == nil && sessionState == "stopped" && taskState == "working" && artifacts == 0 && checkpoints == 0 && checks == 1 {
			if runtime.Stats().ProviderEgress != 0 || runtime.Stats().Reservations != 0 {
				t.Fatalf("missing-binding offline run used provider: %+v", runtime.Stats())
			}
			service.Close()
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatalf("missing-binding task did not fail closed: err=%v", err)
}

func TestRealProviderWorkerAdapterCancelsOfflineTurnWithoutOrphan(t *testing.T) {
	dsn := os.Getenv("POLIS_R05B1_TEST_DSN")
	if dsn == "" {
		t.Skip("dedicated R0.5B1 PostgreSQL required")
	}
	ctx := context.Background()
	runtime := provider.NewFakeRuntime(provider.FakeRuntimeConfig{TurnDelay: 2 * time.Second})
	k, err := kernel.Open(ctx, dsn, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer k.Close()
	company := "r05b1-cancel-company"
	if _, err = k.TXCreateCompany(ctx, company); err != nil {
		t.Fatal(err)
	}
	adapter, err := NewRealProviderWorkerAdapter(k, runtime)
	if err != nil {
		t.Fatal(err)
	}
	service := NewService(k, adapter)
	created, err := service.CreateMission(ctx, company, CreateMissionRequest{Title: "cancel bridge", Goal: "cancel during an offline turn", RequestID: "cancel-create"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = service.StartMission(ctx, company, MissionCommandRequest{MissionID: created.TargetID, RequestID: "cancel-start"}); err != nil {
		t.Fatal(err)
	}
	if _, err = service.CancelMission(ctx, company, MissionCommandRequest{MissionID: created.TargetID, RequestID: "cancel-command"}); err != nil {
		t.Fatal(err)
	}

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	var live int
	if err = pool.QueryRow(ctx, "SELECT count(*) FROM worker_sessions WHERE company_id=$1 AND state!='stopped'", company).Scan(&live); err != nil {
		t.Fatal(err)
	}
	if live != 0 {
		t.Fatalf("orphan live sessions = %d", live)
	}
	if runtime.Stats().ProviderEgress != 0 {
		t.Fatalf("offline cancellation used provider: %+v", runtime.Stats())
	}
}

func TestRealProviderWorkerAdapterClosesReservationAfterPreTurnStop(t *testing.T) {
	dsn := os.Getenv("POLIS_R0_5B16_TEST_DSN")
	if dsn == "" {
		t.Skip("dedicated R0.5B16 PostgreSQL required")
	}
	ctx := context.Background()
	k, err := kernel.Open(ctx, dsn, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer k.Close()
	company := fmt.Sprintf("r05b16-reservation-stop-%d", time.Now().UTC().UnixNano())
	if _, err = k.TXCreateCompany(ctx, company); err != nil {
		t.Fatal(err)
	}
	runtime := &reservationTrackingRuntime{FakeRuntime: provider.NewFakeRuntime(provider.FakeRuntimeConfig{TurnDelay: 2 * time.Second})}
	adapter, err := NewRealProviderWorkerAdapter(k, runtime)
	if err != nil {
		t.Fatal(err)
	}
	service := NewService(k, adapter)
	created, err := service.CreateMission(ctx, company, CreateMissionRequest{
		Title:              "reservation lifecycle stop",
		Goal:               "close the reservation when a no-turn worker is stopped",
		RequestID:          "r0-5b16-reservation-create",
		AcceptanceContract: &taskvalidation.AcceptanceContract{Revision: taskvalidation.AcceptanceContractRevision, RequiredText: []string{"Mission ID: {{mission_id}}", "Task ID: {{task_id}}", "Acknowledgement:", "Task summary:"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = service.StartMission(ctx, company, MissionCommandRequest{MissionID: created.TargetID, RequestID: "r0-5b16-reservation-start"}); err != nil {
		t.Fatal(err)
	}
	if _, err = service.CancelMission(ctx, company, MissionCommandRequest{MissionID: created.TargetID, RequestID: "r0-5b16-reservation-stop"}); err != nil {
		t.Fatal(err)
	}
	if got := runtime.closeReasons(); len(got) != 1 || got[0] == "" {
		t.Fatalf("reservation close calls=%v, want one classified close", got)
	}
}

func TestRealProviderWorkerRetainsOwnershipWhenStopPersistenceFails(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("long-lived process fixture is Linux-specific")
	}
	dsn := os.Getenv("POLIS_TEST_DSN")
	if dsn == "" {
		t.Skip("dedicated temporary PostgreSQL required")
	}
	ctx := context.Background()
	k, err := kernel.Open(ctx, dsn, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer k.Close()
	companyID := fmt.Sprintf("r1-real-stop-retry-%x", time.Now().UTC().UnixNano())
	if _, err = k.TXCreateCompany(ctx, companyID); err != nil {
		t.Fatal(err)
	}
	runtimeAdapter, err := NewRealProviderWorkerAdapter(k, provider.NewFakeRuntime(provider.FakeRuntimeConfig{TurnDelay: time.Second}))
	if err != nil {
		t.Fatal(err)
	}
	missionID := "r1-stop-retry-mission"
	task, err := k.TXCreateProbe(ctx, k.LocalScope(companyID), missionID)
	if err != nil {
		t.Fatal(err)
	}
	binding, err := k.TXNewWorker(ctx, k.LocalScope(companyID), task.ID, "r1-stop-retry")
	if err != nil {
		t.Fatal(err)
	}
	if err = k.TXBindWorkerProcessHost(ctx, binding, runner.CurrentProcessContainmentMetadata()); err != nil {
		t.Fatal(err)
	}
	process, err := runner.Start(binding.SessionID(), []string{"/bin/sh", "-c", "exec /bin/sleep 60"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer process.Stop()
	session := &stopRetryProviderSession{process: process}
	if err = k.TXAttachWorker(ctx, binding, process); err != nil {
		t.Fatal(err)
	}
	if err = k.TXValidateWorker(ctx, binding); err != nil {
		t.Fatal(err)
	}
	if err = k.TXActivateWorker(ctx, binding, "r1-stop-retry"); err != nil {
		t.Fatal(err)
	}
	worker := &providerWorker{binding: binding, reservation: provider.Reservation{ID: "r1-stop-retry"}, session: session, cancel: func() {}, done: make(chan struct{}), ready: make(chan struct{})}
	key := companyID + "/" + missionID
	runtimeAdapter.workers[key] = worker
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if err = runtimeAdapter.Stop(canceled, companyID, missionID); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled stop error=%v, want context.Canceled", err)
	}
	if retained := runtimeAdapter.workers[key]; retained != worker {
		t.Fatal("provider worker ownership was discarded after the stop-state write failed")
	}
	if process.HasExited() {
		t.Fatal("failed stop-state write unexpectedly stopped the process")
	}
	if err = runtimeAdapter.Stop(ctx, companyID, missionID); err != nil {
		t.Fatalf("retry stop: %v", err)
	}
	if _, ok := runtimeAdapter.workers[key]; ok {
		t.Fatal("provider worker ownership remained after a confirmed stop")
	}
	if !process.HasExited() {
		t.Fatal("process remained alive after a confirmed stop")
	}
}

func TestRealProviderStartRetainsSessionWhenUnactivatedCleanupFails(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("long-lived process fixture is Linux-specific")
	}
	dsn := os.Getenv("POLIS_TEST_DSN")
	if dsn == "" {
		t.Skip("dedicated temporary PostgreSQL required")
	}
	ctx := context.Background()
	k, err := kernel.Open(ctx, dsn, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer k.Close()
	companyID := fmt.Sprintf("r1-start-stop-retry-%x", time.Now().UTC().UnixNano())
	if _, err = k.TXCreateCompany(ctx, companyID); err != nil {
		t.Fatal(err)
	}
	startCtx, cancelStart := context.WithCancel(ctx)
	runtimeAdapter := &cancelAfterProviderStartRuntime{Runtime: provider.NewFakeRuntime(provider.FakeRuntimeConfig{TurnDelay: time.Second}), cancel: cancelStart}
	adapter, err := NewRealProviderWorkerAdapter(k, runtimeAdapter)
	if err != nil {
		t.Fatal(err)
	}
	service := NewService(k, adapter)
	defer service.Close()
	created, err := service.CreateMission(ctx, companyID, CreateMissionRequest{
		Title: "retry unactivated provider stop", Goal: "retain startup process ownership after the first stop fails",
		RequestID: companyID + "-create", AcceptanceContract: r05b5AcceptanceContractPointer(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = k.TXStartMissionCommand(ctx, k.LocalScope(companyID), created.TargetID, companyID+"-mission-start"); err != nil {
		t.Fatal(err)
	}
	if err = adapter.Start(startCtx, companyID, created.TargetID); err == nil {
		t.Fatal("provider start unexpectedly succeeded after its attach context was canceled")
	}
	if runtimeAdapter.session == nil || runtimeAdapter.session.Process() == nil {
		t.Fatal("provider start did not retain its created process fixture")
	}
	process := runtimeAdapter.session.Process()
	defer func() {
		if !process.HasExited() {
			_, _ = runtimeAdapter.session.Stop(context.Background())
		}
	}()
	if process.HasExited() {
		t.Fatal("injected first cleanup failure unexpectedly stopped the provider process")
	}
	if err = adapter.Stop(ctx, companyID, created.TargetID); err != nil {
		t.Fatalf("retry unactivated provider stop: %v", err)
	}
	if !process.HasExited() {
		t.Fatal("provider process remained alive after its retained owner retried cleanup")
	}
}

func TestRealProviderStartRetainsReservationWhenNoProcessCleanupFails(t *testing.T) {
	dsn := os.Getenv("POLIS_TEST_DSN")
	if dsn == "" {
		t.Skip("dedicated temporary PostgreSQL required")
	}
	ctx := context.Background()
	k, err := kernel.Open(ctx, dsn, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer k.Close()
	companyID := fmt.Sprintf("r1-no-process-close-retry-%x", time.Now().UTC().UnixNano())
	if _, err = k.TXCreateCompany(ctx, companyID); err != nil {
		t.Fatal(err)
	}
	providerRuntime := &failFirstReservationCloseRuntime{
		Runtime:  provider.NewFakeRuntime(provider.FakeRuntimeConfig{TurnDelay: time.Second}),
		startErr: errors.New("injected provider process creation failure"),
	}
	adapter, err := NewRealProviderWorkerAdapter(k, providerRuntime)
	if err != nil {
		t.Fatal(err)
	}
	service := NewService(k, adapter)
	defer service.Close()
	created, err := service.CreateMission(ctx, companyID, CreateMissionRequest{
		Title: "retry no-process reservation close", Goal: "retain the provider reservation after stop finalization succeeds",
		RequestID: companyID + "-create", AcceptanceContract: r05b5AcceptanceContractPointer(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = k.TXStartMissionCommand(ctx, k.LocalScope(companyID), created.TargetID, companyID+"-mission-start"); err != nil {
		t.Fatal(err)
	}
	if err = adapter.Start(ctx, companyID, created.TargetID); err == nil {
		t.Fatal("provider start unexpectedly succeeded without creating a process")
	}
	if calls := providerRuntime.closeCallCount(); calls != 1 {
		t.Fatalf("reservation close calls after injected failure=%d, want the initial attempt", calls)
	}
	if err = adapter.Stop(ctx, companyID, created.TargetID); err != nil {
		t.Fatalf("retry no-process cleanup: %v", err)
	}
	if calls := providerRuntime.closeCallCount(); calls != 2 {
		t.Fatalf("reservation close calls after retry=%d, want 2", calls)
	}
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	var state string
	if err = pool.QueryRow(ctx, "SELECT state FROM worker_sessions WHERE company_id=$1", companyID).Scan(&state); err != nil {
		t.Fatal(err)
	}
	if state != "stopped" {
		t.Fatalf("no-process WorkerSession state=%q after reservation cleanup retry", state)
	}
}

func TestRealProviderStopReplaysConfirmationAfterAmbiguousCommit(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("long-lived process fixture is Linux-specific")
	}
	dsn := os.Getenv("POLIS_TEST_DSN")
	if dsn == "" {
		t.Skip("dedicated temporary PostgreSQL required")
	}
	ctx := context.Background()
	k, err := kernel.Open(ctx, dsn, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer k.Close()
	companyID := fmt.Sprintf("r1-provider-stop-confirm-replay-%x", time.Now().UTC().UnixNano())
	scope, err := k.TXCreateCompany(ctx, companyID)
	if err != nil {
		t.Fatal(err)
	}
	task, err := k.TXCreateProbe(ctx, scope, "r1-provider-stop-confirm-replay-task")
	if err != nil {
		t.Fatal(err)
	}
	binding, err := k.TXNewWorker(ctx, scope, task.ID, "r1-provider-stop-confirm-replay")
	if err != nil {
		t.Fatal(err)
	}
	if err = k.TXBindWorkerProcessHost(ctx, binding, runner.CurrentProcessContainmentMetadata()); err != nil {
		t.Fatal(err)
	}
	process, err := runner.Start(binding.SessionID(), []string{"/bin/sh", "-c", "exec /bin/sleep 60"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer process.Stop()
	if err = k.TXAttachWorker(ctx, binding, process); err != nil {
		t.Fatal(err)
	}
	if err = k.TXBeginStop(ctx, binding); err != nil {
		t.Fatal(err)
	}
	proof, err := process.Stop()
	if err != nil {
		t.Fatal(err)
	}
	if err = k.TXConfirmStopped(ctx, binding, proof); err != nil {
		t.Fatal(err)
	}
	runtimeAdapter, err := NewRealProviderWorkerAdapter(k, provider.NewFakeRuntime(provider.FakeRuntimeConfig{}))
	if err != nil {
		t.Fatal(err)
	}
	missionID := "r1-provider-stop-confirm-replay-mission"
	worker := &providerWorker{
		companyID: companyID, missionID: missionID, binding: binding,
		reservation: provider.Reservation{ID: "offline-no-provider-reservation"},
		session:     &stopRetryProviderSession{process: process}, cancel: func() {},
		processStopped: true, stopProof: proof, done: make(chan struct{}), runDone: make(chan struct{}), ready: make(chan struct{}),
	}
	key := companyID + "/" + missionID
	runtimeAdapter.workers[key] = worker
	if err = runtimeAdapter.Stop(ctx, companyID, missionID); err != nil {
		t.Fatalf("Provider adapter could not replay the committed stop: %v", err)
	}
	if _, exists := runtimeAdapter.workers[key]; exists {
		t.Fatal("Provider Worker ownership remained after replayed stop confirmation")
	}
}

func TestBusinessPreflightRetainsOwnerWhenProcessStopFails(t *testing.T) {
	dsn := os.Getenv("POLIS_TEST_DSN")
	if dsn == "" {
		t.Skip("dedicated temporary PostgreSQL required")
	}
	ctx := context.Background()
	k, err := kernel.Open(ctx, dsn, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer k.Close()
	companyID := fmt.Sprintf("r1-preflight-stop-retry-%x", time.Now().UTC().UnixNano())
	if _, err = k.TXCreateCompany(ctx, companyID); err != nil {
		t.Fatal(err)
	}
	startCtx, cancelStart := context.WithCancel(ctx)
	runtimeAdapter := &cancelAfterLocalQualificationRuntime{localQualificationFakeRuntime: newLocalQualificationFakeRuntime(), cancel: cancelStart}
	adapter, err := NewRealProviderWorkerAdapter(k, runtimeAdapter)
	if err != nil {
		t.Fatal(err)
	}
	service := NewService(k, adapter)
	defer service.Close()
	created, err := service.CreateMission(ctx, companyID, CreateMissionRequest{
		Title: "preflight stop retry", Goal: "retain a business preflight process when cleanup first fails",
		RequestID: companyID + "-create", AcceptanceContract: r05b5AcceptanceContractPointer(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = k.TXStartMissionCommand(ctx, k.LocalScope(companyID), created.TargetID, companyID+"-mission-start"); err != nil {
		t.Fatal(err)
	}
	if _, err = adapter.QualifyLocalBusinessContext(startCtx, companyID, created.TargetID); err == nil {
		t.Fatal("preflight unexpectedly passed after its process-attach context was canceled")
	}
	if runtimeAdapter.session == nil || runtimeAdapter.session.Process() == nil {
		t.Fatal("preflight did not retain its started session fixture")
	}
	process := runtimeAdapter.session.Process()
	defer func() {
		if !process.HasExited() {
			_, _ = runtimeAdapter.session.Stop(context.Background())
		}
	}()
	if process.HasExited() {
		t.Fatal("injected first preflight stop failure unexpectedly stopped the process")
	}
	if err = adapter.Stop(ctx, companyID, created.TargetID); err != nil {
		t.Fatalf("retry retained preflight process cleanup: %v", err)
	}
	if !process.HasExited() {
		t.Fatal("preflight process remained alive after owner retry")
	}
}

func TestLocalLaunchQualificationRetainsSessionWhenStopFails(t *testing.T) {
	ctx := context.Background()
	runtimeAdapter := &cancelAfterLocalQualificationRuntime{localQualificationFakeRuntime: newLocalQualificationFakeRuntime()}
	adapter, err := NewRealProviderWorkerLaunchQualificationAdapter(runtimeAdapter)
	if err != nil {
		t.Fatal(err)
	}
	defer adapter.Close()
	const sessionID = "r1-local-launch-stop-retry"
	if err = adapter.QualifyLocalLaunch(ctx, sessionID); err == nil {
		t.Fatal("local launch unexpectedly succeeded after its first stop failed")
	}
	if runtimeAdapter.session == nil || runtimeAdapter.session.Process() == nil {
		t.Fatal("local launch did not retain the process fixture")
	}
	process := runtimeAdapter.session.Process()
	if process.HasExited() {
		t.Fatal("injected first stop failure unexpectedly stopped the local process")
	}
	if err = adapter.RetryLocalLaunchCleanup(ctx, sessionID); err != nil {
		t.Fatalf("retry local launch cleanup: %v", err)
	}
	if !process.HasExited() {
		t.Fatal("local qualification process remained alive after retry")
	}
}

type failFirstReservationCloseRuntime struct {
	provider.Runtime
	startErr error
	mu       sync.Mutex
	closes   int
}

func (runtime *failFirstReservationCloseRuntime) Start(context.Context, provider.SessionStartOptions) (provider.Session, error) {
	return nil, runtime.startErr
}

func (runtime *failFirstReservationCloseRuntime) CloseReservation(ctx context.Context, reservation provider.Reservation, reason string) error {
	runtime.mu.Lock()
	runtime.closes++
	call := runtime.closes
	runtime.mu.Unlock()
	if call == 1 {
		return errors.New("injected reservation close failure")
	}
	return runtime.Runtime.CloseReservation(ctx, reservation, reason)
}

func (runtime *failFirstReservationCloseRuntime) closeCallCount() int {
	runtime.mu.Lock()
	defer runtime.mu.Unlock()
	return runtime.closes
}

type cancelAfterProviderStartRuntime struct {
	provider.Runtime
	cancel  context.CancelFunc
	session provider.Session
}

func (runtime *cancelAfterProviderStartRuntime) Start(ctx context.Context, options provider.SessionStartOptions) (provider.Session, error) {
	session, err := runtime.Runtime.Start(ctx, options)
	if err != nil {
		return nil, err
	}
	runtime.session = &failFirstStopProviderSession{Session: session}
	if runtime.cancel != nil {
		runtime.cancel()
	}
	return runtime.session, nil
}

type failFirstStopProviderSession struct {
	provider.Session
	mu     sync.Mutex
	failed bool
}

func (session *failFirstStopProviderSession) Stop(ctx context.Context) (runner.StopProof, error) {
	session.mu.Lock()
	if !session.failed {
		session.failed = true
		session.mu.Unlock()
		return runner.StopProof{}, errors.New("injected unactivated stop failure")
	}
	session.mu.Unlock()
	return session.Session.Stop(ctx)
}

func r05b5AcceptanceContractPointer() *taskvalidation.AcceptanceContract {
	contract := r05b5AcceptanceContract()
	return &contract
}

type stopRetryProviderSession struct {
	process *runner.Process
}

func (session *stopRetryProviderSession) Process() *runner.Process                        { return session.process }
func (*stopRetryProviderSession) Initialize(context.Context, codex.TransportPolicy) error { return nil }
func (*stopRetryProviderSession) StartThread(context.Context, provider.ThreadStartOptions) (string, error) {
	return "stop-retry-thread", nil
}
func (*stopRetryProviderSession) Turn(context.Context, string, string, codex.TurnOptions, provider.ToolHandler) (provider.TurnResult, error) {
	return provider.TurnResult{}, nil
}
func (session *stopRetryProviderSession) Stop(ctx context.Context) (runner.StopProof, error) {
	if err := ctx.Err(); err != nil {
		return runner.StopProof{}, err
	}
	return session.process.Stop()
}

func TestRealProviderWorkerAdapterReservationBearingPreflightStopsBeforeTurn(t *testing.T) {
	dsn := os.Getenv("POLIS_R0_5B16_TEST_DSN")
	if dsn == "" {
		t.Skip("dedicated R0.5B16 PostgreSQL required")
	}
	ctx := context.Background()
	k, err := kernel.Open(ctx, dsn, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer k.Close()
	company := fmt.Sprintf("r05b16-reservation-preflight-%d", time.Now().UTC().UnixNano())
	if _, err = k.TXCreateCompany(ctx, company); err != nil {
		t.Fatal(err)
	}
	runtime := &reservationTrackingRuntime{FakeRuntime: provider.NewFakeRuntime(provider.FakeRuntimeConfig{TurnDelay: 2 * time.Second})}
	adapter, err := NewRealProviderWorkerAdapter(k, runtime)
	if err != nil {
		t.Fatal(err)
	}
	service := NewService(k, adapter)
	created, err := service.CreateMission(ctx, company, CreateMissionRequest{
		Title:              "reservation bridge preflight",
		Goal:               "reach initialize and thread start without a provider turn",
		RequestID:          "r0-5b16-preflight-create",
		AcceptanceContract: &taskvalidation.AcceptanceContract{Revision: taskvalidation.AcceptanceContractRevision, RequiredText: []string{"Mission ID: {{mission_id}}", "Task ID: {{task_id}}", "Acknowledgement:", "Task summary:"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = k.TXStartMissionCommand(ctx, k.LocalScope(company), created.TargetID, "r0-5b16-preflight-start"); err != nil {
		t.Fatal(err)
	}
	qualifier, ok := any(adapter).(interface {
		QualifyLocalReservedBusinessContext(context.Context, string, string) (BusinessContextPreflightResult, error)
	})
	if !ok {
		t.Fatal("real provider adapter has no reservation-bearing local qualification boundary")
	}
	result, err := qualifier.QualifyLocalReservedBusinessContext(ctx, company, created.TargetID)
	if err != nil {
		t.Fatalf("reservation-bearing preflight result=%+v error=%v", result, err)
	}
	if !result.ProcessStarted || !result.InitializePassed || !result.ThreadStarted || !result.CleanStop {
		t.Fatalf("reservation-bearing preflight did not reach clean thread boundary: %+v", result)
	}
	if got := runtime.closeReasons(); len(got) != 1 || got[0] == "" {
		t.Fatalf("reservation close calls=%v, want one classified close", got)
	}
	if got := runtime.turnCalls(); got != 0 {
		t.Fatalf("reservation-bearing preflight started %d turns", got)
	}
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	rows, err := pool.Query(ctx, "SELECT payload->>'status' FROM events WHERE company_id=$1 AND kind='provider.runtime.lifecycle' ORDER BY company_seq", company)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var phases []string
	for rows.Next() {
		var phase string
		if err = rows.Scan(&phase); err != nil {
			t.Fatal(err)
		}
		phases = append(phases, phase)
	}
	if err = rows.Err(); err != nil {
		t.Fatal(err)
	}
	wantPhases := []string{"adapter_start_entered", "reservation_bound", "process_created", "process_attached"}
	if len(phases) < len(wantPhases) {
		t.Fatalf("lifecycle phases=%v, want at least %v", phases, wantPhases)
	}
	for index, phase := range wantPhases {
		if phases[index] != phase {
			t.Fatalf("lifecycle phases=%v, want prefix %v", phases, wantPhases)
		}
	}
}

type reservationTrackingRuntime struct {
	*provider.FakeRuntime
	mu        sync.Mutex
	closed    []string
	turnCount int
}

func (r *reservationTrackingRuntime) ExecutionProfile() provider.ExecutionProfile {
	profile := r.FakeRuntime.ExecutionProfile()
	profile.Purpose = provider.ProductReservationBridgeQualificationPurpose
	return profile
}

func (r *reservationTrackingRuntime) Reserve(ctx context.Context, _ provider.ExecutionAuthorization) (provider.Reservation, error) {
	if err := ctx.Err(); err != nil {
		return provider.Reservation{}, err
	}
	return provider.Reservation{ID: "offline-reservation-bridge"}, nil
}

func (r *reservationTrackingRuntime) Start(ctx context.Context, options provider.SessionStartOptions) (provider.Session, error) {
	session, err := r.FakeRuntime.Start(ctx, options)
	if err != nil {
		return nil, err
	}
	return reservationTrackingSession{Session: session, runtime: r}, nil
}

func (r *reservationTrackingRuntime) CloseReservation(_ context.Context, _ provider.Reservation, reason string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.closed = append(r.closed, reason)
	return nil
}

func (r *reservationTrackingRuntime) closeReasons() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.closed...)
}

func (r *reservationTrackingRuntime) turnCalls() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.turnCount
}

type reservationTrackingSession struct {
	provider.Session
	runtime *reservationTrackingRuntime
}

func (s reservationTrackingSession) Turn(ctx context.Context, thread, prompt string, options codex.TurnOptions, handler provider.ToolHandler) (provider.TurnResult, error) {
	s.runtime.mu.Lock()
	s.runtime.turnCount++
	s.runtime.mu.Unlock()
	return s.Session.Turn(ctx, thread, prompt, options, handler)
}

func TestRealProviderWorkerAdapterProcessStartFailureFinalizesSession(t *testing.T) {
	dsn := os.Getenv("POLIS_R05B1_TEST_DSN")
	if dsn == "" {
		t.Skip("dedicated R0.5B1 PostgreSQL required")
	}
	ctx := context.Background()
	k, err := kernel.Open(ctx, dsn, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer k.Close()
	company := "r05b1-start-failure-company"
	if _, err = k.TXCreateCompany(ctx, company); err != nil {
		t.Fatal(err)
	}
	base := provider.NewFakeRuntime(provider.FakeRuntimeConfig{})
	adapter, err := NewRealProviderWorkerAdapter(k, startFailureRuntime{Runtime: base})
	if err != nil {
		t.Fatal(err)
	}
	service := NewService(k, adapter)
	created, err := service.CreateMission(ctx, company, CreateMissionRequest{Title: "process failure bridge", Goal: "fail before provider process start", RequestID: "failure-create"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = service.StartMission(ctx, company, MissionCommandRequest{MissionID: created.TargetID, RequestID: "failure-start"}); err == nil {
		t.Fatal("provider process start failure was accepted")
	}
	var live, stopped int
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if err = pool.QueryRow(ctx, "SELECT count(*) FILTER (WHERE state!='stopped'), count(*) FILTER (WHERE state='stopped') FROM worker_sessions WHERE company_id=$1", company).Scan(&live, &stopped); err != nil {
		t.Fatal(err)
	}
	if live != 0 || stopped != 1 {
		t.Fatalf("failed provider start left invalid session state: live=%d stopped=%d", live, stopped)
	}
}

func TestRealProviderWorkerAdapterDisconnectIsInconclusiveAndStopped(t *testing.T) {
	dsn := os.Getenv("POLIS_R05B1_TEST_DSN")
	if dsn == "" {
		t.Skip("dedicated R0.5B1 PostgreSQL required")
	}
	ctx := context.Background()
	k, err := kernel.Open(ctx, dsn, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer k.Close()
	company := "r05b1-disconnect-company"
	if _, err = k.TXCreateCompany(ctx, company); err != nil {
		t.Fatal(err)
	}
	base := provider.NewFakeRuntime(provider.FakeRuntimeConfig{})
	adapter, err := NewRealProviderWorkerAdapter(k, disconnectRuntime{Runtime: base})
	if err != nil {
		t.Fatal(err)
	}
	service := NewService(k, adapter)
	created, err := service.CreateMission(ctx, company, CreateMissionRequest{Title: "disconnect bridge", Goal: "classify transport disconnect", RequestID: "disconnect-create"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = service.StartMission(ctx, company, MissionCommandRequest{MissionID: created.TargetID, RequestID: "disconnect-start"}); err != nil {
		t.Fatal(err)
	}
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	waitForTerminalEvent(t, ctx, pool, company, "provider.turn.inconclusive")
	var state string
	if err = pool.QueryRow(ctx, "SELECT state FROM worker_sessions WHERE company_id=$1 ORDER BY id DESC LIMIT 1", company).Scan(&state); err != nil {
		t.Fatal(err)
	}
	if state != "stopped" {
		t.Fatalf("disconnect session state=%s, want stopped", state)
	}
}

func TestRealProviderWorkerAdapterInitializationFailureUsesInitializationEvent(t *testing.T) {
	dsn := os.Getenv("POLIS_R05B15_TEST_DSN")
	if dsn == "" {
		t.Skip("dedicated R0.5B15 PostgreSQL required")
	}
	ctx := context.Background()
	k, err := kernel.Open(ctx, dsn, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer k.Close()
	company := fmt.Sprintf("r05b15-initialize-failure-%d", time.Now().UTC().UnixNano())
	if _, err = k.TXCreateCompany(ctx, company); err != nil {
		t.Fatal(err)
	}
	base := provider.NewFakeRuntime(provider.FakeRuntimeConfig{})
	adapter, err := NewRealProviderWorkerAdapter(k, initializationFailureRuntime{Runtime: base})
	if err != nil {
		t.Fatal(err)
	}
	service := NewService(k, adapter)
	created, err := service.CreateMission(ctx, company, CreateMissionRequest{
		Title:              "initialize failure classification",
		Goal:               "exercise the business provider initialization boundary",
		RequestID:          "r0-5b15-initialize-create",
		AcceptanceContract: &taskvalidation.AcceptanceContract{Revision: taskvalidation.AcceptanceContractRevision, RequiredText: []string{"Mission ID: {{mission_id}}", "Task ID: {{task_id}}", "Acknowledgement:", "Task summary:"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = service.StartMission(ctx, company, MissionCommandRequest{MissionID: created.TargetID, RequestID: "r0-5b15-initialize-start"}); err != nil {
		t.Fatal(err)
	}
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		var expected, turnFailure int
		if err = pool.QueryRow(ctx, "SELECT count(*) FILTER (WHERE kind='provider.runtime.initialization_failed'), count(*) FILTER (WHERE kind='provider.turn.inconclusive') FROM events WHERE company_id=$1", company).Scan(&expected, &turnFailure); err != nil {
			t.Fatal(err)
		}
		if turnFailure > 0 {
			t.Fatalf("initialize failure was reported as a provider turn failure: expected=%d turn_failure=%d", expected, turnFailure)
		}
		if expected == 1 {
			var failureCategory string
			if err = pool.QueryRow(ctx, "SELECT COALESCE(data->>'failure_category','') FROM worker_observations WHERE company_id=$1 AND reason='provider_initialization' ORDER BY id DESC LIMIT 1", company).Scan(&failureCategory); err != nil {
				t.Fatal(err)
			}
			if failureCategory == "" {
				t.Fatal("initialize failure observation has no safe failure_category")
			}
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatal("timed out waiting for provider.runtime.initialization_failed")
}

func TestRealProviderWorkerAdapterBusinessContextLocalPreflightAvoidsReservation(t *testing.T) {
	dsn := os.Getenv("POLIS_R05B15_TEST_DSN")
	if dsn == "" {
		t.Skip("dedicated R0.5B15 PostgreSQL required")
	}
	ctx := context.Background()
	k, err := kernel.Open(ctx, dsn, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer k.Close()
	company := fmt.Sprintf("r05b15-business-preflight-%d", time.Now().UTC().UnixNano())
	if _, err = k.TXCreateCompany(ctx, company); err != nil {
		t.Fatal(err)
	}
	runtime := newLocalQualificationFakeRuntime()
	adapter, err := NewRealProviderWorkerAdapter(k, runtime)
	if err != nil {
		t.Fatal(err)
	}
	service := NewService(k, adapter)
	created, err := service.CreateMission(ctx, company, CreateMissionRequest{
		Title:              "business context local preflight",
		Goal:               "reach initialize and thread start without a provider turn",
		RequestID:          "r0-5b15-preflight-create",
		AcceptanceContract: &taskvalidation.AcceptanceContract{Revision: taskvalidation.AcceptanceContractRevision, RequiredText: []string{"Mission ID: {{mission_id}}", "Task ID: {{task_id}}", "Acknowledgement:", "Task summary:"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = k.TXStartMissionCommand(ctx, k.LocalScope(company), created.TargetID, "r0-5b15-preflight-start"); err != nil {
		t.Fatal(err)
	}
	result, err := adapter.QualifyLocalBusinessContext(ctx, company, created.TargetID)
	if err != nil {
		t.Fatalf("business preflight result=%+v error type=%T error=%q detail=%+v", result, err, err, err)
	}
	if !result.ProcessStarted || !result.InitializePassed || !result.ThreadStarted || result.RegisteredToolCount != 7 || !result.BusinessBindingPassed || !result.CleanStop {
		t.Fatalf("local business preflight did not pass its boundary: %+v", result)
	}
	if stats := runtime.Stats(); stats.Reservations != 0 || stats.ProviderEgress != 0 {
		t.Fatalf("local business preflight used provider accounting: %+v", stats)
	}
}

func TestRealProviderWorkerAdapterThreadStartFailureUsesThreadStartEvent(t *testing.T) {
	dsn := os.Getenv("POLIS_R05B15_TEST_DSN")
	if dsn == "" {
		t.Skip("dedicated R0.5B15 PostgreSQL required")
	}
	ctx := context.Background()
	k, err := kernel.Open(ctx, dsn, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer k.Close()
	company := fmt.Sprintf("r05b15-thread-failure-%d", time.Now().UTC().UnixNano())
	if _, err = k.TXCreateCompany(ctx, company); err != nil {
		t.Fatal(err)
	}
	base := provider.NewFakeRuntime(provider.FakeRuntimeConfig{})
	adapter, err := NewRealProviderWorkerAdapter(k, threadStartFailureRuntime{Runtime: base})
	if err != nil {
		t.Fatal(err)
	}
	service := NewService(k, adapter)
	created, err := service.CreateMission(ctx, company, CreateMissionRequest{Title: "thread start failure classification", Goal: "classify thread start before a turn", RequestID: "r0-5b15-thread-create", AcceptanceContract: &taskvalidation.AcceptanceContract{Revision: taskvalidation.AcceptanceContractRevision, RequiredText: []string{"Mission ID: {{mission_id}}", "Task ID: {{task_id}}", "Acknowledgement:", "Task summary:"}}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = service.StartMission(ctx, company, MissionCommandRequest{MissionID: created.TargetID, RequestID: "r0-5b15-thread-start"}); err != nil {
		t.Fatal(err)
	}
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		var expected, turnFailure int
		if err = pool.QueryRow(ctx, "SELECT count(*) FILTER (WHERE kind='provider.runtime.thread_start_failed'), count(*) FILTER (WHERE kind='provider.turn.inconclusive') FROM events WHERE company_id=$1", company).Scan(&expected, &turnFailure); err != nil {
			t.Fatal(err)
		}
		if turnFailure > 0 {
			t.Fatalf("thread start failure was reported as a provider turn failure: expected=%d turn_failure=%d", expected, turnFailure)
		}
		if expected == 1 {
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatal("timed out waiting for provider.runtime.thread_start_failed")
}

type startFailureRuntime struct{ provider.Runtime }

func (startFailureRuntime) Mode() string { return "fake" }
func (startFailureRuntime) Start(context.Context, provider.SessionStartOptions) (provider.Session, error) {
	return nil, errors.New("offline provider process start failed")
}

type disconnectRuntime struct{ provider.Runtime }

func (disconnectRuntime) Mode() string { return "fake" }
func (r disconnectRuntime) Start(ctx context.Context, options provider.SessionStartOptions) (provider.Session, error) {
	session, err := r.Runtime.Start(ctx, options)
	if err != nil {
		return nil, err
	}
	return disconnectSession{Session: session}, nil
}

type disconnectSession struct{ provider.Session }

func (disconnectSession) Turn(context.Context, string, string, codex.TurnOptions, provider.ToolHandler) (provider.TurnResult, error) {
	return provider.TurnResult{State: "failed", Outcome: "transport_disconnect", ProviderEgress: 0}, errors.New("offline transport disconnected before terminal")
}

type initializationFailureRuntime struct{ provider.Runtime }

func (r initializationFailureRuntime) Mode() string { return "fake" }
func (r initializationFailureRuntime) Start(ctx context.Context, options provider.SessionStartOptions) (provider.Session, error) {
	session, err := r.Runtime.Start(ctx, options)
	if err != nil {
		return nil, err
	}
	return initializationFailureSession{Session: session}, nil
}

type initializationFailureSession struct{ provider.Session }

func (initializationFailureSession) Initialize(context.Context, codex.TransportPolicy) error {
	return errors.New("offline initialize failed before protocol acknowledgement")
}

type threadStartFailureRuntime struct{ provider.Runtime }

func (threadStartFailureRuntime) Mode() string { return "fake" }
func (r threadStartFailureRuntime) Start(ctx context.Context, options provider.SessionStartOptions) (provider.Session, error) {
	session, err := r.Runtime.Start(ctx, options)
	if err != nil {
		return nil, err
	}
	return threadStartFailureSession{Session: session}, nil
}

type threadStartFailureSession struct{ provider.Session }

func (threadStartFailureSession) StartThread(context.Context, provider.ThreadStartOptions) (string, error) {
	return "", errors.New("offline thread start failed before a turn")
}

type localQualificationFakeRuntime struct {
	*provider.FakeRuntime
	profile provider.ExecutionProfile
}

type cancelAfterLocalQualificationRuntime struct {
	*localQualificationFakeRuntime
	cancel  context.CancelFunc
	session provider.Session
}

func (runtime *cancelAfterLocalQualificationRuntime) StartLocalQualification(ctx context.Context, options provider.SessionStartOptions) (provider.Session, error) {
	session, err := runtime.localQualificationFakeRuntime.StartLocalQualification(ctx, options)
	if err != nil {
		return nil, err
	}
	runtime.session = &failFirstStopProviderSession{Session: session}
	if runtime.cancel != nil {
		runtime.cancel()
	}
	return runtime.session, nil
}

func newLocalQualificationFakeRuntime() *localQualificationFakeRuntime {
	base := provider.NewFakeRuntime(provider.FakeRuntimeConfig{TurnDelay: time.Millisecond})
	profile := base.ExecutionProfile()
	profile.Purpose = provider.ProductLocalProcessLaunchQualificationPurpose
	profile.ToolCallLimit = 0
	return &localQualificationFakeRuntime{FakeRuntime: base, profile: profile}
}

func (r *localQualificationFakeRuntime) ExecutionProfile() provider.ExecutionProfile {
	return r.profile
}
func (r *localQualificationFakeRuntime) StartLocalQualification(ctx context.Context, options provider.SessionStartOptions) (provider.Session, error) {
	return r.FakeRuntime.Start(ctx, options)
}

func waitForTerminalEvent(t *testing.T, ctx context.Context, pool *pgxpool.Pool, company, kind string) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		var count int
		if err := pool.QueryRow(ctx, "SELECT count(*) FROM events WHERE company_id=$1 AND kind=$2", company, kind).Scan(&count); err == nil && count == 1 {
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", kind)
}

func waitForProviderTerminal(t *testing.T, ctx context.Context, pool *pgxpool.Pool, company string, sessionState, taskState *string, artifacts, terminalEvents *int) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		var eventCount int
		var artifactCount int
		var currentSession, currentTask string
		err := pool.QueryRow(ctx, "SELECT state FROM worker_sessions WHERE company_id=$1 ORDER BY id DESC LIMIT 1", company).Scan(&currentSession)
		if err == nil {
			err = pool.QueryRow(ctx, "SELECT state FROM tasks WHERE company_id=$1 AND kind='compat' ORDER BY id DESC LIMIT 1", company).Scan(&currentTask)
		}
		if err == nil {
			err = pool.QueryRow(ctx, "SELECT count(*) FROM artifacts WHERE company_id=$1", company).Scan(&artifactCount)
		}
		if err == nil {
			err = pool.QueryRow(ctx, "SELECT count(*) FROM events WHERE company_id=$1 AND kind='provider.turn.completed'", company).Scan(&eventCount)
		}
		if err == nil && currentSession == "stopped" && currentTask == "candidate" && artifactCount == 1 && eventCount == 1 {
			*sessionState, *taskState, *artifacts, *terminalEvents = currentSession, currentTask, artifactCount, eventCount
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	var sessions, tasks, checks, checkpoints, terminal, events []byte
	_ = pool.QueryRow(ctx, "SELECT COALESCE(json_agg(json_build_object('id',id,'task_id',task_id,'state',state,'profile',profile)), '[]'::json) FROM worker_sessions WHERE company_id=$1", company).Scan(&sessions)
	_ = pool.QueryRow(ctx, "SELECT COALESCE(json_agg(json_build_object('id',id,'state',state,'kind',kind)), '[]'::json) FROM tasks WHERE company_id=$1", company).Scan(&tasks)
	_ = pool.QueryRow(ctx, "SELECT COALESCE(json_agg(json_build_object('id',id,'phase',phase,'passed',passed,'report',report)), '[]'::json) FROM worker_checks WHERE company_id=$1", company).Scan(&checks)
	_ = pool.QueryRow(ctx, "SELECT COALESCE(json_agg(data), '[]'::json) FROM worker_checkpoints WHERE company_id=$1", company).Scan(&checkpoints)
	_ = pool.QueryRow(ctx, "SELECT COALESCE(json_agg(data), '[]'::json) FROM worker_observations WHERE company_id=$1 AND reason='provider_terminal'", company).Scan(&terminal)
	_ = pool.QueryRow(ctx, "SELECT COALESCE(json_agg(json_build_object('kind',kind,'payload',payload)), '[]'::json) FROM events WHERE company_id=$1", company).Scan(&events)
	t.Fatalf("timed out waiting for offline provider terminal state: sessions=%s tasks=%s checks=%s checkpoints=%s terminal=%s events=%s", sessions, tasks, checks, checkpoints, terminal, events)
}

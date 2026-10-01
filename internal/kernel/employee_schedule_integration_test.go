// pattern: Imperative Shell
package kernel

import (
	"context"
	"os"
	"sync"
	"testing"
	"time"

	"polis/internal/taskvalidation"
)

func TestNewCompanyCreatesSleepingEmployeeSchedules(t *testing.T) {
	dsn := os.Getenv("POLIS_TEST_DSN")
	if dsn == "" {
		t.Skip("dedicated PostgreSQL required")
	}
	k, err := Open(context.Background(), dsn, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer k.Close()
	scope, err := k.TXCreateCompany(context.Background(), "schedule-company-"+newID())
	if err != nil {
		t.Fatal(err)
	}
	schedule, err := k.EmployeeSchedule(context.Background(), scope, "emp-backend")
	if err != nil || schedule.State != "sleeping" || schedule.WorkGeneration != 0 {
		t.Fatalf("new company employee schedule=(%+v,%v), want sleeping generation 0", schedule, err)
	}
}

func TestActionablePeerWorkCannotBeLostAcrossEmployeeSleepReconciliation(t *testing.T) {
	env := newPeerHardeningEnv(t, 20)

	if err := env.k.TXBeginStop(env.ctx, env.frontend); err != nil {
		t.Fatal(err)
	}
	proof, err := env.frontendProcess.Stop()
	if err != nil {
		t.Fatal(err)
	}
	if err = env.k.TXConfirmStopped(env.ctx, env.frontend, proof); err != nil {
		t.Fatal(err)
	}
	if _, err = env.k.TXReconcileEmployeeSchedule(env.ctx, env.scope, "emp-frontend", "sleep-before-message"); err != nil {
		t.Fatal(err)
	}

	revision := acceptPeerRevision(t, env, "GET /items", `{"items":[],"next_cursor":""}`, "schedule-race")
	input := PeerSendInput{FromTask: env.fixture.Backend.ID, ToTask: env.fixture.Frontend.ID, ContractRevisionID: revision.ID, Body: "apply the accepted revision", Actionable: true}
	first, err := env.k.TXPeerSend(env.ctx, env.backend, input, "schedule-race-first")
	if err != nil {
		t.Fatal(err)
	}
	schedule, err := env.k.EmployeeSchedule(env.ctx, env.scope, "emp-frontend")
	if err != nil {
		t.Fatal(err)
	}
	if schedule.State != "wake_pending" || schedule.WorkGeneration != 2 {
		t.Fatalf("peer request schedule=%+v, want task plus peer wake generation 2", schedule)
	}

	if _, err = env.k.TXPeerSend(env.ctx, env.backend, input, "schedule-race-first"); err != nil {
		t.Fatal(err)
	}
	schedule, err = env.k.EmployeeSchedule(env.ctx, env.scope, "emp-frontend")
	if err != nil {
		t.Fatal(err)
	}
	if schedule.WorkGeneration != 2 {
		t.Fatalf("idempotent peer-send replay advanced work generation: %+v", schedule)
	}

	const concurrentSignals = 8
	var wait sync.WaitGroup
	errs := make(chan error, concurrentSignals*2)
	for i := 0; i < concurrentSignals; i++ {
		wait.Add(2)
		go func(index int) {
			defer wait.Done()
			message := input
			message.Body = "apply revision " + newID()
			_, sendErr := env.k.TXPeerSend(env.ctx, env.backend, message, "schedule-race-send-"+newID())
			errs <- sendErr
		}(i)
		go func(index int) {
			defer wait.Done()
			_, reconcileErr := env.k.TXReconcileEmployeeSchedule(env.ctx, env.scope, "emp-frontend", "schedule-race-sleep-"+newID())
			errs <- reconcileErr
		}(i)
	}
	wait.Wait()
	close(errs)
	for operationErr := range errs {
		if operationErr != nil {
			t.Fatal(operationErr)
		}
	}

	schedule, err = env.k.EmployeeSchedule(env.ctx, env.scope, "emp-frontend")
	if err != nil {
		t.Fatal(err)
	}
	if schedule.State != "wake_pending" || schedule.WorkGeneration != concurrentSignals+2 {
		t.Fatalf("concurrent work/sleep result=%+v, want durable wake_pending generation %d", schedule, concurrentSignals+2)
	}
	var pendingSignals int
	if err = env.k.pool.QueryRow(env.ctx, `SELECT count(*) FROM peer_work_signals s JOIN obligations o
ON o.company_id=s.company_id AND o.id=s.obligation_id
WHERE s.company_id=$1 AND s.recipient='emp-frontend' AND s.state IN ('pending','observed','acknowledged') AND o.state='pending'`, env.scope.company).Scan(&pendingSignals); err != nil {
		t.Fatal(err)
	}
	if pendingSignals != concurrentSignals+1 {
		t.Fatalf("durable pending peer signal count=%d, want %d (first message id %s)", pendingSignals, concurrentSignals+1, first.ID)
	}
}

func TestFYIPeerMessageDoesNotWakeEmployeeSchedule(t *testing.T) {
	env := newPeerHardeningEnv(t, 20)
	if err := env.k.TXBeginStop(env.ctx, env.frontend); err != nil {
		t.Fatal(err)
	}
	proof, err := env.frontendProcess.Stop()
	if err != nil {
		t.Fatal(err)
	}
	if err = env.k.TXConfirmStopped(env.ctx, env.frontend, proof); err != nil {
		t.Fatal(err)
	}
	if _, err = env.k.TXReconcileEmployeeSchedule(env.ctx, env.scope, "emp-frontend", "fyi-sleep"); err != nil {
		t.Fatal(err)
	}
	revision := acceptPeerRevision(t, env, "GET /items", `{"items":[],"next_cursor":""}`, "fyi-schedule")
	_, err = env.k.TXPeerSend(env.ctx, env.backend, PeerSendInput{
		FromTask: env.fixture.Backend.ID, ToTask: env.fixture.Frontend.ID,
		ContractRevisionID: revision.ID, Body: "FYI only", Actionable: false,
	}, "fyi-schedule-send")
	if err != nil {
		t.Fatal(err)
	}
	schedule, err := env.k.EmployeeSchedule(env.ctx, env.scope, "emp-frontend")
	if err != nil {
		t.Fatal(err)
	}
	if schedule.State != "sleeping" || schedule.WorkGeneration != 1 {
		t.Fatalf("FYI changed sleeping schedule: %+v", schedule)
	}
}

func TestMissionPauseBlocksPeerWorkAndResumeRestoresPendingWake(t *testing.T) {
	env := newPeerHardeningEnv(t, 20)
	revision := acceptPeerRevision(t, env, "GET /items", `{"items":[],"next_cursor":""}`, "paused-schedule")
	input := PeerSendInput{FromTask: env.fixture.Backend.ID, ToTask: env.fixture.Frontend.ID, ContractRevisionID: revision.ID, Body: "apply while paused", Actionable: true}
	if _, err := env.k.TXPeerSend(env.ctx, env.backend, input, "paused-schedule-first"); err != nil {
		t.Fatal(err)
	}
	if err := env.k.TXBeginStop(env.ctx, env.frontend); err != nil {
		t.Fatal(err)
	}
	proof, err := env.frontendProcess.Stop()
	if err != nil {
		t.Fatal(err)
	}
	if err = env.k.TXConfirmStopped(env.ctx, env.frontend, proof); err != nil {
		t.Fatal(err)
	}

	missionID := env.fixture.Frontend.Mission
	if _, err = env.k.TXSetMissionPausedCommand(env.ctx, env.scope, missionID, true, "paused-schedule-pause"); err != nil {
		t.Fatal(err)
	}
	schedule, err := env.k.EmployeeSchedule(env.ctx, env.scope, "emp-frontend")
	if err != nil {
		t.Fatal(err)
	}
	if schedule.State != "paused" || schedule.WorkGeneration != 2 {
		t.Fatalf("paused employee schedule=%+v, want paused with its pending work preserved", schedule)
	}
	if _, err = env.k.TXPeerSend(env.ctx, env.backend, input, "paused-schedule-denied"); err == nil {
		t.Fatal("mission pause allowed a new actionable peer message")
	}
	if _, err = env.k.TXReconcileEmployeeSchedule(env.ctx, env.scope, "emp-frontend", "paused-schedule-reconcile"); err != nil {
		t.Fatal(err)
	}
	schedule, err = env.k.EmployeeSchedule(env.ctx, env.scope, "emp-frontend")
	if err != nil || schedule.State != "paused" {
		t.Fatalf("schedule reconciliation while mission paused=(%+v,%v), want paused", schedule, err)
	}
	if _, err = env.k.TXSetMissionPausedCommand(env.ctx, env.scope, missionID, false, "paused-schedule-resume"); err != nil {
		t.Fatal(err)
	}
	schedule, err = env.k.EmployeeSchedule(env.ctx, env.scope, "emp-frontend")
	if err != nil || schedule.State != "wake_pending" || schedule.WorkGeneration != 2 {
		t.Fatalf("resumed employee schedule=(%+v,%v), want preserved wake_pending generation 2", schedule, err)
	}
}

func TestObservedPeerObligationKeepsStoppedEmployeeWakePending(t *testing.T) {
	env := newPeerHardeningEnv(t, 20)
	revision := acceptPeerRevision(t, env, "GET /items", `{"items":[],"next_cursor":""}`, "observed-schedule")
	message, err := env.k.TXPeerSend(env.ctx, env.backend, PeerSendInput{
		FromTask: env.fixture.Backend.ID, ToTask: env.fixture.Frontend.ID,
		ContractRevisionID: revision.ID, Body: "apply the observed request", Actionable: true,
	}, "observed-schedule-send")
	if err != nil {
		t.Fatal(err)
	}
	if err = env.k.TXPeerDeliver(env.ctx, env.frontend, message.ID, "observed-schedule-deliver"); err != nil {
		t.Fatal(err)
	}
	if err = env.k.TXPeerObserve(env.ctx, env.frontend, message.ID, "observed-schedule-observe"); err != nil {
		t.Fatal(err)
	}
	if err = env.k.TXBeginStop(env.ctx, env.frontend); err != nil {
		t.Fatal(err)
	}
	proof, err := env.frontendProcess.Stop()
	if err != nil {
		t.Fatal(err)
	}
	if err = env.k.TXConfirmStopped(env.ctx, env.frontend, proof); err != nil {
		t.Fatal(err)
	}
	schedule, err := env.k.EmployeeSchedule(env.ctx, env.scope, "emp-frontend")
	if err != nil || schedule.State != "wake_pending" || schedule.WorkGeneration != 2 {
		t.Fatalf("observed outstanding peer request schedule=(%+v,%v), want wake_pending generation 2", schedule, err)
	}
}

func TestReadyTasksAdvanceEmployeeScheduleOnMissionStartAndResume(t *testing.T) {
	dsn := os.Getenv("POLIS_TEST_DSN")
	if dsn == "" {
		t.Skip("dedicated PostgreSQL required")
	}
	ctx := context.Background()
	k, err := Open(ctx, dsn, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer k.Close()
	scope, err := k.TXCreateCompany(ctx, "ready-task-schedule-"+newID())
	if err != nil {
		t.Fatal(err)
	}
	mission, err := k.TXCreateMissionGoal(ctx, scope, "Wake ready tasks", "Wake each owner when a ready Task appears", "ready-task-mission-create")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = k.TXStartMissionCommand(ctx, scope, mission.ID, "ready-task-mission-start"); err != nil {
		t.Fatal(err)
	}
	planner, err := k.EmployeeSchedule(ctx, scope, "emp-planning")
	if err != nil || planner.State != "wake_pending" || planner.WorkGeneration != 1 {
		t.Fatalf("Mission start Task schedule=(%+v,%v), want wake_pending generation 1", planner, err)
	}
	if _, err = k.TXPrepareProductTask(ctx, scope, mission.ID, "Implement the approved product goal", "ready-task-backend-create"); err != nil {
		t.Fatal(err)
	}
	backend, err := k.EmployeeSchedule(ctx, scope, "emp-backend")
	if err != nil || backend.State != "wake_pending" || backend.WorkGeneration != 1 {
		t.Fatalf("product Task schedule=(%+v,%v), want wake_pending generation 1", backend, err)
	}
	if _, err = k.TXSetMissionPausedCommand(ctx, scope, mission.ID, true, "ready-task-mission-pause"); err != nil {
		t.Fatal(err)
	}
	backend, err = k.EmployeeSchedule(ctx, scope, "emp-backend")
	if err != nil || backend.State != "paused" || backend.WorkGeneration != 1 {
		t.Fatalf("paused Task schedule=(%+v,%v), want paused with generation preserved", backend, err)
	}
	if _, err = k.pool.Exec(ctx, `INSERT INTO tasks(company_id,id,mission_id,owner,kind,state)
VALUES($1,$2,$3,'emp-review','review','ready')`, scope.company, newID(), mission.ID); err != nil {
		t.Fatal(err)
	}
	reviewer, err := k.EmployeeSchedule(ctx, scope, "emp-review")
	if err != nil || reviewer.State != "paused" || reviewer.WorkGeneration != 1 {
		t.Fatalf("ready Task created while paused=(%+v,%v), want paused generation 1", reviewer, err)
	}
	if _, err = k.TXSetMissionPausedCommand(ctx, scope, mission.ID, false, "ready-task-mission-resume"); err != nil {
		t.Fatal(err)
	}
	backend, err = k.EmployeeSchedule(ctx, scope, "emp-backend")
	if err != nil || backend.State != "wake_pending" || backend.WorkGeneration != 1 {
		t.Fatalf("resumed ready Task schedule=(%+v,%v), want wake_pending generation 1", backend, err)
	}
	reviewer, err = k.EmployeeSchedule(ctx, scope, "emp-review")
	if err != nil || reviewer.State != "wake_pending" || reviewer.WorkGeneration != 1 {
		t.Fatalf("resumed paused-period Task schedule=(%+v,%v), want wake_pending generation 1", reviewer, err)
	}
}

func TestNextProductWorkerDispatchCandidateIsAgeOrderedAndHonorsBarriers(t *testing.T) {
	dsn := os.Getenv("POLIS_TEST_DSN")
	if dsn == "" {
		t.Skip("dedicated PostgreSQL required")
	}
	ctx := context.Background()
	k, err := Open(ctx, dsn, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer k.Close()

	createCandidate := func(name string) (Scope, string) {
		t.Helper()
		scope, createErr := k.TXCreateCompany(ctx, "dispatch-fairness-"+name+"-"+newID())
		if createErr != nil {
			t.Fatal(createErr)
		}
		contract := &taskvalidation.AcceptanceContract{
			Revision:     taskvalidation.AcceptanceContractRevision,
			RequiredText: []string{"Task summary:"},
		}
		goal := "verify oldest eligible product work is selected"
		mission, createErr := k.TXCreateMissionGoalWithAcceptance(ctx, scope, "Dispatch fairness", goal, contract, "dispatch-mission-create-"+newID())
		if createErr != nil {
			t.Fatal(createErr)
		}
		if _, createErr = k.TXStartMissionCommand(ctx, scope, mission.ID, "dispatch-mission-start-"+newID()); createErr != nil {
			t.Fatal(createErr)
		}
		if _, createErr = k.TXPrepareProductTask(ctx, scope, mission.ID, goal, "dispatch-task-create-"+newID()); createErr != nil {
			t.Fatal(createErr)
		}
		return scope, mission.ID
	}
	oldestScope, oldestMission := createCandidate("oldest")
	nextScope, _ := createCandidate("next")
	if _, err = k.pool.Exec(ctx, `UPDATE employee_schedules SET updated_at=$3 WHERE company_id=$1 AND employee_id='emp-backend'`, oldestScope.company, "emp-backend", time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}
	if _, err = k.pool.Exec(ctx, `UPDATE employee_schedules SET updated_at=$3 WHERE company_id=$1 AND employee_id='emp-backend'`, nextScope.company, "emp-backend", time.Date(2000, 1, 2, 0, 0, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}
	candidate, found, err := k.NextProductWorkerDispatchCandidate(ctx, "")
	if err != nil || !found || candidate.CompanyID != oldestScope.company || candidate.MissionID != oldestMission {
		t.Fatalf("oldest dispatch candidate=(%+v,%t,%v), want oldest company mission %q", candidate, found, err, oldestMission)
	}
	if _, err = k.pool.Exec(ctx, `UPDATE employee_schedules SET state='waiting_quota' WHERE company_id=$1 AND employee_id='emp-backend'`, oldestScope.company); err != nil {
		t.Fatal(err)
	}
	candidate, found, err = k.NextProductWorkerDispatchCandidate(ctx, "")
	if err != nil || !found || candidate.CompanyID != nextScope.company {
		t.Fatalf("quota-barrier candidate=(%+v,%t,%v), want next company %q", candidate, found, err, nextScope.company)
	}
	if _, err = k.TXSetMissionPausedCommand(ctx, k.LocalScope(nextScope.company), candidate.MissionID, true, "dispatch-pause-"+newID()); err != nil {
		t.Fatal(err)
	}
	candidate, found, err = k.NextProductWorkerDispatchCandidate(ctx, "")
	if err != nil || found {
		t.Fatalf("paused-only dispatch candidate=(%+v,%t,%v), want no candidate", candidate, found, err)
	}
}

func TestKernelStartupScanRestoresReadyTaskWakeAfterLostHint(t *testing.T) {
	dsn := os.Getenv("POLIS_TEST_DSN")
	if dsn == "" {
		t.Skip("dedicated PostgreSQL required")
	}
	ctx := context.Background()
	k, err := Open(ctx, dsn, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	scope, err := k.TXCreateCompany(ctx, "startup-wake-scan-"+newID())
	if err != nil {
		k.Close()
		t.Fatal(err)
	}
	mission, err := k.TXCreateMissionGoal(ctx, scope, "Recover a pending Task", "Reconcile a durable ready Task after restart", "startup-wake-mission-create")
	if err != nil {
		k.Close()
		t.Fatal(err)
	}
	if _, err = k.TXStartMissionCommand(ctx, scope, mission.ID, "startup-wake-mission-start"); err != nil {
		k.Close()
		t.Fatal(err)
	}
	if _, err = k.TXPrepareProductTask(ctx, scope, mission.ID, "Run the prepared Task", "startup-wake-task-create"); err != nil {
		k.Close()
		t.Fatal(err)
	}
	before, err := k.EmployeeSchedule(ctx, scope, "emp-backend")
	if err != nil || before.State != "wake_pending" || before.WorkGeneration == 0 {
		k.Close()
		t.Fatalf("pre-restart schedule=(%+v,%v), want durable wake_pending", before, err)
	}
	if _, err = k.pool.Exec(ctx, `UPDATE employee_schedules SET state='sleeping',checked_generation=work_generation
WHERE company_id=$1 AND employee_id='emp-backend'`, scope.company); err != nil {
		k.Close()
		t.Fatal(err)
	}
	k.Close()

	restarted, err := Open(ctx, dsn, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer restarted.Close()
	after, err := restarted.EmployeeSchedule(ctx, scope, "emp-backend")
	if err != nil || after.State != "wake_pending" || after.WorkGeneration != before.WorkGeneration {
		t.Fatalf("post-restart schedule=(%+v,%v), want wake_pending at preserved generation %d", after, err, before.WorkGeneration)
	}
}

func TestCancellingPausedMissionClearsScheduleBarrierForNextMission(t *testing.T) {
	dsn := os.Getenv("POLIS_TEST_DSN")
	if dsn == "" {
		t.Skip("dedicated PostgreSQL required")
	}
	ctx := context.Background()
	k, err := Open(ctx, dsn, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer k.Close()
	scope, err := k.TXCreateCompany(ctx, "cancel-paused-schedule-"+newID())
	if err != nil {
		t.Fatal(err)
	}
	first, err := k.TXCreateMissionGoal(ctx, scope, "First Mission", "Start, pause, and cancel", "cancel-paused-mission-create-1")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = k.TXStartMissionCommand(ctx, scope, first.ID, "cancel-paused-mission-start-1"); err != nil {
		t.Fatal(err)
	}
	if _, err = k.TXSetMissionPausedCommand(ctx, scope, first.ID, true, "cancel-paused-mission-pause"); err != nil {
		t.Fatal(err)
	}
	if _, err = k.TXCancelMission(ctx, scope, first.ID, "cancel-paused-mission-cancel"); err != nil {
		t.Fatal(err)
	}
	planner, err := k.EmployeeSchedule(ctx, scope, "emp-planning")
	if err != nil || planner.State != "sleeping" {
		t.Fatalf("cancelled Mission schedule=(%+v,%v), want sleeping", planner, err)
	}

	second, err := k.TXCreateMissionGoal(ctx, scope, "Second Mission", "Ready work must wake after cancellation", "cancel-paused-mission-create-2")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = k.TXStartMissionCommand(ctx, scope, second.ID, "cancel-paused-mission-start-2"); err != nil {
		t.Fatal(err)
	}
	planner, err = k.EmployeeSchedule(ctx, scope, "emp-planning")
	if err != nil || planner.State != "wake_pending" {
		t.Fatalf("next Mission ready Task schedule=(%+v,%v), want wake_pending", planner, err)
	}
}

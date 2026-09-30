// pattern: Imperative Shell
package kernel

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"polis/internal/core"
	"polis/internal/intake"
)

func TestCreateDailyRoutineRequiresTaskInstruction(t *testing.T) {
	k, scope, fixture := newDailyRoutineEnv(t)
	schedule := core.DailyRoutineSchedule{
		RoutineID: "routine-missing-instruction-" + newID(), Timezone: "Asia/Shanghai", LocalTime: "09:00",
		NextLogicalDay: "2026-09-30", CatchUpPolicy: core.RoutineCatchUpCoalesceLatest, MaxCatchUp: 1,
	}
	_, err := k.TXCreateDailyRoutine(context.Background(), scope, fixture.Backend.Mission, "emp-review", schedule, "routine-missing-instruction-create-"+newID())
	if err != core.Malformed {
		t.Fatalf("CreateDailyRoutine without task instruction error=%v, want malformed", err)
	}
}

func TestMigratedActiveLegacyRoutineRequiresInstructionThenDelivers(t *testing.T) {
	dsn := os.Getenv("POLIS_TEST_DSN")
	if dsn == "" {
		t.Skip("dedicated PostgreSQL upgrade fixture required")
	}
	ctx := context.Background()
	k, err := Open(ctx, dsn, t.TempDir())
	must(t, err)
	t.Cleanup(func() { k.Close() })
	scope := Scope{company: "wake_migration_pending"}
	routineID := "active-legacy-routine"
	var state string
	var taskID sql.NullString
	if err = k.pool.QueryRow(ctx, `SELECT state,task_id FROM routine_occurrences WHERE company_id=$1 AND routine_id=$2`, scope.company, routineID).Scan(&state, &taskID); err != nil {
		t.Fatal(err)
	}
	if state != "needs_instruction" || taskID.Valid {
		t.Fatalf("upgraded active legacy occurrence=(%s,%v), want needs_instruction/no Task", state, taskID)
	}
	instruction := "Review the legacy scheduled responsibility and report one next step."
	receipt, err := k.TXSetDailyRoutineTaskInstruction(ctx, scope, "pending-mission", routineID, instruction, "active-legacy-instruction-set")
	must(t, err)
	if receipt.Status != "instruction_set" || receipt.Revision != 1 {
		t.Fatalf("legacy instruction repair receipt=%+v, want one delivered occurrence", receipt)
	}
	repaired, err := k.TXSetDailyRoutineTaskInstruction(ctx, scope, "pending-mission", routineID, instruction, "active-legacy-instruction-confirm")
	must(t, err)
	if repaired.Revision != 0 {
		t.Fatalf("replaying an already repaired legacy Routine created additional Tasks: %+v", repaired)
	}
	var count int
	if err = k.pool.QueryRow(ctx, `SELECT count(*) FROM tasks WHERE company_id=$1 AND plan->>'routine_id'=$2`, scope.company, routineID).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("active legacy Routine Task count=%d, want exactly one", count)
	}
}

func TestDailyRoutineMaterializationPersistsAndWakesOwner(t *testing.T) {
	k, scope, fixture := newDailyRoutineEnv(t)
	ctx := context.Background()
	ownerID := "emp-review"
	before, err := k.EmployeeSchedule(ctx, scope, ownerID)
	must(t, err)
	_, err = k.TXReconcileEmployeeSchedule(ctx, scope, ownerID, "routine-before-sleep-"+newID())
	must(t, err)
	before, err = k.EmployeeSchedule(ctx, scope, ownerID)
	must(t, err)
	if before.State != core.EmployeeScheduleSleeping {
		t.Fatalf("routine owner schedule before materialization=%+v, want sleeping", before)
	}

	missionID := fixture.Backend.Mission
	inputBytes := []byte("# reference input\nReview the bounded routine source.\n")
	inputUpload, err := intake.PrepareUpload("routine-reference.md", "text/markdown", inputBytes)
	must(t, err)
	inputRevision, err := k.TXAddMissionInput(ctx, scope, missionID, "", "routine-input-"+newID(), inputUpload, inputBytes)
	must(t, err)
	routineID := "routine-daily-" + newID()
	schedule := core.DailyRoutineSchedule{
		RoutineID: routineID, TaskInstruction: "Review the assigned queue and record the next safe action.", Timezone: "Asia/Shanghai", LocalTime: "09:00",
		NextLogicalDay: "2026-09-28", CatchUpPolicy: core.RoutineCatchUpCoalesceLatest, MaxCatchUp: 1,
	}
	created, err := k.TXCreateDailyRoutine(ctx, scope, missionID, ownerID, schedule, "routine-create-"+newID())
	must(t, err)
	if created.ID != routineID || created.Status != "active" {
		t.Fatalf("daily routine receipt=%+v", created)
	}
	initialDue := readEmployeeScheduleNextDue(t, k, scope, ownerID)
	if !initialDue.Valid || !initialDue.Time.Equal(time.Date(2026, time.September, 28, 1, 0, 0, 0, time.UTC)) {
		t.Fatalf("initial next_due_at=%+v, want 2026-09-28T01:00:00Z", initialDue)
	}

	now := time.Date(2026, time.September, 30, 2, 0, 0, 0, time.UTC)
	materializeKey := "routine-materialize-" + newID()
	first, err := k.TXMaterializeDailyRoutine(ctx, scope, routineID, now, materializeKey)
	must(t, err)
	if first.NextLogicalDay != "2026-10-01" || len(first.Occurrences) != 1 {
		t.Fatalf("materialization=%+v, want one latest occurrence and next day 2026-10-01", first)
	}
	occurrence := first.Occurrences[0]
	if occurrence.OccurrenceKey != routineID+":2026-09-30" || occurrence.CoalescedFrom != "2026-09-28" || occurrence.CoalescedThrough != "2026-09-30" {
		t.Fatalf("materialized occurrence=%+v", occurrence)
	}

	var persisted int
	err = k.pool.QueryRow(ctx, `SELECT count(*) FROM routine_occurrences
WHERE company_id=$1 AND routine_id=$2 AND occurrence_key=$3 AND state='delivered'`, scope.company, routineID, occurrence.OccurrenceKey).Scan(&persisted)
	must(t, err)
	if persisted != 1 {
		t.Fatalf("delivered routine occurrence rows=%d, want 1", persisted)
	}
	var occurrenceState, taskID string
	if err = k.pool.QueryRow(ctx, `SELECT state,COALESCE(task_id,'') FROM routine_occurrences WHERE company_id=$1 AND routine_id=$2 AND occurrence_key=$3`, scope.company, routineID, occurrence.OccurrenceKey).Scan(&occurrenceState, &taskID); err != nil {
		t.Fatal(err)
	}
	if occurrenceState != "delivered" || taskID == "" {
		t.Fatalf("Routine occurrence state=%q task_id=%q, want delivered Task link", occurrenceState, taskID)
	}
	if _, err = k.TXNewProductProviderWorkerWithToolBudget(ctx, scope, taskID, "offline-model/medium", 16); err != core.Denied {
		t.Fatalf("Product Provider accepted Routine compute Task: %v", err)
	}
	var providerSessions int
	if err = k.pool.QueryRow(ctx, `SELECT count(*) FROM worker_sessions WHERE company_id=$1 AND task_id=$2`, scope.company, taskID).Scan(&providerSessions); err != nil {
		t.Fatal(err)
	}
	if providerSessions != 0 {
		t.Fatalf("denied Product Provider admission persisted %d sessions", providerSessions)
	}
	after, err := k.EmployeeSchedule(ctx, scope, ownerID)
	must(t, err)
	if after.State != core.EmployeeScheduleWakePending || after.WorkGeneration != before.WorkGeneration+1 {
		t.Fatalf("routine wake schedule=%+v, want wake_pending generation %d", after, before.WorkGeneration+1)
	}
	nextDue := readEmployeeScheduleNextDue(t, k, scope, ownerID)
	if !nextDue.Valid || !nextDue.Time.Equal(time.Date(2026, time.October, 1, 1, 0, 0, 0, time.UTC)) {
		t.Fatalf("next_due_at after materialization=%+v, want 2026-10-01T01:00:00Z", nextDue)
	}

	replay, err := k.TXMaterializeDailyRoutine(ctx, scope, routineID, now, materializeKey)
	must(t, err)
	if len(replay.Occurrences) != 1 || replay.Occurrences[0].OccurrenceKey != occurrence.OccurrenceKey || replay.NextLogicalDay != first.NextLogicalDay {
		t.Fatalf("materialization replay=%+v, want original result %+v", replay, first)
	}
	afterReplay, err := k.EmployeeSchedule(ctx, scope, ownerID)
	must(t, err)
	if afterReplay.WorkGeneration != after.WorkGeneration {
		t.Fatalf("idempotent materialization replay advanced work generation: before=%+v after=%+v", after, afterReplay)
	}
	binding, err := k.TXNewWorker(ctx, scope, taskID, "offline/fake")
	must(t, err)
	handover, err := k.Handover(ctx, binding)
	must(t, err)
	var taskPlan routineTaskPlan
	if err = json.Unmarshal(handover.Task.Plan, &taskPlan); err != nil {
		t.Fatalf("routine Task plan is not available in Worker Handover: %v", err)
	}
	if taskPlan.Template != "daily-routine-task@1" || taskPlan.RoutineID != routineID || taskPlan.TaskInstruction != schedule.TaskInstruction || taskPlan.OccurrenceKey != occurrence.OccurrenceKey || taskPlan.LogicalDay != occurrence.LogicalDay || !taskPlan.ScheduledAt.Equal(occurrence.ScheduledAt) || taskPlan.CoalescedFrom != occurrence.CoalescedFrom || taskPlan.CoalescedThrough != occurrence.CoalescedThrough {
		t.Fatalf("Worker Handover routine Task plan=%v, want frozen instruction and occurrence", taskPlan)
	}
	if !strings.Contains(handover.Workspace.Content, schedule.TaskInstruction) {
		t.Fatalf("routine Task workspace omitted the frozen instruction: %q", handover.Workspace.Content)
	}
	var storedWorkspaceDigest string
	if err = k.pool.QueryRow(ctx, `SELECT digest FROM worker_workspaces WHERE company_id=$1 AND task_id=$2`, scope.company, taskID).Scan(&storedWorkspaceDigest); err != nil {
		t.Fatal(err)
	}
	if storedWorkspaceDigest != handover.Workspace.Digest {
		t.Fatalf("Worker workspace digest=%s, persisted digest=%s", handover.Workspace.Digest, storedWorkspaceDigest)
	}
	inputManifest, err := k.TaskInputManifest(ctx, scope, taskID)
	must(t, err)
	if len(inputManifest.Manifest.CandidateInputs) != 1 || inputManifest.Manifest.CandidateInputs[0].ContentDigest != inputRevision.ContentDigest {
		t.Fatalf("Routine Task input manifest=%+v, want frozen MissionInput digest %s", inputManifest.Manifest, inputRevision.ContentDigest)
	}
	if len(inputManifest.Digest) != 64 {
		t.Fatalf("Routine Task aggregate input manifest digest has %d hex characters", len(inputManifest.Digest))
	}
}

func TestPausedMissionPersistsDailyRoutineWithoutWakingUntilResume(t *testing.T) {
	k, scope, fixture := newDailyRoutineEnv(t)
	ctx := context.Background()
	routineID := "routine-paused-" + newID()
	schedule := core.DailyRoutineSchedule{
		RoutineID: routineID, TaskInstruction: "Review the assigned queue and record the next safe action.", Timezone: "Asia/Shanghai", LocalTime: "09:00",
		NextLogicalDay: "2026-09-30", CatchUpPolicy: core.RoutineCatchUpCoalesceLatest, MaxCatchUp: 1,
	}
	_, err := k.TXCreateDailyRoutine(ctx, scope, fixture.Backend.Mission, "emp-review", schedule, "routine-paused-create-"+newID())
	must(t, err)
	_, err = k.TXSetMissionPausedCommand(ctx, scope, fixture.Backend.Mission, true, "routine-mission-pause-"+newID())
	must(t, err)

	_, err = k.TXMaterializeDailyRoutine(ctx, scope, routineID, time.Date(2026, time.September, 30, 2, 0, 0, 0, time.UTC), "routine-paused-materialize-"+newID())
	must(t, err)
	scheduleState, err := k.EmployeeSchedule(ctx, scope, "emp-review")
	must(t, err)
	if scheduleState.State != core.EmployeeSchedulePaused || scheduleState.PauseReason != "mission_paused" {
		t.Fatalf("paused routine schedule=%+v, want paused barrier", scheduleState)
	}
	if nextDue := readEmployeeScheduleNextDue(t, k, scope, "emp-review"); nextDue.Valid {
		t.Fatalf("paused Routine kept next_due_at=%+v, want null until Mission resume", nextDue)
	}
	var delivered int
	err = k.pool.QueryRow(ctx, `SELECT count(*) FROM routine_occurrences WHERE company_id=$1 AND routine_id=$2 AND state='delivered' AND task_id IS NOT NULL`, scope.company, routineID).Scan(&delivered)
	must(t, err)
	if delivered != 1 {
		t.Fatalf("delivered routine Tasks during pause=%d, want 1", delivered)
	}
	var pausedTaskID string
	if err = k.pool.QueryRow(ctx, `SELECT task_id FROM routine_occurrences WHERE company_id=$1 AND routine_id=$2`, scope.company, routineID).Scan(&pausedTaskID); err != nil {
		t.Fatal(err)
	}
	if _, err = k.TXNewWorker(ctx, scope, pausedTaskID, "offline/fake"); err != core.Denied {
		t.Fatalf("paused Routine Task Worker admission error=%v, want denied", err)
	}

	_, err = k.TXSetMissionPausedCommand(ctx, scope, fixture.Backend.Mission, false, "routine-mission-resume-"+newID())
	must(t, err)
	scheduleState, err = k.EmployeeSchedule(ctx, scope, "emp-review")
	must(t, err)
	if scheduleState.State != core.EmployeeScheduleWakePending {
		t.Fatalf("resumed routine schedule=%+v, want wake_pending", scheduleState)
	}
	if nextDue := readEmployeeScheduleNextDue(t, k, scope, "emp-review"); !nextDue.Valid || !nextDue.Time.Equal(time.Date(2026, time.October, 1, 1, 0, 0, 0, time.UTC)) {
		t.Fatalf("resumed Routine next_due_at=%+v, want 2026-10-01T01:00:00Z", nextDue)
	}
}

func TestCancelledMissionRoutineOccurrenceDoesNotRewakeEmployee(t *testing.T) {
	k, scope, fixture := newDailyRoutineEnv(t)
	ctx := context.Background()
	location, err := time.LoadLocation("Asia/Shanghai")
	must(t, err)
	logicalDay := time.Now().In(location).AddDate(0, 0, -1).Format(time.DateOnly)
	routineID := "routine-cancelled-" + newID()
	schedule := core.DailyRoutineSchedule{
		RoutineID: routineID, TaskInstruction: "Review the assigned queue and record the next safe action.", Timezone: "Asia/Shanghai", LocalTime: "23:59",
		NextLogicalDay: logicalDay, CatchUpPolicy: core.RoutineCatchUpCoalesceLatest, MaxCatchUp: 1,
	}
	_, err = k.TXCreateDailyRoutine(ctx, scope, fixture.Backend.Mission, "emp-review", schedule, "routine-cancel-create-"+newID())
	must(t, err)
	_, err = k.TXMaterializeDailyRoutine(ctx, scope, routineID, time.Now().UTC(), "routine-cancel-materialize-"+newID())
	must(t, err)
	_, err = k.TXSetMissionPausedCommand(ctx, scope, fixture.Backend.Mission, true, "routine-cancel-pause-"+newID())
	must(t, err)
	_, err = k.TXCancelMission(ctx, scope, fixture.Backend.Mission, "routine-cancel-mission-"+newID())
	must(t, err)
	scheduleState, err := k.EmployeeSchedule(ctx, scope, "emp-review")
	must(t, err)
	if scheduleState.State != core.EmployeeScheduleSleeping || scheduleState.PauseReason != "" {
		t.Fatalf("cancelled Mission routine schedule=%+v, want sleeping without pause barrier", scheduleState)
	}
	if due := readEmployeeScheduleNextDue(t, k, scope, "emp-review"); due.Valid {
		t.Fatalf("cancelled Mission kept next_due_at=%+v, want null", due)
	}
	var occurrenceState string
	if err = k.pool.QueryRow(ctx, `SELECT state FROM routine_occurrences WHERE company_id=$1 AND routine_id=$2`, scope.company, routineID).Scan(&occurrenceState); err != nil {
		t.Fatal(err)
	}
	if occurrenceState != "cancelled" {
		t.Fatalf("cancelled Mission Routine occurrence state=%q, want cancelled", occurrenceState)
	}
}

func TestLegacyRoutineWithoutInstructionWaitsThenDeliversOnceSupplemented(t *testing.T) {
	k, scope, fixture := newDailyRoutineEnv(t)
	ctx := context.Background()
	routineID := "routine-legacy-instruction-" + newID()
	schedule := core.DailyRoutineSchedule{
		RoutineID: routineID, TaskInstruction: "Temporary valid instruction before simulating a legacy row.", Timezone: "UTC", LocalTime: "09:00",
		NextLogicalDay: "2026-09-30", CatchUpPolicy: core.RoutineCatchUpCoalesceLatest, MaxCatchUp: 1,
	}
	_, err := k.TXCreateDailyRoutine(ctx, scope, fixture.Backend.Mission, "emp-review", schedule, "routine-legacy-create-"+newID())
	must(t, err)
	if _, err = k.pool.Exec(ctx, `UPDATE routines SET task_instruction=NULL WHERE company_id=$1 AND id=$2`, scope.company, routineID); err != nil {
		t.Fatal(err)
	}
	result, err := k.TXMaterializeDailyRoutine(ctx, scope, routineID, time.Date(2026, time.September, 30, 10, 0, 0, 0, time.UTC), "routine-legacy-materialize-"+newID())
	must(t, err)
	if len(result.Occurrences) != 1 || result.Occurrences[0].State != "needs_instruction" || result.Occurrences[0].TaskID != "" {
		t.Fatalf("legacy occurrence=%+v, want needs_instruction and no Task", result.Occurrences)
	}
	instruction := "Review the current queue and report one safe next step."
	_, err = k.TXSetDailyRoutineTaskInstruction(ctx, scope, fixture.Backend.Mission, routineID, instruction, "routine-legacy-instruction-set-"+newID())
	must(t, err)
	var delivered, tasks int
	if err = k.pool.QueryRow(ctx, `SELECT count(*) FROM routine_occurrences WHERE company_id=$1 AND routine_id=$2 AND state='delivered' AND task_instruction_snapshot=$3 AND task_id IS NOT NULL`, scope.company, routineID, instruction).Scan(&delivered); err != nil {
		t.Fatal(err)
	}
	if err = k.pool.QueryRow(ctx, `SELECT count(*) FROM tasks WHERE company_id=$1 AND mission_id=$2 AND plan->>'routine_id'=$3 AND plan->>'task_instruction'=$4`, scope.company, fixture.Backend.Mission, routineID, instruction).Scan(&tasks); err != nil {
		t.Fatal(err)
	}
	if delivered != 1 || tasks != 1 {
		t.Fatalf("supplemented Routine delivered occurrences=%d tasks=%d, want exactly one each", delivered, tasks)
	}
	var taskID, occurrenceState string
	if err = k.pool.QueryRow(ctx, `SELECT task_id,state FROM routine_occurrences WHERE company_id=$1 AND routine_id=$2`, scope.company, routineID).Scan(&taskID, &occurrenceState); err != nil {
		t.Fatal(err)
	}
	if _, err = k.pool.Exec(ctx, `UPDATE tasks SET state='completed' WHERE company_id=$1 AND id=$2`, scope.company, taskID); err != nil {
		t.Fatal(err)
	}
	if err = k.pool.QueryRow(ctx, `SELECT state FROM routine_occurrences WHERE company_id=$1 AND routine_id=$2`, scope.company, routineID).Scan(&occurrenceState); err != nil {
		t.Fatal(err)
	}
	if occurrenceState != "completed" {
		t.Fatalf("completed Routine Task left occurrence state=%q", occurrenceState)
	}
}

func TestRoutineMaterializationRollsBackTaskAndManifestWhenOccurrenceInsertFails(t *testing.T) {
	adminDSN := os.Getenv("POLIS_TEST_ADMIN_DSN")
	if adminDSN == "" {
		t.Skip("dedicated PostgreSQL admin connection required for failure injection")
	}
	k, scope, fixture := newDailyRoutineEnv(t)
	ctx := context.Background()
	admin, err := pgxpool.New(ctx, adminDSN)
	must(t, err)
	t.Cleanup(admin.Close)
	suffix := strings.ReplaceAll(newID(), "-", "")
	functionName, triggerName := "fail_routine_insert_"+suffix, "fail_routine_insert_trigger_"+suffix
	if _, err = admin.Exec(ctx, `CREATE FUNCTION `+functionName+`() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN RAISE EXCEPTION 'forced Routine occurrence insertion failure'; END
$$`); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = admin.Exec(context.Background(), `DROP TRIGGER IF EXISTS `+triggerName+` ON routine_occurrences`)
		_, _ = admin.Exec(context.Background(), `DROP FUNCTION IF EXISTS `+functionName+`() `)
	})
	routineID := "routine-atomic-" + newID()
	schedule := core.DailyRoutineSchedule{
		RoutineID: routineID, TaskInstruction: "Review the assigned queue and record the next safe action.", Timezone: "UTC", LocalTime: "09:00",
		NextLogicalDay: "2026-09-29", CatchUpPolicy: core.RoutineCatchUpCoalesceLatest, MaxCatchUp: 1,
	}
	_, err = k.TXCreateDailyRoutine(ctx, scope, fixture.Backend.Mission, "emp-review", schedule, "routine-atomic-create-"+newID())
	must(t, err)
	before, err := k.EmployeeSchedule(ctx, scope, "emp-review")
	must(t, err)
	if _, err = admin.Exec(ctx, `CREATE TRIGGER `+triggerName+` BEFORE INSERT ON routine_occurrences FOR EACH ROW EXECUTE FUNCTION `+functionName+`() `); err != nil {
		t.Fatal(err)
	}
	materializationRequestID := "routine-atomic-materialize-" + newID()
	_, err = k.TXMaterializeDailyRoutine(ctx, scope, routineID, time.Date(2026, time.September, 30, 10, 0, 0, 0, time.UTC), materializationRequestID)
	if err == nil {
		t.Fatal("forced occurrence insert failure was ignored")
	}
	var materializations, occurrences, tasks, manifests, workspaces int
	queries := []struct {
		query string
		dest  *int
	}{
		{`SELECT count(*) FROM routine_materializations WHERE company_id=$1 AND routine_id=$2 AND request_id=$3`, &materializations},
		{`SELECT count(*) FROM routine_occurrences WHERE company_id=$1 AND routine_id=$2`, &occurrences},
		{`SELECT count(*) FROM tasks WHERE company_id=$1 AND mission_id=$2 AND plan->>'routine_id'=$3`, &tasks},
		{`SELECT count(*) FROM task_input_manifests WHERE company_id=$1 AND mission_id=$2 AND task_id IN (SELECT id FROM tasks WHERE company_id=$1 AND plan->>'routine_id'=$3)`, &manifests},
		{`SELECT count(*) FROM worker_workspaces WHERE company_id=$1 AND task_id IN (SELECT id FROM tasks WHERE company_id=$1 AND plan->>'routine_id'=$2)`, &workspaces},
	}
	args := [][]any{{scope.company, routineID, materializationRequestID}, {scope.company, routineID}, {scope.company, fixture.Backend.Mission, routineID}, {scope.company, fixture.Backend.Mission, routineID}, {scope.company, routineID}}
	for i, item := range queries {
		if err = k.pool.QueryRow(ctx, item.query, args[i]...).Scan(item.dest); err != nil {
			t.Fatal(err)
		}
		if *item.dest != 0 {
			t.Fatalf("failed Routine materialization left query %d with %d rows", i, *item.dest)
		}
	}
	var nextLogicalDay string
	if err = k.pool.QueryRow(ctx, `SELECT to_char(next_logical_day,'YYYY-MM-DD') FROM routines WHERE company_id=$1 AND id=$2`, scope.company, routineID).Scan(&nextLogicalDay); err != nil {
		t.Fatal(err)
	}
	if nextLogicalDay != schedule.NextLogicalDay {
		t.Fatalf("failed Routine materialization advanced logical day to %s", nextLogicalDay)
	}
	after, err := k.EmployeeSchedule(ctx, scope, "emp-review")
	must(t, err)
	if after.WorkGeneration != before.WorkGeneration {
		t.Fatalf("failed Routine materialization changed employee schedule generation: before=%+v after=%+v", before, after)
	}
}

func newDailyRoutineEnv(t *testing.T) (*Kernel, Scope, PeerFixture) {
	t.Helper()
	dsn := os.Getenv("POLIS_TEST_DSN")
	if dsn == "" {
		t.Skip("dedicated PostgreSQL required")
	}
	ctx := context.Background()
	k, err := Open(ctx, dsn, t.TempDir())
	must(t, err)
	scope, err := k.TXCreateCompany(ctx, "routine-company-"+newID())
	must(t, err)
	fixture, err := k.TXCreatePeerFixture(ctx, scope, "routine-fixture-"+newID())
	must(t, err)
	t.Cleanup(func() { k.Close() })
	return k, scope, fixture
}

func readEmployeeScheduleNextDue(t *testing.T, k *Kernel, scope Scope, employeeID string) sql.NullTime {
	t.Helper()
	var due sql.NullTime
	if err := k.pool.QueryRow(context.Background(), `SELECT next_due_at FROM employee_schedules WHERE company_id=$1 AND employee_id=$2`, scope.company, employeeID).Scan(&due); err != nil {
		t.Fatal(err)
	}
	return due
}

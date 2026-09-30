// pattern: Imperative Shell
package kernel

import (
	"context"
	"testing"
	"time"

	"polis/internal/core"
)

func TestEmployeeScheduleReconcilerListensReconnectsAndScans(t *testing.T) {
	k, scope, _ := newDailyRoutineEnv(t)
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	ownerID := "emp-backend"
	setEmployeeScheduleSleeping(t, ctx, k, scope, ownerID)
	stopListener, err := k.startEmployeeScheduleReconciler(ctx, time.Hour, nil)
	must(t, err)
	defer stopListener()
	waitForEmployeeScheduleState(t, ctx, k, scope, ownerID, core.EmployeeScheduleWakePending)

	setEmployeeScheduleSleeping(t, ctx, k, scope, ownerID)
	if _, err = k.pool.Exec(ctx, `SELECT pg_notify('polis_employee_wake',$1)`, scope.company+":"+ownerID); err != nil {
		t.Fatal(err)
	}
	waitForEmployeeScheduleState(t, ctx, k, scope, ownerID, core.EmployeeScheduleWakePending)

	setEmployeeScheduleSleeping(t, ctx, k, scope, ownerID)
	var listenerPID int
	if err = k.pool.QueryRow(ctx, `SELECT pid FROM pg_stat_activity
WHERE application_name='polis_employee_schedule_listener' AND pid<>pg_backend_pid() ORDER BY pid LIMIT 1`).Scan(&listenerPID); err != nil {
		t.Fatal(err)
	}
	var terminated bool
	if err = k.pool.QueryRow(ctx, `SELECT pg_terminate_backend($1)`, listenerPID).Scan(&terminated); err != nil || !terminated {
		t.Fatalf("terminate listener PID %d: terminated=%t err=%v", listenerPID, terminated, err)
	}
	waitForEmployeeScheduleState(t, ctx, k, scope, ownerID, core.EmployeeScheduleWakePending)

	stopPeriodic, err := k.startEmployeeScheduleReconciler(ctx, 20*time.Millisecond, nil)
	must(t, err)
	defer stopPeriodic()
	setEmployeeScheduleSleeping(t, ctx, k, scope, ownerID)
	waitForEmployeeScheduleState(t, ctx, k, scope, ownerID, core.EmployeeScheduleWakePending)
}

func TestEmployeeScheduleWakeHintStaysWithinCompanyScope(t *testing.T) {
	k, firstScope, _ := newDailyRoutineEnv(t)
	ctx := context.Background()
	secondScope, err := k.TXCreateCompany(ctx, "routine-company-other-"+newID())
	must(t, err)
	_, err = k.TXCreatePeerFixture(ctx, secondScope, "routine-fixture-other-"+newID())
	must(t, err)
	stop, err := k.startEmployeeScheduleReconciler(ctx, time.Hour, nil)
	must(t, err)
	defer stop()
	setEmployeeScheduleSleeping(t, ctx, k, firstScope, "emp-backend")
	setEmployeeScheduleSleeping(t, ctx, k, secondScope, "emp-backend")
	if _, err = k.pool.Exec(ctx, `SELECT pg_notify('polis_employee_wake',$1)`, secondScope.company+":emp-backend"); err != nil {
		t.Fatal(err)
	}
	waitForEmployeeScheduleState(t, ctx, k, secondScope, "emp-backend", core.EmployeeScheduleWakePending)
	first, err := k.EmployeeSchedule(ctx, firstScope, "emp-backend")
	must(t, err)
	if first.State != core.EmployeeScheduleSleeping {
		t.Fatalf("unrelated company's schedule changed after a scoped wake hint: %+v", first)
	}
}

func TestEmployeeScheduleReconcilerMaterializesDueDailyRoutine(t *testing.T) {
	k, scope, fixture := newDailyRoutineEnv(t)
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	location, err := time.LoadLocation("Asia/Shanghai")
	must(t, err)
	logicalDay := time.Now().In(location).AddDate(0, 0, -1).Format(time.DateOnly)
	routineID := "routine-auto-due-" + newID()
	schedule := core.DailyRoutineSchedule{
		RoutineID: routineID, TaskInstruction: "Review the assigned queue and record the next safe action.", Timezone: "Asia/Shanghai", LocalTime: "23:59",
		NextLogicalDay: logicalDay, CatchUpPolicy: core.RoutineCatchUpCoalesceLatest, MaxCatchUp: 1,
	}
	_, err = k.TXCreateDailyRoutine(ctx, scope, fixture.Backend.Mission, "emp-review", schedule, "routine-auto-create-"+newID())
	must(t, err)
	stop, err := k.startEmployeeScheduleReconciler(ctx, 20*time.Millisecond, nil)
	must(t, err)
	defer stop()
	waitForRoutineOccurrenceCount(t, ctx, k, scope, routineID, 1)
	scheduleState, err := k.EmployeeSchedule(ctx, scope, "emp-review")
	must(t, err)
	if scheduleState.State != core.EmployeeScheduleWakePending {
		t.Fatalf("automatically materialized Routine schedule=%+v, want wake_pending", scheduleState)
	}
	deadline := time.NewTimer(100 * time.Millisecond)
	defer deadline.Stop()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		var count int
		if err = k.pool.QueryRow(ctx, `SELECT count(*) FROM routine_occurrences WHERE company_id=$1 AND routine_id=$2 AND state='delivered' AND task_id IS NOT NULL`, scope.company, routineID).Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count != 1 {
			t.Fatalf("automatic Routine scanner created %d occurrences, want 1", count)
		}
		select {
		case <-deadline.C:
			return
		case <-ticker.C:
		}
	}
}

func setEmployeeScheduleSleeping(t *testing.T, ctx context.Context, k *Kernel, scope Scope, employeeID string) {
	t.Helper()
	_, err := k.pool.Exec(ctx, `UPDATE employee_schedules SET state='sleeping',checked_generation=work_generation,pause_reason=NULL
WHERE company_id=$1 AND employee_id=$2`, scope.company, employeeID)
	if err != nil {
		t.Fatal(err)
	}
}

func waitForEmployeeScheduleState(t *testing.T, ctx context.Context, k *Kernel, scope Scope, employeeID string, want core.EmployeeScheduleState) {
	t.Helper()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		schedule, err := k.EmployeeSchedule(ctx, scope, employeeID)
		if err == nil && schedule.State == want {
			return
		}
		select {
		case <-ctx.Done():
			t.Fatalf("waiting for employee schedule state %s: last=%+v err=%v: %v", want, schedule, err, ctx.Err())
		case <-ticker.C:
		}
	}
}

func waitForRoutineOccurrenceCount(t *testing.T, ctx context.Context, k *Kernel, scope Scope, routineID string, want int) {
	t.Helper()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		var count int
		err := k.pool.QueryRow(ctx, `SELECT count(*) FROM routine_occurrences WHERE company_id=$1 AND routine_id=$2 AND state='delivered' AND task_id IS NOT NULL`, scope.company, routineID).Scan(&count)
		if err == nil && count == want {
			return
		}
		select {
		case <-ctx.Done():
			t.Fatalf("waiting for %d delivered Routine Tasks: count=%d err=%v: %v", want, count, err, ctx.Err())
		case <-ticker.C:
		}
	}
}

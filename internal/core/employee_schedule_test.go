// pattern: Functional Core
package core

import "testing"

func TestActionableWorkAdvancesSleepingSchedule(t *testing.T) {
	schedule := EmployeeSchedule{State: EmployeeScheduleSleeping, WorkGeneration: 4, CheckedGeneration: 4}

	got := RecordEmployeeWorkSignal(schedule, true)
	if got.State != EmployeeScheduleWakePending || got.WorkGeneration != 5 {
		t.Fatalf("actionable work schedule = %+v, want wake_pending at generation 5", got)
	}
}

func TestSleepReconciliationRejectsStaleGenerationAndPendingWork(t *testing.T) {
	schedule := EmployeeSchedule{State: EmployeeScheduleQuiescing, WorkGeneration: 9, CheckedGeneration: 8}

	got := ReconcileEmployeeSleep(schedule, 8, false, false, false, false)
	if got.State != EmployeeScheduleWakePending || got.CheckedGeneration != 8 {
		t.Fatalf("stale sleep reconciliation = %+v, want wake_pending without advancing checked generation", got)
	}

	got = ReconcileEmployeeSleep(schedule, 9, true, false, false, false)
	if got.State != EmployeeScheduleWakePending || got.CheckedGeneration != 9 {
		t.Fatalf("sleep reconciliation with actionable work = %+v, want wake_pending", got)
	}
}

func TestPausedScheduleRecordsWorkButRequiresExplicitResume(t *testing.T) {
	schedule := EmployeeSchedule{State: EmployeeSchedulePaused, WorkGeneration: 2, CheckedGeneration: 2}

	got := RecordEmployeeWorkSignal(schedule, true)
	if got.State != EmployeeSchedulePaused || got.WorkGeneration != 3 {
		t.Fatalf("paused schedule after work arrival = %+v, want paused at generation 3", got)
	}

	got = ResumeEmployeeSchedule(got, true, false)
	if got.State != EmployeeScheduleWakePending {
		t.Fatalf("resumed schedule with pending work = %+v, want wake_pending", got)
	}
}

func TestNonActionableSignalDoesNotWakeOrAdvanceSchedule(t *testing.T) {
	schedule := EmployeeSchedule{State: EmployeeScheduleSleeping, WorkGeneration: 7, CheckedGeneration: 7}

	got := RecordEmployeeWorkSignal(schedule, false)
	if got != schedule {
		t.Fatalf("non-actionable signal changed schedule: got %+v want %+v", got, schedule)
	}
}

func TestScheduleWaitingOnQuotaDoesNotBecomeSleeping(t *testing.T) {
	schedule := EmployeeSchedule{State: EmployeeScheduleWaitingQuota, WorkGeneration: 5, CheckedGeneration: 5}

	got := ReconcileEmployeeSleep(schedule, 5, false, false, false, false)
	if got.State != EmployeeScheduleWaitingQuota {
		t.Fatalf("quota-waiting schedule reconciliation = %+v, want waiting_quota", got)
	}
}

func TestUnpausedScheduleDoesNotKeepAStalePauseBarrier(t *testing.T) {
	schedule := EmployeeSchedule{State: EmployeeSchedulePaused, WorkGeneration: 5, CheckedGeneration: 5}

	got := ReconcileEmployeeSleep(schedule, 5, true, false, false, false)
	if got.State != EmployeeScheduleWakePending {
		t.Fatalf("unpaused schedule with actionable work = %+v, want wake_pending", got)
	}
}

func TestUnpausedScheduleWithoutWorkClearsStalePauseBarrier(t *testing.T) {
	schedule := EmployeeSchedule{State: EmployeeSchedulePaused, WorkGeneration: 5, CheckedGeneration: 5}

	got := ReconcileEmployeeSleep(schedule, 5, false, false, false, false)
	if got.State != EmployeeScheduleSleeping {
		t.Fatalf("unpaused schedule without work = %+v, want sleeping", got)
	}
}

func TestReconcileEmployeeScheduleKeepsRestoringWorkerAdmitted(t *testing.T) {
	schedule := EmployeeSchedule{State: EmployeeScheduleAdmitted, WorkGeneration: 6, CheckedGeneration: 5}

	got := ReconcileEmployeeSleep(schedule, 6, true, true, false, false)
	if got != schedule {
		t.Fatalf("restoring Worker reconciliation = %+v, want admitted schedule unchanged %+v", got, schedule)
	}
}

func TestReconcileEmployeeScheduleMarksActiveWorkerWorking(t *testing.T) {
	schedule := EmployeeSchedule{State: EmployeeScheduleAdmitted, WorkGeneration: 6, CheckedGeneration: 5}

	got := ReconcileEmployeeSleep(schedule, 6, true, false, true, false)
	if got.State != EmployeeScheduleWorking || got.CheckedGeneration != 6 {
		t.Fatalf("active Worker reconciliation = %+v, want working with generation 6 checked", got)
	}
}

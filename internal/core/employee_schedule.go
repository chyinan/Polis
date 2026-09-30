// pattern: Functional Core
package core

type EmployeeScheduleState string

const (
	EmployeeScheduleQuiescing    EmployeeScheduleState = "quiescing"
	EmployeeScheduleSleeping     EmployeeScheduleState = "sleeping"
	EmployeeScheduleWakePending  EmployeeScheduleState = "wake_pending"
	EmployeeScheduleAdmitted     EmployeeScheduleState = "admitted"
	EmployeeScheduleWorking      EmployeeScheduleState = "working"
	EmployeeSchedulePaused       EmployeeScheduleState = "paused"
	EmployeeScheduleWaitingQuota EmployeeScheduleState = "waiting_quota"
)

type EmployeeSchedule struct {
	State             EmployeeScheduleState
	WorkGeneration    int64
	CheckedGeneration int64
}

// RecordEmployeeWorkSignal advances only for actionable work. Paused schedules
// preserve the work for a later explicit resume.
func RecordEmployeeWorkSignal(schedule EmployeeSchedule, actionable bool) EmployeeSchedule {
	if !actionable {
		return schedule
	}
	schedule.WorkGeneration++
	switch schedule.State {
	case EmployeeSchedulePaused, EmployeeScheduleWaitingQuota, EmployeeScheduleWorking, EmployeeScheduleAdmitted:
	default:
		schedule.State = EmployeeScheduleWakePending
	}
	return schedule
}

// ReconcileEmployeeSleep commits sleeping only when the caller's locked
// database snapshot still matches the schedule generation and no work or live
// Worker remains. The caller must hold the EmployeeSchedule row lock while it
// gathers these observations and persists the result.
func ReconcileEmployeeSleep(schedule EmployeeSchedule, observedGeneration int64, actionableWork, admittedWorker, activeWorker, paused bool) EmployeeSchedule {
	if paused {
		schedule.State = EmployeeSchedulePaused
		return schedule
	}
	if schedule.State == EmployeeScheduleWaitingQuota {
		return schedule
	}
	if admittedWorker {
		schedule.State = EmployeeScheduleAdmitted
		return schedule
	}
	if activeWorker {
		schedule.State = EmployeeScheduleWorking
		schedule.CheckedGeneration = observedGeneration
		return schedule
	}
	if actionableWork || observedGeneration != schedule.WorkGeneration {
		schedule.State = EmployeeScheduleWakePending
		if observedGeneration == schedule.WorkGeneration {
			schedule.CheckedGeneration = observedGeneration
		}
		return schedule
	}
	schedule.State = EmployeeScheduleSleeping
	schedule.CheckedGeneration = observedGeneration
	return schedule
}

// ResumeEmployeeSchedule never admits work while quota state is unresolved.
// Pending work or an unobserved generation produces a persistent wake request.
func ResumeEmployeeSchedule(schedule EmployeeSchedule, actionableWork, activeWorker bool) EmployeeSchedule {
	if schedule.State != EmployeeSchedulePaused {
		return schedule
	}
	if activeWorker {
		schedule.State = EmployeeScheduleWorking
		return schedule
	}
	if actionableWork || schedule.WorkGeneration != schedule.CheckedGeneration {
		schedule.State = EmployeeScheduleWakePending
		return schedule
	}
	schedule.State = EmployeeScheduleSleeping
	return schedule
}

// pattern: Imperative Shell
package kernel

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"polis/internal/core"
)

type DailyRoutineMaterialization struct {
	RoutineID      string
	NextLogicalDay string
	SkippedFrom    string
	SkippedThrough string
	Occurrences    []core.DailyRoutineOccurrence
}

type routineTaskPlan struct {
	Template         string    `json:"template"`
	RoutineID        string    `json:"routine_id"`
	OccurrenceKey    string    `json:"occurrence_key"`
	LogicalDay       string    `json:"logical_day"`
	ScheduledAt      time.Time `json:"scheduled_at"`
	CoalescedFrom    string    `json:"coalesced_from,omitempty"`
	CoalescedThrough string    `json:"coalesced_through,omitempty"`
	TaskInstruction  string    `json:"task_instruction"`
}

func (k *Kernel) TXCreateDailyRoutine(ctx context.Context, scope Scope, missionID, employeeID string, schedule core.DailyRoutineSchedule, key string) (Receipt, error) {
	if !core.ValidID(scope.company) || !core.ValidID(missionID) || !core.ValidID(employeeID) || core.ValidateDailyRoutineSchedule(schedule) != nil || core.ValidateDailyRoutineTaskInstruction(schedule.TaskInstruction) != nil {
		return Receipt{}, core.Malformed
	}
	nextDueAt, err := core.ResolveDailyRoutineTime(schedule.NextLogicalDay, schedule.LocalTime, schedule.Timezone)
	if err != nil {
		return Receipt{}, core.Malformed
	}
	input := struct {
		MissionID  string
		EmployeeID string
		Schedule   core.DailyRoutineSchedule
	}{missionID, employeeID, schedule}
	return k.TXWrite(ctx, scope, nil, key, "routine.daily.create", input, func(tx pgx.Tx) (Receipt, error) {
		var missionState, companyState string
		err := tx.QueryRow(ctx, `SELECT m.state,c.state FROM missions m JOIN companies c ON c.id=m.company_id WHERE m.company_id=$1 AND m.id=$2`, scope.company, missionID).Scan(&missionState, &companyState)
		if errors.Is(err, pgx.ErrNoRows) {
			return Receipt{}, core.OutOfScope
		}
		if err != nil {
			return Receipt{}, err
		}
		if companyState != "active" {
			return Receipt{}, core.ConflictError{Reason: "daily Routine company is not active", CurrentState: companyState}
		}
		if missionState != "active" && missionState != "paused" {
			return Receipt{}, core.ConflictError{Reason: "daily Routine requires an active or paused Mission", CurrentState: missionState}
		}
		var employeeEnabled bool
		err = tx.QueryRow(ctx, `SELECT enabled FROM employees WHERE company_id=$1 AND id=$2`, scope.company, employeeID).Scan(&employeeEnabled)
		if errors.Is(err, pgx.ErrNoRows) {
			return Receipt{}, core.OutOfScope
		}
		if err != nil {
			return Receipt{}, err
		}
		if !employeeEnabled {
			return Receipt{}, core.Denied
		}
		_, err = tx.Exec(ctx, `INSERT INTO routines(company_id,id,mission_id,employee_id,timezone,local_time,next_logical_day,catch_up_policy,max_catch_up,next_due_at,task_instruction)
VALUES($1,$2,$3,$4,$5,$6,$7::date,$8,$9,$10,$11)`, scope.company, schedule.RoutineID, missionID, employeeID,
			schedule.Timezone, schedule.LocalTime, schedule.NextLogicalDay, string(schedule.CatchUpPolicy), schedule.MaxCatchUp, nextDueAt, schedule.TaskInstruction)
		if err != nil {
			return Receipt{}, err
		}
		if err = ensureEmployeeScheduleTX(ctx, tx, scope, employeeID); err != nil {
			return Receipt{}, err
		}
		if err = refreshEmployeeNextDueTX(ctx, tx, scope, employeeID); err != nil {
			return Receipt{}, err
		}
		return Receipt{ID: schedule.RoutineID, Status: "active"}, nil
	})
}

// TXSetDailyRoutineTaskInstruction repairs legacy Routine definitions without
// inventing task content. Existing blocked occurrences and their Tasks are
// updated atomically; delivered Tasks keep their immutable snapshots.
func (k *Kernel) TXSetDailyRoutineTaskInstruction(ctx context.Context, scope Scope, missionID, routineID, instruction, key string) (Receipt, error) {
	if !core.ValidID(scope.company) || !core.ValidID(missionID) || !core.ValidID(routineID) || !core.ValidID(key) || core.ValidateDailyRoutineTaskInstruction(instruction) != nil {
		return Receipt{}, core.Malformed
	}
	input := struct {
		MissionID   string
		RoutineID   string
		Instruction string
	}{missionID, routineID, instruction}
	return k.TXWrite(ctx, scope, nil, key, "routine.daily.instruction", input, func(tx pgx.Tx) (Receipt, error) {
		var routineMissionID, employeeID, missionState, companyState, currentInstruction string
		err := tx.QueryRow(ctx, `SELECT r.mission_id,r.employee_id,m.state,c.state,COALESCE(r.task_instruction,'')
FROM routines r JOIN missions m ON m.company_id=r.company_id AND m.id=r.mission_id
JOIN companies c ON c.id=r.company_id WHERE r.company_id=$1 AND r.id=$2 FOR UPDATE OF r`, scope.company, routineID).Scan(
			&routineMissionID, &employeeID, &missionState, &companyState, &currentInstruction,
		)
		if errors.Is(err, pgx.ErrNoRows) {
			return Receipt{}, core.OutOfScope
		}
		if err != nil {
			return Receipt{}, err
		}
		if routineMissionID != missionID {
			return Receipt{}, core.OutOfScope
		}
		if companyState != "active" || (missionState != "active" && missionState != "paused") {
			return Receipt{}, core.ConflictError{Reason: "daily Routine instruction requires an active or paused Mission", CurrentState: missionState}
		}
		if currentInstruction != "" && currentInstruction != instruction {
			return Receipt{}, core.ConflictError{Reason: "daily Routine instruction is immutable after creation", CurrentState: "instruction_set"}
		}
		if currentInstruction == "" {
			if _, err = tx.Exec(ctx, `UPDATE routines SET task_instruction=$3,updated_at=now() WHERE company_id=$1 AND id=$2`, scope.company, routineID, instruction); err != nil {
				return Receipt{}, err
			}
		}
		rows, err := tx.Query(ctx, `SELECT occurrence_key,to_char(logical_day,'YYYY-MM-DD'),scheduled_at,
COALESCE(to_char(coalesced_from,'YYYY-MM-DD'),''),COALESCE(to_char(coalesced_through,'YYYY-MM-DD'),'')
FROM routine_occurrences WHERE company_id=$1 AND routine_id=$2 AND state='needs_instruction'
ORDER BY scheduled_at,occurrence_key FOR UPDATE`, scope.company, routineID)
		if err != nil {
			return Receipt{}, err
		}
		occurrences := make([]core.DailyRoutineOccurrence, 0, 10)
		for rows.Next() {
			var occurrence core.DailyRoutineOccurrence
			if err = rows.Scan(&occurrence.OccurrenceKey, &occurrence.LogicalDay, &occurrence.ScheduledAt, &occurrence.CoalescedFrom, &occurrence.CoalescedThrough); err != nil {
				rows.Close()
				return Receipt{}, err
			}
			occurrences = append(occurrences, occurrence)
		}
		if err = rows.Err(); err != nil {
			rows.Close()
			return Receipt{}, err
		}
		rows.Close()
		for _, occurrence := range occurrences {
			taskID, taskErr := k.createRoutineTaskTX(ctx, tx, scope, routineMissionID, employeeID, routineID, occurrence, instruction)
			if taskErr != nil {
				return Receipt{}, taskErr
			}
			if _, err = tx.Exec(ctx, `UPDATE routine_occurrences SET state='delivered',task_instruction_snapshot=$4,task_id=$5
WHERE company_id=$1 AND routine_id=$2 AND occurrence_key=$3 AND state='needs_instruction' AND task_id IS NULL`,
				scope.company, routineID, occurrence.OccurrenceKey, instruction, taskID); err != nil {
				return Receipt{}, err
			}
		}
		return Receipt{ID: routineID, Status: "instruction_set", Revision: int64(len(occurrences))}, nil
	})
}

func (k *Kernel) createRoutineTaskTX(ctx context.Context, tx pgx.Tx, scope Scope, missionID, employeeID, routineID string, occurrence core.DailyRoutineOccurrence, instruction string) (string, error) {
	taskID := newID()
	planData, err := json.Marshal(routineTaskPlan{
		Template: "daily-routine-task@1", RoutineID: routineID, OccurrenceKey: occurrence.OccurrenceKey,
		LogicalDay: occurrence.LogicalDay, ScheduledAt: occurrence.ScheduledAt,
		CoalescedFrom: occurrence.CoalescedFrom, CoalescedThrough: occurrence.CoalescedThrough,
		TaskInstruction: instruction,
	})
	if err != nil {
		return "", err
	}
	workspace := fmt.Sprintf("# Scheduled routine\n\n%s\n\nOccurrence: %s\nScheduled at: %s\n", instruction, occurrence.OccurrenceKey, occurrence.ScheduledAt.Format(time.RFC3339))
	digest, err := k.putBlobInTX(ctx, tx, scope.company, []byte(workspace))
	if err != nil {
		return "", err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO tasks(company_id,id,mission_id,owner,kind,state,plan)
VALUES($1,$2,$3,$4,$5,'ready',$6::jsonb)`, scope.company, taskID, missionID, employeeID, core.TaskKindCompute, planData); err != nil {
		return "", err
	}
	if err = bindMissionInputManifest(ctx, tx, scope, missionID, taskID); err != nil {
		return "", err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO worker_workspaces(company_id,task_id,digest) VALUES($1,$2,$3)`, scope.company, taskID, digest); err != nil {
		return "", err
	}
	return taskID, nil
}

func (k *Kernel) TXMaterializeDailyRoutine(ctx context.Context, scope Scope, routineID string, now time.Time, key string) (DailyRoutineMaterialization, error) {
	if !core.ValidID(scope.company) || !core.ValidID(routineID) || now.IsZero() {
		return DailyRoutineMaterialization{}, core.Malformed
	}
	now = now.UTC()
	input := struct {
		RoutineID string
		Now       time.Time
	}{routineID, now}
	_, err := k.TXWrite(ctx, scope, nil, key, "routine.daily.materialize", input, func(tx pgx.Tx) (Receipt, error) {
		var schedule core.DailyRoutineSchedule
		var missionID, employeeID, missionState, companyState string
		err := tx.QueryRow(ctx, `SELECT r.mission_id,r.employee_id,m.state,c.state,r.timezone,r.local_time,
to_char(r.next_logical_day,'YYYY-MM-DD'),r.catch_up_policy,r.max_catch_up,COALESCE(r.task_instruction,'')
FROM routines r JOIN missions m ON m.company_id=r.company_id AND m.id=r.mission_id
JOIN companies c ON c.id=r.company_id
WHERE r.company_id=$1 AND r.id=$2 FOR UPDATE OF r`, scope.company, routineID).Scan(
			&missionID, &employeeID, &missionState, &companyState, &schedule.Timezone, &schedule.LocalTime,
			&schedule.NextLogicalDay, &schedule.CatchUpPolicy, &schedule.MaxCatchUp, &schedule.TaskInstruction,
		)
		if errors.Is(err, pgx.ErrNoRows) {
			return Receipt{}, core.OutOfScope
		}
		if err != nil {
			return Receipt{}, err
		}
		if companyState != "active" {
			return Receipt{}, core.ConflictError{Reason: "daily Routine company is not active", CurrentState: companyState}
		}
		if missionState != "active" && missionState != "paused" {
			return Receipt{}, core.ConflictError{Reason: "daily Routine Mission is not active", CurrentState: missionState}
		}
		schedule.RoutineID = routineID
		plan, err := core.PlanDailyRoutineOccurrences(schedule, now)
		if err != nil {
			return Receipt{}, err
		}
		nextDueAt, err := core.ResolveDailyRoutineTime(plan.NextLogicalDay, schedule.LocalTime, schedule.Timezone)
		if err != nil {
			return Receipt{}, core.Integrity
		}
		_, err = tx.Exec(ctx, `INSERT INTO routine_materializations(company_id,routine_id,request_id,planned_at,next_logical_day,occurrence_count,skipped_from,skipped_through)
VALUES($1,$2,$3,$4,$5::date,$6,NULLIF($7,'')::date,NULLIF($8,'')::date)`, scope.company, routineID, key, now,
			plan.NextLogicalDay, len(plan.Occurrences), plan.SkippedFrom, plan.SkippedThrough)
		if err != nil {
			return Receipt{}, err
		}
		for _, occurrence := range plan.Occurrences {
			state, taskID, instructionSnapshot := "needs_instruction", "", ""
			if schedule.TaskInstruction != "" {
				taskID, err = k.createRoutineTaskTX(ctx, tx, scope, missionID, employeeID, routineID, occurrence, schedule.TaskInstruction)
				if err != nil {
					return Receipt{}, err
				}
				state, instructionSnapshot = "delivered", schedule.TaskInstruction
			}
			_, err = tx.Exec(ctx, `INSERT INTO routine_occurrences(company_id,routine_id,occurrence_key,logical_day,scheduled_at,coalesced_from,coalesced_through,materialization_request_id,state,task_instruction_snapshot,task_id)
VALUES($1,$2,$3,$4::date,$5,NULLIF($6,'')::date,NULLIF($7,'')::date,$8,$9,NULLIF($10,''),NULLIF($11,''))`, scope.company, routineID,
				occurrence.OccurrenceKey, occurrence.LogicalDay, occurrence.ScheduledAt, occurrence.CoalescedFrom,
				occurrence.CoalescedThrough, key, state, instructionSnapshot, taskID)
			if err != nil {
				return Receipt{}, err
			}
		}
		if _, err = tx.Exec(ctx, `UPDATE routines SET next_logical_day=$3::date,next_due_at=$4,updated_at=now() WHERE company_id=$1 AND id=$2`, scope.company, routineID, plan.NextLogicalDay, nextDueAt); err != nil {
			return Receipt{}, err
		}
		if err = refreshEmployeeNextDueTX(ctx, tx, scope, employeeID); err != nil {
			return Receipt{}, err
		}
		status := "no_due"
		if len(plan.Occurrences) > 0 {
			status = "materialized"
		}
		return Receipt{ID: routineID, Status: status, Revision: int64(len(plan.Occurrences))}, nil
	})
	if err != nil {
		return DailyRoutineMaterialization{}, err
	}
	return k.dailyRoutineMaterialization(ctx, scope, routineID, key)
}

func (k *Kernel) dailyRoutineMaterialization(ctx context.Context, scope Scope, routineID, key string) (DailyRoutineMaterialization, error) {
	result := DailyRoutineMaterialization{RoutineID: routineID, Occurrences: make([]core.DailyRoutineOccurrence, 0)}
	err := k.pool.QueryRow(ctx, `SELECT to_char(next_logical_day,'YYYY-MM-DD'),
COALESCE(to_char(skipped_from,'YYYY-MM-DD'),''),COALESCE(to_char(skipped_through,'YYYY-MM-DD'),'')
FROM routine_materializations WHERE company_id=$1 AND routine_id=$2 AND request_id=$3`, scope.company, routineID, key).Scan(
		&result.NextLogicalDay, &result.SkippedFrom, &result.SkippedThrough,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return DailyRoutineMaterialization{}, core.Integrity
	}
	if err != nil {
		return DailyRoutineMaterialization{}, err
	}
	rows, err := k.pool.Query(ctx, `SELECT occurrence_key,to_char(logical_day,'YYYY-MM-DD'),scheduled_at,
COALESCE(to_char(coalesced_from,'YYYY-MM-DD'),''),COALESCE(to_char(coalesced_through,'YYYY-MM-DD'),''),state,COALESCE(task_id,'')
FROM routine_occurrences WHERE company_id=$1 AND routine_id=$2 AND materialization_request_id=$3 ORDER BY logical_day`, scope.company, routineID, key)
	if err != nil {
		return DailyRoutineMaterialization{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var occurrence core.DailyRoutineOccurrence
		if err = rows.Scan(&occurrence.OccurrenceKey, &occurrence.LogicalDay, &occurrence.ScheduledAt,
			&occurrence.CoalescedFrom, &occurrence.CoalescedThrough, &occurrence.State, &occurrence.TaskID); err != nil {
			return DailyRoutineMaterialization{}, err
		}
		result.Occurrences = append(result.Occurrences, occurrence)
	}
	return result, rows.Err()
}

func refreshEmployeeNextDueTX(ctx context.Context, tx pgx.Tx, scope Scope, employeeID string) error {
	var nextDue sql.NullTime
	err := tx.QueryRow(ctx, `SELECT r.next_due_at
FROM routines r JOIN missions m ON m.company_id=r.company_id AND m.id=r.mission_id
JOIN companies c ON c.id=r.company_id
WHERE r.company_id=$1 AND r.employee_id=$2 AND c.state='active' AND m.state='active' AND r.scheduling_active AND r.next_due_at IS NOT NULL
ORDER BY r.next_due_at,r.id LIMIT 1`, scope.company, employeeID).Scan(&nextDue)
	if errors.Is(err, pgx.ErrNoRows) {
		nextDue.Valid = false
	} else if err != nil {
		return err
	}
	var nextDueValue any
	if nextDue.Valid {
		nextDueValue = nextDue.Time
	}
	_, err = tx.Exec(ctx, `UPDATE employee_schedules SET next_due_at=$3,updated_at=now()
WHERE company_id=$1 AND employee_id=$2`, scope.company, employeeID, nextDueValue)
	return err
}

// pattern: Imperative Shell
package workbench

import (
	"context"
	"database/sql"
	"time"

	"polis/internal/core"
)

type DailyRoutineView struct {
	RoutineID                   string  `json:"routineId"`
	MissionID                   string  `json:"missionId"`
	EmployeeID                  string  `json:"employeeId"`
	Timezone                    string  `json:"timezone"`
	LocalTime                   string  `json:"localTime"`
	NextLogicalDay              string  `json:"nextLogicalDay"`
	CatchUpPolicy               string  `json:"catchUpPolicy"`
	MaxCatchUp                  int     `json:"maxCatchUp"`
	NextDueAt                   *string `json:"nextDueAt"`
	TaskInstruction             *string `json:"taskInstruction"`
	NeedsInstructionOccurrences int64   `json:"needsInstructionOccurrences"`
	LinkedTaskCount             int64   `json:"linkedTaskCount"`
}

type DailyRoutineReader interface {
	ListDailyRoutines(ctx context.Context, companyID, missionID string) ([]DailyRoutineView, error)
}

func (s *PostgresReadStore) ListDailyRoutines(ctx context.Context, companyID, missionID string) ([]DailyRoutineView, error) {
	if err := validateCompanyID(companyID); err != nil || !core.ValidID(missionID) {
		return nil, core.Malformed
	}
	var exists bool
	if err := s.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM missions WHERE company_id=$1 AND id=$2)`, companyID, missionID).Scan(&exists); err != nil {
		return nil, err
	}
	if !exists {
		return nil, core.OutOfScope
	}
	rows, err := s.pool.Query(ctx, `SELECT r.id,r.mission_id,r.employee_id,r.timezone,r.local_time,
to_char(r.next_logical_day,'YYYY-MM-DD'),r.catch_up_policy,r.max_catch_up,r.next_due_at,r.task_instruction,
(SELECT count(*) FROM routine_occurrences o WHERE o.company_id=r.company_id AND o.routine_id=r.id AND o.state='needs_instruction'),
(SELECT count(*) FROM routine_occurrences o WHERE o.company_id=r.company_id AND o.routine_id=r.id AND o.task_id IS NOT NULL)
FROM routines r WHERE r.company_id=$1 AND r.mission_id=$2 ORDER BY r.next_due_at NULLS LAST,r.id`, companyID, missionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]DailyRoutineView, 0)
	for rows.Next() {
		var item DailyRoutineView
		var dueAt sql.NullTime
		var instruction sql.NullString
		if err = rows.Scan(&item.RoutineID, &item.MissionID, &item.EmployeeID, &item.Timezone, &item.LocalTime,
			&item.NextLogicalDay, &item.CatchUpPolicy, &item.MaxCatchUp, &dueAt, &instruction,
			&item.NeedsInstructionOccurrences, &item.LinkedTaskCount); err != nil {
			return nil, err
		}
		if dueAt.Valid {
			formatted := dueAt.Time.UTC().Format(time.RFC3339Nano)
			item.NextDueAt = &formatted
		}
		if instruction.Valid {
			item.TaskInstruction = &instruction.String
		}
		items = append(items, item)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

var _ DailyRoutineReader = (*PostgresReadStore)(nil)

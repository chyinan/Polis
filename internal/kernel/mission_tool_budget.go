// pattern: Imperative Shell
package kernel

import (
	"context"
	"errors"
	"math"
	"strings"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"polis/internal/core"
)

type MissionToolCallBudget struct {
	MissionID           string `json:"missionId"`
	Title               string `json:"title"`
	MissionState        string `json:"missionState"`
	ToolCallLimit       *int64 `json:"toolCallLimit"`
	ToolCallsUsed       int64  `json:"toolCallsUsed"`
	ToolCallsRemaining  int64  `json:"toolCallsRemaining"`
	Revision            int64  `json:"revision"`
	State               string `json:"state"`
	AllocationCount     int64  `json:"allocationCount"`
	LastReason          string `json:"lastReason,omitempty"`
	LastAllocatedAt     string `json:"lastAllocatedAt,omitempty"`
	RejectionCount      int64  `json:"rejectionCount"`
	LastRejectionAt     string `json:"lastRejectionAt,omitempty"`
	LastRejectionRoute  string `json:"lastRejectionRoute,omitempty"`
	LastRejectionReason string `json:"lastRejectionReason,omitempty"`
	LastRejectionTaskID string `json:"lastRejectionTaskId,omitempty"`
}

type MissionToolCallBudgetList struct {
	Items     []MissionToolCallBudget `json:"items"`
	Truncated bool                    `json:"truncated"`
}

type MissionToolCallBudgetChangeInput struct {
	RequestID        string
	ExpectedLimit    *int64
	ExpectedRevision int64
	ResultingLimit   int64
	Reason           string
}

type missionToolCallBudgetSnapshot struct {
	State    string
	Limit    pgtype.Int8
	Used     int64
	Revision int64
}

func lockMissionToolCallBudgetTX(ctx context.Context, tx pgx.Tx, companyID, missionID string) (missionToolCallBudgetSnapshot, error) {
	var snapshot missionToolCallBudgetSnapshot
	err := tx.QueryRow(ctx, `SELECT state,mission_tool_call_limit,mission_tool_calls_used,mission_tool_call_budget_revision
FROM missions WHERE company_id=$1 AND id=$2 FOR UPDATE`, companyID, missionID).
		Scan(&snapshot.State, &snapshot.Limit, &snapshot.Used, &snapshot.Revision)
	if errors.Is(err, pgx.ErrNoRows) {
		return missionToolCallBudgetSnapshot{}, core.OutOfScope
	}
	return snapshot, err
}

func missionToolCallBudgetRejectionReason(snapshot missionToolCallBudgetSnapshot) string {
	if !snapshot.Limit.Valid {
		return "mission_budget_pending"
	}
	if snapshot.Limit.Int64 <= snapshot.Used {
		return "mission_limit"
	}
	return ""
}

func recordMissionToolCallBudgetRejection(ctx context.Context, tx pgx.Tx, companyID, missionID, taskID, sessionID, requestID, route, reason string, snapshot missionToolCallBudgetSnapshot) error {
	var session any
	if sessionID != "" {
		session = sessionID
	}
	_, err := tx.Exec(ctx, `INSERT INTO mission_tool_call_budget_rejections(company_id,request_id,mission_id,task_id,
worker_session_id,route,reason,mission_tool_call_limit,mission_tool_calls_used,mission_budget_revision)
VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`, companyID, requestID, missionID, taskID, session, route, reason,
		snapshot.Limit, snapshot.Used, snapshot.Revision)
	return err
}

func (k *Kernel) ListMissionToolCallBudgets(ctx context.Context, scope Scope, limit int) (MissionToolCallBudgetList, error) {
	if k == nil || ctx == nil || !core.ValidID(scope.company) || limit < 1 || limit > 100 {
		return MissionToolCallBudgetList{}, core.Malformed
	}
	tx, err := k.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return MissionToolCallBudgetList{}, err
	}
	defer tx.Rollback(ctx)
	if err = k.checkRuntimeLease(ctx, tx); err != nil {
		return MissionToolCallBudgetList{}, err
	}
	rows, err := tx.Query(ctx, `SELECT m.id,m.title,m.state,m.mission_tool_call_limit,m.mission_tool_calls_used,
m.mission_tool_call_budget_revision,
(SELECT count(*) FROM mission_tool_call_budget_allocations a WHERE a.company_id=m.company_id AND a.mission_id=m.id),
COALESCE((SELECT a.reason FROM mission_tool_call_budget_allocations a WHERE a.company_id=m.company_id AND a.mission_id=m.id ORDER BY a.revision DESC LIMIT 1),''),
COALESCE((SELECT a.created_at::text FROM mission_tool_call_budget_allocations a WHERE a.company_id=m.company_id AND a.mission_id=m.id ORDER BY a.revision DESC LIMIT 1),''),
(SELECT count(*) FROM mission_tool_call_budget_rejections d WHERE d.company_id=m.company_id AND d.mission_id=m.id),
COALESCE((SELECT d.occurred_at::text FROM mission_tool_call_budget_rejections d WHERE d.company_id=m.company_id AND d.mission_id=m.id ORDER BY d.occurred_at DESC,d.request_id DESC LIMIT 1),''),
COALESCE((SELECT d.route FROM mission_tool_call_budget_rejections d WHERE d.company_id=m.company_id AND d.mission_id=m.id ORDER BY d.occurred_at DESC,d.request_id DESC LIMIT 1),''),
COALESCE((SELECT d.reason FROM mission_tool_call_budget_rejections d WHERE d.company_id=m.company_id AND d.mission_id=m.id ORDER BY d.occurred_at DESC,d.request_id DESC LIMIT 1),''),
COALESCE((SELECT d.task_id FROM mission_tool_call_budget_rejections d WHERE d.company_id=m.company_id AND d.mission_id=m.id ORDER BY d.occurred_at DESC,d.request_id DESC LIMIT 1),'')
FROM missions m WHERE m.company_id=$1 ORDER BY m.id LIMIT $2`, scope.company, limit+1)
	if err != nil {
		return MissionToolCallBudgetList{}, err
	}
	defer rows.Close()
	result := MissionToolCallBudgetList{Items: make([]MissionToolCallBudget, 0, limit)}
	for rows.Next() {
		var item MissionToolCallBudget
		var cap pgtype.Int8
		if err = rows.Scan(&item.MissionID, &item.Title, &item.MissionState, &cap, &item.ToolCallsUsed,
			&item.Revision, &item.AllocationCount, &item.LastReason, &item.LastAllocatedAt,
			&item.RejectionCount, &item.LastRejectionAt, &item.LastRejectionRoute, &item.LastRejectionReason, &item.LastRejectionTaskID); err != nil {
			return MissionToolCallBudgetList{}, err
		}
		if len(result.Items) == limit {
			result.Truncated = true
			break
		}
		item.State = "pending"
		item.ToolCallsRemaining = 0
		if cap.Valid {
			current := cap.Int64
			item.ToolCallLimit = &current
			if item.ToolCallsUsed >= current {
				item.State = "exhausted"
			} else {
				item.State = "available"
				item.ToolCallsRemaining = current - item.ToolCallsUsed
			}
		}
		if item.AllocationCount == 0 {
			item.LastReason = ""
			item.LastAllocatedAt = ""
		}
		if item.RejectionCount == 0 {
			item.LastRejectionAt = ""
			item.LastRejectionRoute = ""
			item.LastRejectionReason = ""
			item.LastRejectionTaskID = ""
		}
		result.Items = append(result.Items, item)
	}
	if err = rows.Err(); err != nil {
		return MissionToolCallBudgetList{}, err
	}
	rows.Close()
	return result, tx.Commit(ctx)
}

func (k *Kernel) TXChangeMissionToolCallBudget(ctx context.Context, scope Scope, missionID string, input MissionToolCallBudgetChangeInput) (Receipt, error) {
	reason := strings.TrimSpace(input.Reason)
	if k == nil || ctx == nil || !core.ValidID(scope.company) || !core.ValidID(missionID) ||
		!core.ValidID(input.RequestID) || input.ExpectedRevision < 0 || input.ResultingLimit < 1 ||
		(input.ExpectedLimit != nil && *input.ExpectedLimit < 1) || utf8.RuneCountInString(reason) < 1 || utf8.RuneCountInString(reason) > 500 {
		return Receipt{}, core.Malformed
	}
	command := struct {
		MissionID string
		Input     MissionToolCallBudgetChangeInput
		Reason    string
	}{missionID, input, reason}
	return k.TXWrite(ctx, scope, nil, input.RequestID, "mission.tool_budget.change", command, func(tx pgx.Tx) (Receipt, error) {
		var state string
		var currentLimit pgtype.Int8
		var used, revision int64
		err := tx.QueryRow(ctx, `SELECT state,mission_tool_call_limit,mission_tool_calls_used,mission_tool_call_budget_revision
FROM missions WHERE company_id=$1 AND id=$2 FOR UPDATE`, scope.company, missionID).Scan(&state, &currentLimit, &used, &revision)
		if errors.Is(err, pgx.ErrNoRows) {
			return Receipt{}, core.OutOfScope
		}
		if err != nil {
			return Receipt{}, err
		}
		if currentLimit.Valid != (input.ExpectedLimit != nil) ||
			(currentLimit.Valid && currentLimit.Int64 != *input.ExpectedLimit) || revision != input.ExpectedRevision {
			return Receipt{}, core.Conflict
		}
		if revision == math.MaxInt64 || input.ResultingLimit < used {
			return Receipt{}, core.Conflict
		}
		operation := "configure"
		var previous any
		additional := input.ResultingLimit
		if currentLimit.Valid {
			if input.ResultingLimit <= currentLimit.Int64 {
				return Receipt{}, core.Denied
			}
			operation = "allocate"
			previous = currentLimit.Int64
			additional = input.ResultingLimit - currentLimit.Int64
		}
		newRevision := revision + 1
		if _, err = tx.Exec(ctx, `INSERT INTO mission_tool_call_budget_allocations(company_id,mission_id,revision,request_id,
expected_revision,operation,previous_limit,additional_tool_calls,resulting_limit,authorized_by,reason)
VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,'local-owner',$10)`, scope.company, missionID, newRevision, input.RequestID,
			input.ExpectedRevision, operation, previous, additional, input.ResultingLimit, reason); err != nil {
			return Receipt{}, err
		}
		if _, err = tx.Exec(ctx, `UPDATE missions SET mission_tool_call_limit=$3,mission_tool_call_budget_revision=$4
WHERE company_id=$1 AND id=$2`, scope.company, missionID, input.ResultingLimit, newRevision); err != nil {
			return Receipt{}, err
		}
		status := "mission_budget_configured"
		if operation == "allocate" {
			status = "mission_budget_allocated"
		}
		return Receipt{ID: missionID, Status: status, Revision: newRevision}, nil
	})
}

func configureMissionToolCallBudgetTX(ctx context.Context, tx pgx.Tx, companyID, missionID, requestID string, limit int64, reason string) error {
	if limit < 1 {
		return core.Malformed
	}
	var currentLimit pgtype.Int8
	var used, revision int64
	if err := tx.QueryRow(ctx, `SELECT mission_tool_call_limit,mission_tool_calls_used,mission_tool_call_budget_revision
FROM missions WHERE company_id=$1 AND id=$2 FOR UPDATE`, companyID, missionID).Scan(&currentLimit, &used, &revision); err != nil {
		return err
	}
	if currentLimit.Valid || revision != 0 || limit < used {
		return core.Conflict
	}
	if _, err := tx.Exec(ctx, `INSERT INTO mission_tool_call_budget_allocations(company_id,mission_id,revision,request_id,
expected_revision,operation,previous_limit,additional_tool_calls,resulting_limit,authorized_by,reason)
VALUES($1,$2,1,$3,0,'configure',NULL,$4,$4,'local-owner',$5)`, companyID, missionID, requestID, limit, reason); err != nil {
		return err
	}
	_, err := tx.Exec(ctx, `UPDATE missions SET mission_tool_call_limit=$3,mission_tool_call_budget_revision=1
WHERE company_id=$1 AND id=$2`, companyID, missionID, limit)
	return err
}

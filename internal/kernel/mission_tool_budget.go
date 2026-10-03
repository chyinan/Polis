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
	MissionID                string `json:"missionId"`
	Title                    string `json:"title"`
	MissionState             string `json:"missionState"`
	ToolCallLimit            *int64 `json:"toolCallLimit"`
	ToolCallsUsed            int64  `json:"toolCallsUsed"`
	ToolCallsRemaining       int64  `json:"toolCallsRemaining"`
	ClosingReserveToolCalls  int64  `json:"closingReserveToolCalls"`
	ClosingReserveRemaining  int64  `json:"closingReserveRemaining"`
	ClosingReserveRevision   int64  `json:"closingReserveRevision"`
	Revision                 int64  `json:"revision"`
	State                    string `json:"state"`
	AllocationCount          int64  `json:"allocationCount"`
	LastReason               string `json:"lastReason,omitempty"`
	LastAllocatedAt          string `json:"lastAllocatedAt,omitempty"`
	LastClosingReserveReason string `json:"lastClosingReserveReason,omitempty"`
	LastClosingReserveAt     string `json:"lastClosingReserveAt,omitempty"`
	RejectionCount           int64  `json:"rejectionCount"`
	LastRejectionAt          string `json:"lastRejectionAt,omitempty"`
	LastRejectionRoute       string `json:"lastRejectionRoute,omitempty"`
	LastRejectionReason      string `json:"lastRejectionReason,omitempty"`
	LastRejectionTaskID      string `json:"lastRejectionTaskId,omitempty"`
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

type MissionToolCallClosingReserveInput struct {
	RequestID               string
	ReservedCalls           int64
	ExpectedLimit           *int64
	ExpectedBudgetRevision  int64
	ExpectedReserveRevision int64
	Reason                  string
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

func missionToolCallBudgetRejectionReason(snapshot missionToolCallBudgetSnapshot, closingTask bool, reserveRemaining int64) string {
	if !snapshot.Limit.Valid {
		return "mission_budget_pending"
	}
	if snapshot.Limit.Int64 <= snapshot.Used {
		return "mission_limit"
	}
	if !closingTask && snapshot.Limit.Int64-snapshot.Used <= reserveRemaining {
		return "mission_closing_reserve"
	}
	return ""
}

func recordMissionToolCallBudgetRejection(ctx context.Context, tx pgx.Tx, companyID, missionID, taskID, sessionID, requestID, route, reason string, snapshot missionToolCallBudgetSnapshot, reserve, reserveRemaining, reserveRevision int64) error {
	var session any
	if sessionID != "" {
		session = sessionID
	}
	_, err := tx.Exec(ctx, `INSERT INTO mission_tool_call_budget_rejections(company_id,request_id,mission_id,task_id,
worker_session_id,route,reason,mission_tool_call_limit,mission_tool_calls_used,mission_budget_revision,
mission_closing_reserve_tool_calls,mission_closing_reserve_remaining,mission_closing_reserve_revision)
VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)`, companyID, requestID, missionID, taskID, session, route, reason,
		snapshot.Limit, snapshot.Used, snapshot.Revision, reserve, reserveRemaining, reserveRevision)
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
(SELECT COALESCE((SELECT r.reserved_tool_calls FROM mission_tool_call_closing_reserves r WHERE r.company_id=m.company_id AND r.mission_id=m.id ORDER BY r.revision DESC LIMIT 1),0)),
(SELECT COALESCE((SELECT r.revision FROM mission_tool_call_closing_reserves r WHERE r.company_id=m.company_id AND r.mission_id=m.id ORDER BY r.revision DESC LIMIT 1),0)),
(SELECT COALESCE((SELECT r.closing_tool_calls_used_at_revision FROM mission_tool_call_closing_reserves r WHERE r.company_id=m.company_id AND r.mission_id=m.id ORDER BY r.revision DESC LIMIT 1),0)),
(SELECT COALESCE(sum(t.task_tool_calls_used),0)::bigint FROM tasks t WHERE t.company_id=m.company_id AND t.mission_id=m.id AND t.kind IN ('review','peer_review')),
COALESCE((SELECT r.reason FROM mission_tool_call_closing_reserves r WHERE r.company_id=m.company_id AND r.mission_id=m.id ORDER BY r.revision DESC LIMIT 1),''),
COALESCE((SELECT r.created_at::text FROM mission_tool_call_closing_reserves r WHERE r.company_id=m.company_id AND r.mission_id=m.id ORDER BY r.revision DESC LIMIT 1),''),
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
		var closingUsedAtRevision, closingUsed int64
		if err = rows.Scan(&item.MissionID, &item.Title, &item.MissionState, &cap, &item.ToolCallsUsed,
			&item.Revision, &item.AllocationCount, &item.LastReason, &item.LastAllocatedAt,
			&item.ClosingReserveToolCalls, &item.ClosingReserveRevision, &closingUsedAtRevision, &closingUsed,
			&item.LastClosingReserveReason, &item.LastClosingReserveAt,
			&item.RejectionCount, &item.LastRejectionAt, &item.LastRejectionRoute, &item.LastRejectionReason, &item.LastRejectionTaskID); err != nil {
			return MissionToolCallBudgetList{}, err
		}
		if len(result.Items) == limit {
			result.Truncated = true
			break
		}
		item.State = "pending"
		item.ToolCallsRemaining = 0
		if closingUsed < closingUsedAtRevision {
			return MissionToolCallBudgetList{}, core.Integrity
		}
		item.ClosingReserveRemaining = item.ClosingReserveToolCalls - (closingUsed - closingUsedAtRevision)
		if item.ClosingReserveRemaining < 0 {
			item.ClosingReserveRemaining = 0
		}
		if cap.Valid {
			current := cap.Int64
			item.ToolCallLimit = &current
			if item.ToolCallsUsed >= current {
				item.State = "exhausted"
			} else {
				item.State = "available"
				item.ToolCallsRemaining = current - item.ToolCallsUsed
				if item.ClosingReserveRemaining > 0 && item.ToolCallsRemaining <= item.ClosingReserveRemaining {
					item.State = "closing_reserved"
				}
			}
		}
		if item.AllocationCount == 0 {
			item.LastReason = ""
			item.LastAllocatedAt = ""
		}
		if item.ClosingReserveRevision == 0 {
			item.LastClosingReserveReason = ""
			item.LastClosingReserveAt = ""
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
		_, reserveRemaining, _, reserveErr := missionToolCallClosingReserveSnapshot(ctx, tx, scope.company, missionID)
		if reserveErr != nil {
			return Receipt{}, reserveErr
		}
		if input.ResultingLimit-used < reserveRemaining {
			return Receipt{}, core.ToolCallBudgetExceeded
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

func missionToolCallClosingReserveSnapshot(ctx context.Context, q problemBudgetQueryRower, companyID, missionID string) (int64, int64, int64, error) {
	var reserved, revision, usedAtRevision, closingUsed int64
	err := q.QueryRow(ctx, `SELECT
 COALESCE((SELECT r.reserved_tool_calls FROM mission_tool_call_closing_reserves r WHERE r.company_id=$1 AND r.mission_id=$2 ORDER BY r.revision DESC LIMIT 1),0),
 COALESCE((SELECT r.revision FROM mission_tool_call_closing_reserves r WHERE r.company_id=$1 AND r.mission_id=$2 ORDER BY r.revision DESC LIMIT 1),0),
 COALESCE((SELECT r.closing_tool_calls_used_at_revision FROM mission_tool_call_closing_reserves r WHERE r.company_id=$1 AND r.mission_id=$2 ORDER BY r.revision DESC LIMIT 1),0),
 (SELECT COALESCE(sum(t.task_tool_calls_used),0)::bigint FROM tasks t WHERE t.company_id=$1 AND t.mission_id=$2 AND t.kind IN ('review','peer_review'))`, companyID, missionID).
		Scan(&reserved, &revision, &usedAtRevision, &closingUsed)
	if err != nil {
		return 0, 0, 0, err
	}
	if closingUsed < usedAtRevision {
		return 0, 0, 0, core.Integrity
	}
	remaining := reserved - (closingUsed - usedAtRevision)
	if remaining < 0 {
		remaining = 0
	}
	return reserved, remaining, revision, nil
}

func (k *Kernel) TXSetMissionToolCallClosingReserve(ctx context.Context, scope Scope, missionID string, input MissionToolCallClosingReserveInput) (Receipt, error) {
	reason := strings.TrimSpace(input.Reason)
	if k == nil || ctx == nil || !core.ValidID(scope.company) || !core.ValidID(missionID) ||
		!core.ValidID(input.RequestID) || input.ReservedCalls < 0 || input.ExpectedBudgetRevision < 0 || input.ExpectedReserveRevision < 0 ||
		(input.ExpectedLimit != nil && *input.ExpectedLimit < 1) ||
		(input.ExpectedLimit == nil && input.ExpectedBudgetRevision != 0) ||
		(input.ExpectedLimit != nil && input.ExpectedBudgetRevision == 0) ||
		utf8.RuneCountInString(reason) < 1 || utf8.RuneCountInString(reason) > 500 {
		return Receipt{}, core.Malformed
	}
	command := struct {
		MissionID string
		Input     MissionToolCallClosingReserveInput
		Reason    string
	}{missionID, input, reason}
	return k.TXWrite(ctx, scope, nil, input.RequestID, "mission.tool_budget.closing_reserve", command, func(tx pgx.Tx) (Receipt, error) {
		snapshot, err := lockMissionToolCallBudgetTX(ctx, tx, scope.company, missionID)
		if err != nil {
			return Receipt{}, err
		}
		if snapshot.Limit.Valid != (input.ExpectedLimit != nil) ||
			(snapshot.Limit.Valid && snapshot.Limit.Int64 != *input.ExpectedLimit) || snapshot.Revision != input.ExpectedBudgetRevision {
			return Receipt{}, core.Conflict
		}
		var reserveRevision, previousReserved int64
		err = tx.QueryRow(ctx, `SELECT revision,reserved_tool_calls FROM mission_tool_call_closing_reserves
WHERE company_id=$1 AND mission_id=$2 ORDER BY revision DESC LIMIT 1`, scope.company, missionID).Scan(&reserveRevision, &previousReserved)
		if errors.Is(err, pgx.ErrNoRows) {
			err = nil
		}
		if err != nil {
			return Receipt{}, err
		}
		if input.ExpectedReserveRevision != reserveRevision {
			return Receipt{}, core.Conflict
		}
		if reserveRevision == math.MaxInt64 {
			return Receipt{}, core.Integrity
		}
		if snapshot.Limit.Valid && input.ReservedCalls > snapshot.Limit.Int64-snapshot.Used {
			return Receipt{}, core.ToolCallBudgetExceeded
		}
		var closingUsed int64
		if err = tx.QueryRow(ctx, `SELECT COALESCE(sum(task_tool_calls_used),0)::bigint FROM tasks
WHERE company_id=$1 AND mission_id=$2 AND kind IN ('review','peer_review')`, scope.company, missionID).Scan(&closingUsed); err != nil {
			return Receipt{}, err
		}
		newRevision := reserveRevision + 1
		_, err = tx.Exec(ctx, `INSERT INTO mission_tool_call_closing_reserves(company_id,mission_id,revision,request_id,
expected_reserve_revision,expected_mission_budget_revision,expected_mission_tool_call_limit,previous_reserved_tool_calls,
reserved_tool_calls,closing_tool_calls_used_at_revision,authorized_by,reason)
VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,'local-owner',$11)`, scope.company, missionID, newRevision, input.RequestID,
			reserveRevision, snapshot.Revision, nullableMissionLimit(snapshot.Limit), previousReserved, input.ReservedCalls, closingUsed, reason)
		if err != nil {
			return Receipt{}, err
		}
		return Receipt{ID: missionID, Status: "mission_closing_reserve_updated", Revision: newRevision}, nil
	})
}

func nullableMissionLimit(limit pgtype.Int8) any {
	if !limit.Valid {
		return nil
	}
	return limit.Int64
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

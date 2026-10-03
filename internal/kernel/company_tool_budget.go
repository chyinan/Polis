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

type CompanyToolCallBudget struct {
	CompanyID                string `json:"companyId"`
	ToolCallLimit            *int64 `json:"toolCallLimit"`
	ToolCallsUsed            int64  `json:"toolCallsUsed"`
	ToolCallsRemaining       int64  `json:"toolCallsRemaining"`
	ClosingReserveToolCalls  int64  `json:"closingReserveToolCalls"`
	ClosingReserveRemaining  int64  `json:"closingReserveRemaining"`
	ClosingReserveRevision   int64  `json:"closingReserveRevision"`
	LastClosingReserveReason string `json:"lastClosingReserveReason,omitempty"`
	LastClosingReserveAt     string `json:"lastClosingReserveAt,omitempty"`
	Revision                 int64  `json:"revision"`
	State                    string `json:"state"`
	AllocationCount          int64  `json:"allocationCount"`
	LastReason               string `json:"lastReason,omitempty"`
	LastAllocatedAt          string `json:"lastAllocatedAt,omitempty"`
	RejectionCount           int64  `json:"rejectionCount"`
	LastRejectionAt          string `json:"lastRejectionAt,omitempty"`
	LastRejectionRoute       string `json:"lastRejectionRoute,omitempty"`
	LastRejectionReason      string `json:"lastRejectionReason,omitempty"`
	LastRejectionTaskID      string `json:"lastRejectionTaskId,omitempty"`
}

type CompanyToolCallBudgetChangeInput struct {
	RequestID        string
	ExpectedLimit    *int64
	ExpectedRevision int64
	ResultingLimit   int64
	Reason           string
}

type CompanyToolCallClosingReserveInput struct {
	RequestID               string
	ReservedCalls           int64
	ExpectedLimit           *int64
	ExpectedBudgetRevision  int64
	ExpectedReserveRevision int64
	Reason                  string
}

type companyToolCallBudgetSnapshot struct {
	Limit    pgtype.Int8
	Used     int64
	Revision int64
}

func lockCompanyToolCallBudgetTX(ctx context.Context, tx pgx.Tx, companyID string) (companyToolCallBudgetSnapshot, error) {
	var snapshot companyToolCallBudgetSnapshot
	err := tx.QueryRow(ctx, `SELECT company_tool_call_limit,company_tool_calls_used,company_tool_call_budget_revision
FROM companies WHERE id=$1 FOR UPDATE`, companyID).Scan(&snapshot.Limit, &snapshot.Used, &snapshot.Revision)
	if errors.Is(err, pgx.ErrNoRows) {
		return companyToolCallBudgetSnapshot{}, core.OutOfScope
	}
	return snapshot, err
}

func companyToolCallBudgetRejectionReason(snapshot companyToolCallBudgetSnapshot, closingTask bool, reserveRemaining int64) string {
	if !snapshot.Limit.Valid {
		return "company_budget_pending"
	}
	if snapshot.Limit.Int64 <= snapshot.Used {
		return "company_limit"
	}
	if !closingTask && snapshot.Limit.Int64-snapshot.Used <= reserveRemaining {
		return "company_closing_reserve"
	}
	return ""
}

func recordCompanyToolCallBudgetRejection(ctx context.Context, tx pgx.Tx, companyID, taskID, sessionID, requestID, route, reason string, snapshot companyToolCallBudgetSnapshot, reserve, reserveRemaining, reserveRevision int64) error {
	var session any
	if sessionID != "" {
		session = sessionID
	}
	_, err := tx.Exec(ctx, `INSERT INTO company_tool_call_budget_rejections(company_id,request_id,task_id,worker_session_id,
route,reason,company_tool_call_limit,company_tool_calls_used,company_budget_revision,
company_closing_reserve_tool_calls,company_closing_reserve_remaining,company_closing_reserve_revision)
VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`, companyID, requestID, taskID, session, route, reason,
		snapshot.Limit, snapshot.Used, snapshot.Revision, reserve, reserveRemaining, reserveRevision)
	return err
}

func (k *Kernel) GetCompanyToolCallBudget(ctx context.Context, scope Scope) (CompanyToolCallBudget, error) {
	if k == nil || ctx == nil || !core.ValidID(scope.company) {
		return CompanyToolCallBudget{}, core.Malformed
	}
	tx, err := k.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return CompanyToolCallBudget{}, err
	}
	defer tx.Rollback(ctx)
	if err = k.checkRuntimeLease(ctx, tx); err != nil {
		return CompanyToolCallBudget{}, err
	}
	var item CompanyToolCallBudget
	var cap pgtype.Int8
	var reserveBaseline, closingUsed int64
	err = tx.QueryRow(ctx, `SELECT c.id,c.company_tool_call_limit,c.company_tool_calls_used,c.company_tool_call_budget_revision,
COALESCE((SELECT r.reserved_tool_calls FROM company_tool_call_closing_reserves r WHERE r.company_id=c.id ORDER BY r.revision DESC LIMIT 1),0),
COALESCE((SELECT r.revision FROM company_tool_call_closing_reserves r WHERE r.company_id=c.id ORDER BY r.revision DESC LIMIT 1),0),
COALESCE((SELECT r.closing_tool_calls_used_at_revision FROM company_tool_call_closing_reserves r WHERE r.company_id=c.id ORDER BY r.revision DESC LIMIT 1),0),
(SELECT COALESCE(sum(t.task_tool_calls_used),0)::bigint FROM tasks t WHERE t.company_id=c.id AND t.kind IN ('review','peer_review')),
COALESCE((SELECT r.reason FROM company_tool_call_closing_reserves r WHERE r.company_id=c.id ORDER BY r.revision DESC LIMIT 1),''),
COALESCE((SELECT r.created_at::text FROM company_tool_call_closing_reserves r WHERE r.company_id=c.id ORDER BY r.revision DESC LIMIT 1),''),
(SELECT count(*) FROM company_tool_call_budget_allocations a WHERE a.company_id=c.id),
COALESCE((SELECT a.reason FROM company_tool_call_budget_allocations a WHERE a.company_id=c.id ORDER BY a.revision DESC LIMIT 1),''),
COALESCE((SELECT a.created_at::text FROM company_tool_call_budget_allocations a WHERE a.company_id=c.id ORDER BY a.revision DESC LIMIT 1),''),
(SELECT count(*) FROM company_tool_call_budget_rejections d WHERE d.company_id=c.id),
COALESCE((SELECT d.occurred_at::text FROM company_tool_call_budget_rejections d WHERE d.company_id=c.id ORDER BY d.occurred_at DESC,d.request_id DESC LIMIT 1),''),
COALESCE((SELECT d.route FROM company_tool_call_budget_rejections d WHERE d.company_id=c.id ORDER BY d.occurred_at DESC,d.request_id DESC LIMIT 1),''),
COALESCE((SELECT d.reason FROM company_tool_call_budget_rejections d WHERE d.company_id=c.id ORDER BY d.occurred_at DESC,d.request_id DESC LIMIT 1),''),
COALESCE((SELECT d.task_id FROM company_tool_call_budget_rejections d WHERE d.company_id=c.id ORDER BY d.occurred_at DESC,d.request_id DESC LIMIT 1),'')
FROM companies c WHERE c.id=$1`, scope.company).Scan(&item.CompanyID, &cap, &item.ToolCallsUsed, &item.Revision,
		&item.ClosingReserveToolCalls, &item.ClosingReserveRevision, &reserveBaseline, &closingUsed,
		&item.LastClosingReserveReason, &item.LastClosingReserveAt,
		&item.AllocationCount, &item.LastReason, &item.LastAllocatedAt, &item.RejectionCount, &item.LastRejectionAt,
		&item.LastRejectionRoute, &item.LastRejectionReason, &item.LastRejectionTaskID)
	if errors.Is(err, pgx.ErrNoRows) {
		return CompanyToolCallBudget{}, core.OutOfScope
	}
	if err != nil {
		return CompanyToolCallBudget{}, err
	}
	item.State = "pending"
	if closingUsed < reserveBaseline {
		return CompanyToolCallBudget{}, core.Integrity
	}
	item.ClosingReserveRemaining = item.ClosingReserveToolCalls - (closingUsed - reserveBaseline)
	if item.ClosingReserveRemaining < 0 {
		item.ClosingReserveRemaining = 0
	}
	if cap.Valid {
		limit := cap.Int64
		item.ToolCallLimit = &limit
		if item.ToolCallsUsed >= limit {
			item.State = "exhausted"
		} else {
			item.ToolCallsRemaining = limit - item.ToolCallsUsed
			item.State = "available"
			if item.ClosingReserveRemaining > 0 && item.ToolCallsRemaining <= item.ClosingReserveRemaining {
				item.State = "closing_reserved"
			}
		}
	}
	if item.ClosingReserveRevision == 0 {
		item.LastClosingReserveReason, item.LastClosingReserveAt = "", ""
	}
	if item.AllocationCount == 0 {
		item.LastReason, item.LastAllocatedAt = "", ""
	}
	if item.RejectionCount == 0 {
		item.LastRejectionAt, item.LastRejectionRoute, item.LastRejectionReason, item.LastRejectionTaskID = "", "", "", ""
	}
	return item, tx.Commit(ctx)
}

func (k *Kernel) TXChangeCompanyToolCallBudget(ctx context.Context, scope Scope, input CompanyToolCallBudgetChangeInput) (Receipt, error) {
	reason := strings.TrimSpace(input.Reason)
	if k == nil || ctx == nil || !core.ValidID(scope.company) || !core.ValidID(input.RequestID) ||
		input.ExpectedRevision < 0 || input.ResultingLimit < 1 ||
		(input.ExpectedLimit != nil && *input.ExpectedLimit < 1) ||
		utf8.RuneCountInString(reason) < 1 || utf8.RuneCountInString(reason) > 500 {
		return Receipt{}, core.Malformed
	}
	command := struct {
		Input  CompanyToolCallBudgetChangeInput
		Reason string
	}{input, reason}
	return k.TXWrite(ctx, scope, nil, input.RequestID, "company.tool_budget.change", command, func(tx pgx.Tx) (Receipt, error) {
		snapshot, err := lockCompanyToolCallBudgetTX(ctx, tx, scope.company)
		if err != nil {
			return Receipt{}, err
		}
		if snapshot.Limit.Valid != (input.ExpectedLimit != nil) ||
			(snapshot.Limit.Valid && snapshot.Limit.Int64 != *input.ExpectedLimit) || snapshot.Revision != input.ExpectedRevision {
			return Receipt{}, core.Conflict
		}
		if snapshot.Revision == math.MaxInt64 || input.ResultingLimit < snapshot.Used {
			return Receipt{}, core.Conflict
		}
		_, reserveRemaining, _, reserveErr := companyToolCallClosingReserveSnapshot(ctx, tx, scope.company)
		if reserveErr != nil {
			return Receipt{}, reserveErr
		}
		if input.ResultingLimit-snapshot.Used < reserveRemaining {
			return Receipt{}, core.ToolCallBudgetExceeded
		}
		operation := "configure"
		var previous any
		additional := input.ResultingLimit
		if snapshot.Limit.Valid {
			if input.ResultingLimit <= snapshot.Limit.Int64 {
				return Receipt{}, core.Denied
			}
			operation = "allocate"
			previous = snapshot.Limit.Int64
			additional = input.ResultingLimit - snapshot.Limit.Int64
		}
		newRevision := snapshot.Revision + 1
		if _, err = tx.Exec(ctx, `INSERT INTO company_tool_call_budget_allocations(company_id,revision,request_id,expected_revision,
operation,previous_limit,additional_tool_calls,resulting_limit,authorized_by,reason)
VALUES($1,$2,$3,$4,$5,$6,$7,$8,'local-owner',$9)`, scope.company, newRevision, input.RequestID, snapshot.Revision,
			operation, previous, additional, input.ResultingLimit, reason); err != nil {
			return Receipt{}, err
		}
		if _, err = tx.Exec(ctx, `UPDATE companies SET company_tool_call_limit=$2,company_tool_call_budget_revision=$3
WHERE id=$1`, scope.company, input.ResultingLimit, newRevision); err != nil {
			return Receipt{}, err
		}
		status := "company_budget_configured"
		if operation == "allocate" {
			status = "company_budget_allocated"
		}
		return Receipt{ID: scope.company, Status: status, Revision: newRevision}, nil
	})
}

func companyToolCallClosingReserveSnapshot(ctx context.Context, q problemBudgetQueryRower, companyID string) (int64, int64, int64, error) {
	var reserved, revision, usedAtRevision, closingUsed int64
	err := q.QueryRow(ctx, `SELECT
 COALESCE((SELECT r.reserved_tool_calls FROM company_tool_call_closing_reserves r WHERE r.company_id=$1 ORDER BY r.revision DESC LIMIT 1),0),
 COALESCE((SELECT r.revision FROM company_tool_call_closing_reserves r WHERE r.company_id=$1 ORDER BY r.revision DESC LIMIT 1),0),
 COALESCE((SELECT r.closing_tool_calls_used_at_revision FROM company_tool_call_closing_reserves r WHERE r.company_id=$1 ORDER BY r.revision DESC LIMIT 1),0),
 (SELECT COALESCE(sum(t.task_tool_calls_used),0)::bigint FROM tasks t WHERE t.company_id=$1 AND t.kind IN ('review','peer_review'))`, companyID).
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

func (k *Kernel) TXSetCompanyToolCallClosingReserve(ctx context.Context, scope Scope, input CompanyToolCallClosingReserveInput) (Receipt, error) {
	reason := strings.TrimSpace(input.Reason)
	if k == nil || ctx == nil || !core.ValidID(scope.company) || !core.ValidID(input.RequestID) ||
		input.ReservedCalls < 0 || input.ExpectedBudgetRevision < 0 || input.ExpectedReserveRevision < 0 ||
		(input.ExpectedLimit != nil && *input.ExpectedLimit < 1) ||
		(input.ExpectedLimit == nil && input.ExpectedBudgetRevision != 0) ||
		(input.ExpectedLimit != nil && input.ExpectedBudgetRevision == 0) ||
		utf8.RuneCountInString(reason) < 1 || utf8.RuneCountInString(reason) > 500 {
		return Receipt{}, core.Malformed
	}
	command := struct {
		Input  CompanyToolCallClosingReserveInput
		Reason string
	}{input, reason}
	return k.TXWrite(ctx, scope, nil, input.RequestID, "company.tool_budget.closing_reserve", command, func(tx pgx.Tx) (Receipt, error) {
		snapshot, err := lockCompanyToolCallBudgetTX(ctx, tx, scope.company)
		if err != nil {
			return Receipt{}, err
		}
		if snapshot.Limit.Valid != (input.ExpectedLimit != nil) ||
			(snapshot.Limit.Valid && snapshot.Limit.Int64 != *input.ExpectedLimit) || snapshot.Revision != input.ExpectedBudgetRevision {
			return Receipt{}, core.Conflict
		}
		var reserveRevision, previousReserved int64
		err = tx.QueryRow(ctx, `SELECT revision,reserved_tool_calls FROM company_tool_call_closing_reserves
WHERE company_id=$1 ORDER BY revision DESC LIMIT 1`, scope.company).Scan(&reserveRevision, &previousReserved)
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
WHERE company_id=$1 AND kind IN ('review','peer_review')`, scope.company).Scan(&closingUsed); err != nil {
			return Receipt{}, err
		}
		newRevision := reserveRevision + 1
		_, err = tx.Exec(ctx, `INSERT INTO company_tool_call_closing_reserves(company_id,revision,request_id,
expected_reserve_revision,expected_company_budget_revision,expected_company_tool_call_limit,previous_reserved_tool_calls,
reserved_tool_calls,closing_tool_calls_used_at_revision,authorized_by,reason)
VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,'local-owner',$10)`, scope.company, newRevision, input.RequestID,
			reserveRevision, snapshot.Revision, nullableCompanyToolCallLimit(snapshot.Limit), previousReserved, input.ReservedCalls, closingUsed, reason)
		if err != nil {
			return Receipt{}, err
		}
		return Receipt{ID: scope.company, Status: "company_closing_reserve_updated", Revision: newRevision}, nil
	})
}

func nullableCompanyToolCallLimit(limit pgtype.Int8) any {
	if !limit.Valid {
		return nil
	}
	return limit.Int64
}

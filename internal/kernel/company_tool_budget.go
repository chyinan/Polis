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
	CompanyID           string `json:"companyId"`
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

type CompanyToolCallBudgetChangeInput struct {
	RequestID        string
	ExpectedLimit    *int64
	ExpectedRevision int64
	ResultingLimit   int64
	Reason           string
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

func companyToolCallBudgetRejectionReason(snapshot companyToolCallBudgetSnapshot) string {
	if !snapshot.Limit.Valid {
		return "company_budget_pending"
	}
	if snapshot.Limit.Int64 <= snapshot.Used {
		return "company_limit"
	}
	return ""
}

func recordCompanyToolCallBudgetRejection(ctx context.Context, tx pgx.Tx, companyID, taskID, sessionID, requestID, route, reason string, snapshot companyToolCallBudgetSnapshot) error {
	var session any
	if sessionID != "" {
		session = sessionID
	}
	_, err := tx.Exec(ctx, `INSERT INTO company_tool_call_budget_rejections(company_id,request_id,task_id,worker_session_id,
route,reason,company_tool_call_limit,company_tool_calls_used,company_budget_revision)
VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9)`, companyID, requestID, taskID, session, route, reason, snapshot.Limit, snapshot.Used, snapshot.Revision)
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
	err = tx.QueryRow(ctx, `SELECT c.id,c.company_tool_call_limit,c.company_tool_calls_used,c.company_tool_call_budget_revision,
(SELECT count(*) FROM company_tool_call_budget_allocations a WHERE a.company_id=c.id),
COALESCE((SELECT a.reason FROM company_tool_call_budget_allocations a WHERE a.company_id=c.id ORDER BY a.revision DESC LIMIT 1),''),
COALESCE((SELECT a.created_at::text FROM company_tool_call_budget_allocations a WHERE a.company_id=c.id ORDER BY a.revision DESC LIMIT 1),''),
(SELECT count(*) FROM company_tool_call_budget_rejections d WHERE d.company_id=c.id),
COALESCE((SELECT d.occurred_at::text FROM company_tool_call_budget_rejections d WHERE d.company_id=c.id ORDER BY d.occurred_at DESC,d.request_id DESC LIMIT 1),''),
COALESCE((SELECT d.route FROM company_tool_call_budget_rejections d WHERE d.company_id=c.id ORDER BY d.occurred_at DESC,d.request_id DESC LIMIT 1),''),
COALESCE((SELECT d.reason FROM company_tool_call_budget_rejections d WHERE d.company_id=c.id ORDER BY d.occurred_at DESC,d.request_id DESC LIMIT 1),''),
COALESCE((SELECT d.task_id FROM company_tool_call_budget_rejections d WHERE d.company_id=c.id ORDER BY d.occurred_at DESC,d.request_id DESC LIMIT 1),'')
FROM companies c WHERE c.id=$1`, scope.company).Scan(&item.CompanyID, &cap, &item.ToolCallsUsed, &item.Revision,
		&item.AllocationCount, &item.LastReason, &item.LastAllocatedAt, &item.RejectionCount, &item.LastRejectionAt,
		&item.LastRejectionRoute, &item.LastRejectionReason, &item.LastRejectionTaskID)
	if errors.Is(err, pgx.ErrNoRows) {
		return CompanyToolCallBudget{}, core.OutOfScope
	}
	if err != nil {
		return CompanyToolCallBudget{}, err
	}
	item.State = "pending"
	if cap.Valid {
		limit := cap.Int64
		item.ToolCallLimit = &limit
		if item.ToolCallsUsed >= limit {
			item.State = "exhausted"
		} else {
			item.State = "available"
			item.ToolCallsRemaining = limit - item.ToolCallsUsed
		}
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

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

type ProblemToolCallBudget struct {
	ProblemKey               string `json:"problemKey"`
	MissionID                string `json:"missionId"`
	TaskCount                int64  `json:"taskCount"`
	WorkerSessionAttempts    int64  `json:"workerSessionAttempts"`
	ToolCallLimit            *int64 `json:"toolCallLimit"`
	ToolCallsUsed            int64  `json:"toolCallsUsed"`
	ToolCallsRemaining       int64  `json:"toolCallsRemaining"`
	AllocationRevision       int64  `json:"allocationRevision"`
	ClosingReserveToolCalls  int64  `json:"closingReserveToolCalls"`
	ClosingReserveRemaining  int64  `json:"closingReserveRemaining"`
	ClosingReserveRevision   int64  `json:"closingReserveRevision"`
	State                    string `json:"state"`
	LastAllocationReason     string `json:"lastAllocationReason,omitempty"`
	LastAllocatedAt          string `json:"lastAllocatedAt,omitempty"`
	LastClosingReserveReason string `json:"lastClosingReserveReason,omitempty"`
	LastClosingReserveAt     string `json:"lastClosingReserveAt,omitempty"`
}

type ProblemToolCallBudgetList struct {
	Items     []ProblemToolCallBudget `json:"items"`
	Truncated bool                    `json:"truncated"`
}

type ProblemToolCallAllocationInput struct {
	RequestID        string
	AdditionalCalls  int64
	ExpectedLimit    int64
	ExpectedRevision int64
	Reason           string
}

type ProblemToolCallClosingReserveInput struct {
	RequestID               string
	ExpectedLimit           int64
	ExpectedBudgetRevision  int64
	ExpectedReserveRevision int64
	ReservedCalls           int64
	Reason                  string
}

type problemBudgetQueryRower interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}

func isProblemClosingTaskKind(kind string) bool {
	return kind == "review" || kind == "peer_review"
}

func problemToolCallClosingReserveRemaining(ctx context.Context, q problemBudgetQueryRower, companyID, problemKey string) (int64, error) {
	var reserved, usedAtRevision, closeoutUsed int64
	err := q.QueryRow(ctx, `SELECT
 COALESCE((SELECT r.reserved_tool_calls FROM problem_tool_call_closing_reserves r WHERE r.company_id=$1 AND r.problem_key=$2 ORDER BY r.revision DESC LIMIT 1),0),
 COALESCE((SELECT r.closeout_tool_calls_used_at_revision FROM problem_tool_call_closing_reserves r WHERE r.company_id=$1 AND r.problem_key=$2 ORDER BY r.revision DESC LIMIT 1),0),
 (SELECT COALESCE(sum(t.task_tool_calls_used),0)::bigint FROM tasks t WHERE t.company_id=$1 AND t.problem_key=$2 AND t.kind IN ('review','peer_review'))`, companyID, problemKey).Scan(&reserved, &usedAtRevision, &closeoutUsed)
	if err != nil {
		return 0, err
	}
	if closeoutUsed < usedAtRevision {
		return 0, core.Integrity
	}
	spent := closeoutUsed - usedAtRevision
	if spent >= reserved {
		return 0, nil
	}
	return reserved - spent, nil
}

func ValidProblemKey(key string) bool {
	return strings.HasPrefix(key, "problem:") && core.ValidID(strings.TrimPrefix(key, "problem:"))
}

func (k *Kernel) ListProblemToolCallBudgets(ctx context.Context, scope Scope, limit int) (ProblemToolCallBudgetList, error) {
	if k == nil || ctx == nil || !core.ValidID(scope.company) || limit < 1 || limit > 100 {
		return ProblemToolCallBudgetList{}, core.Malformed
	}
	tx, err := k.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return ProblemToolCallBudgetList{}, err
	}
	defer tx.Rollback(ctx)
	if err = k.checkRuntimeLease(ctx, tx); err != nil {
		return ProblemToolCallBudgetList{}, err
	}
	rows, err := tx.Query(ctx, `SELECT pb.problem_key,
 (SELECT min(t.mission_id) FROM tasks t WHERE t.company_id=pb.company_id AND t.problem_key=pb.problem_key),
 (SELECT count(*) FROM tasks t WHERE t.company_id=pb.company_id AND t.problem_key=pb.problem_key),
 (SELECT count(*) FROM worker_sessions s JOIN tasks t ON t.company_id=s.company_id AND t.id=s.task_id WHERE t.company_id=pb.company_id AND t.problem_key=pb.problem_key),
 pb.tool_call_limit,pb.tool_calls_used,pb.revision,
 COALESCE((SELECT r.reserved_tool_calls FROM problem_tool_call_closing_reserves r WHERE r.company_id=pb.company_id AND r.problem_key=pb.problem_key ORDER BY r.revision DESC LIMIT 1),0),
 COALESCE((SELECT r.revision FROM problem_tool_call_closing_reserves r WHERE r.company_id=pb.company_id AND r.problem_key=pb.problem_key ORDER BY r.revision DESC LIMIT 1),0),
 COALESCE((SELECT r.closeout_tool_calls_used_at_revision FROM problem_tool_call_closing_reserves r WHERE r.company_id=pb.company_id AND r.problem_key=pb.problem_key ORDER BY r.revision DESC LIMIT 1),0),
 (SELECT COALESCE(sum(t.task_tool_calls_used),0)::bigint FROM tasks t WHERE t.company_id=pb.company_id AND t.problem_key=pb.problem_key AND t.kind IN ('review','peer_review')),
 COALESCE((SELECT a.reason FROM problem_tool_call_allocations a WHERE a.company_id=pb.company_id AND a.problem_key=pb.problem_key ORDER BY a.revision DESC LIMIT 1),''),
 COALESCE((SELECT a.created_at::text FROM problem_tool_call_allocations a WHERE a.company_id=pb.company_id AND a.problem_key=pb.problem_key ORDER BY a.revision DESC LIMIT 1),''),
 COALESCE((SELECT r.reason FROM problem_tool_call_closing_reserves r WHERE r.company_id=pb.company_id AND r.problem_key=pb.problem_key ORDER BY r.revision DESC LIMIT 1),''),
 COALESCE((SELECT r.created_at::text FROM problem_tool_call_closing_reserves r WHERE r.company_id=pb.company_id AND r.problem_key=pb.problem_key ORDER BY r.revision DESC LIMIT 1),'')
FROM problem_tool_call_budget_effective pb
WHERE pb.company_id=$1 AND EXISTS(SELECT 1 FROM tasks t WHERE t.company_id=pb.company_id AND t.problem_key=pb.problem_key)
ORDER BY pb.problem_key LIMIT $2`, scope.company, limit+1)
	if err != nil {
		return ProblemToolCallBudgetList{}, err
	}
	defer rows.Close()
	result := ProblemToolCallBudgetList{Items: make([]ProblemToolCallBudget, 0, limit)}
	for rows.Next() {
		var item ProblemToolCallBudget
		var toolCallLimit pgtype.Int8
		var closeoutUsedAtRevision, closeoutUsed int64
		if err = rows.Scan(&item.ProblemKey, &item.MissionID, &item.TaskCount, &item.WorkerSessionAttempts,
			&toolCallLimit, &item.ToolCallsUsed, &item.AllocationRevision, &item.ClosingReserveToolCalls, &item.ClosingReserveRevision,
			&closeoutUsedAtRevision, &closeoutUsed, &item.LastAllocationReason, &item.LastAllocatedAt,
			&item.LastClosingReserveReason, &item.LastClosingReserveAt); err != nil {
			return ProblemToolCallBudgetList{}, err
		}
		if item.ClosingReserveToolCalls > 0 && closeoutUsed > closeoutUsedAtRevision {
			item.ClosingReserveRemaining = item.ClosingReserveToolCalls - (closeoutUsed - closeoutUsedAtRevision)
			if item.ClosingReserveRemaining < 0 {
				item.ClosingReserveRemaining = 0
			}
		}
		if len(result.Items) == limit {
			result.Truncated = true
			break
		}
		item.State = "pending"
		item.ToolCallsRemaining = 0
		if toolCallLimit.Valid {
			currentLimit := toolCallLimit.Int64
			item.ToolCallLimit = &currentLimit
			switch {
			case currentLimit == 0:
				item.State = "unbounded"
			case item.ToolCallsUsed >= currentLimit:
				item.State = "exhausted"
				item.ToolCallsRemaining = 0
			default:
				item.ToolCallsRemaining = currentLimit - item.ToolCallsUsed
				if item.ClosingReserveRemaining > 0 && item.ToolCallsRemaining <= item.ClosingReserveRemaining {
					item.State = "closing_reserved"
				} else {
					item.State = "available"
				}
			}
		}
		result.Items = append(result.Items, item)
	}
	if err = rows.Err(); err != nil {
		return ProblemToolCallBudgetList{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return ProblemToolCallBudgetList{}, err
	}
	return result, nil
}

func (k *Kernel) TXAllocateProblemToolCalls(ctx context.Context, scope Scope, problemKey string, input ProblemToolCallAllocationInput) (Receipt, error) {
	reason := strings.TrimSpace(input.Reason)
	if k == nil || ctx == nil || !core.ValidID(scope.company) || !ValidProblemKey(problemKey) ||
		!core.ValidID(input.RequestID) || input.AdditionalCalls < 1 || input.ExpectedLimit < 1 ||
		input.ExpectedRevision < 1 || utf8.RuneCountInString(reason) < 1 || utf8.RuneCountInString(reason) > 500 {
		return Receipt{}, core.Malformed
	}
	command := struct {
		ProblemKey string
		Input      ProblemToolCallAllocationInput
		Reason     string
	}{problemKey, input, reason}
	return k.TXWrite(ctx, scope, nil, input.RequestID, "problem.tool_budget.allocate", command, func(tx pgx.Tx) (Receipt, error) {
		var baseLimit pgtype.Int8
		var used int64
		err := tx.QueryRow(ctx, `SELECT tool_call_limit,tool_calls_used FROM problem_tool_call_budgets
WHERE company_id=$1 AND problem_key=$2 FOR UPDATE`, scope.company, problemKey).Scan(&baseLimit, &used)
		if errors.Is(err, pgx.ErrNoRows) {
			return Receipt{}, core.OutOfScope
		}
		if err != nil {
			return Receipt{}, err
		}
		if !baseLimit.Valid || baseLimit.Int64 == 0 {
			return Receipt{}, core.Denied
		}
		var addedSoFar, revision int64
		if err = tx.QueryRow(ctx, `SELECT COALESCE(sum(additional_tool_calls),0)::bigint,COALESCE(max(revision),1)
FROM problem_tool_call_allocations WHERE company_id=$1 AND problem_key=$2`, scope.company, problemKey).Scan(&addedSoFar, &revision); err != nil {
			return Receipt{}, err
		}
		if addedSoFar > math.MaxInt64-baseLimit.Int64 || revision == math.MaxInt64 {
			return Receipt{}, core.Integrity
		}
		currentLimit := baseLimit.Int64 + addedSoFar
		if input.ExpectedLimit != currentLimit || input.ExpectedRevision != revision {
			return Receipt{}, core.Conflict
		}
		if input.AdditionalCalls > math.MaxInt64-currentLimit {
			return Receipt{}, core.TooLarge
		}
		resultingLimit := currentLimit + input.AdditionalCalls
		newRevision := revision + 1
		_, err = tx.Exec(ctx, `INSERT INTO problem_tool_call_allocations(company_id,problem_key,revision,request_id,additional_tool_calls,previous_limit,resulting_limit,authorized_by,reason)
VALUES($1,$2,$3,$4,$5,$6,$7,'local-owner',$8)`, scope.company, problemKey, newRevision, input.RequestID, input.AdditionalCalls, currentLimit, resultingLimit, reason)
		if err != nil {
			return Receipt{}, err
		}
		return Receipt{ID: problemKey, Status: "allocated", Revision: newRevision}, nil
	})
}

func (k *Kernel) TXSetProblemToolCallClosingReserve(ctx context.Context, scope Scope, problemKey string, input ProblemToolCallClosingReserveInput) (Receipt, error) {
	reason := strings.TrimSpace(input.Reason)
	if k == nil || ctx == nil || !core.ValidID(scope.company) || !ValidProblemKey(problemKey) ||
		!core.ValidID(input.RequestID) || input.ExpectedLimit < 0 || input.ExpectedBudgetRevision < 1 ||
		input.ExpectedReserveRevision < 0 || input.ReservedCalls < 0 ||
		utf8.RuneCountInString(reason) < 1 || utf8.RuneCountInString(reason) > 500 {
		return Receipt{}, core.Malformed
	}
	command := struct {
		ProblemKey string
		Input      ProblemToolCallClosingReserveInput
		Reason     string
	}{problemKey, input, reason}
	return k.TXWrite(ctx, scope, nil, input.RequestID, "problem.tool_budget.closing_reserve", command, func(tx pgx.Tx) (Receipt, error) {
		var baseLimit pgtype.Int8
		var used int64
		if err := tx.QueryRow(ctx, `SELECT tool_call_limit,tool_calls_used FROM problem_tool_call_budgets
WHERE company_id=$1 AND problem_key=$2 FOR UPDATE`, scope.company, problemKey).Scan(&baseLimit, &used); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return Receipt{}, core.OutOfScope
			}
			return Receipt{}, err
		}
		if baseLimit.Valid && baseLimit.Int64 == 0 {
			return Receipt{}, core.Denied
		}
		var addedSoFar, budgetRevision int64
		if err := tx.QueryRow(ctx, `SELECT COALESCE(sum(additional_tool_calls),0)::bigint,COALESCE(max(revision),1)
FROM problem_tool_call_allocations WHERE company_id=$1 AND problem_key=$2`, scope.company, problemKey).Scan(&addedSoFar, &budgetRevision); err != nil {
			return Receipt{}, err
		}
		currentLimit := int64(0)
		if !baseLimit.Valid {
			if used != 0 || addedSoFar != 0 || input.ExpectedLimit != 0 || input.ExpectedBudgetRevision != budgetRevision {
				return Receipt{}, core.Conflict
			}
		} else {
			if addedSoFar > math.MaxInt64-baseLimit.Int64 {
				return Receipt{}, core.Integrity
			}
			currentLimit = baseLimit.Int64 + addedSoFar
			if input.ExpectedLimit != currentLimit || input.ExpectedBudgetRevision != budgetRevision {
				return Receipt{}, core.Conflict
			}
			if input.ReservedCalls > currentLimit-used {
				return Receipt{}, core.ToolCallBudgetExceeded
			}
		}
		var reserveRevision, previousReserved int64
		err := tx.QueryRow(ctx, `SELECT revision,reserved_tool_calls FROM problem_tool_call_closing_reserves
WHERE company_id=$1 AND problem_key=$2 ORDER BY revision DESC LIMIT 1`, scope.company, problemKey).Scan(&reserveRevision, &previousReserved)
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
		var closeoutUsed int64
		if err := tx.QueryRow(ctx, `SELECT COALESCE(sum(task_tool_calls_used),0)::bigint FROM tasks
WHERE company_id=$1 AND problem_key=$2 AND kind IN ('review','peer_review')`, scope.company, problemKey).Scan(&closeoutUsed); err != nil {
			return Receipt{}, err
		}
		newRevision := reserveRevision + 1
		_, err = tx.Exec(ctx, `INSERT INTO problem_tool_call_closing_reserves(company_id,problem_key,revision,request_id,
 expected_reserve_revision,expected_budget_revision,expected_tool_call_limit,previous_reserved_tool_calls,
 reserved_tool_calls,closeout_tool_calls_used_at_revision,authorized_by,reason)
VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,'local-owner',$11)`, scope.company, problemKey, newRevision,
			input.RequestID, reserveRevision, budgetRevision, currentLimit, previousReserved, input.ReservedCalls, closeoutUsed, reason)
		if err != nil {
			return Receipt{}, err
		}
		return Receipt{ID: problemKey, Status: "reserve_updated", Revision: newRevision}, nil
	})
}

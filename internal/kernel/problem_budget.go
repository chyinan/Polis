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
	ProblemKey            string `json:"problemKey"`
	MissionID             string `json:"missionId"`
	TaskCount             int64  `json:"taskCount"`
	WorkerSessionAttempts int64  `json:"workerSessionAttempts"`
	ToolCallLimit         *int64 `json:"toolCallLimit"`
	ToolCallsUsed         int64  `json:"toolCallsUsed"`
	ToolCallsRemaining    int64  `json:"toolCallsRemaining"`
	AllocationRevision    int64  `json:"allocationRevision"`
	State                 string `json:"state"`
	LastAllocationReason  string `json:"lastAllocationReason,omitempty"`
	LastAllocatedAt       string `json:"lastAllocatedAt,omitempty"`
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
 COALESCE((SELECT a.reason FROM problem_tool_call_allocations a WHERE a.company_id=pb.company_id AND a.problem_key=pb.problem_key ORDER BY a.revision DESC LIMIT 1),''),
 COALESCE((SELECT a.created_at::text FROM problem_tool_call_allocations a WHERE a.company_id=pb.company_id AND a.problem_key=pb.problem_key ORDER BY a.revision DESC LIMIT 1),'')
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
		if err = rows.Scan(&item.ProblemKey, &item.MissionID, &item.TaskCount, &item.WorkerSessionAttempts,
			&toolCallLimit, &item.ToolCallsUsed, &item.AllocationRevision, &item.LastAllocationReason, &item.LastAllocatedAt); err != nil {
			return ProblemToolCallBudgetList{}, err
		}
		if len(result.Items) == limit {
			result.Truncated = true
			break
		}
		item.ToolCallsRemaining = -1
		item.State = "pending"
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
				item.State = "available"
				item.ToolCallsRemaining = currentLimit - item.ToolCallsUsed
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

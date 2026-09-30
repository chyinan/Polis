// pattern: Imperative Shell
package kernel

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"polis/internal/core"
)

type OperatorInstructionInput struct {
	MissionID  string
	TaskID     string
	EmployeeID string
	Content    string
}

type OperatorInstruction struct {
	ID         string  `json:"instructionId"`
	MissionID  *string `json:"missionId"`
	TaskID     string  `json:"taskId"`
	EmployeeID string  `json:"employeeId"`
	Content    string  `json:"content"`
	State      string  `json:"state"`
	CreatedAt  string  `json:"createdAt"`
}

type OperatorInstructionResponseSummary struct {
	EmployeeID  string `json:"employeeId"`
	Outcome     string `json:"outcome"`
	Summary     string `json:"summary"`
	RespondedAt string `json:"respondedAt"`
}

type OperatorInstructionResponseInput struct {
	InstructionID string
	Outcome       OperatorInstructionOutcome
	Summary       string
}

type OperatorGuidanceInbox struct {
	Items     []OperatorInstruction `json:"items"`
	Truncated bool                  `json:"truncated"`
}

const maxOperatorGuidanceItems = 16

func (k *Kernel) TXCreateOperatorInstruction(ctx context.Context, s Scope, input OperatorInstructionInput, key string) (OperatorInstruction, error) {
	if (input.MissionID != "" && !core.ValidID(input.MissionID)) || (input.TaskID != "" && (!core.ValidID(input.TaskID) || input.MissionID == "")) || (input.EmployeeID != "" && !core.ValidID(input.EmployeeID)) || strings.TrimSpace(input.Content) == "" || len(input.Content) > core.MaxContent {
		return OperatorInstruction{}, core.Malformed
	}
	receipt, err := k.TXWrite(ctx, s, nil, key, "operator.instruction.created", input, func(tx pgx.Tx) (Receipt, error) {
		if input.MissionID != "" {
			var missionState string
			if err := tx.QueryRow(ctx, "SELECT state FROM missions WHERE company_id=$1 AND id=$2", s.company, input.MissionID).Scan(&missionState); errors.Is(err, pgx.ErrNoRows) {
				return Receipt{}, core.OutOfScope
			} else if err != nil {
				return Receipt{}, err
			} else if missionState != "active" {
				return Receipt{}, core.ConflictError{Reason: "operator instructions require an active mission", CurrentState: missionState}
			}
		} else {
			var companyState string
			if err := tx.QueryRow(ctx, "SELECT state FROM companies WHERE id=$1", s.company).Scan(&companyState); errors.Is(err, pgx.ErrNoRows) {
				return Receipt{}, core.OutOfScope
			} else if err != nil {
				return Receipt{}, err
			} else if companyState != "active" {
				return Receipt{}, core.ConflictError{Reason: "company-wide guidance requires an active company", CurrentState: companyState}
			}
		}
		if input.TaskID != "" {
			var owner string
			if err := tx.QueryRow(ctx, `SELECT t.owner FROM tasks t JOIN employees e ON e.company_id=t.company_id AND e.id=t.owner AND e.enabled
WHERE t.company_id=$1 AND t.id=$2 AND t.mission_id=$3`, s.company, input.TaskID, input.MissionID).Scan(&owner); errors.Is(err, pgx.ErrNoRows) {
				return Receipt{}, core.OutOfScope
			} else if err != nil {
				return Receipt{}, err
			}
			if input.EmployeeID != "" && owner != input.EmployeeID {
				return Receipt{}, core.OutOfScope
			}
		}
		if input.MissionID != "" && input.EmployeeID != "" && input.TaskID == "" {
			var ownsMissionTask bool
			if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM tasks WHERE company_id=$1 AND mission_id=$2 AND owner=$3)`, s.company, input.MissionID, input.EmployeeID).Scan(&ownsMissionTask); err != nil {
				return Receipt{}, err
			}
			if !ownsMissionTask {
				return Receipt{}, core.OutOfScope
			}
		}
		if input.EmployeeID != "" {
			var exists bool
			if err := tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM employees WHERE company_id=$1 AND id=$2 AND enabled)", s.company, input.EmployeeID).Scan(&exists); err != nil {
				return Receipt{}, err
			}
			if !exists {
				return Receipt{}, core.OutOfScope
			}
		}
		id := newID()
		if _, err := tx.Exec(ctx, `INSERT INTO operator_instructions(company_id,id,mission_id,task_id,employee_id,content,state)
VALUES($1,$2,NULLIF($3,''),NULLIF($4,''),NULLIF($5,''),$6,'pending')`, s.company, id, input.MissionID, input.TaskID, input.EmployeeID, strings.TrimSpace(input.Content)); err != nil {
			return Receipt{}, err
		}
		var recipientTag pgconn.CommandTag
		var recipientErr error
		switch {
		case input.EmployeeID != "":
			recipientTag, recipientErr = tx.Exec(ctx, `INSERT INTO operator_instruction_recipients(company_id,instruction_id,employee_id)
SELECT $1,$2,id FROM employees WHERE company_id=$1 AND id=$3 AND enabled`, s.company, id, input.EmployeeID)
		case input.TaskID != "":
			recipientTag, recipientErr = tx.Exec(ctx, `INSERT INTO operator_instruction_recipients(company_id,instruction_id,employee_id)
SELECT $1,$2,t.owner FROM tasks t JOIN employees e ON e.company_id=t.company_id AND e.id=t.owner AND e.enabled
WHERE t.company_id=$1 AND t.id=$3`, s.company, id, input.TaskID)
		case input.MissionID != "":
			recipientTag, recipientErr = tx.Exec(ctx, `INSERT INTO operator_instruction_recipients(company_id,instruction_id,employee_id)
SELECT DISTINCT $1,$2,t.owner FROM tasks t JOIN employees e ON e.company_id=t.company_id AND e.id=t.owner AND e.enabled
WHERE t.company_id=$1 AND t.mission_id=$3`, s.company, id, input.MissionID)
		default:
			recipientTag, recipientErr = tx.Exec(ctx, `INSERT INTO operator_instruction_recipients(company_id,instruction_id,employee_id)
SELECT $1,$2,id FROM employees WHERE company_id=$1 AND enabled`, s.company, id)
		}
		if recipientErr != nil {
			return Receipt{}, recipientErr
		}
		if recipientTag.RowsAffected() == 0 {
			return Receipt{}, core.OutOfScope
		}
		return Receipt{ID: id, Status: "pending"}, nil
	})
	if err != nil {
		return OperatorInstruction{}, err
	}
	var createdAt time.Time
	if err = k.pool.QueryRow(ctx, `SELECT created_at FROM operator_instructions WHERE company_id=$1 AND id=$2`, s.company, receipt.ID).Scan(&createdAt); err != nil {
		return OperatorInstruction{}, err
	}
	var missionID *string
	if input.MissionID != "" {
		missionValue := input.MissionID
		missionID = &missionValue
	}
	return OperatorInstruction{ID: receipt.ID, MissionID: missionID, TaskID: input.TaskID, EmployeeID: input.EmployeeID, Content: strings.TrimSpace(input.Content), State: receipt.Status, CreatedAt: createdAt.UTC().Format(time.RFC3339Nano)}, nil
}

func (k *Kernel) ReadPendingOperatorInstructions(ctx context.Context, binding Binding) (OperatorGuidanceInbox, error) {
	out := OperatorGuidanceInbox{Items: []OperatorInstruction{}}
	tx, err := k.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return out, err
	}
	defer tx.Rollback(ctx)
	if _, err = k.checkSession(ctx, tx, binding, false); err != nil {
		return out, err
	}
	var missionID, taskID string
	if err = tx.QueryRow(ctx, `SELECT t.mission_id,t.id
FROM worker_sessions s JOIN tasks t ON t.company_id=s.company_id AND t.id=s.task_id
WHERE s.company_id=$1 AND s.id=$2`, binding.scope.company, binding.session).Scan(&missionID, &taskID); err != nil {
		return out, err
	}
	rows, err := tx.Query(ctx, `SELECT i.id,i.mission_id,COALESCE(i.task_id,''),COALESCE(i.employee_id,''),i.content,i.state,i.created_at::text
FROM operator_instructions i
JOIN operator_instruction_recipients recipient ON recipient.company_id=i.company_id AND recipient.instruction_id=i.id AND recipient.employee_id=$4
WHERE i.company_id=$1 AND (i.mission_id IS NULL OR i.mission_id=$2) AND i.state='pending'
AND (i.task_id IS NULL OR i.task_id=$3)
AND NOT EXISTS (SELECT 1 FROM operator_instruction_responses response WHERE response.company_id=i.company_id AND response.instruction_id=i.id AND response.employee_id=$4)
ORDER BY i.created_at DESC,i.id DESC LIMIT $5`, binding.scope.company, missionID, taskID, binding.employee, maxOperatorGuidanceItems+1)
	if err != nil {
		return out, err
	}
	for rows.Next() {
		var item OperatorInstruction
		if err = rows.Scan(&item.ID, &item.MissionID, &item.TaskID, &item.EmployeeID, &item.Content, &item.State, &item.CreatedAt); err != nil {
			rows.Close()
			return out, err
		}
		out.Items = append(out.Items, item)
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return out, err
	}
	if out.Truncated = len(out.Items) > maxOperatorGuidanceItems; out.Truncated {
		out.Items = out.Items[:maxOperatorGuidanceItems]
	}
	if err = tx.Commit(ctx); err != nil {
		return OperatorGuidanceInbox{}, err
	}
	return out, nil
}

func (k *Kernel) TXRespondToOperatorInstruction(ctx context.Context, binding Binding, input OperatorInstructionResponseInput, requestID string) (Receipt, error) {
	input, err := normalizeOperatorInstructionResponse(input)
	if err != nil || !core.ValidID(requestID) {
		return Receipt{}, core.Malformed
	}
	return k.TXWrite(ctx, binding.scope, &binding, requestID, "operator.instruction.responded", input, func(tx pgx.Tx) (Receipt, error) {
		var missionID *string
		var taskID, employeeID *string
		var state string
		err := tx.QueryRow(ctx, `SELECT mission_id,task_id,employee_id,state FROM operator_instructions
WHERE company_id=$1 AND id=$2 FOR UPDATE`, binding.scope.company, input.InstructionID).Scan(&missionID, &taskID, &employeeID, &state)
		if errors.Is(err, pgx.ErrNoRows) {
			return Receipt{}, core.OutOfScope
		}
		if err != nil {
			return Receipt{}, err
		}
		var activeMissionID, activeTaskID string
		if err = tx.QueryRow(ctx, `SELECT t.mission_id,t.id
FROM worker_sessions s JOIN tasks t ON t.company_id=s.company_id AND t.id=s.task_id
WHERE s.company_id=$1 AND s.id=$2`, binding.scope.company, binding.session).Scan(&activeMissionID, &activeTaskID); err != nil {
			return Receipt{}, err
		}
		if missionID != nil && *missionID != activeMissionID || taskID != nil && *taskID != activeTaskID || employeeID != nil && *employeeID != binding.employee {
			return Receipt{}, core.OutOfScope
		}
		var isRecipient bool
		if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM operator_instruction_recipients WHERE company_id=$1 AND instruction_id=$2 AND employee_id=$3)`, binding.scope.company, input.InstructionID, binding.employee).Scan(&isRecipient); err != nil {
			return Receipt{}, err
		}
		if !isRecipient {
			return Receipt{}, core.OutOfScope
		}
		if state != "pending" {
			return Receipt{}, core.ConflictError{Reason: "operator instruction already has a response", CurrentState: state}
		}
		responseID := newID()
		if _, err = tx.Exec(ctx, `INSERT INTO operator_instruction_responses(company_id,response_id,instruction_id,employee_id,session_id,task_id,outcome,summary,request_id)
VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9)`, binding.scope.company, responseID, input.InstructionID, binding.employee, binding.session, activeTaskID, string(input.Outcome), input.Summary, requestID); err != nil {
			if isUniqueViolation(err) {
				return Receipt{}, core.Conflict
			}
			return Receipt{}, err
		}
		var recipientCount, responseCount int64
		if err = tx.QueryRow(ctx, `SELECT count(*) FROM operator_instruction_recipients WHERE company_id=$1 AND instruction_id=$2`, binding.scope.company, input.InstructionID).Scan(&recipientCount); err != nil {
			return Receipt{}, err
		}
		if err = tx.QueryRow(ctx, `SELECT count(*) FROM operator_instruction_responses WHERE company_id=$1 AND instruction_id=$2`, binding.scope.company, input.InstructionID).Scan(&responseCount); err != nil {
			return Receipt{}, err
		}
		if recipientCount == 0 || responseCount > recipientCount {
			return Receipt{}, core.Integrity
		}
		if responseCount == recipientCount {
			var hasNeedsClarification, hasRejection bool
			if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM operator_instruction_responses WHERE company_id=$1 AND instruction_id=$2 AND outcome='needs_clarification'),
EXISTS(SELECT 1 FROM operator_instruction_responses WHERE company_id=$1 AND instruction_id=$2 AND outcome='rejected')`, binding.scope.company, input.InstructionID).Scan(&hasNeedsClarification, &hasRejection); err != nil {
				return Receipt{}, err
			}
			completionState := string(OperatorInstructionApplied)
			if hasNeedsClarification {
				completionState = string(OperatorInstructionNeedsClarification)
			} else if hasRejection {
				completionState = string(OperatorInstructionRejected)
			}
			commandTag, updateErr := tx.Exec(ctx, `UPDATE operator_instructions SET state=$3,applied_at=CASE WHEN $3='applied' THEN clock_timestamp() ELSE NULL END
WHERE company_id=$1 AND id=$2 AND state='pending'`, binding.scope.company, input.InstructionID, completionState)
			if updateErr != nil {
				return Receipt{}, updateErr
			}
			if commandTag.RowsAffected() != 1 {
				return Receipt{}, core.Conflict
			}
		}
		return Receipt{ID: responseID, Status: string(input.Outcome)}, nil
	})
}

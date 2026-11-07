// pattern: Imperative Shell
package kernel

import (
	"context"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
	"polis/internal/core"
)

type OperatorInstructionInput struct {
	MissionID  string
	TaskID     string
	EmployeeID string
	Content    string
}

type OperatorInstruction struct {
	ID         string
	MissionID  string
	TaskID     string
	EmployeeID string
	Content    string
	State      string
	CreatedAt  string
}

func (k *Kernel) TXCreateOperatorInstruction(ctx context.Context, s Scope, input OperatorInstructionInput, key string) (OperatorInstruction, error) {
	if !core.ValidID(input.MissionID) || (input.TaskID != "" && !core.ValidID(input.TaskID)) || (input.EmployeeID != "" && !core.ValidID(input.EmployeeID)) || strings.TrimSpace(input.Content) == "" || len(input.Content) > core.MaxContent {
		return OperatorInstruction{}, core.Malformed
	}
	receipt, err := k.TXWrite(ctx, s, nil, key, "operator.instruction.created", input, func(tx pgx.Tx) (Receipt, error) {
		var missionState string
		if err := tx.QueryRow(ctx, "SELECT state FROM missions WHERE company_id=$1 AND id=$2", s.company, input.MissionID).Scan(&missionState); errors.Is(err, pgx.ErrNoRows) {
			return Receipt{}, core.OutOfScope
		} else if err != nil {
			return Receipt{}, err
		} else if missionState != "active" {
			return Receipt{}, core.ConflictError{Reason: "operator instructions require an active mission", CurrentState: missionState}
		}
		if input.TaskID != "" {
			var exists bool
			if err := tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM tasks WHERE company_id=$1 AND id=$2 AND mission_id=$3)", s.company, input.TaskID, input.MissionID).Scan(&exists); err != nil {
				return Receipt{}, err
			}
			if !exists {
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
VALUES($1,$2,$3,NULLIF($4,''),NULLIF($5,''),$6,'pending')`, s.company, id, input.MissionID, input.TaskID, input.EmployeeID, strings.TrimSpace(input.Content)); err != nil {
			return Receipt{}, err
		}
		return Receipt{ID: id, Status: "pending"}, nil
	})
	if err != nil {
		return OperatorInstruction{}, err
	}
	return OperatorInstruction{ID: receipt.ID, MissionID: input.MissionID, TaskID: input.TaskID, EmployeeID: input.EmployeeID, Content: strings.TrimSpace(input.Content), State: receipt.Status}, nil
}

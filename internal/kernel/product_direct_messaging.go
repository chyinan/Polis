// pattern: Imperative Shell
package kernel

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
	"polis/internal/core"
)

const ProductDirectMessageTargetLimit = 16

type ProductDirectMessageTarget struct {
	EmployeeID string `json:"employee_id"`
	TaskID     string `json:"task_id"`
	TaskKind   string `json:"task_kind"`
	State      string `json:"state"`
}

type ProductDirectMessageInput struct {
	ToEmployeeID string `json:"to_employee_id"`
	ToTaskID     string `json:"to_task_id"`
	Body         string `json:"body"`
	Actionable   bool   `json:"actionable"`
}

type ProductDirectMessage struct {
	ID           string `json:"id"`
	TaskID       string `json:"task_id"`
	SenderID     string `json:"sender_id"`
	RecipientID  string `json:"recipient_id"`
	Kind         string `json:"kind"`
	Body         string `json:"body"`
	ObligationID string `json:"obligation_id,omitempty"`
	State        string `json:"state"`
}

type ProductDirectInbox struct {
	Message *ProductDirectMessage `json:"message,omitempty"`
}

type ProductDirectApplyRequest struct {
	ObligationID      string   `json:"obligation_id"`
	WorkspaceRevision int64    `json:"workspace_revision"`
	EvidenceRefs      []string `json:"evidence_refs"`
}

// ProductDirectMessageTargets exposes only bounded, same-Mission Tasks owned
// by another member of the fixed Polis roster. It is called only when the
// trusted product adapter selects the separately qualified message surface.
func (k *Kernel) ProductDirectMessageTargets(ctx context.Context, b Binding) ([]ProductDirectMessageTarget, bool, error) {
	tx, err := k.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return nil, false, err
	}
	defer tx.Rollback(ctx)
	if _, err = k.checkSession(ctx, tx, b, false); err != nil {
		return nil, false, err
	}
	var missionID string
	if err = tx.QueryRow(ctx, `SELECT t.mission_id FROM tasks t
JOIN worker_sessions ws ON ws.company_id=t.company_id AND ws.task_id=t.id
WHERE t.company_id=$1 AND t.id=$2 AND ws.id=$3 AND t.owner=$4`, b.scope.company, b.task, b.session, b.employee).Scan(&missionID); err != nil {
		return nil, false, err
	}
	rows, err := tx.Query(ctx, `SELECT id,owner,kind,state FROM tasks
WHERE company_id=$1 AND mission_id=$2 AND id<>$3 AND owner<>$4
  AND owner IN ($5,$6,$7,$8) AND state IN ('ready','working')
ORDER BY owner,id LIMIT $9`, b.scope.company, missionID, b.task, b.employee,
		core.EmployeePlanningID, core.EmployeeBackendID, core.EmployeeFrontendID, core.EmployeeReviewID,
		ProductDirectMessageTargetLimit+1)
	if err != nil {
		return nil, false, err
	}
	targets := make([]ProductDirectMessageTarget, 0, ProductDirectMessageTargetLimit)
	truncated := false
	for rows.Next() {
		var target ProductDirectMessageTarget
		if err = rows.Scan(&target.TaskID, &target.EmployeeID, &target.TaskKind, &target.State); err != nil {
			rows.Close()
			return nil, false, err
		}
		if len(targets) == ProductDirectMessageTargetLimit {
			truncated = true
			break
		}
		targets = append(targets, target)
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return nil, false, err
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, false, err
	}
	return targets, truncated, nil
}

// TXProductDirectMessage persists an FYI or an actionable request from the
// exact current WorkerSession Task to one explicit same-Mission recipient.
func (k *Kernel) TXProductDirectMessage(ctx context.Context, b Binding, input ProductDirectMessageInput, key string) (ProductDirectMessage, error) {
	var out ProductDirectMessage
	if input.ToEmployeeID == "" || input.ToTaskID == "" || input.Body == "" || len(input.Body) > core.MaxContent {
		return out, core.Malformed
	}
	if !fixedDirectMessageEmployee(input.ToEmployeeID) || input.ToEmployeeID == b.employee || input.ToTaskID == b.task {
		return out, core.Denied
	}
	receipt, err := k.TXWrite(ctx, b.scope, &b, key, "product.collab.send", input, func(tx pgx.Tx) (Receipt, error) {
		// Lock the Mission before its Tasks. This keeps a concurrent pause from
		// committing between the active-state check and the message write.
		var sourceMissionID, missionState string
		if err := tx.QueryRow(ctx, `SELECT mission_id FROM tasks WHERE company_id=$1 AND id=$2`, b.scope.company, b.task).Scan(&sourceMissionID); err != nil {
			return Receipt{}, err
		}
		if err := tx.QueryRow(ctx, `SELECT state FROM missions WHERE company_id=$1 AND id=$2 FOR SHARE`, b.scope.company, sourceMissionID).Scan(&missionState); err != nil {
			return Receipt{}, err
		}
		if missionState != "active" {
			return Receipt{}, core.Denied
		}
		// Lock both Task rows in stable ID order so a concurrent submission
		// cannot make a ready/working target terminal between validation and send.
		type taskState struct{ owner, mission, state string }
		lockedTasks := make(map[string]taskState, 2)
		rows, err := tx.Query(ctx, `SELECT id,owner,mission_id,state FROM tasks
WHERE company_id=$1 AND id=ANY($2::text[]) ORDER BY id FOR UPDATE`, b.scope.company, []string{b.task, input.ToTaskID})
		if err != nil {
			return Receipt{}, err
		}
		for rows.Next() {
			var id string
			var task taskState
			if err = rows.Scan(&id, &task.owner, &task.mission, &task.state); err != nil {
				rows.Close()
				return Receipt{}, err
			}
			lockedTasks[id] = task
		}
		rows.Close()
		if err = rows.Err(); err != nil {
			return Receipt{}, err
		}
		source, sourceFound := lockedTasks[b.task]
		target, targetFound := lockedTasks[input.ToTaskID]
		if !sourceFound || !targetFound {
			return Receipt{}, core.Denied
		}
		sourceOwner, sourceMission, sourceState := source.owner, source.mission, source.state
		if sourceMission != sourceMissionID {
			return Receipt{}, core.Integrity
		}
		if sourceOwner != b.employee || sourceState != "working" {
			return Receipt{}, core.Denied
		}
		targetOwner, targetMission, targetState := target.owner, target.mission, target.state
		if targetOwner != input.ToEmployeeID || !fixedDirectMessageEmployee(targetOwner) || targetMission != sourceMission ||
			(targetState != "ready" && targetState != "working") {
			return Receipt{}, core.Denied
		}
		messageID := newID()
		kind := "fyi"
		obligationID := ""
		if input.Actionable {
			kind = "request"
			obligationID = messageID
		}
		if _, err = tx.Exec(ctx, `INSERT INTO messages(company_id,id,mission_id,task_id,sender,recipient,kind,body)
VALUES($1,$2,$3,$4,$5,$6,$7,$8)`, b.scope.company, messageID, sourceMission, input.ToTaskID, b.employee, input.ToEmployeeID, kind, input.Body); err != nil {
			return Receipt{}, err
		}
		if input.Actionable {
			if _, err = tx.Exec(ctx, `INSERT INTO obligations(company_id,id,task_id,owner,state) VALUES($1,$2,$3,$4,'pending')`, b.scope.company, obligationID, input.ToTaskID, input.ToEmployeeID); err != nil {
				return Receipt{}, err
			}
			if _, err = tx.Exec(ctx, `INSERT INTO peer_work_signals(company_id,id,message_id,obligation_id,recipient,state) VALUES($1,$2,$3,$4,$5,'pending')`, b.scope.company, newID(), messageID, obligationID, input.ToEmployeeID); err != nil {
				return Receipt{}, err
			}
			if err = signalEmployeeScheduleTX(ctx, tx, b.scope, input.ToEmployeeID); err != nil {
				return Receipt{}, err
			}
		}
		out = ProductDirectMessage{ID: messageID, TaskID: input.ToTaskID, SenderID: b.employee, RecipientID: input.ToEmployeeID,
			Kind: kind, Body: input.Body, ObligationID: obligationID, State: "persisted"}
		return Receipt{ID: messageID, Status: "persisted"}, nil
	})
	if err != nil {
		return out, err
	}
	if out.ID == "" {
		if err = k.pool.QueryRow(ctx, `SELECT m.id,m.task_id,m.sender,m.recipient,m.kind,m.body,m.delivery_state,COALESCE(o.id,'')
FROM messages m LEFT JOIN obligations o ON o.company_id=m.company_id AND o.id=m.id
WHERE m.company_id=$1 AND m.id=$2 AND m.sender=$3`, b.scope.company, receipt.ID, b.employee).Scan(
			&out.ID, &out.TaskID, &out.SenderID, &out.RecipientID, &out.Kind, &out.Body, &out.State, &out.ObligationID); err != nil {
			return ProductDirectMessage{}, err
		}
	}
	return out, nil
}

// ProductDirectInbox returns at most one actionable request (oldest first),
// then FYIs in event order. A read persists delivery and observation before it
// returns the body to the current recipient Task.
func (k *Kernel) ProductDirectInbox(ctx context.Context, b Binding) (ProductDirectInbox, error) {
	var out ProductDirectInbox
	if _, err := k.Handover(ctx, b); err != nil {
		return out, err
	}
	for transition := 0; transition < 3; transition++ {
		message, err := k.nextProductDirectMessage(ctx, b)
		if errors.Is(err, pgx.ErrNoRows) {
			return out, nil
		}
		if err != nil {
			return out, err
		}
		switch message.State {
		case "persisted":
			err = k.productDirectMessageState(ctx, b, message.ID, "delivered", "product-direct-deliver-"+message.ID)
		case "delivered":
			err = k.productDirectMessageState(ctx, b, message.ID, "observed", "product-direct-observe-"+message.ID)
		default:
			out.Message = &message
			return out, nil
		}
		if err != nil {
			return out, err
		}
	}
	message, err := k.nextProductDirectMessage(ctx, b)
	if errors.Is(err, pgx.ErrNoRows) {
		return out, nil
	}
	if err != nil {
		return out, err
	}
	out.Message = &message
	return out, nil
}

func (k *Kernel) nextProductDirectMessage(ctx context.Context, b Binding) (ProductDirectMessage, error) {
	var out ProductDirectMessage
	var obligationID string
	err := k.pool.QueryRow(ctx, `SELECT m.id,m.task_id,m.sender,m.recipient,m.kind,m.body,m.delivery_state,COALESCE(o.id,'')
FROM messages m
LEFT JOIN obligations o ON o.company_id=m.company_id AND o.id=m.id
LEFT JOIN events e ON e.company_id=m.company_id AND e.kind='product.collab.send' AND e.payload->>'id'=m.id
JOIN tasks t ON t.company_id=m.company_id AND t.id=m.task_id
WHERE m.company_id=$1 AND m.task_id=$2 AND m.recipient=$3 AND t.owner=$3 AND m.mission_id=t.mission_id
  AND m.contract_revision_id IS NULL
  AND ((o.id IS NOT NULL AND o.state IN ('pending','observed','acknowledged','applied'))
    OR (o.id IS NULL AND m.kind='fyi' AND m.delivery_state IN ('persisted','delivered','observed')))
ORDER BY CASE WHEN o.id IS NOT NULL THEN 0 ELSE 1 END,COALESCE(e.company_seq,0),m.id
LIMIT 1`, b.scope.company, b.task, b.employee).Scan(&out.ID, &out.TaskID, &out.SenderID, &out.RecipientID, &out.Kind, &out.Body, &out.State, &obligationID)
	out.ObligationID = obligationID
	return out, err
}

func (k *Kernel) TXProductDirectMessageAck(ctx context.Context, b Binding, messageID, key string) error {
	if messageID == "" {
		return core.Malformed
	}
	return k.productDirectMessageState(ctx, b, messageID, "acknowledged", key)
}

func (k *Kernel) productDirectMessageState(ctx context.Context, b Binding, messageID, nextState, key string) error {
	if messageID == "" {
		return core.Malformed
	}
	_, err := k.TXWrite(ctx, b.scope, &b, key, "product.collab."+nextState, messageID, func(tx pgx.Tx) (Receipt, error) {
		var taskID, owner, recipient, kind, state, obligationState string
		err := tx.QueryRow(ctx, `SELECT m.task_id,m.sender,m.recipient,m.kind,m.delivery_state,COALESCE(o.state,'')
FROM messages m JOIN tasks t ON t.company_id=m.company_id AND t.id=m.task_id
LEFT JOIN obligations o ON o.company_id=m.company_id AND o.id=m.id
WHERE m.company_id=$1 AND m.id=$2 AND m.contract_revision_id IS NULL AND t.mission_id=m.mission_id
FOR UPDATE OF m`, b.scope.company, messageID).Scan(&taskID, &owner, &recipient, &kind, &state, &obligationState)
		if err != nil {
			return Receipt{}, err
		}
		if taskID != b.task || owner == b.employee || recipient != b.employee || !fixedDirectMessageEmployee(owner) ||
			(kind != "request" && kind != "fyi") {
			return Receipt{}, core.Denied
		}
		if (kind == "request" && obligationState == "") || (kind == "fyi" && obligationState != "") {
			return Receipt{}, core.Integrity
		}
		allowed := (nextState == "delivered" && state == "persisted") ||
			(nextState == "observed" && state == "delivered") ||
			(nextState == "acknowledged" && state == "observed")
		if !allowed {
			return Receipt{}, core.Denied
		}
		if nextState == "observed" {
			var revision int64
			if err = tx.QueryRow(ctx, `SELECT w.revision FROM worker_workspaces w WHERE w.company_id=$1 AND w.task_id=$2`, b.scope.company, taskID).Scan(&revision); err != nil {
				return Receipt{}, err
			}
			if _, err = tx.Exec(ctx, `UPDATE messages SET task_revision=$3 WHERE company_id=$1 AND id=$2`, b.scope.company, messageID, revision); err != nil {
				return Receipt{}, err
			}
		}
		if _, err = tx.Exec(ctx, `UPDATE messages SET delivery_state=$3 WHERE company_id=$1 AND id=$2`, b.scope.company, messageID, nextState); err != nil {
			return Receipt{}, err
		}
		if kind == "request" {
			obligationNext, signalNext := "pending", "pending"
			switch nextState {
			case "observed":
				obligationNext, signalNext = "observed", "observed"
			case "acknowledged":
				obligationNext, signalNext = "observed", "acknowledged"
			}
			if _, err = tx.Exec(ctx, `UPDATE obligations SET state=$3 WHERE company_id=$1 AND id=$2 AND state IN ('pending','observed')`, b.scope.company, messageID, obligationNext); err != nil {
				return Receipt{}, err
			}
			if _, err = tx.Exec(ctx, `UPDATE peer_work_signals SET state=$3 WHERE company_id=$1 AND message_id=$2`, b.scope.company, messageID, signalNext); err != nil {
				return Receipt{}, err
			}
		}
		return Receipt{ID: messageID, Status: nextState}, nil
	})
	return err
}

func (k *Kernel) TXProductDirectApply(ctx context.Context, b Binding, request ProductDirectApplyRequest, key string) (Receipt, error) {
	if request.ObligationID == "" || request.WorkspaceRevision < 1 || len(request.EvidenceRefs) == 0 || len(request.EvidenceRefs) > 8 {
		return Receipt{}, core.Malformed
	}
	evidenceRaw, err := json.Marshal(request.EvidenceRefs)
	if err != nil {
		return Receipt{}, err
	}
	return k.TXWrite(ctx, b.scope, &b, key, "product.collab.apply", request, func(tx pgx.Tx) (Receipt, error) {
		var taskID, owner, recipient, messageState, kind, obligationState string
		err := tx.QueryRow(ctx, `SELECT o.task_id,o.owner,m.recipient,m.delivery_state,m.kind,o.state
FROM obligations o JOIN messages m ON m.company_id=o.company_id AND m.id=o.id
JOIN tasks t ON t.company_id=o.company_id AND t.id=o.task_id
JOIN worker_sessions ws ON ws.company_id=o.company_id AND ws.id=$3 AND ws.task_id=o.task_id
WHERE o.company_id=$1 AND o.id=$2 AND m.contract_revision_id IS NULL AND t.mission_id=m.mission_id
FOR UPDATE OF o,m`, b.scope.company, request.ObligationID, b.session).Scan(&taskID, &owner, &recipient, &messageState, &kind, &obligationState)
		if err != nil || taskID != b.task || owner != b.employee || recipient != b.employee || kind != "request" ||
			(messageState != "observed" && messageState != "acknowledged") || (obligationState != "observed" && obligationState != "acknowledged") {
			return Receipt{}, core.Denied
		}
		var currentRevision, observedRevision int64
		var currentDigest string
		if err = tx.QueryRow(ctx, `SELECT revision,digest FROM worker_workspaces WHERE company_id=$1 AND task_id=$2`, b.scope.company, taskID).Scan(&currentRevision, &currentDigest); err != nil {
			return Receipt{}, err
		}
		if request.WorkspaceRevision != currentRevision || currentRevision < 1 {
			return Receipt{}, core.Denied
		}
		if err = tx.QueryRow(ctx, `SELECT task_revision FROM messages WHERE company_id=$1 AND id=$2`, b.scope.company, request.ObligationID).Scan(&observedRevision); err != nil {
			return Receipt{}, err
		}
		if currentRevision <= observedRevision {
			return Receipt{}, core.Denied
		}
		mutationEvidence, checkedEvidence := false, false
		for _, ref := range request.EvidenceRefs {
			kind, valid, evidenceErr := peerEvidenceRefKind(ctx, tx, b, ref, currentDigest)
			if evidenceErr != nil {
				return Receipt{}, evidenceErr
			}
			if !valid || (kind != "workspace.replace" && kind != "workspace.check" && kind != "collab.apply") {
				return Receipt{}, core.Denied
			}
			if kind == "workspace.replace" {
				mutationEvidence = true
			}
			if kind == "workspace.check" {
				checkedEvidence = true
			}
		}
		if !mutationEvidence || !checkedEvidence {
			return Receipt{}, core.Denied
		}
		if _, err = tx.Exec(ctx, `UPDATE messages SET delivery_state='applied' WHERE company_id=$1 AND id=$2`, b.scope.company, request.ObligationID); err != nil {
			return Receipt{}, err
		}
		if _, err = tx.Exec(ctx, `UPDATE obligations SET state='applied',evidence_ref=$3 WHERE company_id=$1 AND id=$2`, b.scope.company, request.ObligationID, string(evidenceRaw)); err != nil {
			return Receipt{}, err
		}
		if _, err = tx.Exec(ctx, `UPDATE peer_work_signals SET state='applied' WHERE company_id=$1 AND obligation_id=$2`, b.scope.company, request.ObligationID); err != nil {
			return Receipt{}, err
		}
		return Receipt{ID: request.ObligationID, Status: "applied"}, nil
	})
}

func (k *Kernel) TXProductDirectResolve(ctx context.Context, b Binding, obligationID, artifactID, key string) error {
	if obligationID == "" || artifactID == "" {
		return core.Malformed
	}
	_, err := k.TXWrite(ctx, b.scope, &b, key, "product.obligation.resolve", []string{obligationID, artifactID}, func(tx pgx.Tx) (Receipt, error) {
		var taskID, owner, recipient, kind, deliveryState, obligationState, taskState string
		err := tx.QueryRow(ctx, `SELECT o.task_id,o.owner,m.recipient,m.kind,m.delivery_state,o.state,t.state
FROM obligations o JOIN messages m ON m.company_id=o.company_id AND m.id=o.id
JOIN tasks t ON t.company_id=o.company_id AND t.id=o.task_id
JOIN worker_sessions ws ON ws.company_id=o.company_id AND ws.id=$3 AND ws.task_id=o.task_id
WHERE o.company_id=$1 AND o.id=$2 AND m.contract_revision_id IS NULL AND t.mission_id=m.mission_id
FOR UPDATE OF o,m`, b.scope.company, obligationID, b.session).Scan(&taskID, &owner, &recipient, &kind, &deliveryState, &obligationState, &taskState)
		if err != nil || taskID != b.task || owner != b.employee || recipient != b.employee || kind != "request" ||
			deliveryState != "applied" || obligationState != "applied" || taskState != "candidate" {
			return Receipt{}, core.Denied
		}
		var digest string
		if err = tx.QueryRow(ctx, `SELECT a.digest FROM artifacts a JOIN tasks t ON t.company_id=a.company_id AND t.id=a.task_id
JOIN missions m ON m.company_id=t.company_id AND m.id=t.mission_id
WHERE a.company_id=$1 AND a.id=$2 AND a.task_id=$3 AND a.author=$4 AND a.artifact_kind='deliverable' AND a.state='ready' AND a.verdict='candidate' AND a.contract=m.contract`,
			b.scope.company, artifactID, taskID, b.employee).Scan(&digest); err != nil {
			return Receipt{}, core.Denied
		}
		if _, err = tx.Exec(ctx, `UPDATE obligations SET state='fulfilled',evidence_id=$3,evidence_ref=$3 WHERE company_id=$1 AND id=$2`, b.scope.company, obligationID, artifactID); err != nil {
			return Receipt{}, err
		}
		if _, err = tx.Exec(ctx, `UPDATE messages SET delivery_state='resolved' WHERE company_id=$1 AND id=$2`, b.scope.company, obligationID); err != nil {
			return Receipt{}, err
		}
		if _, err = tx.Exec(ctx, `UPDATE peer_work_signals SET state='resolved' WHERE company_id=$1 AND obligation_id=$2`, b.scope.company, obligationID); err != nil {
			return Receipt{}, err
		}
		return Receipt{ID: artifactID, Status: "fulfilled"}, nil
	})
	return err
}

func fixedDirectMessageEmployee(employeeID string) bool {
	switch employeeID {
	case core.EmployeePlanningID, core.EmployeeBackendID, core.EmployeeFrontendID, core.EmployeeReviewID:
		return true
	default:
		return false
	}
}

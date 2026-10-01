// pattern: Imperative Shell
package kernel

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
	"polis/internal/core"
	"polis/internal/taskvalidation"
)

func (k *Kernel) requireProductTaskWorking(ctx context.Context, tx pgx.Tx, b Binding) (string, error) {
	var taskID, taskState, missionState string
	err := tx.QueryRow(ctx, `SELECT s.task_id,t.state,m.state
FROM worker_sessions s
JOIN tasks t ON t.company_id=s.company_id AND t.id=s.task_id
JOIN missions m ON m.company_id=t.company_id AND m.id=t.mission_id
WHERE s.company_id=$1 AND s.id=$2 AND s.state='active'`, b.scope.company, b.session).Scan(&taskID, &taskState, &missionState)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", core.Denied
	}
	if err != nil {
		return "", err
	}
	if taskState != "working" || missionState != "active" {
		return "", core.Denied
	}
	if err = requireMemoryTaskWritableTX(ctx, tx, b.scope.company, taskID); err != nil {
		return "", err
	}
	return taskID, nil
}

// TXProductWorkspaceCheck evaluates the persisted TaskValidationBinding and
// stores a session-, binding-, digest- and revision-scoped result receipt.
func (k *Kernel) TXProductWorkspaceCheck(ctx context.Context, b Binding, key string) (Receipt, taskvalidation.Result, error) {
	handover, err := k.Handover(ctx, b)
	if err != nil {
		return Receipt{}, taskvalidation.Result{}, err
	}
	result := taskvalidation.DefaultRegistry().Validate(handover.Task.ValidationBinding, handover.Workspace.Content)
	if handover.Task.ValidationBinding == nil {
		result.TaskID = handover.Task.ID
		result.MissionID = handover.Task.Mission
	}
	result.WorkspaceDigest = handover.Workspace.Digest
	result.WorkspaceRevision = handover.Workspace.Revision
	result.SessionID = b.session
	result.Epoch = b.epoch
	receipt, err := k.TXWrite(ctx, b.scope, &b, key, "workspace.check", struct {
		TaskID            string
		BindingDigest     string
		WorkspaceDigest   string
		WorkspaceRevision int64
		SessionID         string
	}{handover.Task.ID, result.ConfigurationDigest, handover.Workspace.Digest, handover.Workspace.Revision, b.session}, func(tx pgx.Tx) (Receipt, error) {
		taskID, stateErr := k.requireProductTaskWorking(ctx, tx, b)
		if stateErr != nil {
			return Receipt{}, stateErr
		}
		var currentTaskID, digest string
		var revision int64
		var bindingDigest string
		err = tx.QueryRow(ctx, `SELECT s.task_id,w.digest,w.revision,COALESCE(v.configuration_digest,'')
FROM worker_sessions s
JOIN tasks t ON t.company_id=s.company_id AND t.id=s.task_id
JOIN worker_workspaces w ON w.company_id=s.company_id AND w.task_id=s.task_id
LEFT JOIN task_validation_bindings v ON v.company_id=t.company_id AND v.task_id=t.id
WHERE s.company_id=$1 AND s.id=$2 AND s.state='active'`, b.scope.company, b.session).Scan(&currentTaskID, &digest, &revision, &bindingDigest)
		if err != nil {
			return Receipt{}, err
		}
		if currentTaskID != taskID || taskID != handover.Task.ID || digest != handover.Workspace.Digest || revision != handover.Workspace.Revision || bindingDigest != result.ConfigurationDigest {
			return Receipt{}, core.Conflict
		}
		body, err := json.Marshal(result)
		if err != nil {
			return Receipt{}, err
		}
		id := newID()
		_, err = tx.Exec(ctx, `INSERT INTO worker_checks(company_id,id,session_id,digest,phase,passed,report)
VALUES($1,$2,$3,$4,'product',$5,$6)`, b.scope.company, id, b.session, digest, result.Status == taskvalidation.StatusPass, body)
		return Receipt{ID: id, Status: "persisted"}, err
	})
	return receipt, result, err
}

// TXProductCheckpoint applies the ordinary checkpoint validation plus the
// stricter product TaskValidationBinding receipt policy.
func (k *Kernel) TXProductCheckpoint(ctx context.Context, b Binding, key string, checkpoint Checkpoint) (Receipt, error) {
	return k.txCheckpoint(ctx, b, key, checkpoint, true)
}

type productQualification struct {
	CheckpointID      string
	CheckID           string
	BindingDigest     string
	WorkspaceDigest   string
	WorkspaceRevision int64
	RunnerRevision    string
}

func (k *Kernel) currentProductQualification(ctx context.Context, tx pgx.Tx, b Binding, task Task, expectedWorkspaceDigest string) (productQualification, error) {
	var current productQualification
	var taskID, state string
	var epoch int64
	err := tx.QueryRow(ctx, `SELECT s.task_id,t.state,s.epoch,w.digest,w.revision,v.configuration_digest,v.runner_revision
FROM worker_sessions s
JOIN tasks t ON t.company_id=s.company_id AND t.id=s.task_id
JOIN worker_workspaces w ON w.company_id=s.company_id AND w.task_id=s.task_id
JOIN task_validation_bindings v ON v.company_id=t.company_id AND v.task_id=t.id
WHERE s.company_id=$1 AND s.id=$2 AND s.state='active'`, b.scope.company, b.session).Scan(&taskID, &state, &epoch, &current.WorkspaceDigest, &current.WorkspaceRevision, &current.BindingDigest, &current.RunnerRevision)
	if errors.Is(err, pgx.ErrNoRows) {
		return current, core.Denied
	}
	if err != nil {
		return current, err
	}
	if taskID != task.ID || state != "working" || current.WorkspaceDigest != expectedWorkspaceDigest {
		return current, core.Conflict
	}
	rows, err := tx.Query(ctx, `SELECT id,data FROM worker_checkpoints
WHERE company_id=$1 AND session_id=$2 AND digest=$3 ORDER BY id DESC`, b.scope.company, b.session, current.WorkspaceDigest)
	if err != nil {
		return current, err
	}
	type storedCheckpoint struct {
		id   string
		data []byte
	}
	checkpoints := make([]storedCheckpoint, 0)
	for rows.Next() {
		var item storedCheckpoint
		if err = rows.Scan(&item.id, &item.data); err != nil {
			rows.Close()
			return current, err
		}
		checkpoints = append(checkpoints, item)
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		return current, err
	}
	rows.Close()
	for _, stored := range checkpoints {
		var checkpoint Checkpoint
		if json.Unmarshal(stored.data, &checkpoint) != nil || checkpoint.Kind != CheckpointQualified || checkpoint.FinalizationState != "current" || checkpoint.TaskValidationBindingDigest != current.BindingDigest || checkpoint.WorkspaceDigest != current.WorkspaceDigest || checkpoint.WorkspaceRevision != current.WorkspaceRevision || checkpoint.SessionID != b.session || checkpoint.Epoch != epoch || checkpoint.ValidationStatus != string(taskvalidation.StatusPass) || len(checkpoint.EvidenceRefs) == 0 {
			continue
		}
		valid := true
		for _, checkID := range checkpoint.EvidenceRefs {
			var passed bool
			var raw []byte
			err = tx.QueryRow(ctx, `SELECT passed,report FROM worker_checks WHERE company_id=$1 AND id=$2 AND session_id=$3 AND digest=$4 AND phase='product'`, b.scope.company, checkID, b.session, current.WorkspaceDigest).Scan(&passed, &raw)
			if errors.Is(err, pgx.ErrNoRows) {
				valid = false
				break
			}
			if err != nil {
				return current, err
			}
			var result taskvalidation.Result
			if !passed || json.Unmarshal(raw, &result) != nil || result.Status != taskvalidation.StatusPass || result.TaskID != task.ID || result.ConfigurationDigest != current.BindingDigest || result.WorkspaceDigest != current.WorkspaceDigest || result.WorkspaceRevision != current.WorkspaceRevision || result.SessionID != b.session || result.Epoch != epoch {
				valid = false
				break
			}
		}
		if valid {
			current.CheckpointID = stored.id
			current.CheckID = checkpoint.EvidenceRefs[0]
			return current, nil
		}
	}
	return current, core.Denied
}

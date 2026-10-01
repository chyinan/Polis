// pattern: Imperative Shell
package kernel

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"polis/internal/core"
	"polis/internal/taskvalidation"
)

type ProductDeliveryResult struct {
	TaskID            string `json:"task_id"`
	MissionID         string `json:"mission_id"`
	CheckpointID      string `json:"checkpoint_id"`
	CheckID           string `json:"validation_receipt_id"`
	ArtifactID        string `json:"artifact_id"`
	WorkspaceDigest   string `json:"workspace_digest"`
	WorkspaceRevision int64  `json:"workspace_revision"`
	TaskState         string `json:"task_state"`
	DeliveryState     string `json:"delivery_state"`
}

type productDeliveryFaultPoint string

const (
	productDeliveryAfterStagePersist      productDeliveryFaultPoint = "after_stage_persist"
	productDeliveryAfterCASPublish        productDeliveryFaultPoint = "after_cas_publish"
	productDeliveryAfterCheckpointPersist productDeliveryFaultPoint = "after_checkpoint_persist"
	productDeliveryAfterArtifactPersist   productDeliveryFaultPoint = "after_artifact_persist"
	productDeliveryAfterTaskTransition    productDeliveryFaultPoint = "after_task_transition"
	productDeliveryAfterCommit            productDeliveryFaultPoint = "after_commit"
)

type productDeliveryFault func(productDeliveryFaultPoint) error

type productDeliveryPhaseError struct {
	phase string
	err   error
}

func (e productDeliveryPhaseError) Error() string {
	return fmt.Sprintf("product delivery %s: %v", e.phase, e.err)
}
func (e productDeliveryPhaseError) Unwrap() error { return e.err }

// TXSubmitTaskDelivery is the single explicit final-delivery operation for a
// product Task. Staging and CAS are recoverable prerequisites; checkpoint,
// Artifact qualification, Artifact publication metadata and candidate state
// commit atomically under TXWrite's company lifecycle guard.
func (k *Kernel) TXSubmitTaskDelivery(ctx context.Context, b Binding, w Task, key string, content []byte) (Receipt, ProductDeliveryResult, error) {
	return k.txSubmitTaskDelivery(ctx, b, w, key, content, nil)
}

func (k *Kernel) txSubmitTaskDelivery(ctx context.Context, b Binding, w Task, key string, content []byte, fault productDeliveryFault) (Receipt, ProductDeliveryResult, error) {
	if len(content) == 0 {
		return Receipt{}, ProductDeliveryResult{}, core.Malformed
	}
	if len(content) > core.MaxContent {
		return Receipt{}, ProductDeliveryResult{}, core.TooLarge
	}
	digestBytes := sha256.Sum256(content)
	digest := hex.EncodeToString(digestBytes[:])
	stageKey := "delivery-stage-" + fingerprint(struct {
		TaskID     string
		Generation int64
		Digest     string
	}{w.ID, w.Generation, digest})[:48]
	stage, err := k.TXWrite(ctx, b.scope, &b, stageKey, "artifact.stage", []string{w.ID, digest}, func(tx pgx.Tx) (Receipt, error) {
		task, checkErr := checkWork(ctx, tx, b, w)
		if checkErr != nil {
			return Receipt{}, checkErr
		}
		if task.Kind != core.TaskKindCompat || task.State != "working" {
			return Receipt{}, productDeliveryDenied("task_state_disallows_delivery", "the current Task must be the working provider-executable compat Task.", "task_state", "working compat Task", task.State)
		}
		if _, checkErr = k.currentProductValidation(ctx, tx, b, task, digest); checkErr != nil {
			return Receipt{}, checkErr
		}
		var existing, existingDigest string
		checkErr = tx.QueryRow(ctx, "SELECT id,digest FROM artifact_staging WHERE company_id=$1 AND task_id=$2", b.scope.company, task.ID).Scan(&existing, &existingDigest)
		if checkErr == nil {
			if existingDigest != digest {
				return Receipt{}, productDeliveryDenied("workspace_changed_since_validation", "the staged delivery does not match the currently validated workspace.", "workspace_digest", "current validated workspace digest", "staged_digest_mismatch")
			}
			return Receipt{ID: existing, Status: "staging"}, nil
		}
		if !errors.Is(checkErr, pgx.ErrNoRows) {
			return Receipt{}, checkErr
		}
		id := newID()
		if _, checkErr = tx.Exec(ctx, "INSERT INTO artifact_staging(company_id,id,task_id,digest) VALUES($1,$2,$3,$4)", b.scope.company, id, task.ID, digest); checkErr != nil {
			return Receipt{}, checkErr
		}
		return Receipt{ID: id, Status: "staging"}, nil
	})
	if err != nil {
		return Receipt{}, ProductDeliveryResult{}, wrapDeliveryError("staging", err)
	}
	if err = invokeProductDeliveryFault(fault, productDeliveryAfterStagePersist); err != nil {
		return Receipt{}, ProductDeliveryResult{}, productDeliveryPhaseError{phase: "staging", err: err}
	}
	if _, err = putBlob(k.root, b.scope.company, content); err != nil {
		return Receipt{}, ProductDeliveryResult{}, productDeliveryPhaseError{phase: "staging", err: err}
	}
	if err = invokeProductDeliveryFault(fault, productDeliveryAfterCASPublish); err != nil {
		return Receipt{}, ProductDeliveryResult{}, productDeliveryPhaseError{phase: "staging", err: err}
	}

	receipt, err := k.TXWrite(ctx, b.scope, &b, key, "task.delivery", struct {
		TaskID     string
		Generation int64
		Digest     string
		StageID    string
	}{w.ID, w.Generation, digest, stage.ID}, func(tx pgx.Tx) (Receipt, error) {
		task, checkErr := checkWork(ctx, tx, b, w)
		if checkErr != nil {
			return Receipt{}, checkErr
		}
		if task.Kind != core.TaskKindCompat {
			return Receipt{}, productDeliveryDenied("task_state_disallows_delivery", "only the provider-executable compat Task can be delivered.", "task_kind", "compat", string(task.Kind))
		}

		var existing ProductDeliveryResult
		var existingSession, existingArtifactDigest string
		checkErr = tx.QueryRow(ctx, `SELECT q.task_id,t.mission_id,q.checkpoint_id,q.check_id,q.session_id,q.workspace_digest,q.workspace_revision,q.artifact_id,a.digest,t.state
FROM task_validation_artifact_qualifications q
JOIN artifacts a ON a.company_id=q.company_id AND a.id=q.artifact_id
JOIN tasks t ON t.company_id=q.company_id AND t.id=q.task_id

WHERE q.company_id=$1 AND q.task_id=$2`, b.scope.company, task.ID).Scan(&existing.TaskID, &existing.MissionID, &existing.CheckpointID, &existing.CheckID, &existingSession, &existing.WorkspaceDigest, &existing.WorkspaceRevision, &existing.ArtifactID, &existingArtifactDigest, &existing.TaskState)
		if checkErr == nil {
			if existing.WorkspaceDigest != digest || existingArtifactDigest != digest {
				return Receipt{}, productDeliveryDenied("delivery_already_committed", "this Task already has a committed delivery for a different workspace digest.", "delivery", "one committed delivery for the validated workspace", "different_workspace_digest")
			}
			existing.DeliveryState = "committed"
			return Receipt{ID: existing.ArtifactID, Status: "candidate"}, nil
		}
		if !errors.Is(checkErr, pgx.ErrNoRows) {
			return Receipt{}, checkErr
		}
		if task.State != "working" {
			return Receipt{}, productDeliveryDenied("task_state_disallows_delivery", "the current Task state does not allow final delivery.", "task_state", "working", task.State)
		}
		if checkErr = requireMemoryTaskCleanTX(ctx, tx, b.scope.company, task.ID); checkErr != nil {
			return Receipt{}, checkErr
		}
		qualification, checkErr := k.currentProductValidation(ctx, tx, b, task, digest)
		if checkErr != nil {
			return Receipt{}, checkErr
		}
		checkpoint, checkErr := NewQualifiedProductDeliveryCheckpoint(ProductDeliveryCheckpointInput{
			TaskID: task.ID, BindingDigest: qualification.BindingDigest, WorkspaceDigest: qualification.WorkspaceDigest,
			WorkspaceRevision: qualification.WorkspaceRevision, SessionID: b.session, Epoch: b.epoch,
			CheckID: qualification.CheckID, RunnerRevision: qualification.RunnerRevision,
		})
		if checkErr != nil {
			return Receipt{}, checkErr
		}
		checkpoint.CheckpointPolicyRevision = core.CheckpointPolicyRevision
		checkpoint.ArtifactEligibilityPolicyRevision = core.ArtifactEligibilityPolicyRevision
		checkpoint.ContractSupersessionPolicyRevision = core.PeerContractSupersessionPolicyRevision
		checkpointID := newID()
		checkpointRaw, checkErr := json.Marshal(checkpoint)
		if checkErr != nil {
			return Receipt{}, checkErr
		}
		if _, checkErr = tx.Exec(ctx, "INSERT INTO worker_checkpoints(company_id,id,session_id,digest,data) VALUES($1,$2,$3,$4,$5)", b.scope.company, checkpointID, b.session, qualification.WorkspaceDigest, checkpointRaw); checkErr != nil {
			return Receipt{}, checkErr
		}
		if err = invokeProductDeliveryFault(fault, productDeliveryAfterCheckpointPersist); err != nil {
			return Receipt{}, productDeliveryPhaseError{phase: "publication", err: err}
		}

		var stagedDigest string
		if checkErr = tx.QueryRow(ctx, "SELECT digest FROM artifact_staging WHERE company_id=$1 AND id=$2 AND task_id=$3", b.scope.company, stage.ID, task.ID).Scan(&stagedDigest); checkErr != nil {
			return Receipt{}, productDeliveryPhaseError{phase: "staging", err: checkErr}
		}
		if stagedDigest != qualification.WorkspaceDigest {
			return Receipt{}, productDeliveryDenied("workspace_changed_since_validation", "the durable staging record no longer matches the validated workspace.", "workspace_digest", "validated workspace digest", "staged_digest_mismatch")
		}
		artifactID := stage.ID
		if _, checkErr = tx.Exec(ctx, "INSERT INTO artifacts(company_id,id,task_id,author,digest,bytes,state,contract) SELECT $1,$2,$3,$4,$5,$6,'ready',contract FROM missions WHERE company_id=$1 AND id=$7", b.scope.company, artifactID, task.ID, b.employee, qualification.WorkspaceDigest, len(content), task.Mission); checkErr != nil {
			return Receipt{}, productDeliveryPhaseError{phase: "publication", err: checkErr}
		}
		if _, checkErr = tx.Exec(ctx, `INSERT INTO task_validation_artifact_qualifications(company_id,task_id,artifact_id,checkpoint_id,check_id,session_id,validation_binding_digest,workspace_digest,workspace_revision,runner_revision)
VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`, b.scope.company, task.ID, artifactID, checkpointID, qualification.CheckID, b.session, qualification.BindingDigest, qualification.WorkspaceDigest, qualification.WorkspaceRevision, qualification.RunnerRevision); checkErr != nil {
			return Receipt{}, productDeliveryPhaseError{phase: "publication", err: checkErr}
		}
		if err = invokeProductDeliveryFault(fault, productDeliveryAfterArtifactPersist); err != nil {
			return Receipt{}, productDeliveryPhaseError{phase: "publication", err: err}
		}
		if _, checkErr = tx.Exec(ctx, "UPDATE tasks SET state='candidate' WHERE company_id=$1 AND id=$2 AND state='working'", b.scope.company, task.ID); checkErr != nil {
			return Receipt{}, productDeliveryPhaseError{phase: "publication", err: checkErr}
		}
		if err = invokeProductDeliveryFault(fault, productDeliveryAfterTaskTransition); err != nil {
			return Receipt{}, productDeliveryPhaseError{phase: "publication", err: err}
		}
		return Receipt{ID: artifactID, Status: "candidate"}, nil
	})
	if err != nil {
		return Receipt{}, ProductDeliveryResult{}, wrapDeliveryError("publication", err)
	}
	if err = invokeProductDeliveryFault(fault, productDeliveryAfterCommit); err != nil {
		return receipt, ProductDeliveryResult{}, productDeliveryPhaseError{phase: "response", err: err}
	}
	result, err := k.productDeliveryResult(ctx, b, w.ID, receipt.ID)
	if err != nil {
		return receipt, ProductDeliveryResult{}, productDeliveryPhaseError{phase: "response", err: err}
	}
	return receipt, result, nil
}

func invokeProductDeliveryFault(fault productDeliveryFault, point productDeliveryFaultPoint) error {
	if fault == nil {
		return nil
	}
	return fault(point)
}

func wrapDeliveryError(phase string, err error) error {
	var policy peerToolError
	if errors.As(err, &policy) {
		return err
	}
	return productDeliveryPhaseError{phase: phase, err: err}
}

func productDeliveryDenied(reason, summary, field, expected, actual string) error {
	rejection := newCheckpointRejection(reason, summary)
	rejection.FailingField = field
	rejection.ExpectedPublicShape = expected
	rejection.ActualCategory = actual
	rejection.TransactionOutcome = "not_started"
	return peerToolError{Code: core.Denied, Rejection: rejection}
}

func productDeliveryCodeRejection(err error) (PeerToolRejection, core.Code) {
	var policy peerToolError
	if errors.As(err, &policy) {
		return policy.Rejection, policy.Code
	}
	if errors.Is(err, core.StaleEpoch) {
		return productDeliveryRequestRejection("writer_fenced", "the worker session is fenced; use the current authorized writer session.", "stale_or_fenced_writer"), core.Denied
	}
	if errors.Is(err, core.Conflict) {
		return productDeliveryRequestRejection("workspace_changed_since_validation", "the workspace or delivery identity changed; re-read and validate the current workspace.", "workspace_or_delivery_conflict"), core.Conflict
	}
	var phase productDeliveryPhaseError
	if errors.As(err, &phase) {
		reason := "artifact_publication_failed"
		if phase.phase == "staging" {
			reason = "artifact_staging_failed"
		}
		return productDeliveryFailureRejection(reason, "the delivery control-plane step failed; retry the same explicit task_submit request to recover the staged delivery.", phase.phase, "recoverable delivery transaction boundary", phase.phase+"_failure"), core.Integrity
	}
	if errors.Is(err, core.Denied) {
		return productDeliveryFailureRejection("task_state_disallows_delivery", "the current Task/session state does not allow final delivery.", "task_state", "working compat Task with active authorized session", "policy_denied"), core.Denied
	}
	return PeerToolRejection{}, ""
}

func productDeliveryFailureRejection(reason, summary, field, expected, actual string) PeerToolRejection {
	rejection := newCheckpointRejection(reason, summary)
	rejection.FailingField = field
	rejection.ExpectedPublicShape = expected
	rejection.ActualCategory = actual
	rejection.TransactionOutcome = "not_started"
	return rejection
}

func (k *Kernel) currentProductValidation(ctx context.Context, tx pgx.Tx, b Binding, task Task, expectedDigest string) (productQualification, error) {
	var out productQualification
	var taskID, taskState, missionID, digest, bindingDigest, runnerRevision string
	var workspaceRevision, epoch int64
	err := tx.QueryRow(ctx, `SELECT s.task_id,t.state,t.mission_id,w.digest,w.revision,s.epoch,COALESCE(v.configuration_digest,''),COALESCE(v.runner_revision,'')
FROM worker_sessions s
JOIN tasks t ON t.company_id=s.company_id AND t.id=s.task_id
JOIN worker_workspaces w ON w.company_id=s.company_id AND w.task_id=s.task_id
LEFT JOIN task_validation_bindings v ON v.company_id=t.company_id AND v.task_id=t.id
WHERE s.company_id=$1 AND s.id=$2 AND s.state='active'`, b.scope.company, b.session).Scan(&taskID, &taskState, &missionID, &digest, &workspaceRevision, &epoch, &bindingDigest, &runnerRevision)
	if errors.Is(err, pgx.ErrNoRows) {
		return out, productDeliveryDenied("writer_fenced", "the current worker session is not active; use the current authorized writer.", "session", "active current worker session", "missing_or_inactive")
	}
	if err != nil {
		return out, err
	}
	if taskID != task.ID || taskState != "working" || task.Kind != core.TaskKindCompat {
		return out, productDeliveryDenied("Task_state_disallows_delivery", "the current Task is not an active provider-executable delivery target.", "task_state", "working compat Task", taskState)
	}
	if bindingDigest == "" || runnerRevision == "" {
		return out, productDeliveryDenied("validation_required", "run workspace_check and obtain a PASS receipt for a Task with an immutable TaskValidationBinding.", "validation_receipt", "current PASS receipt bound to this Task", "missing_binding")
	}
	if digest != expectedDigest {
		return out, productDeliveryDenied("workspace_changed_since_validation", "the workspace changed after the validated snapshot; re-read and validate it again.", "workspace_digest", "current workspace digest", "digest_changed")
	}
	rows, err := tx.Query(ctx, `SELECT id,passed,report FROM worker_checks WHERE company_id=$1 AND session_id=$2 AND digest=$3 AND phase='product' ORDER BY id DESC`, b.scope.company, b.session, digest)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	foundCurrent := false
	hadFailed := false
	for rows.Next() {
		foundCurrent = true
		var checkID string
		var passed bool
		var raw []byte
		if err = rows.Scan(&checkID, &passed, &raw); err != nil {
			return out, err
		}
		if !passed {
			hadFailed = true
			continue
		}
		var result taskvalidation.Result
		if json.Unmarshal(raw, &result) != nil {
			return out, productDeliveryDenied("validation_receipt_invalid", "the validation receipt is not a valid product validation result.", "validation_receipt", "typed current product validation result", "malformed_report")
		}
		if result.Status != taskvalidation.StatusPass {
			hadFailed = true
			continue
		}
		if result.TaskID != task.ID || result.MissionID != missionID || result.ConfigurationDigest != bindingDigest || result.WorkspaceDigest != digest || result.WorkspaceRevision != workspaceRevision || result.SessionID != b.session || result.Epoch != epoch {
			return out, productDeliveryDenied("validation_receipt_invalid", "the validation receipt does not belong to the current Task, binding, session or workspace revision.", "validation_receipt", "current Task/session/binding/workspace PASS receipt", "binding_mismatch")
		}
		out.CheckID = checkID
		out.BindingDigest = bindingDigest
		out.WorkspaceDigest = digest
		out.WorkspaceRevision = workspaceRevision
		out.RunnerRevision = runnerRevision
		return out, nil
	}
	if err = rows.Err(); err != nil {
		return out, err
	}
	if foundCurrent && hadFailed {
		return out, productDeliveryDenied("validation_not_passed", "the current workspace validation did not PASS; fix the workspace and run workspace_check again.", "validation_receipt", "PASS", "FAIL")
	}
	var anyChecks int
	if err = tx.QueryRow(ctx, "SELECT count(*) FROM worker_checks WHERE company_id=$1 AND session_id=$2 AND phase='product'", b.scope.company, b.session).Scan(&anyChecks); err != nil {
		return out, err
	}
	if anyChecks > 0 {
		return out, productDeliveryDenied("validation_receipt_stale", "the validation receipt belongs to an older workspace revision or digest; validate the current workspace again.", "validation_receipt", "current workspace PASS receipt", "stale_workspace")
	}
	return out, productDeliveryDenied("validation_required", "run workspace_check and obtain a PASS receipt before task_submit.", "validation_receipt", "current PASS receipt", "missing")
}

func (k *Kernel) productDeliveryResult(ctx context.Context, b Binding, taskID, artifactID string) (ProductDeliveryResult, error) {
	var result ProductDeliveryResult
	err := k.pool.QueryRow(ctx, `SELECT q.task_id,t.mission_id,q.checkpoint_id,q.check_id,q.artifact_id,q.workspace_digest,q.workspace_revision,t.state
FROM task_validation_artifact_qualifications q
JOIN tasks t ON t.company_id=q.company_id AND t.id=q.task_id
WHERE q.company_id=$1 AND q.task_id=$2 AND q.artifact_id=$3`, b.scope.company, taskID, artifactID).Scan(&result.TaskID, &result.MissionID, &result.CheckpointID, &result.CheckID, &result.ArtifactID, &result.WorkspaceDigest, &result.WorkspaceRevision, &result.TaskState)
	if err != nil {
		return result, err
	}
	result.DeliveryState = "committed"
	return result, nil
}

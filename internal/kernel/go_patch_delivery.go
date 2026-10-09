// pattern: Imperative Shell
package kernel

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/jackc/pgx/v5"
	"polis/internal/core"
)

type GoPatchDeliveryInput struct{ TaskID, JobRunID, RequestID string }

func (k *Kernel) goPatchWriter(ctx context.Context, company, task string) (Binding, error) {
	b := Binding{scope: k.LocalScope(company), task: task}
	err := k.pool.QueryRow(ctx, `SELECT r.writer_session_id,r.writer_epoch,t.owner,s.incarnation FROM worker_workspace_roots r JOIN tasks t ON t.company_id=r.company_id AND t.id=r.task_id JOIN worker_sessions s ON s.company_id=r.company_id AND s.id=r.writer_session_id WHERE r.company_id=$1 AND r.task_id=$2 AND s.state='active'`, company, task).Scan(&b.session, &b.epoch, &b.employee, &b.incarnation)
	return b, err
}

func (k *Kernel) goPatchInputsTX(ctx context.Context, tx pgx.Tx, b Binding, job string) (goPatchBuildInput, error) {
	var input goPatchBuildInput
	task, err := taskRow(ctx, tx, b.scope, b.task)
	if err != nil {
		return input, err
	}
	if task.State != "working" || task.ValidationBinding == nil || task.Kind != core.TaskKindCompat || task.Owner != b.employee {
		return input, core.Denied
	}
	if _, err = k.requireProductTaskWorking(ctx, tx, b); err != nil {
		return input, err
	}
	if err = k.requireMemoryTaskCleanTX(ctx, tx, b.scope.company, b.task); err != nil {
		return input, err
	}
	if open, err := missionHasOpenChangeRequestTx(ctx, tx, b.scope, task.Mission); err != nil {
		return input, err
	} else if open {
		return input, core.Conflict
	}
	input.record, err = k.goJobEvidenceTX(ctx, tx, b.scope.company, job)
	if errors.Is(err, pgx.ErrNoRows) {
		return input, core.Denied
	}
	if err != nil {
		return input, err
	}
	binding := input.record.binding
	if binding.TaskID != b.task || binding.MissionID != task.Mission {
		return input, core.Integrity
	}
	var rootID, writer string
	var revision, epoch int64
	if err = tx.QueryRow(ctx, `SELECT id,revision,writer_session_id,writer_epoch FROM worker_workspace_roots WHERE company_id=$1 AND task_id=$2 FOR UPDATE`, b.scope.company, b.task).Scan(&rootID, &revision, &writer, &epoch); err != nil {
		return input, err
	}
	if revision != binding.CandidateWorkspaceRevision || writer != b.session || epoch != b.epoch {
		return input, core.Conflict
	}
	base, err := k.loadGoTestWorkspaceSnapshotTX(ctx, tx, b.scope.company, task.Mission, task.ID, binding.BaseSnapshotArtifactID, rootID)
	if err != nil {
		return input, err
	}
	candidate, err := k.loadGoTestWorkspaceSnapshotTX(ctx, tx, b.scope.company, task.Mission, task.ID, binding.CandidateSnapshotArtifactID, rootID)
	if err != nil {
		return input, err
	}
	input.base = patchSnapshot(base, binding.BaseSnapshotArtifactID)
	input.candidate = patchSnapshot(candidate, binding.CandidateSnapshotArtifactID)
	input.validation = *task.ValidationBinding
	return input, nil
}

// TXPublishGoPatchDelivery is a format-specific producer in the existing
// staged/CAS/checkpoint/Artifact/DeliveryManifest transaction boundary.
func (k *Kernel) TXPublishGoPatchDelivery(ctx context.Context, company string, in GoPatchDeliveryInput) (ProductDeliveryResult, error) {
	var result ProductDeliveryResult
	if k == nil || ctx == nil || !core.ValidID(company) || !core.ValidID(in.TaskID) || !core.ValidID(in.JobRunID) || !core.ValidID(in.RequestID) {
		return result, core.Malformed
	}
	var existingArtifact, existingJob string
	err := k.pool.QueryRow(ctx, `SELECT artifact_id,job_id FROM go_patch_deliveries WHERE company_id=$1 AND task_id=$2`, company, in.TaskID).Scan(&existingArtifact, &existingJob)
	if err == nil {
		if existingJob != in.JobRunID {
			return result, core.Conflict
		}
		return k.productDeliveryResult(ctx, Binding{scope: k.LocalScope(company)}, in.TaskID, existingArtifact)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return result, err
	}
	b, err := k.goPatchWriter(ctx, company, in.TaskID)
	if err != nil {
		return result, err
	}
	var patch []byte
	stage, err := k.TXWrite(ctx, b.scope, &b, "go-patch-stage-"+in.JobRunID, "artifact.stage", in.JobRunID, func(tx pgx.Tx) (Receipt, error) {
		input, err := k.goPatchInputsTX(ctx, tx, b, in.JobRunID)
		if err != nil {
			return Receipt{}, err
		}
		patch, _, err = buildQualifiedGoPatch(input)
		if err != nil {
			return Receipt{}, err
		}
		var id, digest string
		err = tx.QueryRow(ctx, `SELECT id,digest FROM artifact_staging WHERE company_id=$1 AND task_id=$2 AND artifact_kind='deliverable'`, company, in.TaskID).Scan(&id, &digest)
		if err == nil {
			if digest != casDigest(patch) {
				return Receipt{}, core.Conflict
			}
			return Receipt{ID: id, Status: "staging"}, nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return Receipt{}, err
		}
		id = newID()
		_, err = tx.Exec(ctx, `INSERT INTO artifact_staging(company_id,id,task_id,digest) VALUES($1,$2,$3,$4)`, company, id, in.TaskID, casDigest(patch))
		return Receipt{ID: id, Status: "staging"}, err
	})
	if err != nil {
		return result, err
	}
	if len(patch) == 0 {
		// Receipt replay may skip the stage callback. Rebuild only from the same
		// frozen snapshots and receipt; never trust an arbitrary patch path.
		tx, err := k.pool.Begin(ctx)
		if err != nil {
			return result, err
		}
		input, err := k.goPatchInputsTX(ctx, tx, b, in.JobRunID)
		if err == nil {
			patch, _, err = buildQualifiedGoPatch(input)
		}
		_ = tx.Rollback(ctx)
		if err != nil {
			return result, err
		}
	}
	if _, err = k.putBlobWithClaim(ctx, company, patch); err != nil {
		return result, err
	}
	receipt, err := k.TXWrite(ctx, b.scope, &b, in.RequestID, "task.delivery", struct{ Task, Job, Digest string }{in.TaskID, in.JobRunID, casDigest(patch)}, func(tx pgx.Tx) (Receipt, error) {
		input, err := k.goPatchInputsTX(ctx, tx, b, in.JobRunID)
		if err != nil {
			return Receipt{}, err
		}
		verified, check, err := buildQualifiedGoPatch(input)
		if err != nil {
			return Receipt{}, err
		}
		if casDigest(verified) != casDigest(patch) {
			return Receipt{}, core.Integrity
		}
		var staged string
		if err = tx.QueryRow(ctx, `SELECT digest FROM artifact_staging WHERE company_id=$1 AND id=$2 AND task_id=$3 AND artifact_kind='deliverable'`, company, stage.ID, in.TaskID).Scan(&staged); err != nil || staged != casDigest(patch) {
			return Receipt{}, core.Integrity
		}
		check.SessionID, check.Epoch = b.session, b.epoch
		checkID, checkpointID := newID(), newID()
		checkRaw, _ := json.Marshal(check)
		if _, err = tx.Exec(ctx, `INSERT INTO worker_checks(company_id,id,session_id,digest,phase,passed,report) VALUES($1,$2,$3,$4,'product',true,$5)`, company, checkID, b.session, check.WorkspaceDigest, checkRaw); err != nil {
			return Receipt{}, err
		}
		cp, err := NewQualifiedProductDeliveryCheckpoint(ProductDeliveryCheckpointInput{TaskID: b.task, BindingDigest: check.ConfigurationDigest, WorkspaceDigest: check.WorkspaceDigest, WorkspaceRevision: check.WorkspaceRevision, SessionID: b.session, Epoch: b.epoch, CheckID: checkID, RunnerRevision: check.RunnerRevision})
		if err != nil {
			return Receipt{}, err
		}
		cp.CheckpointPolicyRevision = core.CheckpointPolicyRevision
		cp.ArtifactEligibilityPolicyRevision = core.ArtifactEligibilityPolicyRevision
		cp.ContractSupersessionPolicyRevision = core.PeerContractSupersessionPolicyRevision
		cpRaw, _ := json.Marshal(cp)
		if _, err = tx.Exec(ctx, `INSERT INTO worker_checkpoints(company_id,id,session_id,digest,data) VALUES($1,$2,$3,$4,$5)`, company, checkpointID, b.session, check.WorkspaceDigest, cpRaw); err != nil {
			return Receipt{}, err
		}
		if _, err = tx.Exec(ctx, `INSERT INTO artifacts(company_id,id,task_id,author,digest,bytes,state,verdict,contract,content_format) SELECT $1,$2,$3,$4,$5,$6,'ready','passed',contract,'go_patch' FROM missions WHERE company_id=$1 AND id=$7`, company, stage.ID, in.TaskID, b.employee, casDigest(patch), len(patch), input.record.binding.MissionID); err != nil {
			return Receipt{}, err
		}
		if _, err = tx.Exec(ctx, `INSERT INTO task_validation_artifact_qualifications(company_id,task_id,artifact_id,checkpoint_id,check_id,session_id,validation_binding_digest,workspace_digest,workspace_revision,runner_revision) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`, company, in.TaskID, stage.ID, checkpointID, checkID, b.session, check.ConfigurationDigest, check.WorkspaceDigest, check.WorkspaceRevision, check.RunnerRevision); err != nil {
			return Receipt{}, err
		}
		if _, err = tx.Exec(ctx, `INSERT INTO go_patch_deliveries(company_id,artifact_id,task_id,job_id,patch_sha256,candidate_snapshot_sha256,candidate_revision,job_binding_sha256,result_sha256,replay_verified) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,true)`, company, stage.ID, in.TaskID, in.JobRunID, casDigest(patch), check.WorkspaceDigest, check.WorkspaceRevision, input.record.report.BindingSHA256, input.record.resultSHA); err != nil {
			return Receipt{}, err
		}
		q := productQualification{CheckpointID: checkpointID, CheckID: checkID, BindingDigest: check.ConfigurationDigest, WorkspaceDigest: check.WorkspaceDigest, WorkspaceRevision: check.WorkspaceRevision, RunnerRevision: check.RunnerRevision}
		if err = persistInitialProductDeliveryManifest(ctx, tx, company, input.record.binding.MissionID, in.TaskID, stage.ID, len(patch), casDigest(patch), checkpointID, q); err != nil {
			return Receipt{}, err
		}
		// DeliveryManifest completion and explicit UserDisposition stay under
		// existing publication policy; no implicit acceptance or source apply.
		_, err = tx.Exec(ctx, `UPDATE tasks SET state='candidate' WHERE company_id=$1 AND id=$2 AND state='working'`, company, in.TaskID)
		return Receipt{ID: stage.ID, Status: "candidate"}, err
	})
	if err != nil {
		return result, err
	}
	return k.productDeliveryResult(ctx, b, in.TaskID, receipt.ID)
}

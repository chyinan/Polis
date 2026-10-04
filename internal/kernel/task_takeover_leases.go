// pattern: Imperative Shell
package kernel

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"polis/internal/core"
	"polis/internal/intake"
)

func (k *Kernel) TXCreateTaskTakeoverLease(ctx context.Context, scope Scope, missionID, taskID, key string) (TaskTakeoverLease, error) {
	if !core.ValidID(missionID) || !core.ValidID(taskID) || !core.ValidID(key) {
		return TaskTakeoverLease{}, core.Malformed
	}
	receipt, err := k.TXWrite(ctx, scope, nil, key, "task.takeover.granted", struct{ MissionID, TaskID string }{missionID, taskID}, func(tx pgx.Tx) (Receipt, error) {
		basis, err := missionChangeBasisTx(ctx, tx, scope, missionID, true)
		if err != nil {
			return Receipt{}, err
		}
		if basis.State != "paused" {
			return Receipt{}, core.ConflictError{Reason: "pause the Mission before requesting a human takeover lease", CurrentState: basis.State}
		}
		var taskState string
		if err = tx.QueryRow(ctx, "SELECT state FROM tasks WHERE company_id=$1 AND id=$2 AND mission_id=$3 FOR UPDATE", scope.company, taskID, missionID).Scan(&taskState); errors.Is(err, pgx.ErrNoRows) {
			return Receipt{}, core.OutOfScope
		} else if err != nil {
			return Receipt{}, err
		}
		if taskState == "completed" || taskState == "cancelled" {
			return Receipt{}, core.ConflictError{Reason: "terminal Tasks cannot be handed to a human editor", CurrentState: taskState}
		}
		var baseDigest string
		var baseRevision int64
		if err = tx.QueryRow(ctx, "SELECT digest,revision FROM worker_workspaces WHERE company_id=$1 AND task_id=$2", scope.company, taskID).Scan(&baseDigest, &baseRevision); errors.Is(err, pgx.ErrNoRows) {
			return Receipt{}, core.ConflictError{Reason: "Task has no persisted workspace to take over", CurrentState: "workspace_missing"}
		} else if err != nil {
			return Receipt{}, err
		}
		treeBinding, treeEntries, err := k.taskTakeoverWorkspaceTreeBindingTX(ctx, tx, scope, missionID, taskID, true)
		if err != nil {
			return Receipt{}, err
		}
		if treeBinding != nil && !taskTakeoverWorkspaceTreeMatchesLegacy(treeEntries, baseDigest, baseRevision) {
			return Receipt{}, core.ConflictError{Reason: "human takeover requires one workspace.txt whose digest and source revision match the frozen Task workspace", CurrentState: "workspace_tree_mismatch"}
		}
		baseRequirementsDigest, err := missionChangeRequirementsDigest(missionID, basis.Title, basis.Goal, basis.AcceptanceContract, basis.InputRevisions)
		if err != nil {
			return Receipt{}, err
		}
		impact, _, err := missionChangeImpactTx(ctx, tx, scope, missionID, baseRequirementsDigest, basis.InputRevisions)
		if err != nil {
			return Receipt{}, err
		}
		if !missionChangeWritersStopped(impact) {
			return Receipt{}, core.ConflictError{Reason: "all Mission WorkerSessions, JobRuns and service leases must be stopped before human takeover", CurrentState: "writers_not_stopped"}
		}
		if open, err := missionHasOpenChangeRequestTx(ctx, tx, scope, missionID); err != nil {
			return Receipt{}, err
		} else if open {
			return Receipt{}, core.ConflictError{Reason: "resolve the open formal change request before starting a human takeover", CurrentState: "change_request_open"}
		}
		var leaseExists bool
		if err = tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM task_takeover_active_slots WHERE company_id=$1 AND task_id=$2)", scope.company, taskID).Scan(&leaseExists); err != nil {
			return Receipt{}, err
		}
		if leaseExists {
			return Receipt{}, core.ConflictError{Reason: "Task already has an active human takeover lease", CurrentState: "takeover_lease_active"}
		}
		leaseID := newID()
		var treeRoot, treeRevision, treeManifest, treeFileCount, treeBytes any
		if treeBinding != nil {
			treeRoot, treeRevision, treeManifest = treeBinding.RootBindingID, treeBinding.Revision, treeBinding.ManifestSHA256
			treeFileCount, treeBytes = treeBinding.FileCount, treeBinding.Bytes
		}
		if _, err = tx.Exec(ctx, `INSERT INTO task_takeover_leases(company_id,lease_id,mission_id,task_id,client_request_id,base_requirements_sha256,base_workspace_digest,base_workspace_revision,created_by,
base_tree_root_id,base_tree_revision,base_tree_manifest_sha256,base_tree_file_count,base_tree_bytes)
VALUES($1,$2,$3,$4,$5,$6,$7,$8,'local_operator',$9,$10,$11,$12,$13)`, scope.company, leaseID, missionID, taskID, key, baseRequirementsDigest, baseDigest, baseRevision, treeRoot, treeRevision, treeManifest, treeFileCount, treeBytes); err != nil {
			return Receipt{}, err
		}
		if _, err = tx.Exec(ctx, `INSERT INTO task_takeover_active_slots(company_id,task_id,lease_id,mission_id) VALUES($1,$2,$3,$4)`, scope.company, taskID, leaseID, missionID); err != nil {
			return Receipt{}, err
		}
		if err = appendTaskTakeoverLeaseEvent(ctx, tx, scope, leaseID, "granted", "worker_tree_stopped", key, nil, nil, nil, nil, nil, nil); err != nil {
			return Receipt{}, err
		}
		if err = appendEvent(ctx, tx, scope, "task.takeover.granted", map[string]any{"mission_id": missionID, "task_id": taskID, "lease_id": leaseID, "base_workspace_digest": baseDigest, "base_workspace_revision": baseRevision}); err != nil {
			return Receipt{}, err
		}
		return Receipt{ID: leaseID, Status: "granted"}, nil
	})
	if err != nil {
		return TaskTakeoverLease{}, err
	}
	return k.TaskTakeoverLease(ctx, scope, missionID, receipt.ID)
}

func (k *Kernel) TXSubmitTaskTakeoverSnapshot(ctx context.Context, scope Scope, missionID, leaseID string, input TaskTakeoverSnapshotInput) (TaskTakeoverLease, error) {
	if !core.ValidID(missionID) || !core.ValidID(leaseID) || !core.ValidID(input.RequestID) || !validTaskInputDigest(input.BaseWorkspaceDigest) || input.BaseWorkspaceRevision < 1 || !validTaskTakeoverHumanEffortSeconds(input.HumanEffortSeconds) {
		return TaskTakeoverLease{}, core.Malformed
	}
	lease, err := k.TaskTakeoverLease(ctx, scope, missionID, leaseID)
	if err != nil {
		return TaskTakeoverLease{}, err
	}
	if lease.BaseWorkspaceDigest != input.BaseWorkspaceDigest || lease.BaseWorkspaceRevision != input.BaseWorkspaceRevision {
		return TaskTakeoverLease{}, core.ConflictError{Reason: "human snapshot does not match an active frozen takeover lease", CurrentState: lease.State}
	}
	content := []byte(input.Content)
	if len(content) == 0 || len(content) > core.MaxContent {
		return TaskTakeoverLease{}, core.TooLarge
	}
	prepared, err := intake.PrepareUpload("human-handover-"+lease.TaskID+".md", "text/markdown", content)
	if err != nil || intake.VerifyPreparedUpload(prepared, content) != nil {
		return TaskTakeoverLease{}, core.Malformed
	}
	baseContent, err := readBlob(k.root, scope.company, lease.BaseWorkspaceDigest)
	if err != nil {
		return TaskTakeoverLease{}, err
	}
	diffSummary, err := taskTakeoverDiffSummary(lease.BaseWorkspaceDigest, lease.BaseWorkspaceRevision, baseContent, content)
	if err != nil {
		return TaskTakeoverLease{}, err
	}
	digest := prepared.ContentDigest
	if digest != diffSummary.SubmittedContentDigest {
		return TaskTakeoverLease{}, core.Integrity
	}
	requestFingerprint := fingerprint(struct {
		MissionID          string
		LeaseID            string
		RequestID          string
		BaseDigest         string
		BaseRevision       int64
		ContentDigest      string
		HumanEffortSeconds int64
	}{missionID, leaseID, input.RequestID, input.BaseWorkspaceDigest, input.BaseWorkspaceRevision, digest, input.HumanEffortSeconds})
	if lease.State != "granted" {
		// Route an exact retry through the standard receipt guard before doing
		// any CAS writes. A new or mismatched request still conflicts here.
		receipt, replayErr := k.TXWrite(ctx, scope, nil, input.RequestID, "task.takeover.snapshot.returned", requestFingerprint, func(pgx.Tx) (Receipt, error) {
			return Receipt{}, core.ConflictError{Reason: "takeover lease is no longer active", CurrentState: lease.State}
		})
		if replayErr != nil {
			return TaskTakeoverLease{}, replayErr
		}
		return k.TaskTakeoverLease(ctx, scope, missionID, receipt.ID)
	}
	storedDigest, err := k.putBlobWithClaim(ctx, scope.company, content)
	if err != nil {
		return TaskTakeoverLease{}, err
	}
	if storedDigest != digest {
		return TaskTakeoverLease{}, core.Integrity
	}
	receipt, err := k.TXWrite(ctx, scope, nil, input.RequestID, "task.takeover.snapshot.returned", requestFingerprint, func(tx pgx.Tx) (Receipt, error) {
		currentLease, state, err := taskTakeoverLeaseForUpdate(ctx, tx, scope, missionID, leaseID)
		if err != nil {
			return Receipt{}, err
		}
		if state != "granted" || currentLease.BaseWorkspaceDigest != input.BaseWorkspaceDigest || currentLease.BaseWorkspaceRevision != input.BaseWorkspaceRevision {
			return Receipt{}, core.ConflictError{Reason: "takeover lease is no longer active or its frozen base differs", CurrentState: state}
		}
		basis, err := missionChangeBasisTx(ctx, tx, scope, missionID, true)
		if err != nil {
			return Receipt{}, err
		}
		if basis.State != "paused" {
			return Receipt{}, core.ConflictError{Reason: "Mission resumed before the human snapshot was returned", CurrentState: basis.State}
		}
		baseRequirementsDigest, err := missionChangeRequirementsDigest(missionID, basis.Title, basis.Goal, basis.AcceptanceContract, basis.InputRevisions)
		if err != nil {
			return Receipt{}, err
		}
		if baseRequirementsDigest != currentLease.BaseRequirementsSHA256 {
			return Receipt{}, core.ConflictError{Reason: "Mission requirements or inputs changed after the takeover lease was granted", CurrentState: "base_requirements_changed"}
		}
		impact, _, err := missionChangeImpactTx(ctx, tx, scope, missionID, baseRequirementsDigest, basis.InputRevisions)
		if err != nil {
			return Receipt{}, err
		}
		if !missionChangeWritersStopped(impact) {
			return Receipt{}, core.ConflictError{Reason: "a WorkerSession, JobRun or service lease became active before handback", CurrentState: "writers_not_stopped"}
		}
		if open, err := missionHasOpenChangeRequestTx(ctx, tx, scope, missionID); err != nil {
			return Receipt{}, err
		} else if open {
			return Receipt{}, core.ConflictError{Reason: "formal change request opened while the takeover lease was active", CurrentState: "change_request_open"}
		}
		var currentDigest string
		var currentRevision int64
		if err = tx.QueryRow(ctx, "SELECT digest,revision FROM worker_workspaces WHERE company_id=$1 AND task_id=$2", scope.company, currentLease.TaskID).Scan(&currentDigest, &currentRevision); err != nil {
			return Receipt{}, err
		}
		if currentDigest != input.BaseWorkspaceDigest || currentRevision != input.BaseWorkspaceRevision {
			return Receipt{}, core.ConflictError{Reason: "Task workspace changed after the takeover lease was granted", CurrentState: "workspace_revision_changed"}
		}
		treeBinding, treeEntries, err := k.taskTakeoverWorkspaceTreeBindingTX(ctx, tx, scope, missionID, currentLease.TaskID, true)
		if err != nil {
			return Receipt{}, err
		}
		if !sameTaskTakeoverWorkspaceTreeBinding(currentLease.WorkspaceTree, treeBinding) {
			return Receipt{}, core.ConflictError{Reason: "Task workspace tree changed after the takeover lease was granted", CurrentState: "workspace_tree_revision_changed"}
		}
		if treeBinding != nil && !taskTakeoverWorkspaceTreeMatchesLegacy(treeEntries, currentDigest, currentRevision) {
			return Receipt{}, core.ConflictError{Reason: "Task file tree no longer matches the single-file takeover baseline", CurrentState: "workspace_tree_mismatch"}
		}
		var activeSlot bool
		if err = tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM task_takeover_active_slots WHERE company_id=$1 AND task_id=$2 AND lease_id=$3)", scope.company, currentLease.TaskID, leaseID).Scan(&activeSlot); err != nil {
			return Receipt{}, err
		}
		if !activeSlot {
			return Receipt{}, core.ConflictError{Reason: "takeover lease no longer owns the Task handback slot", CurrentState: "lease_not_active"}
		}
		inputID := newID()
		if _, err = tx.Exec(ctx, `INSERT INTO mission_inputs(company_id,input_id,mission_id,revision,request_id,source_kind,display_name,media_type,byte_size,content_digest,state,image_width,image_height,request_fingerprint)
VALUES($1,$2,$3,1,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)`, scope.company, inputID, missionID, input.RequestID, prepared.SourceKind, prepared.DisplayName, prepared.MediaType, prepared.ByteSize, prepared.ContentDigest, string(prepared.State), prepared.ImageWidth, prepared.ImageHeight, requestFingerprint); err != nil {
			return Receipt{}, err
		}
		var humanEffortSeconds *int32
		if input.HumanEffortSeconds > 0 {
			seconds := int32(input.HumanEffortSeconds)
			humanEffortSeconds = &seconds
		}
		diffJSON, err := json.Marshal(diffSummary)
		if err != nil {
			return Receipt{}, err
		}
		if err = appendTaskTakeoverLeaseEvent(ctx, tx, scope, leaseID, "returned", "snapshot_base_verified", input.RequestID,
			&inputID, int64Pointer(1), &digest, int64Pointer(int64(len(content))), humanEffortSeconds, diffJSON); err != nil {
			return Receipt{}, err
		}
		if _, err = tx.Exec(ctx, "DELETE FROM task_takeover_active_slots WHERE company_id=$1 AND task_id=$2 AND lease_id=$3", scope.company, currentLease.TaskID, leaseID); err != nil {
			return Receipt{}, err
		}
		if err = appendEvent(ctx, tx, scope, "mission.input.human_handover", map[string]any{
			"mission_id": missionID, "task_id": currentLease.TaskID, "lease_id": leaseID, "input_id": inputID, "revision": 1, "content_digest": digest,
		}); err != nil {
			return Receipt{}, err
		}
		if err = appendEvent(ctx, tx, scope, "task.takeover.returned", map[string]any{
			"mission_id": missionID, "task_id": currentLease.TaskID, "lease_id": leaseID, "base_workspace_digest": input.BaseWorkspaceDigest,
			"base_workspace_revision": input.BaseWorkspaceRevision, "snapshot_digest": digest, "human_effort_seconds": input.HumanEffortSeconds,
		}); err != nil {
			return Receipt{}, err
		}
		return Receipt{ID: leaseID, Status: "returned"}, nil
	})
	if err != nil {
		return TaskTakeoverLease{}, err
	}
	return k.TaskTakeoverLease(ctx, scope, missionID, receipt.ID)
}

func (k *Kernel) TXReleaseTaskTakeoverLease(ctx context.Context, scope Scope, missionID, leaseID, key string) (TaskTakeoverLease, error) {
	if !core.ValidID(missionID) || !core.ValidID(leaseID) || !core.ValidID(key) {
		return TaskTakeoverLease{}, core.Malformed
	}
	receipt, err := k.TXWrite(ctx, scope, nil, key, "task.takeover.released", []string{missionID, leaseID}, func(tx pgx.Tx) (Receipt, error) {
		lease, state, err := taskTakeoverLeaseForUpdate(ctx, tx, scope, missionID, leaseID)
		if err != nil {
			return Receipt{}, err
		}
		if state != "granted" {
			return Receipt{}, core.ConflictError{Reason: "only an active takeover lease can be released without a snapshot", CurrentState: state}
		}
		basis, err := missionChangeBasisTx(ctx, tx, scope, missionID, true)
		if err != nil {
			return Receipt{}, err
		}
		if basis.State != "paused" {
			return Receipt{}, core.ConflictError{Reason: "Mission resumed before the takeover lease was released", CurrentState: basis.State}
		}
		baseRequirementsDigest, err := missionChangeRequirementsDigest(missionID, basis.Title, basis.Goal, basis.AcceptanceContract, basis.InputRevisions)
		if err != nil {
			return Receipt{}, err
		}
		impact, _, err := missionChangeImpactTx(ctx, tx, scope, missionID, baseRequirementsDigest, basis.InputRevisions)
		if err != nil {
			return Receipt{}, err
		}
		if !missionChangeWritersStopped(impact) {
			return Receipt{}, core.ConflictError{Reason: "a WorkerSession, JobRun or service lease became active before the takeover lease was released", CurrentState: "writers_not_stopped"}
		}
		if _, err = tx.Exec(ctx, "DELETE FROM task_takeover_active_slots WHERE company_id=$1 AND task_id=$2 AND lease_id=$3", scope.company, lease.TaskID, leaseID); err != nil {
			return Receipt{}, err
		}
		if err = appendTaskTakeoverLeaseEvent(ctx, tx, scope, leaseID, "released", "operator_returned_no_snapshot", key, nil, nil, nil, nil, nil, nil); err != nil {
			return Receipt{}, err
		}
		if err = appendEvent(ctx, tx, scope, "task.takeover.released", map[string]any{"mission_id": missionID, "task_id": lease.TaskID, "lease_id": leaseID}); err != nil {
			return Receipt{}, err
		}
		return Receipt{ID: leaseID, Status: "released"}, nil
	})
	if err != nil {
		return TaskTakeoverLease{}, err
	}
	return k.TaskTakeoverLease(ctx, scope, missionID, receipt.ID)
}

func (k *Kernel) TaskTakeoverLease(ctx context.Context, scope Scope, missionID, leaseID string) (TaskTakeoverLease, error) {
	if !core.ValidID(missionID) || !core.ValidID(leaseID) {
		return TaskTakeoverLease{}, core.Malformed
	}
	var out TaskTakeoverLease
	var snapshotInputID, snapshotDigest *string
	var snapshotRevision, snapshotBytes pgtype.Int8
	var humanEffortSeconds pgtype.Int4
	var diffJSON []byte
	var createdAt time.Time
	var treeRoot, treeManifest pgtype.Text
	var treeRevision, treeBytes pgtype.Int8
	var treeFileCount pgtype.Int4
	err := k.pool.QueryRow(ctx, `SELECT l.lease_id,l.mission_id,l.task_id,l.client_request_id,l.base_requirements_sha256,l.base_workspace_digest,l.base_workspace_revision,l.created_at,
 l.base_tree_root_id,l.base_tree_revision,l.base_tree_manifest_sha256,l.base_tree_file_count,l.base_tree_bytes,
latest.state,latest.snapshot_input_id,latest.snapshot_revision,latest.snapshot_digest,latest.snapshot_bytes,latest.human_effort_seconds,latest.diff_summary
FROM task_takeover_leases l
JOIN LATERAL (SELECT state,snapshot_input_id,snapshot_revision,snapshot_digest,snapshot_bytes,human_effort_seconds,diff_summary
 FROM task_takeover_lease_events e WHERE e.company_id=l.company_id AND e.lease_id=l.lease_id ORDER BY event_seq DESC LIMIT 1) latest ON true
WHERE l.company_id=$1 AND l.mission_id=$2 AND l.lease_id=$3`, scope.company, missionID, leaseID).Scan(
		&out.LeaseID, &out.MissionID, &out.TaskID, &out.ClientRequestID, &out.BaseRequirementsSHA256, &out.BaseWorkspaceDigest, &out.BaseWorkspaceRevision, &createdAt,
		&treeRoot, &treeRevision, &treeManifest, &treeFileCount, &treeBytes,
		&out.State, &snapshotInputID, &snapshotRevision, &snapshotDigest, &snapshotBytes, &humanEffortSeconds, &diffJSON)
	if errors.Is(err, pgx.ErrNoRows) {
		return TaskTakeoverLease{}, core.OutOfScope
	}
	if err != nil {
		return TaskTakeoverLease{}, err
	}
	var validTreeBinding bool
	out.WorkspaceTree, validTreeBinding = taskTakeoverWorkspaceTreeBindingFromDB(treeRoot, treeManifest, treeRevision, treeFileCount, treeBytes)
	if !validTreeBinding {
		return TaskTakeoverLease{}, core.Integrity
	}
	out.SnapshotInputID = snapshotInputID
	if snapshotRevision.Valid {
		out.SnapshotRevision = int64Pointer(snapshotRevision.Int64)
	}
	out.SnapshotDigest = snapshotDigest
	if snapshotBytes.Valid {
		out.SnapshotBytes = int64Pointer(snapshotBytes.Int64)
	}
	if humanEffortSeconds.Valid {
		seconds := int64(humanEffortSeconds.Int32)
		out.HumanEffortSeconds = &seconds
	}
	if len(diffJSON) > 2 {
		var diff TaskTakeoverDiffSummary
		if json.Unmarshal(diffJSON, &diff) != nil {
			return TaskTakeoverLease{}, core.Integrity
		}
		out.DiffSummary = &diff
	}
	out.CreatedAt = createdAt.UTC().Format(time.RFC3339Nano)
	out.Events, err = k.taskTakeoverLeaseEvents(ctx, scope, leaseID)
	if err != nil {
		return TaskTakeoverLease{}, err
	}
	return out, nil
}

func taskTakeoverWorkspaceTreeBindingFromDB(root, manifest pgtype.Text, revision pgtype.Int8, count pgtype.Int4, bytes pgtype.Int8) (*TaskTakeoverWorkspaceTreeBinding, bool) {
	if !root.Valid && !manifest.Valid && !revision.Valid && !count.Valid && !bytes.Valid {
		return nil, true
	}
	if !root.Valid || !manifest.Valid || !revision.Valid || !count.Valid || !bytes.Valid || root.String == "" || revision.Int64 < 1 || !validSHA256(manifest.String) || count.Int32 < 1 || bytes.Int64 < 1 {
		return nil, false
	}
	return &TaskTakeoverWorkspaceTreeBinding{RootBindingID: root.String, Revision: revision.Int64, ManifestSHA256: manifest.String, FileCount: count.Int32, Bytes: bytes.Int64}, true
}

func (k *Kernel) TaskTakeoverLeases(ctx context.Context, scope Scope, missionID string) ([]TaskTakeoverLease, error) {
	if !core.ValidID(missionID) {
		return nil, core.Malformed
	}
	rows, err := k.pool.Query(ctx, `SELECT lease_id FROM task_takeover_leases WHERE company_id=$1 AND mission_id=$2 ORDER BY created_at DESC,lease_id DESC LIMIT 50`, scope.company, missionID)
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, 8)
	for rows.Next() {
		var leaseID string
		if err = rows.Scan(&leaseID); err != nil {
			rows.Close()
			return nil, err
		}
		ids = append(ids, leaseID)
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()
	out := make([]TaskTakeoverLease, 0, len(ids))
	for _, leaseID := range ids {
		lease, readErr := k.TaskTakeoverLease(ctx, scope, missionID, leaseID)
		if readErr != nil {
			return nil, readErr
		}
		out = append(out, lease)
	}
	return out, nil
}

func (k *Kernel) TaskTakeoverWorkspaceManifest(ctx context.Context, scope Scope, missionID, leaseID string) (TaskTakeoverWorkspaceManifest, error) {
	var out TaskTakeoverWorkspaceManifest
	if !core.ValidID(missionID) || !core.ValidID(leaseID) {
		return out, core.Malformed
	}
	tx, err := k.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead})
	if err != nil {
		return out, err
	}
	defer tx.Rollback(ctx)
	lease, entries, err := k.taskTakeoverWorkspaceReadBaselineTX(ctx, tx, scope, missionID, leaseID)
	if err != nil {
		return out, err
	}
	out = TaskTakeoverWorkspaceManifest{LeaseID: leaseID, MissionID: missionID, TaskID: lease.TaskID, WorkspaceTree: *lease.WorkspaceTree, Entries: entries}
	if err = appendEvent(ctx, tx, scope, "task.takeover.workspace.manifest.read", map[string]any{
		"lease_id": leaseID, "mission_id": missionID, "task_id": lease.TaskID, "root_binding_id": lease.WorkspaceTree.RootBindingID,
		"workspace_revision": lease.WorkspaceTree.Revision, "manifest_sha256": lease.WorkspaceTree.ManifestSHA256,
		"file_count": lease.WorkspaceTree.FileCount, "bytes": lease.WorkspaceTree.Bytes,
	}); err != nil {
		return TaskTakeoverWorkspaceManifest{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return TaskTakeoverWorkspaceManifest{}, err
	}
	return out, nil
}

func (k *Kernel) ReadTaskTakeoverWorkspaceFile(ctx context.Context, scope Scope, missionID, leaseID, relativePath string) (TaskTakeoverWorkspaceFile, error) {
	var out TaskTakeoverWorkspaceFile
	if !core.ValidID(missionID) || !core.ValidID(leaseID) || !ValidWorkspaceRelativePath(relativePath) {
		return out, core.Malformed
	}
	tx, err := k.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead})
	if err != nil {
		return out, err
	}
	defer tx.Rollback(ctx)
	lease, entries, err := k.taskTakeoverWorkspaceReadBaselineTX(ctx, tx, scope, missionID, leaseID)
	if err != nil {
		return out, err
	}
	var selected *WorkspaceTreeEntry
	for index := range entries {
		if entries[index].RelativePath == relativePath {
			selected = &entries[index]
			break
		}
	}
	if selected == nil {
		return out, core.OutOfScope
	}
	content, err := readBlobBounded(k.root, scope.company, selected.Digest, workspaceTreeMaxFileBytes)
	if err != nil || int64(len(content)) != selected.Bytes || !utf8.Valid(content) {
		return out, core.Integrity
	}
	out = TaskTakeoverWorkspaceFile{LeaseID: leaseID, MissionID: missionID, TaskID: lease.TaskID, ManifestSHA256: lease.WorkspaceTree.ManifestSHA256,
		RelativePath: selected.RelativePath, Digest: selected.Digest, Bytes: selected.Bytes, FileRevision: selected.FileRevision, Revision: lease.WorkspaceTree.Revision,
		ContentType: selected.ContentType, Content: string(content)}
	if err = appendEvent(ctx, tx, scope, "task.takeover.workspace.file.read", map[string]any{
		"lease_id": leaseID, "mission_id": missionID, "task_id": lease.TaskID, "root_binding_id": lease.WorkspaceTree.RootBindingID,
		"workspace_revision": lease.WorkspaceTree.Revision, "manifest_sha256": lease.WorkspaceTree.ManifestSHA256,
		"relative_path": selected.RelativePath, "sha256": selected.Digest, "bytes": selected.Bytes,
	}); err != nil {
		return TaskTakeoverWorkspaceFile{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return TaskTakeoverWorkspaceFile{}, err
	}
	return out, nil
}

func (k *Kernel) taskTakeoverWorkspaceReadBaselineTX(ctx context.Context, tx pgx.Tx, scope Scope, missionID, leaseID string) (TaskTakeoverLease, []WorkspaceTreeEntry, error) {
	if err := k.guardWithSessionMode(ctx, tx, scope, nil, false); err != nil {
		return TaskTakeoverLease{}, nil, err
	}
	var missionState string
	if err := tx.QueryRow(ctx, "SELECT state FROM missions WHERE company_id=$1 AND id=$2 FOR SHARE", scope.company, missionID).Scan(&missionState); errors.Is(err, pgx.ErrNoRows) {
		return TaskTakeoverLease{}, nil, core.OutOfScope
	} else if err != nil {
		return TaskTakeoverLease{}, nil, err
	}
	if missionState != "paused" {
		return TaskTakeoverLease{}, nil, core.ConflictError{Reason: "Task takeover workspace reads require a paused Mission", CurrentState: missionState}
	}
	lease, state, err := taskTakeoverLeaseForUpdate(ctx, tx, scope, missionID, leaseID)
	if err != nil {
		return TaskTakeoverLease{}, nil, err
	}
	if state != "granted" {
		return TaskTakeoverLease{}, nil, core.ConflictError{Reason: "Task takeover workspace reads require an active lease", CurrentState: state}
	}
	if lease.WorkspaceTree == nil {
		return TaskTakeoverLease{}, nil, core.ConflictError{Reason: "Task takeover lease has no frozen Schema 103 workspace tree binding", CurrentState: "workspace_tree_unbound"}
	}
	var active bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM task_takeover_active_slots WHERE company_id=$1 AND task_id=$2 AND lease_id=$3 AND mission_id=$4)`,
		scope.company, lease.TaskID, leaseID, missionID).Scan(&active); err != nil {
		return TaskTakeoverLease{}, nil, err
	}
	if !active {
		return TaskTakeoverLease{}, nil, core.ConflictError{Reason: "Task takeover lease no longer owns its Task slot", CurrentState: "lease_not_active"}
	}
	current, entries, err := k.taskTakeoverWorkspaceTreeBindingTX(ctx, tx, scope, missionID, lease.TaskID, false)
	if err != nil {
		return TaskTakeoverLease{}, nil, err
	}
	if !sameTaskTakeoverWorkspaceTreeBinding(lease.WorkspaceTree, current) {
		return TaskTakeoverLease{}, nil, core.ConflictError{Reason: "Task workspace no longer matches the lease's frozen manifest", CurrentState: "workspace_tree_revision_changed"}
	}
	return lease, entries, nil
}

// taskTakeoverWorkspaceTreeBindingTX builds the canonical, bounded manifest
// fingerprint while holding the root row lock. Workspace tree writers update
// that row in the same transaction, so a lease cannot pin a mixed revision.
func (k *Kernel) taskTakeoverWorkspaceTreeBindingTX(ctx context.Context, tx pgx.Tx, scope Scope, missionID, taskID string, verifyCAS bool) (*TaskTakeoverWorkspaceTreeBinding, []WorkspaceTreeEntry, error) {
	var rootID, owner, class, rootMission, taskOwner, taskMission string
	var revision int64
	err := tx.QueryRow(ctx, `SELECT r.id,r.owner,r.read_write_class,r.mission_id,r.revision,t.owner,t.mission_id
FROM worker_workspace_roots r JOIN tasks t ON t.company_id=r.company_id AND t.id=r.task_id
WHERE r.company_id=$1 AND r.task_id=$2 FOR UPDATE OF r`, scope.company, taskID).Scan(&rootID, &owner, &class, &rootMission, &revision, &taskOwner, &taskMission)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, err
	}
	if class != "task_private" || owner != taskOwner || rootMission != missionID || taskMission != missionID || revision < 1 {
		return nil, nil, core.ConflictError{Reason: "Task workspace tree is not private to this Mission and Task owner", CurrentState: "workspace_tree_scope_mismatch"}
	}
	rows, err := tx.Query(ctx, `SELECT relative_path,digest,bytes,file_revision,source_revision,content_type FROM worker_workspace_files
WHERE company_id=$1 AND workspace_id=$2 ORDER BY relative_path COLLATE "C"`, scope.company, rootID)
	if err != nil {
		return nil, nil, err
	}
	entries := make([]WorkspaceTreeEntry, 0, 16)
	var total int64
	for rows.Next() {
		var entry WorkspaceTreeEntry
		if err = rows.Scan(&entry.RelativePath, &entry.Digest, &entry.Bytes, &entry.FileRevision, &entry.SourceRevision, &entry.ContentType); err != nil {
			rows.Close()
			return nil, nil, err
		}
		if !ValidWorkspaceRelativePath(entry.RelativePath) || !validSHA256(entry.Digest) || entry.Bytes < 1 || entry.Bytes > workspaceTreeMaxFileBytes ||
			entry.FileRevision < 1 || entry.SourceRevision < 1 || entry.ContentType != "text/utf-8" {
			rows.Close()
			return nil, nil, core.Integrity
		}
		total += entry.Bytes
		if total > workspaceTreeMaxTotalBytes || len(entries) >= workspaceTreeMaxFiles {
			rows.Close()
			return nil, nil, core.TooLarge
		}
		entries = append(entries, entry)
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return nil, nil, err
	}
	if len(entries) == 0 || !validWorkspaceTreeEntrySequence(entries) {
		return nil, nil, core.ConflictError{Reason: "Task workspace tree is empty or has a noncanonical manifest", CurrentState: "workspace_tree_invalid"}
	}
	manifest := workspaceTreeManifest{Version: "polis-workspace-snapshot@1", CompanyID: scope.company, MissionID: missionID, TaskID: taskID, RootBindingID: rootID, Revision: revision, Entries: entries}
	manifestBytes, err := json.Marshal(manifest)
	if err != nil {
		return nil, nil, err
	}
	if len(manifestBytes) > 8<<20 {
		return nil, nil, core.TooLarge
	}
	if verifyCAS {
		for _, entry := range entries {
			content, readErr := readBlobBounded(k.root, scope.company, entry.Digest, workspaceTreeMaxFileBytes)
			if readErr != nil || int64(len(content)) != entry.Bytes || !utf8.Valid(content) {
				return nil, nil, core.Integrity
			}
		}
	}
	digest := sha256.Sum256(manifestBytes)
	return &TaskTakeoverWorkspaceTreeBinding{RootBindingID: rootID, Revision: revision, ManifestSHA256: hex.EncodeToString(digest[:]), FileCount: int32(len(entries)), Bytes: total}, entries, nil
}

func taskTakeoverWorkspaceTreeMatchesLegacy(entries []WorkspaceTreeEntry, digest string, revision int64) bool {
	return len(entries) == 1 && entries[0].RelativePath == "workspace.txt" && entries[0].Digest == digest && entries[0].SourceRevision == revision
}

func sameTaskTakeoverWorkspaceTreeBinding(pinned, current *TaskTakeoverWorkspaceTreeBinding) bool {
	if pinned == nil || current == nil {
		return pinned == nil && current == nil
	}
	return *pinned == *current
}

func taskTakeoverLeaseForUpdate(ctx context.Context, tx pgx.Tx, scope Scope, missionID, leaseID string) (TaskTakeoverLease, string, error) {
	var lease TaskTakeoverLease
	var state string
	var treeRoot, treeManifest pgtype.Text
	var treeRevision, treeBytes pgtype.Int8
	var treeFileCount pgtype.Int4
	err := tx.QueryRow(ctx, `SELECT l.lease_id,l.mission_id,l.task_id,l.client_request_id,l.base_requirements_sha256,l.base_workspace_digest,l.base_workspace_revision,
l.base_tree_root_id,l.base_tree_revision,l.base_tree_manifest_sha256,l.base_tree_file_count,l.base_tree_bytes,latest.state
FROM task_takeover_leases l
JOIN LATERAL (SELECT state FROM task_takeover_lease_events e WHERE e.company_id=l.company_id AND e.lease_id=l.lease_id ORDER BY event_seq DESC LIMIT 1) latest ON true
WHERE l.company_id=$1 AND l.mission_id=$2 AND l.lease_id=$3 FOR UPDATE OF l`, scope.company, missionID, leaseID).Scan(
		&lease.LeaseID, &lease.MissionID, &lease.TaskID, &lease.ClientRequestID, &lease.BaseRequirementsSHA256, &lease.BaseWorkspaceDigest, &lease.BaseWorkspaceRevision,
		&treeRoot, &treeRevision, &treeManifest, &treeFileCount, &treeBytes, &state)
	if errors.Is(err, pgx.ErrNoRows) {
		return TaskTakeoverLease{}, "", core.OutOfScope
	}
	if err != nil {
		return TaskTakeoverLease{}, "", err
	}
	var validTreeBinding bool
	lease.WorkspaceTree, validTreeBinding = taskTakeoverWorkspaceTreeBindingFromDB(treeRoot, treeManifest, treeRevision, treeFileCount, treeBytes)
	if !validTreeBinding {
		return TaskTakeoverLease{}, "", core.Integrity
	}
	return lease, state, nil
}

func appendTaskTakeoverLeaseEvent(ctx context.Context, tx pgx.Tx, scope Scope, leaseID, state, reasonCode, requestID string,
	snapshotInputID *string, snapshotRevision *int64, snapshotDigest *string, snapshotBytes *int64, humanEffortSeconds *int32, diffJSON []byte) error {
	if len(diffJSON) == 0 {
		diffJSON = []byte(`{}`)
	}
	var inputRevision any
	if snapshotRevision != nil {
		inputRevision = *snapshotRevision
	}
	_, err := tx.Exec(ctx, `INSERT INTO task_takeover_lease_events(company_id,event_id,lease_id,state,snapshot_input_id,snapshot_revision,snapshot_digest,snapshot_bytes,diff_summary,human_effort_seconds,reason_code,actor,command_request_id)
VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,'local_operator',$12)`, scope.company, newID(), leaseID, state,
		snapshotInputID, inputRevision, snapshotDigest, snapshotBytes, diffJSON, humanEffortSeconds, reasonCode, requestID)
	return err
}

func (k *Kernel) taskTakeoverLeaseEvents(ctx context.Context, scope Scope, leaseID string) ([]TaskTakeoverLeaseEvent, error) {
	rows, err := k.pool.Query(ctx, `SELECT event_id,state,snapshot_input_id,snapshot_revision,snapshot_digest,snapshot_bytes,human_effort_seconds,diff_summary,reason_code,created_at
FROM task_takeover_lease_events WHERE company_id=$1 AND lease_id=$2 ORDER BY event_seq`, scope.company, leaseID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	events := make([]TaskTakeoverLeaseEvent, 0, 3)
	for rows.Next() {
		var event TaskTakeoverLeaseEvent
		var snapshotInputID, snapshotDigest *string
		var snapshotRevision, snapshotBytes pgtype.Int8
		var effort pgtype.Int4
		var diffJSON []byte
		var createdAt time.Time
		if err = rows.Scan(&event.EventID, &event.State, &snapshotInputID, &snapshotRevision, &snapshotDigest, &snapshotBytes, &effort, &diffJSON, &event.ReasonCode, &createdAt); err != nil {
			return nil, err
		}
		event.SnapshotInputID = snapshotInputID
		event.SnapshotDigest = snapshotDigest
		if snapshotRevision.Valid {
			event.SnapshotRevision = int64Pointer(snapshotRevision.Int64)
		}
		if snapshotBytes.Valid {
			event.SnapshotBytes = int64Pointer(snapshotBytes.Int64)
		}
		if effort.Valid {
			seconds := int64(effort.Int32)
			event.HumanEffortSeconds = &seconds
		}
		if len(diffJSON) > 2 {
			var diff TaskTakeoverDiffSummary
			if json.Unmarshal(diffJSON, &diff) != nil {
				return nil, core.Integrity
			}
			event.DiffSummary = &diff
		}
		event.CreatedAt = createdAt.UTC().Format(time.RFC3339Nano)
		events = append(events, event)
	}
	return events, rows.Err()
}

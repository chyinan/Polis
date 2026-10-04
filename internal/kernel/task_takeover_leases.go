// pattern: Imperative Shell
package kernel

import (
	"context"
	"encoding/json"
	"errors"
	"time"

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
		matches, err := taskTakeoverWorkspaceTreeMatchesLegacyTX(ctx, tx, scope, missionID, taskID, baseDigest, baseRevision)
		if err != nil {
			return Receipt{}, err
		}
		if !matches {
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
		if _, err = tx.Exec(ctx, `INSERT INTO task_takeover_leases(company_id,lease_id,mission_id,task_id,client_request_id,base_requirements_sha256,base_workspace_digest,base_workspace_revision,created_by)
VALUES($1,$2,$3,$4,$5,$6,$7,$8,'local_operator')`, scope.company, leaseID, missionID, taskID, key, baseRequirementsDigest, baseDigest, baseRevision); err != nil {
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
		matches, err := taskTakeoverWorkspaceTreeMatchesLegacyTX(ctx, tx, scope, missionID, currentLease.TaskID, currentDigest, currentRevision)
		if err != nil {
			return Receipt{}, err
		}
		if !matches {
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
	err := k.pool.QueryRow(ctx, `SELECT l.lease_id,l.mission_id,l.task_id,l.client_request_id,l.base_requirements_sha256,l.base_workspace_digest,l.base_workspace_revision,l.created_at,
latest.state,latest.snapshot_input_id,latest.snapshot_revision,latest.snapshot_digest,latest.snapshot_bytes,latest.human_effort_seconds,latest.diff_summary
FROM task_takeover_leases l
JOIN LATERAL (SELECT state,snapshot_input_id,snapshot_revision,snapshot_digest,snapshot_bytes,human_effort_seconds,diff_summary
 FROM task_takeover_lease_events e WHERE e.company_id=l.company_id AND e.lease_id=l.lease_id ORDER BY event_seq DESC LIMIT 1) latest ON true
WHERE l.company_id=$1 AND l.mission_id=$2 AND l.lease_id=$3`, scope.company, missionID, leaseID).Scan(
		&out.LeaseID, &out.MissionID, &out.TaskID, &out.ClientRequestID, &out.BaseRequirementsSHA256, &out.BaseWorkspaceDigest, &out.BaseWorkspaceRevision, &createdAt,
		&out.State, &snapshotInputID, &snapshotRevision, &snapshotDigest, &snapshotBytes, &humanEffortSeconds, &diffJSON)
	if errors.Is(err, pgx.ErrNoRows) {
		return TaskTakeoverLease{}, core.OutOfScope
	}
	if err != nil {
		return TaskTakeoverLease{}, err
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

// taskTakeoverWorkspaceTreeMatchesLegacyTX keeps the bounded, legacy human
// handback path aligned with Schema 103's private Task tree. Tasks without a
// tree retain the pre-Schema-103 behavior. Once a tree exists, it must still
// be exactly the workspace.txt represented by worker_workspaces at the same
// source revision; a multi-file or divergent tree cannot be handed back as if
// it were a complete frozen workspace.
func taskTakeoverWorkspaceTreeMatchesLegacyTX(ctx context.Context, tx pgx.Tx, scope Scope, missionID, taskID, digest string, revision int64) (bool, error) {
	var rootExists bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM worker_workspace_roots WHERE company_id=$1 AND task_id=$2)`, scope.company, taskID).Scan(&rootExists); err != nil {
		return false, err
	}
	if !rootExists {
		return true, nil
	}
	var matches bool
	err := tx.QueryRow(ctx, `SELECT r.read_write_class='task_private' AND r.owner=t.owner AND r.mission_id=t.mission_id
	AND r.mission_id=$3 AND t.mission_id=$3
	AND count(f.relative_path)=1
	AND COALESCE(bool_and(f.relative_path='workspace.txt' AND f.digest=$4 AND f.source_revision=$5),false)
FROM worker_workspace_roots r
JOIN tasks t ON t.company_id=r.company_id AND t.id=r.task_id
LEFT JOIN worker_workspace_files f ON f.company_id=r.company_id AND f.workspace_id=r.id
WHERE r.company_id=$1 AND r.task_id=$2
GROUP BY r.id,r.read_write_class,r.owner,r.mission_id,t.owner,t.mission_id`, scope.company, taskID, missionID, digest, revision).Scan(&matches)
	return matches, err
}

func taskTakeoverLeaseForUpdate(ctx context.Context, tx pgx.Tx, scope Scope, missionID, leaseID string) (TaskTakeoverLease, string, error) {
	var lease TaskTakeoverLease
	var state string
	err := tx.QueryRow(ctx, `SELECT l.lease_id,l.mission_id,l.task_id,l.client_request_id,l.base_requirements_sha256,l.base_workspace_digest,l.base_workspace_revision,latest.state
FROM task_takeover_leases l
JOIN LATERAL (SELECT state FROM task_takeover_lease_events e WHERE e.company_id=l.company_id AND e.lease_id=l.lease_id ORDER BY event_seq DESC LIMIT 1) latest ON true
WHERE l.company_id=$1 AND l.mission_id=$2 AND l.lease_id=$3 FOR UPDATE OF l`, scope.company, missionID, leaseID).Scan(
		&lease.LeaseID, &lease.MissionID, &lease.TaskID, &lease.ClientRequestID, &lease.BaseRequirementsSHA256, &lease.BaseWorkspaceDigest, &lease.BaseWorkspaceRevision, &state)
	if errors.Is(err, pgx.ErrNoRows) {
		return TaskTakeoverLease{}, "", core.OutOfScope
	}
	return lease, state, err
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

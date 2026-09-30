// pattern: Imperative Shell
package kernel

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"polis/internal/core"
)

type CrossBackendHandoverInput struct {
	HandoverID                string
	TaskID                    string
	SourceJobID               string
	TargetEnvironmentRevision string
	RequestID                 string
}

func (k *Kernel) HasPendingCrossBackendHandover(ctx context.Context, companyID, missionID string) (bool, error) {
	if !core.ValidID(companyID) || !core.ValidID(missionID) {
		return false, core.Malformed
	}
	var pending bool
	err := k.pool.QueryRow(ctx, `SELECT EXISTS(
SELECT 1 FROM cross_backend_handovers h JOIN missions m ON m.company_id=h.company_id AND m.id=h.mission_id
WHERE h.company_id=$1 AND h.mission_id=$2 AND m.state='paused'
AND NOT EXISTS(SELECT 1 FROM job_runs j WHERE j.company_id=h.company_id AND j.handover_id=h.handover_id)
)`, companyID, missionID).Scan(&pending)
	return pending, err
}

func (k *Kernel) TXCreateCrossBackendHandover(ctx context.Context, companyID string, input CrossBackendHandoverInput) (CrossBackendHandoverRecord, error) {
	if !core.ValidID(companyID) || !core.ValidID(input.TaskID) || !core.ValidID(input.SourceJobID) ||
		!core.ValidID(input.TargetEnvironmentRevision) || !core.ValidID(input.RequestID) ||
		(input.HandoverID != "" && !core.ValidID(input.HandoverID)) {
		return CrossBackendHandoverRecord{}, core.Malformed
	}
	targetVerified, err := k.verifyProjectEnvironmentSource(ctx, companyID, input.TargetEnvironmentRevision)
	if err != nil {
		return CrossBackendHandoverRecord{}, err
	}
	if !targetVerified {
		return CrossBackendHandoverRecord{}, core.Denied
	}
	inputManifest, err := k.TaskInputManifest(ctx, k.LocalScope(companyID), input.TaskID)
	if err != nil {
		if errors.Is(err, core.OutOfScope) {
			return CrossBackendHandoverRecord{}, core.Denied
		}
		return CrossBackendHandoverRecord{}, err
	}
	if input.HandoverID == "" {
		input.HandoverID = stableCapabilityID("handover", companyID, input.RequestID)
	}
	_, err = k.TXWrite(ctx, k.LocalScope(companyID), nil, input.RequestID, "environment.cross_backend.handover", input, func(tx pgx.Tx) (Receipt, error) {
		if err := ensureEnvironmentCompanyWritable(ctx, tx, companyID); err != nil {
			return Receipt{}, err
		}
		var taskMission, taskKind, taskOwner, taskState, missionState string
		if err := tx.QueryRow(ctx, `SELECT t.mission_id,t.kind,t.owner,t.state,m.state
FROM tasks t JOIN missions m ON m.company_id=t.company_id AND m.id=t.mission_id
WHERE t.company_id=$1 AND t.id=$2 FOR UPDATE OF t,m`, companyID, input.TaskID).Scan(&taskMission, &taskKind, &taskOwner, &taskState, &missionState); errors.Is(err, pgx.ErrNoRows) {
			return Receipt{}, core.OutOfScope
		} else if err != nil {
			return Receipt{}, err
		}
		if missionState != "paused" || taskState != "working" || taskKind != string(core.TaskKindCompat) || taskOwner != core.EmployeeBackendID {
			return Receipt{}, core.Denied
		}
		var liveSession bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM worker_sessions s JOIN tasks t ON t.company_id=s.company_id AND t.id=s.task_id
WHERE s.company_id=$1 AND t.mission_id=$2 AND s.state<>'stopped')`, companyID, taskMission).Scan(&liveSession); err != nil {
			return Receipt{}, err
		}
		if liveSession {
			return Receipt{}, core.Denied
		}
		var unsafeMissionWork bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS(
 SELECT 1 FROM job_runs j JOIN tasks t ON t.company_id=j.company_id AND t.id=j.task_id
 JOIN LATERAL (SELECT state FROM job_run_events WHERE company_id=j.company_id AND job_id=j.job_id ORDER BY event_seq DESC LIMIT 1) event ON true
 WHERE j.company_id=$1 AND t.mission_id=$2 AND event.state IN ('accepted','starting','running','outcome_unknown')
) OR EXISTS(
 SELECT 1 FROM project_environment_revisions revision
 JOIN LATERAL (SELECT state FROM environment_preparation_events e JOIN environment_preparation_runs r ON r.company_id=e.company_id AND r.run_id=e.run_id
 WHERE r.company_id=revision.company_id AND r.revision_id=revision.revision_id ORDER BY e.event_seq DESC LIMIT 1) prep ON true
 WHERE revision.company_id=$1 AND revision.mission_id=$2 AND prep.state IN ('accepted','starting','running','outcome_unknown')
) OR EXISTS(
 SELECT 1 FROM job_runs j JOIN tasks t ON t.company_id=j.company_id AND t.id=j.task_id
 JOIN LATERAL (SELECT readiness,lease_expires_at FROM service_endpoint_events WHERE company_id=j.company_id AND job_id=j.job_id ORDER BY generation DESC,event_seq DESC LIMIT 1) endpoint ON true
 WHERE j.company_id=$1 AND t.mission_id=$2 AND j.kind='service' AND endpoint.readiness<>'revoked' AND (endpoint.lease_expires_at IS NULL OR endpoint.lease_expires_at>clock_timestamp())
) OR EXISTS(
 SELECT 1 FROM mission_inputs WHERE company_id=$1 AND mission_id=$2 AND state='uploading'
)`, companyID, taskMission).Scan(&unsafeMissionWork); err != nil {
			return Receipt{}, err
		}
		if unsafeMissionWork {
			return Receipt{}, core.Denied
		}

		var sourceSessionID, sourceIncarnation, sourceEnvID, sourceProfile, sourceState, sourceDigest, sourcePackage, sourceLock string
		if err := tx.QueryRow(ctx, `SELECT j.session_id,s.incarnation,j.environment_revision_id,source.profile_id,event.state,
source.source_revision_sha256,source.package_json_sha256,source.lockfile_sha256
FROM job_runs j
JOIN tasks t ON t.company_id=j.company_id AND t.id=j.task_id
JOIN worker_sessions s ON s.company_id=j.company_id AND s.id=j.session_id
JOIN project_environment_revisions source ON source.company_id=j.company_id AND source.revision_id=j.environment_revision_id
JOIN LATERAL (SELECT state FROM job_run_events WHERE company_id=j.company_id AND job_id=j.job_id ORDER BY event_seq DESC LIMIT 1) event ON true
WHERE j.company_id=$1 AND j.job_id=$2 AND j.task_id=$3 AND t.mission_id=$4
FOR UPDATE OF j,s`, companyID, input.SourceJobID, input.TaskID, taskMission).Scan(
			&sourceSessionID, &sourceIncarnation, &sourceEnvID, &sourceProfile, &sourceState, &sourceDigest, &sourcePackage, &sourceLock); errors.Is(err, pgx.ErrNoRows) {
			return Receipt{}, core.OutOfScope
		} else if err != nil {
			return Receipt{}, err
		}
		if sourceState != "exited" && sourceState != "failed" && sourceState != "cancelled" {
			return Receipt{}, core.Denied
		}
		if sourceIncarnation == "" {
			return Receipt{}, core.Integrity
		}
		var latestJobID, latestState string
		if err := tx.QueryRow(ctx, `SELECT j.job_id,event.state FROM job_runs j
JOIN LATERAL (SELECT state FROM job_run_events WHERE company_id=j.company_id AND job_id=j.job_id ORDER BY event_seq DESC LIMIT 1) event ON true
WHERE j.company_id=$1 AND j.task_id=$2 ORDER BY j.created_at DESC,j.job_id DESC LIMIT 1`, companyID, input.TaskID).Scan(&latestJobID, &latestState); errors.Is(err, pgx.ErrNoRows) {
			return Receipt{}, core.OutOfScope
		} else if err != nil {
			return Receipt{}, err
		}
		if latestJobID != input.SourceJobID || (latestState != "exited" && latestState != "failed" && latestState != "cancelled") {
			return Receipt{}, core.Denied
		}
		var workspaceDigest string
		var workspaceRevision int64
		if err := tx.QueryRow(ctx, `SELECT digest,revision FROM worker_workspaces WHERE company_id=$1 AND task_id=$2`, companyID, input.TaskID).Scan(&workspaceDigest, &workspaceRevision); errors.Is(err, pgx.ErrNoRows) {
			return Receipt{}, core.Denied
		} else if err != nil {
			return Receipt{}, err
		}
		var inputManifestDigest string
		if err := tx.QueryRow(ctx, `SELECT manifest_digest FROM task_input_manifests WHERE company_id=$1 AND task_id=$2 AND mission_id=$3`, companyID, input.TaskID, taskMission).Scan(&inputManifestDigest); errors.Is(err, pgx.ErrNoRows) {
			return Receipt{}, core.Denied
		} else if err != nil {
			return Receipt{}, err
		}
		if inputManifest.MissionID != taskMission || inputManifest.Digest != inputManifestDigest {
			return Receipt{}, core.Integrity
		}

		var targetMission, targetProfile, targetDigest, targetPackage, targetLock, targetPolicyDigest, targetToolchainDigest string
		var targetPolicyManifest []byte
		if err := tx.QueryRow(ctx, `SELECT mission_id,profile_id,source_revision_sha256,package_json_sha256,lockfile_sha256,policy_sha256,policy_manifest,toolchain_sha256
FROM project_environment_revisions WHERE company_id=$1 AND revision_id=$2`, companyID, input.TargetEnvironmentRevision).Scan(
			&targetMission, &targetProfile, &targetDigest, &targetPackage, &targetLock, &targetPolicyDigest, &targetPolicyManifest, &targetToolchainDigest); errors.Is(err, pgx.ErrNoRows) {
			return Receipt{}, core.OutOfScope
		} else if err != nil {
			return Receipt{}, err
		}
		var sourceMission, sourceInputID, sourceProjectRoot, targetInputID, targetProjectRoot string
		var sourceInputRevision, targetInputRevision int64
		if err := tx.QueryRow(ctx, `SELECT mission_id,source_input_id,source_input_revision,project_root_relative FROM project_environment_revisions WHERE company_id=$1 AND revision_id=$2`, companyID, sourceEnvID).
			Scan(&sourceMission, &sourceInputID, &sourceInputRevision, &sourceProjectRoot); err != nil {
			return Receipt{}, err
		}
		if err := tx.QueryRow(ctx, `SELECT source_input_id,source_input_revision,project_root_relative FROM project_environment_revisions WHERE company_id=$1 AND revision_id=$2`, companyID, input.TargetEnvironmentRevision).
			Scan(&targetInputID, &targetInputRevision, &targetProjectRoot); err != nil {
			return Receipt{}, err
		}
		if targetMission != taskMission || sourceMission != taskMission || !supportedCrossBackendHandoverProfiles(sourceProfile, targetProfile) ||
			sourceDigest != targetDigest || sourcePackage != targetPackage || sourceLock != targetLock ||
			sourceInputID == "" || sourceInputID != targetInputID || sourceInputRevision != targetInputRevision || sourceProjectRoot != targetProjectRoot {
			return Receipt{}, core.Denied
		}
		_, canonicalPolicyDigest, policyErr := canonicalEnvironmentPolicyManifest(targetPolicyManifest)
		decision, decisionDigest, err := latestEnvironmentPolicyDecision(ctx, tx, companyID, input.TargetEnvironmentRevision)
		if err != nil {
			return Receipt{}, err
		}
		qualified, err := k.environmentExecutorQualified(ctx, tx, companyID, targetProfile, targetToolchainDigest)
		if err != nil {
			return Receipt{}, err
		}
		preparationState, err := latestReadyEnvironmentRevisionState(ctx, tx, companyID, input.TargetEnvironmentRevision)
		if err != nil {
			return Receipt{}, err
		}
		if policyErr != nil || canonicalPolicyDigest != targetPolicyDigest || decision != "approved" || decisionDigest != targetPolicyDigest || !qualified || preparationState != "ready" {
			return Receipt{}, core.Denied
		}

		record := CrossBackendHandoverRecord{
			CompanyID: companyID, HandoverID: input.HandoverID, MissionID: taskMission, TaskID: input.TaskID,
			SourceJobID: input.SourceJobID, SourceSessionID: sourceSessionID, SourceRuntimeIncarnation: sourceIncarnation,
			SourceEnvironmentRevision: sourceEnvID, SourceProfileID: sourceProfile,
			TargetEnvironmentRevision: input.TargetEnvironmentRevision, TargetProfileID: targetProfile,
			ProjectSourceSHA256: sourceDigest, PackageJSONSHA256: sourcePackage, LockfileSHA256: sourceLock,
			WorkspaceDigest: workspaceDigest, WorkspaceRevision: workspaceRevision, TaskInputManifestSHA256: inputManifestDigest,
			TargetPolicySHA256: targetPolicyDigest, TargetToolchainSHA256: targetToolchainDigest, RequestID: input.RequestID,
		}
		record.RecordSHA256 = crossBackendHandoverRecordDigest(record)
		if _, err := tx.Exec(ctx, `INSERT INTO cross_backend_handovers(company_id,handover_id,mission_id,task_id,source_job_id,source_session_id,source_runtime_incarnation,
source_environment_revision_id,source_profile_id,target_environment_revision_id,target_profile_id,project_source_sha256,package_json_sha256,lockfile_sha256,
workspace_digest,workspace_revision,task_input_manifest_sha256,target_policy_sha256,target_toolchain_sha256,request_id,record_sha256,created_by)
VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,'local-owner')`,
			record.CompanyID, record.HandoverID, record.MissionID, record.TaskID, record.SourceJobID, record.SourceSessionID, record.SourceRuntimeIncarnation,
			record.SourceEnvironmentRevision, record.SourceProfileID, record.TargetEnvironmentRevision, record.TargetProfileID,
			record.ProjectSourceSHA256, record.PackageJSONSHA256, record.LockfileSHA256, record.WorkspaceDigest, record.WorkspaceRevision,
			record.TaskInputManifestSHA256, record.TargetPolicySHA256, record.TargetToolchainSHA256, record.RequestID, record.RecordSHA256); err != nil {
			if isUniqueViolation(err) {
				return Receipt{}, core.Conflict
			}
			return Receipt{}, err
		}
		return Receipt{ID: record.HandoverID, Status: "ready"}, nil
	})
	if err != nil {
		return CrossBackendHandoverRecord{}, err
	}
	return k.GetCrossBackendHandover(ctx, companyID, input.HandoverID)
}

func (k *Kernel) GetCrossBackendHandover(ctx context.Context, companyID, handoverID string) (CrossBackendHandoverRecord, error) {
	if !core.ValidID(companyID) || !core.ValidID(handoverID) {
		return CrossBackendHandoverRecord{}, core.Malformed
	}
	var record CrossBackendHandoverRecord
	err := k.pool.QueryRow(ctx, `SELECT company_id,handover_id,mission_id,task_id,source_job_id,source_session_id,source_runtime_incarnation,
source_environment_revision_id,source_profile_id,target_environment_revision_id,target_profile_id,project_source_sha256,package_json_sha256,lockfile_sha256,
workspace_digest,workspace_revision,task_input_manifest_sha256,target_policy_sha256,target_toolchain_sha256,request_id,record_sha256,created_at::text
FROM cross_backend_handovers WHERE company_id=$1 AND handover_id=$2`, companyID, handoverID).Scan(
		&record.CompanyID, &record.HandoverID, &record.MissionID, &record.TaskID, &record.SourceJobID, &record.SourceSessionID,
		&record.SourceRuntimeIncarnation, &record.SourceEnvironmentRevision, &record.SourceProfileID, &record.TargetEnvironmentRevision,
		&record.TargetProfileID, &record.ProjectSourceSHA256, &record.PackageJSONSHA256, &record.LockfileSHA256,
		&record.WorkspaceDigest, &record.WorkspaceRevision, &record.TaskInputManifestSHA256, &record.TargetPolicySHA256,
		&record.TargetToolchainSHA256, &record.RequestID, &record.RecordSHA256, &record.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return CrossBackendHandoverRecord{}, core.OutOfScope
	}
	if err != nil {
		return CrossBackendHandoverRecord{}, err
	}
	if !VerifyCrossBackendHandoverRecord(record) {
		return CrossBackendHandoverRecord{}, core.Integrity
	}
	return record, nil
}

func (k *Kernel) ListTaskCrossBackendHandovers(ctx context.Context, companyID, taskID string) ([]CrossBackendHandoverRecord, error) {
	if !core.ValidID(companyID) || !core.ValidID(taskID) {
		return nil, core.Malformed
	}
	rows, err := k.pool.Query(ctx, `SELECT handover_id FROM cross_backend_handovers WHERE company_id=$1 AND task_id=$2 ORDER BY created_at DESC,handover_id LIMIT 100`, companyID, taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ids := make([]string, 0)
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	items := make([]CrossBackendHandoverRecord, 0, len(ids))
	for _, id := range ids {
		item, readErr := k.GetCrossBackendHandover(ctx, companyID, id)
		if readErr != nil {
			return nil, readErr
		}
		items = append(items, item)
	}
	return items, nil
}

func validateJobRunBackendTransition(ctx context.Context, tx pgx.Tx, companyID string, input JobRunInput, targetProfile, targetSessionID string) error {
	var unresolvedUnknownJob bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(
SELECT 1 FROM job_runs j
JOIN LATERAL (SELECT state FROM job_run_events WHERE company_id=j.company_id AND job_id=j.job_id ORDER BY event_seq DESC LIMIT 1) event ON true
WHERE j.company_id=$1 AND j.task_id=$2 AND event.state='outcome_unknown'
)`, companyID, input.TaskID).Scan(&unresolvedUnknownJob); err != nil {
		return err
	}
	if unresolvedUnknownJob {
		return core.Denied
	}
	var latestJobID, latestProfile, latestState string
	err := tx.QueryRow(ctx, `SELECT j.job_id,revision.profile_id,event.state FROM job_runs j
JOIN project_environment_revisions revision ON revision.company_id=j.company_id AND revision.revision_id=j.environment_revision_id
JOIN LATERAL (SELECT state FROM job_run_events WHERE company_id=j.company_id AND job_id=j.job_id ORDER BY event_seq DESC LIMIT 1) event ON true
WHERE j.company_id=$1 AND j.task_id=$2 ORDER BY j.created_at DESC,j.job_id DESC LIMIT 1`, companyID, input.TaskID).Scan(&latestJobID, &latestProfile, &latestState)
	if errors.Is(err, pgx.ErrNoRows) {
		if input.HandoverID != "" {
			return core.Denied
		}
		return nil
	}
	if err != nil {
		return err
	}
	if latestProfile == targetProfile {
		if input.HandoverID != "" {
			return core.Denied
		}
		return nil
	}
	if input.HandoverID == "" || (latestState != "exited" && latestState != "failed" && latestState != "cancelled") {
		return core.Denied
	}

	var record CrossBackendHandoverRecord
	err = tx.QueryRow(ctx, `SELECT company_id,handover_id,mission_id,task_id,source_job_id,source_session_id,source_runtime_incarnation,
source_environment_revision_id,source_profile_id,target_environment_revision_id,target_profile_id,project_source_sha256,package_json_sha256,lockfile_sha256,
workspace_digest,workspace_revision,task_input_manifest_sha256,target_policy_sha256,target_toolchain_sha256,request_id,record_sha256,created_at::text
FROM cross_backend_handovers WHERE company_id=$1 AND handover_id=$2 FOR UPDATE`, companyID, input.HandoverID).Scan(
		&record.CompanyID, &record.HandoverID, &record.MissionID, &record.TaskID, &record.SourceJobID, &record.SourceSessionID,
		&record.SourceRuntimeIncarnation, &record.SourceEnvironmentRevision, &record.SourceProfileID, &record.TargetEnvironmentRevision,
		&record.TargetProfileID, &record.ProjectSourceSHA256, &record.PackageJSONSHA256, &record.LockfileSHA256,
		&record.WorkspaceDigest, &record.WorkspaceRevision, &record.TaskInputManifestSHA256, &record.TargetPolicySHA256,
		&record.TargetToolchainSHA256, &record.RequestID, &record.RecordSHA256, &record.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return core.Denied
	}
	if err != nil {
		return err
	}
	if !VerifyCrossBackendHandoverRecord(record) ||
		record.TaskID != input.TaskID || record.SourceJobID != latestJobID || record.TargetEnvironmentRevision != input.EnvironmentRevisionID || record.TargetProfileID != targetProfile {
		return core.Denied
	}
	var missionState, currentWorkspaceDigest, currentManifestDigest, sourceSessionState, sourceIncarnation string
	var currentWorkspaceRevision int64
	if err = tx.QueryRow(ctx, `SELECT m.state,w.digest,w.revision,manifest.manifest_digest
FROM tasks t JOIN missions m ON m.company_id=t.company_id AND m.id=t.mission_id
JOIN worker_workspaces w ON w.company_id=t.company_id AND w.task_id=t.id
JOIN task_input_manifests manifest ON manifest.company_id=t.company_id AND manifest.task_id=t.id
WHERE t.company_id=$1 AND t.id=$2 AND t.mission_id=$3`, companyID, record.TaskID, record.MissionID).Scan(
		&missionState, &currentWorkspaceDigest, &currentWorkspaceRevision, &currentManifestDigest); errors.Is(err, pgx.ErrNoRows) {
		return core.Denied
	} else if err != nil {
		return err
	}
	if missionState != "active" || currentWorkspaceDigest != record.WorkspaceDigest || currentWorkspaceRevision != record.WorkspaceRevision || currentManifestDigest != record.TaskInputManifestSHA256 ||
		targetSessionID == record.SourceSessionID {
		return core.Denied
	}
	var sourceState, sourceSessionID, sourceProfile, sourceDigest, sourcePackage, sourceLock string
	if err = tx.QueryRow(ctx, `SELECT event.state,j.session_id,source.profile_id,source.source_revision_sha256,source.package_json_sha256,source.lockfile_sha256
FROM job_runs j JOIN project_environment_revisions source ON source.company_id=j.company_id AND source.revision_id=j.environment_revision_id
JOIN LATERAL (SELECT state FROM job_run_events WHERE company_id=j.company_id AND job_id=j.job_id ORDER BY event_seq DESC LIMIT 1) event ON true
WHERE j.company_id=$1 AND j.job_id=$2 AND j.task_id=$3`, companyID, record.SourceJobID, record.TaskID).Scan(
		&sourceState, &sourceSessionID, &sourceProfile, &sourceDigest, &sourcePackage, &sourceLock); errors.Is(err, pgx.ErrNoRows) {
		return core.Denied
	} else if err != nil {
		return err
	}
	if sourceState != "exited" && sourceState != "failed" && sourceState != "cancelled" {
		return core.Denied
	}
	if sourceSessionID != record.SourceSessionID || sourceProfile != record.SourceProfileID || sourceDigest != record.ProjectSourceSHA256 ||
		sourcePackage != record.PackageJSONSHA256 || sourceLock != record.LockfileSHA256 {
		return core.Denied
	}
	if err = tx.QueryRow(ctx, "SELECT state,incarnation FROM worker_sessions WHERE company_id=$1 AND id=$2", companyID, record.SourceSessionID).Scan(&sourceSessionState, &sourceIncarnation); err != nil {
		return err
	}
	if sourceSessionState != "stopped" || sourceIncarnation != record.SourceRuntimeIncarnation {
		return core.Denied
	}
	var targetDigest, targetPackage, targetLock, targetPolicy, targetToolchain string
	if err = tx.QueryRow(ctx, `SELECT source_revision_sha256,package_json_sha256,lockfile_sha256,policy_sha256,toolchain_sha256
FROM project_environment_revisions WHERE company_id=$1 AND revision_id=$2`, companyID, record.TargetEnvironmentRevision).Scan(
		&targetDigest, &targetPackage, &targetLock, &targetPolicy, &targetToolchain); err != nil {
		return err
	}
	if targetDigest != record.ProjectSourceSHA256 || targetPackage != record.PackageJSONSHA256 || targetLock != record.LockfileSHA256 ||
		targetPolicy != record.TargetPolicySHA256 || targetToolchain != record.TargetToolchainSHA256 {
		return core.Denied
	}
	var alreadyConsumed bool
	if err = tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM job_runs WHERE company_id=$1 AND handover_id=$2)", companyID, record.HandoverID).Scan(&alreadyConsumed); err != nil {
		return err
	}
	if alreadyConsumed {
		return core.Conflict
	}
	return nil
}

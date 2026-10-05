// pattern: Imperative Shell
package kernel

import (
	"context"
	"database/sql"

	"github.com/jackc/pgx/v5"
	"polis/internal/core"
	"polis/internal/environment"
)

const maxProductEnvironmentStatusRevisions = 50

type ProductEnvironmentRevisionStatus struct {
	RevisionID            string  `json:"revisionId"`
	SourceInputID         string  `json:"sourceInputId"`
	SourceInputRevision   string  `json:"sourceInputRevision"`
	ProfileID             string  `json:"profileId"`
	SourceRevisionSHA256  string  `json:"sourceRevisionSha256"`
	PackageJSONSHA256     string  `json:"packageJsonSha256"`
	LockfileSHA256        string  `json:"lockfileSha256"`
	PolicySHA256          string  `json:"policySha256"`
	ToolchainSHA256       string  `json:"toolchainSha256"`
	PolicyDecision        string  `json:"policyDecision"`
	ExecutorQualification string  `json:"executorQualification"`
	PreparationState      string  `json:"preparationState"`
	PreparationReason     string  `json:"preparationReason"`
	PreparationRunID      *string `json:"preparationRunId,omitempty"`
}

type ProductTaskEnvironmentStatus struct {
	MissionID    string                             `json:"missionId"`
	Environments []ProductEnvironmentRevisionStatus `json:"environments"`
	Truncated    bool                               `json:"truncated"`
}

// ProductTaskEnvironmentStatus returns bounded metadata for the exact Mission
// of the bound Worker Task. It never returns source bytes, workspace files,
// preparation logs, executor fingerprints, or evidence inputs.
func (k *Kernel) ProductTaskEnvironmentStatus(ctx context.Context, binding Binding) (ProductTaskEnvironmentStatus, error) {
	result := ProductTaskEnvironmentStatus{Environments: []ProductEnvironmentRevisionStatus{}}
	if !core.ValidID(binding.scope.company) || !core.ValidID(binding.task) || !core.ValidID(binding.session) {
		return result, core.Malformed
	}
	tx, err := k.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return result, err
	}
	defer tx.Rollback(ctx)
	if _, err = k.checkSession(ctx, tx, binding, false); err != nil {
		return result, err
	}
	var taskID string
	if err = tx.QueryRow(ctx, `SELECT s.task_id,t.mission_id
FROM worker_sessions s JOIN tasks t ON t.company_id=s.company_id AND t.id=s.task_id
WHERE s.company_id=$1 AND s.id=$2`, binding.scope.company, binding.session).Scan(&taskID, &result.MissionID); err != nil {
		return result, err
	}
	if taskID != binding.task {
		return result, core.OutOfScope
	}

	windowsFingerprint := k.executorFingerprints[environment.WindowsNodeNPMProfile]
	linuxFingerprint := k.executorFingerprints[environment.LinuxNodeNPMProfile]
	rows, err := tx.Query(ctx, `SELECT r.revision_id,COALESCE(r.source_input_id,''),COALESCE(r.source_input_revision,0)::text,r.profile_id,
r.source_revision_sha256,r.package_json_sha256,r.lockfile_sha256,r.policy_sha256,r.policy_manifest,r.toolchain_sha256,
CASE WHEN policy.decision='revoked' THEN 'revoked'
     WHEN policy.decision='approved' AND (r.policy_manifest IS NULL OR r.source_input_id IS NULL OR policy.policy_sha256 IS DISTINCT FROM r.policy_sha256) THEN 'revocation_required'
     WHEN r.policy_manifest IS NULL THEN 'unverified'
     WHEN r.source_input_id IS NULL THEN 'source_unverified'
     ELSE COALESCE(policy.decision,'not_approved') END,
CASE WHEN executor.decision='revoked' THEN 'revoked'
     WHEN executor.decision='qualified' AND executor.qualified_until IS NOT NULL AND executor.qualified_until<=clock_timestamp() THEN 'expired'
     WHEN executor.decision='qualified'
       AND executor.executor_fingerprint_sha256=CASE r.profile_id WHEN 'windows-node-npm@1' THEN $4 WHEN 'linux-node-npm@1' THEN $7 ELSE '' END
       AND executor.host_fingerprint_sha256=CASE r.profile_id WHEN 'windows-node-npm@1' THEN $5 WHEN 'linux-node-npm@1' THEN $8 ELSE '' END
       AND executor.isolation_policy_sha256=CASE r.profile_id WHEN 'windows-node-npm@1' THEN $6 WHEN 'linux-node-npm@1' THEN $9 ELSE '' END
       AND executor.isolation_profile=CASE r.profile_id WHEN 'windows-node-npm@1' THEN $2 WHEN 'linux-node-npm@1' THEN $3 ELSE '' END THEN 'qualified'
     WHEN executor.decision='qualified' THEN 'identity_stale'
     ELSE COALESCE(executor.decision,'unqualified') END,
COALESCE(preparation.state,'unprepared'),COALESCE(preparation.reason_code,'none'),preparation.run_id
FROM project_environment_revisions r
LEFT JOIN LATERAL (
 SELECT decision,policy_sha256 FROM environment_policy_events
 WHERE company_id=r.company_id AND revision_id=r.revision_id ORDER BY event_seq DESC LIMIT 1
) policy ON true
LEFT JOIN LATERAL (
 SELECT decision,qualified_until,executor_fingerprint_sha256,host_fingerprint_sha256,isolation_policy_sha256,isolation_profile
 FROM environment_executor_qualification_events
 WHERE company_id=r.company_id AND profile_id=r.profile_id AND toolchain_sha256=r.toolchain_sha256
 ORDER BY event_seq DESC LIMIT 1
) executor ON true
LEFT JOIN LATERAL (
 SELECT run.run_id,event.state,event.reason_code FROM environment_preparation_runs run
 JOIN LATERAL (SELECT state,reason_code FROM environment_preparation_events
   WHERE company_id=run.company_id AND run_id=run.run_id ORDER BY event_seq DESC LIMIT 1) event ON true
 WHERE run.company_id=r.company_id AND run.revision_id=r.revision_id ORDER BY run.created_at DESC LIMIT 1
) preparation ON true
WHERE r.company_id=$1 AND r.mission_id=$10
ORDER BY r.created_at DESC,r.revision_id LIMIT $11`, binding.scope.company,
		environment.WindowsNodeIsolationProfile, environment.LinuxNodeIsolationProfile,
		windowsFingerprint.ExecutorSHA256, windowsFingerprint.HostSHA256, windowsFingerprint.IsolationPolicySHA256,
		linuxFingerprint.ExecutorSHA256, linuxFingerprint.HostSHA256, linuxFingerprint.IsolationPolicySHA256,
		result.MissionID, maxProductEnvironmentStatusRevisions+1)
	if err != nil {
		return result, err
	}
	defer rows.Close()
	for rows.Next() {
		if len(result.Environments) == maxProductEnvironmentStatusRevisions {
			result.Truncated = true
			rows.Close()
			break
		}
		var item ProductEnvironmentRevisionStatus
		var runID sql.NullString
		var policyManifest []byte
		if err = rows.Scan(&item.RevisionID, &item.SourceInputID, &item.SourceInputRevision, &item.ProfileID,
			&item.SourceRevisionSHA256, &item.PackageJSONSHA256, &item.LockfileSHA256, &item.PolicySHA256, &policyManifest, &item.ToolchainSHA256,
			&item.PolicyDecision, &item.ExecutorQualification, &item.PreparationState, &item.PreparationReason, &runID); err != nil {
			return result, err
		}
		policy, _, policyDigest, policyErr := environment.ParseProjectEnvironmentPolicy(policyManifest)
		if policyErr != nil || policyDigest != item.PolicySHA256 || policy.ProfileID != item.ProfileID {
			if item.PolicyDecision != "revoked" {
				if item.PolicyDecision == "approved" {
					item.PolicyDecision = "revocation_required"
				} else {
					item.PolicyDecision = "unverified"
				}
			}
		}
		if runID.Valid {
			item.PreparationRunID = &runID.String
		}
		result.Environments = append(result.Environments, item)
	}
	if err = rows.Err(); err != nil {
		return result, err
	}
	if err = tx.Commit(ctx); err != nil {
		return result, err
	}
	return result, nil
}

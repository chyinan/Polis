// pattern: Imperative Shell
package workbench

import (
	"context"
	"database/sql"
	"encoding/json"

	"polis/internal/core"
	"polis/internal/environment"
	"polis/internal/kernel"
)

type ProjectEnvironmentRevisionView struct {
	CompanyID                 string          `json:"companyId"`
	RevisionID                string          `json:"revisionId"`
	MissionID                 string          `json:"missionId"`
	SourceInputID             string          `json:"sourceInputId"`
	SourceInputRevision       string          `json:"sourceInputRevision"`
	ProjectRootRelative       string          `json:"projectRootRelative"`
	SourceBindingStatus       string          `json:"sourceBindingStatus"`
	ProfileID                 string          `json:"profileId"`
	SourceRevisionSHA256      string          `json:"sourceRevisionSha256"`
	PackageJSONSHA256         string          `json:"packageJsonSha256"`
	LockfileSHA256            string          `json:"lockfileSha256"`
	PolicySHA256              string          `json:"policySha256"`
	PolicyManifest            json.RawMessage `json:"policyManifest"`
	ToolchainSHA256           string          `json:"toolchainSha256"`
	ExecutorFingerprintSHA256 *string         `json:"executorFingerprintSha256"`
	HostFingerprintSHA256     *string         `json:"hostFingerprintSha256"`
	IsolationPolicySHA256     *string         `json:"isolationPolicySha256"`
	ExecutorEvidenceSHA256    *string         `json:"executorEvidenceSha256"`
	ExecutorEvidenceInputID   *string         `json:"executorEvidenceInputId"`
	ExecutorEvidenceRevision  *string         `json:"executorEvidenceInputRevision"`
	PolicyDecision            string          `json:"policyDecision"`
	ExecutorQualification     string          `json:"executorQualification"`
	PreparationState          string          `json:"preparationState"`
	PreparationReason         string          `json:"preparationReason"`
	PreparationRunID          *string         `json:"preparationRunId"`
	CreatedAt                 string          `json:"createdAt"`
}

type ServiceEndpointView struct {
	Generation           string  `json:"generation"`
	BindAddress          string  `json:"bindAddress"`
	Port                 string  `json:"port"`
	Readiness            string  `json:"readiness"`
	SourceRevisionSHA256 string  `json:"sourceRevisionSha256"`
	HealthcheckSHA256    string  `json:"healthcheckSha256"`
	LeaseExpiresAt       *string `json:"leaseExpiresAt"`
}

type JobRunView struct {
	CompanyID             string               `json:"companyId"`
	JobID                 string               `json:"jobId"`
	TaskID                string               `json:"taskId"`
	SessionID             string               `json:"sessionId"`
	EnvironmentRevisionID string               `json:"environmentRevisionId"`
	HandoverID            string               `json:"handoverId"`
	Kind                  string               `json:"kind"`
	ServiceID             string               `json:"serviceId"`
	State                 string               `json:"state"`
	Readiness             string               `json:"readiness"`
	ExitCode              *int32               `json:"exitCode"`
	ReasonCode            string               `json:"reasonCode"`
	StdoutOffset          string               `json:"stdoutOffset"`
	StderrOffset          string               `json:"stderrOffset"`
	StdoutBytes           string               `json:"stdoutBytes"`
	StderrBytes           string               `json:"stderrBytes"`
	LogsTruncated         bool                 `json:"logsTruncated"`
	LogGap                bool                 `json:"logGap"`
	LogManifestSHA256     *string              `json:"logManifestSha256"`
	ServiceEndpoint       *ServiceEndpointView `json:"serviceEndpoint"`
	CreatedAt             string               `json:"createdAt"`
	UpdatedAt             string               `json:"updatedAt"`
}

type EnvironmentJobLifecycleReader interface {
	ListProjectEnvironments(ctx context.Context, companyID string) ([]ProjectEnvironmentRevisionView, error)
	ListTaskJobRuns(ctx context.Context, companyID, taskID string) ([]JobRunView, error)
	ListTaskCrossBackendHandovers(ctx context.Context, companyID, taskID string) ([]kernel.CrossBackendHandoverRecord, error)
}

func (s *PostgresReadStore) ListProjectEnvironments(ctx context.Context, companyID string) ([]ProjectEnvironmentRevisionView, error) {
	if err := validateCompanyID(companyID); err != nil {
		return nil, err
	}
	windowsFingerprint := s.executorFingerprints[environment.WindowsNodeNPMProfile]
	linuxFingerprint := s.executorFingerprints[environment.LinuxNodeNPMProfile]
	rows, err := s.pool.Query(ctx, `SELECT r.revision_id,COALESCE(r.mission_id,''),COALESCE(r.source_input_id,''),COALESCE(r.source_input_revision,0)::text,COALESCE(r.project_root_relative,''),r.profile_id,r.source_revision_sha256,r.package_json_sha256,r.lockfile_sha256,r.policy_sha256,r.policy_manifest,r.toolchain_sha256,
NULLIF(CASE r.profile_id WHEN 'windows-node-npm@1' THEN $4 WHEN 'linux-node-npm@1' THEN $7 ELSE '' END,''),NULLIF(CASE r.profile_id WHEN 'windows-node-npm@1' THEN $5 WHEN 'linux-node-npm@1' THEN $8 ELSE '' END,''),NULLIF(CASE r.profile_id WHEN 'windows-node-npm@1' THEN $6 WHEN 'linux-node-npm@1' THEN $9 ELSE '' END,''),
CASE WHEN policy.decision='revoked' THEN 'revoked' WHEN policy.decision='approved' AND (r.policy_manifest IS NULL OR r.source_input_id IS NULL) THEN 'revocation_required' WHEN r.policy_manifest IS NULL THEN 'unverified' WHEN r.source_input_id IS NULL THEN 'source_unverified' ELSE COALESCE(policy.decision,'not_approved') END,COALESCE(executor.qualification,'unqualified'),executor.evidence_sha256,executor.evidence_input_id,executor.evidence_input_revision::text,COALESCE(preparation.state,'unprepared'),COALESCE(preparation.reason_code,'none'),preparation.run_id,r.created_at::text
FROM project_environment_revisions r
LEFT JOIN LATERAL (
 SELECT decision,policy_sha256 FROM environment_policy_events
 WHERE company_id=r.company_id AND revision_id=r.revision_id ORDER BY event_seq DESC LIMIT 1
) policy ON true
LEFT JOIN LATERAL (
	 SELECT CASE
       WHEN decision='revoked' THEN 'revoked'
       WHEN decision='qualified' AND qualified_until IS NOT NULL AND qualified_until<=clock_timestamp() THEN 'expired'
       WHEN decision='qualified'
         AND executor_fingerprint_sha256=CASE r.profile_id WHEN 'windows-node-npm@1' THEN $4 WHEN 'linux-node-npm@1' THEN $7 ELSE '' END
         AND host_fingerprint_sha256=CASE r.profile_id WHEN 'windows-node-npm@1' THEN $5 WHEN 'linux-node-npm@1' THEN $8 ELSE '' END
         AND isolation_policy_sha256=CASE r.profile_id WHEN 'windows-node-npm@1' THEN $6 WHEN 'linux-node-npm@1' THEN $9 ELSE '' END
         AND isolation_profile=CASE r.profile_id WHEN 'windows-node-npm@1' THEN $2 WHEN 'linux-node-npm@1' THEN $3 ELSE '' END
         THEN 'qualified'
       WHEN decision='qualified' THEN 'identity_stale'
       ELSE decision END AS qualification,evidence_sha256,evidence_input_id,evidence_input_revision
	FROM environment_executor_qualification_events
	WHERE company_id=r.company_id AND profile_id=r.profile_id AND toolchain_sha256=r.toolchain_sha256 ORDER BY event_seq DESC LIMIT 1
) executor ON true
LEFT JOIN LATERAL (
 SELECT run.run_id,event.state,event.reason_code FROM environment_preparation_runs run
 JOIN LATERAL (SELECT state,reason_code FROM environment_preparation_events WHERE company_id=run.company_id AND run_id=run.run_id ORDER BY event_seq DESC LIMIT 1) event ON true
 WHERE run.company_id=r.company_id AND run.revision_id=r.revision_id ORDER BY run.created_at DESC LIMIT 1
) preparation ON true
WHERE r.company_id=$1 ORDER BY r.created_at DESC,r.revision_id`, companyID,
		environment.WindowsNodeIsolationProfile, environment.LinuxNodeIsolationProfile,
		windowsFingerprint.ExecutorSHA256, windowsFingerprint.HostSHA256, windowsFingerprint.IsolationPolicySHA256,
		linuxFingerprint.ExecutorSHA256, linuxFingerprint.HostSHA256, linuxFingerprint.IsolationPolicySHA256)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]ProjectEnvironmentRevisionView, 0)
	for rows.Next() {
		var item ProjectEnvironmentRevisionView
		var runID, executorFingerprint, hostFingerprint, isolationPolicy, executorEvidenceSHA256, executorEvidenceInputID, executorEvidenceRevision sql.NullString
		var policyManifest []byte
		if err = rows.Scan(&item.RevisionID, &item.MissionID, &item.SourceInputID, &item.SourceInputRevision, &item.ProjectRootRelative, &item.ProfileID, &item.SourceRevisionSHA256, &item.PackageJSONSHA256, &item.LockfileSHA256, &item.PolicySHA256, &policyManifest, &item.ToolchainSHA256, &executorFingerprint, &hostFingerprint, &isolationPolicy, &item.PolicyDecision, &item.ExecutorQualification, &executorEvidenceSHA256, &executorEvidenceInputID, &executorEvidenceRevision, &item.PreparationState, &item.PreparationReason, &runID, &item.CreatedAt); err != nil {
			return nil, err
		}
		if executorFingerprint.Valid {
			item.ExecutorFingerprintSHA256 = &executorFingerprint.String
		}
		if hostFingerprint.Valid {
			item.HostFingerprintSHA256 = &hostFingerprint.String
		}
		if isolationPolicy.Valid {
			item.IsolationPolicySHA256 = &isolationPolicy.String
		}
		if executorEvidenceSHA256.Valid {
			item.ExecutorEvidenceSHA256 = &executorEvidenceSHA256.String
		}
		if executorEvidenceInputID.Valid {
			item.ExecutorEvidenceInputID = &executorEvidenceInputID.String
		}
		if executorEvidenceRevision.Valid {
			item.ExecutorEvidenceRevision = &executorEvidenceRevision.String
		}
		item.SourceBindingStatus = "unverified"
		if item.MissionID != "" && item.SourceInputID != "" && item.SourceInputRevision != "0" && item.ProjectRootRelative != "" {
			item.SourceBindingStatus = "bound"
		}
		item.PolicyManifest = json.RawMessage(policyManifest)
		if runID.Valid {
			item.PreparationRunID = &runID.String
		}
		if len(policyManifest) == 0 {
			if item.PolicyDecision != "revoked" && item.PolicyDecision != "revocation_required" {
				item.PolicyDecision = "unverified"
			}
		} else {
			_, canonical, digest, policyErr := environment.ParseProjectEnvironmentPolicy(policyManifest)
			if policyErr != nil || digest != item.PolicySHA256 {
				item.PolicyManifest = nil
				if item.PolicyDecision != "revoked" {
					if item.PolicyDecision == "approved" {
						item.PolicyDecision = "revocation_required"
					} else {
						item.PolicyDecision = "unverified"
					}
				}
			} else {
				item.PolicyManifest = json.RawMessage(canonical)
			}
		}
		item.CompanyID = companyID
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *PostgresReadStore) ListTaskJobRuns(ctx context.Context, companyID, taskID string) ([]JobRunView, error) {
	if err := validateCompanyID(companyID); err != nil || !core.ValidID(taskID) {
		return nil, core.Malformed
	}
	rows, err := s.pool.Query(ctx, `SELECT j.job_id,j.task_id,j.session_id,j.environment_revision_id,COALESCE(j.handover_id,''),j.kind,COALESCE(j.service_id,''),
COALESCE(event.state,'accepted'),CASE WHEN endpoint.readiness='revoked' OR (endpoint.readiness IS NOT NULL AND (endpoint.lease_expires_at<=clock_timestamp() OR COALESCE(event.state,'accepted') IN ('exited','failed','cancelled','outcome_unknown') OR (j.kind='service' AND (policy_event.decision IS DISTINCT FROM 'approved' OR policy_event.policy_sha256 IS DISTINCT FROM revision.policy_sha256)))) THEN 'unhealthy' ELSE COALESCE(event.readiness,CASE WHEN j.kind='service' THEN 'not_ready' ELSE 'not_applicable' END) END,event.exit_code,
COALESCE(event.reason_code,'job_accepted'),COALESCE(event.stdout_offset,0)::text,COALESCE(event.stderr_offset,0)::text,
COALESCE(event.stdout_bytes,0)::text,COALESCE(event.stderr_bytes,0)::text,COALESCE(event.logs_truncated,false),COALESCE(event.log_gap,false),event.log_manifest_sha256,
endpoint.generation::text,endpoint.bind_address,endpoint.port::text,CASE WHEN endpoint.readiness!='revoked' AND endpoint.readiness IS NOT NULL AND (endpoint.lease_expires_at<=clock_timestamp() OR COALESCE(event.state,'accepted') IN ('exited','failed','cancelled','outcome_unknown') OR (j.kind='service' AND (policy_event.decision IS DISTINCT FROM 'approved' OR policy_event.policy_sha256 IS DISTINCT FROM revision.policy_sha256))) THEN 'unhealthy' ELSE endpoint.readiness END,endpoint.source_revision_sha256,endpoint.healthcheck_sha256,CASE WHEN endpoint.readiness='revoked' THEN NULL ELSE endpoint.lease_expires_at::text END,
j.created_at::text,COALESCE(event.created_at,j.created_at)::text
FROM job_runs j
JOIN project_environment_revisions revision ON revision.company_id=j.company_id AND revision.revision_id=j.environment_revision_id
LEFT JOIN LATERAL (
 SELECT decision,policy_sha256 FROM environment_policy_events WHERE company_id=j.company_id AND revision_id=j.environment_revision_id ORDER BY event_seq DESC LIMIT 1
) policy_event ON j.kind='service'
LEFT JOIN LATERAL (
 SELECT state,readiness,exit_code,reason_code,stdout_offset,stderr_offset,stdout_bytes,stderr_bytes,logs_truncated,log_gap,log_manifest_sha256,created_at
 FROM job_run_events WHERE company_id=j.company_id AND job_id=j.job_id ORDER BY event_seq DESC LIMIT 1
) event ON true
LEFT JOIN LATERAL (
 SELECT generation,bind_address,port,readiness,source_revision_sha256,healthcheck_sha256,lease_expires_at
 FROM service_endpoint_events WHERE company_id=j.company_id AND job_id=j.job_id ORDER BY generation DESC,event_seq DESC LIMIT 1
) endpoint ON j.kind='service'
WHERE j.company_id=$1 AND j.task_id=$2 ORDER BY j.created_at DESC,j.job_id DESC LIMIT 300`, companyID, taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]JobRunView, 0)
	for rows.Next() {
		var item JobRunView
		var exitCode sql.NullInt32
		var logDigest, endpointGeneration, endpointAddress, endpointPort, endpointReadiness, endpointSource, endpointHealth, endpointExpiry sql.NullString
		if err = rows.Scan(&item.JobID, &item.TaskID, &item.SessionID, &item.EnvironmentRevisionID, &item.HandoverID, &item.Kind, &item.ServiceID, &item.State, &item.Readiness, &exitCode, &item.ReasonCode, &item.StdoutOffset, &item.StderrOffset, &item.StdoutBytes, &item.StderrBytes, &item.LogsTruncated, &item.LogGap, &logDigest, &endpointGeneration, &endpointAddress, &endpointPort, &endpointReadiness, &endpointSource, &endpointHealth, &endpointExpiry, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		if exitCode.Valid {
			item.ExitCode = &exitCode.Int32
		}
		item.CompanyID = companyID
		if logDigest.Valid {
			item.LogManifestSHA256 = &logDigest.String
		}
		if endpointGeneration.Valid {
			item.ServiceEndpoint = &ServiceEndpointView{Generation: endpointGeneration.String, BindAddress: endpointAddress.String, Port: endpointPort.String, Readiness: endpointReadiness.String, SourceRevisionSHA256: endpointSource.String, HealthcheckSHA256: endpointHealth.String}
			if endpointExpiry.Valid {
				item.ServiceEndpoint.LeaseExpiresAt = &endpointExpiry.String
			}
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *PostgresReadStore) ListTaskCrossBackendHandovers(ctx context.Context, companyID, taskID string) ([]kernel.CrossBackendHandoverRecord, error) {
	if err := validateCompanyID(companyID); err != nil || !core.ValidID(taskID) {
		return nil, core.Malformed
	}
	rows, err := s.pool.Query(ctx, `SELECT company_id,handover_id,mission_id,task_id,source_job_id,source_session_id,source_runtime_incarnation,
source_environment_revision_id,source_profile_id,target_environment_revision_id,target_profile_id,project_source_sha256,package_json_sha256,lockfile_sha256,
workspace_digest,workspace_revision,task_input_manifest_sha256,target_policy_sha256,target_toolchain_sha256,request_id,record_sha256,created_at::text
FROM cross_backend_handovers WHERE company_id=$1 AND task_id=$2 ORDER BY created_at DESC,handover_id LIMIT 100`, companyID, taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]kernel.CrossBackendHandoverRecord, 0)
	for rows.Next() {
		var item kernel.CrossBackendHandoverRecord
		if err = rows.Scan(&item.CompanyID, &item.HandoverID, &item.MissionID, &item.TaskID, &item.SourceJobID, &item.SourceSessionID,
			&item.SourceRuntimeIncarnation, &item.SourceEnvironmentRevision, &item.SourceProfileID, &item.TargetEnvironmentRevision,
			&item.TargetProfileID, &item.ProjectSourceSHA256, &item.PackageJSONSHA256, &item.LockfileSHA256,
			&item.WorkspaceDigest, &item.WorkspaceRevision, &item.TaskInputManifestSHA256, &item.TargetPolicySHA256,
			&item.TargetToolchainSHA256, &item.RequestID, &item.RecordSHA256, &item.CreatedAt); err != nil {
			return nil, err
		}
		if !kernel.VerifyCrossBackendHandoverRecord(item) {
			return nil, core.Integrity
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

var _ EnvironmentJobLifecycleReader = (*PostgresReadStore)(nil)

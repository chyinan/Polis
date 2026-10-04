// pattern: Imperative Shell
package kernel

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"

	"github.com/jackc/pgx/v5"
	"polis/internal/core"
)

const maxJobRunLogArtifactBytes = 2 << 20

type JobRunRecord struct {
	CompanyID             string `json:"companyId"`
	JobID                 string `json:"jobId"`
	TaskID                string `json:"taskId"`
	SessionID             string `json:"sessionId"`
	EnvironmentRevisionID string `json:"environmentRevisionId"`
	HandoverID            string `json:"handoverId"`
	Kind                  string `json:"kind"`
	ServiceID             string `json:"serviceId"`
	State                 string `json:"state"`
	Readiness             string `json:"readiness"`
	ExitCode              *int   `json:"exitCode"`
	ReasonCode            string `json:"reasonCode"`
	StdoutOffset          int64  `json:"stdoutOffset"`
	StderrOffset          int64  `json:"stderrOffset"`
	LogsTruncated         bool   `json:"logsTruncated"`
	LogGap                bool   `json:"logGap"`
	LogManifestSHA256     string `json:"logManifestSha256"`
	CreatedAt             string `json:"createdAt"`
}

type JobRunLogArtifact struct {
	CompanyID      string `json:"companyId"`
	JobID          string `json:"jobId"`
	ManifestSHA256 string `json:"manifestSha256"`
	Content        []byte `json:"content"`
}

func (k *Kernel) GetJobRun(ctx context.Context, companyID, jobID string) (JobRunRecord, error) {
	if !core.ValidID(companyID) || !core.ValidID(jobID) {
		return JobRunRecord{}, core.Malformed
	}
	var record JobRunRecord
	err := k.pool.QueryRow(ctx, `SELECT j.company_id,j.job_id,j.task_id,j.session_id,j.environment_revision_id,COALESCE(j.handover_id,''),j.kind,COALESCE(j.service_id,''),
	e.state,e.readiness,e.exit_code,e.reason_code,e.stdout_offset,e.stderr_offset,e.logs_truncated,e.log_gap,COALESCE(e.log_manifest_sha256,''),j.created_at::text
FROM job_runs j
JOIN LATERAL (SELECT state,readiness,exit_code,reason_code,stdout_offset,stderr_offset,logs_truncated,log_gap,log_manifest_sha256
	FROM job_run_events WHERE company_id=j.company_id AND job_id=j.job_id ORDER BY event_seq DESC LIMIT 1) e ON true
WHERE j.company_id=$1 AND j.job_id=$2`, companyID, jobID).Scan(
		&record.CompanyID, &record.JobID, &record.TaskID, &record.SessionID, &record.EnvironmentRevisionID, &record.HandoverID, &record.Kind, &record.ServiceID,
		&record.State, &record.Readiness, &record.ExitCode, &record.ReasonCode, &record.StdoutOffset, &record.StderrOffset,
		&record.LogsTruncated, &record.LogGap, &record.LogManifestSHA256, &record.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return JobRunRecord{}, core.OutOfScope
	}
	return record, err
}

func (k *Kernel) ListMissionJobRuns(ctx context.Context, companyID, missionID string) ([]JobRunRecord, error) {
	if !core.ValidID(companyID) || !core.ValidID(missionID) {
		return nil, core.Malformed
	}
	rows, err := k.pool.Query(ctx, `SELECT j.job_id FROM job_runs j JOIN tasks t ON t.company_id=j.company_id AND t.id=j.task_id
WHERE j.company_id=$1 AND t.mission_id=$2 ORDER BY j.created_at DESC,j.job_id DESC LIMIT 301`, companyID, missionID)
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0)
	for rows.Next() {
		var jobID string
		if err = rows.Scan(&jobID); err != nil {
			rows.Close()
			return nil, err
		}
		ids = append(ids, jobID)
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()
	if len(ids) > 300 {
		return nil, core.Denied
	}
	jobs := make([]JobRunRecord, 0, len(ids))
	for _, jobID := range ids {
		job, readErr := k.GetJobRun(ctx, companyID, jobID)
		if readErr != nil {
			return nil, readErr
		}
		jobs = append(jobs, job)
	}
	return jobs, nil
}

func (k *Kernel) MissionHasOutstandingProjectJobWork(ctx context.Context, companyID, missionID string) (bool, error) {
	if !core.ValidID(companyID) || !core.ValidID(missionID) {
		return false, core.Malformed
	}
	var outstanding bool
	err := k.pool.QueryRow(ctx, `SELECT EXISTS(
 SELECT 1 FROM job_runs j JOIN tasks t ON t.company_id=j.company_id AND t.id=j.task_id
 JOIN LATERAL (SELECT state FROM job_run_events WHERE company_id=j.company_id AND job_id=j.job_id ORDER BY event_seq DESC LIMIT 1) event ON true
 WHERE j.company_id=$1 AND t.mission_id=$2 AND event.state IN ('accepted','starting','running','outcome_unknown')
) OR EXISTS(
 SELECT 1 FROM job_runs j JOIN tasks t ON t.company_id=j.company_id AND t.id=j.task_id
 JOIN LATERAL (SELECT readiness,lease_expires_at FROM service_endpoint_events WHERE company_id=j.company_id AND job_id=j.job_id ORDER BY generation DESC,event_seq DESC LIMIT 1) endpoint ON true
 WHERE j.company_id=$1 AND t.mission_id=$2 AND j.kind='service' AND endpoint.readiness<>'revoked' AND (endpoint.lease_expires_at IS NULL OR endpoint.lease_expires_at>clock_timestamp())
)`, companyID, missionID).Scan(&outstanding)
	return outstanding, err
}

func (k *Kernel) MissionHasOutstandingJobWork(ctx context.Context, companyID, missionID string) (bool, error) {
	if !core.ValidID(companyID) || !core.ValidID(missionID) {
		return false, core.Malformed
	}
	return missionHasOutstandingJobWork(ctx, k.pool, companyID, missionID)
}

type missionWorkQueryer interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}

// missionHasOutstandingJobWork can run against either the pool or a guarded
// lifecycle transaction. Lifecycle transitions use the transactional form so
// admissions cannot slip between an external stop check and the state change.
func missionHasOutstandingJobWork(ctx context.Context, queryer missionWorkQueryer, companyID, missionID string) (bool, error) {
	var outstanding bool
	err := queryer.QueryRow(ctx, `SELECT EXISTS(
 SELECT 1 FROM worker_sessions s JOIN tasks t ON t.company_id=s.company_id AND t.id=s.task_id
 WHERE s.company_id=$1 AND t.mission_id=$2 AND s.state<>'stopped'
) OR EXISTS(
 SELECT 1 FROM job_runs j JOIN tasks t ON t.company_id=j.company_id AND t.id=j.task_id
 JOIN LATERAL (SELECT state FROM job_run_events WHERE company_id=j.company_id AND job_id=j.job_id ORDER BY event_seq DESC LIMIT 1) event ON true
 WHERE j.company_id=$1 AND t.mission_id=$2 AND event.state IN ('accepted','starting','running','outcome_unknown')
) OR EXISTS(
 SELECT 1 FROM job_runs j JOIN tasks t ON t.company_id=j.company_id AND t.id=j.task_id
 JOIN LATERAL (SELECT readiness,lease_expires_at FROM service_endpoint_events WHERE company_id=j.company_id AND job_id=j.job_id ORDER BY generation DESC,event_seq DESC LIMIT 1) endpoint ON true
 WHERE j.company_id=$1 AND t.mission_id=$2 AND j.kind='service' AND endpoint.readiness<>'revoked' AND (endpoint.lease_expires_at IS NULL OR endpoint.lease_expires_at>clock_timestamp())
)`, companyID, missionID).Scan(&outstanding)
	return outstanding, err
}

func (k *Kernel) StoreJobRunLogArtifact(ctx context.Context, companyID, jobID string, content []byte) (string, error) {
	if !core.ValidID(companyID) || !core.ValidID(jobID) || len(content) == 0 || len(content) > maxJobRunLogArtifactBytes {
		return "", core.Malformed
	}
	if _, err := k.GetJobRun(ctx, companyID, jobID); err != nil {
		return "", err
	}
	return k.putBlobWithClaim(ctx, companyID, content)
}

func (k *Kernel) GetJobRunLogArtifact(ctx context.Context, companyID, jobID string) (JobRunLogArtifact, error) {
	record, err := k.GetJobRun(ctx, companyID, jobID)
	if err != nil {
		return JobRunLogArtifact{}, err
	}
	if record.LogManifestSHA256 == "" {
		return JobRunLogArtifact{CompanyID: companyID, JobID: jobID, Content: []byte{}}, nil
	}
	content, err := readBlob(k.root, companyID, record.LogManifestSHA256)
	if err != nil {
		return JobRunLogArtifact{}, err
	}
	digest := sha256.Sum256(content)
	if hex.EncodeToString(digest[:]) != record.LogManifestSHA256 {
		return JobRunLogArtifact{}, core.Integrity
	}
	return JobRunLogArtifact{CompanyID: companyID, JobID: jobID, ManifestSHA256: record.LogManifestSHA256, Content: content}, nil
}

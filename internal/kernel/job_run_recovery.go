// pattern: Imperative Shell
package kernel

import (
	"context"

	"polis/internal/core"
	"polis/internal/environment"
)

type UnrestoredProjectJobRun struct {
	CompanyID string
	JobID     string
	ProfileID string
}

func (k *Kernel) ListUnrestoredProjectJobRuns(ctx context.Context, profileID string) ([]UnrestoredProjectJobRun, error) {
	if _, supported := environment.IsolationProfileForNodeProfile(profileID); !supported {
		return nil, core.Malformed
	}
	rows, err := k.pool.Query(ctx, `SELECT j.company_id,j.job_id,r.profile_id
FROM job_runs j
JOIN companies c ON c.id=j.company_id
JOIN project_environment_revisions r ON r.company_id=j.company_id AND r.revision_id=j.environment_revision_id
JOIN LATERAL (SELECT state FROM job_run_events WHERE company_id=j.company_id AND job_id=j.job_id ORDER BY event_seq DESC LIMIT 1) latest ON true
WHERE c.state='active' AND r.profile_id=$1 AND latest.state='outcome_unknown'
ORDER BY j.company_id,j.job_id`, profileID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]UnrestoredProjectJobRun, 0)
	for rows.Next() {
		var item UnrestoredProjectJobRun
		if err = rows.Scan(&item.CompanyID, &item.JobID, &item.ProfileID); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

// HasUnrestoredLinuxNodeHostWork reports whether Linux Node Jobs or Worker
// cgroups may still belong to a prior control process and need reconciliation.
func (k *Kernel) HasUnrestoredLinuxNodeHostWork(ctx context.Context) (bool, error) {
	var hasWork bool
	err := k.pool.QueryRow(ctx, `SELECT EXISTS(
 SELECT 1 FROM job_runs j
 JOIN project_environment_revisions r ON r.company_id=j.company_id AND r.revision_id=j.environment_revision_id
 JOIN LATERAL (SELECT state FROM job_run_events WHERE company_id=j.company_id AND job_id=j.job_id ORDER BY event_seq DESC LIMIT 1) latest ON true
 WHERE r.profile_id=$1 AND latest.state=ANY($2::text[])
) OR EXISTS(
 SELECT 1 FROM environment_preparation_runs p
 JOIN project_environment_revisions r ON r.company_id=p.company_id AND r.revision_id=p.revision_id
 JOIN LATERAL (SELECT state FROM environment_preparation_events WHERE company_id=p.company_id AND run_id=p.run_id ORDER BY event_seq DESC LIMIT 1) latest ON true
 WHERE r.profile_id=$1 AND latest.state=ANY($2::text[])
) OR EXISTS(
 SELECT 1 FROM worker_sessions s
 JOIN LATERAL (
  SELECT data FROM worker_observations
  WHERE company_id=s.company_id AND session_id=s.id AND reason IN ('process_containment','process_containment_bound')
  ORDER BY CASE reason WHEN 'process_containment_bound' THEN 0 ELSE 1 END,id DESC LIMIT 1
 ) containment ON true
 WHERE s.state<>'stopped' AND containment.data->>'host_os'='linux' AND containment.data->>'profile' IN ('linux_worker_cgroup_v2@1','linux_process_group@1')
)`, environment.LinuxNodeNPMProfile, []string{"accepted", "starting", "running", "outcome_unknown"}).Scan(&hasWork)
	return hasWork, err
}

// TXMarkUnrestoredJobRunsUnknown fences jobs whose process handles belonged to
// a previous control process. It records uncertainty instead of replaying an
// executable side effect after restart.
func (k *Kernel) TXMarkUnrestoredJobRunsUnknown(ctx context.Context, recoveryID string) (int, error) {
	if !core.ValidID(recoveryID) {
		return 0, core.Malformed
	}
	rows, err := k.pool.Query(ctx, `SELECT j.company_id,j.job_id
FROM job_runs j
JOIN companies c ON c.id=j.company_id
JOIN LATERAL (SELECT state FROM job_run_events WHERE company_id=j.company_id AND job_id=j.job_id ORDER BY event_seq DESC LIMIT 1) latest ON true
WHERE c.state='active' AND latest.state=ANY($1::text[])
ORDER BY j.company_id,j.job_id`, []string{
		string(environment.JobAccepted), string(environment.JobStarting), string(environment.JobRunning),
	})
	if err != nil {
		return 0, err
	}
	type activeJob struct{ companyID, jobID string }
	active := make([]activeJob, 0)
	for rows.Next() {
		var job activeJob
		if err = rows.Scan(&job.companyID, &job.jobID); err != nil {
			rows.Close()
			return 0, err
		}
		active = append(active, job)
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		return 0, err
	}
	rows.Close()
	marked := 0
	for _, job := range active {
		record, readErr := k.GetJobRun(ctx, job.companyID, job.jobID)
		if readErr != nil {
			return marked, readErr
		}
		requestID := stableCapabilityID("job-restart", job.companyID, recoveryID+"-"+job.jobID)
		if _, err = k.TXRecordJobRunEvent(ctx, job.companyID, JobRunEventInput{
			JobID: job.jobID, State: string(environment.JobOutcomeUnknown), Readiness: record.Readiness,
			ReasonCode: "job_process_not_restored", StdoutOffset: record.StdoutOffset, StderrOffset: record.StderrOffset,
			LogsTruncated: record.LogsTruncated, LogGap: record.LogGap, RequestID: requestID,
		}); err != nil {
			return marked, err
		}
		marked++
	}
	return marked, nil
}

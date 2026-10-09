// pattern: Imperative Shell
package kernel

import (
	"context"
	"github.com/jackc/pgx/v5"
	"polis/internal/core"
	"polis/internal/gotestprofile"
	"polis/internal/gotestsupervisor"
)

// GoTestJobRequest rehydrates only the already frozen, DB-bound CAS snapshot.
// It never accepts an external path or mutable source directory.
func (k *Kernel) GoTestJobRequest(ctx context.Context, companyID, jobID string) (gotestsupervisor.Request, error) {
	var request gotestsupervisor.Request
	if k == nil || ctx == nil || !core.ValidID(companyID) || !core.ValidID(jobID) {
		return request, core.Malformed
	}
	tx, err := k.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead})
	if err != nil {
		return request, err
	}
	defer tx.Rollback(ctx)
	var raw []byte
	var sha, rootID string
	if err = tx.QueryRow(ctx, `SELECT b.binding,b.binding_sha256,r.id FROM go_test_job_bindings b JOIN worker_workspace_roots r ON r.company_id=b.company_id AND r.task_id=b.task_id WHERE b.company_id=$1 AND b.job_id=$2`, companyID, jobID).Scan(&raw, &sha, &rootID); err != nil {
		return request, err
	}
	b, _, err := gotestprofile.ParseJobBinding(raw, sha)
	if err != nil || b.CompanyID != companyID || b.JobRunID != jobID {
		return request, core.Integrity
	}
	snap, err := k.loadGoTestWorkspaceSnapshotTX(ctx, tx, companyID, b.MissionID, b.TaskID, b.CandidateSnapshotArtifactID, rootID)
	if err != nil {
		return request, err
	}
	if snap.ArtifactSHA256 != b.CandidateSnapshotSHA256 || snap.Manifest.Revision != b.CandidateWorkspaceRevision {
		return request, core.Integrity
	}
	request = gotestsupervisor.Request{Protocol: gotestsupervisor.Protocol, Binding: b, BindingSHA256: sha, Files: snap.Files}
	if gotestsupervisor.ValidateRequest(request) != nil {
		return request, core.Integrity
	}
	return request, tx.Commit(ctx)
}

func (k *Kernel) TXClaimGoTestJobDispatch(ctx context.Context, companyID, jobID, claimID string) (bool, error) {
	if !core.ValidID(jobID) || !core.ValidID(claimID) {
		return false, core.Malformed
	}
	claimed := false
	_, err := k.TXWrite(ctx, k.LocalScope(companyID), nil, "go-dispatch-"+jobID, "go_test.job.dispatch", claimID, func(tx pgx.Tx) (Receipt, error) {
		var revision, expected int64
		var taskState, missionState string
		if err := tx.QueryRow(ctx, `SELECT r.revision,b.candidate_workspace_revision,t.state,m.state FROM go_test_job_bindings b JOIN worker_workspace_roots r ON r.company_id=b.company_id AND r.task_id=b.task_id JOIN tasks t ON t.company_id=b.company_id AND t.id=b.task_id JOIN missions m ON m.company_id=b.company_id AND m.id=b.mission_id WHERE b.company_id=$1 AND b.job_id=$2 FOR UPDATE OF r`, companyID, jobID).Scan(&revision, &expected, &taskState, &missionState); err != nil {
			return Receipt{}, err
		}
		state, _, _, _, err := latestJobRunEvent(ctx, tx, companyID, jobID)
		if err != nil {
			return Receipt{}, err
		}
		if state != "accepted" || revision != expected || taskState != "working" || missionState != "active" {
			return Receipt{}, core.Conflict
		}
		_, err = tx.Exec(ctx, `INSERT INTO job_run_events(company_id,event_id,job_id,state,readiness,reason_code) VALUES($1,$2,$3,'starting','not_applicable','go_dispatch_claimed')`, companyID, stableCapabilityID("go-start", jobID), jobID)
		claimed = err == nil
		return Receipt{ID: jobID, Status: "starting"}, err
	})
	return claimed, err
}

// TXConfirmGoJobStopped is called only after the fixed executor has proved
// the entire named systemd job cgroup empty; it does not fabricate a test exit.
func (k *Kernel) TXConfirmGoJobStopped(ctx context.Context, company, jobID, requestID string) (Receipt, error) {
	return k.TXWrite(ctx, k.LocalScope(company), nil, requestID, "go_test.job.stopped", jobID, func(tx pgx.Tx) (Receipt, error) {
		var kind string
		if err := tx.QueryRow(ctx, `SELECT kind FROM job_runs WHERE company_id=$1 AND job_id=$2`, company, jobID).Scan(&kind); err != nil {
			return Receipt{}, err
		}
		if kind != "go_test" {
			return Receipt{}, core.Denied
		}
		state, _, _, _, err := latestJobRunEvent(ctx, tx, company, jobID)
		if err != nil {
			return Receipt{}, err
		}
		if state != "starting" && state != "running" && state != "outcome_unknown" {
			return Receipt{ID: jobID, Status: state}, nil
		}
		_, err = tx.Exec(ctx, `INSERT INTO job_run_events(company_id,event_id,job_id,state,readiness,reason_code) VALUES($1,$2,$3,'cancelled','not_applicable','go_job_group_stop_confirmed')`, company, stableCapabilityID("go-stopped", jobID), jobID)
		return Receipt{ID: jobID, Status: "cancelled"}, err
	})
}

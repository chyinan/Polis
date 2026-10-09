// pattern: Imperative Shell
package kernel

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/jackc/pgx/v5"
	"polis/internal/core"
	"polis/internal/environment"
	"polis/internal/gotestprofile"
	"polis/internal/gotestsupervisor"
)

// TXRecordGoTestJobReport is an internal control-plane boundary. No public
// endpoint accepts a caller-supplied execution report.
func (k *Kernel) TXRecordGoTestJobReport(ctx context.Context, companyID, requestID string, report gotestsupervisor.Report) (Receipt, error) {
	if k == nil || ctx == nil || !core.ValidID(companyID) || !core.ValidID(requestID) || !core.ValidID(report.JobRunID) {
		return Receipt{}, core.Malformed
	}
	var raw []byte
	var storedDigest string
	if err := k.pool.QueryRow(ctx, "SELECT binding,binding_sha256 FROM go_test_job_bindings WHERE company_id=$1 AND job_id=$2", companyID, report.JobRunID).Scan(&raw, &storedDigest); err != nil {
		return Receipt{}, err
	}
	binding, _, err := gotestprofile.ParseJobBinding(raw, storedDigest)
	if err != nil || gotestsupervisor.ValidateReport(binding, report) != nil {
		return Receipt{}, core.Integrity
	}
	// Logs use the same immutable CAS format and read API as other JobRuns.
	logManifest := struct {
		SchemaVersion string `json:"schemaVersion"`
		CompanyID     string `json:"companyId"`
		JobID         string `json:"jobId"`
		Stdout        []byte `json:"stdout"`
		Stderr        []byte `json:"stderr"`
		StdoutSHA256  string `json:"stdoutSha256"`
		StderrSHA256  string `json:"stderrSha256"`
		LogsTruncated bool   `json:"logsTruncated"`
		LogGap        bool   `json:"logGap"`
	}{
		"project-job-logs@1", companyID, report.JobRunID, report.Stdout.Data, report.Stderr.Data, report.Stdout.SHA256, report.Stderr.SHA256, report.LogsTruncated, false}
	logRaw, err := json.Marshal(logManifest)
	if err != nil {
		return Receipt{}, err
	}
	logDigest, err := k.putBlobWithClaim(ctx, companyID, logRaw)
	if err != nil {
		return Receipt{}, err
	}
	metadata := report
	metadata.Stdout.Data = nil
	metadata.Stderr.Data = nil
	receiptRaw, err := json.Marshal(metadata)
	if err != nil || len(receiptRaw) > 1<<20 {
		return Receipt{}, core.TooLarge
	}
	receiptDigest := casDigest(receiptRaw)
	resultRaw, err := json.Marshal(struct {
		Outcome               string `json:"outcome"`
		Reason                string `json:"reasonCode"`
		ProcessStarted        bool   `json:"processStarted"`
		ExitObserved          bool   `json:"exitObserved"`
		ExitCode              *int   `json:"exitCode,omitempty"`
		CleanupConfirmed      bool   `json:"cleanupConfirmed"`
		ExecutionKind         string `json:"executionKind"`
		LogManifestSHA256     string `json:"logManifestSha256"`
		BindingSHA256         string `json:"bindingSha256"`
		QualificationEligible bool   `json:"qualificationEligible"`
	}{
		report.Outcome, report.Reason, report.ProcessStarted, report.ExitObserved, report.ExitCode, report.CleanupConfirmed, report.ExecutionKind, logDigest, storedDigest, report.QualificationEligible})
	if err != nil {
		return Receipt{}, err
	}
	return k.TXWrite(ctx, k.LocalScope(companyID), nil, requestID, "go_test.job.result", struct{ JobID, BindingSHA256, ReportSHA256 string }{report.JobRunID, storedDigest, receiptDigest}, func(tx pgx.Tx) (Receipt, error) {
		var digest, kind string
		if err := tx.QueryRow(ctx, "SELECT b.binding_sha256,j.kind FROM go_test_job_bindings b JOIN job_runs j ON j.company_id=b.company_id AND j.job_id=b.job_id WHERE b.company_id=$1 AND b.job_id=$2 FOR UPDATE OF j", companyID, report.JobRunID).Scan(&digest, &kind); err != nil {
			return Receipt{}, err
		}
		if digest != storedDigest || kind != "go_test" {
			return Receipt{}, core.Integrity
		}
		var previousReceipt string
		err := tx.QueryRow(ctx, "SELECT runner_receipt_sha256 FROM go_test_job_results WHERE company_id=$1 AND job_id=$2", companyID, report.JobRunID).Scan(&previousReceipt)
		if err == nil {
			if previousReceipt != receiptDigest {
				return Receipt{}, core.Conflict
			}
			return Receipt{ID: report.JobRunID, Status: report.State}, nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return Receipt{}, err
		}
		previous, _, _, _, err := latestJobRunEvent(ctx, tx, companyID, report.JobRunID)
		if err != nil {
			return Receipt{}, err
		}
		if report.ProcessStarted && (previous == string(environment.JobAccepted) || previous == string(environment.JobStarting)) {
			states := []string{string(environment.JobRunning)}
			if previous == string(environment.JobAccepted) {
				states = append([]string{string(environment.JobStarting)}, states...)
			}
			for _, state := range states {
				if _, err = tx.Exec(ctx, "INSERT INTO job_run_events(company_id,event_id,job_id,state,readiness,reason_code) VALUES($1,$2,$3,$4,'not_applicable','supervisor_process_observed')", companyID, stableCapabilityID("go-event", report.JobRunID, state), report.JobRunID, state); err != nil {
					return Receipt{}, err
				}
			}
			previous = string(environment.JobRunning)
		}
		if previous != report.State && !environment.CanTransitionJob(previous, report.State) {
			return Receipt{}, core.Conflict
		}
		if _, err = tx.Exec(ctx, "INSERT INTO go_test_job_results(company_id,job_id,outcome,result,result_sha256,runner_receipt,runner_receipt_sha256) VALUES($1,$2,$3,$4,$5,$6,$7)", companyID, report.JobRunID, report.Outcome, resultRaw, casDigest(resultRaw), receiptRaw, receiptDigest); err != nil {
			return Receipt{}, err
		}
		if previous != report.State {
			_, err = tx.Exec(ctx, "INSERT INTO job_run_events(company_id,event_id,job_id,state,readiness,exit_code,reason_code,stdout_offset,stderr_offset,stdout_bytes,stderr_bytes,log_manifest_sha256,logs_truncated) VALUES($1,$2,$3,$4,'not_applicable',$5,$6,$7::bigint,$8::bigint,$7::integer,$8::integer,$9,$10)", companyID, stableCapabilityID("go-terminal", report.JobRunID), report.JobRunID, report.State, report.ExitCode, report.Reason, len(report.Stdout.Data), len(report.Stderr.Data), logDigest, report.LogsTruncated)
			if err != nil {
				return Receipt{}, err
			}
		}
		return Receipt{ID: report.JobRunID, Status: report.State}, nil
	})
}

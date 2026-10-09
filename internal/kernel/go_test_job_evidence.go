// pattern: Imperative Shell
package kernel

import (
	"context"
	"encoding/json"
	"github.com/jackc/pgx/v5"
	"polis/internal/core"
	"polis/internal/gotestprofile"
	"polis/internal/gotestsupervisor"
)

type goJobEvidenceRecord struct {
	binding   gotestprofile.JobBinding
	report    gotestsupervisor.Report
	resultSHA string
	logSHA    string
}

func (k *Kernel) goJobEvidenceTX(ctx context.Context, tx pgx.Tx, company, job string) (goJobEvidenceRecord, error) {
	var out goJobEvidenceRecord
	var raw, result, receipt []byte
	var bindingSHA, receiptSHA, outcome, state string
	err := tx.QueryRow(ctx, `SELECT b.binding,b.binding_sha256,r.result,r.result_sha256,r.runner_receipt,r.runner_receipt_sha256,r.outcome,event.state FROM go_test_job_bindings b JOIN go_test_job_results r ON r.company_id=b.company_id AND r.job_id=b.job_id JOIN LATERAL (SELECT state FROM job_run_events WHERE company_id=b.company_id AND job_id=b.job_id ORDER BY event_seq DESC LIMIT 1) event ON true WHERE b.company_id=$1 AND b.job_id=$2`, company, job).Scan(&raw, &bindingSHA, &result, &out.resultSHA, &receipt, &receiptSHA, &outcome, &state)
	if err != nil {
		return out, err
	}
	b, _, err := gotestprofile.ParseJobBinding(raw, bindingSHA)
	if err != nil || b.CompanyID != company || b.JobRunID != job || casDigest(result) != out.resultSHA {
		return out, core.Integrity
	}
	var envelope struct {
		Outcome    string `json:"outcome"`
		BindingSHA string `json:"bindingSha256"`
		LogSHA     string `json:"logManifestSha256"`
		Kind       string `json:"executionKind"`
		Qualified  bool   `json:"qualificationEligible"`
	}
	if json.Unmarshal(result, &envelope) != nil || envelope.Outcome != outcome || envelope.BindingSHA != bindingSHA || !validSHA256(envelope.LogSHA) {
		return out, core.Integrity
	}
	logs, err := readBlobBounded(k.root, company, envelope.LogSHA, 2<<20)
	if err != nil {
		return out, err
	}
	r, err := gotestsupervisor.RestoreRecordedReport(b, receipt, receiptSHA, logs, envelope.LogSHA)
	if err != nil || r.State != state || r.Outcome != outcome || r.ExecutionKind != envelope.Kind || r.QualificationEligible != envelope.Qualified {
		return out, core.Integrity
	}
	out.binding = b
	out.report = r
	out.logSHA = envelope.LogSHA
	return out, nil
}

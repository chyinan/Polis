// pattern: Functional Core
package gotestsupervisor

import (
	"encoding/json"
	"polis/internal/gotestprofile"
)

// RestoreRecordedReport verifies the immutable receipt and log references
// before rehydrating bounded bytes. Database labels alone cannot qualify a job.
func RestoreRecordedReport(b gotestprofile.JobBinding, receipt []byte, receiptSHA string, rawLogs []byte, logsSHA string) (Report, error) {
	var r Report
	if len(receipt) > 1<<20 || len(rawLogs) > 2<<20 || SHA256(receipt) != receiptSHA || SHA256(rawLogs) != logsSHA || json.Unmarshal(receipt, &r) != nil {
		return r, ErrInvalid
	}
	var logs struct {
		SchemaVersion string `json:"schemaVersion"`
		CompanyID     string `json:"companyId"`
		JobID         string `json:"jobId"`
		Stdout        []byte `json:"stdout"`
		Stderr        []byte `json:"stderr"`
		StdoutSHA     string `json:"stdoutSha256"`
		StderrSHA     string `json:"stderrSha256"`
		Truncated     bool   `json:"logsTruncated"`
		Gap           bool   `json:"logGap"`
	}
	if json.Unmarshal(rawLogs, &logs) != nil || logs.SchemaVersion != "project-job-logs@1" || logs.CompanyID != b.CompanyID || logs.JobID != b.JobRunID || logs.StdoutSHA != r.Stdout.SHA256 || logs.StderrSHA != r.Stderr.SHA256 || logs.Gap || logs.Truncated != r.LogsTruncated {
		return r, ErrInvalid
	}
	r.Stdout.Data = logs.Stdout
	r.Stderr.Data = logs.Stderr
	if ValidateReport(b, r) != nil {
		return r, ErrInvalid
	}
	return r, nil
}

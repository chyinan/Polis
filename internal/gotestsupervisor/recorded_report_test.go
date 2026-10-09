// pattern: Functional Core
package gotestsupervisor

import (
	"encoding/json"
	"testing"
)

func TestRecordedReportRehydratesOnlyMatchingCASLogs(t *testing.T) {
	b := testBinding()
	r := NewUnstartedReport(b, "real", "blocked", "host_profile_unqualified", HostCheck{})
	raw, _ := json.Marshal(r)
	logs := []byte(`{"schemaVersion":"project-job-logs@1","companyId":"company","jobId":"job-test","stdout":"","stderr":"","stdoutSha256":"` + SHA256(nil) + `","stderrSha256":"` + SHA256(nil) + `","logsTruncated":false,"logGap":false}`)
	got, err := RestoreRecordedReport(b, raw, SHA256(raw), logs, SHA256(logs))
	if err != nil {
		t.Fatal(err)
	}
	if got.QualificationEligible || got.ProcessStarted || got.ExitCode != nil {
		t.Fatal("blocked stored report became execution evidence")
	}
	if _, err = RestoreRecordedReport(b, raw, SHA256(raw), append(logs, ' '), SHA256(logs)); err == nil {
		t.Fatal("CAS log tampering accepted")
	}
	b.CandidateWorkspaceRevision++
	if _, err = RestoreRecordedReport(b, raw, SHA256(raw), logs, SHA256(logs)); err == nil {
		t.Fatal("stored report transferred to another candidate")
	}
}

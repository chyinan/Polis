// pattern: Functional Core
package gotestsupervisor

import (
	"bytes"
	"context"
	"encoding/json"
	"polis/internal/fixture"
	"polis/internal/gotestprofile"
	"strings"
	"testing"
)

func TestRealUnqualifiedHostNeverStartsCandidate(t *testing.T) {
	b := testBinding()
	content := []byte(fixture.Source)
	files := []gotestprofile.SnapshotFile{{RelativePath: "formatter.go", Digest: SHA256(content), Bytes: int64(len(content)), ContentType: "text/utf-8", EntryType: "regular", Content: content}}
	b.CandidateFilesSHA256, _, _ = gotestprofile.ValidateFixtureSnapshotFiles(files)
	_, bindingSHA, _ := gotestprofile.CanonicalizeJobBinding(b)
	r := Request{Protocol: Protocol, Binding: b, BindingSHA256: bindingSHA, Files: files}
	raw, _ := json.Marshal(r)
	parsed, e := ParseRequest(raw)
	if e != nil {
		t.Fatal(e)
	}
	report := Execute(context.Background(), parsed, DefaultConfig())
	if report.ProcessStarted || report.ExitObserved || report.ExitCode != nil || report.TestResult != nil || report.QualificationEligible || report.State != "blocked" {
		t.Fatalf("unqualified host fabricated execution: %+v", report)
	}
	if e = ValidateReport(b, report); e != nil {
		t.Fatal(e)
	}
}

func TestCLISafeCommandsAndInvalidInput(t *testing.T) {
	var out, diagnostics bytes.Buffer
	if code := Main([]string{"--help"}, &out, &diagnostics); code != 0 {
		t.Fatalf("help exit=%d", code)
	}
	if code := Main([]string{"--job-id=../escape"}, &out, &diagnostics); code != 2 {
		t.Fatalf("escaping job id exit=%d", code)
	}
	if code := Main([]string{"--check-input=/definitely-missing-polis-fixture"}, &out, &diagnostics); code != 2 {
		t.Fatalf("missing private request exit=%d", code)
	}
}

func testBinding() gotestprofile.JobBinding {
	p := gotestprofile.FixtureManifest(strings.Repeat("a", 64))
	_, pd, _ := gotestprofile.Canonicalize(p)
	return gotestprofile.JobBinding{SchemaVersion: gotestprofile.JobBindingSchema, JobRunID: "job-test", CompanyID: "company", MissionID: "mission", TaskID: "task", TaskValidationBindingSHA256: strings.Repeat("b", 64), BaseCommitOID: strings.Repeat("c", 40), InputManifestSHA256: strings.Repeat("d", 64), BaseSnapshotArtifactID: "base", BaseSnapshotSHA256: strings.Repeat("e", 64), BaseWorkspaceRevision: 1, BaseFilesSHA256: strings.Repeat("f", 64), CandidateSnapshotArtifactID: "candidate", CandidateSnapshotSHA256: strings.Repeat("0", 64), CandidateWorkspaceRevision: 2, CandidateFilesSHA256: strings.Repeat("1", 64), ProfileSHA256: pd, Profile: p}
}
func TestUnstartedReportCannotClaimExitOrPass(t *testing.T) {
	b := testBinding()
	r := NewUnstartedReport(b, "real", "blocked", "host_profile_unqualified", HostCheck{Reason: "unit_missing"})
	if err := ValidateReport(b, r); err != nil {
		t.Fatal(err)
	}
	zero := 0
	r.ExitCode = &zero
	if ValidateReport(b, r) == nil {
		t.Fatal("no-start report accepted a fabricated zero exit")
	}
	r.ExitCode = nil
	r.Outcome = "passed"
	if ValidateReport(b, r) == nil {
		t.Fatal("no-start report claimed pass")
	}
}

func TestLauncherFailureDoesNotFabricateCandidateExit(t *testing.T) {
	b := testBinding()
	r := NewUnstartedReport(b, "simulation", "failed", "sandbox_initialization_failed", HostCheck{})
	code := 1
	r.LauncherExitCode = &code
	if ValidateReport(b, r) == nil {
		t.Fatal("launcher exit accepted without a launcher")
	}
	r.LauncherStarted, r.LauncherExitObserved = true, true
	if err := ValidateReport(b, r); err != nil {
		t.Fatal(err)
	}
	if r.ExitCode != nil || r.ExitObserved || r.ProcessStarted {
		t.Fatal("launcher-only failure fabricated candidate execution")
	}
}
func TestReportRejectsDifferentCandidateAndLogBytes(t *testing.T) {
	b := testBinding()
	r := NewUnstartedReport(b, "simulation", "failed", "start_failed", HostCheck{})
	r.CandidateRevision++
	if ValidateReport(b, r) == nil {
		t.Fatal("cross-candidate report accepted")
	}
	r.CandidateRevision--
	r.Stdout.Data = []byte("PASS")
	if ValidateReport(b, r) == nil {
		t.Fatal("unbound log accepted")
	}
}

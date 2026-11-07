// pattern: Functional Core
package kernel

import (
	"encoding/json"
	"polis/internal/core"
	"polis/internal/runner"
	"strings"
	"testing"
)

func TestReviewerCheckResultDoesNotExposeVerifierOutput(t *testing.T) {
	view := redactReviewerReport(runner.Report{Passed: true, Output: "HIDDEN_REGRESSION_NAME", Digest: strings.Repeat("a", 64), Phase: "full"})
	if !view.Passed || view.Digest == "" || view.Phase != "full" {
		t.Fatalf("review result lost safe fields: %+v", view)
	}
	raw, err := json.Marshal(view)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "HIDDEN_REGRESSION_NAME") || strings.Contains(string(raw), "output") {
		t.Fatalf("review result exposed hidden verifier output: %s", raw)
	}
}

func TestReviewerToolPolicyDeniesCandidateWrites(t *testing.T) {
	for _, name := range []string{"workspace_replace", "artifact_submit", "unknown"} {
		if reviewerToolAllowed(name) {
			t.Fatalf("reviewer tool policy allowed %q", name)
		}
	}
	for _, name := range []string{"work_current", "context_read", "workspace_read", "review_submit"} {
		if !reviewerToolAllowed(name) {
			t.Fatalf("reviewer tool policy denied %q", name)
		}
	}
}

func TestReviewerVerdictRequiresExplicitOutcome(t *testing.T) {
	for _, verdict := range []string{"passed", "failed", "inconclusive"} {
		if err := validateReviewerVerdict(verdict); err != nil {
			t.Fatalf("valid reviewer verdict %q rejected: %v", verdict, err)
		}
	}
	if err := validateReviewerVerdict("candidate accepted by hidden verifier"); err == nil {
		t.Fatal("free-form reviewer verdict accepted")
	}
}

func TestReviewerToolsRejectUnboundEvidenceAtEntry(t *testing.T) {
	tools := ReviewerTools{Evidence: ReviewerEvidence{SubjectRevision: "wrong", ArtifactID: "candidate", CandidateDigest: "digest", Contract: "signed-zero@1", AllowedPath: "formatter.go"}}
	result := tools.Call(nil, "work_current", "review-current", []byte(`{}`))
	if result.Error != string(core.Malformed) {
		t.Fatalf("unbound reviewer evidence was accepted: %q", result.Error)
	}
}

func TestReviewerEvidenceRefsAreFrozenAllowlistOnly(t *testing.T) {
	evidence := ReviewerEvidence{SubjectRevision: "e8c48b1b8536d9c5c55c8469bfed0c60b5cdad0c", ArtifactID: "57761d55c7b0dbf63668b9f8010de41d", CandidateDigest: strings.Repeat("a", 64), Contract: "signed-zero@1", TaskInput: "task", AllowedPath: "formatter.go"}
	for _, ref := range []string{evidence.ArtifactID, evidence.CandidateDigest, evidence.SubjectRevision, evidence.Contract, evidence.AllowedPath} {
		if !reviewerEvidenceRefAllowed(evidence, ref) {
			t.Fatalf("frozen evidence ref rejected: %q", ref)
		}
	}
	for _, ref := range []string{"worker-check-receipt", "hidden-test-output", "wrong-artifact", "wrong-revision"} {
		if reviewerEvidenceRefAllowed(evidence, ref) {
			t.Fatalf("unallowlisted evidence ref accepted: %q", ref)
		}
	}
}

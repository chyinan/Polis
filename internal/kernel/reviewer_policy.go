// pattern: Functional Core
package kernel

import (
	"errors"
	"polis/internal/runner"
	"strings"
)

type ReviewerCheckResult struct {
	Passed bool   `json:"passed"`
	Digest string `json:"digest"`
	Phase  string `json:"phase"`
}

func redactReviewerReport(report runner.Report) ReviewerCheckResult {
	return ReviewerCheckResult{Passed: report.Passed, Digest: report.Digest, Phase: report.Phase}
}

func reviewerToolAllowed(name string) bool {
	switch name {
	case "work_current", "context_read", "workspace_read", "review_submit":
		return true
	default:
		return false
	}
}

func reviewerEvidenceRefAllowed(evidence ReviewerEvidence, ref string) bool {
	switch ref {
	case evidence.SubjectRevision, evidence.ArtifactID, evidence.CandidateDigest, evidence.Contract, evidence.AllowedPath:
		return true
	default:
		return false
	}
}

func validateReviewerEvidenceBinding(evidence ReviewerEvidence) error {
	if evidence.SubjectRevision != "e8c48b1b8536d9c5c55c8469bfed0c60b5cdad0c" || evidence.ArtifactID == "" || len(evidence.CandidateDigest) != 64 || strings.Trim(evidence.CandidateDigest, "0123456789abcdef") != "" || evidence.Contract != "signed-zero@1" || evidence.TaskInput == "" || evidence.AllowedPath != "formatter.go" {
		return errors.New("reviewer evidence binding is invalid")
	}
	return nil
}

func validateReviewerVerdict(verdict string) error {
	switch verdict {
	case "passed", "failed", "inconclusive":
		return nil
	default:
		return coreMalformedReviewerVerdict{}
	}
}

type coreMalformedReviewerVerdict struct{}

func (coreMalformedReviewerVerdict) Error() string { return "invalid reviewer verdict" }

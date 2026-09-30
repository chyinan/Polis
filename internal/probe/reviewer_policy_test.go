// pattern: Functional Core
package probe

import (
	"strings"
	"testing"
)

func TestBuildReviewerPromptUsesOnlyAllowlistedEvidence(t *testing.T) {
	pack := reviewerEvidencePack{
		SubjectRevision:  R02SubjectRevision,
		ArtifactID:       "57761d55c7b0dbf63668b9f8010de41d",
		CandidateDigest:  strings.Repeat("a", 64),
		Contract:         "signed-zero@1",
		TaskInput:        "Preserve the signed-zero compatibility contract for the frozen candidate.",
		AllowedPath:      "formatter.go",
		CandidateContent: "package formatter\n\nfunc Render(v float64) string { return \"candidate\" }\n",
	}

	prompt, err := buildReviewerPrompt(pack)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{R02SubjectRevision, pack.ArtifactID, pack.CandidateDigest, pack.Contract, pack.AllowedPath, pack.CandidateContent} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("reviewer prompt omitted allowlisted value %q", want)
		}
	}
	for _, forbidden := range []string{
		"HandoverBundle",
		"Checkpoints",
		"Obligations",
		"negative zero remains",
		"positive values carry",
		"collapsing all zeros",
		"rejected route",
		"R0_2_REPORT",
		"offline oracle",
		"verifier result",
		"workspace_check",
		"work_checkpoint",
	} {
		if strings.Contains(strings.ToLower(prompt), strings.ToLower(forbidden)) {
			t.Fatalf("reviewer prompt leaked forbidden material %q", forbidden)
		}
	}
}

func TestReviewerEvidenceRejectsContaminatedCandidateMetadata(t *testing.T) {
	pack := reviewerEvidencePack{
		SubjectRevision:  R02SubjectRevision,
		ArtifactID:       "57761d55c7b0dbf63668b9f8010de41d",
		CandidateDigest:  strings.Repeat("a", 64),
		Contract:         "signed-zero@1",
		TaskInput:        "Preserve the signed-zero compatibility contract for the frozen candidate.",
		AllowedPath:      "formatter.go",
		CandidateContent: "package formatter\n\nfunc Render(v float64) string { return \"candidate\" }\n",
		ProvenanceNote:   "rejected route from the Medium transcript",
	}

	if err := validateReviewerEvidence(pack); err == nil {
		t.Fatal("contaminated reviewer metadata was accepted")
	}
}

func TestReviewerContaminationPreflightRejectsHiddenMaterial(t *testing.T) {
	result := runReviewerContaminationPreflight(reviewerPreflightInput{
		Pack: reviewerEvidencePack{
			SubjectRevision:  R02SubjectRevision,
			ArtifactID:       "57761d55c7b0dbf63668b9f8010de41d",
			CandidateDigest:  strings.Repeat("a", 64),
			Contract:         "signed-zero@1",
			TaskInput:        "Preserve the signed-zero compatibility contract for the frozen candidate.",
			AllowedPath:      "formatter.go",
			CandidateContent: "candidate",
		},
		Prompt:        "candidate only; rejected route from the old transcript",
		ToolNames:     []string{"polis_work_current", "polis_context_read", "polis_workspace_read", "polis_review_submit"},
		Metadata:      map[string]string{"subject_revision": R02SubjectRevision, "artifact_id": "57761d55c7b0dbf63668b9f8010de41d", "contract": "signed-zero@1", "task_input": "Preserve the signed-zero compatibility contract.", "allowed_path": "formatter.go"},
		FileAllowlist: []string{"formatter.go"},
	})
	if result.Passed || len(result.Findings) == 0 {
		t.Fatalf("contaminated preflight passed: %+v", result)
	}
}

func TestReviewerContaminationPreflightPassesSafeAllowlist(t *testing.T) {
	pack := reviewerEvidencePack{
		SubjectRevision:  R02SubjectRevision,
		ArtifactID:       "57761d55c7b0dbf63668b9f8010de41d",
		CandidateDigest:  strings.Repeat("a", 64),
		Contract:         "signed-zero@1",
		TaskInput:        "Preserve the signed-zero compatibility contract for the frozen candidate.",
		AllowedPath:      "formatter.go",
		CandidateContent: "candidate",
	}
	prompt, err := buildReviewerPrompt(pack)
	if err != nil {
		t.Fatal(err)
	}
	result := runReviewerContaminationPreflight(reviewerPreflightInput{
		Pack: pack, Prompt: prompt,
		ToolNames:     []string{"polis_work_current", "polis_context_read", "polis_workspace_read", "polis_review_submit"},
		Metadata:      map[string]string{"subject_revision": R02SubjectRevision, "artifact_id": pack.ArtifactID, "candidate_digest": pack.CandidateDigest, "contract": pack.Contract, "task_input": pack.TaskInput, "allowed_path": pack.AllowedPath},
		FileAllowlist: []string{pack.AllowedPath},
	})
	if !result.Passed {
		t.Fatalf("safe reviewer preflight failed: %+v", result)
	}
}

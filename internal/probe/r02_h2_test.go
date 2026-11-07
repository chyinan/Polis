// pattern: Imperative Shell
package probe

import (
	"os"
	"path/filepath"
	"testing"
)

func TestR02H2RejectsHistoricalAllowancePath(t *testing.T) {
	old := t.TempDir()
	cfg := Config{Model: "gpt-5.6-luna", HighLimit: 1, DSN: "dedicated", AuthFile: "auth", Evidence: t.TempDir(), Root: t.TempDir()}
	_, err := RunR02H2(cfg, old, filepath.Join(old, "allowance.json"))
	if err == nil {
		t.Fatal("R0.2H2 accepted the historical allowance path")
	}
}

func TestReviewerPreflightAgainstPersistedR02Evidence(t *testing.T) {
	root := os.Getenv("POLIS_R02_EVIDENCE")
	if root == "" {
		t.Skip("POLIS_R02_EVIDENCE required")
	}
	old, err := loadRecoveredReal(root)
	if err != nil {
		t.Fatal(err)
	}
	if old.Artifact != R02ArtifactID {
		t.Fatalf("subject artifact changed: %s", old.Artifact)
	}
	pack := reviewerEvidencePack{SubjectRevision: R02SubjectRevision, ArtifactID: old.Artifact, CandidateDigest: digestFor(old.Content), Contract: "signed-zero@1", TaskInput: "Preserve the signed-zero compatibility contract and implement the authorized formatting milestones for the frozen formatter candidate.", AllowedPath: "formatter.go", CandidateContent: old.Content}
	prompt, err := buildReviewerPrompt(pack)
	if err != nil {
		t.Fatal(err)
	}
	result := runReviewerContaminationPreflight(reviewerPreflightInput{Pack: pack, Prompt: prompt, ToolNames: reviewerToolNames(), Metadata: map[string]string{"subject_revision": pack.SubjectRevision, "artifact_id": pack.ArtifactID, "candidate_digest": pack.CandidateDigest, "contract": pack.Contract, "task_input": pack.TaskInput, "allowed_path": pack.AllowedPath}, FileAllowlist: []string{"formatter.go"}})
	if !result.Passed {
		t.Fatalf("persisted R0.2 evidence failed reviewer preflight: %+v", result)
	}
}

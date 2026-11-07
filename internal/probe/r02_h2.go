// pattern: Imperative Shell
package probe

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"polis/internal/codex"
	"polis/internal/kernel"
	"polis/internal/runner"
	"time"
)

const R02ArtifactID = "57761d55c7b0dbf63668b9f8010de41d"

func RunR02H2(cfg Config, oldEvidence, allowancePath string) (result R02Result, err error) {
	result = R02Result{Status: "not_run", Model: cfg.Model, SubjectRevision: R02SubjectRevision, Preflight: "not_run", Version: runner.NativeVersion, Started: time.Now().UTC(), Usage: R02Usage{Money: "not_estimated", Limit: "R0.2H2: one independent High; concurrency 1; no Medium"}}
	if cfg.Model != "gpt-5.6-luna" || cfg.HighLimit != 1 {
		return result, errors.New("R0.2H2 requires Luna and High limit 1")
	}
	if oldEvidence == "" || cfg.Evidence == "" || cfg.DSN == "" || cfg.AuthFile == "" {
		return result, errors.New("R0.2H2 requires old evidence, dedicated DSN, evidence directory and auth file")
	}
	if allowancePath == "" {
		allowancePath = filepath.Join(cfg.Evidence, "allowance.json")
	}
	if filepath.Clean(allowancePath) == filepath.Clean(filepath.Join(oldEvidence, "allowance.json")) {
		return result, errors.New("R0.2H2 requires a distinct High allowance")
	}
	if e := os.MkdirAll(cfg.Evidence, 0700); e != nil {
		return result, e
	}
	defer func() {
		result.Finished = time.Now().UTC()
		if err != nil {
			result.Error = err.Error()
		}
		_ = writeJSON(filepath.Join(cfg.Evidence, "result.json"), result)
	}()
	old, e := loadRecoveredReal(oldEvidence)
	if e != nil {
		return result, e
	}
	if old.Artifact != R02ArtifactID {
		return result, fmt.Errorf("subject artifact changed: got %s", old.Artifact)
	}
	if digestFor(old.Content) == "" {
		return result, errors.New("subject artifact content digest is empty")
	}
	pack := reviewerEvidencePack{SubjectRevision: R02SubjectRevision, ArtifactID: old.Artifact, CandidateDigest: digestFor(old.Content), Contract: "signed-zero@1", TaskInput: "Preserve the signed-zero compatibility contract and implement the authorized formatting milestones for the frozen formatter candidate.", AllowedPath: "formatter.go", CandidateContent: old.Content}
	prompt, e := buildReviewerPrompt(pack)
	if e != nil {
		return result, e
	}
	toolNames := reviewerToolNames()
	metadata := map[string]string{"subject_revision": pack.SubjectRevision, "artifact_id": pack.ArtifactID, "candidate_digest": pack.CandidateDigest, "contract": pack.Contract, "task_input": pack.TaskInput, "allowed_path": pack.AllowedPath}
	preflight := runReviewerContaminationPreflight(reviewerPreflightInput{Pack: pack, Prompt: prompt, ToolNames: toolNames, Metadata: metadata, FileAllowlist: []string{pack.AllowedPath}})
	result.Preflight = "passed"
	if !preflight.Passed {
		result.Preflight = "failed"
		return result, fmt.Errorf("reviewer contamination preflight failed: %v", preflight.Findings)
	}
	if e = writeJSON(filepath.Join(cfg.Evidence, "reviewer-allowlist.json"), map[string]any{"subject_revision": pack.SubjectRevision, "artifact_id": pack.ArtifactID, "candidate_digest": pack.CandidateDigest, "contract": pack.Contract, "task_input": pack.TaskInput, "allowed_files": []string{pack.AllowedPath}, "tools": toolNames, "hidden_verifier": "trusted_controller_only"}); e != nil {
		return result, e
	}
	if e = writeJSON(filepath.Join(cfg.Evidence, "contamination-preflight.json"), preflight); e != nil {
		return result, e
	}
	budget, e := codex.NewBudget(allowancePath, 1, 1)
	if e != nil {
		return result, e
	}
	result.Usage.Medium = budget.Medium
	result.Usage.High = budget.High
	if e = budget.Reserve("high"); e != nil {
		return result, e
	}
	result.Usage.High = budget.High
	deadline := budget.Started.Add(10 * time.Minute)
	ctx, cancel := context.WithDeadline(context.Background(), deadline)
	defer cancel()
	k, e := kernel.Open(ctx, cfg.DSN, filepath.Join(cfg.Root, "blobs"))
	if e != nil {
		return result, e
	}
	defer k.Close()
	s, e := k.TXCreateCompany(ctx, "r02h2-review")
	if e != nil {
		return result, e
	}
	_, e = k.TXCreateProbe(ctx, s, "mission-r02h2")
	if e != nil {
		return result, e
	}
	if _, e = k.TXRestoreCandidateForReview(ctx, s, "mission-r02h2", old.Artifact, []byte(old.Content)); e != nil {
		return result, e
	}
	reviewTask, e := k.TXCreateReviewProbe(ctx, s, "mission-r02h2", old.Artifact)
	if e != nil {
		return result, e
	}
	review, e := runR02ReviewSession(ctx, cfg, k, s, reviewTask.ID, old.Artifact, 1, "R0.2H2 independent behavioral review")
	result.CapabilityDigest = review.capability
	result.Turns = append(result.Turns, review.turn)
	result.ReviewerVerdict = review.turn.ReviewerVerdict
	result.Usage.High = budget.High
	if e != nil {
		return result, e
	}
	checker := runner.Verifier{GoRoot: cfg.GoRoot, Scratch: cfg.Root, Context: ctx}
	report, verifyErr := k.VerifyProbe(ctx, s, old.Artifact, "full", checker)
	if report.Passed {
		result.HiddenVerifier = "passed"
	} else {
		result.HiddenVerifier = "failed"
	}
	if review.turn.ReviewerVerdict != "" && review.turn.ReviewerSubmissionReceipt != "" && hasAll(review.turn.ToolEvents, "workspace_read", "work_current", "context_read", "review_submit") && !containsAny(review.turn.ToolEvents, "workspace_replace", "artifact_submit", "workspace_check", "work_checkpoint") {
		result.ReviewIsolation = "passed"
	} else {
		result.ReviewIsolation = "failed"
	}
	if verifyErr != nil {
		result.Status = "failed"
		result.Behavior = "failed"
		return result, verifyErr
	}
	if result.ReviewerVerdict != "passed" || result.HiddenVerifier != "passed" || result.ReviewIsolation != "passed" {
		result.Status = "failed"
		result.Behavior = "failed"
		return result, errors.New("R0.2H2 reviewer, hidden verifier and isolation did not all pass")
	}
	result.Handover = "passed"
	result.Behavior = "passed"
	result.Status = "passed"
	return result, nil
}

func reviewerToolNames() []string {
	var names []string
	for _, raw := range codex.ReviewerTools() {
		if tool, ok := raw.(map[string]any); ok {
			if name, ok := tool["name"].(string); ok {
				names = append(names, name)
			}
		}
	}
	return names
}

// pattern: Functional Core
package probe

import (
	"errors"
	"fmt"
	"strings"
)

const R02SubjectRevision = "e8c48b1b8536d9c5c55c8469bfed0c60b5cdad0c"

type reviewerEvidencePack struct {
	SubjectRevision  string
	ArtifactID       string
	CandidateDigest  string
	Contract         string
	AllowedPath      string
	TaskInput        string
	CandidateContent string
	ProvenanceNote   string
}

type reviewerPreflightInput struct {
	Pack          reviewerEvidencePack
	Prompt        string
	ToolNames     []string
	Metadata      map[string]string
	FileAllowlist []string
}

type reviewerPreflightResult struct {
	Passed   bool
	Findings []string
}

type reviewerInputEstimate struct {
	Name            string `json:"name"`
	Source          string `json:"source"`
	Bytes           int    `json:"bytes"`
	EstimatedTokens int    `json:"estimated_tokens"`
}

func estimateReviewerInput(name, source, value string) reviewerInputEstimate {
	bytes := len([]byte(value))
	return reviewerInputEstimate{Name: name, Source: source, Bytes: bytes, EstimatedTokens: (bytes + 3) / 4}
}

func estimateReviewerInputContext(developerInstructions, toolSchema, prompt string, toolResults []string) []reviewerInputEstimate {
	components := []reviewerInputEstimate{
		estimateReviewerInput("developer_instructions", "codex.Client.StartReviewerThread developerInstructions", developerInstructions),
		estimateReviewerInput("dynamic_tools", "codex.ReviewerTools JSON schema", toolSchema),
		estimateReviewerInput("review_prompt", "buildReviewerPrompt frozen input + candidate", prompt),
	}
	for i, result := range toolResults {
		components = append(components, estimateReviewerInput(fmt.Sprintf("tool_result_%d", i+1), "Reviewer tool response serialized into native turn context", result))
	}
	return components
}

func validateReviewerEvidence(pack reviewerEvidencePack) error {
	if pack.SubjectRevision != R02SubjectRevision {
		return fmt.Errorf("unexpected subject revision")
	}
	if pack.ArtifactID == "" || pack.CandidateDigest == "" || pack.Contract == "" || pack.AllowedPath == "" || pack.TaskInput == "" || pack.CandidateContent == "" {
		return errors.New("reviewer evidence pack is incomplete")
	}
	if len(pack.CandidateDigest) != 64 || strings.Trim(pack.CandidateDigest, "0123456789abcdef") != "" {
		return errors.New("reviewer candidate digest is invalid")
	}
	if pack.Contract != "signed-zero@1" || pack.AllowedPath != "formatter.go" {
		return errors.New("reviewer evidence contract is invalid")
	}
	if len(pack.CandidateContent) > 4096 {
		return errors.New("reviewer candidate is too large")
	}
	if pack.ProvenanceNote != "" {
		return errors.New("reviewer provenance is not allowlisted")
	}
	return nil
}

func buildReviewerPrompt(pack reviewerEvidencePack) (string, error) {
	if err := validateReviewerEvidence(pack); err != nil {
		return "", err
	}
	return fmt.Sprintf("Review the frozen candidate independently. Use only the supplied read-only Polis tools. Decide whether the candidate satisfies the frozen contract from the candidate and frozen input alone. Submit one explicit verdict with allowlisted evidence references, confidence and limitations. Do not modify the candidate.\nSubject revision: %s\nArtifact: %s\nCandidate digest: %s\nContract: %s\nTask input: %s\nAllowed path: %s\nCandidate content:\n%s\n", pack.SubjectRevision, pack.ArtifactID, pack.CandidateDigest, pack.Contract, pack.TaskInput, pack.AllowedPath, pack.CandidateContent), nil
}

func runReviewerContaminationPreflight(input reviewerPreflightInput) reviewerPreflightResult {
	result := reviewerPreflightResult{Passed: true}
	fail := func(finding string) {
		result.Passed = false
		result.Findings = append(result.Findings, finding)
	}
	if err := validateReviewerEvidence(input.Pack); err != nil {
		fail(err.Error())
	}
	for _, forbidden := range []string{
		"handoverbundle", "checkpoints", "obligations", "rejected route", "medium transcript", "r0_2_report", "offline oracle", "negative zero remains", "positive values carry", "collapsing all zeros",
	} {
		if strings.Contains(strings.ToLower(input.Prompt), forbidden) {
			fail("reviewer prompt contains forbidden material: " + forbidden)
		}
	}
	allowedTools := map[string]bool{
		"polis_work_current":   true,
		"polis_context_read":   true,
		"polis_workspace_read": true,
		"polis_review_submit":  true,
	}
	for _, name := range input.ToolNames {
		if !allowedTools[name] || name == "polis_workspace_replace" || name == "polis_artifact_submit" {
			fail("reviewer tool is outside read-only allowlist: " + name)
		}
	}
	if len(input.FileAllowlist) != 1 || input.FileAllowlist[0] != "formatter.go" {
		fail("reviewer file allowlist is not exactly formatter.go")
	}
	allowedMetadata := map[string]bool{"subject_revision": true, "artifact_id": true, "candidate_digest": true, "contract": true, "task_input": true, "allowed_path": true}
	for key, value := range input.Metadata {
		if !allowedMetadata[key] {
			fail("reviewer metadata key is not allowlisted: " + key)
		}
		if strings.Contains(strings.ToLower(value), "transcript") || strings.Contains(strings.ToLower(value), "rejected") || strings.Contains(strings.ToLower(value), "oracle") {
			fail("reviewer metadata contains provenance contamination: " + key)
		}
	}
	for _, key := range []string{"subject_revision", "artifact_id", "contract", "task_input", "allowed_path"} {
		if input.Metadata[key] == "" {
			fail("reviewer metadata is missing: " + key)
		}
	}
	return result
}

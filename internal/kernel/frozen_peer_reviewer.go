// pattern: Imperative Shell
package kernel

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"time"

	"polis/internal/core"
)

type FrozenPeerReviewEvidence struct {
	SubjectRevision    string   `json:"subject_revision"`
	TaskInput          string   `json:"task_input"`
	ContractRevisionID string   `json:"contract_revision_id"`
	MessageID          string   `json:"message_id"`
	ObligationID       string   `json:"obligation_id"`
	BackendArtifactID  string   `json:"backend_artifact_id"`
	FrontendArtifactID string   `json:"frontend_artifact_id"`
	BackendDigest      string   `json:"backend_digest"`
	FrontendDigest     string   `json:"frontend_digest"`
	BackendContent     string   `json:"-"`
	FrontendContent    string   `json:"-"`
	AllowedRefs        []string `json:"allowed_refs"`
	ReviewRecordPath   string   `json:"-"`
}

type FrozenPeerReviewTaskView struct {
	SubjectRevision    string `json:"subject_revision"`
	TaskInput          string `json:"task_input"`
	ContractRevisionID string `json:"contract_revision_id"`
	MessageID          string `json:"message_id"`
	ObligationID       string `json:"obligation_id"`
	BackendArtifactID  string `json:"backend_artifact_id"`
	FrontendArtifactID string `json:"frontend_artifact_id"`
	BackendDigest      string `json:"backend_digest"`
	FrontendDigest     string `json:"frontend_digest"`
}

type FrozenPeerReviewWorkspaceView struct {
	BackendPath     string `json:"backend_path"`
	BackendDigest   string `json:"backend_digest"`
	BackendContent  string `json:"backend_content"`
	FrontendPath    string `json:"frontend_path"`
	FrontendDigest  string `json:"frontend_digest"`
	FrontendContent string `json:"frontend_content"`
}

type FrozenPeerReviewRecord struct {
	Receipt     Receipt                  `json:"receipt"`
	Submission  ReviewerSubmission       `json:"submission"`
	Subject     FrozenPeerReviewEvidence `json:"subject"`
	SubmittedAt time.Time                `json:"submitted_at"`
}

// FrozenPeerReviewerTools serves only the immutable subject package. Its sole
// write is an independent review record; it cannot reach Polis subject state.
type FrozenPeerReviewerTools struct {
	Evidence FrozenPeerReviewEvidence
	mu       sync.Mutex
}

func (t *FrozenPeerReviewerTools) Call(ctx context.Context, name, callID string, raw []byte) ToolResult {
	if len(callID) == 0 || len(callID) > 256 {
		return ToolResult{Error: string(core.Malformed)}
	}
	switch name {
	case "work_current", "context_read", "workspace_read", "review_submit":
	default:
		return ToolResult{Error: string(core.Denied)}
	}
	result, err := t.call(ctx, name, raw)
	if err != nil {
		var known core.Code
		if errors.As(err, &known) {
			return ToolResult{Error: known.Error()}
		}
		return ToolResult{Error: "OUTCOME_UNKNOWN", Detail: err.Error()}
	}
	return result
}

func (t *FrozenPeerReviewerTools) call(ctx context.Context, name string, raw []byte) (ToolResult, error) {
	switch name {
	case "work_current", "context_read":
		var args struct{}
		if err := strictArgs(raw, &args); err != nil {
			return ToolResult{}, err
		}
		return ToolResult{Data: FrozenPeerReviewTaskView{
			SubjectRevision: t.Evidence.SubjectRevision, TaskInput: t.Evidence.TaskInput,
			ContractRevisionID: t.Evidence.ContractRevisionID, MessageID: t.Evidence.MessageID,
			ObligationID: t.Evidence.ObligationID, BackendArtifactID: t.Evidence.BackendArtifactID,
			FrontendArtifactID: t.Evidence.FrontendArtifactID, BackendDigest: t.Evidence.BackendDigest,
			FrontendDigest: t.Evidence.FrontendDigest,
		}}, nil
	case "workspace_read":
		var args struct{}
		if err := strictArgs(raw, &args); err != nil {
			return ToolResult{}, err
		}
		return ToolResult{Data: FrozenPeerReviewWorkspaceView{
			BackendPath: "backend candidate artifact", BackendDigest: t.Evidence.BackendDigest, BackendContent: t.Evidence.BackendContent,
			FrontendPath: "frontend candidate artifact", FrontendDigest: t.Evidence.FrontendDigest, FrontendContent: t.Evidence.FrontendContent,
		}}, nil
	case "review_submit":
		var submission ReviewerSubmission
		if err := strictArgs(raw, &submission); err != nil {
			return ToolResult{}, err
		}
		if err := validateFrozenReviewerSubmission(submission, t.Evidence.AllowedRefs); err != nil {
			return ToolResult{}, err
		}
		return t.persistReview(ctx, submission)
	default:
		return ToolResult{}, core.Denied
	}
}

func validateFrozenReviewerSubmission(submission ReviewerSubmission, allowedRefs []string) error {
	if err := validateReviewerVerdict(submission.Verdict); err != nil || len(submission.Findings) == 0 || len(submission.Evidence) == 0 || len(submission.Limitations) == 0 {
		return core.Malformed
	}
	if submission.Confidence != "low" && submission.Confidence != "medium" && submission.Confidence != "high" {
		return core.Malformed
	}
	for _, finding := range submission.Findings {
		if len(finding) == 0 || len(finding) > 512 {
			return core.Malformed
		}
	}
	for _, limitation := range submission.Limitations {
		if len(limitation) == 0 || len(limitation) > 512 {
			return core.Malformed
		}
	}
	for _, ref := range submission.Evidence {
		if len(ref) == 0 || len(ref) > 512 || !containsReviewRef(allowedRefs, ref) {
			return core.Denied
		}
	}
	return nil
}

func containsReviewRef(refs []string, wanted string) bool {
	for _, ref := range refs {
		if ref == wanted {
			return true
		}
	}
	return false
}

func (t *FrozenPeerReviewerTools) persistReview(ctx context.Context, submission ReviewerSubmission) (ToolResult, error) {
	_ = ctx
	t.mu.Lock()
	defer t.mu.Unlock()
	path := t.Evidence.ReviewRecordPath
	if path == "" {
		return ToolResult{}, core.Malformed
	}
	if raw, err := os.ReadFile(path); err == nil {
		var existing FrozenPeerReviewRecord
		if json.Unmarshal(raw, &existing) == nil && reflect.DeepEqual(existing.Submission, submission) {
			return ToolResult{Receipt: &existing.Receipt, Data: submission}, nil
		}
		return ToolResult{}, core.Conflict
	} else if !errors.Is(err, os.ErrNotExist) {
		return ToolResult{}, err
	}
	record := FrozenPeerReviewRecord{Receipt: Receipt{ID: frozenReviewID(), Status: submission.Verdict}, Submission: submission, Subject: t.Evidence, SubmittedAt: time.Now().UTC()}
	if err := writeFrozenReviewJSON(path, record); err != nil {
		return ToolResult{}, err
	}
	return ToolResult{Receipt: &record.Receipt, Data: submission}, nil
}

func frozenReviewID() string {
	return "review-" + time.Now().UTC().Format("20060102T150405.000000000Z07:00")
}

func writeFrozenReviewJSON(path string, value any) error {
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	temp, err := os.CreateTemp(dir, ".review-*")
	if err != nil {
		return err
	}
	tempPath := temp.Name()
	defer os.Remove(tempPath)
	if _, err := temp.Write(append(raw, '\n')); err != nil {
		temp.Close()
		return err
	}
	if err := temp.Sync(); err != nil {
		temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	return os.Rename(tempPath, path)
}

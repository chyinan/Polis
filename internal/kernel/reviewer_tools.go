// pattern: Imperative Shell
package kernel

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/jackc/pgx/v5"
	"polis/internal/core"
)

type ReviewerEvidence struct {
	SubjectRevision string
	ArtifactID      string
	CandidateDigest string
	Contract        string
	TaskInput       string
	AllowedPath     string
}

type ReviewerTaskView struct {
	SubjectRevision string `json:"subject_revision"`
	ArtifactID      string `json:"artifact_id"`
	CandidateDigest string `json:"candidate_digest"`
	Contract        string `json:"contract"`
	AllowedPath     string `json:"allowed_path"`
	TaskInput       string `json:"task_input"`
}

type ReviewerWorkspaceView struct {
	Path    string `json:"path"`
	Digest  string `json:"digest"`
	Content string `json:"content"`
}

type ReviewerTools struct {
	Kernel   *Kernel
	Binding  Binding
	Evidence ReviewerEvidence
}

type ReviewerSubmission struct {
	Verdict     string   `json:"verdict"`
	Findings    []string `json:"findings"`
	Evidence    []string `json:"evidence"`
	Confidence  string   `json:"confidence"`
	Limitations []string `json:"limitations"`
}

func (t ReviewerTools) Call(ctx context.Context, name, callID string, raw []byte) ToolResult {
	if len(callID) == 0 || len(callID) > 256 {
		return ToolResult{Error: string(core.Malformed)}
	}
	if !reviewerToolAllowed(name) {
		if t.Kernel != nil {
			_ = t.Kernel.RecordHistorical(ctx, t.Binding, name, raw, core.Denied.Error())
		}
		return ToolResult{Error: string(core.Denied)}
	}
	if e := validateReviewerEvidenceBinding(t.Evidence); e != nil {
		return ToolResult{Error: string(core.Malformed)}
	}
	if t.Kernel == nil {
		return ToolResult{Error: "OUTCOME_UNKNOWN", Detail: "reviewer kernel is unavailable"}
	}
	key := "review-tool-" + fingerprint([]string{t.Binding.session, callID})[:60]
	result, e := t.call(ctx, name, key, raw)
	if e != nil {
		if errors.Is(e, core.StaleEpoch) || errors.Is(e, core.Denied) {
			_ = t.Kernel.RecordHistorical(ctx, t.Binding, name, raw, e.Error())
		}
		var known core.Code
		if errors.As(e, &known) {
			return ToolResult{Error: known.Error()}
		}
		return ToolResult{Error: "OUTCOME_UNKNOWN", Detail: e.Error()}
	}
	return result
}

func (t ReviewerTools) call(ctx context.Context, name, key string, raw []byte) (ToolResult, error) {
	view := ReviewerTaskView{SubjectRevision: t.Evidence.SubjectRevision, ArtifactID: t.Evidence.ArtifactID, CandidateDigest: t.Evidence.CandidateDigest, Contract: t.Evidence.Contract, TaskInput: t.Evidence.TaskInput, AllowedPath: t.Evidence.AllowedPath}
	switch name {
	case "work_current", "context_read":
		var args struct{}
		if e := strictArgs(raw, &args); e != nil {
			return ToolResult{}, e
		}
		return ToolResult{Data: view}, nil
	case "workspace_read":
		var args struct{}
		if e := strictArgs(raw, &args); e != nil {
			return ToolResult{}, e
		}
		workspace, e := t.Kernel.Workspace(ctx, t.Binding)
		if e != nil {
			return ToolResult{}, e
		}
		if workspace.Digest != t.Evidence.CandidateDigest {
			return ToolResult{}, core.Integrity
		}
		return ToolResult{Data: ReviewerWorkspaceView{Path: t.Evidence.AllowedPath, Digest: workspace.Digest, Content: workspace.Content}}, nil
	case "review_submit":
		var submission ReviewerSubmission
		if e := strictArgs(raw, &submission); e != nil {
			return ToolResult{}, e
		}
		if e := validateReviewerVerdict(submission.Verdict); e != nil || len(submission.Findings) == 0 || len(submission.Evidence) == 0 || len(submission.Limitations) == 0 || (submission.Confidence != "low" && submission.Confidence != "medium" && submission.Confidence != "high") {
			return ToolResult{}, core.Malformed
		}
		for _, finding := range submission.Findings {
			if len(finding) == 0 || len(finding) > 512 {
				return ToolResult{}, core.Malformed
			}
		}
		for _, evidence := range submission.Evidence {
			if len(evidence) == 0 || len(evidence) > 512 {
				return ToolResult{}, core.Malformed
			}
			if !reviewerEvidenceRefAllowed(t.Evidence, evidence) {
				return ToolResult{}, core.Denied
			}
		}
		for _, limitation := range submission.Limitations {
			if len(limitation) == 0 || len(limitation) > 512 {
				return ToolResult{}, core.Malformed
			}
		}
		receipt, e := t.Kernel.TXWrite(ctx, t.Binding.scope, &t.Binding, key, "review.submit", submission, func(tx pgx.Tx) (Receipt, error) {
			var task, kind, owner, mission, plan string
			if e := tx.QueryRow(ctx, "SELECT t.id,t.kind,t.owner,t.mission_id,t.plan::text FROM worker_sessions s JOIN tasks t ON t.company_id=s.company_id AND t.id=s.task_id WHERE s.company_id=$1 AND s.id=$2", t.Binding.scope.company, t.Binding.session).Scan(&task, &kind, &owner, &mission, &plan); e != nil {
				return Receipt{}, e
			}
			var planData struct {
				ReviewOf string `json:"review_of"`
			}
			if json.Unmarshal([]byte(plan), &planData) != nil || kind != "review" || owner != "emp-review" || mission == "" || planData.ReviewOf != t.Evidence.ArtifactID {
				return Receipt{}, core.Denied
			}
			var digest, contract string
			if e := tx.QueryRow(ctx, "SELECT a.digest,m.contract FROM artifacts a JOIN tasks t ON t.company_id=a.company_id AND t.id=a.task_id JOIN missions m ON m.company_id=t.company_id AND m.id=t.mission_id WHERE a.company_id=$1 AND a.id=$2 AND a.state='ready' AND a.verdict='candidate'", t.Binding.scope.company, t.Evidence.ArtifactID).Scan(&digest, &contract); e != nil {
				return Receipt{}, e
			}
			if digest != t.Evidence.CandidateDigest || contract != t.Evidence.Contract {
				return Receipt{}, core.Denied
			}
			if e := requireMemoryTaskCleanTX(ctx, tx, t.Binding.scope.company, task); e != nil {
				return Receipt{}, e
			}
			_, e := tx.Exec(ctx, "UPDATE tasks SET state='completed' WHERE company_id=$1 AND id=$2", t.Binding.scope.company, task)
			if e != nil {
				return Receipt{}, e
			}
			return Receipt{ID: t.Binding.session, Status: submission.Verdict}, nil
		})
		return ToolResult{Receipt: &receipt, Data: submission}, e
	default:
		return ToolResult{}, core.Denied
	}
}

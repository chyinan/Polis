// pattern: Imperative Shell
package kernel

import (
	"context"
	"errors"
	"polis/internal/core"
)

type PeerReviewerTools struct {
	Kernel   *Kernel
	Binding  Binding
	Evidence PeerReviewEvidence
}

func (t PeerReviewerTools) Call(ctx context.Context, name, callID string, raw []byte) ToolResult {
	if len(callID) == 0 || len(callID) > 256 {
		return ToolResult{Error: string(core.Malformed)}
	}
	if name != "work_current" && name != "context_read" && name != "workspace_read" && name != "review_submit" {
		return ToolResult{Error: string(core.Denied)}
	}
	if t.Kernel == nil {
		return ToolResult{Error: "OUTCOME_UNKNOWN", Detail: "reviewer kernel unavailable"}
	}
	key := "peer-review-tool-" + fingerprint([]string{t.Binding.session, callID})[:60]
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

func (t PeerReviewerTools) call(ctx context.Context, name, key string, raw []byte) (ToolResult, error) {
	var args struct{}
	if name != "review_submit" {
		if e := strictArgs(raw, &args); e != nil {
			return ToolResult{}, e
		}
	}
	switch name {
	case "work_current", "context_read":
		return ToolResult{Data: t.Evidence}, nil
	case "workspace_read":
		view, e := t.Kernel.PeerIntegrationView(ctx, t.Binding, t.Evidence)
		return ToolResult{Data: view}, e
	case "review_submit":
		var submission ReviewerSubmission
		if e := strictArgs(raw, &submission); e != nil {
			return ToolResult{}, e
		}
		if e := validateReviewerVerdict(submission.Verdict); e != nil || len(submission.Findings) == 0 || len(submission.Evidence) == 0 || len(submission.Limitations) == 0 || (submission.Confidence != "low" && submission.Confidence != "medium" && submission.Confidence != "high") {
			return ToolResult{}, core.Malformed
		}
		for _, ref := range submission.Evidence {
			if !peerReviewEvidenceAllowed(t.Evidence, ref) {
				return ToolResult{}, core.Denied
			}
		}
		r, e := t.Kernel.TXPeerReviewSubmit(ctx, t.Binding, t.Evidence, submission, key)
		return ToolResult{Receipt: &r, Data: submission}, e
	default:
		return ToolResult{}, core.Denied
	}
}

func peerReviewEvidenceAllowed(e PeerReviewEvidence, ref string) bool {
	for _, allowed := range e.AllowedRefs {
		if ref == allowed {
			return true
		}
	}
	return false
}

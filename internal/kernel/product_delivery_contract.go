// pattern: Functional Core
package kernel

import (
	"bytes"
	"encoding/json"
	"errors"

	"polis/internal/core"
	"polis/internal/taskvalidation"
)

// ParseProductTaskDeliveryRequest validates the public decision boundary. The
// submit call itself is the Agent's explicit choice; all delivery data is
// authoritative control-plane context and therefore has no public fields.
func ParseProductTaskDeliveryRequest(raw []byte) (PeerToolRejection, core.Code) {
	if len(raw) > 8192 {
		return productDeliveryRequestRejection("request_too_large", "submit only an empty JSON object to explicitly deliver the current workspace.", "too_large"), core.TooLarge
	}
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || trimmed[0] != '{' {
		return productDeliveryRequestRejection("invalid_delivery_request", "submit an empty JSON object to explicitly deliver the current workspace.", "not_empty_object"), core.Malformed
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(trimmed, &fields); err != nil || fields == nil {
		return productDeliveryRequestRejection("invalid_delivery_request", "submit an empty JSON object to explicitly deliver the current workspace.", "not_empty_object"), core.Malformed
	}
	if len(fields) != 0 {
		return productDeliveryRequestRejection("delivery_fields_not_allowed", "do not echo Task, session, workspace or validation fields; the explicit task_submit call supplies the decision.", "agent_supplied_derived_fields"), core.Malformed
	}
	return PeerToolRejection{}, ""
}

func productDeliveryRequestRejection(reason, summary, actual string) PeerToolRejection {
	rejection := newCheckpointRejection(reason, summary)
	rejection.ExpectedPublicShape = "{}"
	rejection.ActualCategory = actual
	rejection.TransactionOutcome = "not_started"
	return rejection
}

// ProductDeliveryCheckpointInput contains only authoritative values gathered
// by the delivery transaction. It is intentionally not a provider request.
type ProductDeliveryCheckpointInput struct {
	TaskID            string
	BindingDigest     string
	WorkspaceDigest   string
	WorkspaceRevision int64
	SessionID         string
	Epoch             int64
	CheckID           string
	RunnerRevision    string
}

// NewQualifiedProductDeliveryCheckpoint creates the canonical final
// checkpoint. The caller's submit decision is represented by invoking the
// delivery operation; no model-authored checkpoint prose is needed here.
func NewQualifiedProductDeliveryCheckpoint(input ProductDeliveryCheckpointInput) (Checkpoint, error) {
	for _, value := range []string{input.TaskID, input.BindingDigest, input.WorkspaceDigest, input.SessionID, input.CheckID, input.RunnerRevision} {
		if value == "" {
			return Checkpoint{}, errors.New("product delivery checkpoint input is incomplete")
		}
	}
	if input.WorkspaceRevision <= 0 || input.Epoch <= 0 {
		return Checkpoint{}, errors.New("product delivery checkpoint input has invalid workspace or epoch")
	}
	return Checkpoint{
		AcceptanceCheckerRevision:   input.RunnerRevision,
		TaskValidationBindingDigest: input.BindingDigest,
		ValidationStatus:            string(taskvalidation.StatusPass),
		FinalizationState:           "current",
		Kind:                        CheckpointQualified,
		Summary:                     "current validated workspace submitted as final Task deliverable",
		Facts:                       []string{"current workspace passed the bound Task validation"},
		Decisions:                   []string{"submit the current validated workspace"},
		Rejected:                    []string{},
		EvidenceRefs:                []string{input.CheckID},
		WorkspaceDigest:             input.WorkspaceDigest,
		WorkspaceRevision:           input.WorkspaceRevision,
		SessionID:                   input.SessionID,
		Epoch:                       input.Epoch,
	}, nil
}

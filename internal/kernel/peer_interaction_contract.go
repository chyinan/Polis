// pattern: Functional Core
package kernel

import (
	"bytes"
	"encoding/json"
	"io"
	"polis/internal/core"
)

var peerApplyReferenceTypes = []string{"obligation_id", "contract_revision_id", "workspace_revision", "evidence_refs[]"}
var peerApplyAcceptedEvidenceTypes = []string{"workspace.replace", "workspace.check", "collab.apply"}

type PeerApplyRequest struct {
	ObligationID       string   `json:"obligation_id"`
	ContractRevisionID string   `json:"contract_revision_id"`
	WorkspaceRevision  int64    `json:"workspace_revision"`
	EvidenceRefs       []string `json:"evidence_refs"`
}

type InvalidEvidenceRef struct {
	Ref          string `json:"ref"`
	DetectedType string `json:"detected_type"`
	Reason       string `json:"reason"`
}

// PeerToolRejection is the public, non-secret explanation returned with a
// rejected peer operation. It describes references and state, never checker
// implementation or a business answer.
type PeerToolRejection struct {
	ReasonCode               string               `json:"reason_code"`
	ActionableSummary        string               `json:"actionable_summary"`
	FailingField             string               `json:"failing_field,omitempty"`
	ExpectedPublicShape      string               `json:"expected_public_shape,omitempty"`
	ActualCategory           string               `json:"actual_category,omitempty"`
	ExpectedReferenceTypes   []string             `json:"expected_reference_types"`
	AcceptedEvidenceTypes    []string             `json:"accepted_evidence_types,omitempty"`
	InvalidEvidenceRefs      []InvalidEvidenceRef `json:"invalid_evidence_refs,omitempty"`
	TargetEmployeeID         string               `json:"target_employee_id,omitempty"`
	TargetTaskID             string               `json:"target_task_id,omitempty"`
	CurrentTaskState         string               `json:"current_task_state,omitempty"`
	TransactionOutcome       string               `json:"transaction_outcome,omitempty"`
	CurrentObligationID      string               `json:"current_obligation_id"`
	CurrentContractRevision  string               `json:"current_contract_revision"`
	CurrentWorkspaceRevision int64                `json:"current_workspace_revision"`
}

type peerToolError struct {
	Code      core.Code
	Rejection PeerToolRejection
}

func (e peerToolError) Error() string { return e.Code.Error() }
func (e peerToolError) Unwrap() error { return e.Code }

func newPeerToolRejection(reason, summary string) PeerToolRejection {
	return PeerToolRejection{
		ReasonCode:             reason,
		ActionableSummary:      summary,
		ExpectedReferenceTypes: append([]string(nil), peerApplyReferenceTypes...),
		AcceptedEvidenceTypes:  append([]string(nil), peerApplyAcceptedEvidenceTypes...),
	}
}

func newCheckpointRejection(reason, summary string) PeerToolRejection {
	return PeerToolRejection{
		ReasonCode:             reason,
		ActionableSummary:      summary,
		ExpectedReferenceTypes: []string{"evidence_refs[] (receipt_id)"},
	}
}

func validatePeerApplyRequest(request PeerApplyRequest) (PeerToolRejection, core.Code) {
	if !core.ValidID(request.ObligationID) || !core.ValidID(request.ContractRevisionID) {
		return newPeerToolRejection("missing_current_references", "supply the current obligation_id and contract_revision_id from the peer handover."), core.Malformed
	}
	if request.WorkspaceRevision <= 0 {
		return newPeerToolRejection("invalid_workspace_revision", "supply the positive workspace_revision returned by workspace_read or work_current."), core.Malformed
	}
	if len(request.EvidenceRefs) == 0 {
		return newPeerToolRejection("missing_evidence_refs", "supply receipt IDs in evidence_refs[]; do not submit source content."), core.Malformed
	}
	if len(request.EvidenceRefs) > 8 {
		return newPeerToolRejection("too_many_evidence_refs", "supply at most eight receipt IDs in evidence_refs[]."), core.Malformed
	}
	for _, ref := range request.EvidenceRefs {
		if !core.ValidID(ref) {
			return newPeerToolRejection("invalid_evidence_ref", "evidence_refs[] must contain receipt IDs, not prose or source content."), core.Malformed
		}
	}
	return PeerToolRejection{}, ""
}

func parsePeerApplyRequest(raw []byte) (PeerApplyRequest, PeerToolRejection, core.Code) {
	if len(raw) > 8192 {
		return PeerApplyRequest{}, newPeerToolRejection("request_too_large", "submit only the structured apply references; source content is not accepted."), core.TooLarge
	}
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || trimmed[0] != '{' {
		return PeerApplyRequest{}, newPeerToolRejection("malformed_apply_request", "submit a JSON object containing the current obligation, contract, workspace revision and receipt references."), core.Malformed
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(trimmed, &fields); err != nil {
		return PeerApplyRequest{}, newPeerToolRejection("malformed_apply_request", "submit a JSON object containing the current obligation, contract, workspace revision and receipt references."), core.Malformed
	}
	if _, present := fields["content"]; present {
		return PeerApplyRequest{}, newPeerToolRejection("candidate_content_not_accepted", "collab_apply records an already-persisted workspace change; use workspace_replace to write the workspace, then submit receipt IDs here."), core.Malformed
	}
	allowed := map[string]bool{"obligation_id": true, "contract_revision_id": true, "workspace_revision": true, "evidence_refs": true}
	for key := range fields {
		if !allowed[key] {
			return PeerApplyRequest{}, newPeerToolRejection("unknown_apply_field", "collab_apply accepts only obligation_id, contract_revision_id, workspace_revision and evidence_refs[]."), core.Malformed
		}
	}
	var request PeerApplyRequest
	if err := json.Unmarshal(trimmed, &request); err != nil {
		return PeerApplyRequest{}, newPeerToolRejection("malformed_apply_request", "submit typed current references and a receipt-ID evidence_refs[] array."), core.Malformed
	}
	for _, key := range []string{"obligation_id", "contract_revision_id", "workspace_revision", "evidence_refs"} {
		if _, present := fields[key]; !present {
			return PeerApplyRequest{}, newPeerToolRejection("missing_apply_field", "include obligation_id, contract_revision_id, workspace_revision and evidence_refs[]."), core.Malformed
		}
	}
	rejection, code := validatePeerApplyRequest(request)
	return request, rejection, code
}

func validateEvidenceRefs(refs []string) (PeerToolRejection, core.Code) {
	if len(refs) == 0 {
		return newPeerToolRejection("missing_evidence_refs", "include at least one receipt ID in evidence_refs[]."), core.Malformed
	}
	if len(refs) > 8 {
		return newPeerToolRejection("too_many_evidence_refs", "include at most eight receipt IDs in evidence_refs[]."), core.Malformed
	}
	for _, ref := range refs {
		if !core.ValidID(ref) {
			return newPeerToolRejection("invalid_evidence_ref", "evidence_refs[] must contain receipt IDs, not prose."), core.Malformed
		}
	}
	return PeerToolRejection{}, ""
}

func peerCheckpointDenied(reason, summary, currentContract string, currentWorkspace int64) error {
	rejection := newCheckpointRejection(reason, summary)
	switch reason {
	case "missing_evidence_refs", "qualified_checkpoint_requires_check_receipt", "qualified_check_not_passed", "qualified_check_stale", "evidence_ref_invalid":
		rejection.FailingField = "evidence_refs"
		rejection.ExpectedPublicShape = "one or more current passing workspace_check receipt references"
		rejection.ActualCategory = "receipt_not_accepted"
	case "validation_not_configured":
		rejection.FailingField = "TaskValidationBinding"
		rejection.ExpectedPublicShape = "Task with an immutable TaskValidationBinding"
		rejection.ActualCategory = "validation_not_configured"
	}
	rejection.CurrentContractRevision = currentContract
	rejection.CurrentWorkspaceRevision = currentWorkspace
	return peerToolError{Code: core.Denied, Rejection: rejection}
}

func parsePeerCheckpointRequest(raw []byte) (Checkpoint, PeerToolRejection, core.Code) {
	if len(raw) > 8192 {
		return Checkpoint{}, newCheckpointRejection("request_too_large", "submit checkpoint metadata and receipt IDs only."), core.TooLarge
	}
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || trimmed[0] != '{' {
		return Checkpoint{}, newCheckpointRejection("malformed_checkpoint_request", "submit a JSON checkpoint object with evidence_refs[]."), core.Malformed
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(trimmed, &fields); err != nil {
		return Checkpoint{}, newCheckpointRejection("malformed_checkpoint_request", "submit a JSON checkpoint object with evidence_refs[]."), core.Malformed
	}
	if _, present := fields["evidence"]; present {
		return Checkpoint{}, newCheckpointRejection("evidence_refs_required", "checkpoint evidence must be receipt IDs in evidence_refs[]; prose belongs in summary or failed_checks."), core.Malformed
	}
	var checkpoint Checkpoint
	decoder := json.NewDecoder(bytes.NewReader(trimmed))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&checkpoint); err != nil {
		return Checkpoint{}, newCheckpointRejection("malformed_checkpoint_request", "submit the checkpoint fields with evidence_refs[] as receipt IDs."), core.Malformed
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return Checkpoint{}, newCheckpointRejection("malformed_checkpoint_request", "submit exactly one JSON checkpoint object."), core.Malformed
	}
	if _, present := fields["evidence_refs"]; !present {
		return Checkpoint{}, newCheckpointRejection("missing_evidence_refs", "include receipt IDs in evidence_refs[] for the checkpoint evidence."), core.Malformed
	}
	rejection, code := validateEvidenceRefs(checkpoint.EvidenceRefs)
	return checkpoint, rejection, code
}

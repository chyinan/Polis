// pattern: Functional Core
package kernel

import (
	"bytes"
	"encoding/json"
	"io"
	"polis/internal/core"
)

const productCheckpointEvidencePurpose = "workspace_check_pass"

// ProductCheckpointEvidenceRef is the provider-facing typed representation of
// a receipt. The persisted Checkpoint continues to store only receipt IDs so
// the authorization and session binding rules remain unchanged.
type ProductCheckpointEvidenceRef struct {
	ReceiptID string `json:"receipt_id"`
	Proves    string `json:"proves"`
}

type productCheckpointRequest struct {
	Kind         string                         `json:"kind"`
	Summary      string                         `json:"summary"`
	Facts        []string                       `json:"facts"`
	Decisions    []string                       `json:"decisions"`
	Rejected     []string                       `json:"rejected"`
	EvidenceRefs []ProductCheckpointEvidenceRef `json:"evidence_refs"`
	NextAction   string                         `json:"next_action,omitempty"`
	FailedChecks []string                       `json:"failed_checks,omitempty"`
}

func productCheckpointRejection(reason, summary, field, expected, actual string) (PeerToolRejection, core.Code) {
	rejection := newCheckpointRejection(reason, summary)
	rejection.FailingField = field
	rejection.ExpectedPublicShape = expected
	rejection.ActualCategory = actual
	return rejection, core.Malformed
}

func parseProductCheckpointRequest(raw []byte) (Checkpoint, PeerToolRejection, core.Code) {
	if len(raw) > 8192 {
		rejection, code := productCheckpointRejection("request_too_large", "submit checkpoint metadata and typed receipt references only.", "request", "one JSON object no larger than 8192 bytes", "too_large")
		return Checkpoint{}, rejection, code
	}
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || trimmed[0] != '{' {
		rejection, code := productCheckpointRejection("invalid_field_type", "submit one JSON checkpoint object.", "request", "JSON object", "not_object")
		return Checkpoint{}, rejection, code
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(trimmed, &fields); err != nil {
		rejection, code := productCheckpointRejection("invalid_field_type", "submit one valid JSON checkpoint object.", "request", "valid JSON object", "malformed_json")
		return Checkpoint{}, rejection, code
	}
	allowed := map[string]bool{"kind": true, "summary": true, "facts": true, "decisions": true, "rejected": true, "evidence_refs": true, "next_action": true, "failed_checks": true}
	for field := range fields {
		if !allowed[field] {
			rejection, code := productCheckpointRejection("invalid_field", "remove fields outside the public checkpoint contract.", field, "one of kind, summary, facts, decisions, rejected, evidence_refs, next_action or failed_checks", "unknown_field")
			return Checkpoint{}, rejection, code
		}
	}
	for _, field := range []string{"kind", "summary", "facts", "decisions", "rejected", "evidence_refs"} {
		value, present := fields[field]
		if !present {
			rejection, code := productCheckpointRejection("missing_required_field", "include every semantic checkpoint field.", field, "field is present; rejected may be an empty array", "missing")
			return Checkpoint{}, rejection, code
		}
		if bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			rejection, code := productCheckpointRejection("invalid_field_type", "use the documented JSON type for every checkpoint field.", field, "non-null public field value", "null")
			return Checkpoint{}, rejection, code
		}
	}
	if rejection, code := validateProductEvidenceRefShape(fields["evidence_refs"]); code != "" {
		return Checkpoint{}, rejection, code
	}
	var request productCheckpointRequest
	decoder := json.NewDecoder(bytes.NewReader(trimmed))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		rejection, code := productCheckpointRejection("invalid_field_type", "use the documented JSON types for checkpoint fields and typed evidence_refs[].", "request", "strict checkpoint object", "type_mismatch")
		return Checkpoint{}, rejection, code
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		rejection, code := productCheckpointRejection("invalid_field_type", "submit exactly one JSON checkpoint object.", "request", "one JSON object", "multiple_values")
		return Checkpoint{}, rejection, code
	}
	if request.Kind != CheckpointProgress && request.Kind != CheckpointQualified {
		rejection, code := productCheckpointRejection("invalid_field_value", "kind must be progress or qualified.", "kind", "progress or qualified", "unsupported_value")
		return Checkpoint{}, rejection, code
	}
	if request.Summary == "" || len(request.Summary) > 512 {
		rejection, code := productCheckpointRejection("invalid_field_value", "summary must be a non-empty sentence of at most 512 characters.", "summary", "non-empty string, max 512 characters", "empty_or_too_long")
		return Checkpoint{}, rejection, code
	}
	for _, item := range []struct {
		name string
		list []string
	}{
		{"facts", request.Facts}, {"decisions", request.Decisions},
	} {
		if len(item.list) == 0 {
			rejection, code := productCheckpointRejection("missing_required_field", "include at least one entry in this checkpoint field.", item.name, "array with at least one string", "empty_array")
			return Checkpoint{}, rejection, code
		}
		if rejection, code := validateProductCheckpointStrings(item.name, item.list, false); code != "" {
			return Checkpoint{}, rejection, code
		}
	}
	if rejection, code := validateProductCheckpointStrings("rejected", request.Rejected, true); code != "" {
		return Checkpoint{}, rejection, code
	}
	if len(request.FailedChecks) > 8 {
		rejection, code := productCheckpointRejection("invalid_field_value", "failed_checks accepts at most eight strings.", "failed_checks", "array with at most eight strings", "too_many_items")
		return Checkpoint{}, rejection, code
	}
	if rejection, code := validateProductCheckpointStrings("failed_checks", request.FailedChecks, true); code != "" {
		return Checkpoint{}, rejection, code
	}
	if request.Kind == CheckpointProgress && request.NextAction == "" {
		rejection, code := productCheckpointRejection("missing_required_field", "progress checkpoints require a non-empty next_action.", "next_action", "non-empty string for progress", "missing_or_empty")
		return Checkpoint{}, rejection, code
	}
	if len(request.NextAction) > 512 {
		rejection, code := productCheckpointRejection("invalid_field_value", "next_action accepts at most 512 characters.", "next_action", "string, max 512 characters", "too_long")
		return Checkpoint{}, rejection, code
	}
	if len(request.EvidenceRefs) == 0 {
		rejection, code := productCheckpointRejection("missing_required_field", "include at least one current workspace_check receipt reference.", "evidence_refs", "array with 1 to 8 typed receipt references", "empty_array")
		return Checkpoint{}, rejection, code
	}
	if len(request.EvidenceRefs) > 8 {
		rejection, code := productCheckpointRejection("invalid_field_value", "evidence_refs accepts at most eight receipt references.", "evidence_refs", "array with 1 to 8 typed receipt references", "too_many_items")
		return Checkpoint{}, rejection, code
	}
	receiptIDs := make([]string, len(request.EvidenceRefs))
	for index, ref := range request.EvidenceRefs {
		field := "evidence_refs[" + itoa(index) + "]"
		if !core.ValidID(ref.ReceiptID) {
			rejection, code := productCheckpointRejection("invalid_receipt_ref", "receipt_id must be a receipt ID returned by the current worker session.", field+".receipt_id", "alphanumeric, underscore or hyphen receipt ID of 1 to 80 characters", "not_receipt_id")
			return Checkpoint{}, rejection, code
		}
		if ref.Proves != productCheckpointEvidencePurpose {
			rejection, code := productCheckpointRejection("invalid_receipt_ref", "proves must identify the current passing workspace_check receipt.", field+".proves", productCheckpointEvidencePurpose, "unsupported_proof")
			return Checkpoint{}, rejection, code
		}
		receiptIDs[index] = ref.ReceiptID
	}
	return Checkpoint{Kind: request.Kind, Summary: request.Summary, Facts: request.Facts, Decisions: request.Decisions, Rejected: request.Rejected, EvidenceRefs: receiptIDs, NextAction: request.NextAction, FailedChecks: request.FailedChecks}, PeerToolRejection{}, ""
}

func validateProductEvidenceRefShape(raw json.RawMessage) (PeerToolRejection, core.Code) {
	var refs []json.RawMessage
	if err := json.Unmarshal(raw, &refs); err != nil {
		rejection, code := productCheckpointRejection("invalid_field_type", "evidence_refs must be an array of typed receipt reference objects.", "evidence_refs", "array of {receipt_id, proves} objects", "not_array")
		return rejection, code
	}
	for index, rawRef := range refs {
		var object map[string]json.RawMessage
		if err := json.Unmarshal(rawRef, &object); err != nil || object == nil {
			rejection, code := productCheckpointRejection("invalid_field_type", "each evidence_refs item must be a typed receipt reference object.", "evidence_refs["+itoa(index)+"]", "object with receipt_id and proves", "not_object")
			return rejection, code
		}
		for _, field := range []string{"receipt_id", "proves"} {
			if _, present := object[field]; !present {
				rejection, code := productCheckpointRejection("missing_required_field", "each evidence_refs item needs receipt_id and proves.", "evidence_refs["+itoa(index)+"]."+field, "required field", "missing")
				return rejection, code
			}
		}
	}
	return PeerToolRejection{}, ""
}

func validateProductCheckpointStrings(field string, values []string, allowEmpty bool) (PeerToolRejection, core.Code) {
	if !allowEmpty && len(values) == 0 {
		rejection, code := productCheckpointRejection("missing_required_field", "include at least one entry in this checkpoint field.", field, "array with at least one string", "empty_array")
		return rejection, code
	}
	if len(values) > 8 {
		rejection, code := productCheckpointRejection("invalid_field_value", "checkpoint arrays accept at most eight strings.", field, "array with at most eight strings", "too_many_items")
		return rejection, code
	}
	for index, value := range values {
		if value == "" || len(value) > 512 {
			rejection, code := productCheckpointRejection("invalid_field_value", "checkpoint array entries must be non-empty strings of at most 512 characters.", field+"["+itoa(index)+"]", "non-empty string, max 512 characters", "empty_or_too_long")
			return rejection, code
		}
	}
	return PeerToolRejection{}, ""
}

func itoa(value int) string {
	if value == 0 {
		return "0"
	}
	result := make([]byte, 0, 4)
	for value > 0 {
		result = append([]byte{byte('0' + value%10)}, result...)
		value /= 10
	}
	return string(result)
}

// pattern: Functional Core
package kernel

import (
	"encoding/json"
	"errors"
	"testing"

	"polis/internal/core"
	"polis/internal/taskvalidation"
)

func TestProductDeliveryFailureClassificationIsStructured(t *testing.T) {
	tests := []struct {
		name   string
		err    error
		reason string
		code   core.Code
	}{
		{name: "writer fenced", err: core.StaleEpoch, reason: "writer_fenced", code: core.Denied},
		{name: "staging failure", err: productDeliveryPhaseError{phase: "staging", err: errors.New("cas unavailable")}, reason: "artifact_staging_failed", code: core.Integrity},
		{name: "publication failure", err: productDeliveryPhaseError{phase: "publication", err: errors.New("database unavailable")}, reason: "artifact_publication_failed", code: core.Integrity},
		{name: "task state", err: core.Denied, reason: "task_state_disallows_delivery", code: core.Denied},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			rejection, code := productDeliveryCodeRejection(test.err)
			if code != test.code || rejection.ReasonCode != test.reason || rejection.TransactionOutcome == "" {
				t.Fatalf("classification=%+v code=%s want reason=%s code=%s with transaction outcome", rejection, code, test.reason, test.code)
			}
		})
	}
}

func TestProductTaskDeliveryRequestAcceptsOnlyExplicitEmptyDecision(t *testing.T) {
	if rejection, code := ParseProductTaskDeliveryRequest([]byte(`{}`)); code != "" || rejection.ReasonCode != "" {
		t.Fatalf("empty delivery request rejected: rejection=%+v code=%s", rejection, code)
	}
	for _, raw := range []string{`{"task_id":"task-1"}`, `{"validation_receipt":"check-1"}`, `null`, `[]`} {
		rejection, code := ParseProductTaskDeliveryRequest([]byte(raw))
		if code == "" || rejection.ReasonCode == "" || rejection.TransactionOutcome != "not_started" {
			t.Fatalf("delivery request %s was not rejected before transaction: rejection=%+v code=%s", raw, rejection, code)
		}
	}
	if _, err := json.Marshal(PeerToolRejection{}); err != nil {
		t.Fatal(err)
	}
}

func TestQualifiedProductDeliveryCheckpointIsDeterministicAndControlPlaneBound(t *testing.T) {
	input := ProductDeliveryCheckpointInput{
		TaskID:            "task-1",
		BindingDigest:     "b123",
		WorkspaceDigest:   "a123",
		WorkspaceRevision: 4,
		SessionID:         "session-1",
		Epoch:             7,
		CheckID:           "check-1",
		RunnerRevision:    taskvalidation.TextContainsAllRunnerRevision,
	}
	checkpoint, err := NewQualifiedProductDeliveryCheckpoint(input)
	if err != nil {
		t.Fatal(err)
	}
	if checkpoint.Kind != CheckpointQualified || checkpoint.Summary == "" || len(checkpoint.Facts) != 1 || len(checkpoint.Decisions) != 1 || checkpoint.Rejected == nil || len(checkpoint.Rejected) != 0 {
		t.Fatalf("unexpected deterministic checkpoint shape: %+v", checkpoint)
	}
	if len(checkpoint.EvidenceRefs) != 1 || checkpoint.EvidenceRefs[0] != input.CheckID || checkpoint.TaskValidationBindingDigest != input.BindingDigest || checkpoint.WorkspaceDigest != input.WorkspaceDigest || checkpoint.WorkspaceRevision != input.WorkspaceRevision || checkpoint.SessionID != input.SessionID || checkpoint.Epoch != input.Epoch || checkpoint.AcceptanceCheckerRevision != input.RunnerRevision || checkpoint.ValidationStatus != string(taskvalidation.StatusPass) || checkpoint.FinalizationState != "current" {
		t.Fatalf("checkpoint did not bind authoritative delivery inputs: %+v", checkpoint)
	}
	second, err := NewQualifiedProductDeliveryCheckpoint(input)
	if err != nil || second.Summary != checkpoint.Summary || second.Decisions[0] != checkpoint.Decisions[0] {
		t.Fatalf("checkpoint generation is not deterministic: first=%+v second=%+v err=%v", checkpoint, second, err)
	}
}

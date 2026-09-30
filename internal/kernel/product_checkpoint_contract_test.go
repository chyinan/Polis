// pattern: Functional Core
package kernel

import (
	"encoding/json"
	"testing"
)

func TestParseProductCheckpointAllowsExplicitNoRejections(t *testing.T) {
	raw, err := json.Marshal(map[string]any{
		"kind": "qualified", "summary": "workspace accepted", "facts": []string{"all required text is present"},
		"decisions": []string{"publish the validated workspace"}, "rejected": []string{},
		"evidence_refs": []map[string]string{{"receipt_id": "check-receipt-1", "proves": "workspace_check_pass"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	checkpoint, rejection, code := parseProductCheckpointRequest(raw)
	if code != "" || rejection.ReasonCode != "" {
		t.Fatalf("valid no-rejection checkpoint rejected: code=%q rejection=%+v", code, rejection)
	}
	if checkpoint.Kind != CheckpointQualified || len(checkpoint.EvidenceRefs) != 1 || checkpoint.EvidenceRefs[0] != "check-receipt-1" || len(checkpoint.Rejected) != 0 {
		t.Fatalf("unexpected parsed checkpoint: %+v", checkpoint)
	}
}

func TestParseProductCheckpointReportsFieldLevelReceiptError(t *testing.T) {
	raw, err := json.Marshal(map[string]any{
		"kind": "qualified", "summary": "workspace accepted", "facts": []string{"all required text is present"},
		"decisions": []string{"publish the validated workspace"}, "rejected": []string{},
		"evidence_refs": []map[string]string{{"receipt_id": "prose is not a receipt", "proves": "workspace_check_pass"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	_, rejection, code := parseProductCheckpointRequest(raw)
	if code != "MALFORMED_INPUT" || rejection.ReasonCode != "invalid_receipt_ref" || rejection.FailingField != "evidence_refs[0].receipt_id" || rejection.ExpectedPublicShape == "" || rejection.ActualCategory == "" {
		t.Fatalf("receipt error is not actionable: code=%q rejection=%+v", code, rejection)
	}
}

func TestParseProductCheckpointRejectsMissingSemanticField(t *testing.T) {
	raw := []byte(`{"kind":"qualified","summary":"workspace accepted","facts":["present"],"decisions":["publish"],"rejected":[],"evidence_refs":[{"receipt_id":"check-receipt-1","proves":"workspace_check_pass"}]}`)
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		t.Fatal(err)
	}
	delete(fields, "decisions")
	raw, err := json.Marshal(fields)
	if err != nil {
		t.Fatal(err)
	}
	_, rejection, code := parseProductCheckpointRequest(raw)
	if code != "MALFORMED_INPUT" || rejection.ReasonCode != "missing_required_field" || rejection.FailingField != "decisions" {
		t.Fatalf("missing field error is not actionable: code=%q rejection=%+v", code, rejection)
	}
}

func TestFrozenLive2MalformedCheckpointShapesRemainSpecificFailures(t *testing.T) {
	base := map[string]any{
		"kind": "qualified", "summary": "workspace accepted", "facts": []string{"all required text is present"},
		"decisions": []string{"publish the validated workspace"}, "rejected": []string{},
		"evidence_refs": []string{"5d571cd58e6dc80707aa66171a1aff5f"},
	}
	cases := []struct {
		name   string
		mutate func(map[string]any)
		field  string
		reason string
	}{
		{name: "live2-call-6-prose-refs", mutate: func(value map[string]any) {
			value["evidence_refs"] = []string{"validation passed", "workspace was replaced"}
		}, field: "evidence_refs[0]", reason: "invalid_field_type"},
		{name: "live2-call-7-string-receipt", mutate: func(value map[string]any) {}, field: "evidence_refs[0]", reason: "invalid_field_type"},
		{name: "live2-call-9-empty-semantic-arrays", mutate: func(value map[string]any) {
			value["kind"] = "progress"
			value["decisions"] = []string{}
			value["rejected"] = []string{}
			value["evidence_refs"] = []string{}
		}, field: "decisions", reason: "missing_required_field"},
		{name: "live2-call-10-prose-receipts", mutate: func(value map[string]any) {
			value["rejected"] = []string{"none"}
			value["evidence_refs"] = []string{"workspace check passed in prose", "replace receipt in prose"}
		}, field: "evidence_refs[0]", reason: "invalid_field_type"},
		{name: "live2-call-11-empty-payload", mutate: func(value map[string]any) {
			value["facts"] = []string{}
			value["decisions"] = []string{}
			value["rejected"] = []string{}
			value["evidence_refs"] = []string{}
		}, field: "facts", reason: "missing_required_field"},
		{name: "live2-call-13-empty-rejected", mutate: func(value map[string]any) {
			value["evidence_refs"] = []string{"5d571cd58e6dc80707aa66171a1aff5f", "562b5c53a66e9e5c8e326d8d5d13a55cbf56504ea70ac1b085b5a7a4efd805f2"}
		}, field: "evidence_refs[0]", reason: "invalid_field_type"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			value := make(map[string]any, len(base))
			for key, item := range base {
				value[key] = item
			}
			testCase.mutate(value)
			raw, err := json.Marshal(value)
			if err != nil {
				t.Fatal(err)
			}
			_, rejection, code := parseProductCheckpointRequest(raw)
			if code != "MALFORMED_INPUT" || rejection.ReasonCode != testCase.reason || rejection.FailingField != testCase.field {
				t.Fatalf("frozen malformed payload changed classification: code=%q rejection=%+v", code, rejection)
			}
		})
	}
}

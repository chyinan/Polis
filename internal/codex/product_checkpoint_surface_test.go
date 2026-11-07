// pattern: Functional Core
package codex

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestProductCheckpointSurfaceExplainsReceiptAndNoRejectionEncoding(t *testing.T) {
	var checkpoint map[string]any
	for _, raw := range ProductEmployeeTools() {
		tool := raw.(map[string]any)
		if tool["name"] == "polis_work_checkpoint" {
			checkpoint = tool
		}
	}
	if checkpoint == nil {
		t.Fatal("product checkpoint tool is missing")
	}
	description := strings.ToLower(checkpoint["description"].(string))
	for _, phrase := range []string{"rejected: []", "receipt_id", "workspace_check", "task_submit"} {
		if !strings.Contains(description, phrase) {
			t.Fatalf("checkpoint description missing %q: %s", phrase, description)
		}
	}
	schema := checkpoint["inputSchema"].(map[string]any)
	properties := schema["properties"].(map[string]any)
	rejected := properties["rejected"].(map[string]any)
	if _, ok := rejected["minItems"]; ok {
		t.Fatalf("rejected must allow the explicit empty array: %#v", rejected)
	}
	evidence := properties["evidence_refs"].(map[string]any)
	items := evidence["items"].(map[string]any)
	if items["type"] != "object" {
		t.Fatalf("evidence_refs items are not typed receipt objects: %#v", items)
	}
	if _, ok := items["properties"].(map[string]any)["receipt_id"]; !ok {
		t.Fatalf("receipt_id is not described: %#v", items)
	}
	encoded, err := json.Marshal(ProductEmployeeTools())
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "MALFORMED_INPUT") {
		t.Fatal("provider-visible schema should describe inputs, not internal error codes")
	}
}

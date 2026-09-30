// pattern: Functional Core
package codex

import (
	"strings"
	"testing"
)

func TestProductSurfaceExposesExplicitTaskDeliveryTool(t *testing.T) {
	var delivery map[string]any
	for _, raw := range ProductEmployeeTools() {
		tool := raw.(map[string]any)
		if tool["name"] == "polis_task_submit" {
			delivery = tool
		}
		if tool["name"] == "polis_artifact_submit" {
			t.Fatal("product surface still exposes the choreography-level artifact_submit tool")
		}
	}
	if delivery == nil {
		t.Fatal("product surface is missing polis_task_submit")
	}
	description := strings.ToLower(delivery["description"].(string))
	for _, phrase := range []string{"validation", "submit", "checkpoint", "artifact", "candidate"} {
		if !strings.Contains(description, phrase) {
			t.Fatalf("task_submit description missing %q: %s", phrase, description)
		}
	}
	schema := delivery["inputSchema"].(map[string]any)
	if got := len(schema["properties"].(map[string]any)); got != 0 {
		t.Fatalf("task_submit requires control-plane-derived fields: %d properties", got)
	}
}

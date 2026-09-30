// pattern: Functional Core
package codex

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestReviewerToolsAreReadOnlyAndDoNotExposeWriterTools(t *testing.T) {
	allowed := map[string]bool{
		"polis_work_current":   true,
		"polis_context_read":   true,
		"polis_workspace_read": true,
		"polis_review_submit":  true,
	}
	for _, raw := range ReviewerTools() {
		tool, ok := raw.(map[string]any)
		if !ok {
			t.Fatalf("reviewer tool has unexpected type %T", raw)
		}
		name, ok := tool["name"].(string)
		if !ok || !allowed[name] {
			t.Fatalf("reviewer tool is not allowlisted: %#v", tool["name"])
		}
		if name == "polis_workspace_replace" || name == "polis_artifact_submit" {
			t.Fatalf("reviewer tool exposes candidate writer: %s", name)
		}
	}
	if len(ReviewerTools()) != len(allowed) {
		t.Fatalf("reviewer tool count changed: got %d want %d", len(ReviewerTools()), len(allowed))
	}
	encoded, err := json.Marshal(ReviewerTools())
	if err != nil {
		t.Fatal(err)
	}
	if string(encoded) == "" {
		t.Fatal("reviewer tool schema is empty")
	}
	if strings.Contains(string(encoded), "verifier") || strings.Contains(string(encoded), "workspace_check") || strings.Contains(string(encoded), "work_checkpoint") {
		t.Fatalf("reviewer schema exposes hidden verifier/checkpoint semantics: %s", encoded)
	}
}

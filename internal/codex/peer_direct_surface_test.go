// pattern: Functional Core
package codex

import "testing"

func TestPeerFrontendSurfaceExposesExplicitDirectSendTarget(t *testing.T) {
	for _, raw := range PeerFrontendToolsWithDirectMessaging() {
		tool, ok := raw.(map[string]any)
		if !ok || tool["name"] != "polis_collab_send" {
			continue
		}
		schema, ok := tool["inputSchema"].(map[string]any)
		if !ok {
			t.Fatal("collab_send has no input schema")
		}
		properties, ok := schema["properties"].(map[string]any)
		if !ok {
			t.Fatal("collab_send has no properties")
		}
		for _, name := range []string{"to_employee_id", "to_task_id", "contract_revision_id", "body", "actionable"} {
			if _, exists := properties[name]; !exists {
				t.Fatalf("collab_send omitted explicit field %q", name)
			}
		}
		return
	}
	t.Fatal("peer frontend tool surface omitted collab_send")
}

func TestHistoricalPeerFrontendSurfaceDoesNotAddDirectSend(t *testing.T) {
	for _, raw := range PeerFrontendTools() {
		tool, ok := raw.(map[string]any)
		if ok && tool["name"] == "polis_collab_send" {
			t.Fatal("historical peer frontend surface changed")
		}
	}
}

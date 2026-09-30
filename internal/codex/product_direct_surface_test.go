// pattern: Functional Core
package codex

import (
	"reflect"
	"testing"
)

func TestProductDirectMessagingSurfaceIsAdditive(t *testing.T) {
	base := ProductEmployeeTools()
	got := ProductEmployeeToolsWithDirectMessaging()
	if len(base) != 7 {
		t.Fatalf("base product surface has %d tools, want the frozen seven-tool contract", len(base))
	}
	if len(got) != len(base)+5 {
		t.Fatalf("direct-message surface has %d tools, want %d", len(got), len(base)+5)
	}
	if !reflect.DeepEqual(got[:len(base)], base) {
		t.Fatal("direct-message surface changed the frozen base product tools")
	}
	want := []string{"polis_collab_send", "polis_collab_inbox", "polis_collab_ack", "polis_collab_apply", "polis_obligation_resolve"}
	for index, raw := range got[len(base):] {
		tool, ok := raw.(map[string]any)
		if !ok || tool["name"] != want[index] {
			t.Fatalf("direct tool %d = %#v, want %q", index, raw, want[index])
		}
	}
}

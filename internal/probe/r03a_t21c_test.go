// pattern: Imperative Shell
package probe

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// Optional evidence export is exclusive-create and never touches T21B.
func TestT21CRevisedCanonicalSurface(t *testing.T) {
	surface, tools, raw, err := buildT21ToolSurface()
	if err != nil {
		t.Fatal(err)
	}
	var roundtrip any
	if err = json.Unmarshal(raw, &roundtrip); err != nil {
		t.Fatal(err)
	}
	again, err := json.Marshal(roundtrip)
	if err != nil || !bytes.Equal(raw, again) {
		t.Fatal("canonical round-trip mismatch")
	}
	_, _, repeated, err := buildT21ToolSurface()
	if err != nil || !bytes.Equal(raw, repeated) {
		t.Fatal("unstable registry serialization")
	}
	base := filepath.Join("..", "..", "evidence", "development", "r0.3a-t21b")
	oldRaw, err := os.ReadFile(filepath.Join(base, "execution-config.json"))
	if err != nil {
		t.Fatal(err)
	}
	var old t21ExecutionConfig
	if err = json.Unmarshal(oldRaw, &old); err != nil {
		t.Fatal(err)
	}
	var previous []map[string]any
	var payload struct {
		Tools []map[string]any `json:"dynamicTools"`
	}
	if err = json.Unmarshal([]byte(old.ThreadStartPayloadCanonicalJSON), &payload); err != nil {
		t.Fatal(err)
	}
	previous = payload.Tools
	if len(previous) != 11 || len(tools) != 11 {
		t.Fatal("exact-11 cardinality mismatch")
	}
	diff := []map[string]any{}
	for i, tool := range tools {
		current := tool.(map[string]any)
		if current["name"] != previous[i]["name"] {
			t.Fatal("name/order drift")
		}
		if surface.Tools[i].BindingIdentity != old.ToolSurface.Tools[i].BindingIdentity || surface.Tools[i].AuthorizationClass != old.ToolSurface.Tools[i].AuthorizationClass {
			t.Fatal("unapproved declared binding drift")
		}
		// JSON normalize maps before exact semantic comparison.
		encoded, _ := json.Marshal(current)
		var normalized map[string]any
		if err = json.Unmarshal(encoded, &normalized); err != nil {
			t.Fatal(err)
		}
		for _, field := range []string{"description", "inputSchema", "type"} {
			if reflect.DeepEqual(normalized[field], previous[i][field]) {
				continue
			}
			name := current["name"].(string)
			approved := field == "description" && (name == "polis_work_current" || name == "polis_context_read" || name == "polis_work_checkpoint" || name == "polis_collab_send") || field == "inputSchema" && (name == "polis_work_checkpoint" || name == "polis_collab_apply" || name == "polis_collab_send")
			diff = append(diff, map[string]any{"name": name, "field": field, "old": previous[i][field], "current": normalized[field], "within_hardening_scope": approved})
			if !approved {
				t.Fatalf("unapproved drift: %s %s", name, field)
			}
		}
	}
	payloadRaw, err := json.Marshal(t21NormalizedThreadPayload(tools, r03aT21DeveloperInstruction))
	if err != nil {
		t.Fatal(err)
	}
	surface.ThreadStartPayloadDigest = digest(payloadRaw)
	surface.ThreadStartPayloadBytes = len(payloadRaw)
	if err = surface.Validate(); err != nil {
		t.Fatal(err)
	}
	bindingsRaw, _ := json.Marshal(surface.Tools)
	revised := old.CanonicalManifest
	revised.ToolSurface = surface
	revised.Base.Combination.CapabilityDigest = t21CapabilityDigest(old.CanonicalManifest.Base.Combination.CapabilityDigest, surface)
	fingerprint := revised.Base.Combination.CurrentFingerprintV7(revised)
	report := map[string]any{"qualification": "R0.3A-T21C", "source": "codex.PeerBackendTools()", "surface": surface, "tools": tools, "tool_names": t21ToolNames(surface), "binding_metadata_digest": digest(bindingsRaw), "binding_metadata_note": "declared handler bindings; runtime policy requires independent tests", "old_manifest_digest": old.ToolManifestDigest, "old_schema_bytes": old.AggregateSchemaBytes, "semantic_diff": diff, "canonical_round_trip": "PASS", "serialization_stability": "PASS", "name_order_uniqueness": "PASS", "unrelated_schema_drift": false, "execution_manifest": revised, "proposed_fingerprint": fingerprint, "qualification_status": "PENDING_OFFLINE_COMPATIBILITY", "medium": 0, "high": 0, "provider_egress": 0}
	if dir := os.Getenv("POLIS_T21C_OFFLINE_EVIDENCE"); dir != "" {
		if err = os.MkdirAll(dir, 0700); err != nil {
			t.Fatal(err)
		}
		for name, value := range map[string]any{"revised-manifest.json": report, "tool-registry.json": tools, "semantic-diff.json": diff} {
			encoded, err := json.MarshalIndent(value, "", "  ")
			if err != nil {
				t.Fatal(err)
			}
			f, err := os.OpenFile(filepath.Join(dir, name), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
			if err != nil {
				t.Fatal(err)
			}
			_, writeErr := f.Write(append(encoded, '\n'))
			syncErr := f.Sync()
			closeErr := f.Close()
			if writeErr != nil || syncErr != nil || closeErr != nil {
				t.Fatalf("evidence write: %v %v %v", writeErr, syncErr, closeErr)
			}
		}
	}
	t.Logf("tools=%d bytes=%d manifest=%s schema=%s fingerprint=%s changes=%d", surface.ToolCount, surface.AggregateSchemaBytes, surface.AggregateManifestDigest, surface.AggregateSchemaDigest, fingerprint.CanonicalManifestDigest, len(diff))
}

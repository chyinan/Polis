package kernel

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"polis/internal/core"
)

func TestVisibleToolDescriptorReadsCachedSchemaWithoutCalling(t *testing.T) {
	frozen, err := freezeToolDescriptor("polis_probe_ping", "Ping the probe.", json.RawMessage(`{"type":"object"}`), nil)
	if err != nil {
		t.Fatal(err)
	}
	qualification, err := json.Marshal(map[string]any{
		"probeStatus": "passed", "toolNames": []string{"polis_probe_ping"}, "toolsCalled": 0, "command": "node",
		"toolDescriptors": []CachedToolDescriptor{frozen},
	})
	if err != nil {
		t.Fatal(err)
	}
	source := toolCatalogSource{ID: "memory-server", Name: "memory", Status: "candidate", DescriptorDigest: "sha256:abc", Qualification: qualification}
	view, err := visibleToolDescriptor("approved", []string{"memory-server"}, []toolCatalogSource{source}, "memory-server:polis_probe_ping")
	if err != nil || view.Description != "Ping the probe." || view.ToolsCalled != 0 || view.Callable || string(view.OutputSchema) != "null" || view.DescriptorDigest != frozen.Digest {
		t.Fatalf("view = %+v %v", view, err)
	}
	encoded, err := json.Marshal(view)
	if err != nil || strings.Contains(string(encoded), "node") || strings.Contains(string(encoded), `"command"`) {
		t.Fatalf("descriptor leaked command: %s %v", encoded, err)
	}
	namesOnly, err := json.Marshal(map[string]any{"probeStatus": "passed", "toolNames": []string{"polis_probe_ping"}, "toolsCalled": 0})
	if err != nil {
		t.Fatal(err)
	}
	source.Qualification = namesOnly
	if _, err = visibleToolDescriptor("approved", []string{"memory-server"}, []toolCatalogSource{source}, "memory-server:polis_probe_ping"); !errors.Is(err, core.OutOfScope) {
		t.Fatalf("missing body err = %v", err)
	}
	source.Status = "revoked"
	source.Qualification = qualification
	if _, err = visibleToolDescriptor("approved", []string{"memory-server"}, []toolCatalogSource{source}, "memory-server:polis_probe_ping"); !errors.Is(err, core.OutOfScope) {
		t.Fatalf("revoked err = %v", err)
	}
	dirty, err := json.Marshal(map[string]any{"probeStatus": "passed", "toolNames": []string{"polis_probe_ping"}, "toolsCalled": 1, "toolDescriptors": []CachedToolDescriptor{frozen}})
	if err != nil {
		t.Fatal(err)
	}
	source.Status = "candidate"
	source.Qualification = dirty
	if _, err = visibleToolDescriptor("approved", []string{"memory-server"}, []toolCatalogSource{source}, "memory-server:polis_probe_ping"); !errors.Is(err, core.Integrity) {
		t.Fatalf("called err = %v", err)
	}
	frozen.Digest = "sha256:" + strings.Repeat("00", 32)
	mismatch, err := json.Marshal(map[string]any{"probeStatus": "passed", "toolNames": []string{"polis_probe_ping"}, "toolsCalled": 0, "toolDescriptors": []CachedToolDescriptor{frozen}})
	if err != nil {
		t.Fatal(err)
	}
	source.Qualification = mismatch
	if _, err = visibleToolDescriptor("approved", []string{"memory-server"}, []toolCatalogSource{source}, "memory-server:polis_probe_ping"); !errors.Is(err, core.Integrity) {
		t.Fatalf("digest err = %v", err)
	}
}

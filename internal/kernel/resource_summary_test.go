package kernel

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"polis/internal/core"
)

func TestVisibleResourcePageReadsCachedNamesWithoutReading(t *testing.T) {
	passed := json.RawMessage(`{"probeStatus":"passed","toolsCalled":0,"resourcesRead":0,"command":"node","resourceSummaries":[{"uri":"polis://probe/b","name":"B"},{"uri":"polis://probe/a","name":"A"}]}`)
	sources := []toolCatalogSource{{ID: "memory-server", Name: "memory", Status: "candidate", DescriptorDigest: "sha256:abc", Qualification: passed}}
	draft, err := visibleResourcePage("draft", []string{"memory-server"}, sources, "memory-server", "", 32)
	if err != nil || len(draft.Items) != 0 {
		t.Fatalf("draft = %+v %v", draft, err)
	}
	revoked := []toolCatalogSource{{ID: "memory-server", Name: "memory", Status: "revoked", DescriptorDigest: "sha256:abc", Qualification: passed}}
	hidden, err := visibleResourcePage("approved", []string{"memory-server"}, revoked, "memory-server", "", 32)
	if err != nil || len(hidden.Items) != 0 {
		t.Fatalf("revoked = %+v %v", hidden, err)
	}
	first, err := visibleResourcePage("approved", []string{"memory-server"}, sources, "memory-server", "", 1)
	if err != nil || len(first.Items) != 1 || first.Items[0].URI != "polis://probe/a" || first.Items[0].ResourcesRead != 0 || first.Items[0].Readable || first.NextCursor == nil {
		t.Fatalf("first = %+v %v", first, err)
	}
	encoded, err := json.Marshal(first.Items[0])
	if err != nil || strings.Contains(string(encoded), "node") || strings.Contains(string(encoded), `"command"`) {
		t.Fatalf("summary leaked command: %s %v", encoded, err)
	}
	second, err := visibleResourcePage("approved", []string{"memory-server"}, sources, "memory-server", *first.NextCursor, 32)
	if err != nil || len(second.Items) != 1 || second.Items[0].Name != "B" || second.NextCursor != nil {
		t.Fatalf("second = %+v %v", second, err)
	}
	namesOnly := []toolCatalogSource{{ID: "memory-server", Name: "memory", Status: "candidate", DescriptorDigest: "sha256:abc", Qualification: json.RawMessage(`{"probeStatus":"passed","toolNames":["polis_probe_ping"],"toolsCalled":0}`)}}
	empty, err := visibleResourcePage("approved", []string{"memory-server"}, namesOnly, "memory-server", "", 32)
	if err != nil || len(empty.Items) != 0 || empty.NextCursor != nil {
		t.Fatalf("names only = %+v %v", empty, err)
	}
	dirty := []toolCatalogSource{{ID: "memory-server", Name: "memory", Status: "candidate", DescriptorDigest: "sha256:abc", Qualification: json.RawMessage(`{"probeStatus":"passed","toolsCalled":0,"resourcesRead":1,"resourceSummaries":[{"uri":"polis://probe/a","name":"A"}]}`)}}
	if _, err = visibleResourcePage("approved", []string{"memory-server"}, dirty, "memory-server", "", 32); !errors.Is(err, core.Integrity) {
		t.Fatalf("read receipt err = %v", err)
	}
}

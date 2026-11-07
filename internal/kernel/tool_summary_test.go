package kernel

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"polis/internal/core"
)

func TestVisibleToolPageReadsCachedNamesWithoutCalling(t *testing.T) {
	passed := json.RawMessage(`{"probeStatus":"passed","toolNames":["create_entities","search_nodes"],"toolsCalled":0,"command":"node"}`)
	sources := []toolCatalogSource{
		{ID: "memory-server", Name: "memory", Status: "candidate", DescriptorDigest: "sha256:abc", Qualification: passed},
		{ID: "other-server", Name: "other", Status: "revoked", DescriptorDigest: "sha256:def", Qualification: passed},
		{ID: "quiet-server", Name: "quiet", Status: "unverified", DescriptorDigest: "sha256:ghi", Qualification: json.RawMessage(`{}`)},
	}
	draft, err := visibleToolPage("draft", []string{"memory-server"}, sources, "", 32)
	if err != nil || len(draft.Items) != 0 || draft.NextCursor != nil {
		t.Fatalf("draft = %+v %v", draft, err)
	}
	first, err := visibleToolPage("approved", []string{"memory-server", "other-server", "quiet-server"}, sources, "", 1)
	if err != nil || len(first.Items) != 1 || first.Items[0].ToolName != "create_entities" || first.Items[0].ToolsCalled != 0 || first.Items[0].Callable || first.NextCursor == nil {
		t.Fatalf("first = %+v %v", first, err)
	}
	encoded, err := json.Marshal(first.Items[0])
	if err != nil || strings.Contains(string(encoded), "node") || strings.Contains(string(encoded), `"command"`) {
		t.Fatalf("summary leaked command: %s %v", encoded, err)
	}
	second, err := visibleToolPage("approved", []string{"memory-server"}, sources, *first.NextCursor, 32)
	if err != nil || len(second.Items) != 1 || second.Items[0].ToolName != "search_nodes" || second.NextCursor != nil {
		t.Fatalf("second = %+v %v", second, err)
	}
	dirty := []toolCatalogSource{{ID: "memory-server", Name: "memory", Status: "candidate", DescriptorDigest: "sha256:abc", Qualification: json.RawMessage(`{"probeStatus":"passed","toolNames":["create_entities"],"toolsCalled":1}`)}}
	if _, err = visibleToolPage("approved", []string{"memory-server"}, dirty, "", 32); !errors.Is(err, core.Integrity) {
		t.Fatalf("dirty cache err = %v", err)
	}
}

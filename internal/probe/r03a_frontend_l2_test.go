// pattern: Functional Core
package probe

import "testing"

func TestR03AFrontendL2HistoricalSurfaceSummary(t *testing.T) {
	old, _, _, err := loadHandoverV4FrontendToolSurface("../../evidence/development/r0.3a-real-frontend-handover-v4/2b7a6fd917d335459b0dbf32dda34158/protocol.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	current, _, _, err := buildR03AFrontendToolSurface()
	if err != nil {
		t.Fatal(err)
	}
	if old.ToolCount != 12 || current.ToolCount != 12 {
		t.Fatalf("unexpected Frontend surface count: old=%d current=%d", old.ToolCount, current.ToolCount)
	}
	t.Logf("old_manifest=%s old_schema=%s old_schema_bytes=%d current_manifest=%s current_schema=%s current_schema_bytes=%d", old.AggregateManifestDigest, old.AggregateSchemaDigest, old.AggregateSchemaBytes, current.AggregateManifestDigest, current.AggregateSchemaDigest, current.AggregateSchemaBytes)
}

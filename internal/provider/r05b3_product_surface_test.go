package provider

import (
	"encoding/json"
	"testing"
)

func TestR05B3ProductSurfaceIsVersionedAndGeneric(t *testing.T) {
	surface := ProductToolSurface()
	const b2Manifest = "730b9b2aba9de7ca015e800b1a597de37a22bd0c299dc7ad10e022c8399388d9"
	if surface.ManifestDigest == b2Manifest {
		t.Fatal("B3 semantic remediation must not reuse the stale B2 provider-visible manifest")
	}
	if surface.ToolCount == 0 || surface.AggregateSchemaDigest == "" || surface.AggregateSchemaBytes == 0 {
		t.Fatalf("incomplete product tool surface metadata: %+v", surface)
	}
	for _, value := range surface.Tools {
		tool := value.(map[string]any)
		raw, err := json.Marshal(tool)
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("product_tool=%s exact=%s", tool["name"], raw)
	}
	t.Logf("tool_count=%d manifest=%s aggregate_schema_digest=%s aggregate_schema_bytes=%d old_b2_manifest=%s provider_surface_changed=true qualification=%s", surface.ToolCount, surface.ManifestDigest, surface.AggregateSchemaDigest, surface.AggregateSchemaBytes, b2Manifest, ProductToolSurfaceQualification)
}

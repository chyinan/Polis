// pattern: Imperative Shell
package provider

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestCurrentProductRuntimeSurfaceHasExactV4Identity(t *testing.T) {
	const (
		wantManifest = "60192ba130e504ad380fd524ec02ab536a8724723098f0d3fb91ecc0f3983ea4"
		wantSchema   = "5ee61b10d249a11614d176fda5c3df86f26e0b650c1074b705dc48b4405c35cc"
		wantB2       = "2b403fc0f3c9a1513473828e66203becb7ac5202f298f917034fd95929a9f3b9"
	)
	wantNames := []string{
		"polis_work_current",
		"polis_context_read",
		"polis_workspace_read",
		"polis_workspace_replace",
		"polis_workspace_check",
		"polis_work_checkpoint",
		"polis_task_submit",
	}

	productionRuntime := NewCodexRuntime(CodexRuntimeConfig{ToolSurface: ProductToolSurface()})
	surface := productionRuntime.ToolSurface()
	if surface.ToolCount != 7 || surface.ManifestDigest != wantManifest || surface.AggregateSchemaBytes != 2206 || surface.AggregateSchemaDigest != wantSchema {
		t.Fatalf("current product runtime surface drifted: count=%d manifest=%s schema_bytes=%d schema_digest=%s", surface.ToolCount, surface.ManifestDigest, surface.AggregateSchemaBytes, surface.AggregateSchemaDigest)
	}
	if surface.ManifestDigest == wantB2 || ProductToolSurfaceQualification != "polis-product-tool-surface@4" {
		t.Fatalf("current surface reused stale qualification: surface=%s manifest=%s", ProductToolSurfaceQualification, surface.ManifestDigest)
	}
	if again := ProductToolSurface(); !reflect.DeepEqual(surface, again) {
		t.Fatal("product runtime surface materialization is not deterministic")
	}
	if len(surface.Tools) != len(wantNames) {
		t.Fatalf("registered tools=%d want=%d", len(surface.Tools), len(wantNames))
	}
	for index, value := range surface.Tools {
		tool, ok := value.(map[string]any)
		if !ok || tool["name"] != wantNames[index] {
			t.Fatalf("tool %d=%v want name %s", index+1, tool, wantNames[index])
		}
		raw, err := json.Marshal(tool)
		if err != nil {
			t.Fatal(err)
		}
		definition := strings.ToLower(string(raw))
		for _, forbidden := range []string{"formatter.go", "formatting-only", "pagination", "frozen probe", "smoke-specific", "non-empty fake acceptance"} {
			if strings.Contains(definition, forbidden) {
				t.Fatalf("tool %s contains stale/smoke-specific semantics %q: %s", wantNames[index], forbidden, raw)
			}
		}
		t.Logf("product_tool=%s exact=%s", wantNames[index], raw)
	}
	t.Logf("surface_id=%s tool_count=%d manifest=%s aggregate_schema_bytes=%d aggregate_schema_digest=%s historical_b2=%s status=STALE", ProductToolSurfaceQualification, surface.ToolCount, surface.ManifestDigest, surface.AggregateSchemaBytes, surface.AggregateSchemaDigest, wantB2)
}

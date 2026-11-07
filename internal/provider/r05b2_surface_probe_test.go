// pattern: Functional Core
package provider

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"testing"

	"polis/internal/codex"
)

func TestR05B2FrozenLegacySurfaceMatchesRecordedDigest(t *testing.T) {
	surface := ToolSurfaceFromTools(codex.Tools())
	expectedNames := []string{"polis_work_current", "polis_context_read", "polis_workspace_read", "polis_workspace_replace", "polis_workspace_check", "polis_work_checkpoint", "polis_artifact_submit"}
	if surface.ToolCount != len(expectedNames) {
		t.Fatalf("product tool count=%d, want %d", surface.ToolCount, len(expectedNames))
	}
	seen := make(map[string]struct{}, len(surface.Tools))
	for index, value := range surface.Tools {
		tool, ok := value.(map[string]any)
		if !ok {
			t.Fatalf("tool %d is not an object: %T", index+1, value)
		}
		name, ok := tool["name"].(string)
		if !ok || name != expectedNames[index] {
			t.Fatalf("tool registration %d=%q, want %q", index+1, name, expectedNames[index])
		}
		if _, exists := seen[name]; exists {
			t.Fatalf("duplicate product tool name %q", name)
		}
		seen[name] = struct{}{}
	}
	raw, err := json.Marshal(surface.Tools)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("current_product_tool_count=%d", surface.ToolCount)
	t.Logf("current_product_manifest_digest=%s", surface.ManifestDigest)
	var schemas []json.RawMessage
	type toolEntry struct {
		Name                string          `json:"name"`
		Description         string          `json:"description"`
		Schema              json.RawMessage `json:"schema"`
		SchemaBytes         int             `json:"schema_bytes"`
		SchemaDigest        string          `json:"schema_digest"`
		DescriptionDigest   string          `json:"description_digest"`
		RegistrationOrdinal int             `json:"registration_ordinal"`
	}
	entries := make([]toolEntry, 0, len(surface.Tools))
	for index, value := range surface.Tools {
		tool, ok := value.(map[string]any)
		if !ok {
			t.Fatalf("tool %d is not an object: %T", index+1, value)
		}
		schemaRaw, marshalErr := json.Marshal(tool["inputSchema"])
		if marshalErr != nil {
			t.Fatal(marshalErr)
		}
		description, ok := tool["description"].(string)
		if !ok {
			t.Fatalf("tool %d has no description", index+1)
		}
		schemaHash := sha256.Sum256(schemaRaw)
		descriptionHash := sha256.Sum256([]byte(description))
		entries = append(entries, toolEntry{Name: tool["name"].(string), Description: description, Schema: schemaRaw, SchemaBytes: len(schemaRaw), SchemaDigest: hex.EncodeToString(schemaHash[:]), DescriptionDigest: hex.EncodeToString(descriptionHash[:]), RegistrationOrdinal: index + 1})
		schemas = append(schemas, schemaRaw)
	}
	aggregateSchemaRaw, err := json.Marshal(schemas)
	if err != nil {
		t.Fatal(err)
	}
	aggregateSchemaHash := sha256.Sum256(aggregateSchemaRaw)
	schemaBytes := 0
	for _, entry := range entries {
		schemaBytes += entry.SchemaBytes
	}
	report := map[string]any{
		"schema_version":                  "r05b2-legacy-product-tool-surface@1",
		"source":                          "codex.Tools (frozen B2 manifest; no longer product-registered)",
		"tool_count":                      surface.ToolCount,
		"tool_manifest_digest":            surface.ManifestDigest,
		"tool_manifest_bytes":             len(raw),
		"aggregate_schema_digest":         hex.EncodeToString(aggregateSchemaHash[:]),
		"aggregate_schema_bytes":          schemaBytes,
		"aggregate_schema_digest_method":  "sha256(json.Marshal(ordered []json.RawMessage inputSchema)); bytes=sum(len(json.Marshal(each inputSchema))",
		"tools":                           entries,
		"exact_provider_visible_manifest": json.RawMessage(raw),
	}
	reportRaw, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("exact_product_surface=%s", reportRaw)
	const authorizedDigest = "730b9b2aba9de7ca015e800b1a597de37a22bd0c299dc7ad10e022c8399388d9"
	if surface.ToolCount != 7 || surface.ManifestDigest != authorizedDigest {
		t.Fatalf("current production product surface drift: tool_count=%d manifest=%s expected_tool_count=7 expected_manifest=%s", surface.ToolCount, surface.ManifestDigest, authorizedDigest)
	}
}

func TestR05B2FrozenLegacySurfaceReportsSchemaDigestSeparately(t *testing.T) {
	surface := ToolSurfaceFromTools(codex.Tools())
	const expectedSchemaDigest = "12008e917b4b5a71dc5fa70e8c2388450205bc08fce1bddc84bae3742e155717"
	if surface.AggregateSchemaDigest != expectedSchemaDigest || surface.AggregateSchemaBytes != 1318 {
		t.Fatalf("product aggregate schema=%s/%d, want %s/1318 (manifest digest is %s)", surface.AggregateSchemaDigest, surface.AggregateSchemaBytes, expectedSchemaDigest, surface.ManifestDigest)
	}
}

func TestR05B2AggregateToolSchemaMetadataRejectsMalformedRegistration(t *testing.T) {
	for name, tools := range map[string][]any{
		"non-object tool":   {"not-a-tool"},
		"missing schema":    {map[string]any{"name": "polis_missing_schema"}},
		"non-object schema": {map[string]any{"name": "polis_bad_schema", "inputSchema": "not-an-object"}},
	} {
		t.Run(name, func(t *testing.T) {
			if _, _, err := AggregateToolSchemaMetadata(tools); err == nil {
				t.Fatal("malformed tool registration produced aggregate schema metadata")
			}
		})
	}
}

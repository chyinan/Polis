// pattern: Imperative Shell
package control

import (
	"encoding/json"
	"errors"
	"testing"

	"polis/internal/core"
)

func TestParseControlledMCPCallArgumentsRequiresExactBoundToolRequest(t *testing.T) {
	raw := json.RawMessage(`{"capability_id":"mcp-1","tool_name":"lookup","tool_schema_sha256":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","arguments":{"key":"sample"}}`)
	parsed, err := parseControlledMCPCallArguments(raw)
	if err != nil || parsed.CapabilityID != "mcp-1" || parsed.ToolName != "lookup" ||
		parsed.ToolSchemaSHA256 != string(repeatBytes('a', 64)) || string(parsed.Arguments) != `{"key":"sample"}` {
		t.Fatalf("valid controlled MCP call parsed=(%+v,%v)", parsed, err)
	}
	for _, invalid := range []string{
		`{"capability_id":"mcp-1","tool_name":"lookup","tool_schema_sha256":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","arguments":{},"command":"node"}`,
		`{"capability_id":"mcp-1","tool_name":"lookup","tool_schema_sha256":"bad","arguments":{}}`,
		`[]`,
		`{"capability_id":"mcp-1","tool_name":"lookup","tool_schema_sha256":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","arguments":{}} {}`,
	} {
		if _, err = parseControlledMCPCallArguments(json.RawMessage(invalid)); !errors.Is(err, core.Malformed) {
			t.Errorf("invalid controlled MCP call %s error=%v, want %s", invalid, err, core.Malformed)
		}
	}
}

func repeatBytes(value byte, count int) []byte {
	result := make([]byte, count)
	for index := range result {
		result[index] = value
	}
	return result
}

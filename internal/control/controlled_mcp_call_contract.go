// pattern: Functional Core
package control

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"io"
	"strings"

	"polis/internal/core"
	"polis/internal/mcptransport"
)

type controlledMCPCallArguments struct {
	CapabilityID     string          `json:"capability_id"`
	ToolName         string          `json:"tool_name"`
	ToolSchemaSHA256 string          `json:"tool_schema_sha256"`
	Arguments        json.RawMessage `json:"arguments"`
}

func parseControlledMCPCallArguments(raw json.RawMessage) (controlledMCPCallArguments, error) {
	var parsed controlledMCPCallArguments
	if len(raw) == 0 || len(raw) > mcptransport.MaxStdioCallArgumentsBytes+2048 {
		return parsed, core.Malformed
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&parsed); err != nil {
		return controlledMCPCallArguments{}, core.Malformed
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return controlledMCPCallArguments{}, core.Malformed
	}
	if !core.ValidID(parsed.CapabilityID) || !validControlledMCPToolName(parsed.ToolName) || !validControlledMCPDigest(parsed.ToolSchemaSHA256) ||
		len(parsed.Arguments) == 0 || len(parsed.Arguments) > mcptransport.MaxStdioCallArgumentsBytes || !json.Valid(parsed.Arguments) {
		return controlledMCPCallArguments{}, core.Malformed
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal(parsed.Arguments, &object); err != nil || object == nil {
		return controlledMCPCallArguments{}, core.Malformed
	}
	parsed.Arguments = append(json.RawMessage(nil), parsed.Arguments...)
	return parsed, nil
}

func validControlledMCPToolName(value string) bool {
	if len(value) == 0 || len(value) > mcptransport.MaxStdioToolNameBytes {
		return false
	}
	for _, char := range value {
		if (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') || (char >= '0' && char <= '9') || char == '_' || char == '-' || char == '.' {
			continue
		}
		return false
	}
	return true
}

func validControlledMCPDigest(value string) bool {
	if len(value) != 64 {
		return false
	}
	decoded, err := hex.DecodeString(value)
	if err != nil || len(decoded) != 32 {
		return false
	}
	return value == strings.ToLower(value)
}

// pattern: Functional Core
package kernel

import (
	"bytes"
	"encoding/json"
	"errors"
	"unicode/utf8"

	"polis/internal/core"
	"polis/internal/mcptransport"
)

const maxPersistedStdioToolResultBytes = 72 << 10

func validateStdioMCPToolCallIntent(input StdioMCPToolCallIntentInput) error {
	if !validMCPProviderCallID(input.ProviderCallID) || input.CapabilityID == "" || !validStdioMCPToolName(input.ToolName) || !validCapabilityDigest(input.ToolSchemaSHA256) {
		return errors.New("stdio MCP call identity is invalid")
	}
	if len(input.Arguments) == 0 || len(input.Arguments) > mcptransport.MaxStdioCallArgumentsBytes || !json.Valid(input.Arguments) {
		return errors.New("stdio MCP call arguments are invalid or oversized")
	}
	decoder := json.NewDecoder(bytes.NewReader(input.Arguments))
	decoder.UseNumber()
	var object map[string]json.RawMessage
	if err := decoder.Decode(&object); err != nil || object == nil {
		return errors.New("stdio MCP call arguments must be a JSON object")
	}
	return nil
}

func validateStdioMCPToolResult(record StdioMCPToolCallRecord, result mcptransport.StdioToolResult) ([]byte, string, error) {
	if result.ToolName != record.ToolName || result.ToolSchemaSHA256 != record.ToolSchemaSHA256 ||
		(result.ContentBoundary != mcptransport.StdioResultContentBoundary && result.ContentBoundary != mcptransport.StreamableHTTPResultContentBoundary) || len(result.Content) == 0 {
		return nil, "", errors.New("MCP result does not match the reserved untrusted text boundary")
	}
	totalTextBytes := 0
	for _, content := range result.Content {
		if !utf8.ValidString(content.Text) {
			return nil, "", errors.New("stdio MCP result text is not valid UTF-8")
		}
		totalTextBytes += len(content.Text)
		if totalTextBytes > mcptransport.MaxStdioToolResultBytes {
			return nil, "", errors.New("stdio MCP result exceeds its content bound")
		}
	}
	encoded, err := json.Marshal(result)
	if err != nil || len(encoded) > maxPersistedStdioToolResultBytes {
		return nil, "", errors.New("stdio MCP result exceeds its persistence bound")
	}
	return encoded, digestCapabilityBytes(encoded), nil
}

func validateStdioMCPToolCallStart(record StdioMCPToolCallRecord, input StdioMCPToolCallIntentInput, authorization StdioMCPToolAuthorization, binding Binding) error {
	if record.Status != "dispatching" || record.CompanyID != binding.scope.company || record.SessionID != binding.session || record.EmployeeID != binding.employee ||
		record.ProviderCallID != input.ProviderCallID || record.CapabilityID != input.CapabilityID || record.ToolName != input.ToolName ||
		record.ToolSchemaSHA256 != input.ToolSchemaSHA256 || record.ArgumentsSHA256 != digestCapabilityBytes(input.Arguments) ||
		record.RuntimeQualificationID != authorization.RuntimeQualification.RuntimeQualificationID ||
		record.CompanyID != authorization.RuntimeQualification.CompanyID {
		return core.Denied
	}
	return nil
}

func validateStdioMCPToolCallOwner(record StdioMCPToolCallRecord, binding Binding) error {
	if record.CompanyID != binding.scope.company || record.SessionID != binding.session || record.EmployeeID != binding.employee {
		return core.Denied
	}
	return nil
}

func validMCPProviderCallID(value string) bool {
	if len(value) == 0 || len(value) > 128 {
		return false
	}
	for _, char := range value {
		if (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') || (char >= '0' && char <= '9') || char == '_' || char == '-' || char == '.' || char == ':' {
			continue
		}
		return false
	}
	return true
}

func validStdioMCPToolName(value string) bool {
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

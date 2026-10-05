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
	if record.Status != "dispatching" || record.CompanyID != binding.scope.company || record.SessionID != binding.session || record.EmployeeID != binding.employee || record.TaskID != binding.task ||
		record.ProviderCallID != input.ProviderCallID || record.CapabilityID != input.CapabilityID || record.ToolName != input.ToolName ||
		record.ToolSchemaSHA256 != input.ToolSchemaSHA256 || record.ArgumentsSHA256 != digestCapabilityBytes(input.Arguments) ||
		record.RuntimeQualificationID != authorization.RuntimeQualification.RuntimeQualificationID || record.EmployeeEpoch != binding.epoch ||
		record.GrantRevision != authorization.GrantRevision || record.TargetSHA256 != authorization.TargetSHA256 ||
		record.InputSHA256 != digestCapabilityBytes(input.Arguments) ||
		record.DispatchPermitID != stableCapabilityID("mcp-permit", binding.scope.company, binding.session, input.ProviderCallID) ||
		record.AttemptID != stableCapabilityID("mcp-attempt", binding.scope.company, binding.session, input.ProviderCallID) ||
		record.CompanyID != authorization.RuntimeQualification.CompanyID {
		return core.Denied
	}
	return nil
}

func ValidateStdioMCPToolCallStart(record StdioMCPToolCallRecord, input StdioMCPToolCallIntentInput, authorization StdioMCPToolAuthorization, binding Binding) error {
	return validateStdioMCPToolCallStart(record, input, authorization, binding)
}

func validateStdioMCPToolDispatchPermit(permit StdioMCPToolDispatchPermit, input StdioMCPToolCallIntentInput, authorization StdioMCPToolAuthorization, binding Binding) error {
	if permit.Status != "issued" || permit.CompanyID != binding.scope.company || permit.SessionID != binding.session || permit.TaskID != binding.task ||
		permit.EmployeeID != binding.employee || permit.EmployeeEpoch != binding.epoch || permit.GrantRevision != authorization.GrantRevision ||
		permit.CapabilityID != input.CapabilityID || permit.CapabilityVersion != authorization.RuntimeQualification.VersionDigest ||
		permit.CapabilityQualificationID != authorization.RuntimeQualification.CapabilityQualificationID ||
		permit.RuntimeQualificationID != authorization.RuntimeQualification.RuntimeQualificationID || permit.Transport != authorization.Transport ||
		permit.ProviderCallID != input.ProviderCallID || permit.ToolName != input.ToolName || permit.ToolSchemaSHA256 != input.ToolSchemaSHA256 ||
		permit.TargetSHA256 != authorization.TargetSHA256 || permit.InputSHA256 != digestCapabilityBytes(input.Arguments) ||
		permit.ActionID != stableCapabilityID("mcp-call", binding.scope.company, binding.session, input.ProviderCallID) ||
		permit.AttemptID != stableCapabilityID("mcp-attempt", binding.scope.company, binding.session, input.ProviderCallID) ||
		permit.PermitID != stableCapabilityID("mcp-permit", binding.scope.company, binding.session, input.ProviderCallID) {
		return core.Denied
	}
	return nil
}

func ValidateStdioMCPToolDispatchPermit(permit StdioMCPToolDispatchPermit, input StdioMCPToolCallIntentInput, authorization StdioMCPToolAuthorization, binding Binding) error {
	return validateStdioMCPToolDispatchPermit(permit, input, authorization, binding)
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

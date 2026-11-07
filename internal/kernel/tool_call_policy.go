// pattern: Functional Core
package kernel

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"

	"polis/internal/core"
)

const (
	toolCallPolicyRevision = "r15-tools-call@1"
	maxToolCallArguments   = 64 * 1024
	maxToolCallResult      = 256 * 1024
	maxToolCallStderr      = 4096
)

type ToolCallRequest struct {
	CompanyID         string          `json:"companyId"`
	EmployeeID        string          `json:"employeeId"`
	TaskID            string          `json:"taskId,omitempty"`
	BindingID         string          `json:"bindingId"`
	Handle            string          `json:"handle"`
	DescriptorDigest  string          `json:"descriptorDigest"`
	Arguments         json.RawMessage `json:"arguments"`
	IdempotencyKey    string          `json:"idempotencyKey"`
	PolicyRevision    string          `json:"policyRevision"`
	AuthorizationRef  string          `json:"authorizationRef,omitempty"`
	EffectClassHint   string          `json:"effectClassHint,omitempty"`
}

type MCPToolCallQualification struct {
	CompanyID        string  `json:"companyId"`
	ID               string  `json:"id"`
	BindingID        string  `json:"bindingId"`
	DefinitionID     string  `json:"definitionId"`
	ServerID         string  `json:"serverId"`
	ToolName         string  `json:"toolName"`
	Handle           string  `json:"handle"`
	DescriptorDigest string  `json:"descriptorDigest"`
	PolicyRevision   string  `json:"policyRevision"`
	EffectClass      string  `json:"effectClass"`
	Status           string  `json:"status"`
	AuthorizationRef string  `json:"authorizationRef"`
	Transport        string  `json:"transport"`
	QualifiedBy      string  `json:"qualifiedBy"`
	QualifiedAt      *string `json:"qualifiedAt"`
	ExpiresAt        *string `json:"expiresAt"`
	Notes            string  `json:"notes"`
	CreatedAt        string  `json:"createdAt"`
	UpdatedAt        string  `json:"updatedAt"`
}

type MCPToolCallReceipt struct {
	CompanyID        string          `json:"companyId"`
	ID               string          `json:"id"`
	EmployeeID       string          `json:"employeeId"`
	TaskID           *string         `json:"taskId,omitempty"`
	BindingID        string          `json:"bindingId"`
	DefinitionID     string          `json:"definitionId"`
	QualificationID  *string         `json:"qualificationId,omitempty"`
	Handle           string          `json:"handle"`
	DescriptorDigest string          `json:"descriptorDigest"`
	ArgumentsDigest  string          `json:"argumentsDigest"`
	RequestDigest    string          `json:"requestDigest"`
	IdempotencyKey   string          `json:"idempotencyKey"`
	PolicyRevision   string          `json:"policyRevision"`
	AuthorizationRef string          `json:"authorizationRef"`
	EffectClass      string          `json:"effectClass"`
	Outcome          string          `json:"outcome"`
	Code             string          `json:"code"`
	ResultDigest     string          `json:"resultDigest"`
	ResultJSON       json.RawMessage `json:"resultJson,omitempty"`
	ErrorJSON        json.RawMessage `json:"errorJson,omitempty"`
	ProcessObserved  json.RawMessage `json:"processObserved"`
	EgressObserved   json.RawMessage `json:"egressObserved"`
	StartedAt        *string         `json:"startedAt,omitempty"`
	FinishedAt       *string         `json:"finishedAt,omitempty"`
	CreatedAt        string          `json:"createdAt"`
}

type ToolCallGateInput struct {
	Request            ToolCallRequest
	BindingStatus      string
	BindingFound       bool
	BindingSkillIDs    []string
	BindingMCPIDs      []string
	Definition         *toolCatalogSource
	Descriptor         *CachedToolDescriptor
	Qualification      *MCPToolCallQualification
}

type ToolCallGateResult struct {
	OK           bool
	Code         string
	DefinitionID string
	ServerID     string
	ToolName     string
	EffectClass  string
}

var allowedToolCallEffectClasses = map[string]struct{}{"read": {}, "idempotent": {}, "effectful": {}, "unknown": {}}
var allowedToolCallQualificationStatuses = map[string]struct{}{"draft": {}, "approved": {}, "disabled": {}, "revoked": {}, "expired": {}}

func validateToolCallRequest(req ToolCallRequest) (ToolCallRequest, error) {
	if !core.ValidID(req.CompanyID) {
		return req, core.Malformed
	}
	if !validEmployeeID(req.EmployeeID) {
		return req, core.Denied
	}
	if strings.TrimSpace(req.TaskID) != "" && !core.ValidID(req.TaskID) {
		return req, core.Malformed
	}
	if !core.ValidID(req.BindingID) || req.DescriptorDigest == "" || strings.TrimSpace(req.IdempotencyKey) == "" {
		return req, core.Malformed
	}
	if len(req.IdempotencyKey) > 256 || strings.ContainsAny(req.IdempotencyKey, "\r\n\t") {
		return req, core.Malformed
	}
	if strings.TrimSpace(req.PolicyRevision) == "" || len(req.PolicyRevision) > 128 {
		return req, core.Malformed
	}
	serverID, toolName, err := splitToolHandle(req.Handle)
	if err != nil {
		return req, err
	}
	_ = serverID
	_ = toolName
	if len(req.Arguments) == 0 {
		req.Arguments = json.RawMessage(`{}`)
	}
	if !json.Valid(req.Arguments) {
		return req, core.Malformed
	}
	if len(req.Arguments) > maxToolCallArguments {
		return req, core.TooLarge
	}
	var args any
	if err = json.Unmarshal(req.Arguments, &args); err != nil {
		return req, core.Malformed
	}
	if _, ok := args.(map[string]any); !ok {
		return req, core.Malformed
	}
	if req.EffectClassHint != "" {
		if _, ok := allowedToolCallEffectClasses[req.EffectClassHint]; !ok {
			return req, core.Malformed
		}
	}
	return req, nil
}

func toolCallRequestDigest(req ToolCallRequest) string {
	canonicalArgs, _ := marshalCanonicalJSON(req.Arguments)
	encoded, _ := json.Marshal(struct {
		CompanyID        string          `json:"companyId"`
		EmployeeID       string          `json:"employeeId"`
		TaskID           string          `json:"taskId"`
		BindingID        string          `json:"bindingId"`
		Handle           string          `json:"handle"`
		DescriptorDigest string          `json:"descriptorDigest"`
		Arguments        json.RawMessage `json:"arguments"`
		PolicyRevision   string          `json:"policyRevision"`
		AuthorizationRef string          `json:"authorizationRef"`
	}{req.CompanyID, req.EmployeeID, strings.TrimSpace(req.TaskID), req.BindingID, req.Handle, req.DescriptorDigest, canonicalArgs, req.PolicyRevision, req.AuthorizationRef})
	sum := sha256.Sum256(encoded)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func toolCallArgumentsDigest(arguments json.RawMessage) string {
	canonical, err := marshalCanonicalJSON(arguments)
	if err != nil {
		canonical = []byte(`{}`)
	}
	sum := sha256.Sum256(canonical)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func marshalCanonicalJSON(raw json.RawMessage) (json.RawMessage, error) {
	if len(raw) == 0 {
		return json.RawMessage(`{}`), nil
	}
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil, err
	}
	return marshalCanonical(value)
}

func toolCallGate(input ToolCallGateInput) ToolCallGateResult {
	serverID, toolName, err := splitToolHandle(input.Request.Handle)
	if err != nil {
		return ToolCallGateResult{Code: "MALFORMED"}
	}
	result := ToolCallGateResult{ServerID: serverID, ToolName: toolName, EffectClass: "unknown"}
	if !input.BindingFound {
		return ToolCallGateResult{Code: "BINDING_NOT_APPROVED", ServerID: serverID, ToolName: toolName}
	}
	switch input.BindingStatus {
	case "approved":
	case "draft":
		return denyGate(result, "BINDING_NOT_APPROVED")
	case "disabled":
		return denyGate(result, "BINDING_DISABLED")
	case "revoked":
		return denyGate(result, "BINDING_REVOKED")
	default:
		return denyGate(result, "BINDING_NOT_APPROVED")
	}
	bound := false
	for _, id := range input.BindingMCPIDs {
		if id == serverID {
			bound = true
			break
		}
	}
	if !bound {
		return denyGate(result, "BINDING_TARGET_MISSING")
	}
	if input.Definition == nil {
		return denyGate(result, "DEFINITION_UNVERIFIED")
	}
	if input.Definition.ID != serverID {
		return denyGate(result, "MALFORMED")
	}
	result.DefinitionID = input.Definition.ID
	switch input.Definition.Status {
	case "revoked":
		return denyGate(result, "DEFINITION_REVOKED")
	case "unverified", "":
		return denyGate(result, "DEFINITION_UNVERIFIED")
	}
	if input.Descriptor == nil {
		return denyGate(result, "DESCRIPTOR_MISSING")
	}
	if input.Request.DescriptorDigest != input.Descriptor.Digest {
		return denyGate(result, "DESCRIPTOR_DIGEST_MISMATCH")
	}
	if err = validateArgumentsAgainstSchema(input.Descriptor.InputSchema, input.Request.Arguments); err != nil {
		if errors.Is(err, core.TooLarge) {
			return denyGate(result, "ARGUMENTS_TOO_LARGE")
		}
		return denyGate(result, "SCHEMA_INVALID")
	}
	if input.Qualification == nil {
		return denyGate(result, "CALL_NOT_QUALIFIED")
	}
	q := input.Qualification
	if q.BindingID != input.Request.BindingID || q.DefinitionID != input.Definition.ID || q.Handle != input.Request.Handle || q.DescriptorDigest != input.Request.DescriptorDigest || q.PolicyRevision != input.Request.PolicyRevision {
		return denyGate(result, "CALL_NOT_QUALIFIED")
	}
	switch q.Status {
	case "approved":
	case "expired":
		return denyGate(result, "CALL_QUALIFICATION_EXPIRED")
	case "revoked", "disabled", "draft":
		return denyGate(result, "CALL_QUALIFICATION_REVOKED")
	default:
		return denyGate(result, "CALL_NOT_QUALIFIED")
	}
	if q.Transport != "stdio" {
		return denyGate(result, "TRANSPORT_NOT_QUALIFIED")
	}
	if q.EffectClass != "" {
		result.EffectClass = q.EffectClass
	}
	if q.EffectClass == "effectful" || q.EffectClass == "unknown" {
		if strings.TrimSpace(input.Request.AuthorizationRef) == "" && strings.TrimSpace(q.AuthorizationRef) == "" {
			return denyGate(result, "AUTHORIZATION_REQUIRED")
		}
	}
	result.OK = true
	return result
}

func denyGate(result ToolCallGateResult, code string) ToolCallGateResult {
	result.OK = false
	result.Code = code
	return result
}

func validateArgumentsAgainstSchema(schemaRaw, argsRaw json.RawMessage) error {
	if len(argsRaw) > maxToolCallArguments {
		return core.TooLarge
	}
	if len(schemaRaw) == 0 || string(bytes.TrimSpace(schemaRaw)) == "null" || string(bytes.TrimSpace(schemaRaw)) == "{}" {
		return nil
	}
	// Minimal structural validation: schema must be an object and arguments must be
	// a JSON object (enforced by caller). Full JSON Schema validation is deferred
	// to a dedicated validator; unknown/complex keywords fail closed only on
	// malformed schema, not on unsupported keywords.
	var schema any
	if err := json.Unmarshal(schemaRaw, &schema); err != nil {
		return core.Malformed
	}
	schemaObj, ok := schema.(map[string]any)
	if !ok {
		return core.Malformed
	}
	var args any
	if err := json.Unmarshal(argsRaw, &args); err != nil {
		return core.Malformed
	}
	argsObj, ok := args.(map[string]any)
	if !ok {
		return core.Malformed
	}
	if req, ok := schemaObj["required"].([]any); ok {
		for _, item := range req {
			name, ok := item.(string)
			if !ok {
				continue
			}
			if _, exists := argsObj[name]; !exists {
				return core.Denied
			}
		}
	}
	if props, ok := schemaObj["properties"].(map[string]any); ok && schemaObj["additionalProperties"] == false {
		for key := range argsObj {
			if _, exists := props[key]; !exists {
				return core.Denied
			}
		}
	}
	return nil
}

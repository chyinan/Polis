// pattern: Functional Core
package kernel

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math"
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
		if errors.Is(err, core.Malformed) {
			return denyGate(result, "SCHEMA_INVALID")
		}
		return denyGate(result, "ARGUMENTS_SCHEMA_VIOLATION")
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
	if len(schemaRaw) == 0 {
		return nil
	}
	trimmedSchema := bytes.TrimSpace(schemaRaw)
	if len(trimmedSchema) == 0 || bytes.Equal(trimmedSchema, []byte("null")) || bytes.Equal(trimmedSchema, []byte("{}")) || bytes.Equal(trimmedSchema, []byte("true")) {
		return nil
	}
	if bytes.Equal(trimmedSchema, []byte("false")) {
		return core.Denied
	}
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
	if !validateJSONSchemaValue(schemaObj, args, 0) {
		return core.Denied
	}
	return nil
}

const maxJSONSchemaDepth = 32

func validateJSONSchemaValue(schema map[string]any, value any, depth int) bool {
	if depth > maxJSONSchemaDepth || schema == nil {
		return true
	}
	if v, ok := schema["const"]; ok && !jsonSchemaEqual(v, value) {
		return false
	}
	if enum, ok := schema["enum"].([]any); ok {
		match := false
		for _, item := range enum {
			if jsonSchemaEqual(item, value) {
				match = true
				break
			}
		}
		if !match {
			return false
		}
	}
	if allOf, ok := schema["allOf"].([]any); ok {
		for _, sub := range allOf {
			if s, ok := sub.(map[string]any); ok && !validateJSONSchemaValue(s, value, depth+1) {
				return false
			}
		}
	}
	if anyOf, ok := schema["anyOf"].([]any); ok && len(anyOf) > 0 {
		match := false
		for _, sub := range anyOf {
			if s, ok := sub.(map[string]any); ok && validateJSONSchemaValue(s, value, depth+1) {
				match = true
				break
			}
		}
		if !match {
			return false
		}
	}
	if oneOf, ok := schema["oneOf"].([]any); ok && len(oneOf) > 0 {
		matches := 0
		for _, sub := range oneOf {
			if s, ok := sub.(map[string]any); ok && validateJSONSchemaValue(s, value, depth+1) {
				matches++
			}
		}
		if matches != 1 {
			return false
		}
	}
	if t := schema["type"]; t != nil && !jsonSchemaTypeMatches(t, value) {
		return false
	}
	switch v := value.(type) {
	case map[string]any:
		return validateJSONObjectSchema(schema, v, depth)
	case []any:
		return validateJSONArraySchema(schema, v, depth)
	case string:
		return validateJSONStringSchema(schema, v)
	case float64:
		return validateJSONNumberSchema(schema, v)
	case bool:
		return true
	case nil:
		return true
	default:
		return true
	}
}

func jsonSchemaTypeMatches(t any, value any) bool {
	switch tt := t.(type) {
	case string:
		return jsonSchemaSingleTypeMatches(tt, value)
	case []any:
		for _, item := range tt {
			if s, ok := item.(string); ok && jsonSchemaSingleTypeMatches(s, value) {
				return true
			}
		}
		return false
	default:
		return true
	}
}

func jsonSchemaSingleTypeMatches(t string, value any) bool {
	switch t {
	case "object":
		_, ok := value.(map[string]any)
		return ok
	case "array":
		_, ok := value.([]any)
		return ok
	case "string":
		_, ok := value.(string)
		return ok
	case "number":
		_, ok := value.(float64)
		return ok
	case "integer":
		n, ok := value.(float64)
		return ok && !math.IsNaN(n) && !math.IsInf(n, 0) && math.Trunc(n) == n
	case "boolean":
		_, ok := value.(bool)
		return ok
	case "null":
		return value == nil
	default:
		return true
	}
}

func validateJSONObjectSchema(schema map[string]any, obj map[string]any, depth int) bool {
	if req, ok := schema["required"].([]any); ok {
		for _, item := range req {
			name, ok := item.(string)
			if !ok {
				continue
			}
			if _, exists := obj[name]; !exists {
				return false
			}
		}
	}
	props, _ := schema["properties"].(map[string]any)
	additional, hasAdditional := schema["additionalProperties"]
	for key, val := range obj {
		if propSchema, ok := props[key]; ok {
			if s, ok := propSchema.(map[string]any); ok && !validateJSONSchemaValue(s, val, depth+1) {
				return false
			}
			continue
		}
		if hasAdditional {
			switch a := additional.(type) {
			case bool:
				if !a {
					return false
				}
			case map[string]any:
				if !validateJSONSchemaValue(a, val, depth+1) {
					return false
				}
			}
		}
	}
	return true
}

func validateJSONArraySchema(schema map[string]any, arr []any, depth int) bool {
	if min, ok := jsonSchemaInt(schema["minItems"]); ok && len(arr) < min {
		return false
	}
	if max, ok := jsonSchemaInt(schema["maxItems"]); ok && len(arr) > max {
		return false
	}
	if items, ok := schema["items"].(map[string]any); ok {
		for _, item := range arr {
			if !validateJSONSchemaValue(items, item, depth+1) {
				return false
			}
		}
	}
	return true
}

func validateJSONStringSchema(schema map[string]any, s string) bool {
	if min, ok := jsonSchemaInt(schema["minLength"]); ok && len([]rune(s)) < min {
		return false
	}
	if max, ok := jsonSchemaInt(schema["maxLength"]); ok && len([]rune(s)) > max {
		return false
	}
	return true
}

func validateJSONNumberSchema(schema map[string]any, n float64) bool {
	if min, ok := jsonSchemaNumber(schema["minimum"]); ok && n < min {
		return false
	}
	if max, ok := jsonSchemaNumber(schema["maximum"]); ok && n > max {
		return false
	}
	if min, ok := jsonSchemaNumber(schema["exclusiveMinimum"]); ok && n <= min {
		return false
	}
	if max, ok := jsonSchemaNumber(schema["exclusiveMaximum"]); ok && n >= max {
		return false
	}
	return true
}

func jsonSchemaInt(v any) (int, bool) {
	n, ok := jsonSchemaNumber(v)
	if !ok {
		return 0, false
	}
	if math.Trunc(n) != n {
		return 0, false
	}
	return int(n), true
}

func jsonSchemaNumber(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	default:
		return 0, false
	}
}

func jsonSchemaEqual(a, b any) bool {
	ab, err := json.Marshal(a)
	if err != nil {
		return false
	}
	bb, err := json.Marshal(b)
	if err != nil {
		return false
	}
	return bytes.Equal(ab, bb)
}

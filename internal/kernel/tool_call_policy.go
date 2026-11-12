// pattern: Functional Core
package kernel

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math"
	"net"
	"net/mail"
	"net/url"
	"regexp"
	"strings"
	"time"

	"polis/internal/core"
)

const (
	toolCallPolicyRevision = "r15-tools-call@1"
	maxToolCallArguments   = 64 * 1024
	maxToolCallResult      = 256 * 1024
	maxToolCallStderr      = 4096
)

type ToolCallRequest struct {
	CompanyID          string          `json:"companyId"`
	EmployeeID         string          `json:"employeeId"`
	TaskID             string          `json:"taskId,omitempty"`
	BindingID          string          `json:"bindingId"`
	SessionID          string          `json:"sessionId,omitempty"`
	SessionEpoch       int64           `json:"sessionEpoch,omitempty"`
	SessionIncarnation string          `json:"sessionIncarnation,omitempty"`
	Handle             string          `json:"handle"`
	DescriptorDigest   string          `json:"descriptorDigest"`
	Arguments          json.RawMessage `json:"arguments"`
	IdempotencyKey     string          `json:"idempotencyKey"`
	PolicyRevision     string          `json:"policyRevision"`
	AuthorizationRef   string          `json:"authorizationRef,omitempty"`
	EffectClassHint    string          `json:"effectClassHint,omitempty"`
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
	CompanyID          string          `json:"companyId"`
	ID                 string          `json:"id"`
	EmployeeID         string          `json:"employeeId"`
	TaskID             *string         `json:"taskId,omitempty"`
	SessionID          *string         `json:"sessionId,omitempty"`
	SessionEpoch       *int64          `json:"sessionEpoch,omitempty"`
	SessionIncarnation string          `json:"sessionIncarnation,omitempty"`
	BindingID          string          `json:"bindingId"`
	DefinitionID       string          `json:"definitionId"`
	QualificationID    *string         `json:"qualificationId,omitempty"`
	Handle             string          `json:"handle"`
	DescriptorDigest   string          `json:"descriptorDigest"`
	ArgumentsDigest    string          `json:"argumentsDigest"`
	RequestDigest      string          `json:"requestDigest"`
	IdempotencyKey     string          `json:"idempotencyKey"`
	PolicyRevision     string          `json:"policyRevision"`
	AuthorizationRef   string          `json:"authorizationRef"`
	EffectClass        string          `json:"effectClass"`
	Outcome            string          `json:"outcome"`
	Code               string          `json:"code"`
	ResultDigest       string          `json:"resultDigest"`
	ResultJSON         json.RawMessage `json:"resultJson,omitempty"`
	ErrorJSON          json.RawMessage `json:"errorJson,omitempty"`
	ProcessObserved    json.RawMessage `json:"processObserved"`
	EgressObserved     json.RawMessage `json:"egressObserved"`
	StartedAt          *string         `json:"startedAt,omitempty"`
	FinishedAt         *string         `json:"finishedAt,omitempty"`
	CreatedAt          string          `json:"createdAt"`
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
	Detail       string
	DetailPath   string
	DetailReason string
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
	if strings.TrimSpace(req.SessionID) != "" && !core.ValidID(req.SessionID) {
		return req, core.Malformed
	}
	if req.SessionEpoch < 0 {
		return req, core.Malformed
	}
	if len(req.SessionIncarnation) > 256 || strings.ContainsAny(req.SessionIncarnation, "\r\n\t") {
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
		CompanyID          string          `json:"companyId"`
		EmployeeID         string          `json:"employeeId"`
		TaskID             string          `json:"taskId"`
		BindingID          string          `json:"bindingId"`
		SessionID          string          `json:"sessionId"`
		SessionEpoch       int64           `json:"sessionEpoch"`
		SessionIncarnation string          `json:"sessionIncarnation"`
		Handle             string          `json:"handle"`
		DescriptorDigest   string          `json:"descriptorDigest"`
		Arguments          json.RawMessage `json:"arguments"`
		PolicyRevision     string          `json:"policyRevision"`
		AuthorizationRef   string          `json:"authorizationRef"`
	}{req.CompanyID, req.EmployeeID, strings.TrimSpace(req.TaskID), req.BindingID, strings.TrimSpace(req.SessionID), req.SessionEpoch, strings.TrimSpace(req.SessionIncarnation), req.Handle, req.DescriptorDigest, canonicalArgs, req.PolicyRevision, req.AuthorizationRef})
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
		result = denyGate(result, "ARGUMENTS_SCHEMA_VIOLATION")
		var sv *schemaViolationError
		if errors.As(err, &sv) && sv != nil {
			result.DetailPath = sv.Path
			result.DetailReason = sv.Message
			if sv.Path != "" {
				result.Detail = sv.Path + ": " + sv.Message
			} else {
				result.Detail = sv.Message
			}
		}
		return result
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
	if q.Transport != "stdio" && q.Transport != "streamable_http" {
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

type schemaViolationError struct {
	Path    string
	Message string
}

func (e *schemaViolationError) Error() string {
	if e == nil {
		return "schema violation"
	}
	if e.Path != "" {
		return e.Path + ": " + e.Message
	}
	return e.Message
}

func schemaViolation(path, message string) error {
	if message == "" {
		message = "arguments do not satisfy schema"
	}
	return &schemaViolationError{Path: path, Message: message}
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
		return schemaViolation("$", "schema is false")
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
	if path, msg, ok := validateJSONSchemaValue(schemaObj, schemaObj, args, "$", 0, nil); !ok {
		return schemaViolation(path, msg)
	}
	return nil
}

const maxJSONSchemaDepth = 32

func schemaOK() (string, string, bool) { return "", "", true }
func schemaFail(path, msg string) (string, string, bool) {
	if msg == "" {
		msg = "schema constraint failed"
	}
	if path == "" {
		path = "$"
	}
	return path, msg, false
}

func validateJSONSchemaValue(root, schema map[string]any, value any, path string, depth int, refStack map[string]struct{}) (string, string, bool) {
	if depth > maxJSONSchemaDepth {
		return schemaFail(path, "schema depth limit exceeded")
	}
	if schema == nil {
		return schemaOK()
	}
	if ref, ok := schema["$ref"].(string); ok && ref != "" {
		if !strings.HasPrefix(ref, "#") {
			return schemaFail(path, "external $ref is not allowed")
		}
		if refStack == nil {
			refStack = map[string]struct{}{}
		}
		if _, seen := refStack[ref]; seen {
			return schemaFail(path, "cyclic $ref")
		}
		resolved, ok := resolveJSONPointerSchema(root, ref)
		if !ok {
			return schemaFail(path, "unresolved $ref")
		}
		nextStack := make(map[string]struct{}, len(refStack)+1)
		for k := range refStack {
			nextStack[k] = struct{}{}
		}
		nextStack[ref] = struct{}{}
		return validateJSONSchemaValue(root, resolved, value, path, depth+1, nextStack)
	}
	if v, ok := schema["const"]; ok && !jsonSchemaEqual(v, value) {
		return schemaFail(path, "const mismatch")
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
			return schemaFail(path, "enum mismatch")
		}
	}
	if allOf, ok := schema["allOf"].([]any); ok {
		for _, sub := range allOf {
			if s, ok := sub.(map[string]any); ok {
				if p, m, okv := validateJSONSchemaValue(root, s, value, path, depth+1, refStack); !okv {
					return p, m, false
				}
			}
		}
	}
	if anyOf, ok := schema["anyOf"].([]any); ok && len(anyOf) > 0 {
		match := false
		for _, sub := range anyOf {
			if s, ok := sub.(map[string]any); ok {
				if _, _, okv := validateJSONSchemaValue(root, s, value, path, depth+1, refStack); okv {
					match = true
					break
				}
			}
		}
		if !match {
			return schemaFail(path, "anyOf mismatch")
		}
	}
	if oneOf, ok := schema["oneOf"].([]any); ok && len(oneOf) > 0 {
		matches := 0
		for _, sub := range oneOf {
			if s, ok := sub.(map[string]any); ok {
				if _, _, okv := validateJSONSchemaValue(root, s, value, path, depth+1, refStack); okv {
					matches++
				}
			}
		}
		if matches != 1 {
			return schemaFail(path, "oneOf mismatch")
		}
	}
	if t := schema["type"]; t != nil && !jsonSchemaTypeMatches(t, value) {
		return schemaFail(path, "type mismatch")
	}
	switch v := value.(type) {
	case map[string]any:
		return validateJSONObjectSchema(root, schema, v, path, depth, refStack)
	case []any:
		return validateJSONArraySchema(root, schema, v, path, depth, refStack)
	case string:
		return validateJSONStringSchema(schema, v, path)
	case float64:
		return validateJSONNumberSchema(schema, v, path)
	case bool, nil:
		return schemaOK()
	default:
		return schemaOK()
	}
}

func resolveJSONPointerSchema(root map[string]any, ref string) (map[string]any, bool) {
	if ref == "#" {
		return root, true
	}
	if !strings.HasPrefix(ref, "#/") {
		return nil, false
	}
	var current any = root
	for _, raw := range strings.Split(strings.TrimPrefix(ref, "#/"), "/") {
		token := strings.ReplaceAll(strings.ReplaceAll(raw, "~1", "/"), "~0", "~")
		switch node := current.(type) {
		case map[string]any:
			next, ok := node[token]
			if !ok {
				return nil, false
			}
			current = next
		case []any:
			idx := 0
			for _, ch := range token {
				if ch < '0' || ch > '9' {
					return nil, false
				}
				idx = idx*10 + int(ch-'0')
			}
			if idx < 0 || idx >= len(node) {
				return nil, false
			}
			current = node[idx]
		default:
			return nil, false
		}
	}
	obj, ok := current.(map[string]any)
	return obj, ok
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

func validateJSONObjectSchema(root, schema map[string]any, obj map[string]any, path string, depth int, refStack map[string]struct{}) (string, string, bool) {
	if req, ok := schema["required"].([]any); ok {
		for _, item := range req {
			name, ok := item.(string)
			if !ok {
				continue
			}
			if _, exists := obj[name]; !exists {
				return schemaFail(path+"."+name, "required property missing")
			}
		}
	}
	props, _ := schema["properties"].(map[string]any)
	additional, hasAdditional := schema["additionalProperties"]
	for key, val := range obj {
		childPath := path + "." + key
		if propSchema, ok := props[key]; ok {
			if s, ok := propSchema.(map[string]any); ok {
				if p, m, okv := validateJSONSchemaValue(root, s, val, childPath, depth+1, refStack); !okv {
					return p, m, false
				}
			}
			continue
		}
		if hasAdditional {
			switch a := additional.(type) {
			case bool:
				if !a {
					return schemaFail(childPath, "additional property not allowed")
				}
			case map[string]any:
				if p, m, okv := validateJSONSchemaValue(root, a, val, childPath, depth+1, refStack); !okv {
					return p, m, false
				}
			}
		}
	}
	return schemaOK()
}

func validateJSONArraySchema(root, schema map[string]any, arr []any, path string, depth int, refStack map[string]struct{}) (string, string, bool) {
	if min, ok := jsonSchemaInt(schema["minItems"]); ok && len(arr) < min {
		return schemaFail(path, "minItems violated")
	}
	if max, ok := jsonSchemaInt(schema["maxItems"]); ok && len(arr) > max {
		return schemaFail(path, "maxItems violated")
	}
	if items, ok := schema["items"].(map[string]any); ok {
		for i, item := range arr {
			if p, m, okv := validateJSONSchemaValue(root, items, item, path+"["+itoa(i)+"]", depth+1, refStack); !okv {
				return p, m, false
			}
		}
	}
	return schemaOK()
}



const maxJSONSchemaPatternLength = 1024

var uuidPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[1-5][0-9a-fA-F]{3}-[89abAB][0-9a-fA-F]{3}-[0-9a-fA-F]{12}$`)

func validateJSONStringSchema(schema map[string]any, s, path string) (string, string, bool) {
	if min, ok := jsonSchemaInt(schema["minLength"]); ok && len([]rune(s)) < min {
		return schemaFail(path, "minLength violated")
	}
	if max, ok := jsonSchemaInt(schema["maxLength"]); ok && len([]rune(s)) > max {
		return schemaFail(path, "maxLength violated")
	}
	if pattern, ok := schema["pattern"].(string); ok && pattern != "" {
		if len(pattern) > maxJSONSchemaPatternLength {
			return schemaFail(path, "pattern too long")
		}
		re, err := regexp.Compile(pattern)
		if err != nil {
			return schemaFail(path, "pattern invalid")
		}
		if !re.MatchString(s) {
			return schemaFail(path, "pattern mismatch")
		}
	}
	if format, ok := schema["format"].(string); ok && format != "" {
		if !validateJSONSchemaFormat(format, s) {
			return schemaFail(path, "format mismatch")
		}
	}
	return schemaOK()
}

func validateJSONSchemaFormat(format, value string) bool {
	// Unknown formats are treated as annotations (pass) to avoid breaking schemas.
	switch format {
	case "date-time":
		_, err := time.Parse(time.RFC3339Nano, value)
		if err == nil {
			return true
		}
		_, err = time.Parse(time.RFC3339, value)
		return err == nil
	case "date":
		_, err := time.Parse("2006-01-02", value)
		return err == nil
	case "time":
		for _, layout := range []string{"15:04:05Z07:00", "15:04:05.999999999Z07:00", "15:04:05", "15:04:05.999999999"} {
			if _, err := time.Parse(layout, value); err == nil {
				return true
			}
		}
		return false
	case "email":
		if len(value) > 254 {
			return false
		}
		addr, err := mail.ParseAddress(value)
		return err == nil && addr != nil && addr.Address == value && strings.Contains(value, "@")
	case "uri", "uri-reference":
		if len(value) > 2048 {
			return false
		}
		_, err := url.Parse(value)
		return err == nil
	case "uuid":
		return uuidPattern.MatchString(value)
	case "ipv4":
		ip := net.ParseIP(value)
		return ip != nil && ip.To4() != nil && !strings.Contains(value, ":")
	case "ipv6":
		ip := net.ParseIP(value)
		return ip != nil && strings.Contains(value, ":")
	case "hostname":
		if len(value) == 0 || len(value) > 253 {
			return false
		}
		for _, label := range strings.Split(strings.TrimSuffix(value, "."), ".") {
			if len(label) == 0 || len(label) > 63 {
				return false
			}
		}
		return true
	default:
		return true
	}
}

func validateJSONNumberSchema(schema map[string]any, n float64, path string) (string, string, bool) {
	if min, ok := jsonSchemaNumber(schema["minimum"]); ok && n < min {
		return schemaFail(path, "minimum violated")
	}
	if max, ok := jsonSchemaNumber(schema["maximum"]); ok && n > max {
		return schemaFail(path, "maximum violated")
	}
	if min, ok := jsonSchemaNumber(schema["exclusiveMinimum"]); ok && n <= min {
		return schemaFail(path, "exclusiveMinimum violated")
	}
	if max, ok := jsonSchemaNumber(schema["exclusiveMaximum"]); ok && n >= max {
		return schemaFail(path, "exclusiveMaximum violated")
	}
	return schemaOK()
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

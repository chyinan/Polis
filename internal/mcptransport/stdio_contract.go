// pattern: Functional Core
package mcptransport

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"math/big"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"
)

const (
	StdioProfile20260728        = "polis-controlled-stdio-mcp-2026-07-28@1"
	StdioResultContentBoundary  = "untrusted_stdio_mcp_text"
	StdioServerInfoMetadataKey  = "io.modelcontextprotocol/serverInfo"
	MaxStdioMessageBytes        = 1 << 20
	MaxStdioToolListBytes       = 256 << 10
	MaxStdioToolSchemaBytes     = 32 << 10
	MaxStdioCallArgumentsBytes  = 64 << 10
	MaxStdioToolCount           = 64
	MaxStdioToolNameBytes       = 128
	MaxStdioToolDescriptionSize = 4 << 10
	MaxStdioToolResultBytes     = 64 << 10
	MaxStdioSchemaProperties    = 64
	MaxStdioSchemaEnumValues    = 128
)

type StdioServerIdentity struct {
	Name        string          `json:"name"`
	Version     string          `json:"version"`
	Title       string          `json:"title,omitempty"`
	Description string          `json:"description,omitempty"`
	WebsiteURL  string          `json:"websiteUrl,omitempty"`
	Icons       json.RawMessage `json:"icons,omitempty"`
}

type StdioToolDefinition struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	InputSchema json.RawMessage `json:"inputSchema"`
}

type StdioTextContent struct {
	Text string `json:"text"`
}

type StdioToolResult struct {
	ToolName         string             `json:"toolName"`
	ToolSchemaSHA256 string             `json:"toolSchemaSha256"`
	ContentBoundary  string             `json:"contentBoundary"`
	Content          []StdioTextContent `json:"content"`
}

type stdioSchemaProperty struct {
	typeName  string
	minimum   *big.Rat
	maximum   *big.Rat
	minLength *int
	maxLength *int
	enum      []any
}

type stdioInputSchema struct {
	properties map[string]stdioSchemaProperty
	required   map[string]struct{}
}

func prepareStdioToolDefinitions(raw json.RawMessage) ([]StdioToolDefinition, map[string]stdioInputSchema, string, error) {
	if len(raw) == 0 || len(raw) > MaxStdioToolListBytes {
		return nil, nil, "", errors.New("stdio MCP tool list is empty or exceeds its bound")
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var values []any
	if err := decoder.Decode(&values); err != nil || values == nil {
		return nil, nil, "", errors.New("stdio MCP tool list is not a JSON array")
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return nil, nil, "", errors.New("stdio MCP tool list contains trailing JSON")
	}
	if len(values) > MaxStdioToolCount {
		return nil, nil, "", errors.New("stdio MCP tool count exceeds its bound")
	}
	type canonicalTool struct {
		name        string
		definition  json.RawMessage
		public      StdioToolDefinition
		inputSchema stdioInputSchema
	}
	canonicalTools := make([]canonicalTool, 0, len(values))
	seenNames := make(map[string]struct{}, len(values))
	for _, value := range values {
		definition, ok := value.(map[string]any)
		if !ok {
			return nil, nil, "", errors.New("stdio MCP tool definition is not an object")
		}
		name, ok := definition["name"].(string)
		if !ok || !validStdioToolName(name) {
			return nil, nil, "", errors.New("stdio MCP tool name is missing or invalid")
		}
		if _, exists := seenNames[name]; exists {
			return nil, nil, "", errors.New("stdio MCP tool list contains duplicate names")
		}
		seenNames[name] = struct{}{}
		description := ""
		if rawDescription, exists := definition["description"]; exists {
			description, ok = rawDescription.(string)
			if !ok || len(description) > MaxStdioToolDescriptionSize || !utf8.ValidString(description) {
				return nil, nil, "", errors.New("stdio MCP tool description is invalid or oversized")
			}
		}
		inputValue, exists := definition["inputSchema"]
		if !exists {
			return nil, nil, "", errors.New("stdio MCP tool input schema is missing")
		}
		schemaBytes, err := json.Marshal(inputValue)
		if err != nil || len(schemaBytes) == 0 || len(schemaBytes) > MaxStdioToolSchemaBytes {
			return nil, nil, "", errors.New("stdio MCP tool input schema is invalid or oversized")
		}
		inputSchema, err := validateStdioInputSchema(inputValue)
		if err != nil {
			return nil, nil, "", err
		}
		canonicalDefinition, err := json.Marshal(definition)
		if err != nil || len(canonicalDefinition) > MaxStdioToolSchemaBytes+MaxStdioToolDescriptionSize+MaxStdioToolNameBytes+4096 {
			return nil, nil, "", errors.New("stdio MCP tool definition is invalid or oversized")
		}
		canonicalTools = append(canonicalTools, canonicalTool{
			name: name, definition: canonicalDefinition,
			public:      StdioToolDefinition{Name: name, Description: description, InputSchema: json.RawMessage(schemaBytes)},
			inputSchema: inputSchema,
		})
	}
	sort.Slice(canonicalTools, func(left, right int) bool { return canonicalTools[left].name < canonicalTools[right].name })
	canonicalDefinitions := make([]json.RawMessage, len(canonicalTools))
	tools := make([]StdioToolDefinition, len(canonicalTools))
	schemas := make(map[string]stdioInputSchema, len(canonicalTools))
	for index, tool := range canonicalTools {
		canonicalDefinitions[index] = tool.definition
		tool.public.InputSchema = append(json.RawMessage(nil), tool.public.InputSchema...)
		tools[index] = tool.public
		schemas[tool.name] = tool.inputSchema
	}
	manifest, err := json.Marshal(struct {
		Profile         string            `json:"profile"`
		ProtocolVersion string            `json:"protocolVersion"`
		Tools           []json.RawMessage `json:"tools"`
	}{StdioProfile20260728, ProtocolVersion20260728, canonicalDefinitions})
	if err != nil || len(manifest) > MaxStdioToolListBytes {
		return nil, nil, "", errors.New("stdio MCP tool manifest is invalid or oversized")
	}
	digest := sha256.Sum256(manifest)
	return tools, schemas, hex.EncodeToString(digest[:]), nil
}

// StdioToolSchemaDigest returns the validated canonical digest for a pinned
// stdio profile tool list. It is safe to persist as qualification evidence.
func StdioToolSchemaDigest(raw json.RawMessage) (string, error) {
	_, _, digest, err := prepareStdioToolDefinitions(raw)
	return digest, err
}

// ValidatePinnedToolArguments validates an object against the fixed MCP tool
// input-schema subset used by the controlled stdio and Streamable HTTP profiles.
func ValidatePinnedToolArguments(inputSchema, arguments json.RawMessage) error {
	if len(inputSchema) == 0 || len(inputSchema) > MaxStdioToolSchemaBytes || len(arguments) == 0 || len(arguments) > MaxStdioCallArgumentsBytes {
		return errors.New("MCP tool arguments or schema are outside their bound")
	}
	decoder := json.NewDecoder(bytes.NewReader(inputSchema))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return errors.New("MCP tool input schema is invalid")
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return errors.New("MCP tool input schema contains trailing JSON")
	}
	schema, err := validateStdioInputSchema(value)
	if err != nil {
		return err
	}
	return validateStdioToolArguments(schema, arguments)
}

func validateStdioInputSchema(value any) (stdioInputSchema, error) {
	root, ok := value.(map[string]any)
	if !ok || root["type"] != "object" || root["additionalProperties"] != false {
		return stdioInputSchema{}, errors.New("stdio MCP tool schema must be a closed object")
	}
	allowedRootFields := map[string]struct{}{"type": {}, "title": {}, "description": {}, "properties": {}, "required": {}, "additionalProperties": {}, "$schema": {}}
	for key := range root {
		if _, allowed := allowedRootFields[key]; !allowed {
			return stdioInputSchema{}, errors.New("stdio MCP tool schema uses an unsupported root keyword")
		}
	}
	properties, ok := root["properties"].(map[string]any)
	if !ok || len(properties) > MaxStdioSchemaProperties {
		return stdioInputSchema{}, errors.New("stdio MCP tool schema properties are invalid or oversized")
	}
	parsed := stdioInputSchema{properties: make(map[string]stdioSchemaProperty, len(properties)), required: make(map[string]struct{})}
	for name, rawProperty := range properties {
		if !validStdioToolName(name) {
			return stdioInputSchema{}, errors.New("stdio MCP tool schema property name is invalid")
		}
		propertyMap, ok := rawProperty.(map[string]any)
		if !ok {
			return stdioInputSchema{}, errors.New("stdio MCP tool property schema must be an object")
		}
		property, err := parseStdioSchemaProperty(propertyMap)
		if err != nil {
			return stdioInputSchema{}, err
		}
		parsed.properties[name] = property
	}
	if rawRequired, exists := root["required"]; exists {
		required, ok := rawRequired.([]any)
		if !ok || len(required) > len(properties) {
			return stdioInputSchema{}, errors.New("stdio MCP tool required field list is invalid")
		}
		for _, value := range required {
			name, ok := value.(string)
			if !ok {
				return stdioInputSchema{}, errors.New("stdio MCP tool required field name is invalid")
			}
			if _, exists := parsed.properties[name]; !exists {
				return stdioInputSchema{}, errors.New("stdio MCP tool requires an unknown field")
			}
			if _, duplicate := parsed.required[name]; duplicate {
				return stdioInputSchema{}, errors.New("stdio MCP tool required field list has a duplicate")
			}
			parsed.required[name] = struct{}{}
		}
	}
	return parsed, nil
}

func parseStdioSchemaProperty(value map[string]any) (stdioSchemaProperty, error) {
	allowedFields := map[string]struct{}{"type": {}, "description": {}, "enum": {}, "minimum": {}, "maximum": {}, "minLength": {}, "maxLength": {}}
	for key := range value {
		if _, allowed := allowedFields[key]; !allowed {
			return stdioSchemaProperty{}, errors.New("stdio MCP property schema uses an unsupported keyword")
		}
	}
	typeName, ok := value["type"].(string)
	if !ok || (typeName != "string" && typeName != "boolean" && typeName != "integer" && typeName != "number") {
		return stdioSchemaProperty{}, errors.New("stdio MCP properties support only bounded primitive types")
	}
	property := stdioSchemaProperty{typeName: typeName}
	if description, exists := value["description"]; exists {
		text, ok := description.(string)
		if !ok || len(text) > MaxStdioToolDescriptionSize || !utf8.ValidString(text) {
			return stdioSchemaProperty{}, errors.New("stdio MCP property description is invalid")
		}
	}
	if rawMinimum, exists := value["minimum"]; exists {
		minimum, ok := exactNumberValue(rawMinimum)
		if !ok || (typeName != "number" && typeName != "integer") {
			return stdioSchemaProperty{}, errors.New("stdio MCP numeric minimum is invalid for its property type")
		}
		property.minimum = minimum
	}
	if rawMaximum, exists := value["maximum"]; exists {
		maximum, ok := exactNumberValue(rawMaximum)
		if !ok || (typeName != "number" && typeName != "integer") {
			return stdioSchemaProperty{}, errors.New("stdio MCP numeric maximum is invalid for its property type")
		}
		property.maximum = maximum
	}
	if property.minimum != nil && property.maximum != nil && property.minimum.Cmp(property.maximum) > 0 {
		return stdioSchemaProperty{}, errors.New("stdio MCP numeric property bounds are inverted")
	}
	if rawMinLength, exists := value["minLength"]; exists {
		minimum, ok := integerValue(rawMinLength)
		if !ok || typeName != "string" || minimum < 0 || minimum > MaxStdioCallArgumentsBytes {
			return stdioSchemaProperty{}, errors.New("stdio MCP minimum string length is invalid")
		}
		property.minLength = &minimum
	}
	if rawMaxLength, exists := value["maxLength"]; exists {
		maximum, ok := integerValue(rawMaxLength)
		if !ok || typeName != "string" || maximum < 0 || maximum > MaxStdioCallArgumentsBytes {
			return stdioSchemaProperty{}, errors.New("stdio MCP maximum string length is invalid")
		}
		property.maxLength = &maximum
	}
	if property.minLength != nil && property.maxLength != nil && *property.minLength > *property.maxLength {
		return stdioSchemaProperty{}, errors.New("stdio MCP string property bounds are inverted")
	}
	if rawEnum, exists := value["enum"]; exists {
		enumeration, ok := rawEnum.([]any)
		if !ok || len(enumeration) == 0 || len(enumeration) > MaxStdioSchemaEnumValues {
			return stdioSchemaProperty{}, errors.New("stdio MCP property enum is empty or oversized")
		}
		for _, enumValue := range enumeration {
			if !stdioPrimitiveMatches(typeName, enumValue) {
				return stdioSchemaProperty{}, errors.New("stdio MCP property enum contains an incompatible value")
			}
		}
		property.enum = enumeration
	}
	return property, nil
}

func validateStdioToolArguments(schema stdioInputSchema, arguments json.RawMessage) error {
	if len(arguments) == 0 || len(arguments) > MaxStdioCallArgumentsBytes {
		return errors.New("stdio MCP tool arguments are empty or oversized")
	}
	decoder := json.NewDecoder(bytes.NewReader(arguments))
	decoder.UseNumber()
	var values map[string]any
	if err := decoder.Decode(&values); err != nil || values == nil {
		return errors.New("stdio MCP tool arguments must be a JSON object")
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return errors.New("stdio MCP tool arguments contain trailing JSON")
	}
	for name := range schema.required {
		if _, exists := values[name]; !exists {
			return errors.New("stdio MCP tool arguments omit a required field")
		}
	}
	for name, value := range values {
		property, exists := schema.properties[name]
		if !exists || !stdioPrimitiveMatches(property.typeName, value) {
			return errors.New("stdio MCP tool arguments contain an unknown or incompatible field")
		}
		if err := validateStdioArgumentValue(property, value); err != nil {
			return err
		}
	}
	return nil
}

func validateStdioArgumentValue(property stdioSchemaProperty, value any) error {
	if property.typeName == "string" {
		text := value.(string)
		length := utf8.RuneCountInString(text)
		if property.minLength != nil && length < *property.minLength || property.maxLength != nil && length > *property.maxLength {
			return errors.New("stdio MCP string argument violates its length bounds")
		}
	}
	if property.typeName == "integer" || property.typeName == "number" {
		number, ok := exactNumberValue(value)
		if !ok || property.minimum != nil && number.Cmp(property.minimum) < 0 || property.maximum != nil && number.Cmp(property.maximum) > 0 {
			return errors.New("stdio MCP numeric argument violates its range bounds")
		}
	}
	if len(property.enum) > 0 {
		matched := false
		if property.typeName == "integer" || property.typeName == "number" {
			numericValue, _ := exactNumberValue(value)
			for _, candidate := range property.enum {
				numericCandidate, ok := exactNumberValue(candidate)
				if ok && numericValue.Cmp(numericCandidate) == 0 {
					matched = true
					break
				}
			}
		} else {
			encodedValue, _ := json.Marshal(value)
			for _, candidate := range property.enum {
				encodedCandidate, _ := json.Marshal(candidate)
				if bytes.Equal(encodedValue, encodedCandidate) {
					matched = true
					break
				}
			}
		}
		if !matched {
			return errors.New("stdio MCP argument is outside its enumerated values")
		}
	}
	return nil
}

func stdioPrimitiveMatches(typeName string, value any) bool {
	switch typeName {
	case "string":
		_, ok := value.(string)
		return ok
	case "boolean":
		_, ok := value.(bool)
		return ok
	case "integer":
		number, ok := exactNumberValue(value)
		return ok && number.IsInt()
	case "number":
		_, ok := exactNumberValue(value)
		return ok
	default:
		return false
	}
}

func exactNumberValue(value any) (*big.Rat, bool) {
	number, ok := value.(json.Number)
	if !ok || len(number.String()) == 0 || len(number.String()) > 128 {
		return nil, false
	}
	if exponentIndex := strings.IndexAny(number.String(), "eE"); exponentIndex >= 0 {
		exponent, err := strconv.Atoi(number.String()[exponentIndex+1:])
		if err != nil || exponent < -308 || exponent > 308 {
			return nil, false
		}
	}
	parsed, ok := new(big.Rat).SetString(number.String())
	return parsed, ok
}

func integerValue(value any) (int, bool) {
	number, ok := value.(json.Number)
	if !ok || len(number.String()) == 0 || len(number.String()) > 32 {
		return 0, false
	}
	parsed, err := strconv.ParseInt(number.String(), 10, 64)
	if err != nil || parsed > int64(^uint(0)>>1) || parsed < -int64(^uint(0)>>1)-1 {
		return 0, false
	}
	return int(parsed), true
}

func validStdioToolName(value string) bool {
	if value == "" || len(value) > MaxStdioToolNameBytes {
		return false
	}
	for _, character := range value {
		if character < 0x21 || character > 0x7e {
			return false
		}
	}
	return true
}

func stdioDigest(encoded []byte) string {
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:])
}

func validStdioSHA256(value string) bool {
	if len(value) != 64 {
		return false
	}
	decoded, err := hex.DecodeString(value)
	return err == nil && len(decoded) == sha256.Size
}

func validStdioText(value string) bool {
	return utf8.ValidString(value) && !strings.ContainsRune(value, '\x00')
}

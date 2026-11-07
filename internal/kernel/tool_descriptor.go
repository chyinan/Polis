package kernel

import (
	"bytes"
	"encoding/json"
	"slices"
	"strings"

	"polis/internal/core"
)

type ToolDescriptorView struct {
	ServerID         string          `json:"serverId"`
	ServerName       string          `json:"serverName"`
	ToolName         string          `json:"toolName"`
	Description      string          `json:"description"`
	InputSchema      json.RawMessage `json:"inputSchema"`
	OutputSchema     json.RawMessage `json:"outputSchema"`
	DescriptorDigest string          `json:"descriptorDigest"`
	Handle           string          `json:"handle"`
	ToolsCalled      int             `json:"toolsCalled"`
	Callable         bool            `json:"callable"`
}

type CachedToolDescriptor struct {
	Name         string          `json:"name"`
	Description  string          `json:"description"`
	InputSchema  json.RawMessage `json:"inputSchema"`
	OutputSchema json.RawMessage `json:"outputSchema,omitempty"`
	Digest       string          `json:"digest"`
}

func visibleToolDescriptor(bindingStatus string, boundIDs []string, sources []toolCatalogSource, handle string) (ToolDescriptorView, error) {
	serverID, toolName, err := splitToolHandle(handle)
	if err != nil {
		return ToolDescriptorView{}, err
	}
	if bindingStatus != "approved" {
		return ToolDescriptorView{}, core.OutOfScope
	}
	bound := map[string]struct{}{}
	for _, id := range boundIDs {
		if !core.ValidID(id) {
			return ToolDescriptorView{}, core.Malformed
		}
		bound[id] = struct{}{}
	}
	if _, ok := bound[serverID]; !ok {
		return ToolDescriptorView{}, core.OutOfScope
	}
	var source *toolCatalogSource
	for i := range sources {
		if sources[i].ID == serverID {
			source = &sources[i]
			break
		}
	}
	if source == nil || source.Status == "revoked" || source.Status == "unverified" || source.Status == "" {
		return ToolDescriptorView{}, core.OutOfScope
	}
	if source.DescriptorDigest == "" || !core.ValidID(source.ID) {
		return ToolDescriptorView{}, core.Malformed
	}
	var receipt struct {
		ProbeStatus     string                 `json:"probeStatus"`
		ToolNames       []string               `json:"toolNames"`
		ToolsCalled     int                    `json:"toolsCalled"`
		ToolDescriptors []CachedToolDescriptor `json:"toolDescriptors"`
	}
	if len(source.Qualification) > 0 && json.Unmarshal(source.Qualification, &receipt) != nil {
		return ToolDescriptorView{}, core.Malformed
	}
	if receipt.ProbeStatus != "passed" {
		return ToolDescriptorView{}, core.OutOfScope
	}
	if receipt.ToolsCalled != 0 {
		return ToolDescriptorView{}, core.Integrity
	}
	named := false
	for _, name := range receipt.ToolNames {
		if name == toolName {
			named = true
			break
		}
	}
	if !named {
		return ToolDescriptorView{}, core.OutOfScope
	}
	var found *CachedToolDescriptor
	for i := range receipt.ToolDescriptors {
		if receipt.ToolDescriptors[i].Name == toolName {
			if found != nil {
				return ToolDescriptorView{}, core.Malformed
			}
			found = &receipt.ToolDescriptors[i]
		}
	}
	if found == nil {
		return ToolDescriptorView{}, core.OutOfScope
	}
	checked, err := checkedToolDescriptor(*found)
	if err != nil {
		return ToolDescriptorView{}, err
	}
	output := checked.OutputSchema
	if len(output) == 0 {
		output = json.RawMessage("null")
	}
	return ToolDescriptorView{
		ServerID: source.ID, ServerName: source.Name, ToolName: checked.Name, Description: checked.Description,
		InputSchema: checked.InputSchema, OutputSchema: output, DescriptorDigest: checked.Digest,
		Handle: handle, ToolsCalled: 0, Callable: false,
	}, nil
}

func splitToolHandle(handle string) (string, string, error) {
	if handle == "" || strings.ContainsAny(handle, " \t\r\n/") {
		return "", "", core.Malformed
	}
	serverID, toolName, ok := strings.Cut(handle, ":")
	if !ok || !core.ValidID(serverID) || toolName == "" || strings.Contains(toolName, ":") {
		return "", "", core.Malformed
	}
	return serverID, toolName, nil
}

func freezeToolDescriptor(name, description string, inputSchema, outputSchema json.RawMessage) (CachedToolDescriptor, error) {
	if name == "" || strings.ContainsAny(name, ":/ \t\r\n") {
		return CachedToolDescriptor{}, core.Malformed
	}
	if len(description) > maxManifestDocument {
		return CachedToolDescriptor{}, core.TooLarge
	}
	input, err := canonicalObjectSchema(inputSchema)
	if err != nil {
		return CachedToolDescriptor{}, err
	}
	var output json.RawMessage
	if len(bytes.TrimSpace(outputSchema)) > 0 && string(bytes.TrimSpace(outputSchema)) != "null" {
		output, err = canonicalObjectSchema(outputSchema)
		if err != nil {
			return CachedToolDescriptor{}, err
		}
	}
	digest, err := toolDescriptorDigest(name, description, input)
	if err != nil {
		return CachedToolDescriptor{}, err
	}
	return CachedToolDescriptor{Name: name, Description: description, InputSchema: input, OutputSchema: output, Digest: digest}, nil
}

func checkedToolDescriptor(item CachedToolDescriptor) (CachedToolDescriptor, error) {
	frozen, err := freezeToolDescriptor(item.Name, item.Description, item.InputSchema, item.OutputSchema)
	if err != nil {
		return CachedToolDescriptor{}, err
	}
	if item.Digest == "" || item.Digest != frozen.Digest {
		return CachedToolDescriptor{}, core.Integrity
	}
	return frozen, nil
}

func toolDescriptorDigest(name, description string, inputSchema json.RawMessage) (string, error) {
	encoded, err := json.Marshal(struct {
		Name        string          `json:"name"`
		Description string          `json:"description"`
		InputSchema json.RawMessage `json:"inputSchema"`
	}{Name: name, Description: description, InputSchema: inputSchema})
	if err != nil {
		return "", err
	}
	return contentDigest(encoded), nil
}

func canonicalObjectSchema(raw json.RawMessage) (json.RawMessage, error) {
	if len(bytes.TrimSpace(raw)) == 0 {
		raw = json.RawMessage(`{}`)
	}
	if !json.Valid(raw) {
		return nil, core.Malformed
	}
	if len(raw) > maxManifestDocument {
		return nil, core.TooLarge
	}
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil, core.Malformed
	}
	if _, ok := value.(map[string]any); !ok {
		return nil, core.Malformed
	}
	encoded, err := marshalCanonical(value)
	if err != nil {
		return nil, err
	}
	if len(encoded) > maxManifestDocument {
		return nil, core.TooLarge
	}
	return encoded, nil
}

func marshalCanonical(value any) ([]byte, error) {
	switch typed := value.(type) {
	case map[string]any:
		keys := make([]string, 0, len(typed))
		for key := range typed {
			keys = append(keys, key)
		}
		slices.Sort(keys)
		var buf bytes.Buffer
		buf.WriteByte('{')
		for i, key := range keys {
			if i > 0 {
				buf.WriteByte(',')
			}
			keyBytes, err := json.Marshal(key)
			if err != nil {
				return nil, err
			}
			buf.Write(keyBytes)
			buf.WriteByte(':')
			child, err := marshalCanonical(typed[key])
			if err != nil {
				return nil, err
			}
			buf.Write(child)
		}
		buf.WriteByte('}')
		return buf.Bytes(), nil
	case []any:
		var buf bytes.Buffer
		buf.WriteByte('[')
		for i, item := range typed {
			if i > 0 {
				buf.WriteByte(',')
			}
			child, err := marshalCanonical(item)
			if err != nil {
				return nil, err
			}
			buf.Write(child)
		}
		buf.WriteByte(']')
		return buf.Bytes(), nil
	default:
		return json.Marshal(typed)
	}
}

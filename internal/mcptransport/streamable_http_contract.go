// pattern: Functional Core
package mcptransport

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"unicode/utf8"
)

const (
	StreamableHTTPProfile20260728       = "polis-streamable-http-mcp-2026-07-28@1"
	StreamableHTTPResultContentBoundary = "untrusted_streamable_http_mcp_text"
	maxStreamableHTTPResultContentCount = 64
)

// PrepareStreamableHTTPToolList validates the fully collected fixed-profile
// tools/list catalog and returns a deterministic schema digest.
func PrepareStreamableHTTPToolList(result json.RawMessage) ([]StdioToolDefinition, string, error) {
	if len(result) == 0 || len(result) > MaxStdioToolListBytes {
		return nil, "", errors.New("Streamable HTTP MCP tool list is empty or exceeds its bound")
	}
	var response struct {
		Tools      json.RawMessage `json:"tools"`
		NextCursor string          `json:"nextCursor"`
	}
	if err := ValidateStrictJSONStructKeys(result, &response); err != nil {
		return nil, "", errors.Join(errors.New("Streamable HTTP MCP tools/list result has ambiguous fields"), err)
	}
	decoder := json.NewDecoder(bytes.NewReader(result))
	if err := decoder.Decode(&response); err != nil || len(response.Tools) == 0 {
		return nil, "", errors.New("Streamable HTTP MCP tools/list result is invalid")
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return nil, "", errors.New("Streamable HTTP MCP tools/list result has trailing JSON")
	}
	if response.NextCursor != "" {
		return nil, "", errors.New("Streamable HTTP MCP tool-list input must be a complete catalog")
	}
	tools, _, _, err := prepareStdioToolDefinitions(response.Tools)
	if err != nil || len(tools) == 0 {
		return nil, "", errors.Join(errors.New("Streamable HTTP MCP tool schema is outside the fixed profile"), err)
	}
	canonicalTools := make([]json.RawMessage, 0, len(tools))
	for _, tool := range tools {
		encoded, marshalErr := json.Marshal(tool)
		if marshalErr != nil {
			return nil, "", errors.Join(errors.New("Streamable HTTP MCP tool schema could not be canonicalized"), marshalErr)
		}
		canonicalTools = append(canonicalTools, encoded)
	}
	manifest, err := json.Marshal(struct {
		Profile         string            `json:"profile"`
		ProtocolVersion string            `json:"protocolVersion"`
		Tools           []json.RawMessage `json:"tools"`
	}{StreamableHTTPProfile20260728, ProtocolVersion20260728, canonicalTools})
	if err != nil || len(manifest) > MaxStdioToolListBytes {
		return nil, "", errors.Join(errors.New("Streamable HTTP MCP tool manifest exceeds its bound"), err)
	}
	digest := sha256.Sum256(manifest)
	return tools, hex.EncodeToString(digest[:]), nil
}

func decodeStreamableHTTPToolListPage(result json.RawMessage) ([]json.RawMessage, string, error) {
	if len(result) == 0 || len(result) > MaxStdioMessageBytes || !utf8.Valid(result) || !json.Valid(result) {
		return nil, "", errors.New("Streamable HTTP MCP tools/list page is invalid or oversized")
	}
	var response struct {
		Tools      json.RawMessage `json:"tools"`
		NextCursor *string         `json:"nextCursor"`
	}
	if err := ValidateStrictJSONStructKeys(result, &response); err != nil {
		return nil, "", errors.Join(errors.New("Streamable HTTP MCP tools/list page has ambiguous fields"), err)
	}
	decoder := json.NewDecoder(bytes.NewReader(result))
	if err := decoder.Decode(&response); err != nil {
		return nil, "", errors.New("Streamable HTTP MCP tools/list page is invalid")
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return nil, "", errors.New("Streamable HTTP MCP tools/list page has trailing JSON")
	}
	tools, err := decodeMCPToolDefinitionsPage(response.Tools)
	if err != nil {
		return nil, "", err
	}
	cursor := ""
	if response.NextCursor != nil {
		cursor = *response.NextCursor
	}
	return tools, cursor, nil
}

// PrepareStreamableHTTPToolResult accepts only a bounded MCP text result. It
// deliberately drops metadata and rejects media/resource content so the
// provider cannot treat remote structured data as trusted control input.
func PrepareStreamableHTTPToolResult(toolName, schemaDigest string, result json.RawMessage) (StdioToolResult, error) {
	if !validStdioToolName(toolName) || !validDigest(schemaDigest) || len(result) == 0 || len(result) > MaxStdioToolResultBytes {
		return StdioToolResult{}, errors.New("Streamable HTTP MCP tool result is invalid or exceeds its bound")
	}
	var response struct {
		IsError           bool            `json:"isError"`
		Content           json.RawMessage `json:"content"`
		StructuredContent json.RawMessage `json:"structuredContent"`
	}
	if err := ValidateStrictJSONStructKeys(result, &response); err != nil {
		return StdioToolResult{}, errors.Join(errors.New("Streamable HTTP MCP tool result has ambiguous fields"), err)
	}
	decoder := json.NewDecoder(bytes.NewReader(result))
	if err := decoder.Decode(&response); err != nil || len(response.Content) == 0 || response.StructuredContent != nil {
		return StdioToolResult{}, errors.New("Streamable HTTP MCP tool result is outside the text-only profile")
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return StdioToolResult{}, errors.New("Streamable HTTP MCP tool result has trailing JSON")
	}
	if response.IsError {
		return StdioToolResult{}, errors.New("Streamable HTTP MCP tool returned an error result")
	}
	var content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if err := ValidateStrictJSONStructKeys(response.Content, &content); err != nil {
		return StdioToolResult{}, errors.Join(errors.New("Streamable HTTP MCP text content has ambiguous fields"), err)
	}
	if err := json.Unmarshal(response.Content, &content); err != nil || content == nil || len(content) == 0 || len(content) > maxStreamableHTTPResultContentCount {
		return StdioToolResult{}, errors.New("Streamable HTTP MCP tool result content is invalid or exceeds its bound")
	}
	texts := make([]StdioTextContent, 0, len(content))
	contentBytes := 0
	for _, item := range content {
		if item.Type != "text" || !utf8.ValidString(item.Text) {
			return StdioToolResult{}, errors.New("Streamable HTTP MCP tool result contains unsupported media")
		}
		contentBytes += len(item.Text)
		if contentBytes > MaxStdioToolResultBytes {
			return StdioToolResult{}, errors.New("Streamable HTTP MCP text result exceeds its bound")
		}
		texts = append(texts, StdioTextContent{Text: item.Text})
	}
	return StdioToolResult{
		ToolName: toolName, ToolSchemaSHA256: schemaDigest,
		ContentBoundary: StreamableHTTPResultContentBoundary, Content: texts,
	}, nil
}

func validDigest(value string) bool {
	if len(value) != 64 {
		return false
	}
	for _, character := range value {
		if !((character >= '0' && character <= '9') || (character >= 'a' && character <= 'f')) {
			return false
		}
	}
	return true
}

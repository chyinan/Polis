// pattern: Imperative Shell
package control

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"

	"polis/internal/mcptransport"
)

type streamableHTTPMCPProcess struct {
	client     streamableHTTPMCPClient
	toolSchema string
	closed     bool
}

type streamableHTTPMCPClient interface {
	ListTools(context.Context) (json.RawMessage, error)
	CallTool(context.Context, string, map[string]any, json.RawMessage) (json.RawMessage, error)
	CloseIdleConnections()
}

func (factory *appContainerControlledMCPFactory) StartStreamableHTTP(ctx context.Context, endpoint, expectedToolSchema string) (controlledMCPProcess, error) {
	return (directStreamableHTTPMCPProcessFactory{}).StartStreamableHTTP(ctx, endpoint, expectedToolSchema)
}

func newStreamableHTTPMCPProcess(client streamableHTTPMCPClient, expectedToolSchema string) *streamableHTTPMCPProcess {
	return &streamableHTTPMCPProcess{client: client, toolSchema: expectedToolSchema}
}

func (process *streamableHTTPMCPProcess) ToolSchemaSHA256() string {
	if process == nil {
		return ""
	}
	return process.toolSchema
}

func (process *streamableHTTPMCPProcess) CallTool(ctx context.Context, name string, rawArguments []byte) (mcptransport.StdioToolResult, error) {
	return mcptransport.StdioToolResult{}, errors.New("MCP dispatch permit is required")
}

func (process *streamableHTTPMCPProcess) CallToolWithPermit(ctx context.Context, name string, rawArguments []byte, consume func(context.Context) error) (mcptransport.StdioToolResult, error) {
	if process == nil || process.client == nil || process.closed || ctx == nil || ctx.Err() != nil {
		return mcptransport.StdioToolResult{}, errors.New("Streamable HTTP MCP Worker client is unavailable")
	}
	if consume == nil {
		return mcptransport.StdioToolResult{}, errors.New("MCP dispatch permit is required")
	}
	if os.Getenv("POLIS_MCP_STREAMABLE_HTTP_ENABLED") != "1" {
		return mcptransport.StdioToolResult{}, errors.New("Streamable HTTP MCP Worker egress is disabled")
	}
	toolList, err := process.client.ListTools(ctx)
	if err != nil {
		return mcptransport.StdioToolResult{}, err
	}
	tools, observedSchema, err := mcptransport.PrepareStreamableHTTPToolList(toolList)
	if err != nil {
		return mcptransport.StdioToolResult{}, err
	}
	if observedSchema != process.toolSchema {
		return mcptransport.StdioToolResult{}, &mcptransport.ToolSchemaDriftError{ObservedDigest: observedSchema}
	}
	var selected *mcptransport.StdioToolDefinition
	for index := range tools {
		if tools[index].Name == name {
			selected = &tools[index]
			break
		}
	}
	if selected == nil {
		return mcptransport.StdioToolResult{}, errors.New("Streamable HTTP MCP tool is outside the approved schema")
	}
	if err = mcptransport.ValidatePinnedToolArguments(selected.InputSchema, rawArguments); err != nil {
		return mcptransport.StdioToolResult{}, err
	}
	decoder := json.NewDecoder(bytes.NewReader(rawArguments))
	decoder.UseNumber()
	var arguments map[string]any
	if err = decoder.Decode(&arguments); err != nil || arguments == nil {
		return mcptransport.StdioToolResult{}, errors.New("Streamable HTTP MCP tool arguments are invalid")
	}
	if err = decoder.Decode(new(any)); err != io.EOF {
		return mcptransport.StdioToolResult{}, errors.New("Streamable HTTP MCP tool arguments contain trailing JSON")
	}
	if err = consume(ctx); err != nil {
		return mcptransport.StdioToolResult{}, err
	}
	result, err := process.client.CallTool(ctx, name, arguments, selected.InputSchema)
	if err != nil {
		return mcptransport.StdioToolResult{}, err
	}
	return mcptransport.PrepareStreamableHTTPToolResult(name, process.toolSchema, result)
}

func (process *streamableHTTPMCPProcess) Stop(context.Context) error {
	if process == nil || process.closed {
		return nil
	}
	process.closed = true
	process.client.CloseIdleConnections()
	return nil
}

func validSHA256Digest(value string) bool {
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

var _ streamableHTTPMCPProcessFactory = (*appContainerControlledMCPFactory)(nil)
var _ controlledMCPProcess = (*streamableHTTPMCPProcess)(nil)

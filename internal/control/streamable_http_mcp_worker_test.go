// pattern: Functional Core
package control

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"polis/internal/mcptransport"
)

type fakeStreamableHTTPMCPClient struct {
	toolList   json.RawMessage
	toolResult json.RawMessage
	listErr    error
	callErr    error
	listCalls  int
	callCalls  int
	callName   string
	callArgs   map[string]any
	callSchema json.RawMessage
	closed     int
}

func (client *fakeStreamableHTTPMCPClient) ListTools(context.Context) (json.RawMessage, error) {
	client.listCalls++
	return client.toolList, client.listErr
}

func (client *fakeStreamableHTTPMCPClient) CallTool(_ context.Context, name string, arguments map[string]any, schema json.RawMessage) (json.RawMessage, error) {
	client.callCalls++
	client.callName = name
	client.callArgs = arguments
	client.callSchema = append(json.RawMessage(nil), schema...)
	return client.toolResult, client.callErr
}

func (*fakeStreamableHTTPMCPClient) CloseIdleConnections() {}

func TestStreamableHTTPMCPWorkerChecksSchemaAndArgumentsBeforeCall(t *testing.T) {
	t.Setenv("POLIS_MCP_STREAMABLE_HTTP_ENABLED", "1")
	toolList := json.RawMessage(`{"tools":[{"name":"lookup","description":"Read a fixture item.","inputSchema":{"type":"object","properties":{"key":{"type":"string","minLength":1,"maxLength":32}},"required":["key"],"additionalProperties":false}}]}`)
	_, digest, err := mcptransport.PrepareStreamableHTTPToolList(toolList)
	if err != nil {
		t.Fatal(err)
	}
	client := &fakeStreamableHTTPMCPClient{
		toolList:   toolList,
		toolResult: json.RawMessage(`{"content":[{"type":"text","text":"untrusted fixture data"}]}`),
	}
	process := newStreamableHTTPMCPProcess(client, digest)
	result, err := process.CallToolWithPermit(context.Background(), "lookup", []byte(`{"key":"fixture"}`), func(context.Context) error { return nil })
	if err != nil || result.ContentBoundary != mcptransport.StreamableHTTPResultContentBoundary || result.Content[0].Text != "untrusted fixture data" {
		t.Fatalf("Streamable HTTP Worker result=%+v error=%v", result, err)
	}
	if client.listCalls != 1 || client.callCalls != 1 || client.callName != "lookup" || client.callArgs["key"] != "fixture" || len(client.callSchema) == 0 {
		t.Fatalf("Streamable HTTP Worker client calls=%d/%d name=%q args=%v schema=%s", client.listCalls, client.callCalls, client.callName, client.callArgs, client.callSchema)
	}
	if _, err = process.CallToolWithPermit(context.Background(), "lookup", []byte(`{"key":7}`), func(context.Context) error { return nil }); err == nil || client.callCalls != 1 {
		t.Fatalf("invalid argument reached remote call: calls=%d error=%v", client.callCalls, err)
	}
}

func TestStreamableHTTPMCPWorkerBlocksSchemaDriftBeforeCall(t *testing.T) {
	t.Setenv("POLIS_MCP_STREAMABLE_HTTP_ENABLED", "1")
	approvedToolList := json.RawMessage(`{"tools":[{"name":"lookup","inputSchema":{"type":"object","properties":{"key":{"type":"string"}},"additionalProperties":false}}]}`)
	changedToolList := json.RawMessage(`{"tools":[{"name":"lookup","inputSchema":{"type":"object","properties":{"key":{"type":"integer"}},"additionalProperties":false}}]}`)
	_, approvedDigest, err := mcptransport.PrepareStreamableHTTPToolList(approvedToolList)
	if err != nil {
		t.Fatal(err)
	}
	client := &fakeStreamableHTTPMCPClient{toolList: changedToolList}
	process := newStreamableHTTPMCPProcess(client, approvedDigest)
	_, err = process.CallToolWithPermit(context.Background(), "lookup", []byte(`{"key":"fixture"}`), func(context.Context) error { return nil })
	var drift *mcptransport.ToolSchemaDriftError
	if !errors.As(err, &drift) || client.callCalls != 0 || drift.ObservedDigest == approvedDigest {
		t.Fatalf("changed remote schema was not blocked before call: err=%v calls=%d drift=%+v", err, client.callCalls, drift)
	}
}

func TestStreamableHTTPMCPWorkerRejectsErrorAndMediaResults(t *testing.T) {
	t.Setenv("POLIS_MCP_STREAMABLE_HTTP_ENABLED", "1")
	toolList := json.RawMessage(`{"tools":[{"name":"lookup","inputSchema":{"type":"object","properties":{"key":{"type":"string"}},"additionalProperties":false}}]}`)
	_, digest, err := mcptransport.PrepareStreamableHTTPToolList(toolList)
	if err != nil {
		t.Fatal(err)
	}
	for _, rawResult := range []string{
		`{"isError":true,"content":[{"type":"text","text":"remote error"}]}`,
		`{"content":[{"type":"image","data":"AA==","mimeType":"image/png"}]}`,
	} {
		client := &fakeStreamableHTTPMCPClient{toolList: toolList, toolResult: json.RawMessage(rawResult)}
		process := newStreamableHTTPMCPProcess(client, digest)
		if _, err = process.CallToolWithPermit(context.Background(), "lookup", []byte(`{"key":"fixture"}`), func(context.Context) error { return nil }); err == nil {
			t.Fatalf("unsupported result was accepted: %s", rawResult)
		}
	}
}

func TestStreamableHTTPMCPWorkerEgressGateDefaultsOff(t *testing.T) {
	const schema = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	t.Setenv("POLIS_MCP_STREAMABLE_HTTP_ENABLED", "")
	if _, err := (directStreamableHTTPMCPProcessFactory{}).StartStreamableHTTP(context.Background(), "https://mcp.example.com/v1/mcp", schema); err == nil {
		t.Fatal("Streamable HTTP MCP Worker egress was enabled without the explicit configuration gate")
	}
	t.Setenv("POLIS_MCP_STREAMABLE_HTTP_ENABLED", "1")
	process, err := (directStreamableHTTPMCPProcessFactory{}).StartStreamableHTTP(context.Background(), "https://mcp.example.com/v1/mcp", schema)
	if err != nil || process.ToolSchemaSHA256() != schema {
		t.Fatalf("explicitly enabled HTTP process construction (no request): process=%v error=%v", process, err)
	}
	if err = process.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}
}

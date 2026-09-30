// pattern: Functional Core
package mcptransport

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"
)

func TestStreamableHTTPUsesPinned20260728RequestMetadata(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost || request.URL.Path != "/mcp" {
			t.Errorf("request route=%s %s", request.Method, request.URL.Path)
		}
		if request.Header.Get("Accept") != "application/json, text/event-stream" || request.Header.Get("MCP-Protocol-Version") != ProtocolVersion20260728 || request.Header.Get("Mcp-Method") != "tools/list" {
			t.Errorf("request headers=%v", request.Header)
		}
		if request.Header.Get("Mcp-Session-Id") != "" {
			t.Errorf("stateless profile sent a session header: %v", request.Header)
		}
		var body map[string]any
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		params, _ := body["params"].(map[string]any)
		metadata, _ := params["_meta"].(map[string]any)
		if metadata[ProtocolVersionMetadataKey] != ProtocolVersion20260728 {
			t.Errorf("request metadata=%v", metadata)
		}
		response.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(response, `{"jsonrpc":"2.0","id":1,"result":{"tools":[]}}`)
	}))
	defer server.Close()
	client := newTestClient(server.URL+"/mcp", server.Client())
	result, err := client.ListTools(context.Background())
	if err != nil || string(result) != `{"tools":[]}` {
		t.Fatalf("tools/list result=%s error=%v", result, err)
	}
}

func TestStreamableHTTPReadsJSONAndRequestScopedSSE(t *testing.T) {
	cases := []struct {
		name        string
		contentType string
		body        string
	}{
		{name: "json", contentType: "application/json", body: `{"jsonrpc":"2.0","id":1,"result":{"ok":true}}`},
		{name: "sse", contentType: "text/event-stream", body: ": keepalive\n\nid: prime\ndata:\n\nevent: message\ndata: {\"jsonrpc\":\"2.0\",\"method\":\"notifications/progress\",\"params\":{}}\n\ndata: {\"jsonrpc\":\"2.0\",\"id\":1,\"result\":{\"ok\":true}}\n\n"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
				response.Header().Set("Content-Type", test.contentType)
				_, _ = fmt.Fprint(response, test.body)
			}))
			defer server.Close()
			result, err := newTestClient(server.URL, server.Client()).Request(context.Background(), "tools/list", map[string]any{})
			if err != nil || string(result) != `{"ok":true}` {
				t.Fatalf("response result=%s error=%v", result, err)
			}
		})
	}
}

func TestStreamableHTTPCallToolMirrorsSafeSchemaHeaders(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Mcp-Name") != "query" || request.Header.Get("Mcp-Param-Region") != "north-1" {
			t.Errorf("routable headers=%v", request.Header)
		}
		response.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(response, `{"jsonrpc":"2.0","id":1,"result":{"content":[]}}`)
	}))
	defer server.Close()
	schema := json.RawMessage(`{"type":"object","properties":{"region":{"type":"string","x-mcp-header":"Region"}}}`)
	_, err := newTestClient(server.URL, server.Client()).CallTool(context.Background(), "query", map[string]any{"region": "north-1"}, schema)
	if err != nil {
		t.Fatal(err)
	}
}

func TestStreamableHTTPRejectsUnsafeCustomHeaderSchemas(t *testing.T) {
	invalidSchemas := []string{
		`{"type":"object","properties":{"items":{"type":"array","items":{"type":"object","properties":{"region":{"type":"string","x-mcp-header":"Region"}}}}}}`,
		`{"type":"object","properties":{"region":{"type":"string","x-mcp-header":"Region"},"other":{"type":"string","x-mcp-header":"region"}}}`,
		`{"type":"object","properties":{"region":{"type":"number","x-mcp-header":"Region"}}}`,
		`{"type":"object","x-mcp-header":"Region"}`,
	}
	for _, schema := range invalidSchemas {
		if _, err := extractMCPParamHeaders(json.RawMessage(schema), map[string]any{"region": "north-1"}); err == nil {
			t.Fatalf("invalid MCP custom-header schema was accepted: %s", schema)
		}
	}
}

func TestStreamableHTTPEgressPolicyRejectsPrivateAndSpecialAddressRanges(t *testing.T) {
	for _, address := range []string{"127.0.0.1", "10.0.0.1", "100.64.0.1", "192.0.2.1", "198.18.0.1", "::2", "::1.2.3.4", "100::1", "2001:db8::1", "3fff::1", "4000::1", "5f00::1", "::1"} {
		if publicMCPAddress(netip.MustParseAddr(address)) {
			t.Fatalf("special address allowed for Streamable HTTP: %s", address)
		}
	}
	for _, address := range []string{"8.8.8.8", "2606:4700:4700::1111"} {
		if !publicMCPAddress(netip.MustParseAddr(address)) {
			t.Fatalf("public address denied for Streamable HTTP: %s", address)
		}
	}
}

func TestStreamableHTTPEncodesUnsafeNameAndParameterValues(t *testing.T) {
	value := "line1\nline2"
	encoded := encodeHeaderValue(value)
	if !strings.HasPrefix(encoded, "=?base64?") || !strings.HasSuffix(encoded, "?=") {
		t.Fatalf("unsafe header value was not encoded: %q", encoded)
	}
	decoded, err := base64.StdEncoding.DecodeString(strings.TrimSuffix(strings.TrimPrefix(encoded, "=?base64?"), "?="))
	if err != nil || string(decoded) != value {
		t.Fatalf("encoded header roundtrip=%q error=%v", decoded, err)
	}
	if got := encodeHeaderValue("region-1"); got != "region-1" {
		t.Fatalf("safe header value=%q", got)
	}
}

func TestStreamableHTTPRejectsMismatchedResponsesAndUnsupportedContentTypes(t *testing.T) {
	cases := []struct {
		name        string
		contentType string
		body        string
	}{
		{name: "id mismatch", contentType: "application/json", body: `{"jsonrpc":"2.0","id":99,"result":{}}`},
		{name: "legacy session error", contentType: "application/json", body: `{"jsonrpc":"2.0","id":1,"error":{"code":-32600,"message":"initialize required"}}`},
		{name: "unsupported media type", contentType: "text/plain", body: `not json`},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
				response.Header().Set("Content-Type", test.contentType)
				_, _ = fmt.Fprint(response, test.body)
			}))
			defer server.Close()
			if _, err := newTestClient(server.URL, server.Client()).ListTools(context.Background()); err == nil {
				t.Fatal("invalid server response was accepted")
			}
		})
	}
}

func TestStreamableHTTPRejectsUnpinnedOrCredentialedEndpoints(t *testing.T) {
	for _, endpoint := range []string{"http://example.com/mcp", "https://user:secret@example.com/mcp", "https://example.com/mcp?token=secret", "https://example.com/mcp?", "https://example.com/mcp#", "https://127.0.0.1/mcp", "https://[::1]/mcp", "https://[::2]/mcp", "https://[::1.2.3.4]/mcp", "https://[::ffff:8.8.8.8]/mcp", "https://[100::1]/mcp", "https://[3fff::1]/mcp", "https://[4000::1]/mcp", "https://[5f00::1]/mcp", "https://localhost/mcp", "https://mcp.example.com%2f.attacker/mcp"} {
		if _, err := NewClient(endpoint); err == nil {
			t.Fatalf("endpoint accepted: %s", endpoint)
		}
	}
}

func TestCanonicalStreamableHTTPEndpointPinsHostAndDefaultPort(t *testing.T) {
	canonical, err := CanonicalEndpoint("https://MCP.Example.com:443/v1/mcp")
	if err != nil || canonical != "https://mcp.example.com/v1/mcp" {
		t.Fatalf("canonical MCP endpoint=%q error=%v", canonical, err)
	}
}

func newTestClient(endpoint string, client *http.Client) *Client {
	return &Client{endpoint: endpoint, httpClient: client, maxResponseBytes: 1 << 20}
}

func TestStreamableHTTPToolSchemaUsesPinnedProfileAndCanonicalToolOrder(t *testing.T) {
	first := json.RawMessage(`{"tools":[{"name":"zeta","inputSchema":{"type":"object","properties":{"key":{"type":"string"}},"additionalProperties":false}},{"name":"alpha","inputSchema":{"type":"object","properties":{"id":{"type":"string"}},"additionalProperties":false}}]}`)
	second := json.RawMessage(`{"tools":[{"name":"alpha","inputSchema":{"type":"object","properties":{"id":{"type":"string"}},"additionalProperties":false}},{"name":"zeta","inputSchema":{"type":"object","properties":{"key":{"type":"string"}},"additionalProperties":false}}]}`)
	firstTools, firstDigest, err := PrepareStreamableHTTPToolList(first)
	if err != nil {
		t.Fatal(err)
	}
	secondTools, secondDigest, err := PrepareStreamableHTTPToolList(second)
	if err != nil {
		t.Fatal(err)
	}
	if firstDigest != secondDigest || len(firstTools) != 2 || firstTools[0].Name != "alpha" || secondTools[0].Name != "alpha" {
		t.Fatalf("tool list canonicalization differs: first=%+v/%s second=%+v/%s", firstTools, firstDigest, secondTools, secondDigest)
	}
	stdioDigest, err := StdioToolSchemaDigest(json.RawMessage(`[ {"name":"alpha","inputSchema":{"type":"object","properties":{"id":{"type":"string"}},"additionalProperties":false}},{"name":"zeta","inputSchema":{"type":"object","properties":{"key":{"type":"string"}},"additionalProperties":false}} ]`))
	if err != nil || firstDigest == stdioDigest {
		t.Fatalf("Streamable HTTP profile digest=%s stdio digest=%s error=%v", firstDigest, stdioDigest, err)
	}
}

func TestStreamableHTTPToolResultAcceptsOnlyBoundedTextContent(t *testing.T) {
	result, err := PrepareStreamableHTTPToolResult("lookup", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", json.RawMessage(`{"content":[{"type":"text","text":"untrusted remote result"}]}`))
	if err != nil || result.ContentBoundary != StreamableHTTPResultContentBoundary || len(result.Content) != 1 || result.Content[0].Text != "untrusted remote result" {
		t.Fatalf("Streamable HTTP tool result=%+v error=%v", result, err)
	}
	for _, raw := range []string{
		`{"isError":true,"content":[{"type":"text","text":"failed"}]}`,
		`{"content":[{"type":"image","data":"AA==","mimeType":"image/png"}]}`,
		`{"content":[{"type":"text","text":"line1"},{"type":"resource_link","uri":"https://example.test"}]}`,
	} {
		if _, err = PrepareStreamableHTTPToolResult("lookup", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", json.RawMessage(raw)); err == nil {
			t.Fatalf("unsupported Streamable HTTP tool result was accepted: %s", raw)
		}
	}
}

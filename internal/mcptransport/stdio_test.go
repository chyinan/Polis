// pattern: Imperative Shell
package mcptransport

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestStdioClientDiscoversPinnedModernServerListsToolsAndCallsWithSchemaBinding(t *testing.T) {
	var methods []string
	callCount := 0
	var stateMu sync.Mutex
	client := newStdioTestClient(t, func(method string, request map[string]json.RawMessage, callNumber int) map[string]any {
		var params map[string]json.RawMessage
		_ = json.Unmarshal(request["params"], &params)
		var metadata map[string]json.RawMessage
		_ = json.Unmarshal(params["_meta"], &metadata)
		var protocolVersion string
		_ = json.Unmarshal(metadata[stdioProtocolMetaKey], &protocolVersion)
		var clientInfo map[string]string
		_ = json.Unmarshal(metadata[stdioClientInfoKey], &clientInfo)
		if protocolVersion != stdioProtocolVersion || clientInfo["name"] != "Polis" || len(metadata[stdioClientCapsKey]) == 0 {
			t.Errorf("stdio request metadata=%v", metadata)
		}
		stateMu.Lock()
		methods = append(methods, method)
		stateMu.Unlock()
		switch method {
		case "server/discover":
			return stdioTestResponse(request, map[string]any{
				"resultType": "complete", "supportedVersions": []string{ProtocolVersion20260728},
				"capabilities": map[string]any{"tools": map[string]any{}},
				"_meta":        map[string]any{StdioServerInfoMetadataKey: map[string]any{"name": "read-only-reference", "version": "1.2.0"}},
				"ttlMs":        0, "cacheScope": "private",
			})
		case "tools/list":
			return stdioTestResponse(request, map[string]any{
				"resultType": "complete", "ttlMs": 0, "cacheScope": "private",
				"tools": []any{stdioTestTool()},
			})
		case "tools/call":
			var params map[string]json.RawMessage
			_ = json.Unmarshal(request["params"], &params)
			var arguments map[string]any
			_ = json.Unmarshal(params["arguments"], &arguments)
			if arguments["query"] != "needle" {
				t.Errorf("tools/call arguments=%v", arguments)
			}
			stateMu.Lock()
			callCount++
			stateMu.Unlock()
			return stdioTestResponse(request, map[string]any{
				"resultType": "complete", "isError": false,
				"content": []any{map[string]any{"type": "text", "text": "bounded reference result"}},
			})
		default:
			t.Errorf("unexpected MCP method %q", method)
			return stdioTestError(request, -32601)
		}
	})

	identity, err := client.Discover(context.Background())
	if err != nil || identity.Name != "read-only-reference" || identity.Version != "1.2.0" {
		t.Fatalf("server/discover identity=%+v error=%v", identity, err)
	}
	tools, schemaDigest, err := client.ListTools(context.Background())
	if err != nil || len(tools) != 1 || tools[0].Name != "lookup" || schemaDigest == "" {
		t.Fatalf("tools/list tools=%+v digest=%q error=%v", tools, schemaDigest, err)
	}
	result, err := client.CallTool(context.Background(), schemaDigest, "lookup", json.RawMessage(`{"query":"needle"}`))
	if err != nil || result.ToolName != "lookup" || result.ToolSchemaSHA256 != schemaDigest || result.ContentBoundary != StdioResultContentBoundary || len(result.Content) != 1 || result.Content[0].Text != "bounded reference result" {
		t.Fatalf("tools/call result=%+v error=%v", result, err)
	}
	stateMu.Lock()
	gotMethods := strings.Join(methods, ",")
	stateMu.Unlock()
	if gotMethods != "server/discover,tools/list,tools/list,tools/call" {
		t.Fatalf("MCP methods=%q, want discover/list/recheck/call", gotMethods)
	}
	if _, err = client.CallTool(context.Background(), schemaDigest, "lookup", json.RawMessage(`{"query":"needle","extra":true}`)); err == nil {
		t.Fatal("stdio MCP tool accepted an undeclared argument")
	}
	stateMu.Lock()
	gotCallCount := callCount
	stateMu.Unlock()
	if gotCallCount != 1 {
		t.Fatalf("stdio MCP dispatched %d calls after rejecting invalid arguments", gotCallCount)
	}
}

func TestStdioClientRejectsUnboundedToolSchemasAndMessages(t *testing.T) {
	invalidSchemas := []map[string]any{
		{"type": "object", "properties": map[string]any{"query": map[string]any{"type": "string"}}},
		{"type": "object", "additionalProperties": false, "properties": map[string]any{"query": map[string]any{"type": "object"}}},
		{"type": "object", "additionalProperties": false, "properties": map[string]any{"query": map[string]any{"type": "array"}}},
	}
	for index, schema := range invalidSchemas {
		client := newStdioTestClient(t, func(method string, request map[string]json.RawMessage, _ int) map[string]any {
			if method == "server/discover" {
				return stdioTestResponse(request, stdioTestDiscovery())
			}
			return stdioTestResponse(request, map[string]any{"resultType": "complete", "ttlMs": 0, "cacheScope": "private", "tools": []any{map[string]any{"name": "lookup", "inputSchema": schema}}})
		})
		if _, err := client.Discover(context.Background()); err != nil {
			t.Fatal(err)
		}
		if _, _, err := client.ListTools(context.Background()); err == nil {
			t.Fatalf("unbounded tool schema %d was accepted: %v", index, schema)
		}
		_ = client.Close()
	}
	oversized := bytes.Repeat([]byte{'x'}, MaxStdioMessageBytes+1)
	if _, err := readBoundedStdioLine(bufio.NewReader(bytes.NewReader(oversized))); err == nil {
		t.Fatal("stdio transport accepted an oversized message")
	}
}

func TestStdioClientRejectsToolSchemaDriftBeforeCalling(t *testing.T) {
	callCount := 0
	client := newStdioTestClient(t, func(method string, request map[string]json.RawMessage, callNumber int) map[string]any {
		if method == "server/discover" {
			return stdioTestResponse(request, stdioTestDiscovery())
		}
		if method == "tools/list" {
			tool := stdioTestTool()
			if callNumber > 2 {
				tool["description"] = "changed after approval"
			}
			return stdioTestResponse(request, map[string]any{"resultType": "complete", "ttlMs": 0, "cacheScope": "private", "tools": []any{tool}})
		}
		if method == "tools/call" {
			callCount++
		}
		return stdioTestResponse(request, map[string]any{"resultType": "complete", "content": []any{map[string]any{"type": "text", "text": "should not run"}}})
	})
	if _, err := client.Discover(context.Background()); err != nil {
		t.Fatal(err)
	}
	_, approvedDigest, err := client.ListTools(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if _, err = client.CallTool(context.Background(), approvedDigest, "lookup", json.RawMessage(`{"query":"needle"}`)); err == nil {
		t.Fatal("tool schema drift did not invalidate the approved capability")
	} else {
		var drift *ToolSchemaDriftError
		if !errors.As(err, &drift) || drift.ObservedDigest == "" || drift.ObservedDigest == approvedDigest {
			t.Fatalf("schema drift error=%v, want the changed observed schema digest", err)
		}
	}
	if callCount != 0 {
		t.Fatalf("MCP tool call count=%d after schema drift, want 0", callCount)
	}
}

func TestStdioClientRejectsIntegerArgumentThatRoundsAboveApprovedMaximum(t *testing.T) {
	var callMu sync.Mutex
	callCount := 0
	tool := map[string]any{
		"name": "bounded_count",
		"inputSchema": map[string]any{
			"type": "object", "additionalProperties": false, "required": []string{"count"},
			"properties": map[string]any{"count": map[string]any{"type": "integer", "maximum": json.Number("9007199254740992")}},
		},
	}
	client := newStdioTestClient(t, func(method string, request map[string]json.RawMessage, _ int) map[string]any {
		switch method {
		case "server/discover":
			return stdioTestResponse(request, stdioTestDiscovery())
		case "tools/list":
			return stdioTestResponse(request, map[string]any{"resultType": "complete", "ttlMs": 0, "cacheScope": "private", "tools": []any{tool}})
		case "tools/call":
			callMu.Lock()
			callCount++
			callMu.Unlock()
			return stdioTestResponse(request, map[string]any{"resultType": "complete", "content": []any{map[string]any{"type": "text", "text": "called"}}})
		default:
			return stdioTestError(request, -32601)
		}
	})
	if _, err := client.Discover(context.Background()); err != nil {
		t.Fatal(err)
	}
	_, digest, err := client.ListTools(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if _, err = client.CallTool(context.Background(), digest, "bounded_count", json.RawMessage(`{"count":9007199254740993}`)); err == nil {
		t.Fatal("integer above the exact approved maximum passed rounded float validation")
	}
	callMu.Lock()
	gotCalls := callCount
	callMu.Unlock()
	if gotCalls != 0 {
		t.Fatalf("stdio MCP dispatched %d calls after numeric bound rejection", gotCalls)
	}
}

func TestStdioNumericEnumsUseMathematicalEquality(t *testing.T) {
	cases := []struct {
		name      string
		enumValue json.Number
		argument  string
	}{
		{name: "decimal schema integer argument", enumValue: json.Number("1.0"), argument: `{"value":1}`},
		{name: "integer schema decimal argument", enumValue: json.Number("1"), argument: `{"value":1.0}`},
		{name: "exponent schema decimal argument", enumValue: json.Number("1e3"), argument: `{"value":1000}`},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			schema, err := validateStdioInputSchema(map[string]any{
				"type": "object", "additionalProperties": false, "required": []any{"value"},
				"properties": map[string]any{"value": map[string]any{"type": "number", "enum": []any{test.enumValue}}},
			})
			if err != nil {
				t.Fatal(err)
			}
			if err = validateStdioToolArguments(schema, json.RawMessage(test.argument)); err != nil {
				t.Fatalf("mathematically equal numeric enum was rejected: %v", err)
			}
		})
	}
}

func TestStdioClientRejectsMalformedToolContent(t *testing.T) {
	malformedResults := []struct {
		name   string
		result map[string]any
	}{
		{name: "missing content", result: map[string]any{"resultType": "complete"}},
		{name: "null content", result: map[string]any{"resultType": "complete", "content": nil}},
		{name: "missing text", result: map[string]any{"resultType": "complete", "content": []any{map[string]any{"type": "text"}}}},
		{name: "non-string text", result: map[string]any{"resultType": "complete", "content": []any{map[string]any{"type": "text", "text": 3}}}},
	}
	for _, test := range malformedResults {
		t.Run(test.name, func(t *testing.T) {
			client := newStdioTestClient(t, func(method string, request map[string]json.RawMessage, _ int) map[string]any {
				switch method {
				case "server/discover":
					return stdioTestResponse(request, stdioTestDiscovery())
				case "tools/list":
					return stdioTestResponse(request, map[string]any{"resultType": "complete", "ttlMs": 0, "cacheScope": "private", "tools": []any{stdioTestTool()}})
				case "tools/call":
					return stdioTestResponse(request, test.result)
				default:
					return stdioTestError(request, -32601)
				}
			})
			if _, err := client.Discover(context.Background()); err != nil {
				t.Fatal(err)
			}
			_, digest, err := client.ListTools(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			if _, err = client.CallTool(context.Background(), digest, "lookup", json.RawMessage(`{"query":"needle"}`)); err == nil {
				t.Fatal("malformed content was accepted as a successful empty result")
			}
		})
	}
	emptyTextClient := newStdioTestClient(t, func(method string, request map[string]json.RawMessage, _ int) map[string]any {
		switch method {
		case "server/discover":
			return stdioTestResponse(request, stdioTestDiscovery())
		case "tools/list":
			return stdioTestResponse(request, map[string]any{"resultType": "complete", "ttlMs": 0, "cacheScope": "private", "tools": []any{stdioTestTool()}})
		default:
			return stdioTestResponse(request, map[string]any{"resultType": "complete", "content": []any{map[string]any{"type": "text", "text": ""}}})
		}
	})
	if _, err := emptyTextClient.Discover(context.Background()); err != nil {
		t.Fatal(err)
	}
	_, emptyTextDigest, err := emptyTextClient.ListTools(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	emptyTextResult, err := emptyTextClient.CallTool(context.Background(), emptyTextDigest, "lookup", json.RawMessage(`{"query":"needle"}`))
	if err != nil || len(emptyTextResult.Content) != 1 || emptyTextResult.Content[0].Text != "" {
		t.Fatalf("explicit empty text result=%+v error=%v", emptyTextResult, err)
	}
}

func TestStdioClientRequiresDiscoveryCacheMetadata(t *testing.T) {
	client := newStdioTestClient(t, func(method string, request map[string]json.RawMessage, _ int) map[string]any {
		discovery := stdioTestDiscovery()
		delete(discovery, "ttlMs")
		return stdioTestResponse(request, discovery)
	})
	if _, err := client.Discover(context.Background()); err == nil {
		t.Fatal("stdio MCP discovery without required ttlMs was accepted")
	}
}

func TestStdioClientRejectsInvalidUTF8JSONRPCMessages(t *testing.T) {
	invalid := []byte{'{', '"', 'x', '"', ':', '"', 0xff, '"', '}'}
	var value map[string]any
	if err := decodeStrictStdioJSON(invalid, &value); err == nil {
		t.Fatal("stdio transport accepted a non-UTF-8 JSON-RPC message")
	}
}

func TestStdioClientRejectsLegacyServerAndNonTextToolContent(t *testing.T) {
	legacy := newStdioTestClient(t, func(method string, request map[string]json.RawMessage, _ int) map[string]any {
		if method == "server/discover" {
			return stdioTestError(request, -32601)
		}
		return stdioTestResponse(request, map[string]any{"resultType": "complete"})
	})
	if _, err := legacy.Discover(context.Background()); err == nil {
		t.Fatal("stdio client fell back to an unpinned legacy protocol")
	}

	client := newStdioTestClient(t, func(method string, request map[string]json.RawMessage, _ int) map[string]any {
		switch method {
		case "server/discover":
			return stdioTestResponse(request, stdioTestDiscovery())
		case "tools/list":
			return stdioTestResponse(request, map[string]any{"resultType": "complete", "ttlMs": 0, "cacheScope": "private", "tools": []any{stdioTestTool()}})
		default:
			return stdioTestResponse(request, map[string]any{"resultType": "complete", "content": []any{map[string]any{"type": "resource", "resource": map[string]any{}}}})
		}
	})
	if _, err := client.Discover(context.Background()); err != nil {
		t.Fatal(err)
	}
	_, digest, err := client.ListTools(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if _, err = client.CallTool(context.Background(), digest, "lookup", json.RawMessage(`{"query":"needle"}`)); err == nil {
		t.Fatal("non-text MCP content passed the fixed read-only result profile")
	}
}

func TestStdioClientSendsCancellationNotificationBeforeClosingThePipe(t *testing.T) {
	clientConn, serverConn := net.Pipe()
	client, err := NewStdioClient(clientConn)
	if err != nil {
		t.Fatal(err)
	}
	defer serverConn.Close()
	type receivedMessage struct {
		body json.RawMessage
		err  error
	}
	receivedRequest := make(chan receivedMessage, 1)
	receivedCancellation := make(chan receivedMessage, 1)
	go func() {
		reader := bufio.NewReader(serverConn)
		requestLine, readErr := reader.ReadBytes('\n')
		if readErr != nil {
			receivedRequest <- receivedMessage{err: readErr}
			return
		}
		receivedRequest <- receivedMessage{body: append(json.RawMessage(nil), requestLine...)}
		cancellationLine, readErr := reader.ReadBytes('\n')
		if readErr != nil {
			receivedCancellation <- receivedMessage{err: readErr}
			return
		}
		receivedCancellation <- receivedMessage{body: append(json.RawMessage(nil), cancellationLine...)}
		_, _ = io.Copy(io.Discard, reader)
	}()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, discoverErr := client.Discover(ctx)
		done <- discoverErr
	}()
	request := <-receivedRequest
	if request.err != nil {
		t.Fatal(request.err)
	}
	cancel()
	select {
	case err = <-done:
		if err == nil {
			t.Fatal("cancelled server/discover returned success")
		}
	case <-time.After(1500 * time.Millisecond):
		t.Fatal("stdio request did not stop after its context was cancelled")
	}
	cancellation := <-receivedCancellation
	if cancellation.err != nil {
		t.Fatalf("stdio cancellation notification was not sent before stream close: %v", cancellation.err)
	}
	var requestMessage, cancellationMessage map[string]json.RawMessage
	if err = json.Unmarshal(request.body, &requestMessage); err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(cancellation.body, &cancellationMessage); err != nil {
		t.Fatal(err)
	}
	var cancellationMethod string
	_ = json.Unmarshal(cancellationMessage["method"], &cancellationMethod)
	var cancellationParams map[string]json.RawMessage
	_ = json.Unmarshal(cancellationMessage["params"], &cancellationParams)
	if cancellationMethod != "notifications/cancelled" || !bytes.Equal(bytes.TrimSpace(requestMessage["id"]), bytes.TrimSpace(cancellationParams["requestId"])) {
		t.Fatalf("stdio cancellation does not reference the original request: request=%s notification=%s", request.body, cancellation.body)
	}
}

func newStdioTestClient(t *testing.T, handler func(string, map[string]json.RawMessage, int) map[string]any) *StdioClient {
	t.Helper()
	clientConn, serverConn := net.Pipe()
	client, err := NewStdioClient(clientConn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.Close(); _ = serverConn.Close() })
	go func() {
		reader := bufio.NewReader(serverConn)
		callNumber := 0
		for {
			line, readErr := reader.ReadBytes('\n')
			if readErr != nil {
				return
			}
			var request map[string]json.RawMessage
			if json.Unmarshal(line, &request) != nil {
				return
			}
			var method string
			_ = json.Unmarshal(request["method"], &method)
			callNumber++
			response := handler(method, request, callNumber)
			if json.NewEncoder(serverConn).Encode(response) != nil {
				return
			}
		}
	}()
	return client
}

func stdioTestResponse(request map[string]json.RawMessage, result any) map[string]any {
	var id any
	_ = json.Unmarshal(request["id"], &id)
	return map[string]any{"jsonrpc": "2.0", "id": id, "result": result}
}

func stdioTestError(request map[string]json.RawMessage, code int) map[string]any {
	var id any
	_ = json.Unmarshal(request["id"], &id)
	return map[string]any{"jsonrpc": "2.0", "id": id, "error": map[string]any{"code": code, "message": "fixture error"}}
}

func stdioTestDiscovery() map[string]any {
	return map[string]any{
		"resultType": "complete", "supportedVersions": []string{ProtocolVersion20260728},
		"capabilities": map[string]any{"tools": map[string]any{}},
		"_meta":        map[string]any{StdioServerInfoMetadataKey: map[string]any{"name": "read-only-reference", "version": "1.2.0"}},
		"ttlMs":        0, "cacheScope": "private",
	}
}

func stdioTestTool() map[string]any {
	return map[string]any{
		"name": "lookup", "description": "Read one bounded reference.",
		"inputSchema": map[string]any{
			"type": "object", "additionalProperties": false, "required": []string{"query"},
			"properties": map[string]any{"query": map[string]any{"type": "string", "minLength": 1, "maxLength": 64}},
		},
	}
}

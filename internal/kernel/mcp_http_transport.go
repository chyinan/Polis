package kernel

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"polis/internal/core"
)

const (
	mcpHTTPProbeTimeout = 20 * time.Second
	mcpHTTPCallTimeout  = 30 * time.Second
	mcpHTTPMaxBody      = 256 * 1024
)

func validateMCPHTTPEndpoint(raw string) (*url.URL, error) {
	if raw == "" || len(raw) > 512 || strings.ContainsAny(raw, "\r\n \t") {
		return nil, core.Malformed
	}
	u, err := url.Parse(raw)
	if err != nil || u == nil {
		return nil, core.Malformed
	}
	if u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return nil, core.Malformed
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, core.Malformed
	}
	if u.Hostname() == "" {
		return nil, core.Malformed
	}
	return u, nil
}

func isLocalMCPHost(host string) bool {
	h := strings.ToLower(strings.TrimSpace(host))
	if h == "localhost" || h == "127.0.0.1" || h == "::1" || h == "[::1]" {
		return true
	}
	ip := net.ParseIP(h)
	return ip != nil && ip.IsLoopback()
}

func postMCPHTTP(ctx context.Context, endpoint string, payload any) (json.RawMessage, int, error) {
	u, err := validateMCPHTTPEndpoint(endpoint)
	if err != nil {
		return nil, 0, err
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, 0, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), bytes.NewReader(body))
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	client := &http.Client{Timeout: 0} // context controls timeout
	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	status := resp.StatusCode
	if status == http.StatusAccepted || status == http.StatusNoContent {
		return nil, status, nil
	}
	if status < 200 || status >= 300 {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
		return nil, status, fmt.Errorf("%w: http status %d", core.Denied, status)
	}
	ct := resp.Header.Get("Content-Type")
	if strings.HasPrefix(ct, "text/event-stream") {
		msg, err := readFirstSSEJSON(resp.Body)
		return msg, status, err
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, mcpHTTPMaxBody+1))
	if err != nil {
		return nil, status, err
	}
	if len(data) > mcpHTTPMaxBody {
		return nil, status, core.TooLarge
	}
	data = bytes.TrimSpace(data)
	if len(data) == 0 {
		return nil, status, nil
	}
	return json.RawMessage(data), status, nil
}

func readFirstSSEJSON(r io.Reader) (json.RawMessage, error) {
	sc := bufio.NewScanner(r)
	buf := make([]byte, 0, 64*1024)
	sc.Buffer(buf, mcpHTTPMaxBody)
	var dataLines []string
	for sc.Scan() {
		line := sc.Text()
		if line == "" {
			if len(dataLines) > 0 {
				payload := strings.Join(dataLines, "\n")
				if strings.TrimSpace(payload) != "" {
					return json.RawMessage([]byte(payload)), nil
				}
				dataLines = nil
			}
			continue
		}
		if strings.HasPrefix(line, "data:") {
			dataLines = append(dataLines, strings.TrimSpace(strings.TrimPrefix(line, "data:")))
		}
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	if len(dataLines) > 0 {
		payload := strings.Join(dataLines, "\n")
		if strings.TrimSpace(payload) != "" {
			return json.RawMessage([]byte(payload)), nil
		}
	}
	return nil, core.Malformed
}

func httpRoundTrip(ctx context.Context, endpoint string, id int, method string, params any) (json.RawMessage, error) {
	body, _, err := postMCPHTTP(ctx, endpoint, map[string]any{"jsonrpc": "2.0", "id": id, "method": method, "params": params})
	if err != nil {
		return nil, err
	}
	if len(body) == 0 {
		return nil, core.Malformed
	}
	var msg struct {
		Result json.RawMessage `json:"result"`
		Error  json.RawMessage `json:"error"`
	}
	if err = json.Unmarshal(body, &msg); err != nil {
		return nil, core.Malformed
	}
	if len(msg.Error) > 0 && string(bytes.TrimSpace(msg.Error)) != "null" {
		return nil, fmt.Errorf("%w: mcp http %s failed", core.Denied, method)
	}
	if len(msg.Result) == 0 {
		return nil, core.Malformed
	}
	return msg.Result, nil
}

func QualifyStreamableHTTPMCP(ctx context.Context, endpoint string) (StdioProbeResult, error) {
	ctx, cancel := context.WithTimeout(ctx, mcpHTTPProbeTimeout)
	defer cancel()
	if _, err := validateMCPHTTPEndpoint(endpoint); err != nil {
		return StdioProbeResult{}, err
	}
	initResult, err := httpRoundTrip(ctx, endpoint, 1, "initialize", map[string]any{
		"protocolVersion": "2024-11-05",
		"capabilities":    map[string]any{},
		"clientInfo":      map[string]any{"name": "polis-capability-probe", "version": "0.15.0"},
	})
	if err != nil {
		return StdioProbeResult{}, err
	}
	// Best-effort initialized notification (no id).
	_, _, _ = postMCPHTTP(ctx, endpoint, map[string]any{"jsonrpc": "2.0", "method": "notifications/initialized", "params": map[string]any{}})
	toolsResult, err := httpRoundTrip(ctx, endpoint, 2, "tools/list", map[string]any{})
	if err != nil {
		return StdioProbeResult{}, err
	}
	resourcesResult, err := httpRoundTrip(ctx, endpoint, 3, "resources/list", map[string]any{})
	if err != nil {
		// resources/list may be unsupported; treat as empty for bounded qualification.
		resourcesResult = json.RawMessage(`{"resources":[]}`)
	}
	result, err := probeResult(initResult, toolsResult, resourcesResult)
	if err != nil {
		return StdioProbeResult{}, err
	}
	result.NetworkRestricted = false
	return result, nil
}

func CallStreamableHTTPMCPTool(ctx context.Context, endpoint, toolName string, arguments json.RawMessage) (StdioCallResult, error) {
	if toolName == "" || strings.ContainsAny(toolName, "\r\n\t") {
		return StdioCallResult{}, core.Malformed
	}
	if len(arguments) == 0 {
		arguments = json.RawMessage(`{}`)
	}
	if !json.Valid(arguments) || len(arguments) > maxToolCallArguments {
		if len(arguments) > maxToolCallArguments {
			return StdioCallResult{}, core.TooLarge
		}
		return StdioCallResult{}, core.Malformed
	}
	ctx, cancel := context.WithTimeout(ctx, mcpHTTPCallTimeout)
	defer cancel()
	if _, err := validateMCPHTTPEndpoint(endpoint); err != nil {
		return StdioCallResult{}, err
	}
	initResult, err := httpRoundTrip(ctx, endpoint, 1, "initialize", map[string]any{
		"protocolVersion": "2024-11-05",
		"capabilities":    map[string]any{},
		"clientInfo":      map[string]any{"name": "polis-tool-call", "version": "0.15.0"},
	})
	if err != nil {
		return StdioCallResult{}, err
	}
	_, _, _ = postMCPHTTP(ctx, endpoint, map[string]any{"jsonrpc": "2.0", "method": "notifications/initialized", "params": map[string]any{}})
	var init struct {
		ProtocolVersion string `json:"protocolVersion"`
		ServerInfo      struct {
			Name    string `json:"name"`
			Version string `json:"version"`
		} `json:"serverInfo"`
	}
	if err = json.Unmarshal(initResult, &init); err != nil {
		return StdioCallResult{}, core.Malformed
	}
	callResult, err := httpRoundTrip(ctx, endpoint, 2, "tools/call", map[string]any{
		"name":      toolName,
		"arguments": json.RawMessage(arguments),
	})
	if err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return StdioCallResult{ProtocolVersion: init.ProtocolVersion, ServerName: init.ServerInfo.Name, ServerVersion: init.ServerInfo.Version, ToolsCalled: 1}, context.DeadlineExceeded
		}
		return StdioCallResult{ProtocolVersion: init.ProtocolVersion, ServerName: init.ServerInfo.Name, ServerVersion: init.ServerInfo.Version, ToolsCalled: 1}, err
	}
	if len(callResult) > mcpCallMaxResult {
		return StdioCallResult{ProtocolVersion: init.ProtocolVersion, ServerName: init.ServerInfo.Name, ServerVersion: init.ServerInfo.Version, ToolsCalled: 1}, core.TooLarge
	}
	var envelope struct {
		IsError bool `json:"isError"`
	}
	_ = json.Unmarshal(callResult, &envelope)
	return StdioCallResult{ProtocolVersion: init.ProtocolVersion, ServerName: init.ServerInfo.Name, ServerVersion: init.ServerInfo.Version, Result: callResult, IsError: envelope.IsError, ToolsCalled: 1}, nil
}

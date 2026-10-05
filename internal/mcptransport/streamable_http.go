// pattern: Imperative Shell
package mcptransport

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
)

const (
	ProtocolVersion20260728    = "2026-07-28"
	ProtocolVersionMetadataKey = "io.modelcontextprotocol/protocolVersion"
	ClientInfoMetadataKey      = "io.modelcontextprotocol/clientInfo"
	ClientCapabilitiesKey      = "io.modelcontextprotocol/clientCapabilities"
	maxRequestBytes            = 1 << 20
	maxResponseBytes           = 1 << 20
	maxSSELineBytes            = 64 << 10
	maxSchemaDepth             = 32
	maxSchemaNodes             = 10_000
	maxHeaderValueBytes        = 8 << 10
	maxResolvedAddresses       = 16
	maxToolListDuration        = 2 * time.Minute
)

var errEndpointPolicy = errors.New("MCP endpoint is outside the fixed Streamable HTTP policy")

// IANA assigns IPv6 global unicast addresses from 2000::/3; IsGlobalUnicast
// alone also accepts reserved ranges outside this allocation.
var ipv6GlobalUnicast = netip.MustParsePrefix("2000::/3")

type Client struct {
	endpoint         string
	httpClient       *http.Client
	maxResponseBytes int64
	nextID           atomic.Uint64
}

func (client *Client) CloseIdleConnections() {
	if client != nil && client.httpClient != nil {
		client.httpClient.CloseIdleConnections()
	}
}

type HTTPStatusError struct{ StatusCode int }

func (err HTTPStatusError) Error() string {
	return fmt.Sprintf("MCP Streamable HTTP returned status %d", err.StatusCode)
}

type JSONRPCError struct{ Code int64 }

func (err JSONRPCError) Error() string {
	return fmt.Sprintf("MCP server returned JSON-RPC error %d", err.Code)
}

func NewClient(endpoint string) (*Client, error) {
	canonicalEndpoint, err := CanonicalEndpoint(endpoint)
	if err != nil {
		return nil, err
	}
	parsed, _ := url.Parse(canonicalEndpoint)
	hostname := strings.ToLower(parsed.Hostname())
	port := parsed.Port()
	if port == "" {
		port = "443"
	}
	address := net.JoinHostPort(hostname, port)
	transport := &http.Transport{
		Proxy:                 nil,
		TLSClientConfig:       &tls.Config{MinVersion: tls.VersionTLS12, ServerName: hostname},
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: 15 * time.Second,
		IdleConnTimeout:       30 * time.Second,
		MaxIdleConns:          2,
		MaxIdleConnsPerHost:   2,
		DisableCompression:    true,
		DialContext: func(ctx context.Context, network, requestedAddress string) (net.Conn, error) {
			if requestedAddress != address {
				return nil, errEndpointPolicy
			}
			ips, lookupErr := net.DefaultResolver.LookupIPAddr(ctx, hostname)
			if lookupErr != nil || len(ips) == 0 || len(ips) > maxResolvedAddresses {
				return nil, errors.Join(errEndpointPolicy, lookupErr)
			}
			dialer := &net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}
			var dialErrors []error
			for _, candidate := range ips {
				ip, ok := netip.AddrFromSlice(candidate.IP)
				if !ok || !publicMCPAddress(ip) {
					continue
				}
				connection, dialErr := dialer.DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
				if dialErr == nil {
					return connection, nil
				}
				dialErrors = append(dialErrors, dialErr)
			}
			return nil, errors.Join(append([]error{errEndpointPolicy}, dialErrors...)...)
		},
	}
	client := &http.Client{
		Transport: transport,
		Timeout:   45 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	return &Client{endpoint: canonicalEndpoint, httpClient: client, maxResponseBytes: maxResponseBytes}, nil
}

func CanonicalEndpoint(endpoint string) (string, error) {
	parsed, err := validateEndpoint(endpoint)
	if err != nil {
		return "", err
	}
	hostname := strings.ToLower(parsed.Hostname())
	port := parsed.Port()
	switch {
	case port == "" || port == "443":
		if strings.Contains(hostname, ":") {
			parsed.Host = "[" + hostname + "]"
		} else {
			parsed.Host = hostname
		}
	default:
		parsed.Host = net.JoinHostPort(hostname, port)
	}
	if parsed.Path == "" {
		parsed.Path = "/"
	}
	return parsed.String(), nil
}

func validateEndpoint(endpoint string) (*url.URL, error) {
	if len(endpoint) == 0 || len(endpoint) > 512 || strings.TrimSpace(endpoint) != endpoint {
		return nil, errEndpointPolicy
	}
	parsed, err := url.Parse(endpoint)
	if err != nil || !strings.EqualFold(parsed.Scheme, "https") || parsed.Hostname() == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.ForceQuery || parsed.Fragment != "" || parsed.RawFragment != "" || strings.Contains(endpoint, "#") || parsed.Opaque != "" {
		return nil, errEndpointPolicy
	}
	parsed.Scheme = "https"
	if ip, ipErr := netip.ParseAddr(parsed.Hostname()); ipErr == nil {
		if !publicMCPAddress(ip) {
			return nil, errEndpointPolicy
		}
	} else if !validMCPHostname(parsed.Hostname()) {
		return nil, errEndpointPolicy
	}
	if parsed.Port() != "" {
		port, portErr := strconv.Atoi(parsed.Port())
		if portErr != nil || port < 1 || port > 65535 {
			return nil, errEndpointPolicy
		}
	}
	return parsed, nil
}

func validMCPHostname(host string) bool {
	if host == "" || len(host) > 253 || strings.HasSuffix(host, ".") {
		return false
	}
	host = strings.ToLower(host)
	if host == "localhost" || strings.HasSuffix(host, ".localhost") || strings.HasSuffix(host, ".local") || strings.HasSuffix(host, ".internal") {
		return false
	}
	for _, label := range strings.Split(host, ".") {
		if len(label) == 0 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for _, character := range label {
			if !(character >= 'a' && character <= 'z' || character >= '0' && character <= '9' || character == '-') {
				return false
			}
		}
	}
	return true
}

func publicMCPAddress(ip netip.Addr) bool {
	if !ip.IsValid() || ip.Is4In6() || (ip.Is6() && !ipv6GlobalUnicast.Contains(ip)) {
		return false
	}
	ip = ip.Unmap()
	if !ip.IsValid() || !ip.IsGlobalUnicast() || ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsMulticast() || ip.IsUnspecified() {
		return false
	}
	blocked := []netip.Prefix{
		netip.MustParsePrefix("0.0.0.0/8"), netip.MustParsePrefix("100.64.0.0/10"),
		netip.MustParsePrefix("192.0.0.0/24"), netip.MustParsePrefix("192.0.2.0/24"),
		netip.MustParsePrefix("192.88.99.0/24"), netip.MustParsePrefix("198.18.0.0/15"),
		netip.MustParsePrefix("198.51.100.0/24"), netip.MustParsePrefix("203.0.113.0/24"),
		netip.MustParsePrefix("240.0.0.0/4"), netip.MustParsePrefix("64:ff9b::/96"),
		netip.MustParsePrefix("64:ff9b:1::/48"), netip.MustParsePrefix("100::/64"), netip.MustParsePrefix("2001:db8::/32"),
		netip.MustParsePrefix("3fff::/20"), netip.MustParsePrefix("5f00::/16"),
		netip.MustParsePrefix("2001::/23"), netip.MustParsePrefix("2002::/16"),
	}
	for _, prefix := range blocked {
		if prefix.Contains(ip) {
			return false
		}
	}
	return true
}

func (client *Client) ListTools(ctx context.Context) (json.RawMessage, error) {
	if client == nil || ctx == nil {
		return nil, errors.New("MCP Streamable HTTP tools/list context is unavailable")
	}
	listCtx, cancel := context.WithTimeout(ctx, maxToolListDuration)
	defer cancel()
	allTools := make([]json.RawMessage, 0)
	seenCursors := make(map[string]struct{})
	cursor := ""
	complete := false
	for pageNumber := 0; pageNumber < MaxMCPToolListPages; pageNumber++ {
		params := map[string]any{}
		if cursor != "" {
			params["cursor"] = cursor
		}
		result, err := client.Request(listCtx, "tools/list", params)
		if err != nil {
			return nil, err
		}
		pageTools, nextCursor, err := decodeStreamableHTTPToolListPage(result)
		if err != nil {
			return nil, err
		}
		if err = appendBoundedMCPToolDefinitions(&allTools, pageTools); err != nil {
			return nil, err
		}
		if nextCursor == "" {
			complete = true
			break
		}
		if err = validateMCPToolListCursor(nextCursor); err != nil {
			return nil, err
		}
		if _, exists := seenCursors[nextCursor]; exists {
			return nil, errors.New("Streamable HTTP MCP tool-list pagination repeated a cursor")
		}
		seenCursors[nextCursor] = struct{}{}
		if pageNumber == MaxMCPToolListPages-1 {
			return nil, errors.New("Streamable HTTP MCP tool-list pagination exceeded its page bound")
		}
		cursor = nextCursor
	}
	if !complete {
		return nil, errors.New("Streamable HTTP MCP tool-list pagination did not complete")
	}
	encoded, err := json.Marshal(struct {
		Tools []json.RawMessage `json:"tools"`
	}{Tools: allTools})
	if err != nil || len(encoded) > MaxStdioToolListBytes {
		return nil, errors.Join(errors.New("Streamable HTTP MCP tool catalog exceeds its aggregate-byte bound"), err)
	}
	return encoded, nil
}

func (client *Client) ListResources(ctx context.Context) (json.RawMessage, error) {
	return client.Request(ctx, "resources/list", map[string]any{})
}

func (client *Client) ReadResource(ctx context.Context, uri string) (json.RawMessage, error) {
	return client.Request(ctx, "resources/read", map[string]any{"uri": uri})
}

func (client *Client) ListPrompts(ctx context.Context) (json.RawMessage, error) {
	return client.Request(ctx, "prompts/list", map[string]any{})
}

func (client *Client) GetPrompt(ctx context.Context, name string, arguments map[string]any) (json.RawMessage, error) {
	if arguments == nil {
		arguments = map[string]any{}
	}
	return client.Request(ctx, "prompts/get", map[string]any{"name": name, "arguments": arguments})
}

func (client *Client) CallTool(ctx context.Context, name string, arguments map[string]any, inputSchema json.RawMessage) (json.RawMessage, error) {
	if arguments == nil {
		arguments = map[string]any{}
	}
	params := map[string]any{"name": name, "arguments": arguments}
	return client.request(ctx, "tools/call", params, inputSchema)
}

func (client *Client) Request(ctx context.Context, method string, params map[string]any) (json.RawMessage, error) {
	return client.request(ctx, method, params, nil)
}

func (client *Client) request(ctx context.Context, method string, params map[string]any, inputSchema json.RawMessage) (json.RawMessage, error) {
	if client == nil || client.httpClient == nil || ctx == nil || !supportedMethod(method) {
		return nil, errors.New("MCP Streamable HTTP request is invalid")
	}
	if params == nil {
		params = map[string]any{}
	} else {
		copied := make(map[string]any, len(params))
		for key, value := range params {
			copied[key] = value
		}
		params = copied
	}
	if _, exists := params["_meta"]; exists {
		return nil, errors.New("MCP request metadata is controlled by the client")
	}
	params["_meta"] = map[string]any{
		ProtocolVersionMetadataKey: ProtocolVersion20260728,
		ClientInfoMetadataKey:      map[string]string{"name": "Polis", "version": "0.1"},
		ClientCapabilitiesKey:      map[string]any{},
	}
	requestID := client.nextID.Add(1)
	message := map[string]any{"jsonrpc": "2.0", "id": requestID, "method": method, "params": params}
	content, err := json.Marshal(message)
	if err != nil || len(content) > maxRequestBytes {
		return nil, errors.Join(err, errors.New("MCP Streamable HTTP request exceeds its bound"))
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, client.endpoint, bytes.NewReader(content))
	if err != nil {
		return nil, errEndpointPolicy
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json, text/event-stream")
	request.Header.Set("MCP-Protocol-Version", ProtocolVersion20260728)
	request.Header.Set("Mcp-Method", method)
	if requiresNameHeader(method) {
		name, ok := standardHeaderName(method, params)
		if !ok || len(name) > maxHeaderValueBytes {
			return nil, errors.New("MCP request name or URI is missing or outside its bound")
		}
		request.Header.Set("Mcp-Name", encodeHeaderValue(name))
	}
	if method == "tools/call" {
		arguments, ok := params["arguments"].(map[string]any)
		if !ok {
			return nil, errors.New("MCP tool arguments must be an object")
		}
		extra, extraErr := extractMCPParamHeaders(inputSchema, arguments)
		if extraErr != nil {
			return nil, extraErr
		}
		for name, value := range extra {
			request.Header.Set(name, value)
		}
	}
	response, err := client.httpClient.Do(request)
	if err != nil {
		return nil, errors.Join(errors.New("MCP Streamable HTTP request failed"), err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, HTTPStatusError{StatusCode: response.StatusCode}
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, client.maxResponseBytes+1))
	if err != nil || int64(len(body)) > client.maxResponseBytes {
		return nil, errors.Join(errors.New("MCP Streamable HTTP response exceeds its bound or could not be read"), err)
	}
	contentType, _, err := mime.ParseMediaType(response.Header.Get("Content-Type"))
	if err != nil {
		return nil, errors.New("MCP Streamable HTTP response content type is invalid")
	}
	switch contentType {
	case "application/json":
		return extractJSONRPCResult(body, requestID)
	case "text/event-stream":
		return extractSSEResponse(body, requestID)
	default:
		return nil, errors.New("MCP Streamable HTTP response media type is unsupported")
	}
}

func supportedMethod(method string) bool {
	switch method {
	case "tools/list", "tools/call", "resources/list", "resources/read", "prompts/list", "prompts/get":
		return true
	default:
		return false
	}
}

func standardHeaderName(method string, params map[string]any) (string, bool) {
	key := ""
	switch method {
	case "tools/call", "prompts/get":
		key = "name"
	case "resources/read":
		key = "uri"
	default:
		return "", false
	}
	name, ok := params[key].(string)
	return name, ok && name != ""
}

func requiresNameHeader(method string) bool {
	switch method {
	case "tools/call", "resources/read", "prompts/get":
		return true
	default:
		return false
	}
}

func encodeHeaderValue(value string) string {
	unsafe := value == "" || strings.TrimSpace(value) != value || strings.HasPrefix(value, "=?base64?") && strings.HasSuffix(value, "?=")
	for _, character := range value {
		if character < 0x20 || character > 0x7e {
			unsafe = true
			break
		}
	}
	if unsafe {
		return "=?base64?" + base64.StdEncoding.EncodeToString([]byte(value)) + "?="
	}
	return value
}

type jsonRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  json.RawMessage `json:"result"`
	Error   *struct {
		Code int64 `json:"code"`
	} `json:"error"`
	Method string `json:"method"`
}

func extractJSONRPCResult(content []byte, expectedID uint64) (json.RawMessage, error) {
	return validateJSONRPCResponse(content, expectedID)
}

func extractSSEResponse(content []byte, expectedID uint64) (json.RawMessage, error) {
	if len(content) > maxResponseBytes {
		return nil, errors.New("MCP SSE response exceeds its bound")
	}
	scanner := bufio.NewScanner(bytes.NewReader(content))
	scanner.Buffer(make([]byte, 4096), maxSSELineBytes)
	var data []string
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			result, done, err := processSSEEvent(data, expectedID)
			if err != nil || done {
				return result, err
			}
			data = nil
			continue
		}
		if strings.HasPrefix(line, ":") {
			continue
		}
		field, value, found := strings.Cut(line, ":")
		if !found {
			field, value = line, ""
		}
		if strings.HasPrefix(value, " ") {
			value = strings.TrimPrefix(value, " ")
		}
		if field == "data" {
			data = append(data, value)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, errors.New("MCP SSE response line exceeded its bound")
	}
	result, done, err := processSSEEvent(data, expectedID)
	if err != nil {
		return nil, err
	}
	if done {
		return result, nil
	}
	return nil, errors.New("MCP SSE response ended without a matching JSON-RPC result")
}

func processSSEEvent(data []string, expectedID uint64) (json.RawMessage, bool, error) {
	if len(data) == 0 || strings.Join(data, "\n") == "" {
		return nil, false, nil
	}
	content := []byte(strings.Join(data, "\n"))
	var message jsonRPCResponse
	if err := json.Unmarshal(content, &message); err != nil || message.JSONRPC != "2.0" {
		return nil, false, errors.New("MCP SSE event is not a JSON-RPC message")
	}
	if message.Method != "" && len(message.ID) == 0 {
		return nil, false, nil
	}
	result, err := validateJSONRPCResponse(content, expectedID)
	return result, err == nil, err
}

func validateJSONRPCResponse(content []byte, expectedID uint64) (json.RawMessage, error) {
	if err := rejectDuplicateJSONKeys(content); err != nil {
		return nil, errors.Join(errors.New("MCP server response contains ambiguous JSON"), err)
	}
	var response jsonRPCResponse
	if err := json.Unmarshal(content, &response); err != nil || response.JSONRPC != "2.0" || len(response.ID) == 0 || response.Method != "" {
		return nil, errors.New("MCP server response is not a JSON-RPC response")
	}
	var responseID json.Number
	decoder := json.NewDecoder(bytes.NewReader(response.ID))
	decoder.UseNumber()
	if err := decoder.Decode(&responseID); err != nil || responseID.String() != strconv.FormatUint(expectedID, 10) {
		return nil, errors.New("MCP server response ID does not match the request")
	}
	if response.Error != nil {
		return nil, JSONRPCError{Code: response.Error.Code}
	}
	if len(response.Result) == 0 {
		return nil, errors.New("MCP server response omitted result")
	}
	return append(json.RawMessage(nil), response.Result...), nil
}

func extractMCPParamHeaders(schema json.RawMessage, arguments map[string]any) (map[string]string, error) {
	if len(schema) == 0 || len(schema) > maxRequestBytes {
		return nil, errors.New("MCP tool input schema is unavailable or outside its bound")
	}
	decoder := json.NewDecoder(bytes.NewReader(schema))
	decoder.UseNumber()
	var root map[string]any
	if err := decoder.Decode(&root); err != nil || root == nil || root["type"] != "object" {
		return nil, errors.New("MCP tool input schema is invalid")
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return nil, errors.New("MCP tool input schema contains trailing data")
	}
	type headerBinding struct {
		name     string
		typeName string
		path     []string
	}
	bindings := make([]headerBinding, 0)
	seenNames := make(map[string]struct{})
	nodes := 0
	countNode := func() error {
		nodes++
		if nodes > maxSchemaNodes {
			return errors.New("MCP tool schema exceeds its node bound")
		}
		return nil
	}
	var walk func(map[string]any, []string, bool, int) error
	walk = func(schemaNode map[string]any, path []string, staticPath bool, depth int) error {
		if depth > maxSchemaDepth {
			return errors.New("MCP tool schema exceeds its depth bound")
		}
		if err := countNode(); err != nil {
			return err
		}
		if annotation, exists := schemaNode["x-mcp-header"]; exists {
			name, ok := annotation.(string)
			if !ok || !staticPath || len(path) == 0 || !validHeaderToken(name) {
				return errors.New("MCP tool schema has an unsupported custom-header annotation")
			}
			typeName, ok := schemaNode["type"].(string)
			if !ok || (typeName != "string" && typeName != "integer" && typeName != "boolean") {
				return errors.New("MCP custom header requires a string, integer or boolean property")
			}
			key := strings.ToLower(name)
			if _, duplicate := seenNames[key]; duplicate {
				return errors.New("MCP tool schema repeats a custom-header name")
			}
			seenNames[key] = struct{}{}
			bindings = append(bindings, headerBinding{name: name, typeName: typeName, path: append([]string(nil), path...)})
		}
		if rawProperties, hasProperties := schemaNode["properties"]; hasProperties {
			propertySchemas, ok := rawProperties.(map[string]any)
			if !ok {
				return errors.New("MCP tool schema properties must be an object")
			}
			for property, raw := range propertySchemas {
				child, ok := raw.(map[string]any)
				if !ok {
					return errors.New("MCP tool property schema must be an object")
				}
				if err := walk(child, append(append([]string(nil), path...), property), staticPath, depth+1); err != nil {
					return err
				}
			}
		}
		for key, raw := range schemaNode {
			if key == "properties" || key == "x-mcp-header" {
				continue
			}
			if err := walkSchemaValue(raw, func(child map[string]any) error { return walk(child, nil, false, depth+1) }, depth+1, countNode); err != nil {
				return err
			}
		}
		return nil
	}
	if err := walk(root, nil, true, 0); err != nil {
		return nil, err
	}
	result := make(map[string]string, len(bindings))
	for _, binding := range bindings {
		value, present := valueAtPath(arguments, binding.path)
		if !present || value == nil {
			continue
		}
		text, err := scalarHeaderValue(value, binding.typeName)
		if err != nil || len(text) > maxHeaderValueBytes {
			return nil, errors.New("MCP tool argument cannot be represented as a bounded custom header")
		}
		result["Mcp-Param-"+binding.name] = encodeHeaderValue(text)
	}
	return result, nil
}

func walkSchemaValue(value any, visit func(map[string]any) error, depth int, countNode func() error) error {
	if depth > maxSchemaDepth {
		return errors.New("MCP tool schema exceeds its depth bound")
	}
	switch child := value.(type) {
	case map[string]any:
		return visit(child)
	case []any:
		if err := countNode(); err != nil {
			return err
		}
		for _, item := range child {
			if err := walkSchemaValue(item, visit, depth+1, countNode); err != nil {
				return err
			}
		}
	default:
		return countNode()
	}
	return nil
}

func valueAtPath(values map[string]any, path []string) (any, bool) {
	var current any = values
	for _, component := range path {
		object, ok := current.(map[string]any)
		if !ok {
			return nil, false
		}
		current, ok = object[component]
		if !ok {
			return nil, false
		}
	}
	return current, true
}

func scalarHeaderValue(value any, expectedType string) (string, error) {
	switch expectedType {
	case "string":
		text, ok := value.(string)
		if !ok {
			return "", errors.New("not a string")
		}
		return text, nil
	case "boolean":
		boolean, ok := value.(bool)
		if !ok {
			return "", errors.New("not a boolean")
		}
		return strconv.FormatBool(boolean), nil
	case "integer":
		var integer int64
		switch number := value.(type) {
		case json.Number:
			parsed, err := number.Int64()
			if err != nil {
				return "", err
			}
			integer = parsed
		case int:
			integer = int64(number)
		case int64:
			integer = number
		default:
			return "", errors.New("not an integer")
		}
		if integer < -9007199254740991 || integer > 9007199254740991 {
			return "", errors.New("integer exceeds JavaScript safe range")
		}
		return strconv.FormatInt(integer, 10), nil
	default:
		return "", errors.New("unsupported MCP header scalar")
	}
}

func validHeaderToken(value string) bool {
	if value == "" || len(value) > 64 {
		return false
	}
	for _, character := range value {
		if !(character >= 'a' && character <= 'z' || character >= 'A' && character <= 'Z' || character >= '0' && character <= '9' || strings.ContainsRune("!#$%&'*+-.^_`|~", character)) {
			return false
		}
	}
	return true
}

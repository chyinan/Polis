// pattern: Imperative Shell
package mcptransport

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync"
	"sync/atomic"
	"time"
	"unicode/utf8"
)

const (
	stdioProtocolVersion    = "2026-07-28"
	stdioProtocolMetaKey    = "io.modelcontextprotocol/protocolVersion"
	stdioClientInfoKey      = "io.modelcontextprotocol/clientInfo"
	stdioClientCapsKey      = "io.modelcontextprotocol/clientCapabilities"
	stdioRequestTimeout     = 30 * time.Second
	stdioCancelWriteTimeout = 250 * time.Millisecond
)

var (
	errStdioClosed            = errors.New("stdio MCP transport is closed")
	errStdioProtocol          = errors.New("stdio MCP peer violated the pinned protocol profile")
	errStdioToolNotFound      = errors.New("stdio MCP tool is not present in the approved schema")
	ErrStdioToolSchemaChanged = errors.New("stdio MCP tool schema changed after qualification")
)

type ToolSchemaDriftError struct {
	ObservedDigest string
}

func (err *ToolSchemaDriftError) Error() string { return ErrStdioToolSchemaChanged.Error() }
func (err *ToolSchemaDriftError) Unwrap() error { return ErrStdioToolSchemaChanged }

type StdioClient struct {
	stream      io.ReadWriteCloser
	reader      *bufio.Reader
	operationMu sync.Mutex
	closeOnce   sync.Once
	closed      atomic.Bool
	nextID      atomic.Uint64
	discovered  bool
	identity    StdioServerIdentity
	tools       []StdioToolDefinition
	toolSchemas map[string]stdioInputSchema
	toolDigest  string
}

type stdioRPCError struct {
	Code    int64           `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

type stdioRPCMessage struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Method  string          `json:"method"`
	Result  json.RawMessage `json:"result"`
	Error   *stdioRPCError  `json:"error"`
}

func NewStdioClient(stream io.ReadWriteCloser) (*StdioClient, error) {
	if stream == nil {
		return nil, errors.New("stdio MCP stream is required")
	}
	return &StdioClient{stream: stream, reader: bufio.NewReaderSize(stream, 64<<10)}, nil
}

// Close closes the transport streams. The process owner must then stop and
// wait for its sandboxed child; this client does not launch arbitrary commands.
func (client *StdioClient) Close() error {
	if client == nil {
		return nil
	}
	client.closed.Store(true)
	var closeErr error
	client.closeOnce.Do(func() { closeErr = client.stream.Close() })
	return closeErr
}

func (client *StdioClient) Discover(ctx context.Context) (StdioServerIdentity, error) {
	if ctx == nil || client == nil {
		return StdioServerIdentity{}, errStdioProtocol
	}
	client.operationMu.Lock()
	defer client.operationMu.Unlock()
	if client.discovered {
		return client.identity, nil
	}
	result, err := client.roundTripLocked(ctx, "server/discover", nil)
	if err != nil {
		return StdioServerIdentity{}, err
	}
	var discovery struct {
		ResultType        string                     `json:"resultType"`
		SupportedVersions []string                   `json:"supportedVersions"`
		Capabilities      map[string]json.RawMessage `json:"capabilities"`
		Metadata          map[string]json.RawMessage `json:"_meta"`
		Instructions      string                     `json:"instructions,omitempty"`
		TTLMS             *int64                     `json:"ttlMs"`
		CacheScope        string                     `json:"cacheScope"`
	}
	if err = decodeStrictStdioJSON(result, &discovery); err != nil || discovery.ResultType != "complete" || discovery.TTLMS == nil || *discovery.TTLMS < 0 || (discovery.CacheScope != "private" && discovery.CacheScope != "public") {
		return StdioServerIdentity{}, errors.Join(errStdioProtocol, err)
	}
	if len(discovery.Instructions) > MaxStdioToolDescriptionSize || !validStdioText(discovery.Instructions) {
		return StdioServerIdentity{}, errors.New("stdio MCP discovery instructions are invalid or oversized")
	}
	versionSupported := false
	for _, version := range discovery.SupportedVersions {
		if version == stdioProtocolVersion {
			versionSupported = true
		}
	}
	if !versionSupported || len(discovery.Capabilities) != 1 {
		return StdioServerIdentity{}, errStdioProtocol
	}
	toolsCapability, hasTools := discovery.Capabilities["tools"]
	var emptyToolsCapability map[string]json.RawMessage
	if !hasTools || json.Unmarshal(toolsCapability, &emptyToolsCapability) != nil || emptyToolsCapability == nil || len(emptyToolsCapability) != 0 {
		return StdioServerIdentity{}, errors.New("stdio MCP server must expose only the controlled tools capability")
	}
	serverInfoRaw, exists := discovery.Metadata[StdioServerInfoMetadataKey]
	if !exists {
		return StdioServerIdentity{}, errors.New("stdio MCP discovery omitted its server identity")
	}
	var identity StdioServerIdentity
	if err = decodeStrictStdioJSON(serverInfoRaw, &identity); err != nil || !validStdioToolName(identity.Name) || !validStdioToolName(identity.Version) || len(identity.Title) > MaxStdioToolDescriptionSize || len(identity.Description) > MaxStdioToolDescriptionSize || len(identity.WebsiteURL) > 2048 || !validStdioText(identity.Title) || !validStdioText(identity.Description) {
		return StdioServerIdentity{}, errors.Join(errors.New("stdio MCP server identity is invalid"), err)
	}
	client.identity = identity
	client.discovered = true
	return identity, nil
}

func (client *StdioClient) ListTools(ctx context.Context) ([]StdioToolDefinition, string, error) {
	if ctx == nil || client == nil {
		return nil, "", errStdioProtocol
	}
	client.operationMu.Lock()
	defer client.operationMu.Unlock()
	if !client.discovered {
		return nil, "", errors.New("stdio MCP discovery must complete before tools/list")
	}
	return client.listToolsLocked(ctx)
}

func (client *StdioClient) listToolsLocked(ctx context.Context) ([]StdioToolDefinition, string, error) {
	listCtx, cancel := context.WithTimeout(ctx, stdioRequestTimeout)
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
		result, err := client.roundTripLocked(listCtx, "tools/list", params)
		if err != nil {
			return nil, "", err
		}
		var response struct {
			ResultType string                     `json:"resultType"`
			Tools      json.RawMessage            `json:"tools"`
			NextCursor string                     `json:"nextCursor,omitempty"`
			Metadata   map[string]json.RawMessage `json:"_meta,omitempty"`
			TTLMS      *int64                     `json:"ttlMs"`
			CacheScope string                     `json:"cacheScope"`
		}
		if err = decodeStrictStdioJSON(result, &response); err != nil || response.ResultType != "complete" || response.TTLMS == nil || *response.TTLMS < 0 || (response.CacheScope != "private" && response.CacheScope != "public") {
			return nil, "", errors.Join(errStdioProtocol, err)
		}
		pageTools, pageErr := decodeMCPToolDefinitionsPage(response.Tools)
		if pageErr != nil {
			return nil, "", errors.Join(errStdioProtocol, pageErr)
		}
		if err = appendBoundedMCPToolDefinitions(&allTools, pageTools); err != nil {
			return nil, "", err
		}
		if response.NextCursor == "" {
			complete = true
			break
		}
		if err = validateMCPToolListCursor(response.NextCursor); err != nil {
			return nil, "", err
		}
		if _, exists := seenCursors[response.NextCursor]; exists {
			return nil, "", errors.New("stdio MCP tool-list pagination repeated a cursor")
		}
		seenCursors[response.NextCursor] = struct{}{}
		if pageNumber == MaxMCPToolListPages-1 {
			return nil, "", errors.New("stdio MCP tool-list pagination exceeded its page bound")
		}
		cursor = response.NextCursor
	}
	if !complete {
		return nil, "", errors.New("stdio MCP tool-list pagination did not complete")
	}
	encodedTools, err := json.Marshal(allTools)
	if err != nil {
		return nil, "", err
	}
	tools, schemas, digest, err := prepareStdioToolDefinitions(encodedTools)
	if err != nil {
		return nil, "", err
	}
	client.tools, client.toolSchemas, client.toolDigest = tools, schemas, digest
	return cloneStdioTools(tools), digest, nil
}

// CallTool re-reads the tool list immediately before dispatch. Any changed
// schema digest invalidates the caller's approval and blocks the process call.
func (client *StdioClient) CallTool(ctx context.Context, approvedToolSchemaSHA256, name string, arguments json.RawMessage) (StdioToolResult, error) {
	if ctx == nil || client == nil || !validStdioSHA256(approvedToolSchemaSHA256) || !validStdioToolName(name) {
		return StdioToolResult{}, errStdioProtocol
	}
	client.operationMu.Lock()
	defer client.operationMu.Unlock()
	if !client.discovered {
		return StdioToolResult{}, errors.New("stdio MCP discovery must complete before tools/call")
	}
	_, digest, err := client.listToolsLocked(ctx)
	if err != nil {
		return StdioToolResult{}, err
	}
	if digest != approvedToolSchemaSHA256 {
		return StdioToolResult{}, &ToolSchemaDriftError{ObservedDigest: digest}
	}
	if _, exists := client.toolSchemas[name]; !exists {
		return StdioToolResult{}, errStdioToolNotFound
	}
	if len(arguments) == 0 || len(arguments) > MaxStdioCallArgumentsBytes {
		return StdioToolResult{}, errors.New("stdio MCP call arguments are empty or oversized")
	}
	if err = validateStdioToolArguments(client.toolSchemas[name], arguments); err != nil {
		return StdioToolResult{}, err
	}
	var argumentObject map[string]any
	decoder := json.NewDecoder(bytes.NewReader(arguments))
	decoder.UseNumber()
	if err = decoder.Decode(&argumentObject); err != nil || argumentObject == nil {
		return StdioToolResult{}, errors.New("stdio MCP call arguments must be an object")
	}
	result, err := client.roundTripLocked(ctx, "tools/call", map[string]any{"name": name, "arguments": argumentObject})
	if err != nil {
		return StdioToolResult{}, err
	}
	parsed, err := parseStdioToolResult(result)
	if err != nil {
		return StdioToolResult{}, err
	}
	return StdioToolResult{ToolName: name, ToolSchemaSHA256: digest, ContentBoundary: StdioResultContentBoundary, Content: parsed}, nil
}

func (client *StdioClient) roundTripLocked(ctx context.Context, method string, supplied map[string]any) (json.RawMessage, error) {
	ctx, cancel := context.WithTimeout(ctx, stdioRequestTimeout)
	defer cancel()
	if client.closed.Load() {
		return nil, errStdioClosed
	}
	params := make(map[string]any, len(supplied)+1)
	for key, value := range supplied {
		if key == "_meta" {
			return nil, errors.New("stdio MCP request metadata is owned by the transport")
		}
		params[key] = value
	}
	params["_meta"] = map[string]any{
		stdioProtocolMetaKey: stdioProtocolVersion,
		stdioClientInfoKey:   map[string]string{"name": "Polis", "version": "0.1"},
		stdioClientCapsKey:   map[string]any{},
	}
	requestID := client.nextID.Add(1)
	message, err := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": requestID, "method": method, "params": params})
	if err != nil || len(message)+1 > MaxStdioMessageBytes {
		return nil, errors.Join(err, errors.New("stdio MCP request exceeds its bound"))
	}
	message = append(message, '\n')
	writeDone := make(chan error, 1)
	go func() { writeDone <- writeStdioMessage(client.stream, message) }()
	select {
	case <-ctx.Done():
		if waitForCompletedStdioWrite(writeDone) {
			client.sendCancellationNotification(requestID)
		}
		_ = client.Close()
		return nil, ctx.Err()
	case err = <-writeDone:
		if err != nil {
			_ = client.Close()
			return nil, errors.Join(errors.New("stdio MCP request write failed"), err)
		}
	}
	responseDone := make(chan stdioRPCMessage, 1)
	responseError := make(chan error, 1)
	go func() {
		response, readErr := client.readResponse(requestID)
		if readErr != nil {
			responseError <- readErr
			return
		}
		responseDone <- response
	}()
	select {
	case <-ctx.Done():
		client.sendCancellationNotification(requestID)
		_ = client.Close()
		return nil, ctx.Err()
	case err = <-responseError:
		_ = client.Close()
		return nil, errors.Join(errors.New("stdio MCP response failed"), err)
	case response := <-responseDone:
		if response.Error != nil {
			return nil, JSONRPCError{Code: response.Error.Code}
		}
		return append(json.RawMessage(nil), response.Result...), nil
	}
}

func waitForCompletedStdioWrite(writeDone <-chan error) bool {
	timer := time.NewTimer(stdioCancelWriteTimeout)
	defer timer.Stop()
	select {
	case err := <-writeDone:
		return err == nil
	case <-timer.C:
		return false
	}
}

func (client *StdioClient) readResponse(requestID uint64) (stdioRPCMessage, error) {
	line, err := readBoundedStdioLine(client.reader)
	if err != nil {
		return stdioRPCMessage{}, err
	}
	var message stdioRPCMessage
	if err = decodeStrictStdioJSON(line, &message); err != nil {
		return stdioRPCMessage{}, err
	}
	if message.JSONRPC != "2.0" || message.Method != "" || !stdioResponseIDMatches(message.ID, requestID) || len(message.Result) == 0 && message.Error == nil {
		return stdioRPCMessage{}, errStdioProtocol
	}
	return message, nil
}

func (client *StdioClient) sendCancellationNotification(requestID uint64) {
	params := map[string]any{
		"requestId": requestID,
		"reason":    "client request context cancelled",
		"_meta": map[string]any{
			stdioProtocolMetaKey: stdioProtocolVersion,
			stdioClientInfoKey:   map[string]string{"name": "Polis", "version": "0.1"},
			stdioClientCapsKey:   map[string]any{},
		},
	}
	message, err := json.Marshal(map[string]any{"jsonrpc": "2.0", "method": "notifications/cancelled", "params": params})
	if err != nil || len(message)+1 > MaxStdioMessageBytes {
		return
	}
	message = append(message, '\n')
	writeDone := make(chan struct{}, 1)
	go func() {
		_ = writeStdioMessage(client.stream, message)
		writeDone <- struct{}{}
	}()
	timer := time.NewTimer(stdioCancelWriteTimeout)
	defer timer.Stop()
	select {
	case <-writeDone:
	case <-timer.C:
	}
}

func readBoundedStdioLine(reader *bufio.Reader) ([]byte, error) {
	line := make([]byte, 0, 4096)
	for {
		part, err := reader.ReadSlice('\n')
		if len(line)+len(part) > MaxStdioMessageBytes {
			return nil, errors.New("stdio MCP message exceeds its byte bound")
		}
		line = append(line, part...)
		if err == bufio.ErrBufferFull {
			continue
		}
		if err != nil {
			return nil, err
		}
		line = bytes.TrimSuffix(line, []byte{'\n'})
		line = bytes.TrimSuffix(line, []byte{'\r'})
		if len(line) == 0 {
			return nil, errors.New("stdio MCP message line is empty")
		}
		return line, nil
	}
}

func writeStdioMessage(writer io.Writer, message []byte) error {
	for len(message) > 0 {
		written, err := writer.Write(message)
		if err != nil {
			return err
		}
		if written <= 0 {
			return io.ErrShortWrite
		}
		message = message[written:]
	}
	return nil
}

func decodeStrictStdioJSON(raw []byte, target any) error {
	if len(raw) == 0 || len(raw) > MaxStdioMessageBytes || !utf8.Valid(raw) || !json.Valid(raw) {
		return errStdioProtocol
	}
	if err := rejectDuplicateJSONKeys(raw); err != nil {
		return errors.Join(errStdioProtocol, err)
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return errStdioProtocol
	}
	return nil
}

func stdioResponseIDMatches(raw json.RawMessage, expected uint64) bool {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var actual json.Number
	if err := decoder.Decode(&actual); err != nil {
		return false
	}
	return actual.String() == fmt.Sprintf("%d", expected)
}

func parseStdioToolResult(raw json.RawMessage) ([]StdioTextContent, error) {
	var result struct {
		ResultType        string                     `json:"resultType"`
		IsError           bool                       `json:"isError"`
		Content           json.RawMessage            `json:"content"`
		StructuredContent json.RawMessage            `json:"structuredContent"`
		Metadata          map[string]json.RawMessage `json:"_meta,omitempty"`
	}
	if err := decodeStrictStdioJSON(raw, &result); err != nil || result.ResultType != "complete" || result.IsError {
		return nil, errors.Join(errors.New("stdio MCP tool returned an incomplete or failed result"), err)
	}
	if len(result.StructuredContent) > 0 && string(result.StructuredContent) != "null" {
		return nil, errors.New("stdio MCP structured results are outside the controlled text profile")
	}
	if len(result.Content) == 0 || bytes.Equal(bytes.TrimSpace(result.Content), []byte("null")) {
		return nil, errors.New("stdio MCP tool result content is missing or null")
	}
	var contentBlocks []json.RawMessage
	if err := decodeStrictStdioJSON(result.Content, &contentBlocks); err != nil || contentBlocks == nil {
		return nil, errors.Join(errors.New("stdio MCP tool result content must be an array"), err)
	}
	content := make([]StdioTextContent, 0, len(contentBlocks))
	var totalBytes int
	for _, rawPart := range contentBlocks {
		var part struct {
			Type        string          `json:"type"`
			Text        *string         `json:"text"`
			Annotations json.RawMessage `json:"annotations,omitempty"`
			Metadata    json.RawMessage `json:"_meta,omitempty"`
		}
		if err := decodeStrictStdioJSON(rawPart, &part); err != nil || part.Type != "text" || part.Text == nil || !validStdioText(*part.Text) {
			return nil, errors.Join(errors.New("stdio MCP result contains unsupported content"), err)
		}
		totalBytes += len(*part.Text)
		if totalBytes > MaxStdioToolResultBytes {
			return nil, errors.New("stdio MCP text result exceeds its byte bound")
		}
		content = append(content, StdioTextContent{Text: *part.Text})
	}
	return content, nil
}

func cloneStdioTools(source []StdioToolDefinition) []StdioToolDefinition {
	clone := make([]StdioToolDefinition, len(source))
	for index, tool := range source {
		clone[index] = tool
		clone[index].InputSchema = append(json.RawMessage(nil), tool.InputSchema...)
	}
	return clone
}

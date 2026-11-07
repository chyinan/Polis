// pattern: Imperative Shell
package kernel

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"polis/internal/core"
)

const (
	mcpProbeTimeout     = 20 * time.Second
	mcpProbeMaxMessage  = 256 * 1024
	mcpProbeMaxMessages = 16
)

type StdioProbeResult struct {
	ProtocolVersion   string                  `json:"protocolVersion"`
	ServerName        string                  `json:"serverName"`
	ServerVersion     string                  `json:"serverVersion"`
	ToolNames         []string                `json:"toolNames"`
	ToolDescriptors   []CachedToolDescriptor  `json:"toolDescriptors"`
	ResourceSummaries []CachedResourceSummary `json:"resourceSummaries"`
	DescriptorDigest  string                  `json:"descriptorDigest"`
	ToolsCalled       int                     `json:"toolsCalled"`
	NetworkRestricted bool                    `json:"networkRestricted"`
}

func QualifyStdioMCP(ctx context.Context, command string, args []string) (StdioProbeResult, error) {
	return qualifyStdioMCP(ctx, command, args, nil)
}

func qualifyStdioMCP(ctx context.Context, command string, args []string, extraEnv []string) (StdioProbeResult, error) {
	if err := validateProbeCommand(command, args); err != nil {
		return StdioProbeResult{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, mcpProbeTimeout)
	defer cancel()
	workDir, err := os.MkdirTemp("", "polis-mcp-probe-")
	if err != nil {
		return StdioProbeResult{}, err
	}
	defer os.RemoveAll(workDir)
	cmd := exec.CommandContext(ctx, command, args...)
	cmd.Dir = workDir
	cmd.Env = append(probeEnvironment(), extraEnv...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return StdioProbeResult{}, err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return StdioProbeResult{}, err
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return StdioProbeResult{}, err
	}
	if err = cmd.Start(); err != nil {
		return StdioProbeResult{}, err
	}
	defer func() {
		_ = stdin.Close()
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		_ = cmd.Wait()
	}()
	go func() { _, _ = io.Copy(io.Discard, io.LimitReader(stderr, 4096)) }()
	reader := bufio.NewReader(stdout)
	initResult, err := roundTrip(ctx, stdin, reader, 1, "initialize", map[string]any{
		"protocolVersion": "2024-11-05",
		"capabilities":    map[string]any{},
		"clientInfo":      map[string]any{"name": "polis-capability-probe", "version": "0.10.0"},
	})
	if err != nil {
		return StdioProbeResult{}, err
	}
	if err = writeMCP(stdin, rpcRequest{JSONRPC: "2.0", Method: "notifications/initialized"}); err != nil {
		return StdioProbeResult{}, err
	}
	toolsResult, err := roundTrip(ctx, stdin, reader, 2, "tools/list", map[string]any{})
	if err != nil {
		return StdioProbeResult{}, err
	}
	resourcesResult, err := roundTrip(ctx, stdin, reader, 3, "resources/list", map[string]any{})
	if errors.Is(err, core.Denied) {
		resourcesResult = json.RawMessage(`{"resources":[]}`)
		err = nil
	}
	if err != nil {
		return StdioProbeResult{}, err
	}
	return probeResult(initResult, toolsResult, resourcesResult)
}

func validateProbeCommand(command string, args []string) error {
	command = strings.TrimSpace(command)
	if command == "" || len(command) > 512 || strings.ContainsAny(command, "\r\n") || len(args) > maxManifestListItems {
		return core.Malformed
	}
	for _, arg := range args {
		if len(arg) > 512 || strings.ContainsAny(arg, "\r\n") {
			return core.Malformed
		}
	}
	return nil
}

func probeEnvironment() []string {
	keys := []string{"PATH", "PATHEXT", "SYSTEMROOT", "WINDIR", "COMSPEC", "TEMP", "TMP", "USERPROFILE", "HOMEDRIVE", "HOMEPATH", "PROGRAMFILES", "PROGRAMFILES(X86)", "PROGRAMDATA", "APPDATA", "LOCALAPPDATA", "COMMONPROGRAMFILES", "SYSTEMDRIVE"}
	env := make([]string, 0, len(keys))
	for _, key := range keys {
		if value, ok := os.LookupEnv(key); ok && !strings.ContainsAny(value, "\r\n") {
			env = append(env, key+"="+value)
		}
	}
	return env
}

type rpcRequest struct {
	JSONRPC string `json:"jsonrpc"`
	ID      int    `json:"id,omitempty"`
	Method  string `json:"method"`
	Params  any    `json:"params,omitempty"`
}

func roundTrip(ctx context.Context, stdin io.Writer, reader *bufio.Reader, id int, method string, params any) (json.RawMessage, error) {
	if err := writeMCP(stdin, rpcRequest{JSONRPC: "2.0", ID: id, Method: method, Params: params}); err != nil {
		return nil, err
	}
	want := strconv.Itoa(id)
	for range mcpProbeMaxMessages {
		body, err := readMCP(ctx, reader)
		if err != nil {
			return nil, err
		}
		var message struct {
			ID     json.RawMessage `json:"id"`
			Result json.RawMessage `json:"result"`
			Error  json.RawMessage `json:"error"`
		}
		if err = json.Unmarshal(body, &message); err != nil {
			return nil, core.Malformed
		}
		if len(message.ID) == 0 || string(bytes.TrimSpace(message.ID)) != want {
			continue
		}
		if len(message.Error) > 0 && string(message.Error) != "null" {
			return nil, fmt.Errorf("%w: mcp probe %s failed", core.Denied, method)
		}
		if len(message.Result) == 0 {
			return nil, core.Malformed
		}
		return message.Result, nil
	}
	return nil, core.Denied
}

func writeMCP(w io.Writer, payload any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	body = append(body, '\n')
	_, err = w.Write(body)
	return err
}

func readMCP(ctx context.Context, reader *bufio.Reader) ([]byte, error) {
	type frame struct {
		body []byte
		err  error
	}
	done := make(chan frame, 1)
	go func() {
		body, err := readMCPFrame(reader)
		done <- frame{body, err}
	}()
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case result := <-done:
		if result.err != nil {
			return nil, result.err
		}
		return result.body, nil
	}
}

func readMCPFrame(reader *bufio.Reader) ([]byte, error) {
	peek, err := reader.Peek(1)
	if err != nil {
		return nil, err
	}
	if peek[0] == '{' {
		line, err := reader.ReadBytes('\n')
		if err != nil && !errors.Is(err, io.EOF) {
			return nil, err
		}
		line = bytes.TrimSpace(line)
		if len(line) == 0 || len(line) > mcpProbeMaxMessage {
			return nil, core.Malformed
		}
		return line, nil
	}
	length := 0
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return nil, err
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			break
		}
		if strings.HasPrefix(strings.ToLower(line), "content-length:") {
			n, convErr := strconv.Atoi(strings.TrimSpace(line[len("content-length:"):]))
			if convErr != nil || n <= 0 || n > mcpProbeMaxMessage {
				return nil, core.Malformed
			}
			length = n
		}
	}
	if length == 0 {
		return nil, core.Malformed
	}
	body := make([]byte, length)
	_, err = io.ReadFull(reader, body)
	return body, err
}

func probeResult(initResult, toolsResult, resourcesResult json.RawMessage) (StdioProbeResult, error) {
	var init struct {
		ProtocolVersion string `json:"protocolVersion"`
		ServerInfo      struct {
			Name    string `json:"name"`
			Version string `json:"version"`
		} `json:"serverInfo"`
	}
	if err := json.Unmarshal(initResult, &init); err != nil || init.ProtocolVersion == "" || init.ServerInfo.Name == "" {
		return StdioProbeResult{}, core.Malformed
	}
	var listed struct {
		Tools []struct {
			Name         string          `json:"name"`
			Description  string          `json:"description"`
			InputSchema  json.RawMessage `json:"inputSchema"`
			OutputSchema json.RawMessage `json:"outputSchema"`
		} `json:"tools"`
	}
	if err := json.Unmarshal(toolsResult, &listed); err != nil {
		return StdioProbeResult{}, core.Malformed
	}
	if len(listed.Tools) > maxManifestListItems {
		return StdioProbeResult{}, core.TooLarge
	}
	names := make([]string, 0, len(listed.Tools))
	descriptors := make([]CachedToolDescriptor, 0, len(listed.Tools))
	canonical := make([]map[string]any, 0, len(listed.Tools))
	seen := map[string]struct{}{}
	for _, tool := range listed.Tools {
		if tool.Name == "" || strings.ContainsAny(tool.Name, "\r\n") {
			return StdioProbeResult{}, core.Malformed
		}
		if _, ok := seen[tool.Name]; ok {
			return StdioProbeResult{}, core.Malformed
		}
		seen[tool.Name] = struct{}{}
		schema := tool.InputSchema
		if len(schema) == 0 {
			schema = json.RawMessage(`{}`)
		}
		if !json.Valid(schema) {
			return StdioProbeResult{}, core.Malformed
		}
		frozen, freezeErr := freezeToolDescriptor(tool.Name, tool.Description, schema, tool.OutputSchema)
		if freezeErr != nil && !errors.Is(freezeErr, core.TooLarge) {
			names = append(names, tool.Name)
			canonical = append(canonical, map[string]any{"name": tool.Name, "inputSchema": json.RawMessage(schema)})
			continue
		}
		if freezeErr != nil {
			return StdioProbeResult{}, freezeErr
		}
		names = append(names, tool.Name)
		descriptors = append(descriptors, frozen)
		canonical = append(canonical, map[string]any{"name": tool.Name, "inputSchema": json.RawMessage(schema)})
	}
	encoded, err := json.Marshal(canonical)
	if err != nil {
		return StdioProbeResult{}, err
	}
	resources, err := parseResourceList(resourcesResult)
	if err != nil {
		return StdioProbeResult{}, err
	}
	return StdioProbeResult{
		ProtocolVersion: init.ProtocolVersion, ServerName: init.ServerInfo.Name, ServerVersion: init.ServerInfo.Version,
		ToolNames: names, ToolDescriptors: descriptors, ResourceSummaries: resources, DescriptorDigest: contentDigest(encoded), ToolsCalled: 0, NetworkRestricted: false,
	}, nil
}

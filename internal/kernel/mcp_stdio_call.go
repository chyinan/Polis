// pattern: Imperative Shell
package kernel

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"polis/internal/core"
)

const (
	mcpCallTimeout    = 30 * time.Second
	mcpCallMaxResult  = maxToolCallResult
	mcpCallMaxStderr  = maxToolCallStderr
)

type StdioCallResult struct {
	ProtocolVersion string          `json:"protocolVersion"`
	ServerName      string          `json:"serverName"`
	ServerVersion   string          `json:"serverVersion"`
	Result          json.RawMessage `json:"result"`
	IsError         bool            `json:"isError"`
	ToolsCalled     int             `json:"toolsCalled"`
	StderrSummary   string          `json:"stderrSummary,omitempty"`
}

func CallStdioMCPTool(ctx context.Context, command string, args []string, toolName string, arguments json.RawMessage) (StdioCallResult, error) {
	return callStdioMCPTool(ctx, command, args, toolName, arguments, nil)
}

func callStdioMCPTool(ctx context.Context, command string, args []string, toolName string, arguments json.RawMessage, extraEnv []string) (StdioCallResult, error) {
	if err := validateProbeCommand(command, args); err != nil {
		return StdioCallResult{}, err
	}
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
	ctx, cancel := context.WithTimeout(ctx, mcpCallTimeout)
	defer cancel()
	workDir, err := os.MkdirTemp("", "polis-mcp-call-")
	if err != nil {
		return StdioCallResult{}, err
	}
	defer os.RemoveAll(workDir)
	cmd := exec.CommandContext(ctx, command, args...)
	cmd.Dir = workDir
	cmd.Env = append(probeEnvironment(), extraEnv...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return StdioCallResult{}, err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return StdioCallResult{}, err
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return StdioCallResult{}, err
	}
	if err = cmd.Start(); err != nil {
		return StdioCallResult{}, err
	}
	stderrBuf := &limitedBuffer{max: mcpCallMaxStderr}
	defer func() {
		_ = stdin.Close()
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		_ = cmd.Wait()
	}()
	go func() { _, _ = io.Copy(stderrBuf, io.LimitReader(stderr, int64(mcpCallMaxStderr)+1)) }()
	reader := bufio.NewReader(stdout)
	initResult, err := roundTrip(ctx, stdin, reader, 1, "initialize", map[string]any{
		"protocolVersion": "2024-11-05",
		"capabilities":    map[string]any{},
		"clientInfo":      map[string]any{"name": "polis-tool-call", "version": "0.15.0"},
	})
	if err != nil {
		return StdioCallResult{StderrSummary: stderrBuf.String()}, err
	}
	if err = writeMCP(stdin, rpcRequest{JSONRPC: "2.0", Method: "notifications/initialized"}); err != nil {
		return StdioCallResult{StderrSummary: stderrBuf.String()}, err
	}
	var init struct {
		ProtocolVersion string `json:"protocolVersion"`
		ServerInfo      struct {
			Name    string `json:"name"`
			Version string `json:"version"`
		} `json:"serverInfo"`
	}
	if err = json.Unmarshal(initResult, &init); err != nil {
		return StdioCallResult{StderrSummary: stderrBuf.String()}, core.Malformed
	}
	callResult, err := roundTrip(ctx, stdin, reader, 2, "tools/call", map[string]any{
		"name":      toolName,
		"arguments": json.RawMessage(arguments),
	})
	if err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return StdioCallResult{ProtocolVersion: init.ProtocolVersion, ServerName: init.ServerInfo.Name, ServerVersion: init.ServerInfo.Version, ToolsCalled: 1, StderrSummary: stderrBuf.String()}, context.DeadlineExceeded
		}
		return StdioCallResult{ProtocolVersion: init.ProtocolVersion, ServerName: init.ServerInfo.Name, ServerVersion: init.ServerInfo.Version, ToolsCalled: 1, StderrSummary: stderrBuf.String()}, err
	}
	if len(callResult) > mcpCallMaxResult {
		return StdioCallResult{ProtocolVersion: init.ProtocolVersion, ServerName: init.ServerInfo.Name, ServerVersion: init.ServerInfo.Version, ToolsCalled: 1, StderrSummary: stderrBuf.String()}, core.TooLarge
	}
	var envelope struct {
		IsError bool `json:"isError"`
	}
	_ = json.Unmarshal(callResult, &envelope)
	return StdioCallResult{
		ProtocolVersion: init.ProtocolVersion, ServerName: init.ServerInfo.Name, ServerVersion: init.ServerInfo.Version,
		Result: callResult, IsError: envelope.IsError, ToolsCalled: 1, StderrSummary: stderrBuf.String(),
	}, nil
}

type limitedBuffer struct {
	buf bytes.Buffer
	max int
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	remain := b.max - b.buf.Len()
	if remain <= 0 {
		return len(p), nil
	}
	if len(p) > remain {
		p = p[:remain]
	}
	n, err := b.buf.Write(p)
	if err != nil {
		return n, err
	}
	if n < len(p) {
		return len(p), nil
	}
	return n, nil
}

func (b *limitedBuffer) String() string { return b.buf.String() }

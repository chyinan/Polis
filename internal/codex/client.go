// pattern: Imperative Shell
package codex

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"polis/internal/runner"
	"strings"
	"sync"
	"time"
)

type Message struct {
	ID     json.RawMessage `json:"id,omitempty"`
	Method string          `json:"method,omitempty"`
	Params json.RawMessage `json:"params,omitempty"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  json.RawMessage `json:"error,omitempty"`
}
type Client struct {
	process  *runner.Process
	nextID   int
	pending  []Message
	incoming chan Message
	readErr  chan error
	log      *os.File
	logMu    sync.Mutex
	wg       sync.WaitGroup
	closed   chan struct{}
	model    string
}
type TurnResult struct {
	State    string
	Boundary bool
}

func New(p *runner.Process, evidence string) (*Client, error) {
	return NewWithModel(p, evidence, "gpt-5.6-sol")
}

func NewWithModel(p *runner.Process, evidence, model string) (*Client, error) {
	if model == "" {
		return nil, errors.New("model profile required")
	}
	if e := os.MkdirAll(evidence, 0700); e != nil {
		return nil, e
	}
	f, e := os.OpenFile(filepath.Join(evidence, "protocol.jsonl"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if e != nil {
		return nil, e
	}
	c := &Client{process: p, incoming: make(chan Message, 64), readErr: make(chan error, 2), log: f, closed: make(chan struct{}), model: model}
	c.wg.Add(2)
	go func() {
		defer c.wg.Done()
		scan := bufio.NewScanner(p.Out)
		scan.Buffer(make([]byte, 4096), 1<<20)
		for scan.Scan() {
			var m Message
			if e := json.Unmarshal(scan.Bytes(), &m); e != nil {
				c.readErr <- e
				return
			}
			if e := c.record("receive", json.RawMessage(append([]byte(nil), scan.Bytes()...))); e != nil {
				c.readErr <- e
				return
			}
			select {
			case c.incoming <- m:
			case <-c.closed:
				return
			}
		}
		if e := scan.Err(); e != nil {
			c.readErr <- e
		} else {
			c.readErr <- io.EOF
		}
	}()
	go func() {
		defer c.wg.Done()
		data, e := io.ReadAll(io.LimitReader(p.Err, 32769))
		if len(data) > 32768 {
			p.Stop()
			data = data[:32768]
		}
		if len(data) > 0 {
			_ = c.record("stderr", string(data))
		}
		if e != nil {
			select {
			case c.readErr <- e:
			default:
			}
		}
	}()
	return c, nil
}
func (c *Client) Close() {
	close(c.closed)
	c.process.Stop()
	c.process.Out.Close()
	c.process.Err.Close()
	c.wg.Wait()
	c.log.Close()
}
func (c *Client) record(direction string, v any) error {
	c.logMu.Lock()
	defer c.logMu.Unlock()
	info, e := c.log.Stat()
	if e != nil {
		return e
	}
	if info.Size() > 32<<20 {
		return errors.New("evidence limit exceeded")
	}
	raw, e := json.Marshal(struct {
		Time      time.Time `json:"time"`
		Direction string    `json:"direction"`
		Data      any       `json:"data"`
	}{time.Now().UTC(), direction, v})
	if e != nil {
		return e
	}
	if _, e = c.log.Write(append(raw, '\n')); e != nil {
		return e
	}
	return c.log.Sync()
}
func (c *Client) send(m Message) error {
	if e := c.record("send", m); e != nil {
		return e
	}
	raw, e := json.Marshal(m)
	if e != nil {
		return e
	}
	_, e = c.process.In.Write(append(raw, '\n'))
	return e
}
func (c *Client) receive(ctx context.Context) (Message, error) {
	select {
	case m := <-c.incoming:
		if m.Method == "warning" {
			var p struct {
				Message string `json:"message"`
			}
			if json.Unmarshal(m.Params, &p) == nil && strings.Contains(p.Message, "Code Mode is unavailable") {
				return Message{}, errors.New("native tool capability unavailable: " + p.Message)
			}
		}
		return m, nil
	case e := <-c.readErr:
		return Message{}, fmt.Errorf("outcome_unknown: transport: %w", e)
	case <-ctx.Done():
		return Message{}, fmt.Errorf("outcome_unknown: %w", ctx.Err())
	}
}
func (c *Client) Request(ctx context.Context, method string, params any) (json.RawMessage, error) {
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	c.nextID++
	id := json.RawMessage(fmt.Sprint(c.nextID))
	raw, e := json.Marshal(params)
	if e != nil {
		return nil, e
	}
	if e = c.send(Message{ID: id, Method: method, Params: raw}); e != nil {
		return nil, e
	}
	for {
		m, e := c.receive(ctx)
		if e != nil {
			return nil, e
		}
		if m.Method == "" && bytes.Equal(m.ID, id) {
			if len(m.Error) > 0 {
				return nil, fmt.Errorf("native request rejected: %s", m.Error)
			}
			return m.Result, nil
		}
		c.pending = append(c.pending, m)
		if len(c.pending) > 256 {
			return nil, errors.New("native notification limit exceeded")
		}
	}
}
func (c *Client) Initialize(ctx context.Context) error {
	raw, e := c.Request(ctx, "initialize", map[string]any{"clientInfo": map[string]any{"name": "polis", "version": "0.1.0"}, "capabilities": map[string]any{"experimentalApi": true}})
	if e != nil {
		return e
	}
	var result struct {
		UserAgent string `json:"userAgent"`
	}
	if e = json.Unmarshal(raw, &result); e != nil {
		return e
	}
	if !strings.Contains(result.UserAgent, "0.151.0") {
		return errors.New("Codex version mismatch")
	}
	return c.send(Message{Method: "initialized"})
}
func (c *Client) StartThread(ctx context.Context, effort string) (string, error) {
	params := map[string]any{"model": c.model, "allowProviderModelFallback": false, "approvalPolicy": "never", "sandbox": "read-only", "cwd": "/work", "environments": []any{}, "ephemeral": true, "dynamicTools": Tools(), "config": map[string]any{"model_reasoning_effort": effort}, "developerInstructions": "You are one fixed Polis employee. Use only polis_* dynamic tools for company context and changes. Native thread IDs are not task authority. Do not use shell, apply_patch, web, external MCP, delegation or account tools. Tool receipts, not natural-language completion, determine progress."}
	raw, e := c.Request(ctx, "thread/start", params)
	if e != nil {
		return "", e
	}
	var r struct {
		Thread struct {
			ID string `json:"id"`
		} `json:"thread"`
		Model    string `json:"model"`
		Effort   string `json:"reasoningEffort"`
		Approval string `json:"approvalPolicy"`
		Sandbox  struct {
			Type string `json:"type"`
		} `json:"sandbox"`
	}
	if e = json.Unmarshal(raw, &r); e != nil {
		return "", e
	}
	if r.Thread.ID == "" || r.Model != c.model || r.Effort != effort || r.Approval != "never" || r.Sandbox.Type != "readOnly" {
		return "", errors.New("native profile or permission mismatch")
	}
	return r.Thread.ID, nil
}
func (c *Client) Turn(ctx context.Context, thread, effort, prompt string, handler func(string, string, json.RawMessage) (json.RawMessage, bool)) (TurnResult, error) {
	raw, e := c.Request(ctx, "turn/start", map[string]any{"threadId": thread, "model": c.model, "effort": effort, "input": []any{map[string]any{"type": "text", "text": prompt}}})
	if e != nil {
		return TurnResult{}, e
	}
	var r struct {
		Turn struct {
			ID string `json:"id"`
		} `json:"turn"`
	}
	if e = json.Unmarshal(raw, &r); e != nil {
		return TurnResult{}, e
	}
	if r.Turn.ID == "" {
		return TurnResult{}, errors.New("missing turn ID")
	}
	calls := 0
	for {
		var m Message
		if len(c.pending) > 0 {
			m = c.pending[0]
			c.pending = c.pending[1:]
		} else {
			m, e = c.receive(ctx)
			if e != nil {
				return TurnResult{}, e
			}
		}
		if m.Method == "item/tool/call" {
			calls++
			if calls > 32 {
				return TurnResult{}, errors.New("tool-call limit exceeded")
			}
			var p struct {
				Thread    string          `json:"threadId"`
				Turn      string          `json:"turnId"`
				CallID    string          `json:"callId"`
				Tool      string          `json:"tool"`
				Namespace *string         `json:"namespace"`
				Args      json.RawMessage `json:"arguments"`
			}
			if e = json.Unmarshal(m.Params, &p); e != nil {
				return TurnResult{}, e
			}
			if p.Thread != thread || p.Turn != r.Turn.ID || p.CallID == "" || len(m.ID) == 0 || (p.Namespace != nil && *p.Namespace != "") {
				return TurnResult{}, errors.New("native correlation mismatch")
			}
			if !strings.HasPrefix(p.Tool, "polis_") {
				return TurnResult{}, errors.New("unapproved native tool")
			}
			result, boundary := handler(strings.TrimPrefix(p.Tool, "polis_"), p.CallID, p.Args)
			if e = c.record("tool_result", map[string]any{"call_id": p.CallID, "tool": p.Tool, "result": json.RawMessage(result)}); e != nil {
				return TurnResult{}, e
			}
			if boundary {
				return TurnResult{State: "interrupted_after_checkpoint", Boundary: true}, nil
			}
			var status struct {
				Error string `json:"error"`
			}
			if e = json.Unmarshal(result, &status); e != nil {
				return TurnResult{}, e
			}
			body, e := json.Marshal(map[string]any{"contentItems": []any{map[string]any{"type": "inputText", "text": string(result)}}, "success": status.Error == ""})
			if e != nil {
				return TurnResult{}, e
			}
			if e = c.send(Message{ID: m.ID, Result: body}); e != nil {
				return TurnResult{}, e
			}
		} else if m.Method == "turn/completed" {
			var p struct {
				Thread string `json:"threadId"`
				Turn   struct {
					ID, Status string
					Error      any
				} `json:"turn"`
			}
			if e = json.Unmarshal(m.Params, &p); e != nil {
				return TurnResult{}, e
			}
			if p.Thread != thread || p.Turn.ID != r.Turn.ID {
				return TurnResult{}, errors.New("completed turn mismatch")
			}
			if p.Turn.Status != "completed" {
				return TurnResult{State: p.Turn.Status}, fmt.Errorf("native turn failed: %v", p.Turn.Error)
			}
			return TurnResult{State: "completed"}, nil
		} else if len(m.ID) > 0 && m.Method != "" {
			return TurnResult{}, fmt.Errorf("unapproved server request: %s", m.Method)
		}
	}
}

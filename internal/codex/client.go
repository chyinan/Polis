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
	process               *runner.Process
	nextID                int
	pending               []Message
	incoming              chan Message
	readErr               chan error
	log                   *os.File
	logMu                 sync.Mutex
	wg                    sync.WaitGroup
	closed                chan struct{}
	model                 string
	expectedNativeVersion string
	lifecycleMu           sync.Mutex
	initializeEvidence    InitializeLifecycleEvidence
}
type TurnResult struct {
	State                    string
	Boundary                 bool
	ToolCalls                int `json:"tool_calls"`
	Usage                    TokenUsage
	UsageUpdates             int
	NativeDurationMS         int64
	Lifecycle                []TurnLifecycleEvent
	StopAcknowledged         bool
	TerminationConfirmed     bool
	ReconciliationRequired   bool
	TransportPolicy          TransportPolicy         `json:"transport_policy"`
	TransportPolicySnapshot  TransportPolicySnapshot `json:"transport_policy_snapshot"`
	Phase                    TransportPhase          `json:"phase"`
	ReconnectWindowStartedAt *time.Time              `json:"reconnect_window_started_at,omitempty"`
	ReconnectWindowExpiredAt *time.Time              `json:"reconnect_window_expired_at,omitempty"`
	ReconnectAttemptCount    int                     `json:"reconnect_attempt_count"`
	ReconnectRecovered       bool                    `json:"reconnect_recovered"`
	OutcomeClassification    string                  `json:"outcome_classification,omitempty"`
}

type TokenUsage struct {
	Total struct {
		TotalTokens           int64 `json:"totalTokens"`
		InputTokens           int64 `json:"inputTokens"`
		CachedInputTokens     int64 `json:"cachedInputTokens"`
		CacheWriteInputTokens int64 `json:"cacheWriteInputTokens"`
		OutputTokens          int64 `json:"outputTokens"`
		ReasoningOutputTokens int64 `json:"reasoningOutputTokens"`
	} `json:"total"`
	Last struct {
		TotalTokens           int64 `json:"totalTokens"`
		InputTokens           int64 `json:"inputTokens"`
		CachedInputTokens     int64 `json:"cachedInputTokens"`
		CacheWriteInputTokens int64 `json:"cacheWriteInputTokens"`
		OutputTokens          int64 `json:"outputTokens"`
		ReasoningOutputTokens int64 `json:"reasoningOutputTokens"`
	} `json:"last"`
}

func New(p *runner.Process, evidence string) (*Client, error) {
	return NewWithModel(p, evidence, "gpt-5.6-sol")
}

func NewWithModel(p *runner.Process, evidence, model string) (*Client, error) {
	return NewWithModelAndVersion(p, evidence, model, runner.NativeVersionNumber)
}

func NewWithModelAndVersion(p *runner.Process, evidence, model, expectedNativeVersion string) (*Client, error) {
	if model == "" {
		return nil, errors.New("model profile required")
	}
	return newClient(p, evidence, model, expectedNativeVersion)
}

// NewForModelCatalog creates an app-server client that can list model metadata
// but cannot start a model thread because it has no selected model.
func NewForModelCatalog(p *runner.Process, evidence, expectedNativeVersion string) (*Client, error) {
	return newClient(p, evidence, "", expectedNativeVersion)
}

func newClient(p *runner.Process, evidence, model, expectedNativeVersion string) (*Client, error) {
	if expectedNativeVersion == "" {
		return nil, errors.New("native version required")
	}
	if e := os.MkdirAll(evidence, 0700); e != nil {
		return nil, e
	}
	f, e := os.OpenFile(filepath.Join(evidence, "protocol.jsonl"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if e != nil {
		return nil, e
	}
	c := &Client{process: p, incoming: make(chan Message, 64), readErr: make(chan error, 2), log: f, closed: make(chan struct{}), model: model, expectedNativeVersion: expectedNativeVersion, initializeEvidence: InitializeLifecycleEvidence{Phase: "initialize", ProcessCreated: p != nil, PIDPresent: p != nil && p.PID() > 0, StdoutPipeState: "open", StderrCategory: "unavailable"}}
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
			if e := c.record("receive", redactInboundProtocolMessageForLog(m, scan.Bytes())); e != nil {
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
			c.lifecycleMu.Lock()
			c.initializeEvidence.StdoutPipeState = "read_error"
			c.lifecycleMu.Unlock()
			c.readErr <- e
		} else {
			c.lifecycleMu.Lock()
			c.initializeEvidence.StdoutPipeState = "closed"
			c.initializeEvidence.ChildExitObserved = c.process.HasExited()
			c.lifecycleMu.Unlock()
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
			c.lifecycleMu.Lock()
			c.initializeEvidence.StderrCategory = safeStderrCategory(data)
			c.lifecycleMu.Unlock()
			_ = c.record("stderr", map[string]any{"category": safeStderrCategory(data), "bytes": len(data)})
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

func (c *Client) InitializationEvidence() InitializeLifecycleEvidence {
	c.lifecycleMu.Lock()
	defer c.lifecycleMu.Unlock()
	evidence := c.initializeEvidence
	if c.process != nil {
		evidence.ProcessCreated = true
		evidence.PIDPresent = c.process.PID() > 0
		evidence.ChildExitObserved = evidence.ChildExitObserved || c.process.HasExited()
	}
	return evidence
}

func (c *Client) recordLifecycle(phase string) error {
	if phase == "" {
		return errors.New("initialize lifecycle phase is required")
	}
	if err := c.record("lifecycle", InitializeLifecycleEvent{Phase: phase}); err != nil {
		return err
	}
	c.lifecycleMu.Lock()
	c.initializeEvidence.Lifecycle = append(c.initializeEvidence.Lifecycle, InitializeLifecycleEvent{Phase: phase})
	c.lifecycleMu.Unlock()
	return nil
}

func (c *Client) wrapInitializationError(err error) error {
	evidence := c.InitializationEvidence()
	evidence.Phase = "initialize"
	evidence.SafeMessage = safeInitializationMessage(err, evidence)
	evidence.ReasonCode = classifyInitializationReason(err, evidence)
	evidence.FailureCategory = ClassifyInitializationFailureCategory(err, evidence)
	return &InitializationFailure{InitializeLifecycleEvidence: evidence, Cause: err}
}

func classifyInitializationReason(err error, evidence InitializeLifecycleEvidence) string {
	if evidence.ChildExitObserved && !evidence.InitializeRequestSent {
		return "child_exited_before_initialize"
	}
	if evidence.ChildExitObserved {
		return "child_exited_during_initialize"
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "initialize_timeout"
	}
	if errors.Is(err, io.EOF) || strings.Contains(err.Error(), "transport: EOF") {
		return "pipe_closed_before_initialize"
	}
	if strings.Contains(err.Error(), "native request rejected") {
		return "initialize_response_rejected"
	}
	if strings.Contains(err.Error(), "json") || strings.Contains(err.Error(), "JSON") {
		return "initialize_response_malformed"
	}
	if strings.Contains(err.Error(), "write") || strings.Contains(err.Error(), "closed") {
		return "initialize_request_write_failed"
	}
	if evidence.InitializeRequestSent {
		return "initialize_failed"
	}
	return "initialize_request_not_sent"
}

func safeInitializationMessage(err error, evidence InitializeLifecycleEvidence) string {
	if evidence.ChildExitObserved {
		if evidence.InitializeRequestSent {
			return "child exited during initialize"
		}
		return "child exited before initialize"
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "initialize timeout"
	}
	if errors.Is(err, io.EOF) || strings.Contains(err.Error(), "transport: EOF") {
		return "protocol pipe closed before initialize acknowledgement"
	}
	if strings.Contains(err.Error(), "native request rejected") {
		return "provider rejected initialize request"
	}
	if strings.Contains(err.Error(), "write") || strings.Contains(err.Error(), "closed") {
		return "initialize request write failed"
	}
	return "provider initialize failed"
}

func safeStderrCategory(data []byte) string {
	text := strings.ToLower(string(data))
	switch {
	case strings.Contains(text, "code-mode host"), strings.Contains(text, "helper"):
		return "helper_failure"
	case strings.Contains(text, "access is denied"), strings.Contains(text, "permission"):
		return "access_denied"
	case strings.Contains(text, "not found"), strings.Contains(text, "no such file"):
		return "executable_or_file_missing"
	default:
		return "stderr_present_unclassified"
	}
}

func (c *Client) send(m Message) error {
	initializeWrite := m.Method == "initialize"
	if initializeWrite {
		if err := c.recordLifecycle("initialize_write_started"); err != nil {
			return err
		}
	}
	if e := c.record("send", redactImageInputMessage(m)); e != nil {
		return e
	}
	raw, e := json.Marshal(m)
	if e != nil {
		return e
	}
	_, e = c.process.In.Write(append(raw, '\n'))
	if e == nil && initializeWrite {
		if lifecycleErr := c.recordLifecycle("initialize_write_completed"); lifecycleErr != nil {
			return lifecycleErr
		}
	}
	return e
}
func (c *Client) receive(ctx context.Context) (Message, error) {
	select {
	case m := <-c.incoming:
		if e := warningError(m); e != nil {
			return Message{}, e
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
	if method == "initialize" {
		c.lifecycleMu.Lock()
		c.initializeEvidence.InitializeRequestSent = true
		c.lifecycleMu.Unlock()
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
			if method == "initialize" {
				if err := c.recordLifecycle("initialize_ack_received"); err != nil {
					return nil, err
				}
				c.lifecycleMu.Lock()
				c.initializeEvidence.InitializeAckReceived = true
				c.lifecycleMu.Unlock()
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
	return c.InitializeWithPolicy(ctx, DefaultTransportPolicy())
}

func (c *Client) InitializeWithPolicy(ctx context.Context, policy TransportPolicy) error {
	c.lifecycleMu.Lock()
	c.initializeEvidence.Phase = "initialize"
	c.lifecycleMu.Unlock()
	if err := policy.Validate(); err != nil {
		return c.wrapInitializationError(err)
	}
	initializeCtx, cancel := context.WithTimeout(ctx, policy.InitializeTimeout)
	defer cancel()
	if err := c.recordLifecycle("initialize_prepare_started"); err != nil {
		return c.wrapInitializationError(err)
	}
	payload := map[string]any{"clientInfo": map[string]any{"name": "polis", "version": "0.1.0"}, "capabilities": map[string]any{"experimentalApi": true}}
	if err := c.recordLifecycle("initialize_payload_ready"); err != nil {
		return c.wrapInitializationError(err)
	}
	raw, e := c.Request(initializeCtx, "initialize", payload)
	if e != nil {
		return c.wrapInitializationError(e)
	}
	var result struct {
		UserAgent string `json:"userAgent"`
	}
	if e = json.Unmarshal(raw, &result); e != nil {
		return c.wrapInitializationError(e)
	}
	if !strings.Contains(result.UserAgent, c.expectedNativeVersion) {
		return c.wrapInitializationError(errors.New("Codex version mismatch"))
	}
	if e = c.send(Message{Method: "initialized"}); e != nil {
		return c.wrapInitializationError(e)
	}
	c.lifecycleMu.Lock()
	c.initializeEvidence.InitializedNotificationSent = true
	c.lifecycleMu.Unlock()
	return nil
}
func (c *Client) StartThread(ctx context.Context, effort string) (string, error) {
	return c.StartThreadWithTools(ctx, effort, Tools(), "You are one fixed Polis employee. Use only polis_* dynamic tools for company context and changes. Native thread IDs are not task authority. Do not use shell, apply_patch, web, external MCP, delegation or account tools. Tool receipts, not natural-language completion, determine progress.")
}

func (c *Client) StartReviewerThread(ctx context.Context, effort string) (string, error) {
	return c.StartThreadWithTools(ctx, effort, ReviewerTools(), "You are an independent Polis reviewer. Use only the supplied read-only reviewer tools. Do not use shell, apply_patch, web, external MCP, delegation or account tools. Do not modify or submit the candidate. Give an explicit review verdict through the review tool.")
}

func (c *Client) StartThreadWithTools(ctx context.Context, effort string, tools []any, developerInstructions string) (string, error) {
	if c.model == "" {
		return "", errors.New("model profile required to start a thread")
	}
	params := map[string]any{"model": c.model, "allowProviderModelFallback": false, "approvalPolicy": "never", "sandbox": "read-only", "cwd": "/work", "environments": []any{}, "ephemeral": true, "dynamicTools": tools, "config": map[string]any{"model_reasoning_effort": effort}, "developerInstructions": developerInstructions}
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
func (c *Client) legacyTurn(ctx context.Context, thread, effort, prompt string, handler func(string, string, json.RawMessage) (json.RawMessage, bool)) (TurnResult, error) {
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
	var usage TokenUsage
	usageUpdates := 0
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
			if calls > RuntimeToolCallSafetyCap {
				return TurnResult{}, fmt.Errorf("runtime tool-call safety cap exhausted: used=%d limit=%d", calls-1, RuntimeToolCallSafetyCap)
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
				return TurnResult{State: "interrupted_after_checkpoint", Boundary: true, Usage: usage, UsageUpdates: usageUpdates}, nil
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
		} else if m.Method == "thread/tokenUsage/updated" {
			var params struct {
				TokenUsage TokenUsage `json:"tokenUsage"`
			}
			if e = json.Unmarshal(m.Params, &params); e != nil {
				return TurnResult{}, e
			}
			usage = params.TokenUsage
			usageUpdates++
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
			return TurnResult{State: "completed", Usage: usage, UsageUpdates: usageUpdates}, nil
		} else if len(m.ID) > 0 && m.Method != "" {
			return TurnResult{}, fmt.Errorf("unapproved server request: %s", m.Method)
		}
	}
}

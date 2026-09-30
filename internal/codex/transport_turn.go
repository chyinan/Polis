// pattern: Imperative Shell
package codex

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"strings"
	"time"
)

var errTurnStopRequested = errors.New("turn stop requested")

type replayedToolCall struct {
	Tool      string
	Arguments string
	Result    []byte
	Boundary  bool
}

func (c *Client) Turn(ctx context.Context, thread, effort, prompt string, handler func(string, string, json.RawMessage) (json.RawMessage, bool)) (TurnResult, error) {
	return c.TurnWithOptions(ctx, thread, effort, prompt, TurnOptions{}, handler)
}

func (c *Client) TurnWithOptions(ctx context.Context, thread, effort, prompt string, options TurnOptions, handler func(string, string, json.RawMessage) (json.RawMessage, bool)) (result TurnResult, err error) {
	defer func() {
		result.OutcomeClassification = ClassifyTurnOutcome(result, err)
	}()
	if options.ToolCallLimit < 0 || options.ToolCallLimit > RuntimeToolCallSafetyCap {
		return TurnResult{}, fmt.Errorf("invalid business tool-call limit: %d", options.ToolCallLimit)
	}
	policy := DefaultTransportPolicy()
	if options.Policy != nil {
		policy = *options.Policy
	} else if options.Timeouts != (TurnTimeouts{}) {
		custom := options.Timeouts.normalized()
		policy.StartAcknowledgement = custom.StartAcknowledgement
		policy.FirstOutputDeadline = custom.FirstValidOutput
		policy.StreamingIdle = custom.StreamingIdle
		policy.ReconnectGrace = custom.ReconnectGrace
		policy.TotalTurnDeadline = custom.Total
		policy.StopReconciliation = custom.StopAcknowledgement
	}
	if err := policy.Validate(); err != nil {
		return TurnResult{}, err
	}
	timeouts := policy.TurnTimeouts()
	turnStart := time.Now()
	outerDeadline := options.OuterDeadline
	if outerDeadline.IsZero() {
		outerDeadline, _ = ctx.Deadline()
	}
	totalCtx, cancel := context.WithTimeout(ctx, timeouts.Total)
	defer cancel()
	ackCtx, ackCancel := context.WithTimeout(totalCtx, timeouts.StartAcknowledgement)
	input, inputErr := buildTurnStartInput(prompt, options.Images)
	if inputErr != nil {
		ackCancel()
		return TurnResult{}, inputErr
	}
	raw, e := c.Request(ackCtx, "turn/start", map[string]any{"threadId": thread, "model": c.model, "effort": effort, "input": input})
	ackCancel()
	if e != nil {
		if errors.Is(ackCtx.Err(), context.DeadlineExceeded) {
			return TurnResult{}, fmt.Errorf("start_ack_deadline_exceeded: %w", e)
		}
		return TurnResult{}, e
	}
	var start struct {
		Turn struct {
			ID string `json:"id"`
		} `json:"turn"`
	}
	if e = json.Unmarshal(raw, &start); e != nil {
		return TurnResult{}, e
	}
	if start.Turn.ID == "" {
		return TurnResult{}, errors.New("missing turn ID")
	}
	result = TurnResult{State: string(TurnRunning), TransportPolicy: policy, TransportPolicySnapshot: policy.Snapshot(), Phase: TransportPhaseWaitingForFirstOutput, Lifecycle: []TurnLifecycleEvent{{At: time.Now(), State: TurnRunning, Method: "turn/start", Phase: string(TransportPhasePreTurn)}, {At: time.Now(), State: TurnRunning, Method: "turn/started", Phase: string(TransportPhaseWaitingForFirstOutput)}}}
	firstOutputDeadline := turnStart.Add(timeouts.FirstValidOutput)
	lastValid := time.Now()
	reconnectSince := time.Time{}
	reconnectDeadline := time.Time{}
	reconnectPhase := ReconnectPhase("")
	firstOutput := false
	stopRequested := false
	replays := map[string]replayedToolCall{}
	calls := 0

	for {
		if !stopRequested && options.Stop != nil {
			select {
			case <-options.Stop:
				stopRequested = true
				result.Lifecycle = append(result.Lifecycle, TurnLifecycleEvent{At: time.Now(), State: TurnStopRequested, Method: "control.stop"})
				ack, stopErr := c.requestTurnInterrupt(totalCtx, timeouts.StopAcknowledgement, thread, start.Turn.ID)
				result.StopAcknowledged = ack
				if stopErr != nil {
					result.ReconciliationRequired = true
					result.Lifecycle = append(result.Lifecycle, TurnLifecycleEvent{At: time.Now(), State: TurnNeedsReconciliation, Method: "turn/interrupt", Reason: stopErr.Error()})
					return result, fmt.Errorf("outcome_requiring_reconciliation: stop acknowledgement failed: %w", stopErr)
				}
			default:
			}
		}
		waitCtx, waitCancel := context.WithCancel(totalCtx)
		deadline, _ := nextTurnDeadline(totalCtx, firstOutput, firstOutputDeadline, lastValid, timeouts.StreamingIdle, reconnectDeadline)
		if !deadline.IsZero() {
			waitCtx, waitCancel = context.WithDeadline(totalCtx, deadline)
		}
		stopChannel := options.Stop
		if stopRequested {
			stopChannel = nil
		}
		m, receiveErr := c.nextTurnMessage(waitCtx, stopChannel)
		waitCancel()
		if receiveErr != nil {
			if errors.Is(receiveErr, errTurnStopRequested) && !stopRequested {
				stopRequested = true
				result.Lifecycle = append(result.Lifecycle, TurnLifecycleEvent{At: time.Now(), State: TurnStopRequested, Method: "control.stop"})
				ack, stopErr := c.requestTurnInterrupt(totalCtx, timeouts.StopAcknowledgement, thread, start.Turn.ID)
				result.StopAcknowledged = ack
				if stopErr != nil {
					result.ReconciliationRequired = true
					result.Lifecycle = append(result.Lifecycle, TurnLifecycleEvent{At: time.Now(), State: TurnNeedsReconciliation, Method: "turn/interrupt", Reason: stopErr.Error()})
					return result, fmt.Errorf("outcome_requiring_reconciliation: stop acknowledgement failed: %w", stopErr)
				}
				continue
			}
			if waitCtx.Err() == context.DeadlineExceeded {
				now := time.Now()
				if !reconnectDeadline.IsZero() && !now.Before(reconnectDeadline) {
					expired := now
					result.ReconnectWindowExpiredAt = &expired
					result.Phase = TransportPhaseTerminalReconciliation
					if reconnectPhase == PreFirstOutputReconnecting {
						result.Lifecycle = append(result.Lifecycle, TurnLifecycleEvent{At: now, State: TurnReconnectDeadline, Method: "timeout", Phase: string(reconnectPhase), Reason: "first valid output deadline elapsed while reconnecting"})
						return result, FirstOutputDeadlineError{Elapsed: now.Sub(turnStart)}
					}
					result.Lifecycle = append(result.Lifecycle, TurnLifecycleEvent{At: now, State: TurnReconnectDeadline, Method: "timeout", Phase: string(reconnectPhase), Reason: "post-output reconnect grace elapsed"})
					if stopRequested {
						result.ReconciliationRequired = true
						return result, fmt.Errorf("outcome_requiring_reconciliation: stop requested during reconnect")
					}
					return result, ReconnectDeadlineError{Elapsed: now.Sub(reconnectSince)}
				}
				if totalCtx.Err() != nil {
					result.Lifecycle = append(result.Lifecycle, TurnLifecycleEvent{At: now, State: TurnTotalDeadline, Method: "timeout", Reason: "total turn deadline elapsed"})
					return result, TurnDeadlineError{Elapsed: now.Sub(turnStart)}
				}
				if !firstOutput {
					return result, FirstOutputDeadlineError{Elapsed: now.Sub(turnStart)}
				}
				return result, StreamingIdleDeadlineError{Elapsed: now.Sub(lastValid)}
			}
			return result, receiveErr
		}

		if m.Method == "error" {
			signal, signalErr := classifyProviderError(m)
			if signalErr != nil {
				return result, signalErr
			}
			if signal.ThreadID != thread || signal.TurnID != start.Turn.ID {
				return result, errors.New("native provider error correlation mismatch")
			}
			if signal.Reconnect {
				result.ReconnectAttemptCount++
				if reconnectSince.IsZero() {
					reconnectSince = time.Now()
					started := reconnectSince
					result.ReconnectWindowStartedAt = &started
					if firstOutput {
						reconnectPhase = PostFirstOutputReconnecting
						result.Phase = TransportPhaseReconnectingAfterOutput
					} else {
						reconnectPhase = PreFirstOutputReconnecting
						result.Phase = TransportPhaseWaitingForFirstOutput
					}
					totalDeadline, _ := totalCtx.Deadline()
					reconnectDeadline = effectiveReconnectDeadline(ReconnectDeadlineInput{Now: reconnectSince, Phase: reconnectPhase, ReconnectStarted: reconnectSince, FirstValidOutputDeadline: firstOutputDeadline, TotalTurnDeadline: totalDeadline, OuterDeadline: outerDeadline, PostOutputGrace: timeouts.ReconnectGrace})
					result.Lifecycle = append(result.Lifecycle, TurnLifecycleEvent{At: reconnectSince, State: TurnReconnecting, Method: "error", Phase: string(reconnectPhase), Reason: "structured responseStreamDisconnected with willRetry"})
				}
				continue
			}
			result.Lifecycle = append(result.Lifecycle, TurnLifecycleEvent{At: time.Now(), State: TurnProviderFailure, Method: "error", Reason: signal.Reason})
			return result, ProviderTerminalError{Detail: signal.Reason}
		}

		if !validTurnEvent(m, thread, start.Turn.ID) {
			if len(m.ID) > 0 && m.Method != "" {
				return result, fmt.Errorf("unapproved server request: %s", m.Method)
			}
			continue
		}
		if !reconnectSince.IsZero() {
			result.Lifecycle = append(result.Lifecycle, TurnLifecycleEvent{At: time.Now(), State: TurnRecovered, Method: m.Method, Phase: string(reconnectPhase), Reason: "valid event for the same turn after reconnect"})
			reconnectSince = time.Time{}
			reconnectDeadline = time.Time{}
			result.ReconnectRecovered = true
			result.Phase = TransportPhaseStreaming
		}
		lastValid = time.Now()
		if isValidTurnOutput(m.Method) {
			firstOutput = true
			result.Phase = TransportPhaseStreaming
		}

		if m.Method == "thread/tokenUsage/updated" {
			var params struct {
				TokenUsage TokenUsage `json:"tokenUsage"`
			}
			if e = json.Unmarshal(m.Params, &params); e != nil {
				return result, e
			}
			result.Usage = params.TokenUsage
			result.UsageUpdates++
			continue
		}
		if m.Method == "item/tool/call" {
			calls++
			if options.ToolCallLimit > 0 && calls > options.ToolCallLimit {
				return result, fmt.Errorf("business tool-call limit exhausted: used=%d limit=%d", calls-1, options.ToolCallLimit)
			}
			if calls > RuntimeToolCallSafetyCap {
				return result, fmt.Errorf("runtime tool-call safety cap exhausted: used=%d limit=%d", calls-1, RuntimeToolCallSafetyCap)
			}
			var p struct {
				Thread string          `json:"threadId"`
				Turn   string          `json:"turnId"`
				CallID string          `json:"callId"`
				Tool   string          `json:"tool"`
				Args   json.RawMessage `json:"arguments"`
				ID     json.RawMessage `json:"-"`
			}
			if e = json.Unmarshal(m.Params, &p); e != nil {
				return result, e
			}
			if p.CallID == "" || len(m.ID) == 0 || (p.Thread != thread || p.Turn != start.Turn.ID) {
				return result, errors.New("native correlation mismatch")
			}
			if !strings.HasPrefix(p.Tool, "polis_") {
				return result, errors.New("unapproved native tool")
			}
			key := p.Turn + "\x00" + p.CallID
			cached, replay := replays[key]
			var toolResult []byte
			var boundary bool
			if replay {
				if cached.Tool != p.Tool || cached.Arguments != string(p.Args) {
					return result, errors.New("outcome_requiring_reconciliation: native tool identity conflict")
				}
				if p.Tool == "polis_skills_load" {
					// Capability revocation takes priority over a response cached
					// before a reconnect. The Kernel idempotently rechecks grants.
					toolResult, boundary = handler(strings.TrimPrefix(p.Tool, "polis_"), p.CallID, p.Args)
				} else {
					toolResult, boundary = cached.Result, cached.Boundary
				}
			} else {
				toolResult, boundary = handler(strings.TrimPrefix(p.Tool, "polis_"), p.CallID, p.Args)
				copyResult := append([]byte(nil), toolResult...)
				replays[key] = replayedToolCall{Tool: p.Tool, Arguments: string(p.Args), Result: copyResult, Boundary: boundary}
			}
			if e = c.record("tool_result", map[string]any{"call_id": p.CallID, "tool": p.Tool, "replayed": replay, "result": json.RawMessage(toolResult)}); e != nil {
				return result, e
			}
			if boundary {
				result.State, result.Boundary = "interrupted_after_checkpoint", true
				return result, nil
			}
			var status struct {
				Error string `json:"error"`
			}
			if e = json.Unmarshal(toolResult, &status); e != nil {
				return result, e
			}
			body, e := json.Marshal(map[string]any{"contentItems": []any{map[string]any{"type": "inputText", "text": string(toolResult)}}, "success": status.Error == ""})
			if e != nil {
				return result, e
			}
			if e = c.send(Message{ID: m.ID, Result: body}); e != nil {
				return result, e
			}
			continue
		}
		if m.Method == "turn/completed" {
			result.Phase = TransportPhaseTerminalReconciliation
			var p struct {
				Thread string `json:"threadId"`
				Turn   struct {
					ID     string `json:"id"`
					Status string `json:"status"`
					Error  any    `json:"error"`
				} `json:"turn"`
			}
			if e = json.Unmarshal(m.Params, &p); e != nil {
				return result, e
			}
			if p.Thread != thread || p.Turn.ID != start.Turn.ID {
				return result, errors.New("completed turn mismatch")
			}
			if stopRequested {
				result.State, result.TerminationConfirmed = "stopped", true
				result.Lifecycle = append(result.Lifecycle, TurnLifecycleEvent{At: time.Now(), State: TurnTerminationConfirmed, Method: "turn/completed", Reason: p.Turn.Status})
				return result, nil
			}
			if p.Turn.Status != "completed" {
				return result, ProviderTerminalError{Detail: fmt.Sprintf("turn status %s: %v", p.Turn.Status, p.Turn.Error)}
			}
			result.State = "completed"
			return result, nil
		}
	}
}

func buildTurnStartInput(prompt string, images []TurnImage) ([]any, error) {
	if len(images) > MaxTurnImages {
		return nil, errors.New("turn image count exceeds its bound")
	}
	input := make([]any, 0, len(images)+1)
	input = append(input, map[string]any{"type": "text", "text": prompt})
	usedBytes := int64(0)
	for _, item := range images {
		if (item.MediaType != "image/png" && item.MediaType != "image/jpeg") || len(item.Content) == 0 || len(item.Content) > MaxTurnImageBytes || usedBytes+int64(len(item.Content)) > MaxTurnImageTotalBytes {
			return nil, errors.New("turn image media type or bytes exceed the supported bound")
		}
		config, format, err := image.DecodeConfig(bytes.NewReader(item.Content))
		if err != nil || (item.MediaType == "image/png" && format != "png") || (item.MediaType == "image/jpeg" && format != "jpeg") || config.Width < 1 || config.Height < 1 || config.Width > 20_000 || config.Height > 20_000 || int64(config.Width)*int64(config.Height) > 100_000_000 {
			return nil, errors.New("turn image bytes do not match their validated image representation")
		}
		dataURL := "data:" + item.MediaType + ";base64," + base64.StdEncoding.EncodeToString(item.Content)
		input = append(input, map[string]any{"type": "image", "url": dataURL, "detail": "auto"})
		usedBytes += int64(len(item.Content))
	}
	encoded, err := json.Marshal(input)
	if err != nil || len(encoded) > MaxTurnInputJSONBytes {
		return nil, errors.Join(err, errors.New("turn image input JSON exceeds its bound"))
	}
	return input, nil
}

func nextTurnDeadline(totalCtx context.Context, firstOutput bool, firstOutputDeadline, lastValid time.Time, idle time.Duration, reconnect time.Time) (time.Time, string) {
	var deadline time.Time
	if !firstOutput {
		deadline = firstOutputDeadline
	} else {
		deadline = lastValid.Add(idle)
	}
	if !reconnect.IsZero() && (deadline.IsZero() || reconnect.Before(deadline)) {
		deadline = reconnect
	}
	if total, ok := totalCtx.Deadline(); ok && (deadline.IsZero() || total.Before(deadline)) {
		deadline = total
	}
	return deadline, ""
}

func validTurnEvent(m Message, thread, turn string) bool {
	switch m.Method {
	case "thread/status/changed":
		var params struct {
			ThreadID string `json:"threadId"`
		}
		return json.Unmarshal(m.Params, &params) == nil && params.ThreadID == thread
	case "turn/started", "turn/completed":
		var params struct {
			ThreadID string `json:"threadId"`
			Turn     struct {
				ID string `json:"id"`
			} `json:"turn"`
		}
		return json.Unmarshal(m.Params, &params) == nil && params.ThreadID == thread && params.Turn.ID == turn
	case "thread/tokenUsage/updated", "item/started", "item/completed", "item/tool/call", "item/agentMessage/delta", "item/agentMessage/completed":
		var params struct {
			ThreadID string `json:"threadId"`
			TurnID   string `json:"turnId"`
		}
		return json.Unmarshal(m.Params, &params) == nil && params.ThreadID == thread && params.TurnID == turn
	default:
		return false
	}
}

func isValidTurnOutput(method string) bool {
	return method == "item/tool/call" || method == "item/agentMessage/delta" || method == "item/agentMessage/completed" || method == "thread/tokenUsage/updated"
}

func (c *Client) receiveTurn(ctx context.Context, stop <-chan struct{}) (Message, error) {
	select {
	case <-stop:
		return Message{}, errTurnStopRequested
	default:
	}
	select {
	case m := <-c.incoming:
		if e := warningError(m); e != nil {
			return Message{}, e
		}
		return m, nil
	case e := <-c.readErr:
		return Message{}, fmt.Errorf("outcome_unknown: transport: %w", e)
	case <-stop:
		return Message{}, errTurnStopRequested
	case <-ctx.Done():
		return Message{}, fmt.Errorf("outcome_unknown: %w", ctx.Err())
	}
}

func (c *Client) nextTurnMessage(ctx context.Context, stop <-chan struct{}) (Message, error) {
	select {
	case <-stop:
		return Message{}, errTurnStopRequested
	default:
	}
	if len(c.pending) > 0 {
		m := c.pending[0]
		c.pending = c.pending[1:]
		return m, nil
	}
	return c.receiveTurn(ctx, stop)
}

func (c *Client) requestTurnInterrupt(parent context.Context, timeout time.Duration, thread, turn string) (bool, error) {
	ctx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()
	_, e := c.Request(ctx, "turn/interrupt", map[string]any{"threadId": thread, "turnId": turn})
	if e != nil {
		return false, e
	}
	return true, nil
}

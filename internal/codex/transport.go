// pattern: Functional Core
package codex

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

type TurnLifecycleState string

type TransportPhase string

const (
	TransportPhasePreTurn                 TransportPhase = "PRE_TURN"
	TransportPhaseWaitingForFirstOutput   TransportPhase = "WAITING_FOR_FIRST_OUTPUT"
	TransportPhaseStreaming               TransportPhase = "STREAMING"
	TransportPhaseReconnectingAfterOutput TransportPhase = "RECONNECTING_AFTER_OUTPUT"
	TransportPhaseTerminalReconciliation  TransportPhase = "TERMINAL_RECONCILIATION"
)

const (
	OutcomeProviderStreamNonrecoverable = "PROVIDER_STREAM_NONRECOVERABLE"
	OutcomeProviderTerminal             = "PROVIDER_TERMINAL"
	OutcomeFirstOutputDeadline          = "FIRST_OUTPUT_DEADLINE"
	OutcomeStreamingIdleDeadline        = "STREAMING_IDLE_DEADLINE"
	OutcomeTotalTurnDeadline            = "TOTAL_TURN_DEADLINE"
)

const (
	TurnRunning              TurnLifecycleState = "normal_running"
	TurnReconnecting         TurnLifecycleState = "reconnecting"
	TurnRecovered            TurnLifecycleState = "recovered"
	TurnProviderFailure      TurnLifecycleState = "explicit_terminal_provider_failure"
	TurnReconnectDeadline    TurnLifecycleState = "reconnect_deadline_exceeded"
	TurnTotalDeadline        TurnLifecycleState = "total_turn_deadline_exceeded"
	TurnStopRequested        TurnLifecycleState = "stop_requested"
	TurnTerminationConfirmed TurnLifecycleState = "actual_termination_confirmed"
	TurnNeedsReconciliation  TurnLifecycleState = "outcome_requiring_reconciliation"
)

type TurnLifecycleEvent struct {
	At     time.Time          `json:"at"`
	State  TurnLifecycleState `json:"state"`
	Method string             `json:"method"`
	Phase  string             `json:"phase,omitempty"`
	Reason string             `json:"reason,omitempty"`
}

type ReconnectPhase string

const (
	PreFirstOutputReconnecting  ReconnectPhase = "pre_first_output_reconnecting"
	PostFirstOutputReconnecting ReconnectPhase = "post_first_output_reconnecting"
)

type ReconnectDeadlineInput struct {
	Now                      time.Time
	Phase                    ReconnectPhase
	ReconnectStarted         time.Time
	FirstValidOutputDeadline time.Time
	TotalTurnDeadline        time.Time
	OuterDeadline            time.Time
	PostOutputGrace          time.Duration
}

func effectiveReconnectDeadline(input ReconnectDeadlineInput) time.Time {
	var candidates []time.Time
	switch input.Phase {
	case PreFirstOutputReconnecting:
		candidates = append(candidates, input.FirstValidOutputDeadline)
	case PostFirstOutputReconnecting:
		if input.PostOutputGrace > 0 && !input.ReconnectStarted.IsZero() {
			candidates = append(candidates, input.ReconnectStarted.Add(input.PostOutputGrace))
		}
	}
	candidates = append(candidates, input.TotalTurnDeadline, input.OuterDeadline)
	var deadline time.Time
	for _, candidate := range candidates {
		if candidate.IsZero() || candidate.Before(input.Now) {
			continue
		}
		if deadline.IsZero() || candidate.Before(deadline) {
			deadline = candidate
		}
	}
	return deadline
}

type TurnTimeouts struct {
	StartAcknowledgement time.Duration
	FirstValidOutput     time.Duration
	StreamingIdle        time.Duration
	ReconnectGrace       time.Duration
	Total                time.Duration
	StopAcknowledgement  time.Duration
}

const TransportPolicyRevision = "r03a-transport-policy@1"

type TransportPolicy struct {
	Revision             string        `json:"transport_policy_revision"`
	InitializeTimeout    time.Duration `json:"initialize_timeout"`
	StartAcknowledgement time.Duration `json:"start_acknowledgement_timeout"`
	FirstOutputDeadline  time.Duration `json:"first_output_deadline"`
	ReconnectGrace       time.Duration `json:"reconnect_grace"`
	StreamingIdle        time.Duration `json:"streaming_idle"`
	TotalTurnDeadline    time.Duration `json:"total_turn_deadline"`
	StopReconciliation   time.Duration `json:"stop_reconciliation_timeout"`
}

// TransportPolicySnapshot is the bounded, unit-explicit representation stored
// in execution and run evidence. It avoids making evidence consumers infer
// time.Duration's nanosecond encoding.
type TransportPolicySnapshot struct {
	Revision               string `json:"transport_policy_revision"`
	InitializeTimeoutMS    int64  `json:"initialize_timeout_ms"`
	StartAcknowledgementMS int64  `json:"start_acknowledgement_ms"`
	FirstOutputDeadlineMS  int64  `json:"first_output_deadline_ms"`
	ReconnectGraceMS       int64  `json:"reconnect_grace_ms"`
	StreamingIdleMS        int64  `json:"streaming_idle_ms"`
	TotalTurnDeadlineMS    int64  `json:"total_turn_deadline_ms"`
	ReconciliationMS       int64  `json:"reconciliation_ms"`
}

func DefaultTransportPolicy() TransportPolicy {
	return TransportPolicy{Revision: TransportPolicyRevision, InitializeTimeout: 30 * time.Second, StartAcknowledgement: 30 * time.Second, FirstOutputDeadline: 90 * time.Second, ReconnectGrace: 30 * time.Second, StreamingIdle: 90 * time.Second, TotalTurnDeadline: 10 * time.Minute, StopReconciliation: 5 * time.Second}
}

func (p TransportPolicy) Validate() error {
	if p.Revision == "" || p.InitializeTimeout <= 0 || p.StartAcknowledgement <= 0 || p.FirstOutputDeadline <= 0 || p.ReconnectGrace <= 0 || p.StreamingIdle <= 0 || p.TotalTurnDeadline <= 0 || p.StopReconciliation <= 0 {
		return errors.New("transport policy is incomplete")
	}
	return nil
}

func (p TransportPolicy) Snapshot() TransportPolicySnapshot {
	return TransportPolicySnapshot{
		Revision:               p.Revision,
		InitializeTimeoutMS:    p.InitializeTimeout.Milliseconds(),
		StartAcknowledgementMS: p.StartAcknowledgement.Milliseconds(),
		FirstOutputDeadlineMS:  p.FirstOutputDeadline.Milliseconds(),
		ReconnectGraceMS:       p.ReconnectGrace.Milliseconds(),
		StreamingIdleMS:        p.StreamingIdle.Milliseconds(),
		TotalTurnDeadlineMS:    p.TotalTurnDeadline.Milliseconds(),
		ReconciliationMS:       p.StopReconciliation.Milliseconds(),
	}
}

func ClassifyTurnOutcome(result TurnResult, err error) string {
	if err == nil {
		return "COMPLETED"
	}
	var reconnect ReconnectDeadlineError
	if errors.As(err, &reconnect) && result.ReconnectWindowStartedAt != nil {
		return OutcomeProviderStreamNonrecoverable
	}
	var provider ProviderTerminalError
	if errors.As(err, &provider) {
		return OutcomeProviderTerminal
	}
	var first FirstOutputDeadlineError
	if errors.As(err, &first) {
		return OutcomeFirstOutputDeadline
	}
	var idle StreamingIdleDeadlineError
	if errors.As(err, &idle) {
		return OutcomeStreamingIdleDeadline
	}
	var total TurnDeadlineError
	if errors.As(err, &total) {
		return OutcomeTotalTurnDeadline
	}
	return "OTHER_FAILURE"
}

func (p TransportPolicy) TurnTimeouts() TurnTimeouts {
	return TurnTimeouts{StartAcknowledgement: p.StartAcknowledgement, FirstValidOutput: p.FirstOutputDeadline, StreamingIdle: p.StreamingIdle, ReconnectGrace: p.ReconnectGrace, Total: p.TotalTurnDeadline, StopAcknowledgement: p.StopReconciliation}
}

func DefaultTurnTimeouts() TurnTimeouts {
	return DefaultTransportPolicy().TurnTimeouts()
}

func (t TurnTimeouts) normalized() TurnTimeouts {
	d := DefaultTurnTimeouts()
	if t.StartAcknowledgement <= 0 {
		t.StartAcknowledgement = d.StartAcknowledgement
	}
	if t.FirstValidOutput <= 0 {
		t.FirstValidOutput = d.FirstValidOutput
	}
	if t.StreamingIdle <= 0 {
		t.StreamingIdle = d.StreamingIdle
	}
	if t.ReconnectGrace <= 0 {
		t.ReconnectGrace = d.ReconnectGrace
	}
	if t.Total <= 0 {
		t.Total = d.Total
	}
	if t.StopAcknowledgement <= 0 {
		t.StopAcknowledgement = d.StopAcknowledgement
	}
	return t
}

type TurnOptions struct {
	Timeouts      TurnTimeouts
	Policy        *TransportPolicy
	Stop          <-chan struct{}
	OuterDeadline time.Time
	Images        []TurnImage
	// ToolCallLimit is the explicitly authorized business limit. Zero means
	// this generic transport caller has no business allowance; the absolute
	// runtime safety cap still applies.
	ToolCallLimit int
}

type TurnImage struct {
	MediaType string
	Content   []byte
}

const (
	MaxTurnImages          = 4
	MaxTurnImageBytes      = 4 << 20
	MaxTurnImageTotalBytes = 8 << 20
	MaxTurnInputJSONBytes  = 12 << 20
)

const RuntimeToolCallSafetyCap = 256

type ProviderTerminalError struct{ Detail string }

func (e ProviderTerminalError) Error() string { return "provider_terminal_failure: " + e.Detail }

type ReconnectDeadlineError struct{ Elapsed time.Duration }

func (e ReconnectDeadlineError) Error() string {
	return fmt.Sprintf("reconnect_deadline_exceeded: elapsed=%s", e.Elapsed)
}

type TurnDeadlineError struct{ Elapsed time.Duration }

func (e TurnDeadlineError) Error() string {
	return fmt.Sprintf("total_turn_deadline_exceeded: elapsed=%s", e.Elapsed)
}

type FirstOutputDeadlineError struct{ Elapsed time.Duration }

func (e FirstOutputDeadlineError) Error() string {
	return fmt.Sprintf("first_valid_output_deadline_exceeded: elapsed=%s", e.Elapsed)
}

type StreamingIdleDeadlineError struct{ Elapsed time.Duration }

func (e StreamingIdleDeadlineError) Error() string {
	return fmt.Sprintf("streaming_idle_deadline_exceeded: elapsed=%s", e.Elapsed)
}

type providerSignal struct {
	Reconnect bool
	Terminal  bool
	WillRetry bool
	ThreadID  string
	TurnID    string
	Reason    string
}

func warningError(m Message) error {
	if m.Method != "warning" {
		return nil
	}
	var p struct {
		Code string `json:"code"`
	}
	if e := json.Unmarshal(m.Params, &p); e != nil {
		return e
	}
	if p.Code == "code_mode_unavailable" {
		return errors.New("native tool capability unavailable: structured code_mode_unavailable warning")
	}
	return nil
}

func classifyProviderError(m Message) (providerSignal, error) {
	if m.Method != "error" {
		return providerSignal{}, nil
	}
	var p struct {
		WillRetry bool   `json:"willRetry"`
		ThreadID  string `json:"threadId"`
		TurnID    string `json:"turnId"`
		Error     struct {
			CodexErrorInfo struct {
				ResponseStreamDisconnected json.RawMessage `json:"responseStreamDisconnected"`
			} `json:"codexErrorInfo"`
			AdditionalDetails string `json:"additionalDetails"`
		} `json:"error"`
	}
	if e := json.Unmarshal(m.Params, &p); e != nil {
		return providerSignal{}, e
	}
	if p.ThreadID == "" || p.TurnID == "" {
		return providerSignal{}, errors.New("native provider error missing thread or turn identity")
	}
	hasDisconnect := len(p.Error.CodexErrorInfo.ResponseStreamDisconnected) > 0 && string(p.Error.CodexErrorInfo.ResponseStreamDisconnected) != "null"
	if hasDisconnect && p.WillRetry {
		return providerSignal{Reconnect: true, WillRetry: true, ThreadID: p.ThreadID, TurnID: p.TurnID, Reason: p.Error.AdditionalDetails}, nil
	}
	return providerSignal{Terminal: true, WillRetry: p.WillRetry, ThreadID: p.ThreadID, TurnID: p.TurnID, Reason: p.Error.AdditionalDetails}, nil
}

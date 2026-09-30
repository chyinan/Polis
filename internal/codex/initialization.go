// pattern: Functional Core
package codex

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
)

type InitializeLifecycleEvent struct {
	Phase string `json:"phase"`
}

// InitializeLifecycleEvidence is the safe, phase-scoped evidence emitted by
// the provider protocol client before a model turn exists. It deliberately
// excludes raw stderr, prompts, credentials and unrestricted environment data.
type InitializeLifecycleEvidence struct {
	Phase                       string                     `json:"phase"`
	ReasonCode                  string                     `json:"reason_code"`
	ProcessCreated              bool                       `json:"process_created"`
	PIDPresent                  bool                       `json:"pid_present"`
	ChildExitObserved           bool                       `json:"child_exit_observed"`
	InitializeRequestSent       bool                       `json:"initialize_request_sent"`
	InitializeAckReceived       bool                       `json:"initialize_ack_received"`
	InitializedNotificationSent bool                       `json:"initialized_notification_sent"`
	StdoutPipeState             string                     `json:"stdout_pipe_state"`
	StderrCategory              string                     `json:"stderr_category"`
	SafeMessage                 string                     `json:"safe_message"`
	Lifecycle                   []InitializeLifecycleEvent `json:"lifecycle,omitempty"`
	FailureCategory             string                     `json:"failure_category,omitempty"`
}

func ClassifyInitializationFailureCategory(err error, evidence InitializeLifecycleEvidence) string {
	if err == nil {
		return ""
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return "context_cancelled"
	}
	if evidence.ChildExitObserved {
		return "process_handle_lost"
	}
	text := strings.ToLower(err.Error())
	if errors.Is(err, io.EOF) || strings.Contains(text, "transport: eof") || strings.Contains(text, "pipe closed") {
		return "pipe_closed"
	}
	if strings.Contains(text, "stdin") || strings.Contains(text, "broken pipe") || strings.Contains(text, "file already closed") {
		return "stdin_unavailable"
	}
	if strings.Contains(text, "protocol.jsonl") || strings.Contains(text, "evidence") || strings.Contains(text, "evidence limit") {
		return "persistence_failed_before_initialize"
	}
	if strings.Contains(text, "write") {
		return "initialize_write_failed"
	}
	return "initialize_failed"
}

// InitializationFailure distinguishes provider-session setup from a turn.
// Cause is retained only in memory for local callers and is excluded from
// serialized evidence because it may contain paths or platform text.
type InitializationFailure struct {
	InitializeLifecycleEvidence
	Cause error `json:"-"`
}

func (e *InitializationFailure) Error() string {
	if e == nil {
		return "provider initialization failed"
	}
	return fmt.Sprintf("provider initialization failed: phase=%s reason_code=%s safe_message=%s", e.Phase, e.ReasonCode, e.SafeMessage)
}

func (e *InitializationFailure) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

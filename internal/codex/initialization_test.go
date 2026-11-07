// pattern: Functional Core
package codex

import (
	"context"
	"errors"
	"testing"
)

func TestClassifyInitializationFailurePhases(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		evidence InitializeLifecycleEvidence
		want     string
	}{
		{name: "child exits before request", err: errors.New("transport: EOF"), evidence: InitializeLifecycleEvidence{ChildExitObserved: true}, want: "child_exited_before_initialize"},
		{name: "child exits during request", err: errors.New("transport: EOF"), evidence: InitializeLifecycleEvidence{ChildExitObserved: true, InitializeRequestSent: true}, want: "child_exited_during_initialize"},
		{name: "timeout", err: context.DeadlineExceeded, evidence: InitializeLifecycleEvidence{InitializeRequestSent: true}, want: "initialize_timeout"},
		{name: "pipe closes", err: errors.New("outcome_unknown: transport: EOF"), evidence: InitializeLifecycleEvidence{InitializeRequestSent: true}, want: "pipe_closed_before_initialize"},
		{name: "rejected response", err: errors.New("native request rejected: malformed"), evidence: InitializeLifecycleEvidence{InitializeRequestSent: true}, want: "initialize_response_rejected"},
		{name: "malformed response", err: errors.New("invalid json response"), evidence: InitializeLifecycleEvidence{InitializeRequestSent: true}, want: "initialize_response_malformed"},
		{name: "request write", err: errors.New("write: file already closed"), evidence: InitializeLifecycleEvidence{}, want: "initialize_request_write_failed"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := classifyInitializationReason(test.err, test.evidence); got != test.want {
				t.Fatalf("classifyInitializationReason() = %q, want %q", got, test.want)
			}
		})
	}
}

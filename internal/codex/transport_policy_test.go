// pattern: Functional Core
package codex

import (
	"errors"
	"testing"
	"time"
)

func TestDefaultTransportPolicyIsExplicitAndBounded(t *testing.T) {
	policy := DefaultTransportPolicy()
	if err := policy.Validate(); err != nil {
		t.Fatal(err)
	}
	if policy.Revision != TransportPolicyRevision || policy.ReconnectGrace != 30*time.Second || policy.FirstOutputDeadline != 90*time.Second || policy.StreamingIdle != 90*time.Second || policy.TotalTurnDeadline != 10*time.Minute || policy.StopReconciliation != 5*time.Second {
		t.Fatalf("unexpected effective transport policy: %+v", policy)
	}
	snapshot := policy.Snapshot()
	if snapshot.ReconnectGraceMS != 30000 || snapshot.FirstOutputDeadlineMS != 90000 || snapshot.TotalTurnDeadlineMS != 600000 || snapshot.ReconciliationMS != 5000 {
		t.Fatalf("unexpected evidence snapshot: %+v", snapshot)
	}
}

func TestTransportPolicyDriftChangesTurnPolicyBinding(t *testing.T) {
	base := DefaultTransportPolicy()
	drifted := base
	drifted.ReconnectGrace = 31 * time.Second
	if base.Snapshot() == drifted.Snapshot() {
		t.Fatal("reconnect policy drift was not observable in the snapshot")
	}
}

func TestPostOutputReconnectDeadlineIsProviderStreamNonrecoverable(t *testing.T) {
	started := time.Unix(10, 0)
	expired := started.Add(30 * time.Second)
	result := TurnResult{
		ReconnectWindowStartedAt: &started,
		ReconnectWindowExpiredAt: &expired,
		Phase:                    TransportPhaseTerminalReconciliation,
	}
	err := ReconnectDeadlineError{Elapsed: 30 * time.Second}
	if got := ClassifyTurnOutcome(result, err); got != OutcomeProviderStreamNonrecoverable {
		t.Fatalf("outcome=%q", got)
	}
	if got := ClassifyTurnOutcome(TurnResult{}, errors.New("local failure")); got != "OTHER_FAILURE" {
		t.Fatalf("generic outcome=%q", got)
	}
}

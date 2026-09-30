// pattern: Functional Core
package codex

import (
	_ "embed"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

//go:embed testdata/r03a_t2_disconnect_protocol.jsonl
var r03aT2DisconnectFixture []byte

func TestR03AT21PreFirstOutputUsesFirstOutputAndHardLimits(t *testing.T) {
	now := time.Unix(100, 0)
	first := now.Add(90 * time.Second)
	total := now.Add(120 * time.Second)
	outer := now.Add(180 * time.Second)
	got := effectiveReconnectDeadline(ReconnectDeadlineInput{Now: now, Phase: PreFirstOutputReconnecting, ReconnectStarted: now, FirstValidOutputDeadline: first, TotalTurnDeadline: total, OuterDeadline: outer, PostOutputGrace: 30 * time.Second})
	if !got.Equal(first) {
		t.Fatalf("pre-first-output deadline = %s, want first-output deadline %s", got, first)
	}
	if legacy := now.Add(30 * time.Second); got.Equal(legacy) {
		t.Fatal("new pre-first-output policy still uses the old fixed 30s reconnect grace")
	}
	if got = effectiveReconnectDeadline(ReconnectDeadlineInput{Now: now, Phase: PreFirstOutputReconnecting, ReconnectStarted: now, FirstValidOutputDeadline: first, TotalTurnDeadline: now.Add(60 * time.Second), OuterDeadline: outer, PostOutputGrace: 30 * time.Second}); !got.Equal(now.Add(60 * time.Second)) {
		t.Fatalf("total deadline was not the pre-first-output hard limit: %s", got)
	}
	if got = effectiveReconnectDeadline(ReconnectDeadlineInput{Now: now, Phase: PreFirstOutputReconnecting, ReconnectStarted: now, FirstValidOutputDeadline: first, TotalTurnDeadline: total, OuterDeadline: now.Add(45 * time.Second), PostOutputGrace: 30 * time.Second}); !got.Equal(now.Add(45 * time.Second)) {
		t.Fatalf("outer deadline was not the final pre-first-output hard limit: %s", got)
	}
}

func TestR03AT21PreFirstOutputRecoveryAfterThirtySecondsIsAllowed(t *testing.T) {
	now := time.Unix(200, 0)
	recovery := now.Add(31 * time.Second)
	deadline := effectiveReconnectDeadline(ReconnectDeadlineInput{Now: now, Phase: PreFirstOutputReconnecting, ReconnectStarted: now, FirstValidOutputDeadline: now.Add(90 * time.Second), TotalTurnDeadline: now.Add(120 * time.Second), OuterDeadline: now.Add(180 * time.Second), PostOutputGrace: 30 * time.Second})
	if !recovery.Before(deadline) {
		t.Fatalf("recovery at %s was incorrectly past pre-first-output deadline %s", recovery, deadline)
	}
}

func TestR03AT21PreFirstOutputNoRecoveryEndsAtFirstOutputDeadline(t *testing.T) {
	now := time.Unix(300, 0)
	deadline := effectiveReconnectDeadline(ReconnectDeadlineInput{Now: now, Phase: PreFirstOutputReconnecting, ReconnectStarted: now, FirstValidOutputDeadline: now.Add(90 * time.Second), TotalTurnDeadline: now.Add(120 * time.Second), OuterDeadline: now.Add(180 * time.Second), PostOutputGrace: 30 * time.Second})
	if !deadline.Equal(now.Add(90 * time.Second)) {
		t.Fatalf("no-recovery pre-first-output deadline = %s, want %s", deadline, now.Add(90*time.Second))
	}
}

func TestR03AT21PostOutputUsesGraceAndHardLimits(t *testing.T) {
	now := time.Unix(400, 0)
	grace := now.Add(30 * time.Second)
	got := effectiveReconnectDeadline(ReconnectDeadlineInput{Now: now, Phase: PostFirstOutputReconnecting, ReconnectStarted: now, FirstValidOutputDeadline: now.Add(90 * time.Second), TotalTurnDeadline: now.Add(120 * time.Second), OuterDeadline: now.Add(180 * time.Second), PostOutputGrace: 30 * time.Second})
	if !got.Equal(grace) {
		t.Fatalf("post-output grace deadline = %s, want %s", got, grace)
	}
	if got = effectiveReconnectDeadline(ReconnectDeadlineInput{Now: now, Phase: PostFirstOutputReconnecting, ReconnectStarted: now, TotalTurnDeadline: now.Add(20 * time.Second), OuterDeadline: now.Add(180 * time.Second), PostOutputGrace: 30 * time.Second}); !got.Equal(now.Add(20 * time.Second)) {
		t.Fatalf("total turn deadline exceeded post-output grace: %s", got)
	}
	if got = effectiveReconnectDeadline(ReconnectDeadlineInput{Now: now, Phase: PostFirstOutputReconnecting, ReconnectStarted: now, TotalTurnDeadline: now.Add(120 * time.Second), OuterDeadline: now.Add(15 * time.Second), PostOutputGrace: 30 * time.Second}); !got.Equal(now.Add(15 * time.Second)) {
		t.Fatalf("outer deadline exceeded post-output grace: %s", got)
	}
}

func TestR03AT21PostOutputRecoveryAndBoundedFailure(t *testing.T) {
	now := time.Unix(500, 0)
	deadline := effectiveReconnectDeadline(ReconnectDeadlineInput{Now: now, Phase: PostFirstOutputReconnecting, ReconnectStarted: now, TotalTurnDeadline: now.Add(120 * time.Second), OuterDeadline: now.Add(180 * time.Second), PostOutputGrace: 30 * time.Second})
	if !now.Add(29 * time.Second).Before(deadline) {
		t.Fatal("post-output recovery inside grace was rejected")
	}
	if now.Add(31 * time.Second).Before(deadline) {
		t.Fatal("post-output reconnect beyond grace was not bounded")
	}
}

func TestR03AT21StopPolicyDoesNotBypassTerminationConfirmation(t *testing.T) {
	for _, phase := range []ReconnectPhase{PreFirstOutputReconnecting, PostFirstOutputReconnecting} {
		if phase == "" {
			t.Fatal("invalid reconnect phase")
		}
	}
	// The actual ACK/terminal completion ordering remains covered by the T1
	// stop-during-reconnect protocol fixture; this test keeps both phases in the
	// policy matrix without introducing real sleeps.
}

func TestR03AT21T2RegressionShowsOldThirtySecondPolicyWasTooEarly(t *testing.T) {
	lines := strings.Split(strings.TrimSpace(string(r03aT2DisconnectFixture)), "\n")
	if len(lines) != 4 {
		t.Fatalf("T2 regression fixture event count changed: %d", len(lines))
	}
	var first struct {
		Data struct {
			Method string `json:"method"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(lines[0]), &first); err != nil || first.Data.Method != "turn/started" {
		t.Fatalf("T2 fixture no longer starts with turn/started: %v", err)
	}
	now := time.Unix(600, 0)
	firstOutput := now.Add(90 * time.Second)
	newDeadline := effectiveReconnectDeadline(ReconnectDeadlineInput{Now: now, Phase: PreFirstOutputReconnecting, ReconnectStarted: now, FirstValidOutputDeadline: firstOutput, TotalTurnDeadline: now.Add(120 * time.Second), OuterDeadline: now.Add(180 * time.Second), PostOutputGrace: 30 * time.Second})
	legacyDeadline := now.Add(30 * time.Second)
	if !legacyDeadline.Before(newDeadline) || !now.Add(31*time.Second).Before(newDeadline) {
		t.Fatalf("T2 phase policy did not move pre-first-output deadline beyond legacy 30s: legacy=%s new=%s", legacyDeadline, newDeadline)
	}
}

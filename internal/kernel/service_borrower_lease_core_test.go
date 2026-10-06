// pattern: Functional Core
package kernel

import (
	"errors"
	"testing"
	"time"

	"polis/internal/core"
)

func TestValidateServiceBorrowerLeaseScopeRequiresSameMissionDistinctTaskAndExactSessions(t *testing.T) {
	if err := validateServiceBorrowerLeaseScope("company", "mission", "mission", "owner-task", "borrower-task", "owner-session", "borrower-session", 1); err != nil {
		t.Fatalf("valid borrower lease scope rejected: %v", err)
	}
	for name, input := range map[string]struct {
		company, ownerMission, borrowerMission, ownerTask, borrowerTask, ownerSession, borrowerSession string
		generation                                                                                     int
	}{
		"cross mission":            {company: "company", ownerMission: "mission", borrowerMission: "other-mission", ownerTask: "owner-task", borrowerTask: "borrower-task", ownerSession: "owner-session", borrowerSession: "borrower-session", generation: 1},
		"same task":                {company: "company", ownerMission: "mission", borrowerMission: "mission", ownerTask: "owner-task", borrowerTask: "owner-task", ownerSession: "owner-session", borrowerSession: "borrower-session", generation: 1},
		"missing borrower session": {company: "company", ownerMission: "mission", borrowerMission: "mission", ownerTask: "owner-task", borrowerTask: "borrower-task", ownerSession: "owner-session", borrowerSession: "", generation: 1},
		"invalid generation":       {company: "company", ownerMission: "mission", borrowerMission: "mission", ownerTask: "owner-task", borrowerTask: "borrower-task", ownerSession: "owner-session", borrowerSession: "borrower-session", generation: 0},
	} {
		t.Run(name, func(t *testing.T) {
			if err := validateServiceBorrowerLeaseScope(input.company, input.ownerMission, input.borrowerMission, input.ownerTask, input.borrowerTask, input.ownerSession, input.borrowerSession, input.generation); !errors.Is(err, core.OutOfScope) {
				t.Fatalf("scope error=%v, want %v", err, core.OutOfScope)
			}
		})
	}
}

func TestServiceBorrowerLeaseExpiryUsesEndpointAndPolicyBounds(t *testing.T) {
	now := time.Date(2026, 10, 6, 12, 0, 0, 0, time.UTC)
	endpointExpiry := now.Add(30 * time.Minute)
	expires, idle := serviceBorrowerLeaseTimes(now, endpointExpiry)
	if !expires.Equal(now.Add(ServiceBorrowerLeaseMaxTTL)) || !idle.Equal(now.Add(ServiceBorrowerLeaseIdleGrace)) {
		t.Fatalf("lease times=(%s,%s), want 15m/2m", expires, idle)
	}
	expires, _ = serviceBorrowerLeaseTimes(now, now.Add(5*time.Minute))
	if !expires.Equal(now.Add(5 * time.Minute)) {
		t.Fatalf("lease expiry=%s, want endpoint bound %s", expires, now.Add(5*time.Minute))
	}
	expires, idle = serviceBorrowerLeaseTimes(now, now.Add(30*time.Second))
	if !expires.Equal(now.Add(30*time.Second)) || !idle.Equal(expires) {
		t.Fatalf("short endpoint lease times=(%s,%s), want both endpoint bound", expires, idle)
	}
}

func TestServiceBorrowerLeaseExpiresAtTTLOrIdleBoundary(t *testing.T) {
	now := time.Date(2026, 10, 6, 12, 0, 0, 0, time.UTC)
	if serviceBorrowerLeaseExpired(now, now.Add(time.Minute), now.Add(time.Minute)) {
		t.Fatal("lease expired before either boundary")
	}
	if !serviceBorrowerLeaseExpired(now.Add(time.Minute), now.Add(time.Minute), now.Add(2*time.Minute)) {
		t.Fatal("lease did not expire at its TTL boundary")
	}
	if !serviceBorrowerLeaseExpired(now.Add(2*time.Minute), now.Add(5*time.Minute), now.Add(2*time.Minute)) {
		t.Fatal("lease did not expire at its idle boundary")
	}
}

// pattern: Functional Core
package kernel

import (
	"testing"
	"time"
)

func TestProductDeliveryFeedbackExpiryIsDerivedWithoutImplicitAcceptance(t *testing.T) {
	deadline := time.Date(2026, 10, 13, 0, 0, 0, 0, time.UTC)
	if productDeliveryFeedbackExpired("awaiting_feedback", &deadline, deadline.Add(-time.Nanosecond)) {
		t.Fatal("feedback was expired before its deadline")
	}
	if !productDeliveryFeedbackExpired("awaiting_feedback", &deadline, deadline) {
		t.Fatal("feedback was not expired at its deadline")
	}
	for _, state := range []string{"not_requested", "accepted", "changes_requested"} {
		if productDeliveryFeedbackExpired(state, &deadline, deadline.Add(time.Hour)) {
			t.Fatalf("state %q was treated as an expired awaiting-feedback window", state)
		}
	}
	if productDeliveryFeedbackExpired("awaiting_feedback", nil, deadline.Add(time.Hour)) {
		t.Fatal("awaiting feedback without a deadline was treated as expired")
	}
}

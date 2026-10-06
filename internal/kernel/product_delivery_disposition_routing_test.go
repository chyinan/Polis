// pattern: Functional Core
package kernel

import "testing"

func TestProductDeliveryChangeRoutingUsesMissionLifecycle(t *testing.T) {
	tests := []struct {
		missionState string
		wantRoute    string
	}{
		{missionState: "active", wantRoute: "mission_change_request"},
		{missionState: "paused", wantRoute: "mission_change_request"},
		{missionState: "succeeded", wantRoute: "company_backlog"},
		{missionState: "ended_not_met", wantRoute: "company_backlog"},
		{missionState: "cancelled", wantRoute: "company_backlog"},
	}
	for _, test := range tests {
		if got := productDeliveryChangeRoute(test.missionState); got != test.wantRoute {
			t.Errorf("productDeliveryChangeRoute(%q) = %q, want %q", test.missionState, got, test.wantRoute)
		}
	}
}

func TestProductDeliveryChangeRoutingRejectsUnknownMissionState(t *testing.T) {
	if got := productDeliveryChangeRoute("draft"); got != "" {
		t.Fatalf("productDeliveryChangeRoute(draft) = %q, want empty", got)
	}
}

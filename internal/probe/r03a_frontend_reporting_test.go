// pattern: Functional Core
package probe

import "testing"

func TestFrontendSessionRollupRetainsStartedSessionsWithoutArtifact(t *testing.T) {
	turns := []R03ATurn{
		{Number: 1, Employee: "emp-frontend", SessionID: "initial-session", Status: "passed", TurnStarted: true, ProviderEgress: true, ToolEvents: []string{"workspace_check"}},
		{Number: 2, Employee: "emp-frontend", SessionID: "successor-session", Status: "passed", TurnStarted: true, ProviderEgress: true, ToolEvents: []string{"workspace_check", "work_checkpoint"}},
	}
	rollup := rollupFrontendSessions(turns)
	if rollup.MediumStarted != 2 || rollup.ProviderEgress != 2 || !rollup.SuccessorStarted || rollup.ArtifactState != "NOT_CREATED" || rollup.BusinessResult != "FAILED" {
		t.Fatalf("session rollup lost no-artifact failure state: %+v", rollup)
	}
	if len(rollup.Sessions) != 2 || rollup.Sessions[0].SessionID != "initial-session" || rollup.Sessions[1].SessionID != "successor-session" {
		t.Fatalf("session records were not retained: %+v", rollup.Sessions)
	}
}

func TestFrontendSessionRollupDoesNotCallInitialBoundaryBusinessFailure(t *testing.T) {
	rollup := rollupFrontendSessions([]R03ATurn{{
		Number: 1, Employee: "emp-frontend", SessionID: "initial-session", Status: "passed",
		TurnStarted: true, ProviderEgress: true, TurnCompleted: true, StopConfirmed: true,
	}})
	if rollup.BusinessResult != "IN_PROGRESS" || rollup.TransportState != "TERMINALLY_RECONCILED" {
		t.Fatalf("initial handover boundary was misclassified: %+v", rollup)
	}
}

func TestFrontendSessionRollupMarksUnreconciledSessionTransport(t *testing.T) {
	rollup := rollupFrontendSessions([]R03ATurn{{
		Number: 2, Employee: "emp-frontend", SessionID: "successor-session", Status: "failed",
		TurnStarted: true, ProviderEgress: true,
	}})
	if rollup.TransportState != "UNRESOLVED" || rollup.BusinessResult != "FAILED" {
		t.Fatalf("unreconciled successor was misclassified: %+v", rollup)
	}
}

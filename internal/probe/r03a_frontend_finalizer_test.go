// pattern: Functional Core
package probe

import "testing"

func TestFinalizeCompletedFrontendFailureAllowsMissingArtifact(t *testing.T) {
	result := map[string]any{"status": "NOT_STARTED", "provider_egress": 1}
	finalizeCompletedFrontendFailure(result, R03ATurn{ToolCallsUsed: 31, ToolCallsRemaining: 17}, assertError("artifact lookup returned no rows"))
	if result["status"] != "FAILED" || result["frontend_provider_turn"] != "COMPLETED" || result["frontend_real_execution"] != "FAILED" || result["frontend_artifact"] != "NOT_CREATED" || result["qualified_checkpoint"] != "NOT_CONFIRMED" || result["obligation_fulfilled"] != "NO" {
		t.Fatalf("failed terminal result was not finalized: %+v", result)
	}
	if result["tool_calls_used"] != 31 || result["tool_calls_remaining"] != 17 {
		t.Fatalf("session accounting was not preserved: %+v", result)
	}
}

func TestRefreshFrontendTerminalStatusDoesNotBackslideStartedFailure(t *testing.T) {
	result := map[string]any{
		"status":                           "NOT_STARTED",
		"allowance_created":                true,
		"provider_egress":                  1,
		"frontend_real_execution":          "FAILED",
		"frontend_session":                 R03ATurn{SessionID: "started-session", ProviderEgress: true},
		"transport_outcome_classification": "OTHER_FAILURE",
	}
	refreshFrontendTerminalStatus(result)
	if result["status"] != FrontendTerminalFailed {
		t.Fatalf("started failed execution backslid to %v", result["status"])
	}
}

type testError string

func (e testError) Error() string { return string(e) }

func assertError(message string) error { return testError(message) }

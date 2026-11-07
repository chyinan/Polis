// pattern: Functional Core
package probe

import "testing"

func TestClassifyT20PassRequiresOutputCompletionAndResolvedTransport(t *testing.T) {
	result := ClassifyT20Outcome(T20OutcomeInput{
		TurnStarted:         true,
		FirstValidOutput:    true,
		TurnCompleted:       true,
		UnresolvedTransport: false,
	})
	if result.Result != T20Passed || result.L1Qualification != T20L1Qualified {
		t.Fatalf("expected T20 PASS, got %+v", result)
	}
}

func TestClassifyT20PreOutputReconnectAsInconclusive(t *testing.T) {
	result := ClassifyT20Outcome(T20OutcomeInput{
		TurnStarted:         true,
		ReconnectCount:      2,
		UnresolvedTransport: true,
		FailureMode:         T20NoOutputReconnect,
	})
	if result.Result != T20Inconclusive || result.L1Qualification != T20L1Unqualified || result.FailureMode != T20NoOutputReconnect {
		t.Fatalf("expected structured reconnect inconclusive result, got %+v", result)
	}
}

func TestClassifyT20PreservesDifferentFailureMode(t *testing.T) {
	result := ClassifyT20Outcome(T20OutcomeInput{
		TurnStarted:         true,
		FailureMode:         T20StructuredProviderError,
		UnresolvedTransport: true,
	})
	if result.Result != T20Inconclusive || result.FailureMode != T20StructuredProviderError {
		t.Fatalf("different failure was recast: %+v", result)
	}
}

func TestCompareT20ControlledFactorsRejectsAnyDrift(t *testing.T) {
	expected := t20TestFactors()
	actual := expected
	actual.AuthCredentialRevisionFingerprint = "changed"
	comparison := CompareT20ControlledFactors(expected, actual)
	if comparison.Passed || len(comparison.Changed) != 1 || comparison.Changed[0].Field != "auth_credential_revision_fingerprint" {
		t.Fatalf("auth drift was not rejected: %+v", comparison)
	}
}

func TestCompareT20ControlledFactorsAcceptsExactT19Profile(t *testing.T) {
	comparison := CompareT20ControlledFactors(t20TestFactors(), t20TestFactors())
	if !comparison.Passed || len(comparison.Changed) != 0 {
		t.Fatalf("exact T19 profile was not accepted: %+v", comparison)
	}
}

func t20TestFactors() T20ControlledFactors {
	return T20ControlledFactors{
		CodexVersion: "0.153.4", BinarySHA256: "binary", CodeModeHostSHA256: "host", Model: "gpt-5.6-luna", Effort: "medium",
		EffectiveConfigDigest: "config", EffectiveTransportConfigDigest: "transport", AuthSourceClass: "controlled_diagnostic_auth_material",
		AuthIdentityFingerprint: "identity", AuthCredentialRevisionFingerprint: "revision", ProxyPolicy: "no_injected_proxy",
		ProviderTransportPolicy: "native_default", DynamicToolCount: "0", SandboxProfile: "read-only", RuntimeProfile: "windows-native",
		CodexHomeProfile: "windows_native_diagnostic", CWDRole: "diagnostic_workspace", PromptDigest: "prompt",
		DeveloperInstructionDigest: "developer", NativeProtocolSchemaDigest: "protocol", CapabilityDigest: "capability",
	}
}

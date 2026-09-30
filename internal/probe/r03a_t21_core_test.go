// pattern: Functional Core
package probe

import "testing"

func TestClassifyT21ExactSurfaceCompletionAsQualified(t *testing.T) {
	result := ClassifyT21Outcome(T21OutcomeInput{
		ExactToolSurface:    true,
		TurnStarted:         true,
		FirstValidOutput:    true,
		TurnCompleted:       true,
		SentinelExact:       true,
		UnresolvedTransport: false,
		AttemptedToolCalls:  0,
		BusinessSideEffects: 0,
	})
	if result.Result != T21Passed || result.L2Qualification != T21L2Qualified || result.ToolRegistrationTransport != T21PassedLower || result.SentinelInstructionFollowing != T21PassedLower || result.BusinessSideEffectIsolation != T21PassedLower {
		t.Fatalf("expected fully qualified T21 result, got %+v", result)
	}
}

func TestClassifyT21DeniedUnexpectedToolCallSeparately(t *testing.T) {
	result := ClassifyT21Outcome(T21OutcomeInput{
		ExactToolSurface:    true,
		TurnStarted:         true,
		FirstValidOutput:    true,
		TurnCompleted:       true,
		SentinelExact:       true,
		AttemptedToolCalls:  1,
		BusinessSideEffects: 0,
	})
	if result.Result != T21Passed || result.ToolRegistrationTransport != T21PassedLower || result.SentinelInstructionFollowing != T21FailedLower || result.BusinessSideEffectIsolation != T21PassedLower {
		t.Fatalf("unexpected separate tool-call classification: %+v", result)
	}
}

func TestClassifyT21KeepsTransportPassSeparateFromSentinelMismatch(t *testing.T) {
	result := ClassifyT21Outcome(T21OutcomeInput{
		ExactToolSurface:    true,
		TurnStarted:         true,
		FirstValidOutput:    true,
		TurnCompleted:       true,
		SentinelExact:       false,
		BusinessSideEffects: 0,
	})
	if result.Result != T21Passed || result.ToolRegistrationTransport != T21PassedLower || result.SentinelInstructionFollowing != T21FailedLower {
		t.Fatalf("sentinel mismatch incorrectly changed transport result: %+v", result)
	}
}

func TestClassifyT21RejectsUnresolvedTransport(t *testing.T) {
	result := ClassifyT21Outcome(T21OutcomeInput{
		ExactToolSurface:    true,
		TurnStarted:         true,
		UnresolvedTransport: true,
		FailureMode:         T21NoOutputReconnect,
	})
	if result.Result != T21Inconclusive || result.L2Qualification != T21L2Unqualified || result.FailureMode != T21NoOutputReconnect {
		t.Fatalf("expected T21 inconclusive result, got %+v", result)
	}
}

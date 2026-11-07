// pattern: Functional Core
package probe

const (
	T21Passed                    = "PASSED"
	T21Inconclusive              = "INCONCLUSIVE"
	T21L2Qualified               = "QUALIFIED"
	T21L2Unqualified             = "UNQUALIFIED"
	T21PassedLower               = "passed"
	T21FailedLower               = "failed"
	T21NoOutputReconnect         = "no_output_reconnect"
	T21StructuredProviderError   = "structured_provider_error"
	T21StdioFailure              = "stdio_failure"
	T21ToolRegistrationTransport = "tool_registration_transport"
)

type T21OutcomeInput struct {
	ExactToolSurface    bool
	TurnStarted         bool
	FirstValidOutput    bool
	TurnCompleted       bool
	SentinelExact       bool
	UnresolvedTransport bool
	AttemptedToolCalls  int
	BusinessSideEffects int
	FailureMode         string
}

type T21Outcome struct {
	Result                       string `json:"result"`
	L2Qualification              string `json:"l2_qualification"`
	ToolRegistrationTransport    string `json:"tool_registration_transport"`
	SentinelInstructionFollowing string `json:"sentinel_instruction_following"`
	BusinessSideEffectIsolation  string `json:"business_side_effect_isolation"`
	FailureMode                  string `json:"failure_mode"`
}

func ClassifyT21Outcome(input T21OutcomeInput) T21Outcome {
	basePass := input.ExactToolSurface && input.TurnStarted && input.FirstValidOutput && input.TurnCompleted && !input.UnresolvedTransport
	if basePass && input.BusinessSideEffects == 0 {
		sentinel := T21PassedLower
		if input.AttemptedToolCalls > 0 || !input.SentinelExact {
			sentinel = T21FailedLower
		}
		return T21Outcome{
			Result:                       T21Passed,
			L2Qualification:              T21L2Qualified,
			ToolRegistrationTransport:    T21PassedLower,
			SentinelInstructionFollowing: sentinel,
			BusinessSideEffectIsolation:  T21PassedLower,
			FailureMode:                  "none",
		}
	}
	failureMode := input.FailureMode
	if failureMode == "" {
		failureMode = T21StdioFailure
	}
	return T21Outcome{
		Result:                       T21Inconclusive,
		L2Qualification:              T21L2Unqualified,
		ToolRegistrationTransport:    ifString(input.ExactToolSurface, T21PassedLower, T21FailedLower),
		SentinelInstructionFollowing: T21FailedLower,
		BusinessSideEffectIsolation:  ifString(input.BusinessSideEffects == 0, T21PassedLower, T21FailedLower),
		FailureMode:                  failureMode,
	}
}

func ifString(condition bool, whenTrue, whenFalse string) string {
	if condition {
		return whenTrue
	}
	return whenFalse
}

// pattern: Functional Core
package probe

import "strings"

const (
	FrontendTerminalNotStarted   = "NOT_STARTED"
	FrontendTerminalPassed       = "PASSED"
	FrontendTerminalFailed       = "FAILED"
	FrontendTerminalInconclusive = "INCONCLUSIVE"
)

// FrontendTerminalExecution is the execution evidence needed to derive the
// terminal status. Artifact/checkpoint presence is deliberately not included:
// an absent artifact is a valid outcome of a real failed business execution.
type FrontendTerminalExecution struct {
	AllowanceConsumed        bool
	SessionCreated           bool
	ProviderEgress           bool
	BusinessExecutionEntered bool
	BusinessVerdict          string
	TransportOutcome         string
}

// DeriveFrontendTerminalStatus is the single status transition boundary for
// Frontend business results. NOT_STARTED is reserved for runs that never
// entered business execution. Transport uncertainty takes precedence over a
// partial business verdict; otherwise the persisted business verdict wins.
func DeriveFrontendTerminalStatus(input FrontendTerminalExecution) string {
	entered := input.AllowanceConsumed || input.SessionCreated || input.ProviderEgress || input.BusinessExecutionEntered
	if !entered {
		return FrontendTerminalNotStarted
	}
	if frontendTransportIsInconclusive(input.TransportOutcome) {
		return FrontendTerminalInconclusive
	}
	switch strings.ToUpper(strings.TrimSpace(input.BusinessVerdict)) {
	case FrontendTerminalPassed:
		return FrontendTerminalPassed
	case FrontendTerminalFailed:
		return FrontendTerminalFailed
	case FrontendTerminalInconclusive:
		return FrontendTerminalInconclusive
	default:
		return FrontendTerminalInconclusive
	}
}

func frontendTransportIsInconclusive(outcome string) bool {
	switch strings.ToUpper(strings.TrimSpace(outcome)) {
	case "PROVIDER_STREAM_NONRECOVERABLE",
		"PROVIDER_TERMINAL",
		"FIRST_OUTPUT_DEADLINE",
		"STREAMING_IDLE_DEADLINE",
		"TOTAL_TURN_DEADLINE":
		return true
	default:
		return false
	}
}

// pattern: Functional Core
package probe

type FrontendSessionRollup struct {
	Sessions           []R03ATurn
	MediumStarted      int
	ProviderEgress     int
	SuccessorStarted   bool
	ToolCallsUsed      int
	ToolCallsRemaining int
	ArtifactState      string
	BusinessResult     string
	TransportState     string
}

func rollupFrontendSessions(turns []R03ATurn) FrontendSessionRollup {
	rollup := FrontendSessionRollup{Sessions: append([]R03ATurn(nil), turns...), ArtifactState: "NOT_CREATED", BusinessResult: "NOT_RUN", TransportState: "NOT_STARTED"}
	allTerminal := len(turns) > 0
	for _, turn := range turns {
		if turn.TurnStarted || turn.ProviderEgress {
			rollup.MediumStarted++
		}
		if turn.ProviderEgress {
			rollup.ProviderEgress++
		}
		if turn.Number == 2 && (turn.TurnStarted || turn.ProviderEgress) {
			rollup.SuccessorStarted = true
		}
		if !turn.TurnCompleted || !turn.StopConfirmed {
			allTerminal = false
		}
		if turn.ToolCallsUsed > 0 {
			rollup.ToolCallsUsed += turn.ToolCallsUsed
		} else {
			rollup.ToolCallsUsed += len(turn.ToolEvents)
		}
		rollup.ToolCallsRemaining += int(turn.ToolCallsRemaining)
		if turn.ArtifactID != "" {
			rollup.ArtifactState = "CREATED"
			rollup.BusinessResult = "PASSED"
		}
	}
	if rollup.ArtifactState != "CREATED" {
		if rollup.SuccessorStarted {
			rollup.BusinessResult = "FAILED"
		} else if len(turns) > 0 {
			rollup.BusinessResult = "IN_PROGRESS"
		}
	}
	if allTerminal {
		rollup.TransportState = "TERMINALLY_RECONCILED"
	} else if len(turns) > 0 {
		rollup.TransportState = "UNRESOLVED"
	}
	return rollup
}

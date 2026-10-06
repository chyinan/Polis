// pattern: Functional Core
package spec

import "fmt"

type FixedTeamTaskRevisionDecision string

const (
	TaskRevisionDecisionApproved FixedTeamTaskRevisionDecision = "approved"
	TaskRevisionDecisionRejected FixedTeamTaskRevisionDecision = "rejected"
	TaskRevisionDecisionRevoked  FixedTeamTaskRevisionDecision = "revoked"

	TaskRevisionStateProposed FixedTeamTaskRevisionDecision = "proposed"
	TaskRevisionStateApproved FixedTeamTaskRevisionDecision = "approved"
	TaskRevisionStateRejected FixedTeamTaskRevisionDecision = "rejected"
	TaskRevisionStateRevoked  FixedTeamTaskRevisionDecision = "revoked"
)

// TaskRevisionDecisionTransition is the pure owner-decision lifecycle for a
// semantic TaskRevision. Qualification is intentionally not a transition
// here: an approved row remains unverified until a separately authorized
// qualification process supplies evidence.
func TaskRevisionDecisionTransition(current string, decision FixedTeamTaskRevisionDecision) (string, error) {
	if current == "" {
		current = string(TaskRevisionStateProposed)
	}
	switch current {
	case string(TaskRevisionStateProposed):
		if decision == TaskRevisionDecisionApproved {
			return string(TaskRevisionStateApproved), nil
		}
		if decision == TaskRevisionDecisionRejected {
			return string(TaskRevisionStateRejected), nil
		}
	case string(TaskRevisionStateApproved):
		if decision == TaskRevisionDecisionRevoked {
			return string(TaskRevisionStateRevoked), nil
		}
	}
	return "", fmt.Errorf("task revision decision %q is invalid from state %q", decision, current)
}

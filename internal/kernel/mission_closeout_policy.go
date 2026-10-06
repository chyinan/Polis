// pattern: Functional Core
package kernel

import "polis/internal/core"

func validateMissionSuccessDeliveryAcceptance(readyPassed, acceptedDisposition bool) error {
	if !readyPassed {
		return core.ConflictError{Reason: "Mission success evidence must reference exact ready Artifacts with an independent passed review", CurrentState: "acceptance_evidence_required"}
	}
	if !acceptedDisposition {
		return core.ConflictError{Reason: "Mission success requires an explicit accepted delivery disposition", CurrentState: "delivery_acceptance_required"}
	}
	return nil
}

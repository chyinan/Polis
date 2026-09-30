// pattern: Imperative Shell
package control

import (
	"context"
	"crypto/sha256"
	"encoding/hex"

	"polis/internal/core"
	"polis/internal/kernel"
)

func (s *Service) SetHumanInterventionState(ctx context.Context, companyID string, request SetHumanInterventionStateRequest) (CommandReceipt, error) {
	if !core.ValidID(companyID) || !core.ValidID(request.InterventionID) || (request.State != "acknowledged" && request.State != "resolved") {
		return CommandReceipt{}, core.Malformed
	}
	if err := validateRequestID(request.RequestID); err != nil {
		return CommandReceipt{}, err
	}
	intervention, err := s.runtime.TXSetHumanInterventionState(ctx, s.runtime.LocalScope(companyID), request.InterventionID, request.State, request.RequestID)
	if err != nil {
		return CommandReceipt{}, err
	}
	return commandReceiptForTarget("human_intervention."+request.State, request.RequestID, "human_intervention", intervention.ID, intervention.State), nil
}

func recordProviderReadinessIntervention(ctx context.Context, runtime *kernel.Kernel, companyID, missionID, requestID string) (kernel.HumanIntervention, error) {
	problemHash := sha256.Sum256([]byte(companyID + "\x00" + missionID + "\x00provider-readiness"))
	requestHash := sha256.Sum256([]byte("provider-readiness\x00" + requestID))
	return runtime.TXCreateHumanIntervention(ctx, runtime.LocalScope(companyID), kernel.HumanInterventionInput{
		MissionID: missionID, ProblemKey: "prov-" + hex.EncodeToString(problemHash[:8]), Severity: "high",
		ReasonCode: "handover_required", AffectedScope: "mission", ProtectionAction: "no_mutation",
		RequiredAction: "reauthorize_in_workbench",
	}, "hi-"+hex.EncodeToString(requestHash[:12]))
}

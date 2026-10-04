// pattern: Imperative Shell
package control

import (
	"context"
	"errors"

	"polis/internal/core"
)

func (s *Service) CloseMission(ctx context.Context, companyID string, request MissionCloseoutCommandRequest) (CommandReceipt, error) {
	if !core.ValidID(companyID) || !core.ValidID(request.MissionID) || validateRequestID(request.RequestID) != nil ||
		(request.Outcome != "succeeded" && request.Outcome != "ended_not_met") {
		return CommandReceipt{}, core.Malformed
	}
	if s.worker == nil {
		return CommandReceipt{}, errors.New("deterministic worker adapter is unavailable")
	}
	scope := s.runtime.LocalScope(companyID)
	s.projectLifecycleMu.Lock()
	defer s.projectLifecycleMu.Unlock()

	intent, err := s.runtime.TXBeginMissionCloseout(ctx, scope, request.MissionID, request.Outcome, request.Rationale, request.AcceptanceArtifactIDs, request.RequestID)
	if err != nil {
		return CommandReceipt{}, err
	}
	details, err := s.runtime.MissionDetails(ctx, scope, request.MissionID)
	if err != nil {
		return CommandReceipt{}, err
	}
	if details.State == request.Outcome {
		return commandReceipt("mission.closeout", request.RequestID, intent.ID, details.State), nil
	}
	if details.State != "closing" {
		return CommandReceipt{}, core.ConflictError{Reason: "Mission closeout intent does not match the durable lifecycle state", CurrentState: details.State}
	}
	if err = s.stopMissionProjectJobs(ctx, companyID, request.MissionID, request.RequestID, "closeout"); err != nil {
		return CommandReceipt{}, err
	}
	if err = s.worker.Stop(ctx, companyID, request.MissionID); err != nil {
		return CommandReceipt{}, core.ConflictError{Reason: "worker stop is not confirmed; Mission remains closing and new work admission is blocked", CurrentState: "reconcile_required"}
	}
	outstandingWork, err := s.runtime.MissionHasOutstandingJobWork(ctx, companyID, request.MissionID)
	if err != nil {
		return CommandReceipt{}, err
	}
	if outstandingWork {
		return CommandReceipt{}, core.ConflictError{Reason: "Mission remains closing until execution reaches a confirmed stop", CurrentState: "reconcile_required"}
	}
	finalized, err := s.runtime.TXFinalizeMissionCloseout(ctx, scope, request.MissionID)
	if err != nil {
		return CommandReceipt{}, err
	}
	return commandReceipt("mission.closeout", request.RequestID, finalized.ID, finalized.Status), nil
}

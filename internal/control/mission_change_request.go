// pattern: Imperative Shell
package control

import (
	"context"
	"strings"

	"polis/internal/core"
	"polis/internal/kernel"
	"polis/internal/taskvalidation"
)

func (s *Service) CreateMissionChangeRequest(ctx context.Context, companyID, missionID string, request CreateMissionChangeRequestRequest) (kernel.MissionChangeRequest, error) {
	if !core.ValidID(companyID) || !core.ValidID(missionID) || !core.ValidID(request.RequestID) ||
		strings.TrimSpace(request.ChangeSummary) == "" || taskvalidation.ValidateContract(request.ProposedAcceptanceContract) != nil {
		return kernel.MissionChangeRequest{}, core.Malformed
	}
	return s.runtime.TXCreateMissionChangeRequest(ctx, s.runtime.LocalScope(companyID), missionID, kernel.MissionChangeRequestInput{
		ChangeSummary: request.ChangeSummary, ProposedTitle: request.ProposedTitle, ProposedGoal: request.ProposedGoal,
		ProposedAcceptanceContract: request.ProposedAcceptanceContract, BlockPreviousResults: request.BlockPreviousResults,
	}, request.RequestID)
}

func (s *Service) ListMissionChangeRequests(ctx context.Context, companyID, missionID string) ([]kernel.MissionChangeRequest, error) {
	if !core.ValidID(companyID) || !core.ValidID(missionID) {
		return nil, core.Malformed
	}
	return s.runtime.MissionChangeRequests(ctx, s.runtime.LocalScope(companyID), missionID)
}

func (s *Service) ConsiderMissionChangeRequest(ctx context.Context, companyID, missionID, changeRequestID string, request MissionChangeRequestCommand) (kernel.MissionChangeRequest, error) {
	if !validMissionChangeRequestCommand(companyID, missionID, changeRequestID, request.RequestID) || !validSHA256Digest(request.AssessmentSHA256) {
		return kernel.MissionChangeRequest{}, core.Malformed
	}
	return s.runtime.TXConsiderMissionChangeRequestWithAssessment(ctx, s.runtime.LocalScope(companyID), missionID, changeRequestID, request.AssessmentSHA256, request.RequestID)
}

func (s *Service) DeclineMissionChangeRequest(ctx context.Context, companyID, missionID, changeRequestID string, request MissionChangeRequestCommand) (kernel.MissionChangeRequest, error) {
	if !validMissionChangeRequestCommand(companyID, missionID, changeRequestID, request.RequestID) {
		return kernel.MissionChangeRequest{}, core.Malformed
	}
	return s.runtime.TXDeclineMissionChangeRequest(ctx, s.runtime.LocalScope(companyID), missionID, changeRequestID, request.RequestID)
}

func (s *Service) ApplyMissionChangeRequest(ctx context.Context, companyID, missionID, changeRequestID string, request MissionChangeRequestCommand) (kernel.MissionChangeRequest, error) {
	if !validMissionChangeRequestCommand(companyID, missionID, changeRequestID, request.RequestID) || len(request.ImpactSHA256) != 64 {
		return kernel.MissionChangeRequest{}, core.Malformed
	}
	return s.runtime.TXApplyMissionChangeRequest(ctx, s.runtime.LocalScope(companyID), missionID, changeRequestID, request.ImpactSHA256, request.RequestID)
}

func validMissionChangeRequestCommand(companyID, missionID, changeRequestID, requestID string) bool {
	return core.ValidID(companyID) && core.ValidID(missionID) && core.ValidID(changeRequestID) && core.ValidID(requestID)
}

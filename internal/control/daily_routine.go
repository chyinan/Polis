// pattern: Imperative Shell
package control

import (
	"context"

	"polis/internal/core"
)

type CreateDailyRoutineRequest struct {
	RoutineID       string `json:"routineId"`
	EmployeeID      string `json:"employeeId"`
	TaskInstruction string `json:"taskInstruction"`
	Timezone        string `json:"timezone"`
	LocalTime       string `json:"localTime"`
	NextLogicalDay  string `json:"nextLogicalDay"`
	CatchUpPolicy   string `json:"catchUpPolicy"`
	MaxCatchUp      int    `json:"maxCatchUp"`
	RequestID       string `json:"requestId"`
}

type SetDailyRoutineTaskInstructionRequest struct {
	TaskInstruction string `json:"taskInstruction"`
	RequestID       string `json:"requestId"`
}

type DailyRoutineCommandService interface {
	CreateDailyRoutine(ctx context.Context, companyID, missionID string, request CreateDailyRoutineRequest) (CommandReceipt, error)
	SetDailyRoutineTaskInstruction(ctx context.Context, companyID, missionID, routineID string, request SetDailyRoutineTaskInstructionRequest) (CommandReceipt, error)
}

func validateCreateDailyRoutineRequest(companyID, missionID string, request CreateDailyRoutineRequest) error {
	if !core.ValidID(companyID) || !core.ValidID(missionID) || !core.ValidID(request.RoutineID) || !core.ValidID(request.EmployeeID) {
		return core.Malformed
	}
	if err := validateRequestID(request.RequestID); err != nil {
		return err
	}
	if err := core.ValidateDailyRoutineSchedule(core.DailyRoutineSchedule{
		RoutineID: request.RoutineID, Timezone: request.Timezone, LocalTime: request.LocalTime,
		NextLogicalDay: request.NextLogicalDay, CatchUpPolicy: core.RoutineCatchUpPolicy(request.CatchUpPolicy), MaxCatchUp: request.MaxCatchUp,
	}); err != nil {
		return err
	}
	return core.ValidateDailyRoutineTaskInstruction(request.TaskInstruction)
}

func (s *Service) CreateDailyRoutine(ctx context.Context, companyID, missionID string, request CreateDailyRoutineRequest) (CommandReceipt, error) {
	if err := validateCreateDailyRoutineRequest(companyID, missionID, request); err != nil {
		return CommandReceipt{}, err
	}
	schedule := core.DailyRoutineSchedule{
		RoutineID: request.RoutineID, TaskInstruction: request.TaskInstruction, Timezone: request.Timezone,
		LocalTime: request.LocalTime, NextLogicalDay: request.NextLogicalDay,
		CatchUpPolicy: core.RoutineCatchUpPolicy(request.CatchUpPolicy), MaxCatchUp: request.MaxCatchUp,
	}
	receipt, err := s.runtime.TXCreateDailyRoutine(ctx, s.runtime.LocalScope(companyID), missionID, request.EmployeeID, schedule, request.RequestID)
	if err != nil {
		return CommandReceipt{}, err
	}
	return commandReceiptForTarget("routine.daily.create", request.RequestID, "routine", receipt.ID, receipt.Status), nil
}

func (s *Service) SetDailyRoutineTaskInstruction(ctx context.Context, companyID, missionID, routineID string, request SetDailyRoutineTaskInstructionRequest) (CommandReceipt, error) {
	if !core.ValidID(companyID) || !core.ValidID(missionID) || !core.ValidID(routineID) {
		return CommandReceipt{}, core.Malformed
	}
	if err := validateRequestID(request.RequestID); err != nil {
		return CommandReceipt{}, err
	}
	if err := core.ValidateDailyRoutineTaskInstruction(request.TaskInstruction); err != nil {
		return CommandReceipt{}, err
	}
	receipt, err := s.runtime.TXSetDailyRoutineTaskInstruction(ctx, s.runtime.LocalScope(companyID), missionID, routineID, request.TaskInstruction, request.RequestID)
	if err != nil {
		return CommandReceipt{}, err
	}
	return commandReceiptForTarget("routine.daily.instruction", request.RequestID, "routine", routineID, receipt.Status), nil
}

var _ DailyRoutineCommandService = (*Service)(nil)

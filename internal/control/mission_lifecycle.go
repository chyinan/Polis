// pattern: Imperative Shell
package control

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"polis/internal/core"
)

func (s *Service) PauseMission(ctx context.Context, companyID string, request MissionCommandRequest) (CommandReceipt, error) {
	if !core.ValidID(companyID) || validateMissionCommandRequest(request) != nil {
		return CommandReceipt{}, core.Malformed
	}
	if s.worker == nil {
		return CommandReceipt{}, fmt.Errorf("worker adapter is unavailable")
	}
	scope := s.runtime.LocalScope(companyID)
	prior, found, err := s.runtime.LookupMissionPauseCommand(ctx, scope, request.MissionID, true, request.RequestID)
	if err != nil {
		return CommandReceipt{}, err
	}
	if found {
		return commandReceipt("mission.pause", request.RequestID, prior.ID, prior.Status), nil
	}
	s.projectLifecycleMu.Lock()
	defer s.projectLifecycleMu.Unlock()
	prior, found, err = s.runtime.LookupMissionPauseCommand(ctx, scope, request.MissionID, true, request.RequestID)
	if err != nil {
		return CommandReceipt{}, err
	}
	if found {
		return commandReceipt("mission.pause", request.RequestID, prior.ID, prior.Status), nil
	}
	if err = s.stopMissionProjectJobs(ctx, companyID, request.MissionID, request.RequestID, "pause"); err != nil {
		return CommandReceipt{}, err
	}
	if err = s.worker.Stop(ctx, companyID, request.MissionID); err != nil {
		return CommandReceipt{}, core.ConflictError{Reason: "worker stop is not confirmed; Mission state was not changed", CurrentState: "active"}
	}
	outstandingWork, err := s.runtime.MissionHasOutstandingJobWork(ctx, companyID, request.MissionID)
	if err != nil {
		return CommandReceipt{}, err
	}
	if outstandingWork {
		return CommandReceipt{}, core.ConflictError{Reason: "Mission execution has not reached a confirmed stop; Mission remains active", CurrentState: "reconcile_required"}
	}
	receipt, err := s.runtime.TXSetMissionPausedCommand(ctx, scope, request.MissionID, true, request.RequestID)
	if err != nil {
		return CommandReceipt{}, err
	}
	return commandReceipt("mission.pause", request.RequestID, receipt.ID, receipt.Status), nil
}

func (s *Service) stopMissionProjectJobs(ctx context.Context, companyID, missionID, requestID, action string) error {
	jobs, err := s.runtime.ListMissionJobRuns(ctx, companyID, missionID)
	if err != nil {
		return err
	}
	for _, job := range jobs {
		terminal := job.State == "exited" || job.State == "failed" || job.State == "cancelled"
		if terminal && job.Kind != "service" {
			continue
		}
		stopped, stopErr := s.StopProjectJob(ctx, companyID, job.JobID, StopProjectJobRequest{RequestID: projectJobLifecycleStopRequestID(requestID, job.JobID)})
		if stopErr != nil || (stopped.State != "exited" && stopped.State != "failed" && stopped.State != "cancelled") {
			return core.ConflictError{Reason: "project JobRun stop is not confirmed; Mission " + action + " is blocked", CurrentState: stopped.State}
		}
	}
	projectJobWorkOutstanding, err := s.runtime.MissionHasOutstandingProjectJobWork(ctx, companyID, missionID)
	if err != nil {
		return err
	}
	if projectJobWorkOutstanding {
		return core.ConflictError{Reason: "project JobRun or service endpoint remains active; Mission " + action + " is blocked", CurrentState: "project_job_reconcile_required"}
	}
	return nil
}

func projectJobLifecycleStopRequestID(lifecycleRequestID, jobID string) string {
	digest := sha256.Sum256([]byte(lifecycleRequestID + "\x00" + jobID))
	return "mission-job-stop-" + hex.EncodeToString(digest[:12])
}

func (s *Service) ResumeMission(ctx context.Context, companyID string, request MissionCommandRequest) (CommandReceipt, error) {
	if !core.ValidID(companyID) || validateMissionCommandRequest(request) != nil {
		return CommandReceipt{}, core.Malformed
	}
	if s.worker == nil {
		return CommandReceipt{}, fmt.Errorf("worker adapter is unavailable")
	}
	scope := s.runtime.LocalScope(companyID)
	prior, found, err := s.runtime.LookupMissionPauseCommand(ctx, scope, request.MissionID, false, request.RequestID)
	if err != nil {
		return CommandReceipt{}, err
	}
	if found {
		return commandReceipt("mission.resume", request.RequestID, prior.ID, prior.Status), nil
	}
	s.projectLifecycleMu.Lock()
	defer s.projectLifecycleMu.Unlock()
	prior, found, err = s.runtime.LookupMissionPauseCommand(ctx, scope, request.MissionID, false, request.RequestID)
	if err != nil {
		return CommandReceipt{}, err
	}
	if found {
		return commandReceipt("mission.resume", request.RequestID, prior.ID, prior.Status), nil
	}
	pendingProjectHandover, err := s.runtime.HasPendingCrossBackendHandover(ctx, companyID, request.MissionID)
	if err != nil {
		return CommandReceipt{}, err
	}
	if !pendingProjectHandover {
		err = s.worker.Readiness(ctx)
	}
	if !pendingProjectHandover && err != nil {
		intervention, interventionErr := recordProviderReadinessIntervention(ctx, s.runtime, companyID, request.MissionID, request.RequestID)
		if interventionErr != nil {
			return CommandReceipt{}, fmt.Errorf("provider runtime is unavailable and the required human intervention could not be persisted: %w", interventionErr)
		}
		if s.qqSender != nil {
			_, _ = s.DispatchHumanIntervention(ctx, companyID, intervention.ID)
		}
		return CommandReceipt{}, core.ConflictError{Reason: "provider runtime is unavailable; Mission remains paused", CurrentState: "provider_unavailable"}
	}
	receipt, err := s.runtime.TXSetMissionPausedCommand(ctx, scope, request.MissionID, false, request.RequestID)
	if err != nil {
		return CommandReceipt{}, err
	}
	if pendingProjectHandover {
		return commandReceipt("mission.resume", request.RequestID, receipt.ID, receipt.Status), nil
	}
	if err = s.worker.Start(ctx, companyID, request.MissionID); err != nil {
		if stopErr := s.worker.Stop(ctx, companyID, request.MissionID); stopErr != nil {
			return CommandReceipt{}, core.ConflictError{Reason: "Mission resume outcome is unknown; worker cleanup was not confirmed", CurrentState: "reconcile_required"}
		}
		rollbackKey := missionResumeRollbackKey(request.RequestID)
		if _, rollbackErr := s.runtime.TXSetMissionPausedCommand(ctx, scope, request.MissionID, true, rollbackKey); rollbackErr != nil {
			return CommandReceipt{}, fmt.Errorf("Mission resume failed and safe pause could not be confirmed: %w", rollbackErr)
		}
		intervention, interventionErr := recordProviderReadinessIntervention(ctx, s.runtime, companyID, request.MissionID, request.RequestID)
		if interventionErr != nil {
			return CommandReceipt{}, fmt.Errorf("Mission resume failed and human intervention could not be persisted: %w", interventionErr)
		}
		if s.qqSender != nil {
			_, _ = s.DispatchHumanIntervention(ctx, companyID, intervention.ID)
		}
		return CommandReceipt{}, core.ConflictError{Reason: "provider worker did not resume; Mission was returned to paused", CurrentState: "paused"}
	}
	return commandReceipt("mission.resume", request.RequestID, receipt.ID, receipt.Status), nil
}

func missionResumeRollbackKey(requestID string) string {
	digest := sha256.Sum256([]byte(requestID))
	return "resume-rollback-" + hex.EncodeToString(digest[:12])
}

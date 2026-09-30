// pattern: Imperative Shell
package control

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"runtime"
	"time"

	"polis/internal/core"
	"polis/internal/environment"
	"polis/internal/kernel"
)

const maxEnvironmentPreparationArtifactBytes = 65536

type EnvironmentPreparationResult struct {
	TerminalState string
	ReasonCode    string
	Evidence      []byte
	Logs          []byte
	LogsTruncated bool
}

type preparedEnvironmentRunRef struct {
	companyID string
	runID     string
}

type ProjectEnvironmentPreparationExecutor interface {
	PrepareProjectEnvironment(context.Context, string, ProjectEnvironmentSnapshotLoader) (EnvironmentPreparationResult, error)
}

type ProjectEnvironmentSnapshotLoader func(context.Context) (kernel.ProjectEnvironmentExecutionSnapshot, error)

func (s *Service) SetProjectEnvironmentPreparationExecutor(executor ProjectEnvironmentPreparationExecutor) {
	s.preparationMu.Lock()
	defer s.preparationMu.Unlock()
	if executor != nil && s.preparationCtx == nil {
		s.preparationCtx, s.preparationCancel = context.WithCancel(context.Background())
	}
	s.preparationRunner = executor
}

func (s *Service) ReconcileEnvironmentPreparationsAfterRestart(ctx context.Context) (int, error) {
	// Command startup calls this only after Kernel.Open has acquired the
	// database-scoped runtime advisory lock and before the HTTP listener starts.
	recoveryID, err := environmentPreparationEventRequestID()
	if err != nil {
		return 0, err
	}
	preparations, err := s.runtime.TXMarkUnrestoredEnvironmentPreparationsUnknown(ctx, recoveryID)
	if err != nil {
		return preparations, fmt.Errorf("mark environment preparations unknown after restart: %w", err)
	}
	jobs, err := s.runtime.TXMarkUnrestoredJobRunsUnknown(ctx, recoveryID)
	if err != nil {
		return preparations + jobs, fmt.Errorf("mark project JobRuns unknown after restart: %w", err)
	}
	reconciledJobs := 0
	for _, profileID := range s.projectJobRecoveryProfiles() {
		reconciled, reconcileErr := s.reconcileUnrestoredProjectJobs(ctx, profileID)
		reconciledJobs += reconciled
		if reconcileErr != nil {
			return preparations + jobs + reconciledJobs, reconcileErr
		}
	}
	return preparations + jobs + reconciledJobs, nil
}

func (s *Service) projectJobRecoveryProfiles() []string {
	s.projectJobsMu.Lock()
	executor := s.projectJobRunner
	s.projectJobsMu.Unlock()
	profiles := make([]string, 0, 2)
	if executor != nil {
		for _, profileID := range []string{environment.WindowsNodeNPMProfile, environment.LinuxNodeNPMProfile} {
			if executor.SupportsProjectJobProfile(profileID) {
				profiles = append(profiles, profileID)
			}
		}
	}
	if len(profiles) == 0 && runtime.GOOS == "windows" {
		profiles = append(profiles, environment.WindowsNodeNPMProfile)
	}
	return profiles
}

func (s *Service) reconcileUnrestoredProjectJobs(ctx context.Context, profileID string) (int, error) {
	jobs, err := s.runtime.ListUnrestoredProjectJobRuns(ctx, profileID)
	if err != nil {
		return 0, fmt.Errorf("list unrestored project JobRuns for profile %q: %w", profileID, err)
	}
	reconciled := 0
	for _, job := range jobs {
		requestID, requestErr := environmentPreparationEventRequestID()
		if requestErr != nil {
			return reconciled, requestErr
		}
		stopped, stopErr := s.StopProjectJob(ctx, job.CompanyID, job.JobID, StopProjectJobRequest{RequestID: requestID})
		if stopErr != nil {
			return reconciled, fmt.Errorf("failed to reconcile interrupted project JobRun %s: %w", job.JobID, stopErr)
		}
		if stopped.State != string(environment.JobCancelled) {
			return reconciled, core.ConflictError{Reason: "interrupted project JobRun remains unresolved", CurrentState: stopped.State}
		}
		reconciled++
	}
	return reconciled, nil
}

func (s *Service) EnsureProjectEnvironment(ctx context.Context, companyID, revisionID string, request EnsureEnvironmentRequest) (kernel.EnvironmentPreparationRun, error) {
	if !core.ValidID(companyID) || !core.ValidID(revisionID) || validateRequestID(request.RequestID) != nil {
		return kernel.EnvironmentPreparationRun{}, core.Malformed
	}
	revision, err := s.runtime.GetProjectEnvironmentRevision(ctx, companyID, revisionID)
	if err != nil {
		return kernel.EnvironmentPreparationRun{}, err
	}
	authorizedStorageFingerprint := ""
	if revision.ProfileID == environment.WindowsNodeNPMProfile {
		if fingerprint, available := s.runtime.CurrentEnvironmentExecutorFingerprint(revision.ProfileID); available {
			authorizedStorageFingerprint = fingerprint.IsolationPolicySHA256
		}
	}
	run, err := s.runtime.TXRequestEnvironmentPreparation(ctx, companyID, revisionID, request.RequestID)
	if err != nil {
		return run, err
	}
	s.preparationMu.RLock()
	executor := s.preparationRunner
	s.preparationMu.RUnlock()
	if run.State == string(environment.PreparationReady) {
		available := executor != nil
		if availability, ok := executor.(interface{ HasPreparedEnvironment(string) bool }); ok {
			available = availability.HasPreparedEnvironment(revisionID)
		}
		if available {
			return run, nil
		}
		if err = s.recordEnvironmentPreparationEvent(ctx, companyID, run.RunID, environment.PreparationOutcomeUnknown, "prepared_environment_not_available", "", "", 0, false); err != nil {
			return kernel.EnvironmentPreparationRun{}, err
		}
		return s.runtime.GetEnvironmentPreparationRun(ctx, companyID, run.RunID)
	}
	if run.State != string(environment.PreparationAccepted) {
		return run, nil
	}
	if revision.ProfileID == environment.WindowsNodeNPMProfile {
		currentFingerprint, available := s.runtime.CurrentEnvironmentExecutorFingerprint(revision.ProfileID)
		if !available || !environment.WindowsWorkspacePreparationFingerprintMatches(authorizedStorageFingerprint, currentFingerprint.IsolationPolicySHA256) {
			if err = s.recordEnvironmentPreparationEvent(ctx, companyID, run.RunID, environment.PreparationBlockedUnqualified, "environment_workspace_volume_changed", "", "", 0, false); err != nil {
				return kernel.EnvironmentPreparationRun{}, err
			}
			return s.runtime.GetEnvironmentPreparationRun(ctx, companyID, run.RunID)
		}
		authorizedStorageFingerprint = currentFingerprint.IsolationPolicySHA256
	}
	if executor == nil {
		if err = s.recordEnvironmentPreparationEvent(ctx, companyID, run.RunID, environment.PreparationBlockedUnqualified, "environment_executor_not_configured", "", "", 0, false); err != nil {
			return kernel.EnvironmentPreparationRun{}, err
		}
		return s.runtime.GetEnvironmentPreparationRun(ctx, companyID, run.RunID)
	}
	if profileSupport, ok := executor.(interface{ SupportsProjectEnvironmentProfile(string) bool }); ok {
		if !profileSupport.SupportsProjectEnvironmentProfile(revision.ProfileID) {
			if err = s.recordEnvironmentPreparationEvent(ctx, companyID, run.RunID, environment.PreparationBlockedUnqualified, "environment_executor_profile_unavailable", "", "", 0, false); err != nil {
				return kernel.EnvironmentPreparationRun{}, err
			}
			return s.runtime.GetEnvironmentPreparationRun(ctx, companyID, run.RunID)
		}
	}

	progressCtx, cancelProgress := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	err = s.recordEnvironmentPreparationEvent(progressCtx, companyID, run.RunID, environment.PreparationStarting, "environment_preparation_starting", "", "", 0, false)
	if err != nil {
		failedErr := s.recordEnvironmentPreparationEvent(progressCtx, companyID, run.RunID, environment.PreparationFailed, "environment_preparation_not_started", "", "", 0, false)
		if failedErr != nil {
			cancelProgress()
			return kernel.EnvironmentPreparationRun{}, errors.Join(err, failedErr)
		}
		current, readErr := s.runtime.GetEnvironmentPreparationRun(progressCtx, companyID, run.RunID)
		cancelProgress()
		return current, readErr
	}
	err = s.recordEnvironmentPreparationEvent(progressCtx, companyID, run.RunID, environment.PreparationRunning, "environment_preparation_running", "", "", 0, false)
	if err != nil {
		failedErr := s.recordEnvironmentPreparationEvent(progressCtx, companyID, run.RunID, environment.PreparationFailed, "environment_preparation_not_started", "", "", 0, false)
		if failedErr != nil {
			cancelProgress()
			return kernel.EnvironmentPreparationRun{}, errors.Join(err, failedErr)
		}
		current, readErr := s.runtime.GetEnvironmentPreparationRun(progressCtx, companyID, run.RunID)
		cancelProgress()
		return current, readErr
	}
	cancelProgress()
	executionCtx, cancelExecution := s.projectEnvironmentExecutionContext(ctx)
	defer cancelExecution()
	loadSnapshot := func(loadCtx context.Context) (kernel.ProjectEnvironmentExecutionSnapshot, error) {
		snapshot, snapshotErr := s.runtime.GetProjectEnvironmentExecutionSnapshot(loadCtx, companyID, revisionID)
		if snapshotErr == nil && revision.ProfileID == environment.WindowsNodeNPMProfile {
			snapshot.AuthorizedIsolationPolicySHA256 = authorizedStorageFingerprint
		}
		return snapshot, snapshotErr
	}
	result, executeErr := executor.PrepareProjectEnvironment(executionCtx, run.RunID, loadSnapshot)
	return s.finishEnvironmentPreparation(ctx, companyID, run.RunID, result, executeErr)
}

func (s *Service) projectEnvironmentExecutionContext(requestCtx context.Context) (context.Context, context.CancelFunc) {
	s.preparationMu.Lock()
	if s.preparationCtx == nil {
		s.preparationCtx, s.preparationCancel = context.WithCancel(context.Background())
	}
	lifetimeCtx := s.preparationCtx
	s.preparationMu.Unlock()
	executionCtx, cancel := context.WithCancel(requestCtx)
	stop := context.AfterFunc(lifetimeCtx, cancel)
	return executionCtx, func() {
		stop()
		cancel()
	}
}

func (s *Service) finishEnvironmentPreparation(ctx context.Context, companyID, runID string, result EnvironmentPreparationResult, executeErr error) (kernel.EnvironmentPreparationRun, error) {
	terminalState := environment.PreparationState(result.TerminalState)
	if !isEnvironmentPreparationTerminal(terminalState) || (executeErr != nil && terminalState == environment.PreparationReady) {
		terminalState = environment.PreparationOutcomeUnknown
	}
	reasonCode := result.ReasonCode
	if reasonCode == "" {
		reasonCode = environmentPreparationReason(terminalState)
	}
	evidenceSHA := ""
	logs := result.Logs
	logsTruncated := result.LogsTruncated
	if len(logs) > maxEnvironmentPreparationArtifactBytes {
		logs = logs[:maxEnvironmentPreparationArtifactBytes]
		logsTruncated = true
	}
	persistCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer cancel()
	if len(result.Evidence) > 0 {
		digest, err := s.runtime.StoreEnvironmentPreparationArtifact(persistCtx, companyID, runID, result.Evidence)
		if err != nil {
			terminalState = environment.PreparationOutcomeUnknown
			reasonCode = "environment_evidence_persist_failed"
		} else {
			evidenceSHA = digest
		}
	}
	logSHA := ""
	if len(logs) > 0 {
		digest, err := s.runtime.StoreEnvironmentPreparationArtifact(persistCtx, companyID, runID, logs)
		if err != nil {
			terminalState = environment.PreparationOutcomeUnknown
			reasonCode = "environment_log_persist_failed"
		} else {
			logSHA = digest
		}
	}
	if terminalState == environment.PreparationReady && evidenceSHA == "" {
		terminalState = environment.PreparationFailed
		reasonCode = "environment_evidence_missing"
	}
	eventRequestID, err := environmentPreparationEventRequestID()
	if err != nil {
		return kernel.EnvironmentPreparationRun{}, err
	}
	_, err = s.runtime.TXRecordEnvironmentPreparationEvent(persistCtx, companyID, kernel.EnvironmentPreparationEventInput{
		RunID: runID, State: string(terminalState), ReasonCode: reasonCode, EvidenceSHA256: evidenceSHA,
		LogSHA256: logSHA, LogBytes: len(logs), LogTruncated: logsTruncated, RequestID: eventRequestID,
	})
	if err != nil {
		if executeErr != nil {
			return kernel.EnvironmentPreparationRun{}, errors.Join(executeErr, err)
		}
		return kernel.EnvironmentPreparationRun{}, err
	}
	run, err := s.runtime.GetEnvironmentPreparationRun(persistCtx, companyID, runID)
	if err != nil {
		return kernel.EnvironmentPreparationRun{}, err
	}
	return run, nil
}

func (s *Service) recordEnvironmentPreparationEvent(ctx context.Context, companyID, runID string, state environment.PreparationState, reasonCode, evidenceSHA, logSHA string, logBytes int, logsTruncated bool) error {
	requestID, err := environmentPreparationEventRequestID()
	if err != nil {
		return err
	}
	_, err = s.runtime.TXRecordEnvironmentPreparationEvent(ctx, companyID, kernel.EnvironmentPreparationEventInput{
		RunID: runID, State: string(state), ReasonCode: reasonCode, EvidenceSHA256: evidenceSHA,
		LogSHA256: logSHA, LogBytes: logBytes, LogTruncated: logsTruncated, RequestID: requestID,
	})
	return err
}

func isEnvironmentPreparationTerminal(state environment.PreparationState) bool {
	switch state {
	case environment.PreparationReady, environment.PreparationFailed, environment.PreparationCancelled, environment.PreparationOutcomeUnknown:
		return true
	default:
		return false
	}
}

func environmentPreparationReason(state environment.PreparationState) string {
	switch state {
	case environment.PreparationReady:
		return "environment_preparation_ready"
	case environment.PreparationFailed:
		return "environment_preparation_failed"
	case environment.PreparationCancelled:
		return "environment_preparation_cancelled"
	default:
		return "environment_outcome_unknown"
	}
}

func environmentPreparationEventRequestID() (string, error) {
	var random [16]byte
	if _, err := rand.Read(random[:]); err != nil {
		return "", err
	}
	return "env-prep-" + hex.EncodeToString(random[:]), nil
}

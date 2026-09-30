// pattern: Imperative Shell
package control

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"polis/internal/core"
	"polis/internal/environment"
	"polis/internal/kernel"
	"polis/internal/runner"
)

const maxProjectJobLogManifestBytes = 2 << 20

type ProjectJobExecutionRequest struct {
	CompanyID             string
	JobID                 string
	ProfileID             string
	EnvironmentRevisionID string
	ServiceID             string
	ServiceProbe          *environment.ServiceProbeSpec
	ScriptPath            string
	Args                  []string
}

type ProjectJobExecutor interface {
	SupportsProjectJobProfile(profileID string) bool
	HasPreparedEnvironment(revisionID string) bool
	LaunchProjectJob(context.Context, ProjectJobExecutionRequest) (runner.AppContainerProcess, error)
	StopProjectJob(context.Context, string, string) error
	ForgetProjectJob(string)
}

type activeProjectJob struct {
	companyID            string
	jobID                string
	process              runner.AppContainerProcess
	service              *activeProjectService
	ctx                  context.Context
	cancel               context.CancelFunc
	done                 chan struct{}
	stopMu               sync.Mutex
	terminalMu           sync.Mutex
	stopRequested        bool
	pendingTerminalEvent *kernel.JobRunEventInput
	finishOnce           sync.Once
	cleanupOnce          sync.Once
	monitorOnce          sync.Once
}

func (active *activeProjectJob) markStopRequested() {
	active.terminalMu.Lock()
	active.stopRequested = true
	active.terminalMu.Unlock()
}

func (active *activeProjectJob) wasStopRequested() bool {
	active.terminalMu.Lock()
	defer active.terminalMu.Unlock()
	return active.stopRequested
}

func (active *activeProjectJob) prepareTerminalEvent(input kernel.JobRunEventInput) kernel.JobRunEventInput {
	active.terminalMu.Lock()
	defer active.terminalMu.Unlock()
	if active.pendingTerminalEvent == nil {
		input.RequestID = projectJobTerminalEventRequestID(active.companyID, input)
		copy := input
		active.pendingTerminalEvent = &copy
	}
	return *active.pendingTerminalEvent
}

func (active *activeProjectJob) pendingTerminal() (kernel.JobRunEventInput, bool) {
	active.terminalMu.Lock()
	defer active.terminalMu.Unlock()
	if active.pendingTerminalEvent == nil {
		return kernel.JobRunEventInput{}, false
	}
	return *active.pendingTerminalEvent, true
}

func projectJobTerminalEventRequestID(companyID string, input kernel.JobRunEventInput) string {
	input.RequestID = ""
	content, _ := json.Marshal(struct {
		CompanyID string
		Input     kernel.JobRunEventInput
	}{CompanyID: companyID, Input: input})
	return "project-job-terminal-" + digestBytes(content)[:32]
}

type activeProjectService struct {
	definition           environment.ProjectServiceDefinition
	verifier             environment.ServiceEndpointOwnerVerifier
	sourceRevisionSHA256 string
	healthcheckSHA256    string
	ready                chan struct{}
	readyOnce            sync.Once
	mu                   sync.Mutex
	endpointRecorded     bool
	endpointGeneration   int
	probeSequence        uint64
	pendingRevocation    *kernel.ServiceEndpointEventInput
	stopUnconfirmed      bool
	stopFailureReason    string
	failureReason        string
	browserIngress       *ServiceBrowserIngress
}

type projectJobLogManifest struct {
	SchemaVersion string `json:"schemaVersion"`
	CompanyID     string `json:"companyId"`
	JobID         string `json:"jobId"`
	Stdout        []byte `json:"stdout"`
	Stderr        []byte `json:"stderr"`
	StdoutSHA256  string `json:"stdoutSha256"`
	StderrSHA256  string `json:"stderrSha256"`
	LogsTruncated bool   `json:"logsTruncated"`
	LogGap        bool   `json:"logGap"`
}

func (s *Service) SetProjectJobExecutor(executor ProjectJobExecutor) {
	s.projectJobsMu.Lock()
	defer s.projectJobsMu.Unlock()
	if s.projectJobsClosed {
		return
	}
	if s.projectJobsCtx == nil {
		s.projectJobsCtx, s.projectJobsCancel = context.WithCancel(context.Background())
	}
	s.projectJobRunner = executor
	if s.projectJobs == nil {
		s.projectJobs = make(map[string]*activeProjectJob)
	}
}

func normalizeStartProjectJobRequest(request StartProjectJobRequest) (StartProjectJobRequest, error) {
	if !core.ValidID(request.TaskID) || (request.SessionID != "" && !core.ValidID(request.SessionID)) || !core.ValidID(request.EnvironmentRevisionID) || (request.HandoverID != "" && !core.ValidID(request.HandoverID)) || validateRequestID(request.RequestID) != nil {
		return StartProjectJobRequest{}, core.Malformed
	}
	switch request.Kind {
	case "batch":
		if request.ServiceID != "" {
			return StartProjectJobRequest{}, core.Malformed
		}
		scriptPath, err := environment.NormalizeNodeProjectScriptPath(request.ScriptPath)
		if err != nil || environment.ValidateNodeProjectScriptArgs(request.Args) != nil {
			return StartProjectJobRequest{}, core.Malformed
		}
		request.ScriptPath = scriptPath
		request.Args = append([]string(nil), request.Args...)
	case "service":
		if !core.ValidID(request.ServiceID) || request.ScriptPath != "" || len(request.Args) != 0 {
			return StartProjectJobRequest{}, core.Malformed
		}
	default:
		return StartProjectJobRequest{}, core.Malformed
	}
	return request, nil
}

func buildProjectJobLogManifest(companyID, jobID string, stdout, stderr []byte, truncated, gap bool) ([]byte, error) {
	manifest := projectJobLogManifest{
		SchemaVersion: "project-job-logs@1", CompanyID: companyID, JobID: jobID,
		Stdout: append([]byte(nil), stdout...), Stderr: append([]byte(nil), stderr...),
		StdoutSHA256: digestBytes(stdout), StderrSHA256: digestBytes(stderr), LogsTruncated: truncated, LogGap: gap,
	}
	content, err := json.Marshal(manifest)
	if err != nil || len(content) > maxProjectJobLogManifestBytes {
		return nil, errors.Join(err, core.Malformed)
	}
	return content, nil
}

func parseProjectJobLogManifest(content []byte) (projectJobLogManifest, error) {
	var manifest projectJobLogManifest
	decoder := json.NewDecoder(bytes.NewReader(content))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&manifest); err != nil || manifest.SchemaVersion != "project-job-logs@1" || len(content) > maxProjectJobLogManifestBytes {
		return projectJobLogManifest{}, core.Integrity
	}
	if manifest.StdoutSHA256 != digestBytes(manifest.Stdout) || manifest.StderrSHA256 != digestBytes(manifest.Stderr) {
		return projectJobLogManifest{}, core.Integrity
	}
	return manifest, nil
}

func digestBytes(content []byte) string {
	digest := sha256.Sum256(content)
	return hex.EncodeToString(digest[:])
}

func (s *Service) StartProjectJob(ctx context.Context, companyID string, request StartProjectJobRequest) (kernel.JobRunRecord, error) {
	request, err := normalizeStartProjectJobRequest(request)
	if err != nil || !core.ValidID(companyID) {
		return kernel.JobRunRecord{}, core.Malformed
	}
	s.projectLifecycleMu.RLock()
	projectLifecycleReadLocked := true
	defer func() {
		if projectLifecycleReadLocked {
			s.projectLifecycleMu.RUnlock()
		}
	}()
	lifecycleCtx, release, executor, err := s.beginProjectJobCommand()
	if err != nil {
		return kernel.JobRunRecord{}, err
	}
	defer release()
	if executor == nil || !executor.HasPreparedEnvironment(request.EnvironmentRevisionID) {
		return kernel.JobRunRecord{}, core.ConflictError{Reason: "qualified prepared environment is unavailable", CurrentState: "environment_not_ready"}
	}
	snapshot, err := s.runtime.GetProjectEnvironmentExecutionSnapshot(ctx, companyID, request.EnvironmentRevisionID)
	if err != nil {
		return kernel.JobRunRecord{}, err
	}
	if _, supported := environment.IsolationProfileForNodeProfile(snapshot.Revision.ProfileID); !supported || !executor.SupportsProjectJobProfile(snapshot.Revision.ProfileID) {
		return kernel.JobRunRecord{}, core.Denied
	}
	scriptPath := request.ScriptPath
	jobArgs := request.Args
	var projectService *activeProjectService
	var projectServiceProbe *environment.ServiceProbeSpec
	if request.Kind == "service" {
		var selected *environment.ProjectServiceDefinition
		for index := range snapshot.Policy.Services {
			if snapshot.Policy.Services[index].ID == request.ServiceID {
				definition := snapshot.Policy.Services[index]
				selected = &definition
				break
			}
		}
		verifier, ok := executor.(environment.ServiceEndpointOwnerVerifier)
		if selected == nil || !ok || !projectSnapshotContainsScript(snapshot, selected.ScriptPath) {
			return kernel.JobRunRecord{}, core.Denied
		}
		healthcheckDigest, digestErr := environment.ServiceProbeSpecSHA256(selected.Probe)
		if digestErr != nil {
			return kernel.JobRunRecord{}, core.Integrity
		}
		scriptPath = selected.ScriptPath
		jobArgs = nil
		projectService = &activeProjectService{
			definition: *selected, verifier: verifier, sourceRevisionSHA256: snapshot.Revision.SourceRevisionSHA256,
			healthcheckSHA256: healthcheckDigest, ready: make(chan struct{}),
		}
		serviceProbe := selected.Probe
		projectServiceProbe = &serviceProbe
	} else if !projectSnapshotContainsScript(snapshot, scriptPath) {
		return kernel.JobRunRecord{}, core.Denied
	}
	argvDigest, err := projectJobArgvDigest(scriptPath, jobArgs)
	if err != nil {
		return kernel.JobRunRecord{}, err
	}
	workingDirectoryDigest := digestBytes([]byte(snapshot.Revision.ProjectRootRelative))
	isolationProfile, _ := environment.IsolationProfileForNodeProfile(snapshot.Revision.ProfileID)
	networkPolicyDigest := digestBytes([]byte(isolationProfile + ":deny_all"))
	if projectServiceProbe != nil {
		networkPolicyDigest, err = projectJobServiceNetworkPolicyDigest(isolationProfile, *projectServiceProbe)
		if err != nil {
			return kernel.JobRunRecord{}, core.Integrity
		}
	}
	receipt, err := s.runtime.TXCreateJobRun(ctx, companyID, kernel.JobRunInput{
		TaskID: request.TaskID, SessionID: request.SessionID, EnvironmentRevisionID: request.EnvironmentRevisionID, HandoverID: request.HandoverID,
		Kind: request.Kind, ServiceID: request.ServiceID, SourceRevisionSHA256: snapshot.Revision.SourceRevisionSHA256, ArgvSHA256: argvDigest,
		WorkingDirectorySHA256: workingDirectoryDigest, NetworkPolicySHA256: networkPolicyDigest,
		EnvironmentAllowlist: nil, TimeoutMS: snapshot.Policy.TimeoutMS, OutputLimitBytes: snapshot.Policy.OutputLimitBytes,
		RequestID: request.RequestID,
	})
	if err != nil {
		return kernel.JobRunRecord{}, err
	}
	job, err := s.runtime.GetJobRun(ctx, companyID, receipt.ID)
	if err != nil || job.State != string(environment.JobAccepted) {
		return job, err
	}
	jobCtx, cancelJob := context.WithCancel(lifecycleCtx)
	active := &activeProjectJob{companyID: companyID, jobID: job.JobID, service: projectService, ctx: jobCtx, cancel: cancelJob, done: make(chan struct{})}
	s.projectJobsMu.Lock()
	if s.projectJobsClosed {
		cancelJob()
	}
	s.projectJobs[job.JobID] = active
	s.projectJobsWG.Add(1)
	s.projectJobsMu.Unlock()
	if err = s.recordProjectJobEvent(companyID, kernel.JobRunEventInput{JobID: job.JobID, State: string(environment.JobStarting), Readiness: projectJobReadiness(request.Kind), ReasonCode: "project_job_starting"}); err != nil {
		current, readErr := s.getProjectJobAfterCommand(companyID, job.JobID)
		active.cancel()
		s.finishProjectJobWithoutProcess(active, executor)
		if readErr != nil {
			return kernel.JobRunRecord{}, errors.Join(err, readErr)
		}
		if current.State != string(environment.JobAccepted) {
			return current, nil
		}
		return current, err
	}
	if jobCtx.Err() != nil {
		current, readErr := s.getProjectJobAfterCommand(companyID, job.JobID)
		if readErr == nil && (current.State == string(environment.JobAccepted) || current.State == string(environment.JobStarting)) {
			err = s.recordProjectJobEvent(companyID, kernel.JobRunEventInput{JobID: job.JobID, State: string(environment.JobCancelled), Readiness: projectJobReadiness(request.Kind), ReasonCode: "project_job_cancelled_before_launch"})
		}
		active.cancel()
		s.finishProjectJobWithoutProcess(active, executor)
		if readErr != nil || err != nil {
			return current, errors.Join(readErr, err)
		}
		return s.getProjectJobAfterCommand(companyID, job.JobID)
	}
	process, launchErr := executor.LaunchProjectJob(jobCtx, ProjectJobExecutionRequest{
		CompanyID: companyID, JobID: job.JobID, ProfileID: snapshot.Revision.ProfileID, EnvironmentRevisionID: request.EnvironmentRevisionID,
		ServiceID: request.ServiceID, ServiceProbe: projectServiceProbe, ScriptPath: scriptPath, Args: jobArgs,
	})
	if process != nil {
		s.projectJobsMu.Lock()
		active.process = process
		s.projectJobsMu.Unlock()
	}
	if launchErr != nil {
		if process != nil {
			active.cancel()
			go s.monitorProjectJob(active, executor, snapshot.Policy.TimeoutMS, snapshot.Policy.OutputLimitBytes)
			return s.getProjectJobAfterCommand(companyID, job.JobID)
		}
		terminal := environment.JobFailed
		reason := "project_job_launch_failed"
		if errors.Is(launchErr, runner.ErrAppContainerProcessStopUnconfirmed) {
			terminal, reason = environment.JobOutcomeUnknown, "project_job_launch_stop_unconfirmed"
		} else if errors.Is(launchErr, environment.ErrLinuxNodeCgroupCleanupUnconfirmed) {
			terminal, reason = environment.JobOutcomeUnknown, "project_job_launch_cgroup_cleanup_unconfirmed"
		} else if errors.Is(launchErr, context.Canceled) || jobCtx.Err() != nil {
			terminal, reason = environment.JobCancelled, "project_job_cancelled_before_launch"
		}
		persistErr := s.recordProjectJobEvent(companyID, kernel.JobRunEventInput{JobID: job.JobID, State: string(terminal), Readiness: projectJobReadiness(request.Kind), ReasonCode: reason})
		s.finishProjectJobWithoutProcess(active, executor)
		current, readErr := s.getProjectJobAfterCommand(companyID, job.JobID)
		if persistErr != nil || readErr != nil {
			return current, errors.Join(launchErr, persistErr, readErr)
		}
		return current, nil
	}
	if process == nil {
		launchErr = errors.New("project job executor returned no process handle")
		persistErr := s.recordProjectJobEvent(companyID, kernel.JobRunEventInput{JobID: job.JobID, State: string(environment.JobFailed), Readiness: projectJobReadiness(request.Kind), ReasonCode: "project_job_process_handle_missing"})
		s.finishProjectJobWithoutProcess(active, executor)
		current, readErr := s.getProjectJobAfterCommand(companyID, job.JobID)
		if persistErr != nil || readErr != nil {
			return current, errors.Join(launchErr, persistErr, readErr)
		}
		return current, nil
	}
	if jobCtx.Err() == nil {
		current, readErr := s.getProjectJobAfterCommand(companyID, job.JobID)
		if readErr != nil {
			active.cancel()
			go s.monitorProjectJob(active, executor, snapshot.Policy.TimeoutMS, snapshot.Policy.OutputLimitBytes)
			return current, readErr
		}
		if current.State != string(environment.JobStarting) {
			active.cancel()
			go s.monitorProjectJob(active, executor, snapshot.Policy.TimeoutMS, snapshot.Policy.OutputLimitBytes)
			return current, nil
		}
		if err = s.recordProjectJobEvent(companyID, kernel.JobRunEventInput{JobID: job.JobID, State: string(environment.JobRunning), Readiness: projectJobReadiness(request.Kind), ReasonCode: "project_job_running"}); err != nil {
			active.cancel()
			go s.monitorProjectJob(active, executor, snapshot.Policy.TimeoutMS, snapshot.Policy.OutputLimitBytes)
			current, readErr := s.getProjectJobAfterCommand(companyID, job.JobID)
			return current, errors.Join(err, readErr)
		}
	}
	go s.monitorProjectJob(active, executor, snapshot.Policy.TimeoutMS, snapshot.Policy.OutputLimitBytes)
	if projectService != nil {
		s.projectLifecycleMu.RUnlock()
		projectLifecycleReadLocked = false
		select {
		case <-projectService.ready:
			return s.getProjectJobAfterCommand(companyID, job.JobID)
		case <-active.done:
			return s.getProjectJobAfterCommand(companyID, job.JobID)
		case <-ctx.Done():
			active.cancel()
			_ = executor.StopProjectJob(context.Background(), companyID, job.JobID)
			<-active.done
			return kernel.JobRunRecord{}, ctx.Err()
		}
	}
	return s.getProjectJobAfterCommand(companyID, job.JobID)
}

func (s *Service) StopProjectJob(ctx context.Context, companyID, jobID string, request StopProjectJobRequest) (kernel.JobRunRecord, error) {
	if !core.ValidID(companyID) || !core.ValidID(jobID) || validateRequestID(request.RequestID) != nil {
		return kernel.JobRunRecord{}, core.Malformed
	}
	job, err := s.runtime.GetJobRun(ctx, companyID, jobID)
	if err != nil {
		return job, err
	}
	s.projectJobsMu.Lock()
	active := s.projectJobs[jobID]
	executor := s.projectJobRunner
	s.projectJobsMu.Unlock()
	if job.State == string(environment.JobExited) || job.State == string(environment.JobFailed) || job.State == string(environment.JobCancelled) {
		if active == nil || active.service == nil {
			if job.Kind != "service" {
				return job, nil
			}
			return s.revokePersistedTerminalServiceEndpoint(ctx, companyID, job)
		}
		if executor == nil {
			return job, nil
		}
		active.stopMu.Lock()
		defer active.stopMu.Unlock()
		if err = s.persistPendingProjectJobTerminal(active); err != nil {
			return job, err
		}
		s.cleanupProjectJob(active, executor)
		return s.runtime.GetJobRun(ctx, companyID, jobID)
	}
	if job.State != string(environment.JobAccepted) && job.State != string(environment.JobRunning) && job.State != string(environment.JobStarting) && job.State != string(environment.JobOutcomeUnknown) {
		return job, nil
	}
	if active == nil {
		var handled bool
		active, executor, job, handled, err = s.resolveProjectJobForStop(ctx, companyID, jobID, job)
		if err != nil || handled {
			return job, err
		}
	}
	if executor == nil {
		return job, core.ConflictError{Reason: "job process handle is unavailable; it will not be replayed", CurrentState: string(environment.JobOutcomeUnknown)}
	}
	active.stopMu.Lock()
	active.markStopRequested()
	active.cancel()
	if ingressErr := closeProjectServiceBrowserIngress(active); ingressErr != nil && active.service != nil {
		active.service.mu.Lock()
		active.service.failureReason = "service_browser_ingress_revocation_unconfirmed"
		active.service.mu.Unlock()
	}
	s.projectJobsMu.Lock()
	process := active.process
	s.projectJobsMu.Unlock()
	stopConfirmed := false
	if process != nil {
		if stopErr := executor.StopProjectJob(context.Background(), companyID, jobID); stopErr != nil {
			latest, readErr := s.getProjectJobAfterCommand(companyID, jobID)
			if readErr == nil && (latest.State == string(environment.JobRunning) || latest.State == string(environment.JobStarting)) {
				_ = s.recordProjectJobEvent(companyID, kernel.JobRunEventInput{JobID: jobID, State: string(environment.JobOutcomeUnknown), Readiness: projectJobReadinessForActive(active), ReasonCode: "project_job_stop_unconfirmed"})
				s.monitorUnconfirmedProjectJobExit(executor, active)
			}
		} else {
			stopConfirmed = true
		}
	}
	active.stopMu.Unlock()
	select {
	case <-active.done:
	case <-ctx.Done():
		return kernel.JobRunRecord{}, ctx.Err()
	}
	latest, err := s.runtime.GetJobRun(ctx, companyID, jobID)
	if err != nil || !stopConfirmed || (latest.State != string(environment.JobOutcomeUnknown) && latest.State != string(environment.JobRunning) && latest.State != string(environment.JobStarting)) {
		return latest, err
	}
	if _, pending := active.pendingTerminal(); pending {
		if err = s.persistPendingProjectJobTerminal(active); err != nil {
			return latest, err
		}
		s.cleanupProjectJob(active, executor)
		return s.runtime.GetJobRun(ctx, companyID, jobID)
	}
	if active.service != nil {
		if err = s.revokeProjectServiceEndpoint(active); err != nil {
			return latest, err
		}
	}
	if err = s.recordProjectJobEvent(companyID, kernel.JobRunEventInput{
		JobID: jobID, State: string(environment.JobCancelled), Readiness: projectJobReadinessForActive(active),
		ReasonCode: "project_job_resource_cleanup_confirmed", StdoutOffset: latest.StdoutOffset, StderrOffset: latest.StderrOffset,
		LogsTruncated: latest.LogsTruncated, LogGap: latest.LogGap,
	}); err != nil {
		current, readErr := s.runtime.GetJobRun(ctx, companyID, jobID)
		if readErr != nil || current.State != string(environment.JobCancelled) {
			return latest, errors.Join(err, readErr)
		}
		latest = current
	}
	s.cleanupProjectJob(active, executor)
	return s.runtime.GetJobRun(ctx, companyID, jobID)
}

func (s *Service) resolveProjectJobForStop(ctx context.Context, companyID, jobID string, fallback kernel.JobRunRecord) (*activeProjectJob, ProjectJobExecutor, kernel.JobRunRecord, bool, error) {
	current, err := s.runtime.GetJobRun(ctx, companyID, jobID)
	if err != nil {
		return nil, nil, fallback, true, err
	}
	if current.State == string(environment.JobAccepted) {
		writeErr := s.recordProjectJobEvent(companyID, kernel.JobRunEventInput{
			JobID: jobID, State: string(environment.JobCancelled), Readiness: projectJobReadiness(current.Kind),
			ReasonCode: "project_job_cancelled_before_process_registration",
		})
		latest, readErr := s.runtime.GetJobRun(ctx, companyID, jobID)
		if readErr == nil && latest.State == string(environment.JobCancelled) {
			current = latest
		} else if writeErr != nil {
			s.projectJobsMu.Lock()
			active := s.projectJobs[jobID]
			executor := s.projectJobRunner
			s.projectJobsMu.Unlock()
			if active != nil {
				return active, executor, latest, false, nil
			}
			return nil, nil, latest, true, errors.Join(writeErr, readErr)
		} else {
			current = latest
		}
	}
	s.projectJobsMu.Lock()
	active := s.projectJobs[jobID]
	executor := s.projectJobRunner
	s.projectJobsMu.Unlock()
	if active != nil {
		return active, executor, current, false, nil
	}
	if current.State != string(environment.JobRunning) && current.State != string(environment.JobStarting) && current.State != string(environment.JobOutcomeUnknown) {
		return nil, nil, current, true, nil
	}
	if current.State == string(environment.JobOutcomeUnknown) {
		reconciled := false
		var reconcileErr error
		if executor != nil {
			if windowsReconciler, ok := executor.(interface {
				ReconcileUnrestoredProjectJob(string, string, string) error
			}); ok {
				revision, revisionErr := s.runtime.GetProjectEnvironmentRevision(ctx, companyID, current.EnvironmentRevisionID)
				if revisionErr != nil {
					return nil, nil, current, true, revisionErr
				}
				if revision.ProfileID == environment.WindowsNodeNPMProfile {
					reconcileErr = windowsReconciler.ReconcileUnrestoredProjectJob(companyID, jobID, current.EnvironmentRevisionID)
					reconciled = reconcileErr == nil
				}
			}
			if !reconciled {
				if reconciler, ok := executor.(interface {
					ReconcilePendingProjectJob(string, string) error
				}); ok {
					reconcileErr = reconciler.ReconcilePendingProjectJob(companyID, jobID)
					reconciled = reconcileErr == nil
				}
			}
		} else {
			revision, revisionErr := s.runtime.GetProjectEnvironmentRevision(ctx, companyID, current.EnvironmentRevisionID)
			if revisionErr != nil {
				return nil, nil, current, true, revisionErr
			}
			if revision.ProfileID == environment.WindowsNodeNPMProfile {
				reconcileCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
				_, reconcileErr = runner.ReconcileWindowsProcessTree(reconcileCtx, jobID)
				cancel()
				reconciled = reconcileErr == nil
			}
		}
		if reconciled {
			latest, latestErr := s.getProjectJobAfterCommand(companyID, jobID)
			if latestErr != nil {
				return nil, nil, latest, true, latestErr
			}
			if latest.State == string(environment.JobOutcomeUnknown) {
				if latest.Kind == "service" {
					latest, latestErr = s.revokePersistedTerminalServiceEndpoint(ctx, companyID, latest)
					if latestErr != nil {
						return nil, nil, latest, true, latestErr
					}
				}
				persistErr := s.recordProjectJobEvent(companyID, kernel.JobRunEventInput{
					JobID: jobID, State: string(environment.JobCancelled), Readiness: projectJobReadiness(current.Kind),
					ReasonCode: "project_job_resource_cleanup_confirmed", StdoutOffset: latest.StdoutOffset, StderrOffset: latest.StderrOffset,
					LogsTruncated: latest.LogsTruncated, LogGap: latest.LogGap,
				})
				if persistErr != nil {
					return nil, nil, latest, true, persistErr
				}
				final, finalErr := s.getProjectJobAfterCommand(companyID, jobID)
				return nil, nil, final, true, finalErr
			}
			return nil, nil, latest, true, nil
		}
	}
	return nil, nil, current, true, core.ConflictError{Reason: "job process handle is unavailable; it will not be replayed", CurrentState: string(environment.JobOutcomeUnknown)}
}

func (s *Service) GetProjectJobLogs(ctx context.Context, companyID, jobID string) (kernel.JobRunLogArtifact, error) {
	if !core.ValidID(companyID) || !core.ValidID(jobID) {
		return kernel.JobRunLogArtifact{}, core.Malformed
	}
	return s.runtime.GetJobRunLogArtifact(ctx, companyID, jobID)
}

func (s *Service) CreateProjectJobBrowserSession(ctx context.Context, companyID, jobID string, request CreateProjectJobBrowserSessionRequest) (ServiceBrowserSession, error) {
	if !core.ValidID(companyID) || !core.ValidID(jobID) || validateRequestID(request.RequestID) != nil {
		return ServiceBrowserSession{}, core.Malformed
	}
	s.projectJobsMu.Lock()
	active := s.projectJobs[jobID]
	s.projectJobsMu.Unlock()
	if active == nil || active.companyID != companyID || active.service == nil || active.process == nil {
		return ServiceBrowserSession{}, core.Denied
	}
	active.stopMu.Lock()
	defer active.stopMu.Unlock()
	s.projectJobsMu.Lock()
	shuttingDown := s.projectJobsClosed || s.projectJobs[jobID] != active
	s.projectJobsMu.Unlock()
	if shuttingDown {
		return ServiceBrowserSession{}, core.ConflictError{Reason: "service browser access is unavailable while the JobRun is closing", CurrentState: string(environment.JobOutcomeUnknown)}
	}
	job, err := s.runtime.GetJobRun(ctx, companyID, jobID)
	if err != nil {
		return ServiceBrowserSession{}, err
	}
	if job.Kind != "service" || job.State != string(environment.JobRunning) || job.Readiness != environment.ServiceReady {
		return ServiceBrowserSession{}, core.ConflictError{Reason: "service browser access requires a running, ready service JobRun", CurrentState: job.State + "/" + job.Readiness}
	}
	service := active.service
	select {
	case <-service.ready:
	default:
		return ServiceBrowserSession{}, core.ConflictError{Reason: "service endpoint readiness has not been confirmed", CurrentState: environment.ServiceNotReady}
	}
	service.mu.Lock()
	defer service.mu.Unlock()
	if service.failureReason != "" || service.stopUnconfirmed || !service.endpointRecorded {
		return ServiceBrowserSession{}, core.ConflictError{Reason: "service endpoint is not available for browser access", CurrentState: environment.ServiceUnhealthy}
	}
	if service.browserIngress == nil {
		ingress, ingressErr := NewServiceBrowserIngress(active.process.PID(), service.definition.Probe, service.verifier)
		if ingressErr != nil {
			return ServiceBrowserSession{}, errors.Join(core.Denied, ingressErr)
		}
		service.browserIngress = ingress
	}
	return service.browserIngress.CreateSession(request.RequestID)
}

func (s *Service) beginProjectJobCommand() (context.Context, func(), ProjectJobExecutor, error) {
	s.projectJobsMu.Lock()
	defer s.projectJobsMu.Unlock()
	if s.projectJobsClosed {
		return nil, nil, nil, errors.New("project job service is closed")
	}
	if s.projectJobsCtx == nil {
		s.projectJobsCtx, s.projectJobsCancel = context.WithCancel(context.Background())
	}
	if s.projectJobs == nil {
		s.projectJobs = make(map[string]*activeProjectJob)
	}
	s.projectJobsWG.Add(1)
	return s.projectJobsCtx, s.projectJobsWG.Done, s.projectJobRunner, nil
}

func (s *Service) closeProjectJobs() error {
	s.projectJobsMu.Lock()
	s.projectJobsClosed = true
	cancel := s.projectJobsCancel
	active := make([]*activeProjectJob, 0, len(s.projectJobs))
	for _, job := range s.projectJobs {
		active = append(active, job)
	}
	executor := s.projectJobRunner
	s.projectJobsMu.Unlock()
	if cancel != nil {
		cancel()
	}
	for _, job := range active {
		job.stopMu.Lock()
		job.markStopRequested()
		job.cancel()
		if job.service != nil {
			if ingressErr := closeProjectServiceBrowserIngress(job); ingressErr != nil {
				job.service.mu.Lock()
				job.service.failureReason = "service_browser_ingress_revocation_unconfirmed"
				job.service.mu.Unlock()
			}
		}
		if executor != nil {
			s.projectJobsMu.Lock()
			process := job.process
			s.projectJobsMu.Unlock()
			if process != nil {
				_ = executor.StopProjectJob(context.Background(), job.companyID, job.jobID)
			}
		}
		job.stopMu.Unlock()
	}
	s.projectJobsWG.Wait()
	return nil
}

func (s *Service) monitorProjectJob(active *activeProjectJob, executor ProjectJobExecutor, timeoutMS, outputLimit int) {
	process := active.process
	streamLimit := (outputLimit - len("stdout:\n") - len("\nstderr:\n")) / 2
	stdout := capturePreparationStream(process.Stdout(), streamLimit)
	stderr := capturePreparationStream(process.Stderr(), streamLimit)
	jobCtx, cancel := context.WithTimeout(active.ctx, time.Duration(timeoutMS)*time.Millisecond)
	var serviceDone chan struct{}
	if active.service != nil {
		serviceDone = make(chan struct{})
		go s.monitorProjectServiceEndpoint(jobCtx, active, executor, serviceDone)
	}
	processExitCode, waitErr := process.Wait(jobCtx)
	contextErr := jobCtx.Err()
	cancel()
	if serviceDone != nil {
		<-serviceDone
	}
	if waitErr != nil {
		stdout.abort()
		stderr.abort()
	}
	stdoutResult, stderrResult := stdout.wait(), stderr.wait()
	stdoutBytes := redactRegistryProxyCredentials(stdoutResult.content)
	stderrBytes := redactRegistryProxyCredentials(stderrResult.content)
	truncated := stdoutResult.truncated || stderrResult.truncated
	state, reason, exitCode := classifyProjectJobCompletion(waitErr, contextErr, processExitCode)
	if active.service != nil {
		active.service.mu.Lock()
		stopUnconfirmed := active.service.stopUnconfirmed
		stopFailureReason := active.service.stopFailureReason
		failureReason := active.service.failureReason
		active.service.mu.Unlock()
		state, reason, exitCode = classifyProjectServiceJobCompletion(waitErr, contextErr, processExitCode, stopUnconfirmed, stopFailureReason, failureReason)
	}
	processUnconfirmed := errors.Is(waitErr, runner.ErrAppContainerProcessStopUnconfirmed)
	resourceCleanupUnconfirmed := errors.Is(waitErr, environment.ErrLinuxNodeCgroupCleanupUnconfirmed)
	if resourceCleanupUnconfirmed {
		if marker, ok := executor.(interface {
			MarkProjectJobCgroupCleanupUnconfirmed(string, string)
		}); ok {
			marker.MarkProjectJobCgroupCleanupUnconfirmed(active.companyID, active.jobID)
		}
	}
	if stdoutResult.err != nil || stderrResult.err != nil {
		state, reason, exitCode = environment.JobOutcomeUnknown, "project_job_log_capture_failed", nil
	}
	if active.wasStopRequested() && !processUnconfirmed && !resourceCleanupUnconfirmed && waitErr == nil {
		state, reason, exitCode = environment.JobCancelled, "project_job_cancelled", nil
	}
	input := kernel.JobRunEventInput{JobID: active.jobID, State: string(state), Readiness: projectJobReadinessForActive(active), ExitCode: exitCode, ReasonCode: reason}
	if len(stdoutBytes)+len(stderrBytes) > 0 {
		manifest, manifestErr := buildProjectJobLogManifest(active.companyID, active.jobID, stdoutBytes, stderrBytes, truncated, false)
		if manifestErr == nil {
			persistCtx, persistCancel := context.WithTimeout(context.Background(), 10*time.Second)
			manifestDigest, storeErr := s.runtime.StoreJobRunLogArtifact(persistCtx, active.companyID, active.jobID, manifest)
			persistCancel()
			if storeErr == nil {
				input.StdoutOffset, input.StderrOffset = int64(len(stdoutBytes)), int64(len(stderrBytes))
				input.StdoutBytes, input.StderrBytes = len(stdoutBytes), len(stderrBytes)
				input.LogsTruncated = truncated
				input.LogManifestSHA256 = manifestDigest
			} else {
				input.State, input.ReasonCode, input.ExitCode = string(environment.JobOutcomeUnknown), "project_job_log_persist_failed", nil
				state, reason = environment.JobOutcomeUnknown, "project_job_log_persist_failed"
			}
		} else {
			input.State, input.ReasonCode, input.ExitCode = string(environment.JobOutcomeUnknown), "project_job_log_manifest_failed", nil
			state, reason = environment.JobOutcomeUnknown, "project_job_log_manifest_failed"
		}
	}
	endpointRevocationUnconfirmed := false
	if active.service != nil {
		if err := s.revokeProjectServiceEndpoint(active); err != nil {
			endpointRevocationUnconfirmed = true
		}
	}
	pendingTerminal := !processUnconfirmed && !resourceCleanupUnconfirmed
	if pendingTerminal {
		input = active.prepareTerminalEvent(input)
	}
	terminalEventPersisted := false
	if err := s.recordProjectJobEvent(active.companyID, input); err == nil {
		terminalEventPersisted = true
	} else {
		current, readErr := s.getProjectJobAfterCommand(active.companyID, active.jobID)
		if readErr == nil && current.State == input.State {
			terminalEventPersisted = true
		}
		if !pendingTerminal && readErr == nil && (current.State == string(environment.JobRunning) || current.State == string(environment.JobStarting)) {
			_ = s.recordProjectJobEvent(active.companyID, kernel.JobRunEventInput{JobID: active.jobID, State: string(environment.JobOutcomeUnknown), Readiness: projectJobReadinessForActive(active), ReasonCode: "project_job_terminal_state_unconfirmed"})
		}
	}
	if processUnconfirmed || resourceCleanupUnconfirmed || endpointRevocationUnconfirmed || !terminalEventPersisted {
		s.monitorUnconfirmedProjectJobExit(executor, active)
		s.finishProjectJob(active, executor, true)
	} else {
		s.finishProjectJob(active, executor, false)
	}
	_ = state
	_ = reason
}

func (s *Service) monitorProjectServiceEndpoint(ctx context.Context, active *activeProjectJob, executor ProjectJobExecutor, done chan<- struct{}) {
	defer close(done)
	service := active.service
	if service == nil {
		return
	}
	probeInterval := projectServiceProbeInterval(service.definition.Probe)
	for {
		if ctx.Err() != nil {
			return
		}
		evidence, probeErr := environment.ProbeServiceEndpoint(ctx, active.process.PID(), service.definition.Probe, service.verifier)
		if probeErr != nil {
			if ctx.Err() != nil {
				return
			}
			service.mu.Lock()
			endpointRecorded := service.endpointRecorded
			service.mu.Unlock()
			if errors.Is(probeErr, environment.ErrServiceEndpointOwnerUnverified) && !endpointRecorded {
				if !waitProjectServiceProbe(ctx, 200*time.Millisecond) {
					return
				}
				continue
			}
			s.stopProjectServiceAfterProbeFailure(active, executor, "service_endpoint_owner_unverified")
			return
		}
		if evidence.LeaseExpiresAt.IsZero() {
			service.mu.Lock()
			endpointRecorded := service.endpointRecorded
			service.mu.Unlock()
			if !endpointRecorded {
				if !waitProjectServiceProbe(ctx, 200*time.Millisecond) {
					return
				}
				continue
			}
			s.stopProjectServiceAfterProbeFailure(active, executor, "service_endpoint_evidence_incomplete")
			return
		}
		requestID := nextProjectServiceEndpointRequestID(active.jobID, service, "probe")
		leaseExpiresAt := evidence.LeaseExpiresAt
		// The append may commit even if the call returns an error. Mark the
		// endpoint as possibly recorded before the transaction so shutdown and
		// stop paths always attempt a compensating revoke with a stable request.
		service.mu.Lock()
		service.endpointRecorded = true
		service.endpointGeneration = 1
		service.mu.Unlock()
		_, writeErr := s.runtime.TXRecordServiceEndpointEvent(ctx, active.companyID, kernel.ServiceEndpointEventInput{
			JobID: active.jobID, Generation: 1, BindAddress: evidence.BindAddress, Port: evidence.Port,
			Readiness: string(evidence.Readiness), SourceRevisionSHA256: service.sourceRevisionSHA256,
			HealthcheckSHA256: evidence.HealthcheckSHA256, ProbedAt: evidence.ProbedAt,
			LeaseExpiresAt: &leaseExpiresAt, RequestID: requestID,
		})
		if writeErr != nil {
			if ctx.Err() != nil {
				return
			}
			s.stopProjectServiceAfterProbeFailure(active, executor, "service_endpoint_persist_failed")
			return
		}
		if evidence.Readiness == environment.ServiceReady {
			service.readyOnce.Do(func() { close(service.ready) })
		}
		if !waitProjectServiceProbe(ctx, probeInterval) {
			return
		}
	}
}

func (s *Service) stopProjectServiceAfterProbeFailure(active *activeProjectJob, executor ProjectJobExecutor, reason string) {
	_ = s.revokeProjectServiceEndpoint(active)
	stopErr := executor.StopProjectJob(context.Background(), active.companyID, active.jobID)
	if active.service != nil {
		active.service.mu.Lock()
		active.service.failureReason = reason
		active.service.stopUnconfirmed = stopErr != nil
		active.service.stopFailureReason = "service_endpoint_stop_unconfirmed"
		active.service.mu.Unlock()
	}
	active.cancel()
}

func (s *Service) revokeProjectServiceEndpoint(active *activeProjectJob) error {
	service := active.service
	if service == nil {
		return nil
	}
	if err := closeProjectServiceBrowserIngress(active); err != nil {
		return fmt.Errorf("service browser ingress revocation is unconfirmed: %w", err)
	}
	input, recorded := pendingProjectServiceRevocationInput(active)
	if !recorded {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_, err := s.runtime.TXRecordServiceEndpointEvent(ctx, active.companyID, input)
	if err != nil {
		return err
	}
	service.mu.Lock()
	service.endpointRecorded = false
	service.pendingRevocation = nil
	service.mu.Unlock()
	return nil
}

func closeProjectServiceBrowserIngress(active *activeProjectJob) error {
	if active == nil || active.service == nil {
		return nil
	}
	service := active.service
	service.mu.Lock()
	ingress := service.browserIngress
	service.mu.Unlock()
	if ingress == nil {
		return nil
	}
	if err := ingress.Close(); err != nil {
		return err
	}
	service.mu.Lock()
	if service.browserIngress == ingress {
		service.browserIngress = nil
	}
	service.mu.Unlock()
	return nil
}

func pendingProjectServiceRevocationInput(active *activeProjectJob) (kernel.ServiceEndpointEventInput, bool) {
	if active == nil || active.service == nil {
		return kernel.ServiceEndpointEventInput{}, false
	}
	service := active.service
	service.mu.Lock()
	defer service.mu.Unlock()
	if !service.endpointRecorded {
		return kernel.ServiceEndpointEventInput{}, false
	}
	if service.pendingRevocation == nil {
		service.pendingRevocation = &kernel.ServiceEndpointEventInput{
			JobID: active.jobID, Generation: service.endpointGeneration, BindAddress: service.definition.Probe.BindAddress,
			Port: service.definition.Probe.Port, Readiness: string(environment.ServiceRevoked),
			SourceRevisionSHA256: service.sourceRevisionSHA256, HealthcheckSHA256: service.healthcheckSHA256,
			ProbedAt: time.Now().UTC(), RequestID: stableProjectServiceRevocationRequestID(active.jobID, service.endpointGeneration),
		}
	}
	return *service.pendingRevocation, true
}

func stableProjectServiceRevocationRequestID(jobID string, generation int) string {
	content := []byte(fmt.Sprintf("%s:%d:revoke", jobID, generation))
	return "service-revoke-" + digestBytes(content)[:32]
}

func nextProjectServiceEndpointRequestID(jobID string, service *activeProjectService, action string) string {
	service.mu.Lock()
	service.probeSequence++
	sequence := service.probeSequence
	service.mu.Unlock()
	content := []byte(fmt.Sprintf("%s:%d:%s", jobID, sequence, action))
	return "service-" + action + "-" + digestBytes(content)[:32]
}

func projectServiceProbeInterval(spec environment.ServiceProbeSpec) time.Duration {
	interval := time.Duration(spec.LeaseDurationMS) * time.Millisecond / 2
	if interval < 100*time.Millisecond {
		return 100 * time.Millisecond
	}
	if interval > 5*time.Second {
		return 5 * time.Second
	}
	return interval
}

func waitProjectServiceProbe(ctx context.Context, interval time.Duration) bool {
	timer := time.NewTimer(interval)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

func projectJobReadiness(kind string) string {
	if kind == "service" {
		return ""
	}
	return "not_applicable"
}

func projectJobReadinessForActive(active *activeProjectJob) string {
	if active != nil && active.service != nil {
		return ""
	}
	return "not_applicable"
}

func classifyProjectJobCompletion(waitErr, contextErr error, processExitCode int) (environment.JobState, string, *int) {
	if waitErr == nil {
		exitCode := processExitCode
		if exitCode != 0 {
			return environment.JobExited, "project_job_nonzero_exit", &exitCode
		}
		return environment.JobExited, "none", &exitCode
	}
	if errors.Is(waitErr, environment.ErrLinuxNodeCgroupCleanupUnconfirmed) {
		return environment.JobOutcomeUnknown, "project_job_cgroup_cleanup_unconfirmed", nil
	}
	if errors.Is(waitErr, runner.ErrAppContainerProcessStopUnconfirmed) || (!errors.Is(waitErr, context.Canceled) && !errors.Is(waitErr, context.DeadlineExceeded)) {
		return environment.JobOutcomeUnknown, "project_job_outcome_unknown", nil
	}
	if errors.Is(waitErr, context.DeadlineExceeded) || errors.Is(contextErr, context.DeadlineExceeded) {
		return environment.JobFailed, "project_job_timeout", nil
	}
	return environment.JobCancelled, "project_job_cancelled", nil
}

func classifyProjectServiceJobCompletion(waitErr, contextErr error, processExitCode int, stopUnconfirmed bool, stopFailureReason, failureReason string) (environment.JobState, string, *int) {
	state, reason, exitCode := classifyProjectJobCompletion(waitErr, contextErr, processExitCode)
	if stopUnconfirmed {
		if stopFailureReason == "" {
			stopFailureReason = "service_endpoint_stop_unconfirmed"
		}
		return environment.JobOutcomeUnknown, stopFailureReason, nil
	}
	if state == environment.JobOutcomeUnknown {
		return environment.JobOutcomeUnknown, reason, nil
	}
	if failureReason != "" {
		return environment.JobFailed, failureReason, nil
	}
	return state, reason, exitCode
}

func (s *Service) monitorUnconfirmedProjectJobExit(executor ProjectJobExecutor, active *activeProjectJob) {
	active.monitorOnce.Do(func() {
		go func() {
			_, waitErr := active.process.Wait(context.Background())
			<-active.done
			if waitErr != nil {
				return
			}
			for attempt := 0; attempt < 6; attempt++ {
				if _, pending := active.pendingTerminal(); pending {
					if err := s.persistPendingProjectJobTerminal(active); err == nil {
						s.cleanupProjectJob(active, executor)
						return
					}
					time.Sleep(time.Duration(attempt+1) * 150 * time.Millisecond)
					continue
				}
				if active.service != nil {
					if err := s.revokeProjectServiceEndpoint(active); err != nil {
						time.Sleep(time.Duration(attempt+1) * 150 * time.Millisecond)
						continue
					}
				}
				latest, err := s.getProjectJobAfterCommand(active.companyID, active.jobID)
				if err == nil && (latest.State == string(environment.JobOutcomeUnknown) || latest.State == string(environment.JobRunning) || latest.State == string(environment.JobStarting)) {
					err = s.recordProjectJobEvent(active.companyID, kernel.JobRunEventInput{
						JobID: active.jobID, State: string(environment.JobCancelled), Readiness: projectJobReadinessForActive(active),
						ReasonCode: "project_job_process_exit_confirmed", StdoutOffset: latest.StdoutOffset, StderrOffset: latest.StderrOffset,
						LogsTruncated: latest.LogsTruncated, LogGap: latest.LogGap,
					})
					if err != nil {
						latest, err = s.getProjectJobAfterCommand(active.companyID, active.jobID)
					}
					if err == nil && latest.State == string(environment.JobCancelled) {
						s.cleanupProjectJob(active, executor)
						return
					}
				} else if err == nil && (latest.State == string(environment.JobExited) || latest.State == string(environment.JobFailed) || latest.State == string(environment.JobCancelled)) {
					s.cleanupProjectJob(active, executor)
					return
				}
				time.Sleep(time.Duration(attempt+1) * 150 * time.Millisecond)
			}
		}()
	})
}

func (s *Service) finishProjectJobWithoutProcess(active *activeProjectJob, executor ProjectJobExecutor) {
	s.finishProjectJob(active, executor, false)
}

func (s *Service) finishProjectJob(active *activeProjectJob, executor ProjectJobExecutor, retainProcess bool) {
	active.finishOnce.Do(func() {
		close(active.done)
		s.projectJobsWG.Done()
	})
	if !retainProcess {
		s.cleanupProjectJob(active, executor)
	}
}

func (s *Service) cleanupProjectJob(active *activeProjectJob, executor ProjectJobExecutor) {
	active.cleanupOnce.Do(func() {
		executor.ForgetProjectJob(active.jobID)
		s.projectJobsMu.Lock()
		if s.projectJobs[active.jobID] == active {
			delete(s.projectJobs, active.jobID)
		}
		s.projectJobsMu.Unlock()
	})
}

func (s *Service) recordProjectJobEvent(companyID string, input kernel.JobRunEventInput) error {
	if input.RequestID == "" {
		requestID, err := environmentPreparationEventRequestID()
		if err != nil {
			return err
		}
		input.RequestID = requestID
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_, err := s.runtime.TXRecordJobRunEvent(ctx, companyID, input)
	return err
}

func (s *Service) persistPendingProjectJobTerminal(active *activeProjectJob) error {
	input, pending := active.pendingTerminal()
	if !pending {
		return nil
	}
	if active.service != nil {
		if err := s.revokeProjectServiceEndpoint(active); err != nil {
			return err
		}
	}
	current, err := s.getProjectJobAfterCommand(active.companyID, active.jobID)
	if err != nil {
		return err
	}
	if current.State == input.State {
		return nil
	}
	if !environment.CanTransitionJob(current.State, input.State) {
		return core.ConflictError{Reason: "pending terminal JobRun event cannot follow current state", CurrentState: current.State}
	}
	if err = s.recordProjectJobEvent(active.companyID, input); err != nil {
		current, readErr := s.getProjectJobAfterCommand(active.companyID, active.jobID)
		if readErr == nil && current.State == input.State {
			return nil
		}
		return errors.Join(err, readErr)
	}
	current, err = s.getProjectJobAfterCommand(active.companyID, active.jobID)
	if err != nil {
		return err
	}
	if current.State != input.State {
		return core.ConflictError{Reason: "pending terminal JobRun event was not projected", CurrentState: current.State}
	}
	return nil
}

func (s *Service) revokePersistedTerminalServiceEndpoint(ctx context.Context, companyID string, job kernel.JobRunRecord) (kernel.JobRunRecord, error) {
	latest, found, err := s.runtime.GetLatestServiceEndpointRecord(ctx, companyID, job.JobID)
	if err != nil || !found || latest.Readiness == environment.ServiceRevoked {
		return job, err
	}
	input := kernel.ServiceEndpointEventInput{
		JobID: job.JobID, Generation: latest.Generation, BindAddress: latest.BindAddress, Port: latest.Port,
		Readiness: string(environment.ServiceRevoked), SourceRevisionSHA256: latest.SourceRevisionSHA256,
		HealthcheckSHA256: latest.HealthcheckSHA256, ProbedAt: latest.ProbedAt.Add(time.Microsecond),
		RequestID: stableProjectServiceRevocationRequestID(job.JobID, latest.Generation),
	}
	if _, err = s.runtime.TXRecordServiceEndpointEvent(ctx, companyID, input); err != nil {
		current, currentFound, readErr := s.runtime.GetLatestServiceEndpointRecord(ctx, companyID, job.JobID)
		if readErr != nil || !currentFound || current.Readiness != environment.ServiceRevoked {
			return job, errors.Join(err, readErr)
		}
	}
	return s.runtime.GetJobRun(ctx, companyID, job.JobID)
}

func (s *Service) getProjectJobAfterCommand(companyID, jobID string) (kernel.JobRunRecord, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return s.runtime.GetJobRun(ctx, companyID, jobID)
}

func projectSnapshotContainsScript(snapshot kernel.ProjectEnvironmentExecutionSnapshot, scriptPath string) bool {
	projectScriptPath := scriptPath
	if snapshot.Plan.ProjectRoot != "." {
		projectScriptPath = snapshot.Plan.ProjectRoot + "/" + scriptPath
	}
	for _, file := range snapshot.Files {
		if file.RelativePath == projectScriptPath {
			return true
		}
	}
	return false
}

func projectJobArgvDigest(scriptPath string, args []string) (string, error) {
	canonical, err := json.Marshal(struct {
		ScriptPath string   `json:"scriptPath"`
		Args       []string `json:"args"`
	}{ScriptPath: scriptPath, Args: args})
	if err != nil {
		return "", fmt.Errorf("encode immutable JobRun argv: %w", err)
	}
	return digestBytes(canonical), nil
}

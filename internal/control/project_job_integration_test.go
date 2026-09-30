// pattern: Imperative Shell
package control

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"polis/internal/core"
	"polis/internal/environment"
	"polis/internal/intake"
	"polis/internal/kernel"
	"polis/internal/runner"
)

func TestProjectJobLifecycleLaunchesOncePersistsLogsAndReplaysWithoutExecution(t *testing.T) {
	dsn := os.Getenv("POLIS_TEST_DSN")
	if dsn == "" {
		t.Skip("dedicated PostgreSQL required")
	}
	ctx := context.Background()
	serviceBody := []byte("ready")
	serviceServer := httptest.NewUnstartedServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet || request.URL.Path != "/healthz" {
			t.Errorf("service health request=%s %s", request.Method, request.URL.Path)
		}
		_, _ = response.Write(serviceBody)
	}))
	serviceListener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	serviceServer.Listener = serviceListener
	serviceServer.Start()
	defer serviceServer.Close()
	serviceProbe := environment.ServiceProbeSpec{
		BindAddress: "127.0.0.1", Port: uint16(serviceListener.Addr().(*net.TCPAddr).Port), Path: "/healthz",
		ExpectedStatusCode: http.StatusOK, ExpectedBodySHA256: digestBytes(serviceBody), TimeoutMS: 500, LeaseDurationMS: 1000,
	}
	runtime, err := kernel.Open(ctx, dsn, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Close()
	companyID := fmt.Sprintf("project-job-%d", time.Now().UnixNano())
	scope, err := runtime.TXCreateCompany(ctx, companyID)
	if err != nil {
		t.Fatal(err)
	}
	missionID := "project-job-mission-" + fmt.Sprint(time.Now().UnixNano())
	if err = runtime.TXCreateMission(ctx, scope, missionID); err != nil {
		t.Fatal(err)
	}
	if _, err = runtime.TXStartMissionCommand(ctx, scope, missionID, "project-job-start-mission"); err != nil {
		t.Fatal(err)
	}
	directory, err := intake.PrepareDirectorySnapshot([]intake.DirectoryInputFile{
		{RelativePath: "repo/package.json", MediaType: "application/json", Content: []byte(`{"name":"demo","version":"1.0.0"}`)},
		{RelativePath: "repo/package-lock.json", MediaType: "application/json", Content: []byte(`{"name":"demo","version":"1.0.0","lockfileVersion":3,"packages":{"":{"name":"demo","version":"1.0.0"}}}`)},
		{RelativePath: "repo/scripts/build.mjs", MediaType: "text/javascript", Content: []byte("process.stdout.write('build ok')")},
		{RelativePath: "repo/ui/server.mjs", MediaType: "text/javascript", Content: []byte("process.stdout.write('service fixture')")},
	})
	if err != nil {
		t.Fatal(err)
	}
	source, err := runtime.TXAddMissionInput(ctx, scope, missionID, "", "project-job-source", directory.Upload, directory.Archive)
	if err != nil {
		t.Fatal(err)
	}
	evidenceBytes := []byte("synthetic host qualification evidence for the project JobRun integration test")
	evidenceUpload, err := intake.PrepareUpload("executor-qualification.md", "text/markdown", evidenceBytes)
	if err != nil {
		t.Fatal(err)
	}
	evidenceInput, err := runtime.TXAddMissionInput(ctx, scope, missionID, "", "project-job-qualification-evidence", evidenceUpload, evidenceBytes)
	if err != nil {
		t.Fatal(err)
	}
	toolchainDigest := strings.Repeat("e", 64)
	policy, _, _, err := environment.BuildWindowsNodeEnvironmentPolicy([]string{"registry.npmjs.org"}, 60_000, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	shadowProbe := serviceProbe
	shadowProbe.Port++
	policy.Services = []environment.ProjectServiceDefinition{
		{ID: "web", ScriptPath: "ui/server.mjs", Probe: serviceProbe},
		{ID: "web-shadow", ScriptPath: "ui/server.mjs", Probe: shadowProbe},
	}
	_, policyJSON, _, err := environment.CanonicalizeWindowsNodeEnvironmentPolicy(policy)
	if err != nil {
		t.Fatal(err)
	}
	service := NewService(runtime, &recordingWorkerAdapter{})
	defer service.Close()
	preparation := &recordingEnvironmentPreparationExecutor{}
	service.SetProjectEnvironmentPreparationExecutor(preparation)
	revision, err := service.RegisterProjectEnvironment(ctx, companyID, RegisterProjectEnvironmentRequest{
		ProfileID: environment.WindowsNodeNPMProfile, MissionID: missionID, SourceInputID: source.InputID, SourceInputRevision: "1",
		PolicyManifest: json.RawMessage(policyJSON), ToolchainSHA256: toolchainDigest, RequestID: "project-job-environment-register",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = service.DecideProjectEnvironmentPolicy(ctx, companyID, revision.RevisionID, EnvironmentPolicyDecisionRequest{
		Decision: "approved", Rationale: "fixture approves fixed source and lockfile", RequestID: "project-job-environment-approve",
	}); err != nil {
		t.Fatal(err)
	}
	qualificationPool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer qualificationPool.Close()
	qualifyTestEnvironmentExecutor(t, ctx, runtime, companyID, revision.RevisionID, evidenceInput, "project-job-qualify")
	prepared, err := service.EnsureProjectEnvironment(ctx, companyID, revision.RevisionID, EnsureEnvironmentRequest{RequestID: "project-job-ensure"})
	if err != nil || prepared.State != string(environment.PreparationReady) {
		t.Fatalf("prepared environment=%+v error=%v", prepared, err)
	}
	taskID, sessionID := "project-job-task", "project-job-session"
	_, err = qualificationPool.Exec(ctx, `INSERT INTO tasks(company_id,id,mission_id,owner,kind,state) VALUES($1,$2,$3,$4,'compat','working')`, companyID, taskID, missionID, core.EmployeeBackendID)
	if err != nil {
		t.Fatal(err)
	}
	_, err = qualificationPool.Exec(ctx, `INSERT INTO worker_sessions(company_id,id,employee_id,task_id,generation,epoch,incarnation,profile,state) VALUES($1,$2,$3,$4,1,1,'offline-fixture','offline-fixture','active')`, companyID, sessionID, core.EmployeeBackendID, taskID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = qualificationPool.Exec(ctx, `INSERT INTO job_runs(company_id,job_id,task_id,session_id,environment_revision_id,kind,service_id,source_revision_sha256,argv_sha256,working_directory_sha256,network_policy_sha256,environment_allowlist,timeout_ms,output_limit_bytes,request_id)
VALUES($1,'project-job-service-null-id',$2,$3,$4,'service',NULL,$5,$6,$7,$8,'[]'::jsonb,30000,65536,'project-job-service-null-id')`, companyID, taskID, sessionID, revision.RevisionID, revision.SourceRevisionSHA256, strings.Repeat("1", 64), strings.Repeat("2", 64), strings.Repeat("3", 64)); err == nil {
		t.Fatal("Schema 32 accepted a service JobRun without a persisted service ID")
	}
	jobExecutor := &recordingProjectJobExecutor{processes: []runner.AppContainerProcess{
		&finishedProjectJobProcess{stdout: []byte("build ok"), stderr: []byte("warning")},
		&cancelledProjectJobProcess{stopped: make(chan struct{})},
		&cancelledProjectJobProcess{stopped: make(chan struct{})},
	}}
	service.SetProjectJobExecutor(jobExecutor)
	request := StartProjectJobRequest{
		TaskID: taskID, SessionID: sessionID, EnvironmentRevisionID: revision.RevisionID,
		Kind: "batch", ScriptPath: "scripts/build.mjs", Args: []string{"--check"}, RequestID: "project-job-start",
	}
	preRegistrationSnapshot, err := runtime.GetProjectEnvironmentExecutionSnapshot(ctx, companyID, revision.RevisionID)
	if err != nil {
		t.Fatal(err)
	}
	preRegistrationArgv, err := projectJobArgvDigest(request.ScriptPath, request.Args)
	if err != nil {
		t.Fatal(err)
	}
	preRegistrationIsolation, _ := environment.IsolationProfileForNodeProfile(preRegistrationSnapshot.Revision.ProfileID)
	preRegistrationRequestID := "project-job-stop-before-registration"
	preRegistrationJob, err := runtime.TXCreateJobRun(ctx, companyID, kernel.JobRunInput{
		TaskID: taskID, SessionID: sessionID, EnvironmentRevisionID: revision.RevisionID, Kind: "batch",
		SourceRevisionSHA256: preRegistrationSnapshot.Revision.SourceRevisionSHA256, ArgvSHA256: preRegistrationArgv,
		WorkingDirectorySHA256: digestBytes([]byte(preRegistrationSnapshot.Revision.ProjectRootRelative)),
		NetworkPolicySHA256:    digestBytes([]byte(preRegistrationIsolation + ":deny_all")),
		TimeoutMS:              preRegistrationSnapshot.Policy.TimeoutMS, OutputLimitBytes: preRegistrationSnapshot.Policy.OutputLimitBytes,
		RequestID: preRegistrationRequestID,
	})
	if err != nil {
		t.Fatal(err)
	}
	preRegistrationStopped, err := service.StopProjectJob(ctx, companyID, preRegistrationJob.ID, StopProjectJobRequest{RequestID: "project-job-stop-accepted"})
	if err != nil || preRegistrationStopped.State != string(environment.JobCancelled) {
		t.Fatalf("Stop did not cancel an accepted JobRun before process registration: %+v error=%v", preRegistrationStopped, err)
	}
	preRegistrationStart := request
	preRegistrationStart.RequestID = preRegistrationRequestID
	replayedCancelled, err := service.StartProjectJob(ctx, companyID, preRegistrationStart)
	if err != nil || replayedCancelled.State != string(environment.JobCancelled) || jobExecutor.launches != 0 {
		t.Fatalf("cancelled accepted JobRun was launched on RequestID replay: %+v launches=%d error=%v", replayedCancelled, jobExecutor.launches, err)
	}
	started, err := service.StartProjectJob(ctx, companyID, request)
	if err != nil || started.JobID == "" {
		t.Fatalf("start JobRun=%+v error=%v", started, err)
	}
	deadline := time.Now().Add(3 * time.Second)
	for started.State != string(environment.JobExited) && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
		started, err = runtime.GetJobRun(ctx, companyID, started.JobID)
		if err != nil {
			t.Fatal(err)
		}
	}
	if started.State != string(environment.JobExited) || started.ExitCode == nil || *started.ExitCode != 0 || jobExecutor.launches != 1 {
		t.Fatalf("JobRun completion=%+v executor launches=%d", started, jobExecutor.launches)
	}
	artifact, err := runtime.GetJobRunLogArtifact(ctx, companyID, started.JobID)
	if err != nil {
		t.Fatal(err)
	}
	manifest, err := parseProjectJobLogManifest(artifact.Content)
	if err != nil || string(manifest.Stdout) != "build ok" || string(manifest.Stderr) != "warning" || artifact.ManifestSHA256 != started.LogManifestSHA256 {
		t.Fatalf("JobRun log artifact=%+v manifest=%+v error=%v", artifact, manifest, err)
	}
	replayed, err := service.StartProjectJob(ctx, companyID, request)
	if err != nil || replayed.JobID != started.JobID || replayed.State != string(environment.JobExited) || jobExecutor.launches != 1 {
		t.Fatalf("JobRun replay=%+v launches=%d error=%v", replayed, jobExecutor.launches, err)
	}
	stopRequest := request
	stopRequest.RequestID = "project-job-stop-start"
	stopStarted, err := service.StartProjectJob(ctx, companyID, stopRequest)
	if err != nil || stopStarted.State != string(environment.JobRunning) {
		t.Fatalf("second JobRun did not remain active for stop: %+v error=%v", stopStarted, err)
	}
	stopped, err := service.StopProjectJob(ctx, companyID, stopStarted.JobID, StopProjectJobRequest{RequestID: "project-job-stop"})
	if err != nil || stopped.State != string(environment.JobCancelled) || jobExecutor.launches != 2 {
		t.Fatalf("stopped JobRun=%+v launches=%d error=%v", stopped, jobExecutor.launches, err)
	}
	serviceRequest := StartProjectJobRequest{
		TaskID: taskID, SessionID: sessionID, EnvironmentRevisionID: revision.RevisionID,
		Kind: "service", ServiceID: "web", RequestID: "project-job-service-start",
	}
	service.SetProjectJobExecutor(&unverifiedServiceProjectJobExecutor{executor: &recordingProjectJobExecutor{}})
	var jobsBeforeUnverifiedStart int
	if err = qualificationPool.QueryRow(ctx, `SELECT count(*) FROM job_runs WHERE company_id=$1`, companyID).Scan(&jobsBeforeUnverifiedStart); err != nil {
		t.Fatal(err)
	}
	unverifiedRequest := serviceRequest
	unverifiedRequest.RequestID = "project-job-service-without-owner-proof"
	if _, err = service.StartProjectJob(ctx, companyID, unverifiedRequest); !errors.Is(err, core.Denied) {
		t.Fatalf("service JobRun started without a process-owner verifier: %v", err)
	}
	var jobsAfterUnverifiedStart int
	if err = qualificationPool.QueryRow(ctx, `SELECT count(*) FROM job_runs WHERE company_id=$1`, companyID).Scan(&jobsAfterUnverifiedStart); err != nil || jobsAfterUnverifiedStart != jobsBeforeUnverifiedStart {
		t.Fatalf("unverified service request created a JobRun: jobs=%d before=%d error=%v", jobsAfterUnverifiedStart, jobsBeforeUnverifiedStart, err)
	}
	service.SetProjectJobExecutor(jobExecutor)
	unknownService := serviceRequest
	unknownService.ServiceID = "not-pinned"
	unknownService.RequestID = "project-job-service-not-pinned"
	if _, err = service.StartProjectJob(ctx, companyID, unknownService); !errors.Is(err, core.Denied) || jobExecutor.launches != 2 {
		t.Fatalf("service outside the immutable environment policy was accepted: launches=%d error=%v", jobExecutor.launches, err)
	}
	serviceStarted, err := service.StartProjectJob(ctx, companyID, serviceRequest)
	if err != nil || serviceStarted.State != string(environment.JobRunning) || serviceStarted.Readiness != string(environment.ServiceReady) || serviceStarted.ServiceID != "web" || jobExecutor.launches != 3 {
		t.Fatalf("start pinned service JobRun=%+v launches=%d error=%v", serviceStarted, jobExecutor.launches, err)
	}
	otherServiceSelection := serviceRequest
	otherServiceSelection.ServiceID = "web-shadow"
	if _, err = service.StartProjectJob(ctx, companyID, otherServiceSelection); !errors.Is(err, core.Conflict) || jobExecutor.launches != 3 {
		t.Fatalf("same request id silently rebound to another pinned service: launches=%d error=%v", jobExecutor.launches, err)
	}
	var recordedServiceID string
	if err = qualificationPool.QueryRow(ctx, `SELECT service_id FROM job_runs WHERE company_id=$1 AND job_id=$2`, companyID, serviceStarted.JobID).Scan(&recordedServiceID); err != nil || recordedServiceID != "web" {
		t.Fatalf("JobRun service identity=%q error=%v", recordedServiceID, err)
	}
	time.Sleep(1200 * time.Millisecond)
	var serviceObservationCount int
	if err = qualificationPool.QueryRow(ctx, `SELECT count(*) FROM service_endpoint_events WHERE company_id=$1 AND job_id=$2`, companyID, serviceStarted.JobID).Scan(&serviceObservationCount); err != nil {
		t.Fatal(err)
	}
	if serviceObservationCount < 2 {
		t.Fatalf("service endpoint lease was not renewed: observations=%d", serviceObservationCount)
	}
	serviceStopped, err := service.StopProjectJob(ctx, companyID, serviceStarted.JobID, StopProjectJobRequest{RequestID: "project-job-service-stop"})
	if err != nil || serviceStopped.State != string(environment.JobCancelled) {
		t.Fatalf("stop service JobRun=%+v error=%v", serviceStopped, err)
	}
	var serviceEndpointReadiness string
	if err = qualificationPool.QueryRow(ctx, `SELECT readiness FROM service_endpoint_events WHERE company_id=$1 AND job_id=$2 ORDER BY generation DESC,event_seq DESC LIMIT 1`, companyID, serviceStarted.JobID).Scan(&serviceEndpointReadiness); err != nil || serviceEndpointReadiness != environment.ServiceRevoked {
		t.Fatalf("service stop endpoint readiness=%q error=%v", serviceEndpointReadiness, err)
	}
	restartedServiceJobID := "project-job-restart-terminal-revoke"
	if _, err = qualificationPool.Exec(ctx, `INSERT INTO job_runs(company_id,job_id,task_id,session_id,environment_revision_id,kind,service_id,source_revision_sha256,argv_sha256,working_directory_sha256,network_policy_sha256,environment_allowlist,timeout_ms,output_limit_bytes,request_id)
VALUES($1,$2,$3,$4,$5,'service','web',$6,$7,$8,$9,'[]'::jsonb,30000,65536,'project-job-restart-terminal-revoke')`, companyID, restartedServiceJobID, taskID, sessionID, revision.RevisionID, revision.SourceRevisionSHA256, strings.Repeat("1", 64), strings.Repeat("2", 64), strings.Repeat("3", 64)); err != nil {
		t.Fatal(err)
	}
	for _, event := range []struct{ id, state, readiness string }{
		{"project-job-restart-accepted", "accepted", string(environment.ServiceNotReady)},
		{"project-job-restart-starting", "starting", string(environment.ServiceNotReady)},
		{"project-job-restart-running", "running", string(environment.ServiceReady)},
		{"project-job-restart-exited", "exited", string(environment.ServiceReady)},
	} {
		if _, err = qualificationPool.Exec(ctx, `INSERT INTO job_run_events(company_id,event_id,job_id,state,readiness,reason_code) VALUES($1,$2,$3,$4,$5,'fixture')`, companyID, event.id, restartedServiceJobID, event.state, event.readiness); err != nil {
			t.Fatal(err)
		}
	}
	serviceHealthDigest, err := environment.ServiceProbeSpecSHA256(serviceProbe)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = qualificationPool.Exec(ctx, `INSERT INTO service_endpoint_events(company_id,event_id,job_id,generation,bind_address,port,readiness,source_revision_sha256,healthcheck_sha256,lease_expires_at)
VALUES($1,'project-job-restart-endpoint-ready',$2,1,$3,$4,'ready',$5,$6,clock_timestamp()+interval '1 minute')`, companyID, restartedServiceJobID, serviceProbe.BindAddress, int(serviceProbe.Port), revision.SourceRevisionSHA256, serviceHealthDigest); err != nil {
		t.Fatal(err)
	}
	reconciledServiceJob, err := service.StopProjectJob(ctx, companyID, restartedServiceJobID, StopProjectJobRequest{RequestID: "project-job-restart-terminal-revoke-retry"})
	if err != nil || reconciledServiceJob.State != string(environment.JobExited) {
		t.Fatalf("durable terminal endpoint-revocation retry job=%+v error=%v", reconciledServiceJob, err)
	}
	if err = qualificationPool.QueryRow(ctx, `SELECT readiness FROM service_endpoint_events WHERE company_id=$1 AND job_id=$2 ORDER BY event_seq DESC LIMIT 1`, companyID, restartedServiceJobID).Scan(&serviceEndpointReadiness); err != nil || serviceEndpointReadiness != environment.ServiceRevoked {
		t.Fatalf("durable terminal endpoint was not revoked: readiness=%q error=%v", serviceEndpointReadiness, err)
	}
	restartedUnknownJobID := "project-job-restarted-unknown-service"
	if _, err = qualificationPool.Exec(ctx, `INSERT INTO job_runs(company_id,job_id,task_id,session_id,environment_revision_id,kind,service_id,source_revision_sha256,argv_sha256,working_directory_sha256,network_policy_sha256,environment_allowlist,timeout_ms,output_limit_bytes,request_id)
VALUES($1,$2,$3,$4,$5,'service','web',$6,$7,$8,$9,'[]'::jsonb,30000,65536,'project-job-restarted-unknown-service')`, companyID, restartedUnknownJobID, taskID, sessionID, revision.RevisionID, revision.SourceRevisionSHA256, strings.Repeat("1", 64), strings.Repeat("2", 64), strings.Repeat("3", 64)); err != nil {
		t.Fatal(err)
	}
	for _, event := range []struct{ id, state, readiness string }{
		{"project-job-restarted-accepted", "accepted", string(environment.ServiceNotReady)},
		{"project-job-restarted-starting", "starting", string(environment.ServiceNotReady)},
		{"project-job-restarted-running", "running", string(environment.ServiceReady)},
	} {
		if _, err = qualificationPool.Exec(ctx, `INSERT INTO job_run_events(company_id,event_id,job_id,state,readiness,reason_code) VALUES($1,$2,$3,$4,$5,'fixture')`, companyID, event.id, restartedUnknownJobID, event.state, event.readiness); err != nil {
			t.Fatal(err)
		}
	}
	if _, err = qualificationPool.Exec(ctx, `INSERT INTO service_endpoint_events(company_id,event_id,job_id,generation,bind_address,port,readiness,source_revision_sha256,healthcheck_sha256,lease_expires_at)
VALUES($1,'project-job-restarted-unknown-endpoint',$2,1,$3,$4,'ready',$5,$6,clock_timestamp()+interval '1 minute')`, companyID, restartedUnknownJobID, serviceProbe.BindAddress, int(serviceProbe.Port), revision.SourceRevisionSHA256, serviceHealthDigest); err != nil {
		t.Fatal(err)
	}
	if _, err = runtime.TXRecordJobRunEvent(ctx, companyID, kernel.JobRunEventInput{JobID: restartedUnknownJobID, State: string(environment.JobOutcomeUnknown), ReasonCode: "job_process_not_restored", RequestID: "project-job-restarted-unknown-mark"}); err != nil {
		t.Fatal(err)
	}
	service.SetProjectJobExecutor(&recordingProjectJobExecutor{})
	reconciledRestartedService, err := service.StopProjectJob(ctx, companyID, restartedUnknownJobID, StopProjectJobRequest{RequestID: "project-job-restarted-unknown-stop"})
	if err != nil || reconciledRestartedService.State != string(environment.JobCancelled) {
		t.Fatalf("Windows restart reconciliation did not close unknown JobRun: %+v error=%v", reconciledRestartedService, err)
	}
	if err = qualificationPool.QueryRow(ctx, `SELECT readiness FROM service_endpoint_events WHERE company_id=$1 AND job_id=$2 ORDER BY event_seq DESC LIMIT 1`, companyID, restartedUnknownJobID).Scan(&serviceEndpointReadiness); err != nil || serviceEndpointReadiness != environment.ServiceRevoked {
		t.Fatalf("restarted unknown service endpoint was not revoked: readiness=%q error=%v", serviceEndpointReadiness, err)
	}
	uncertainExecutor := &recordingProjectJobExecutor{processes: []runner.AppContainerProcess{&uncertainStopProjectJobProcess{stopped: make(chan struct{})}}}
	service.SetProjectJobExecutor(uncertainExecutor)
	uncertainRequest := request
	uncertainRequest.RequestID = "project-job-uncertain-stop"
	uncertainStarted, err := service.StartProjectJob(ctx, companyID, uncertainRequest)
	if err != nil || uncertainStarted.State != string(environment.JobRunning) {
		t.Fatalf("uncertain-stop JobRun did not start: %+v error=%v", uncertainStarted, err)
	}
	unknown, err := service.StopProjectJob(ctx, companyID, uncertainStarted.JobID, StopProjectJobRequest{RequestID: "project-job-uncertain-stop-1"})
	if err != nil || unknown.State != string(environment.JobOutcomeUnknown) {
		t.Fatalf("unconfirmed stop state=%+v error=%v", unknown, err)
	}
	retriedStop, err := service.StopProjectJob(ctx, companyID, uncertainStarted.JobID, StopProjectJobRequest{RequestID: "project-job-uncertain-stop-2"})
	if err != nil || retriedStop.State != string(environment.JobCancelled) || uncertainExecutor.stopAttempts != 2 {
		t.Fatalf("retry stop state=%+v stop attempts=%d error=%v", retriedStop, uncertainExecutor.stopAttempts, err)
	}
	blockingExecutor := &blockedLaunchProjectJobExecutor{entered: make(chan struct{})}
	service.SetProjectJobExecutor(blockingExecutor)
	launchRequest := request
	launchRequest.RequestID = "project-job-stop-during-launch"
	launchResult := make(chan struct {
		job kernel.JobRunRecord
		err error
	}, 1)
	go func() {
		job, startErr := service.StartProjectJob(ctx, companyID, launchRequest)
		launchResult <- struct {
			job kernel.JobRunRecord
			err error
		}{job, startErr}
	}()
	select {
	case <-blockingExecutor.entered:
	case <-time.After(time.Second):
		t.Fatal("JobRun did not enter the blocked launch fixture")
	}
	startingJob := waitForJobState(t, ctx, qualificationPool, runtime, companyID, launchRequest.RequestID, environment.JobStarting)
	stoppedDuringLaunch, err := service.StopProjectJob(ctx, companyID, startingJob.JobID, StopProjectJobRequest{RequestID: "project-job-stop-before-handle"})
	if err != nil || stoppedDuringLaunch.State != string(environment.JobCancelled) {
		t.Fatalf("stop during launch=%+v error=%v", stoppedDuringLaunch, err)
	}
	select {
	case result := <-launchResult:
		if result.err != nil || result.job.State != string(environment.JobCancelled) {
			t.Fatalf("launch after cancellation=%+v error=%v", result.job, result.err)
		}
	case <-time.After(time.Second):
		t.Fatal("cancelled launch did not drain its control operation")
	}

	otherTaskID, otherSessionID := "", "project-job-bootstrap-session"
	if err = qualificationPool.QueryRow(ctx, `SELECT id FROM tasks WHERE company_id=$1 AND mission_id=$2 AND kind='bootstrap_plan'`, companyID, missionID).Scan(&otherTaskID); err != nil {
		t.Fatal(err)
	}
	_, err = qualificationPool.Exec(ctx, `INSERT INTO worker_sessions(company_id,id,employee_id,task_id,generation,epoch,incarnation,profile,state) VALUES($1,$2,$3,$4,1,1,'offline-fixture','offline-fixture','active')`, companyID, otherSessionID, core.EmployeePlanningID, otherTaskID)
	if err != nil {
		t.Fatal(err)
	}
	_, err = runtime.TXCreateJobRun(ctx, companyID, kernel.JobRunInput{
		TaskID: otherTaskID, SessionID: otherSessionID, EnvironmentRevisionID: revision.RevisionID, Kind: "batch",
		SourceRevisionSHA256: revision.SourceRevisionSHA256, ArgvSHA256: strings.Repeat("1", 64),
		WorkingDirectorySHA256: strings.Repeat("2", 64), NetworkPolicySHA256: strings.Repeat("3", 64),
		TimeoutMS: 60_000, OutputLimitBytes: 4096, RequestID: "project-job-noncompat-task",
	})
	if !errors.Is(err, core.Denied) {
		t.Fatalf("non-compat Task JobRun error=%v, want %s", err, core.Denied)
	}
	seeded, err := runtime.TXCreateJobRun(ctx, companyID, kernel.JobRunInput{
		TaskID: taskID, SessionID: sessionID, EnvironmentRevisionID: revision.RevisionID, Kind: "batch",
		SourceRevisionSHA256: revision.SourceRevisionSHA256, ArgvSHA256: strings.Repeat("1", 64),
		WorkingDirectorySHA256: strings.Repeat("2", 64), NetworkPolicySHA256: strings.Repeat("3", 64),
		TimeoutMS: 60_000, OutputLimitBytes: 4096, RequestID: "project-job-recovery-seed",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = runtime.TXRecordJobRunEvent(ctx, companyID, kernel.JobRunEventInput{JobID: seeded.ID, State: string(environment.JobStarting), Readiness: "not_applicable", RequestID: "project-job-recovery-starting"}); err != nil {
		t.Fatal(err)
	}
	if _, err = runtime.TXRecordJobRunEvent(ctx, companyID, kernel.JobRunEventInput{JobID: seeded.ID, State: string(environment.JobRunning), Readiness: "not_applicable", RequestID: "project-job-recovery-running"}); err != nil {
		t.Fatal(err)
	}
	marked, err := runtime.TXMarkUnrestoredJobRunsUnknown(ctx, "project-job-recovery-1")
	if err != nil || marked != 1 {
		t.Fatalf("restart recovery marked=%d error=%v, want one running job", marked, err)
	}
	recovered, err := runtime.GetJobRun(ctx, companyID, seeded.ID)
	if err != nil || recovered.State != string(environment.JobOutcomeUnknown) {
		t.Fatalf("restart recovery state=%+v error=%v", recovered, err)
	}
}

func waitForJobState(t *testing.T, ctx context.Context, pool *pgxpool.Pool, runtime *kernel.Kernel, companyID, requestID string, state environment.JobState) kernel.JobRunRecord {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		var jobID string
		err := pool.QueryRow(ctx, `SELECT job_id FROM job_runs WHERE company_id=$1 AND request_id=$2`, companyID, requestID).Scan(&jobID)
		if err == nil {
			job, readErr := runtime.GetJobRun(ctx, companyID, jobID)
			if readErr == nil && job.State == string(state) {
				return job
			}
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("JobRun request %q did not reach %q", requestID, state)
	return kernel.JobRunRecord{}
}

type recordingProjectJobExecutor struct {
	launches     int
	stopAttempts int
	processes    []runner.AppContainerProcess
}

type unverifiedServiceProjectJobExecutor struct{ executor *recordingProjectJobExecutor }

func (executor *unverifiedServiceProjectJobExecutor) HasPreparedEnvironment(revisionID string) bool {
	return executor.executor.HasPreparedEnvironment(revisionID)
}
func (executor *unverifiedServiceProjectJobExecutor) SupportsProjectJobProfile(profileID string) bool {
	return executor.executor.SupportsProjectJobProfile(profileID)
}
func (executor *unverifiedServiceProjectJobExecutor) LaunchProjectJob(ctx context.Context, request ProjectJobExecutionRequest) (runner.AppContainerProcess, error) {
	return executor.executor.LaunchProjectJob(ctx, request)
}
func (executor *unverifiedServiceProjectJobExecutor) StopProjectJob(ctx context.Context, companyID, jobID string) error {
	return executor.executor.StopProjectJob(ctx, companyID, jobID)
}
func (executor *unverifiedServiceProjectJobExecutor) ForgetProjectJob(jobID string) {
	executor.executor.ForgetProjectJob(jobID)
}

type blockedLaunchProjectJobExecutor struct{ entered chan struct{} }

func (*blockedLaunchProjectJobExecutor) HasPreparedEnvironment(string) bool { return true }
func (*blockedLaunchProjectJobExecutor) SupportsProjectJobProfile(profileID string) bool {
	return profileID == environment.WindowsNodeNPMProfile
}
func (executor *blockedLaunchProjectJobExecutor) LaunchProjectJob(ctx context.Context, _ ProjectJobExecutionRequest) (runner.AppContainerProcess, error) {
	close(executor.entered)
	<-ctx.Done()
	return nil, ctx.Err()
}
func (*blockedLaunchProjectJobExecutor) StopProjectJob(context.Context, string, string) error {
	return nil
}
func (*blockedLaunchProjectJobExecutor) ForgetProjectJob(string) {}

func (*recordingProjectJobExecutor) HasPreparedEnvironment(string) bool { return true }
func (*recordingProjectJobExecutor) SupportsProjectJobProfile(profileID string) bool {
	return profileID == environment.WindowsNodeNPMProfile
}
func (executor *recordingProjectJobExecutor) LaunchProjectJob(context.Context, ProjectJobExecutionRequest) (runner.AppContainerProcess, error) {
	process := executor.processes[executor.launches]
	executor.launches++
	return process, nil
}
func (executor *recordingProjectJobExecutor) StopProjectJob(_ context.Context, _ string, jobID string) error {
	if executor.launches > 0 {
		if process, ok := executor.processes[executor.launches-1].(*cancelledProjectJobProcess); ok {
			process.stop()
		}
		if process, ok := executor.processes[executor.launches-1].(*uncertainStopProjectJobProcess); ok {
			executor.stopAttempts++
			if executor.stopAttempts == 1 {
				return errors.New("fixture stop was not confirmed")
			}
			process.stop()
		}
	}
	return nil
}
func (*recordingProjectJobExecutor) ForgetProjectJob(string) {}

func (*recordingProjectJobExecutor) ReconcileUnrestoredProjectJob(companyID, jobID, revisionID string) error {
	if !core.ValidID(companyID) || !core.ValidID(jobID) || !core.ValidID(revisionID) {
		return core.Malformed
	}
	return nil
}

func (*recordingProjectJobExecutor) VerifyServiceEndpointOwner(ctx context.Context, processID int, bindAddress string, port uint16) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if processID <= 0 || bindAddress != "127.0.0.1" || port == 0 {
		return environment.ErrServiceEndpointOwnerUnverified
	}
	return nil
}

func (*recordingProjectJobExecutor) VerifyServiceEndpointConnectionOwner(ctx context.Context, processID int, bindAddress string, port uint16, clientAddress string, clientPort uint16) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if processID <= 0 || bindAddress != "127.0.0.1" || port == 0 || clientPort == 0 || net.ParseIP(clientAddress) == nil || !net.ParseIP(clientAddress).IsLoopback() {
		return environment.ErrServiceEndpointOwnerUnverified
	}
	return nil
}

type finishedProjectJobProcess struct {
	stdout []byte
	stderr []byte
}

func (*finishedProjectJobProcess) PID() int              { return 1 }
func (*finishedProjectJobProcess) Stdin() io.WriteCloser { return nil }
func (process *finishedProjectJobProcess) Stdout() io.ReadCloser {
	return io.NopCloser(bytes.NewReader(process.stdout))
}
func (process *finishedProjectJobProcess) Stderr() io.ReadCloser {
	return io.NopCloser(bytes.NewReader(process.stderr))
}
func (*finishedProjectJobProcess) Wait(context.Context) (int, error) { return 0, nil }
func (*finishedProjectJobProcess) Stop() (runner.StopProof, error)   { return runner.StopProof{}, nil }

type cancelledProjectJobProcess struct {
	stopped chan struct{}
	once    sync.Once
}

func (*cancelledProjectJobProcess) PID() int              { return 2 }
func (*cancelledProjectJobProcess) Stdin() io.WriteCloser { return nil }
func (*cancelledProjectJobProcess) Stdout() io.ReadCloser { return nil }
func (*cancelledProjectJobProcess) Stderr() io.ReadCloser { return nil }
func (process *cancelledProjectJobProcess) Wait(ctx context.Context) (int, error) {
	select {
	case <-process.stopped:
		return 0, context.Canceled
	case <-ctx.Done():
		return 0, ctx.Err()
	}
}
func (process *cancelledProjectJobProcess) Stop() (runner.StopProof, error) {
	process.stop()
	return runner.StopProof{}, nil
}
func (process *cancelledProjectJobProcess) stop() { process.once.Do(func() { close(process.stopped) }) }

type uncertainStopProjectJobProcess struct {
	stopped chan struct{}
	once    sync.Once
}

func (*uncertainStopProjectJobProcess) PID() int              { return 3 }
func (*uncertainStopProjectJobProcess) Stdin() io.WriteCloser { return nil }
func (*uncertainStopProjectJobProcess) Stdout() io.ReadCloser { return nil }
func (*uncertainStopProjectJobProcess) Stderr() io.ReadCloser { return nil }
func (process *uncertainStopProjectJobProcess) Wait(ctx context.Context) (int, error) {
	select {
	case <-process.stopped:
		return 0, nil
	case <-ctx.Done():
		return 0, errors.Join(ctx.Err(), runner.ErrAppContainerProcessStopUnconfirmed)
	}
}
func (process *uncertainStopProjectJobProcess) Stop() (runner.StopProof, error) {
	process.stop()
	return runner.StopProof{}, nil
}
func (process *uncertainStopProjectJobProcess) stop() {
	process.once.Do(func() { close(process.stopped) })
}

// pattern: Imperative Shell
package workbench_test

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"polis/internal/core"
	"polis/internal/environment"
	"polis/internal/intake"
	"polis/internal/kernel"
	"polis/internal/taskvalidation"
	"polis/internal/workbench"
)

func TestEnvironmentReadStoreShowsPolicyAndExecutorQualificationGates(t *testing.T) {
	dsn := os.Getenv("POLIS_TEST_DSN")
	if dsn == "" {
		t.Skip("dedicated PostgreSQL required")
	}
	ctx := context.Background()
	root := t.TempDir()
	runtime, err := kernel.Open(ctx, dsn, root)
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Close()
	companyID := fmt.Sprintf("environment-read-%d", time.Now().UnixNano())
	scope, err := runtime.TXCreateCompany(ctx, companyID)
	if err != nil {
		t.Fatal(err)
	}
	missionID := fmt.Sprintf("environment-read-mission-%d", time.Now().UnixNano())
	if err = runtime.TXCreateMission(ctx, scope, missionID); err != nil {
		t.Fatal(err)
	}
	directory, err := intake.PrepareDirectorySnapshot([]intake.DirectoryInputFile{
		{RelativePath: "repo/package.json", MediaType: "application/json", Content: []byte(`{"name":"demo","version":"1.0.0"}`)},
		{RelativePath: "repo/package-lock.json", MediaType: "application/json", Content: []byte(`{"name":"demo","version":"1.0.0","lockfileVersion":3,"packages":{"":{"name":"demo","version":"1.0.0"}}}`)},
		{RelativePath: "repo/ui/server.mjs", MediaType: "text/javascript", Content: []byte("process.stdout.write('service')")},
	})
	if err != nil {
		t.Fatal(err)
	}
	source, err := runtime.TXAddMissionInput(ctx, scope, missionID, "", "environment-read-source", directory.Upload, directory.Archive)
	if err != nil {
		t.Fatal(err)
	}
	policy, _, _, err := environment.BuildWindowsNodeEnvironmentPolicy([]string{"registry.npmjs.org"}, 180_000, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	probeSpec := environment.ServiceProbeSpec{BindAddress: "127.0.0.1", Port: 3000, Path: "/healthz", ExpectedStatusCode: 200, ExpectedBodySHA256: fmt.Sprintf("%x", sha256.Sum256([]byte("ready"))), TimeoutMS: 500, LeaseDurationMS: 30_000}
	shadowProbe := probeSpec
	shadowProbe.Port++
	policy.Services = []environment.ProjectServiceDefinition{
		{ID: "web", ScriptPath: "ui/server.mjs", Probe: probeSpec},
		{ID: "web-shadow", ScriptPath: "ui/server.mjs", Probe: shadowProbe},
	}
	_, policyJSON, _, err := environment.CanonicalizeWindowsNodeEnvironmentPolicy(policy)
	if err != nil {
		t.Fatal(err)
	}
	revision, err := runtime.TXRegisterProjectEnvironmentRevision(ctx, companyID, kernel.ProjectEnvironmentRevisionInput{
		ProfileID: environment.WindowsNodeNPMProfile, MissionID: missionID, SourceInputID: source.InputID, SourceInputRevision: source.Revision,
		PolicyManifest: json.RawMessage(policyJSON), ToolchainSHA256: strings.Repeat("e", 64),
	}, "environment-read-register")
	if err != nil {
		t.Fatal(err)
	}
	if revision.SourceRevisionSHA256 != source.ContentDigest || revision.ProjectRootRelative != "repo" {
		t.Fatalf("environment row did not retain source CAS binding: %+v", revision)
	}
	_, linuxPolicyJSON, _, err := environment.BuildLinuxNodeEnvironmentPolicy([]string{"registry.npmjs.org"}, 180_000, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	linuxRevision, err := runtime.TXRegisterProjectEnvironmentRevision(ctx, companyID, kernel.ProjectEnvironmentRevisionInput{
		ProfileID: environment.LinuxNodeNPMProfile, MissionID: missionID, SourceInputID: source.InputID, SourceInputRevision: source.Revision,
		PolicyManifest: json.RawMessage(linuxPolicyJSON), ToolchainSHA256: strings.Repeat("d", 64),
	}, "environment-read-linux-register")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = runtime.TXRecordEnvironmentPolicyDecision(ctx, companyID, kernel.EnvironmentPolicyDecisionInput{
		RevisionID: linuxRevision.RevisionID, Decision: "approved", Rationale: "offline Linux profile under separate qualification", RequestID: "environment-read-linux-approve",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err = runtime.TXRequestEnvironmentPreparation(ctx, companyID, linuxRevision.RevisionID, "environment-read-linux-unqualified"); err != nil {
		t.Fatal(err)
	}
	if _, err = runtime.TXRequestEnvironmentPreparation(ctx, companyID, revision.RevisionID, "environment-read-policy-block"); err != nil {
		t.Fatal(err)
	}
	if _, err = runtime.TXRecordEnvironmentPolicyDecision(ctx, companyID, kernel.EnvironmentPolicyDecisionInput{
		RevisionID: revision.RevisionID, Decision: "approved", Rationale: "reviewed lockfile and registry policy", RequestID: "environment-read-approve",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err = runtime.TXRequestEnvironmentPreparation(ctx, companyID, revision.RevisionID, "environment-read-executor-block"); err != nil {
		t.Fatal(err)
	}
	mission, err := runtime.TXCreateMissionGoalWithAcceptance(ctx, scope, "JobRun view", "seed a scoped read-only JobRun projection", &taskvalidation.AcceptanceContract{
		Revision: taskvalidation.AcceptanceContractRevision, RequiredText: []string{"Mission ID: {{mission_id}}"},
	}, "environment-job-view-mission")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = runtime.TXStartMissionCommand(ctx, scope, mission.ID, "environment-job-view-start"); err != nil {
		t.Fatal(err)
	}
	task, err := runtime.TXPrepareProductTask(ctx, scope, mission.ID, "seed job view", "environment-job-view-task")
	if err != nil {
		t.Fatal(err)
	}
	binding, err := runtime.TXNewProductProviderWorkerWithToolBudget(ctx, scope, task.ID, "offline-model/medium", 16)
	if err != nil {
		t.Fatal(err)
	}
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if _, err = pool.Exec(ctx, `INSERT INTO job_runs(company_id,job_id,task_id,session_id,environment_revision_id,kind,service_id,source_revision_sha256,argv_sha256,working_directory_sha256,network_policy_sha256,environment_allowlist,timeout_ms,output_limit_bytes,request_id)
VALUES($1,'job-view-1',$2,$3,$4,'service','web',$5,$6,$7,$8,'["PATH","SYSTEMROOT"]'::jsonb,30000,65536,'environment-job-view-register')`, companyID, task.ID, binding.SessionID(), revision.RevisionID, revision.SourceRevisionSHA256, strings.Repeat("1", 64), strings.Repeat("2", 64), strings.Repeat("3", 64)); err != nil {
		t.Fatal(err)
	}
	for _, event := range []struct{ id, state, readiness, reason string }{
		{"job-view-accepted", "accepted", "not_ready", "job_accepted"},
		{"job-view-starting", "starting", "not_ready", "process_starting"},
		{"job-view-running", "running", "not_ready", "process_running"},
		{"job-view-ready", "running", "ready", "health_probe_passed"},
	} {
		if _, err = pool.Exec(ctx, `INSERT INTO job_run_events(company_id,event_id,job_id,state,readiness,reason_code) VALUES($1,$2,'job-view-1',$3,$4,$5)`, companyID, event.id, event.state, event.readiness, event.reason); err != nil {
			t.Fatal(err)
		}
	}
	if _, err = runtime.TXRecordJobRunEvent(ctx, companyID, kernel.JobRunEventInput{
		JobID: "job-view-1", State: string(environment.JobRunning), Readiness: string(environment.ServiceReady), RequestID: "service-view-readiness-bypass",
	}); !errors.Is(err, core.Denied) {
		t.Fatalf("generic JobRun event marked a service ready without an endpoint observation: %v", err)
	}
	healthcheckDigest, err := environment.ServiceProbeSpecSHA256(probeSpec)
	if err != nil {
		t.Fatal(err)
	}
	probedAt := time.Now().UTC()
	leaseExpiresAt := probedAt.Add(30 * time.Second)
	readyEvent := kernel.ServiceEndpointEventInput{JobID: "job-view-1", Generation: 1, BindAddress: probeSpec.BindAddress, Port: probeSpec.Port, Readiness: string(environment.ServiceReady), SourceRevisionSHA256: revision.SourceRevisionSHA256, HealthcheckSHA256: healthcheckDigest, ProbedAt: probedAt, LeaseExpiresAt: &leaseExpiresAt, RequestID: "service-view-ready"}
	wrongSource := readyEvent
	wrongSource.SourceRevisionSHA256 = strings.Repeat("f", 64)
	wrongSource.RequestID = "service-view-wrong-source"
	if _, err = runtime.TXRecordServiceEndpointEvent(ctx, companyID, wrongSource); !errors.Is(err, core.Integrity) {
		t.Fatalf("endpoint event with a different source revision error=%v, want %s", err, core.Integrity)
	}
	wrongLease := readyEvent
	wrongLeaseExpiry := probedAt.Add(31 * time.Second)
	wrongLease.LeaseExpiresAt = &wrongLeaseExpiry
	wrongLease.RequestID = "service-view-wrong-lease"
	if _, err = runtime.TXRecordServiceEndpointEvent(ctx, companyID, wrongLease); !errors.Is(err, core.Malformed) {
		t.Fatalf("endpoint event with a lease outside the pinned duration error=%v, want %s", err, core.Malformed)
	}
	wrongEndpoint := readyEvent
	wrongEndpoint.Port = 3001
	wrongEndpoint.RequestID = "service-view-wrong-endpoint"
	if _, err = runtime.TXRecordServiceEndpointEvent(ctx, companyID, wrongEndpoint); !errors.Is(err, core.Denied) {
		t.Fatalf("endpoint event for an unpinned port error=%v, want %s", err, core.Denied)
	}
	shadowDigest, err := environment.ServiceProbeSpecSHA256(shadowProbe)
	if err != nil {
		t.Fatal(err)
	}
	otherServiceEndpoint := readyEvent
	otherServiceEndpoint.Port = shadowProbe.Port
	otherServiceEndpoint.HealthcheckSHA256 = shadowDigest
	otherServiceEndpoint.ProbedAt = probedAt.Add(time.Second)
	shadowExpiry := otherServiceEndpoint.ProbedAt.Add(time.Duration(shadowProbe.LeaseDurationMS) * time.Millisecond)
	otherServiceEndpoint.LeaseExpiresAt = &shadowExpiry
	otherServiceEndpoint.RequestID = "service-view-wrong-service-identity"
	if _, err = runtime.TXRecordServiceEndpointEvent(ctx, companyID, otherServiceEndpoint); !errors.Is(err, core.Denied) {
		t.Fatalf("service endpoint event crossed the JobRun's pinned service id: %v", err)
	}
	readyReceipt, err := runtime.TXRecordServiceEndpointEvent(ctx, companyID, readyEvent)
	if err != nil {
		t.Fatalf("persist verified service endpoint: %v", err)
	}
	staleProbe := readyEvent
	staleProbe.ProbedAt = probedAt.Add(-2 * time.Second)
	staleLeaseExpiry := staleProbe.ProbedAt.Add(30 * time.Second)
	staleProbe.LeaseExpiresAt = &staleLeaseExpiry
	staleProbe.RequestID = "service-view-stale-probe"
	if _, err = runtime.TXRecordServiceEndpointEvent(ctx, companyID, staleProbe); !errors.Is(err, core.Conflict) {
		t.Fatalf("stale service probe replaced a newer observation: %v", err)
	}
	readyReplay, err := runtime.TXRecordServiceEndpointEvent(ctx, companyID, readyEvent)
	if err != nil || readyReplay.Status != string(environment.ServiceReady) || readyReplay.ID != readyReceipt.ID {
		t.Fatalf("service endpoint receipt replay=%+v error=%v", readyReplay, err)
	}
	changedGeneration := readyEvent
	changedGeneration.Generation = 2
	changedGeneration.RequestID = "service-view-premature-generation"
	if _, err = runtime.TXRecordServiceEndpointEvent(ctx, companyID, changedGeneration); !errors.Is(err, core.Conflict) {
		t.Fatalf("unrevoked endpoint generation change error=%v, want %s", err, core.Conflict)
	}
	store, err := workbench.NewPostgresReadStoreWithBlobRoot(ctx, dsn, root)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	environments, err := store.ListProjectEnvironments(ctx, companyID)
	if err != nil {
		t.Fatal(err)
	}
	if len(environments) != 2 {
		t.Fatalf("environment lifecycle projection count = %d, want Windows and Linux profiles: %+v", len(environments), environments)
	}
	var windowsProjected, linuxProjected bool
	for _, environmentView := range environments {
		if environmentView.RevisionID == revision.RevisionID {
			windowsProjected = environmentView.SourceBindingStatus == "bound" && environmentView.MissionID == missionID && environmentView.SourceInputID == source.InputID && environmentView.ProjectRootRelative == "repo" && environmentView.PolicyDecision == "approved" && environmentView.ExecutorQualification == "unqualified" && environmentView.PreparationState == string(environment.PreparationBlockedUnqualified)
		}
		if environmentView.RevisionID == linuxRevision.RevisionID {
			linuxPolicy, _, _, policyErr := environment.ParseLinuxNodeEnvironmentPolicy(environmentView.PolicyManifest)
			linuxProjected = policyErr == nil && environmentView.ProfileID == environment.LinuxNodeNPMProfile && linuxPolicy.NetworkPolicy == "deny_all" && environmentView.PolicyDecision == "approved" && environmentView.ExecutorQualification == "unqualified" && environmentView.PreparationState == string(environment.PreparationBlockedUnqualified)
		}
	}
	if !windowsProjected || !linuxProjected {
		t.Fatalf("environment lifecycle projection = %+v", environments)
	}
	jobs, err := store.ListTaskJobRuns(ctx, companyID, task.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(jobs) != 1 || jobs[0].JobID != "job-view-1" || jobs[0].State != "running" || jobs[0].Readiness != "ready" || jobs[0].ServiceEndpoint == nil || jobs[0].ServiceEndpoint.Port != "3000" {
		t.Fatalf("JobRun and ServiceEndpoint projection = %+v", jobs)
	}
	handovers, err := store.ListTaskCrossBackendHandovers(ctx, companyID, task.ID)
	if err != nil || len(handovers) != 0 {
		t.Fatalf("empty cross-backend handover projection = %+v error=%v", handovers, err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO service_endpoint_events(company_id,event_id,job_id,generation,bind_address,port,readiness,source_revision_sha256,healthcheck_sha256,lease_expires_at)
VALUES($1,'service-view-expired','job-view-1',1,'127.0.0.1',3000,'ready',$2,$3,clock_timestamp()-interval '1 second')`, companyID, revision.SourceRevisionSHA256, healthcheckDigest); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO service_endpoint_events(company_id,event_id,job_id,generation,bind_address,port,readiness,source_revision_sha256,healthcheck_sha256,lease_expires_at)
VALUES($1,'service-view-revoked-with-lease','job-view-1',2,'127.0.0.1',3000,'revoked',$2,$3,clock_timestamp()+interval '1 minute')`, companyID, revision.SourceRevisionSHA256, healthcheckDigest); err == nil {
		t.Fatal("Schema 33 accepted a revoked endpoint with a live lease")
	}
	jobs, err = store.ListTaskJobRuns(ctx, companyID, task.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(jobs) != 1 || jobs[0].Readiness != "unhealthy" || jobs[0].ServiceEndpoint == nil || jobs[0].ServiceEndpoint.Readiness != "unhealthy" {
		t.Fatalf("expired service lease was projected as ready: %+v", jobs)
	}
	freshProbe := readyEvent
	freshProbe.ProbedAt = time.Now().UTC()
	freshLease := freshProbe.ProbedAt.Add(30 * time.Second)
	freshProbe.LeaseExpiresAt = &freshLease
	freshProbe.RequestID = "service-view-ready-before-policy-revoke"
	if _, err = runtime.TXRecordServiceEndpointEvent(ctx, companyID, freshProbe); err != nil {
		t.Fatalf("renew endpoint before policy revocation: %v", err)
	}
	if _, err = runtime.TXRecordEnvironmentPolicyDecision(ctx, companyID, kernel.EnvironmentPolicyDecisionInput{
		RevisionID: revision.RevisionID, Decision: "revoked", Rationale: "revoke the reviewed environment before the next service lease", RequestID: "service-view-policy-revoke",
	}); err != nil {
		t.Fatalf("revoke approved environment policy: %v", err)
	}
	jobs, err = store.ListTaskJobRuns(ctx, companyID, task.ID)
	if err != nil || len(jobs) != 1 || jobs[0].Readiness != "unhealthy" || jobs[0].ServiceEndpoint == nil || jobs[0].ServiceEndpoint.Readiness != "unhealthy" {
		t.Fatalf("revoked policy left an unexpired service endpoint ready: %+v error=%v", jobs, err)
	}
	renewal := freshProbe
	renewal.ProbedAt = time.Now().UTC()
	renewalLease := renewal.ProbedAt.Add(30 * time.Second)
	renewal.LeaseExpiresAt = &renewalLease
	renewal.RequestID = "service-view-renew-after-policy-revoke"
	if _, err = runtime.TXRecordServiceEndpointEvent(ctx, companyID, renewal); !errors.Is(err, core.Denied) {
		t.Fatalf("endpoint lease renewed after environment policy revocation: %v", err)
	}
	if _, err = runtime.TXRecordServiceEndpointEvent(ctx, companyID, kernel.ServiceEndpointEventInput{
		JobID: readyEvent.JobID, Generation: readyEvent.Generation, BindAddress: readyEvent.BindAddress, Port: readyEvent.Port,
		Readiness: "revoked", SourceRevisionSHA256: readyEvent.SourceRevisionSHA256, HealthcheckSHA256: readyEvent.HealthcheckSHA256,
		ProbedAt:  time.Now().UTC(),
		RequestID: "service-view-revoke",
	}); err != nil {
		t.Fatalf("persist endpoint revocation: %v", err)
	}
	jobs, err = store.ListTaskJobRuns(ctx, companyID, task.ID)
	if err != nil || len(jobs) != 1 || jobs[0].Readiness != "unhealthy" || jobs[0].ServiceEndpoint == nil || jobs[0].ServiceEndpoint.Readiness != "revoked" {
		t.Fatalf("revoked service lease projection=%+v error=%v", jobs, err)
	}
	revival := renewal
	revival.Generation = 2
	revival.RequestID = "service-view-new-generation-after-policy-revoke"
	if _, err = runtime.TXRecordServiceEndpointEvent(ctx, companyID, revival); !errors.Is(err, core.Denied) {
		t.Fatalf("new endpoint generation accepted after environment policy revocation: %v", err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO job_runs(company_id,job_id,task_id,session_id,environment_revision_id,kind,service_id,source_revision_sha256,argv_sha256,working_directory_sha256,network_policy_sha256,environment_allowlist,timeout_ms,output_limit_bytes,request_id)
VALUES($1,'job-view-terminal',$2,$3,$4,'service','web',$5,$6,$7,$8,'["PATH","SYSTEMROOT"]'::jsonb,30000,65536,'environment-job-view-terminal-register')`, companyID, task.ID, binding.SessionID(), revision.RevisionID, revision.SourceRevisionSHA256, strings.Repeat("1", 64), strings.Repeat("2", 64), strings.Repeat("3", 64)); err != nil {
		t.Fatalf("seed terminal service JobRun: %v", err)
	}
	for _, event := range []struct{ id, state, readiness string }{
		{"job-view-terminal-accepted", "accepted", "not_ready"},
		{"job-view-terminal-starting", "starting", "not_ready"},
		{"job-view-terminal-running", "running", "not_ready"},
		{"job-view-terminal-ready", "running", "ready"},
	} {
		if _, err = pool.Exec(ctx, `INSERT INTO job_run_events(company_id,event_id,job_id,state,readiness,reason_code) VALUES($1,$2,'job-view-terminal',$3,$4,'fixture')`, companyID, event.id, event.state, event.readiness); err != nil {
			t.Fatalf("seed terminal service JobRun event: %v", err)
		}
	}
	if _, err = pool.Exec(ctx, `INSERT INTO service_endpoint_events(company_id,event_id,job_id,generation,bind_address,port,readiness,source_revision_sha256,healthcheck_sha256,lease_expires_at)
VALUES($1,'service-view-terminal-ready','job-view-terminal',1,'127.0.0.1',3000,'ready',$2,$3,clock_timestamp()+interval '5 minutes')`, companyID, revision.SourceRevisionSHA256, healthcheckDigest); err != nil {
		t.Fatalf("seed terminal service endpoint: %v", err)
	}
	exitCode := 0
	if _, err = runtime.TXRecordJobRunEvent(ctx, companyID, kernel.JobRunEventInput{JobID: "job-view-terminal", State: string(environment.JobExited), ExitCode: &exitCode, RequestID: "job-view-terminal-exit"}); err != nil {
		t.Fatalf("record terminal service JobRun event: %v", err)
	}
	jobs, err = store.ListTaskJobRuns(ctx, companyID, task.ID)
	if err != nil {
		t.Fatal(err)
	}
	var terminalProjected bool
	for _, job := range jobs {
		if job.JobID == "job-view-terminal" {
			terminalProjected = job.State == string(environment.JobExited) && job.Readiness == "unhealthy" && job.ServiceEndpoint != nil && job.ServiceEndpoint.Readiness == "unhealthy"
		}
	}
	if !terminalProjected {
		t.Fatalf("terminal service JobRun retained a ready endpoint lease: %+v", jobs)
	}
	if _, err = runtime.TXRecordJobRunEvent(ctx, companyID, kernel.JobRunEventInput{
		JobID: "job-view-1", State: string(environment.JobCancelled), RequestID: "service-view-job-terminal",
	}); err != nil {
		t.Fatalf("close service JobRun projection fixture: %v", err)
	}
	if err = runtime.TXFinalizeWorkerBeforeProcess(ctx, binding, "JobRun read projection fixture complete", "environment-job-view-finalize"); err != nil {
		t.Fatal(err)
	}
}

func TestLegacyEnvironmentPolicyWithoutManifestIsUnverifiedAndCanBeRevoked(t *testing.T) {
	dsn := os.Getenv("POLIS_TEST_DSN")
	if dsn == "" {
		t.Skip("dedicated PostgreSQL required")
	}
	ctx := context.Background()
	root := t.TempDir()
	runtime, err := kernel.Open(ctx, dsn, root)
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Close()
	companyID := fmt.Sprintf("legacy-environment-%d", time.Now().UnixNano())
	if _, err = runtime.TXCreateCompany(ctx, companyID); err != nil {
		t.Fatal(err)
	}
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	revisionID := "legacy-environment-1"
	policyDigest := strings.Repeat("d", 64)
	if _, err = pool.Exec(ctx, `INSERT INTO project_environment_revisions(company_id,revision_id,profile_id,source_revision_sha256,package_json_sha256,lockfile_sha256,policy_sha256,policy_manifest,toolchain_sha256,created_by)
VALUES($1,$2,$3,$4,$5,$6,$7,NULL,$8,'legacy-fixture')`, companyID, revisionID, environment.WindowsNodeNPMProfile, strings.Repeat("a", 64), strings.Repeat("b", 64), strings.Repeat("c", 64), policyDigest, strings.Repeat("e", 64)); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO environment_policy_events(company_id,event_id,revision_id,policy_sha256,decision,rationale,actor,request_id)
VALUES($1,'legacy-env-policy-approved',$2,$3,'approved','legacy approval lacks the reviewed policy body','legacy-import','legacy-env-policy-request')`, companyID, revisionID, policyDigest); err != nil {
		t.Fatal(err)
	}
	run, err := runtime.TXRequestEnvironmentPreparation(ctx, companyID, revisionID, "legacy-env-ensure")
	if err != nil {
		t.Fatal(err)
	}
	if run.State != string(environment.PreparationBlockedSourceUnverified) || run.ReasonCode != "environment_source_binding_unverified" {
		t.Fatalf("legacy preparation was not blocked: %+v", run)
	}
	store, err := workbench.NewPostgresReadStoreWithBlobRoot(ctx, dsn, root)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	environments, err := store.ListProjectEnvironments(ctx, companyID)
	if err != nil {
		t.Fatal(err)
	}
	if len(environments) != 1 || environments[0].SourceBindingStatus != "unverified" || environments[0].PolicyDecision != "revocation_required" || environments[0].PolicyManifest != nil {
		t.Fatalf("legacy environment policy projection = %+v", environments)
	}
	if _, err = runtime.TXRecordEnvironmentPolicyDecision(ctx, companyID, kernel.EnvironmentPolicyDecisionInput{
		RevisionID: revisionID, Decision: "revoked", Rationale: "revoke legacy approval without a verifiable policy body", RequestID: "legacy-env-revoke",
	}); err != nil {
		t.Fatal(err)
	}
	environments, err = store.ListProjectEnvironments(ctx, companyID)
	if err != nil {
		t.Fatal(err)
	}
	if len(environments) != 1 || environments[0].PolicyDecision != "revoked" {
		t.Fatalf("legacy policy revocation projection = %+v", environments)
	}
}

// pattern: Imperative Shell
package kernel

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"polis/internal/core"
	"polis/internal/fixture"
	"polis/internal/gotestprofile"
	"polis/internal/gotestsupervisor"
	"polis/internal/intake"
	"polis/internal/pgfixture"
	"polis/internal/runner"
	"polis/internal/taskvalidation"
	"strings"
	"testing"
)

func TestGoJobPersistsRealHostGateAndSimulatedStartFailure(t *testing.T) {
	dsn := pgfixture.DSN(t, "POLIS_TEST_DSN")
	if dsn == "" {
		t.Skip("dedicated PG18 harness required")
	}
	ctx := context.Background()
	k, e := Open(ctx, dsn, t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(k.Close)
	scope, e := createPGTestCompany(k, ctx, "go-job-persistence")
	must(t, e)
	acceptance := &taskvalidation.AcceptanceContract{Revision: taskvalidation.AcceptanceContractGoTestRevision, RequiredText: []string{"fixed Go fixture only"}, GoTestProfileRevision: gotestprofile.FixtureRevision}
	mission, e := createPGTestMissionGoalWithAcceptance(k, ctx, scope, "Go persistence", "offline fixed fixture", acceptance, "go-mission")
	must(t, e)
	repo := t.TempDir()
	must(t, os.WriteFile(filepath.Join(repo, "go.mod"), []byte("module fixture\n\ngo 1.26\n"), 0600))
	must(t, os.WriteFile(filepath.Join(repo, "formatter.go"), []byte(fixture.Source), 0600))
	must(t, os.WriteFile(filepath.Join(repo, "frozen_test.go"), []byte(fixture.Tests("full")), 0600))
	git := func(args ...string) string {
		t.Helper()
		c := exec.Command("git", args...)
		c.Dir = repo
		c.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1")
		raw, err := c.CombinedOutput()
		if err != nil {
			t.Fatalf("trusted Git fixture setup: %v: %s", err, raw)
		}
		return strings.TrimSpace(string(raw))
	}
	git("init", "--quiet")
	git("add", "--", "go.mod", "formatter.go", "frozen_test.go")
	git("-c", "user.name=Polis Fixture", "-c", "user.email=fixture@invalid", "-c", "commit.gpgsign=false", "commit", "--quiet", "-m", "trusted fixture")
	imported, e := intake.ImportGitCommit(ctx, repo, git("rev-parse", "HEAD"))
	must(t, e)
	_, e = k.TXAddMissionInput(ctx, scope, mission.ID, "", "go-import", imported.Upload, imported.Archive)
	must(t, e)
	_, e = k.TXStartMissionCommand(ctx, scope, mission.ID, "go-start")
	must(t, e)
	task, e := k.TXPrepareProductTask(ctx, scope, mission.ID, "offline fixed fixture", "go-task")
	must(t, e)
	binding, e := k.TXNewProductProviderWorkerWithToolBudget(ctx, scope, task.ID, "deterministic/fake", 32)
	must(t, e)
	process, e := runner.Start(binding.SessionID(), []string{"/bin/sleep", "60"}, nil)
	must(t, e)
	t.Cleanup(func() {
		_ = k.TXBeginStop(context.Background(), binding)
		proof, err := process.Stop()
		if err == nil {
			_ = k.TXConfirmStopped(context.Background(), binding, proof)
		}
	})
	must(t, k.TXAttachWorker(ctx, binding, process))
	must(t, k.TXValidateWorker(ctx, binding))
	must(t, k.TXActivateWorker(ctx, binding, "trusted persistence fixture"))
	_, base, e := k.CreateProductWorkspaceSnapshot(ctx, binding, "go-base", 1)
	must(t, e)
	_, e = k.WriteProductWorkspaceFile(ctx, binding, "go-edit", "formatter.go", 1, fixture.Source+"\n// candidate changed deterministically; no Go code executed\n")
	must(t, e)
	_, candidate, e := k.CreateProductWorkspaceSnapshot(ctx, binding, "go-candidate", 2)
	must(t, e)
	binary := os.Getenv("POLIS_GO_SUPERVISOR_BINARY")
	raw, e := os.ReadFile(binary)
	must(t, e)
	sha := sha256.Sum256(raw)
	makeJob := func(key string) gotestprofile.JobBinding {
		t.Helper()
		b, err := k.PrepareGoTestJobBinding(ctx, scope.company, PrepareGoTestJobBindingInput{RequestID: key, TaskID: task.ID, BaseSnapshotArtifactID: base.ArtifactID, CandidateSnapshotArtifactID: candidate.ArtifactID, RunnerSHA256: hex.EncodeToString(sha[:])})
		must(t, err)
		_, err = k.TXCreateGoTestJobRun(ctx, scope.company, BlockedGoTestJobInput{RequestID: key, Binding: b})
		must(t, err)
		return b
	}
	b := makeJob("go-job-blocked")
	var host gotestsupervisor.HostCheck
	check := exec.Command(binary, "--inspect", "--job-id="+b.JobRunID)
	checked, checkErr := check.Output()
	var exit *exec.ExitError
	if !errors.As(checkErr, &exit) || exit.ExitCode() != 3 || json.Unmarshal(checked, &host) != nil {
		t.Fatalf("actual supervisor preflight: %v %s", checkErr, checked)
	}
	var files []gotestprofile.SnapshotFile
	for _, entry := range candidate.Entries {
		content, err := readBlobBounded(k.root, scope.company, entry.Digest, workspaceTreeMaxFileBytes)
		must(t, err)
		files = append(files, gotestprofile.SnapshotFile{RelativePath: entry.RelativePath, Digest: entry.Digest, Bytes: entry.Bytes, ContentType: entry.ContentType, EntryType: "regular", Content: content})
	}
	_, bindingDigest, _ := gotestprofile.CanonicalizeJobBinding(b)
	requestBytes, _ := json.Marshal(gotestsupervisor.Request{Protocol: gotestsupervisor.Protocol, Binding: b, BindingSHA256: bindingDigest, Files: files})
	requestPath := filepath.Join(t.TempDir(), "request.json")
	must(t, os.WriteFile(requestPath, requestBytes, 0600))
	if output, err := exec.Command(binary, "--check-input="+requestPath).CombinedOutput(); err != nil {
		t.Fatalf("actual supervisor input check: %v %s", err, output)
	}
	if host.Qualified {
		t.Fatal("unprepared test host qualified")
	}
	report := gotestsupervisor.NewUnstartedReport(b, "real", "blocked", "host_profile_unqualified", host)
	_, e = k.TXRecordGoTestJobReport(ctx, scope.company, "go-report", report)
	must(t, e)
	_, e = k.TXRecordGoTestJobReport(ctx, scope.company, "go-report", report)
	must(t, e)
	_, e = k.TXRecordGoTestJobReport(ctx, scope.company, "go-report-duplicate", report)
	must(t, e)
	var results, terminals int
	must(t, k.pool.QueryRow(ctx, "SELECT count(*) FROM go_test_job_results WHERE company_id=$1 AND job_id=$2", scope.company, b.JobRunID).Scan(&results))
	must(t, k.pool.QueryRow(ctx, "SELECT count(*) FROM job_run_events WHERE company_id=$1 AND job_id=$2 AND state='blocked' AND exit_code IS NULL", scope.company, b.JobRunID).Scan(&terminals))
	if results != 1 || terminals != 1 {
		t.Fatalf("duplicate terminal report: results=%d events=%d", results, terminals)
	}
	job, e := k.GetJobRun(ctx, scope.company, b.JobRunID)
	must(t, e)
	if job.State != "blocked" || job.ExitCode != nil {
		t.Fatalf("gate denial fabricated execution: %+v", job)
	}
	if job.LogManifestSHA256 == "" {
		t.Fatal("durable gate report lost its immutable log reference")
	}
	logBytes, e := readBlobBounded(k.root, scope.company, job.LogManifestSHA256, 2<<20)
	must(t, e)
	if casDigest(logBytes) != job.LogManifestSHA256 {
		t.Fatal("stored log reference did not read back from CAS")
	}
	changedTerminal := report
	changedTerminal.Reason = "different_terminal_report"
	_, e = k.TXRecordGoTestJobReport(ctx, scope.company, "go-terminal-conflict", changedTerminal)
	if !errors.Is(e, core.Conflict) {
		t.Fatalf("terminal result was overwritten: %v", e)
	}
	altered := report
	altered.CandidateRevision++
	_, e = k.TXRecordGoTestJobReport(ctx, scope.company, "go-stale-report", altered)
	if !errors.Is(e, core.Integrity) {
		t.Fatalf("cross-candidate report error=%v", e)
	}
	second := makeJob("go-job-start-failure")
	wrongJob := report
	wrongJob.JobRunID = second.JobRunID
	_, e = k.TXRecordGoTestJobReport(ctx, scope.company, "go-wrong-job", wrongJob)
	if !errors.Is(e, core.Integrity) {
		t.Fatalf("receipt from another JobRun was accepted: %v", e)
	}
	simulated := gotestsupervisor.NewUnstartedReport(second, "simulation", "failed", "process_start_failed", gotestsupervisor.HostCheck{Reason: "explicit executor test double"})
	_, e = k.TXRecordGoTestJobReport(ctx, scope.company, "go-simulated-failure", simulated)
	must(t, e)
	failedJob, e := k.GetJobRun(ctx, scope.company, second.JobRunID)
	must(t, e)
	if failedJob.State != "failed" || failedJob.ExitCode != nil {
		t.Fatalf("simulated startup failure fabricated a candidate exit: %+v", failedJob)
	}
	simulated.Outcome = "passed"
	simulated.QualificationEligible = true
	_, e = k.TXRecordGoTestJobReport(ctx, scope.company, "go-forged-pass", simulated)
	if !errors.Is(e, core.Integrity) {
		t.Fatalf("simulated no-start pass accepted: %v", e)
	}
	var qualifications int
	must(t, k.pool.QueryRow(ctx, "SELECT count(*) FROM task_validation_artifact_qualifications WHERE company_id=$1 AND task_id=$2", scope.company, task.ID).Scan(&qualifications))
	if qualifications != 0 {
		t.Fatal("no-start reports created qualifications")
	}
}

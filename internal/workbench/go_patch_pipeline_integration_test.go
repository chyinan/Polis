// pattern: Imperative Shell
package workbench

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"github.com/jackc/pgx/v5/pgxpool"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"polis/internal/control"
	"polis/internal/core"
	"polis/internal/fixture"
	"polis/internal/gotestprofile"
	"polis/internal/gotestsupervisor"
	"polis/internal/intake"
	"polis/internal/kernel"
	"polis/internal/pgfixture"
	"polis/internal/runner"
	"polis/internal/taskvalidation"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

type simulationGoExecutor struct {
	runs atomic.Int32
	mode string
}

func (e *simulationGoExecutor) Readiness(context.Context, string) gotestsupervisor.HostCheck {
	return gotestsupervisor.HostCheck{Qualified: true, QualificationSHA256: strings.Repeat("a", 64), Limits: gotestprofile.FixtureManifest(strings.Repeat("b", 64)).Limits, Reason: "explicit simulation only"}
}
func (e *simulationGoExecutor) Run(ctx context.Context, r gotestsupervisor.Request) gotestsupervisor.Report {
	e.runs.Add(1)
	if e.mode == "cancel" {
		<-ctx.Done()
		report := gotestsupervisor.NewUnstartedReport(r.Binding, "simulation", "cancelled", "simulation_cancelled", gotestsupervisor.HostCheck{})
		report.Outcome = gotestprofile.OutcomeCancelled
		report.Cancelled = true
		return report
	}
	report := gotestsupervisor.NewUnstartedReport(r.Binding, "real", "failed", "simulation_start_failure", gotestsupervisor.HostCheck{})
	if e.mode == "forged_pass" {
		report.State = "exited"
		report.Outcome = "passed"
		report.QualificationEligible = true
	}
	return report
}
func (e *simulationGoExecutor) Stop(context.Context, string) error { return nil }

func TestGoDispatchActualAPIPersistsSimulationAndDeniesPatch(t *testing.T) {
	for _, mode := range []string{"failure", "forged_pass", "cancel", "default_off"} {
		t.Run(mode, func(t *testing.T) {
			dsn := pgfixture.DSN(t, "POLIS_TEST_DSN")
			if dsn == "" {
				t.Skip("dedicated PG18 harness required")
			}
			ctx := context.Background()
			blob := t.TempDir()
			k, err := kernel.Open(ctx, dsn, blob)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(k.Close)
			company := "go-http-" + mode
			scope, err := createPGTestCompany(k, ctx, company)
			if err != nil {
				t.Fatal(err)
			}
			contract := &taskvalidation.AcceptanceContract{Revision: taskvalidation.AcceptanceContractGoTestRevision, GoTestProfileRevision: gotestprofile.FixtureRevision, RequiredText: []string{"package formatter"}}
			mission, err := k.TXCreateMissionGoalWithAcceptance(ctx, scope, "Go HTTP", "fixed fixture", contract, "mission")
			if err != nil {
				t.Fatal(err)
			}
			if err = configurePGTestMission(k, ctx, scope, mission.ID); err != nil {
				t.Fatal(err)
			}
			repo := t.TempDir()
			for path, content := range map[string]string{"formatter.go": fixture.Source, "go.mod": "module fixture\n\ngo 1.26\n", "frozen_test.go": fixture.Tests("full")} {
				if err = os.WriteFile(filepath.Join(repo, path), []byte(content), 0600); err != nil {
					t.Fatal(err)
				}
			}
			git := func(args ...string) string {
				t.Helper()
				c := exec.Command("git", args...)
				c.Dir = repo
				raw, err := c.CombinedOutput()
				if err != nil {
					t.Fatalf("trusted Git fixture: %v %s", err, raw)
				}
				return strings.TrimSpace(string(raw))
			}
			git("init", "--quiet")
			git("add", "--", "formatter.go", "go.mod", "frozen_test.go")
			git("-c", "user.name=Polis Fixture", "-c", "user.email=fixture@invalid", "-c", "commit.gpgsign=false", "commit", "--quiet", "-m", "trusted fixture")
			input, err := intake.ImportGitCommit(ctx, repo, git("rev-parse", "HEAD"))
			if err != nil {
				t.Fatal(err)
			}
			if _, err = k.TXAddMissionInput(ctx, scope, mission.ID, "", "import", input.Upload, input.Archive); err != nil {
				t.Fatal(err)
			}
			if _, err = k.TXStartMissionCommand(ctx, scope, mission.ID, "start"); err != nil {
				t.Fatal(err)
			}
			task, err := k.TXPrepareProductTask(ctx, scope, mission.ID, "fixed fixture", "task")
			if err != nil {
				t.Fatal(err)
			}
			writer, err := k.TXNewProductProviderWorkerWithToolBudget(ctx, scope, task.ID, "deterministic/fake", 32)
			if err != nil {
				t.Fatal(err)
			}
			process, err := runner.Start(writer.SessionID(), []string{"/bin/sleep", "120"}, nil)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				_ = k.TXBeginStop(context.Background(), writer)
				proof, e := process.Stop()
				if e == nil {
					_ = k.TXConfirmStopped(context.Background(), writer, proof)
				}
			})
			if err = k.TXAttachWorker(ctx, writer, process); err != nil {
				t.Fatal(err)
			}
			if err = k.TXValidateWorker(ctx, writer); err != nil {
				t.Fatal(err)
			}
			if err = k.TXActivateWorker(ctx, writer, "trusted offline driver"); err != nil {
				t.Fatal(err)
			}
			_, base, err := k.CreateProductWorkspaceSnapshot(ctx, writer, "base", 1)
			if err != nil {
				t.Fatal(err)
			}
			if _, err = k.WriteProductWorkspaceFile(ctx, writer, "edit", "formatter.go", 1, fixture.Source+"\n// deterministic candidate; never executed\n"); err != nil {
				t.Fatal(err)
			}
			_, candidate, err := k.CreateProductWorkspaceSnapshot(ctx, writer, "candidate", 2)
			if err != nil {
				t.Fatal(err)
			}
			executor := &simulationGoExecutor{mode: mode}
			service := control.NewService(k, nil)
			if err = service.SetGoTestJobExecutor(executor); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = service.Close() })
			store, err := NewPostgresReadStoreWithBlobRoot(ctx, dsn, blob)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(store.Close)
			server := httptest.NewServer(NewHandler(store, service))
			t.Cleanup(server.Close)
			post := func(path string, body any) (int, []byte) {
				t.Helper()
				raw, _ := json.Marshal(body)
				response, err := http.Post(server.URL+path, "application/json", bytes.NewReader(raw))
				if err != nil {
					t.Fatal(err)
				}
				defer response.Body.Close()
				content, _ := io.ReadAll(response.Body)
				return response.StatusCode, content
			}
			path := "/api/workbench/companies/" + company + "/tasks/" + task.ID
			payload := map[string]any{"kind": "go_test", "baseSnapshotArtifactId": base.ArtifactID, "candidateSnapshotArtifactId": candidate.ArtifactID, "requestId": "go-command", "executeGoTest": mode != "default_off"}
			status, raw := post(path+"/jobs", payload)
			if status != http.StatusAccepted {
				t.Fatalf("real API start: %d %s", status, raw)
			}
			var job kernel.JobRunRecord
			if json.Unmarshal(raw, &job) != nil || job.JobID == "" {
				t.Fatal("missing actual JobRun identity")
			}
			status, _ = post(path+"/jobs", payload)
			if status != http.StatusAccepted {
				t.Fatal("idempotent start rejected")
			}
			if mode == "cancel" {
				second := map[string]any{"kind": "go_test", "baseSnapshotArtifactId": base.ArtifactID, "candidateSnapshotArtifactId": candidate.ArtifactID, "requestId": "second-active", "executeGoTest": true}
				if code, _ := post(path+"/jobs", second); code != http.StatusConflict {
					t.Fatalf("concurrent Go execution admitted: %d", code)
				}
				stopCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
				defer cancel()
				if _, err = service.StopProjectJob(stopCtx, company, job.JobID, control.StopProjectJobRequest{RequestID: "stop"}); err != nil {
					t.Fatal(err)
				}
			}
			deadline := time.Now().Add(10 * time.Second)
			for time.Now().Before(deadline) {
				job, err = k.GetJobRun(ctx, company, job.JobID)
				if err != nil {
					t.Fatal(err)
				}
				if job.State == "failed" || job.State == "cancelled" || job.State == "blocked" {
					break
				}
				time.Sleep(10 * time.Millisecond)
			}
			if job.State != "failed" && job.State != "cancelled" && job.State != "blocked" {
				t.Fatalf("simulation did not reach real durable terminal: %+v", job)
			}
			wantRuns := int32(1)
			if mode == "default_off" {
				wantRuns = 0
			}
			if executor.runs.Load() != wantRuns {
				t.Fatalf("dispatch replayed or default-off failed: %d", executor.runs.Load())
			}
			response, err := http.Get(server.URL + path + "/jobs")
			if err != nil {
				t.Fatal(err)
			}
			defer response.Body.Close()
			viewRaw, _ := io.ReadAll(response.Body)
			if response.StatusCode != http.StatusOK || !bytes.Contains(viewRaw, []byte(candidate.Digest)) || !bytes.Contains(viewRaw, []byte(`"qualificationEligible":false`)) || !bytes.Contains(viewRaw, []byte(`"executionKind":"simulation"`)) {
				t.Fatalf("authoritative readback: %d %s", response.StatusCode, viewRaw)
			}
			status, raw = post(path+"/go-patch", map[string]string{"jobRunId": job.JobID, "requestId": "deny-patch"})
			if status < 400 {
				t.Fatalf("simulation published patch: %d %s", status, raw)
			}
			pool, err := pgxpool.New(ctx, dsn)
			if err != nil {
				t.Fatal(err)
			}
			defer pool.Close()
			var results, quals, patches int
			if err = pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM go_test_job_results WHERE company_id=$1),(SELECT count(*) FROM task_validation_artifact_qualifications WHERE company_id=$1),(SELECT count(*) FROM go_patch_deliveries WHERE company_id=$1)`, company).Scan(&results, &quals, &patches); err != nil {
				t.Fatal(err)
			}
			if results != 1 || quals != 0 || patches != 0 {
				t.Fatalf("terminal duplication or fabricated qualification: results=%d qualifications=%d patches=%d", results, quals, patches)
			}
			if _, err = k.TXPublishGoPatchDelivery(ctx, company, kernel.GoPatchDeliveryInput{TaskID: task.ID, JobRunID: job.JobID, RequestID: "again"}); !errors.Is(err, core.Denied) {
				t.Fatalf("unqualified patch boundary error=%v", err)
			}
		})
	}
}

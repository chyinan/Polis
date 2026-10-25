// pattern: Imperative Shell
package workbench

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"polis/internal/control"
	"polis/internal/desktop"
	"polis/internal/fixture"
	"polis/internal/gotestprofile"
	"polis/internal/gotestsupervisor"
	"polis/internal/installationauth"
	"polis/internal/intake"
	"polis/internal/kernel"
	"polis/internal/pgfixture"
	"polis/internal/runner"
	"polis/internal/taskvalidation"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// This test is intentionally opt-in. It starts an actual systemd-managed
// candidate Go JobRun and coordinates with the local Windows browser runner.
func TestRealGoPatchWorkbenchE2E(t *testing.T) {
	if os.Getenv("POLIS_SELF_DEV_REAL_GO_PATCH_E2E") != "1" {
		t.Skip("set POLIS_SELF_DEV_REAL_GO_PATCH_E2E=1 only for the explicitly qualified host E2E")
	}
	if os.Getenv("POLIS_GO_TOOLCHAIN_ROOT") != "/run/polis-self-dev-go-toolchain" {
		t.Fatal("real Go Patch E2E requires the boot-scoped read-only qualified toolchain stage")
	}
	dsn := pgfixture.DSN(t, "POLIS_TEST_DSN")
	if dsn == "" {
		t.Fatal("dedicated PostgreSQL 18 harness required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()
	root, err := repositoryRoot()
	if err != nil {
		t.Fatal(err)
	}
	runID := os.Getenv("POLIS_SELF_DEV_E2E_RUN_ID")
	if !validSelfDevE2ERunID(runID) {
		t.Fatal("a unique POLIS_SELF_DEV_E2E_RUN_ID is required")
	}
	evidenceBase := filepath.Join(root, "evidence", "development", "polis-self-dev-v1-go-patch-delivery-20261009")
	evidence, err := filepath.Abs(os.Getenv("POLIS_SELF_DEV_E2E_EVIDENCE"))
	if err != nil {
		t.Fatal(err)
	}
	relativeEvidence, err := filepath.Rel(evidenceBase, evidence)
	if err != nil || relativeEvidence == "." || relativeEvidence == ".." || strings.HasPrefix(relativeEvidence, ".."+string(os.PathSeparator)) || filepath.Base(evidence) != runID {
		t.Fatalf("E2E evidence path is not the unique run directory: %s", evidence)
	}
	if _, err = os.Lstat(evidence); err == nil || !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("refusing to overwrite existing E2E evidence path: %s", evidence)
	}
	if err = os.MkdirAll(evidence, 0700); err != nil {
		t.Fatal(err)
	}
	statePath := filepath.Join(evidence, "fixture-state.json")
	completePath := filepath.Join(evidence, "browser-complete.json")
	viteMode := "selfdev-e2e-" + runID
	envPath := filepath.Join(root, "frontend", ".env."+viteMode+".local")
	for _, path := range []string{statePath, completePath, envPath} {
		if _, statErr := os.Lstat(path); statErr == nil || !errors.Is(statErr, os.ErrNotExist) {
			t.Fatalf("refusing to overwrite existing E2E state: %s", path)
		}
	}
	t.Cleanup(func() {
		_ = os.Remove(statePath)
		_ = os.Remove(envPath)
	})

	k, err := kernel.Open(ctx, dsn, os.Getenv("POLIS_BLOB_ROOT"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(k.Close)
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	companyID := "selfdev-e2e-" + strings.ToLower(randomSuffix())
	scope, err := createPGTestCompany(k, ctx, companyID)
	if err != nil {
		t.Fatal(err)
	}
	acceptance := &taskvalidation.AcceptanceContract{
		Revision:              taskvalidation.AcceptanceContractGoTestRevision,
		GoTestProfileRevision: gotestprofile.FixtureRevision,
		RequiredText:          []string{"package formatter"},
	}
	mission, err := createPGTestMissionGoalWithAcceptance(k, ctx, scope, "Disposable Go Patch E2E", "exercise the restricted Go test and Workbench review chain", acceptance, "e2e-mission-"+randomSuffix())
	if err != nil {
		t.Fatal(err)
	}
	repository := t.TempDir()
	for name, content := range map[string]string{
		"go.mod":         "module fixture\n\ngo 1.26\n",
		"formatter.go":   fixture.Source,
		"frozen_test.go": fixture.Tests("full"),
	} {
		if err = os.WriteFile(filepath.Join(repository, name), []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}
	git := func(args ...string) string {
		t.Helper()
		command := exec.Command("git", args...)
		command.Dir = repository
		command.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1")
		output, runErr := command.CombinedOutput()
		if runErr != nil {
			t.Fatalf("synthetic Git fixture setup: %v: %s", runErr, output)
		}
		return strings.TrimSpace(string(output))
	}
	git("init", "--quiet")
	git("add", "--", "go.mod", "formatter.go", "frozen_test.go")
	git("-c", "user.name=Polis E2E Fixture", "-c", "user.email=fixture@invalid", "-c", "commit.gpgsign=false", "commit", "--quiet", "-m", "synthetic fixed Go fixture")
	imported, err := intake.ImportGitCommit(ctx, repository, git("rev-parse", "HEAD"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = k.TXAddMissionInput(ctx, scope, mission.ID, "", "e2e-git-input", imported.Upload, imported.Archive); err != nil {
		t.Fatal(err)
	}
	if _, err = k.TXStartMissionCommand(ctx, scope, mission.ID, "e2e-start-"+randomSuffix()); err != nil {
		t.Fatal(err)
	}
	task, err := k.TXPrepareProductTask(ctx, scope, mission.ID, "run the fixed offline Go fixture", "e2e-task-"+randomSuffix())
	if err != nil {
		t.Fatal(err)
	}
	worker, err := k.TXNewProductProviderWorkerWithToolBudget(ctx, scope, task.ID, "deterministic/fake", 32)
	if err != nil {
		t.Fatal(err)
	}
	workerProcess, err := runner.Start(worker.SessionID(), []string{"/bin/sleep", "1800"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	workerStopped := false
	stopFixtureWorker := func() {
		if workerStopped {
			return
		}
		stopCtx, stopCancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer stopCancel()
		_ = k.TXBeginStop(stopCtx, worker)
		proof, stopErr := workerProcess.Stop()
		if stopErr == nil {
			_ = k.TXConfirmStopped(stopCtx, worker, proof)
			workerStopped = true
		}
	}
	t.Cleanup(stopFixtureWorker)
	if err = k.TXAttachWorker(ctx, worker, workerProcess); err != nil {
		t.Fatal(err)
	}
	if err = k.TXValidateWorker(ctx, worker); err != nil {
		t.Fatal(err)
	}
	if err = k.TXActivateWorker(ctx, worker, "deterministic fixture only; no provider adapter"); err != nil {
		t.Fatal(err)
	}
	var workerState, workerProfile string
	if err = pool.QueryRow(ctx, `SELECT state,profile FROM worker_sessions WHERE company_id=$1 AND id=$2`, companyID, worker.SessionID()).Scan(&workerState, &workerProfile); err != nil {
		t.Fatal(err)
	}
	if workerState != "active" || workerProfile != "deterministic/fake" {
		t.Fatalf("fixture session is not the exact active fake: state=%s profile=%s", workerState, workerProfile)
	}
	_, baseSnapshot, err := k.CreateProductWorkspaceSnapshot(ctx, worker, "e2e-base-"+randomSuffix(), 1)
	if err != nil {
		t.Fatal(err)
	}
	candidateSource := strings.Replace(fixture.Source,
		"return strconv.FormatFloat(v, 'f', -1, 64)",
		"rendered := strconv.FormatFloat(v, 'f', -1, 64)\n  if v > 0 { return \"+\" + rendered }\n  return rendered", 1)
	if candidateSource == fixture.Source {
		t.Fatal("synthetic candidate could not find the frozen Render implementation")
	}
	candidateMarker := "func RenderWithUnit(v float64, unit string) string"
	candidateSource += `
func RenderWithUnit(v float64, unit string) string {
 rendered := Render(v)
 if unit == "" { return rendered }
 return rendered + " " + unit
}
`
	_, err = k.WriteProductWorkspaceFile(ctx, worker, "e2e-candidate-edit-"+randomSuffix(), "formatter.go", 1, candidateSource)
	if err != nil {
		t.Fatal(err)
	}
	_, candidateSnapshot, err := k.CreateProductWorkspaceSnapshot(ctx, worker, "e2e-candidate-"+randomSuffix(), 2)
	if err != nil {
		t.Fatal(err)
	}

	var inputManifestSHA, validationBindingSHA string
	var validationContract []byte
	if err = pool.QueryRow(ctx, `SELECT i.manifest_digest,b.configuration_digest,b.contract
FROM task_input_manifests i JOIN task_validation_bindings b
  ON b.company_id=i.company_id AND b.task_id=i.task_id AND b.mission_id=i.mission_id
WHERE i.company_id=$1 AND i.task_id=$2`, companyID, task.ID).Scan(&inputManifestSHA, &validationBindingSHA, &validationContract); err != nil {
		t.Fatal(err)
	}
	contractDigest := sha256.Sum256(validationContract)
	limitationsDetail := "Disposable synthetic fixture only; fixed tests do not establish general correctness, malicious-code isolation, production readiness, or downstream applicability."
	licenseDetail := "Synthetic fixture source only; this E2E makes no third-party attribution or production license determination."
	limitsDigest := sha256.Sum256([]byte(limitationsDetail))
	licenseDigest := sha256.Sum256([]byte(licenseDetail))
	ownerPassword := "polis-e2e-" + randomSuffix() + randomSuffix()
	ownerStore := installationauth.NewStore(pool)
	bootstrapCode, _, err := ownerStore.IssueBootstrap(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err = ownerStore.InitializeOwner(ctx, bootstrapCode, []byte(ownerPassword)); err != nil {
		t.Fatal(err)
	}
	readStore, err := NewPostgresReadStoreWithBlobRoot(ctx, dsn, os.Getenv("POLIS_BLOB_ROOT"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(readStore.Close)
	service := control.NewService(k, nil)
	t.Cleanup(func() { _ = service.Close() })
	router := http.NewServeMux()
	router.Handle("/api/workbench/", NewHandler(readStore, service))
	router.Handle("/api/installation/owner/", desktop.OwnerSetupHandler(ownerStore, false))
	bindAddress, err := wslPrivateIPv4()
	if err != nil {
		t.Fatal(err)
	}
	listener, err := net.Listen("tcp", net.JoinHostPort(bindAddress, "0"))
	if err != nil {
		t.Fatalf("bind disposable API only to the WSL private interface: %v", err)
	}
	server := httptest.NewUnstartedServer(ownerStore.SessionMiddleware(desktop.Middleware("", router)))
	server.Listener = listener
	server.Start()
	t.Cleanup(server.Close)

	oldSupervisor, hadSupervisor := os.LookupEnv("POLIS_GO_SUPERVISOR_BINARY")
	if err = os.Setenv("POLIS_GO_SUPERVISOR_BINARY", gotestsupervisor.InstalledBinary); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if hadSupervisor {
			_ = os.Setenv("POLIS_GO_SUPERVISOR_BINARY", oldSupervisor)
		} else {
			_ = os.Unsetenv("POLIS_GO_SUPERVISOR_BINARY")
		}
	})
	state := map[string]any{
		"baseUrl":                     server.URL,
		"viteMode":                    viteMode,
		"companyId":                   companyID,
		"taskId":                      task.ID,
		"baseSnapshotArtifactId":      baseSnapshot.ArtifactID,
		"candidateSnapshotArtifactId": candidateSnapshot.ArtifactID,
		"candidateMarker":             candidateMarker,
		"ownerPassword":               ownerPassword,
		"sourceInputs":                map[string]string{"reference": "task_input_manifests/" + task.ID, "digest": inputManifestSHA, "detail": "Exact frozen TaskInputManifest for the disposable committed fixture."},
		"environmentBuild":            map[string]string{"reference": "task_validation_bindings/" + task.ID, "digest": validationBindingSHA, "detail": "Current immutable TaskValidationBinding for polis-go-test-fixture@1."},
		"runInstructions":             map[string]string{"reference": "task_validation_contract/" + task.ID, "digest": hex.EncodeToString(contractDigest[:]), "detail": "Fixed Go profile command and frozen full test manifest; no caller command or arguments."},
		"limitations":                 map[string]string{"reference": "polis.delivery.limitations@self-dev-e2e", "digest": hex.EncodeToString(limitsDigest[:]), "detail": limitationsDetail},
		"licenseSource":               map[string]string{"reference": "polis.delivery.license-source@self-dev-e2e", "digest": hex.EncodeToString(licenseDigest[:]), "detail": licenseDetail},
	}
	stateRaw, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	stateFile, err := os.OpenFile(statePath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = stateFile.Write(stateRaw); err == nil {
		err = stateFile.Sync()
	}
	if closeErr := stateFile.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		t.Fatal(err)
	}
	frontendEnv := strings.Join([]string{
		"VITE_WORKBENCH_MODE=real",
		"VITE_WORKBENCH_API_BASE_URL=/api/workbench",
		"VITE_WORKBENCH_COMPANY_ID=" + companyID,
		"VITE_WORKBENCH_LIVE_UPDATES=deferred",
		"VITE_WORKBENCH_BACKEND_URL=" + server.URL,
		"",
	}, "\n")
	if err = os.WriteFile(envPath, []byte(frontendEnv), 0600); err != nil {
		t.Fatal(err)
	}

	t.Logf("fixture ready for local browser: company=%s mission=%s task=%s api=%s evidence=%s", companyID, mission.ID, task.ID, server.URL, evidence)
	for {
		if _, err = os.Stat(completePath); err == nil {
			break
		} else if !errors.Is(err, os.ErrNotExist) {
			t.Fatal(err)
		}
		select {
		case <-ctx.Done():
			t.Fatalf("browser E2E completion was not received: %v", ctx.Err())
		case <-time.After(200 * time.Millisecond):
		}
	}
	completeRaw, err := os.ReadFile(completePath)
	if err != nil {
		t.Fatal(err)
	}
	var browserResult struct {
		Result         string `json:"result"`
		CompanyID      string `json:"companyId"`
		TaskID         string `json:"taskId"`
		JobRunID       string `json:"jobRunId"`
		ArtifactID     string `json:"artifactId"`
		JobState       string `json:"jobState"`
		Outcome        string `json:"outcome"`
		Disposition    string `json:"disposition"`
		DiffVisible    bool   `json:"diffVisible"`
		NoAutoApply    bool   `json:"noAutoApply"`
		ExternalAction string `json:"externalAction"`
	}
	if json.Unmarshal(completeRaw, &browserResult) != nil || browserResult.Result != "passed" || browserResult.CompanyID != companyID || browserResult.TaskID != task.ID || browserResult.JobRunID == "" || browserResult.ArtifactID == "" || browserResult.JobState != "exited" || browserResult.Outcome != "passed" || browserResult.Disposition != "accepted" || !browserResult.DiffVisible || !browserResult.NoAutoApply || browserResult.ExternalAction != "not_run" {
		t.Fatalf("browser E2E evidence is incomplete: %s", completeRaw)
	}

	job, err := k.GetJobRun(ctx, companyID, browserResult.JobRunID)
	if err != nil || job.State != "exited" || job.ExitCode == nil || *job.ExitCode != 0 {
		t.Fatalf("real Go JobRun terminal projection mismatch: job=%+v err=%v", job, err)
	}
	var outcome string
	var resultRaw, receiptRaw []byte
	if err = pool.QueryRow(ctx, `SELECT outcome,result,runner_receipt FROM go_test_job_results WHERE company_id=$1 AND job_id=$2`, companyID, browserResult.JobRunID).Scan(&outcome, &resultRaw, &receiptRaw); err != nil {
		t.Fatal(err)
	}
	var resultEnvelope struct {
		Outcome               string `json:"outcome"`
		ProcessStarted        bool   `json:"processStarted"`
		ExitObserved          bool   `json:"exitObserved"`
		ExitCode              *int   `json:"exitCode"`
		CleanupConfirmed      bool   `json:"cleanupConfirmed"`
		ExecutionKind         string `json:"executionKind"`
		LogManifestSHA256     string `json:"logManifestSha256"`
		BindingSHA256         string `json:"bindingSha256"`
		QualificationEligible bool   `json:"qualificationEligible"`
	}
	var report gotestsupervisor.Report
	if json.Unmarshal(resultRaw, &resultEnvelope) != nil || json.Unmarshal(receiptRaw, &report) != nil ||
		outcome != gotestprofile.OutcomePassed || resultEnvelope.Outcome != gotestprofile.OutcomePassed ||
		resultEnvelope.ExecutionKind != "real" || resultEnvelope.BindingSHA256 != report.BindingSHA256 ||
		resultEnvelope.LogManifestSHA256 == "" || resultEnvelope.LogManifestSHA256 != job.LogManifestSHA256 ||
		!resultEnvelope.ProcessStarted || !resultEnvelope.ExitObserved || resultEnvelope.ExitCode == nil || *resultEnvelope.ExitCode != 0 ||
		!resultEnvelope.CleanupConfirmed || !resultEnvelope.QualificationEligible ||
		report.ExecutionKind != "real" || report.State != "exited" || !report.DispatchAccepted || !report.CleanupConfirmed ||
		!report.QualificationEligible || !report.Host.Qualified || report.TestResult == nil || report.TestResult.Outcome != gotestprofile.OutcomePassed {
		t.Fatalf("stored supervisor result is not a qualified real pass: outcome=%s summary=%+v receipt=%+v", outcome, resultEnvelope, report)
	}
	var patchRows, qualifications int
	if err = pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM go_patch_deliveries WHERE company_id=$1 AND task_id=$2 AND job_id=$3 AND artifact_id=$4 AND replay_verified),
(SELECT count(*) FROM task_validation_artifact_qualifications WHERE company_id=$1 AND task_id=$2 AND artifact_id=$4)`, companyID, task.ID, browserResult.JobRunID, browserResult.ArtifactID).Scan(&patchRows, &qualifications); err != nil {
		t.Fatal(err)
	}
	if patchRows != 1 || qualifications != 1 {
		t.Fatalf("formal Patch/qualification publication mismatch: patches=%d qualifications=%d", patchRows, qualifications)
	}
	var manifestRevision, manifestState, dispositionState, actor string
	if err = pool.QueryRow(ctx, `SELECT m.revision::text,m.state,d.state,d.actor
FROM delivery_manifest_revisions m JOIN delivery_user_dispositions d
  ON d.company_id=m.company_id AND d.delivery_id=m.delivery_id AND d.manifest_revision=m.revision
WHERE m.company_id=$1 AND m.delivery_id=$2 ORDER BY m.revision DESC,d.revision DESC LIMIT 1`, companyID, browserResult.ArtifactID).Scan(&manifestRevision, &manifestState, &dispositionState, &actor); err != nil {
		t.Fatal(err)
	}
	if manifestRevision != "2" || manifestState != "ready" || dispositionState != "accepted" || actor != "installation-owner" {
		t.Fatalf("durable delivery disposition mismatch: manifest=%s/%s disposition=%s actor=%s", manifestRevision, manifestState, dispositionState, actor)
	}
	stopFixtureWorker()
	var finalWorkerState string
	if err = pool.QueryRow(ctx, `SELECT state FROM worker_sessions WHERE company_id=$1 AND id=$2`, companyID, worker.SessionID()).Scan(&finalWorkerState); err != nil || finalWorkerState != "stopped" {
		t.Fatalf("deterministic fixture WorkerSession stop was not confirmed in the database: state=%s err=%v", finalWorkerState, err)
	}
	fmt.Fprintf(os.Stdout, "real_go_job=passed\npatch_artifact=%s\nmanifest=%s/%s\ndisposition=%s/%s\nprovider_egress=0\nreal_model_worker_started=0\nautomatic_merge=NO\nautomatic_deploy=NO\n", browserResult.ArtifactID, manifestRevision, manifestState, dispositionState, actor)
}

func repositoryRoot() (string, error) {
	workingDirectory, err := os.Getwd()
	if err != nil {
		return "", err
	}
	return filepath.Abs(filepath.Join(workingDirectory, "..", ".."))
}

func wslPrivateIPv4() (string, error) {
	interfaceValue, err := net.InterfaceByName("eth0")
	if err != nil {
		return "", fmt.Errorf("WSL eth0 is required for the isolated Windows-browser fixture: %w", err)
	}
	addresses, err := interfaceValue.Addrs()
	if err != nil {
		return "", err
	}
	for _, address := range addresses {
		var ip net.IP
		switch value := address.(type) {
		case *net.IPNet:
			ip = value.IP
		case *net.IPAddr:
			ip = value.IP
		}
		if ipv4 := ip.To4(); ipv4 != nil && !ipv4.IsLoopback() && !ipv4.IsLinkLocalUnicast() {
			return ipv4.String(), nil
		}
	}
	return "", fmt.Errorf("WSL eth0 has no usable private IPv4 address")
}

func randomSuffix() string {
	return rand.Text()[:12]
}

func validSelfDevE2ERunID(value string) bool {
	if len(value) < 8 || len(value) > 48 {
		return false
	}
	for _, character := range value {
		if !(character >= 'a' && character <= 'z' || character >= 'A' && character <= 'Z' || character >= '0' && character <= '9' || character == '-' || character == '_') {
			return false
		}
	}
	return true
}

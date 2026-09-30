// pattern: Imperative Shell
package workbench_test

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/giraffesyo/pdf/pdftest"
	"github.com/jackc/pgx/v5/pgxpool"
	"polis/internal/control"
	"polis/internal/intake"
	"polis/internal/kernel"
	"polis/internal/provider"
	"polis/internal/taskvalidation"
	"polis/internal/workbench"
)

func TestTaskInputManifestReadShowsFinalWorkerDeliveryStatus(t *testing.T) {
	dsn := os.Getenv("POLIS_TEST_DSN")
	if dsn == "" {
		t.Skip("dedicated PostgreSQL required")
	}
	ctx := context.Background()
	blobRoot := t.TempDir()
	runtime, err := kernel.Open(ctx, dsn, blobRoot)
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Close()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	companyID := fmt.Sprintf("input-delivery-read-%d", time.Now().UnixNano())
	if _, err = runtime.TXCreateCompany(ctx, companyID); err != nil {
		t.Fatal(err)
	}
	fakeRuntime := provider.NewFakeRuntime(provider.FakeRuntimeConfig{TurnDelay: time.Millisecond})
	worker, err := control.NewRealProviderWorkerAdapter(runtime, fakeRuntime)
	if err != nil {
		t.Fatal(err)
	}
	service := control.NewService(runtime, worker)
	defer service.Close()
	mission, err := service.CreateMission(ctx, companyID, control.CreateMissionRequest{
		Title: "Input delivery view", Goal: "Read bound task inputs and record a local Worker turn.", RequestID: "input-delivery-read-mission",
		AcceptanceContract: &taskvalidation.AcceptanceContract{Revision: taskvalidation.AcceptanceContractRevision, RequiredText: []string{"Mission ID: {{mission_id}}", "Task ID: {{task_id}}", "Task summary:"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	content := []byte("read this bound text")
	if _, err = service.UploadMissionInput(ctx, companyID, control.UploadMissionInputRequest{MissionID: mission.TargetID, RequestID: "input-delivery-read-upload"}, "goal.md", "text/markdown", content); err != nil {
		t.Fatal(err)
	}
	if _, err = service.StartMission(ctx, companyID, control.MissionCommandRequest{MissionID: mission.TargetID, RequestID: "input-delivery-read-start"}); err != nil {
		t.Fatal(err)
	}
	var taskID string
	if err = pool.QueryRow(ctx, "SELECT id FROM tasks WHERE company_id=$1 AND mission_id=$2 AND kind='compat'", companyID, mission.TargetID).Scan(&taskID); err != nil {
		t.Fatal(err)
	}
	manifest, err := runtime.TaskInputManifest(ctx, runtime.LocalScope(companyID), taskID)
	if err != nil {
		t.Fatal(err)
	}
	waitForFinalInputDelivery(t, ctx, pool, companyID, taskID)
	if len(manifest.Manifest.CandidateInputs) != 1 {
		t.Fatalf("frozen Task input candidates = %d, want 1", len(manifest.Manifest.CandidateInputs))
	}
	store, err := workbench.NewPostgresReadStoreWithBlobRoot(ctx, dsn, blobRoot)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	view, err := store.GetTaskInputManifest(ctx, companyID, taskID)
	if err != nil {
		t.Fatal(err)
	}
	expectedPayload, err := intake.PrepareModelInputContext(manifest.Manifest, manifest.Digest, map[string][]byte{manifest.Manifest.CandidateInputs[0].InputID: content})
	if err != nil {
		t.Fatal(err)
	}
	if view.DeliveryStatus != "local_context_loaded" || view.Manifest.DeliveryStatus != "not_delivered" || view.PayloadDigest == nil || *view.PayloadDigest != expectedPayload.PayloadDigest {
		t.Fatalf("input manifest delivery projection = outer %q, frozen manifest %q", view.DeliveryStatus, view.Manifest.DeliveryStatus)
	}
	if len(view.IncludedInputIDs) != 1 || view.IncludedInputIDs[0] != manifest.Manifest.CandidateInputs[0].InputID || len(view.InputExclusions) != 0 || view.PayloadDigest == nil || len(*view.PayloadDigest) != 64 {
		t.Fatalf("input delivery refs were not projected accurately: %+v", view)
	}
}

func TestEmptyTaskInputDeliveryIsRecordedAndReadAsNotRequired(t *testing.T) {
	dsn := os.Getenv("POLIS_TEST_DSN")
	if dsn == "" {
		t.Skip("dedicated PostgreSQL required")
	}
	ctx := context.Background()
	blobRoot := t.TempDir()
	runtime, err := kernel.Open(ctx, dsn, blobRoot)
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Close()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	companyID := fmt.Sprintf("input-empty-read-%d", time.Now().UnixNano())
	if _, err = runtime.TXCreateCompany(ctx, companyID); err != nil {
		t.Fatal(err)
	}
	fakeRuntime := provider.NewFakeRuntime(provider.FakeRuntimeConfig{TurnDelay: time.Millisecond})
	worker, err := control.NewRealProviderWorkerAdapter(runtime, fakeRuntime)
	if err != nil {
		t.Fatal(err)
	}
	service := control.NewService(runtime, worker)
	defer service.Close()
	mission, err := service.CreateMission(ctx, companyID, control.CreateMissionRequest{
		Title: "No input mission", Goal: "Run without any registered MissionInputs.", RequestID: "input-empty-read-mission",
		AcceptanceContract: &taskvalidation.AcceptanceContract{Revision: taskvalidation.AcceptanceContractRevision, RequiredText: []string{"Mission ID: {{mission_id}}", "Task ID: {{task_id}}", "Task summary:"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = service.StartMission(ctx, companyID, control.MissionCommandRequest{MissionID: mission.TargetID, RequestID: "input-empty-read-start"}); err != nil {
		t.Fatal(err)
	}
	var taskID string
	if err = pool.QueryRow(ctx, "SELECT id FROM tasks WHERE company_id=$1 AND mission_id=$2 AND kind='compat'", companyID, mission.TargetID).Scan(&taskID); err != nil {
		t.Fatal(err)
	}
	manifest, err := runtime.TaskInputManifest(ctx, runtime.LocalScope(companyID), taskID)
	if err != nil {
		t.Fatal(err)
	}
	waitForFinalInputDelivery(t, ctx, pool, companyID, taskID)
	if len(manifest.Manifest.CandidateInputs) != 0 {
		t.Fatalf("empty Task has %d candidate inputs", len(manifest.Manifest.CandidateInputs))
	}
	store, err := workbench.NewPostgresReadStoreWithBlobRoot(ctx, dsn, blobRoot)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	view, err := store.GetTaskInputManifest(ctx, companyID, taskID)
	if err != nil {
		t.Fatal(err)
	}
	if view.DeliveryStatus != "not_required" || view.PayloadDigest == nil || len(*view.PayloadDigest) != 64 || len(view.IncludedInputPaths) != 0 || len(view.InputExclusions) != 0 || len(view.Manifest.CandidateInputs) != 0 {
		t.Fatalf("empty Task input delivery readback = %+v", view)
	}
}

func waitForFinalInputDelivery(t *testing.T, ctx context.Context, pool *pgxpool.Pool, companyID, taskID string) {
	t.Helper()
	deadline, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		var exists bool
		if err := pool.QueryRow(deadline, `SELECT EXISTS(SELECT 1 FROM task_input_delivery_attempts WHERE company_id=$1 AND task_id=$2 AND phase='final')`, companyID, taskID).Scan(&exists); err != nil {
			t.Fatal(err)
		}
		if exists {
			return
		}
		select {
		case <-deadline.Done():
			t.Fatal("Worker did not persist a final Task input delivery receipt")
		case <-ticker.C:
		}
	}
}

func TestQualifiedArtifactDeliveryManifestAndPackageReadFromPostgresAndCAS(t *testing.T) {
	dsn := os.Getenv("POLIS_TEST_DSN")
	if dsn == "" {
		t.Skip("dedicated PostgreSQL required")
	}
	ctx := context.Background()
	blobRoot := t.TempDir()
	runtime, err := kernel.Open(ctx, dsn, blobRoot)
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Close()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	companyID := fmt.Sprintf("artifact-download-%d", time.Now().UnixNano())
	if _, err = runtime.TXCreateCompany(ctx, companyID); err != nil {
		t.Fatal(err)
	}
	adapter, err := control.NewRealProviderWorkerAdapter(runtime, provider.NewFakeRuntime(provider.FakeRuntimeConfig{}))
	if err != nil {
		t.Fatal(err)
	}
	service := control.NewService(runtime, adapter)
	defer service.Close()
	mission, err := service.CreateMission(ctx, companyID, control.CreateMissionRequest{
		Title: "Artifact delivery verification", Goal: "Create a qualified local fixture artifact.", RequestID: "artifact-download-create",
		AcceptanceContract: &taskvalidation.AcceptanceContract{Revision: taskvalidation.AcceptanceContractRevision, RequiredText: []string{"Mission ID: {{mission_id}}", "Task ID: {{task_id}}", "Acknowledgement:", "Task summary:"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = service.StartMission(ctx, companyID, control.MissionCommandRequest{MissionID: mission.TargetID, RequestID: "artifact-download-start"}); err != nil {
		t.Fatal(err)
	}
	var artifactID, taskID string
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		err = pool.QueryRow(ctx, `SELECT id,task_id FROM artifacts WHERE company_id=$1 AND state='ready' LIMIT 1`, companyID).Scan(&artifactID, &taskID)
		if err == nil {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if artifactID == "" || taskID == "" {
		t.Fatalf("fake Worker did not publish a ready artifact: %v", err)
	}
	store, err := workbench.NewPostgresReadStoreWithBlobRoot(ctx, dsn, blobRoot)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	manifest, err := store.GetArtifactDeliveryManifest(ctx, companyID, artifactID)
	if err != nil {
		t.Fatal(err)
	}
	if manifest.Manifest.CompanyID != companyID || manifest.Manifest.ArtifactID != artifactID || manifest.Manifest.TaskID != taskID || len(manifest.ManifestSHA256) != 64 {
		t.Fatalf("artifact delivery manifest scope or hash = %+v", manifest)
	}
	packaged, err := store.GetArtifactDeliveryPackage(ctx, companyID, artifactID)
	if err != nil {
		t.Fatal(err)
	}
	packageHash := sha256.Sum256(packaged.Archive)
	if packaged.PackageSHA256 != hex.EncodeToString(packageHash[:]) || packaged.ManifestSHA256 != manifest.ManifestSHA256 {
		t.Fatal("download package hashes differ from the PostgreSQL manifest response")
	}
	archive, err := zip.NewReader(bytes.NewReader(packaged.Archive), int64(len(packaged.Archive)))
	if err != nil {
		t.Fatal(err)
	}
	entries := make(map[string][]byte, len(archive.File))
	for _, file := range archive.File {
		reader, openErr := file.Open()
		if openErr != nil {
			t.Fatal(openErr)
		}
		content, readErr := io.ReadAll(reader)
		_ = reader.Close()
		if readErr != nil {
			t.Fatal(readErr)
		}
		entries[file.Name] = content
	}
	if len(entries) != 3 || len(entries["artifact.bin"]) == 0 || len(entries["manifest.json"]) == 0 || len(entries["SHA256SUMS"]) == 0 {
		t.Fatalf("download ZIP entry set or artifact bytes are invalid: %v", mapKeys(entries))
	}
	artifactDigest := sha256.Sum256(entries["artifact.bin"])
	manifestDigest := sha256.Sum256(entries["manifest.json"])
	if hex.EncodeToString(artifactDigest[:]) != manifest.Manifest.Content.SHA256 || strconv.Itoa(len(entries["artifact.bin"])) != manifest.Manifest.Content.ByteSize || hex.EncodeToString(manifestDigest[:]) != manifest.ManifestSHA256 || !bytes.Contains(entries["manifest.json"], []byte(manifest.Manifest.ArtifactID)) {
		t.Fatal("download artifact or embedded manifest differs from the qualified PostgreSQL record")
	}
	if !strings.Contains(string(entries["SHA256SUMS"]), manifest.Manifest.Content.SHA256+"  artifact.bin") {
		t.Fatal("download checksum list omits the qualified artifact hash")
	}
	beforeChange, err := store.GetCompanyOverview(ctx, companyID)
	if err != nil {
		t.Fatal(err)
	}
	var previousVerdict, previousTaskAcceptance string
	for _, artifact := range beforeChange.Artifacts {
		if artifact.ArtifactID == artifactID {
			previousVerdict = artifact.Verdict
		}
	}
	for _, task := range beforeChange.Tasks {
		if task.TaskID == taskID {
			previousTaskAcceptance = task.Acceptance
		}
	}
	if previousVerdict == "" || previousTaskAcceptance == "" {
		t.Fatalf("initial Artifact/Task projection is missing: verdict=%q acceptance=%q", previousVerdict, previousTaskAcceptance)
	}
	change, err := runtime.TXCreateMissionChangeRequest(ctx, runtime.LocalScope(companyID), mission.TargetID, kernel.MissionChangeRequestInput{
		ChangeSummary: "Update the formal acceptance boundary.", ProposedGoal: "Use the updated acceptance boundary for the next Mission.", BlockPreviousResults: true,
	}, "artifact-delivery-change-"+strconv.FormatInt(time.Now().UnixNano(), 10))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = store.GetArtifactDeliveryManifest(ctx, companyID, artifactID); err == nil {
		t.Fatal("artifact manifest remained available while a blocking formal change request was open")
	}
	if _, err = store.GetArtifactDeliveryPackage(ctx, companyID, artifactID); err == nil {
		t.Fatal("artifact ZIP remained available while a blocking formal change request was open")
	}
	blockedRequest := httptest.NewRequest(http.MethodGet, "/api/workbench/companies/"+companyID+"/artifacts/"+artifactID+"/manifest", nil)
	blockedResponse := httptest.NewRecorder()
	workbench.NewHandler(store).ServeHTTP(blockedResponse, blockedRequest)
	if blockedResponse.Code != http.StatusConflict {
		t.Fatalf("blocked delivery manifest HTTP status=%d, body=%s, want 409", blockedResponse.Code, blockedResponse.Body.String())
	}
	blockedOverview, err := store.GetCompanyOverview(ctx, companyID)
	if err != nil {
		t.Fatal(err)
	}
	var blockedArtifactFound, blockedTaskFound bool
	for _, artifact := range blockedOverview.Artifacts {
		if artifact.ArtifactID == artifactID && artifact.Verdict != "invalidated" {
			t.Fatalf("blocked Artifact projection verdict=%q, want invalidated", artifact.Verdict)
		}
		blockedArtifactFound = blockedArtifactFound || artifact.ArtifactID == artifactID
	}
	for _, task := range blockedOverview.Tasks {
		if task.TaskID == taskID && task.Acceptance != "inconclusive" {
			t.Fatalf("blocked Task acceptance=%q, want inconclusive", task.Acceptance)
		}
		blockedTaskFound = blockedTaskFound || task.TaskID == taskID
	}
	if !blockedArtifactFound || !blockedTaskFound {
		t.Fatal("blocking the old result removed its Artifact or Task from the historical overview")
	}
	declined, err := runtime.TXDeclineMissionChangeRequest(ctx, runtime.LocalScope(companyID), mission.TargetID, change.ID, "artifact-delivery-change-decline-"+strconv.FormatInt(time.Now().UnixNano(), 10))
	if err != nil || declined.State != "declined" {
		t.Fatalf("declined blocked change request=%+v err=%v", declined, err)
	}
	if _, err = store.GetArtifactDeliveryPackage(ctx, companyID, artifactID); err != nil {
		t.Fatalf("declining the requirement change did not restore artifact delivery: %v", err)
	}
	restoredOverview, err := store.GetCompanyOverview(ctx, companyID)
	if err != nil {
		t.Fatal(err)
	}
	var restoredVerdict, restoredTaskAcceptance string
	for _, artifact := range restoredOverview.Artifacts {
		if artifact.ArtifactID == artifactID {
			restoredVerdict = artifact.Verdict
		}
	}
	for _, task := range restoredOverview.Tasks {
		if task.TaskID == taskID {
			restoredTaskAcceptance = task.Acceptance
		}
	}
	if restoredVerdict != previousVerdict || restoredTaskAcceptance != previousTaskAcceptance {
		t.Fatalf("declining the request did not restore authoritative Artifact/Task projection: before=(%q,%q) after=(%q,%q)", previousVerdict, previousTaskAcceptance, restoredVerdict, restoredTaskAcceptance)
	}
}

func mapKeys(values map[string][]byte) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	return keys
}

func TestTaskInputManifestReadProjectsPerFileDirectoryDeliveryReceipt(t *testing.T) {
	dsn := os.Getenv("POLIS_TEST_DSN")
	if dsn == "" {
		t.Skip("dedicated PostgreSQL required")
	}
	ctx := context.Background()
	blobRoot := t.TempDir()
	runtime, err := kernel.Open(ctx, dsn, blobRoot)
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Close()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	companyID := fmt.Sprintf("directory-delivery-read-%d", time.Now().UnixNano())
	scope, err := runtime.TXCreateCompany(ctx, companyID)
	if err != nil {
		t.Fatal(err)
	}
	mission, err := runtime.TXCreateMissionGoalWithAcceptance(ctx, scope, "Directory input delivery view", "verify per-file delivery readback", &taskvalidation.AcceptanceContract{
		Revision: taskvalidation.AcceptanceContractRevision, RequiredText: []string{"Mission ID: {{mission_id}}"},
	}, "directory-delivery-read-mission")
	if err != nil {
		t.Fatal(err)
	}
	files := []intake.DirectoryInputFile{
		{RelativePath: "project/README.md", MediaType: "text/markdown", Content: []byte("directory source text")},
		{RelativePath: "project/data.bin", MediaType: "application/octet-stream", Content: []byte("opaque sibling")},
	}
	preparedDirectory, err := intake.PrepareDirectorySnapshot(files)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = runtime.TXAddMissionInput(ctx, scope, mission.ID, "", "directory-delivery-read-input", preparedDirectory.Upload, preparedDirectory.Archive); err != nil {
		t.Fatal(err)
	}
	if _, err = runtime.TXStartMissionCommand(ctx, scope, mission.ID, "directory-delivery-read-start"); err != nil {
		t.Fatal(err)
	}
	task, err := runtime.TXPrepareProductTask(ctx, scope, mission.ID, "read the directory source", "directory-delivery-read-task")
	if err != nil {
		t.Fatal(err)
	}
	binding, err := runtime.TXNewProductProviderWorkerWithToolBudget(ctx, scope, task.ID, "offline-model/medium", 16)
	if err != nil {
		t.Fatal(err)
	}
	inputContext, err := runtime.ProductTaskInputContext(ctx, binding)
	if err != nil {
		t.Fatal(err)
	}
	refs := make([]intake.ModelInputDeliveryRef, 0, len(inputContext.Payload.Inputs))
	for _, item := range inputContext.Payload.Inputs {
		refs = append(refs, intake.ModelInputDeliveryRef{InputID: item.Reference.InputID, RelativePath: item.RelativePath, MediaType: item.MediaType, ByteSize: item.ByteSize, ContentDigest: item.ContentDigest})
	}
	refsJSON, err := json.Marshal(refs)
	if err != nil {
		t.Fatal(err)
	}
	exclusionsJSON, err := json.Marshal(inputContext.Payload.Excluded)
	if err != nil {
		t.Fatal(err)
	}
	deliveryID := fmt.Sprintf("directory-delivery-read-%d", time.Now().UnixNano())
	if _, err = pool.Exec(ctx, `INSERT INTO task_input_delivery_attempts(company_id,delivery_id,task_id,session_id,phase,outcome,manifest_digest,payload_digest,input_refs,input_exclusions,provider_egress)
VALUES($1,$2,$3,$4,'final','local_context_loaded',$5,$6,$7,$8,0)`, companyID, deliveryID, task.ID, binding.SessionID(), inputContext.ManifestDigest, inputContext.Payload.PayloadDigest, refsJSON, exclusionsJSON); err != nil {
		t.Fatal(err)
	}
	if err = runtime.TXFinalizeWorkerBeforeProcess(ctx, binding, "directory delivery projection complete", "directory-delivery-read-stop"); err != nil {
		t.Fatal(err)
	}
	store, err := workbench.NewPostgresReadStoreWithBlobRoot(ctx, dsn, blobRoot)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	view, err := store.GetTaskInputManifest(ctx, companyID, task.ID)
	if err != nil {
		t.Fatal(err)
	}
	if view.DeliveryStatus != "local_context_loaded" || len(view.IncludedInputPaths) != 1 || view.IncludedInputPaths[0].RelativePath != "project/README.md" || len(view.InputExclusions) != 1 || view.InputExclusions[0].RelativePath != "project/data.bin" {
		t.Fatalf("directory per-file delivery projection = %+v", view)
	}
}

func TestZIPInputIsDeliveredToWorkerAndReadBackPerFile(t *testing.T) {
	dsn := os.Getenv("POLIS_TEST_DSN")
	if dsn == "" {
		t.Skip("dedicated PostgreSQL required")
	}
	ctx := context.Background()
	blobRoot := t.TempDir()
	runtime, err := kernel.Open(ctx, dsn, blobRoot)
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Close()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	companyID := fmt.Sprintf("zip-delivery-read-%d", time.Now().UnixNano())
	scope, err := runtime.TXCreateCompany(ctx, companyID)
	if err != nil {
		t.Fatal(err)
	}
	mission, err := runtime.TXCreateMissionGoalWithAcceptance(ctx, scope, "ZIP input delivery", "deliver verified ZIP text to a worker", &taskvalidation.AcceptanceContract{
		Revision: taskvalidation.AcceptanceContractRevision, RequiredText: []string{"Mission ID: {{mission_id}}"},
	}, "zip-delivery-read-mission")
	if err != nil {
		t.Fatal(err)
	}
	var archive bytes.Buffer
	zipWriter := zip.NewWriter(&archive)
	for _, file := range []struct{ name, content string }{
		{"repo/README.md", "ZIP sentinel must reach the worker"},
		{"repo/data.bin", "opaque ZIP sibling"},
	} {
		entry, createErr := zipWriter.Create(file.name)
		if createErr != nil {
			t.Fatal(createErr)
		}
		if _, err = io.WriteString(entry, file.content); err != nil {
			t.Fatal(err)
		}
	}
	if err = zipWriter.Close(); err != nil {
		t.Fatal(err)
	}
	preparedZIP, err := intake.PrepareUpload("source.zip", "application/zip", archive.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	if preparedZIP.SourceKind != "zip_snapshot" || preparedZIP.State != intake.StatePartial {
		t.Fatalf("ZIP intake metadata = %+v", preparedZIP)
	}
	input, err := runtime.TXAddMissionInput(ctx, scope, mission.ID, "", "zip-delivery-read-input", preparedZIP, archive.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	pdfSource := readbackTestPDF("PDF_WORKBENCH_READBACK_SENTINEL")
	preparedPDF, pdfPackage, err := intake.PrepareMissionInput("source.pdf", "application/pdf", pdfSource)
	if err != nil {
		t.Fatal(err)
	}
	pdfInput, err := runtime.TXAddMissionInput(ctx, scope, mission.ID, "", "pdf-delivery-read-input", preparedPDF, pdfPackage)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = runtime.TXStartMissionCommand(ctx, scope, mission.ID, "zip-delivery-read-start"); err != nil {
		t.Fatal(err)
	}
	task, err := runtime.TXPrepareProductTask(ctx, scope, mission.ID, "read the ZIP project source", "zip-delivery-read-task")
	if err != nil {
		t.Fatal(err)
	}
	binding, err := runtime.TXNewProductProviderWorkerWithToolBudget(ctx, scope, task.ID, "offline-model/medium", 16)
	if err != nil {
		t.Fatal(err)
	}
	inputContext, err := runtime.ProductTaskInputContext(ctx, binding)
	if err != nil {
		t.Fatal(err)
	}
	if len(inputContext.Payload.Inputs) != 3 || !hasDeliveryInput(inputContext.Payload.Inputs, input.InputID, "repo/README.md", "ZIP sentinel must reach the worker") || !hasDeliveryInput(inputContext.Payload.Inputs, pdfInput.InputID, "pdf/extracted.txt", "PDF_WORKBENCH_READBACK_SENTINEL") || !strings.Contains(inputContext.Payload.PromptSection, "ZIP sentinel must reach the worker") || !strings.Contains(inputContext.Payload.PromptSection, "PDF_WORKBENCH_READBACK_SENTINEL") {
		t.Fatalf("ZIP text was not delivered into the worker input: %+v", inputContext.Payload)
	}
	if len(inputContext.Payload.Excluded) != 2 || !hasDeliveryExclusion(inputContext.Payload.Excluded, input.InputID, "repo/data.bin") || !hasDeliveryExclusion(inputContext.Payload.Excluded, pdfInput.InputID, "pdf/original.pdf") {
		t.Fatalf("unsupported ZIP/PDF source files were not classified: %+v", inputContext.Payload.Excluded)
	}
	refs := make([]intake.ModelInputDeliveryRef, 0, len(inputContext.Payload.Inputs))
	for _, item := range inputContext.Payload.Inputs {
		refs = append(refs, intake.ModelInputDeliveryRef{InputID: item.Reference.InputID, RelativePath: item.RelativePath, MediaType: item.MediaType, ByteSize: item.ByteSize, ContentDigest: item.ContentDigest})
	}
	refsJSON, err := json.Marshal(refs)
	if err != nil {
		t.Fatal(err)
	}
	exclusionsJSON, err := json.Marshal(inputContext.Payload.Excluded)
	if err != nil {
		t.Fatal(err)
	}
	deliveryID := fmt.Sprintf("zip-delivery-read-%d", time.Now().UnixNano())
	if _, err = pool.Exec(ctx, `INSERT INTO task_input_delivery_attempts(company_id,delivery_id,task_id,session_id,phase,outcome,manifest_digest,payload_digest,input_refs,input_exclusions,provider_egress)
VALUES($1,$2,$3,$4,'final','local_context_loaded',$5,$6,$7,$8,0)`, companyID, deliveryID, task.ID, binding.SessionID(), inputContext.ManifestDigest, inputContext.Payload.PayloadDigest, refsJSON, exclusionsJSON); err != nil {
		t.Fatal(err)
	}
	if err = runtime.TXFinalizeWorkerBeforeProcess(ctx, binding, "ZIP delivery projection complete", "zip-delivery-read-stop"); err != nil {
		t.Fatal(err)
	}
	store, err := workbench.NewPostgresReadStoreWithBlobRoot(ctx, dsn, blobRoot)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	view, err := store.GetTaskInputManifest(ctx, companyID, task.ID)
	if err != nil {
		t.Fatal(err)
	}
	if view.DeliveryStatus != "local_context_loaded" || len(view.IncludedInputPaths) != 3 || !hasReadbackPath(view.IncludedInputPaths, "repo/README.md") || !hasReadbackPath(view.IncludedInputPaths, "pdf/extracted.txt") || len(view.InputExclusions) != 2 || !hasReadbackExclusion(view.InputExclusions, "repo/data.bin") || !hasReadbackExclusion(view.InputExclusions, "pdf/original.pdf") || len(view.Manifest.CandidateInputs) != 2 || !hasCandidateSource(view.Manifest.CandidateInputs, "zip_snapshot") || !hasCandidateSource(view.Manifest.CandidateInputs, "pdf_snapshot") {
		t.Fatalf("ZIP/PDF per-file delivery projection = %+v", view)
	}
}

func readbackTestPDF(value string) []byte {
	content := "BT /F1 12 Tf 72 720 Td (" + value + ") Tj ET"
	return pdftest.Build(1,
		pdftest.Catalog(2),
		pdftest.Pages(3),
		pdftest.Page(2, 4, "<< /Font << /F1 5 0 R >> >>"),
		pdftest.Stream("", content),
		pdftest.Helvetica(),
	)
}

func hasDeliveryInput(inputs []intake.ModelInputPayload, inputID, path, marker string) bool {
	for _, input := range inputs {
		if input.Reference.InputID == inputID && input.RelativePath == path && strings.Contains(input.Text, marker) {
			return true
		}
	}
	return false
}

func hasDeliveryExclusion(exclusions []intake.ModelInputExclusion, inputID, path string) bool {
	for _, exclusion := range exclusions {
		if exclusion.InputID == inputID && exclusion.RelativePath == path && exclusion.Reason == "representation_not_supported" {
			return true
		}
	}
	return false
}

func hasReadbackPath(paths []intake.ModelInputDeliveryRef, path string) bool {
	for _, item := range paths {
		if item.RelativePath == path {
			return true
		}
	}
	return false
}

func hasReadbackExclusion(exclusions []intake.ModelInputExclusion, path string) bool {
	for _, item := range exclusions {
		if item.RelativePath == path && item.Reason == "representation_not_supported" {
			return true
		}
	}
	return false
}

func hasCandidateSource(inputs []intake.ModelInputManifestEntry, sourceKind string) bool {
	for _, item := range inputs {
		if item.SourceKind == sourceKind {
			return true
		}
	}
	return false
}

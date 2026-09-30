// pattern: Imperative Shell
package workbench_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"polis/internal/control"
	"polis/internal/intake"
	"polis/internal/kernel"
	"polis/internal/workbench"
)

func TestWorkbenchDirectoryInputIsScopedAndStoredAsOneSnapshot(t *testing.T) {
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
	suffix := fmt.Sprint(time.Now().UnixNano())
	companyID := "directory-http-" + suffix
	scope, err := runtime.TXCreateCompany(ctx, companyID)
	if err != nil {
		t.Fatal(err)
	}
	missionID := "mission-" + suffix
	if err = runtime.TXCreateMission(ctx, scope, missionID); err != nil {
		t.Fatal(err)
	}
	readStore, err := workbench.NewPostgresReadStoreWithBlobRoot(ctx, dsn, root)
	if err != nil {
		t.Fatal(err)
	}
	defer readStore.Close()
	handler := workbench.NewHandler(readStore, control.NewService(runtime, nil))

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	for _, field := range [][2]string{{"requestId", "directory-http-1"}, {"paths", "project/README.md"}} {
		if err = writer.WriteField(field[0], field[1]); err != nil {
			t.Fatal(err)
		}
	}
	part, err := writer.CreateFormFile("files", "README.md")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = part.Write([]byte("# Project\n")); err != nil {
		t.Fatal(err)
	}
	if err = writer.Close(); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/workbench/companies/"+companyID+"/missions/"+missionID+"/inputs/directory", &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusAccepted {
		t.Fatalf("directory upload status = %d: %s", response.Code, response.Body.String())
	}
	var receipt control.MissionInputCommandReceipt
	if err = json.NewDecoder(response.Body).Decode(&receipt); err != nil {
		t.Fatal(err)
	}
	if receipt.CompanyID != companyID || receipt.MissionID != missionID || receipt.RequestID != "directory-http-1" || receipt.SourceKind != "directory_snapshot" || receipt.DisplayName != "project" || receipt.State != string(intake.StateUsable) {
		t.Fatalf("directory upload receipt = %+v", receipt)
	}
	archive, err := os.ReadFile(filepath.Join(root, companyID, receipt.ContentDigest))
	if err != nil {
		t.Fatal(err)
	}
	prepared := intake.PreparedUpload{SourceKind: receipt.SourceKind, DisplayName: receipt.DisplayName, MediaType: receipt.MediaType, ByteSize: receipt.ByteSize, ContentDigest: receipt.ContentDigest, State: intake.State(receipt.State)}
	if err = intake.VerifyPreparedUpload(prepared, archive); err != nil {
		t.Fatalf("stored directory snapshot did not verify: %v", err)
	}

	listRequest := httptest.NewRequest(http.MethodGet, "/api/workbench/companies/"+companyID+"/missions/"+missionID+"/inputs", nil)
	listResponse := httptest.NewRecorder()
	handler.ServeHTTP(listResponse, listRequest)
	if listResponse.Code != http.StatusOK {
		t.Fatalf("directory input list status = %d: %s", listResponse.Code, listResponse.Body.String())
	}
	var listed []workbench.MissionInputView
	if err = json.NewDecoder(listResponse.Body).Decode(&listed); err != nil {
		t.Fatal(err)
	}
	if len(listed) != 1 || listed[0].SourceKind != "directory_snapshot" || listed[0].ContentDigest != receipt.ContentDigest {
		t.Fatalf("listed directory inputs = %+v", listed)
	}
	mission, err := runtime.MissionDetails(ctx, scope, missionID)
	if err != nil {
		t.Fatal(err)
	}
	if mission.State != "draft" {
		t.Fatalf("directory upload activated Mission: %q", mission.State)
	}
	if _, err = runtime.TXStartMissionCommand(ctx, scope, missionID, "directory-http-start"); err != nil {
		t.Fatal(err)
	}
	task, err := runtime.TXPrepareProductTask(ctx, scope, missionID, "read the bound directory snapshot", "directory-http-task-prepare")
	if err != nil {
		t.Fatal(err)
	}
	manifestRequest := httptest.NewRequest(http.MethodGet, "/api/workbench/companies/"+companyID+"/tasks/"+task.ID+"/input-manifest", nil)
	manifestResponse := httptest.NewRecorder()
	handler.ServeHTTP(manifestResponse, manifestRequest)
	if manifestResponse.Code != http.StatusOK {
		t.Fatalf("Task input manifest status = %d: %s", manifestResponse.Code, manifestResponse.Body.String())
	}
	var manifestView workbench.TaskInputManifestView
	if err = json.NewDecoder(manifestResponse.Body).Decode(&manifestView); err != nil {
		t.Fatal(err)
	}
	if manifestView.TaskID != task.ID || manifestView.ManifestDigest == "" || manifestView.DeliveryStatus != "not_delivered" || len(manifestView.Manifest.CandidateInputs) != 1 || manifestView.Manifest.CandidateInputs[0].SourceKind != "directory_snapshot" {
		t.Fatalf("Task input manifest view = %+v", manifestView)
	}
}

// pattern: Imperative Shell
package workbench_test

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/giraffesyo/pdf/pdftest"
	"polis/internal/control"
	"polis/internal/intake"
	"polis/internal/kernel"
	"polis/internal/workbench"
)

func TestWorkbenchMissionInputUploadStoresAndListsWithoutStartingMission(t *testing.T) {
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

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	companyID := "input-http-" + suffix
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

	content := []byte("# Mission goal\nKeep the source unchanged.\n")
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	requestID := "input-http-" + suffix
	if err = writer.WriteField("requestId", requestID); err != nil {
		t.Fatal(err)
	}
	file, err := writer.CreateFormFile("file", "goal.md")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = file.Write(content); err != nil {
		t.Fatal(err)
	}
	if err = writer.Close(); err != nil {
		t.Fatal(err)
	}
	uploadRequest := httptest.NewRequest(http.MethodPost, "/api/workbench/companies/"+companyID+"/missions/"+missionID+"/inputs", &body)
	uploadRequest.Header.Set("Content-Type", writer.FormDataContentType())
	uploadResponse := httptest.NewRecorder()
	handler.ServeHTTP(uploadResponse, uploadRequest)
	if uploadResponse.Code != http.StatusAccepted {
		t.Fatalf("upload status = %d: %s", uploadResponse.Code, uploadResponse.Body.String())
	}
	var receipt control.MissionInputCommandReceipt
	if err = json.NewDecoder(uploadResponse.Body).Decode(&receipt); err != nil {
		t.Fatal(err)
	}
	if receipt.CompanyID != companyID || receipt.MissionID != missionID || receipt.RequestID != requestID || receipt.Revision != 1 || receipt.State != "usable" {
		t.Fatalf("upload receipt = %+v", receipt)
	}
	stored, err := os.ReadFile(filepath.Join(root, companyID, receipt.ContentDigest))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(stored, content) {
		t.Fatal("CAS input differs from uploaded source")
	}

	var zipBytes bytes.Buffer
	zipWriter := zip.NewWriter(&zipBytes)
	entry, err := zipWriter.Create("repo/README.md")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = io.WriteString(entry, "bounded ZIP source"); err != nil {
		t.Fatal(err)
	}
	if err = zipWriter.Close(); err != nil {
		t.Fatal(err)
	}
	var zipBody bytes.Buffer
	zipForm := multipart.NewWriter(&zipBody)
	zipRequestID := "zip-input-http-" + suffix
	if err = zipForm.WriteField("requestId", zipRequestID); err != nil {
		t.Fatal(err)
	}
	zipFile, err := zipForm.CreateFormFile("file", "project.zip")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = zipFile.Write(zipBytes.Bytes()); err != nil {
		t.Fatal(err)
	}
	if err = zipForm.Close(); err != nil {
		t.Fatal(err)
	}
	zipRequest := httptest.NewRequest(http.MethodPost, "/api/workbench/companies/"+companyID+"/missions/"+missionID+"/inputs", &zipBody)
	zipRequest.Header.Set("Content-Type", zipForm.FormDataContentType())
	zipResponse := httptest.NewRecorder()
	handler.ServeHTTP(zipResponse, zipRequest)
	if zipResponse.Code != http.StatusAccepted {
		t.Fatalf("ZIP upload status = %d: %s", zipResponse.Code, zipResponse.Body.String())
	}
	var zipReceipt control.MissionInputCommandReceipt
	if err = json.NewDecoder(zipResponse.Body).Decode(&zipReceipt); err != nil {
		t.Fatal(err)
	}
	if zipReceipt.RequestID != zipRequestID || zipReceipt.SourceKind != "zip_snapshot" || zipReceipt.State != "usable" {
		t.Fatalf("ZIP upload receipt = %+v", zipReceipt)
	}

	pdfRequestID := "pdf-input-http-" + suffix
	pdfBytes := testWorkbenchPDF("PDF_HTTP_SOURCE_SENTINEL")
	var pdfBody bytes.Buffer
	pdfForm := multipart.NewWriter(&pdfBody)
	if err = pdfForm.WriteField("requestId", pdfRequestID); err != nil {
		t.Fatal(err)
	}
	pdfFile, err := pdfForm.CreateFormFile("file", "brief.pdf")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = pdfFile.Write(pdfBytes); err != nil {
		t.Fatal(err)
	}
	if err = pdfForm.Close(); err != nil {
		t.Fatal(err)
	}
	pdfRequest := httptest.NewRequest(http.MethodPost, "/api/workbench/companies/"+companyID+"/missions/"+missionID+"/inputs", &pdfBody)
	pdfRequest.Header.Set("Content-Type", pdfForm.FormDataContentType())
	pdfResponse := httptest.NewRecorder()
	handler.ServeHTTP(pdfResponse, pdfRequest)
	if pdfResponse.Code != http.StatusAccepted {
		t.Fatalf("PDF upload status = %d: %s", pdfResponse.Code, pdfResponse.Body.String())
	}
	var pdfReceipt control.MissionInputCommandReceipt
	if err = json.NewDecoder(pdfResponse.Body).Decode(&pdfReceipt); err != nil {
		t.Fatal(err)
	}
	if pdfReceipt.RequestID != pdfRequestID || pdfReceipt.SourceKind != "pdf_snapshot" || pdfReceipt.State != string(intake.StatePartial) {
		t.Fatalf("PDF upload receipt = %+v", pdfReceipt)
	}
	storedPDF, err := os.ReadFile(filepath.Join(root, companyID, pdfReceipt.ContentDigest))
	if err != nil {
		t.Fatal(err)
	}
	if err = intake.VerifyPreparedUpload(intake.PreparedUpload{SourceKind: pdfReceipt.SourceKind, DisplayName: pdfReceipt.DisplayName, MediaType: pdfReceipt.MediaType, ByteSize: pdfReceipt.ByteSize, ContentDigest: pdfReceipt.ContentDigest, State: intake.State(pdfReceipt.State)}, storedPDF); err != nil {
		t.Fatalf("stored HTTP PDF package did not verify: %v", err)
	}

	listRequest := httptest.NewRequest(http.MethodGet, "/api/workbench/companies/"+companyID+"/missions/"+missionID+"/inputs", nil)
	listResponse := httptest.NewRecorder()
	handler.ServeHTTP(listResponse, listRequest)
	if listResponse.Code != http.StatusOK {
		t.Fatalf("list status = %d: %s", listResponse.Code, listResponse.Body.String())
	}
	var items []workbench.MissionInputView
	if err = json.NewDecoder(listResponse.Body).Decode(&items); err != nil {
		t.Fatal(err)
	}
	if len(items) != 3 || items[0].CompanyID != companyID || items[0].MissionID != missionID || items[0].RequestID != pdfRequestID || items[0].SourceKind != "pdf_snapshot" || items[0].DisplayName != "brief.pdf" || items[1].RequestID != zipRequestID || items[1].SourceKind != "zip_snapshot" || items[2].RequestID != requestID || items[2].DisplayName != "goal.md" {
		t.Fatalf("listed inputs = %+v", items)
	}
	mission, err := runtime.MissionDetails(ctx, scope, missionID)
	if err != nil {
		t.Fatal(err)
	}
	if mission.State != "draft" {
		t.Fatalf("upload changed mission state to %q", mission.State)
	}
}

func testWorkbenchPDF(value string) []byte {
	content := "BT /F1 12 Tf 72 720 Td (" + value + ") Tj ET"
	return pdftest.Build(1,
		pdftest.Catalog(2),
		pdftest.Pages(3),
		pdftest.Page(2, 4, "<< /Font << /F1 5 0 R >> >>"),
		pdftest.Stream("", content),
		pdftest.Helvetica(),
	)
}

// pattern: Imperative Shell
package workbench

import (
	"bytes"
	"context"
	"errors"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"polis/internal/control"
	"polis/internal/core"
	"polis/internal/kernel"
)

type fakeStdioMCPPackageImportService struct {
	fakeCommandService
	companyID string
	request   control.ImportStdioMCPPackageRequest
}

func (service *fakeStdioMCPPackageImportService) ImportStdioMCPPackage(_ context.Context, companyID string, request control.ImportStdioMCPPackageRequest) (kernel.StdioMCPPackageRevision, error) {
	service.companyID = companyID
	service.request = request
	return kernel.StdioMCPPackageRevision{CompanyID: companyID, ID: "mcp-package-revision-1", ServerID: request.ServerID, Revision: request.Revision, ManifestDigest: "digest", Manifest: []byte(`{}`)}, nil
}

func TestHandlerImportsStdioMCPPackageWithoutAcceptingClientDigests(t *testing.T) {
	service := &fakeStdioMCPPackageImportService{}
	archive := []byte("zip bytes")
	body, contentType := skillPackageMultipart(t, []string{"revision", "requestId"}, []string{"1.0.0", "mcp-package-import-1"}, "mcp-package.zip", archive)
	request := httptest.NewRequest(http.MethodPost, "/api/workbench/companies/company-1/capabilities/mcp-packages", body)
	request.Header.Set("Content-Type", contentType)
	recorder := httptest.NewRecorder()
	NewHandler(fakeReadModel{}, service).ServeHTTP(recorder, request)
	if recorder.Code != http.StatusAccepted {
		t.Fatalf("MCP package import HTTP status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	if service.companyID != "company-1" || service.request.ServerID != "" || service.request.Revision != "1.0.0" || service.request.RequestID != "mcp-package-import-1" || !bytes.Equal(service.request.Archive, archive) {
		t.Fatalf("MCP package import service scope/request=(%q,%+v)", service.companyID, service.request)
	}
}

func TestHandlerImportsStdioMCPPackageForAnExistingDefinition(t *testing.T) {
	service := &fakeStdioMCPPackageImportService{}
	body, contentType := skillPackageMultipart(t, []string{"serverId", "revision", "requestId"}, []string{"mcp-server-1", "2.0.0", "mcp-package-import-2"}, "mcp-package.zip", []byte("zip bytes"))
	request := httptest.NewRequest(http.MethodPost, "/api/workbench/companies/company-1/capabilities/mcp-packages", body)
	request.Header.Set("Content-Type", contentType)
	recorder := httptest.NewRecorder()
	NewHandler(fakeReadModel{}, service).ServeHTTP(recorder, request)
	if recorder.Code != http.StatusAccepted || service.request.ServerID != "mcp-server-1" {
		t.Fatalf("existing-definition package import response=%d serverId=%q body=%s", recorder.Code, service.request.ServerID, recorder.Body.String())
	}
}

func TestHandlerRejectsUnexpectedMCPPackageMultipartFields(t *testing.T) {
	service := &fakeStdioMCPPackageImportService{}
	body, contentType := skillPackageMultipart(t, []string{"revision", "requestId", "descriptorDigest"}, []string{"1.0.0", "mcp-package-import-3", "caller-digest"}, "mcp-package.zip", []byte("zip bytes"))
	request := httptest.NewRequest(http.MethodPost, "/api/workbench/companies/company-1/capabilities/mcp-packages", body)
	request.Header.Set("Content-Type", contentType)
	recorder := httptest.NewRecorder()
	NewHandler(fakeReadModel{}, service).ServeHTTP(recorder, request)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("unexpected package digest multipart field status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestHandlerBoundsStdioMCPPackageUpload(t *testing.T) {
	service := &fakeStdioMCPPackageImportService{}
	body, contentType := skillPackageMultipart(t, []string{"revision", "requestId"}, []string{"1.0.0", "mcp-package-large"}, "large.zip", bytes.Repeat([]byte{'x'}, (8<<20)+1))
	request := httptest.NewRequest(http.MethodPost, "/api/workbench/companies/company-1/capabilities/mcp-packages", body)
	request.Header.Set("Content-Type", contentType)
	recorder := httptest.NewRecorder()
	NewHandler(fakeReadModel{}, service).ServeHTTP(recorder, request)
	if recorder.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("oversized MCP package upload status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestParseStdioMCPPackageUploadRemovesSpillFilesAfterRejectingMultipleFiles(t *testing.T) {
	spillRoot := filepath.Join(t.TempDir(), "multipart-spill")
	if err := os.Mkdir(spillRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TMPDIR", spillRoot)
	t.Setenv("TEMP", spillRoot)
	t.Setenv("TMP", spillRoot)

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	for _, field := range []string{"bundle", "extra"} {
		part, err := writer.CreateFormFile(field, field+".zip")
		if err != nil {
			t.Fatal(err)
		}
		if _, err = part.Write(bytes.Repeat([]byte{'x'}, 64<<10)); err != nil {
			t.Fatal(err)
		}
	}
	for key, value := range map[string]string{"revision": "1.0.0", "requestId": "mcp-upload-multiple-files"} {
		if err := writer.WriteField(key, value); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/workbench/companies/company-1/capabilities/mcp-packages", &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	_, err := parseStdioMCPPackageUpload(httptest.NewRecorder(), request)
	if !errors.Is(err, core.Malformed) {
		t.Fatalf("multiple package files error=%v, want %s", err, core.Malformed)
	}
	entries, err := os.ReadDir(spillRoot)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("multipart parser left spill files after rejecting request: %v", entries)
	}
}

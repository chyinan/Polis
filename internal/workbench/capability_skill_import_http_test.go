// pattern: Imperative Shell
package workbench

import (
	"bytes"
	"context"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"polis/internal/control"
	"polis/internal/kernel"
)

type fakeSkillPackageImportService struct {
	fakeCommandService
	companyID string
	request   control.ImportReadOnlySkillPackageRequest
}

func (f *fakeSkillPackageImportService) ImportReadOnlySkillPackage(_ context.Context, companyID string, request control.ImportReadOnlySkillPackageRequest) (kernel.SkillRevision, error) {
	f.companyID = companyID
	f.request = request
	return kernel.SkillRevision{CompanyID: companyID, ID: "skill-imported", PackageID: "design-review", Revision: request.Revision, ContentDigest: "digest", Status: "candidate"}, nil
}

func TestHandlerImportsReadOnlySkillPackageFromMultipartWithoutClientDigests(t *testing.T) {
	service := &fakeSkillPackageImportService{}
	body, contentType := skillPackageMultipart(t, []string{"revision", "requestId"}, []string{"1.0.0", "skill-import-1"}, "design-review.zip", []byte("zip bytes"))
	request := httptest.NewRequest(http.MethodPost, "/api/workbench/companies/company-1/capabilities/skills", body)
	request.Header.Set("Content-Type", contentType)
	recorder := httptest.NewRecorder()
	NewHandler(fakeReadModel{}, service).ServeHTTP(recorder, request)
	if recorder.Code != http.StatusAccepted {
		t.Fatalf("Skill import status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	if service.companyID != "company-1" || service.request.Revision != "1.0.0" || service.request.RequestID != "skill-import-1" || string(service.request.Archive) != "zip bytes" {
		t.Fatalf("Skill package import crossed scope or changed bytes: company=%q request=%+v", service.companyID, service.request)
	}
}

func TestHandlerRejectsUnexpectedSkillPackageMultipartFields(t *testing.T) {
	service := &fakeSkillPackageImportService{}
	body, contentType := skillPackageMultipart(t, []string{"revision", "requestId", "contentDigest"}, []string{"1.0.0", "skill-import-1", "caller-digest"}, "design-review.zip", []byte("zip bytes"))
	request := httptest.NewRequest(http.MethodPost, "/api/workbench/companies/company-1/capabilities/skills", body)
	request.Header.Set("Content-Type", contentType)
	recorder := httptest.NewRecorder()
	NewHandler(fakeReadModel{}, service).ServeHTTP(recorder, request)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("unexpected field status=%d body=%s, want 400", recorder.Code, recorder.Body.String())
	}
	if service.companyID != "" {
		t.Fatal("unexpected multipart fields reached the import service")
	}
}

func TestHandlerBoundsSkillPackageUploadBody(t *testing.T) {
	service := &fakeSkillPackageImportService{}
	body, contentType := skillPackageMultipart(t, []string{"revision", "requestId"}, []string{"1.0.0", "skill-import-large"}, "large.zip", bytes.Repeat([]byte{'x'}, (8<<20)+1))
	request := httptest.NewRequest(http.MethodPost, "/api/workbench/companies/company-1/capabilities/skills", body)
	request.Header.Set("Content-Type", contentType)
	recorder := httptest.NewRecorder()
	NewHandler(fakeReadModel{}, service).ServeHTTP(recorder, request)
	if recorder.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("oversized Skill package status=%d body=%s, want 413", recorder.Code, recorder.Body.String())
	}
	if service.companyID != "" {
		t.Fatal("oversized Skill package reached the import service")
	}
}

func skillPackageMultipart(t *testing.T, fieldNames, fieldValues []string, filename string, content []byte) (*bytes.Buffer, string) {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	for index, name := range fieldNames {
		if err := writer.WriteField(name, fieldValues[index]); err != nil {
			t.Fatal(err)
		}
	}
	file, err := writer.CreateFormFile("bundle", filename)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = file.Write(content); err != nil {
		t.Fatal(err)
	}
	if err = writer.Close(); err != nil {
		t.Fatal(err)
	}
	return &body, writer.FormDataContentType()
}

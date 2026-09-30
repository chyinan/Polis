// pattern: Imperative Shell
package workbench

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"polis/internal/intake"
)

func TestHandlerRejectsOversizedMissionInputBeforeCallingService(t *testing.T) {
	service := &fakeMissionInputCommandService{}
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if err := writer.WriteField("requestId", "upload-too-large"); err != nil {
		t.Fatal(err)
	}
	file, err := writer.CreateFormFile("file", "large.txt")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = file.Write(bytes.Repeat([]byte{'x'}, intake.MaxUploadBytes+1)); err != nil {
		t.Fatal(err)
	}
	if err = writer.Close(); err != nil {
		t.Fatal(err)
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/workbench/companies/company-1/missions/mission-1/inputs", &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	NewHandler(fakeReadModel{}, service).ServeHTTP(recorder, request)
	if recorder.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("oversized upload status = %d: %s", recorder.Code, recorder.Body.String())
	}
	if service.filename != "" || len(service.content) != 0 {
		t.Fatal("oversized upload reached the command service")
	}
}

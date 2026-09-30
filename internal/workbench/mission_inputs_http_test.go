// pattern: Imperative Shell
package workbench

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"polis/internal/control"
)

type fakeMissionInputReader struct {
	fakeReadModel
	companyID string
	missionID string
	items     []MissionInputView
}

func (f fakeMissionInputReader) ListMissionInputs(_ context.Context, companyID, missionID string) ([]MissionInputView, error) {
	if companyID != f.companyID || missionID != f.missionID {
		return nil, errCompanyNotFound
	}
	return f.items, nil
}

type fakeMissionInputCommandService struct {
	fakeCommandService
	request   control.UploadMissionInputRequest
	companyID string
	filename  string
	mediaType string
	content   []byte
}

func (f *fakeMissionInputCommandService) UploadMissionInput(_ context.Context, companyID string, request control.UploadMissionInputRequest, filename, mediaType string, content []byte) (control.MissionInputCommandReceipt, error) {
	f.companyID = companyID
	f.request = request
	f.filename = filename
	f.mediaType = mediaType
	f.content = append([]byte(nil), content...)
	return control.MissionInputCommandReceipt{CompanyID: companyID, InputID: "input-1", MissionID: request.MissionID, Revision: 1, RequestID: request.RequestID, SourceKind: "upload", DisplayName: filename, MediaType: mediaType, ByteSize: int64(len(content)), ContentDigest: "abc", State: "usable"}, nil
}

func TestHandlerListsMissionInputRevisionsInCompanyScope(t *testing.T) {
	model := fakeMissionInputReader{
		companyID: "company-1",
		missionID: "mission-1",
		items:     []MissionInputView{{CompanyID: "company-1", InputID: "input-1", MissionID: "mission-1", RequestID: "read-1", Revision: "1", SourceKind: "upload", DisplayName: "goal.md", MediaType: "text/markdown", ByteSize: "25", ContentDigest: "abc", State: "usable", CreatedAt: "2026-09-23T00:00:00Z"}},
	}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/workbench/companies/company-1/missions/mission-1/inputs", nil)
	NewHandler(model).ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("GET inputs status = %d: %s", recorder.Code, recorder.Body.String())
	}
	var items []MissionInputView
	if err := json.NewDecoder(recorder.Body).Decode(&items); err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].InputID != "input-1" || items[0].DisplayName != "goal.md" {
		t.Fatalf("unexpected mission inputs: %+v", items)
	}
}

func TestHandlerUploadsMissionInputWithoutStartingMission(t *testing.T) {
	service := &fakeMissionInputCommandService{}
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	_ = writer.WriteField("requestId", "upload-1")
	_ = writer.WriteField("inputId", "")
	file, err := writer.CreateFormFile("file", "goal.md")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.Write([]byte("# goal\n")); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/workbench/companies/company-1/missions/mission-1/inputs", &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	NewHandler(fakeReadModel{}, service).ServeHTTP(recorder, request)
	if recorder.Code != http.StatusAccepted {
		t.Fatalf("POST inputs status = %d: %s", recorder.Code, recorder.Body.String())
	}
	if service.companyID != "company-1" || service.request.MissionID != "mission-1" || service.request.RequestID != "upload-1" || service.filename != "goal.md" || string(service.content) != "# goal\n" {
		t.Fatalf("upload request crossed incorrect scope or metadata: %+v", service)
	}
	if service.mediaType != "application/octet-stream" && service.mediaType != "text/markdown" && service.mediaType != "text/plain" {
		t.Fatalf("unexpected browser media type: %q", service.mediaType)
	}
}

// pattern: Imperative Shell
package workbench

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"polis/internal/control"
	"polis/internal/core"
	"polis/internal/organization"
)

type fakeReadModel struct {
	overview CompanyOverviewView
	activity ActivityView
}

type fakeStreamingReadModel struct {
	fakeReadModel
	event ActivityEvent
}

type fakeCollaborationReadModel struct {
	fakeReadModel
}

type fakeWorkspaceReadModel struct {
	fakeReadModel
}

func (fakeWorkspaceReadModel) GetOperations(context.Context, string) (OperationsView, error) {
	return OperationsView{CompanyID: "company-1", ToolCallsUsed: "3", ToolCallsLimit: "8", ToolBudgetQuality: "reported", InputTokens: nil, OutputTokens: nil, ElapsedRuntime: nil, PostgreSQLStatus: "ready", CASStatus: "ready", EventStreamStatus: "ready", LastRuntimeError: nil}, nil
}

func (fakeWorkspaceReadModel) GetWorkspace(context.Context, string, string) (WorkspaceView, error) {
	return WorkspaceView{TaskID: "task-1", Revision: "3", Digest: "digest-1", Files: []WorkspaceFile{{Path: "workspace.txt", Bytes: "12", Content: "hello Polis\n"}}, ChangedFiles: []string{"workspace.txt"}, Checkpoints: []CheckpointSummary{}}, nil
}

func (fakeWorkspaceReadModel) GetArtifact(context.Context, string, string) (ArtifactDetailView, error) {
	return ArtifactDetailView{ArtifactID: "artifact-1", TaskID: "task-1", Digest: "digest-1", Bytes: "12", State: "ready", Verdict: "candidate", Content: "hello Polis\n", ContentAvailable: true}, nil
}

func (fakeCollaborationReadModel) ListCollaboration(context.Context, string) ([]CollaborationItem, error) {
	return []CollaborationItem{{MessageID: "message-1", MissionID: "mission-1", TaskID: "task-1", SenderEmployeeID: "emp-backend", RecipientEmployeeID: "emp-frontend", Content: "Please apply the accepted contract.", Kind: "request", DeliveryState: "acknowledged", ContractRevisionID: stringPointer("revision-1"), ObligationID: stringPointer("obligation-1"), ObligationState: stringPointer("pending"), EvidenceRef: nil, TaskRevision: "2"}}, nil
}

func (f fakeStreamingReadModel) StreamActivity(_ context.Context, _ string, _ int64, emit func(ActivityEvent) error) error {
	return emit(f.event)
}

func (f fakeReadModel) GetCompanyOverview(context.Context, string) (CompanyOverviewView, error) {
	return f.overview, nil
}

func (f fakeReadModel) ListActivity(context.Context, ActivityQuery) (ActivityView, error) {
	return f.activity, nil
}

func TestHandlerServesScopedOverview(t *testing.T) {
	model := fakeReadModel{overview: CompanyOverviewView{Meta: ViewMeta{CompanyID: "company-1"}}}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/workbench/companies/company-1/overview", nil)

	NewHandler(model).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	var response CompanyOverviewView
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Meta.CompanyID != "company-1" {
		t.Fatalf("company id = %q, want company-1", response.Meta.CompanyID)
	}
}

func TestHandlerPreservesActivityPaginationShape(t *testing.T) {
	model := fakeReadModel{activity: ActivityView{NextCursor: stringPointer("67")}}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/workbench/companies/company-1/activity?snapshot_cursor=company-seq:70&limit=3", nil)

	NewHandler(model).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	var raw map[string]any
	if err := json.NewDecoder(recorder.Body).Decode(&raw); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if raw["nextCursor"] != "67" {
		t.Fatalf("nextCursor = %#v, want exact next cursor 67", raw["nextCursor"])
	}
}

func TestHandlerRejectsUnboundedActivityLimit(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/workbench/companies/company-1/activity?snapshot_cursor=company-seq:70&limit=101", nil)

	NewHandler(fakeReadModel{}).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusBadRequest)
	}
}

func TestHandlerStreamsScopedActivityWithSequenceID(t *testing.T) {
	event := ActivityEvent{ID: "event-1", CompanySeq: "71", OccurredAt: "2026-09-21T00:00:00Z", Kind: "mission_started", Actor: ActivityActor{Kind: "system", ID: "controller", Label: "控制面"}, Subject: EntityRef{Kind: "company", ID: "company-1", Label: "Company"}, Summary: "mission started", Detail: "started", Tone: "info", EvidenceRefs: []string{}, Metadata: map[string]string{}}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/workbench/companies/company-1/stream?cursor=company-seq:70", nil)
	NewHandler(fakeStreamingReadModel{event: event}).ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	if !strings.Contains(recorder.Body.String(), "id: 71\nevent: activity\n") {
		t.Fatalf("SSE framing = %q", recorder.Body.String())
	}
}

func TestHandlerServesCollaborationProjection(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/workbench/companies/company-1/collaboration", nil)
	NewHandler(fakeCollaborationReadModel{}).ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	var result []CollaborationItem
	if err := json.NewDecoder(recorder.Body).Decode(&result); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(result) != 1 || result[0].ObligationState == nil || *result[0].ObligationState != "pending" {
		t.Fatalf("collaboration = %+v", result)
	}
}

func TestHandlerServesAuthorizedWorkspacePreview(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/workbench/companies/company-1/tasks/task-1/workspace", nil)
	NewHandler(fakeWorkspaceReadModel{}).ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	var result WorkspaceView
	if err := json.NewDecoder(recorder.Body).Decode(&result); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(result.Files) != 1 || result.Files[0].Path != "workspace.txt" {
		t.Fatalf("workspace preview = %+v", result)
	}
}

func TestHandlerServesOperationsProjection(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/workbench/companies/company-1/operations", nil)
	NewHandler(fakeWorkspaceReadModel{}).ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	var result OperationsView
	if err := json.NewDecoder(recorder.Body).Decode(&result); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if result.ToolBudgetQuality != "reported" || result.InputTokens != nil {
		t.Fatalf("operations projection = %+v", result)
	}
}

type fakeCommandService struct {
	receipt control.CommandReceipt
	err     error
}

type fakeNotificationCommandService struct {
	fakeCommandService
	testReceipt control.NotificationTestReceipt
}

func (f fakeNotificationCommandService) ConfigureNotificationRoute(context.Context, string, control.ConfigureNotificationRouteRequest) (control.CommandReceipt, error) {
	return control.CommandReceipt{CommandID: "route-1", CommandType: "notification.route.update", TargetType: "company", TargetID: "company-1", RequestID: "route-1", Accepted: true, AcceptedAt: "2026-09-22T00:00:00Z", ResultingState: "configured"}, nil
}

func (f fakeNotificationCommandService) TestNotification(context.Context, string, string) (control.NotificationTestReceipt, error) {
	return f.testReceipt, nil
}

func (f fakeCommandService) CreateMission(context.Context, string, control.CreateMissionRequest) (control.CommandReceipt, error) {
	return f.receipt, f.err
}

func (f fakeCommandService) StartMission(context.Context, string, control.MissionCommandRequest) (control.CommandReceipt, error) {
	return f.receipt, f.err
}

func (f fakeCommandService) CancelMission(context.Context, string, control.MissionCommandRequest) (control.CommandReceipt, error) {
	return f.receipt, f.err
}

func TestHTTPAcceptsMissionCreateCommand(t *testing.T) {
	receipt := control.CommandReceipt{CommandID: "request-1", CommandType: "mission.create", TargetType: "mission", TargetID: "mission-1", RequestID: "request-1", Accepted: true, AcceptedAt: "2026-09-16T00:00:00Z", ResultingState: "draft"}
	body, err := json.Marshal(control.CreateMissionRequest{Title: "goal", Goal: "body", RequestID: "request-1"})
	if err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/workbench/companies/company-1/missions", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	NewHandler(fakeReadModel{}, fakeCommandService{receipt: receipt}).ServeHTTP(recorder, request)
	if recorder.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusAccepted)
	}
	if recorder.Body.String() == "" {
		t.Fatal("missing command receipt")
	}
}

func TestHTTPReturnsStructuredMissionConflict(t *testing.T) {
	body := []byte(`{"requestId":"request-2"}`)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/workbench/companies/company-1/missions/mission-1/start", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	NewHandler(fakeReadModel{}, fakeCommandService{err: core.Conflict}).ServeHTTP(recorder, request)
	if recorder.Code != http.StatusConflict {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusConflict)
	}
	var result map[string]string
	if err := json.Unmarshal(recorder.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result["code"] != string(core.Conflict) || result["targetId"] != "mission-1" {
		t.Fatalf("structured conflict = %+v", result)
	}
}

func TestHTTPAcceptsNotificationRouteCommand(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/workbench/companies/company-1/notifications/route", bytes.NewReader([]byte(`{"adapter":"local","destination":"local://workbench","enabled":true,"requestId":"route-1"}`)))
	request.Header.Set("Content-Type", "application/json")
	NewHandler(fakeReadModel{}, fakeNotificationCommandService{}).ServeHTTP(recorder, request)
	if recorder.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusAccepted)
	}
}

func TestHTTPAcceptsNotificationTestCommand(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/workbench/companies/company-1/notifications/test", bytes.NewReader([]byte(`{"requestId":"test-1"}`)))
	request.Header.Set("Content-Type", "application/json")
	NewHandler(fakeReadModel{}, fakeNotificationCommandService{testReceipt: control.NotificationTestReceipt{IntentID: "intent-1", DeliveryID: "delivery-1", Adapter: "local", State: "delivered", RequestID: "test-1", Accepted: true, AcceptedAt: "2026-09-22T00:00:00Z"}}).ServeHTTP(recorder, request)
	if recorder.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusAccepted)
	}
}

type fakeOrganizationCommandService struct{}

func (fakeOrganizationCommandService) CreateMission(context.Context, string, control.CreateMissionRequest) (control.CommandReceipt, error) {
	return control.CommandReceipt{}, nil
}

func (fakeOrganizationCommandService) StartMission(context.Context, string, control.MissionCommandRequest) (control.CommandReceipt, error) {
	return control.CommandReceipt{}, nil
}

func (fakeOrganizationCommandService) CancelMission(context.Context, string, control.MissionCommandRequest) (control.CommandReceipt, error) {
	return control.CommandReceipt{}, nil
}

func (fakeOrganizationCommandService) ListCompanies(context.Context) ([]control.CompanySummary, error) {
	return []control.CompanySummary{{ID: "company-1", Name: "Company One", State: "active", Roster: organization.DefaultRoster()}}, nil
}

func (fakeOrganizationCommandService) GetCompany(context.Context, string) (control.CompanySummary, error) {
	return control.CompanySummary{}, nil
}

func (fakeOrganizationCommandService) CreateCompany(context.Context, control.CreateCompanyRequest) (control.CommandReceipt, error) {
	return control.CommandReceipt{}, nil
}

func (fakeOrganizationCommandService) UpdateCompany(context.Context, string, control.UpdateCompanyRequest) (control.CommandReceipt, error) {
	return control.CommandReceipt{}, nil
}

func (fakeOrganizationCommandService) ArchiveCompany(context.Context, string, string) (control.CommandReceipt, error) {
	return control.CommandReceipt{}, nil
}

func (fakeOrganizationCommandService) GetRuntimeSettings(context.Context, string) (control.RuntimeSettings, error) {
	return control.RuntimeSettings{CompanyID: "company-1", WorkerMode: "deterministic", Provider: "deterministic", Model: "deterministic/fake", Effort: "bounded", Profile: "deterministic/fake", AuthReadiness: "not_required", RuntimeVersion: "local", RuntimeReadiness: "ready", ProductSurfaceQualification: "not_applicable", WorkspaceRoot: "C:/workspace", PostgreSQLStatus: "ready", CASStatus: "ready", EventStreamStatus: "deferred"}, nil
}

func (fakeOrganizationCommandService) UpdateRuntimeSettings(context.Context, string, control.UpdateRuntimeSettingsRequest) (control.CommandReceipt, error) {
	return control.CommandReceipt{CommandID: "settings-1", CommandType: "runtime.settings.update", TargetType: "company", TargetID: "company-1", RequestID: "settings-1", Accepted: true, AcceptedAt: "2026-09-21T00:00:00Z", ResultingState: "restart_required"}, nil
}

func TestHandlerServesSafeRuntimeSettings(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/workbench/companies/company-1/settings", nil)
	NewHandler(fakeReadModel{}, fakeOrganizationCommandService{}).ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	var result map[string]any
	if err := json.NewDecoder(recorder.Body).Decode(&result); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if result["authReadiness"] != "not_required" || result["runtimeReadiness"] != "ready" {
		t.Fatalf("unexpected runtime settings: %+v", result)
	}
	if _, exists := result["authFile"]; exists {
		t.Fatal("runtime settings exposed provider auth file")
	}
}

func TestHandlerAcceptsRuntimeSettingsUpdate(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/workbench/companies/company-1/settings", bytes.NewReader([]byte(`{"provider":"deterministic","model":"deterministic/fake","effort":"bounded","profile":"deterministic/fake","requestId":"settings-1"}`)))
	request.Header.Set("Content-Type", "application/json")
	NewHandler(fakeReadModel{}, fakeOrganizationCommandService{}).ServeHTTP(recorder, request)
	if recorder.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusAccepted)
	}
}

func TestHTTPRejectsNonJSONCommandBody(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/workbench/companies/company-1/missions", bytes.NewReader([]byte(`{"title":"goal","goal":"body","requestId":"request-1"}`)))
	request.Header.Set("Content-Type", "text/plain")
	NewHandler(fakeReadModel{}, fakeCommandService{}).ServeHTTP(recorder, request)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusBadRequest)
	}
}

func TestHandlerServesCompanyList(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/workbench/companies", nil)
	NewHandler(fakeReadModel{}, fakeOrganizationCommandService{}).ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	var result []control.CompanySummary
	if err := json.NewDecoder(recorder.Body).Decode(&result); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(result) != 1 || result[0].ID != "company-1" {
		t.Fatalf("company list = %+v", result)
	}
}

func stringPointer(value string) *string {
	return &value
}

// pattern: Imperative Shell
package workbench

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"polis/internal/control"
	"polis/internal/core"
	"polis/internal/desktop"
	"polis/internal/environment"
	"polis/internal/kernel"
	"polis/internal/organization"
)

type fakeReadModel struct {
	overview CompanyOverviewView
	activity ActivityView
}

type fakeDailyRoutineReadModel struct {
	fakeReadModel
	companyID string
	missionID string
	items     []DailyRoutineView
}

func (f *fakeDailyRoutineReadModel) ListDailyRoutines(_ context.Context, companyID, missionID string) ([]DailyRoutineView, error) {
	f.companyID, f.missionID = companyID, missionID
	return f.items, nil
}

type fakeDailyRoutineCommandService struct {
	fakeCommandService
	companyID   string
	missionID   string
	routineID   string
	createInput control.CreateDailyRoutineRequest
	setInput    control.SetDailyRoutineTaskInstructionRequest
}

func (f *fakeDailyRoutineCommandService) CreateDailyRoutine(_ context.Context, companyID, missionID string, request control.CreateDailyRoutineRequest) (control.CommandReceipt, error) {
	f.companyID, f.missionID, f.createInput = companyID, missionID, request
	return control.CommandReceipt{CommandID: "routine-create", CommandType: "routine.daily.create", TargetType: "routine", TargetID: request.RoutineID, RequestID: request.RequestID, Accepted: true, AcceptedAt: "2026-09-30T00:00:00Z", ResultingState: "active"}, nil
}

func (f *fakeDailyRoutineCommandService) SetDailyRoutineTaskInstruction(_ context.Context, companyID, missionID, routineID string, request control.SetDailyRoutineTaskInstructionRequest) (control.CommandReceipt, error) {
	f.companyID, f.missionID, f.routineID, f.setInput = companyID, missionID, routineID, request
	return control.CommandReceipt{CommandID: "routine-instruction", CommandType: "routine.daily.instruction", TargetType: "routine", TargetID: routineID, RequestID: request.RequestID, Accepted: true, AcceptedAt: "2026-09-30T00:00:00Z", ResultingState: "instruction_set"}, nil
}

type runtimeQualificationRouteService struct {
	request                control.ApproveStdioMCPRuntimeQualificationRequest
	observationRequest     control.ObserveStdioMCPRuntimeRequest
	httpObservation        control.ObserveStreamableHTTPMCPRuntimeRequest
	company                string
	observationCompany     string
	httpObservationCompany string
}

func (*runtimeQualificationRouteService) CreateMission(context.Context, string, control.CreateMissionRequest) (control.CommandReceipt, error) {
	return control.CommandReceipt{}, nil
}
func (*runtimeQualificationRouteService) StartMission(context.Context, string, control.MissionCommandRequest) (control.CommandReceipt, error) {
	return control.CommandReceipt{}, nil
}
func (*runtimeQualificationRouteService) CancelMission(context.Context, string, control.MissionCommandRequest) (control.CommandReceipt, error) {
	return control.CommandReceipt{}, nil
}
func (service *runtimeQualificationRouteService) ApproveStdioMCPRuntimeQualification(_ context.Context, companyID string, request control.ApproveStdioMCPRuntimeQualificationRequest) (kernel.Receipt, error) {
	service.company = companyID
	service.request = request
	return kernel.Receipt{ID: request.RuntimeQualificationID, Status: "qualified"}, nil
}

func (service *runtimeQualificationRouteService) ObserveStdioMCPRuntime(_ context.Context, companyID string, request control.ObserveStdioMCPRuntimeRequest) (kernel.StdioMCPRuntimeQualification, error) {
	service.observationCompany = companyID
	service.observationRequest = request
	return kernel.StdioMCPRuntimeQualification{CompanyID: companyID, RuntimeQualificationID: "runtime-qualification-1", CapabilityID: request.ServerID, Status: "observed_unqualified"}, nil
}

func (service *runtimeQualificationRouteService) ObserveStreamableHTTPMCPRuntime(_ context.Context, companyID string, request control.ObserveStreamableHTTPMCPRuntimeRequest) (kernel.StdioMCPRuntimeQualification, error) {
	service.httpObservationCompany = companyID
	service.httpObservation = request
	return kernel.StdioMCPRuntimeQualification{CompanyID: companyID, RuntimeQualificationID: "http-runtime-qualification-1", CapabilityID: request.CapabilityID, Transport: "streamable_http", Status: "observed_unqualified"}, nil
}

func TestRuntimeQualificationApprovalCommandDispatchesScopedPayload(t *testing.T) {
	service := &runtimeQualificationRouteService{}
	body := `{"runtimeQualificationId":"runtime-qualification-1","rationale":"reviewed pinned process and schema","requestId":"runtime-approval-1"}`
	request := httptest.NewRequest(http.MethodPost, "/api/workbench/companies/company-1/capabilities/runtime-approve", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	NewHandler(fakeReadModel{}, service).ServeHTTP(response, request)
	if response.Code != http.StatusAccepted {
		t.Fatalf("runtime qualification approval HTTP status=%d body=%s", response.Code, response.Body.String())
	}
	if service.company != "company-1" || service.request.RuntimeQualificationID != "runtime-qualification-1" || service.request.RequestID != "runtime-approval-1" {
		t.Fatalf("runtime qualification approval dispatch=(%q,%+v)", service.company, service.request)
	}
}

func TestRuntimeObservationCommandDispatchesOnlyPinnedPackageAndQualificationIDs(t *testing.T) {
	service := &runtimeQualificationRouteService{}
	body := `{"serverId":"mcp-1","packageRevisionId":"package-rev-1","capabilityQualificationId":"qualification-1","requestId":"runtime-observe-1"}`
	request := httptest.NewRequest(http.MethodPost, "/api/workbench/companies/company-1/capabilities/runtime-observe", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	NewHandler(fakeReadModel{}, service).ServeHTTP(response, request)
	if response.Code != http.StatusAccepted {
		t.Fatalf("runtime observation HTTP status=%d body=%s", response.Code, response.Body.String())
	}
	if service.observationCompany != "company-1" || service.observationRequest.ServerID != "mcp-1" || service.observationRequest.PackageRevisionID != "package-rev-1" || service.observationRequest.CapabilityQualificationID != "qualification-1" || service.observationRequest.RequestID != "runtime-observe-1" {
		t.Fatalf("runtime observation dispatch=(%q,%+v)", service.observationCompany, service.observationRequest)
	}
}

func TestStreamableHTTPRuntimeObservationCommandDispatchesScopedIdentifiers(t *testing.T) {
	service := &runtimeQualificationRouteService{}
	body := `{"capabilityId":"mcp-http-1","capabilityQualificationId":"qualification-http-1","requestId":"runtime-http-observe-1"}`
	request := httptest.NewRequest(http.MethodPost, "/api/workbench/companies/company-1/capabilities/runtime-observe-http", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	NewHandler(fakeReadModel{}, service).ServeHTTP(response, request)
	if response.Code != http.StatusAccepted {
		t.Fatalf("Streamable HTTP runtime observation HTTP status=%d body=%s", response.Code, response.Body.String())
	}
	if service.httpObservationCompany != "company-1" || service.httpObservation.CapabilityID != "mcp-http-1" || service.httpObservation.CapabilityQualificationID != "qualification-http-1" || service.httpObservation.RequestID != "runtime-http-observe-1" {
		t.Fatalf("Streamable HTTP observation dispatch=(%q,%+v)", service.httpObservationCompany, service.httpObservation)
	}
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

type fakeArtifactDeliveryReadModel struct {
	fakeReadModel
	manifest     ArtifactDeliveryManifestResponse
	packageValue ArtifactDeliveryPackage
}

func (model fakeArtifactDeliveryReadModel) GetArtifactDeliveryManifest(_ context.Context, _, _ string) (ArtifactDeliveryManifestResponse, error) {
	return model.manifest, nil
}

func (model fakeArtifactDeliveryReadModel) GetArtifactDeliveryPackage(_ context.Context, _, _ string) (ArtifactDeliveryPackage, error) {
	return model.packageValue, nil
}

type fakeEnvironmentJobReadModel struct {
	fakeReadModel
}

func (fakeEnvironmentJobReadModel) ListProjectEnvironments(_ context.Context, companyID string) ([]ProjectEnvironmentRevisionView, error) {
	return []ProjectEnvironmentRevisionView{{CompanyID: companyID, RevisionID: "environment-1", ProfileID: "windows-node-npm@1", PolicyDecision: "approved", ExecutorQualification: "unqualified", PreparationState: "blocked_unqualified", PreparationReason: "environment_executor_not_qualified"}}, nil
}

func (fakeEnvironmentJobReadModel) ListTaskJobRuns(_ context.Context, companyID, taskID string) ([]JobRunView, error) {
	return []JobRunView{{CompanyID: companyID, JobID: "job-1", TaskID: taskID, SessionID: "session-1", EnvironmentRevisionID: "environment-1", Kind: "service", ServiceID: "web", State: "running", Readiness: "ready", StdoutOffset: "128", StderrOffset: "0", CreatedAt: "2026-09-24T00:00:00Z", UpdatedAt: "2026-09-24T00:00:01Z"}}, nil
}

func (fakeEnvironmentJobReadModel) ListTaskCrossBackendHandovers(_ context.Context, companyID, taskID string) ([]kernel.CrossBackendHandoverRecord, error) {
	return []kernel.CrossBackendHandoverRecord{{CompanyID: companyID, HandoverID: "handover-1", TaskID: taskID, SourceProfileID: "windows-node-npm@1", TargetProfileID: "linux-node-npm@1"}}, nil
}

type fakeFeedbackReadModel struct{ fakeReadModel }

type fakeGitHubFeedbackCommandService struct{ fakeCommandService }

type fakeProjectJobCommandService struct {
	fakeCommandService
	startRequest    control.StartProjectJobRequest
	stopJobID       string
	stopRequest     control.StopProjectJobRequest
	browserJobID    string
	browserRequest  control.CreateProjectJobBrowserSessionRequest
	handoverTask    string
	handoverRequest control.CreateProjectEnvironmentHandoverRequest
}

func (service *fakeProjectJobCommandService) StartProjectJob(_ context.Context, companyID string, request control.StartProjectJobRequest) (kernel.JobRunRecord, error) {
	service.startRequest = request
	return kernel.JobRunRecord{CompanyID: companyID, JobID: "job-1", TaskID: request.TaskID, Kind: request.Kind, ServiceID: request.ServiceID, State: "running", Readiness: "not_applicable", ReasonCode: "project_job_running"}, nil
}

func (service *fakeProjectJobCommandService) StopProjectJob(_ context.Context, companyID, jobID string, request control.StopProjectJobRequest) (kernel.JobRunRecord, error) {
	service.stopJobID, service.stopRequest = jobID, request
	return kernel.JobRunRecord{CompanyID: companyID, JobID: jobID, Kind: "batch", State: "cancelled", Readiness: "not_applicable", ReasonCode: "project_job_cancelled"}, nil
}

func (service *fakeProjectJobCommandService) GetProjectJobLogs(_ context.Context, companyID, jobID string) (kernel.JobRunLogArtifact, error) {
	return kernel.JobRunLogArtifact{CompanyID: companyID, JobID: jobID, ManifestSHA256: "digest", Content: []byte("manifest")}, nil
}

func (service *fakeProjectJobCommandService) CreateProjectJobBrowserSession(_ context.Context, companyID, jobID string, request control.CreateProjectJobBrowserSessionRequest) (control.ServiceBrowserSession, error) {
	service.browserJobID, service.browserRequest = jobID, request
	return control.ServiceBrowserSession{URL: "http://127.0.0.1:43123/_polis/open/one-time-ticket", ExpiresAt: time.Date(2026, 9, 29, 12, 5, 0, 0, time.UTC)}, nil
}

func (service *fakeProjectJobCommandService) CreateTaskEnvironmentHandover(_ context.Context, companyID, taskID string, request control.CreateProjectEnvironmentHandoverRequest) (kernel.CrossBackendHandoverRecord, error) {
	service.handoverTask, service.handoverRequest = taskID, request
	return kernel.CrossBackendHandoverRecord{CompanyID: companyID, HandoverID: "handover-1", TaskID: taskID, SourceProfileID: "windows-node-npm@1", TargetProfileID: "linux-node-npm@1"}, nil
}

type fakeGitHubFeedbackCredentialCommandService struct {
	fakeCommandService
	storedToken string
	deleted     bool
}

func (s *fakeGitHubFeedbackCredentialCommandService) StoreGitHubFeedbackCredential(_ context.Context, token string) (control.GitHubFeedbackCredentialReceipt, error) {
	s.storedToken = token
	return control.GitHubFeedbackCredentialReceipt{CredentialRef: control.GitHubFeedbackCredentialRef, Stored: true}, nil
}

func (s *fakeGitHubFeedbackCredentialCommandService) DeleteGitHubFeedbackCredential(context.Context) (control.GitHubFeedbackCredentialReceipt, error) {
	s.deleted = true
	return control.GitHubFeedbackCredentialReceipt{CredentialRef: control.GitHubFeedbackCredentialRef, Stored: false}, nil
}

func (fakeGitHubFeedbackCommandService) RegisterGitHubFeedbackSource(_ context.Context, companyID string, _ control.RegisterGitHubFeedbackSourceRequest) (control.GitHubFeedbackSourceView, error) {
	return control.GitHubFeedbackSourceView{CompanyID: companyID, SourceID: "source-1", State: "draft"}, nil
}

func (fakeGitHubFeedbackCommandService) ProbeGitHubFeedbackSource(_ context.Context, companyID, sourceID string, input control.GitHubFeedbackProbeRequest) (control.GitHubFeedbackProbeReceipt, error) {
	return control.GitHubFeedbackProbeReceipt{CompanyID: companyID, SourceID: sourceID, RequestID: input.RequestID, PermissionStatus: "verified", Coverage: "complete"}, nil
}

func (fakeGitHubFeedbackCommandService) DecideGitHubFeedbackSource(_ context.Context, companyID, sourceID string, _ control.GitHubFeedbackDecisionRequest) (control.GitHubFeedbackSourceView, error) {
	return control.GitHubFeedbackSourceView{CompanyID: companyID, SourceID: sourceID, State: "approved"}, nil
}

func (fakeGitHubFeedbackCommandService) PollGitHubFeedbackSource(_ context.Context, companyID, sourceID string, input control.GitHubFeedbackPollRequest) (control.GitHubFeedbackPollReceipt, error) {
	return control.GitHubFeedbackPollReceipt{CompanyID: companyID, SourceID: sourceID, RequestID: input.RequestID, ScanID: "scan-1", Coverage: "complete"}, nil
}

func (fakeGitHubFeedbackCommandService) SetGitHubFeedbackBacklogStatus(_ context.Context, companyID string, input control.GitHubFeedbackBacklogStatusRequest) (control.GitHubFeedbackBacklogStatusReceipt, error) {
	return control.GitHubFeedbackBacklogStatusReceipt{
		CompanyID: companyID, SourceID: input.SourceID, ProviderItemID: input.ProviderItemID,
		IssueNumber: 7, RevisionSHA256: input.RevisionSHA256, Status: input.Status,
		Rationale: input.Rationale, EventID: "backlog-event-1", RequestID: input.RequestID,
		RemoteState: "open", RemoteUnchanged: true,
	}, nil
}

func (fakeGitHubFeedbackCommandService) SetGitHubFeedbackCollectionPolicy(_ context.Context, companyID string, input control.GitHubFeedbackCollectionPolicyRequest) (control.GitHubFeedbackCollectionPolicyReceipt, error) {
	return control.GitHubFeedbackCollectionPolicyReceipt{
		CompanyID: companyID, SourceID: input.SourceID, Enabled: input.Enabled, IntervalSeconds: input.IntervalSeconds,
		Rationale: input.Rationale, UpdatedAt: "2026-09-29T00:00:00Z", RequestID: input.RequestID,
		ExternalEnabled: false, SchedulerEnabled: false,
	}, nil
}

func (fakeFeedbackReadModel) GetCompanyFeedback(_ context.Context, companyID string) (CompanyFeedbackView, error) {
	return CompanyFeedbackView{CompanyID: companyID, Sources: []FeedbackSourceView{}, Issues: []FeedbackIssueView{}}, nil
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

func TestArtifactManifestAndPackageRoutesRequireDesktopTokenAndPreserveHeaders(t *testing.T) {
	content := []byte("browser route artifact")
	contentHash := sha256.Sum256(content)
	manifest := ArtifactDeliveryManifestView{
		SchemaVersion: ArtifactDeliveryManifestSchema, CompanyID: "company-1", ArtifactID: "artifact-1", TaskID: "task-1",
		Content: ArtifactDeliveryContent{FileName: "artifact.bin", ContentType: "application/octet-stream", ByteSize: "22", SHA256: hex.EncodeToString(contentHash[:])},
		State:   "ready", Verdict: "passed",
		Qualification: ArtifactDeliveryQualification{CheckpointID: "checkpoint-1", ValidationReceiptID: "receipt-1", TaskValidationBindingDigest: string(repeatByte('a', 64)), WorkspaceDigest: string(repeatByte('b', 64)), WorkspaceRevision: "1", RunnerRevision: "runner-test@1"},
		CreatedAt:     "2026-09-25T00:00:00Z",
	}
	packaged, err := buildArtifactDeliveryPackage(manifest, content)
	if err != nil {
		t.Fatal(err)
	}
	model := fakeArtifactDeliveryReadModel{
		manifest:     ArtifactDeliveryManifestResponse{Manifest: manifest, ManifestSHA256: packaged.ManifestSHA256},
		packageValue: packaged,
	}
	handler := desktop.Middleware("desktop-session", NewHandler(model))
	for _, path := range []string{
		"/api/workbench/companies/company-1/artifacts/artifact-1/manifest",
		"/api/workbench/companies/company-1/artifacts/artifact-1/download",
	} {
		request := httptest.NewRequest(http.MethodGet, path, nil)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != http.StatusUnauthorized {
			t.Errorf("tokenless route %s status=%d, want %d", path, response.Code, http.StatusUnauthorized)
		}
	}

	manifestRequest := httptest.NewRequest(http.MethodGet, "/api/workbench/companies/company-1/artifacts/artifact-1/manifest", nil)
	manifestRequest.Header.Set("X-Polis-Desktop-Token", "desktop-session")
	manifestResponse := httptest.NewRecorder()
	handler.ServeHTTP(manifestResponse, manifestRequest)
	if manifestResponse.Code != http.StatusOK || manifestResponse.Header().Get("X-Polis-Manifest-SHA256") != packaged.ManifestSHA256 {
		t.Fatalf("authorized manifest response=%d headers=%v body=%s", manifestResponse.Code, manifestResponse.Header(), manifestResponse.Body.String())
	}

	packageRequest := httptest.NewRequest(http.MethodGet, "/api/workbench/companies/company-1/artifacts/artifact-1/download", nil)
	packageRequest.Header.Set("X-Polis-Desktop-Token", "desktop-session")
	packageResponse := httptest.NewRecorder()
	handler.ServeHTTP(packageResponse, packageRequest)
	if packageResponse.Code != http.StatusOK || packageResponse.Header().Get("Content-Type") != "application/zip" || packageResponse.Header().Get("X-Content-SHA256") != packaged.PackageSHA256 || packageResponse.Header().Get("X-Polis-Manifest-SHA256") != packaged.ManifestSHA256 || !bytes.Equal(packageResponse.Body.Bytes(), packaged.Archive) {
		t.Fatalf("authorized package response=%d headers=%v bytesMatch=%v", packageResponse.Code, packageResponse.Header(), bytes.Equal(packageResponse.Body.Bytes(), packaged.Archive))
	}
}

func TestHandlerServesEnvironmentQualificationStatus(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/workbench/companies/company-1/environments", nil)
	NewHandler(fakeEnvironmentJobReadModel{}).ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	var result []ProjectEnvironmentRevisionView
	if err := json.NewDecoder(recorder.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	if len(result) != 1 || result[0].PreparationState != "blocked_unqualified" || result[0].ExecutorQualification != "unqualified" {
		t.Fatalf("environment status = %+v", result)
	}
}

func TestHandlerServesTaskJobRunLifecycle(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/workbench/companies/company-1/tasks/task-1/jobs", nil)
	NewHandler(fakeEnvironmentJobReadModel{}).ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	var result []JobRunView
	if err := json.NewDecoder(recorder.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	if len(result) != 1 || result[0].JobID != "job-1" || result[0].Readiness != "ready" {
		t.Fatalf("job run status = %+v", result)
	}
}

func TestHandlerStartsAndStopsCompanyScopedProjectJob(t *testing.T) {
	service := &fakeProjectJobCommandService{}
	startRecorder := httptest.NewRecorder()
	startRequest := httptest.NewRequest(http.MethodPost, "/api/workbench/companies/company-1/tasks/task-1/jobs", bytes.NewReader([]byte(`{"taskId":"task-spoofed","sessionId":"session-1","environmentRevisionId":"environment-1","handoverId":"handover-1","kind":"batch","scriptPath":"scripts/build.mjs","args":[],"requestId":"job-start-1"}`)))
	startRequest.Header.Set("Content-Type", "application/json")
	NewHandler(fakeReadModel{}, service).ServeHTTP(startRecorder, startRequest)
	if startRecorder.Code != http.StatusAccepted || service.startRequest.TaskID != "task-1" || service.startRequest.ScriptPath != "scripts/build.mjs" || service.startRequest.HandoverID != "handover-1" {
		t.Fatalf("start response=%d body=%s request=%+v", startRecorder.Code, startRecorder.Body.String(), service.startRequest)
	}
	serviceStartRecorder := httptest.NewRecorder()
	serviceStartRequest := httptest.NewRequest(http.MethodPost, "/api/workbench/companies/company-1/tasks/task-1/jobs", bytes.NewReader([]byte(`{"sessionId":"session-1","environmentRevisionId":"environment-1","kind":"service","serviceId":"web","requestId":"job-service-start-1"}`)))
	serviceStartRequest.Header.Set("Content-Type", "application/json")
	NewHandler(fakeReadModel{}, service).ServeHTTP(serviceStartRecorder, serviceStartRequest)
	if serviceStartRecorder.Code != http.StatusAccepted || service.startRequest.Kind != "service" || service.startRequest.ServiceID != "web" || service.startRequest.ScriptPath != "" || len(service.startRequest.Args) != 0 {
		t.Fatalf("service start response=%d body=%s request=%+v", serviceStartRecorder.Code, serviceStartRecorder.Body.String(), service.startRequest)
	}
	stopRecorder := httptest.NewRecorder()
	stopRequest := httptest.NewRequest(http.MethodPost, "/api/workbench/companies/company-1/jobs/job-1/stop", bytes.NewReader([]byte(`{"requestId":"job-stop-1"}`)))
	stopRequest.Header.Set("Content-Type", "application/json")
	NewHandler(fakeReadModel{}, service).ServeHTTP(stopRecorder, stopRequest)
	if stopRecorder.Code != http.StatusAccepted || service.stopJobID != "job-1" || service.stopRequest.RequestID != "job-stop-1" {
		t.Fatalf("stop response=%d body=%s job=%q request=%+v", stopRecorder.Code, stopRecorder.Body.String(), service.stopJobID, service.stopRequest)
	}
}

func TestHandlerCreatesAndListsCompanyScopedCrossBackendHandover(t *testing.T) {
	service := &fakeProjectJobCommandService{}
	createRecorder := httptest.NewRecorder()
	createRequest := httptest.NewRequest(http.MethodPost, "/api/workbench/companies/company-1/tasks/task-1/environment-handovers", bytes.NewReader([]byte(`{"sourceJobId":"job-1","targetEnvironmentRevisionId":"linux-environment-1","requestId":"handover-create-1"}`)))
	createRequest.Header.Set("Content-Type", "application/json")
	NewHandler(fakeEnvironmentJobReadModel{}, service).ServeHTTP(createRecorder, createRequest)
	if createRecorder.Code != http.StatusAccepted || service.handoverTask != "task-1" || service.handoverRequest.SourceJobID != "job-1" || service.handoverRequest.TargetEnvironmentRevision != "linux-environment-1" {
		t.Fatalf("create handover response=%d body=%s task=%q request=%+v", createRecorder.Code, createRecorder.Body.String(), service.handoverTask, service.handoverRequest)
	}
	listRecorder := httptest.NewRecorder()
	listRequest := httptest.NewRequest(http.MethodGet, "/api/workbench/companies/company-1/tasks/task-1/environment-handovers", nil)
	NewHandler(fakeEnvironmentJobReadModel{}, service).ServeHTTP(listRecorder, listRequest)
	if listRecorder.Code != http.StatusOK || !strings.Contains(listRecorder.Body.String(), `"handoverId":"handover-1"`) {
		t.Fatalf("list handovers response=%d body=%s", listRecorder.Code, listRecorder.Body.String())
	}
}

func TestHandlerServesCompanyScopedProjectJobLogs(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/workbench/companies/company-1/jobs/job-1/logs", nil)
	NewHandler(fakeReadModel{}, &fakeProjectJobCommandService{}).ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), "manifestSha256") {
		t.Fatalf("logs response=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestHandlerCreatesCompanyScopedProjectJobBrowserSession(t *testing.T) {
	service := &fakeProjectJobCommandService{}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/workbench/companies/company-1/jobs/job-1/browser-session", strings.NewReader(`{"requestId":"browser-session-1"}`))
	request.Header.Set("Content-Type", "application/json")
	NewHandler(fakeReadModel{}, service).ServeHTTP(recorder, request)
	if recorder.Code != http.StatusCreated || service.browserJobID != "job-1" || service.browserRequest.RequestID != "browser-session-1" || !strings.Contains(recorder.Body.String(), "_polis/open/one-time-ticket") {
		t.Fatalf("browser session response=%d body=%s job=%q request=%+v", recorder.Code, recorder.Body.String(), service.browserJobID, service.browserRequest)
	}
	getRecorder := httptest.NewRecorder()
	getRequest := httptest.NewRequest(http.MethodGet, "/api/workbench/companies/company-1/jobs/job-1/browser-session", nil)
	NewHandler(fakeReadModel{}, service).ServeHTTP(getRecorder, getRequest)
	if getRecorder.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET browser session status=%d body=%s", getRecorder.Code, getRecorder.Body.String())
	}
}

func TestHandlerServesCompanyFeedbackReadModel(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/workbench/companies/company-1/feedback", nil)
	NewHandler(fakeFeedbackReadModel{}).ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("GET feedback status = %d, body=%s", recorder.Code, recorder.Body.String())
	}
	var result CompanyFeedbackView
	if err := json.NewDecoder(recorder.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	if result.CompanyID != "company-1" || result.Sources == nil || result.Issues == nil {
		t.Fatalf("feedback view = %+v", result)
	}
}

func TestHandlerRoutesExplicitGitHubFeedbackPollCommand(t *testing.T) {
	service := fakeGitHubFeedbackCommandService{}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/workbench/companies/company-1/feedback/sources/source-1/scan", strings.NewReader(`{"requestId":"scan-request-1"}`))
	request.Header.Set("Content-Type", "application/json")
	NewHandler(fakeReadModel{}, service).ServeHTTP(recorder, request)
	if recorder.Code != http.StatusAccepted {
		t.Fatalf("POST feedback scan status = %d, body=%s", recorder.Code, recorder.Body.String())
	}
	var result control.GitHubFeedbackPollReceipt
	if err := json.NewDecoder(recorder.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	if result.CompanyID != "company-1" || result.SourceID != "source-1" || result.RequestID != "scan-request-1" || result.Coverage != "complete" {
		t.Fatalf("feedback poll receipt = %+v", result)
	}
}

func TestHandlerRoutesCompanyBacklogDecisionWithoutRemoteIssueMutation(t *testing.T) {
	service := fakeGitHubFeedbackCommandService{}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/workbench/companies/company-1/feedback/backlog", strings.NewReader(`{"sourceId":"source-1","providerItemId":"101","revisionSha256":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","status":"handled","rationale":"triaged internally","requestId":"backlog-request-1"}`))
	request.Header.Set("Content-Type", "application/json")
	NewHandler(fakeReadModel{}, service).ServeHTTP(recorder, request)
	if recorder.Code != http.StatusAccepted {
		t.Fatalf("POST feedback backlog status = %d, body=%s", recorder.Code, recorder.Body.String())
	}
	var result control.GitHubFeedbackBacklogStatusReceipt
	if err := json.NewDecoder(recorder.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	if result.CompanyID != "company-1" || result.SourceID != "source-1" || result.ProviderItemID != 101 || result.Status != "handled" || result.RemoteState != "open" || !result.RemoteUnchanged {
		t.Fatalf("feedback backlog receipt = %+v", result)
	}
}

func TestHandlerRoutesGitHubFeedbackCollectionPolicyCommand(t *testing.T) {
	service := fakeGitHubFeedbackCommandService{}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/workbench/companies/company-1/feedback/collection-policy", strings.NewReader(`{"sourceId":"source-1","enabled":true,"intervalSeconds":3600,"rationale":"bounded read-only collection","requestId":"collection-policy-1"}`))
	request.Header.Set("Content-Type", "application/json")
	NewHandler(fakeReadModel{}, service).ServeHTTP(recorder, request)
	if recorder.Code != http.StatusAccepted {
		t.Fatalf("POST feedback collection policy status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	var result control.GitHubFeedbackCollectionPolicyReceipt
	if err := json.NewDecoder(recorder.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	if result.CompanyID != "company-1" || result.SourceID != "source-1" || !result.Enabled || result.IntervalSeconds != 3600 || result.RequestID != "collection-policy-1" || result.ExternalEnabled || result.SchedulerEnabled {
		t.Fatalf("feedback collection policy receipt=%+v", result)
	}
}

func TestHandlerStoresGitHubCredentialWithoutReturningIt(t *testing.T) {
	const token = "github_pat_abcdefghijklmnopqrstuvwxyz012345"
	service := &fakeGitHubFeedbackCredentialCommandService{}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/workbench/companies/company-1/feedback/credentials", strings.NewReader(`{"token":"`+token+`"}`))
	request.Header.Set("Content-Type", "application/json")
	NewHandler(fakeReadModel{}, service).ServeHTTP(recorder, request)
	if recorder.Code != http.StatusAccepted || service.storedToken != token {
		t.Fatalf("credential store status=%d stored=%t body=%s", recorder.Code, service.storedToken == token, recorder.Body.String())
	}
	if strings.Contains(recorder.Body.String(), token) || strings.Contains(recorder.Body.String(), "token") {
		t.Fatalf("credential response leaked token material: %s", recorder.Body.String())
	}
}

func TestHandlerDeletesGitHubCredentialWithoutReturningIt(t *testing.T) {
	service := &fakeGitHubFeedbackCredentialCommandService{}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/workbench/companies/company-1/feedback/credentials/delete", strings.NewReader(`{}`))
	request.Header.Set("Content-Type", "application/json")
	NewHandler(fakeReadModel{}, service).ServeHTTP(recorder, request)
	if recorder.Code != http.StatusAccepted || !service.deleted || strings.Contains(recorder.Body.String(), "token") {
		t.Fatalf("credential delete status=%d deleted=%t body=%s", recorder.Code, service.deleted, recorder.Body.String())
	}
}

func TestHTTPAcceptsEnvironmentPlanningAndEnsureCommands(t *testing.T) {
	service := fakeEnvironmentCommandService{}
	_, policyManifest, _, err := environment.BuildWindowsNodeEnvironmentPolicy([]string{"registry.npmjs.org"}, 180_000, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	registerBody, err := json.Marshal(control.RegisterProjectEnvironmentRequest{
		ProfileID: environment.WindowsNodeNPMProfile, MissionID: "mission-1", SourceInputID: "input-1", SourceInputRevision: "1",
		PolicyManifest: json.RawMessage(policyManifest), ToolchainSHA256: strings.Repeat("e", 64), RequestID: "environment-1",
	})
	if err != nil {
		t.Fatal(err)
	}
	requests := []struct {
		path string
		body string
	}{
		{"/api/workbench/companies/company-1/environments", string(registerBody)},
		{"/api/workbench/companies/company-1/environments/env-1/policy", `{"decision":"approved","rationale":"reviewed lockfile and source policy","requestId":"environment-policy-1"}`},
		{"/api/workbench/companies/company-1/environments/env-1/executor-qualification", `{"decision":"qualified","evidenceInputId":"input-1","evidenceInputRevision":"2","rationale":"reviewed current host report","requestId":"environment-executor-qualification-1"}`},
		{"/api/workbench/companies/company-1/environments/env-1/preparation", `{"requestId":"environment-ensure-1"}`},
	}
	for _, item := range requests {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, item.path, strings.NewReader(item.body))
		request.Header.Set("Content-Type", "application/json")
		NewHandler(fakeReadModel{}, service).ServeHTTP(recorder, request)
		if recorder.Code != http.StatusAccepted {
			t.Errorf("POST %s status = %d, body=%s", item.path, recorder.Code, recorder.Body.String())
		}
	}
	forgedSourceBody, err := json.Marshal(map[string]any{
		"profileId": environment.WindowsNodeNPMProfile, "missionId": "mission-1", "sourceInputId": "input-1", "sourceInputRevision": "1",
		"sourceRevisionSha256": strings.Repeat("a", 64), "policyManifest": json.RawMessage(policyManifest),
		"toolchainSha256": strings.Repeat("e", 64), "requestId": "environment-forged-source",
	})
	if err != nil {
		t.Fatal(err)
	}
	forgedRecorder := httptest.NewRecorder()
	forgedRequest := httptest.NewRequest(http.MethodPost, "/api/workbench/companies/company-1/environments", strings.NewReader(string(forgedSourceBody)))
	forgedRequest.Header.Set("Content-Type", "application/json")
	NewHandler(fakeReadModel{}, service).ServeHTTP(forgedRecorder, forgedRequest)
	if forgedRecorder.Code != http.StatusBadRequest {
		t.Fatalf("caller-supplied environment provenance status = %d, want %d: %s", forgedRecorder.Code, http.StatusBadRequest, forgedRecorder.Body.String())
	}
}

type fakeEnvironmentCommandService struct{ fakeOrganizationCommandService }

func (fakeEnvironmentCommandService) RegisterProjectEnvironment(_ context.Context, companyID string, request control.RegisterProjectEnvironmentRequest) (kernel.ProjectEnvironmentRevision, error) {
	return kernel.ProjectEnvironmentRevision{CompanyID: companyID, RevisionID: "env-1", ProfileID: request.ProfileID}, nil
}

func (fakeEnvironmentCommandService) DecideProjectEnvironmentPolicy(_ context.Context, _, _ string, request control.EnvironmentPolicyDecisionRequest) (kernel.Receipt, error) {
	return kernel.Receipt{ID: "event-1", Status: request.Decision}, nil
}

func (fakeEnvironmentCommandService) DecideProjectEnvironmentExecutorQualification(_ context.Context, _, _ string, request control.EnvironmentExecutorQualificationRequest) (kernel.Receipt, error) {
	return kernel.Receipt{ID: "qualification-1", Status: request.Decision}, nil
}

func (fakeEnvironmentCommandService) EnsureProjectEnvironment(_ context.Context, companyID, revisionID string, _ control.EnsureEnvironmentRequest) (kernel.EnvironmentPreparationRun, error) {
	return kernel.EnvironmentPreparationRun{CompanyID: companyID, RevisionID: revisionID, RunID: "run-1", State: string(environment.PreparationBlockedUnqualified), ReasonCode: "environment_executor_not_qualified"}, nil
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

type fakeHumanInterventionCommandService struct {
	fakeCommandService
	companyID string
	request   control.SetHumanInterventionStateRequest
}

type fakeMissionChangeRequestCommandService struct {
	fakeCommandService
	companyID string
	missionID string
	request   control.CreateMissionChangeRequestRequest
	result    kernel.MissionChangeRequest
	action    string
	changeID  string
	command   control.MissionChangeRequestCommand
}

type fakeTaskTakeoverLeaseService struct {
	fakeCommandService
	action       string
	companyID    string
	missionID    string
	taskID       string
	leaseID      string
	leaseRequest control.TaskTakeoverLeaseCommand
	snapshot     control.TaskTakeoverSnapshotCommand
	result       kernel.TaskTakeoverLease
}

func (f *fakeTaskTakeoverLeaseService) ListTaskTakeoverLeases(_ context.Context, companyID, missionID string) ([]kernel.TaskTakeoverLease, error) {
	f.action, f.companyID, f.missionID = "list", companyID, missionID
	return []kernel.TaskTakeoverLease{f.result}, nil
}

func (f *fakeTaskTakeoverLeaseService) CreateTaskTakeoverLease(_ context.Context, companyID, missionID, taskID string, request control.TaskTakeoverLeaseCommand) (kernel.TaskTakeoverLease, error) {
	f.action, f.companyID, f.missionID, f.taskID, f.leaseRequest = "create", companyID, missionID, taskID, request
	return f.result, nil
}

func (f *fakeTaskTakeoverLeaseService) SubmitTaskTakeoverSnapshot(_ context.Context, companyID, missionID, leaseID string, request control.TaskTakeoverSnapshotCommand) (kernel.TaskTakeoverLease, error) {
	f.action, f.companyID, f.missionID, f.leaseID, f.snapshot = "snapshot", companyID, missionID, leaseID, request
	return f.result, nil
}

func (f *fakeTaskTakeoverLeaseService) ReleaseTaskTakeoverLease(_ context.Context, companyID, missionID, leaseID string, request control.TaskTakeoverLeaseCommand) (kernel.TaskTakeoverLease, error) {
	f.action, f.companyID, f.missionID, f.leaseID, f.leaseRequest = "release", companyID, missionID, leaseID, request
	return f.result, nil
}

func (f *fakeMissionChangeRequestCommandService) CreateMissionChangeRequest(_ context.Context, companyID, missionID string, request control.CreateMissionChangeRequestRequest) (kernel.MissionChangeRequest, error) {
	f.companyID, f.missionID, f.request = companyID, missionID, request
	return f.result, nil
}

func (f *fakeMissionChangeRequestCommandService) ListMissionChangeRequests(_ context.Context, companyID, missionID string) ([]kernel.MissionChangeRequest, error) {
	f.companyID, f.missionID, f.action = companyID, missionID, "list"
	return []kernel.MissionChangeRequest{f.result}, nil
}

func (f *fakeMissionChangeRequestCommandService) ConsiderMissionChangeRequest(_ context.Context, companyID, missionID, changeID string, request control.MissionChangeRequestCommand) (kernel.MissionChangeRequest, error) {
	f.companyID, f.missionID, f.changeID, f.action, f.command = companyID, missionID, changeID, "consider", request
	return f.result, nil
}

func (f *fakeMissionChangeRequestCommandService) DeclineMissionChangeRequest(_ context.Context, companyID, missionID, changeID string, request control.MissionChangeRequestCommand) (kernel.MissionChangeRequest, error) {
	f.companyID, f.missionID, f.changeID, f.action, f.command = companyID, missionID, changeID, "decline", request
	return f.result, nil
}

func (f *fakeMissionChangeRequestCommandService) ApplyMissionChangeRequest(_ context.Context, companyID, missionID, changeID string, request control.MissionChangeRequestCommand) (kernel.MissionChangeRequest, error) {
	f.companyID, f.missionID, f.changeID, f.action, f.command = companyID, missionID, changeID, "apply", request
	return f.result, nil
}

func (f *fakeHumanInterventionCommandService) SetHumanInterventionState(_ context.Context, companyID string, request control.SetHumanInterventionStateRequest) (control.CommandReceipt, error) {
	f.companyID = companyID
	f.request = request
	return control.CommandReceipt{CommandID: request.RequestID, CommandType: "human_intervention." + request.State, TargetType: "human_intervention", TargetID: request.InterventionID, RequestID: request.RequestID, Accepted: true, AcceptedAt: "2026-09-23T00:00:00Z", ResultingState: request.State}, nil
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

func (f fakeCommandService) PauseMission(context.Context, string, control.MissionCommandRequest) (control.CommandReceipt, error) {
	return f.receipt, f.err
}

func (f fakeCommandService) ResumeMission(context.Context, string, control.MissionCommandRequest) (control.CommandReceipt, error) {
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

func TestHTTPAcceptsFormalMissionPauseAndResumeCommands(t *testing.T) {
	for _, item := range []struct {
		path string
		body string
	}{
		{path: "/api/workbench/companies/company-1/missions/mission-1/pause", body: `{"requestId":"pause-1"}`},
		{path: "/api/workbench/companies/company-1/missions/mission-1/resume", body: `{"requestId":"resume-1"}`},
	} {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, item.path, bytes.NewReader([]byte(item.body)))
		request.Header.Set("Content-Type", "application/json")
		NewHandler(fakeReadModel{}, fakeCommandService{receipt: control.CommandReceipt{CommandID: "mission-lifecycle", CommandType: "mission.pause", TargetType: "mission", TargetID: "mission-1", RequestID: "request", Accepted: true, AcceptedAt: "2026-09-23T00:00:00Z", ResultingState: "paused"}}).ServeHTTP(recorder, request)
		if recorder.Code != http.StatusAccepted {
			t.Fatalf("lifecycle route %s status = %d: %s", item.path, recorder.Code, recorder.Body.String())
		}
	}
}

func TestParseHumanInterventionStateCommand(t *testing.T) {
	path, ok := parsePath("/api/workbench/companies/company-1/human-interventions/intervention-1/acknowledge")
	if !ok || path.companyID != "company-1" || path.resourceID != "intervention-1" || path.endpoint != "human_interventions.acknowledge" {
		t.Fatalf("human intervention command path = %+v, ok=%v", path, ok)
	}
}

func TestParseMissionChangeRequestCollectionAndActions(t *testing.T) {
	collection, ok := parsePath("/api/workbench/companies/company-1/missions/mission-1/change-requests")
	if !ok || collection.companyID != "company-1" || collection.missionID != "mission-1" || collection.endpoint != "missions.change_requests" {
		t.Fatalf("change request collection path = %+v, ok=%v", collection, ok)
	}
	for _, action := range []string{"consider", "decline", "apply"} {
		parsed, ok := parsePath("/api/workbench/companies/company-1/missions/mission-1/change-requests/change-1/" + action)
		if !ok || parsed.companyID != "company-1" || parsed.missionID != "mission-1" || parsed.resourceID != "change-1" || parsed.endpoint != "mission_change_requests."+action {
			t.Fatalf("change request %s path = %+v, ok=%v", action, parsed, ok)
		}
	}
}

func TestParsesDailyRoutineRoutes(t *testing.T) {
	listPath, ok := parsePath("/api/workbench/companies/company-1/missions/mission-1/routines")
	if !ok || listPath.endpoint != "missions.routines" || listPath.companyID != "company-1" || listPath.missionID != "mission-1" {
		t.Fatalf("Routine list path=%+v parsed=%t", listPath, ok)
	}
	repairPath, ok := parsePath("/api/workbench/companies/company-1/missions/mission-1/routines/routine-1/instruction")
	if !ok || repairPath.endpoint != "missions.routine.instruction" || repairPath.resourceID != "routine-1" {
		t.Fatalf("Routine instruction path=%+v parsed=%t", repairPath, ok)
	}
}

func TestHTTPReadsDailyRoutinesWithinMissionScope(t *testing.T) {
	model := &fakeDailyRoutineReadModel{items: []DailyRoutineView{{RoutineID: "routine-1", MissionID: "mission-1", EmployeeID: "emp-backend", Timezone: "Asia/Shanghai", LocalTime: "09:00", NextLogicalDay: "2026-09-30", CatchUpPolicy: "coalesce_latest", MaxCatchUp: 1, NeedsInstructionOccurrences: 2}}}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/workbench/companies/company-1/missions/mission-1/routines", nil)
	NewHandler(model).ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || model.companyID != "company-1" || model.missionID != "mission-1" {
		t.Fatalf("Routine list response status=%d scope=%s/%s body=%s", recorder.Code, model.companyID, model.missionID, recorder.Body.String())
	}
	var items []DailyRoutineView
	if err := json.NewDecoder(recorder.Body).Decode(&items); err != nil || len(items) != 1 || items[0].RoutineID != "routine-1" {
		t.Fatalf("Routine list response=%+v decodeError=%v", items, err)
	}
}

func TestHTTPCreatesAndRepairsDailyRoutine(t *testing.T) {
	service := &fakeDailyRoutineCommandService{}
	createBody := `{"routineId":"routine-1","employeeId":"emp-backend","taskInstruction":"Review the approved queue.","timezone":"Asia/Shanghai","localTime":"09:00","nextLogicalDay":"2026-09-30","catchUpPolicy":"coalesce_latest","maxCatchUp":1,"requestId":"routine-create-1"}`
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/workbench/companies/company-1/missions/mission-1/routines", strings.NewReader(createBody))
	request.Header.Set("Content-Type", "application/json")
	NewHandler(fakeReadModel{}, service).ServeHTTP(recorder, request)
	if recorder.Code != http.StatusAccepted || service.companyID != "company-1" || service.missionID != "mission-1" || service.createInput.RoutineID != "routine-1" || service.createInput.RequestID != "routine-create-1" {
		t.Fatalf("Routine create response status=%d captured=%+v body=%s", recorder.Code, service.createInput, recorder.Body.String())
	}
	repairBody := `{"taskInstruction":"Review the legacy queue.","requestId":"routine-repair-1"}`
	recorder = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodPost, "/api/workbench/companies/company-1/missions/mission-1/routines/routine-1/instruction", strings.NewReader(repairBody))
	request.Header.Set("Content-Type", "application/json")
	NewHandler(fakeReadModel{}, service).ServeHTTP(recorder, request)
	if recorder.Code != http.StatusAccepted || service.routineID != "routine-1" || service.setInput.RequestID != "routine-repair-1" || service.setInput.TaskInstruction != "Review the legacy queue." {
		t.Fatalf("Routine instruction response status=%d captured=%+v body=%s", recorder.Code, service.setInput, recorder.Body.String())
	}
}

func TestParseTaskTakeoverLeaseRoutes(t *testing.T) {
	list, ok := parsePath("/api/workbench/companies/company-1/missions/mission-1/takeover-leases")
	if !ok || list.companyID != "company-1" || list.missionID != "mission-1" || list.endpoint != "missions.takeover_leases" {
		t.Fatalf("takeover lease list route = %+v, ok=%v", list, ok)
	}
	create, ok := parsePath("/api/workbench/companies/company-1/missions/mission-1/tasks/task-1/takeover-lease")
	if !ok || create.companyID != "company-1" || create.missionID != "mission-1" || create.resourceID != "task-1" || create.endpoint != "tasks.takeover_lease" {
		t.Fatalf("takeover lease create route = %+v, ok=%v", create, ok)
	}
	for _, action := range []string{"snapshot", "release"} {
		parsed, ok := parsePath("/api/workbench/companies/company-1/missions/mission-1/takeover-leases/lease-1/" + action)
		if !ok || parsed.companyID != "company-1" || parsed.missionID != "mission-1" || parsed.resourceID != "lease-1" || parsed.endpoint != "takeover_leases."+action {
			t.Fatalf("takeover lease %s route = %+v, ok=%v", action, parsed, ok)
		}
	}
}

func TestHTTPCreatesFormalMissionChangeRequest(t *testing.T) {
	body := []byte(`{"requestId":"change-command-1","changeSummary":"Update the accepted behavior for empty result sets.","proposedTitle":"API revision","proposedGoal":"Return an empty item list for an empty dataset.","proposedAcceptanceContract":{"revision":"text-acceptance@1","required_text":["Mission ID: {{mission_id}}","Task ID: {{task_id}}","Acknowledgement:","Task summary:"]},"blockPreviousResults":true}`)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/workbench/companies/company-1/missions/mission-1/change-requests", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	service := &fakeMissionChangeRequestCommandService{result: kernel.MissionChangeRequest{ID: "change-1", MissionID: "mission-1", State: "queued"}}
	NewHandler(fakeReadModel{}, service).ServeHTTP(recorder, request)
	if recorder.Code != http.StatusAccepted {
		t.Fatalf("formal change request status=%d body=%s, want %d", recorder.Code, recorder.Body.String(), http.StatusAccepted)
	}
	if service.companyID != "company-1" || service.missionID != "mission-1" || service.request.RequestID != "change-command-1" || service.request.ChangeSummary == "" || service.request.ProposedAcceptanceContract == nil || !service.request.BlockPreviousResults {
		t.Fatalf("formal change request was not correctly scoped or decoded: %+v", service)
	}
}

func TestHTTPListsAndTransitionsFormalMissionChangeRequests(t *testing.T) {
	for _, item := range []struct {
		method       string
		path         string
		body         string
		action       string
		impactDigest string
	}{
		{method: http.MethodGet, path: "/api/workbench/companies/company-1/missions/mission-1/change-requests", action: "list"},
		{method: http.MethodPost, path: "/api/workbench/companies/company-1/missions/mission-1/change-requests/change-1/consider", body: `{"requestId":"consider-1"}`, action: "consider"},
		{method: http.MethodPost, path: "/api/workbench/companies/company-1/missions/mission-1/change-requests/change-1/decline", body: `{"requestId":"decline-1"}`, action: "decline"},
		{method: http.MethodPost, path: "/api/workbench/companies/company-1/missions/mission-1/change-requests/change-1/apply", body: `{"requestId":"apply-1","impactSHA256":"` + strings.Repeat("a", 64) + `"}`, action: "apply", impactDigest: strings.Repeat("a", 64)},
	} {
		service := &fakeMissionChangeRequestCommandService{result: kernel.MissionChangeRequest{ID: "change-1", MissionID: "mission-1", State: "queued"}}
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(item.method, item.path, bytes.NewReader([]byte(item.body)))
		if item.method == http.MethodPost {
			request.Header.Set("Content-Type", "application/json")
		}
		NewHandler(fakeReadModel{}, service).ServeHTTP(recorder, request)
		wantStatus := http.StatusAccepted
		if item.method == http.MethodGet {
			wantStatus = http.StatusOK
		}
		if recorder.Code != wantStatus {
			t.Fatalf("%s %s status=%d body=%s want=%d", item.method, item.path, recorder.Code, recorder.Body.String(), wantStatus)
		}
		if service.action != item.action || service.companyID != "company-1" || service.missionID != "mission-1" {
			t.Fatalf("%s route dispatch = %+v", item.action, service)
		}
		if item.action != "list" && (service.changeID != "change-1" || service.command.ImpactSHA256 != item.impactDigest) {
			t.Fatalf("%s command target/body = %+v", item.action, service)
		}
	}
}

func TestHTTPTaskTakeoverLeaseCommandsAreScopedAndDecoded(t *testing.T) {
	for _, item := range []struct {
		method string
		path   string
		body   string
		action string
	}{
		{method: http.MethodGet, path: "/api/workbench/companies/company-1/missions/mission-1/takeover-leases", action: "list"},
		{method: http.MethodPost, path: "/api/workbench/companies/company-1/missions/mission-1/tasks/task-1/takeover-lease", body: `{"requestId":"takeover-create"}`, action: "create"},
		{method: http.MethodPost, path: "/api/workbench/companies/company-1/missions/mission-1/takeover-leases/lease-1/snapshot", body: `{"requestId":"takeover-snapshot","baseWorkspaceDigest":"` + strings.Repeat("a", 64) + `","baseWorkspaceRevision":7,"content":"# handover","humanEffortSeconds":42}`, action: "snapshot"},
		{method: http.MethodPost, path: "/api/workbench/companies/company-1/missions/mission-1/takeover-leases/lease-1/release", body: `{"requestId":"takeover-release"}`, action: "release"},
	} {
		service := &fakeTaskTakeoverLeaseService{result: kernel.TaskTakeoverLease{LeaseID: "lease-1", MissionID: "mission-1", State: "granted"}}
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(item.method, item.path, bytes.NewReader([]byte(item.body)))
		if item.method == http.MethodPost {
			request.Header.Set("Content-Type", "application/json")
		}
		NewHandler(fakeReadModel{}, service).ServeHTTP(recorder, request)
		wantStatus := http.StatusAccepted
		if item.method == http.MethodGet {
			wantStatus = http.StatusOK
		}
		if recorder.Code != wantStatus {
			t.Fatalf("%s %s status=%d body=%s want=%d", item.method, item.path, recorder.Code, recorder.Body.String(), wantStatus)
		}
		if service.action != item.action || service.companyID != "company-1" || service.missionID != "mission-1" {
			t.Fatalf("%s takeover dispatch = %+v", item.action, service)
		}
		if item.action == "create" && (service.taskID != "task-1" || service.leaseRequest.RequestID != "takeover-create") {
			t.Fatalf("takeover create target/body = %+v", service)
		}
		if item.action == "snapshot" && (service.leaseID != "lease-1" || service.snapshot.BaseWorkspaceRevision != 7 || service.snapshot.HumanEffortSeconds != 42 || service.snapshot.Content != "# handover") {
			t.Fatalf("takeover snapshot target/body = %+v", service)
		}
		if item.action == "release" && (service.leaseID != "lease-1" || service.leaseRequest.RequestID != "takeover-release") {
			t.Fatalf("takeover release target/body = %+v", service)
		}
	}
}

func TestHTTPAcceptsHumanInterventionStateCommands(t *testing.T) {
	for _, item := range []struct {
		action string
		state  string
	}{
		{action: "acknowledge", state: "acknowledged"},
		{action: "resolve", state: "resolved"},
	} {
		service := &fakeHumanInterventionCommandService{}
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/api/workbench/companies/company-1/human-interventions/intervention-1/"+item.action, bytes.NewReader([]byte(`{"requestId":"human-state-1"}`)))
		request.Header.Set("Content-Type", "application/json")
		NewHandler(fakeReadModel{}, service).ServeHTTP(recorder, request)
		if recorder.Code != http.StatusAccepted {
			t.Fatalf("state command %s status = %d: %s", item.action, recorder.Code, recorder.Body.String())
		}
		if service.companyID != "company-1" || service.request.InterventionID != "intervention-1" || service.request.State != item.state || service.request.RequestID != "human-state-1" {
			t.Fatalf("state command %s was not scoped from the route: company=%q request=%+v", item.action, service.companyID, service.request)
		}
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

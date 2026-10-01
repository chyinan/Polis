// pattern: Functional Core
package control

import (
	"context"
	"encoding/json"
	"strings"

	"polis/internal/core"
	"polis/internal/domainworkflow"
	"polis/internal/kernel"
	"polis/internal/organization"
	"polis/internal/provider"
	"polis/internal/taskvalidation"
)

type CreateMissionRequest struct {
	Title              string                             `json:"title"`
	Goal               string                             `json:"goal"`
	RequestID          string                             `json:"requestId"`
	AcceptanceContract *taskvalidation.AcceptanceContract `json:"acceptanceContract,omitempty"`
}

type MissionCommandRequest struct {
	MissionID string `json:"missionId,omitempty"`
	RequestID string `json:"requestId"`
}

type CreateOperatorInstructionRequest struct {
	MissionID  string `json:"missionId"`
	TaskID     string `json:"taskId,omitempty"`
	EmployeeID string `json:"employeeId,omitempty"`
	Content    string `json:"content"`
	RequestID  string `json:"requestId"`
}

type CreateMissionChangeRequestRequest struct {
	RequestID                  string                             `json:"requestId"`
	ChangeSummary              string                             `json:"changeSummary"`
	ProposedTitle              string                             `json:"proposedTitle,omitempty"`
	ProposedGoal               string                             `json:"proposedGoal,omitempty"`
	ProposedAcceptanceContract *taskvalidation.AcceptanceContract `json:"proposedAcceptanceContract,omitempty"`
	BlockPreviousResults       bool                               `json:"blockPreviousResults"`
}

type MissionChangeRequestCommand struct {
	RequestID    string `json:"requestId"`
	ImpactSHA256 string `json:"impactSha256,omitempty"`
}

type OperatorInstructionReceipt struct {
	InstructionID         string                                      `json:"instructionId"`
	MissionID             *string                                     `json:"missionId"`
	TaskID                *string                                     `json:"taskId"`
	EmployeeID            *string                                     `json:"employeeId"`
	Content               string                                      `json:"content"`
	State                 string                                      `json:"state"`
	CreatedAt             string                                      `json:"createdAt"`
	ResponseOutcome       *string                                     `json:"responseOutcome"`
	ResponseSummary       *string                                     `json:"responseSummary"`
	RespondedByEmployeeID *string                                     `json:"respondedByEmployeeId"`
	RespondedAt           *string                                     `json:"respondedAt"`
	Responses             []kernel.OperatorInstructionResponseSummary `json:"responses"`
	RequestID             string                                      `json:"requestId"`
	Accepted              bool                                        `json:"accepted"`
	AcceptedAt            string                                      `json:"acceptedAt"`
}

type CreateCompanyRequest struct {
	organization.CompanyDraft
	RequestID string `json:"requestId"`
}

type UpdateCompanyRequest struct {
	organization.CompanyDraft
	RequestID string `json:"requestId"`
}

type UpdateRuntimeSettingsRequest struct {
	Provider  string `json:"provider"`
	Model     string `json:"model"`
	Effort    string `json:"effort"`
	Profile   string `json:"profile"`
	RequestID string `json:"requestId"`
}

type ConfigureNotificationRouteRequest struct {
	Adapter       string `json:"adapter"`
	Destination   string `json:"destination"`
	SafetyAlias   string `json:"safetyAlias,omitempty"`
	CredentialRef string `json:"credentialRef,omitempty"`
	Enabled       bool   `json:"enabled"`
	RequestID     string `json:"requestId"`
}

type NotificationTestReceipt struct {
	IntentID   string `json:"intentId"`
	DeliveryID string `json:"deliveryId"`
	Adapter    string `json:"adapter"`
	State      string `json:"state"`
	RequestID  string `json:"requestId"`
	Accepted   bool   `json:"accepted"`
	AcceptedAt string `json:"acceptedAt"`
}

type SetHumanInterventionStateRequest struct {
	InterventionID string `json:"interventionId,omitempty"`
	State          string `json:"state,omitempty"`
	RequestID      string `json:"requestId"`
}

type CompanySummary struct {
	ID            string                       `json:"id"`
	Name          string                       `json:"name"`
	WorkspaceRoot string                       `json:"workspaceRoot"`
	State         string                       `json:"state"`
	Roster        []organization.EmployeeDraft `json:"roster"`
}

type CommandReceipt struct {
	CommandID      string `json:"commandId"`
	CommandType    string `json:"commandType"`
	TargetType     string `json:"targetType"`
	TargetID       string `json:"targetId"`
	RequestID      string `json:"requestId"`
	Accepted       bool   `json:"accepted"`
	AcceptedAt     string `json:"acceptedAt"`
	ResultingState string `json:"resultingState"`
}

type CommandService interface {
	CreateMission(ctx context.Context, companyID string, request CreateMissionRequest) (CommandReceipt, error)
	StartMission(ctx context.Context, companyID string, request MissionCommandRequest) (CommandReceipt, error)
	CancelMission(ctx context.Context, companyID string, request MissionCommandRequest) (CommandReceipt, error)
}

type MissionLifecycleService interface {
	PauseMission(ctx context.Context, companyID string, request MissionCommandRequest) (CommandReceipt, error)
	ResumeMission(ctx context.Context, companyID string, request MissionCommandRequest) (CommandReceipt, error)
}

type OrganizationService interface {
	ListCompanies(ctx context.Context) ([]CompanySummary, error)
	GetCompany(ctx context.Context, companyID string) (CompanySummary, error)
	CreateCompany(ctx context.Context, request CreateCompanyRequest) (CommandReceipt, error)
	UpdateCompany(ctx context.Context, companyID string, request UpdateCompanyRequest) (CommandReceipt, error)
	ArchiveCompany(ctx context.Context, companyID, requestID string) (CommandReceipt, error)
}

type RuntimeSettings struct {
	CompanyID                   string `json:"companyId"`
	WorkerMode                  string `json:"workerMode"`
	Provider                    string `json:"provider"`
	Model                       string `json:"model"`
	Effort                      string `json:"effort"`
	Profile                     string `json:"profile"`
	AuthReadiness               string `json:"authReadiness"`
	RuntimeVersion              string `json:"runtimeVersion"`
	RuntimeReadiness            string `json:"runtimeReadiness"`
	ProductSurfaceQualification string `json:"productSurfaceQualification"`
	WorkspaceRoot               string `json:"workspaceRoot"`
	PostgreSQLStatus            string `json:"postgresqlStatus"`
	CASStatus                   string `json:"casStatus"`
	EventStreamStatus           string `json:"eventStreamStatus"`
}

type RuntimeSettingsService interface {
	GetRuntimeSettings(ctx context.Context, companyID string) (RuntimeSettings, error)
	UpdateRuntimeSettings(ctx context.Context, companyID string, request UpdateRuntimeSettingsRequest) (CommandReceipt, error)
}

type CodexModelCatalogService interface {
	ListCodexModels(ctx context.Context, companyID string) (provider.CodexModelCatalog, error)
}

type OperatorInstructionService interface {
	CreateOperatorInstruction(ctx context.Context, companyID string, request CreateOperatorInstructionRequest) (OperatorInstructionReceipt, error)
}

type MissionChangeRequestService interface {
	CreateMissionChangeRequest(ctx context.Context, companyID, missionID string, request CreateMissionChangeRequestRequest) (kernel.MissionChangeRequest, error)
	ListMissionChangeRequests(ctx context.Context, companyID, missionID string) ([]kernel.MissionChangeRequest, error)
	ConsiderMissionChangeRequest(ctx context.Context, companyID, missionID, changeRequestID string, request MissionChangeRequestCommand) (kernel.MissionChangeRequest, error)
	DeclineMissionChangeRequest(ctx context.Context, companyID, missionID, changeRequestID string, request MissionChangeRequestCommand) (kernel.MissionChangeRequest, error)
	ApplyMissionChangeRequest(ctx context.Context, companyID, missionID, changeRequestID string, request MissionChangeRequestCommand) (kernel.MissionChangeRequest, error)
}

type TaskTakeoverLeaseCommand struct {
	RequestID string `json:"requestId"`
}

type TaskTakeoverSnapshotCommand struct {
	RequestID             string `json:"requestId"`
	BaseWorkspaceDigest   string `json:"baseWorkspaceDigest"`
	BaseWorkspaceRevision int64  `json:"baseWorkspaceRevision"`
	Content               string `json:"content"`
	HumanEffortSeconds    int64  `json:"humanEffortSeconds"`
}

type TaskTakeoverLeaseService interface {
	ListTaskTakeoverLeases(ctx context.Context, companyID, missionID string) ([]kernel.TaskTakeoverLease, error)
	CreateTaskTakeoverLease(ctx context.Context, companyID, missionID, taskID string, request TaskTakeoverLeaseCommand) (kernel.TaskTakeoverLease, error)
	SubmitTaskTakeoverSnapshot(ctx context.Context, companyID, missionID, leaseID string, request TaskTakeoverSnapshotCommand) (kernel.TaskTakeoverLease, error)
	ReleaseTaskTakeoverLease(ctx context.Context, companyID, missionID, leaseID string, request TaskTakeoverLeaseCommand) (kernel.TaskTakeoverLease, error)
}

type NotificationCommandService interface {
	ConfigureNotificationRoute(ctx context.Context, companyID string, request ConfigureNotificationRouteRequest) (CommandReceipt, error)
	TestNotification(ctx context.Context, companyID, requestID string) (NotificationTestReceipt, error)
}

type HumanInterventionCommandService interface {
	SetHumanInterventionState(ctx context.Context, companyID string, request SetHumanInterventionStateRequest) (CommandReceipt, error)
}

type RegisterProjectEnvironmentRequest struct {
	RevisionID          string          `json:"revisionId"`
	ProfileID           string          `json:"profileId"`
	MissionID           string          `json:"missionId"`
	SourceInputID       string          `json:"sourceInputId"`
	SourceInputRevision string          `json:"sourceInputRevision"`
	PolicyManifest      json.RawMessage `json:"policyManifest"`
	ToolchainSHA256     string          `json:"toolchainSha256"`
	RequestID           string          `json:"requestId"`
}

type EnvironmentPolicyDecisionRequest struct {
	Decision  string `json:"decision"`
	Rationale string `json:"rationale"`
	RequestID string `json:"requestId"`
}

type EnvironmentExecutorQualificationRequest struct {
	Decision              string `json:"decision"`
	EvidenceInputID       string `json:"evidenceInputId"`
	EvidenceInputRevision string `json:"evidenceInputRevision"`
	QualifiedUntil        string `json:"qualifiedUntil,omitempty"`
	Rationale             string `json:"rationale"`
	RequestID             string `json:"requestId"`
}

type EnsureEnvironmentRequest struct {
	RequestID string `json:"requestId"`
}

type StartProjectJobRequest struct {
	TaskID                string   `json:"taskId"`
	SessionID             string   `json:"sessionId"`
	EnvironmentRevisionID string   `json:"environmentRevisionId"`
	HandoverID            string   `json:"handoverId,omitempty"`
	Kind                  string   `json:"kind"`
	ServiceID             string   `json:"serviceId,omitempty"`
	ScriptPath            string   `json:"scriptPath"`
	Args                  []string `json:"args"`
	RequestID             string   `json:"requestId"`
}

type StopProjectJobRequest struct {
	RequestID string `json:"requestId"`
}

type CreateProjectJobBrowserSessionRequest struct {
	RequestID string `json:"requestId"`
}

type EnvironmentLifecycleService interface {
	RegisterProjectEnvironment(ctx context.Context, companyID string, request RegisterProjectEnvironmentRequest) (kernel.ProjectEnvironmentRevision, error)
	DecideProjectEnvironmentPolicy(ctx context.Context, companyID, revisionID string, request EnvironmentPolicyDecisionRequest) (kernel.Receipt, error)
	DecideProjectEnvironmentExecutorQualification(ctx context.Context, companyID, revisionID string, request EnvironmentExecutorQualificationRequest) (kernel.Receipt, error)
	EnsureProjectEnvironment(ctx context.Context, companyID, revisionID string, request EnsureEnvironmentRequest) (kernel.EnvironmentPreparationRun, error)
}

type ProjectJobLifecycleService interface {
	StartProjectJob(ctx context.Context, companyID string, request StartProjectJobRequest) (kernel.JobRunRecord, error)
	StopProjectJob(ctx context.Context, companyID, jobID string, request StopProjectJobRequest) (kernel.JobRunRecord, error)
	GetProjectJobLogs(ctx context.Context, companyID, jobID string) (kernel.JobRunLogArtifact, error)
	CreateProjectJobBrowserSession(ctx context.Context, companyID, jobID string, request CreateProjectJobBrowserSessionRequest) (ServiceBrowserSession, error)
}

type CreateProjectEnvironmentHandoverRequest struct {
	SourceJobID               string `json:"sourceJobId"`
	TargetEnvironmentRevision string `json:"targetEnvironmentRevisionId"`
	RequestID                 string `json:"requestId"`
}

type ProjectEnvironmentHandoverCommandService interface {
	CreateTaskEnvironmentHandover(ctx context.Context, companyID, taskID string, request CreateProjectEnvironmentHandoverRequest) (kernel.CrossBackendHandoverRecord, error)
}

type CapabilityCatalog struct {
	Skills                                    []kernel.SkillRevision                `json:"skills"`
	MCPServers                                []kernel.MCPServerDefinition          `json:"mcpServers"`
	MCPPackages                               []kernel.StdioMCPPackageRevision      `json:"mcpPackages"`
	Qualifications                            []kernel.CapabilityQualification      `json:"qualifications"`
	Bindings                                  []kernel.EmployeeCapabilityBinding    `json:"bindings"`
	Decisions                                 []kernel.CapabilityDecisionRecord     `json:"decisions"`
	RuntimeQualifications                     []kernel.StdioMCPRuntimeQualification `json:"runtimeQualifications"`
	Revocations                               []kernel.CapabilityRevocationStatus   `json:"revocations"`
	RevocationsTruncated                      bool                                  `json:"revocationsTruncated"`
	RuntimeObservationAvailable               bool                                  `json:"runtimeObservationAvailable"`
	StreamableHTTPRuntimeObservationAvailable bool                                  `json:"streamableHttpRuntimeObservationAvailable"`
}

type ImportSkillRequest struct {
	ID             string          `json:"id"`
	PublisherScope string          `json:"publisherScope"`
	PackageID      string          `json:"packageId"`
	Revision       string          `json:"revision"`
	DisplayName    string          `json:"displayName"`
	SourceRef      string          `json:"sourceRef"`
	ContentDigest  string          `json:"contentDigest"`
	Manifest       json.RawMessage `json:"manifest"`
	RequestID      string          `json:"requestId"`
}

type ImportReadOnlySkillPackageRequest struct {
	Revision  string
	RequestID string
	Archive   []byte
}

type ImportStdioMCPPackageRequest struct {
	ServerID  string
	Revision  string
	RequestID string
	Archive   []byte
}

type SkillPackageImportService interface {
	ImportReadOnlySkillPackage(ctx context.Context, companyID string, request ImportReadOnlySkillPackageRequest) (kernel.SkillRevision, error)
}

type StdioMCPPackageImportService interface {
	ImportStdioMCPPackage(ctx context.Context, companyID string, request ImportStdioMCPPackageRequest) (kernel.StdioMCPPackageRevision, error)
}

type ObserveStdioMCPRuntimeRequest struct {
	ServerID                  string `json:"serverId"`
	PackageRevisionID         string `json:"packageRevisionId"`
	CapabilityQualificationID string `json:"capabilityQualificationId"`
	RequestID                 string `json:"requestId"`
}

type StdioMCPRuntimeObservationService interface {
	ObserveStdioMCPRuntime(ctx context.Context, companyID string, request ObserveStdioMCPRuntimeRequest) (kernel.StdioMCPRuntimeQualification, error)
}

type StdioMCPRuntimeObserver interface {
	Observe(ctx context.Context, companyID string, request ObserveStdioMCPRuntimeRequest) (kernel.StdioMCPRuntimeQualification, error)
	Close() error
}

type ObserveStreamableHTTPMCPRuntimeRequest struct {
	CapabilityID              string `json:"capabilityId"`
	CapabilityQualificationID string `json:"capabilityQualificationId"`
	RequestID                 string `json:"requestId"`
}

type StreamableHTTPMCPRuntimeObservationService interface {
	ObserveStreamableHTTPMCPRuntime(ctx context.Context, companyID string, request ObserveStreamableHTTPMCPRuntimeRequest) (kernel.StdioMCPRuntimeQualification, error)
}

type StreamableHTTPMCPRuntimeObserver interface {
	Observe(ctx context.Context, endpoint string) (json.RawMessage, error)
}

type RegisterMCPRequest struct {
	ID               string   `json:"id"`
	Name             string   `json:"name"`
	Transport        string   `json:"transport"`
	Endpoint         *string  `json:"endpoint"`
	Command          *string  `json:"command"`
	Args             []string `json:"args"`
	DescriptorDigest string   `json:"descriptorDigest"`
	RequestID        string   `json:"requestId"`
}

type QualifyCapabilityRequest struct {
	CapabilityKind string `json:"capabilityKind"`
	CapabilityID   string `json:"capabilityId"`
	RequestID      string `json:"requestId"`
}

type DecideCapabilityRequest struct {
	CapabilityKind  string `json:"capabilityKind"`
	CapabilityID    string `json:"capabilityId"`
	QualificationID string `json:"qualificationId"`
	Decision        string `json:"decision"`
	Rationale       string `json:"rationale"`
	RequestID       string `json:"requestId"`
}

type ApproveStdioMCPRuntimeQualificationRequest struct {
	RuntimeQualificationID string `json:"runtimeQualificationId"`
	Rationale              string `json:"rationale"`
	RequestID              string `json:"requestId"`
}

type BindEmployeeCapabilityRequest struct {
	EmployeeID      string `json:"employeeId"`
	CapabilityKind  string `json:"capabilityKind"`
	CapabilityID    string `json:"capabilityId"`
	QualificationID string `json:"qualificationId"`
	Reason          string `json:"reason"`
	RequestID       string `json:"requestId"`
}

type CapabilityService interface {
	ListCapabilityCatalog(ctx context.Context, companyID string) (CapabilityCatalog, error)
	ImportSkill(ctx context.Context, companyID string, request ImportSkillRequest) (kernel.SkillRevision, error)
	RegisterMCP(ctx context.Context, companyID string, request RegisterMCPRequest) (kernel.MCPServerDefinition, error)
	QualifyCapability(ctx context.Context, companyID string, request QualifyCapabilityRequest) (kernel.CapabilityQualification, error)
	DecideCapability(ctx context.Context, companyID string, request DecideCapabilityRequest) (kernel.Receipt, error)
	BindEmployeeCapability(ctx context.Context, companyID string, request BindEmployeeCapabilityRequest) (kernel.Receipt, error)
	RevokeEmployeeCapability(ctx context.Context, companyID string, request BindEmployeeCapabilityRequest) (kernel.Receipt, error)
}

type MCPRuntimeQualificationService interface {
	ApproveStdioMCPRuntimeQualification(ctx context.Context, companyID string, request ApproveStdioMCPRuntimeQualificationRequest) (kernel.Receipt, error)
}

type DomainEvidenceCommandRequest struct {
	domainworkflow.DomainEvidenceSubmission
	RequestID string `json:"requestId"`
}

type DomainEvidenceReviewCommandRequest struct {
	domainworkflow.DomainEvidenceReview
	RequestID string `json:"requestId"`
}

type DomainEvidenceSubstantiveAssessmentCommandRequest struct {
	domainworkflow.DomainEvidenceSubstantiveReview
	EvidenceDigest string `json:"evidenceDigest"`
	RequestID      string `json:"requestId"`
}

type DomainProfileQualificationCommandRequest struct {
	Decision              kernel.DomainProfileQualificationDecision `json:"decision"`
	ProfileRevision       string                                    `json:"profileRevision"`
	EvidenceInputID       string                                    `json:"evidenceInputId,omitempty"`
	EvidenceInputRevision string                                    `json:"evidenceInputRevision,omitempty"`
	Rationale             string                                    `json:"rationale"`
	RequestID             string                                    `json:"requestId"`
}

type DomainContentSourceAuthorizationCommandRequest struct {
	SourceInputID       string                          `json:"sourceInputId"`
	SourceInputRevision int64                           `json:"sourceInputRevision,string"`
	SourceSHA256        string                          `json:"sourceSha256"`
	State               kernel.DomainContentSourceState `json:"state"`
	Rationale           string                          `json:"rationale"`
	RequestID           string                          `json:"requestId"`
}

type DomainContentDraftCommandRequest struct {
	DraftInputID       string   `json:"draftInputId"`
	DraftInputRevision int64    `json:"draftInputRevision,string"`
	WriterEmployeeID   string   `json:"writerEmployeeId"`
	CriticalClaims     []string `json:"criticalClaims"`
	ConstraintsPassed  bool     `json:"constraintsPassed"`
	RequestID          string   `json:"requestId"`
}

type DomainContentReviewCommandRequest struct {
	Review       domainworkflow.ContentReview         `json:"review"`
	Sample       domainworkflow.ContentSampleEvidence `json:"sample"`
	CorrectionID string                               `json:"correctionId,omitempty"`
	RequestID    string                               `json:"requestId"`
}

type DomainContentPublicationCommandRequest struct {
	ReviewID  string `json:"reviewId"`
	RequestID string `json:"requestId"`
}

type DomainContentCorrectionCommandRequest struct {
	PublicationID           string `json:"publicationId"`
	CorrectionDraftInputID  string `json:"correctionDraftInputId"`
	CorrectionDraftRevision string `json:"correctionDraftRevision,string"`
	Rationale               string `json:"rationale"`
	RequestID               string `json:"requestId"`
}

type DomainContentFeedbackCommandRequest struct {
	PublicationID string                               `json:"publicationId"`
	Category      kernel.DomainContentFeedbackCategory `json:"category"`
	Note          string                               `json:"note"`
	RequestID     string                               `json:"requestId"`
}

type ContentOperationsCommandService interface {
	SetContentSourceAuthorization(ctx context.Context, companyID string, request DomainContentSourceAuthorizationCommandRequest) (kernel.DomainContentSourceEventRecord, error)
	RegisterContentDraft(ctx context.Context, companyID string, request DomainContentDraftCommandRequest) (kernel.DomainContentDraftRecord, error)
	RecordContentReview(ctx context.Context, companyID, draftInputID string, draftRevision int64, request DomainContentReviewCommandRequest) (kernel.DomainContentReviewRecord, error)
	SimulateContentPublication(ctx context.Context, companyID string, request DomainContentPublicationCommandRequest) (kernel.DomainContentPublicationRecord, error)
	RecordContentCorrection(ctx context.Context, companyID string, request DomainContentCorrectionCommandRequest) (kernel.DomainContentCorrectionRecord, error)
	RecordContentFeedback(ctx context.Context, companyID string, request DomainContentFeedbackCommandRequest) (kernel.DomainContentFeedbackRecord, error)
}

type DomainEvidenceService interface {
	ListDomainEvidence(ctx context.Context, companyID string) (kernel.DomainEvidenceLedger, error)
	RecordDomainEvidence(ctx context.Context, companyID string, request DomainEvidenceCommandRequest) (kernel.DomainEvidenceRecord, error)
	RecordDomainEvidenceReview(ctx context.Context, companyID, recordID string, request DomainEvidenceReviewCommandRequest) (kernel.DomainEvidenceReviewRecord, error)
	ListDomainEvidenceArtifactPreviewEntries(ctx context.Context, companyID, recordID string, area domainworkflow.DomainEvidenceArea) (kernel.DomainEvidenceArtifactPreviewManifest, error)
	ReadDomainEvidenceArtifactPreview(ctx context.Context, companyID, recordID string, area domainworkflow.DomainEvidenceArea, relativePath string) (kernel.DomainEvidenceArtifactPreview, error)
}

type DomainEvidenceSubstantiveAssessmentService interface {
	RecordDomainEvidenceSubstantiveAssessment(ctx context.Context, companyID, recordID string, request DomainEvidenceSubstantiveAssessmentCommandRequest) (kernel.DomainEvidenceSubstantiveAssessmentRecord, error)
}

type DomainProfileQualificationService interface {
	RecordDomainProfileQualification(ctx context.Context, companyID, profileID string, request DomainProfileQualificationCommandRequest) (kernel.DomainProfileQualificationRecord, error)
}

type WorkerAdapter interface {
	Mode() string
	Readiness(context.Context) error
	ToolSurface() provider.ToolSurface
	Start(ctx context.Context, companyID, missionID string) error
	Stop(ctx context.Context, companyID, missionID string) error
	Close()
}

func validateCreateMissionRequest(request CreateMissionRequest) error {
	if strings.TrimSpace(request.Title) == "" || len(request.Title) > 200 {
		return core.Malformed
	}
	if strings.TrimSpace(request.Goal) == "" || len(request.Goal) > core.MaxContent {
		return core.Malformed
	}
	if taskvalidation.ValidateContract(request.AcceptanceContract) != nil {
		return core.Malformed
	}
	return validateRequestID(request.RequestID)
}

func validateMissionCommandRequest(request MissionCommandRequest) error {
	if request.MissionID != "" && !core.ValidID(request.MissionID) {
		return core.Malformed
	}
	return validateRequestID(request.RequestID)
}

func validateOperatorInstructionRequest(request CreateOperatorInstructionRequest) error {
	if (request.MissionID != "" && !core.ValidID(request.MissionID)) || (request.TaskID != "" && (!core.ValidID(request.TaskID) || request.MissionID == "")) || (request.EmployeeID != "" && !core.ValidID(request.EmployeeID)) || strings.TrimSpace(request.Content) == "" || len(request.Content) > core.MaxContent {
		return core.Malformed
	}
	return validateRequestID(request.RequestID)
}

func validateCreateCompanyRequest(request CreateCompanyRequest) error {
	if err := organization.ValidateCompanyDraft(request.CompanyDraft); err != nil {
		return core.Malformed
	}
	return validateRequestID(request.RequestID)
}

func validateUpdateCompanyRequest(companyID string, request UpdateCompanyRequest) error {
	if request.ID != "" && request.ID != companyID {
		return core.Malformed
	}
	request.ID = companyID
	if err := organization.ValidateCompanyDraft(request.CompanyDraft); err != nil {
		return core.Malformed
	}
	return validateRequestID(request.RequestID)
}

func validateRequestID(requestID string) error {
	if !core.ValidID(requestID) {
		return core.Malformed
	}
	return nil
}

// pattern: Functional Core
package control

import (
	"context"
	"encoding/json"
	"strings"

	"polis/internal/core"
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

type OperatorInstructionReceipt struct {
	InstructionID string `json:"instructionId"`
	MissionID     string `json:"missionId"`
	TaskID        string `json:"taskId,omitempty"`
	EmployeeID    string `json:"employeeId,omitempty"`
	Content       string `json:"content"`
	State         string `json:"state"`
	RequestID     string `json:"requestId"`
	Accepted      bool   `json:"accepted"`
	AcceptedAt    string `json:"acceptedAt"`
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
	Adapter     string `json:"adapter"`
	Destination string `json:"destination"`
	Enabled     bool   `json:"enabled"`
	RequestID   string `json:"requestId"`
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

type NotificationCommandService interface {
	ConfigureNotificationRoute(ctx context.Context, companyID string, request ConfigureNotificationRouteRequest) (CommandReceipt, error)
	TestNotification(ctx context.Context, companyID, requestID string) (NotificationTestReceipt, error)
}

type CapabilityCatalog struct {
	Skills     []kernel.SkillRevision             `json:"skills"`
	MCPServers []kernel.MCPServerDefinition       `json:"mcpServers"`
	Bindings   []kernel.EmployeeCapabilityBinding `json:"bindings"`
	Loads      []kernel.SkillLoad                 `json:"loads"`
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

type ImportSkillDocumentRequest struct {
	PublisherScope string `json:"publisherScope"`
	PackageID      string `json:"packageId"`
	Revision       string `json:"revision"`
	SourceRef      string `json:"sourceRef"`
	Document       string `json:"document"`
	RequestID      string `json:"requestId"`
}

type SkillRevisionStatusRequest struct {
	SkillID   string `json:"skillId"`
	RequestID string `json:"requestId"`
}

type ImportSkillLibraryRequest struct {
	PublisherScope string `json:"publisherScope"`
	PackageID      string `json:"packageId,omitempty"`
	Revision       string `json:"revision"`
	RequestID      string `json:"requestId"`
}

type LibraryImportResult struct {
	Skills   []kernel.SkillRevision       `json:"skills"`
	MCP      []kernel.MCPServerDefinition `json:"mcpServers"`
	Rejected []kernel.ScanRejection       `json:"rejected"`
}

type ListVisibleResourcesRequest struct {
	EmployeeID string `json:"employeeId"`
	ServerID   string `json:"serverId"`
	Cursor     string `json:"cursor"`
	Limit      int    `json:"limit"`
	RequestID  string `json:"requestId"`
}

type ListVisibleToolsRequest struct {
	EmployeeID string `json:"employeeId"`
	Cursor     string `json:"cursor"`
	Limit      int    `json:"limit"`
	RequestID  string `json:"requestId"`
}

type DescribeVisibleToolRequest struct {
	EmployeeID string `json:"employeeId"`
	Handle     string `json:"handle"`
	RequestID  string `json:"requestId"`
}

type CallMCPToolRequest struct {
	EmployeeID       string          `json:"employeeId"`
	TaskID           string          `json:"taskId,omitempty"`
	BindingID        string          `json:"bindingId"`
	Handle           string          `json:"handle"`
	DescriptorDigest string          `json:"descriptorDigest"`
	Arguments        json.RawMessage `json:"arguments"`
	IdempotencyKey   string          `json:"idempotencyKey"`
	PolicyRevision   string          `json:"policyRevision,omitempty"`
	AuthorizationRef string          `json:"authorizationRef,omitempty"`
	RequestID        string          `json:"requestId"`
}

type UpsertMCPToolCallQualificationRequest struct {
	BindingID        string `json:"bindingId"`
	DefinitionID     string `json:"definitionId"`
	Handle           string `json:"handle"`
	DescriptorDigest string `json:"descriptorDigest"`
	PolicyRevision   string `json:"policyRevision,omitempty"`
	EffectClass      string `json:"effectClass,omitempty"`
	Status           string `json:"status,omitempty"`
	AuthorizationRef string `json:"authorizationRef,omitempty"`
	QualifiedBy      string `json:"qualifiedBy,omitempty"`
	Notes            string `json:"notes,omitempty"`
	RequestID        string `json:"requestId"`
}

type ListMCPToolCallQualificationsRequest struct {
	BindingID string `json:"bindingId,omitempty"`
	RequestID string `json:"requestId"`
}

type ListMCPToolCallReceiptsRequest struct {
	BindingID string `json:"bindingId,omitempty"`
	Handle    string `json:"handle,omitempty"`
	Limit     int    `json:"limit,omitempty"`
	RequestID string `json:"requestId"`
}

type ListVisibleSkillsRequest struct {
	EmployeeID string `json:"employeeId"`
	Cursor     string `json:"cursor"`
	Limit      int    `json:"limit"`
	RequestID  string `json:"requestId"`
}

type LoadSkillRequest struct {
	TaskID     string `json:"taskId"`
	EmployeeID string `json:"employeeId"`
	SkillID    string `json:"skillId"`
	RequestID  string `json:"requestId"`
}

type ReadSkillReferenceRequest struct {
	TaskID     string `json:"taskId"`
	EmployeeID string `json:"employeeId"`
	SkillID    string `json:"skillId"`
	Reference  string `json:"reference"`
	RequestID  string `json:"requestId"`
}

type QualifyMCPRequest struct {
	MCPID     string `json:"mcpId"`
	RequestID string `json:"requestId"`
}

type UpsertCapabilityBindingRequest struct {
	EmployeeID       string   `json:"employeeId"`
	SkillRevisionIDs []string `json:"skillRevisionIds"`
	MCPDefinitionIDs []string `json:"mcpDefinitionIds"`
	Status           string   `json:"status"`
	RequestID        string   `json:"requestId"`
}

type CapabilityService interface {
	ListCapabilityCatalog(ctx context.Context, companyID string) (CapabilityCatalog, error)
	ImportSkill(ctx context.Context, companyID string, request ImportSkillRequest) (kernel.SkillRevision, error)
	ImportSkillDocument(ctx context.Context, companyID string, request ImportSkillDocumentRequest) (kernel.SkillRevision, error)
	ImportSkillLibrary(ctx context.Context, companyID string, request ImportSkillLibraryRequest) (LibraryImportResult, error)
	SetSkillRevisionStatus(ctx context.Context, companyID string, request SkillRevisionStatusRequest, action string) (kernel.SkillRevision, error)
	ListVisibleResources(ctx context.Context, companyID string, request ListVisibleResourcesRequest) (kernel.ResourceSummaryPage, error)
	ListVisibleTools(ctx context.Context, companyID string, request ListVisibleToolsRequest) (kernel.ToolSummaryPage, error)
	DescribeVisibleTool(ctx context.Context, companyID string, request DescribeVisibleToolRequest) (kernel.ToolDescriptorView, error)
	ListVisibleSkills(ctx context.Context, companyID string, request ListVisibleSkillsRequest) (kernel.SkillSummaryPage, error)
	LoadSkill(ctx context.Context, companyID string, request LoadSkillRequest) (kernel.SkillLoad, error)
	ReadSkillReference(ctx context.Context, companyID string, request ReadSkillReferenceRequest) (kernel.SkillReferenceRead, error)
	RegisterMCP(ctx context.Context, companyID string, request RegisterMCPRequest) (kernel.MCPServerDefinition, error)
	QualifyMCP(ctx context.Context, companyID string, request QualifyMCPRequest) (kernel.MCPServerDefinition, error)
	UpsertCapabilityBinding(ctx context.Context, companyID string, request UpsertCapabilityBindingRequest) (kernel.EmployeeCapabilityBinding, error)
	ListMCPToolCallQualifications(ctx context.Context, companyID string, request ListMCPToolCallQualificationsRequest) ([]kernel.MCPToolCallQualification, error)
	UpsertMCPToolCallQualification(ctx context.Context, companyID string, request UpsertMCPToolCallQualificationRequest) (kernel.MCPToolCallQualification, error)
	CallMCPTool(ctx context.Context, companyID string, request CallMCPToolRequest) (kernel.MCPToolCallReceipt, error)
	ListMCPToolCallReceipts(ctx context.Context, companyID string, request ListMCPToolCallReceiptsRequest) ([]kernel.MCPToolCallReceipt, error)
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
	if !core.ValidID(request.MissionID) || (request.TaskID != "" && !core.ValidID(request.TaskID)) || (request.EmployeeID != "" && !core.ValidID(request.EmployeeID)) || strings.TrimSpace(request.Content) == "" || len(request.Content) > core.MaxContent {
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

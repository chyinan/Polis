// pattern: Imperative Shell
package control

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"polis/internal/core"
	"polis/internal/kernel"
	"polis/internal/organization"
	"polis/internal/provider"
)

type Service struct {
	runtime      *kernel.Kernel
	worker       WorkerAdapter
	modelCatalog provider.ModelCatalogProvider
}

func NewService(runtime *kernel.Kernel, worker WorkerAdapter, modelCatalog ...provider.ModelCatalogProvider) *Service {
	service := &Service{runtime: runtime, worker: worker}
	if len(modelCatalog) > 0 {
		service.modelCatalog = modelCatalog[0]
	}
	return service
}

func (s *Service) CreateMission(ctx context.Context, companyID string, request CreateMissionRequest) (CommandReceipt, error) {
	if err := validateCreateMissionRequest(request); err != nil {
		return CommandReceipt{}, err
	}
	receipt, err := s.runtime.TXCreateMissionGoalWithAcceptance(ctx, s.runtime.LocalScope(companyID), request.Title, request.Goal, request.AcceptanceContract, request.RequestID)
	if err != nil {
		return CommandReceipt{}, err
	}
	return commandReceipt("mission.create", request.RequestID, receipt.ID, receipt.Status), nil
}

func (s *Service) ListCompanies(ctx context.Context) ([]CompanySummary, error) {
	companies, err := s.runtime.ListCompanyDetails(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]CompanySummary, 0, len(companies))
	for _, company := range companies {
		result = append(result, companySummary(company))
	}
	return result, nil
}

func (s *Service) GetCompany(ctx context.Context, companyID string) (CompanySummary, error) {
	company, err := s.runtime.CompanyDetails(ctx, companyID)
	if err != nil {
		return CompanySummary{}, err
	}
	return companySummary(company), nil
}

func (s *Service) ListCapabilityCatalog(ctx context.Context, companyID string) (CapabilityCatalog, error) {
	catalog, err := s.runtime.ListCapabilityCatalog(ctx, companyID)
	if err != nil {
		return CapabilityCatalog{}, err
	}
	if catalog.Bindings == nil {
		catalog.Bindings = []kernel.EmployeeCapabilityBinding{}
	}
	if catalog.Loads == nil {
		catalog.Loads = []kernel.SkillLoad{}
	}
	return CapabilityCatalog{Skills: catalog.Skills, MCPServers: catalog.MCPServers, Bindings: catalog.Bindings, Loads: catalog.Loads}, nil
}

func (s *Service) ImportSkill(ctx context.Context, companyID string, request ImportSkillRequest) (kernel.SkillRevision, error) {
	if strings.TrimSpace(request.RequestID) == "" {
		return kernel.SkillRevision{}, core.Malformed
	}
	return s.runtime.TXImportSkillRevision(ctx, companyID, kernel.SkillRevisionInput{ID: request.ID, PublisherScope: request.PublisherScope, PackageID: request.PackageID, Revision: request.Revision, DisplayName: request.DisplayName, SourceRef: request.SourceRef, ContentDigest: request.ContentDigest, Manifest: request.Manifest})
}

func (s *Service) RegisterMCP(ctx context.Context, companyID string, request RegisterMCPRequest) (kernel.MCPServerDefinition, error) {
	if strings.TrimSpace(request.RequestID) == "" {
		return kernel.MCPServerDefinition{}, core.Malformed
	}
	return s.runtime.TXRegisterMCPServerDefinition(ctx, companyID, kernel.MCPServerDefinitionInput{ID: request.ID, Name: request.Name, Transport: request.Transport, Endpoint: request.Endpoint, Command: request.Command, Args: request.Args, DescriptorDigest: request.DescriptorDigest})
}

func (s *Service) ImportSkillDocument(ctx context.Context, companyID string, request ImportSkillDocumentRequest) (kernel.SkillRevision, error) {
	if strings.TrimSpace(request.RequestID) == "" {
		return kernel.SkillRevision{}, core.Malformed
	}
	return s.runtime.TXImportSkillDocument(ctx, companyID, kernel.SkillDocumentInput{PublisherScope: request.PublisherScope, PackageID: request.PackageID, Revision: request.Revision, SourceRef: request.SourceRef, Document: []byte(request.Document)})
}

func (s *Service) ImportSkillLibrary(ctx context.Context, companyID string, request ImportSkillLibraryRequest) (LibraryImportResult, error) {
	if strings.TrimSpace(request.RequestID) == "" || (request.PublisherScope != "group" && request.PublisherScope != "company") {
		return LibraryImportResult{}, core.Malformed
	}
	root := strings.TrimSpace(os.Getenv("POLIS_SKILL_LIBRARY_ROOT"))
	if root == "" {
		return LibraryImportResult{}, fmt.Errorf("%w: POLIS_SKILL_LIBRARY_ROOT is not configured", core.Denied)
	}
	if strings.TrimSpace(request.PackageID) != "" {
		skill, readErr := kernel.ReadSkillPackage(root, strings.TrimSpace(request.PackageID))
		if readErr != nil {
			return LibraryImportResult{}, readErr
		}
		item, importErr := s.runtime.TXImportSkillDocument(ctx, companyID, kernel.SkillDocumentInput{PublisherScope: request.PublisherScope, Revision: request.Revision, SourceRef: skill.SourceRef, Document: skill.Document, ScriptsPresent: skill.ScriptsPresent, References: skill.References})
		if importErr != nil {
			return LibraryImportResult{Skills: []kernel.SkillRevision{}, MCP: []kernel.MCPServerDefinition{}, Rejected: []kernel.ScanRejection{{SourceRef: skill.SourceRef, Reason: importErr.Error()}}}, nil
		}
		return LibraryImportResult{Skills: []kernel.SkillRevision{item}, MCP: []kernel.MCPServerDefinition{}, Rejected: []kernel.ScanRejection{}}, nil
	}
	scan, err := kernel.ScanSkillLibrary(root)
	if err != nil {
		return LibraryImportResult{}, err
	}
	result := LibraryImportResult{Skills: []kernel.SkillRevision{}, MCP: []kernel.MCPServerDefinition{}, Rejected: scan.Rejected}
	for _, skill := range scan.Skills {
		item, importErr := s.runtime.TXImportSkillDocument(ctx, companyID, kernel.SkillDocumentInput{PublisherScope: request.PublisherScope, Revision: request.Revision, SourceRef: skill.SourceRef, Document: skill.Document, ScriptsPresent: skill.ScriptsPresent, References: skill.References})
		if importErr != nil {
			result.Rejected = append(result.Rejected, kernel.ScanRejection{SourceRef: skill.SourceRef, Reason: importErr.Error()})
			continue
		}
		result.Skills = append(result.Skills, item)
	}
	for _, definition := range scan.MCP {
		item, importErr := s.runtime.TXImportMCPDocument(ctx, companyID, definition.Document)
		if importErr != nil {
			result.Rejected = append(result.Rejected, kernel.ScanRejection{SourceRef: definition.SourceRef, Reason: importErr.Error()})
			continue
		}
		result.MCP = append(result.MCP, item)
	}
	return result, nil
}

func (s *Service) QualifyMCP(ctx context.Context, companyID string, request QualifyMCPRequest) (kernel.MCPServerDefinition, error) {
	if strings.TrimSpace(request.RequestID) == "" {
		return kernel.MCPServerDefinition{}, core.Malformed
	}
	return s.runtime.QualifyRegisteredMCP(ctx, companyID, request.MCPID)
}

func (s *Service) ListVisibleResources(ctx context.Context, companyID string, request ListVisibleResourcesRequest) (kernel.ResourceSummaryPage, error) {
	if strings.TrimSpace(request.RequestID) == "" {
		return kernel.ResourceSummaryPage{}, core.Malformed
	}
	limit := request.Limit
	if limit == 0 {
		limit = 32
	}
	return s.runtime.TXListVisibleResources(ctx, companyID, request.EmployeeID, request.ServerID, request.Cursor, limit)
}

func (s *Service) DescribeVisibleTool(ctx context.Context, companyID string, request DescribeVisibleToolRequest) (kernel.ToolDescriptorView, error) {
	if strings.TrimSpace(request.RequestID) == "" {
		return kernel.ToolDescriptorView{}, core.Malformed
	}
	return s.runtime.TXDescribeVisibleTool(ctx, companyID, request.EmployeeID, request.Handle)
}

func (s *Service) ListVisibleTools(ctx context.Context, companyID string, request ListVisibleToolsRequest) (kernel.ToolSummaryPage, error) {
	if strings.TrimSpace(request.RequestID) == "" {
		return kernel.ToolSummaryPage{}, core.Malformed
	}
	limit := request.Limit
	if limit == 0 {
		limit = 32
	}
	return s.runtime.TXListVisibleTools(ctx, companyID, request.EmployeeID, request.Cursor, limit)
}

func (s *Service) ListVisibleSkills(ctx context.Context, companyID string, request ListVisibleSkillsRequest) (kernel.SkillSummaryPage, error) {
	if strings.TrimSpace(request.RequestID) == "" {
		return kernel.SkillSummaryPage{}, core.Malformed
	}
	limit := request.Limit
	if limit == 0 {
		limit = 32
	}
	return s.runtime.TXListVisibleSkills(ctx, companyID, request.EmployeeID, request.Cursor, limit)
}

func (s *Service) LoadSkill(ctx context.Context, companyID string, request LoadSkillRequest) (kernel.SkillLoad, error) {
	if strings.TrimSpace(request.RequestID) == "" {
		return kernel.SkillLoad{}, core.Malformed
	}
	return s.runtime.TXLoadSkill(ctx, companyID, request.TaskID, request.EmployeeID, request.SkillID)
}

func (s *Service) ReadSkillReference(ctx context.Context, companyID string, request ReadSkillReferenceRequest) (kernel.SkillReferenceRead, error) {
	if strings.TrimSpace(request.RequestID) == "" {
		return kernel.SkillReferenceRead{}, core.Malformed
	}
	return s.runtime.TXReadSkillReference(ctx, companyID, request.TaskID, request.EmployeeID, request.SkillID, request.Reference)
}

func (s *Service) SetSkillRevisionStatus(ctx context.Context, companyID string, request SkillRevisionStatusRequest, action string) (kernel.SkillRevision, error) {
	if strings.TrimSpace(request.RequestID) == "" || (action != "approve" && action != "revoke") {
		return kernel.SkillRevision{}, core.Malformed
	}
	return s.runtime.TXSetSkillRevisionStatus(ctx, companyID, request.SkillID, action)
}

func (s *Service) UpsertCapabilityBinding(ctx context.Context, companyID string, request UpsertCapabilityBindingRequest) (kernel.EmployeeCapabilityBinding, error) {
	if strings.TrimSpace(request.RequestID) == "" {
		return kernel.EmployeeCapabilityBinding{}, core.Malformed
	}
	return s.runtime.TXUpsertEmployeeCapabilityBinding(ctx, companyID, kernel.CapabilityBindingInput{EmployeeID: request.EmployeeID, SkillRevisionIDs: request.SkillRevisionIDs, MCPDefinitionIDs: request.MCPDefinitionIDs, Status: request.Status})
}

func (s *Service) ListMCPToolCallQualifications(ctx context.Context, companyID string, request ListMCPToolCallQualificationsRequest) ([]kernel.MCPToolCallQualification, error) {
	if strings.TrimSpace(request.RequestID) == "" {
		return nil, core.Malformed
	}
	return s.runtime.TXListMCPToolCallQualifications(ctx, companyID, request.BindingID)
}

func (s *Service) UpsertMCPToolCallQualification(ctx context.Context, companyID string, request UpsertMCPToolCallQualificationRequest) (kernel.MCPToolCallQualification, error) {
	if strings.TrimSpace(request.RequestID) == "" {
		return kernel.MCPToolCallQualification{}, core.Malformed
	}
	policy := strings.TrimSpace(request.PolicyRevision)
	if policy == "" {
		policy = "r15-tools-call@1"
	}
	effect := strings.TrimSpace(request.EffectClass)
	if effect == "" {
		effect = "unknown"
	}
	status := strings.TrimSpace(request.Status)
	if status == "" {
		status = "approved"
	}
	return s.runtime.TXUpsertMCPToolCallQualification(ctx, companyID, kernel.MCPToolCallQualificationInput{BindingID: request.BindingID, DefinitionID: request.DefinitionID, Handle: request.Handle, DescriptorDigest: request.DescriptorDigest, PolicyRevision: policy, EffectClass: effect, Status: status, AuthorizationRef: request.AuthorizationRef, QualifiedBy: request.QualifiedBy, Notes: request.Notes})
}

func (s *Service) CallMCPTool(ctx context.Context, companyID string, request CallMCPToolRequest) (kernel.MCPToolCallReceipt, error) {
	if strings.TrimSpace(request.RequestID) == "" {
		return kernel.MCPToolCallReceipt{}, core.Malformed
	}
	policy := strings.TrimSpace(request.PolicyRevision)
	if policy == "" {
		policy = "r15-tools-call@1"
	}
	return s.runtime.TXCallMCPTool(ctx, kernel.ToolCallRequest{CompanyID: companyID, EmployeeID: request.EmployeeID, TaskID: request.TaskID, BindingID: request.BindingID, SessionID: request.SessionID, SessionEpoch: request.SessionEpoch, SessionIncarnation: request.SessionIncarnation, Handle: request.Handle, DescriptorDigest: request.DescriptorDigest, Arguments: request.Arguments, IdempotencyKey: request.IdempotencyKey, PolicyRevision: policy, AuthorizationRef: request.AuthorizationRef})
}

func (s *Service) ListMCPToolCallReceipts(ctx context.Context, companyID string, request ListMCPToolCallReceiptsRequest) ([]kernel.MCPToolCallReceipt, error) {
	if strings.TrimSpace(request.RequestID) == "" {
		return nil, core.Malformed
	}
	return s.runtime.TXListMCPToolCallReceipts(ctx, companyID, kernel.ListMCPToolCallReceiptsInput{BindingID: request.BindingID, Handle: request.Handle, Limit: request.Limit})
}

func (s *Service) GetMCPToolCallReceipt(ctx context.Context, companyID string, request GetMCPToolCallReceiptRequest) (kernel.MCPToolCallReceipt, error) {
	if strings.TrimSpace(request.RequestID) == "" {
		return kernel.MCPToolCallReceipt{}, core.Malformed
	}
	return s.runtime.TXGetMCPToolCallReceipt(ctx, companyID, request.ReceiptID)
}

func (s *Service) GetRuntimeSettings(ctx context.Context, companyID string) (RuntimeSettings, error) {
	company, err := s.runtime.CompanyDetails(ctx, companyID)
	if err != nil {
		return RuntimeSettings{}, err
	}
	settings := RuntimeSettings{CompanyID: companyID, WorkerMode: s.worker.Mode(), Provider: company.Provider, Model: company.Model, Effort: company.Effort, Profile: company.Profile, WorkspaceRoot: company.WorkspaceRoot, PostgreSQLStatus: "ready", CASStatus: "unavailable", EventStreamStatus: "deferred"}
	if root := strings.TrimSpace(os.Getenv("POLIS_BLOB_ROOT")); root != "" {
		if info, statErr := os.Stat(filepath.Clean(root)); statErr == nil && info.IsDir() {
			settings.CASStatus = "ready"
		}
	}
	if settings.WorkerMode == "deterministic" {
		settings.AuthReadiness = "not_required"
		if readiness, ok := providerCredentialReadiness(s.runtime, companyID, settings.Provider); ok {
			settings.AuthReadiness = readiness
		}
		settings.RuntimeVersion = "local"
		if settings.Provider == "deterministic" {
			settings.RuntimeReadiness = "ready"
		} else {
			settings.RuntimeReadiness = "restart_required"
		}
		settings.ProductSurfaceQualification = "not_applicable"
		return settings, nil
	}
	activeProvider := firstNonEmpty(os.Getenv("POLIS_PROVIDER_TRANSPORT"), "unconfigured")
	activeModel := firstNonEmpty(os.Getenv("POLIS_PROVIDER_MODEL"), "unconfigured")
	activeEffort := firstNonEmpty(os.Getenv("POLIS_PROVIDER_EFFORT"), "unconfigured")
	if settings.Provider == "" {
		settings.Provider = activeProvider
	}
	if settings.Model == "" {
		settings.Model = activeModel
	}
	if settings.Effort == "" {
		settings.Effort = activeEffort
	}
	if settings.Profile == "" {
		settings.Profile = settings.Model + "/" + settings.Effort
	}
	settings.RuntimeVersion = firstNonEmpty(os.Getenv("POLIS_PROVIDER_EXPECTED_VERSION"), "unconfigured")
	settings.ProductSurfaceQualification = provider.ProductToolSurfaceQualification
	if readiness, ok := providerCredentialReadiness(s.runtime, companyID, settings.Provider); ok {
		settings.AuthReadiness = readiness
	} else {
		settings.AuthReadiness = providerAuthReadiness(activeProvider)
	}
	if settings.Provider != activeProvider || settings.Model != activeModel || settings.Effort != activeEffort {
		settings.RuntimeReadiness = "restart_required"
		return settings, nil
	}
	if err := s.worker.Readiness(ctx); err != nil {
		settings.RuntimeReadiness = "unavailable"
	} else {
		settings.RuntimeReadiness = "ready"
	}
	return settings, nil
}

func (s *Service) UpdateRuntimeSettings(ctx context.Context, companyID string, request UpdateRuntimeSettingsRequest) (CommandReceipt, error) {
	if !core.ValidID(companyID) || strings.TrimSpace(request.Provider) == "" || strings.TrimSpace(request.Model) == "" || strings.TrimSpace(request.Effort) == "" || strings.TrimSpace(request.Profile) == "" {
		return CommandReceipt{}, core.Malformed
	}
	if err := validateRequestID(request.RequestID); err != nil {
		return CommandReceipt{}, err
	}
	if request.Provider != "deterministic" && request.Provider != "fake" && request.Provider != "codex" && request.Provider != "openai" && request.Provider != "deepseek" {
		return CommandReceipt{}, core.Malformed
	}
	if request.Provider == "openai" || request.Provider == "deepseek" {
		endpoint, err := normalizeOpenAIEndpoint(request.Profile)
		if err != nil {
			return CommandReceipt{}, err
		}
		if err = validateOpenAIModel(request.Model); err != nil {
			return CommandReceipt{}, err
		}
		request.Profile = endpoint
		if request.Provider == "openai" {
			request.Effort = "chat"
		} else {
			request.Effort, err = normalizeDeepSeekEffort(request.Effort)
			if err != nil {
				return CommandReceipt{}, err
			}
		}
		key := strings.TrimSpace(request.APIKey)
		if request.ClearAPIKey && key != "" {
			return CommandReceipt{}, core.Malformed
		}
		if key != "" {
			if err = validateOpenAIAPIKey(key); err != nil {
				return CommandReceipt{}, err
			}
		}
		request.APIKey = key
	} else if strings.TrimSpace(request.APIKey) != "" || request.ClearAPIKey {
		return CommandReceipt{}, core.Malformed
	}
	receipt, err := s.runtime.TXUpdateRuntimeSettings(ctx, companyID, request.Provider, request.Model, request.Effort, request.Profile, request.RequestID)
	if err != nil {
		return CommandReceipt{}, err
	}
	if err = syncProviderCredential(s.runtime, companyID, request); err != nil {
		return CommandReceipt{}, err
	}
	return commandReceiptForTarget("runtime.settings.update", request.RequestID, "company", companyID, receipt.Status), nil
}

func providerCredentialReadiness(runtime *kernel.Kernel, companyID, providerName string) (string, bool) {
	switch providerName {
	case "openai":
		return openAIAuthReadiness(runtime.OpenAICredentialConfigured(companyID)), true
	case "deepseek":
		return openAIAuthReadiness(runtime.DeepSeekCredentialConfigured(companyID)), true
	default:
		return "", false
	}
}

func syncProviderCredential(runtime *kernel.Kernel, companyID string, request UpdateRuntimeSettingsRequest) error {
	if request.Provider != "openai" {
		if err := runtime.ClearOpenAIAPIKey(companyID); err != nil {
			return err
		}
	} else if request.ClearAPIKey {
		if err := runtime.ClearOpenAIAPIKey(companyID); err != nil {
			return err
		}
	} else if request.APIKey != "" {
		if err := runtime.WriteOpenAIAPIKey(companyID, request.APIKey); err != nil {
			return err
		}
	}
	if request.Provider != "deepseek" {
		if err := runtime.ClearDeepSeekAPIKey(companyID); err != nil {
			return err
		}
	} else if request.ClearAPIKey {
		if err := runtime.ClearDeepSeekAPIKey(companyID); err != nil {
			return err
		}
	} else if request.APIKey != "" {
		if err := runtime.WriteDeepSeekAPIKey(companyID, request.APIKey); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) ListCodexModels(ctx context.Context, companyID string) (provider.CodexModelCatalog, error) {
	if !core.ValidID(companyID) {
		return provider.CodexModelCatalog{}, core.Malformed
	}
	if _, err := s.runtime.CompanyDetails(ctx, companyID); err != nil {
		return provider.CodexModelCatalog{}, err
	}
	if s.modelCatalog == nil {
		return provider.CodexModelCatalog{}, errors.New("Codex model catalog is not configured")
	}
	return s.modelCatalog.ListModels(ctx)
}

func (s *Service) CreateCompany(ctx context.Context, request CreateCompanyRequest) (CommandReceipt, error) {
	if err := validateCreateCompanyRequest(request); err != nil {
		return CommandReceipt{}, err
	}
	if _, err := s.runtime.TXCreateCompanyWithOrganization(ctx, request.CompanyDraft); err != nil {
		return CommandReceipt{}, err
	}
	return commandReceiptForTarget("company.create", request.RequestID, "company", request.ID, "active"), nil
}

func (s *Service) UpdateCompany(ctx context.Context, companyID string, request UpdateCompanyRequest) (CommandReceipt, error) {
	request.ID = companyID
	if err := validateUpdateCompanyRequest(companyID, request); err != nil {
		return CommandReceipt{}, err
	}
	if _, err := s.runtime.TXUpdateCompanyWithOrganization(ctx, request.CompanyDraft, request.RequestID); err != nil {
		return CommandReceipt{}, err
	}
	return commandReceiptForTarget("company.update", request.RequestID, "company", companyID, "active"), nil
}

func (s *Service) ArchiveCompany(ctx context.Context, companyID, requestID string) (CommandReceipt, error) {
	if !core.ValidID(companyID) {
		return CommandReceipt{}, core.Malformed
	}
	if err := validateRequestID(requestID); err != nil {
		return CommandReceipt{}, err
	}
	receipt, err := s.runtime.TXArchiveCompany(ctx, companyID, requestID)
	if err != nil {
		return CommandReceipt{}, err
	}
	return commandReceiptForTarget("company.archive", requestID, "company", companyID, receipt.Status), nil
}

func (s *Service) StartMission(ctx context.Context, companyID string, request MissionCommandRequest) (CommandReceipt, error) {
	if err := validateMissionCommandRequest(request); err != nil {
		return CommandReceipt{}, err
	}
	if s.worker == nil {
		return CommandReceipt{}, errors.New("worker adapter is unavailable")
	}
	if err := s.worker.Readiness(ctx); err != nil {
		return CommandReceipt{}, core.ConflictError{Reason: "provider runtime is unavailable", CurrentState: "provider_unavailable"}
	}
	receipt, err := s.runtime.TXStartMissionCommand(ctx, s.runtime.LocalScope(companyID), request.MissionID, request.RequestID)
	if err != nil {
		return CommandReceipt{}, err
	}
	if err = s.worker.Start(ctx, companyID, request.MissionID); err != nil {
		return CommandReceipt{}, err
	}
	return commandReceipt("mission.start", request.RequestID, receipt.ID, receipt.Status), nil
}

func (s *Service) CancelMission(ctx context.Context, companyID string, request MissionCommandRequest) (CommandReceipt, error) {
	if err := validateMissionCommandRequest(request); err != nil {
		return CommandReceipt{}, err
	}
	if s.worker == nil {
		return CommandReceipt{}, errors.New("deterministic worker adapter is unavailable")
	}
	if err := s.worker.Stop(ctx, companyID, request.MissionID); err != nil {
		return CommandReceipt{}, err
	}
	receipt, err := s.runtime.TXCancelMission(ctx, s.runtime.LocalScope(companyID), request.MissionID, request.RequestID)
	if err != nil {
		return CommandReceipt{}, err
	}
	return commandReceipt("mission.cancel", request.RequestID, receipt.ID, receipt.Status), nil
}

func (s *Service) CreateOperatorInstruction(ctx context.Context, companyID string, request CreateOperatorInstructionRequest) (OperatorInstructionReceipt, error) {
	if err := validateOperatorInstructionRequest(request); err != nil {
		return OperatorInstructionReceipt{}, err
	}
	instruction, err := s.runtime.TXCreateOperatorInstruction(ctx, s.runtime.LocalScope(companyID), kernel.OperatorInstructionInput{MissionID: request.MissionID, TaskID: request.TaskID, EmployeeID: request.EmployeeID, Content: request.Content}, request.RequestID)
	if err != nil {
		return OperatorInstructionReceipt{}, err
	}
	return OperatorInstructionReceipt{InstructionID: instruction.ID, MissionID: instruction.MissionID, TaskID: instruction.TaskID, EmployeeID: instruction.EmployeeID, Content: instruction.Content, State: instruction.State, RequestID: request.RequestID, Accepted: true, AcceptedAt: time.Now().UTC().Format(time.RFC3339Nano)}, nil
}

func (s *Service) ConfigureNotificationRoute(ctx context.Context, companyID string, request ConfigureNotificationRouteRequest) (CommandReceipt, error) {
	if !core.ValidID(companyID) || (request.Adapter != "local" && request.Adapter != "webhook") || len(request.Destination) > 512 {
		return CommandReceipt{}, core.Malformed
	}
	if err := validateRequestID(request.RequestID); err != nil {
		return CommandReceipt{}, err
	}
	receipt, err := s.runtime.TXConfigureNotificationRoute(ctx, s.runtime.LocalScope(companyID), kernel.NotificationRouteInput{Adapter: request.Adapter, Destination: request.Destination, Enabled: request.Enabled}, request.RequestID)
	if err != nil {
		return CommandReceipt{}, err
	}
	return commandReceiptForTarget("notification.route.update", request.RequestID, "company", companyID, receipt.Status), nil
}

func (s *Service) TestNotification(ctx context.Context, companyID, requestID string) (NotificationTestReceipt, error) {
	if !core.ValidID(companyID) {
		return NotificationTestReceipt{}, core.Malformed
	}
	if err := validateRequestID(requestID); err != nil {
		return NotificationTestReceipt{}, err
	}
	receipt, err := s.runtime.TXTestNotification(ctx, s.runtime.LocalScope(companyID), requestID)
	if err != nil {
		return NotificationTestReceipt{}, err
	}
	return NotificationTestReceipt{IntentID: receipt.IntentID, DeliveryID: receipt.DeliveryID, Adapter: receipt.Adapter, State: receipt.State, RequestID: requestID, Accepted: true, AcceptedAt: time.Now().UTC().Format(time.RFC3339Nano)}, nil
}

func (s *Service) Close() {
	if s != nil && s.worker != nil {
		s.worker.Close()
	}
}

func commandReceipt(commandType, requestID, targetID, resultingState string) CommandReceipt {
	return CommandReceipt{CommandID: requestID, CommandType: commandType, TargetType: "mission", TargetID: targetID, RequestID: requestID, Accepted: true, AcceptedAt: time.Now().UTC().Format(time.RFC3339Nano), ResultingState: resultingState}
}

func commandReceiptForTarget(commandType, requestID, targetType, targetID, resultingState string) CommandReceipt {
	return CommandReceipt{CommandID: requestID, CommandType: commandType, TargetType: targetType, TargetID: targetID, RequestID: requestID, Accepted: true, AcceptedAt: time.Now().UTC().Format(time.RFC3339Nano), ResultingState: resultingState}
}

func companySummary(company kernel.CompanyDetails) CompanySummary {
	roster := make([]organization.EmployeeDraft, len(company.Roster))
	copy(roster, company.Roster)
	return CompanySummary{ID: company.ID, Name: company.Name, WorkspaceRoot: company.WorkspaceRoot, State: company.State, Roster: roster}
}

func openAIAuthReadiness(configured bool) string {
	if configured {
		return "configured"
	}
	return "missing"
}

func providerAuthReadiness(transport string) string {
	if transport == "fake" {
		return "not_required"
	}
	path := strings.TrimSpace(os.Getenv("POLIS_PROVIDER_AUTH_FILE"))
	if path == "" {
		return "missing"
	}
	if _, err := os.Stat(filepath.Clean(path)); err != nil {
		return "invalid"
	}
	return "ready"
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

var _ OrganizationService = (*Service)(nil)
var _ RuntimeSettingsService = (*Service)(nil)
var _ OperatorInstructionService = (*Service)(nil)
var _ NotificationCommandService = (*Service)(nil)

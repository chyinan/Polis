// pattern: Imperative Shell
package control

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"polis/internal/core"
	"polis/internal/domainworkflow"
	"polis/internal/environment"
	githubfeedback "polis/internal/feedback/github"
	"polis/internal/intake"
	"polis/internal/kernel"
	"polis/internal/organization"
	"polis/internal/provider"
)

type Service struct {
	runtime                          *kernel.Kernel
	worker                           WorkerAdapter
	modelCatalog                     provider.ModelCatalogProvider
	preparationMu                    sync.RWMutex
	preparationRunner                ProjectEnvironmentPreparationExecutor
	preparationCtx                   context.Context
	preparationCancel                context.CancelFunc
	projectJobsMu                    sync.Mutex
	projectJobsClosed                bool
	projectJobsCtx                   context.Context
	projectJobsCancel                context.CancelFunc
	projectJobsWG                    sync.WaitGroup
	projectJobs                      map[string]*activeProjectJob
	projectJobRunner                 ProjectJobExecutor
	projectLifecycleMu               sync.RWMutex
	mcpRuntimeObserver               StdioMCPRuntimeObserver
	streamableHTTPMCPRuntimeObserver StreamableHTTPMCPRuntimeObserver
	githubFeedback                   GitHubFeedbackProvider
	githubCredentials                githubfeedback.GitHubCredentialStore
	githubFeedbackSchedulerEnabled   bool
	qqDispatchMu                     sync.Mutex
	qqSender                         QQNotificationSender
	qqRetryCancel                    context.CancelFunc
	qqRetryDone                      chan struct{}
	automaticProductDispatchCursor   string
}

type mcpObserverShutdownWorker interface {
	QuiesceForMCPObserver() error
	CloseAfterMCPObserver() error
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
	streamableHTTPWorkerSurfaceReady := s.streamableHTTPMCPRuntimeObserver != nil && s.worker != nil &&
		s.worker.ToolSurface().ManifestDigest == provider.ProductControlledMCPToolSurfaceV2().ManifestDigest
	for index := range catalog.Bindings {
		binding := &catalog.Bindings[index]
		if binding.CapabilityKind != "mcp" || binding.ExecutionStatus != "runtime_qualified_dispatch_unavailable" {
			continue
		}
		for _, server := range catalog.MCPServers {
			if server.ID == binding.CapabilityID && server.Transport == "streamable_http" {
				if s.streamableHTTPMCPRuntimeObserver == nil {
					binding.ExecutionStatus = "runtime_qualified_dispatch_disabled"
				} else if streamableHTTPWorkerSurfaceReady {
					binding.ExecutionStatus = "runtime_qualified_dispatch_available"
				}
				break
			}
		}
	}
	return CapabilityCatalog{Skills: catalog.Skills, MCPServers: catalog.MCPServers, MCPPackages: catalog.MCPPackages, Qualifications: catalog.Qualifications, Bindings: catalog.Bindings, Decisions: catalog.Decisions, RuntimeQualifications: catalog.RuntimeQualifications, RuntimeObservationAvailable: s.mcpRuntimeObserver != nil, StreamableHTTPRuntimeObservationAvailable: s.streamableHTTPMCPRuntimeObserver != nil}, nil
}

func (s *Service) SetStdioMCPRuntimeObserver(observer StdioMCPRuntimeObserver) {
	if s != nil {
		s.mcpRuntimeObserver = observer
	}
}

func (s *Service) SetStreamableHTTPMCPRuntimeObserver(observer StreamableHTTPMCPRuntimeObserver) {
	if s != nil {
		s.streamableHTTPMCPRuntimeObserver = observer
	}
}

func (s *Service) ObserveStreamableHTTPMCPRuntime(ctx context.Context, companyID string, request ObserveStreamableHTTPMCPRuntimeRequest) (kernel.StdioMCPRuntimeQualification, error) {
	if s == nil || s.runtime == nil || !core.ValidID(companyID) || !core.ValidID(request.CapabilityID) ||
		!core.ValidID(request.CapabilityQualificationID) || validateRequestID(request.RequestID) != nil {
		return kernel.StdioMCPRuntimeQualification{}, core.Malformed
	}
	observer := s.streamableHTTPMCPRuntimeObserver
	if observer == nil {
		return kernel.StdioMCPRuntimeQualification{}, core.Denied
	}
	lockedContext, releaseServerLock, err := s.runtime.LockStdioMCPPackageServer(ctx, companyID, request.CapabilityID)
	if err != nil {
		return kernel.StdioMCPRuntimeQualification{}, err
	}
	defer releaseServerLock()
	endpoint, err := s.runtime.StreamableHTTPMCPObservationTarget(lockedContext, companyID, request.CapabilityID, request.CapabilityQualificationID)
	if err != nil {
		return kernel.StdioMCPRuntimeQualification{}, err
	}
	toolList, err := observer.Observe(lockedContext, endpoint)
	if err != nil {
		return kernel.StdioMCPRuntimeQualification{}, err
	}
	return s.runtime.TXRecordStreamableHTTPMCPRuntimeQualification(lockedContext, companyID, kernel.StreamableHTTPMCPRuntimeObservationInput{
		CapabilityID: request.CapabilityID, CapabilityQualificationID: request.CapabilityQualificationID,
		ToolList: toolList, RequestID: request.RequestID,
	})
}

func (s *Service) ObserveStdioMCPRuntime(ctx context.Context, companyID string, request ObserveStdioMCPRuntimeRequest) (kernel.StdioMCPRuntimeQualification, error) {
	if s == nil || s.runtime == nil || !core.ValidID(companyID) || !core.ValidID(request.ServerID) || !core.ValidID(request.PackageRevisionID) || !core.ValidID(request.CapabilityQualificationID) || validateRequestID(request.RequestID) != nil {
		return kernel.StdioMCPRuntimeQualification{}, core.Malformed
	}
	if s.mcpRuntimeObserver == nil {
		return kernel.StdioMCPRuntimeQualification{}, core.Denied
	}
	return s.mcpRuntimeObserver.Observe(ctx, companyID, request)
}

func (s *Service) ImportSkill(ctx context.Context, companyID string, request ImportSkillRequest) (kernel.SkillRevision, error) {
	if strings.TrimSpace(request.RequestID) == "" {
		return kernel.SkillRevision{}, core.Malformed
	}
	return s.runtime.TXImportSkillRevisionCommand(ctx, companyID, kernel.SkillRevisionInput{ID: request.ID, PublisherScope: request.PublisherScope, PackageID: request.PackageID, Revision: request.Revision, DisplayName: request.DisplayName, SourceRef: request.SourceRef, ContentDigest: request.ContentDigest, Manifest: request.Manifest}, request.RequestID)
}

func (s *Service) ImportReadOnlySkillPackage(ctx context.Context, companyID string, request ImportReadOnlySkillPackageRequest) (kernel.SkillRevision, error) {
	if strings.TrimSpace(request.RequestID) == "" {
		return kernel.SkillRevision{}, core.Malformed
	}
	return s.runtime.TXImportReadOnlySkillPackage(ctx, companyID, kernel.ReadOnlySkillPackageInput{Revision: request.Revision, Archive: request.Archive}, request.RequestID)
}

func (s *Service) ImportStdioMCPPackage(ctx context.Context, companyID string, request ImportStdioMCPPackageRequest) (kernel.StdioMCPPackageRevision, error) {
	if err := validateRequestID(request.RequestID); err != nil {
		return kernel.StdioMCPPackageRevision{}, err
	}
	if strings.TrimSpace(request.ServerID) != "" && !core.ValidID(request.ServerID) {
		return kernel.StdioMCPPackageRevision{}, core.Malformed
	}
	return s.runtime.TXImportStdioMCPPackage(ctx, companyID, kernel.StdioMCPPackageInput{
		ServerID: request.ServerID, Revision: request.Revision, Archive: request.Archive, RequestID: request.RequestID,
	})
}

func (s *Service) RegisterMCP(ctx context.Context, companyID string, request RegisterMCPRequest) (kernel.MCPServerDefinition, error) {
	if strings.TrimSpace(request.RequestID) == "" {
		return kernel.MCPServerDefinition{}, core.Malformed
	}
	return s.runtime.TXRegisterMCPServerDefinitionCommand(ctx, companyID, kernel.MCPServerDefinitionInput{ID: request.ID, Name: request.Name, Transport: request.Transport, Endpoint: request.Endpoint, Command: request.Command, Args: request.Args, DescriptorDigest: request.DescriptorDigest}, request.RequestID)
}

func (s *Service) QualifyCapability(ctx context.Context, companyID string, request QualifyCapabilityRequest) (kernel.CapabilityQualification, error) {
	if err := validateRequestID(request.RequestID); err != nil {
		return kernel.CapabilityQualification{}, err
	}
	return s.runtime.TXQualifyCapability(ctx, companyID, kernel.CapabilityQualificationInput{CapabilityKind: request.CapabilityKind, CapabilityID: request.CapabilityID, RequestID: request.RequestID})
}

func (s *Service) DecideCapability(ctx context.Context, companyID string, request DecideCapabilityRequest) (kernel.Receipt, error) {
	if err := validateRequestID(request.RequestID); err != nil {
		return kernel.Receipt{}, err
	}
	return s.runtime.TXDecideCapability(ctx, companyID, kernel.CapabilityDecisionInput{CapabilityKind: request.CapabilityKind, CapabilityID: request.CapabilityID, QualificationID: request.QualificationID, Decision: request.Decision, Rationale: request.Rationale, RequestID: request.RequestID})
}

func (s *Service) ApproveStdioMCPRuntimeQualification(ctx context.Context, companyID string, request ApproveStdioMCPRuntimeQualificationRequest) (kernel.Receipt, error) {
	if err := validateRequestID(request.RequestID); err != nil {
		return kernel.Receipt{}, err
	}
	if err := validateRequestID(request.RuntimeQualificationID); err != nil {
		return kernel.Receipt{}, err
	}
	return kernel.Receipt{}, s.runtime.TXApproveStdioMCPRuntimeQualification(ctx, companyID, kernel.StdioMCPRuntimeDecisionInput{
		RuntimeQualificationID: request.RuntimeQualificationID, Rationale: request.Rationale, RequestID: request.RequestID,
	})
}

func (s *Service) BindEmployeeCapability(ctx context.Context, companyID string, request BindEmployeeCapabilityRequest) (kernel.Receipt, error) {
	if err := validateRequestID(request.RequestID); err != nil {
		return kernel.Receipt{}, err
	}
	return s.runtime.TXBindEmployeeCapability(ctx, companyID, kernel.EmployeeCapabilityBindingInput{EmployeeID: request.EmployeeID, CapabilityKind: request.CapabilityKind, CapabilityID: request.CapabilityID, QualificationID: request.QualificationID, Reason: request.Reason, RequestID: request.RequestID})
}

func (s *Service) RevokeEmployeeCapability(ctx context.Context, companyID string, request BindEmployeeCapabilityRequest) (kernel.Receipt, error) {
	if err := validateRequestID(request.RequestID); err != nil {
		return kernel.Receipt{}, err
	}
	return s.runtime.TXRevokeEmployeeCapability(ctx, companyID, kernel.EmployeeCapabilityBindingInput{EmployeeID: request.EmployeeID, CapabilityKind: request.CapabilityKind, CapabilityID: request.CapabilityID, QualificationID: request.QualificationID, Reason: request.Reason, RequestID: request.RequestID})
}

func (s *Service) ListDomainEvidence(ctx context.Context, companyID string) (kernel.DomainEvidenceLedger, error) {
	return s.runtime.ListDomainEvidence(ctx, companyID)
}

func (s *Service) SetContentSourceAuthorization(ctx context.Context, companyID string, request DomainContentSourceAuthorizationCommandRequest) (kernel.DomainContentSourceEventRecord, error) {
	if err := validateRequestID(request.RequestID); err != nil {
		return kernel.DomainContentSourceEventRecord{}, err
	}
	return s.runtime.TXSetContentSourceAuthorization(ctx, companyID, kernel.DomainContentSourceAuthorizationInput{
		InputID: request.SourceInputID, Revision: request.SourceInputRevision, SHA256: request.SourceSHA256,
		State: request.State, Rationale: request.Rationale, RequestID: request.RequestID,
	})
}

func (s *Service) RegisterContentDraft(ctx context.Context, companyID string, request DomainContentDraftCommandRequest) (kernel.DomainContentDraftRecord, error) {
	if err := validateRequestID(request.RequestID); err != nil {
		return kernel.DomainContentDraftRecord{}, err
	}
	snapshot, err := s.runtime.ReadContentOperationInput(ctx, companyID, request.DraftInputID, request.DraftInputRevision, 1<<20)
	if err != nil {
		return kernel.DomainContentDraftRecord{}, err
	}
	return s.runtime.TXRegisterContentDraft(ctx, companyID, kernel.DomainContentDraftInput{
		DraftInputID: request.DraftInputID,
		Draft: domainworkflow.ContentDraft{Revision: request.DraftInputRevision, WriterEmployeeID: request.WriterEmployeeID,
			BodySHA256: snapshot.Reference.SHA256, CriticalClaims: request.CriticalClaims, ConstraintsPassed: request.ConstraintsPassed},
		RequestID: request.RequestID,
	})
}

func (s *Service) RecordContentReview(ctx context.Context, companyID, draftInputID string, draftRevision int64, request DomainContentReviewCommandRequest) (kernel.DomainContentReviewRecord, error) {
	if err := validateRequestID(request.RequestID); err != nil {
		return kernel.DomainContentReviewRecord{}, err
	}
	return s.runtime.TXRecordContentReview(ctx, companyID, draftInputID, draftRevision, kernel.DomainContentReviewRequest{
		Review: request.Review, Sample: request.Sample, CorrectionID: request.CorrectionID, RequestID: request.RequestID,
	})
}

func (s *Service) SimulateContentPublication(ctx context.Context, companyID string, request DomainContentPublicationCommandRequest) (kernel.DomainContentPublicationRecord, error) {
	if err := validateRequestID(request.RequestID); err != nil {
		return kernel.DomainContentPublicationRecord{}, err
	}
	return s.runtime.TXSimulateContentPublication(ctx, companyID, request.ReviewID, request.RequestID)
}

func (s *Service) RecordContentCorrection(ctx context.Context, companyID string, request DomainContentCorrectionCommandRequest) (kernel.DomainContentCorrectionRecord, error) {
	if err := validateRequestID(request.RequestID); err != nil {
		return kernel.DomainContentCorrectionRecord{}, err
	}
	revision, err := strconv.ParseInt(request.CorrectionDraftRevision, 10, 64)
	if err != nil || revision < 1 || strconv.FormatInt(revision, 10) != request.CorrectionDraftRevision {
		return kernel.DomainContentCorrectionRecord{}, core.Malformed
	}
	return s.runtime.TXRecordContentCorrection(ctx, companyID, request.PublicationID, request.CorrectionDraftInputID, revision, request.Rationale, request.RequestID)
}

func (s *Service) RecordContentFeedback(ctx context.Context, companyID string, request DomainContentFeedbackCommandRequest) (kernel.DomainContentFeedbackRecord, error) {
	if err := validateRequestID(request.RequestID); err != nil {
		return kernel.DomainContentFeedbackRecord{}, err
	}
	return s.runtime.TXRecordContentFeedback(ctx, companyID, request.PublicationID, kernel.DomainContentFeedbackInput{
		Category: request.Category, Note: request.Note, RequestID: request.RequestID,
	})
}

func (s *Service) RecordDomainEvidence(ctx context.Context, companyID string, request DomainEvidenceCommandRequest) (kernel.DomainEvidenceRecord, error) {
	if err := validateRequestID(request.RequestID); err != nil {
		return kernel.DomainEvidenceRecord{}, err
	}
	return s.runtime.TXRecordDomainEvidence(ctx, companyID, request.DomainEvidenceSubmission, request.RequestID)
}

func (s *Service) RecordDomainEvidenceReview(ctx context.Context, companyID, recordID string, request DomainEvidenceReviewCommandRequest) (kernel.DomainEvidenceReviewRecord, error) {
	if err := validateRequestID(request.RequestID); err != nil {
		return kernel.DomainEvidenceReviewRecord{}, err
	}
	return s.runtime.TXRecordDomainEvidenceReview(ctx, companyID, recordID, kernel.DomainEvidenceReviewRequest{
		DomainEvidenceReview: request.DomainEvidenceReview,
		RequestID:            request.RequestID,
	})
}

func (s *Service) RecordDomainEvidenceSubstantiveAssessment(ctx context.Context, companyID, recordID string, request DomainEvidenceSubstantiveAssessmentCommandRequest) (kernel.DomainEvidenceSubstantiveAssessmentRecord, error) {
	if err := validateRequestID(request.RequestID); err != nil {
		return kernel.DomainEvidenceSubstantiveAssessmentRecord{}, err
	}
	return s.runtime.TXRecordDomainEvidenceSubstantiveAssessment(ctx, companyID, recordID, kernel.DomainEvidenceSubstantiveAssessmentRequest{
		DomainEvidenceSubstantiveReview: request.DomainEvidenceSubstantiveReview,
		EvidenceDigest:                  request.EvidenceDigest,
		RequestID:                       request.RequestID,
	})
}

func (s *Service) RecordDomainProfileQualification(ctx context.Context, companyID, profileID string, request DomainProfileQualificationCommandRequest) (kernel.DomainProfileQualificationRecord, error) {
	if err := validateRequestID(request.RequestID); err != nil {
		return kernel.DomainProfileQualificationRecord{}, err
	}
	evidenceRevision := int64(0)
	if request.Decision == kernel.DomainProfileQualificationQualified {
		parsed, err := strconv.ParseInt(request.EvidenceInputRevision, 10, 64)
		if err != nil || parsed < 1 || !core.ValidID(request.EvidenceInputID) {
			return kernel.DomainProfileQualificationRecord{}, core.Malformed
		}
		evidenceRevision = parsed
	} else if request.EvidenceInputID != "" || request.EvidenceInputRevision != "" {
		return kernel.DomainProfileQualificationRecord{}, core.Malformed
	}
	return s.runtime.TXRecordDomainProfileQualification(ctx, companyID, kernel.DomainProfileQualificationInput{
		ProfileID: profileID, ProfileRevision: request.ProfileRevision, Decision: request.Decision,
		EvidenceInputID: request.EvidenceInputID, EvidenceInputRevision: evidenceRevision,
		Rationale: request.Rationale, RequestID: request.RequestID,
	})
}

func (s *Service) ListDomainEvidenceArtifactPreviewEntries(ctx context.Context, companyID, recordID string, area domainworkflow.DomainEvidenceArea) (kernel.DomainEvidenceArtifactPreviewManifest, error) {
	return s.runtime.ListDomainEvidenceArtifactPreviewEntries(ctx, companyID, recordID, area)
}

func (s *Service) ReadDomainEvidenceArtifactPreview(ctx context.Context, companyID, recordID string, area domainworkflow.DomainEvidenceArea, relativePath string) (kernel.DomainEvidenceArtifactPreview, error) {
	return s.runtime.ReadDomainEvidenceArtifactPreview(ctx, companyID, recordID, area, relativePath)
}

func (s *Service) RegisterProjectEnvironment(ctx context.Context, companyID string, request RegisterProjectEnvironmentRequest) (kernel.ProjectEnvironmentRevision, error) {
	inputRevision, revisionErr := strconv.ParseInt(request.SourceInputRevision, 10, 64)
	if !core.ValidID(companyID) || !core.ValidID(request.MissionID) || !core.ValidID(request.SourceInputID) || revisionErr != nil || inputRevision <= 0 || validateRequestID(request.RequestID) != nil {
		return kernel.ProjectEnvironmentRevision{}, core.Malformed
	}
	return s.runtime.TXRegisterProjectEnvironmentRevision(ctx, companyID, kernel.ProjectEnvironmentRevisionInput{
		RevisionID: request.RevisionID, ProfileID: request.ProfileID, MissionID: request.MissionID, SourceInputID: request.SourceInputID, SourceInputRevision: inputRevision,
		PolicyManifest: request.PolicyManifest, ToolchainSHA256: request.ToolchainSHA256,
	}, request.RequestID)
}

func (s *Service) DecideProjectEnvironmentPolicy(ctx context.Context, companyID, revisionID string, request EnvironmentPolicyDecisionRequest) (kernel.Receipt, error) {
	if !core.ValidID(companyID) || !core.ValidID(revisionID) || validateRequestID(request.RequestID) != nil {
		return kernel.Receipt{}, core.Malformed
	}
	return s.runtime.TXRecordEnvironmentPolicyDecision(ctx, companyID, kernel.EnvironmentPolicyDecisionInput{
		RevisionID: revisionID, Decision: request.Decision, Rationale: request.Rationale, RequestID: request.RequestID,
	})
}

func (s *Service) DecideProjectEnvironmentExecutorQualification(ctx context.Context, companyID, revisionID string, request EnvironmentExecutorQualificationRequest) (kernel.Receipt, error) {
	evidenceRevision, err := strconv.ParseInt(request.EvidenceInputRevision, 10, 64)
	if !core.ValidID(companyID) || !core.ValidID(revisionID) || !core.ValidID(request.EvidenceInputID) || err != nil || evidenceRevision <= 0 || validateRequestID(request.RequestID) != nil {
		return kernel.Receipt{}, core.Malformed
	}
	var qualifiedUntil *time.Time
	if request.QualifiedUntil != "" {
		parsed, parseErr := time.Parse(time.RFC3339Nano, request.QualifiedUntil)
		if parseErr != nil {
			return kernel.Receipt{}, core.Malformed
		}
		qualifiedUntil = &parsed
	}
	return s.runtime.TXRecordEnvironmentExecutorQualification(ctx, companyID, kernel.EnvironmentExecutorQualificationInput{
		RevisionID: revisionID, Decision: request.Decision, EvidenceInputID: request.EvidenceInputID, EvidenceInputRevision: evidenceRevision,
		QualifiedUntil: qualifiedUntil, Rationale: request.Rationale, RequestID: request.RequestID,
	})
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
	settings.AuthReadiness = providerAuthReadiness(activeProvider)
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
	if request.Provider != "deterministic" && request.Provider != "fake" && request.Provider != "codex" {
		return CommandReceipt{}, core.Malformed
	}
	receipt, err := s.runtime.TXUpdateRuntimeSettings(ctx, companyID, request.Provider, request.Model, request.Effort, request.Profile, request.RequestID)
	if err != nil {
		return CommandReceipt{}, err
	}
	return commandReceiptForTarget("runtime.settings.update", request.RequestID, "company", companyID, receipt.Status), nil
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
	scope := s.runtime.LocalScope(companyID)
	prior, found, err := s.runtime.LookupMissionStartCommand(ctx, scope, request.MissionID, request.RequestID)
	if err != nil {
		return CommandReceipt{}, err
	}
	if found {
		s.projectLifecycleMu.Lock()
		defer s.projectLifecycleMu.Unlock()
		prior, found, err = s.runtime.LookupMissionStartCommand(ctx, scope, request.MissionID, request.RequestID)
		if err != nil {
			return CommandReceipt{}, err
		}
		if !found {
			return CommandReceipt{}, core.ConflictError{Reason: "Mission start receipt changed during lifecycle reconciliation", CurrentState: "start_outcome_unknown"}
		}
		return s.confirmMissionStartReplay(ctx, scope, request, prior)
	}
	if s.worker == nil {
		return CommandReceipt{}, errors.New("worker adapter is unavailable")
	}
	s.projectLifecycleMu.Lock()
	defer s.projectLifecycleMu.Unlock()
	prior, found, err = s.runtime.LookupMissionStartCommand(ctx, scope, request.MissionID, request.RequestID)
	if err != nil {
		return CommandReceipt{}, err
	}
	if found {
		return s.confirmMissionStartReplay(ctx, scope, request, prior)
	}
	if err := s.worker.Readiness(ctx); err != nil {
		intervention, interventionErr := recordProviderReadinessIntervention(ctx, s.runtime, companyID, request.MissionID, request.RequestID)
		if interventionErr != nil {
			return CommandReceipt{}, fmt.Errorf("provider runtime is unavailable and the required human intervention could not be persisted: %w", interventionErr)
		}
		if s.qqSender != nil {
			_, _ = s.DispatchHumanIntervention(ctx, companyID, intervention.ID)
		}
		return CommandReceipt{}, core.ConflictError{Reason: "provider runtime is unavailable", CurrentState: "provider_unavailable"}
	}
	receipt, err := s.runtime.TXStartMissionCommand(ctx, scope, request.MissionID, request.RequestID)
	if err != nil {
		return CommandReceipt{}, err
	}
	if err = s.worker.Start(ctx, companyID, request.MissionID); err != nil {
		if _, outcomeErr := s.runtime.TXRecordMissionStartOutcome(ctx, scope, request.MissionID, request.RequestID, "outcome_unknown"); outcomeErr != nil {
			return CommandReceipt{}, errors.Join(err, outcomeErr)
		}
		return CommandReceipt{}, err
	}
	if _, err = s.runtime.TXRecordMissionStartOutcome(ctx, scope, request.MissionID, request.RequestID, "started"); err != nil {
		return CommandReceipt{}, core.ConflictError{Reason: "Worker start returned successfully but its outcome was not persisted; reconcile the Mission before retrying", CurrentState: "start_outcome_unknown"}
	}
	return commandReceipt("mission.start", request.RequestID, receipt.ID, receipt.Status), nil
}

func (s *Service) confirmMissionStartReplay(ctx context.Context, scope kernel.Scope, request MissionCommandRequest, prior kernel.Receipt) (CommandReceipt, error) {
	outcome, found, err := s.runtime.LookupMissionStartOutcome(ctx, scope, request.MissionID, request.RequestID)
	if err != nil {
		return CommandReceipt{}, err
	}
	if !found || outcome.Status != "started" || outcome.RuntimeIncarnation != s.runtime.Incarnation() {
		return CommandReceipt{}, core.ConflictError{Reason: "Mission start is unresolved for a prior runtime incarnation. Host-side reconciliation must confirm the persisted WorkerSession process containment ended; the Workbench cannot verify or clear it, so do not retry yet", CurrentState: "reconcile_required"}
	}
	return commandReceipt("mission.start", request.RequestID, prior.ID, prior.Status), nil
}

func (s *Service) CancelMission(ctx context.Context, companyID string, request MissionCommandRequest) (CommandReceipt, error) {
	if err := validateMissionCommandRequest(request); err != nil {
		return CommandReceipt{}, err
	}
	scope := s.runtime.LocalScope(companyID)
	prior, found, err := s.runtime.LookupMissionCancelCommand(ctx, scope, request.MissionID, request.RequestID)
	if err != nil {
		return CommandReceipt{}, err
	}
	if found {
		return commandReceipt("mission.cancel", request.RequestID, prior.ID, prior.Status), nil
	}
	if s.worker == nil {
		return CommandReceipt{}, errors.New("deterministic worker adapter is unavailable")
	}
	s.projectLifecycleMu.Lock()
	defer s.projectLifecycleMu.Unlock()
	prior, found, err = s.runtime.LookupMissionCancelCommand(ctx, scope, request.MissionID, request.RequestID)
	if err != nil {
		return CommandReceipt{}, err
	}
	if found {
		return commandReceipt("mission.cancel", request.RequestID, prior.ID, prior.Status), nil
	}
	if s.worker == nil {
		return CommandReceipt{}, errors.New("deterministic worker adapter is unavailable")
	}
	if err = s.stopMissionProjectJobs(ctx, companyID, request.MissionID, request.RequestID, "cancellation"); err != nil {
		return CommandReceipt{}, err
	}
	if err = s.worker.Stop(ctx, companyID, request.MissionID); err != nil {
		return CommandReceipt{}, core.ConflictError{Reason: "worker stop is not confirmed; Mission cancellation is blocked", CurrentState: "active"}
	}
	outstandingWork, err := s.runtime.MissionHasOutstandingJobWork(ctx, companyID, request.MissionID)
	if err != nil {
		return CommandReceipt{}, err
	}
	if outstandingWork {
		return CommandReceipt{}, core.ConflictError{Reason: "Mission execution has not reached a confirmed stop; cancellation is blocked", CurrentState: "reconcile_required"}
	}
	receipt, err := s.runtime.TXCancelMission(ctx, scope, request.MissionID, request.RequestID)
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
	var taskID, employeeID *string
	if instruction.TaskID != "" {
		taskIDValue := instruction.TaskID
		taskID = &taskIDValue
	}
	if instruction.EmployeeID != "" {
		employeeIDValue := instruction.EmployeeID
		employeeID = &employeeIDValue
	}
	return OperatorInstructionReceipt{InstructionID: instruction.ID, MissionID: instruction.MissionID, TaskID: taskID, EmployeeID: employeeID, Content: instruction.Content, State: instruction.State, CreatedAt: instruction.CreatedAt, Responses: []kernel.OperatorInstructionResponseSummary{}, RequestID: request.RequestID, Accepted: true, AcceptedAt: time.Now().UTC().Format(time.RFC3339Nano)}, nil
}

func (s *Service) ConfigureNotificationRoute(ctx context.Context, companyID string, request ConfigureNotificationRouteRequest) (CommandReceipt, error) {
	if !core.ValidID(companyID) || (request.Adapter != "local" && request.Adapter != "webhook" && request.Adapter != "qq_official") || len(request.Destination) > 512 {
		return CommandReceipt{}, core.Malformed
	}
	if err := validateRequestID(request.RequestID); err != nil {
		return CommandReceipt{}, err
	}
	receipt, err := s.runtime.TXConfigureNotificationRoute(ctx, s.runtime.LocalScope(companyID), kernel.NotificationRouteInput{Adapter: request.Adapter, Destination: request.Destination, SafetyAlias: request.SafetyAlias, CredentialRef: request.CredentialRef, Enabled: request.Enabled}, request.RequestID)
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

func (s *Service) Close() error {
	if s == nil {
		return nil
	}
	s.qqDispatchMu.Lock()
	cancelRetry, retryDone := s.qqRetryCancel, s.qqRetryDone
	s.qqRetryCancel, s.qqRetryDone, s.qqSender = nil, nil, nil
	s.qqDispatchMu.Unlock()
	if cancelRetry != nil {
		cancelRetry()
		<-retryDone
	}
	s.preparationMu.Lock()
	cancelPreparation := s.preparationCancel
	s.preparationCancel = nil
	s.preparationMu.Unlock()
	if cancelPreparation != nil {
		cancelPreparation()
	}
	var closeErrors []error
	if err := s.closeProjectJobs(); err != nil {
		closeErrors = append(closeErrors, err)
	}
	observer := s.mcpRuntimeObserver
	phasedWorker, supportsPhasedClose := s.worker.(mcpObserverShutdownWorker)
	if supportsPhasedClose {
		workerErr := phasedWorker.QuiesceForMCPObserver()
		var observerErr error
		if observer != nil {
			observerErr = observer.Close()
			if observerErr == nil {
				s.mcpRuntimeObserver = nil
			}
		}
		if workerErr != nil {
			closeErrors = append(closeErrors, workerErr)
		}
		if observerErr != nil {
			closeErrors = append(closeErrors, observerErr)
		}
		if workerErr == nil && observerErr == nil {
			if err := phasedWorker.CloseAfterMCPObserver(); err != nil {
				closeErrors = append(closeErrors, err)
			}
		}
	} else {
		if s.worker != nil {
			s.worker.Close()
		}
		if observer != nil {
			if err := observer.Close(); err != nil {
				closeErrors = append(closeErrors, err)
			} else {
				s.mcpRuntimeObserver = nil
			}
		}
	}
	s.preparationMu.RLock()
	preparationRunner := s.preparationRunner
	s.preparationMu.RUnlock()
	if invalidator, ok := preparationRunner.(interface {
		PreparedRunsForShutdown() []preparedEnvironmentRunRef
	}); ok {
		for _, prepared := range invalidator.PreparedRunsForShutdown() {
			persistCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			if err := s.recordEnvironmentPreparationEvent(persistCtx, prepared.companyID, prepared.runID, environment.PreparationOutcomeUnknown, "prepared_environment_profile_closed", "", "", 0, false); err != nil {
				closeErrors = append(closeErrors, err)
			}
			cancel()
		}
	}
	if closer, ok := preparationRunner.(interface{ Close() error }); ok {
		if err := closer.Close(); err != nil {
			closeErrors = append(closeErrors, err)
		}
	}
	return errors.Join(closeErrors...)
}

func commandReceipt(commandType, requestID, targetID, resultingState string) CommandReceipt {
	return CommandReceipt{CommandID: requestID, CommandType: commandType, TargetType: "mission", TargetID: targetID, RequestID: requestID, Accepted: true, AcceptedAt: time.Now().UTC().Format(time.RFC3339Nano), ResultingState: resultingState}
}

func commandReceiptForTarget(commandType, requestID, targetType, targetID, resultingState string) CommandReceipt {
	return CommandReceipt{CommandID: requestID, CommandType: commandType, TargetType: targetType, TargetID: targetID, RequestID: requestID, Accepted: true, AcceptedAt: time.Now().UTC().Format(time.RFC3339Nano), ResultingState: resultingState}
}

func (s *Service) UploadMissionInput(ctx context.Context, companyID string, request UploadMissionInputRequest, filename, mediaType string, content []byte) (MissionInputCommandReceipt, error) {
	if !core.ValidID(companyID) {
		return MissionInputCommandReceipt{}, core.Malformed
	}
	if err := validateUploadMissionInputRequest(request); err != nil {
		return MissionInputCommandReceipt{}, err
	}
	prepared, storedContent, err := intake.PrepareMissionInput(filename, mediaType, content)
	if err != nil {
		return MissionInputCommandReceipt{}, fmt.Errorf("%w: %v", core.Malformed, err)
	}
	revision, err := s.runtime.TXAddMissionInput(ctx, s.runtime.LocalScope(companyID), request.MissionID, request.InputID, request.RequestID, prepared, storedContent)
	if err != nil {
		return MissionInputCommandReceipt{}, err
	}
	return MissionInputCommandReceipt{
		CompanyID: companyID, InputID: revision.InputID, MissionID: revision.MissionID, Revision: revision.Revision,
		RequestID: revision.RequestID, SourceKind: revision.SourceKind, DisplayName: revision.DisplayName,
		MediaType: revision.MediaType, ByteSize: revision.ByteSize, ContentDigest: revision.ContentDigest, State: revision.State,
	}, nil
}

func companySummary(company kernel.CompanyDetails) CompanySummary {
	roster := make([]organization.EmployeeDraft, len(company.Roster))
	copy(roster, company.Roster)
	return CompanySummary{ID: company.ID, Name: company.Name, WorkspaceRoot: company.WorkspaceRoot, State: company.State, Roster: roster}
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
var _ StdioMCPRuntimeObservationService = (*Service)(nil)
var _ StreamableHTTPMCPRuntimeObservationService = (*Service)(nil)

var _ MissionInputService = (*Service)(nil)

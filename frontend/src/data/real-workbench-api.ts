// pattern: Imperative Shell

import {invalidateProtectedScope, observeProtectedResponse} from '../lib/authorization-state';
import type {MemoryCorrectionCommandReceiptView} from '../domain/workbench';
import type {MissionCloseoutOptions} from './workbench-api';
import type {ProposeMemoryCorrectionOptions, ReviewMemoryCorrectionOptions} from './workbench-api';
import type {MissionInputCommandReceipt, MissionInputView, TaskInputManifestView} from '../domain/mission-input';
import type {CompanyToolCallBudgetChangeReceiptView, CompanyToolCallBudgetView, CompanyToolCallClosingReserveReceiptView} from '../domain/workbench';
import type {ChangeCompanyToolCallBudgetOptions, SetCompanyToolCallClosingReserveOptions} from './workbench-api';
import type {MissionToolCallBudgetChangeReceiptView, MissionToolCallBudgetListView, MissionToolCallClosingReserveReceiptView} from '../domain/workbench';
import type {ChangeMissionToolCallBudgetOptions, MissionToolCallBudgetQueryOptions, SetMissionToolCallClosingReserveOptions} from './workbench-api';
import type {DailyRoutineCommandReceipt, DailyRoutineView, MemoryCorrectionQueueView, MemoryTaskRevalidationPreviewView, MemoryTaskRevalidationReceipt, MemoryTaskStatusView, ProblemToolCallAllocationReceiptView, ProblemToolCallBudgetListView, ProblemToolCallClosingReserveReceiptView, TaskToolCallAllocationReceiptView, TaskToolCallIncompleteClosureReceiptView} from '../domain/workbench';
import {validateDailyRoutineCommandReceipt, validateDailyRoutines} from '../domain/daily-routine-validation';
import {validateMemoryTaskRevalidationPreview, validateMemoryTaskRevalidationReceipt, validateMemoryTaskStatus} from '../domain/memory-revalidation-validation';
import {validateMemoryCorrectionCommandReceipt, validateMemoryCorrectionQueue} from '../domain/memory-correction-validation';
import {validateCompanyToolCallBudget, validateCompanyToolCallBudgetChangeReceipt, validateCompanyToolCallClosingReserveReceipt, validateMissionToolCallBudgetChangeReceipt, validateMissionToolCallBudgetList, validateMissionToolCallClosingReserveReceipt, validateProblemToolCallAllocationReceipt, validateProblemToolCallBudgetList, validateProblemToolCallClosingReserveReceipt, validateTaskToolCallAllocationReceipt, validateTaskToolCallIncompleteClosureReceipt} from '../domain/problem-budget-validation';
import type {AllocateProblemToolCallsOptions, AllocateTaskToolCallsOptions, CloseTaskToolBudgetIncompleteOptions, CreateDailyRoutineOptions, DailyRoutineQueryOptions, MemoryCorrectionQueueQueryOptions, MemoryTaskRevalidationPreviewOptions, MemoryTaskStatusQueryOptions, ProblemToolCallBudgetQueryOptions, RevalidateMemoryTaskOptions, SetDailyRoutineTaskInstructionOptions, SetProblemToolCallClosingReserveOptions} from './workbench-api';
import type {StdioMCPPackageRevisionView} from '../domain/workbench';
import {validateStdioMCPPackageRevision} from '../domain/workbench-validation';
import type {ServiceBrowserSessionView} from '../domain/workbench';
import {validateServiceBrowserSession} from '../domain/workbench-validation';
import type {CreateProjectJobBrowserSessionOptions} from './workbench-api';
import type {ResearchSimulationRunView, ResearchSourceView} from '../domain/workbench';
import {validateResearchSimulationRun} from '../domain/workbench-validation';
import type {RunResearchSimulationOptions} from './workbench-api';
import type {DomainContentCorrectionView, DomainContentDraftView, DomainContentFeedbackView, DomainContentPublicationView, DomainContentReviewView, DomainContentSourceEventView} from '../domain/workbench';
import {validateDomainContentCorrection, validateDomainContentDraft, validateDomainContentFeedback, validateDomainContentPublication, validateDomainContentReview, validateDomainContentSourceEvent, validateResearchSource, validateResearchSources} from '../domain/workbench-validation';
import type {RecordContentReviewOptions, RegisterContentDraftOptions, ResearchSourcesQueryOptions, SetContentSourceAuthorizationOptions, SetResearchSourceAuthorizationOptions} from './workbench-api';
import type {RecordContentCorrectionOptions, RecordContentFeedbackOptions, SimulateContentPublicationOptions} from './workbench-api';
import type {GitHubFeedbackCollectionPolicyReceipt} from '../domain/workbench';
import {validateGitHubFeedbackCollectionPolicyReceipt} from '../domain/workbench-validation';
import type {SetGitHubFeedbackCollectionPolicyOptions} from './workbench-api';
import {type ImportStdioMCPPackageOptions, type ObserveStdioMCPRuntimeOptions, type ObserveStreamableHTTPMCPRuntimeOptions, MAX_STDIO_MCP_PACKAGE_BYTES} from './workbench-api';
import type {ActivityView, ArtifactDeliveryManifestResponse, ArtifactDetailView, CapabilityCatalogView, CodexModelCatalogView, CollaborationItem, CompanyCommandReceipt, CompanyFeedbackView, CompanyOverviewView, CompanySummaryView, CrossBackendHandoverView, DomainEvidenceArtifactPreviewManifestView, DomainEvidenceArtifactPreviewView, DomainEvidenceLedgerView, DomainEvidenceRecordView, DomainEvidenceReviewRecordView, DomainEvidenceSubstantiveAssessmentRecordView, DomainProfileQualificationRecordView, DurableDeliveryManifestCompletionReceipt, DurableDeliveryManifestInvalidationReceipt, DurableDeliveryResponse, DurableUserDispositionCommandReceipt, EnvironmentExecutorQualificationReceipt, EnvironmentPolicyDecisionReceipt, EnvironmentPreparationRunView, GitHubCredentialReceipt, GitHubFeedbackBacklogStatusReceipt, GitHubFeedbackPollReceipt, GitHubFeedbackProbeReceipt, GitHubFeedbackSourceCommandReceipt, HumanInterventionCommandReceipt, JobRunCommandReceipt, JobRunLogArtifactView, JobRunView, MissionChangeRequestView, MissionCommandReceipt, NotificationsView, OperatorInstructionReceipt, OperatorInstructionView, OperationsView, ProjectEnvironmentRevisionView, RuntimeSettingsView, TaskTakeoverLeaseView, WorkspaceView} from '../domain/workbench';
import type {TaskTakeoverWorkspaceFileView, TaskTakeoverWorkspaceManifestView} from '../domain/workbench';
import {validateActivityEvent, validateActivityView, validateArtifactDeliveryManifest, validateArtifactDetail, validateCapabilityCatalog, validateCodexModelCatalog, validateCollaboration, validateCompanyFeedback, validateCompanyList, validateCompanyOverview, validateDomainEvidenceArtifactPreviewManifest, validateDomainEvidenceLedger, validateDomainEvidenceRecord, validateDomainEvidenceReviewRecord, validateDomainEvidenceSubstantiveAssessmentRecord, validateDomainProfileQualificationRecord, validateDurableDelivery, validateDurableDeliveryManifestCompletionReceipt, validateDurableDeliveryManifestInvalidationReceipt, validateDurableUserDispositionReceipt, validateEnvironmentExecutorQualificationReceipt, validateEnvironmentPolicyDecisionReceipt, validateEnvironmentPreparationRun, validateGitHubCredentialReceipt, validateGitHubFeedbackBacklogStatusReceipt, validateGitHubFeedbackPollReceipt, validateGitHubFeedbackProbeReceipt, validateGitHubFeedbackSourceReceipt, validateHumanInterventionCommandReceipt, validateJobRunCommandReceipt, validateJobRunLogArtifact, validateMissionChangeRequest, validateMissionChangeRequests, validateMissionCommandReceipt, validateMissionInputCommandReceipt, validateMissionInputs, validateNotifications, validateOperatorInstructionReceipt, validateOperatorInstructions, validateOperations, validateProjectEnvironmentRevisions, validateRuntimeSettings, validateTaskCrossBackendHandovers, validateTaskInputManifest, validateTaskJobRuns, validateTaskTakeoverLease, validateTaskTakeoverLeases, validateWorkspace, validationMessage} from '../domain/workbench-validation';
import {validateTaskTakeoverWorkspaceFile, validateTaskTakeoverWorkspaceManifest} from '../domain/workbench-validation';
import type {TaskTakeoverWorkspaceFileQueryOptions, TaskTakeoverWorkspaceManifestQueryOptions} from './workbench-api';
import type {TaskTakeoverDirectorySnapshotOptions} from './workbench-api';
import {assertCompanyScope, CommandApiError, isValidActivityLimit, isValidOpaqueCursor, type ActivityEventListener, type ActivityQueryOptions, type ActivityStreamOptions, type ActivityStreamStatusListener, type ArchiveCompanyOptions, type BindEmployeeCapabilityOptions, type CompanyDraftOptions, type CompanyScopeOptions, type CompleteDurableDeliveryManifestOptions, type ConfigureNotificationRouteOptions, type CreateMissionOptions, type CreateMissionChangeRequestOptions, type CreateOperatorInstructionOptions, type CreateTaskEnvironmentHandoverOptions, type CreateTaskTakeoverLeaseOptions, type DecideCapabilityOptions, type DecideGitHubFeedbackSourceOptions, type DeleteGitHubCredentialOptions, type EnvironmentExecutorQualificationOptions, type EnvironmentPolicyDecisionOptions, type EnsureEnvironmentOptions, type GetDomainEvidenceArtifactPreviewOptions, type ImportSkillOptions, type ImportReadOnlySkillPackageOptions, type InvalidateDurableDeliveryManifestOptions, type ListDomainEvidenceArtifactPreviewEntriesOptions, type MissionChangeRequestCommandOptions, type MissionChangeRequestQueryOptions, type MissionCommandOptions, type MissionInputQueryOptions, type PollGitHubFeedbackSourceOptions, type ProbeGitHubFeedbackSourceOptions, type QualifyCapabilityOptions, type RecordDomainEvidenceOptions, type RecordDomainEvidenceReviewOptions, type RecordDomainEvidenceSubstantiveAssessmentOptions, type RecordDomainProfileQualificationOptions, type RegisterGitHubFeedbackSourceOptions, type ReleaseTaskTakeoverLeaseOptions, type ReviewCapabilityRevocationOptions, type SetGitHubFeedbackBacklogStatusOptions, type SetHumanInterventionStateOptions, type StoreGitHubCredentialOptions, type TaskCrossBackendHandoversQueryOptions, type TaskInputManifestQueryOptions, type TaskJobLogsQueryOptions, type TaskJobRunsQueryOptions, type StartTaskJobRunOptions, type StopTaskJobRunOptions, type TaskTakeoverLeaseQueryOptions, type TaskTakeoverSnapshotOptions, type UploadMissionDirectoryInputOptions, type UploadMissionInputOptions, type OperatorInstructionQueryOptions, type RegisterMCPOptions, type TestNotificationOptions, type UpdateCompanyOptions, type UpdateRuntimeSettingsOptions, type RecordDurableUserDispositionOptions, type WorkbenchApi, MAX_MISSION_DIRECTORY_BYTES, MAX_MISSION_DIRECTORY_FILES, MAX_MISSION_INPUT_BYTES, MAX_SKILL_PACKAGE_BYTES} from './workbench-api';

function isSafeWorkspaceRelativePath(value: string): boolean {
  return value.length > 0 && value.length <= 1024 && !value.startsWith('/') && !value.endsWith('/') && !value.includes('\\') && !value.includes('%') && !value.includes(':')
    && value.split('/').every(part => part.length > 0 && part.length <= 255 && part !== '.' && part !== '..' && part.trim() === part && !/[\u0000-\u001f\u007f]/.test(part));
}

function normalizeDurableDeliveryResponse(value: unknown): unknown {
  if (typeof value !== 'object' || value === null || Array.isArray(value)) return value;
  const record = value as Record<string, unknown>;
  return Object.hasOwn(record, 'feedbackBacklog') ? value : {...record, feedbackBacklog: []};
}

function isCanonicalPositiveInt64(value: string): boolean {
  const maxInt64 = '9223372036854775807';
  return /^[1-9]\d{0,18}$/.test(value) && (value.length < maxInt64.length || value <= maxInt64);
}

export class RealWorkbenchApi implements WorkbenchApi {
  readonly mode = 'real' as const;
  private readonly stream: EventStreamClient;

  constructor(private readonly baseUrl: string = '/api/workbench', enableLiveUpdates = false, private readonly sessionToken: string | null = null) {
    this.stream = new EventStreamClient(baseUrl, enableLiveUpdates, sessionToken);
  }

  async listCompanies(): Promise<ReadonlyArray<CompanySummaryView>> {
    const raw = await this.get('/companies');
    const result = validateCompanyList(raw);
    if (!result.success) {
      throw new Error(`failed to load companies: ${validationMessage(result.issues)}`);
    }
    return result.value;
  }

  async getRuntimeSettings(options: CompanyScopeOptions): Promise<RuntimeSettingsView> {
    assertCompanyScope(options.companyId);
    const raw = await this.get(`/companies/${encodeURIComponent(options.companyId)}/settings`);
    const result = validateRuntimeSettings(raw);
    if (!result.success) {
      throw new Error(`failed to load runtime settings: ${validationMessage(result.issues)}`);
    }
    if (result.value.companyId !== options.companyId) {
      throw new Error('failed to load runtime settings: response company scope does not match request');
    }
    return result.value;
  }

  async getCodexModelCatalog(options: CompanyScopeOptions): Promise<CodexModelCatalogView> {
    assertCompanyScope(options.companyId);
    const raw = await this.get(`/companies/${encodeURIComponent(options.companyId)}/settings/models`);
    const result = validateCodexModelCatalog(raw);
    if (!result.success) {
      throw new Error(`failed to load Codex model catalog: ${validationMessage(result.issues)}`);
    }
    return result.value;
  }

  async createCompany(options: CompanyDraftOptions): Promise<CompanyCommandReceipt> {
    assertCompanyScope(options.id);
    assertRequestID(options.requestId);
    const raw = await this.post('/companies', options.requestId, {id: options.id, name: options.name, workspaceRoot: options.workspaceRoot, roster: options.roster, requestId: options.requestId, teamCoverageConfirmationSha256: options.teamCoverageConfirmationSha256}, options.teamCoverageConfirmationSha256 !== undefined);
    return validateCompanyCommandReceipt(raw, 'company.create');
  }

  async updateCompany(options: UpdateCompanyOptions): Promise<CompanyCommandReceipt> {
    assertCompanyScope(options.companyId);
    assertRequestID(options.requestId);
    const raw = await this.post(`/companies/${encodeURIComponent(options.companyId)}/organization`, options.requestId, {name: options.name, workspaceRoot: options.workspaceRoot, roster: options.roster, requestId: options.requestId, teamCoverageConfirmationSha256: options.teamCoverageConfirmationSha256}, options.teamCoverageConfirmationSha256 !== undefined);
    return validateCompanyCommandReceipt(raw, options.teamCoverageConfirmationSha256 === undefined ? 'company.update' : 'company.team_coverage.confirm');
  }

  async archiveCompany(options: ArchiveCompanyOptions): Promise<CompanyCommandReceipt> {
    assertCompanyScope(options.companyId);
    assertRequestID(options.requestId);
    const raw = await this.post(`/companies/${encodeURIComponent(options.companyId)}/archive`, options.requestId, {requestId: options.requestId});
    return validateCompanyCommandReceipt(raw, 'company.archive');
  }

  async updateRuntimeSettings(options: UpdateRuntimeSettingsOptions): Promise<CompanyCommandReceipt> {
    assertCompanyScope(options.companyId);
    assertRequestID(options.requestId);
    const raw = await this.post(`/companies/${encodeURIComponent(options.companyId)}/settings`, options.requestId, {provider: options.provider, model: options.model, effort: options.effort, profile: options.profile, requestId: options.requestId});
    return validateCompanyCommandReceipt(raw, 'runtime.settings.update');
  }

  async listOperatorInstructions(options: OperatorInstructionQueryOptions): Promise<ReadonlyArray<OperatorInstructionView>> {
    assertCompanyScope(options.companyId);
    const raw = await this.get(`/companies/${encodeURIComponent(options.companyId)}/instructions`);
    const result = validateOperatorInstructions(raw);
    if (!result.success) {
      throw new Error(`failed to load operator instructions: ${validationMessage(result.issues)}`);
    }
    return result.value;
  }

  async sendOperatorInstruction(options: CreateOperatorInstructionOptions): Promise<OperatorInstructionReceipt> {
    assertCompanyScope(options.companyId);
    if (options.missionId !== null) assertCompanyScope(options.missionId);
    assertRequestID(options.requestId);
    const raw = await this.post(`/companies/${encodeURIComponent(options.companyId)}/instructions`, options.requestId, {missionId: options.missionId, taskId: options.taskId, employeeId: options.employeeId, content: options.content, requestId: options.requestId});
    const result = validateOperatorInstructionReceipt(raw);
    if (!result.success) {
      throw new Error(`failed to parse operator instruction receipt: ${validationMessage(result.issues)}`);
    }
    return result.value;
  }

  async listMissionChangeRequests(options: MissionChangeRequestQueryOptions): Promise<ReadonlyArray<MissionChangeRequestView>> {
    assertCompanyScope(options.companyId);
    assertCompanyScope(options.missionId);
    const raw = await this.get(`/companies/${encodeURIComponent(options.companyId)}/missions/${encodeURIComponent(options.missionId)}/change-requests`);
    const result = validateMissionChangeRequests(raw, options.missionId);
    if (!result.success) {
      throw new Error(`failed to load formal change requests: ${validationMessage(result.issues)}`);
    }
    return result.value;
  }

  async createMissionChangeRequest(options: CreateMissionChangeRequestOptions): Promise<MissionChangeRequestView> {
    assertCompanyScope(options.companyId);
    assertCompanyScope(options.missionId);
    assertRequestID(options.requestId);
    const raw = await this.post(`/companies/${encodeURIComponent(options.companyId)}/missions/${encodeURIComponent(options.missionId)}/change-requests`, options.requestId, {
      requestId: options.requestId,
      changeSummary: options.changeSummary,
      proposedTitle: options.proposedTitle,
      proposedGoal: options.proposedGoal,
      proposedAcceptanceContract: options.proposedAcceptanceContract,
      blockPreviousResults: options.blockPreviousResults,
    });
    const result = validateMissionChangeRequest(raw, options.missionId);
    if (!result.success || result.value.clientRequestId !== options.requestId) {
      throw new Error(`failed to parse formal change request: ${result.success ? 'request ID differs from the submitted command' : validationMessage(result.issues)}`);
    }
    return result.value;
  }

  async considerMissionChangeRequest(options: MissionChangeRequestCommandOptions): Promise<MissionChangeRequestView> {
    return this.missionChangeRequestAction(options, 'consider');
  }

  async declineMissionChangeRequest(options: MissionChangeRequestCommandOptions): Promise<MissionChangeRequestView> {
    return this.missionChangeRequestAction(options, 'decline');
  }

  async applyMissionChangeRequest(options: MissionChangeRequestCommandOptions): Promise<MissionChangeRequestView> {
    return this.missionChangeRequestAction(options, 'apply');
  }

  private async missionChangeRequestAction(options: MissionChangeRequestCommandOptions, action: 'consider' | 'decline' | 'apply'): Promise<MissionChangeRequestView> {
    assertCompanyScope(options.companyId);
    assertCompanyScope(options.missionId);
    assertCompanyScope(options.changeRequestId);
    assertRequestID(options.requestId);
    if (action === 'apply' && (options.impactSha256 === undefined || !/^[0-9a-f]{64}$/.test(options.impactSha256))) {
      throw new CommandApiError('MALFORMED_INPUT', 400, 'failed to apply formal change request: the reviewed impact digest is required', options.changeRequestId);
    }
    if (action === 'consider' && (options.assessmentSha256 === undefined || !/^[0-9a-f]{64}$/.test(options.assessmentSha256))) {
      throw new CommandApiError('MALFORMED_INPUT', 400, 'failed to consider formal change request: the current Planning assessment digest is required', options.changeRequestId);
    }
    const body = action === 'apply' ? {requestId: options.requestId, impactSha256: options.impactSha256}
      : action === 'consider' ? {requestId: options.requestId, assessmentSha256: options.assessmentSha256}
        : {requestId: options.requestId};
    const raw = await this.post(`/companies/${encodeURIComponent(options.companyId)}/missions/${encodeURIComponent(options.missionId)}/change-requests/${encodeURIComponent(options.changeRequestId)}/${action}`, options.requestId, body);
    const result = validateMissionChangeRequest(raw, options.missionId);
    if (!result.success) {
      throw new Error(`failed to parse formal change request: ${validationMessage(result.issues)}`);
    }
    return result.value;
  }

  async listTaskTakeoverLeases(options: TaskTakeoverLeaseQueryOptions): Promise<ReadonlyArray<TaskTakeoverLeaseView>> {
    assertCompanyScope(options.companyId);
    assertCompanyScope(options.missionId);
    const raw = await this.get(`/companies/${encodeURIComponent(options.companyId)}/missions/${encodeURIComponent(options.missionId)}/takeover-leases`);
    const result = validateTaskTakeoverLeases(raw, options.missionId);
    if (!result.success) throw new Error(`failed to load human takeover leases: ${validationMessage(result.issues)}`);
    return result.value;
  }

  async getTaskTakeoverWorkspaceManifest(options: TaskTakeoverWorkspaceManifestQueryOptions): Promise<TaskTakeoverWorkspaceManifestView> {
    assertCompanyScope(options.companyId);
    assertCompanyScope(options.missionId);
    assertCompanyScope(options.leaseId);
    const raw = await this.get(`/companies/${encodeURIComponent(options.companyId)}/missions/${encodeURIComponent(options.missionId)}/takeover-leases/${encodeURIComponent(options.leaseId)}/workspace`);
    const result = validateTaskTakeoverWorkspaceManifest(raw, options.missionId, options.leaseId);
    if (!result.success) throw new Error(`failed to load frozen takeover workspace manifest: ${validationMessage(result.issues)}`);
    return result.value;
  }

  async readTaskTakeoverWorkspaceFile(options: TaskTakeoverWorkspaceFileQueryOptions): Promise<TaskTakeoverWorkspaceFileView> {
    assertCompanyScope(options.companyId);
    assertCompanyScope(options.missionId);
    assertCompanyScope(options.leaseId);
    const query = new URLSearchParams({path: options.relativePath});
    const raw = await this.get(`/companies/${encodeURIComponent(options.companyId)}/missions/${encodeURIComponent(options.missionId)}/takeover-leases/${encodeURIComponent(options.leaseId)}/workspace/file?${query.toString()}`);
    const result = validateTaskTakeoverWorkspaceFile(raw, options.missionId, options.leaseId, options.relativePath, options.manifestSha256);
    if (!result.success) throw new Error(`failed to load frozen takeover workspace file: ${validationMessage(result.issues)}`);
    return result.value;
  }

  async createTaskTakeoverLease(options: CreateTaskTakeoverLeaseOptions): Promise<TaskTakeoverLeaseView> {
    assertCompanyScope(options.companyId);
    assertCompanyScope(options.missionId);
    assertCompanyScope(options.taskId);
    assertRequestID(options.requestId);
    const raw = await this.post(`/companies/${encodeURIComponent(options.companyId)}/missions/${encodeURIComponent(options.missionId)}/tasks/${encodeURIComponent(options.taskId)}/takeover-lease`, options.requestId, {requestId: options.requestId});
    const result = validateTaskTakeoverLease(raw, options.missionId);
    if (!result.success || result.value.clientRequestId !== options.requestId || result.value.taskId !== options.taskId || result.value.state !== 'granted') {
      throw new Error(`failed to parse human takeover lease: ${result.success ? 'scope, request ID, or state differs from the command' : validationMessage(result.issues)}`);
    }
    return result.value;
  }

  async submitTaskTakeoverSnapshot(options: TaskTakeoverSnapshotOptions): Promise<TaskTakeoverLeaseView> {
    assertCompanyScope(options.companyId);
    assertCompanyScope(options.missionId);
    assertCompanyScope(options.leaseId);
    assertRequestID(options.requestId);
    if (!/^[0-9a-f]{64}$/.test(options.baseWorkspaceDigest) || !Number.isSafeInteger(options.baseWorkspaceRevision) || options.baseWorkspaceRevision < 1
      || !Number.isSafeInteger(options.humanEffortSeconds) || options.humanEffortSeconds < 0 || options.humanEffortSeconds > 86_400
      || new TextEncoder().encode(options.content).length === 0 || new TextEncoder().encode(options.content).length > 4096) {
      throw new CommandApiError('MALFORMED_INPUT', 400, 'failed to return human workspace snapshot: the frozen base, content, or effort value is invalid', options.leaseId);
    }
    const raw = await this.post(`/companies/${encodeURIComponent(options.companyId)}/missions/${encodeURIComponent(options.missionId)}/takeover-leases/${encodeURIComponent(options.leaseId)}/snapshot`, options.requestId, {
      requestId: options.requestId,
      baseWorkspaceDigest: options.baseWorkspaceDigest,
      baseWorkspaceRevision: options.baseWorkspaceRevision,
      content: options.content,
      humanEffortSeconds: options.humanEffortSeconds,
    });
    const result = validateTaskTakeoverLease(raw, options.missionId);
    if (!result.success || result.value.leaseId !== options.leaseId || result.value.state !== 'returned'
      || result.value.baseWorkspaceDigest !== options.baseWorkspaceDigest || result.value.baseWorkspaceRevision !== options.baseWorkspaceRevision) {
      throw new Error(`failed to parse returned human snapshot: ${result.success ? 'lease or frozen base differs from the command' : validationMessage(result.issues)}`);
    }
    return result.value;
  }

  async submitTaskTakeoverDirectorySnapshot(options: TaskTakeoverDirectorySnapshotOptions): Promise<TaskTakeoverLeaseView> {
    assertCompanyScope(options.companyId);
    assertCompanyScope(options.missionId);
    assertCompanyScope(options.leaseId);
    assertRequestID(options.requestId);
    let totalBytes = 0;
    if (!/^[0-9a-f]{64}$/.test(options.baseWorkspaceTreeSha256) || !Number.isSafeInteger(options.humanEffortSeconds)
      || options.humanEffortSeconds < 0 || options.humanEffortSeconds > 86_400 || options.files.length < 1 || options.files.length > MAX_MISSION_DIRECTORY_FILES) {
      throw new CommandApiError('MALFORMED_INPUT', 400, 'failed to return workspace tree: the frozen manifest, file count, or effort value is invalid', options.leaseId);
    }
    const form = new FormData();
    form.append('requestId', options.requestId);
    form.append('baseWorkspaceTreeSha256', options.baseWorkspaceTreeSha256);
    form.append('humanEffortSeconds', String(options.humanEffortSeconds));
    for (const file of options.files) {
      if (!isSafeWorkspaceRelativePath(file.relativePath)) {
        throw new CommandApiError('MALFORMED_INPUT', 400, 'failed to return workspace tree: a file path is invalid', options.leaseId);
      }
      const bytes = new TextEncoder().encode(file.content).length;
      if (bytes < 1 || bytes > 2 * 1024 * 1024 || totalBytes + bytes > MAX_MISSION_DIRECTORY_BYTES) {
        throw new CommandApiError('MALFORMED_INPUT', 400, 'failed to return workspace tree: the directory exceeds its file or byte bounds', options.leaseId);
      }
      totalBytes += bytes;
      form.append('paths', file.relativePath);
      const fileName = file.relativePath.split('/').at(-1) || 'workspace.txt';
      form.append('files', new Blob([file.content], {type: 'text/plain; charset=utf-8'}), fileName);
    }
    const raw = await this.postMultipart(`/companies/${encodeURIComponent(options.companyId)}/missions/${encodeURIComponent(options.missionId)}/takeover-leases/${encodeURIComponent(options.leaseId)}/directory-snapshot`, options.requestId, form, 'failed to return human workspace tree');
    const result = validateTaskTakeoverLease(raw, options.missionId);
    if (!result.success || result.value.leaseId !== options.leaseId || result.value.state !== 'returned'
      || result.value.workspaceTree?.manifestSha256 !== options.baseWorkspaceTreeSha256) {
      throw new Error(`failed to parse returned human workspace tree: ${result.success ? 'lease or frozen manifest differs from the command' : validationMessage(result.issues)}`);
    }
    return result.value;
  }

  async releaseTaskTakeoverLease(options: ReleaseTaskTakeoverLeaseOptions): Promise<TaskTakeoverLeaseView> {
    assertCompanyScope(options.companyId);
    assertCompanyScope(options.missionId);
    assertCompanyScope(options.leaseId);
    assertRequestID(options.requestId);
    const raw = await this.post(`/companies/${encodeURIComponent(options.companyId)}/missions/${encodeURIComponent(options.missionId)}/takeover-leases/${encodeURIComponent(options.leaseId)}/release`, options.requestId, {requestId: options.requestId});
    const result = validateTaskTakeoverLease(raw, options.missionId);
    if (!result.success || result.value.leaseId !== options.leaseId || result.value.state !== 'released') {
      throw new Error(`failed to parse released human takeover lease: ${result.success ? 'lease or state differs from the command' : validationMessage(result.issues)}`);
    }
    return result.value;
  }

  async listCollaboration(options: CompanyScopeOptions): Promise<ReadonlyArray<CollaborationItem>> {
    assertCompanyScope(options.companyId);
    const raw = await this.get(`/companies/${encodeURIComponent(options.companyId)}/collaboration`);
    const result = validateCollaboration(raw);
    if (!result.success) {
      throw new Error(`failed to load collaboration: ${validationMessage(result.issues)}`);
    }
    return result.value;
  }

  async getWorkspace(options: Readonly<{companyId: string; taskId: string}>): Promise<WorkspaceView> {
    assertCompanyScope(options.companyId);
    assertCompanyScope(options.taskId);
    const raw = await this.get(`/companies/${encodeURIComponent(options.companyId)}/tasks/${encodeURIComponent(options.taskId)}/workspace`);
    const result = validateWorkspace(raw);
    if (!result.success) {
      throw new Error(`failed to load workspace: ${validationMessage(result.issues)}`);
    }
    return result.value;
  }

  async getArtifact(options: Readonly<{companyId: string; artifactId: string}>): Promise<ArtifactDetailView> {
    assertCompanyScope(options.companyId);
    assertCompanyScope(options.artifactId);
    const raw = await this.get(`/companies/${encodeURIComponent(options.companyId)}/artifacts/${encodeURIComponent(options.artifactId)}/detail`);
    const result = validateArtifactDetail(raw);
    if (!result.success) {
      throw new Error(`failed to load artifact: ${validationMessage(result.issues)}`);
    }
    return result.value;
  }

  async getOperations(options: CompanyScopeOptions): Promise<OperationsView> {
    assertCompanyScope(options.companyId);
    const raw = await this.get(`/companies/${encodeURIComponent(options.companyId)}/operations`);
    const result = validateOperations(raw, options.companyId);
    if (!result.success) {
      throw new Error(`failed to load operations: ${validationMessage(result.issues)}`);
    }
    return result.value;
  }

  async listNotifications(options: CompanyScopeOptions): Promise<NotificationsView> {
    assertCompanyScope(options.companyId);
    const raw = await this.get(`/companies/${encodeURIComponent(options.companyId)}/notifications`);
    const result = validateNotifications(raw);
    if (!result.success) {
      throw new Error(`failed to load notifications: ${validationMessage(result.issues)}`);
    }
    return result.value;
  }

  async listCapabilityCatalog(options: CompanyScopeOptions): Promise<CapabilityCatalogView> {
    assertCompanyScope(options.companyId);
    const raw = await this.get(`/companies/${encodeURIComponent(options.companyId)}/capabilities`);
    const result = validateCapabilityCatalog(raw, options.companyId);
    if (!result.success) throw new Error(`failed to load capability catalog: ${validationMessage(result.issues)}`);
    return result.value;
  }

  async importSkill(options: ImportSkillOptions): Promise<unknown> {
    assertCompanyScope(options.companyId);
    assertRequestID(options.requestId);
    return this.post(`/companies/${encodeURIComponent(options.companyId)}/capabilities/skills`, options.requestId, options);
  }

  async importReadOnlySkillPackage(options: ImportReadOnlySkillPackageOptions): Promise<unknown> {
    assertCompanyScope(options.companyId);
    assertRequestID(options.requestId);
    if (options.bundleFile.size <= 0 || options.bundleFile.size > MAX_SKILL_PACKAGE_BYTES) {
      throw new Error('Skill ZIP must be between 1 byte and 8 MiB');
    }
    const body = new FormData();
    body.append('revision', options.revision);
    body.append('requestId', options.requestId);
    body.append('bundle', options.bundleFile, options.bundleFile.name);
    return this.postMultipart(`/companies/${encodeURIComponent(options.companyId)}/capabilities/skills`, options.requestId, body, 'failed to import read-only Skill package');
  }

  async importStdioMCPPackage(options: ImportStdioMCPPackageOptions): Promise<StdioMCPPackageRevisionView> {
    assertCompanyScope(options.companyId);
    assertRequestID(options.requestId);
    if (options.serverId !== null) assertRequestID(options.serverId);
    if (options.bundleFile.size <= 0 || options.bundleFile.size > MAX_STDIO_MCP_PACKAGE_BYTES) {
      throw new Error('stdio MCP ZIP must be between 1 byte and 8 MiB');
    }
    const body = new FormData();
    if (options.serverId !== null) body.append('serverId', options.serverId);
    body.append('revision', options.revision);
    body.append('requestId', options.requestId);
    body.append('bundle', options.bundleFile, options.bundleFile.name);
    const raw = await this.postMultipart(`/companies/${encodeURIComponent(options.companyId)}/capabilities/mcp-packages`, options.requestId, body, 'failed to import controlled stdio MCP package');
    const result = validateStdioMCPPackageRevision(raw, options.companyId);
    if (!result.success) throw new Error(`failed to import controlled stdio MCP package: ${validationMessage(result.issues)}`);
    return result.value;
  }

  async registerMCP(options: RegisterMCPOptions): Promise<unknown> {
    assertCompanyScope(options.companyId);
    assertRequestID(options.requestId);
    return this.post(`/companies/${encodeURIComponent(options.companyId)}/capabilities/mcp`, options.requestId, options);
  }

  async getCompanyFeedback(options: CompanyScopeOptions): Promise<CompanyFeedbackView> {
    assertCompanyScope(options.companyId);
    const raw = await this.get(`/companies/${encodeURIComponent(options.companyId)}/feedback`);
    const result = validateCompanyFeedback(raw, options.companyId);
    if (!result.success) {
      throw new Error(`failed to load feedback: ${validationMessage(result.issues)}`);
    }
    return result.value;
  }

  async listDomainEvidence(options: CompanyScopeOptions): Promise<DomainEvidenceLedgerView> {
    assertCompanyScope(options.companyId);
    const raw = await this.get(`/companies/${encodeURIComponent(options.companyId)}/domain-workflows`);
    const result = validateDomainEvidenceLedger(raw, options.companyId);
    if (!result.success) throw new Error(`failed to load domain evidence: ${validationMessage(result.issues)}`);
    return result.value;
  }

  async runResearchSimulation(options: RunResearchSimulationOptions): Promise<ResearchSimulationRunView> {
    assertCompanyScope(options.companyId);
    assertCompanyScope(options.datasetInputId);
    assertCompanyScope(options.methodInputId);
    assertRequestID(options.requestId);
    if (!/^[1-9]\d{0,18}$/.test(options.datasetInputRevision) || !/^[1-9]\d{0,18}$/.test(options.methodInputRevision)) {
      throw new Error('research input revisions must be positive integer strings');
    }
    if (!/^(0|[1-9]\d{0,19})$/.test(options.seed) || BigInt(options.seed) > 18446744073709551615n) {
      throw new Error('research simulation seed must be an unsigned 64-bit integer string');
    }
    if (!Number.isSafeInteger(options.riskBudgetUnits) || options.riskBudgetUnits <= 0 || options.riskBudgetUnits > 5_120_000) {
      throw new Error('research risk budget must be between 1 and 5,120,000 sample draws');
    }
    const controlDefinition = options.controlDefinition.trim();
    if (controlDefinition.length === 0 || controlDefinition.length > 512) throw new Error('research control definition must contain 1 to 512 characters');
    const raw = await this.post(`/companies/${encodeURIComponent(options.companyId)}/domain-workflows/research-simulations`, options.requestId, {
      datasetInputId: options.datasetInputId, datasetInputRevision: options.datasetInputRevision,
      methodInputId: options.methodInputId, methodInputRevision: options.methodInputRevision,
      seed: options.seed, controlDefinition, riskBudgetUnits: options.riskBudgetUnits, requestId: options.requestId,
    });
    const result = validateResearchSimulationRun(raw, {...options, controlDefinition, riskBudgetUnits: options.riskBudgetUnits});
    if (!result.success) throw new Error(`failed to run research simulation: ${validationMessage(result.issues)}`);
    return result.value;
  }

  async setContentSourceAuthorization(options: SetContentSourceAuthorizationOptions): Promise<DomainContentSourceEventView> {
    assertCompanyScope(options.companyId);
    assertCompanyScope(options.sourceInputId);
    assertRequestID(options.requestId);
    if (!/^[1-9]\d{0,18}$/.test(options.sourceInputRevision) || !/^[a-f0-9]{64}$/.test(options.sourceSha256)
      || (options.state !== 'authorized' && options.state !== 'revoked') || options.rationale.trim() === '' || Array.from(options.rationale.trim()).length > 1000) {
      throw new Error('content source authorization must bind a valid input revision, digest, state, and rationale');
    }
    const rationale = options.rationale.trim();
    const raw = await this.post(`/companies/${encodeURIComponent(options.companyId)}/domain-workflows/content-sources`, options.requestId, {
      sourceInputId: options.sourceInputId, sourceInputRevision: options.sourceInputRevision, sourceSha256: options.sourceSha256,
      state: options.state, rationale, requestId: options.requestId,
    });
    const result = validateDomainContentSourceEvent(raw, {
      companyId: options.companyId, inputId: options.sourceInputId, revision: options.sourceInputRevision,
      sha256: options.sourceSha256, state: options.state, requestId: options.requestId,
    });
    if (!result.success) throw new Error(`failed to authorize content source: ${validationMessage(result.issues)}`);
    return result.value;
  }

  async setResearchSourceAuthorization(options: SetResearchSourceAuthorizationOptions): Promise<ResearchSourceView> {
    assertCompanyScope(options.companyId);
    assertCompanyScope(options.sourceId);
    assertRequestID(options.requestId);
    if (options.state === 'authorized') {
      assertCompanyScope(options.missionId);
      if (options.profileRevision !== 'research-source@1' || !/^[a-f0-9]{64}$/.test(options.identitySha256) || !/^[a-f0-9]{64}$/.test(options.dataSha256)) {
        throw new Error('research source authorization must bind the fixed profile and identity/data digests');
      }
      let origin: URL;
      try {
        origin = new URL(options.origin);
      } catch {
        throw new Error('research source origin is invalid');
      }
      if (origin.protocol !== 'https:' || origin.username !== '' || origin.password !== '' || origin.pathname !== '/' || origin.search !== '' || origin.hash !== '') {
        throw new Error('research source origin must be an HTTPS origin without credentials or path');
      }
      if (options.searchEndpoint !== '') {
        let endpoint: URL;
        try {
          endpoint = new URL(options.searchEndpoint);
        } catch {
          throw new Error('research search endpoint is invalid');
        }
        if (endpoint.protocol !== 'https:' || endpoint.username !== '' || endpoint.password !== '' || endpoint.search !== '' || endpoint.hash !== '' || endpoint.origin !== origin.origin || options.searchRankingRevision !== 'research-ranking@1') {
          throw new Error('research search endpoint must stay on the registered origin and ranking revision');
        }
      } else if (options.searchCredentialRef !== '' || options.searchRankingRevision !== '') {
        throw new Error('research search policy requires an endpoint');
      }
    }
    const raw = await this.post(`/companies/${encodeURIComponent(options.companyId)}/domain-workflows/research-sources`, options.requestId, {
      sourceId: options.sourceId, missionId: options.missionId, origin: options.origin, searchEndpoint: options.searchEndpoint,
      searchCredentialRef: options.searchCredentialRef, searchRankingRevision: options.searchRankingRevision,
      profileRevision: options.profileRevision, identitySha256: options.identitySha256, dataSha256: options.dataSha256,
      state: options.state, rationale: options.rationale.trim(), requestId: options.requestId,
    });
    const result = validateResearchSource(raw, {companyId: options.companyId, sourceId: options.sourceId, missionId: options.missionId, requestId: options.requestId, state: options.state});
    if (!result.success) throw new Error(`failed to authorize research source: ${validationMessage(result.issues)}`);
    return result.value;
  }

  async listResearchSources(options: ResearchSourcesQueryOptions): Promise<ReadonlyArray<ResearchSourceView>> {
    assertCompanyScope(options.companyId);
    assertCompanyScope(options.missionId);
    const raw = await this.get(`/companies/${encodeURIComponent(options.companyId)}/domain-workflows/research-sources?missionId=${encodeURIComponent(options.missionId)}`);
    const result = validateResearchSources(raw, options.companyId);
    if (!result.success) throw new Error(`failed to load research sources: ${validationMessage(result.issues)}`);
    return result.value;
  }

  async registerContentDraft(options: RegisterContentDraftOptions): Promise<DomainContentDraftView> {
    assertCompanyScope(options.companyId);
    assertCompanyScope(options.draftInputId);
    assertCompanyScope(options.writerEmployeeId);
    assertRequestID(options.requestId);
    if (!/^[1-9]\d{0,18}$/.test(options.draftInputRevision) || options.criticalClaims.length === 0 || options.criticalClaims.length > 100
      || options.criticalClaims.some(claimId => !/^[A-Za-z0-9][A-Za-z0-9_-]{0,79}$/.test(claimId))
      || new Set(options.criticalClaims).size !== options.criticalClaims.length) {
      throw new Error('content draft must use a valid pinned input revision and 1 to 100 unique critical claim IDs');
    }
    const raw = await this.post(`/companies/${encodeURIComponent(options.companyId)}/domain-workflows/content-drafts`, options.requestId, {
      draftInputId: options.draftInputId, draftInputRevision: options.draftInputRevision, writerEmployeeId: options.writerEmployeeId,
      criticalClaims: options.criticalClaims, constraintsPassed: options.constraintsPassed, requestId: options.requestId,
    });
    const result = validateDomainContentDraft(raw, {
      companyId: options.companyId, inputId: options.draftInputId, revision: options.draftInputRevision,
      writerEmployeeId: options.writerEmployeeId, requestId: options.requestId,
    });
    if (!result.success) throw new Error(`failed to register versioned content draft: ${validationMessage(result.issues)}`);
    return result.value;
  }

  async recordContentReview(options: RecordContentReviewOptions): Promise<DomainContentReviewView> {
    assertCompanyScope(options.companyId);
    assertCompanyScope(options.draftInputId);
    assertRequestID(options.requestId);
    if (!/^[1-9]\d{0,18}$/.test(options.draftRevision) || options.review.draftRevision !== options.draftRevision
      || options.sample.draftRevision !== options.draftRevision || options.sample.sampledByEmployeeId !== options.review.checkerEmployeeId
      || !/^[a-f0-9]{64}$/.test(options.sample.draftSha256) || (options.correctionId !== '' && !/^[A-Za-z0-9][A-Za-z0-9_-]{0,79}$/.test(options.correctionId))) {
      throw new Error('content review and human sample must bind the same exact draft revision and checker');
    }
    const raw = await this.post(`/companies/${encodeURIComponent(options.companyId)}/domain-workflows/content-reviews`, options.requestId, {
      draftInputId: options.draftInputId, draftRevision: options.draftRevision, correctionId: options.correctionId,
      review: options.review, sample: options.sample, requestId: options.requestId,
    });
    const result = validateDomainContentReview(raw, {
      companyId: options.companyId, draftInputId: options.draftInputId, draftRevision: options.draftRevision, requestId: options.requestId,
    });
    if (!result.success) throw new Error(`failed to record content fact-check: ${validationMessage(result.issues)}`);
    return result.value;
  }

  async simulateContentPublication(options: SimulateContentPublicationOptions): Promise<DomainContentPublicationView> {
    assertCompanyScope(options.companyId);
    assertCompanyScope(options.reviewId);
    assertRequestID(options.requestId);
    const raw = await this.post(`/companies/${encodeURIComponent(options.companyId)}/domain-workflows/content-publications`, options.requestId, {
      reviewId: options.reviewId, requestId: options.requestId,
    });
    const result = validateDomainContentPublication(raw, options.companyId, options.reviewId, options.requestId);
    if (!result.success) throw new Error(`failed to simulate content publication: ${validationMessage(result.issues)}`);
    return result.value;
  }

  async recordContentCorrection(options: RecordContentCorrectionOptions): Promise<DomainContentCorrectionView> {
    assertCompanyScope(options.companyId);
    assertCompanyScope(options.publicationId);
    assertCompanyScope(options.correctionDraftInputId);
    assertRequestID(options.requestId);
    const rationale = options.rationale.trim();
    if (!/^[1-9]\d{0,18}$/.test(options.correctionDraftRevision) || rationale === '' || Array.from(rationale).length > 2000) {
      throw new Error('content correction requires a pinned newer draft revision and a rationale');
    }
    const raw = await this.post(`/companies/${encodeURIComponent(options.companyId)}/domain-workflows/content-corrections`, options.requestId, {
      publicationId: options.publicationId, correctionDraftInputId: options.correctionDraftInputId,
      correctionDraftRevision: options.correctionDraftRevision, rationale, requestId: options.requestId,
    });
    const result = validateDomainContentCorrection(raw, {
      companyId: options.companyId, publicationId: options.publicationId, draftInputId: options.correctionDraftInputId,
      draftRevision: options.correctionDraftRevision, requestId: options.requestId,
    });
    if (!result.success) throw new Error(`failed to record content correction: ${validationMessage(result.issues)}`);
    return result.value;
  }

  async recordContentFeedback(options: RecordContentFeedbackOptions): Promise<DomainContentFeedbackView> {
    assertCompanyScope(options.companyId);
    assertCompanyScope(options.publicationId);
    assertRequestID(options.requestId);
    const note = options.note.trim();
    if (note === '' || Array.from(note).length > 2000) throw new Error('content feedback must contain 1 to 2000 characters');
    const raw = await this.post(`/companies/${encodeURIComponent(options.companyId)}/domain-workflows/content-feedback`, options.requestId, {
      publicationId: options.publicationId, category: options.category, note, requestId: options.requestId,
    });
    const result = validateDomainContentFeedback(raw, {
      companyId: options.companyId, publicationId: options.publicationId, category: options.category, requestId: options.requestId,
    });
    if (!result.success) throw new Error(`failed to record content feedback: ${validationMessage(result.issues)}`);
    return result.value;
  }

  async recordDomainEvidence(options: RecordDomainEvidenceOptions): Promise<DomainEvidenceRecordView> {
    assertCompanyScope(options.companyId);
    assertRequestID(options.requestId);
    const raw = await this.post(`/companies/${encodeURIComponent(options.companyId)}/domain-evidence`, options.requestId, {
      profileId: options.profileId,
      profileRevision: options.profileRevision,
      evidence: options.evidence,
      requestId: options.requestId,
    });
    const result = validateDomainEvidenceRecord(raw, options.companyId, options.requestId);
    if (!result.success) throw new Error(`failed to record domain evidence: ${validationMessage(result.issues)}`);
    return result.value;
  }

  async recordDomainEvidenceReview(options: RecordDomainEvidenceReviewOptions): Promise<DomainEvidenceReviewRecordView> {
    assertCompanyScope(options.companyId);
    assertCompanyScope(options.recordId);
    assertCompanyScope(options.reviewerEmployeeId);
    assertRequestID(options.requestId);
    const raw = await this.post(`/companies/${encodeURIComponent(options.companyId)}/domain-evidence/${encodeURIComponent(options.recordId)}/review`, options.requestId, {
      outcome: options.outcome,
      reviewerEmployeeId: options.reviewerEmployeeId,
      rationale: options.rationale,
      previewedEvidence: options.previewedEvidence,
      requestId: options.requestId,
    });
    const result = validateDomainEvidenceReviewRecord(raw, options.companyId, options.recordId, options.requestId);
    if (!result.success) throw new Error(`failed to record domain evidence review: ${validationMessage(result.issues)}`);
    return result.value;
  }

  async recordDomainEvidenceSubstantiveAssessment(options: RecordDomainEvidenceSubstantiveAssessmentOptions): Promise<DomainEvidenceSubstantiveAssessmentRecordView> {
    assertCompanyScope(options.companyId);
    assertCompanyScope(options.recordId);
    assertCompanyScope(options.reviewerEmployeeId);
    assertRequestID(options.requestId);
    if (!/^[0-9a-f]{64}$/.test(options.evidenceDigest)) throw new CommandApiError('MALFORMED_INPUT', 400, 'evidence digest is invalid', options.recordId);
    const raw = await this.post(`/companies/${encodeURIComponent(options.companyId)}/domain-evidence/${encodeURIComponent(options.recordId)}/assessment`, options.requestId, {
      evidenceDigest: options.evidenceDigest,
      reviewerEmployeeId: options.reviewerEmployeeId,
      areaAssessments: options.areaAssessments,
      previewedEvidence: options.previewedEvidence,
      requestId: options.requestId,
    });
    const result = validateDomainEvidenceSubstantiveAssessmentRecord(raw, options.companyId, options.recordId, options.evidenceDigest, options.requestId);
    if (!result.success || result.value.reviewerEmployeeId !== options.reviewerEmployeeId
      || JSON.stringify(result.value.areaAssessments) !== JSON.stringify(options.areaAssessments)
      || JSON.stringify(result.value.previewedEvidence) !== JSON.stringify(options.previewedEvidence)) {
      throw new Error(`failed to record substantive domain assessment: ${result.success ? 'reviewer or evidence decisions differ from the command' : validationMessage(result.issues)}`);
    }
    return result.value;
  }

  async recordDomainProfileQualification(options: RecordDomainProfileQualificationOptions): Promise<DomainProfileQualificationRecordView> {
    assertCompanyScope(options.companyId);
    assertCompanyScope(options.profileId);
    assertRequestID(options.requestId);
    if (options.rationale.trim() === '' || Array.from(options.rationale).length > 2000) {
      throw new CommandApiError('MALFORMED_INPUT', 400, 'qualification rationale is invalid', options.profileId);
    }
    if (options.decision === 'qualified') {
      assertCompanyScope(options.evidenceInputId ?? '');
      if (options.evidenceInputRevision === undefined || !/^[1-9]\d*$/.test(options.evidenceInputRevision)) {
        throw new CommandApiError('MALFORMED_INPUT', 400, 'qualification evidence revision is invalid', options.profileId);
      }
    } else if (options.evidenceInputId !== undefined || options.evidenceInputRevision !== undefined) {
      throw new CommandApiError('MALFORMED_INPUT', 400, 'revocation cannot include qualification evidence', options.profileId);
    }
    const body = {
      decision: options.decision,
      profileRevision: options.profileRevision,
      ...(options.decision === 'qualified' ? {evidenceInputId: options.evidenceInputId, evidenceInputRevision: options.evidenceInputRevision} : {}),
      rationale: options.rationale,
      requestId: options.requestId,
    };
    const raw = await this.post(`/companies/${encodeURIComponent(options.companyId)}/domain-workflows/${encodeURIComponent(options.profileId)}/qualification`, options.requestId, body);
    const result = validateDomainProfileQualificationRecord(raw, options.companyId, options.profileId, options.requestId);
    if (!result.success || result.value.decision !== options.decision || result.value.profileRevision !== options.profileRevision ||
      result.value.evidenceInputId !== (options.evidenceInputId ?? '') || result.value.evidenceInputRevision !== (options.evidenceInputRevision ?? '0')) {
      throw new Error(`failed to record domain profile qualification: ${result.success ? 'decision or evidence differs from the command' : validationMessage(result.issues)}`);
    }
    return result.value;
  }

  async getDomainEvidenceArtifactPreview(options: GetDomainEvidenceArtifactPreviewOptions): Promise<DomainEvidenceArtifactPreviewView> {
    assertCompanyScope(options.companyId);
    assertCompanyScope(options.recordId);
    assertCompanyScope(options.inputId);
    if (!Number.isSafeInteger(options.inputRevision) || options.inputRevision <= 0) {
      throw new Error('failed to preview domain evidence: input revision is invalid');
    }
    if (!/^[0-9a-f]{64}$/.test(options.expectedContentSHA256)) {
      throw new Error('failed to preview domain evidence: manifest content digest is invalid');
    }
    if (!/^[0-9a-f]{64}$/.test(options.sourceDigest)) {
      throw new Error('failed to preview domain evidence: source digest is invalid');
    }
    if (!options.relativePath.trim() || options.relativePath.length > 1024 || options.relativePath.startsWith('/') || options.relativePath.includes('\\') || options.relativePath.split('/').some(segment => segment === '' || segment === '.' || segment === '..')) {
      throw new Error('failed to preview domain evidence: entry path is invalid');
    }
    const path = `/companies/${encodeURIComponent(options.companyId)}/domain-evidence/${encodeURIComponent(options.recordId)}/evidence/${encodeURIComponent(options.area)}/preview?path=${encodeURIComponent(options.relativePath)}`;
    const response = await fetch(`${this.baseUrl}${path}`, {
      credentials: 'same-origin',
      headers: this.requestHeaders({'Accept': 'image/png, image/jpeg, text/plain, text/markdown, text/csv, application/json'}),
    });
    observeProtectedResponse(path, response.status);
    if (!response.ok) {
      let detail = '';
      try {
        const body: unknown = await response.json();
        if (isRecord(body) && typeof body.error === 'string') detail = body.error;
      } catch {
        detail = '';
      }
      throw new Error(detail === '' ? `failed to preview domain evidence: http ${response.status}` : `failed to preview domain evidence: ${detail}`);
    }
    const mediaType = response.headers.get('Content-Type')?.split(';')[0] ?? '';
    const allowedMediaTypes = ['application/json', 'image/png', 'image/jpeg', 'text/plain', 'text/markdown', 'text/csv'];
    if (!allowedMediaTypes.includes(mediaType)) {
      throw new Error('domain evidence preview returned an unsupported media type');
    }
    const sourceDigest = response.headers.get('X-Source-SHA256');
    const contentDigest = response.headers.get('X-Content-SHA256');
    const contentLengthText = response.headers.get('Content-Length');
    const fileNameText = response.headers.get('X-Polis-Preview-Filename');
    const inputId = response.headers.get('X-Polis-Input-ID');
    const inputRevision = response.headers.get('X-Polis-Input-Revision');
    if (sourceDigest !== options.sourceDigest || inputId !== options.inputId || inputRevision !== String(options.inputRevision)
      || contentDigest === null || !/^[0-9a-f]{64}$/.test(contentDigest) || contentDigest !== options.expectedContentSHA256
      || contentLengthText === null || !/^\d+$/.test(contentLengthText) || fileNameText === null) {
      throw new Error('domain evidence preview metadata does not match the evidence reference');
    }
    let fileName: string;
    try {
      fileName = decodeURIComponent(fileNameText.replaceAll('+', '%20'));
    } catch {
      throw new Error('domain evidence preview filename is malformed');
    }
    const containsControlCharacter = Array.from(fileName).some(character => {
      const codePoint = character.codePointAt(0) ?? 0;
      return codePoint <= 0x1f || (codePoint >= 0x7f && codePoint <= 0x9f);
    });
    if (fileName.trim() === '' || Array.from(fileName).length > 255 || containsControlCharacter) {
      throw new Error('domain evidence preview filename is invalid');
    }
    const expectedBytes = Number(contentLengthText);
    if (!Number.isSafeInteger(expectedBytes) || expectedBytes <= 0 || expectedBytes > MAX_MISSION_INPUT_BYTES) {
      throw new Error('domain evidence preview exceeds the supported size limit');
    }
    const content = await response.blob();
    if (content.size !== expectedBytes || await sha256Hex(new Uint8Array(await content.arrayBuffer())) !== contentDigest) {
      throw new Error('domain evidence preview bytes do not match the returned checksum');
    }
    let textContent: string | null = null;
    if (mediaType.startsWith('text/') || mediaType === 'application/json') {
      try {
        textContent = new TextDecoder('utf-8', {fatal: true}).decode(await content.arrayBuffer());
      } catch {
        throw new Error('domain evidence preview text is not valid UTF-8');
      }
      if (textContent.includes('\u0000')) throw new Error('domain evidence preview text contains a NUL byte');
    }
    return {
      companyId: options.companyId,
      recordId: options.recordId,
      area: options.area,
      relativePath: options.relativePath,
      inputId: options.inputId,
      inputRevision: String(options.inputRevision),
      sourceDigest,
      contentDigest,
      fileName,
      mediaType,
      byteSize: String(expectedBytes),
      content,
      textContent,
    };
  }

  async listDomainEvidenceArtifactPreviewEntries(options: ListDomainEvidenceArtifactPreviewEntriesOptions): Promise<DomainEvidenceArtifactPreviewManifestView> {
    assertCompanyScope(options.companyId);
    assertCompanyScope(options.recordId);
    assertCompanyScope(options.inputId);
    if (!Number.isSafeInteger(options.inputRevision) || options.inputRevision <= 0 || !/^[0-9a-f]{64}$/.test(options.sourceDigest)) {
      throw new Error('failed to load domain evidence preview entries: input reference is invalid');
    }
    const path = `/companies/${encodeURIComponent(options.companyId)}/domain-evidence/${encodeURIComponent(options.recordId)}/evidence/${encodeURIComponent(options.area)}/preview/entries`;
    const raw = await this.get(path);
    const result = validateDomainEvidenceArtifactPreviewManifest(raw, options);
    if (!result.success) throw new Error(`failed to load domain evidence preview entries: ${validationMessage(result.issues)}`);
    return result.value;
  }

  async storeGitHubFeedbackCredential(options: StoreGitHubCredentialOptions): Promise<GitHubCredentialReceipt> {
    assertCompanyScope(options.companyId);
    assertRequestID(options.requestId);
    const raw = await this.post(`/companies/${encodeURIComponent(options.companyId)}/feedback/credentials`, options.requestId, {token: options.token});
    const result = validateGitHubCredentialReceipt(raw, true);
    if (!result.success) throw new Error(`failed to store GitHub credential: ${validationMessage(result.issues)}`);
    return result.value;
  }

  async deleteGitHubFeedbackCredential(options: DeleteGitHubCredentialOptions): Promise<GitHubCredentialReceipt> {
    assertCompanyScope(options.companyId);
    assertRequestID(options.requestId);
    const raw = await this.post(`/companies/${encodeURIComponent(options.companyId)}/feedback/credentials/delete`, options.requestId, {});
    const result = validateGitHubCredentialReceipt(raw, false);
    if (!result.success) throw new Error(`failed to delete GitHub credential: ${validationMessage(result.issues)}`);
    return result.value;
  }

  async registerGitHubFeedbackSource(options: RegisterGitHubFeedbackSourceOptions): Promise<GitHubFeedbackSourceCommandReceipt> {
    assertCompanyScope(options.companyId);
    assertRequestID(options.requestId);
    const repositoryId = Number(options.repositoryId);
    if (!Number.isSafeInteger(repositoryId) || repositoryId <= 0) throw new Error('GitHub repository id must be a positive integer');
    const raw = await this.post(`/companies/${encodeURIComponent(options.companyId)}/feedback/sources`, options.requestId, {sourceId: options.sourceId ?? '', repositoryId, owner: options.owner, name: options.name, requestId: options.requestId});
    const result = validateGitHubFeedbackSourceReceipt(raw, options.companyId);
    if (!result.success) throw new Error(`failed to register GitHub feedback source: ${validationMessage(result.issues)}`);
    return result.value;
  }

  async probeGitHubFeedbackSource(options: ProbeGitHubFeedbackSourceOptions): Promise<GitHubFeedbackProbeReceipt> {
    assertCompanyScope(options.companyId);
    assertCompanyScope(options.sourceId);
    assertRequestID(options.requestId);
    const raw = await this.post(`/companies/${encodeURIComponent(options.companyId)}/feedback/sources/${encodeURIComponent(options.sourceId)}/probe`, options.requestId, {rationale: options.rationale, requestId: options.requestId});
    const result = validateGitHubFeedbackProbeReceipt(raw, options.companyId, options.sourceId, options.requestId);
    if (!result.success) throw new Error(`failed to probe GitHub source: ${validationMessage(result.issues)}`);
    return result.value;
  }

  async decideGitHubFeedbackSource(options: DecideGitHubFeedbackSourceOptions): Promise<GitHubFeedbackSourceCommandReceipt> {
    assertCompanyScope(options.companyId);
    assertCompanyScope(options.sourceId);
    assertRequestID(options.requestId);
    const raw = await this.post(`/companies/${encodeURIComponent(options.companyId)}/feedback/sources/${encodeURIComponent(options.sourceId)}/decision`, options.requestId, {decision: options.decision, rationale: options.rationale, requestId: options.requestId});
    const result = validateGitHubFeedbackSourceReceipt(raw, options.companyId);
    if (!result.success) throw new Error(`failed to decide GitHub source: ${validationMessage(result.issues)}`);
    return result.value;
  }

  async pollGitHubFeedbackSource(options: PollGitHubFeedbackSourceOptions): Promise<GitHubFeedbackPollReceipt> {
    assertCompanyScope(options.companyId);
    assertCompanyScope(options.sourceId);
    assertRequestID(options.requestId);
    const raw = await this.post(`/companies/${encodeURIComponent(options.companyId)}/feedback/sources/${encodeURIComponent(options.sourceId)}/scan`, options.requestId, {requestId: options.requestId});
    const result = validateGitHubFeedbackPollReceipt(raw, options.companyId, options.sourceId, options.requestId);
    if (!result.success) throw new Error(`failed to poll GitHub source: ${validationMessage(result.issues)}`);
    return result.value;
  }

  async setGitHubFeedbackBacklogStatus(options: SetGitHubFeedbackBacklogStatusOptions): Promise<GitHubFeedbackBacklogStatusReceipt> {
    assertCompanyScope(options.companyId);
    assertCompanyScope(options.sourceId);
    if (!/^[1-9]\d{0,18}$/.test(options.providerItemId)) throw new Error('GitHub issue id is invalid');
    if (!/^[a-f0-9]{64}$/.test(options.revisionSha256)) throw new Error('GitHub issue revision is invalid');
    assertRequestID(options.requestId);
    const raw = await this.post(`/companies/${encodeURIComponent(options.companyId)}/feedback/backlog`, options.requestId, {
      sourceId: options.sourceId, providerItemId: options.providerItemId, revisionSha256: options.revisionSha256, status: options.status,
      rationale: options.rationale, requestId: options.requestId,
    });
    const result = validateGitHubFeedbackBacklogStatusReceipt(raw, options);
    if (!result.success) throw new Error(`failed to update GitHub backlog status: ${validationMessage(result.issues)}`);
    return result.value;
  }

  async setGitHubFeedbackCollectionPolicy(options: SetGitHubFeedbackCollectionPolicyOptions): Promise<GitHubFeedbackCollectionPolicyReceipt> {
    assertCompanyScope(options.companyId);
    assertCompanyScope(options.sourceId);
    assertRequestID(options.requestId);
    if (!Number.isSafeInteger(options.intervalSeconds) || options.intervalSeconds < 900 || options.intervalSeconds > 86400) {
      throw new Error('GitHub collection interval must be between 15 minutes and 24 hours');
    }
    const raw = await this.post(`/companies/${encodeURIComponent(options.companyId)}/feedback/collection-policy`, options.requestId, {
      sourceId: options.sourceId, enabled: options.enabled, intervalSeconds: options.intervalSeconds,
      rationale: options.rationale, requestId: options.requestId,
    });
    const result = validateGitHubFeedbackCollectionPolicyReceipt(raw, options);
    if (!result.success) throw new Error(`failed to update GitHub collection policy: ${validationMessage(result.issues)}`);
    return result.value;
  }

  async qualifyCapability(options: QualifyCapabilityOptions): Promise<unknown> {
    assertCompanyScope(options.companyId);
    assertCompanyScope(options.capabilityId);
    assertRequestID(options.requestId);
    return this.post(`/companies/${encodeURIComponent(options.companyId)}/capabilities/qualify`, options.requestId, options);
  }

  async decideCapability(options: DecideCapabilityOptions): Promise<unknown> {
    assertCompanyScope(options.companyId);
    assertCompanyScope(options.capabilityId);
    assertCompanyScope(options.qualificationId);
    assertRequestID(options.requestId);
    return this.post(`/companies/${encodeURIComponent(options.companyId)}/capabilities/decide`, options.requestId, options);
  }

  async reviewIncompleteCapabilityRevocation(options: ReviewCapabilityRevocationOptions): Promise<unknown> {
    assertCompanyScope(options.companyId);
    assertCompanyScope(options.revocationId);
    assertRequestID(options.requestId);
    if (options.rationale.trim() === '' || new TextEncoder().encode(options.rationale).length > 512) {
      throw new CommandApiError('MALFORMED_INPUT', 400, 'owner review rationale must contain 1 to 512 bytes', options.revocationId);
    }
    return this.post(`/companies/${encodeURIComponent(options.companyId)}/capabilities/revocations/${encodeURIComponent(options.revocationId)}/review`, options.requestId, {
      rationale: options.rationale,
      requestId: options.requestId,
    }, true);
  }

  async approveStdioMCPRuntimeQualification(options: Readonly<{companyId: string; runtimeQualificationId: string; rationale: string; requestId: string}>): Promise<unknown> {
    assertCompanyScope(options.companyId);
    assertCompanyScope(options.runtimeQualificationId);
    assertRequestID(options.requestId);
    const {companyId, ...request} = options;
    return this.post(`/companies/${encodeURIComponent(companyId)}/capabilities/runtime-approve`, options.requestId, request);
  }

  async observeStdioMCPRuntime(options: ObserveStdioMCPRuntimeOptions): Promise<unknown> {
    assertCompanyScope(options.companyId);
    assertCompanyScope(options.serverId);
    assertCompanyScope(options.packageRevisionId);
    assertCompanyScope(options.capabilityQualificationId);
    assertRequestID(options.requestId);
    const {companyId, ...request} = options;
    return this.post(`/companies/${encodeURIComponent(companyId)}/capabilities/runtime-observe`, options.requestId, request);
  }

  async observeStreamableHTTPMCPRuntime(options: ObserveStreamableHTTPMCPRuntimeOptions): Promise<unknown> {
    assertCompanyScope(options.companyId);
    assertCompanyScope(options.capabilityId);
    assertCompanyScope(options.capabilityQualificationId);
    assertRequestID(options.requestId);
    const {companyId, ...request} = options;
    return this.post(`/companies/${encodeURIComponent(companyId)}/capabilities/runtime-observe-http`, options.requestId, request);
  }

  async bindEmployeeCapability(options: BindEmployeeCapabilityOptions): Promise<unknown> {
    assertCompanyScope(options.companyId);
    assertCompanyScope(options.employeeId);
    assertCompanyScope(options.capabilityId);
    assertCompanyScope(options.qualificationId);
    assertRequestID(options.requestId);
    return this.post(`/companies/${encodeURIComponent(options.companyId)}/capabilities/bind`, options.requestId, options);
  }

  async revokeEmployeeCapability(options: BindEmployeeCapabilityOptions): Promise<unknown> {
    assertCompanyScope(options.companyId);
    assertCompanyScope(options.employeeId);
    assertCompanyScope(options.capabilityId);
    assertCompanyScope(options.qualificationId);
    assertRequestID(options.requestId);
    return this.post(`/companies/${encodeURIComponent(options.companyId)}/capabilities/unbind`, options.requestId, options);
  }

  async configureNotificationRoute(options: ConfigureNotificationRouteOptions): Promise<CompanyCommandReceipt> {
    assertCompanyScope(options.companyId);
    assertRequestID(options.requestId);
    const raw = await this.post(`/companies/${encodeURIComponent(options.companyId)}/notifications/route`, options.requestId, {adapter: options.adapter, destination: options.destination, safetyAlias: options.safetyAlias ?? '', credentialRef: options.credentialRef ?? '', enabled: options.enabled, requestId: options.requestId});
    return validateCompanyCommandReceipt(raw, 'notification.route.update');
  }

  async testNotification(options: TestNotificationOptions): Promise<Readonly<{intentId: string; deliveryId: string; adapter: string; state: string; requestId: string; accepted: true; acceptedAt: string}>> {
    assertCompanyScope(options.companyId);
    assertRequestID(options.requestId);
    const raw = await this.post(`/companies/${encodeURIComponent(options.companyId)}/notifications/test`, options.requestId, {requestId: options.requestId});
    if (!isRecord(raw) || typeof raw.intentId !== 'string' || typeof raw.deliveryId !== 'string' || typeof raw.adapter !== 'string' || typeof raw.state !== 'string' || raw.requestId !== options.requestId || raw.accepted !== true || typeof raw.acceptedAt !== 'string') {
      throw new Error('failed to parse notification test receipt: response is malformed');
    }
    return raw as Readonly<{intentId: string; deliveryId: string; adapter: string; state: string; requestId: string; accepted: true; acceptedAt: string}>;
  }

  async getCompanyOverview(options: CompanyScopeOptions): Promise<CompanyOverviewView> {
    assertCompanyScope(options.companyId);
    const raw = await this.get(`/companies/${encodeURIComponent(options.companyId)}/overview`);
    const result = validateCompanyOverview(raw, options.companyId);
    if (!result.success) {
      throw new Error(`failed to load company overview: ${validationMessage(result.issues)}`);
    }
    return result.value;
  }

  async listActivityEvents(options: ActivityQueryOptions): Promise<ActivityView> {
    assertCompanyScope(options.companyId);
    if (!isValidActivityLimit(options.limit)) {
      throw new Error('failed to load activity: limit must be between 1 and 100');
    }
    if (!isValidOpaqueCursor(options.cursor) || !isValidOpaqueCursor(options.snapshotCursor)) {
      throw new Error('failed to load activity: cursor cannot be blank');
    }
    if (options.snapshotCursor === null) {
      throw new Error('failed to load activity: snapshot watermark is required');
    }
    const params = new URLSearchParams({limit: String(options.limit)});
    if (options.cursor !== null) {
      params.set('cursor', options.cursor);
    }
    if (options.snapshotCursor !== null) {
      params.set('snapshot_cursor', options.snapshotCursor);
    }
    const raw = await this.get(`/companies/${encodeURIComponent(options.companyId)}/activity?${params.toString()}`);
    const result = validateActivityView(raw, options.companyId);
    if (!result.success) {
      throw new Error(`failed to load activity: ${validationMessage(result.issues)}`);
    }
    return result.value;
  }

  async listMissionInputs(options: MissionInputQueryOptions): Promise<ReadonlyArray<MissionInputView>> {
    assertCompanyScope(options.companyId);
    assertCompanyScope(options.missionId);
    const raw = await this.get(`/companies/${encodeURIComponent(options.companyId)}/missions/${encodeURIComponent(options.missionId)}/inputs`);
    const result = validateMissionInputs(raw, options.companyId, options.missionId);
    if (!result.success) {
      throw new Error(`failed to parse mission inputs response: ${validationMessage(result.issues)}`);
    }
    return result.value;
  }

  async uploadMissionInput(options: UploadMissionInputOptions): Promise<MissionInputCommandReceipt> {
    assertCompanyScope(options.companyId);
    assertCompanyScope(options.missionId);
    if (options.inputId !== null) assertCompanyScope(options.inputId);
    assertRequestID(options.requestId);
    if (options.file.size > MAX_MISSION_INPUT_BYTES) {
      throw new CommandApiError('MALFORMED_INPUT', 413, 'failed to upload mission input: file exceeds 8 MB', options.missionId);
    }
    const body = new FormData();
    body.set('requestId', options.requestId);
    if (options.inputId !== null) body.set('inputId', options.inputId);
    body.set('file', options.file, options.file.name);
    const raw = await this.postMultipart(`/companies/${encodeURIComponent(options.companyId)}/missions/${encodeURIComponent(options.missionId)}/inputs`, options.requestId, body);
    const result = validateMissionInputCommandReceipt(raw, options.companyId, options.missionId, options.requestId);
    if (!result.success) {
      throw new Error(`failed to parse mission input receipt: ${validationMessage(result.issues)}`);
    }
    return result.value;
  }

  async listDailyRoutines(options: DailyRoutineQueryOptions): Promise<ReadonlyArray<DailyRoutineView>> {
    assertCompanyScope(options.companyId);
    assertCompanyScope(options.missionId);
    const raw = await this.get(`/companies/${encodeURIComponent(options.companyId)}/missions/${encodeURIComponent(options.missionId)}/routines`);
    const result = validateDailyRoutines(raw, options.missionId);
    if (!result.success) {
      throw new Error(`failed to parse daily Routine response: ${validationMessage(result.issues)}`);
    }
    return result.value;
  }

  async getMemoryTaskStatus(options: MemoryTaskStatusQueryOptions): Promise<MemoryTaskStatusView> {
    assertCompanyScope(options.companyId);
    assertCompanyScope(options.taskId);
    const raw = await this.get(`/companies/${encodeURIComponent(options.companyId)}/tasks/${encodeURIComponent(options.taskId)}/memory-impact`);
    const result = validateMemoryTaskStatus(raw);
    if (!result.success) throw new Error(`failed to parse task memory impact response: ${validationMessage(result.issues)}`);
    return result.value;
  }

  async listMemoryCorrections(options: MemoryCorrectionQueueQueryOptions): Promise<MemoryCorrectionQueueView> {
    assertCompanyScope(options.companyId);
    const raw = await this.get(`/companies/${encodeURIComponent(options.companyId)}/memory/corrections`);
    return validateMemoryCorrectionQueue(raw);
  }

  async proposeMemoryCorrection(options: ProposeMemoryCorrectionOptions): Promise<MemoryCorrectionCommandReceiptView> {
    assertCompanyScope(options.companyId);
    assertCompanyScope(options.workerSessionId);
    assertCompanyScope(options.correctionId);
    assertCompanyScope(options.recordId);
    assertRequestID(options.requestId);
    if (!Number.isSafeInteger(options.baseRevision) || options.baseRevision < 1) throw new Error('memory correction base revision is invalid');
    const reason = options.reason.trim();
    const reasonBytes = new TextEncoder().encode(reason).length;
    if (reasonBytes < 1 || reasonBytes > 2048) throw new Error('memory correction reason must be 1–2048 bytes');
    if (!Number.isSafeInteger(options.source.revision) || options.source.revision < 1 || !/^[0-9a-f]{64}$/.test(options.source.sha256)) throw new Error('memory correction source reference is malformed');
    const raw = await this.post(`/companies/${encodeURIComponent(options.companyId)}/memory/corrections/propose`, options.requestId, {
      workerSessionId: options.workerSessionId, correctionId: options.correctionId, recordId: options.recordId,
      baseRevision: options.baseRevision, content: options.content, source: options.source, reason, requestId: options.requestId,
    });
    return validateMemoryCorrectionCommandReceipt(raw);
  }

  async reviewMemoryCorrection(options: ReviewMemoryCorrectionOptions): Promise<MemoryCorrectionCommandReceiptView> {
    assertCompanyScope(options.companyId);
    assertCompanyScope(options.workerSessionId);
    assertCompanyScope(options.correctionId);
    assertRequestID(options.requestId);
    const reason = options.reason.trim();
    const reasonBytes = new TextEncoder().encode(reason).length;
    if (reasonBytes < 1 || reasonBytes > 2048) throw new Error('memory correction review reason must be 1–2048 bytes');
    const raw = await this.post(`/companies/${encodeURIComponent(options.companyId)}/memory/corrections/${encodeURIComponent(options.correctionId)}/review`, options.requestId, {
      workerSessionId: options.workerSessionId, decision: options.decision, reason, requestId: options.requestId,
    });
    return validateMemoryCorrectionCommandReceipt(raw);
  }

  async getCompanyToolCallBudget(options: CompanyScopeOptions): Promise<CompanyToolCallBudgetView> {
    assertCompanyScope(options.companyId);
    const raw = await this.get(`/companies/${encodeURIComponent(options.companyId)}/company-tool-call-budget`);
    return validateCompanyToolCallBudget(raw, options.companyId);
  }

  async changeCompanyToolCallBudget(options: ChangeCompanyToolCallBudgetOptions): Promise<CompanyToolCallBudgetChangeReceiptView> {
    assertCompanyScope(options.companyId);
    assertRequestID(options.requestId);
    if (!Number.isSafeInteger(options.resultingToolCallLimit) || options.resultingToolCallLimit < 1
      || !Number.isSafeInteger(options.expectedRevision) || options.expectedRevision < 0
      || (options.expectedToolCallLimit !== null && (!Number.isSafeInteger(options.expectedToolCallLimit) || options.expectedToolCallLimit < 1))
      || (options.expectedToolCallLimit === null && options.expectedRevision !== 0)
      || (options.expectedToolCallLimit !== null && (options.expectedRevision < 1 || options.resultingToolCallLimit <= options.expectedToolCallLimit))) {
      throw new Error('Company tool-call budget values must be positive, increasing safe integers');
    }
    const reason = options.reason.trim();
    const reasonLength = Array.from(reason).length;
    if (reasonLength < 1 || reasonLength > 500) throw new Error('allocation reason must contain 1–500 characters');
    const raw = await this.post(`/companies/${encodeURIComponent(options.companyId)}/company-tool-call-budget`, options.requestId, {
      expectedToolCallLimit: options.expectedToolCallLimit,
      expectedRevision: options.expectedRevision,
      resultingToolCallLimit: options.resultingToolCallLimit,
      reason,
      requestId: options.requestId,
      confirm: true,
    });
    return validateCompanyToolCallBudgetChangeReceipt(raw, options.companyId, options.expectedRevision + 1, options.expectedToolCallLimit === null);
  }

  async setCompanyToolCallClosingReserve(options: SetCompanyToolCallClosingReserveOptions): Promise<CompanyToolCallClosingReserveReceiptView> {
    assertCompanyScope(options.companyId);
    assertRequestID(options.requestId);
    if (!Number.isSafeInteger(options.reservedToolCalls) || options.reservedToolCalls < 0
      || !Number.isSafeInteger(options.expectedCompanyBudgetRevision) || options.expectedCompanyBudgetRevision < 0
      || !Number.isSafeInteger(options.expectedReserveRevision) || options.expectedReserveRevision < 0
      || (options.expectedCompanyToolCallLimit !== null && (!Number.isSafeInteger(options.expectedCompanyToolCallLimit) || options.expectedCompanyToolCallLimit < 1))
      || (options.expectedCompanyToolCallLimit === null && options.expectedCompanyBudgetRevision !== 0)
      || (options.expectedCompanyToolCallLimit !== null && options.expectedCompanyBudgetRevision < 1)) {
      throw new Error('Company closing reserve values must be safe integers');
    }
    const reason = options.reason.trim();
    const reasonLength = Array.from(reason).length;
    if (reasonLength < 1 || reasonLength > 500) throw new Error('closing reserve reason must contain 1–500 characters');
    const raw = await this.post(`/companies/${encodeURIComponent(options.companyId)}/company-tool-call-budget/closing-reserve`, options.requestId, {
      reservedToolCalls: options.reservedToolCalls,
      expectedCompanyToolCallLimit: options.expectedCompanyToolCallLimit,
      expectedCompanyBudgetRevision: options.expectedCompanyBudgetRevision,
      expectedReserveRevision: options.expectedReserveRevision,
      reason,
      requestId: options.requestId,
      confirm: true,
    });
    return validateCompanyToolCallClosingReserveReceipt(raw, options.companyId, options.expectedReserveRevision + 1);
  }

  async listProblemToolCallBudgets(options: ProblemToolCallBudgetQueryOptions): Promise<ProblemToolCallBudgetListView> {
    assertCompanyScope(options.companyId);
    const raw = await this.get(`/companies/${encodeURIComponent(options.companyId)}/problem-budgets`);
    return validateProblemToolCallBudgetList(raw);
  }

  async listMissionToolCallBudgets(options: MissionToolCallBudgetQueryOptions): Promise<MissionToolCallBudgetListView> {
    assertCompanyScope(options.companyId);
    const raw = await this.get(`/companies/${encodeURIComponent(options.companyId)}/mission-budgets`);
    return validateMissionToolCallBudgetList(raw);
  }

  async changeMissionToolCallBudget(options: ChangeMissionToolCallBudgetOptions): Promise<MissionToolCallBudgetChangeReceiptView> {
    assertCompanyScope(options.companyId);
    assertCompanyScope(options.missionId);
    assertRequestID(options.requestId);
    if (!Number.isSafeInteger(options.resultingToolCallLimit) || options.resultingToolCallLimit < 1
      || !Number.isSafeInteger(options.expectedRevision) || options.expectedRevision < 0
      || (options.expectedToolCallLimit !== null && (!Number.isSafeInteger(options.expectedToolCallLimit) || options.expectedToolCallLimit < 1))
      || (options.expectedToolCallLimit === null && options.expectedRevision !== 0)
      || (options.expectedToolCallLimit !== null && (options.expectedRevision < 1 || options.resultingToolCallLimit <= options.expectedToolCallLimit))) {
      throw new Error('Mission tool-call budget values must be positive, increasing safe integers');
    }
    const reason = options.reason.trim();
    const reasonLength = Array.from(reason).length;
    if (reasonLength < 1 || reasonLength > 500) throw new Error('allocation reason must contain 1–500 characters');
    const raw = await this.post(`/companies/${encodeURIComponent(options.companyId)}/mission-budgets/${encodeURIComponent(options.missionId)}/allocations`, options.requestId, {
      expectedToolCallLimit: options.expectedToolCallLimit,
      expectedRevision: options.expectedRevision,
      resultingToolCallLimit: options.resultingToolCallLimit,
      reason,
      requestId: options.requestId,
      confirm: true,
    });
    return validateMissionToolCallBudgetChangeReceipt(raw, options.missionId, options.expectedRevision + 1, options.expectedToolCallLimit === null);
  }

  async setMissionToolCallClosingReserve(options: SetMissionToolCallClosingReserveOptions): Promise<MissionToolCallClosingReserveReceiptView> {
    assertCompanyScope(options.companyId);
    assertCompanyScope(options.missionId);
    assertRequestID(options.requestId);
    if (!Number.isSafeInteger(options.reservedToolCalls) || options.reservedToolCalls < 0
      || !Number.isSafeInteger(options.expectedMissionBudgetRevision) || options.expectedMissionBudgetRevision < 0
      || !Number.isSafeInteger(options.expectedReserveRevision) || options.expectedReserveRevision < 0
      || (options.expectedMissionToolCallLimit !== null && (!Number.isSafeInteger(options.expectedMissionToolCallLimit) || options.expectedMissionToolCallLimit < 1))
      || (options.expectedMissionToolCallLimit === null && options.expectedMissionBudgetRevision !== 0)
      || (options.expectedMissionToolCallLimit !== null && options.expectedMissionBudgetRevision < 1)) {
      throw new Error('Mission closing reserve values must be safe integers');
    }
    const reason = options.reason.trim();
    const reasonLength = Array.from(reason).length;
    if (reasonLength < 1 || reasonLength > 500) throw new Error('closing reserve reason must contain 1–500 characters');
    const raw = await this.post(`/companies/${encodeURIComponent(options.companyId)}/mission-budgets/${encodeURIComponent(options.missionId)}/closing-reserve`, options.requestId, {
      reservedToolCalls: options.reservedToolCalls,
      expectedMissionToolCallLimit: options.expectedMissionToolCallLimit,
      expectedMissionBudgetRevision: options.expectedMissionBudgetRevision,
      expectedReserveRevision: options.expectedReserveRevision,
      reason,
      requestId: options.requestId,
      confirm: true,
    });
    return validateMissionToolCallClosingReserveReceipt(raw, options.missionId, options.expectedReserveRevision + 1);
  }

  async allocateProblemToolCalls(options: AllocateProblemToolCallsOptions): Promise<ProblemToolCallAllocationReceiptView> {
    assertCompanyScope(options.companyId);
    if (!/^problem:[A-Za-z0-9_-]{1,80}$/.test(options.problemKey)) throw new Error('ProblemKey is malformed');
    assertRequestID(options.requestId);
    if (!Number.isSafeInteger(options.additionalToolCalls) || options.additionalToolCalls < 1
      || !Number.isSafeInteger(options.expectedToolCallLimit) || options.expectedToolCallLimit < 1
      || !Number.isSafeInteger(options.expectedRevision) || options.expectedRevision < 1) {
      throw new Error('ProblemKey allocation values must be positive safe integers');
    }
    const reason = options.reason.trim();
    const reasonLength = Array.from(reason).length;
    if (reasonLength < 1 || reasonLength > 500) throw new Error('allocation reason must contain 1–500 characters');
    const raw = await this.post(`/companies/${encodeURIComponent(options.companyId)}/problem-budgets/${encodeURIComponent(options.problemKey)}/allocations`, options.requestId, {
      additionalToolCalls: options.additionalToolCalls,
      expectedToolCallLimit: options.expectedToolCallLimit,
      expectedRevision: options.expectedRevision,
      reason,
      requestId: options.requestId,
      confirm: true,
    });
    return validateProblemToolCallAllocationReceipt(raw, options.problemKey, options.expectedRevision + 1);
  }

  async allocateTaskToolCalls(options: AllocateTaskToolCallsOptions): Promise<TaskToolCallAllocationReceiptView> {
    assertCompanyScope(options.companyId);
    assertCompanyScope(options.taskId);
    if (!/^problem:[A-Za-z0-9_-]{1,80}$/.test(options.problemKey)) throw new Error('ProblemKey is malformed');
    assertRequestID(options.requestId);
    if (!Number.isSafeInteger(options.additionalToolCalls) || options.additionalToolCalls < 1
      || !Number.isSafeInteger(options.expectedToolCallLimit) || options.expectedToolCallLimit < 1
      || !Number.isSafeInteger(options.expectedTaskRevision) || options.expectedTaskRevision < 1
      || !Number.isSafeInteger(options.expectedProblemRevision) || options.expectedProblemRevision < 1
      || !Number.isSafeInteger(options.expectedReserveRevision) || options.expectedReserveRevision < 0) {
      throw new Error('Task allocation values must be safe integers');
    }
    const reason = options.reason.trim();
    const reasonLength = Array.from(reason).length;
    if (reasonLength < 1 || reasonLength > 500) throw new Error('allocation reason must contain 1–500 characters');
    const raw = await this.post(`/companies/${encodeURIComponent(options.companyId)}/problem-budgets/${encodeURIComponent(options.problemKey)}/tasks/${encodeURIComponent(options.taskId)}/allocations`, options.requestId, {
      additionalToolCalls: options.additionalToolCalls,
      expectedToolCallLimit: options.expectedToolCallLimit,
      expectedTaskRevision: options.expectedTaskRevision,
      expectedProblemRevision: options.expectedProblemRevision,
      expectedReserveRevision: options.expectedReserveRevision,
      reason,
      requestId: options.requestId,
      confirm: true,
    });
    return validateTaskToolCallAllocationReceipt(raw, options.taskId, options.expectedTaskRevision + 1);
  }

  async closeTaskToolBudgetIncomplete(options: CloseTaskToolBudgetIncompleteOptions): Promise<TaskToolCallIncompleteClosureReceiptView> {
    assertCompanyScope(options.companyId);
    assertCompanyScope(options.taskId);
    if (!/^problem:[A-Za-z0-9_-]{1,80}$/.test(options.problemKey)) throw new Error('ProblemKey is malformed');
    assertRequestID(options.requestId);
    for (const value of [options.expectedTaskToolCallsUsed, options.expectedTaskRevision, options.expectedProblemToolCallsUsed, options.expectedProblemRevision, options.expectedClosingReserveToolCalls, options.expectedClosingReserveRemaining, options.expectedReserveRevision]) {
      if (!Number.isSafeInteger(value) || value < 0) throw new Error('Task closeout snapshots must be safe integers');
    }
    if (options.expectedTaskRevision < 1 || options.expectedProblemRevision < 1
      || options.expectedClosingReserveRemaining > options.expectedClosingReserveToolCalls
      || (options.expectedTaskToolCallLimit !== null && (!Number.isSafeInteger(options.expectedTaskToolCallLimit) || options.expectedTaskToolCallLimit < 0))
      || (options.expectedProblemToolCallLimit !== null && (!Number.isSafeInteger(options.expectedProblemToolCallLimit) || options.expectedProblemToolCallLimit < 0))) {
      throw new Error('Task closeout limits or revisions are malformed');
    }
    const reason = options.reason.trim();
    const reasonLength = Array.from(reason).length;
    if (reasonLength < 1 || reasonLength > 500) throw new Error('closeout reason must contain 1–500 characters');
    const raw = await this.post(`/companies/${encodeURIComponent(options.companyId)}/problem-budgets/${encodeURIComponent(options.problemKey)}/tasks/${encodeURIComponent(options.taskId)}/budget-closure`, options.requestId, {
      expectedTaskToolCallLimit: options.expectedTaskToolCallLimit,
      expectedTaskToolCallsUsed: options.expectedTaskToolCallsUsed,
      expectedTaskRevision: options.expectedTaskRevision,
      expectedProblemToolCallLimit: options.expectedProblemToolCallLimit,
      expectedProblemToolCallsUsed: options.expectedProblemToolCallsUsed,
      expectedProblemRevision: options.expectedProblemRevision,
      expectedClosingReserveToolCalls: options.expectedClosingReserveToolCalls,
      expectedClosingReserveRemaining: options.expectedClosingReserveRemaining,
      expectedReserveRevision: options.expectedReserveRevision,
      reason,
      requestId: options.requestId,
      confirm: true,
    });
    return validateTaskToolCallIncompleteClosureReceipt(raw, options.taskId);
  }

  async setProblemToolCallClosingReserve(options: SetProblemToolCallClosingReserveOptions): Promise<ProblemToolCallClosingReserveReceiptView> {
    assertCompanyScope(options.companyId);
    if (!/^problem:[A-Za-z0-9_-]{1,80}$/.test(options.problemKey)) throw new Error('ProblemKey is malformed');
    assertRequestID(options.requestId);
    if (!Number.isSafeInteger(options.reservedToolCalls) || options.reservedToolCalls < 0
      || !Number.isSafeInteger(options.expectedToolCallLimit) || options.expectedToolCallLimit < 0
      || !Number.isSafeInteger(options.expectedBudgetRevision) || options.expectedBudgetRevision < 1
      || !Number.isSafeInteger(options.expectedReserveRevision) || options.expectedReserveRevision < 0) {
      throw new Error('ProblemKey closing reserve values must be safe integers');
    }
    const reason = options.reason.trim();
    const reasonLength = Array.from(reason).length;
    if (reasonLength < 1 || reasonLength > 500) throw new Error('closing reserve reason must contain 1–500 characters');
    const raw = await this.post(`/companies/${encodeURIComponent(options.companyId)}/problem-budgets/${encodeURIComponent(options.problemKey)}/closing-reserve`, options.requestId, {
      reservedToolCalls: options.reservedToolCalls,
      expectedToolCallLimit: options.expectedToolCallLimit,
      expectedBudgetRevision: options.expectedBudgetRevision,
      expectedReserveRevision: options.expectedReserveRevision,
      reason,
      requestId: options.requestId,
      confirm: true,
    });
    return validateProblemToolCallClosingReserveReceipt(raw, options.problemKey, options.expectedReserveRevision + 1);
  }

  async getMemoryTaskRevalidationPreview(options: MemoryTaskRevalidationPreviewOptions): Promise<MemoryTaskRevalidationPreviewView> {
    assertCompanyScope(options.companyId);
    assertCompanyScope(options.taskId);
    assertCompanyScope(options.dependencyId);
    assertCompanyScope(options.correctionId);
    const query = new URLSearchParams({dependencyId: options.dependencyId, correctionId: options.correctionId});
    const raw = await this.get(`/companies/${encodeURIComponent(options.companyId)}/tasks/${encodeURIComponent(options.taskId)}/memory-revalidation/preview?${query.toString()}`);
    const result = validateMemoryTaskRevalidationPreview(raw, options.taskId, options.dependencyId, options.correctionId);
    if (!result.success) throw new Error(`failed to parse memory revalidation preview: ${validationMessage(result.issues)}`);
    const contentDigest = await sha256Hex(new TextEncoder().encode(result.value.replacementContent));
    if (contentDigest !== result.value.replacementContentSha256) throw new Error('replacement memory content digest does not match preview');
    const workspaceDigest = await sha256Hex(new TextEncoder().encode(result.value.workspaceContent));
    if (workspaceDigest !== result.value.workspaceDigest) throw new Error('reviewed workspace content digest does not match preview');
    return result.value;
  }

  async revalidateMemoryTask(options: RevalidateMemoryTaskOptions): Promise<MemoryTaskRevalidationReceipt> {
    assertCompanyScope(options.companyId);
    assertCompanyScope(options.taskId);
    assertCompanyScope(options.dependencyId);
    assertCompanyScope(options.correctionId);
    assertRequestID(options.requestId);
    if (!/^[0-9a-f]{64}$/.test(options.contextSha256)) throw new Error('memory revalidation context digest is malformed');
    const reason = options.reason.trim();
    const reasonBytes = new TextEncoder().encode(reason).length;
    if (reasonBytes < 1 || reasonBytes > 2048) throw new Error('memory revalidation reason must be 1–2048 bytes');
    const raw = await this.post(`/companies/${encodeURIComponent(options.companyId)}/tasks/${encodeURIComponent(options.taskId)}/memory-revalidation`, options.requestId, {
      dependencyId: options.dependencyId,
      correctionId: options.correctionId,
      contextSha256: options.contextSha256,
      reason,
      requestId: options.requestId,
    });
    const result = validateMemoryTaskRevalidationReceipt(raw);
    if (!result.success) throw new Error(`failed to parse memory revalidation receipt: ${validationMessage(result.issues)}`);
    return result.value;
  }

  async getArtifactDeliveryManifest(options: Readonly<{companyId: string; artifactId: string}>): Promise<ArtifactDeliveryManifestResponse> {
    assertCompanyScope(options.companyId);
    assertCompanyScope(options.artifactId);
    const raw = await this.get(`/companies/${encodeURIComponent(options.companyId)}/artifacts/${encodeURIComponent(options.artifactId)}/manifest`);
    const result = validateArtifactDeliveryManifest(raw, options.companyId, options.artifactId);
    if (!result.success) {
      throw new Error(`failed to load artifact delivery manifest: ${validationMessage(result.issues)}`);
    }
    const manifestBytes = new TextEncoder().encode(JSON.stringify(result.value.manifest));
    if (await sha256Hex(manifestBytes) !== result.value.manifestSha256) {
      throw new Error('artifact delivery manifest checksum does not match');
    }
    return result.value;
  }

  async getDurableDelivery(options: Readonly<{companyId: string; artifactId: string}>): Promise<DurableDeliveryResponse> {
    assertCompanyScope(options.companyId);
    assertCompanyScope(options.artifactId);
    const raw = await this.get(`/companies/${encodeURIComponent(options.companyId)}/artifacts/${encodeURIComponent(options.artifactId)}/delivery`);
    const result = validateDurableDelivery(normalizeDurableDeliveryResponse(raw), options.companyId, options.artifactId);
    if (!result.success) {
      throw new Error(`failed to load durable artifact delivery: ${validationMessage(result.issues)}`);
    }
    const manifestBytes = new TextEncoder().encode(JSON.stringify(result.value.manifest));
    if (await sha256Hex(manifestBytes) !== result.value.manifestSha256) {
      throw new Error('durable delivery manifest checksum does not match');
    }
    return result.value;
  }

  async completeDurableDeliveryManifest(options: CompleteDurableDeliveryManifestOptions): Promise<DurableDeliveryManifestCompletionReceipt> {
    assertCompanyScope(options.companyId);
    assertCompanyScope(options.artifactId);
    assertRequestID(options.requestId);
    if (!isCanonicalPositiveInt64(options.expectedManifestRevision)) throw new Error('delivery completion requires a canonical manifest revision');
    const evidence = [options.sourceInputs, options.environmentBuild, options.runInstructions, options.limitations, options.licenseSource];
    if (evidence.some(item => item.reference.trim() !== item.reference || item.reference === '' || !/^[0-9a-f]{64}$/.test(item.digest) || item.detail.trim() === '' || new TextEncoder().encode(item.detail).byteLength > 512)) {
      throw new Error('delivery completion evidence is malformed or missing');
    }
    const raw = await this.post(
      `/companies/${encodeURIComponent(options.companyId)}/artifacts/${encodeURIComponent(options.artifactId)}/delivery/complete`,
      options.requestId,
      {
        requestId: options.requestId,
        expectedManifestRevision: options.expectedManifestRevision,
        sourceInputs: options.sourceInputs,
        environmentBuild: options.environmentBuild,
        runInstructions: options.runInstructions,
        limitations: options.limitations,
        licenseSource: options.licenseSource,
      },
      true,
    );
    const result = validateDurableDeliveryManifestCompletionReceipt(raw, {requestId: options.requestId, companyId: options.companyId, artifactId: options.artifactId, expectedManifestRevision: options.expectedManifestRevision});
    if (!result.success) throw new Error(`failed to complete durable artifact delivery: ${validationMessage(result.issues)}`);
    return result.value;
  }

  async invalidateDurableDeliveryManifest(options: InvalidateDurableDeliveryManifestOptions): Promise<DurableDeliveryManifestInvalidationReceipt> {
    assertCompanyScope(options.companyId);
    assertCompanyScope(options.artifactId);
    assertRequestID(options.requestId);
    const reason = options.reason.trim();
    if (!isCanonicalPositiveInt64(options.expectedManifestRevision) || (options.state !== 'invalidated' && options.state !== 'withdrawn') || reason === '' || new TextEncoder().encode(reason).byteLength > 4096) {
      throw new Error('delivery invalidation requires a current revision, terminal state, and reason');
    }
    const raw = await this.post(
      `/companies/${encodeURIComponent(options.companyId)}/artifacts/${encodeURIComponent(options.artifactId)}/delivery/invalidate`,
      options.requestId,
      {requestId: options.requestId, expectedManifestRevision: options.expectedManifestRevision, state: options.state, reason},
      true,
    );
    const result = validateDurableDeliveryManifestInvalidationReceipt(raw, {
      requestId: options.requestId,
      companyId: options.companyId,
      artifactId: options.artifactId,
      expectedManifestRevision: options.expectedManifestRevision,
      state: options.state,
      reason,
    });
    if (!result.success) throw new Error(`failed to invalidate durable artifact delivery: ${validationMessage(result.issues)}`);
    return result.value;
  }

  async recordDurableUserDisposition(options: RecordDurableUserDispositionOptions): Promise<DurableUserDispositionCommandReceipt> {
    assertCompanyScope(options.companyId);
    assertCompanyScope(options.artifactId);
    assertRequestID(options.requestId);
    const reason = options.reason.trim().replace(/^\u0085+|\u0085+$/gu, '');
    if (!isCanonicalPositiveInt64(options.expectedManifestRevision)
      || !isCanonicalPositiveInt64(options.expectedDispositionRevision)
      || (options.state !== 'accepted' && options.state !== 'changes_requested')
      || reason === '' || new TextEncoder().encode(reason).byteLength > 4096) {
      throw new Error('durable user disposition requires current decimal revisions, an explicit decision, and a reason');
    }
    const raw = await this.post(
      `/companies/${encodeURIComponent(options.companyId)}/artifacts/${encodeURIComponent(options.artifactId)}/delivery/disposition`,
      options.requestId,
      {
        requestId: options.requestId,
        expectedManifestRevision: options.expectedManifestRevision,
        expectedDispositionRevision: options.expectedDispositionRevision,
        state: options.state,
        reason,
      },
      true,
    );
    const result = validateDurableUserDispositionReceipt(raw, {
      requestId: options.requestId,
      companyId: options.companyId,
      artifactId: options.artifactId,
      deliveryId: options.expectedDeliveryId,
      manifestRevision: options.expectedManifestRevision,
      dispositionRevision: options.expectedDispositionRevision,
      state: options.state,
      reason,
    });
    if (!result.success) {
      throw new Error(`failed to record durable user disposition: ${validationMessage(result.issues)}`);
    }
    return result.value;
  }

  async downloadArtifactPackage(options: Readonly<{companyId: string; artifactId: string}>): Promise<Blob> {
    assertCompanyScope(options.companyId);
    assertCompanyScope(options.artifactId);
    const path = `/companies/${encodeURIComponent(options.companyId)}/artifacts/${encodeURIComponent(options.artifactId)}/download`;
    const response = await fetch(`${this.baseUrl}${path}`, {credentials: 'same-origin', headers: this.requestHeaders({'Accept': 'application/zip'})});
    observeProtectedResponse(path, response.status);
    if (!response.ok) {
      throw new Error(`failed to download artifact delivery package: HTTP ${response.status}`);
    }
    if (response.headers.get('Content-Type')?.split(';')[0] !== 'application/zip') {
      throw new Error('artifact delivery response has an unexpected content type');
    }
    const expectedDigest = response.headers.get('X-Content-SHA256');
    const manifestDigest = response.headers.get('X-Polis-Manifest-SHA256');
    if (expectedDigest === null || !/^[0-9a-f]{64}$/.test(expectedDigest) || manifestDigest === null || !/^[0-9a-f]{64}$/.test(manifestDigest)) {
      throw new Error('artifact delivery response is missing its integrity manifest');
    }
    const bytes = await response.arrayBuffer();
    if (bytes.byteLength === 0 || bytes.byteLength > 1024 * 1024) {
      throw new Error('artifact delivery package size is outside the accepted bound');
    }
    const actualDigest = await sha256Hex(new Uint8Array(bytes));
    if (actualDigest !== expectedDigest) {
      throw new Error('artifact delivery package checksum does not match');
    }
    const entries = readStoredZipEntries(new Uint8Array(bytes));
    if (entries.size !== 3 || !entries.has('artifact.bin') || !entries.has('manifest.json') || !entries.has('SHA256SUMS')) {
      throw new Error('artifact delivery package has an unexpected entry set');
    }
    const manifestBytes = entries.get('manifest.json');
    const artifactBytes = entries.get('artifact.bin');
    const checksumBytes = entries.get('SHA256SUMS');
    if (manifestBytes === undefined || artifactBytes === undefined || checksumBytes === undefined || await sha256Hex(manifestBytes) !== manifestDigest) {
      throw new Error('artifact delivery manifest checksum does not match');
    }
    let manifestRaw: unknown;
    try {
      manifestRaw = JSON.parse(new TextDecoder('utf-8', {fatal: true}).decode(manifestBytes)) as unknown;
    } catch {
      throw new Error('artifact delivery manifest is not valid UTF-8 JSON');
    }
    const manifestResult = validateArtifactDeliveryManifest({manifest: manifestRaw, manifestSha256: manifestDigest}, options.companyId, options.artifactId);
    if (!manifestResult.success) {
      throw new Error(`artifact delivery manifest is invalid: ${validationMessage(manifestResult.issues)}`);
    }
    const manifest = manifestResult.value.manifest;
    const expectedChecksums = `${manifestDigest}  manifest.json\n${manifest.content.sha256}  artifact.bin\n`;
    if (new TextDecoder('utf-8', {fatal: true}).decode(checksumBytes) !== expectedChecksums
      || artifactBytes.byteLength !== Number(manifest.content.byteSize)
      || await sha256Hex(artifactBytes) !== manifest.content.sha256) {
      throw new Error('artifact bytes do not match the embedded delivery manifest');
    }
    return new Blob([bytes], {type: 'application/zip'});
  }

  async getTaskInputManifest(options: TaskInputManifestQueryOptions): Promise<TaskInputManifestView> {
    assertCompanyScope(options.companyId);
    assertCompanyScope(options.taskId);
    const raw = await this.get(`/companies/${encodeURIComponent(options.companyId)}/tasks/${encodeURIComponent(options.taskId)}/input-manifest`);
    const result = validateTaskInputManifest(raw, options.companyId, options.taskId);
    if (!result.success) {
      throw new Error(`failed to parse Task input manifest: ${validationMessage(result.issues)}`);
    }
    return result.value;
  }

  async listProjectEnvironments(options: CompanyScopeOptions): Promise<ReadonlyArray<ProjectEnvironmentRevisionView>> {
    assertCompanyScope(options.companyId);
    const raw = await this.get(`/companies/${encodeURIComponent(options.companyId)}/environments`);
    const result = validateProjectEnvironmentRevisions(raw, options.companyId);
    if (!result.success) throw new Error(`failed to parse project environment status: ${validationMessage(result.issues)}`);
    return result.value;
  }

  async decideProjectEnvironmentPolicy(options: EnvironmentPolicyDecisionOptions): Promise<EnvironmentPolicyDecisionReceipt> {
    assertCompanyScope(options.companyId);
    assertCompanyScope(options.revisionId);
    assertRequestID(options.requestId);
    if (options.rationale.trim() === '' || options.rationale.length > 512) throw new CommandApiError('MALFORMED_INPUT', 400, 'failed to decide environment policy: rationale is invalid');
    const path = `/companies/${encodeURIComponent(options.companyId)}/environments/${encodeURIComponent(options.revisionId)}/policy`;
    const raw = await this.post(path, options.requestId, {decision: options.decision, rationale: options.rationale, requestId: options.requestId});
    const result = validateEnvironmentPolicyDecisionReceipt(raw);
    if (!result.success) throw new Error(`failed to parse environment policy receipt: ${validationMessage(result.issues)}`);
    return result.value;
  }

  async decideProjectEnvironmentExecutorQualification(options: EnvironmentExecutorQualificationOptions): Promise<EnvironmentExecutorQualificationReceipt> {
    assertCompanyScope(options.companyId);
    assertCompanyScope(options.revisionId);
    assertCompanyScope(options.evidenceInputId);
    assertRequestID(options.requestId);
    const evidenceRevision = Number(options.evidenceInputRevision);
    if (!/^[1-9]\d{0,17}$/.test(options.evidenceInputRevision) || !Number.isSafeInteger(evidenceRevision) || options.rationale.trim() === '' || options.rationale.length > 512
      || (options.qualifiedUntil !== undefined && (!/^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d+)?(?:Z|[+-]\d{2}:\d{2})$/.test(options.qualifiedUntil) || !Number.isFinite(Date.parse(options.qualifiedUntil)) || options.decision !== 'qualified'))) {
      throw new CommandApiError('MALFORMED_INPUT', 400, 'failed to decide environment executor qualification: evidence, expiry, or rationale is invalid');
    }
    const path = `/companies/${encodeURIComponent(options.companyId)}/environments/${encodeURIComponent(options.revisionId)}/executor-qualification`;
    const raw = await this.post(path, options.requestId, {
      decision: options.decision, evidenceInputId: options.evidenceInputId, evidenceInputRevision: options.evidenceInputRevision,
      qualifiedUntil: options.qualifiedUntil, rationale: options.rationale, requestId: options.requestId,
    });
    const result = validateEnvironmentExecutorQualificationReceipt(raw);
    if (!result.success) throw new Error(`failed to parse environment executor qualification receipt: ${validationMessage(result.issues)}`);
    return result.value;
  }

  async ensureProjectEnvironment(options: EnsureEnvironmentOptions): Promise<EnvironmentPreparationRunView> {
    assertCompanyScope(options.companyId);
    assertCompanyScope(options.revisionId);
    assertRequestID(options.requestId);
    const path = `/companies/${encodeURIComponent(options.companyId)}/environments/${encodeURIComponent(options.revisionId)}/preparation`;
    const raw = await this.post(path, options.requestId, {requestId: options.requestId});
    const result = validateEnvironmentPreparationRun(raw, options.companyId, options.revisionId);
    if (!result.success) throw new Error(`failed to parse environment preparation receipt: ${validationMessage(result.issues)}`);
    return result.value;
  }

  async listTaskJobRuns(options: TaskJobRunsQueryOptions): Promise<ReadonlyArray<JobRunView>> {
    assertCompanyScope(options.companyId);
    assertCompanyScope(options.taskId);
    const raw = await this.get(`/companies/${encodeURIComponent(options.companyId)}/tasks/${encodeURIComponent(options.taskId)}/jobs`);
    const result = validateTaskJobRuns(raw, options.companyId, options.taskId);
    if (!result.success) throw new Error(`failed to parse Task JobRuns: ${validationMessage(result.issues)}`);
    return result.value;
  }

  async listTaskCrossBackendHandovers(options: TaskCrossBackendHandoversQueryOptions): Promise<ReadonlyArray<CrossBackendHandoverView>> {
    assertCompanyScope(options.companyId);
    assertCompanyScope(options.taskId);
    const raw = await this.get(`/companies/${encodeURIComponent(options.companyId)}/tasks/${encodeURIComponent(options.taskId)}/environment-handovers`);
    const result = validateTaskCrossBackendHandovers(raw, options.companyId, options.taskId);
    if (!result.success) throw new Error(`failed to parse Task environment handovers: ${validationMessage(result.issues)}`);
    return result.value;
  }

  async createTaskEnvironmentHandover(options: CreateTaskEnvironmentHandoverOptions): Promise<CrossBackendHandoverView> {
    assertCompanyScope(options.companyId);
    assertCompanyScope(options.taskId);
    assertCompanyScope(options.sourceJobId);
    assertCompanyScope(options.targetEnvironmentRevisionId);
    assertRequestID(options.requestId);
    const raw = await this.post(
      `/companies/${encodeURIComponent(options.companyId)}/tasks/${encodeURIComponent(options.taskId)}/environment-handovers`,
      options.requestId,
      {sourceJobId: options.sourceJobId, targetEnvironmentRevisionId: options.targetEnvironmentRevisionId, requestId: options.requestId},
    );
    const result = validateTaskCrossBackendHandovers([raw], options.companyId, options.taskId);
    if (!result.success || result.value.length !== 1) throw new Error(`failed to parse Task environment handover: ${validationMessage(result.success ? [{path: '', message: 'handover record is missing'}] : result.issues)}`);
    if (result.value[0].sourceJobId !== options.sourceJobId || result.value[0].targetEnvironmentRevisionId !== options.targetEnvironmentRevisionId || result.value[0].requestId !== options.requestId) {
      throw new Error('failed to parse Task environment handover: response does not match request');
    }
    return result.value[0];
  }

  async startTaskJobRun(options: StartTaskJobRunOptions): Promise<JobRunCommandReceipt> {
    assertCompanyScope(options.companyId);
    assertCompanyScope(options.taskId);
    if (options.sessionId !== undefined && options.sessionId !== '') assertCompanyScope(options.sessionId);
    assertCompanyScope(options.environmentRevisionId);
    if (options.handoverId !== undefined) assertCompanyScope(options.handoverId);
    assertRequestID(options.requestId);
    if (options.kind === 'service') assertCompanyScope(options.serviceId);
    const handover = options.handoverId === undefined ? {} : {handoverId: options.handoverId};
    const command = options.kind === 'service'
      ? {
        taskId: options.taskId, sessionId: options.sessionId, environmentRevisionId: options.environmentRevisionId,
        ...handover, kind: 'service', serviceId: options.serviceId, requestId: options.requestId,
      }
      : {
        taskId: options.taskId, sessionId: options.sessionId, environmentRevisionId: options.environmentRevisionId,
        ...handover, kind: 'batch', scriptPath: options.scriptPath, args: options.args, requestId: options.requestId,
      };
    const raw = await this.post(`/companies/${encodeURIComponent(options.companyId)}/tasks/${encodeURIComponent(options.taskId)}/jobs`, options.requestId, command);
    const result = validateJobRunCommandReceipt(raw, options.companyId, options.taskId);
    if (!result.success) throw new Error(`failed to parse JobRun start receipt: ${validationMessage(result.issues)}`);
    return result.value;
  }

  async stopTaskJobRun(options: StopTaskJobRunOptions): Promise<JobRunCommandReceipt> {
    assertCompanyScope(options.companyId);
    assertCompanyScope(options.jobId);
    assertRequestID(options.requestId);
    const raw = await this.post(`/companies/${encodeURIComponent(options.companyId)}/jobs/${encodeURIComponent(options.jobId)}/stop`, options.requestId, {requestId: options.requestId});
    const result = validateJobRunCommandReceipt(raw, options.companyId);
    if (!result.success) throw new Error(`failed to parse JobRun stop receipt: ${validationMessage(result.issues)}`);
    return result.value;
  }

  async getTaskJobLogs(options: TaskJobLogsQueryOptions): Promise<JobRunLogArtifactView> {
    assertCompanyScope(options.companyId);
    assertCompanyScope(options.jobId);
    const raw = await this.get(`/companies/${encodeURIComponent(options.companyId)}/jobs/${encodeURIComponent(options.jobId)}/logs`);
    const result = validateJobRunLogArtifact(raw, options.companyId, options.jobId);
    if (!result.success) throw new Error(`failed to parse JobRun logs: ${validationMessage(result.issues)}`);
    return result.value;
  }

  async createProjectJobBrowserSession(options: CreateProjectJobBrowserSessionOptions): Promise<ServiceBrowserSessionView> {
    assertCompanyScope(options.companyId);
    assertCompanyScope(options.jobId);
    assertRequestID(options.requestId);
    const raw = await this.post(`/companies/${encodeURIComponent(options.companyId)}/jobs/${encodeURIComponent(options.jobId)}/browser-session`, options.requestId, {requestId: options.requestId});
    const result = validateServiceBrowserSession(raw);
    if (!result.success) throw new Error(`failed to parse service browser session: ${validationMessage(result.issues)}`);
    return result.value;
  }

  async uploadMissionDirectoryInput(options: UploadMissionDirectoryInputOptions): Promise<MissionInputCommandReceipt> {
    assertCompanyScope(options.companyId);
    assertCompanyScope(options.missionId);
    if (options.inputId !== null) assertCompanyScope(options.inputId);
    assertRequestID(options.requestId);
    if (options.files.length === 0 || options.files.length > MAX_MISSION_DIRECTORY_FILES) {
      throw new CommandApiError('MALFORMED_INPUT', 400, 'failed to upload mission directory: select 1 to 250 files', options.missionId);
    }
    let totalBytes = 0;
    for (const entry of options.files) {
      if (entry.relativePath.trim() === '' || entry.file.size === 0 || entry.file.size > MAX_MISSION_INPUT_BYTES) {
        throw new CommandApiError('MALFORMED_INPUT', 400, 'failed to upload mission directory: an entry is empty or invalid', options.missionId);
      }
      totalBytes += entry.file.size;
      if (totalBytes > MAX_MISSION_DIRECTORY_BYTES) {
        throw new CommandApiError('MALFORMED_INPUT', 413, 'failed to upload mission directory: files exceed 7 MB', options.missionId);
      }
    }
    const body = new FormData();
    body.set('requestId', options.requestId);
    if (options.inputId !== null) body.set('inputId', options.inputId);
    for (const entry of options.files) {
      body.append('paths', entry.relativePath);
      body.append('files', entry.file, entry.file.name);
    }
    const raw = await this.postMultipart(`/companies/${encodeURIComponent(options.companyId)}/missions/${encodeURIComponent(options.missionId)}/inputs/directory`, options.requestId, body);
    const result = validateMissionInputCommandReceipt(raw, options.companyId, options.missionId, options.requestId);
    if (!result.success) {
      throw new Error(`failed to parse mission directory receipt: ${validationMessage(result.issues)}`);
    }
    return result.value;
  }

  async createMission(options: CreateMissionOptions): Promise<MissionCommandReceipt> {
    assertCompanyScope(options.companyId);
    assertRequestID(options.requestId);
    if (!Number.isSafeInteger(options.protocolToolCallLimit) || Number(options.protocolToolCallLimit) < 1) {
      throw new Error('Mission protocol tool-call limit must be a positive safe integer');
    }
    const raw = await this.post(`/companies/${encodeURIComponent(options.companyId)}/missions`, options.requestId, {title: options.title, goal: options.goal, acceptanceContract: options.acceptanceContract, protocolToolCallLimit: options.protocolToolCallLimit, requestId: options.requestId});
    return validateCommandReceipt(raw, 'mission.create');
  }

  async createDailyRoutine(options: CreateDailyRoutineOptions): Promise<DailyRoutineCommandReceipt> {
    assertCompanyScope(options.companyId);
    assertCompanyScope(options.missionId);
    assertCompanyScope(options.routineId);
    assertCompanyScope(options.employeeId);
    assertRequestID(options.requestId);
    const raw = await this.post(`/companies/${encodeURIComponent(options.companyId)}/missions/${encodeURIComponent(options.missionId)}/routines`, options.requestId, {
      routineId: options.routineId,
      employeeId: options.employeeId,
      taskInstruction: options.taskInstruction,
      timezone: options.timezone,
      localTime: options.localTime,
      nextLogicalDay: options.nextLogicalDay,
      catchUpPolicy: options.catchUpPolicy,
      maxCatchUp: options.maxCatchUp,
      requestId: options.requestId,
    });
    const result = validateDailyRoutineCommandReceipt(raw, 'routine.daily.create', options.routineId, options.requestId);
    if (!result.success) {
      throw new Error(`failed to parse daily Routine creation receipt: ${validationMessage(result.issues)}`);
    }
    return result.value;
  }

  async setDailyRoutineTaskInstruction(options: SetDailyRoutineTaskInstructionOptions): Promise<DailyRoutineCommandReceipt> {
    assertCompanyScope(options.companyId);
    assertCompanyScope(options.missionId);
    assertCompanyScope(options.routineId);
    assertRequestID(options.requestId);
    const raw = await this.post(`/companies/${encodeURIComponent(options.companyId)}/missions/${encodeURIComponent(options.missionId)}/routines/${encodeURIComponent(options.routineId)}/instruction`, options.requestId, {
      taskInstruction: options.taskInstruction,
      requestId: options.requestId,
    });
    const result = validateDailyRoutineCommandReceipt(raw, 'routine.daily.instruction', options.routineId, options.requestId);
    if (!result.success) {
      throw new Error(`failed to parse daily Routine instruction receipt: ${validationMessage(result.issues)}`);
    }
    return result.value;
  }

  async startMission(options: MissionCommandOptions): Promise<MissionCommandReceipt> {
    assertCompanyScope(options.companyId);
    assertCompanyScope(options.missionId);
    assertRequestID(options.requestId);
    const raw = await this.post(`/companies/${encodeURIComponent(options.companyId)}/missions/${encodeURIComponent(options.missionId)}/start`, options.requestId, {requestId: options.requestId});
    return validateCommandReceipt(raw, 'mission.start');
  }

  async pauseMission(options: MissionCommandOptions): Promise<MissionCommandReceipt> {
    assertCompanyScope(options.companyId);
    assertCompanyScope(options.missionId);
    assertRequestID(options.requestId);
    const raw = await this.post(`/companies/${encodeURIComponent(options.companyId)}/missions/${encodeURIComponent(options.missionId)}/pause`, options.requestId, {requestId: options.requestId});
    return validateCommandReceipt(raw, 'mission.pause');
  }

  async resumeMission(options: MissionCommandOptions): Promise<MissionCommandReceipt> {
    assertCompanyScope(options.companyId);
    assertCompanyScope(options.missionId);
    assertRequestID(options.requestId);
    const raw = await this.post(`/companies/${encodeURIComponent(options.companyId)}/missions/${encodeURIComponent(options.missionId)}/resume`, options.requestId, {requestId: options.requestId});
    return validateCommandReceipt(raw, 'mission.resume');
  }

  async cancelMission(options: MissionCommandOptions): Promise<MissionCommandReceipt> {
    assertCompanyScope(options.companyId);
    assertCompanyScope(options.missionId);
    assertRequestID(options.requestId);
    const raw = await this.post(`/companies/${encodeURIComponent(options.companyId)}/missions/${encodeURIComponent(options.missionId)}/cancel`, options.requestId, {requestId: options.requestId});
    return validateCommandReceipt(raw, 'mission.cancel');
  }

  async closeMission(options: MissionCloseoutOptions): Promise<MissionCommandReceipt> {
    assertCompanyScope(options.companyId);
    assertCompanyScope(options.missionId);
    assertRequestID(options.requestId);
    const raw = await this.post(`/companies/${encodeURIComponent(options.companyId)}/missions/${encodeURIComponent(options.missionId)}/closeout`, options.requestId, {
      outcome: options.outcome,
      rationale: options.rationale,
      acceptanceArtifactIds: options.acceptanceArtifactIds,
      requestId: options.requestId,
    });
    return validateCommandReceipt(raw, 'mission.closeout');
  }

  async setHumanInterventionState(options: SetHumanInterventionStateOptions): Promise<HumanInterventionCommandReceipt> {
    assertCompanyScope(options.companyId);
    assertCompanyScope(options.interventionId);
    assertRequestID(options.requestId);
    const action = options.state === 'acknowledged' ? 'acknowledge' : 'resolve';
    const raw = await this.post(`/companies/${encodeURIComponent(options.companyId)}/human-interventions/${encodeURIComponent(options.interventionId)}/${action}`, options.requestId, {requestId: options.requestId});
    const result = validateHumanInterventionCommandReceipt(raw, options.state);
    if (!result.success) {
      throw new Error(`failed to parse human intervention command receipt: ${validationMessage(result.issues)}`);
    }
    return result.value;
  }

  subscribeToActivityEvents(options: ActivityStreamOptions, listener: ActivityEventListener, onStatus?: ActivityStreamStatusListener): () => void {
    assertCompanyScope(options.companyId);
    if (options.cursor === '') {
      throw new Error('failed to subscribe to activity stream: snapshot cursor is required');
    }
    return this.stream.subscribe(options, listener, onStatus);
  }

  private async get(path: string): Promise<unknown> {
    const response = await fetch(`${this.baseUrl}${path}`, {credentials: 'same-origin', headers: this.requestHeaders({'Accept': 'application/json'})});
    observeProtectedResponse(path, response.status);
    if (!response.ok) {
      let detail = '';
      try {
        const body: unknown = await response.json();
        if (typeof body === 'object' && body !== null) {
          const error = (body as {error?: unknown}).error;
          if (typeof error === 'string' && error.trim() !== '') detail = error.trim();
        }
      } catch {
        detail = '';
      }
      throw new Error(detail === '' ? `failed to read workbench API: http ${response.status}` : `failed to read workbench API: ${detail}`);
    }
    return response.json() as Promise<unknown>;
  }

  private async post(path: string, requestID: string, body: unknown, requireOwnerCSRF = false): Promise<unknown> {
    const headers = this.requestHeaders({'Accept': 'application/json', 'Content-Type': 'application/json', 'X-Request-ID': requestID});
    if (requireOwnerCSRF) {
      const csrf = this.ownerCSRFCookie();
      if (csrf !== '') headers['X-Polis-CSRF-Token'] = csrf;
    }
    const response = await fetch(`${this.baseUrl}${path}`, {
      method: 'POST',
      credentials: 'same-origin',
      headers,
      body: JSON.stringify(body),
    });
    observeProtectedResponse(path, response.status);
    if (!response.ok) {
      let raw: unknown = null;
      try {
        raw = await response.json();
      } catch {
        raw = null;
      }
      const code = isRecord(raw) && typeof raw.code === 'string' && raw.code !== '' ? raw.code : 'COMMAND_FAILED';
      const message = isRecord(raw) && typeof raw.error === 'string' && raw.error !== '' ? raw.error : `failed to write workbench API: http ${response.status}`;
      const targetID = isRecord(raw) && typeof raw.targetId === 'string' && raw.targetId !== '' ? raw.targetId : null;
      const currentState = isRecord(raw) && typeof raw.currentState === 'string' && raw.currentState !== '' ? raw.currentState : null;
      throw new CommandApiError(code, response.status, message, targetID, currentState);
    }
    return response.json() as Promise<unknown>;
  }

  private async postMultipart(path: string, requestID: string, body: FormData, failureLabel = 'failed to upload mission input'): Promise<unknown> {
    const response = await fetch(`${this.baseUrl}${path}`, {
      method: 'POST',
      credentials: 'same-origin',
      headers: this.requestHeaders({'Accept': 'application/json', 'X-Request-ID': requestID}),
      body,
    });
    observeProtectedResponse(path, response.status);
    if (!response.ok) {
      let raw: unknown = null;
      try {
        raw = await response.json();
      } catch {
        raw = null;
      }
      const code = isRecord(raw) && typeof raw.code === 'string' && raw.code !== '' ? raw.code : 'COMMAND_FAILED';
      const message = isRecord(raw) && typeof raw.error === 'string' && raw.error !== '' ? raw.error : `${failureLabel}: http ${response.status}`;
      const targetID = isRecord(raw) && typeof raw.targetId === 'string' && raw.targetId !== '' ? raw.targetId : null;
      const currentState = isRecord(raw) && typeof raw.currentState === 'string' && raw.currentState !== '' ? raw.currentState : null;
      throw new CommandApiError(code, response.status, message, targetID, currentState);
    }
    return response.json() as Promise<unknown>;
  }

  private requestHeaders(headers: Record<string, string>): Record<string, string> {
    return this.sessionToken === null ? headers : {...headers, 'X-Polis-Desktop-Token': this.sessionToken};
  }

  private ownerCSRFCookie(): string {
    if (typeof document === 'undefined') return '';
    const prefix = 'polis_owner_csrf=';
    for (const item of document.cookie.split(';')) {
      const cookie = item.trim();
      if (cookie.startsWith(prefix)) return cookie.slice(prefix.length);
    }
    return '';
  }
}

function assertRequestID(requestID: string): void {
  if (!/^[a-zA-Z0-9_-]{1,80}$/.test(requestID)) {
    throw new CommandApiError('MALFORMED_INPUT', 400, 'failed to write workbench command: request id is invalid');
  }
}

function validateCommandReceipt(raw: unknown, expectedType: MissionCommandReceipt['commandType']): MissionCommandReceipt {
  const result = validateMissionCommandReceipt(raw);
  if (!result.success) {
    throw new Error(`failed to parse command receipt: ${validationMessage(result.issues)}`);
  }
  if (result.value.commandType !== expectedType) {
    throw new Error(`failed to parse command receipt: expected ${expectedType}`);
  }
  return result.value;
}

function validateCompanyCommandReceipt(raw: unknown, expectedType: CompanyCommandReceipt['commandType']): CompanyCommandReceipt {
  if (!isRecord(raw) || typeof raw.commandId !== 'string' || raw.commandType !== expectedType || raw.targetType !== 'company' || typeof raw.targetId !== 'string' || typeof raw.requestId !== 'string' || raw.accepted !== true || typeof raw.acceptedAt !== 'string' || !['active', 'archived', 'restart_required', 'configured', 'unverified', 'ready', 'revoked'].includes(String(raw.resultingState))) {
    throw new Error('failed to parse company command receipt: response is malformed');
  }
  return raw as CompanyCommandReceipt;
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

async function sha256Hex(bytes: Uint8Array): Promise<string> {
  const copy = new Uint8Array(bytes.byteLength);
  copy.set(bytes);
  const digest = await crypto.subtle.digest('SHA-256', copy.buffer);
  return Array.from(new Uint8Array(digest), value => value.toString(16).padStart(2, '0')).join('');
}

function readStoredZipEntries(bytes: Uint8Array): ReadonlyMap<string, Uint8Array> {
  const view = new DataView(bytes.buffer, bytes.byteOffset, bytes.byteLength);
  const entries = new Map<string, Uint8Array>();
  const localMetadata = new Map<string, Readonly<{crc: number; size: number; flags: number; offset: number}>>();
  let offset = 0;
  while (offset + 4 <= bytes.byteLength) {
    const signature = view.getUint32(offset, true);
    if (signature === 0x02014b50) break;
    if (signature === 0x06054b50) throw new Error('artifact delivery ZIP is missing its central directory');
    if (signature !== 0x04034b50 || offset + 30 > bytes.byteLength || entries.size >= 8) {
      throw new Error('artifact delivery ZIP structure is invalid');
    }
    const flags = view.getUint16(offset + 6, true);
    const method = view.getUint16(offset + 8, true);
    const expectedCRC = view.getUint32(offset + 14, true);
    const compressedSize = view.getUint32(offset + 18, true);
    const uncompressedSize = view.getUint32(offset + 22, true);
    const nameLength = view.getUint16(offset + 26, true);
    const extraLength = view.getUint16(offset + 28, true);
    if ((flags & 0x0008) !== 0 || method !== 0 || compressedSize !== uncompressedSize || nameLength < 1 || nameLength > 256) {
      throw new Error('artifact delivery ZIP entry uses an unsupported encoding');
    }
    const nameStart = offset + 30;
    const dataStart = nameStart + nameLength + extraLength;
    const dataEnd = dataStart + compressedSize;
    if (dataEnd > bytes.byteLength) throw new Error('artifact delivery ZIP entry is truncated');
    const name = new TextDecoder('utf-8', {fatal: true}).decode(bytes.subarray(nameStart, nameStart + nameLength));
    if (name.startsWith('/') || name.includes('\\') || name.split('/').some(part => part === '..') || entries.has(name)) {
      throw new Error('artifact delivery ZIP contains an unsafe or duplicate path');
    }
    const content = bytes.subarray(dataStart, dataEnd);
    if (crc32(content) !== expectedCRC) throw new Error('artifact delivery ZIP entry checksum does not match');
    entries.set(name, content);
    localMetadata.set(name, {crc: expectedCRC, size: uncompressedSize, flags, offset});
    offset = dataEnd;
  }
  const centralDirectoryOffset = offset;
  const centralNames = new Set<string>();
  while (offset + 4 <= bytes.byteLength && view.getUint32(offset, true) === 0x02014b50) {
    if (offset + 46 > bytes.byteLength) throw new Error('artifact delivery ZIP central directory is truncated');
    const flags = view.getUint16(offset + 8, true);
    const method = view.getUint16(offset + 10, true);
    const expectedCRC = view.getUint32(offset + 16, true);
    const compressedSize = view.getUint32(offset + 20, true);
    const uncompressedSize = view.getUint32(offset + 24, true);
    const nameLength = view.getUint16(offset + 28, true);
    const extraLength = view.getUint16(offset + 30, true);
    const commentLength = view.getUint16(offset + 32, true);
    const localHeaderOffset = view.getUint32(offset + 42, true);
    const nameStart = offset + 46;
    const recordEnd = nameStart + nameLength + extraLength + commentLength;
    if ((flags & 0x0008) !== 0 || method !== 0 || compressedSize !== uncompressedSize || nameLength < 1 || nameLength > 256 || recordEnd > bytes.byteLength) {
      throw new Error('artifact delivery ZIP central entry is invalid');
    }
    const name = new TextDecoder('utf-8', {fatal: true}).decode(bytes.subarray(nameStart, nameStart + nameLength));
    const local = localMetadata.get(name);
    if (local === undefined || local.offset !== localHeaderOffset || local.crc !== expectedCRC || local.size !== uncompressedSize || local.flags !== flags || centralNames.has(name)) {
      throw new Error('artifact delivery ZIP central entries do not match their local headers');
    }
    centralNames.add(name);
    offset = recordEnd;
  }
  if (offset + 22 > bytes.byteLength || view.getUint32(offset, true) !== 0x06054b50) {
    throw new Error('artifact delivery ZIP is missing its end-of-central-directory record');
  }
  const diskNumber = view.getUint16(offset + 4, true);
  const centralDisk = view.getUint16(offset + 6, true);
  const entriesOnDisk = view.getUint16(offset + 8, true);
  const totalEntries = view.getUint16(offset + 10, true);
  const centralSize = view.getUint32(offset + 12, true);
  const centralOffset = view.getUint32(offset + 16, true);
  const commentLength = view.getUint16(offset + 20, true);
  if (diskNumber !== 0 || centralDisk !== 0 || entriesOnDisk !== entries.size || totalEntries !== entries.size
    || centralOffset !== centralDirectoryOffset || centralSize !== offset - centralDirectoryOffset
    || centralNames.size !== entries.size || offset + 22 + commentLength !== bytes.byteLength) {
    throw new Error('artifact delivery ZIP directory summary is inconsistent');
  }
  return entries;
}

function crc32(bytes: Uint8Array): number {
  let crc = 0xffffffff;
  for (const byte of bytes) {
    crc ^= byte;
    for (let bit = 0; bit < 8; bit++) crc = (crc >>> 1) ^ ((crc & 1) === 1 ? 0xedb88320 : 0);
  }
  return (crc ^ 0xffffffff) >>> 0;
}

class EventStreamClient {
  private source: EventSource | null = null;

  constructor(private readonly baseUrl: string, private readonly enabled: boolean, private readonly sessionToken: string | null) {}

  subscribe(options: ActivityStreamOptions, listener: ActivityEventListener, onStatus?: ActivityStreamStatusListener): () => void {
    this.close();
    if (!this.enabled) {
      onStatus?.('closed');
      return () => undefined;
    }
    onStatus?.('connecting');
    let lastSeenSequence = streamSequence(options.cursor);
    const params = new URLSearchParams();
    params.set('cursor', options.cursor);
    if (this.sessionToken !== null) {
      params.set('desktop_token', this.sessionToken);
    }
    const path = `${this.baseUrl}/companies/${encodeURIComponent(options.companyId)}/stream?${params.toString()}`;
    const authorizationScope = `${this.baseUrl}:stream:${options.companyId}`;
    const source = new EventSource(path, {withCredentials: true});
    source.onopen = () => {
      observeProtectedResponse(authorizationScope, 200);
      onStatus?.('open');
    };
    source.onerror = () => {
      if (source.readyState === EventSource.CLOSED) {
        invalidateProtectedScope(authorizationScope);
        onStatus?.('incompatible');
        return;
      }
      onStatus?.('reconnecting');
    };
    source.addEventListener('activity', event => {
      const message = event as MessageEvent<string>;
      try {
        const parsed: unknown = JSON.parse(message.data);
        const result = validateActivityEvent(parsed);
        if (result.success) {
          const sequence = streamSequence(result.value.companySeq);
          if (sequence !== null && lastSeenSequence !== null && sequence <= lastSeenSequence) {
            return;
          }
          if (sequence !== null) {
            lastSeenSequence = sequence;
          }
          listener(result.value);
        } else {
          if (this.source === source) {
            source.close();
            this.source = null;
          }
          onStatus?.('incompatible');
        }
      } catch {
        if (this.source === source) {
          source.close();
          this.source = null;
        }
        onStatus?.('incompatible');
      }
    });
    this.source = source;
    return () => {
      if (this.source === source) {
        this.close();
        onStatus?.('closed');
      }
    };
  }

  private close(): void {
    this.source?.close();
    this.source = null;
  }
}

function streamSequence(cursor: string): number | null {
  const value = cursor.startsWith('company-seq:') ? cursor.slice('company-seq:'.length) : cursor;
  if (!/^\d+$/.test(value)) return null;
  const sequence = Number(value);
  return Number.isSafeInteger(sequence) ? sequence : null;
}

// pattern: Functional Core

import type {
  ActivityEvent,
  ActivityEventKind,
  ActivityView,
  CheckpointSummary,
  ArtifactSummary,
  AttentionItem,
  AcceptanceContract,
  CompanyOverviewView,
  CompanySummaryView,
  CapabilityCatalogView,
  CapabilityDecisionView,
  CapabilityRevocationMCPCallView,
  CapabilityRevocationSessionView,
  CapabilityRevocationStatusView,
  CapabilityQualificationView,
  StdioMCPPackageFileView,
  StdioMCPPackageManifestView,
  StdioMCPPackageRevisionView,
  StdioMCPRuntimeQualificationView,
  DomainEvidenceArtifactPreviewManifestView,
  DomainEvidenceAreaAssessmentView,
  DomainEvidenceAreaView,
  DomainEvidenceLedgerView,
  DomainEvidencePreviewAttestationView,
  DomainEvidenceRecordView,
  DomainEvidenceReviewRecordView,
  DomainEvidenceSubstantiveAssessmentRecordView,
  DomainProfileQualificationRecordView,
  DomainContentSourceEventView,
  DomainContentDraftView,
  DomainContentReviewView,
  DomainContentSourceReferenceView,
  DomainContentClaimReviewView,
  DomainContentSampleEvidenceView,
  DomainContentReviewSubmissionView,
  DomainContentPublicationView,
  DomainContentCorrectionView,
  DomainContentFeedbackView,
  ResearchSimulationRunView,
  DomainEvidenceSubstantiveOutcomeView,
  DomainEvidenceSubmissionView,
  DomainWorkflowProfileView,
  EnvironmentPolicyDecisionReceipt,
  EnvironmentExecutorQualificationReceipt,
  EnvironmentPreparationRunView,
  EmployeeCapabilityBindingView,
  CodexModelCatalogView,
  CodexModelOptionView,
  RuntimeSettingsView,
  OperatorInstructionReceipt,
  OperatorInstructionResponseView,
  OperatorInstructionView,
  CollaborationItem,
  ArtifactDeliveryManifestResponse,
  DurableDeliveryFeedbackBacklogEventView,
  DurableDeliveryManifestCompletionReceipt,
  DurableDeliveryManifestInvalidationReceipt,
  DurableDeliveryResponse,
  DurableUserDispositionCommandReceipt,
  UserDispositionDecision,
  ArtifactDetailView,
  WorkspaceView,
  OperationsView,
  NotificationsView,
  CompanyFeedbackView,
  GitHubCredentialReceipt,
  GitHubFeedbackBacklogStatus,
  GitHubFeedbackBacklogStatusReceipt,
  GitHubFeedbackCollectionPolicyReceipt,
  GitHubFeedbackSourceCommandReceipt,
  GitHubFeedbackProbeReceipt,
  GitHubFeedbackPollReceipt,
  ContractRevisionSummary,
  EmployeeSummary,
  ObligationSummary,
  ResourceSummary,
  StatusKey,
  StatusTone,
  TaskSummary,
  ViewMeta,
  MissionCommandReceipt,
  MissionCommandType,
  MissionCommandResultState,
  HumanInterventionCommandReceipt,
  HumanInterventionState,
  JobRunCommandReceipt,
  JobRunLogArtifactView,
  ServiceBrowserSessionView,
  JobRunView,
  CrossBackendHandoverView,
  MissionChangePlanningAssessmentView,
  MissionChangeRequestView,
  TaskTakeoverDiffSummaryView,
  TaskTakeoverLeaseView,
  TaskTakeoverWorkspaceFileView,
  TaskTakeoverWorkspaceManifestEntryView,
  TaskTakeoverWorkspaceManifestView,
  TaskTakeoverWorkspaceTreeBindingView,
  ProjectEnvironmentRevisionView,
  ProjectEnvironmentPolicyManifestView,
  ProjectServiceDefinitionView,
  ServiceEndpointView,
} from './workbench';
import type {MissionInputCommandReceipt, MissionInputSourceKind, MissionInputState, MissionInputView, ModelInputDeliveryRef, ModelInputExclusion, TaskInputManifestView} from './mission-input';

export type ValidationIssue = Readonly<{
  path: string;
  message: string;
}>;

export type ValidationResult<T> =
  | Readonly<{success: true; value: T}>
  | Readonly<{success: false; issues: ReadonlyArray<ValidationIssue>}>;

const MAX_TASK_INPUT_DELIVERY_REFS = 12;
const ID_PATTERN = /^[A-Za-z0-9][A-Za-z0-9_-]{0,79}$/;
const MAX_INT64_DECIMAL = '9223372036854775807';

function isCanonicalPositiveInt64(value: unknown): value is string {
  return typeof value === 'string' && /^[1-9]\d{0,18}$/.test(value)
    && (value.length < MAX_INT64_DECIMAL.length || value <= MAX_INT64_DECIMAL);
}

const ACTIVITY_EVENT_KINDS: ReadonlyArray<ActivityEventKind> = [
  'mission_created', 'mission_started', 'mission_cancelled', 'provider_turn_completed', 'provider_turn_failed', 'provider_runtime_initialization_failed', 'provider_runtime_thread_start_failed',
  'employee_started_task', 'contract_revision_proposed', 'contract_revision_accepted', 'peer_message_sent',
  'obligation_created', 'obligation_observed', 'workspace_updated', 'checkpoint_saved', 'employee_stopped',
  'successor_resumed', 'old_writer_rejected', 'artifact_submitted', 'acceptance_passed',
];

const STATUS_KEYS: ReadonlyArray<StatusKey> = [
  'working', 'sleeping', 'wake_pending', 'waiting_tool', 'waiting_peer', 'waiting_external',
  'waiting_quota', 'handover', 'paused', 'lost_contact', 'stopped',
];

const STATUS_TONES: ReadonlyArray<StatusTone> = ['info', 'neutral', 'success', 'warning', 'danger'];
const MISSION_COMMAND_TYPES: ReadonlyArray<MissionCommandType> = ['mission.create', 'mission.start', 'mission.pause', 'mission.resume', 'mission.cancel', 'mission.closeout'];
const MISSION_COMMAND_STATES: ReadonlyArray<MissionCommandResultState> = ['draft', 'active', 'paused', 'closing', 'succeeded', 'ended_not_met', 'cancelled'];

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

function isOneOf<T extends string>(value: unknown, values: ReadonlyArray<T>): value is T {
  return typeof value === 'string' && values.includes(value as T);
}

function hasString(record: Record<string, unknown>, key: string): boolean {
  return typeof record[key] === 'string' && record[key] !== '';
}

function isNullableString(value: unknown): value is string | null {
  return value === null || typeof value === 'string';
}


const MISSION_INPUT_STATES: ReadonlyArray<MissionInputState> = ['uploading', 'stored', 'usable', 'partial', 'unsupported', 'rejected'];
const MISSION_INPUT_SOURCES: ReadonlyArray<MissionInputSourceKind> = ['upload', 'directory_snapshot', 'zip_snapshot', 'pdf_snapshot', 'git_snapshot'];

function isMissionInputView(value: unknown, companyId: string, missionId: string): value is MissionInputView {
  if (!isRecord(value)) return false;
  const revision = typeof value.revision === 'string' ? Number(value.revision) : Number.NaN;
  const byteSize = typeof value.byteSize === 'string' ? Number(value.byteSize) : Number.NaN;
  const imageWidth = typeof value.imageWidth === 'string' ? Number(value.imageWidth) : Number.NaN;
  const imageHeight = typeof value.imageHeight === 'string' ? Number(value.imageHeight) : Number.NaN;
  return value.companyId === companyId
    && value.missionId === missionId
    && hasString(value, 'inputId')
    && hasString(value, 'requestId')
    && Number.isSafeInteger(revision) && revision > 0
    && isOneOf(value.sourceKind, MISSION_INPUT_SOURCES)
    && hasString(value, 'displayName')
    && hasString(value, 'mediaType')
    && Number.isSafeInteger(byteSize) && byteSize > 0
    && typeof value.contentDigest === 'string' && /^[0-9a-f]{64}$/.test(value.contentDigest)
    && isOneOf(value.state, MISSION_INPUT_STATES)
    && Number.isSafeInteger(imageWidth) && imageWidth >= 0
    && Number.isSafeInteger(imageHeight) && imageHeight >= 0
    && hasString(value, 'createdAt');
}

function isMissionInputCommandReceipt(value: unknown, companyId: string, missionId: string, requestId: string): value is MissionInputCommandReceipt {
  return isRecord(value)
    && value.companyId === companyId
    && hasString(value, 'inputId')
    && value.missionId === missionId
    && typeof value.revision === 'number' && Number.isSafeInteger(value.revision) && value.revision > 0
    && value.requestId === requestId
    && isOneOf(value.sourceKind, MISSION_INPUT_SOURCES)
    && hasString(value, 'displayName')
    && hasString(value, 'mediaType')
    && typeof value.byteSize === 'number' && Number.isSafeInteger(value.byteSize) && value.byteSize > 0
    && typeof value.contentDigest === 'string' && /^[0-9a-f]{64}$/.test(value.contentDigest)
    && isOneOf(value.state, MISSION_INPUT_STATES);
}

function isModelInputManifestEntry(value: unknown, candidate: boolean): boolean {
  return isRecord(value)
    && hasString(value, 'inputId')
    && typeof value.revision === 'number' && Number.isSafeInteger(value.revision) && value.revision > 0
    && hasString(value, 'requestId')
    && isOneOf(value.sourceKind, MISSION_INPUT_SOURCES)
    && hasString(value, 'displayName')
    && hasString(value, 'mediaType')
    && typeof value.byteSize === 'number' && Number.isSafeInteger(value.byteSize) && value.byteSize > 0
    && typeof value.contentDigest === 'string' && /^[0-9a-f]{64}$/.test(value.contentDigest)
    && isOneOf(value.state, MISSION_INPUT_STATES)
	&& (candidate ? value.state === 'usable' || ((value.sourceKind === 'directory_snapshot' || value.sourceKind === 'zip_snapshot' || value.sourceKind === 'pdf_snapshot' || value.sourceKind === 'git_snapshot') && value.state === 'partial') || (value.sourceKind === 'upload' && value.state === 'partial' && (value.mediaType === 'image/png' || value.mediaType === 'image/jpeg')) : value.state !== 'usable');
}

function isModelInputDeliveryRef(value: unknown): value is ModelInputDeliveryRef {
  if (!isRecord(value) || !hasString(value, 'inputId') || typeof value.relativePath !== 'string' || !hasString(value, 'mediaType')
    || typeof value.byteSize !== 'number' || !Number.isSafeInteger(value.byteSize) || value.byteSize <= 0
    || typeof value.contentDigest !== 'string' || !/^[0-9a-f]{64}$/.test(value.contentDigest)) return false;
  const mediaType = value.mediaType;
  if (typeof mediaType !== 'string') return false;
  if (mediaType === 'image/png' || mediaType === 'image/jpeg') return value.byteSize <= 4 * 1024 * 1024;
  if (['text/plain', 'text/markdown', 'text/csv', 'application/json'].includes(mediaType)) return value.byteSize <= 16 * 1024;
  return false;
}

function isModelInputExclusion(value: unknown): value is ModelInputExclusion {
  return isRecord(value)
    && hasString(value, 'inputId')
    && typeof value.relativePath === 'string'
    && hasString(value, 'mediaType')
    && typeof value.byteSize === 'number' && Number.isSafeInteger(value.byteSize) && value.byteSize > 0
    && typeof value.contentDigest === 'string' && /^[0-9a-f]{64}$/.test(value.contentDigest)
    && isOneOf(value.reason, ['representation_not_supported', 'context_limit']);
}

function isTaskInputManifestView(value: unknown, companyId: string, taskId: string): value is TaskInputManifestView {
  if (!isRecord(value) || !isRecord(value.manifest)) return false;
  const manifest = value.manifest;
  const includedInputIds = value.includedInputIds;
  const includedInputPaths = value.includedInputPaths;
  const inputExclusions = value.inputExclusions;
  const deliveryStatus = value.deliveryStatus;
  const payloadDigest = value.payloadDigest;
  const hasDeliveryAttempt = deliveryStatus !== 'not_delivered';
  if (!Array.isArray(includedInputIds) || !includedInputIds.every(inputId => typeof inputId === 'string' && inputId !== '')
    || new Set(includedInputIds).size !== includedInputIds.length
    || !Array.isArray(includedInputPaths) || !includedInputPaths.every(isModelInputDeliveryRef) || includedInputPaths.length > MAX_TASK_INPUT_DELIVERY_REFS
    || !Array.isArray(inputExclusions)
    || !inputExclusions.every(isModelInputExclusion)
    || !isOneOf(deliveryStatus, ['not_delivered', 'sending', 'provider_delivered', 'local_context_loaded', 'outcome_unknown', 'not_sent', 'not_required'])
    || (hasDeliveryAttempt ? typeof payloadDigest !== 'string' || !/^[0-9a-f]{64}$/.test(payloadDigest) : payloadDigest !== null)) {
    return false;
  }
  const classifiedInputIds = new Set<string>();
  const deliveryPaths = new Set<string>();
  const includedIdsFromRefs = classifiedIncludedInputIds(includedInputPaths);
  let includedTextBytes = 0;
  let includedImageBytes = 0;
  let includedTextFiles = 0;
  let includedImages = 0;
  for (const input of includedInputPaths) {
    const pathKey = input.inputId + '\u0000' + input.relativePath.toLowerCase();
    if (deliveryPaths.has(pathKey)) return false;
    deliveryPaths.add(pathKey);
    classifiedInputIds.add(input.inputId);
    if (input.mediaType === 'image/png' || input.mediaType === 'image/jpeg') {
      includedImages++;
      includedImageBytes += input.byteSize;
    } else {
      includedTextFiles++;
      includedTextBytes += input.byteSize;
    }
  }
  for (const exclusion of inputExclusions) {
    const pathKey = exclusion.inputId + '\u0000' + exclusion.relativePath.toLowerCase();
    if (deliveryPaths.has(pathKey)) return false;
    deliveryPaths.add(pathKey);
    classifiedInputIds.add(exclusion.inputId);
  }
  if (includedTextBytes > 64 * 1024 || includedTextFiles > 8 || includedImageBytes > 8 * 1024 * 1024 || includedImages > 4 || includedInputIds.length !== includedIdsFromRefs.length
    || includedInputIds.some((inputId, index) => inputId !== includedIdsFromRefs[index])) return false;
  return value.companyId === companyId
    && value.taskId === taskId
    && isNullableString(value.missionId)
    && typeof value.manifestDigest === 'string' && /^[0-9a-f]{64}$/.test(value.manifestDigest)
    && isOneOf(value.deliveryStatus, ['not_delivered', 'sending', 'provider_delivered', 'local_context_loaded', 'outcome_unknown', 'not_sent', 'not_required'])
    && hasString(value, 'createdAt')
    && manifest.schemaVersion === 'polis-model-input-manifest@1'
    && manifest.companyId === companyId
    && manifest.missionId === value.missionId
    && manifest.taskId === taskId
    && manifest.deliveryStatus === 'not_delivered'
    && Array.isArray(manifest.candidateInputs)
    && Array.isArray(manifest.excludedInputs)
    && manifest.candidateInputs.every(entry => isModelInputManifestEntry(entry, true))
    && manifest.excludedInputs.every(entry => isModelInputManifestEntry(entry, false))
    && (value.deliveryStatus === 'not_delivered'
      ? classifiedInputIds.size === 0 && inputExclusions.length === 0 && includedInputPaths.length === 0
      : value.deliveryStatus === 'not_required'
        ? manifest.candidateInputs.length === 0 && classifiedInputIds.size === 0 && inputExclusions.length === 0 && includedInputPaths.length === 0
      : classifiedInputIds.size === manifest.candidateInputs.length
        && manifest.candidateInputs.every(entry => classifiedInputIds.has((entry as {inputId: string}).inputId)));
}

function classifiedIncludedInputIds(inputs: ReadonlyArray<ModelInputDeliveryRef>): ReadonlyArray<string> {
  const ids: string[] = [];
  const seen = new Set<string>();
  for (const input of inputs) {
    if (seen.has(input.inputId)) continue;
    seen.add(input.inputId);
    ids.push(input.inputId);
  }
  return ids;
}

function isEmployeeDraft(value: unknown): boolean {
  return isRecord(value)
    && hasString(value, 'id')
    && hasString(value, 'displayName')
    && hasString(value, 'role')
    && hasString(value, 'modelProfile');
}

function isFixedTeamCoverageAssignment(value: unknown): boolean {
  return isRecord(value)
    && hasString(value, 'task_type')
    && hasString(value, 'owner')
    && Array.isArray(value.eligible_independent_checkers)
    && value.eligible_independent_checkers.every(checker => typeof checker === 'string')
    && hasString(value, 'acceptance_path')
    && value.qualification === 'unverified';
}

function isFixedTeamCoverageRoleRevision(value: unknown): boolean {
  if (!isRecord(value) || !isRecord(value.contract)) return false;
  const contract = value.contract;
  return typeof value.revision_sha256 === 'string'
    && /^[a-f0-9]{64}$/.test(value.revision_sha256)
    && value.owner_decision === 'installation_owner_confirmed_fixed_team_mapping'
    && value.qualification === 'unverified'
    && hasString(contract, 'document_version')
    && contract.design_kind === 'fixed_team_coverage_draft'
    && contract.execution_enabled === false
    && contract.role_changes_at_runtime === false
    && contract.template_requires_human_confirmation === true
    && Array.isArray(contract.employee_ids)
    && contract.employee_ids.every(employeeId => typeof employeeId === 'string')
    && Array.isArray(contract.coverage)
    && contract.coverage.every(assignment => isFixedTeamCoverageAssignment(assignment))
    && hasString(contract, 'missing_path')
    && hasString(contract, 'checker_policy')
    && contract.trusted_baseline_mutable_by_workers === false
    && contract.guarantees_semantic_independence === false;
}

function isCompanySummary(value: unknown): value is CompanySummaryView {
  return isRecord(value)
    && hasString(value, 'id')
    && hasString(value, 'name')
    && hasString(value, 'workspaceRoot')
    && isOneOf(value.state, ['active', 'archived'])
    && Array.isArray(value.roster)
    && value.roster.every(employee => isEmployeeDraft(employee))
    && (value.teamCoverageConfirmed === undefined || typeof value.teamCoverageConfirmed === 'boolean')
    && (value.teamCoverageConfirmationSha256 === undefined || typeof value.teamCoverageConfirmationSha256 === 'string')
    && (value.teamCoverageConfirmedAt === undefined || typeof value.teamCoverageConfirmedAt === 'string')
    && (value.teamCoverageRoleRevision === undefined || isFixedTeamCoverageRoleRevision(value.teamCoverageRoleRevision));
}

function isRuntimeReadiness(value: unknown): boolean {
  return isOneOf(value, ['configured', 'missing', 'invalid', 'ready', 'unavailable', 'restart_required', 'not_required', 'not_applicable', 'deferred']);
}

function isRuntimeSettings(value: unknown): value is RuntimeSettingsView {
  return isRecord(value)
    && hasString(value, 'companyId')
    && hasString(value, 'workerMode')
    && hasString(value, 'provider')
    && hasString(value, 'model')
    && hasString(value, 'effort')
    && hasString(value, 'profile')
    && isRuntimeReadiness(value.authReadiness)
    && hasString(value, 'runtimeVersion')
    && isRuntimeReadiness(value.runtimeReadiness)
    && hasString(value, 'productSurfaceQualification')
    && hasString(value, 'workspaceRoot')
    && isRuntimeReadiness(value.postgresqlStatus)
    && isRuntimeReadiness(value.casStatus)
    && isRuntimeReadiness(value.eventStreamStatus);
}

function isCodexReasoningEffortOption(value: unknown): boolean {
  return isRecord(value)
    && typeof value.reasoningEffort === 'string'
    && value.reasoningEffort.trim() !== ''
    && typeof value.description === 'string';
}

function isCodexModelOption(value: unknown): value is CodexModelOptionView {
  return isRecord(value)
    && typeof value.model === 'string'
    && value.model.trim() !== ''
    && typeof value.displayName === 'string'
    && value.displayName.trim() !== ''
    && typeof value.description === 'string'
    && typeof value.isDefault === 'boolean'
    && typeof value.defaultReasoningEffort === 'string'
    && Array.isArray(value.supportedReasoningEfforts)
    && value.supportedReasoningEfforts.every(isCodexReasoningEffortOption);
}

function isCodexModelCatalog(value: unknown): value is CodexModelCatalogView {
  return isRecord(value)
    && Array.isArray(value.models)
    && value.models.every(isCodexModelOption);
}

function isOperatorInstructionResponse(value: unknown): value is OperatorInstructionResponseView {
  return isRecord(value)
    && typeof value.employeeId === 'string'
    && value.employeeId !== ''
    && typeof value.outcome === 'string'
    && isOneOf(value.outcome, ['applied', 'rejected', 'needs_clarification'])
    && typeof value.summary === 'string'
    && value.summary.trim() !== ''
    && [...value.summary].length <= 128
    && new TextEncoder().encode(value.summary).length <= 512
    && typeof value.respondedAt === 'string'
    && value.respondedAt !== '';
}

function isMissionChangeInputRevision(value: unknown): boolean {
  return isRecord(value) && hasString(value, 'inputId')
    && typeof value.revision === 'number' && Number.isSafeInteger(value.revision) && value.revision > 0
    && typeof value.contentDigest === 'string' && /^[0-9a-f]{64}$/.test(value.contentDigest)
    && isOneOf(value.state, ['usable', 'partial', 'unsupported']);
}

function isMissionChangeImpact(value: unknown, missionId: string): boolean {
  return isRecord(value)
    && value.schemaVersion === 'polis-mission-change-impact@1'
    && value.missionId === missionId
    && typeof value.baseRequirementsSha256 === 'string' && /^[0-9a-f]{64}$/.test(value.baseRequirementsSha256)
    && Array.isArray(value.inputRevisions) && value.inputRevisions.every(isMissionChangeInputRevision)
    && Array.isArray(value.tasks) && value.tasks.every(task => isRecord(task)
      && hasString(task, 'taskId') && hasString(task, 'ownerEmployeeId') && hasString(task, 'kind') && hasString(task, 'state')
      && typeof task.generation === 'number' && Number.isSafeInteger(task.generation) && task.generation >= 0
      && isNullableString(task.workspaceDigest) && (task.workspaceDigest === null || /^[0-9a-f]{64}$/.test(task.workspaceDigest))
      && (task.workspaceRevision === null || (typeof task.workspaceRevision === 'number' && Number.isSafeInteger(task.workspaceRevision) && task.workspaceRevision > 0)))
    && Array.isArray(value.artifacts) && value.artifacts.every(artifact => isRecord(artifact)
      && hasString(artifact, 'artifactId') && hasString(artifact, 'taskId')
      && typeof artifact.digest === 'string' && /^[0-9a-f]{64}$/.test(artifact.digest) && hasString(artifact, 'verdict'))
    && Array.isArray(value.activeWorkerSessions) && value.activeWorkerSessions.every(session => isRecord(session)
      && hasString(session, 'sessionId') && hasString(session, 'taskId') && hasString(session, 'employeeId') && hasString(session, 'state'))
    && Array.isArray(value.nonterminalJobRuns) && value.nonterminalJobRuns.every(job => isRecord(job)
      && hasString(job, 'jobId') && hasString(job, 'taskId') && hasString(job, 'state') && hasString(job, 'readiness'))
    && Array.isArray(value.activeServiceEndpoints) && value.activeServiceEndpoints.every(endpoint => isRecord(endpoint)
      && hasString(endpoint, 'jobId') && typeof endpoint.generation === 'number' && Number.isSafeInteger(endpoint.generation) && endpoint.generation > 0
      && isOneOf(endpoint.readiness, ['not_ready', 'ready', 'unhealthy', 'revoked']))
    && Array.isArray(value.activeTaskTakeoverLeases) && value.activeTaskTakeoverLeases.every(lease => isRecord(lease)
      && hasString(lease, 'leaseId') && hasString(lease, 'taskId')
      && typeof lease.baseWorkspaceDigest === 'string' && /^[0-9a-f]{64}$/.test(lease.baseWorkspaceDigest)
      && typeof lease.baseWorkspaceRevision === 'number' && Number.isSafeInteger(lease.baseWorkspaceRevision) && lease.baseWorkspaceRevision > 0)
    && Array.isArray(value.returnedHumanTakeoverSnapshots) && value.returnedHumanTakeoverSnapshots.every(snapshot => isRecord(snapshot)
      && hasString(snapshot, 'leaseId') && hasString(snapshot, 'taskId') && hasString(snapshot, 'inputId')
      && typeof snapshot.inputRevision === 'number' && Number.isSafeInteger(snapshot.inputRevision) && snapshot.inputRevision > 0
      && typeof snapshot.contentDigest === 'string' && /^[0-9a-f]{64}$/.test(snapshot.contentDigest)
      && typeof snapshot.byteSize === 'number' && Number.isSafeInteger(snapshot.byteSize) && snapshot.byteSize > 0
      && (snapshot.humanEffortSeconds === null || (typeof snapshot.humanEffortSeconds === 'number' && Number.isSafeInteger(snapshot.humanEffortSeconds) && snapshot.humanEffortSeconds >= 1 && snapshot.humanEffortSeconds <= 86400)))
    && value.naturalLanguageImpactStatus === 'not_assessed';
}

function isMissionChangeInputRevisionMap(value: unknown): boolean {
  return isRecord(value) && isOneOf(value.origin, ['mission_input', 'task_workspace', 'human_takeover'])
    && (value.origin === 'mission_input' ? value.sourceTaskId === undefined : hasString(value, 'sourceTaskId'))
    && hasString(value, 'previousInputId')
    && typeof value.previousRevision === 'number' && Number.isSafeInteger(value.previousRevision) && value.previousRevision > 0
    && hasString(value, 'successorInputId')
    && typeof value.successorRevision === 'number' && Number.isSafeInteger(value.successorRevision) && value.successorRevision > 0
    && typeof value.contentDigest === 'string' && /^[0-9a-f]{64}$/.test(value.contentDigest);
}

function isMissionChangeRequestEvent(value: unknown): boolean {
  return isRecord(value) && hasString(value, 'eventId')
    && isOneOf(value.state, ['received', 'queued', 'considered', 'applied', 'declined', 'superseded'])
    && (value.impactRevision === null || (typeof value.impactRevision === 'number' && Number.isSafeInteger(value.impactRevision) && value.impactRevision > 0))
    && isNullableString(value.successorMissionId)
    && (value.state === 'applied' ? value.successorMissionId !== null : value.successorMissionId === null)
    && hasString(value, 'reasonCode') && hasString(value, 'createdAt')
    && (value.planningAssessmentId === undefined || hasString(value, 'planningAssessmentId'))
    && (value.planningAssessmentSha256 === undefined || (typeof value.planningAssessmentSha256 === 'string' && /^[0-9a-f]{64}$/.test(value.planningAssessmentSha256)))
    && (value.planningRiskLevel === undefined || isOneOf(value.planningRiskLevel, ['low', 'high', 'uncertain']))
    && ((value.planningAssessmentId === undefined && value.planningAssessmentSha256 === undefined && value.planningRiskLevel === undefined)
      || (typeof value.planningAssessmentId === 'string' && value.planningAssessmentId.trim() !== ''
        && typeof value.planningAssessmentSha256 === 'string' && /^[0-9a-f]{64}$/.test(value.planningAssessmentSha256)
        && isOneOf(value.planningRiskLevel, ['low', 'high', 'uncertain'])))
    && Array.isArray(value.inputRevisionMap) && value.inputRevisionMap.every(isMissionChangeInputRevisionMap);
}

function isMissionChangePlanningAssessment(value: unknown): value is MissionChangePlanningAssessmentView {
  if (!isRecord(value) || value.schemaVersion !== 'polis-mission-change-planning-assessment@1'
    || !hasString(value, 'assessmentId') || typeof value.revision !== 'number' || !Number.isSafeInteger(value.revision) || value.revision < 1
    || !isOneOf(value.status, ['current', 'stale'])
    || typeof value.analysisBasisSha256 !== 'string' || !/^[0-9a-f]{64}$/.test(value.analysisBasisSha256)
    || typeof value.assessmentSha256 !== 'string' || !/^[0-9a-f]{64}$/.test(value.assessmentSha256)
    || !isOneOf(value.riskLevel, ['low', 'high', 'uncertain']) || typeof value.summary !== 'string' || value.summary.trim() === '' || value.summary.length > 4096
    || !Array.isArray(value.affectedTaskIds) || !Array.isArray(value.unaffectedTaskIds) || !Array.isArray(value.uncertainTaskIds)
    || !Array.isArray(value.questions) || !value.questions.every(item => typeof item === 'string' && item.trim() !== '' && item.length <= 512)
    || !Array.isArray(value.recommendedControls) || !value.recommendedControls.every(item => typeof item === 'string' && item.trim() !== '' && item.length <= 1024)
    || !hasString(value, 'workerSessionId') || typeof value.workerTaskId !== 'string' || !/^[A-Za-z0-9_-]{1,80}$/.test(value.workerTaskId)
    || typeof value.workerEpoch !== 'number' || !Number.isSafeInteger(value.workerEpoch) || value.workerEpoch < 1
    || !hasString(value, 'createdAt')) return false;
  const taskIds = [...value.affectedTaskIds, ...value.unaffectedTaskIds, ...value.uncertainTaskIds];
  return taskIds.length <= 512 && taskIds.every(taskId => typeof taskId === 'string' && /^[A-Za-z0-9_-]{1,80}$/.test(taskId))
    && new Set(taskIds).size === taskIds.length
    && value.questions.length <= 12 && value.recommendedControls.length <= 12;
}

function isMissionChangeRequest(value: unknown, missionId: string): value is MissionChangeRequestView {
  return isRecord(value) && hasString(value, 'changeRequestId') && value.missionId === missionId
    && hasString(value, 'clientRequestId')
    && typeof value.baseRequirementsSha256 === 'string' && /^[0-9a-f]{64}$/.test(value.baseRequirementsSha256)
    && typeof value.changeSummary === 'string' && value.changeSummary.trim() !== ''
    && hasString(value, 'proposedTitle') && hasString(value, 'proposedGoal')
    && (value.proposedAcceptanceContract === null || isAcceptanceContract(value.proposedAcceptanceContract))
    && typeof value.blockPreviousResults === 'boolean'
    && isOneOf(value.state, ['received', 'queued', 'considered', 'applied', 'declined', 'superseded'])
    && typeof value.impactRevision === 'number' && Number.isSafeInteger(value.impactRevision) && value.impactRevision > 0
    && typeof value.impactSha256 === 'string' && /^[0-9a-f]{64}$/.test(value.impactSha256)
    && isMissionChangeImpact(value.impact, missionId)
    && (value.planningAssessment === null || isMissionChangePlanningAssessment(value.planningAssessment))
    && isNullableString(value.successorMissionId)
    && (value.state === 'applied' ? value.successorMissionId !== null : value.successorMissionId === null)
    && Array.isArray(value.inputRevisionMap) && value.inputRevisionMap.every(isMissionChangeInputRevisionMap)
    && hasString(value, 'createdAt')
    && Array.isArray(value.events) && value.events.every(isMissionChangeRequestEvent);
}

function isNullablePositiveInteger(value: unknown): value is number | null {
  return value === null || (typeof value === 'number' && Number.isSafeInteger(value) && value > 0);
}

function isTakeoverDiffSummary(value: unknown): value is TaskTakeoverDiffSummaryView {
  return isRecord(value) && hasString(value, 'model')
    && typeof value.baseWorkspaceDigest === 'string' && /^[0-9a-f]{64}$/.test(value.baseWorkspaceDigest)
    && typeof value.baseWorkspaceRevision === 'number' && Number.isSafeInteger(value.baseWorkspaceRevision) && value.baseWorkspaceRevision > 0
    && typeof value.submittedContentDigest === 'string' && /^[0-9a-f]{64}$/.test(value.submittedContentDigest)
    && ['baseBytes', 'submittedBytes', 'removedLines', 'addedLines'].every(key => typeof value[key] === 'number' && Number.isSafeInteger(value[key]) && (value[key] as number) >= 0)
    && (value.baseWorkspaceTreeSha256 === undefined || (typeof value.baseWorkspaceTreeSha256 === 'string' && /^[0-9a-f]{64}$/.test(value.baseWorkspaceTreeSha256)))
    && (value.submittedWorkspaceTreeSha256 === undefined || (typeof value.submittedWorkspaceTreeSha256 === 'string' && /^[0-9a-f]{64}$/.test(value.submittedWorkspaceTreeSha256)))
    && ['addedFiles', 'modifiedFiles', 'deletedFiles'].every(key => value[key] === undefined || (Array.isArray(value[key]) && value[key].every(isWorkspaceRelativePath)))
    && typeof value.changed === 'boolean' && value.changed;
}

function isTaskTakeoverWorkspaceTreeBinding(value: unknown): value is TaskTakeoverWorkspaceTreeBindingView {
  return isRecord(value) && hasString(value, 'rootBindingId')
    && typeof value.revision === 'number' && Number.isSafeInteger(value.revision) && value.revision > 0
    && typeof value.manifestSha256 === 'string' && /^[0-9a-f]{64}$/.test(value.manifestSha256)
    && typeof value.fileCount === 'number' && Number.isSafeInteger(value.fileCount) && value.fileCount >= 1 && value.fileCount <= 512
    && typeof value.bytes === 'number' && Number.isSafeInteger(value.bytes) && value.bytes >= 1 && value.bytes <= 16 * 1024 * 1024;
}

function isWorkspaceRelativePath(value: unknown): value is string {
  if (typeof value !== 'string' || value.length === 0 || value.length > 1024 || value.startsWith('/') || value.endsWith('/') || value.includes('\\') || value.includes('%') || value.includes(':')) return false;
  return value.split('/').every(part => part !== '' && part !== '.' && part !== '..' && part.trim() === part && part.length <= 255 && !/[\u0000-\u001f\u007f]/.test(part));
}

function isTaskTakeoverWorkspaceManifestEntry(value: unknown): value is TaskTakeoverWorkspaceManifestEntryView {
  return isRecord(value) && isWorkspaceRelativePath(value.relativePath)
    && typeof value.sha256 === 'string' && /^[0-9a-f]{64}$/.test(value.sha256)
    && typeof value.bytes === 'number' && Number.isSafeInteger(value.bytes) && value.bytes >= 1 && value.bytes <= 2 * 1024 * 1024
    && typeof value.fileRevision === 'number' && Number.isSafeInteger(value.fileRevision) && value.fileRevision > 0
    && typeof value.sourceRevision === 'number' && Number.isSafeInteger(value.sourceRevision) && value.sourceRevision > 0
    && value.contentType === 'text/utf-8';
}

function isTaskTakeoverLeaseEvent(value: unknown): boolean {
  return isRecord(value) && hasString(value, 'eventId') && isOneOf(value.state, ['granted', 'returned', 'released'])
    && isNullableString(value.snapshotInputId) && isNullablePositiveInteger(value.snapshotRevision)
    && (value.snapshotDigest === null || (typeof value.snapshotDigest === 'string' && /^[0-9a-f]{64}$/.test(value.snapshotDigest)))
    && isNullablePositiveInteger(value.snapshotBytes) && isNullablePositiveInteger(value.humanEffortSeconds)
    && (value.diffSummary === null || isTakeoverDiffSummary(value.diffSummary))
    && hasString(value, 'reasonCode') && hasString(value, 'createdAt')
    && (value.state === 'returned' ? value.snapshotInputId !== null && value.snapshotRevision !== null && value.snapshotDigest !== null && value.snapshotBytes !== null && value.diffSummary !== null
      : value.snapshotInputId === null && value.snapshotRevision === null && value.snapshotDigest === null && value.snapshotBytes === null && value.humanEffortSeconds === null && value.diffSummary === null);
}

function sameTaskTakeoverDiffSummary(left: unknown, right: unknown): boolean {
  if (left === null || right === null) return left === right;
  if (!isTakeoverDiffSummary(left) || !isTakeoverDiffSummary(right)) return false;
  return left.model === right.model && left.baseWorkspaceDigest === right.baseWorkspaceDigest
    && left.baseWorkspaceRevision === right.baseWorkspaceRevision && left.submittedContentDigest === right.submittedContentDigest
    && left.baseBytes === right.baseBytes && left.submittedBytes === right.submittedBytes
    && left.removedLines === right.removedLines && left.addedLines === right.addedLines && left.changed === right.changed
    && left.baseWorkspaceTreeSha256 === right.baseWorkspaceTreeSha256 && left.submittedWorkspaceTreeSha256 === right.submittedWorkspaceTreeSha256
    && JSON.stringify(left.addedFiles ?? []) === JSON.stringify(right.addedFiles ?? [])
    && JSON.stringify(left.modifiedFiles ?? []) === JSON.stringify(right.modifiedFiles ?? [])
    && JSON.stringify(left.deletedFiles ?? []) === JSON.stringify(right.deletedFiles ?? []);
}

function isTaskTakeoverLatestEvent(value: Record<string, unknown>, latest: unknown): boolean {
  return isRecord(latest) && latest.state === value.state
    && latest.snapshotInputId === value.snapshotInputId && latest.snapshotRevision === value.snapshotRevision
    && latest.snapshotDigest === value.snapshotDigest && latest.snapshotBytes === value.snapshotBytes
    && latest.humanEffortSeconds === value.humanEffortSeconds && sameTaskTakeoverDiffSummary(latest.diffSummary, value.diffSummary);
}

function isTaskTakeoverLease(value: unknown, missionId: string): value is TaskTakeoverLeaseView {
  return isRecord(value) && hasString(value, 'leaseId') && value.missionId === missionId && hasString(value, 'taskId')
    && hasString(value, 'clientRequestId')
    && typeof value.baseRequirementsSha256 === 'string' && /^[0-9a-f]{64}$/.test(value.baseRequirementsSha256)
    && typeof value.baseWorkspaceDigest === 'string' && /^[0-9a-f]{64}$/.test(value.baseWorkspaceDigest)
    && typeof value.baseWorkspaceRevision === 'number' && Number.isSafeInteger(value.baseWorkspaceRevision) && value.baseWorkspaceRevision > 0
    && (value.workspaceTree === undefined || isTaskTakeoverWorkspaceTreeBinding(value.workspaceTree))
    && isOneOf(value.state, ['granted', 'returned', 'released'])
    && isNullableString(value.snapshotInputId) && isNullablePositiveInteger(value.snapshotRevision)
    && (value.snapshotDigest === null || (typeof value.snapshotDigest === 'string' && /^[0-9a-f]{64}$/.test(value.snapshotDigest)))
    && isNullablePositiveInteger(value.snapshotBytes) && isNullablePositiveInteger(value.humanEffortSeconds)
    && (value.diffSummary === null || isTakeoverDiffSummary(value.diffSummary))
    && hasString(value, 'createdAt') && Array.isArray(value.events) && value.events.length > 0 && value.events.every(isTaskTakeoverLeaseEvent)
    && isTaskTakeoverLatestEvent(value, value.events[value.events.length - 1])
    && (value.state === 'returned' ? value.snapshotInputId !== null && value.snapshotRevision !== null && value.snapshotDigest !== null && value.snapshotBytes !== null && value.diffSummary !== null
      : value.snapshotInputId === null && value.snapshotRevision === null && value.snapshotDigest === null && value.snapshotBytes === null && value.humanEffortSeconds === null && value.diffSummary === null);
}

function isOperatorInstruction(value: unknown): value is OperatorInstructionView {
  if (!isRecord(value)
    || !hasString(value, 'instructionId')
    || !isNullableString(value.missionId)
    || !isNullableString(value.taskId)
    || !isNullableString(value.employeeId)
    || !hasString(value, 'content')
    || !isOneOf(value.state, ['pending', 'applied', 'rejected', 'needs_clarification'])
    || !hasString(value, 'createdAt')
    || !(value.responseOutcome === null || isOneOf(value.responseOutcome, ['applied', 'rejected', 'needs_clarification']))
    || !isNullableString(value.responseSummary)
    || !isNullableString(value.respondedByEmployeeId)
    || !isNullableString(value.respondedAt)
    || !Array.isArray(value.responses)
    || !value.responses.every(isOperatorInstructionResponse)) {
    return false;
  }
  const hasNoLatestResponse = value.responseOutcome === null && value.responseSummary === null && value.respondedByEmployeeId === null && value.respondedAt === null;
  if (value.responses.length === 0) return hasNoLatestResponse && value.state !== 'needs_clarification';
  if (value.state === 'pending' && (value.taskId !== null || value.employeeId !== null)) return false;
  const latest = value.responses[value.responses.length - 1];
  if (value.responseOutcome !== latest.outcome || value.responseSummary !== latest.summary || value.respondedByEmployeeId !== latest.employeeId || value.respondedAt !== latest.respondedAt) {
    return false;
  }
  const outcomes = value.responses.map(response => response.outcome);
  if (value.state === 'applied' && !outcomes.every(outcome => outcome === 'applied')) return false;
  if (value.state === 'rejected' && (!outcomes.includes('rejected') || outcomes.includes('needs_clarification'))) return false;
  if (value.state === 'needs_clarification' && !outcomes.includes('needs_clarification')) return false;
  return true;
}

function isCollaborationItem(value: unknown): value is CollaborationItem {
  return isRecord(value)
    && hasString(value, 'messageId')
    && hasString(value, 'missionId')
    && hasString(value, 'taskId')
    && hasString(value, 'senderEmployeeId')
    && hasString(value, 'recipientEmployeeId')
    && hasString(value, 'content')
    && hasString(value, 'kind')
    && hasString(value, 'deliveryState')
    && isNullableString(value.contractRevisionId)
    && isNullableString(value.obligationId)
    && isNullableString(value.obligationState)
    && isNullableString(value.evidenceRef)
    && hasString(value, 'taskRevision');
}

function isWorkspaceView(value: unknown): value is WorkspaceView {
  return isRecord(value)
    && hasString(value, 'taskId')
    && hasString(value, 'revision')
    && hasString(value, 'digest')
    && Array.isArray(value.files)
    && value.files.every(file => isRecord(file) && hasString(file, 'path') && hasString(file, 'bytes') && typeof file.content === 'string')
    && Array.isArray(value.changedFiles)
    && value.changedFiles.every(file => typeof file === 'string')
    && Array.isArray(value.checkpoints);
}

function isArtifactDetail(value: unknown): value is ArtifactDetailView {
  return isRecord(value)
    && hasString(value, 'artifactId')
    && hasString(value, 'taskId')
    && hasString(value, 'digest')
    && hasString(value, 'bytes')
    && hasString(value, 'state')
    && hasString(value, 'verdict')
    && typeof value.content === 'string'
    && typeof value.contentAvailable === 'boolean';
}

function isOperations(value: unknown): value is OperationsView {
  return isRecord(value)
    && hasString(value, 'companyId')
    && hasString(value, 'toolCallsUsed')
    && hasString(value, 'toolCallsLimit')
    && isOneOf(value.toolBudgetQuality, ['reported', 'estimated', 'unavailable'])
    && isNullableString(value.inputTokens)
    && isNullableString(value.outputTokens)
    && isNullableString(value.elapsedRuntime)
    && hasString(value, 'workerCount')
    && hasString(value, 'postgresqlStatus')
    && hasString(value, 'casStatus')
    && hasString(value, 'eventStreamStatus')
    && isNullableString(value.lastRuntimeError);
}

function isNotifications(value: unknown): value is NotificationsView {
  return isRecord(value)
    && Array.isArray(value.routes)
    && value.routes.every(route => isRecord(route) && hasString(route, 'routeId') && isOneOf(route.adapter, ['local', 'webhook', 'qq_official']) && typeof route.enabled === 'boolean' && typeof route.destination === 'string' && typeof route.safetyAlias === 'string' && typeof route.credentialRef === 'string' && hasString(route, 'status') && hasString(route, 'routeRevision') && hasString(route, 'qualificationStatus') && typeof route.qualifiedUntil === 'string')
    && Array.isArray(value.deliveries)
    && value.deliveries.every(delivery => isRecord(delivery) && hasString(delivery, 'deliveryId') && hasString(delivery, 'intentId') && hasString(delivery, 'adapter') && hasString(delivery, 'state') && isNullableString(delivery.errorCode) && hasString(delivery, 'createdAt'));
}

export function isAcceptanceContract(value: unknown): value is AcceptanceContract {
  if (!isRecord(value) || value.revision !== 'text-acceptance@1' || !Array.isArray(value.required_text) || value.required_text.length < 1 || value.required_text.length > 8) {
    return false;
  }
  const seen = new Set<string>();
  let totalBytes = 0;
  for (const criterion of value.required_text) {
    if (typeof criterion !== 'string' || criterion.trim() === '' || criterion.trim() !== criterion || new TextEncoder().encode(criterion).length > 512 || !hasSupportedPlaceholders(criterion) || seen.has(criterion)) {
      return false;
    }
    seen.add(criterion);
    totalBytes += new TextEncoder().encode(criterion).length;
  }
  return totalBytes <= 2048;
}

function hasSupportedPlaceholders(value: string): boolean {
  const tokens = value.match(/\{\{[^{}]*\}\}/g) ?? [];
  const stripped = value.replace(/\{\{[^{}]*\}\}/g, '');
  if (stripped.includes('{{') || stripped.includes('}}')) return false;
  return tokens.every(token => token === '{{mission_id}}' || token === '{{task_id}}');
}

function isStringMap(value: unknown): value is Record<string, string> {
  return isRecord(value) && Object.values(value).every(item => typeof item === 'string');
}

function isEntityRef(value: unknown): boolean {
  return isRecord(value) && hasString(value, 'kind') && hasString(value, 'id') && hasString(value, 'label');
}

function isMissionCommandReceipt(value: unknown): value is MissionCommandReceipt {
  return isRecord(value)
    && hasString(value, 'commandId')
    && isOneOf(value.commandType, MISSION_COMMAND_TYPES)
    && value.targetType === 'mission'
    && hasString(value, 'targetId')
    && hasString(value, 'requestId')
    && value.accepted === true
    && hasString(value, 'acceptedAt')
    && isOneOf(value.resultingState, MISSION_COMMAND_STATES);
}

function isActivityActor(value: unknown): boolean {
  return isRecord(value) && isOneOf(value.kind, ['employee', 'system']) && hasString(value, 'id') && hasString(value, 'label');
}

function isViewMeta(value: unknown): value is ViewMeta {
  if (!isRecord(value)) {
    return false;
  }
  return typeof value.schemaVersion === 'number'
    && Number.isSafeInteger(value.schemaVersion)
    && hasString(value, 'companyId')
    && hasString(value, 'entityRevision')
    && hasString(value, 'snapshotCursor')
    && hasString(value, 'observedAt')
    && isOneOf(value.dataMode, ['real', 'simulated', 'unavailable'])
    && isOneOf(value.freshness, ['fresh', 'stale', 'reconnecting', 'unknown'])
    && hasString(value, 'sourceLabel')
    && isOneOf(value.recoveryState, ['operational', 'recovery_required', 'unknown']);
}

export function isActivityEvent(value: unknown): value is ActivityEvent {
  if (!isRecord(value)) {
    return false;
  }
  return hasString(value, 'id')
    && hasString(value, 'companySeq')
    && hasString(value, 'occurredAt')
    && isOneOf(value.kind, ACTIVITY_EVENT_KINDS)
    && isActivityActor(value.actor)
    && isEntityRef(value.subject)
    && hasString(value, 'summary')
    && hasString(value, 'detail')
    && isOneOf(value.tone, STATUS_TONES)
    && Array.isArray(value.evidenceRefs)
    && value.evidenceRefs.every(item => typeof item === 'string')
    && isStringMap(value.metadata);
}

function isToolBudget(value: unknown): boolean {
  return isRecord(value)
    && isNullableString(value.limit)
    && isNullableString(value.used)
    && isNullableString(value.remaining)
    && isOneOf(value.quality, ['reported', 'estimated', 'unavailable']);
}

function isEmployeeStatus(value: unknown): boolean {
  return isRecord(value)
    && isOneOf(value.primary, STATUS_KEYS)
    && isOneOf(value.tone, STATUS_TONES)
    && hasString(value, 'reason')
    && hasString(value, 'activeModelRequests')
    && hasString(value, 'inFlightTools')
    && hasString(value, 'observedAt');
}

function isEmployeeSchedule(value: unknown): boolean {
  return isRecord(value)
    && isOneOf(value.state, ['quiescing', 'sleeping', 'wake_pending', 'admitted', 'working', 'paused', 'waiting_quota'])
    && hasString(value, 'workGeneration')
    && hasString(value, 'checkedGeneration')
    && isNullableString(value.nextDueAt)
    && typeof value.pauseReason === 'string';
}

function isQualification(value: unknown): boolean {
  return isRecord(value)
    && isOneOf(value.status, ['supported', 'limited', 'unsupported', 'unverified'])
    && isNullableString(value.evidenceId)
    && isNullableString(value.policyRevision);
}

function isEmployeeSummary(value: unknown): value is EmployeeSummary {
  if (!isRecord(value)) {
    return false;
  }
  return hasString(value, 'employeeId')
    && hasString(value, 'displayName')
    && hasString(value, 'role')
    && (value.roleRevision === null || (typeof value.roleRevision === 'string' && /^[0-9a-f]{64}$/.test(value.roleRevision)))
    && (value.roleRevisionDetail === undefined || (isRecord(value.roleRevisionDetail)
      && typeof value.roleRevisionDetail.revisionSha256 === 'string' && /^[0-9a-f]{64}$/.test(value.roleRevisionDetail.revisionSha256)
      && value.roleRevisionDetail.revisionSha256 === value.roleRevision
      && value.roleRevisionDetail.employeeId === value.employeeId && typeof value.roleRevisionDetail.roleName === 'string'
      && Array.isArray(value.roleRevisionDetail.taskTypes) && value.roleRevisionDetail.taskTypes.every(item => typeof item === 'string')
      && Array.isArray(value.roleRevisionDetail.taskKinds) && value.roleRevisionDetail.taskKinds.every(item => typeof item === 'string')
      && value.roleRevisionDetail.ownerDecision === 'installation_owner_confirmed_fixed_team_mapping'
      && value.roleRevisionDetail.qualification === 'unverified'))
    && hasString(value, 'epoch')
    && isNullableString(value.sessionId)
    && isNullableString(value.sessionState)
    && isNullableString(value.profile)
    && (value.currentTask === null || isEntityRef(value.currentTask))
    && isEmployeeStatus(value.status)
    && (value.schedule === null || isEmployeeSchedule(value.schedule))
    && isToolBudget(value.toolBudget)
    && isQualification(value.qualification)
    && hasString(value, 'openObligationCount');
}

function isMissionSummary(value: unknown): boolean {
  if (!isRecord(value)) {
    return false;
  }
  return hasString(value, 'missionId')
    && hasString(value, 'title')
    && hasString(value, 'goal')
    && isOneOf(value.state, ['draft', 'active', 'paused', 'closing', 'succeeded', 'ended_not_met', 'cancelled'])
    && hasString(value, 'contract')
    && (value.acceptanceContract === null || isAcceptanceContract(value.acceptanceContract))
    && (value.currentContractRevision === null || isContractRevision(value.currentContractRevision))
    && hasString(value, 'nextMilestone')
    && hasString(value, 'verifiedMilestones')
    && hasString(value, 'milestoneTotal')
    && (!Object.hasOwn(value, 'closeout') || value.closeout === null || isMissionCloseoutSummary(value.closeout))
    && Array.isArray(value.milestones)
    && value.milestones.every(milestone => isRecord(milestone)
      && hasString(milestone, 'id')
      && hasString(milestone, 'label')
      && isOneOf(milestone.state, ['verified', 'current', 'upcoming', 'blocked']));
}

function isMissionCloseoutSummary(value: unknown): boolean {
  return isRecord(value)
    && isOneOf(value.requestedOutcome, ['succeeded', 'ended_not_met', 'cancelled'])
    && hasString(value, 'rationale')
    && Array.isArray(value.acceptanceArtifactIds)
    && value.acceptanceArtifactIds.every(item => typeof item === 'string')
    && hasString(value, 'requestId')
    && hasString(value, 'openedAt')
    && (value.terminalOutcome === null || isOneOf(value.terminalOutcome, ['succeeded', 'ended_not_met', 'cancelled']))
    && (!Object.hasOwn(value, 'report') || isRecord(value.report))
    && isNullableString(value.finishedAt);
}

function isContractRevision(value: unknown): value is ContractRevisionSummary {
  return isRecord(value)
    && hasString(value, 'revisionId')
    && hasString(value, 'revision')
    && hasString(value, 'endpoint')
    && isOneOf(value.state, ['proposed', 'accepted', 'superseded'])
    && hasString(value, 'digest')
    && hasString(value, 'proposerEmployeeId')
    && isNullableString(value.accepterEmployeeId);
}

function isTaskSummary(value: unknown): value is TaskSummary {
  return isRecord(value)
    && hasString(value, 'taskId')
    && hasString(value, 'title')
    && hasString(value, 'kind')
    && isOneOf(value.state, ['ready', 'working', 'candidate', 'completed', 'blocked', 'cancelled'])
    && hasString(value, 'ownerEmployeeId')
    && hasString(value, 'generation')
    && isNullableString(value.contractRevisionId)
    && isNullableString(value.workspaceRevision)
    && isOneOf(value.acceptance, ['not_started', 'candidate', 'passed', 'failed', 'inconclusive'])
    && isNullableString(value.dependencyLabel);
}

function isObligationSummary(value: unknown): value is ObligationSummary {
  return isRecord(value)
    && hasString(value, 'obligationId')
    && hasString(value, 'messageId')
    && hasString(value, 'ownerEmployeeId')
    && isOneOf(value.state, ['pending', 'observed', 'applied', 'fulfilled', 'declined', 'superseded'])
    && isNullableString(value.evidenceRef)
    && hasString(value, 'note');
}

function isArtifactSummary(value: unknown): value is ArtifactSummary {
  return isRecord(value)
    && hasString(value, 'artifactId')
    && hasString(value, 'taskId')
    && hasString(value, 'authorEmployeeId')
    && hasString(value, 'digest')
    && hasString(value, 'bytes')
    && isOneOf(value.state, ['ready', 'missing', 'corrupt'])
    && isOneOf(value.verdict, ['candidate', 'passed', 'failed', 'invalidated'])
    && isNullableString(value.contractRevisionId)
    && isNullableString(value.qualificationId)
    && isNullableString(value.checkpointId);
}

function isCheckpointSummary(value: unknown): value is CheckpointSummary {
  return isRecord(value)
    && hasString(value, 'checkpointId')
    && hasString(value, 'taskId')
    && hasString(value, 'employeeId')
    && hasString(value, 'sessionId')
    && hasString(value, 'sessionState')
    && hasString(value, 'sessionEpoch')
    && hasString(value, 'kind')
    && hasString(value, 'state')
    && hasString(value, 'qualificationState')
    && isNullableString(value.workspaceRevision)
    && isNullableString(value.workspaceDigest)
    && isNullableString(value.artifactId);
}

function isResourceSummary(value: unknown): value is ResourceSummary {
  return isRecord(value)
    && hasString(value, 'toolCallsUsed')
    && hasString(value, 'toolCallsLimit')
    && isOneOf(value.toolBudgetQuality, ['reported', 'estimated', 'unavailable'])
    && isOneOf(value.moneyQuality, ['reported', 'estimated', 'unavailable'])
    && isNullableString(value.moneyAmount)
    && isNullableString(value.currency)
    && hasString(value, 'asOf')
    && hasString(value, 'note');
}

function isAttentionItem(value: unknown): value is AttentionItem {
  return isRecord(value)
    && hasString(value, 'id')
    && isOneOf(value.tone, ['info', 'warning', 'danger'])
    && hasString(value, 'title')
    && hasString(value, 'description')
    && isEntityRef(value.subject)
    && Array.isArray(value.evidenceRefs)
    && value.evidenceRefs.every(item => typeof item === 'string')
    && (value.workflowState === undefined || isOneOf(value.workflowState, ['open', 'acknowledged']))
    && (value.notificationState === undefined || typeof value.notificationState === 'string');
}

const MAX_FEEDBACK_SOURCES = 50;
const MAX_FEEDBACK_ISSUES = 20;
const MAX_FEEDBACK_COMMENTS = 5;
const MAX_FEEDBACK_TITLE_BYTES = 4 * 1024;
const MAX_FEEDBACK_ISSUE_BODY_BYTES = 16 * 1024;
const MAX_FEEDBACK_COMMENT_BODY_BYTES = 4 * 1024;

function isSha256(value: unknown): value is string {
  return typeof value === 'string' && /^[a-f0-9]{64}$/.test(value);
}

function isFeedbackComment(value: unknown): boolean {
  return isRecord(value)
    && hasString(value, 'commentId')
    && isSha256(value.revisionSha256)
    && typeof value.body === 'string'
    && new TextEncoder().encode(value.body).byteLength <= MAX_FEEDBACK_COMMENT_BODY_BYTES
    && isSha256(value.bodySha256)
    && typeof value.bodyTruncated === 'boolean'
    && hasString(value, 'sourceUpdatedAt')
    && hasString(value, 'htmlUrl');
}

function isFeedbackIssue(value: unknown): boolean {
  return isRecord(value)
    && hasString(value, 'sourceId')
    && hasString(value, 'providerItemId')
    && hasString(value, 'issueNumber')
    && isSha256(value.revisionSha256)
    && isOneOf(value.backlogStatus, ['open', 'needs_review', 'triaging', 'waiting', 'handled', 'archived'])
    && hasString(value, 'backlogReason')
    && hasString(value, 'backlogUpdatedAt')
    && typeof value.title === 'string'
    && new TextEncoder().encode(value.title).byteLength <= MAX_FEEDBACK_TITLE_BYTES
    && typeof value.titleTruncated === 'boolean'
    && typeof value.body === 'string'
    && new TextEncoder().encode(value.body).byteLength <= MAX_FEEDBACK_ISSUE_BODY_BYTES
    && isSha256(value.bodySha256)
    && typeof value.bodyTruncated === 'boolean'
    && hasString(value, 'state')
    && hasString(value, 'sourceUpdatedAt')
    && hasString(value, 'observedAt')
    && hasString(value, 'htmlUrl')
    && hasString(value, 'commentCoverage')
    && typeof value.commentCoverageReason === 'string'
    && hasString(value, 'commentCount')
    && typeof value.commentContextPartial === 'boolean'
    && Array.isArray(value.comments)
    && value.comments.length <= MAX_FEEDBACK_COMMENTS
    && value.comments.every(isFeedbackComment);
}

function isFeedbackSource(value: unknown): boolean {
  return isRecord(value)
    && hasString(value, 'sourceId')
    && value.provider === 'github'
    && hasString(value, 'repositoryId')
    && hasString(value, 'repository')
    && hasString(value, 'profileRevision')
    && hasString(value, 'state')
    && hasString(value, 'permissionStatus')
    && hasString(value, 'coverage')
    && typeof value.coverageReason === 'string'
    && isNullableString(value.coveredThrough)
    && isNullableString(value.lastScanAt)
    && typeof value.collectionEnabled === 'boolean'
    && Number.isSafeInteger(value.collectionIntervalSeconds) && (value.collectionIntervalSeconds as number) >= 0
    && typeof value.collectionRationale === 'string'
    && isNullableString(value.collectionNextPollAt)
    && typeof value.collectionLastAttempt === 'string'
    && typeof value.collectionLastReasonCode === 'string';
}

function isCompanyFeedback(value: unknown, companyId: string): value is CompanyFeedbackView {
  return isRecord(value)
    && value.companyId === companyId
    && Array.isArray(value.sources)
    && value.sources.length <= MAX_FEEDBACK_SOURCES
    && value.sources.every(isFeedbackSource)
    && Array.isArray(value.issues)
    && value.issues.length <= MAX_FEEDBACK_ISSUES
    && value.issues.every(isFeedbackIssue);
}

function isArtifactDeliveryManifestResponse(value: unknown, companyId: string, artifactId: string): value is ArtifactDeliveryManifestResponse {
  if (!isRecord(value)) return false;
  const rawManifest = value.manifest;
  if (!isRecord(rawManifest)) return false;
  const rawContent = rawManifest.content;
  const rawQualification = rawManifest.qualification;
  if (!isRecord(rawContent) || !isRecord(rawQualification)) return false;
  const manifest = rawManifest;
  const content = rawContent;
  const qualification = rawQualification;
  const byteSize = typeof content.byteSize === 'string' ? Number(content.byteSize) : Number.NaN;
  return typeof value.manifestSha256 === 'string' && /^[0-9a-f]{64}$/.test(value.manifestSha256)
    && manifest.schemaVersion === 'polis-delivery-manifest@1'
    && manifest.companyId === companyId
    && manifest.artifactId === artifactId
    && hasString(manifest, 'taskId')
    && content.fileName === 'artifact.bin'
    && content.contentType === 'application/octet-stream'
    && Number.isSafeInteger(byteSize) && byteSize > 0 && byteSize <= 64 * 1024 * 1024
    && typeof content.sha256 === 'string' && /^[0-9a-f]{64}$/.test(content.sha256)
    && manifest.state === 'ready'
    && isOneOf(manifest.verdict, ['candidate', 'passed', 'failed', 'invalidated'])
    && hasString(qualification, 'checkpointId')
    && hasString(qualification, 'validationReceiptId')
    && typeof qualification.taskValidationBindingDigest === 'string' && /^[0-9a-f]{64}$/.test(qualification.taskValidationBindingDigest)
    && typeof qualification.workspaceDigest === 'string' && /^[0-9a-f]{64}$/.test(qualification.workspaceDigest)
    && hasString(qualification, 'workspaceRevision')
    && hasString(qualification, 'runnerRevision')
    && hasString(manifest, 'createdAt');
}

function isDurableDeliveryResponse(value: unknown, companyId: string, artifactId: string): value is DurableDeliveryResponse {
  if (!isRecord(value) || !isRecord(value.manifest) || !isRecord(value.userDisposition) || !Array.isArray(value.feedbackBacklog)) return false;
  const manifest = value.manifest;
  const artifact = manifest.artifact;
  const disposition = value.userDisposition;
  const feedbackBacklogValid = value.feedbackBacklog.length <= 32 && value.feedbackBacklog.every((item): item is DurableDeliveryFeedbackBacklogEventView => isRecord(item)
    && hasString(item, 'eventId') && hasString(item, 'deliveryId') && item.deliveryId === artifactId
    && hasString(item, 'manifestRevision') && isCanonicalPositiveInt64(item.manifestRevision)
    && hasString(item, 'dispositionRevision') && isCanonicalPositiveInt64(item.dispositionRevision)
    && hasString(item, 'missionId') && item.missionId === manifest.missionId && hasString(item, 'taskId') && item.taskId === manifest.taskId && item.artifactId === artifactId
    && item.manifestRevision === manifest.revision
    && item.status === 'open' && item.actor === 'system' && hasString(item, 'reason') && typeof item.reason === 'string' && item.reason.length > 0
    && hasString(item, 'requestId') && hasString(item, 'createdAt'));
  const manifestHistory = value.manifestHistory;
  const dispositionHistory = value.dispositionHistory;
  const currentArtifactForHistory = isRecord(artifact) ? artifact : undefined;
  const manifestHistoryRevisions = new Set<string>();
  const manifestHistoryValid = manifestHistory === undefined || (Array.isArray(manifestHistory) && manifestHistory.length <= 32 && manifestHistory.every(item => {
    if (currentArtifactForHistory === undefined || !isRecord(item) || !isRecord(item.manifest) || typeof item.manifestSha256 !== 'string' || !/^[0-9a-f]{64}$/.test(item.manifestSha256)) return false;
    const historical = item.manifest;
    const historicalArtifact = historical.artifact;
    if (!isRecord(historicalArtifact) || historical.schemaVersion !== 'polis-durable-delivery-manifest@1' || historical.deliveryId !== artifactId
      || historical.companyId !== companyId || historical.missionId !== manifest.missionId || historical.taskId !== manifest.taskId || historical.artifactId !== artifactId
      || typeof historical.revision !== 'string' || !isCanonicalPositiveInt64(historical.revision) || manifestHistoryRevisions.has(historical.revision)
      || !isOneOf(historical.state, ['assembling', 'ready', 'invalidated', 'withdrawn']) || historicalArtifact.fileName !== 'artifact.bin'
      || historicalArtifact.byteSize !== currentArtifactForHistory.byteSize || historicalArtifact.sha256 !== currentArtifactForHistory.sha256 || typeof historical.createdAt !== 'string'
      || !Array.isArray(historical.sections) || historical.sections.length > 64) return false;
    manifestHistoryRevisions.add(historical.revision);
    return historical.sections.every(section => isRecord(section) && typeof section.key === 'string' && section.key.length > 0 && section.key.length <= 120
      && typeof section.detail === 'string' && section.detail.length <= 4096
      && isOneOf(section.state, ['available', 'unavailable', 'missing', 'not_requested']));
  }));
  const dispositionHistoryValid = dispositionHistory === undefined || (Array.isArray(dispositionHistory) && dispositionHistory.length <= 32 && dispositionHistory.every(item => isRecord(item)
    && typeof item.revision === 'string' && isCanonicalPositiveInt64(item.revision)
    && typeof item.manifestRevision === 'string' && isCanonicalPositiveInt64(item.manifestRevision) && manifestHistoryRevisions.has(item.manifestRevision)
    && isOneOf(item.state, ['not_requested', 'awaiting_feedback', 'accepted', 'changes_requested'])
    && typeof item.actor === 'string' && typeof item.reason === 'string' && typeof item.requestId === 'string'
    && typeof item.feedbackDeadline === 'string' && typeof item.createdAt === 'string'));
  const revisionRoutes = value.revisionRoutes;
  const revisionRoutesValid = revisionRoutes === undefined || (Array.isArray(revisionRoutes) && revisionRoutes.length <= 16 && revisionRoutes.every(item => {
    if (!isRecord(item) || typeof item.routeId !== 'string' || typeof item.deliveryId !== 'string' || item.deliveryId !== artifactId
      || typeof item.manifestRevision !== 'string' || !isCanonicalPositiveInt64(item.manifestRevision)
      || typeof item.dispositionRevision !== 'string' || !isCanonicalPositiveInt64(item.dispositionRevision)
      || typeof item.missionId !== 'string' || item.missionId !== manifest.missionId || typeof item.changeRequestId !== 'string' || typeof item.reasonCode !== 'string' || typeof item.createdAt !== 'string'
      || !isOneOf(item.state, ['change_request_pending', 'successor_mission_created', 'revision_task_ready'])) return false;
    if (item.state === 'change_request_pending') return item.successorMissionId === '' && item.taskId === '';
    if (item.state === 'successor_mission_created') return typeof item.successorMissionId === 'string' && item.successorMissionId.length > 0 && item.taskId === '';
    return typeof item.successorMissionId === 'string' && item.successorMissionId.length > 0 && typeof item.taskId === 'string' && item.taskId.length > 0;
  }));
  if (!isRecord(artifact) || !Array.isArray(manifest.sections) || !hasString(manifest, 'revision')) return false;
  const byteSize = typeof artifact.byteSize === 'string' && /^[1-9]\d*$/.test(artifact.byteSize) ? Number(artifact.byteSize) : Number.NaN;
  const sectionKeys = new Set<string>();
  const sectionsValid = manifest.sections.length <= 64 && manifest.sections.every(section => {
    if (!isRecord(section) || typeof section.key !== 'string' || section.key.length === 0 || section.key.length > 120
      || typeof section.detail !== 'string' || section.detail.length > 4096
      || !isOneOf(section.state, ['available', 'unavailable', 'missing', 'not_requested'])) return false;
    if (sectionKeys.has(section.key)) return false;
    sectionKeys.add(section.key);
    return true;
  });
  return typeof value.manifestSha256 === 'string' && /^[0-9a-f]{64}$/.test(value.manifestSha256)
    && manifest.schemaVersion === 'polis-durable-delivery-manifest@1'
    && hasString(manifest, 'deliveryId')
    && isCanonicalPositiveInt64(manifest.revision)
    && manifest.companyId === companyId
    && hasString(manifest, 'missionId')
    && hasString(manifest, 'taskId')
    && manifest.artifactId === artifactId
    && isOneOf(manifest.state, ['assembling', 'ready', 'invalidated', 'withdrawn'])
    && artifact.fileName === 'artifact.bin'
    && Number.isSafeInteger(byteSize) && byteSize <= 64 * 1024 * 1024
    && typeof artifact.sha256 === 'string' && /^[0-9a-f]{64}$/.test(artifact.sha256)
    && sectionsValid
    && feedbackBacklogValid
    && manifestHistoryValid
    && dispositionHistoryValid
    && revisionRoutesValid
    && hasString(manifest, 'createdAt')
    && isCanonicalPositiveInt64(disposition.revision)
    && disposition.manifestRevision === manifest.revision
    && isOneOf(disposition.state, ['not_requested', 'awaiting_feedback', 'accepted', 'changes_requested'])
    && typeof disposition.actor === 'string'
    && typeof disposition.reason === 'string'
    && typeof disposition.requestId === 'string'
    && typeof disposition.feedbackDeadline === 'string'
    && typeof disposition.createdAt === 'string';
}

function isTeamSummary(value: unknown): boolean {
  return isRecord(value)
    && hasString(value, 'total')
    && hasString(value, 'working')
    && hasString(value, 'sleeping')
    && hasString(value, 'waiting')
    && hasString(value, 'stopped');
}

function validateMeta(meta: unknown, companyId: string): Array<ValidationIssue> {
  const issues: Array<ValidationIssue> = [];
  if (!isViewMeta(meta)) {
    issues.push({path: 'meta', message: 'meta is not a supported ViewMeta'});
    return issues;
  }
  if (meta.companyId !== companyId) {
    issues.push({path: 'meta.companyId', message: 'response company scope does not match request'});
  }
  return issues;
}


export function validateMissionInputs(value: unknown, companyId: string, missionId: string): ValidationResult<ReadonlyArray<MissionInputView>> {
  if (!Array.isArray(value)) {
    return {success: false, issues: [{path: '', message: 'mission inputs response is not an array'}]};
  }
  const issues: Array<ValidationIssue> = [];
  const seen = new Set<string>();
  value.forEach((item, index) => {
    if (!isMissionInputView(item, companyId, missionId)) {
      issues.push({path: String(index), message: 'mission input contains an unknown, malformed, or cross-scope field'});
      return;
    }
    const key = item.inputId + ':' + item.revision;
    if (seen.has(key)) {
      issues.push({path: String(index), message: 'mission inputs response contains a duplicate revision'});
    }
    seen.add(key);
  });
  return issues.length > 0 ? {success: false, issues} : {success: true, value: value as ReadonlyArray<MissionInputView>};
}

export function validateMissionInputCommandReceipt(value: unknown, companyId: string, missionId: string, requestId: string): ValidationResult<MissionInputCommandReceipt> {
  return isMissionInputCommandReceipt(value, companyId, missionId, requestId)
    ? {success: true, value}
    : {success: false, issues: [{path: '', message: 'mission input command receipt contains an unknown, malformed, or cross-scope field'}]};
}

export function validateTaskInputManifest(value: unknown, companyId: string, taskId: string): ValidationResult<TaskInputManifestView> {
  return isTaskInputManifestView(value, companyId, taskId)
    ? {success: true, value}
    : {success: false, issues: [{path: '', message: 'Task input manifest contains an unknown, malformed, or cross-scope field'}]};
}

export function validateActivityEvent(value: unknown): ValidationResult<ActivityEvent> {
  if (!isActivityEvent(value)) {
    return {success: false, issues: [{path: 'event', message: 'activity event contains an unknown or malformed field'}]};
  }
  return {success: true, value};
}

export function validateMissionCommandReceipt(value: unknown): ValidationResult<MissionCommandReceipt> {
  if (!isMissionCommandReceipt(value)) {
    return {success: false, issues: [{path: 'command', message: 'command receipt contains an unknown or malformed field'}]};
  }
  return {success: true, value};
}

export function validateHumanInterventionCommandReceipt(value: unknown, state: HumanInterventionState): ValidationResult<HumanInterventionCommandReceipt> {
  if (!isRecord(value)
    || typeof value.commandId !== 'string'
    || value.commandType !== `human_intervention.${state}`
    || value.targetType !== 'human_intervention'
    || typeof value.targetId !== 'string'
    || typeof value.requestId !== 'string'
    || value.accepted !== true
    || typeof value.acceptedAt !== 'string'
    || value.resultingState !== state) {
    return {success: false, issues: [{path: 'command', message: 'human intervention command receipt is malformed'}]};
  }
  return {success: true, value: value as HumanInterventionCommandReceipt};
}

export function validateCompanyList(value: unknown): ValidationResult<ReadonlyArray<CompanySummaryView>> {
  if (!Array.isArray(value)) {
    return {success: false, issues: [{path: '', message: 'company list response is not an array'}]};
  }
  const issues: Array<ValidationIssue> = [];
  value.forEach((company, index) => {
    if (!isCompanySummary(company)) {
      issues.push({path: `${index}`, message: 'company summary contains an unknown or malformed field'});
    }
  });
  if (issues.length > 0) {
    return {success: false, issues};
  }
  return {success: true, value: value as ReadonlyArray<CompanySummaryView>};
}

export function validateRuntimeSettings(value: unknown): ValidationResult<RuntimeSettingsView> {
  if (!isRuntimeSettings(value)) {
    return {success: false, issues: [{path: '', message: 'runtime settings response contains an unknown or malformed field'}]};
  }
  return {success: true, value};
}

export function validateCodexModelCatalog(value: unknown): ValidationResult<CodexModelCatalogView> {
  if (!isCodexModelCatalog(value)) {
    return {success: false, issues: [{path: '', message: 'Codex model catalog contains an unknown or malformed field'}]};
  }
  return {success: true, value};
}

export function validateOperatorInstructions(value: unknown): ValidationResult<ReadonlyArray<OperatorInstructionView>> {
  if (!Array.isArray(value)) {
    return {success: false, issues: [{path: '', message: 'operator instructions response is not an array'}]};
  }
  const issues: Array<ValidationIssue> = [];
  value.forEach((item, index) => {
    if (!isOperatorInstruction(item)) {
      issues.push({path: `${index}`, message: 'operator instruction contains an unknown or malformed field'});
    }
  });
  return issues.length > 0 ? {success: false, issues} : {success: true, value: value as ReadonlyArray<OperatorInstructionView>};
}

export function validateOperatorInstructionReceipt(value: unknown): ValidationResult<OperatorInstructionReceipt> {
  if (!isRecord(value) || !isOperatorInstruction(value)) {
    return {success: false, issues: [{path: '', message: 'operator instruction receipt contains an unknown or malformed field'}]};
  }
  const record = value as Record<string, unknown>;
  if (typeof record.requestId !== 'string' || record.requestId === '' || record.accepted !== true || typeof record.acceptedAt !== 'string') {
    return {success: false, issues: [{path: '', message: 'operator instruction receipt contains an unknown or malformed field'}]};
  }
  return {success: true, value: value as OperatorInstructionReceipt};
}

export function validateCollaboration(value: unknown): ValidationResult<ReadonlyArray<CollaborationItem>> {
  if (!Array.isArray(value)) {
    return {success: false, issues: [{path: '', message: 'collaboration response is not an array'}]};
  }
  const issues: Array<ValidationIssue> = [];
  value.forEach((item, index) => {
    if (!isCollaborationItem(item)) {
      issues.push({path: `${index}`, message: 'collaboration item contains an unknown or malformed field'});
    }
  });
  return issues.length > 0 ? {success: false, issues} : {success: true, value: value as ReadonlyArray<CollaborationItem>};
}

export function validateWorkspace(value: unknown): ValidationResult<WorkspaceView> {
  return isWorkspaceView(value) ? {success: true, value} : {success: false, issues: [{path: '', message: 'workspace response contains an unknown or malformed field'}]};
}

export function validateArtifactDetail(value: unknown): ValidationResult<ArtifactDetailView> {
  return isArtifactDetail(value) ? {success: true, value} : {success: false, issues: [{path: '', message: 'artifact response contains an unknown or malformed field'}]};
}

export function validateMissionChangeRequests(value: unknown, missionId: string): ValidationResult<ReadonlyArray<MissionChangeRequestView>> {
  if (!Array.isArray(value)) {
    return {success: false, issues: [{path: '', message: 'mission change request response is not an array'}]};
  }
  const issues: Array<ValidationIssue> = [];
  const ids = new Set<string>();
  value.forEach((item, index) => {
    if (!isMissionChangeRequest(item, missionId)) {
      issues.push({path: `${index}`, message: 'mission change request contains an unknown or malformed field'});
      return;
    }
    if (ids.has(item.changeRequestId)) {
      issues.push({path: `${index}.changeRequestId`, message: 'mission change request IDs are duplicated'});
    }
    ids.add(item.changeRequestId);
  });
  return issues.length > 0 ? {success: false, issues} : {success: true, value: value as ReadonlyArray<MissionChangeRequestView>};
}

export function validateMissionChangeRequest(value: unknown, missionId: string): ValidationResult<MissionChangeRequestView> {
  if (!isMissionChangeRequest(value, missionId)) {
    return {success: false, issues: [{path: '', message: 'mission change request contains an unknown or malformed field'}]};
  }
  return {success: true, value};
}

export function validateTaskTakeoverLeases(value: unknown, missionId: string): ValidationResult<ReadonlyArray<TaskTakeoverLeaseView>> {
  if (!Array.isArray(value)) {
    return {success: false, issues: [{path: '', message: 'Task takeover response is not an array'}]};
  }
  const issues: Array<ValidationIssue> = [];
  const ids = new Set<string>();
  value.forEach((item, index) => {
    if (!isTaskTakeoverLease(item, missionId)) {
      issues.push({path: `${index}`, message: 'Task takeover record contains an unknown, malformed, or cross-scope field'});
      return;
    }
    if (ids.has(item.leaseId)) issues.push({path: `${index}.leaseId`, message: 'Task takeover lease IDs are duplicated'});
    ids.add(item.leaseId);
  });
  return issues.length > 0 ? {success: false, issues} : {success: true, value: value as ReadonlyArray<TaskTakeoverLeaseView>};
}

export function validateTaskTakeoverLease(value: unknown, missionId: string): ValidationResult<TaskTakeoverLeaseView> {
  return isTaskTakeoverLease(value, missionId)
    ? {success: true, value}
    : {success: false, issues: [{path: '', message: 'Task takeover record contains an unknown, malformed, or cross-scope field'}]};
}

export function validateTaskTakeoverWorkspaceManifest(value: unknown, missionId: string, leaseId: string): ValidationResult<TaskTakeoverWorkspaceManifestView> {
  if (!isRecord(value) || value.leaseId !== leaseId || value.missionId !== missionId || !hasString(value, 'taskId')
    || !isTaskTakeoverWorkspaceTreeBinding(value.workspaceTree) || !Array.isArray(value.entries)
    || !value.entries.every(isTaskTakeoverWorkspaceManifestEntry)) {
    return {success: false, issues: [{path: '', message: 'takeover workspace manifest contains malformed or cross-scope data'}]};
  }
  const entries = value.entries as ReadonlyArray<TaskTakeoverWorkspaceManifestEntryView>;
  const paths = new Set(entries.map(entry => entry.relativePath));
  const totalBytes = entries.reduce((total, entry) => total + entry.bytes, 0);
  if (paths.size !== entries.length || entries.length !== value.workspaceTree.fileCount || totalBytes !== value.workspaceTree.bytes) {
    return {success: false, issues: [{path: 'entries', message: 'takeover workspace entries differ from the pinned count or byte total'}]};
  }
  return {success: true, value: value as TaskTakeoverWorkspaceManifestView};
}

export function validateTaskTakeoverWorkspaceFile(value: unknown, missionId: string, leaseId: string, relativePath: string, manifestSha256: string): ValidationResult<TaskTakeoverWorkspaceFileView> {
  if (!isRecord(value) || value.leaseId !== leaseId || value.missionId !== missionId || !hasString(value, 'taskId')
    || value.manifestSha256 !== manifestSha256 || value.relativePath !== relativePath || !isWorkspaceRelativePath(value.relativePath)
    || typeof value.sha256 !== 'string' || !/^[0-9a-f]{64}$/.test(value.sha256)
    || typeof value.bytes !== 'number' || !Number.isSafeInteger(value.bytes) || value.bytes < 1 || value.bytes > 2 * 1024 * 1024
    || typeof value.fileRevision !== 'number' || !Number.isSafeInteger(value.fileRevision) || value.fileRevision < 1
    || typeof value.workspaceRevision !== 'number' || !Number.isSafeInteger(value.workspaceRevision) || value.workspaceRevision < 1
    || value.contentType !== 'text/utf-8' || typeof value.content !== 'string' || new TextEncoder().encode(value.content).length !== value.bytes) {
    return {success: false, issues: [{path: '', message: 'takeover workspace file is malformed, oversized, or outside its frozen manifest'}]};
  }
  return {success: true, value: value as TaskTakeoverWorkspaceFileView};
}

export function validateArtifactDeliveryManifest(value: unknown, companyId: string, artifactId: string): ValidationResult<ArtifactDeliveryManifestResponse> {
  return isArtifactDeliveryManifestResponse(value, companyId, artifactId)
    ? {success: true, value}
    : {success: false, issues: [{path: '', message: 'artifact delivery manifest contains an unknown, malformed, or cross-scope field'}]};
}

export function validateDurableDelivery(value: unknown, companyId: string, artifactId: string): ValidationResult<DurableDeliveryResponse> {
  return isDurableDeliveryResponse(value, companyId, artifactId)
    ? {success: true, value}
    : {success: false, issues: [{path: '', message: 'durable delivery response contains an unknown, malformed, or cross-scope field'}]};
}

export function validateDurableDeliveryManifestCompletionReceipt(value: unknown, expected: Readonly<{requestId: string; companyId: string; artifactId: string; expectedManifestRevision: string}>): ValidationResult<DurableDeliveryManifestCompletionReceipt> {
  const expectedRevision = isCanonicalPositiveInt64(expected.expectedManifestRevision) ? BigInt(expected.expectedManifestRevision) : null;
  const returnedRevision = isRecord(value) && typeof value.manifestRevision === 'string' && isCanonicalPositiveInt64(value.manifestRevision) ? BigInt(value.manifestRevision) : null;
  if (!isRecord(value) || value.requestId !== expected.requestId || value.companyId !== expected.companyId || value.deliveryId !== expected.artifactId
    || value.state !== 'ready' || value.actor !== 'system' || expectedRevision === null || returnedRevision === null || returnedRevision !== expectedRevision + 1n
    || value.dispositionRevision !== '1' || !hasString(value, 'createdAt') || typeof value.createdAt !== 'string' || Number.isNaN(Date.parse(value.createdAt))) {
    return {success: false, issues: [{path: '', message: 'durable delivery completion receipt is malformed or cross-scoped'}]};
  }
  return {success: true, value: value as unknown as DurableDeliveryManifestCompletionReceipt};
}

export function validateDurableDeliveryManifestInvalidationReceipt(value: unknown, expected: Readonly<{requestId: string; companyId: string; artifactId: string; expectedManifestRevision: string; state: 'invalidated' | 'withdrawn'; reason: string}>): ValidationResult<DurableDeliveryManifestInvalidationReceipt> {
  const expectedRevision = isCanonicalPositiveInt64(expected.expectedManifestRevision) ? BigInt(expected.expectedManifestRevision) : null;
  const returnedRevision = isRecord(value) && typeof value.manifestRevision === 'string' && isCanonicalPositiveInt64(value.manifestRevision) ? BigInt(value.manifestRevision) : null;
  if (!isRecord(value) || value.requestId !== expected.requestId || value.companyId !== expected.companyId || value.deliveryId !== expected.artifactId
    || value.state !== expected.state || value.actor !== 'installation-owner' || value.reason !== expected.reason || expectedRevision === null || returnedRevision === null || returnedRevision !== expectedRevision + 1n
    || value.dispositionRevision !== '1' || !isTimestamp(value.createdAt)) {
    return {success: false, issues: [{path: '', message: 'durable delivery invalidation receipt is malformed or does not match the submitted intent'}]};
  }
  return {success: true, value: value as unknown as DurableDeliveryManifestInvalidationReceipt};
}

export function validateDurableUserDispositionReceipt(value: unknown, expected: Readonly<{
  requestId: string;
  companyId: string;
  artifactId: string;
  deliveryId: string;
  manifestRevision: string;
  dispositionRevision: string;
  state: UserDispositionDecision;
  reason: string;
}>): ValidationResult<DurableUserDispositionCommandReceipt> {
  const nextDispositionRevision = incrementDecimalRevision(expected.dispositionRevision);
  if (!isRecord(value)
    || value.requestId !== expected.requestId
    || value.companyId !== expected.companyId
    || expected.artifactId === ''
    || value.deliveryId !== expected.deliveryId
    || value.manifestRevision !== expected.manifestRevision
    || nextDispositionRevision === null
    || value.dispositionRevision !== nextDispositionRevision
    || value.state !== expected.state
    || value.actor !== 'installation-owner'
    || value.reason !== expected.reason
    || !isTimestamp(value.createdAt)) {
    return {success: false, issues: [{path: '', message: 'durable user disposition receipt is malformed or does not match the submitted intent'}]};
  }
  return {success: true, value: value as unknown as DurableUserDispositionCommandReceipt};
}

function incrementDecimalRevision(value: string): string | null {
  if (!isCanonicalPositiveInt64(value)) return null;
  const digits = value.split('');
  for (let index = digits.length - 1; index >= 0; index -= 1) {
    if (digits[index] !== '9') {
      digits[index] = String(Number(digits[index]) + 1);
      return digits.join('');
    }
    digits[index] = '0';
  }
  const next = digits.length < 19 ? `1${digits.join('')}` : null;
  return isCanonicalPositiveInt64(next) ? next : null;
}

export function validateOperations(value: unknown): ValidationResult<OperationsView> {
  return isOperations(value) ? {success: true, value} : {success: false, issues: [{path: '', message: 'operations response contains an unknown or malformed field'}]};
}

export function validateNotifications(value: unknown): ValidationResult<NotificationsView> {
  return isNotifications(value) ? {success: true, value} : {success: false, issues: [{path: '', message: 'notifications response contains an unknown or malformed field'}]};
}

export function validateCompanyFeedback(value: unknown, companyId: string): ValidationResult<CompanyFeedbackView> {
  return isCompanyFeedback(value, companyId) ? {success: true, value} : {success: false, issues: [{path: '', message: 'feedback response contains an unknown, malformed, oversized, or cross-scope field'}]};
}

export function validateGitHubCredentialReceipt(value: unknown, expectedStored: boolean): ValidationResult<GitHubCredentialReceipt> {
  return isRecord(value) && value.credentialRef === 'default-readonly' && value.stored === expectedStored
    ? {success: true, value: {credentialRef: 'default-readonly', stored: expectedStored}}
    : {success: false, issues: [{path: '', message: 'GitHub credential response is malformed'}]};
}

export function validateGitHubFeedbackSourceReceipt(value: unknown, companyId: string): ValidationResult<GitHubFeedbackSourceCommandReceipt> {
  if (!isRecord(value) || value.companyId !== companyId || !hasString(value, 'sourceId') || !hasString(value, 'repositoryId') || !hasString(value, 'repository') || !hasString(value, 'profileRevision') || !hasString(value, 'filterRevision') || !isSha256(value.configurationSha256) || !hasString(value, 'state') || !hasString(value, 'permissionStatus') || !hasString(value, 'createdAt') || (value.permissionProbeSha256 !== undefined && value.permissionProbeSha256 !== '' && !isSha256(value.permissionProbeSha256))) {
    return {success: false, issues: [{path: '', message: 'GitHub source receipt contains an unknown, malformed or cross-scope field'}]};
  }
  const receipt: GitHubFeedbackSourceCommandReceipt = {
    companyId,
    sourceId: value.sourceId as string,
    repositoryId: value.repositoryId as string,
    repository: value.repository as string,
    profileRevision: value.profileRevision as string,
    filterRevision: value.filterRevision as string,
    configurationSha256: value.configurationSha256 as string,
    state: value.state as string,
    permissionStatus: value.permissionStatus as string,
    createdAt: value.createdAt as string,
  };
  if (typeof value.permissionProbeSha256 === 'string' && value.permissionProbeSha256 !== '') {
    return {success: true, value: {...receipt, permissionProbeSha256: value.permissionProbeSha256}};
  }
  return {success: true, value: receipt};
}

export function validateGitHubFeedbackProbeReceipt(value: unknown, companyId: string, sourceId: string, requestId: string): ValidationResult<GitHubFeedbackProbeReceipt> {
  return isRecord(value) && value.companyId === companyId && value.sourceId === sourceId && value.requestId === requestId && hasString(value, 'permissionStatus') && hasString(value, 'coverage') && typeof value.coverageReason === 'string'
    ? {success: true, value: value as unknown as GitHubFeedbackProbeReceipt}
    : {success: false, issues: [{path: '', message: 'GitHub permission-probe receipt contains an unknown, malformed or cross-scope field'}]};
}

export function validateGitHubFeedbackPollReceipt(value: unknown, companyId: string, sourceId: string, requestId: string): ValidationResult<GitHubFeedbackPollReceipt> {
  return isRecord(value)
    && value.companyId === companyId
    && value.sourceId === sourceId
    && value.requestId === requestId
    && hasString(value, 'scanId')
    && hasString(value, 'coverage')
    && typeof value.coverageReason === 'string'
    && isNullableString(value.coveredThrough)
    && Number.isSafeInteger(value.pageCount) && (value.pageCount as number) >= 0
    && Number.isSafeInteger(value.itemCount) && (value.itemCount as number) >= 0
    && Number.isSafeInteger(value.commentScanCount) && (value.commentScanCount as number) >= 0
    && hasString(value, 'commentCoverage')
    && typeof value.commentCoverageReason === 'string'
    && typeof value.replayed === 'boolean'
    ? {success: true, value: value as unknown as GitHubFeedbackPollReceipt}
    : {success: false, issues: [{path: '', message: 'GitHub poll receipt contains an unknown, malformed or cross-scope field'}]};
}

export function validateGitHubFeedbackBacklogStatusReceipt(value: unknown, expected: Readonly<{companyId: string; sourceId: string; providerItemId: string; revisionSha256: string; requestId: string}>): ValidationResult<GitHubFeedbackBacklogStatusReceipt> {
  return isRecord(value)
    && value.companyId === expected.companyId
    && value.sourceId === expected.sourceId
    && value.providerItemId === expected.providerItemId
    && value.revisionSha256 === expected.revisionSha256
    && value.requestId === expected.requestId
    && Number.isSafeInteger(value.issueNumber) && (value.issueNumber as number) > 0
    && isSha256(value.revisionSha256)
    && isOneOf(value.status, ['open', 'needs_review', 'triaging', 'waiting', 'handled', 'archived'] satisfies ReadonlyArray<GitHubFeedbackBacklogStatus>)
    && hasString(value, 'rationale')
    && hasString(value, 'eventId')
    && hasString(value, 'remoteState')
    && value.remoteUnchanged === true
    ? {success: true, value: value as unknown as GitHubFeedbackBacklogStatusReceipt}
    : {success: false, issues: [{path: '', message: 'GitHub backlog receipt contains an unknown, malformed or cross-scope field'}]};
}

export function validateGitHubFeedbackCollectionPolicyReceipt(value: unknown, expected: Readonly<{companyId: string; sourceId: string; requestId: string}>): ValidationResult<GitHubFeedbackCollectionPolicyReceipt> {
  return isRecord(value)
    && value.companyId === expected.companyId
    && value.sourceId === expected.sourceId
    && value.requestId === expected.requestId
    && typeof value.enabled === 'boolean'
    && Number.isSafeInteger(value.intervalSeconds)
    && (value.intervalSeconds as number) >= 900
    && (value.intervalSeconds as number) <= 86400
    && hasString(value, 'rationale')
    && hasString(value, 'updatedAt')
    && isNullableString(value.nextPollAt)
    && typeof value.lastAttempt === 'string'
    && typeof value.lastReasonCode === 'string'
    && typeof value.externalEnabled === 'boolean'
    && typeof value.schedulerEnabled === 'boolean'
    ? {success: true, value: value as unknown as GitHubFeedbackCollectionPolicyReceipt}
    : {success: false, issues: [{path: '', message: 'GitHub collection policy receipt contains an unknown, malformed or cross-scope field'}]};
}

function isSkillRevision(value: unknown, companyId: string): boolean {
  return isRecord(value)
    && value.companyId === companyId
    && hasString(value, 'id')
    && isOneOf(value.publisherScope, ['group', 'company'])
    && hasString(value, 'packageId')
    && hasString(value, 'revision')
    && hasString(value, 'displayName')
    && hasString(value, 'sourceRef')
    && hasString(value, 'contentDigest')
    && isOneOf(value.status, ['candidate', 'approved', 'revoked'])
    && hasString(value, 'createdAt');
}

function isMCPServerDefinition(value: unknown, companyId: string): boolean {
  return isRecord(value)
    && value.companyId === companyId
    && hasString(value, 'id')
    && hasString(value, 'name')
    && isOneOf(value.transport, ['stdio', 'streamable_http'])
    && isNullableString(value.endpoint)
    && isNullableString(value.command)
    && Array.isArray(value.args)
    && value.args.every(item => typeof item === 'string')
    && hasString(value, 'descriptorDigest')
    && isOneOf(value.status, ['unverified', 'candidate', 'approved', 'revoked'])
    && hasString(value, 'createdAt');
}

function isCapabilityQualification(value: unknown, companyId: string): value is CapabilityQualificationView {
  return isRecord(value)
    && value.companyId === companyId
    && hasString(value, 'qualificationId')
    && isOneOf(value.capabilityKind, ['skill', 'mcp'])
    && hasString(value, 'capabilityId')
    && typeof value.versionDigest === 'string' && /^[0-9a-f]{64}$/.test(value.versionDigest)
    && isOneOf(value.profile, ['read_only_skill@1', 'stdio_mcp@1', 'streamable_http_mcp_2026_07_28@1'])
    && isOneOf(value.status, ['metadata_verified', 'needs_external_qualification', 'failed'])
    && typeof value.evidenceDigest === 'string' && /^[0-9a-f]{64}$/.test(value.evidenceDigest)
    && hasString(value, 'createdAt');
}

function isSafeMCPPackagePath(value: unknown): value is string {
  return typeof value === 'string' && value.length > 0 && new TextEncoder().encode(value).length <= 1024
    && !value.startsWith('/') && !value.includes('\\') && !value.includes(':') && !value.includes('\0')
    && value.split('/').every(segment => segment !== '' && segment !== '.' && segment !== '..');
}

function isStdioMCPPackageFile(value: unknown): value is StdioMCPPackageFileView {
  return isRecord(value)
    && isSafeMCPPackagePath(value.relativePath)
    && hasString(value, 'mediaType')
    && Number.isSafeInteger(value.byteSize) && (value.byteSize as number) > 0
    && typeof value.contentSHA256 === 'string' && /^[0-9a-f]{64}$/.test(value.contentSHA256);
}

function isStdioMCPPackageManifest(value: unknown): value is StdioMCPPackageManifestView {
  if (!isRecord(value) || value.schemaVersion !== 'polis-controlled-stdio-mcp@1'
    || !hasString(value, 'name') || !hasString(value, 'serverName') || !hasString(value, 'serverVersion')
    || !isSafeMCPPackagePath(value.command) || !isSafeMCPPackagePath(value.entryPoint)
    || !Array.isArray(value.args) || !value.args.every(argument => typeof argument === 'string' && argument.length <= 1024 && !argument.includes('\0'))
    || !Array.isArray(value.files) || value.files.length === 0 || value.files.length > 63
    || !value.files.every(isStdioMCPPackageFile)) {
    return false;
  }
  const filePaths = value.files.map(file => file.relativePath);
  if (new Set(filePaths.map(path => path.toLowerCase())).size !== filePaths.length
    || filePaths.some((path, index) => index > 0 && filePaths[index - 1] >= path)
    || !filePaths.includes(value.command) || !filePaths.includes(value.entryPoint)) {
    return false;
  }
  return true;
}

function isStdioMCPPackageRevision(value: unknown, companyId: string, servers: ReadonlyArray<unknown>): value is StdioMCPPackageRevisionView {
  if (!isRecord(value) || value.companyId !== companyId || !hasString(value, 'id') || !hasString(value, 'serverId')
    || !hasString(value, 'revision') || typeof value.manifestDigest !== 'string' || !/^[0-9a-f]{64}$/.test(value.manifestDigest)
    || !isStdioMCPPackageManifest(value.manifest) || !hasString(value, 'createdAt')) {
    return false;
  }
  const manifest = value.manifest;
  const server = servers.find(candidate => isRecord(candidate) && candidate.id === value.serverId);
  return isRecord(server) && server.transport === 'stdio' && server.name === manifest.name
    && server.command === manifest.command && Array.isArray(server.args)
    && server.args.length === manifest.args.length
    && server.args.every((argument, index) => argument === manifest.args[index]);
}

function isEmployeeCapabilityBinding(value: unknown, companyId: string): value is EmployeeCapabilityBindingView {
  return isRecord(value)
    && value.companyId === companyId
    && hasString(value, 'eventId')
    && hasString(value, 'employeeId')
    && isOneOf(value.capabilityKind, ['skill', 'mcp'])
    && hasString(value, 'capabilityId')
    && typeof value.versionDigest === 'string' && /^[0-9a-f]{64}$/.test(value.versionDigest)
    && hasString(value, 'qualificationId')
    && isOneOf(value.qualificationStatus, ['metadata_verified', 'needs_external_qualification', 'failed'])
    && isOneOf(value.state, ['bound', 'revoked'])
    && isOneOf(value.executionStatus, ['runtime_unqualified', 'runtime_qualified_dispatch_unavailable', 'runtime_qualified_dispatch_disabled', 'runtime_qualified_dispatch_available'])
    && typeof value.reason === 'string'
    && hasString(value, 'createdAt');
}

function isStdioMCPRuntimeQualification(value: unknown, companyId: string): value is StdioMCPRuntimeQualificationView {
  if (!isRecord(value)) return false;
  const digestFields = ['versionDigest', 'descriptorDigest', 'toolSchemaSha256', 'evidenceDigest'];
  const isHTTP = value.transport === 'streamable_http';
  const isStdio = value.transport === undefined || value.transport === 'stdio';
  return value.companyId === companyId
    && hasString(value, 'runtimeQualificationId')
    && hasString(value, 'capabilityId')
    && hasString(value, 'capabilityQualificationId')
    && digestFields.every(field => typeof value[field] === 'string' && /^[0-9a-f]{64}$/.test(value[field] as string))
    && ((isStdio
      && value.runtimeProfile === 'polis-controlled-stdio-mcp-2026-07-28@1'
      && value.hostOs === 'windows'
      && value.hostProfile === 'windows_appcontainer_deny_all@1'
      && typeof value.commandSha256 === 'string' && /^[0-9a-f]{64}$/.test(value.commandSha256)
      && typeof value.packageManifestSha256 === 'string' && /^[0-9a-f]{64}$/.test(value.packageManifestSha256))
      || (isHTTP
        && value.runtimeProfile === 'polis-streamable-http-mcp-2026-07-28@1'
        && value.hostOs === 'remote'
        && value.hostProfile === 'https_public_dns_pinned@1'
        && value.commandSha256 === '' && value.packageManifestSha256 === ''
        && typeof value.endpoint === 'string' && value.endpoint.startsWith('https://')))
    && hasString(value, 'serverName')
    && hasString(value, 'serverVersion')
    && value.protocolVersion === '2026-07-28'
    && isOneOf(value.status, ['observed_unqualified', 'qualified', 'schema_drift', 'revoked'])
    && hasString(value, 'createdAt');
}

function isCapabilityDecision(value: unknown, companyId: string): value is CapabilityDecisionView {
  return isRecord(value)
    && value.companyId === companyId
    && hasString(value, 'decisionId')
    && isOneOf(value.capabilityKind, ['skill', 'mcp'])
    && hasString(value, 'capabilityId')
    && typeof value.versionDigest === 'string' && /^[0-9a-f]{64}$/.test(value.versionDigest)
    && hasString(value, 'qualificationId')
    && isOneOf(value.decision, ['approved', 'revoked'])
    && hasString(value, 'rationale')
    && hasString(value, 'actor')
    && hasString(value, 'createdAt');
}

function isNonNegativeSafeInteger(value: unknown): value is number {
  return typeof value === 'number' && Number.isSafeInteger(value) && value >= 0;
}

function isCapabilityRevocationSession(value: unknown): value is CapabilityRevocationSessionView {
  return isRecord(value)
    && hasString(value, 'sessionId')
    && hasString(value, 'employeeId')
    && hasString(value, 'taskId')
    && hasString(value, 'missionId')
    && hasString(value, 'state')
    && (value.stateAtRevocation === undefined || typeof value.stateAtRevocation === 'string')
    && isNonNegativeSafeInteger(value.skillLoadCount)
    && isNonNegativeSafeInteger(value.mcpCallCount)
    && isNonNegativeSafeInteger(value.dispatchingMcpCallCount);
}

function isCapabilityRevocationMCPCall(value: unknown): value is CapabilityRevocationMCPCallView {
  return isRecord(value)
    && hasString(value, 'intentId')
    && hasString(value, 'sessionId')
    && hasString(value, 'employeeId')
    && hasString(value, 'toolName')
    && isOneOf(value.status, ['dispatching', 'completed', 'outcome_unknown'])
    && (value.statusAtRevocation === undefined || isOneOf(value.statusAtRevocation, ['dispatching', 'completed', 'outcome_unknown']))
    && (value.reasonCode === undefined || typeof value.reasonCode === 'string')
    && hasString(value, 'createdAt');
}

function isCapabilityRevocationOwnerReview(value: unknown): value is NonNullable<CapabilityRevocationStatusView['ownerReview']> {
  return isRecord(value)
    && value.disposition === 'acknowledged_unresolved'
    && hasString(value, 'rationale')
    && typeof value.rationale === 'string' && value.rationale.trim() !== ''
    && value.actor === 'installation-owner'
    && hasString(value, 'reviewedAt');
}

function isCapabilityRevocationStatus(value: unknown, companyId: string): value is CapabilityRevocationStatusView {
  return isRecord(value)
    && value.companyId === companyId
    && hasString(value, 'revocationId')
    && isOneOf(value.scope, ['capability', 'employee'])
    && isOneOf(value.capabilityKind, ['skill', 'mcp'])
    && hasString(value, 'capabilityId')
    && typeof value.versionDigest === 'string' && /^[0-9a-f]{64}$/.test(value.versionDigest)
    && hasString(value, 'qualificationId')
    && (value.employeeId === undefined || typeof value.employeeId === 'string')
    && hasString(value, 'reason')
    && hasString(value, 'actor')
    && hasString(value, 'acceptedAt')
    && typeof value.revocationAccepted === 'boolean'
    && typeof value.effectiveForNewDispatch === 'boolean'
    && typeof value.quiesced === 'boolean'
    && typeof value.sessionInventoryComplete === 'boolean'
    && (value.ownerReview === undefined || isCapabilityRevocationOwnerReview(value.ownerReview))
    && isNonNegativeSafeInteger(value.affectedSessionCount)
    && isNonNegativeSafeInteger(value.liveSessionCount)
    && Array.isArray(value.sessions) && value.sessions.every(isCapabilityRevocationSession)
    && typeof value.sessionsTruncated === 'boolean'
    && isNonNegativeSafeInteger(value.mcpCallCount)
    && isNonNegativeSafeInteger(value.dispatchingMcpCallCount)
    && Array.isArray(value.mcpCalls) && value.mcpCalls.every(isCapabilityRevocationMCPCall)
    && typeof value.mcpCallsTruncated === 'boolean';
}

export function validateCapabilityCatalog(value: unknown, companyId: string): ValidationResult<CapabilityCatalogView> {
  if (!isRecord(value) || !Array.isArray(value.skills) || !Array.isArray(value.mcpServers) || !Array.isArray(value.mcpPackages) || !Array.isArray(value.qualifications) || !Array.isArray(value.bindings) || !Array.isArray(value.decisions) || !Array.isArray(value.runtimeQualifications)
    || (value.runtimeObservationAvailable !== undefined && typeof value.runtimeObservationAvailable !== 'boolean')
    || (value.streamableHttpRuntimeObservationAvailable !== undefined && typeof value.streamableHttpRuntimeObservationAvailable !== 'boolean')
    || (value.revocations !== undefined && (!Array.isArray(value.revocations) || !value.revocations.every(item => isCapabilityRevocationStatus(item, companyId))))
    || (value.revocationsTruncated !== undefined && typeof value.revocationsTruncated !== 'boolean')
    || !value.skills.every(item => isSkillRevision(item, companyId)) || !value.mcpServers.every(item => isMCPServerDefinition(item, companyId))
    || !value.mcpPackages.every(item => isStdioMCPPackageRevision(item, companyId, value.mcpServers as ReadonlyArray<unknown>))
    || !value.qualifications.every(item => isCapabilityQualification(item, companyId))
    || !value.bindings.every(item => isEmployeeCapabilityBinding(item, companyId))
    || !value.decisions.every(item => isCapabilityDecision(item, companyId))
    || !value.runtimeQualifications.every(item => isStdioMCPRuntimeQualification(item, companyId))) {
    return {success: false, issues: [{path: '', message: 'capability catalog contains an unknown or malformed field'}]};
  }
  for (const runtime of value.runtimeQualifications) {
    const server = value.mcpServers.find(item => isRecord(item) && item.id === runtime.capabilityId);
    const runtimeTransport = runtime.transport ?? 'stdio';
    if (!isRecord(server) || server.transport !== runtimeTransport || server.descriptorDigest !== runtime.versionDigest
      || (runtimeTransport === 'streamable_http' && server.endpoint !== runtime.endpoint)) {
      return {success: false, issues: [{path: 'runtimeQualifications', message: 'runtime qualification does not match its current MCP descriptor'}]};
    }
  }
  return {success: true, value: {
    ...value,
    revocations: Array.isArray(value.revocations) ? value.revocations : [],
    revocationsTruncated: value.revocationsTruncated === true,
    runtimeObservationAvailable: value.runtimeObservationAvailable === true,
    streamableHttpRuntimeObservationAvailable: value.streamableHttpRuntimeObservationAvailable === true,
  } as unknown as CapabilityCatalogView};
}

export function validateStdioMCPPackageRevision(value: unknown, companyId: string): ValidationResult<StdioMCPPackageRevisionView> {
  if (!isRecord(value) || value.companyId !== companyId || !hasString(value, 'id') || !hasString(value, 'serverId')
    || !hasString(value, 'revision') || typeof value.manifestDigest !== 'string' || !/^[0-9a-f]{64}$/.test(value.manifestDigest)
    || !isStdioMCPPackageManifest(value.manifest) || !hasString(value, 'createdAt')) {
    return {success: false, issues: [{path: '', message: 'stdio MCP package revision is malformed or belongs to another company'}]};
  }
  return {success: true, value: value as StdioMCPPackageRevisionView};
}

function isDomainEvidenceArea(value: unknown): value is DomainEvidenceAreaView {
  return isOneOf(value, ['quality', 'intervention', 'recovery', 'cost', 'organization_benefit']);
}

function isDomainWorkflowProfile(value: unknown): value is DomainWorkflowProfileView {
  if (!isRecord(value) || !isOneOf(value.qualificationStatus, ['not_run', 'qualified', 'revoked']) || value.executionEnabled !== false
    || !Array.isArray(value.stages) || value.stages.length === 0 || !value.stages.every(stage => typeof stage === 'string' && stage.trim() !== '')
    || !Array.isArray(value.requiredEvidence) || !value.requiredEvidence.every(isDomainEvidenceArea)) {
    return false;
  }
  const requiredEvidence = value.requiredEvidence;
  const contentAreas = ['quality', 'intervention', 'recovery', 'cost', 'organization_benefit'];
  const researchAreas = ['quality', 'recovery', 'cost', 'organization_benefit'];
  const hasAreas = (expected: ReadonlyArray<string>) => requiredEvidence.length === expected.length && expected.every((area, index) => requiredEvidence[index] === area);
  return (value.id === 'content-operations-reference' && value.revision === 'content-operations@1' && value.domain === 'content_operations' && hasAreas(contentAreas))
    || (value.id === 'research-simulation-reference' && value.revision === 'research-simulation@1' && value.domain === 'research_simulation' && hasAreas(researchAreas));
}

function isDomainProfileQualificationRecord(value: unknown, companyId: string): value is DomainProfileQualificationRecordView {
  if (!isRecord(value) || value.companyId !== companyId || !hasString(value, 'eventId') ||
    !isOneOf(value.decision, ['qualified', 'revoked']) || !hasString(value, 'profileId') || !hasString(value, 'profileRevision') ||
    typeof value.rationale !== 'string' || value.rationale.trim() === '' || value.actor !== 'local-owner' ||
    !hasString(value, 'requestId') || !isTimestamp(value.createdAt) || typeof value.evidenceInputId !== 'string' ||
    !isDecimalCounter(value.evidenceInputRevision) || typeof value.evidenceSha256 !== 'string') {
    return false;
  }
  const profileRevision = value.profileId === 'content-operations-reference' ? 'content-operations@1'
    : value.profileId === 'research-simulation-reference' ? 'research-simulation@1' : null;
  if (profileRevision === null || value.profileRevision !== profileRevision) return false;
  if (value.decision === 'qualified') {
    return value.evidenceInputId.trim() !== '' && value.evidenceInputRevision !== '0' && SHA256_PATTERN.test(value.evidenceSha256);
  }
  return value.evidenceInputId === '' && value.evidenceInputRevision === '0' && value.evidenceSha256 === '';
}

function isResearchSimulationRun(value: unknown, companyId: string): value is ResearchSimulationRunView {
  if (!isRecord(value) || !isRecord(value.output)) return false;
  const output = value.output;
  return value.companyId === companyId
    && hasString(value, 'runId')
    && value.profileId === 'research-simulation-reference'
    && value.profileRevision === 'research-simulation@1'
    && isPositiveIntegerString(value.protocolRevision)
    && hasString(value, 'datasetInputId')
    && isPositiveIntegerString(value.datasetInputRevision)
    && isSha256(value.datasetSha256)
    && hasString(value, 'methodInputId')
    && isPositiveIntegerString(value.methodInputRevision)
    && isSha256(value.methodSha256)
    && isUnsignedIntegerString(value.seed)
    && hasString(value, 'controlDefinition')
    && value.riskUnit === 'sample_draw'
    && isPositiveIntegerString(value.riskBudgetUnits)
    && BigInt(value.riskBudgetUnits as string) <= 5_120_000n
    && isPositiveIntegerString(value.riskConsumedUnits)
    && BigInt(value.riskConsumedUnits as string) <= BigInt(value.riskBudgetUnits as string)
    && isSha256(value.outputSha256)
    && hasString(value, 'requestId')
    && hasString(value, 'createdAt')
    && output.schemaVersion === 'polis-research-simulation-output@1'
    && output.algorithm === 'bootstrap-mean-difference@1'
    && isSafeIntegerNumber(output.protocolRevision) && output.protocolRevision > 0
    && output.datasetSha256 === value.datasetSha256
    && output.methodSha256 === value.methodSha256
    && output.seed === value.seed
    && output.controlDefinition === value.controlDefinition
    && isSafeIntegerNumber(output.iterations) && output.iterations > 0 && output.iterations <= 10000
    && isSafeIntegerNumber(output.sampleSize) && output.sampleSize > 0 && output.sampleSize <= 256
    && isSafeIntegerNumber(output.riskConsumedUnits) && output.riskConsumedUnits > 0
    && output.riskConsumedUnits === 2 * output.iterations * output.sampleSize
    && String(output.riskConsumedUnits) === value.riskConsumedUnits
    && output.riskUnit === value.riskUnit
    && isFiniteNumber(output.controlMean) && isFiniteNumber(output.treatmentMean) && isFiniteNumber(output.meanDifference)
    && isFiniteNumber(output.minimumDifference) && isFiniteNumber(output.maximumDifference)
    && output.minimumDifference <= output.maximumDifference;
}

function isDomainContentSourceEvent(value: unknown, companyId: string): value is DomainContentSourceEventView {
  return isRecord(value) && value.companyId === companyId
    && hasString(value, 'eventSeq') && isPositiveIntegerString(value.eventSeq)
    && hasString(value, 'eventId') && hasString(value, 'inputId') && isPositiveIntegerString(value.revision)
    && isSha256(value.sha256) && isOneOf(value.state, ['authorized', 'revoked'])
    && typeof value.rationale === 'string' && value.rationale.trim() !== '' && Array.from(value.rationale).length <= 1000
    && hasString(value, 'requestId') && hasString(value, 'createdAt');
}

function isDomainContentDraft(value: unknown, companyId: string): value is DomainContentDraftView {
  if (!isRecord(value) || value.companyId !== companyId || !hasString(value, 'draftId') || !hasString(value, 'draftInputId')
    || !isPositiveIntegerString(value.draftRevision) || !isSha256(value.draftSha256) || !hasString(value, 'writerEmployeeId')
    || !Array.isArray(value.criticalClaims) || value.criticalClaims.length === 0 || value.criticalClaims.length > 100
    || typeof value.constraintsPassed !== 'boolean' || !hasString(value, 'requestId') || !hasString(value, 'createdAt')) return false;
  const claimIDs = value.criticalClaims.filter((claimId): claimId is string => typeof claimId === 'string');
  return claimIDs.length === value.criticalClaims.length && claimIDs.every(claimId => ID_PATTERN.test(claimId))
    && new Set(claimIDs).size === claimIDs.length;
}

function isDomainContentSourceReference(value: unknown): value is DomainContentSourceReferenceView {
  return isRecord(value) && hasString(value, 'inputId') && isPositiveIntegerString(value.revision) && isSha256(value.sha256);
}

function isDomainContentClaimReview(value: unknown): value is DomainContentClaimReviewView {
  return isRecord(value) && typeof value.claimId === 'string' && ID_PATTERN.test(value.claimId)
    && isOneOf(value.finding, ['verified', 'inconclusive', 'contradicted'])
    && Array.isArray(value.sources) && value.sources.every(isDomainContentSourceReference)
    && (value.finding !== 'verified' || value.sources.length > 0)
    && (value.finding !== 'inconclusive' || (typeof value.limitation === 'string' && value.limitation.trim() !== '' && Array.from(value.limitation).length <= 1000))
    && (value.limitation === undefined || typeof value.limitation === 'string');
}

function isDomainContentReviewSubmission(value: unknown): value is DomainContentReviewSubmissionView {
  return isRecord(value) && isPositiveIntegerString(value.draftRevision)
    && hasString(value, 'checkerEmployeeId') && typeof value.humanSampled === 'boolean'
    && Array.isArray(value.claims) && value.claims.length > 0 && value.claims.length <= 100
    && value.claims.every(isDomainContentClaimReview)
    && new Set(value.claims.map(claim => claim.claimId)).size === value.claims.length;
}

function isDomainContentSampleEvidence(value: unknown): value is DomainContentSampleEvidenceView {
  return isRecord(value) && isPositiveIntegerString(value.draftRevision)
    && isSha256(value.draftSha256) && isDomainContentSourceReference(value.plan)
    && hasString(value, 'sampledByEmployeeId') && Array.isArray(value.sampledClaimIds)
    && value.sampledClaimIds.length > 0 && value.sampledClaimIds.length <= 100
    && value.sampledClaimIds.every(claimId => typeof claimId === 'string' && ID_PATTERN.test(claimId))
    && new Set(value.sampledClaimIds).size === value.sampledClaimIds.length;
}

function isDomainContentReview(value: unknown, companyId: string): value is DomainContentReviewView {
  if (!isRecord(value) || value.companyId !== companyId || !hasString(value, 'reviewId') || !hasString(value, 'draftInputId')
    || !isPositiveIntegerString(value.draftRevision) || !isSha256(value.draftSha256) || typeof value.correctionId !== 'string' || !hasString(value, 'checkerEmployeeId')
    || !isOneOf(value.outcome, ['accepted', 'inconclusive', 'rejected']) || typeof value.stale !== 'boolean'
    || !isDomainContentReviewSubmission(value.review) || !isDomainContentSampleEvidence(value.sample)
    || !Array.isArray(value.reasonCodes) || !value.reasonCodes.every(item => typeof item === 'string')
    || !hasString(value, 'requestId') || !hasString(value, 'createdAt')) return false;
  const review = value.review as DomainContentReviewSubmissionView;
  const sample = value.sample as DomainContentSampleEvidenceView;
  return review.draftRevision === value.draftRevision
    && review.checkerEmployeeId === value.checkerEmployeeId
    && sample.draftRevision === value.draftRevision
    && sample.draftSha256 === value.draftSha256
    && sample.sampledByEmployeeId === value.checkerEmployeeId
    && sample.sampledClaimIds.every(claimId => review.claims.some(claim => claim.claimId === claimId));
}

function isDomainContentPublication(value: unknown, companyId: string): value is DomainContentPublicationView {
  if (!isRecord(value) || !isRecord(value.receipt) || value.companyId !== companyId || !hasString(value, 'publicationId')
    || !hasString(value, 'reviewId') || !hasString(value, 'draftInputId') || !isPositiveIntegerString(value.draftRevision)
    || !isSha256(value.draftSha256) || value.mode !== 'simulation' || value.externalSideEffects !== false
    || !isSha256(value.receiptSha256) || !hasString(value, 'requestId') || !hasString(value, 'createdAt')) return false;
  const receipt = value.receipt;
  return receipt.schemaVersion === 'polis-content-publication-simulation@1' && receipt.mode === 'simulation'
    && receipt.publicationId === value.publicationId && receipt.reviewId === value.reviewId
    && receipt.draftInputId === value.draftInputId && receipt.draftRevision === value.draftRevision
    && receipt.draftSha256 === value.draftSha256 && receipt.externalSideEffects === false;
}

function isDomainContentCorrection(value: unknown, companyId: string): value is DomainContentCorrectionView {
  return isRecord(value) && value.companyId === companyId && hasString(value, 'correctionId') && hasString(value, 'publicationId')
    && hasString(value, 'correctionDraftInputId') && isPositiveIntegerString(value.correctionDraftRevision)
    && isSha256(value.correctionDraftSha256) && typeof value.rationale === 'string' && value.rationale.trim() !== ''
    && Array.from(value.rationale).length <= 2000 && value.state === 'review_required'
    && hasString(value, 'requestId') && hasString(value, 'createdAt');
}

function isDomainContentFeedback(value: unknown, companyId: string): value is DomainContentFeedbackView {
  return isRecord(value) && value.companyId === companyId && hasString(value, 'feedbackId') && hasString(value, 'publicationId')
    && isOneOf(value.category, ['positive', 'negative', 'mixed', 'inconclusive', 'correction_requested'])
    && typeof value.note === 'string' && value.note.trim() !== '' && Array.from(value.note).length <= 2000
    && isOneOf(value.state, ['recorded', 'review_required']) && hasString(value, 'requestId') && hasString(value, 'createdAt')
    && ((value.category === 'negative' || value.category === 'correction_requested') ? value.state === 'review_required' : true);
}

function isSafeIntegerNumber(value: unknown): value is number {
  return typeof value === 'number' && Number.isSafeInteger(value);
}

function isFiniteNumber(value: unknown): value is number {
  return typeof value === 'number' && Number.isFinite(value);
}

function isPositiveIntegerString(value: unknown): value is string {
  return typeof value === 'string' && /^[1-9]\d{0,18}$/.test(value);
}

function isUnsignedIntegerString(value: unknown): value is string {
  return typeof value === 'string' && /^(0|[1-9]\d{0,19})$/.test(value);
}

function isDomainEvidenceSubmission(value: unknown): value is DomainEvidenceSubmissionView {
  return isRecord(value)
    && hasString(value, 'profileId')
    && hasString(value, 'profileRevision')
    && Array.isArray(value.evidence)
    && value.evidence.every(item => isRecord(item)
      && isDomainEvidenceArea(item.area)
      && isRecord(item.artifact)
      && hasString(item.artifact, 'id')
      && Number.isSafeInteger(item.artifact.revision)
      && (item.artifact.revision as number) > 0
      && typeof item.artifact.sha256 === 'string' && /^[0-9a-f]{64}$/.test(item.artifact.sha256)
      && typeof item.methodSHA256 === 'string' && /^[0-9a-f]{64}$/.test(item.methodSHA256)
      && hasString(item, 'assessedByEmployeeId'));
}

function domainEvidenceAreasForProfile(profileId: string, profileRevision: string): ReadonlyArray<DomainEvidenceAreaView> | null {
  if (profileId === 'content-operations-reference' && profileRevision === 'content-operations@1') {
    return ['quality', 'intervention', 'recovery', 'cost', 'organization_benefit'];
  }
  if (profileId === 'research-simulation-reference' && profileRevision === 'research-simulation@1') {
    return ['quality', 'recovery', 'cost', 'organization_benefit'];
  }
  return null;
}

function isDomainEvidenceReviewRecord(value: unknown, companyId: string): value is DomainEvidenceReviewRecordView {
  if (!isRecord(value)
    || value.companyId !== companyId
    || !hasString(value, 'reviewId')
    || !hasString(value, 'recordId')
    || !isOneOf(value.outcome, ['evidence_references_accepted', 'evidence_references_rejected', 'more_evidence_required'])
    || !hasString(value, 'reviewerEmployeeId')
    || typeof value.rationale !== 'string'
    || value.rationale.trim().length === 0
    || Array.from(value.rationale).length > 2000
    || !Number.isSafeInteger(value.reviewContractRevision)
    || (value.reviewContractRevision !== 0 && value.reviewContractRevision !== 1)
    || !Array.isArray(value.previewedEvidence)
    || !hasString(value, 'requestId')
    || !hasString(value, 'createdAt')) {
    return false;
  }
  if (value.reviewContractRevision === 0) return value.previewedEvidence.length === 0;
  if (value.previewedEvidence.length === 0 || value.previewedEvidence.length > 1250) return false;
  const seenPaths = new Set<string>();
  return value.previewedEvidence.every(item => {
    if (!isDomainEvidencePreviewAttestation(item)) return false;
    const key = `${item.area}:${item.relativePath}`;
    if (seenPaths.has(key)) return false;
    seenPaths.add(key);
    return true;
  });
}

function isDomainEvidenceAreaAssessment(value: unknown): value is DomainEvidenceAreaAssessmentView {
  return isRecord(value) && isDomainEvidenceArea(value.area)
    && isOneOf(value.outcome, ['accepted', 'rejected', 'insufficient'])
    && typeof value.rationale === 'string' && value.rationale.trim() !== ''
    && Array.from(value.rationale).length <= 1000;
}

function substantiveOverallOutcome(assessments: ReadonlyArray<DomainEvidenceAreaAssessmentView>): DomainEvidenceSubstantiveOutcomeView {
  if (assessments.some(item => item.outcome === 'rejected')) return 'evidence_rejected';
  if (assessments.some(item => item.outcome === 'insufficient')) return 'more_evidence_required';
  return 'evidence_accepted';
}

function isDomainEvidenceSubstantiveAssessmentRecord(value: unknown, companyId: string, recordId: string, evidenceDigest: string, submission: DomainEvidenceSubmissionView, review: DomainEvidenceReviewRecordView | null): value is DomainEvidenceSubstantiveAssessmentRecordView {
  const requiredEvidence = domainEvidenceAreasForProfile(submission.profileId, submission.profileRevision);
  if (!isRecord(value) || requiredEvidence === null || review?.outcome !== 'evidence_references_accepted' || review.reviewContractRevision !== 1
    || value.companyId !== companyId || value.recordId !== recordId || value.evidenceDigest !== evidenceDigest
    || !hasString(value, 'assessmentId') || !hasString(value, 'reviewerEmployeeId') || !hasString(value, 'requestId') || !hasString(value, 'createdAt')
    || !isOneOf(value.outcome, ['evidence_accepted', 'evidence_rejected', 'more_evidence_required'])
    || !Array.isArray(value.areaAssessments) || value.areaAssessments.length !== requiredEvidence.length || !value.areaAssessments.every(isDomainEvidenceAreaAssessment)
    || !Array.isArray(value.previewedEvidence) || value.previewedEvidence.length === 0 || value.previewedEvidence.length > requiredEvidence.length * 250
    || !value.previewedEvidence.every(isDomainEvidencePreviewAttestation)) {
    return false;
  }
  const areaAssessments = value.areaAssessments as ReadonlyArray<DomainEvidenceAreaAssessmentView>;
  if (areaAssessments.some((item, index) => item.area !== requiredEvidence[index])
    || value.outcome !== substantiveOverallOutcome(areaAssessments)
    || submission.evidence.some(item => item.assessedByEmployeeId === value.reviewerEmployeeId)) return false;
  const sourceDigestByArea = new Map<DomainEvidenceAreaView, string>(submission.evidence.map(item => [item.area, item.artifact.sha256]));
  const previewedAreas = new Set<DomainEvidenceAreaView>();
  const previewPaths = new Set<string>();
  for (const item of value.previewedEvidence as ReadonlyArray<DomainEvidencePreviewAttestationView>) {
    if (sourceDigestByArea.get(item.area) !== item.sourceDigest || previewPaths.has(`${item.area}:${item.relativePath}`)) return false;
    previewPaths.add(`${item.area}:${item.relativePath}`);
    previewedAreas.add(item.area);
  }
  return requiredEvidence.every(area => previewedAreas.has(area));
}

function isDomainEvidencePreviewAttestation(value: unknown): value is DomainEvidencePreviewAttestationView {
  return isRecord(value)
    && isDomainEvidenceArea(value.area)
    && typeof value.relativePath === 'string' && isSafeEvidenceRelativePath(value.relativePath)
    && typeof value.sourceDigest === 'string' && /^[0-9a-f]{64}$/.test(value.sourceDigest)
    && typeof value.contentDigest === 'string' && /^[0-9a-f]{64}$/.test(value.contentDigest)
    && isEvidencePreviewContentMediaType(value.mediaType)
    && value.relativePath !== 'pdf/extraction.json'
    && value.relativePath.split('/').at(-1) !== '.polis-git-source.json';
}

function isEvidencePreviewContentMediaType(value: unknown): value is string {
  return isOneOf(value, ['application/json', 'image/png', 'image/jpeg', 'text/plain', 'text/markdown', 'text/csv']);
}

export function validateDomainEvidenceReviewRecord(value: unknown, companyId: string, recordId: string, requestId: string): ValidationResult<DomainEvidenceReviewRecordView> {
  return isDomainEvidenceReviewRecord(value, companyId) && value.recordId === recordId && value.requestId === requestId
    ? {success: true, value}
    : {success: false, issues: [{path: '', message: 'domain evidence review contains an unknown, malformed or cross-scope field'}]};
}

export function validateDomainEvidenceSubstantiveAssessmentRecord(value: unknown, companyId: string, recordId: string, evidenceDigest: string, requestId: string): ValidationResult<DomainEvidenceSubstantiveAssessmentRecordView> {
  if (!isRecord(value) || value.companyId !== companyId || value.recordId !== recordId || value.evidenceDigest !== evidenceDigest
    || !hasString(value, 'assessmentId') || !isOneOf(value.outcome, ['evidence_accepted', 'evidence_rejected', 'more_evidence_required'])
    || !hasString(value, 'reviewerEmployeeId') || !hasString(value, 'requestId') || value.requestId !== requestId || !hasString(value, 'createdAt')
    || !Array.isArray(value.areaAssessments) || value.areaAssessments.length === 0 || !value.areaAssessments.every(isDomainEvidenceAreaAssessment)
    || value.outcome !== substantiveOverallOutcome(value.areaAssessments as ReadonlyArray<DomainEvidenceAreaAssessmentView>)
    || !Array.isArray(value.previewedEvidence) || value.previewedEvidence.length === 0 || !value.previewedEvidence.every(isDomainEvidencePreviewAttestation)) {
    return {success: false, issues: [{path: '', message: 'substantive domain assessment contains an unknown, malformed or cross-scope field'}]};
  }
  return {success: true, value: value as unknown as DomainEvidenceSubstantiveAssessmentRecordView};
}

export function validateDomainEvidenceArtifactPreviewManifest(value: unknown, options: Readonly<{
  companyId: string;
  recordId: string;
  area: DomainEvidenceAreaView;
  inputId: string;
  inputRevision: number;
  sourceDigest: string;
}>): ValidationResult<DomainEvidenceArtifactPreviewManifestView> {
  if (!isRecord(value) || value.companyId !== options.companyId || value.recordId !== options.recordId || value.area !== options.area
    || value.inputId !== options.inputId || value.inputRevision !== options.inputRevision || value.sourceDigest !== options.sourceDigest
    || !Array.isArray(value.entries) || value.entries.length === 0 || value.entries.length > 250) {
    return {success: false, issues: [{path: '', message: 'domain evidence preview manifest does not match the company and evidence reference'}]};
  }
  const seenPaths = new Set<string>();
  for (const item of value.entries) {
    if (!isRecord(item) || typeof item.relativePath !== 'string' || !isSafeEvidenceRelativePath(item.relativePath)
      || typeof item.fileName !== 'string' || !isSafeEvidencePreviewFileName(item.fileName)
      || !hasString(item, 'mediaType') || !Number.isSafeInteger(item.byteSize) || (item.byteSize as number) <= 0 || (item.byteSize as number) > 8 * 1024 * 1024
      || typeof item.contentSHA256 !== 'string' || !/^[0-9a-f]{64}$/.test(item.contentSHA256)
      || typeof item.previewable !== 'boolean' || typeof item.reasonCode !== 'string'
       || (item.previewable && (!isEvidencePreviewContentMediaType(item.mediaType) || item.reasonCode !== ''))
      || (!item.previewable && item.reasonCode !== 'evidence_preview_unsupported')
      || seenPaths.has(item.relativePath)) {
      return {success: false, issues: [{path: '', message: 'domain evidence preview manifest contains an unsafe, duplicated or malformed entry'}]};
    }
    seenPaths.add(item.relativePath);
  }
  return {success: true, value: value as unknown as DomainEvidenceArtifactPreviewManifestView};
}

function isSafeEvidenceRelativePath(value: string): boolean {
  return value.length <= 1024 && !value.startsWith('/') && !value.includes('\\')
    && value.split('/').every(segment => segment !== '' && segment !== '.' && segment !== '..');
}

function isSafeEvidencePreviewFileName(value: string): boolean {
  return value.trim() !== '' && Array.from(value).length <= 255 && !Array.from(value).some(character => {
    const codePoint = character.codePointAt(0) ?? 0;
    return codePoint <= 0x1f || (codePoint >= 0x7f && codePoint <= 0x9f);
  });
}

function isDomainEvidenceRecord(value: unknown, companyId: string): value is DomainEvidenceRecordView {
  if (!isRecord(value)
    || value.companyId !== companyId
    || !hasString(value, 'recordId')
    || !hasString(value, 'profileId')
    || !hasString(value, 'profileRevision')
    || !isOneOf(value.readinessStatus, ['incomplete', 'ready_for_review'])
    || value.qualificationStatus !== 'not_run'
    || value.executionEnabled !== false
    || typeof value.evidenceDigest !== 'string' || !/^[0-9a-f]{64}$/.test(value.evidenceDigest)
    || !isDomainEvidenceSubmission(value.submission)
    || !('review' in value)
    || (value.review !== null && !isDomainEvidenceReviewRecord(value.review, companyId))
    || (value.review !== null && isRecord(value.review) && value.review.recordId !== value.recordId)
    || !('assessment' in value)
    || (value.assessment !== null && !isDomainEvidenceSubstantiveAssessmentRecord(value.assessment, companyId, value.recordId as string, value.evidenceDigest as string, value.submission as DomainEvidenceSubmissionView, value.review as DomainEvidenceReviewRecordView | null))
    || value.profileId !== value.submission.profileId
    || value.profileRevision !== value.submission.profileRevision
    || !Array.isArray(value.reasonCodes)
    || !value.reasonCodes.every(reason => typeof reason === 'string')
    || !hasString(value, 'requestId')
    || !hasString(value, 'createdAt')) {
    return false;
  }
  if (value.review !== null && !domainEvidenceReviewMatchesSubmission(value.review, value.submission)) return false;
  const requiredEvidence = domainEvidenceAreasForProfile(value.profileId, value.profileRevision);
  if (requiredEvidence === null) return false;
  const evidenceAreas = value.submission.evidence.map(item => item.area);
  if (evidenceAreas.some(area => !requiredEvidence.includes(area)) || new Set(evidenceAreas).size !== evidenceAreas.length) return false;
  if (value.readinessStatus === 'ready_for_review') {
    return evidenceAreas.length === requiredEvidence.length
      && requiredEvidence.every(area => evidenceAreas.includes(area))
      && value.reasonCodes.length === 1
      && value.reasonCodes[0] === 'domain_evidence_requires_human_qualification_review';
  }
  return evidenceAreas.length < requiredEvidence.length && value.reasonCodes.includes('domain_evidence_area_missing');
}

function domainEvidenceReviewMatchesSubmission(reviewValue: unknown, submissionValue: unknown): boolean {
  if (!isRecord(reviewValue) || !isRecord(submissionValue) || !Array.isArray(submissionValue.evidence)) return false;
  if (reviewValue.reviewContractRevision === 0) return true;
  const sourceDigestByArea = new Map<string, string>();
  for (const evidenceValue of submissionValue.evidence) {
    if (!isRecord(evidenceValue) || !isDomainEvidenceArea(evidenceValue.area) || !isRecord(evidenceValue.artifact) || typeof evidenceValue.artifact.sha256 !== 'string') return false;
    sourceDigestByArea.set(evidenceValue.area, evidenceValue.artifact.sha256);
  }
  const attestedAreas = new Set<string>();
  for (const previewValue of reviewValue.previewedEvidence as ReadonlyArray<unknown>) {
    if (!isDomainEvidencePreviewAttestation(previewValue)) return false;
    if (sourceDigestByArea.get(previewValue.area) !== previewValue.sourceDigest) return false;
    attestedAreas.add(previewValue.area);
  }
  return Array.from(sourceDigestByArea.keys()).every(area => attestedAreas.has(area));
}

export function validateDomainEvidenceLedger(value: unknown, companyId: string): ValidationResult<DomainEvidenceLedgerView> {
  if (!isRecord(value) || value.companyId !== companyId || !Array.isArray(value.profiles) || !value.profiles.every(isDomainWorkflowProfile)
    || !Array.isArray(value.submissions) || !value.submissions.every(item => isDomainEvidenceRecord(item, companyId))
    || !Array.isArray(value.qualifications) || !value.qualifications.every(item => isDomainProfileQualificationRecord(item, companyId))
    || !Array.isArray(value.researchSimulationRuns) || value.researchSimulationRuns.length > 100 || !value.researchSimulationRuns.every(item => isResearchSimulationRun(item, companyId))
    || !Array.isArray(value.contentSourceEvents) || value.contentSourceEvents.length > 200 || !value.contentSourceEvents.every(item => isDomainContentSourceEvent(item, companyId))
    || !Array.isArray(value.contentDrafts) || value.contentDrafts.length > 100 || !value.contentDrafts.every(item => isDomainContentDraft(item, companyId))
    || !Array.isArray(value.contentReviews) || value.contentReviews.length > 100 || !value.contentReviews.every(item => isDomainContentReview(item, companyId))
    || !Array.isArray(value.contentPublications) || value.contentPublications.length > 100 || !value.contentPublications.every(item => isDomainContentPublication(item, companyId))
    || !Array.isArray(value.contentCorrections) || value.contentCorrections.length > 100 || !value.contentCorrections.every(item => isDomainContentCorrection(item, companyId))
    || !Array.isArray(value.contentFeedback) || value.contentFeedback.length > 100 || !value.contentFeedback.every(item => isDomainContentFeedback(item, companyId))) {
    return {success: false, issues: [{path: '', message: 'domain evidence ledger contains an unknown, malformed or cross-scope field'}]};
  }
  const qualifications = value.qualifications as Array<DomainProfileQualificationRecordView>;
  if (new Set(qualifications.map(item => item.profileId)).size !== qualifications.length) {
    return {success: false, issues: [{path: 'qualifications', message: 'domain profile qualification projection contains duplicates'}]};
  }
  const profileStatusMatches = (value.profiles as Array<DomainWorkflowProfileView>).every(profile => {
    const decision = qualifications.find(item => item.profileId === profile.id);
    return profile.qualificationStatus === (decision?.decision ?? 'not_run');
  });
  if (!profileStatusMatches) {
    return {success: false, issues: [{path: 'profiles', message: 'domain profile status differs from its latest qualification decision'}]};
  }
  return {success: true, value: value as unknown as DomainEvidenceLedgerView};
}

export function validateDomainProfileQualificationRecord(value: unknown, companyId: string, profileId: string, requestId: string): ValidationResult<DomainProfileQualificationRecordView> {
  return isDomainProfileQualificationRecord(value, companyId) && value.profileId === profileId && value.requestId === requestId
    ? {success: true, value}
    : {success: false, issues: [{path: '', message: 'domain profile qualification decision is malformed or out of scope'}]};
}

export function validateResearchSimulationRun(value: unknown, expected: Readonly<{
  companyId: string;
  datasetInputId: string;
  datasetInputRevision: string;
  methodInputId: string;
  methodInputRevision: string;
  seed: string;
  controlDefinition: string;
  riskBudgetUnits: number;
  requestId: string;
}>): ValidationResult<ResearchSimulationRunView> {
  return isResearchSimulationRun(value, expected.companyId)
    && value.datasetInputId === expected.datasetInputId
    && value.datasetInputRevision === expected.datasetInputRevision
    && value.methodInputId === expected.methodInputId
    && value.methodInputRevision === expected.methodInputRevision
    && value.seed === expected.seed
    && value.controlDefinition === expected.controlDefinition
    && value.riskBudgetUnits === String(expected.riskBudgetUnits)
    && value.requestId === expected.requestId
    ? {success: true, value: value as unknown as ResearchSimulationRunView}
    : {success: false, issues: [{path: '', message: 'research simulation receipt contains an unknown, malformed or cross-scope field'}]};
}

export function validateDomainContentSourceEvent(value: unknown, expected: Readonly<{
  companyId: string;
  inputId: string;
  revision: string;
  sha256: string;
  state: 'authorized' | 'revoked';
  requestId: string;
}>): ValidationResult<DomainContentSourceEventView> {
  return isDomainContentSourceEvent(value, expected.companyId)
    && value.inputId === expected.inputId && value.revision === expected.revision && value.sha256 === expected.sha256
    && value.state === expected.state && value.requestId === expected.requestId
    ? {success: true, value: value as DomainContentSourceEventView}
    : {success: false, issues: [{path: '', message: 'content source authorization receipt is malformed or out of scope'}]};
}

export function validateDomainContentDraft(value: unknown, expected: Readonly<{
  companyId: string;
  inputId: string;
  revision: string;
  writerEmployeeId: string;
  requestId: string;
}>): ValidationResult<DomainContentDraftView> {
  return isDomainContentDraft(value, expected.companyId)
    && value.draftInputId === expected.inputId && value.draftRevision === expected.revision
    && value.writerEmployeeId === expected.writerEmployeeId && value.requestId === expected.requestId
    ? {success: true, value: value as DomainContentDraftView}
    : {success: false, issues: [{path: '', message: 'versioned content draft receipt is malformed or out of scope'}]};
}

export function validateDomainContentReview(value: unknown, expected: Readonly<{
  companyId: string;
  draftInputId: string;
  draftRevision: string;
  requestId: string;
}>): ValidationResult<DomainContentReviewView> {
  return isDomainContentReview(value, expected.companyId)
    && value.draftInputId === expected.draftInputId && value.draftRevision === expected.draftRevision && value.requestId === expected.requestId
    ? {success: true, value: value as DomainContentReviewView}
    : {success: false, issues: [{path: '', message: 'content review receipt is malformed or out of scope'}]};
}

export function validateDomainContentPublication(value: unknown, companyId: string, reviewId: string, requestId: string): ValidationResult<DomainContentPublicationView> {
  return isDomainContentPublication(value, companyId) && value.reviewId === reviewId && value.requestId === requestId
    ? {success: true, value}
    : {success: false, issues: [{path: '', message: 'simulated content publication receipt is malformed or out of scope'}]};
}

export function validateDomainContentCorrection(value: unknown, expected: Readonly<{
  companyId: string;
  publicationId: string;
  draftInputId: string;
  draftRevision: string;
  requestId: string;
}>): ValidationResult<DomainContentCorrectionView> {
  return isDomainContentCorrection(value, expected.companyId) && value.publicationId === expected.publicationId
    && value.correctionDraftInputId === expected.draftInputId && value.correctionDraftRevision === expected.draftRevision && value.requestId === expected.requestId
    ? {success: true, value}
    : {success: false, issues: [{path: '', message: 'content correction receipt is malformed or out of scope'}]};
}

export function validateDomainContentFeedback(value: unknown, expected: Readonly<{
  companyId: string;
  publicationId: string;
  category: DomainContentFeedbackView['category'];
  requestId: string;
}>): ValidationResult<DomainContentFeedbackView> {
  return isDomainContentFeedback(value, expected.companyId) && value.publicationId === expected.publicationId
    && value.category === expected.category && value.requestId === expected.requestId
    ? {success: true, value}
    : {success: false, issues: [{path: '', message: 'content feedback receipt is malformed or out of scope'}]};
}

export function validateDomainEvidenceRecord(value: unknown, companyId: string, requestId: string): ValidationResult<DomainEvidenceRecordView> {
  return isDomainEvidenceRecord(value, companyId) && value.requestId === requestId
    ? {success: true, value}
    : {success: false, issues: [{path: '', message: 'domain evidence record contains an unknown, malformed or cross-scope field'}]};
}

export function validateActivityView(value: unknown, companyId: string): ValidationResult<ActivityView> {
  if (!isRecord(value)) {
    return {success: false, issues: [{path: '', message: 'activity response is not an object'}]};
  }
  const issues = validateMeta(value.meta, companyId);
  if (!Array.isArray(value.items)) {
    issues.push({path: 'items', message: 'activity items must be an array'});
  } else {
    value.items.forEach((item, index) => {
      if (!isActivityEvent(item)) {
        issues.push({path: `items.${index}`, message: 'activity item is not a supported ActivityEvent'});
      }
    });
  }
  if (value.nextCursor !== null && (typeof value.nextCursor !== 'string' || value.nextCursor.trim() === '')) {
    issues.push({path: 'nextCursor', message: 'nextCursor must be a string or null'});
  }
  if (issues.length > 0) {
    return {success: false, issues};
  }
  return {success: true, value: value as ActivityView};
}

export function validateCompanyOverview(value: unknown, companyId: string): ValidationResult<CompanyOverviewView> {
  if (!isRecord(value)) {
    return {success: false, issues: [{path: '', message: 'company overview response is not an object'}]};
  }
  const issues = validateMeta(value.meta, companyId);
  if (!isRecord(value.company) || value.company.companyId !== companyId || !hasString(value.company, 'name') || !hasString(value.company, 'description')) {
    issues.push({path: 'company', message: 'company identity is missing or outside the requested scope'});
  }
  if (!isMissionSummary(value.mission)) {
    issues.push({path: 'mission', message: 'mission summary is missing or malformed'});
  }
  if (!isTeamSummary(value.team)) {
    issues.push({path: 'team', message: 'team summary is missing or malformed'});
  }
  if (!Array.isArray(value.employees)) {
    issues.push({path: 'employees', message: 'employees must be an array'});
  } else {
    value.employees.forEach((employee, index) => {
      if (!isEmployeeSummary(employee)) {
        issues.push({path: `employees.${index}`, message: 'employee view contains an unknown or malformed field'});
      }
    });
  }
  if (!Array.isArray(value.tasks)) {
    issues.push({path: 'tasks', message: 'tasks must be an array'});
  } else {
    value.tasks.forEach((task, index) => {
      if (!isTaskSummary(task)) {
        issues.push({path: `tasks.${index}`, message: 'task view contains an unknown or malformed field'});
      }
    });
  }
  if (!Array.isArray(value.obligations)) {
    issues.push({path: 'obligations', message: 'obligations must be an array'});
  } else {
    value.obligations.forEach((obligation, index) => {
      if (!isObligationSummary(obligation)) {
        issues.push({path: `obligations.${index}`, message: 'obligation view contains an unknown or malformed field'});
      }
    });
  }
  if (!Array.isArray(value.artifacts)) {
    issues.push({path: 'artifacts', message: 'artifacts must be an array'});
  } else {
    value.artifacts.forEach((artifact, index) => {
      if (!isArtifactSummary(artifact)) {
        issues.push({path: `artifacts.${index}`, message: 'artifact view contains an unknown or malformed field'});
      }
    });
  }
  if (!Array.isArray(value.checkpoints)) {
    issues.push({path: 'checkpoints', message: 'checkpoints must be an array'});
  } else {
    value.checkpoints.forEach((checkpoint, index) => {
      if (!isCheckpointSummary(checkpoint)) {
        issues.push({path: `checkpoints.${index}`, message: 'checkpoint view contains an unknown or malformed field'});
      }
    });
  }
  if (!isResourceSummary(value.resources)) {
    issues.push({path: 'resources', message: 'resource summary is missing or malformed'});
  }
  if (!Array.isArray(value.attention)) {
    issues.push({path: 'attention', message: 'attention items must be an array'});
  } else {
    value.attention.forEach((item, index) => {
      if (!isAttentionItem(item)) {
        issues.push({path: `attention.${index}`, message: 'attention item contains an unknown or malformed field'});
      }
    });
  }
  if (!Array.isArray(value.recentActivity)) {
    issues.push({path: 'recentActivity', message: 'recentActivity must be an array'});
  } else {
    value.recentActivity.forEach((event, index) => {
      if (!isActivityEvent(event)) {
        issues.push({path: `recentActivity.${index}`, message: 'recent activity item is not supported'});
      }
    });
  }
  if (issues.length > 0) {
    return {success: false, issues};
  }
  return {success: true, value: value as CompanyOverviewView};
}

export function validationMessage(issues: ReadonlyArray<ValidationIssue>): string {
  return issues.map(issue => `${issue.path || 'response'}: ${issue.message}`).join('; ');
}

const ENVIRONMENT_PREPARATION_STATES: ReadonlyArray<ProjectEnvironmentRevisionView['preparationState']> = ['unprepared', 'blocked_policy', 'blocked_source_unverified', 'blocked_unqualified', 'accepted', 'starting', 'running', 'ready', 'failed', 'cancelled', 'outcome_unknown'];
const ENVIRONMENT_POLICY_DECISIONS: ReadonlyArray<ProjectEnvironmentRevisionView['policyDecision']> = ['unverified', 'source_unverified', 'not_approved', 'approved', 'revoked', 'revocation_required'];
const ENVIRONMENT_EXECUTOR_QUALIFICATIONS: ReadonlyArray<ProjectEnvironmentRevisionView['executorQualification']> = ['unqualified', 'qualified', 'expired', 'revoked', 'identity_stale'];
const JOB_KINDS: ReadonlyArray<JobRunView['kind']> = ['batch', 'service', 'controlled_input'];
const JOB_STATES: ReadonlyArray<JobRunView['state']> = ['accepted', 'starting', 'running', 'exited', 'failed', 'cancelled', 'outcome_unknown'];
const JOB_READINESS: ReadonlyArray<JobRunView['readiness']> = ['not_applicable', 'not_ready', 'ready', 'unhealthy'];
const NODE_ENVIRONMENT_PROFILES: ReadonlyArray<CrossBackendHandoverView['sourceProfileId']> = ['windows-node-npm@1', 'linux-node-npm@1'];
const SERVICE_READINESS: ReadonlyArray<ServiceEndpointView['readiness']> = ['not_ready', 'ready', 'unhealthy', 'revoked'];
const SHA256_PATTERN = /^[a-f0-9]{64}$/;
const LIFECYCLE_REASON_PATTERN = /^[a-z0-9_-]{1,96}$/;
const REGISTRY_HOST_PATTERN = /^(?=.{1,253}$)(?:[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?)(?:\.(?:[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?))*$/;

function isTimestamp(value: unknown): value is string {
  return typeof value === 'string' && value.trim() !== '';
}

function isDecimalCounter(value: unknown): value is string {
  return typeof value === 'string' && /^\d{1,18}$/.test(value);
}

function isProjectEnvironmentRevisionView(value: unknown, companyId: string): value is ProjectEnvironmentRevisionView {
  if (!isRecord(value)) return false;
  const hasCurrentExecutorIdentity = value.executorFingerprintSha256 !== null && value.hostFingerprintSha256 !== null && value.isolationPolicySha256 !== null;
  const hasExecutorEvidence = value.executorEvidenceSha256 !== null && value.executorEvidenceInputId !== null && value.executorEvidenceInputRevision !== null;
  return isRecord(value)
    && value.companyId === companyId
    && hasString(value, 'revisionId')
    && hasString(value, 'missionId')
    && hasString(value, 'sourceInputId')
    && isDecimalCounter(value.sourceInputRevision)
    && hasString(value, 'projectRootRelative')
    && isOneOf(value.sourceBindingStatus, ['bound', 'unverified'])
    && isOneOf(value.profileId, ['windows-node-npm@1', 'linux-node-npm@1'])
    && typeof value.sourceRevisionSha256 === 'string' && SHA256_PATTERN.test(value.sourceRevisionSha256)
    && typeof value.packageJsonSha256 === 'string' && SHA256_PATTERN.test(value.packageJsonSha256)
    && typeof value.lockfileSha256 === 'string' && SHA256_PATTERN.test(value.lockfileSha256)
    && typeof value.policySha256 === 'string' && SHA256_PATTERN.test(value.policySha256)
    && typeof value.toolchainSha256 === 'string' && SHA256_PATTERN.test(value.toolchainSha256)
    && isNullableString(value.executorFingerprintSha256) && (value.executorFingerprintSha256 === null || SHA256_PATTERN.test(value.executorFingerprintSha256))
    && isNullableString(value.hostFingerprintSha256) && (value.hostFingerprintSha256 === null || SHA256_PATTERN.test(value.hostFingerprintSha256))
    && isNullableString(value.isolationPolicySha256) && (value.isolationPolicySha256 === null || SHA256_PATTERN.test(value.isolationPolicySha256))
    && isNullableString(value.executorEvidenceSha256) && (value.executorEvidenceSha256 === null || SHA256_PATTERN.test(value.executorEvidenceSha256))
    && isNullableString(value.executorEvidenceInputId)
    && isNullableString(value.executorEvidenceInputRevision)
    && ((value.executorEvidenceSha256 === null && value.executorEvidenceInputId === null && value.executorEvidenceInputRevision === null)
      || (value.executorEvidenceSha256 !== null && typeof value.executorEvidenceInputId === 'string' && value.executorEvidenceInputId !== '' && isDecimalCounter(value.executorEvidenceInputRevision)))
    && (value.executorQualification !== 'qualified' || hasCurrentExecutorIdentity && hasExecutorEvidence)
    && isOneOf(value.policyDecision, ENVIRONMENT_POLICY_DECISIONS)
    && (value.policyManifest === null || isProjectEnvironmentPolicyManifestView(value.policyManifest, value.profileId))
    && isOneOf(value.executorQualification, ENVIRONMENT_EXECUTOR_QUALIFICATIONS)
    && isOneOf(value.preparationState, ENVIRONMENT_PREPARATION_STATES)
    && typeof value.preparationReason === 'string' && LIFECYCLE_REASON_PATTERN.test(value.preparationReason)
    && isNullableString(value.preparationRunId)
    && (value.preparationRunId === null || value.preparationRunId !== '')
    && isTimestamp(value.createdAt);
}

function isProjectEnvironmentPolicyManifestView(value: unknown, expectedProfileId: unknown): value is ProjectEnvironmentPolicyManifestView {
  if (!isRecord(value)) return false;
  const timeoutMs = value.timeoutMs;
  const outputLimitBytes = value.outputLimitBytes;
  if (value.schemaVersion !== 'project-environment-policy@1' || value.profileId !== expectedProfileId
    || value.lifecycleScriptsPolicy !== 'ignore'
    || typeof timeoutMs !== 'number' || !Number.isInteger(timeoutMs) || timeoutMs < 1000 || timeoutMs > 600000
    || typeof outputLimitBytes !== 'number' || !Number.isInteger(outputLimitBytes) || outputLimitBytes < 4096 || outputLimitBytes > 1048576
    || !Array.isArray(value.registryHosts)) return false;
  const registryHosts: ReadonlyArray<unknown> = value.registryHosts;
  const hasValidRegistryHosts = registryHosts.length >= 1 && registryHosts.length <= 8
    && registryHosts.every(host => typeof host === 'string' && host === host.toLowerCase() && REGISTRY_HOST_PATTERN.test(host))
    && new Set(registryHosts).size === registryHosts.length;
  const hasValidServices = isProjectServiceDefinitionsView(value.services);
  if (!hasValidServices) return false;
  if (value.profileId === 'linux-node-npm@1') {
    return value.installPolicy === 'npm ci --ignore-scripts --no-audit --no-fund --offline'
      && value.networkPolicy === 'deny_all'
      && hasValidRegistryHosts;
  }
  if (value.profileId !== 'windows-node-npm@1'
    || value.installPolicy !== 'npm ci --ignore-scripts --no-audit --no-fund'
    || value.networkPolicy !== 'registry_allowlist') return false;
  return hasValidRegistryHosts && hasValidServices;
}

function isProjectServiceDefinitionsView(value: unknown): boolean {
  if (value === undefined) return true;
  if (!Array.isArray(value) || value.length > 8) return false;
  const ids = new Set<string>();
  const endpoints = new Set<string>();
  let previousId = '';
  for (const service of value) {
    if (!isRecord(service) || typeof service.id !== 'string' || !/^[a-z][a-z0-9-]{0,63}$/.test(service.id)
      || ids.has(service.id) || service.id <= previousId || typeof service.scriptPath !== 'string' || !isSafeNodeServiceScriptPath(service.scriptPath)
      || !isServiceProbeSpecView(service.probe)) return false;
    const endpointKey = `${service.probe.bindAddress}:${service.probe.port}`;
    if (endpoints.has(endpointKey)) return false;
    ids.add(service.id);
    endpoints.add(endpointKey);
    previousId = service.id;
  }
  return true;
}

function isSafeNodeServiceScriptPath(value: string): boolean {
	if (value.length < 1 || new TextEncoder().encode(value).length > 260 || value.startsWith('/') || value.includes('\\') || value.includes(':') || hasControlCharacter(value)) return false;
  const parts = value.split('/');
  if (parts.some(part => part === '' || part === '.' || part === '..')) return false;
  const extension = parts.at(-1)?.toLowerCase().split('.').at(-1);
  return extension === 'js' || extension === 'cjs' || extension === 'mjs';
}

function isServiceProbeSpecView(value: unknown): value is ProjectServiceDefinitionView['probe'] {
  if (!isRecord(value) || !isOneOf(value.bindAddress, ['127.0.0.1', '::1'])) return false;
  const {port, path, expectedStatusCode, expectedBodySha256, timeoutMs, leaseDurationMs} = value;
  return typeof port === 'number' && Number.isInteger(port) && port >= 1 && port <= 65535
    && typeof path === 'string' && path.length >= 1 && new TextEncoder().encode(path).length <= 256 && path.startsWith('/') && !path.startsWith('//')
    && !path.includes('\\') && !/[?#%]/.test(path) && !hasControlCharacter(path) && !path.split('/').some(part => part === '.' || part === '..')
    && path.replace(/\/{2,}/g, '/') === path && (path === '/' || !path.endsWith('/'))
    && typeof expectedStatusCode === 'number' && Number.isInteger(expectedStatusCode) && expectedStatusCode >= 200 && expectedStatusCode < 300
    && typeof expectedBodySha256 === 'string' && SHA256_PATTERN.test(expectedBodySha256)
    && typeof timeoutMs === 'number' && Number.isInteger(timeoutMs) && timeoutMs >= 100 && timeoutMs <= 5000
    && typeof leaseDurationMs === 'number' && Number.isInteger(leaseDurationMs) && leaseDurationMs >= 1000 && leaseDurationMs >= timeoutMs && leaseDurationMs <= 600000;
}

function hasControlCharacter(value: string): boolean {
  for (const character of value) {
    const code = character.charCodeAt(0);
    if (code < 32 || (code >= 127 && code <= 159)) return true;
  }
  return false;
}

function isServiceEndpointView(value: unknown): value is ServiceEndpointView {
  if (!isRecord(value)) return false;
  const port = typeof value.port === 'string' ? Number(value.port) : Number.NaN;
  const readiness = value.readiness;
  const leaseExpiresAt = value.leaseExpiresAt;
  return isDecimalCounter(value.generation) && value.generation !== '0'
    && isOneOf(value.bindAddress, ['127.0.0.1', '::1', 'localhost'])
    && Number.isInteger(port) && port >= 1 && port <= 65535
    && isOneOf(readiness, SERVICE_READINESS)
    && typeof value.sourceRevisionSha256 === 'string' && SHA256_PATTERN.test(value.sourceRevisionSha256)
    && typeof value.healthcheckSha256 === 'string' && SHA256_PATTERN.test(value.healthcheckSha256)
    && (readiness === 'revoked' ? leaseExpiresAt === null : isTimestamp(leaseExpiresAt));
}

function isJobRunServiceEndpointProjection(kind: unknown, state: unknown, readiness: unknown, endpoint: unknown): boolean {
  if (kind !== 'service') return endpoint === null && readiness === 'not_applicable';
  if (endpoint === null) return readiness === 'not_ready';
  if (!isServiceEndpointView(endpoint)) return false;
  const terminal = isOneOf(state, ['exited', 'failed', 'cancelled', 'outcome_unknown']);
  if (endpoint.readiness === 'ready' && state !== 'running') return false;
  if (terminal && endpoint.readiness !== 'revoked' && endpoint.readiness !== 'unhealthy') return false;
  const projectedReadiness = endpoint.readiness === 'revoked' ? 'unhealthy' : endpoint.readiness;
  return readiness === projectedReadiness;
}

function isJobRunView(value: unknown, companyId: string, taskId: string): value is JobRunView {
  if (!isRecord(value)) return false;
  const exitCodeValid = value.exitCode === null || (typeof value.exitCode === 'number' && Number.isInteger(value.exitCode));
  return value.companyId === companyId
    && hasString(value, 'jobId')
    && value.taskId === taskId
    && hasString(value, 'sessionId')
    && hasString(value, 'environmentRevisionId')
    && typeof value.handoverId === 'string'
    && isOneOf(value.kind, JOB_KINDS)
    && typeof value.serviceId === 'string'
    && isJobRunServiceIdentityProjection(value.kind, value.serviceId)
    && isOneOf(value.state, JOB_STATES)
    && isOneOf(value.readiness, JOB_READINESS)
    && exitCodeValid
    && typeof value.reasonCode === 'string' && LIFECYCLE_REASON_PATTERN.test(value.reasonCode)
    && isDecimalCounter(value.stdoutOffset)
    && isDecimalCounter(value.stderrOffset)
    && isDecimalCounter(value.stdoutBytes)
    && isDecimalCounter(value.stderrBytes)
    && typeof value.logsTruncated === 'boolean'
    && typeof value.logGap === 'boolean'
    && isNullableString(value.logManifestSha256)
    && (value.logManifestSha256 === null || SHA256_PATTERN.test(value.logManifestSha256))
    && isJobRunServiceEndpointProjection(value.kind, value.state, value.readiness, value.serviceEndpoint)
    && isTimestamp(value.createdAt)
    && isTimestamp(value.updatedAt);
}

function isJobRunServiceIdentityProjection(kind: JobRunView['kind'], serviceId: string): boolean {
	return kind === 'service' ? serviceId === 'legacy:unattributed' || /^[a-z][a-z0-9-]{0,63}$/.test(serviceId) : serviceId === '';
}

export function validateProjectEnvironmentRevisions(value: unknown, companyId: string): ValidationResult<ReadonlyArray<ProjectEnvironmentRevisionView>> {
  if (!Array.isArray(value) || value.length > 100 || !value.every(item => isProjectEnvironmentRevisionView(item, companyId))) {
    return {success: false, issues: [{path: '', message: 'project environment response contains an unknown, malformed, or cross-scope revision'}]};
  }
  const ids = value.map(item => item.revisionId);
  if (new Set(ids).size !== ids.length) {
    return {success: false, issues: [{path: '', message: 'project environment response contains duplicate revisions'}]};
  }
  return {success: true, value};
}

export function validateEnvironmentPolicyDecisionReceipt(value: unknown): ValidationResult<EnvironmentPolicyDecisionReceipt> {
  return isRecord(value) && hasString(value, 'id') && isOneOf(value.status, ['approved', 'revoked'])
    ? {success: true, value: value as EnvironmentPolicyDecisionReceipt}
    : {success: false, issues: [{path: '', message: 'environment policy receipt is malformed'}]};
}

export function validateEnvironmentExecutorQualificationReceipt(value: unknown): ValidationResult<EnvironmentExecutorQualificationReceipt> {
  return isRecord(value) && hasString(value, 'id') && isOneOf(value.status, ['qualified', 'revoked'])
    ? {success: true, value: value as EnvironmentExecutorQualificationReceipt}
    : {success: false, issues: [{path: '', message: 'environment executor qualification receipt is malformed'}]};
}

export function validateEnvironmentPreparationRun(value: unknown, companyId: string, revisionId: string): ValidationResult<EnvironmentPreparationRunView> {
  if (!isRecord(value) || value.companyId !== companyId || value.revisionId !== revisionId || !hasString(value, 'runId') || !hasString(value, 'requestId') || typeof value.reasonCode !== 'string' || !LIFECYCLE_REASON_PATTERN.test(value.reasonCode) || !isTimestamp(value.createdAt) || !isOneOf(value.state, ENVIRONMENT_PREPARATION_STATES.slice(1))) {
    return {success: false, issues: [{path: '', message: 'environment preparation response is malformed or outside the requested scope'}]};
  }
  return {success: true, value: value as EnvironmentPreparationRunView};
}

export function validateTaskJobRuns(value: unknown, companyId: string, taskId: string): ValidationResult<ReadonlyArray<JobRunView>> {
  if (!Array.isArray(value) || value.length > 300 || !value.every(item => isJobRunView(item, companyId, taskId))) {
    return {success: false, issues: [{path: '', message: 'JobRun response contains an unknown, malformed, or cross-scope record'}]};
  }
  const ids = value.map(item => item.jobId);
  if (new Set(ids).size !== ids.length) {
    return {success: false, issues: [{path: '', message: 'JobRun response contains duplicate job ids'}]};
  }
  return {success: true, value};
}

function isCrossBackendHandoverView(value: unknown, companyId: string, taskId: string): value is CrossBackendHandoverView {
  if (!isRecord(value)) return false;
  const allowedPair = value.sourceProfileId === 'windows-node-npm@1' && value.targetProfileId === 'linux-node-npm@1'
    || value.sourceProfileId === 'linux-node-npm@1' && value.targetProfileId === 'windows-node-npm@1';
  return value.companyId === companyId
    && value.taskId === taskId
    && hasString(value, 'handoverId')
    && hasString(value, 'missionId')
    && hasString(value, 'sourceJobId')
    && hasString(value, 'sourceSessionId')
    && hasString(value, 'sourceRuntimeIncarnation')
    && hasString(value, 'sourceEnvironmentRevisionId')
    && isOneOf(value.sourceProfileId, NODE_ENVIRONMENT_PROFILES)
    && hasString(value, 'targetEnvironmentRevisionId')
    && isOneOf(value.targetProfileId, NODE_ENVIRONMENT_PROFILES)
    && allowedPair
    && SHA256_PATTERN.test(String(value.projectSourceSha256))
    && SHA256_PATTERN.test(String(value.packageJsonSha256))
    && SHA256_PATTERN.test(String(value.lockfileSha256))
    && SHA256_PATTERN.test(String(value.workspaceDigest))
    && typeof value.workspaceRevision === 'number' && Number.isSafeInteger(value.workspaceRevision) && value.workspaceRevision > 0
    && SHA256_PATTERN.test(String(value.taskInputManifestSha256))
    && SHA256_PATTERN.test(String(value.targetPolicySha256))
    && SHA256_PATTERN.test(String(value.targetToolchainSha256))
    && hasString(value, 'requestId')
    && SHA256_PATTERN.test(String(value.recordSha256))
    && isTimestamp(value.createdAt);
}

export function validateTaskCrossBackendHandovers(value: unknown, companyId: string, taskId: string): ValidationResult<ReadonlyArray<CrossBackendHandoverView>> {
  if (!Array.isArray(value) || value.length > 100 || !value.every(item => isCrossBackendHandoverView(item, companyId, taskId))) {
    return {success: false, issues: [{path: '', message: 'cross-backend handover response contains an unknown, malformed, or cross-scope record'}]};
  }
  const ids = value.map(item => item.handoverId);
  if (new Set(ids).size !== ids.length) {
    return {success: false, issues: [{path: '', message: 'cross-backend handover response contains duplicate handover ids'}]};
  }
  return {success: true, value};
}

export function validateJobRunCommandReceipt(value: unknown, companyId: string, taskId?: string): ValidationResult<JobRunCommandReceipt> {
  if (!isRecord(value) || value.companyId !== companyId || (taskId !== undefined && value.taskId !== taskId) || !hasString(value, 'taskId') || !hasString(value, 'jobId')
    || !isOneOf(value.kind, JOB_KINDS) || typeof value.serviceId !== 'string' || !isJobRunServiceIdentityProjection(value.kind, value.serviceId)
    || !isOneOf(value.state, JOB_STATES) || !isOneOf(value.readiness, JOB_READINESS)
    || typeof value.reasonCode !== 'string' || !LIFECYCLE_REASON_PATTERN.test(value.reasonCode)) {
    return {success: false, issues: [{path: '', message: 'JobRun command receipt is malformed or outside the requested scope'}]};
  }
  return {success: true, value: value as JobRunCommandReceipt};
}

export function validateJobRunLogArtifact(value: unknown, companyId: string, jobId: string): ValidationResult<JobRunLogArtifactView> {
  if (!isRecord(value) || value.companyId !== companyId || value.jobId !== jobId
    || typeof value.manifestSha256 !== 'string' || (value.manifestSha256 !== '' && !SHA256_PATTERN.test(value.manifestSha256))
    || typeof value.content !== 'string' || !/^(?:[A-Za-z0-9+/]{4})*(?:[A-Za-z0-9+/]{2}==|[A-Za-z0-9+/]{3}=)?$/.test(value.content)) {
    return {success: false, issues: [{path: '', message: 'JobRun log artifact is malformed or outside the requested scope'}]};
  }
  return {success: true, value: value as JobRunLogArtifactView};
}

export function validateServiceBrowserSession(value: unknown, now = Date.now()): ValidationResult<ServiceBrowserSessionView> {
  if (!isRecord(value) || typeof value.url !== 'string' || typeof value.expiresAt !== 'string') {
    return {success: false, issues: [{path: '', message: 'service browser session is malformed'}]};
  }
  const {url: urlValue, expiresAt: expiresAtValue} = value;
  let url: URL;
  try {
    url = new URL(urlValue);
  } catch {
    return {success: false, issues: [{path: 'url', message: 'service browser session URL is invalid'}]};
  }
  const expiresAt = Date.parse(expiresAtValue);
  const remaining = expiresAt - now;
  if (url.protocol !== 'http:' || url.hostname !== '127.0.0.1' || url.port === '' || url.username !== '' || url.password !== ''
    || url.search !== '' || url.hash !== '' || !/^\/_polis\/open\/[0-9a-f]{64}$/.test(url.pathname)
    || !Number.isInteger(Number(url.port)) || Number(url.port) < 1 || Number(url.port) > 65535
    || !Number.isFinite(expiresAt) || remaining <= 0 || remaining > 6 * 60_000) {
    return {success: false, issues: [{path: '', message: 'service browser session is outside its loopback or expiry boundary'}]};
  }
  return {success: true, value: value as ServiceBrowserSessionView};
}

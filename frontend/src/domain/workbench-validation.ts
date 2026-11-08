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
  CodexModelCatalogView,
  CodexModelOptionView,
  RuntimeSettingsView,
  OperatorInstructionReceipt,
  OperatorInstructionView,
  CollaborationItem,
  ArtifactDetailView,
  WorkspaceView,
  OperationsView,
  NotificationsView,
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
  SkillReferenceRead,
  SkillSummaryPage,
  ToolDescriptorView,
  VisibleResourcePage,
  ToolSummaryPage,
  MCPToolCallQualificationView,
  MCPToolCallReceiptView,
} from './workbench';

export type ValidationIssue = Readonly<{
  path: string;
  message: string;
}>;

export type ValidationResult<T> =
  | Readonly<{success: true; value: T}>
  | Readonly<{success: false; issues: ReadonlyArray<ValidationIssue>}>;

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
const MISSION_COMMAND_TYPES: ReadonlyArray<MissionCommandType> = ['mission.create', 'mission.start', 'mission.cancel'];
const MISSION_COMMAND_STATES: ReadonlyArray<MissionCommandResultState> = ['draft', 'active', 'cancelled'];

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

function isEmployeeDraft(value: unknown): boolean {
  return isRecord(value)
    && hasString(value, 'id')
    && hasString(value, 'displayName')
    && hasString(value, 'role')
    && hasString(value, 'modelProfile');
}

function isCompanySummary(value: unknown): value is CompanySummaryView {
  return isRecord(value)
    && hasString(value, 'id')
    && hasString(value, 'name')
    && hasString(value, 'workspaceRoot')
    && isOneOf(value.state, ['active', 'archived'])
    && Array.isArray(value.roster)
    && value.roster.every(employee => isEmployeeDraft(employee));
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

function isOperatorInstruction(value: unknown): value is OperatorInstructionView {
  return isRecord(value)
    && hasString(value, 'instructionId')
    && hasString(value, 'missionId')
    && isNullableString(value.taskId)
    && isNullableString(value.employeeId)
    && hasString(value, 'content')
    && isOneOf(value.state, ['pending', 'applied', 'rejected'])
    && hasString(value, 'createdAt');
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
    && value.routes.every(route => isRecord(route) && hasString(route, 'routeId') && isOneOf(route.adapter, ['local', 'webhook']) && typeof route.enabled === 'boolean' && typeof route.destination === 'string' && hasString(route, 'status'))
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
    && isNullableString(value.roleRevision)
    && hasString(value, 'epoch')
    && isNullableString(value.sessionId)
    && isNullableString(value.sessionState)
    && isNullableString(value.profile)
    && (value.currentTask === null || isEntityRef(value.currentTask))
    && isEmployeeStatus(value.status)
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
    && isOneOf(value.state, ['draft', 'active', 'paused', 'succeeded', 'cancelled'])
    && hasString(value, 'contract')
    && (value.acceptanceContract === null || isAcceptanceContract(value.acceptanceContract))
    && (value.currentContractRevision === null || isContractRevision(value.currentContractRevision))
    && hasString(value, 'nextMilestone')
    && hasString(value, 'verifiedMilestones')
    && hasString(value, 'milestoneTotal')
    && Array.isArray(value.milestones)
    && value.milestones.every(milestone => isRecord(milestone)
      && hasString(milestone, 'id')
      && hasString(milestone, 'label')
      && isOneOf(milestone.state, ['verified', 'current', 'upcoming', 'blocked']));
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
    && value.evidenceRefs.every(item => typeof item === 'string');
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

export function validateOperations(value: unknown): ValidationResult<OperationsView> {
  return isOperations(value) ? {success: true, value} : {success: false, issues: [{path: '', message: 'operations response contains an unknown or malformed field'}]};
}

export function validateNotifications(value: unknown): ValidationResult<NotificationsView> {
  return isNotifications(value) ? {success: true, value} : {success: false, issues: [{path: '', message: 'notifications response contains an unknown or malformed field'}]};
}

function isSkillRevision(value: unknown): boolean {
  return isRecord(value)
    && hasString(value, 'companyId')
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

function isMCPServerDefinition(value: unknown): boolean {
  return isRecord(value)
    && hasString(value, 'companyId')
    && hasString(value, 'id')
    && hasString(value, 'name')
    && isOneOf(value.transport, ['stdio', 'streamable_http'])
    && isNullableString(value.endpoint)
    && isNullableString(value.command)
    && Array.isArray(value.args)
    && value.args.every(item => typeof item === 'string')
    && hasString(value, 'descriptorDigest')
    && isOneOf(value.status, ['unverified', 'candidate', 'approved', 'revoked'])
    && isRecord(value.qualification)
    && hasString(value, 'createdAt');
}

function isCapabilityBinding(value: unknown): boolean {
  return isRecord(value)
    && hasString(value, 'companyId')
    && hasString(value, 'id')
    && hasString(value, 'employeeId')
    && isOneOf(value.status, ['draft', 'approved', 'disabled', 'revoked'])
    && Array.isArray(value.skillRevisionIds)
    && value.skillRevisionIds.every(item => typeof item === 'string')
    && Array.isArray(value.mcpDefinitionIds)
    && value.mcpDefinitionIds.every(item => typeof item === 'string')
    && hasString(value, 'createdAt');
}

function isSkillLoad(value: unknown): boolean {
  return isRecord(value)
    && hasString(value, 'companyId')
    && hasString(value, 'id')
    && hasString(value, 'taskId')
    && hasString(value, 'employeeId')
    && hasString(value, 'skillRevisionId')
    && hasString(value, 'contentDigest')
    && hasString(value, 'body')
    && Array.isArray(value.references)
    && value.references.every(item => typeof item === 'string')
    && value.scriptsExecuted === false
    && typeof value.repeated === 'boolean'
    && hasString(value, 'createdAt');
}

function isSkillSummary(value: unknown): boolean {
  return isRecord(value)
    && !('body' in value)
    && !('referenceSnapshots' in value)
    && hasString(value, 'id')
    && hasString(value, 'packageId')
    && hasString(value, 'revision')
    && hasString(value, 'displayName')
    && hasString(value, 'purpose')
    && hasString(value, 'sourceRef')
    && hasString(value, 'contentDigest')
    && value.executionStatus === 'unverified'
    && value.scriptsExecuted === false;
}

function isToolSummary(value: unknown): boolean {
  return isRecord(value)
    && !('command' in value)
    && hasString(value, 'serverId')
    && hasString(value, 'serverName')
    && hasString(value, 'toolName')
    && hasString(value, 'descriptorDigest')
    && hasString(value, 'handle')
    && value.toolsCalled === 0
    && value.callable === false;
}

function isToolDescriptor(value: unknown): boolean {
  return isRecord(value)
    && !('command' in value)
    && hasString(value, 'serverId')
    && hasString(value, 'serverName')
    && hasString(value, 'toolName')
    && hasString(value, 'description')
    && isRecord(value.inputSchema)
    && (value.outputSchema === null || isRecord(value.outputSchema))
    && hasString(value, 'descriptorDigest')
    && hasString(value, 'handle')
    && value.toolsCalled === 0
    && value.callable === false;
}

export function validateToolDescriptor(value: unknown): ValidationResult<ToolDescriptorView> {
  if (!isToolDescriptor(value)) {
    return {success: false, issues: [{path: '', message: 'tool descriptor contains an unknown or malformed field'}]};
  }
  return {success: true, value: value as ToolDescriptorView};
}

function isVisibleResource(value: unknown): boolean {
  return isRecord(value)
    && !('command' in value)
    && hasString(value, 'serverId')
    && hasString(value, 'serverName')
    && hasString(value, 'uri')
    && hasString(value, 'name')
    && value.resourcesRead === 0
    && value.readable === false;
}

export function validateVisibleResourcePage(value: unknown): ValidationResult<VisibleResourcePage> {
  if (!isRecord(value) || !Array.isArray(value.items) || !value.items.every(isVisibleResource) || !(value.nextCursor === null || typeof value.nextCursor === 'string')) {
    return {success: false, issues: [{path: '', message: 'resource summary page contains an unknown or malformed field'}]};
  }
  return {success: true, value: value as VisibleResourcePage};
}

export function validateToolSummaryPage(value: unknown): ValidationResult<ToolSummaryPage> {
  if (!isRecord(value) || !Array.isArray(value.items) || !value.items.every(isToolSummary) || !(value.nextCursor === null || typeof value.nextCursor === 'string')) {
    return {success: false, issues: [{path: '', message: 'tool summary page contains an unknown or malformed field'}]};
  }
  return {success: true, value: value as ToolSummaryPage};
}

export function validateSkillSummaryPage(value: unknown): ValidationResult<SkillSummaryPage> {
  if (!isRecord(value) || !Array.isArray(value.items) || !value.items.every(isSkillSummary) || !(value.nextCursor === null || typeof value.nextCursor === 'string')) {
    return {success: false, issues: [{path: '', message: 'skill summary page contains an unknown or malformed field'}]};
  }
  return {success: true, value: value as SkillSummaryPage};
}

export function validateSkillReferenceRead(value: unknown): ValidationResult<SkillReferenceRead> {
  if (!isRecord(value) || !hasString(value, 'companyId') || !hasString(value, 'taskId') || !hasString(value, 'employeeId') || !hasString(value, 'skillRevisionId') || !hasString(value, 'reference') || !hasString(value, 'contentDigest') || !hasString(value, 'body') || value.scriptsExecuted !== false) {
    return {success: false, issues: [{path: '', message: 'skill reference read contains an unknown or malformed field'}]};
  }
  return {success: true, value: value as SkillReferenceRead};
}

export function validateCapabilityCatalog(value: unknown): ValidationResult<CapabilityCatalogView> {
  if (!isRecord(value) || !Array.isArray(value.skills) || !Array.isArray(value.mcpServers) || !Array.isArray(value.bindings) || !Array.isArray(value.loads) || !value.skills.every(isSkillRevision) || !value.mcpServers.every(isMCPServerDefinition) || !value.bindings.every(isCapabilityBinding) || !value.loads.every(isSkillLoad)) {
    return {success: false, issues: [{path: '', message: 'capability catalog contains an unknown or malformed field'}]};
  }
  return {success: true, value: value as CapabilityCatalogView};
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

function isMCPToolCallQualification(value: unknown): value is MCPToolCallQualificationView {
  return isRecord(value)
    && hasString(value, 'companyId')
    && hasString(value, 'id')
    && hasString(value, 'bindingId')
    && hasString(value, 'definitionId')
    && hasString(value, 'handle')
    && hasString(value, 'descriptorDigest')
    && hasString(value, 'status')
    && hasString(value, 'effectClass')
    && hasString(value, 'transport');
}

export function validateMCPToolCallQualificationList(value: unknown): ValidationResult<ReadonlyArray<MCPToolCallQualificationView>> {
  if (!Array.isArray(value) || !value.every(isMCPToolCallQualification)) {
    return {success: false, issues: [{path: '', message: 'tool-call qualification list contains an unknown or malformed field'}]};
  }
  return {success: true, value: value as ReadonlyArray<MCPToolCallQualificationView>};
}

export function validateMCPToolCallQualification(value: unknown): ValidationResult<MCPToolCallQualificationView> {
  if (!isMCPToolCallQualification(value)) {
    return {success: false, issues: [{path: '', message: 'tool-call qualification contains an unknown or malformed field'}]};
  }
  return {success: true, value: value as MCPToolCallQualificationView};
}

function isMCPToolCallReceipt(value: unknown): value is MCPToolCallReceiptView {
  return isRecord(value)
    && hasString(value, 'companyId')
    && hasString(value, 'id')
    && hasString(value, 'employeeId')
    && hasString(value, 'bindingId')
    && hasString(value, 'handle')
    && hasString(value, 'descriptorDigest')
    && hasString(value, 'outcome')
    && hasString(value, 'createdAt');
}

export function validateMCPToolCallReceiptList(value: unknown): ValidationResult<ReadonlyArray<MCPToolCallReceiptView>> {
  if (!Array.isArray(value) || !value.every(isMCPToolCallReceipt)) {
    return {success: false, issues: [{path: '', message: 'tool-call receipt list contains an unknown or malformed field'}]};
  }
  return {success: true, value: value as ReadonlyArray<MCPToolCallReceiptView>};
}

export function validateMCPToolCallReceipt(value: unknown): ValidationResult<MCPToolCallReceiptView> {
  if (!isMCPToolCallReceipt(value)) {
    return {success: false, issues: [{path: '', message: 'tool-call receipt contains an unknown or malformed field'}]};
  }
  return {success: true, value: value as MCPToolCallReceiptView};
}

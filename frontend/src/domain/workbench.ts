// pattern: Functional Core

export type DataMode = 'real' | 'simulated' | 'unavailable';

export type MissionCommandType = 'mission.create' | 'mission.start' | 'mission.cancel';

export type MissionCommandResultState = 'draft' | 'active' | 'cancelled';

export type CompanyState = 'active' | 'archived';

export type EmployeeDraft = Readonly<{
  id: string;
  displayName: string;
  role: string;
  modelProfile: string;
}>;

export type CompanySummaryView = Readonly<{
  id: string;
  name: string;
  workspaceRoot: string;
  state: CompanyState;
  roster: ReadonlyArray<EmployeeDraft>;
}>;

export type RuntimeReadiness = 'configured' | 'missing' | 'invalid' | 'ready' | 'unavailable' | 'restart_required' | 'not_required' | 'not_applicable' | 'deferred';

export type RuntimeSettingsView = Readonly<{
  companyId: string;
  workerMode: string;
  provider: string;
  model: string;
  effort: string;
  profile: string;
  authReadiness: RuntimeReadiness;
  runtimeVersion: string;
  runtimeReadiness: RuntimeReadiness;
  productSurfaceQualification: string;
  workspaceRoot: string;
  postgresqlStatus: RuntimeReadiness;
  casStatus: RuntimeReadiness;
  eventStreamStatus: RuntimeReadiness;
}>;

export type CodexReasoningEffortOptionView = Readonly<{
  reasoningEffort: string;
  description: string;
}>;

export type CodexModelOptionView = Readonly<{
  model: string;
  displayName: string;
  description: string;
  isDefault: boolean;
  defaultReasoningEffort: string;
  supportedReasoningEfforts: ReadonlyArray<CodexReasoningEffortOptionView>;
}>;

export type CodexModelCatalogView = Readonly<{
  models: ReadonlyArray<CodexModelOptionView>;
}>;

export type OperatorInstructionState = 'pending' | 'applied' | 'rejected';

export type OperatorInstructionView = Readonly<{
  instructionId: string;
  missionId: string;
  taskId: string | null;
  employeeId: string | null;
  content: string;
  state: OperatorInstructionState;
  createdAt: string;
}>;

export type OperatorInstructionReceipt = Readonly<OperatorInstructionView & {
  requestId: string;
  accepted: true;
  acceptedAt: string;
}>;

export type CollaborationItem = Readonly<{
  messageId: string;
  missionId: string;
  taskId: string;
  senderEmployeeId: string;
  recipientEmployeeId: string;
  content: string;
  kind: string;
  deliveryState: string;
  contractRevisionId: string | null;
  obligationId: string | null;
  obligationState: string | null;
  evidenceRef: string | null;
  taskRevision: string;
}>;

export type WorkspaceFile = Readonly<{
  path: string;
  bytes: string;
  content: string;
}>;

export type WorkspaceView = Readonly<{
  taskId: string;
  revision: string;
  digest: string;
  files: ReadonlyArray<WorkspaceFile>;
  changedFiles: ReadonlyArray<string>;
  checkpoints: ReadonlyArray<CheckpointSummary>;
}>;

export type ArtifactDetailView = Readonly<{
  artifactId: string;
  taskId: string;
  digest: string;
  bytes: string;
  state: string;
  verdict: string;
  content: string;
  contentAvailable: boolean;
}>;

export type OperationsView = Readonly<{
  companyId: string;
  toolCallsUsed: string;
  toolCallsLimit: string;
  toolBudgetQuality: 'reported' | 'estimated' | 'unavailable';
  inputTokens: string | null;
  outputTokens: string | null;
  elapsedRuntime: string | null;
  workerCount: string;
  postgresqlStatus: string;
  casStatus: string;
  eventStreamStatus: string;
  lastRuntimeError: string | null;
}>;

export type NotificationRouteView = Readonly<{
  routeId: string;
  adapter: 'local' | 'webhook' | 'qq_official';
  enabled: boolean;
  destination: string;
  status: string;
  config: Record<string, unknown>;
  secretConfigured: boolean;
}>;

export type NotificationDeliveryView = Readonly<{
  deliveryId: string;
  intentId: string;
  adapter: string;
  state: string;
  errorCode: string | null;
  createdAt: string;
}>;

export type NotificationsView = Readonly<{
  routes: ReadonlyArray<NotificationRouteView>;
  deliveries: ReadonlyArray<NotificationDeliveryView>;
}>;

export type SkillRevisionView = Readonly<{
  companyId: string;
  id: string;
  publisherScope: 'group' | 'company';
  packageId: string;
  revision: string;
  displayName: string;
  sourceRef: string;
  contentDigest: string;
  manifest: unknown;
  status: 'candidate' | 'approved' | 'revoked';
  createdAt: string;
}>;

export type MCPServerDefinitionView = Readonly<{
  companyId: string;
  id: string;
  name: string;
  transport: 'stdio' | 'streamable_http';
  endpoint: string | null;
  command: string | null;
  args: ReadonlyArray<string>;
  descriptorDigest: string;
  status: 'unverified' | 'candidate' | 'approved' | 'revoked';
  qualification: unknown;
  createdAt: string;
}>;

export type CapabilityBindingView = Readonly<{
  companyId: string;
  id: string;
  employeeId: string;
  status: 'draft' | 'approved' | 'disabled' | 'revoked';
  skillRevisionIds: ReadonlyArray<string>;
  mcpDefinitionIds: ReadonlyArray<string>;
  qualification: unknown;
  createdAt: string;
}>;

export type ToolSummary = Readonly<{
  serverId: string;
  serverName: string;
  toolName: string;
  descriptorDigest: string;
  handle: string;
  toolsCalled: 0;
  callable: false;
}>;

export type ToolSummaryPage = Readonly<{
  items: ReadonlyArray<ToolSummary>;
  nextCursor: string | null;
}>;

export type ToolDescriptorView = Readonly<{
  serverId: string;
  serverName: string;
  toolName: string;
  description: string;
  inputSchema: Readonly<Record<string, unknown>>;
  outputSchema: Readonly<Record<string, unknown>> | null;
  descriptorDigest: string;
  handle: string;
  toolsCalled: 0;
  callable: false;
}>;

export type MCPToolCallQualificationView = Readonly<{
  companyId: string;
  id: string;
  bindingId: string;
  definitionId: string;
  serverId: string;
  toolName: string;
  handle: string;
  descriptorDigest: string;
  policyRevision: string;
  effectClass: string;
  status: string;
  authorizationRef: string;
  transport: string;
  qualifiedBy: string;
  qualifiedAt?: string | null;
  expiresAt?: string | null;
  notes: string;
  createdAt: string;
  updatedAt: string;
}>;

export type MCPToolCallReceiptView = Readonly<{
  companyId: string;
  id: string;
  employeeId: string;
  taskId?: string | null;
  sessionId?: string | null;
  sessionEpoch?: number | null;
  sessionIncarnation?: string;
  bindingId: string;
  definitionId: string;
  qualificationId?: string | null;
  handle: string;
  descriptorDigest: string;
  argumentsDigest: string;
  requestDigest: string;
  idempotencyKey: string;
  policyRevision: string;
  authorizationRef: string;
  effectClass: string;
  outcome: string;
  code: string;
  resultDigest: string;
  resultJson?: unknown;
  errorJson?: unknown;
  processObserved: unknown;
  egressObserved: unknown;
  startedAt?: string | null;
  finishedAt?: string | null;
  createdAt: string;
}>;

export type VisibleResource = Readonly<{
  serverId: string;
  serverName: string;
  uri: string;
  name: string;
  resourcesRead: 0;
  readable: false;
}>;

export type VisibleResourcePage = Readonly<{
  items: ReadonlyArray<VisibleResource>;
  nextCursor: string | null;
}>;

export type SkillSummary = Readonly<{
  id: string;
  packageId: string;
  revision: string;
  displayName: string;
  purpose: string;
  sourceRef: string;
  contentDigest: string;
  executionStatus: 'unverified';
  scriptsExecuted: false;
}>;

export type SkillSummaryPage = Readonly<{
  items: ReadonlyArray<SkillSummary>;
  nextCursor: string | null;
}>;

export type SkillReferenceRead = Readonly<{
  companyId: string;
  taskId: string;
  employeeId: string;
  skillRevisionId: string;
  reference: string;
  contentDigest: string;
  body: string;
  scriptsExecuted: false;
}>;

export type SkillLoadView = Readonly<{
  companyId: string;
  id: string;
  taskId: string;
  employeeId: string;
  skillRevisionId: string;
  contentDigest: string;
  body: string;
  references: ReadonlyArray<string>;
  scriptsExecuted: false;
  repeated: boolean;
  createdAt: string;
}>;

export type CapabilityCatalogView = Readonly<{
  skills: ReadonlyArray<SkillRevisionView>;
  mcpServers: ReadonlyArray<MCPServerDefinitionView>;
  bindings: ReadonlyArray<CapabilityBindingView>;
  loads: ReadonlyArray<SkillLoadView>;
}>;

export type MissionCommandReceipt = Readonly<{
  commandId: string;
  commandType: MissionCommandType;
  targetType: 'mission';
  targetId: string;
  requestId: string;
  accepted: true;
  acceptedAt: string;
  resultingState: MissionCommandResultState;
}>;

export type CompanyCommandType = 'company.create' | 'company.update' | 'company.archive' | 'runtime.settings.update' | 'notification.route.update';

export type CompanyCommandResultState = 'active' | 'archived' | 'restart_required';

export type CompanyCommandReceipt = Readonly<{
  commandId: string;
  commandType: CompanyCommandType;
  targetType: 'company';
  targetId: string;
  requestId: string;
  accepted: true;
  acceptedAt: string;
  resultingState: CompanyCommandResultState;
}>;

export type Freshness = 'fresh' | 'stale' | 'reconnecting' | 'unknown';

export type AcceptanceContract = Readonly<{
  revision: 'text-acceptance@1';
  required_text: ReadonlyArray<string>;
}>;

export type StatusKey =
  | 'working'
  | 'sleeping'
  | 'wake_pending'
  | 'waiting_tool'
  | 'waiting_peer'
  | 'waiting_external'
  | 'waiting_quota'
  | 'handover'
  | 'paused'
  | 'lost_contact'
  | 'stopped';

export type StatusTone = 'info' | 'neutral' | 'success' | 'warning' | 'danger';

export type ViewMeta = Readonly<{
  schemaVersion: number;
  companyId: string;
  entityRevision: string;
  snapshotCursor: string;
  observedAt: string;
  dataMode: DataMode;
  freshness: Freshness;
  sourceLabel: string;
  recoveryState: 'operational' | 'recovery_required' | 'unknown';
}>;

export type EntityRef = Readonly<{
  kind: string;
  id: string;
  label: string;
}>;

export type ActivityActor = Readonly<{
  kind: 'employee' | 'system';
  id: string;
  label: string;
}>;

export type ToolBudgetView = Readonly<{
  limit: string | null;
  used: string | null;
  remaining: string | null;
  quality: 'reported' | 'estimated' | 'unavailable';
}>;

export type QualificationView = Readonly<{
  status: 'supported' | 'limited' | 'unsupported' | 'unverified';
  evidenceId: string | null;
  policyRevision: string | null;
}>;

export type EmployeeStatusView = Readonly<{
  primary: StatusKey;
  tone: StatusTone;
  reason: string;
  activeModelRequests: string;
  inFlightTools: string;
  observedAt: string;
}>;

export type EmployeeSummary = Readonly<{
  employeeId: string;
  displayName: string;
  role: string;
  roleRevision: string | null;
  epoch: string;
  sessionId: string | null;
  sessionState: string | null;
  profile: string | null;
  currentTask: EntityRef | null;
  status: EmployeeStatusView;
  toolBudget: ToolBudgetView;
  qualification: QualificationView;
  openObligationCount: string;
}>;

export type TaskState = 'ready' | 'working' | 'candidate' | 'completed' | 'blocked' | 'cancelled';

export type TaskSummary = Readonly<{
  taskId: string;
  title: string;
  kind: string;
  state: TaskState;
  ownerEmployeeId: string;
  generation: string;
  contractRevisionId: string | null;
  workspaceRevision: string | null;
  acceptance: 'not_started' | 'candidate' | 'passed' | 'failed' | 'inconclusive';
  dependencyLabel: string | null;
}>;

export type ContractRevisionSummary = Readonly<{
  revisionId: string;
  revision: string;
  endpoint: string;
  state: 'proposed' | 'accepted' | 'superseded';
  digest: string;
  proposerEmployeeId: string;
  accepterEmployeeId: string | null;
}>;

export type ObligationSummary = Readonly<{
  obligationId: string;
  messageId: string;
  ownerEmployeeId: string;
  state: 'pending' | 'observed' | 'applied' | 'fulfilled' | 'declined' | 'superseded';
  evidenceRef: string | null;
  note: string;
}>;

export type ArtifactSummary = Readonly<{
  artifactId: string;
  taskId: string;
  authorEmployeeId: string;
  digest: string;
  bytes: string;
  state: 'ready' | 'missing' | 'corrupt';
  verdict: 'candidate' | 'passed' | 'failed' | 'invalidated';
  contractRevisionId: string | null;
  qualificationId: string | null;
  checkpointId: string | null;
}>;

export type CheckpointSummary = Readonly<{
  checkpointId: string;
  taskId: string;
  employeeId: string;
  sessionId: string;
  sessionState: string;
  sessionEpoch: string;
  kind: string;
  state: string;
  qualificationState: string;
  workspaceRevision: string | null;
  workspaceDigest: string | null;
  artifactId: string | null;
}>;

export type ResourceSummary = Readonly<{
  toolCallsUsed: string;
  toolCallsLimit: string;
  toolBudgetQuality: 'reported' | 'estimated' | 'unavailable';
  moneyQuality: 'reported' | 'estimated' | 'unavailable';
  moneyAmount: string | null;
  currency: string | null;
  asOf: string;
  note: string;
}>;

export type AttentionItem = Readonly<{
  id: string;
  tone: 'info' | 'warning' | 'danger';
  title: string;
  description: string;
  subject: EntityRef;
  evidenceRefs: ReadonlyArray<string>;
}>;

export type MissionSummary = Readonly<{
  missionId: string;
  title: string;
  goal: string;
  state: 'draft' | 'active' | 'paused' | 'succeeded' | 'cancelled';
  contract: string;
  acceptanceContract: AcceptanceContract | null;
  currentContractRevision: ContractRevisionSummary | null;
  nextMilestone: string;
  verifiedMilestones: string;
  milestoneTotal: string;
  milestones: ReadonlyArray<Readonly<{
    id: string;
    label: string;
    state: 'verified' | 'current' | 'upcoming' | 'blocked';
  }>>;
}>;

export type ActivityEventKind =
  | 'mission_created'
  | 'mission_started'
  | 'mission_cancelled'
  | 'provider_turn_completed'
  | 'provider_turn_failed'
  | 'provider_runtime_initialization_failed'
  | 'provider_runtime_thread_start_failed'
  | 'employee_started_task'
  | 'contract_revision_proposed'
  | 'contract_revision_accepted'
  | 'peer_message_sent'
  | 'obligation_created'
  | 'obligation_observed'
  | 'workspace_updated'
  | 'checkpoint_saved'
  | 'employee_stopped'
  | 'successor_resumed'
  | 'old_writer_rejected'
  | 'artifact_submitted'
  | 'acceptance_passed';

export type ActivityEvent = Readonly<{
  id: string;
  companySeq: string;
  occurredAt: string;
  kind: ActivityEventKind;
  actor: ActivityActor;
  subject: EntityRef;
  summary: string;
  detail: string;
  tone: StatusTone;
  evidenceRefs: ReadonlyArray<string>;
  metadata: Readonly<Record<string, string>>;
}>;

export type CompanyOverviewView = Readonly<{
  meta: ViewMeta;
  company: Readonly<{
    companyId: string;
    name: string;
    description: string;
  }>;
  mission: MissionSummary;
  team: Readonly<{
    total: string;
    working: string;
    sleeping: string;
    waiting: string;
    stopped: string;
  }>;
  employees: ReadonlyArray<EmployeeSummary>;
  tasks: ReadonlyArray<TaskSummary>;
  obligations: ReadonlyArray<ObligationSummary>;
  artifacts: ReadonlyArray<ArtifactSummary>;
  checkpoints: ReadonlyArray<CheckpointSummary>;
  resources: ResourceSummary;
  attention: ReadonlyArray<AttentionItem>;
  recentActivity: ReadonlyArray<ActivityEvent>;
}>;

export type ActivityView = Readonly<{
  meta: ViewMeta;
  items: ReadonlyArray<ActivityEvent>;
  nextCursor: string | null;
}>;

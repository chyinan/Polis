// pattern: Imperative Shell

import type {AcceptanceContract, ActivityEvent, ActivityView, ArtifactDetailView, CapabilityCatalogView, CodexModelCatalogView, CollaborationItem, CompanyCommandReceipt, CompanyOverviewView, CompanySummaryView, DataMode, EmployeeDraft, MCPToolCallQualificationView, MCPToolCallReceiptView, MissionCommandReceipt, NotificationsView, OperatorInstructionReceipt, OperatorInstructionView, OperationsView, RuntimeSettingsView, SkillReferenceRead, SkillSummaryPage, ToolDescriptorView, ToolSummaryPage, VisibleResourcePage, WorkspaceView} from '../domain/workbench';

export type CompanyDraftOptions = Readonly<{
  id: string;
  name: string;
  workspaceRoot: string;
  roster: ReadonlyArray<EmployeeDraft>;
  requestId: string;
}>;

export type UpdateCompanyOptions = Readonly<{
  companyId: string;
  name: string;
  workspaceRoot: string;
  roster: ReadonlyArray<EmployeeDraft>;
  requestId: string;
}>;

export type ArchiveCompanyOptions = Readonly<{
  companyId: string;
  requestId: string;
}>;

export type UpdateRuntimeSettingsOptions = Readonly<{
  companyId: string;
  provider: string;
  model: string;
  effort: string;
  profile: string;
  apiKey?: string;
  clearApiKey?: boolean;
  requestId: string;
}>;

export type OperatorInstructionQueryOptions = Readonly<{
  companyId: string;
}>;

export type CreateOperatorInstructionOptions = Readonly<{
  companyId: string;
  missionId: string;
  taskId: string | null;
  employeeId: string | null;
  content: string;
  requestId: string;
}>;

export type ConfigureNotificationRouteOptions = Readonly<{
  companyId: string;
  adapter: 'local' | 'webhook' | 'qq_official';
  destination: string;
  enabled: boolean;
  config?: Record<string, unknown>;
  secret?: string;
  clearSecret?: boolean;
  requestId: string;
}>;

export type TestNotificationOptions = Readonly<{
  companyId: string;
  adapter?: 'local' | 'webhook' | 'qq_official';
  requestId: string;
}>;

export type DispatchNotificationsOptions = Readonly<{
  companyId: string;
  adapter?: 'local' | 'webhook' | 'qq_official';
  limit?: number;
  requestId: string;
}>;

export type ImportSkillOptions = Readonly<{
  companyId: string;
  id?: string;
  publisherScope: 'group' | 'company';
  packageId: string;
  revision: string;
  displayName: string;
  sourceRef: string;
  contentDigest: string;
  manifest?: unknown;
  requestId: string;
}>;

export type ImportSkillDocumentOptions = Readonly<{
  companyId: string;
  publisherScope: 'group' | 'company';
  packageId: string;
  revision: string;
  sourceRef: string;
  document: string;
  requestId: string;
}>;

export type ImportSkillLibraryOptions = Readonly<{
  companyId: string;
  publisherScope: 'group' | 'company';
  packageId: string;
  revision: string;
  requestId: string;
}>;

export type ListVisibleToolsOptions = Readonly<{
  companyId: string;
  employeeId: string;
  cursor: string;
  limit: number;
  requestId: string;
}>;

export type DescribeVisibleToolOptions = Readonly<{
  companyId: string;
  employeeId: string;
  handle: string;
  requestId: string;
}>;

export type ListVisibleResourcesOptions = Readonly<{
  companyId: string;
  employeeId: string;
  serverId: string;
  cursor: string;
  limit: number;
  requestId: string;
}>;

export type ListVisibleSkillsOptions = Readonly<{
  companyId: string;
  employeeId: string;
  cursor: string;
  limit: number;
  requestId: string;
}>;

export type LoadSkillOptions = Readonly<{
  companyId: string;
  taskId: string;
  employeeId: string;
  skillId: string;
  requestId: string;
}>;

export type ReadSkillReferenceOptions = Readonly<{
  companyId: string;
  taskId: string;
  employeeId: string;
  skillId: string;
  reference: string;
  requestId: string;
}>;

export type SkillRevisionStatusOptions = Readonly<{
  companyId: string;
  skillId: string;
  requestId: string;
}>;

export type QualifyMCPOptions = Readonly<{
  companyId: string;
  mcpId: string;
  requestId: string;
}>;

export type UpsertCapabilityBindingOptions = Readonly<{
  companyId: string;
  employeeId: string;
  skillRevisionIds: ReadonlyArray<string>;
  mcpDefinitionIds: ReadonlyArray<string>;
  status: 'draft' | 'approved' | 'disabled' | 'revoked';
  requestId: string;
}>;

export type ListMCPToolCallQualificationsOptions = Readonly<{
  companyId: string;
  bindingId?: string;
  requestId: string;
}>;

export type ListMCPToolCallReceiptsOptions = Readonly<{
  companyId: string;
  bindingId?: string;
  handle?: string;
  limit?: number;
  requestId: string;
}>;

export type GetMCPToolCallReceiptOptions = Readonly<{
  companyId: string;
  receiptId: string;
  requestId: string;
}>;

export type UpsertMCPToolCallQualificationOptions = Readonly<{
  companyId: string;
  bindingId: string;
  definitionId: string;
  handle: string;
  descriptorDigest: string;
  policyRevision?: string;
  effectClass?: 'read' | 'idempotent' | 'effectful' | 'unknown';
  status?: 'draft' | 'approved' | 'disabled' | 'revoked' | 'expired';
  authorizationRef?: string;
  qualifiedBy?: string;
  notes?: string;
  requestId: string;
}>;

export type RegisterMCPOptions = Readonly<{
  companyId: string;
  id: string;
  name: string;
  transport: 'stdio' | 'streamable_http';
  endpoint: string | null;
  command: string | null;
  args: ReadonlyArray<string>;
  descriptorDigest: string;
  requestId: string;
}>;

export type CompanyScopeOptions = Readonly<{
  companyId: string;
}>;

export type ActivityQueryOptions = Readonly<{
  companyId: string;
  cursor: string | null;
  limit: number;
  snapshotCursor: string | null;
}>;

export type ActivityEventListener = (event: ActivityEvent) => void;

export type ActivityStreamStatus = 'connecting' | 'open' | 'reconnecting' | 'incompatible' | 'closed';

export type ActivityStreamOptions = Readonly<{
  companyId: string;
  cursor: string;
}>;

export type ActivityStreamStatusListener = (status: ActivityStreamStatus) => void;

export type CreateMissionOptions = Readonly<{
  companyId: string;
  title: string;
  goal: string;
  acceptanceContract: AcceptanceContract | null;
  requestId: string;
}>;

export type MissionCommandOptions = Readonly<{
  companyId: string;
  missionId: string;
  requestId: string;
}>;

export class CommandApiError extends Error {
  readonly code: string;
  readonly status: number;
  readonly targetId: string | null;
  readonly currentState: string | null;

  constructor(code: string, status: number, message: string, targetId: string | null = null, currentState: string | null = null) {
    super(message);
    this.name = 'CommandApiError';
    this.code = code;
    this.status = status;
    this.targetId = targetId;
    this.currentState = currentState;
  }
}

export type WorkbenchApi = Readonly<{
  readonly mode: DataMode;
  listCompanies(): Promise<ReadonlyArray<CompanySummaryView>>;
  getRuntimeSettings(options: CompanyScopeOptions): Promise<RuntimeSettingsView>;
  getCodexModelCatalog(options: CompanyScopeOptions): Promise<CodexModelCatalogView>;
  createCompany(options: CompanyDraftOptions): Promise<CompanyCommandReceipt>;
  updateCompany(options: UpdateCompanyOptions): Promise<CompanyCommandReceipt>;
  archiveCompany(options: ArchiveCompanyOptions): Promise<CompanyCommandReceipt>;
  updateRuntimeSettings(options: UpdateRuntimeSettingsOptions): Promise<CompanyCommandReceipt>;
  listOperatorInstructions(options: OperatorInstructionQueryOptions): Promise<ReadonlyArray<OperatorInstructionView>>;
  sendOperatorInstruction(options: CreateOperatorInstructionOptions): Promise<OperatorInstructionReceipt>;
  listCollaboration(options: CompanyScopeOptions): Promise<ReadonlyArray<CollaborationItem>>;
  getWorkspace(options: Readonly<{companyId: string; taskId: string}>): Promise<WorkspaceView>;
  getArtifact(options: Readonly<{companyId: string; artifactId: string}>): Promise<ArtifactDetailView>;
  getOperations(options: CompanyScopeOptions): Promise<OperationsView>;
  listNotifications(options: CompanyScopeOptions): Promise<NotificationsView>;
  listCapabilityCatalog(options: CompanyScopeOptions): Promise<CapabilityCatalogView>;
  importSkill(options: ImportSkillOptions): Promise<unknown>;
  importSkillDocument(options: ImportSkillDocumentOptions): Promise<unknown>;
  importSkillLibrary(options: ImportSkillLibraryOptions): Promise<unknown>;
  listVisibleTools(options: ListVisibleToolsOptions): Promise<ToolSummaryPage>;
  describeVisibleTool(options: DescribeVisibleToolOptions): Promise<ToolDescriptorView>;
  listVisibleResources(options: ListVisibleResourcesOptions): Promise<VisibleResourcePage>;
  listVisibleSkills(options: ListVisibleSkillsOptions): Promise<SkillSummaryPage>;
  loadSkill(options: LoadSkillOptions): Promise<unknown>;
  readSkillReference(options: ReadSkillReferenceOptions): Promise<SkillReferenceRead>;
  approveSkill(options: SkillRevisionStatusOptions): Promise<unknown>;
  revokeSkill(options: SkillRevisionStatusOptions): Promise<unknown>;
  upsertCapabilityBinding(options: UpsertCapabilityBindingOptions): Promise<unknown>;
  listMCPToolCallQualifications(options: ListMCPToolCallQualificationsOptions): Promise<ReadonlyArray<MCPToolCallQualificationView>>;
  listMCPToolCallReceipts(options: ListMCPToolCallReceiptsOptions): Promise<ReadonlyArray<MCPToolCallReceiptView>>;
  getMCPToolCallReceipt(options: GetMCPToolCallReceiptOptions): Promise<MCPToolCallReceiptView>;
  upsertMCPToolCallQualification(options: UpsertMCPToolCallQualificationOptions): Promise<MCPToolCallQualificationView>;
  registerMCP(options: RegisterMCPOptions): Promise<unknown>;
  qualifyMCP(options: QualifyMCPOptions): Promise<unknown>;
  configureNotificationRoute(options: ConfigureNotificationRouteOptions): Promise<CompanyCommandReceipt>;
  testNotification(options: TestNotificationOptions): Promise<Readonly<{intentId: string; deliveryId: string; adapter: string; state: string; requestId: string; accepted: true; acceptedAt: string}>>;
  dispatchNotifications(options: DispatchNotificationsOptions): Promise<Readonly<{intents: number; dispatched: number; delivered: number; failed: number; requestId: string; accepted: true; acceptedAt: string}>>;
  getCompanyOverview(options: CompanyScopeOptions): Promise<CompanyOverviewView>;
  listActivityEvents(options: ActivityQueryOptions): Promise<ActivityView>;
  createMission(options: CreateMissionOptions): Promise<MissionCommandReceipt>;
  startMission(options: MissionCommandOptions): Promise<MissionCommandReceipt>;
  cancelMission(options: MissionCommandOptions): Promise<MissionCommandReceipt>;
  subscribeToActivityEvents(options: ActivityStreamOptions, listener: ActivityEventListener, onStatus?: ActivityStreamStatusListener): () => void;
}>;

export function isValidActivityLimit(limit: number): boolean {
  return Number.isInteger(limit) && limit >= 1 && limit <= 100;
}

export function isValidOpaqueCursor(cursor: string | null): boolean {
  return cursor === null || cursor.trim() !== '';
}

export function assertCompanyScope(companyId: string): void {
  if (!/^[a-zA-Z0-9_-]{1,80}$/.test(companyId)) {
    throw new Error('failed to read workbench: invalid company scope');
  }
}

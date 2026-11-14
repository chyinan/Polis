// pattern: Imperative Shell

import type {ActivityView, ArtifactDetailView, CapabilityCatalogView, CodexModelCatalogView, CollaborationItem, CompanyCommandReceipt, CompanyOverviewView, CompanySummaryView, MissionCommandReceipt, NotificationsView, OperatorInstructionReceipt, OperatorInstructionView, OperationsView, RuntimeSettingsView, WorkspaceView} from '../domain/workbench';
import {validateActivityEvent, validateActivityView, validateArtifactDetail, validateCapabilityCatalog, validateCodexModelCatalog, validateCollaboration, validateCompanyList, validateCompanyOverview, validateMCPToolCallQualification, validateMCPToolCallQualificationList, validateMCPToolCallReceipt, validateMCPToolCallReceiptList, validateMissionCommandReceipt, validateNotifications, validateOperatorInstructionReceipt, validateOperatorInstructions, validateOperations, validateRuntimeSettings, validateSkillReferenceRead, validateSkillSummaryPage, validateVisibleResourcePage, validateToolDescriptor, validateToolSummaryPage, validateWorkspace, validationMessage} from '../domain/workbench-validation';
import {assertCompanyScope, CommandApiError, isValidActivityLimit, isValidOpaqueCursor, type ActivityEventListener, type ActivityQueryOptions, type ActivityStreamOptions, type ActivityStreamStatusListener, type ArchiveCompanyOptions, type CompanyDraftOptions, type CompanyScopeOptions, type ConfigureNotificationRouteOptions, type CreateMissionOptions, type CreateOperatorInstructionOptions, type DescribeVisibleToolOptions, type GetMCPToolCallReceiptOptions, type ImportSkillDocumentOptions, type ImportSkillLibraryOptions, type ImportSkillOptions, type ListMCPToolCallQualificationsOptions, type ListMCPToolCallReceiptsOptions, type ListVisibleResourcesOptions, type ListVisibleSkillsOptions, type ListVisibleToolsOptions, type LoadSkillOptions, type MissionCommandOptions, type OperatorInstructionQueryOptions, type QualifyMCPOptions, type ReadSkillReferenceOptions, type RegisterMCPOptions, type SkillRevisionStatusOptions, type TestNotificationOptions, type UpdateCompanyOptions, type UpdateRuntimeSettingsOptions, type UpsertCapabilityBindingOptions, type UpsertMCPToolCallQualificationOptions, type WorkbenchApi} from './workbench-api';

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
    const raw = await this.post('/companies', options.requestId, {id: options.id, name: options.name, workspaceRoot: options.workspaceRoot, roster: options.roster, requestId: options.requestId});
    return validateCompanyCommandReceipt(raw, 'company.create');
  }

  async updateCompany(options: UpdateCompanyOptions): Promise<CompanyCommandReceipt> {
    assertCompanyScope(options.companyId);
    assertRequestID(options.requestId);
    const raw = await this.post(`/companies/${encodeURIComponent(options.companyId)}/organization`, options.requestId, {name: options.name, workspaceRoot: options.workspaceRoot, roster: options.roster, requestId: options.requestId});
    return validateCompanyCommandReceipt(raw, 'company.update');
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
    const body: {provider: string; model: string; effort: string; profile: string; requestId: string; apiKey?: string; clearApiKey?: boolean} = {provider: options.provider, model: options.model, effort: options.effort, profile: options.profile, requestId: options.requestId};
    if (options.apiKey) body.apiKey = options.apiKey;
    if (options.clearApiKey) body.clearApiKey = true;
    const raw = await this.post(`/companies/${encodeURIComponent(options.companyId)}/settings`, options.requestId, body);
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
    assertCompanyScope(options.missionId);
    assertRequestID(options.requestId);
    const raw = await this.post(`/companies/${encodeURIComponent(options.companyId)}/instructions`, options.requestId, {missionId: options.missionId, taskId: options.taskId, employeeId: options.employeeId, content: options.content, requestId: options.requestId});
    const result = validateOperatorInstructionReceipt(raw);
    if (!result.success) {
      throw new Error(`failed to parse operator instruction receipt: ${validationMessage(result.issues)}`);
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
    const result = validateOperations(raw);
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
    const result = validateCapabilityCatalog(raw);
    if (!result.success) throw new Error(`failed to load capability catalog: ${validationMessage(result.issues)}`);
    return result.value;
  }

  async importSkill(options: ImportSkillOptions): Promise<unknown> {
    assertCompanyScope(options.companyId);
    assertRequestID(options.requestId);
    return this.post(`/companies/${encodeURIComponent(options.companyId)}/capabilities/skills`, options.requestId, withoutCompany(options));
  }

  async importSkillDocument(options: ImportSkillDocumentOptions): Promise<unknown> {
    assertCompanyScope(options.companyId);
    assertRequestID(options.requestId);
    return this.post(`/companies/${encodeURIComponent(options.companyId)}/capabilities/skills/manifest`, options.requestId, withoutCompany(options));
  }

  async importSkillLibrary(options: ImportSkillLibraryOptions): Promise<unknown> {
    assertCompanyScope(options.companyId);
    assertRequestID(options.requestId);
    return this.post(`/companies/${encodeURIComponent(options.companyId)}/capabilities/skills/library`, options.requestId, withoutCompany(options));
  }

  async describeVisibleTool(options: DescribeVisibleToolOptions) {
    assertCompanyScope(options.companyId);
    assertRequestID(options.requestId);
    const raw = await this.post(`/companies/${encodeURIComponent(options.companyId)}/capabilities/tools/describe`, options.requestId, {employeeId: options.employeeId, handle: options.handle, requestId: options.requestId});
    const result = validateToolDescriptor(raw);
    if (!result.success) throw new Error(`failed to describe visible tool: ${validationMessage(result.issues)}`);
    return result.value;
  }

  async listVisibleResources(options: ListVisibleResourcesOptions) {
    assertCompanyScope(options.companyId);
    assertRequestID(options.requestId);
    const raw = await this.post(`/companies/${encodeURIComponent(options.companyId)}/capabilities/resources/list`, options.requestId, {employeeId: options.employeeId, serverId: options.serverId, cursor: options.cursor, limit: options.limit, requestId: options.requestId});
    const result = validateVisibleResourcePage(raw);
    if (!result.success) throw new Error(`failed to list visible resources: ${validationMessage(result.issues)}`);
    return result.value;
  }

  async listVisibleTools(options: ListVisibleToolsOptions) {
    assertCompanyScope(options.companyId);
    assertRequestID(options.requestId);
    const raw = await this.post(`/companies/${encodeURIComponent(options.companyId)}/capabilities/tools/list`, options.requestId, {employeeId: options.employeeId, cursor: options.cursor, limit: options.limit, requestId: options.requestId});
    const result = validateToolSummaryPage(raw);
    if (!result.success) throw new Error(`failed to list visible tools: ${validationMessage(result.issues)}`);
    return result.value;
  }

  async listVisibleSkills(options: ListVisibleSkillsOptions) {
    assertCompanyScope(options.companyId);
    assertRequestID(options.requestId);
    const raw = await this.post(`/companies/${encodeURIComponent(options.companyId)}/capabilities/skills/list`, options.requestId, {employeeId: options.employeeId, cursor: options.cursor, limit: options.limit, requestId: options.requestId});
    const result = validateSkillSummaryPage(raw);
    if (!result.success) throw new Error(`failed to list visible skills: ${validationMessage(result.issues)}`);
    return result.value;
  }

  async loadSkill(options: LoadSkillOptions): Promise<unknown> {
    assertCompanyScope(options.companyId);
    assertRequestID(options.requestId);
    return this.post(`/companies/${encodeURIComponent(options.companyId)}/capabilities/skills/load`, options.requestId, {taskId: options.taskId, employeeId: options.employeeId, skillId: options.skillId, requestId: options.requestId});
  }

  async readSkillReference(options: ReadSkillReferenceOptions) {
    assertCompanyScope(options.companyId);
    assertRequestID(options.requestId);
    const raw = await this.post(`/companies/${encodeURIComponent(options.companyId)}/capabilities/skills/reference`, options.requestId, {taskId: options.taskId, employeeId: options.employeeId, skillId: options.skillId, reference: options.reference, requestId: options.requestId});
    const result = validateSkillReferenceRead(raw);
    if (!result.success) throw new Error(`failed to read skill reference: ${validationMessage(result.issues)}`);
    return result.value;
  }

  async approveSkill(options: SkillRevisionStatusOptions): Promise<unknown> {
    return this.postSkillStatus(options, 'approve');
  }

  async revokeSkill(options: SkillRevisionStatusOptions): Promise<unknown> {
    return this.postSkillStatus(options, 'revoke');
  }

  async upsertCapabilityBinding(options: UpsertCapabilityBindingOptions): Promise<unknown> {
    assertCompanyScope(options.companyId);
    assertRequestID(options.requestId);
    return this.post(`/companies/${encodeURIComponent(options.companyId)}/capabilities/bindings`, options.requestId, withoutCompany(options));
  }

  async listMCPToolCallQualifications(options: ListMCPToolCallQualificationsOptions) {
    assertCompanyScope(options.companyId);
    assertRequestID(options.requestId);
    const raw = await this.post(`/companies/${encodeURIComponent(options.companyId)}/capabilities/tools/call-qualifications`, options.requestId, {bindingId: options.bindingId, requestId: options.requestId});
    const result = validateMCPToolCallQualificationList(raw);
    if (!result.success) throw new Error(`failed to list tool-call qualifications: ${validationMessage(result.issues)}`);
    return result.value;
  }

  async listMCPToolCallReceipts(options: ListMCPToolCallReceiptsOptions) {
    assertCompanyScope(options.companyId);
    assertRequestID(options.requestId);
    const raw = await this.post(`/companies/${encodeURIComponent(options.companyId)}/capabilities/tools/call-receipts`, options.requestId, {bindingId: options.bindingId, handle: options.handle, limit: options.limit, requestId: options.requestId});
    const result = validateMCPToolCallReceiptList(raw);
    if (!result.success) throw new Error(`failed to list tool-call receipts: ${validationMessage(result.issues)}`);
    return result.value;
  }

  async getMCPToolCallReceipt(options: GetMCPToolCallReceiptOptions) {
    assertCompanyScope(options.companyId);
    assertRequestID(options.requestId);
    const raw = await this.post(`/companies/${encodeURIComponent(options.companyId)}/capabilities/tools/call-receipt`, options.requestId, {receiptId: options.receiptId, requestId: options.requestId});
    const result = validateMCPToolCallReceipt(raw);
    if (!result.success) throw new Error(`failed to get tool-call receipt: ${validationMessage(result.issues)}`);
    return result.value;
  }

  async upsertMCPToolCallQualification(options: UpsertMCPToolCallQualificationOptions) {
    assertCompanyScope(options.companyId);
    assertRequestID(options.requestId);
    const raw = await this.post(`/companies/${encodeURIComponent(options.companyId)}/capabilities/tools/call-qualification`, options.requestId, withoutCompany(options));
    const result = validateMCPToolCallQualification(raw);
    if (!result.success) throw new Error(`failed to upsert tool-call qualification: ${validationMessage(result.issues)}`);
    return result.value;
  }

  async qualifyMCP(options: QualifyMCPOptions): Promise<unknown> {
    assertCompanyScope(options.companyId);
    assertRequestID(options.requestId);
    return this.post(`/companies/${encodeURIComponent(options.companyId)}/capabilities/mcp/qualify`, options.requestId, {mcpId: options.mcpId, requestId: options.requestId});
  }

  async registerMCP(options: RegisterMCPOptions): Promise<unknown> {
    assertCompanyScope(options.companyId);
    assertRequestID(options.requestId);
    return this.post(`/companies/${encodeURIComponent(options.companyId)}/capabilities/mcp`, options.requestId, withoutCompany(options));
  }

  private postSkillStatus(options: SkillRevisionStatusOptions, action: 'approve' | 'revoke'): Promise<unknown> {
    assertCompanyScope(options.companyId);
    assertRequestID(options.requestId);
    return this.post(`/companies/${encodeURIComponent(options.companyId)}/capabilities/skills/${action}`, options.requestId, {skillId: options.skillId, requestId: options.requestId});
  }

  async configureNotificationRoute(options: ConfigureNotificationRouteOptions): Promise<CompanyCommandReceipt> {
    assertCompanyScope(options.companyId);
    assertRequestID(options.requestId);
    const raw = await this.post(`/companies/${encodeURIComponent(options.companyId)}/notifications/route`, options.requestId, {
      adapter: options.adapter,
      destination: options.destination,
      enabled: options.enabled,
      config: options.config ?? {},
      secret: options.secret,
      clearSecret: options.clearSecret,
      requestId: options.requestId,
    });
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

  async createMission(options: CreateMissionOptions): Promise<MissionCommandReceipt> {
    assertCompanyScope(options.companyId);
    assertRequestID(options.requestId);
    const raw = await this.post(`/companies/${encodeURIComponent(options.companyId)}/missions`, options.requestId, {title: options.title, goal: options.goal, acceptanceContract: options.acceptanceContract, requestId: options.requestId});
    return validateCommandReceipt(raw, 'mission.create');
  }

  async startMission(options: MissionCommandOptions): Promise<MissionCommandReceipt> {
    assertCompanyScope(options.companyId);
    assertCompanyScope(options.missionId);
    assertRequestID(options.requestId);
    const raw = await this.post(`/companies/${encodeURIComponent(options.companyId)}/missions/${encodeURIComponent(options.missionId)}/start`, options.requestId, {requestId: options.requestId});
    return validateCommandReceipt(raw, 'mission.start');
  }

  async cancelMission(options: MissionCommandOptions): Promise<MissionCommandReceipt> {
    assertCompanyScope(options.companyId);
    assertCompanyScope(options.missionId);
    assertRequestID(options.requestId);
    const raw = await this.post(`/companies/${encodeURIComponent(options.companyId)}/missions/${encodeURIComponent(options.missionId)}/cancel`, options.requestId, {requestId: options.requestId});
    return validateCommandReceipt(raw, 'mission.cancel');
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

  private async post(path: string, requestID: string, body: unknown): Promise<unknown> {
    const response = await fetch(`${this.baseUrl}${path}`, {
      method: 'POST',
      credentials: 'same-origin',
      headers: this.requestHeaders({'Accept': 'application/json', 'Content-Type': 'application/json', 'X-Request-ID': requestID}),
      body: JSON.stringify(body),
    });
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

  private requestHeaders(headers: Record<string, string>): Record<string, string> {
    return this.sessionToken === null ? headers : {...headers, 'X-Polis-Desktop-Token': this.sessionToken};
  }
}

function withoutCompany<T extends {companyId: string}>(options: T): Omit<T, 'companyId'> {
  const body: Partial<T> = {...options};
  delete body.companyId;
  return body as Omit<T, 'companyId'>;
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
  if (!isRecord(raw) || typeof raw.commandId !== 'string' || raw.commandType !== expectedType || raw.targetType !== 'company' || typeof raw.targetId !== 'string' || typeof raw.requestId !== 'string' || raw.accepted !== true || typeof raw.acceptedAt !== 'string' || (raw.resultingState !== 'active' && raw.resultingState !== 'archived' && raw.resultingState !== 'restart_required')) {
    throw new Error('failed to parse company command receipt: response is malformed');
  }
  return raw as CompanyCommandReceipt;
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
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
    const source = new EventSource(path, {withCredentials: true});
    source.onopen = () => onStatus?.('open');
    source.onerror = () => onStatus?.('reconnecting');
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

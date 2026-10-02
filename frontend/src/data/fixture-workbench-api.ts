// pattern: Imperative Shell

import type {CreateProjectJobBrowserSessionOptions, ImportStdioMCPPackageOptions, ObserveStdioMCPRuntimeOptions, ObserveStreamableHTTPMCPRuntimeOptions} from './workbench-api';
import type {RunResearchSimulationOptions} from './workbench-api';
import type {RecordContentReviewOptions, RegisterContentDraftOptions, SetContentSourceAuthorizationOptions} from './workbench-api';
import type {RecordContentCorrectionOptions, RecordContentFeedbackOptions, SimulateContentPublicationOptions} from './workbench-api';
import type {CreateDailyRoutineOptions, DailyRoutineQueryOptions, MemoryTaskRevalidationPreviewOptions, MemoryTaskStatusQueryOptions, RevalidateMemoryTaskOptions, SetDailyRoutineTaskInstructionOptions} from './workbench-api';
import type {DailyRoutineCommandReceipt, DailyRoutineView, MemoryTaskRevalidationPreviewView, MemoryTaskRevalidationReceipt, MemoryTaskStatusView} from '../domain/workbench';
import type {StdioMCPPackageRevisionView} from '../domain/workbench';
import type {ServiceBrowserSessionView} from '../domain/workbench';

import type {
  ActivityEvent,
  ActivityView,
  ArtifactDeliveryManifestResponse,
  ArtifactDetailView,
  CapabilityCatalogView,
  CollaborationItem,
  CompanyCommandReceipt,
  CompanyFeedbackView,
  GitHubCredentialReceipt,
  CodexModelCatalogView,
  CompanyOverviewView,
  CompanySummaryView,
  CrossBackendHandoverView,
  DomainEvidenceLedgerView,
  DomainEvidenceArtifactPreviewView,
  DomainEvidenceArtifactPreviewManifestView,
  DomainEvidenceRecordView,
  DomainEvidenceReviewRecordView,
  DomainEvidenceSubstantiveAssessmentRecordView,
  DomainProfileQualificationRecordView,
  EmployeeSummary,
  EnvironmentExecutorQualificationReceipt,
  EnvironmentPolicyDecisionReceipt,
  EnvironmentPreparationRunView,
  HumanInterventionCommandReceipt,
  JobRunCommandReceipt,
  JobRunLogArtifactView,
  JobRunView,
  MissionCommandReceipt,
  MissionChangeRequestView,
  OperatorInstructionReceipt,
  OperatorInstructionView,
  OperationsView,
  NotificationsView,
  ProjectEnvironmentRevisionView,
  RuntimeSettingsView,
  TaskTakeoverLeaseView,
  WorkspaceView,
} from '../domain/workbench';
import type {MissionInputCommandReceipt, MissionInputView, TaskInputManifestView} from '../domain/mission-input'
import {validateActivityView, validateCompanyOverview, validationMessage} from '../domain/workbench-validation';
import {assertCompanyScope, CommandApiError, isValidActivityLimit, isValidOpaqueCursor, type ActivityEventListener, type ActivityQueryOptions, type ActivityStreamOptions, type ActivityStreamStatusListener, type BindEmployeeCapabilityOptions, type CompanyScopeOptions, type CreateMissionOptions, type CreateMissionChangeRequestOptions, type CreateTaskEnvironmentHandoverOptions, type CreateOperatorInstructionOptions, type CreateTaskTakeoverLeaseOptions, type DecideCapabilityOptions, type DecideGitHubFeedbackSourceOptions, type DeleteGitHubCredentialOptions, type EnvironmentExecutorQualificationOptions, type EnvironmentPolicyDecisionOptions, type EnsureEnvironmentOptions, type GetDomainEvidenceArtifactPreviewOptions, type ImportReadOnlySkillPackageOptions, type ListDomainEvidenceArtifactPreviewEntriesOptions, type MissionChangeRequestCommandOptions, type MissionChangeRequestQueryOptions, type MissionCommandOptions, type MissionInputQueryOptions, type PollGitHubFeedbackSourceOptions, type ProbeGitHubFeedbackSourceOptions, type QualifyCapabilityOptions, type RecordDomainEvidenceOptions, type RecordDomainEvidenceReviewOptions, type RecordDomainEvidenceSubstantiveAssessmentOptions, type RecordDomainProfileQualificationOptions, type RegisterGitHubFeedbackSourceOptions, type ReleaseTaskTakeoverLeaseOptions, type SetHumanInterventionStateOptions, type StoreGitHubCredentialOptions, type TaskCrossBackendHandoversQueryOptions, type TaskInputManifestQueryOptions, type TaskJobLogsQueryOptions, type TaskJobRunsQueryOptions, type StartTaskJobRunOptions, type StopTaskJobRunOptions, type TaskTakeoverLeaseQueryOptions, type TaskTakeoverSnapshotOptions, type UploadMissionDirectoryInputOptions, type UploadMissionInputOptions, type OperatorInstructionQueryOptions, type UpdateRuntimeSettingsOptions, type WorkbenchApi} from './workbench-api';

export const FIXTURE_COMPANY_ID = 'r03a-t2-company-1789221294871371900';

const FIXTURE_MISSION_ID = 'r03a-t2-mission-1789221294871371900';

const BACKEND_TASK_ID = '8ac83ed6f11413d8a8d5e9925fe0c1c0';

const FRONTEND_TASK_ID = '3243c57410762f2cd8b666e3ad35c8c9';

const CONTRACT_REVISION_ID = '4f57c71061c10ab5a78f632d8db59321';

const MESSAGE_ID = '7d027b5a8fd2267a68b364e47f4635ca';

const CHECKPOINT_ID = '93054ca68a451d75f38dc98c4d490c0b';

const ARTIFACT_ID = 'a0191e5ce29a56eb347b421df39fb86e';

const FRONTEND_SESSION_ID = 'ccd5906064dce787c3dc864f6eb8aa7b';

const BACKEND_SESSION_ID = '54c3ebd10316add86f0035d94cf4ebb2';

const RUNTIME_INCARNATION = '5fda8dfb9339917c1deaf409b22ad425';

const ACTIVITY_EVENTS: ReadonlyArray<ActivityEvent> = [
  {
    id: 'activity-70',
    companySeq: '70',
    occurredAt: '2026-09-13T11:53:08.0850094Z',
    kind: 'acceptance_passed',
    actor: {kind: 'system', id: 'emp-review', label: '独立验收器'},
    subject: {kind: 'integration_candidate', id: 'integration-r03a-v2', label: 'R0.3A peer integration'},
    summary: '独立集成验收通过',
    detail: '前端接班路径完成，候选产物与当前 ContractRevision、workspace revision 和协作因果证据一致。',
    tone: 'success',
    evidenceRefs: ['integration-r03a-v2', 'r03a-pagination-contract@3', 'r03a-pagination-behavior@1'],
    metadata: {verdict: 'passed', acceptance_scope: 'lifecycle + collaboration + pagination behavior', source: 'R0.3A handover v2'},
  },
  {
    id: 'activity-69',
    companySeq: '69',
    occurredAt: '2026-09-13T11:53:06.5100000Z',
    kind: 'artifact_submitted',
    actor: {kind: 'employee', id: 'emp-frontend', label: '前端工程师'},
    subject: {kind: 'artifact', id: ARTIFACT_ID, label: '前端候选产物'},
    summary: '候选 Artifact 已提交',
    detail: 'Successor session 提交 artifact candidate；它仍需沿独立验收路径解释 verdict，不因提交动作自动变成已交付。',
    tone: 'info',
    evidenceRefs: [ARTIFACT_ID, '16f3c32b05a6a57edaf38d42b4c7ebf706be593c09b836ea70097dc2a0af768f'],
    metadata: {artifact_state: 'ready', artifact_verdict: 'candidate', workspace_revision: '5', session_epoch: '4'},
  },
  {
    id: 'activity-68',
    companySeq: '68',
    occurredAt: '2026-09-13T11:52:57.8810000Z',
    kind: 'checkpoint_saved',
    actor: {kind: 'employee', id: 'emp-frontend', label: '前端工程师'},
    subject: {kind: 'checkpoint', id: CHECKPOINT_ID, label: 'Successor checkpoint'},
    summary: 'Successor 保存 checkpoint',
    detail: 'checkpoint 绑定 emp-frontend epoch 4、当前 workspace revision 5 与 accepted ContractRevision rev3。checkpoint 是进度/资格证据节点，不是 UI 进度条。',
    tone: 'info',
    evidenceRefs: [CHECKPOINT_ID, CONTRACT_REVISION_ID, FRONTEND_SESSION_ID],
    metadata: {kind: 'qualified', finalization_state: 'current', workspace_revision: '5', epoch: '4'},
  },
  {
    id: 'activity-67',
    companySeq: '67',
    occurredAt: '2026-09-13T11:52:30.1710000Z',
    kind: 'workspace_updated',
    actor: {kind: 'employee', id: 'emp-frontend', label: '前端工程师'},
    subject: {kind: 'task', id: FRONTEND_TASK_ID, label: '实现并验证前端响应 consumer'},
    summary: '工作区更新至 revision 5',
    detail: 'workspace_replace receipt 使当前 workspace 前进；它证明了写入发生，不单独证明候选产物已经通过验收。',
    tone: 'info',
    evidenceRefs: ['16f3c32b05a6a57edaf38d42b4c7ebf706be593c09b836ea70097dc2a0af768f'],
    metadata: {task_id: FRONTEND_TASK_ID, workspace_revision: '5', digest: '16f3c32b05a6…', mutation: 'workspace_replace'},
  },
  {
    id: 'activity-66',
    companySeq: '66',
    occurredAt: '2026-09-13T11:51:34.0120000Z',
    kind: 'successor_resumed',
    actor: {kind: 'employee', id: 'emp-frontend', label: '前端工程师'},
    subject: {kind: 'worker_session', id: FRONTEND_SESSION_ID, label: 'Successor session'},
    summary: 'Successor 以 epoch 4 接班恢复',
    detail: 'Employee logical identity 保持为 emp-frontend；新 session 使用 epoch 4 继续同一责任，接班不是新员工。',
    tone: 'success',
    evidenceRefs: [FRONTEND_SESSION_ID, MESSAGE_ID, CONTRACT_REVISION_ID],
    metadata: {employee_id: 'emp-frontend', epoch: '4', incarnation: RUNTIME_INCARNATION, inherited_obligation: MESSAGE_ID},
  },
  {
    id: 'activity-65',
    companySeq: '65',
    occurredAt: '2026-09-13T11:51:33.5108719Z',
    kind: 'employee_stopped',
    actor: {kind: 'system', id: 'controller', label: '控制面'},
    subject: {kind: 'worker_session', id: 'a300088a9d514a9c85e7beff5399ace9', label: 'Initial frontend session'},
    summary: 'Initial session 已停止',
    detail: '旧 session 以 process-handle stop proof 终止；停止事实与后续 successor epoch 4 分开呈现。',
    tone: 'neutral',
    evidenceRefs: ['a300088a9d514a9c85e7beff5399ace9'],
    metadata: {employee_id: 'emp-frontend', epoch: '3', stop_receipt: 'windows-process-handle:2372:waited', turn_completed: 'true'},
  },
  {
    id: 'activity-64',
    companySeq: '64',
    occurredAt: '2026-09-13T11:50:56.2810000Z',
    kind: 'obligation_created',
    actor: {kind: 'system', id: 'controller', label: '控制面'},
    subject: {kind: 'obligation', id: MESSAGE_ID, label: '实现 peer contract 的责任'},
    summary: '为前端任务创建 Obligation',
    detail: '可操作 peer message 产生同 ID obligation；责任保持 pending/observed/applied/fulfilled 的独立生命周期。',
    tone: 'warning',
    evidenceRefs: [MESSAGE_ID, FRONTEND_TASK_ID],
    metadata: {obligation_state: 'pending', message_id: MESSAGE_ID, owner: 'emp-frontend', evidence_ref: 'none_at_creation'},
  },
  {
    id: 'activity-63',
    companySeq: '63',
    occurredAt: '2026-09-13T11:50:55.9020000Z',
    kind: 'peer_message_sent',
    actor: {kind: 'employee', id: 'emp-backend', label: '后端工程师'},
    subject: {kind: 'message', id: MESSAGE_ID, label: 'Peer request → emp-frontend'},
    summary: 'Peer message 发送给前端任务',
    detail: '消息引用 accepted ContractRevision rev3，并创建了前端需要处理的责任；消息发送不等于对方已应用。',
    tone: 'info',
    evidenceRefs: [MESSAGE_ID, CONTRACT_REVISION_ID, BACKEND_TASK_ID, FRONTEND_TASK_ID],
    metadata: {delivery_state: 'acknowledged', kind: 'request', contract_revision: '3', recipient: 'emp-frontend'},
  },
  {
    id: 'activity-62',
    companySeq: '62',
    occurredAt: '2026-09-13T11:50:49.3310000Z',
    kind: 'contract_revision_accepted',
    actor: {kind: 'employee', id: 'emp-planning', label: '规划工程师'},
    subject: {kind: 'contract_revision', id: CONTRACT_REVISION_ID, label: 'Cursor API Contract rev3'},
    summary: 'ContractRevision rev3 已接受',
    detail: '当前 accepted revision 为 rev3；后续 Message、Checkpoint 与 Artifact 都应引用这个版本或明确显示 superseded。',
    tone: 'success',
    evidenceRefs: [CONTRACT_REVISION_ID],
    metadata: {revision: '3', state: 'accepted', endpoint: 'GET /items?cursor=…', proposer: 'emp-backend', accepter: 'emp-planning'},
  },
  {
    id: 'activity-61',
    companySeq: '61',
    occurredAt: '2026-09-13T11:50:46.1100000Z',
    kind: 'contract_revision_proposed',
    actor: {kind: 'employee', id: 'emp-backend', label: '后端工程师'},
    subject: {kind: 'contract_revision', id: CONTRACT_REVISION_ID, label: 'Cursor API Contract rev3'},
    summary: 'Backend 提议 ContractRevision rev3',
    detail: '合同修订是可审查的业务版本；提议本身不代表 accepted，也不替换现有渲染/视图状态。',
    tone: 'info',
    evidenceRefs: [CONTRACT_REVISION_ID, BACKEND_SESSION_ID],
    metadata: {revision: '3', state: 'proposed', session_epoch: '2', endpoint: 'GET /items?cursor=…'},
  },
  {
    id: 'activity-60',
    companySeq: '60',
    occurredAt: '2026-09-13T11:49:02.1510000Z',
    kind: 'old_writer_rejected',
    actor: {kind: 'system', id: 'controller', label: '控制面'},
    subject: {kind: 'worker_session', id: 'a300088a9d514a9c85e7beff5399ace9', label: 'Historical writer'},
    summary: '旧 writer 被拒绝',
    detail: '历史 epoch 的写请求被 STALE_EPOCH fence 拒绝；拒绝是授权事实，不应被 UI 简化为普通网络失败。',
    tone: 'danger',
    evidenceRefs: ['old-writer-rejection.json', 'a300088a9d514a9c85e7beff5399ace9'],
    metadata: {reason_code: 'STALE_EPOCH', old_epoch: '3', current_epoch: '4', business_mutation: 'none'},
  },
];

function createEmployeeSummary(options: Readonly<{
  employeeId: string;
  displayName: string;
  role: string;
  epoch: string;
  roleRevision: string | null;
  sessionId: string | null;
  sessionState: string | null;
  profile: string | null;
  primary: EmployeeSummary['status']['primary'];
  tone: EmployeeSummary['status']['tone'];
  reason: string;
  currentTask: EmployeeSummary['currentTask'];
  openObligationCount: string;
  budget: EmployeeSummary['toolBudget'];
  qualification: EmployeeSummary['qualification'];
}>): EmployeeSummary {
  return {
    employeeId: options.employeeId,
    displayName: options.displayName,
    role: options.role,
    roleRevision: options.roleRevision,
    epoch: options.epoch,
    sessionId: options.sessionId,
    sessionState: options.sessionState,
    profile: options.profile,
    currentTask: options.currentTask,
    status: {
      primary: options.primary,
      tone: options.tone,
      reason: options.reason,
      activeModelRequests: options.primary === 'working' ? '1' : '0',
      inFlightTools: options.primary === 'working' ? '1' : '0',
      observedAt: '2026-09-13T11:53:08.0850094Z',
    },
    schedule: null,
    toolBudget: options.budget,
    qualification: options.qualification,
    openObligationCount: options.openObligationCount,
  };
}

function buildFixtureOverview(): CompanyOverviewView {
  const employees: ReadonlyArray<EmployeeSummary> = [
    createEmployeeSummary({
      employeeId: 'emp-planning',
      displayName: '规划工程师',
      role: '目标、合同与验收协调',
      epoch: '1',
      roleRevision: null,
      sessionId: null,
      sessionState: null,
      profile: 'gpt-5.6-luna/medium',
      primary: 'sleeping',
      tone: 'neutral',
      reason: '当前没有新的可执行规划任务；最近观察保持不变。',
      currentTask: null,
      openObligationCount: '0',
      budget: {limit: null, used: null, remaining: null, quality: 'unavailable'},
      qualification: {status: 'supported', evidenceId: 'r03a-local-qualification', policyRevision: 'r03a-pagination-contract@3'},
    }),
    createEmployeeSummary({
      employeeId: 'emp-backend',
      displayName: '后端工程师',
      role: '接口实现与候选产物',
      epoch: '2',
      roleRevision: null,
      sessionId: BACKEND_SESSION_ID,
      sessionState: 'stopped',
      profile: 'gpt-5.6-luna/medium',
      primary: 'stopped',
      tone: 'neutral',
      reason: 'Backend session 已停止；候选工作资产仍可沿证据链查看。',
      currentTask: {kind: 'task', id: BACKEND_TASK_ID, label: '实现 cursor API 后端'},
      openObligationCount: '0',
      budget: {limit: '48', used: '32', remaining: '16', quality: 'reported'},
      qualification: {status: 'limited', evidenceId: 'r03a-real-backend-employee/continuation-4', policyRevision: 'r03a-pagination-behavior@1'},
    }),
    createEmployeeSummary({
      employeeId: 'emp-frontend',
      displayName: '前端工程师',
      role: '响应 consumer 与交付候选',
      epoch: '4',
      roleRevision: null,
      sessionId: FRONTEND_SESSION_ID,
      sessionState: 'stopped',
      profile: 'gpt-5.6-luna/medium',
      primary: 'stopped',
      tone: 'neutral',
      reason: 'Successor 已完成并停止；Employee 身份保留，epoch 4 的 session 记录可下钻。',
      currentTask: {kind: 'task', id: FRONTEND_TASK_ID, label: '实现并验证前端响应 consumer'},
      openObligationCount: '0',
      budget: {limit: '48', used: '16', remaining: '32', quality: 'reported'},
      qualification: {status: 'supported', evidenceId: 'r0.3a-real-frontend-handover-revised-v2', policyRevision: 'peer-semantic-checker@3'},
    }),
    createEmployeeSummary({
      employeeId: 'emp-review',
      displayName: '独立验收员',
      role: '独立审查与验收证据',
      epoch: '1',
      roleRevision: null,
      sessionId: null,
      sessionState: null,
      profile: 'gpt-5.6-luna/high',
      primary: 'sleeping',
      tone: 'neutral',
      reason: '没有新的候选产物等待独立验收。',
      currentTask: null,
      openObligationCount: '0',
      budget: {limit: null, used: null, remaining: null, quality: 'unavailable'},
      qualification: {status: 'supported', evidenceId: 'r0.3a-h1-independent-peer-collaboration-review', policyRevision: 'review-isolation@1'},
    }),
  ];
  return {
    meta: {
      schemaVersion: 1,
      companyId: FIXTURE_COMPANY_ID,
      entityRevision: '9007199254740993',
      snapshotCursor: 'fixture://r03a-handover-v2/company-seq-70',
      observedAt: '2026-09-13T11:53:08.0850094Z',
      dataMode: 'simulated',
      freshness: 'fresh',
      sourceLabel: '基于已保存的 R0.3A handover v2 证据锚点',
      recoveryState: 'operational',
    },
    company: {
      companyId: FIXTURE_COMPANY_ID,
      name: 'Polis 协作研发公司',
      description: '一条固定编制、可追踪接班与独立验收的长期协作工作台。',
    },
    mission: {
      missionId: FIXTURE_MISSION_ID,
      title: '验证可持续的协作交付链',
      goal: '在保留合同、消息、责任与证据边界的前提下完成 cursor API 协作交付。',
      state: 'active',
      contract: 'r03-api@1',
      acceptanceContract: null,
      currentContractRevision: {
        revisionId: CONTRACT_REVISION_ID,
        revision: '3',
        endpoint: 'GET /items?cursor=…',
        state: 'accepted',
        digest: '5e390a59d40bd8d307b1b4b2280a6c1021980c07be62e4c3e0c1851db997b294',
        proposerEmployeeId: 'emp-backend',
        accepterEmployeeId: 'emp-planning',
      },
      nextMilestone: '等待新的可验证输入或进入受控维护期',
      verifiedMilestones: '3',
      milestoneTotal: '4',
      milestones: [
        {id: 'milestone-contract', label: '合同版本已确认', state: 'verified'},
        {id: 'milestone-obligation', label: '协作责任已闭合', state: 'verified'},
        {id: 'milestone-acceptance', label: '独立验收已通过', state: 'verified'},
        {id: 'milestone-maintenance', label: '受控维护期', state: 'current'},
      ],
    },
    team: {total: '4', working: '0', sleeping: '2', waiting: '0', stopped: '2'},
    employees,
    tasks: [
      {taskId: BACKEND_TASK_ID, title: '实现 cursor API 后端', kind: 'peer_backend', state: 'candidate', ownerEmployeeId: 'emp-backend', generation: '2', contractRevisionId: CONTRACT_REVISION_ID, workspaceRevision: '9', acceptance: 'passed', dependencyLabel: null},
      {taskId: FRONTEND_TASK_ID, title: '实现并验证前端响应 consumer', kind: 'peer_frontend', state: 'completed', ownerEmployeeId: 'emp-frontend', generation: '2', contractRevisionId: CONTRACT_REVISION_ID, workspaceRevision: '5', acceptance: 'passed', dependencyLabel: '依赖后端候选与 accepted rev3'},
    ],
    obligations: [
      {obligationId: MESSAGE_ID, messageId: MESSAGE_ID, ownerEmployeeId: 'emp-frontend', state: 'fulfilled', evidenceRef: ARTIFACT_ID, note: '由 frontend successor epoch 4 完成并绑定 artifact evidence。'},
    ],
    artifacts: [
      {artifactId: ARTIFACT_ID, taskId: FRONTEND_TASK_ID, authorEmployeeId: 'emp-frontend', digest: '16f3c32b05a6a57edaf38d42b4c7ebf706be593c09b836ea70097dc2a0af768f', bytes: '—', state: 'ready', verdict: 'passed', contractRevisionId: CONTRACT_REVISION_ID, qualificationId: null, checkpointId: CHECKPOINT_ID},
    ],
    checkpoints: [
      {checkpointId: CHECKPOINT_ID, taskId: FRONTEND_TASK_ID, employeeId: 'emp-frontend', sessionId: FRONTEND_SESSION_ID, sessionState: 'stopped', sessionEpoch: '4', kind: 'qualified', state: 'current', qualificationState: 'qualified', workspaceRevision: '5', workspaceDigest: '16f3c32b05a6a57edaf38d42b4c7ebf706be593c09b836ea70097dc2a0af768f', artifactId: ARTIFACT_ID},
    ],
    resources: {
      toolCallsUsed: '48',
      toolCallsLimit: '96',
      toolBudgetQuality: 'reported',
      moneyQuality: 'unavailable',
      moneyAmount: null,
      currency: 'USD',
      asOf: '2026-09-13T11:53:08+08:00',
      note: '工具调用预算可见；美元费用未从当前控制面报告，不代表免费。',
    },
    attention: [
      {id: 'attention-simulated', tone: 'info', title: '当前为模拟观察模式', description: '页面基于保存的 R0.3A 证据锚点；未连接实时控制 API，不会触发模型或业务动作。', subject: {kind: 'company', id: FIXTURE_COMPANY_ID, label: '当前公司'}, evidenceRefs: ['fixture://r03a-handover-v2']},
      {id: 'attention-unknown-cost', tone: 'warning', title: '美元费用当前不可得', description: '工具调用预算与实际计费不是同一口径；费用未知不会被转换成 0。', subject: {kind: 'resource', id: 'company-resource-summary', label: '公司资源'}, evidenceRefs: []},
    ],
    recentActivity: ACTIVITY_EVENTS.slice(0, 6),
  };
}

export class FixtureWorkbenchApi implements WorkbenchApi {
  readonly mode = 'simulated' as const;
  private readonly overview: CompanyOverviewView = buildFixtureOverview();

  async listCompanies(): Promise<ReadonlyArray<CompanySummaryView>> {
    return [{id: FIXTURE_COMPANY_ID, name: this.overview.company.name, workspaceRoot: 'fixture://workspace', state: 'active', roster: this.overview.employees.map(employee => ({id: employee.employeeId, displayName: employee.displayName, role: employee.role, modelProfile: employee.profile ?? 'unavailable'}))}];
  }

  async getRuntimeSettings(options: CompanyScopeOptions): Promise<RuntimeSettingsView> {
    assertCompanyScope(options.companyId);
    if (options.companyId !== FIXTURE_COMPANY_ID) {
      throw new Error('failed to load fixture runtime settings: company scope not found');
    }
    return {companyId: options.companyId, workerMode: 'fixture', provider: 'deterministic', model: 'fixture', effort: 'bounded', profile: 'fixture', authReadiness: 'not_required', runtimeVersion: 'fixture', runtimeReadiness: 'ready', productSurfaceQualification: 'not_applicable', workspaceRoot: 'fixture://workspace', postgresqlStatus: 'not_applicable', casStatus: 'not_applicable', eventStreamStatus: 'deferred'};
  }

  async getCodexModelCatalog(options: CompanyScopeOptions): Promise<CodexModelCatalogView> {
    assertCompanyScope(options.companyId);
    if (options.companyId !== FIXTURE_COMPANY_ID) {
      throw new Error('failed to load fixture Codex model catalog: company scope not found');
    }
    return {models: []};
  }

  async createCompany(): Promise<CompanyCommandReceipt> {
    throw new CommandApiError('SIMULATED_MODE', 409, 'commands are unavailable in fixture mode');
  }

  async updateCompany(): Promise<CompanyCommandReceipt> {
    throw new CommandApiError('SIMULATED_MODE', 409, 'commands are unavailable in fixture mode');
  }

  async archiveCompany(): Promise<CompanyCommandReceipt> {
    throw new CommandApiError('SIMULATED_MODE', 409, 'commands are unavailable in fixture mode');
  }

  async updateRuntimeSettings(_options: UpdateRuntimeSettingsOptions): Promise<CompanyCommandReceipt> {
    throw new CommandApiError('SIMULATED_MODE', 409, 'commands are unavailable in fixture mode');
  }

  async listOperatorInstructions(_options: OperatorInstructionQueryOptions): Promise<ReadonlyArray<OperatorInstructionView>> {
    return [];
  }

  async sendOperatorInstruction(_options: CreateOperatorInstructionOptions): Promise<OperatorInstructionReceipt> {
    throw new CommandApiError('SIMULATED_MODE', 409, 'commands are unavailable in fixture mode');
  }

  async listMissionChangeRequests(options: MissionChangeRequestQueryOptions): Promise<ReadonlyArray<MissionChangeRequestView>> {
    assertCompanyScope(options.companyId);
    assertCompanyScope(options.missionId);
    if (options.companyId !== FIXTURE_COMPANY_ID || options.missionId !== FIXTURE_MISSION_ID) {
      throw new Error('failed to load fixture formal changes: Mission scope not found');
    }
    return [];
  }

  async createMissionChangeRequest(_options: CreateMissionChangeRequestOptions): Promise<MissionChangeRequestView> {
    throw new CommandApiError('SIMULATED_MODE', 409, 'formal changes are unavailable in fixture mode');
  }

  async considerMissionChangeRequest(_options: MissionChangeRequestCommandOptions): Promise<MissionChangeRequestView> {
    throw new CommandApiError('SIMULATED_MODE', 409, 'formal changes are unavailable in fixture mode');
  }

  async declineMissionChangeRequest(_options: MissionChangeRequestCommandOptions): Promise<MissionChangeRequestView> {
    throw new CommandApiError('SIMULATED_MODE', 409, 'formal changes are unavailable in fixture mode');
  }

  async applyMissionChangeRequest(_options: MissionChangeRequestCommandOptions): Promise<MissionChangeRequestView> {
    throw new CommandApiError('SIMULATED_MODE', 409, 'formal changes are unavailable in fixture mode');
  }

  async listTaskTakeoverLeases(options: TaskTakeoverLeaseQueryOptions): Promise<ReadonlyArray<TaskTakeoverLeaseView>> {
    assertCompanyScope(options.companyId);
    assertCompanyScope(options.missionId);
    if (options.companyId !== FIXTURE_COMPANY_ID || options.missionId !== FIXTURE_MISSION_ID) {
      throw new Error('failed to load fixture human takeover leases: Mission scope not found');
    }
    return [];
  }

  async createTaskTakeoverLease(_options: CreateTaskTakeoverLeaseOptions): Promise<TaskTakeoverLeaseView> {
    throw new CommandApiError('SIMULATED_MODE', 409, 'human takeover is unavailable in fixture mode');
  }

  async submitTaskTakeoverSnapshot(_options: TaskTakeoverSnapshotOptions): Promise<TaskTakeoverLeaseView> {
    throw new CommandApiError('SIMULATED_MODE', 409, 'human takeover is unavailable in fixture mode');
  }

  async releaseTaskTakeoverLease(_options: ReleaseTaskTakeoverLeaseOptions): Promise<TaskTakeoverLeaseView> {
    throw new CommandApiError('SIMULATED_MODE', 409, 'human takeover is unavailable in fixture mode');
  }

  async listCollaboration(_options: CompanyScopeOptions): Promise<ReadonlyArray<CollaborationItem>> {
    return [];
  }

  async getWorkspace(_options: Readonly<{companyId: string; taskId: string}>): Promise<WorkspaceView> {
    throw new Error('failed to load fixture workspace: no task workspace in fixture snapshot');
  }

  async getArtifact(_options: Readonly<{companyId: string; artifactId: string}>): Promise<ArtifactDetailView> {
    throw new Error('failed to load fixture artifact: no artifact detail in fixture snapshot');
  }

  async getArtifactDeliveryManifest(_options: Readonly<{companyId: string; artifactId: string}>): Promise<ArtifactDeliveryManifestResponse> {
    throw new Error('fixture snapshot has no qualified artifact delivery manifest');
  }

  async downloadArtifactPackage(_options: Readonly<{companyId: string; artifactId: string}>): Promise<Blob> {
    throw new Error('fixture snapshot does not contain an authorized artifact delivery package');
  }

  async getOperations(options: CompanyScopeOptions): Promise<OperationsView> {
    assertCompanyScope(options.companyId);
    return {companyId: options.companyId, toolCallsUsed: this.overview.resources.toolCallsUsed, toolCallsLimit: this.overview.resources.toolCallsLimit, toolBudgetQuality: this.overview.resources.toolBudgetQuality, inputTokens: null, outputTokens: null, elapsedRuntime: null, workerCount: '0', postgresqlStatus: 'not_applicable', casStatus: 'not_applicable', eventStreamStatus: 'deferred', lastRuntimeError: null};
  }

  async listNotifications(_options: CompanyScopeOptions): Promise<NotificationsView> {
    return {routes: [], deliveries: []};
  }

  async getCompanyFeedback(options: CompanyScopeOptions): Promise<CompanyFeedbackView> {
    assertCompanyScope(options.companyId);
    return {companyId: options.companyId, sources: [], issues: []};
  }

  async storeGitHubFeedbackCredential(_options: StoreGitHubCredentialOptions): Promise<GitHubCredentialReceipt> {
    throw new CommandApiError('SIMULATED_MODE', 409, 'credential storage is unavailable in fixture mode');
  }

  async deleteGitHubFeedbackCredential(_options: DeleteGitHubCredentialOptions): Promise<GitHubCredentialReceipt> {
    throw new CommandApiError('SIMULATED_MODE', 409, 'credential storage is unavailable in fixture mode');
  }

  async registerGitHubFeedbackSource(_options: RegisterGitHubFeedbackSourceOptions): Promise<never> {
    throw new CommandApiError('SIMULATED_MODE', 409, 'GitHub sources are unavailable in fixture mode');
  }

  async probeGitHubFeedbackSource(_options: ProbeGitHubFeedbackSourceOptions): Promise<never> {
    throw new CommandApiError('SIMULATED_MODE', 409, 'GitHub permission probes are unavailable in fixture mode');
  }

  async decideGitHubFeedbackSource(_options: DecideGitHubFeedbackSourceOptions): Promise<never> {
    throw new CommandApiError('SIMULATED_MODE', 409, 'GitHub source decisions are unavailable in fixture mode');
  }

  async pollGitHubFeedbackSource(_options: PollGitHubFeedbackSourceOptions): Promise<never> {
    throw new CommandApiError('SIMULATED_MODE', 409, 'GitHub polling is unavailable in fixture mode');
  }

  async setGitHubFeedbackBacklogStatus(_options: Parameters<WorkbenchApi['setGitHubFeedbackBacklogStatus']>[0]): Promise<never> {
    throw new CommandApiError('SIMULATED_MODE', 409, 'GitHub backlog controls are unavailable in fixture mode');
  }

  async setGitHubFeedbackCollectionPolicy(_options: Parameters<WorkbenchApi['setGitHubFeedbackCollectionPolicy']>[0]): Promise<never> {
    throw new CommandApiError('SIMULATED_MODE', 409, 'GitHub collection policy controls are unavailable in fixture mode');
  }

  async listCapabilityCatalog(_options: CompanyScopeOptions): Promise<CapabilityCatalogView> {
    return {skills: [], mcpServers: [], mcpPackages: [], qualifications: [], bindings: [], decisions: [], runtimeQualifications: [], revocations: [], revocationsTruncated: false, runtimeObservationAvailable: false};
  }

  async listDomainEvidence(options: CompanyScopeOptions): Promise<DomainEvidenceLedgerView> {
    return {
      companyId: options.companyId,
      profiles: [
        {
          id: 'content-operations-reference', revision: 'content-operations@1', domain: 'content_operations',
          qualificationStatus: 'not_run', executionEnabled: false,
          stages: ['authorized_sources', 'versioned_draft', 'independent_fact_check', 'human_sample', 'simulated_publication', 'correction_and_feedback'],
          requiredEvidence: ['quality', 'intervention', 'recovery', 'cost', 'organization_benefit'],
        },
        {
          id: 'research-simulation-reference', revision: 'research-simulation@1', domain: 'research_simulation',
          qualificationStatus: 'not_run', executionEnabled: false,
          stages: ['pin_dataset', 'preregister_protocol', 'define_control_and_risk_budget', 'run_simulation', 'independent_evaluation', 'retain_negative_result'],
          requiredEvidence: ['quality', 'recovery', 'cost', 'organization_benefit'],
        },
      ],
      submissions: [],
      qualifications: [],
      researchSimulationRuns: [],
      contentSourceEvents: [],
      contentDrafts: [],
      contentReviews: [],
      contentPublications: [],
      contentCorrections: [],
      contentFeedback: [],
    };
  }

  async runResearchSimulation(_options: RunResearchSimulationOptions): Promise<never> {
    throw new CommandApiError('SIMULATED_MODE', 409, 'Research simulation command is unavailable in fixture mode');
  }

  async setContentSourceAuthorization(_options: SetContentSourceAuthorizationOptions): Promise<never> {
    throw new CommandApiError('SIMULATED_MODE', 409, 'Content source authorization is unavailable in fixture mode');
  }

  async registerContentDraft(_options: RegisterContentDraftOptions): Promise<never> {
    throw new CommandApiError('SIMULATED_MODE', 409, 'Content draft registration is unavailable in fixture mode');
  }

  async recordContentReview(_options: RecordContentReviewOptions): Promise<never> {
    throw new CommandApiError('SIMULATED_MODE', 409, 'Content reviews are unavailable in fixture mode');
  }

  async simulateContentPublication(_options: SimulateContentPublicationOptions): Promise<never> {
    throw new CommandApiError('SIMULATED_MODE', 409, 'Content publication simulation is unavailable in fixture mode');
  }

  async recordContentCorrection(_options: RecordContentCorrectionOptions): Promise<never> {
    throw new CommandApiError('SIMULATED_MODE', 409, 'Content corrections are unavailable in fixture mode');
  }

  async recordContentFeedback(_options: RecordContentFeedbackOptions): Promise<never> {
    throw new CommandApiError('SIMULATED_MODE', 409, 'Content feedback is unavailable in fixture mode');
  }

  async recordDomainEvidence(_options: RecordDomainEvidenceOptions): Promise<DomainEvidenceRecordView> {
    throw new CommandApiError('SIMULATED_MODE', 409, 'domain evidence submissions require the real company ledger');
  }

  async recordDomainEvidenceReview(_options: RecordDomainEvidenceReviewOptions): Promise<DomainEvidenceReviewRecordView> {
    throw new CommandApiError('SIMULATED_MODE', 409, 'domain evidence reviews require the real company ledger');
  }

  async recordDomainEvidenceSubstantiveAssessment(_options: RecordDomainEvidenceSubstantiveAssessmentOptions): Promise<DomainEvidenceSubstantiveAssessmentRecordView> {
    throw new CommandApiError('SIMULATED_MODE', 409, 'substantive domain assessments require the real company ledger');
  }

  async recordDomainProfileQualification(_options: RecordDomainProfileQualificationOptions): Promise<DomainProfileQualificationRecordView> {
    throw new CommandApiError('SIMULATED_MODE', 409, 'domain profile qualification decisions require the real company ledger');
  }

  async getDomainEvidenceArtifactPreview(_options: GetDomainEvidenceArtifactPreviewOptions): Promise<DomainEvidenceArtifactPreviewView> {
    throw new CommandApiError('SIMULATED_MODE', 409, 'domain evidence previews require the real company ledger');
  }

  async listDomainEvidenceArtifactPreviewEntries(_options: ListDomainEvidenceArtifactPreviewEntriesOptions): Promise<DomainEvidenceArtifactPreviewManifestView> {
    throw new CommandApiError('SIMULATED_MODE', 409, 'domain evidence preview entries require the real company ledger');
  }

  async qualifyCapability(_options: QualifyCapabilityOptions): Promise<unknown> {
    throw new Error('fixture snapshot does not contain capability qualification state');
  }

  async decideCapability(_options: DecideCapabilityOptions): Promise<unknown> {
    throw new Error('fixture snapshot does not support capability decisions');
  }

  async approveStdioMCPRuntimeQualification(_options: Readonly<{companyId: string; runtimeQualificationId: string; rationale: string; requestId: string}>): Promise<unknown> {
    throw new CommandApiError('SIMULATED_MODE', 409, 'MCP runtime qualifications are unavailable in fixture mode');
  }

  async observeStdioMCPRuntime(_options: ObserveStdioMCPRuntimeOptions): Promise<unknown> {
    throw new CommandApiError('SIMULATED_MODE', 409, 'MCP runtime observation is unavailable in fixture mode');
  }

  async observeStreamableHTTPMCPRuntime(_options: ObserveStreamableHTTPMCPRuntimeOptions): Promise<unknown> {
    return {status: 'simulated'};
  }

  async bindEmployeeCapability(_options: BindEmployeeCapabilityOptions): Promise<unknown> {
    throw new Error('fixture snapshot does not support employee capability bindings');
  }

  async revokeEmployeeCapability(_options: BindEmployeeCapabilityOptions): Promise<unknown> {
    throw new Error('fixture snapshot does not support employee capability revocation');
  }

  async importSkill(): Promise<unknown> {
    throw new CommandApiError('SIMULATED_MODE', 409, 'capability imports are unavailable in fixture mode');
  }

  async importReadOnlySkillPackage(_options: ImportReadOnlySkillPackageOptions): Promise<unknown> {
    throw new CommandApiError('SIMULATED_MODE', 409, 'Skill package imports are unavailable in fixture mode');
  }

  async importStdioMCPPackage(_options: ImportStdioMCPPackageOptions): Promise<StdioMCPPackageRevisionView> {
    throw new CommandApiError('SIMULATED_MODE', 409, 'controlled stdio MCP package imports are unavailable in fixture mode');
  }

  async registerMCP(): Promise<unknown> {
    throw new CommandApiError('SIMULATED_MODE', 409, 'capability registration is unavailable in fixture mode');
  }

  async configureNotificationRoute(): Promise<CompanyCommandReceipt> {
    throw new CommandApiError('SIMULATED_MODE', 409, 'commands are unavailable in fixture mode');
  }

  async testNotification(): Promise<Readonly<{intentId: string; deliveryId: string; adapter: string; state: string; requestId: string; accepted: true; acceptedAt: string}>> {
    throw new CommandApiError('SIMULATED_MODE', 409, 'commands are unavailable in fixture mode');
  }

  async getCompanyOverview(options: CompanyScopeOptions): Promise<CompanyOverviewView> {
    assertCompanyScope(options.companyId);
    const result = validateCompanyOverview(this.overview, options.companyId);
    if (!result.success) {
      throw new Error(`failed to load fixture overview: ${validationMessage(result.issues)}`);
    }
    return result.value;
  }

  async listDailyRoutines(options: DailyRoutineQueryOptions): Promise<ReadonlyArray<DailyRoutineView>> {
    assertCompanyScope(options.companyId);
    assertCompanyScope(options.missionId);
    if (options.companyId !== FIXTURE_COMPANY_ID || options.missionId !== this.overview.mission.missionId) {
      throw new Error('failed to load fixture daily Routines: scope not found');
    }
    return [];
  }

  async getMemoryTaskStatus(options: MemoryTaskStatusQueryOptions): Promise<MemoryTaskStatusView> {
    assertCompanyScope(options.companyId);
    assertCompanyScope(options.taskId);
    if (options.companyId !== FIXTURE_COMPANY_ID || !this.overview.tasks.some(task => task.taskId === options.taskId)) {
      throw new Error('failed to load fixture task memory impact: scope not found');
    }
    return {state: 'clear', impacts: []};
  }

  async getMemoryTaskRevalidationPreview(_options: MemoryTaskRevalidationPreviewOptions): Promise<MemoryTaskRevalidationPreviewView> {
    throw new CommandApiError('SIMULATED_MODE', 409, 'memory revalidation previews are unavailable in fixture mode');
  }

  async revalidateMemoryTask(_options: RevalidateMemoryTaskOptions): Promise<MemoryTaskRevalidationReceipt> {
    throw new CommandApiError('SIMULATED_MODE', 409, 'memory revalidation commands are unavailable in fixture mode');
  }

  async listActivityEvents(options: ActivityQueryOptions): Promise<ActivityView> {
    assertCompanyScope(options.companyId);
    if (!isValidActivityLimit(options.limit)) {
      throw new Error('failed to load fixture activity: limit must be between 1 and 100');
    }
    if (options.companyId !== FIXTURE_COMPANY_ID) {
      throw new Error('failed to load fixture activity: company scope not found');
    }
    if (!isValidOpaqueCursor(options.cursor) || !isValidOpaqueCursor(options.snapshotCursor)) {
      throw new Error('failed to load fixture activity: cursor cannot be blank');
    }
    if (options.snapshotCursor !== this.overview.meta.snapshotCursor) {
      throw new Error('failed to load fixture activity: snapshot cursor is not recognized');
    }
    const cursorIndex = options.cursor === null ? 0 : ACTIVITY_EVENTS.findIndex(event => event.companySeq === options.cursor);
    if (cursorIndex < 0) {
      throw new Error('failed to load fixture activity: cursor is not recognized for this snapshot');
    }
    const startIndex = cursorIndex;
    const items = ACTIVITY_EVENTS.slice(startIndex, startIndex + options.limit);
    const nextIndex = startIndex + items.length;
    const nextCursor = nextIndex < ACTIVITY_EVENTS.length ? ACTIVITY_EVENTS[nextIndex].companySeq : null;
    const view: ActivityView = {
      meta: this.overview.meta,
      items,
      nextCursor,
    };
    const result = validateActivityView(view, options.companyId);
    if (!result.success) {
      throw new Error(`failed to load fixture activity: ${validationMessage(result.issues)}`);
    }
    return result.value;
  }

  async listMissionInputs(options: MissionInputQueryOptions): Promise<ReadonlyArray<MissionInputView>> {
    assertCompanyScope(options.companyId);
    assertCompanyScope(options.missionId);
    if (options.companyId !== FIXTURE_COMPANY_ID || options.missionId !== this.overview.mission.missionId) {
      throw new Error('failed to load fixture mission inputs: scope not found');
    }
    return [];
  }

  async getTaskInputManifest(_options: TaskInputManifestQueryOptions): Promise<TaskInputManifestView> {
    throw new Error('task input manifests are unavailable in fixture mode');
  }

  async listProjectEnvironments(options: CompanyScopeOptions): Promise<ReadonlyArray<ProjectEnvironmentRevisionView>> {
    assertCompanyScope(options.companyId);
    return [];
  }

  async decideProjectEnvironmentPolicy(_options: EnvironmentPolicyDecisionOptions): Promise<EnvironmentPolicyDecisionReceipt> {
    throw new CommandApiError('SIMULATED_MODE', 409, 'environment policy commands are unavailable in fixture mode');
  }

  async decideProjectEnvironmentExecutorQualification(_options: EnvironmentExecutorQualificationOptions): Promise<EnvironmentExecutorQualificationReceipt> {
    throw new CommandApiError('SIMULATED_MODE', 409, 'environment executor qualification is unavailable in fixture mode');
  }

  async ensureProjectEnvironment(_options: EnsureEnvironmentOptions): Promise<EnvironmentPreparationRunView> {
    throw new CommandApiError('SIMULATED_MODE', 409, 'environment preparation is unavailable in fixture mode');
  }

  async listTaskJobRuns(options: TaskJobRunsQueryOptions): Promise<ReadonlyArray<JobRunView>> {
    assertCompanyScope(options.companyId);
    assertCompanyScope(options.taskId);
    return [];
  }

  async listTaskCrossBackendHandovers(options: TaskCrossBackendHandoversQueryOptions): Promise<ReadonlyArray<CrossBackendHandoverView>> {
    assertCompanyScope(options.companyId);
    assertCompanyScope(options.taskId);
    return [];
  }

  async createTaskEnvironmentHandover(_options: CreateTaskEnvironmentHandoverOptions): Promise<CrossBackendHandoverView> {
    throw new CommandApiError('SIMULATED_MODE', 409, 'cross-backend handovers are unavailable in fixture mode');
  }

  async startTaskJobRun(_options: StartTaskJobRunOptions): Promise<JobRunCommandReceipt> {
    throw new CommandApiError('SIMULATED_MODE', 409, 'project JobRuns cannot be started in fixture mode');
  }

  async stopTaskJobRun(_options: StopTaskJobRunOptions): Promise<JobRunCommandReceipt> {
    throw new CommandApiError('SIMULATED_MODE', 409, 'project JobRuns cannot be stopped in fixture mode');
  }

  async getTaskJobLogs(_options: TaskJobLogsQueryOptions): Promise<JobRunLogArtifactView> {
    throw new CommandApiError('SIMULATED_MODE', 409, 'project JobRun logs are unavailable in fixture mode');
  }

  async createProjectJobBrowserSession(_options: CreateProjectJobBrowserSessionOptions): Promise<ServiceBrowserSessionView> {
    throw new CommandApiError('SIMULATED_MODE', 409, 'service browser sessions are unavailable in fixture mode');
  }

  async uploadMissionInput(_options: UploadMissionInputOptions): Promise<MissionInputCommandReceipt> {
    throw new CommandApiError('SIMULATED_MODE', 409, 'commands are unavailable in fixture mode');
  }

  async uploadMissionDirectoryInput(_options: UploadMissionDirectoryInputOptions): Promise<MissionInputCommandReceipt> {
    throw new CommandApiError('SIMULATED_MODE', 409, 'commands are unavailable in fixture mode');
  }

  async createMission(_options: CreateMissionOptions): Promise<MissionCommandReceipt> {
    throw new CommandApiError('SIMULATED_MODE', 409, 'commands are unavailable in fixture mode');
  }

  async createDailyRoutine(_options: CreateDailyRoutineOptions): Promise<DailyRoutineCommandReceipt> {
    throw new CommandApiError('SIMULATED_MODE', 409, 'commands are unavailable in fixture mode');
  }

  async setDailyRoutineTaskInstruction(_options: SetDailyRoutineTaskInstructionOptions): Promise<DailyRoutineCommandReceipt> {
    throw new CommandApiError('SIMULATED_MODE', 409, 'commands are unavailable in fixture mode');
  }

  async startMission(_options: MissionCommandOptions): Promise<MissionCommandReceipt> {
    throw new CommandApiError('SIMULATED_MODE', 409, 'commands are unavailable in fixture mode');
  }

  async pauseMission(_options: MissionCommandOptions): Promise<MissionCommandReceipt> {
    throw new CommandApiError('SIMULATED_MODE', 409, 'commands are unavailable in fixture mode');
  }

  async resumeMission(_options: MissionCommandOptions): Promise<MissionCommandReceipt> {
    throw new CommandApiError('SIMULATED_MODE', 409, 'commands are unavailable in fixture mode');
  }

  async cancelMission(_options: MissionCommandOptions): Promise<MissionCommandReceipt> {
    throw new CommandApiError('SIMULATED_MODE', 409, 'commands are unavailable in fixture mode');
  }

  async setHumanInterventionState(_options: SetHumanInterventionStateOptions): Promise<HumanInterventionCommandReceipt> {
    throw new CommandApiError('SIMULATED_MODE', 409, 'human intervention commands are unavailable in fixture mode');
  }

  subscribeToActivityEvents(options: ActivityStreamOptions, _listener: ActivityEventListener, onStatus?: ActivityStreamStatusListener): () => void {
    assertCompanyScope(options.companyId);
    if (options.companyId !== FIXTURE_COMPANY_ID) {
      throw new Error('failed to subscribe to fixture activity: company scope not found');
    }
    if (options.cursor !== this.overview.meta.snapshotCursor) {
      throw new Error('failed to subscribe to fixture activity: snapshot cursor is not recognized');
    }
    onStatus?.('closed');
    return () => undefined;
  }
}

// pattern: Imperative Shell

import type {MemoryCorrectionCommandReceiptView} from '../domain/workbench';
import type {MissionInputCommandReceipt, MissionInputView, TaskInputManifestView} from '../domain/mission-input';
import type {CompanyToolCallBudgetChangeReceiptView, CompanyToolCallBudgetView, CompanyToolCallClosingReserveReceiptView} from '../domain/workbench';
import type {DailyRoutineCommandReceipt, MemoryCorrectionQueueView, MemoryTaskRevalidationPreviewView, MemoryTaskRevalidationReceipt, MemoryTaskStatusView, MissionToolCallBudgetChangeReceiptView, MissionToolCallBudgetListView, MissionToolCallClosingReserveReceiptView, ProblemToolCallAllocationReceiptView, ProblemToolCallBudgetListView, ProblemToolCallClosingReserveReceiptView, TaskToolCallAllocationReceiptView, TaskToolCallIncompleteClosureReceiptView} from '../domain/workbench';
import type {ResearchSimulationRunView} from '../domain/workbench';
import type {DomainContentFeedbackCategoryView, DomainContentFeedbackView, DomainContentCorrectionView, DomainContentPublicationView} from '../domain/workbench';
import type {AcceptanceContract, ActivityEvent, ActivityView, ArtifactDeliveryManifestResponse, ArtifactDetailView, CapabilityCatalogView, CodexModelCatalogView, CollaborationItem, CompanyCommandReceipt, CompanyFeedbackView, CompanyOverviewView, CompanySummaryView, CrossBackendHandoverView, DailyRoutineView, DataMode, DomainContentDraftView, DomainContentReviewView, DomainContentReviewSubmissionView, DomainContentSampleEvidenceView, DomainContentSourceEventView, DomainContentSourceStateView, DomainEvidenceArtifactPreviewManifestView, DomainEvidenceArtifactPreviewView, DomainEvidenceAreaAssessmentView, DomainEvidenceAreaView, DomainEvidenceItemView, DomainEvidenceLedgerView, DomainEvidencePreviewAttestationView, DomainEvidenceRecordView, DomainEvidenceReviewOutcomeView, DomainEvidenceReviewRecordView, DomainEvidenceSubstantiveAssessmentRecordView, DomainProfileQualificationRecordView, DurableDeliveryManifestCompletionReceipt, DurableDeliveryManifestInvalidationReceipt, DurableDeliveryResponse, DurableUserDispositionCommandReceipt, EmployeeDraft, EnvironmentExecutorQualificationReceipt, EnvironmentPolicyDecisionReceipt, EnvironmentPreparationRunView, GitHubCredentialReceipt, GitHubFeedbackBacklogStatus, GitHubFeedbackBacklogStatusReceipt, GitHubFeedbackCollectionPolicyReceipt, GitHubFeedbackPollReceipt, GitHubFeedbackProbeReceipt, GitHubFeedbackSourceCommandReceipt, HumanInterventionCommandReceipt, HumanInterventionState, JobRunCommandReceipt, JobRunLogArtifactView, JobRunView, MissionChangeRequestView, MissionCommandReceipt, NotificationsView, OperatorInstructionReceipt, OperatorInstructionView, OperationsView, ProjectEnvironmentRevisionView, ResearchSourceView, RuntimeSettingsView, ServiceBrowserSessionView, StdioMCPPackageRevisionView, TaskTakeoverLeaseView, UserDispositionDecision, WorkspaceView} from '../domain/workbench';
import type {TaskTakeoverWorkspaceFileView, TaskTakeoverWorkspaceManifestView} from '../domain/workbench';

export type CompanyDraftOptions = Readonly<{
  id: string;
  name: string;
  workspaceRoot: string;
  roster: ReadonlyArray<EmployeeDraft>;
  requestId: string;
  teamCoverageConfirmationSha256?: string;
}>;

export type UpdateCompanyOptions = Readonly<{
  companyId: string;
  name: string;
  workspaceRoot: string;
  roster: ReadonlyArray<EmployeeDraft>;
  requestId: string;
  teamCoverageConfirmationSha256?: string;
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
  requestId: string;
}>;

export type OperatorInstructionQueryOptions = Readonly<{
  companyId: string;
}>;

export type CreateOperatorInstructionOptions = Readonly<{
  companyId: string;
  missionId: string | null;
  taskId: string | null;
  employeeId: string | null;
  content: string;
  requestId: string;
}>;

export type ConfigureNotificationRouteOptions = Readonly<{
  companyId: string;
  adapter: 'local' | 'webhook' | 'qq_official';
  destination: string;
  safetyAlias?: string;
  credentialRef?: string;
  enabled: boolean;
  requestId: string;
}>;

export type TestNotificationOptions = Readonly<{
  companyId: string;
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

export type MissionChangeRequestQueryOptions = Readonly<{
  companyId: string;
  missionId: string;
}>;

export type CreateMissionChangeRequestOptions = Readonly<{
  companyId: string;
  missionId: string;
  requestId: string;
  changeSummary: string;
  proposedTitle: string;
  proposedGoal: string;
  proposedAcceptanceContract: AcceptanceContract | null;
  blockPreviousResults: boolean;
}>;

export type MissionChangeRequestCommandOptions = Readonly<{
  companyId: string;
  missionId: string;
  changeRequestId: string;
  requestId: string;
  impactSha256?: string;
  assessmentSha256?: string;
}>;

export type TaskTakeoverLeaseQueryOptions = Readonly<{companyId: string; missionId: string}>;
export type TaskTakeoverWorkspaceManifestQueryOptions = Readonly<{companyId: string; missionId: string; leaseId: string}>;
export type TaskTakeoverWorkspaceFileQueryOptions = Readonly<{companyId: string; missionId: string; leaseId: string; relativePath: string; manifestSha256: string}>;
export type CreateTaskTakeoverLeaseOptions = Readonly<{companyId: string; missionId: string; taskId: string; requestId: string}>;
export type TaskTakeoverSnapshotOptions = Readonly<{
  companyId: string;
  missionId: string;
  leaseId: string;
  requestId: string;
  baseWorkspaceDigest: string;
  baseWorkspaceRevision: number;
  content: string;
  humanEffortSeconds: number;
}>;
export type TaskTakeoverDirectorySnapshotOptions = Readonly<{
  companyId: string;
  missionId: string;
  leaseId: string;
  requestId: string;
  baseWorkspaceTreeSha256: string;
  files: ReadonlyArray<Readonly<{relativePath: string; content: string}>>;
  humanEffortSeconds: number;
}>;
export type ReleaseTaskTakeoverLeaseOptions = Readonly<{companyId: string; missionId: string; leaseId: string; requestId: string}>;

export type RecordDomainEvidenceOptions = Readonly<{
  companyId: string;
  profileId: string;
  profileRevision: string;
  evidence: ReadonlyArray<DomainEvidenceItemView>;
  requestId: string;
}>;

export type SetContentSourceAuthorizationOptions = Readonly<{
  companyId: string;
  sourceInputId: string;
  sourceInputRevision: string;
  sourceSha256: string;
  state: DomainContentSourceStateView;
  rationale: string;
  requestId: string;
}>;

export type SetResearchSourceAuthorizationOptions = Readonly<{
  companyId: string;
  sourceId: string;
  missionId: string;
  origin: string;
  searchEndpoint: string;
  searchCredentialRef: string;
  searchRankingRevision: string;
  profileRevision: string;
  identitySha256: string;
  dataSha256: string;
  state: 'authorized' | 'revoked';
  rationale: string;
  requestId: string;
}>;
export type ResearchSourcesQueryOptions = Readonly<{companyId: string; missionId: string}>;

export type RegisterContentDraftOptions = Readonly<{
  companyId: string;
  draftInputId: string;
  draftInputRevision: string;
  writerEmployeeId: string;
  criticalClaims: ReadonlyArray<string>;
  constraintsPassed: boolean;
  requestId: string;
}>;

export type RecordContentReviewOptions = Readonly<{
  companyId: string;
  draftInputId: string;
  draftRevision: string;
  correctionId: string;
  review: DomainContentReviewSubmissionView;
  sample: DomainContentSampleEvidenceView;
  requestId: string;
}>;

export type SimulateContentPublicationOptions = Readonly<{companyId: string; reviewId: string; requestId: string}>;
export type RecordContentCorrectionOptions = Readonly<{
  companyId: string;
  publicationId: string;
  correctionDraftInputId: string;
  correctionDraftRevision: string;
  rationale: string;
  requestId: string;
}>;
export type RecordContentFeedbackOptions = Readonly<{
  companyId: string;
  publicationId: string;
  category: DomainContentFeedbackCategoryView;
  note: string;
  requestId: string;
}>;

export type RecordDurableUserDispositionOptions = Readonly<{
  companyId: string;
  artifactId: string;
  requestId: string;
  expectedManifestRevision: string;
  expectedDispositionRevision: string;
  expectedDeliveryId: string;
  state: UserDispositionDecision;
  reason: string;
}>;

export type DeliveryManifestEvidenceOptions = Readonly<{reference: string; digest: string; detail: string}>;
export type CompleteDurableDeliveryManifestOptions = Readonly<{
  companyId: string;
  artifactId: string;
  requestId: string;
  expectedManifestRevision: string;
  sourceInputs: DeliveryManifestEvidenceOptions;
  environmentBuild: DeliveryManifestEvidenceOptions;
  runInstructions: DeliveryManifestEvidenceOptions;
  limitations: DeliveryManifestEvidenceOptions;
  licenseSource: DeliveryManifestEvidenceOptions;
}>;

export type InvalidateDurableDeliveryManifestOptions = Readonly<{
  companyId: string;
  artifactId: string;
  expectedManifestRevision: string;
  state: 'invalidated' | 'withdrawn';
  reason: string;
  requestId: string;
}>;

export type ImportReadOnlySkillPackageOptions = Readonly<{
  companyId: string;
  revision: string;
  bundleFile: File;
  requestId: string;
}>;

export const MAX_SKILL_PACKAGE_BYTES = 8 << 20;

export type ImportStdioMCPPackageOptions = Readonly<{
  companyId: string;
  serverId: string | null;
  revision: string;
  bundleFile: File;
  requestId: string;
}>;

export type ObserveStdioMCPRuntimeOptions = Readonly<{
  companyId: string;
  serverId: string;
  packageRevisionId: string;
  capabilityQualificationId: string;
  requestId: string;
}>;

export type ObserveStreamableHTTPMCPRuntimeOptions = Readonly<{
  companyId: string;
  capabilityId: string;
  capabilityQualificationId: string;
  requestId: string;
}>;

export const MAX_STDIO_MCP_PACKAGE_BYTES = 8 << 20;

export type RecordDomainEvidenceReviewOptions = Readonly<{
  companyId: string;
  recordId: string;
  outcome: DomainEvidenceReviewOutcomeView;
  reviewerEmployeeId: string;
  rationale: string;
  previewedEvidence: ReadonlyArray<DomainEvidencePreviewAttestationView>;
  requestId: string;
}>;

export type RecordDomainEvidenceSubstantiveAssessmentOptions = Readonly<{
  companyId: string;
  recordId: string;
  evidenceDigest: string;
  reviewerEmployeeId: string;
  areaAssessments: ReadonlyArray<DomainEvidenceAreaAssessmentView>;
  previewedEvidence: ReadonlyArray<DomainEvidencePreviewAttestationView>;
  requestId: string;
}>;

export type RecordDomainProfileQualificationOptions = Readonly<{
  companyId: string;
  profileId: string;
  profileRevision: string;
  decision: 'qualified' | 'revoked';
  evidenceInputId?: string;
  evidenceInputRevision?: string;
  rationale: string;
  requestId: string;
}>;

export type RunResearchSimulationOptions = Readonly<{
  companyId: string;
  datasetInputId: string;
  datasetInputRevision: string;
  methodInputId: string;
  methodInputRevision: string;
  seed: string;
  controlDefinition: string;
  riskBudgetUnits: number;
  requestId: string;
}>;

export type ListDomainEvidenceArtifactPreviewEntriesOptions = Readonly<{
  companyId: string;
  recordId: string;
  area: DomainEvidenceAreaView;
  inputId: string;
  inputRevision: number;
  sourceDigest: string;
}>;

export type GetDomainEvidenceArtifactPreviewOptions = Readonly<ListDomainEvidenceArtifactPreviewEntriesOptions & {relativePath: string; expectedContentSHA256: string}>;

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

export const MAX_MISSION_INPUT_BYTES = 8 * 1024 * 1024;
export const MAX_MISSION_DIRECTORY_BYTES = 7 * 1024 * 1024;
export const MAX_MISSION_DIRECTORY_FILES = 250;

export type MissionInputQueryOptions = Readonly<{
  companyId: string;
  missionId: string;
}>;

export type DailyRoutineQueryOptions = Readonly<{
  companyId: string;
  missionId: string;
}>;

export type MemoryTaskStatusQueryOptions = Readonly<{companyId: string; taskId: string}>;
export type MemoryCorrectionQueueQueryOptions = Readonly<{companyId: string}>;
export type ProposeMemoryCorrectionOptions = Readonly<{
  companyId: string;
  workerSessionId: string;
  correctionId: string;
  recordId: string;
  baseRevision: number;
  content: string;
  source: Readonly<{kind: 'mission_input' | 'artifact'; id: string; revision: number; sha256: string}>;
  reason: string;
  requestId: string;
}>;
export type ReviewMemoryCorrectionOptions = Readonly<{
  companyId: string;
  correctionId: string;
  workerSessionId: string;
  decision: 'approved' | 'rejected';
  reason: string;
  requestId: string;
}>;
export type ProblemToolCallBudgetQueryOptions = Readonly<{companyId: string}>;
export type MissionToolCallBudgetQueryOptions = Readonly<{companyId: string}>;
export type ChangeCompanyToolCallBudgetOptions = Readonly<{
  companyId: string;
  expectedToolCallLimit: number | null;
  expectedRevision: number;
  resultingToolCallLimit: number;
  reason: string;
  requestId: string;
}>;
export type SetCompanyToolCallClosingReserveOptions = Readonly<{
  companyId: string;
  reservedToolCalls: number;
  expectedCompanyToolCallLimit: number | null;
  expectedCompanyBudgetRevision: number;
  expectedReserveRevision: number;
  reason: string;
  requestId: string;
}>;
export type ChangeMissionToolCallBudgetOptions = Readonly<{
  companyId: string;
  missionId: string;
  expectedToolCallLimit: number | null;
  expectedRevision: number;
  resultingToolCallLimit: number;
  reason: string;
  requestId: string;
}>;
export type SetMissionToolCallClosingReserveOptions = Readonly<{
  companyId: string;
  missionId: string;
  reservedToolCalls: number;
  expectedMissionToolCallLimit: number | null;
  expectedMissionBudgetRevision: number;
  expectedReserveRevision: number;
  reason: string;
  requestId: string;
}>;
export type AllocateProblemToolCallsOptions = Readonly<{
  companyId: string;
  problemKey: string;
  additionalToolCalls: number;
  expectedToolCallLimit: number;
  expectedRevision: number;
  reason: string;
  requestId: string;
}>;
export type SetProblemToolCallClosingReserveOptions = Readonly<{
  companyId: string;
  problemKey: string;
  reservedToolCalls: number;
  expectedToolCallLimit: number;
  expectedBudgetRevision: number;
  expectedReserveRevision: number;
  reason: string;
  requestId: string;
}>;
export type AllocateTaskToolCallsOptions = Readonly<{
  companyId: string;
  problemKey: string;
  taskId: string;
  additionalToolCalls: number;
  expectedToolCallLimit: number;
  expectedTaskRevision: number;
  expectedProblemRevision: number;
  expectedReserveRevision: number;
  reason: string;
  requestId: string;
}>;
export type CloseTaskToolBudgetIncompleteOptions = Readonly<{
  companyId: string;
  problemKey: string;
  taskId: string;
  expectedTaskToolCallLimit: number | null;
  expectedTaskToolCallsUsed: number;
  expectedTaskRevision: number;
  expectedProblemToolCallLimit: number | null;
  expectedProblemToolCallsUsed: number;
  expectedProblemRevision: number;
  expectedClosingReserveToolCalls: number;
  expectedClosingReserveRemaining: number;
  expectedReserveRevision: number;
  reason: string;
  requestId: string;
}>;
export type MemoryTaskRevalidationPreviewOptions = Readonly<{companyId: string; taskId: string; dependencyId: string; correctionId: string}>;
export type RevalidateMemoryTaskOptions = Readonly<{
  companyId: string;
  taskId: string;
  dependencyId: string;
  correctionId: string;
  contextSha256: string;
  reason: string;
  requestId: string;
}>;

export type CreateDailyRoutineOptions = Readonly<{
  companyId: string;
  missionId: string;
  routineId: string;
  employeeId: string;
  taskInstruction: string;
  timezone: string;
  localTime: string;
  nextLogicalDay: string;
  catchUpPolicy: 'skip' | 'coalesce_latest' | 'catch_up';
  maxCatchUp: number;
  requestId: string;
}>;

export type SetDailyRoutineTaskInstructionOptions = Readonly<{
  companyId: string;
  missionId: string;
  routineId: string;
  taskInstruction: string;
  requestId: string;
}>;

export type UploadMissionInputOptions = Readonly<{
  companyId: string;
  missionId: string;
  inputId: string | null;
  requestId: string;
  file: File;
}>;

export type StoreGitHubCredentialOptions = Readonly<{
  companyId: string;
  token: string;
  requestId: string;
}>;

export type DeleteGitHubCredentialOptions = Readonly<{
  companyId: string;
  requestId: string;
}>;

export type RegisterGitHubFeedbackSourceOptions = Readonly<{
  companyId: string;
  sourceId?: string;
  repositoryId: string;
  owner: string;
  name: string;
  requestId: string;
}>;

export type ProbeGitHubFeedbackSourceOptions = Readonly<{
  companyId: string;
  sourceId: string;
  rationale: string;
  requestId: string;
}>;

export type DecideGitHubFeedbackSourceOptions = Readonly<{
  companyId: string;
  sourceId: string;
  decision: 'approved' | 'paused' | 'revoked';
  rationale: string;
  requestId: string;
}>;

export type PollGitHubFeedbackSourceOptions = Readonly<{
  companyId: string;
  sourceId: string;
  requestId: string;
}>;

export type SetGitHubFeedbackCollectionPolicyOptions = Readonly<{
  companyId: string;
  sourceId: string;
  enabled: boolean;
  intervalSeconds: number;
  rationale: string;
  requestId: string;
}>;

export type SetGitHubFeedbackBacklogStatusOptions = Readonly<{
  companyId: string;
  sourceId: string;
  providerItemId: string;
  revisionSha256: string;
  status: Exclude<GitHubFeedbackBacklogStatus, 'needs_review'>;
  rationale: string;
  requestId: string;
}>;

export type TaskInputManifestQueryOptions = Readonly<{
  companyId: string;
  taskId: string;
}>;

export type TaskJobRunsQueryOptions = Readonly<{
  companyId: string;
  taskId: string;
}>;

export type TaskCrossBackendHandoversQueryOptions = Readonly<{
  companyId: string;
  taskId: string;
}>;

export type CreateTaskEnvironmentHandoverOptions = Readonly<{
  companyId: string;
  taskId: string;
  sourceJobId: string;
  targetEnvironmentRevisionId: string;
  requestId: string;
}>;

type StartTaskJobRunBaseOptions = Readonly<{
  companyId: string;
  taskId: string;
  sessionId?: string;
  environmentRevisionId: string;
  handoverId?: string;
  requestId: string;
}>;

export type StartTaskJobRunOptions = StartTaskJobRunBaseOptions & (
  | Readonly<{kind?: 'batch'; scriptPath: string; args: ReadonlyArray<string>}>
  | Readonly<{kind: 'service'; serviceId: string}>
);

export type StopTaskJobRunOptions = Readonly<{
  companyId: string;
  jobId: string;
  requestId: string;
}>;

export type TaskJobLogsQueryOptions = Readonly<{
  companyId: string;
  jobId: string;
}>;

export type CreateProjectJobBrowserSessionOptions = Readonly<{
  companyId: string;
  jobId: string;
  requestId: string;
}>;

export type EnvironmentPolicyDecisionOptions = Readonly<{
  companyId: string;
  revisionId: string;
  decision: 'approved' | 'revoked';
  rationale: string;
  requestId: string;
}>;

export type EnvironmentExecutorQualificationOptions = Readonly<{
  companyId: string;
  revisionId: string;
  decision: 'qualified' | 'revoked';
  evidenceInputId: string;
  evidenceInputRevision: string;
  qualifiedUntil?: string;
  rationale: string;
  requestId: string;
}>;

export type EnsureEnvironmentOptions = Readonly<{
  companyId: string;
  revisionId: string;
  requestId: string;
}>;

export type MissionDirectoryFile = Readonly<{
  relativePath: string;
  file: File;
}>;

export type UploadMissionDirectoryInputOptions = Readonly<{
  companyId: string;
  missionId: string;
  inputId: string | null;
  requestId: string;
  files: ReadonlyArray<MissionDirectoryFile>;
}>;

export type CreateMissionOptions = Readonly<{
  companyId: string;
  title: string;
  goal: string;
  acceptanceContract: AcceptanceContract | null;
  protocolToolCallLimit?: number;
  requestId: string;
}>;

export type MissionCommandOptions = Readonly<{
  companyId: string;
  missionId: string;
  requestId: string;
}>;

export type MissionCloseoutOptions = Readonly<{
  companyId: string;
  missionId: string;
  outcome: 'succeeded' | 'ended_not_met';
  rationale: string;
  acceptanceArtifactIds: ReadonlyArray<string>;
  requestId: string;
}>;

export type QualifyCapabilityOptions = Readonly<{companyId: string; capabilityKind: 'skill' | 'mcp'; capabilityId: string; requestId: string}>;
export type DecideCapabilityOptions = Readonly<{companyId: string; capabilityKind: 'skill' | 'mcp'; capabilityId: string; qualificationId: string; decision: 'approved' | 'revoked'; rationale: string; requestId: string}>;
export type ReviewCapabilityRevocationOptions = Readonly<{companyId: string; revocationId: string; rationale: string; requestId: string}>;
export type ApproveStdioMCPRuntimeQualificationOptions = Readonly<{companyId: string; runtimeQualificationId: string; rationale: string; requestId: string}>;
export type BindEmployeeCapabilityOptions = Readonly<{companyId: string; employeeId: string; capabilityKind: 'skill' | 'mcp'; capabilityId: string; qualificationId: string; reason: string; requestId: string}>;

export type SetHumanInterventionStateOptions = Readonly<{
  companyId: string;
  interventionId: string;
  state: HumanInterventionState;
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
  listMissionChangeRequests(options: MissionChangeRequestQueryOptions): Promise<ReadonlyArray<MissionChangeRequestView>>;
  createMissionChangeRequest(options: CreateMissionChangeRequestOptions): Promise<MissionChangeRequestView>;
  considerMissionChangeRequest(options: MissionChangeRequestCommandOptions): Promise<MissionChangeRequestView>;
  declineMissionChangeRequest(options: MissionChangeRequestCommandOptions): Promise<MissionChangeRequestView>;
  applyMissionChangeRequest(options: MissionChangeRequestCommandOptions): Promise<MissionChangeRequestView>;
  listTaskTakeoverLeases(options: TaskTakeoverLeaseQueryOptions): Promise<ReadonlyArray<TaskTakeoverLeaseView>>;
  getTaskTakeoverWorkspaceManifest(options: TaskTakeoverWorkspaceManifestQueryOptions): Promise<TaskTakeoverWorkspaceManifestView>;
  readTaskTakeoverWorkspaceFile(options: TaskTakeoverWorkspaceFileQueryOptions): Promise<TaskTakeoverWorkspaceFileView>;
  createTaskTakeoverLease(options: CreateTaskTakeoverLeaseOptions): Promise<TaskTakeoverLeaseView>;
  submitTaskTakeoverSnapshot(options: TaskTakeoverSnapshotOptions): Promise<TaskTakeoverLeaseView>;
  submitTaskTakeoverDirectorySnapshot(options: TaskTakeoverDirectorySnapshotOptions): Promise<TaskTakeoverLeaseView>;
  releaseTaskTakeoverLease(options: ReleaseTaskTakeoverLeaseOptions): Promise<TaskTakeoverLeaseView>;
  listCollaboration(options: CompanyScopeOptions): Promise<ReadonlyArray<CollaborationItem>>;
  getWorkspace(options: Readonly<{companyId: string; taskId: string}>): Promise<WorkspaceView>;
  getArtifact(options: Readonly<{companyId: string; artifactId: string}>): Promise<ArtifactDetailView>;
  getDurableDelivery(options: Readonly<{companyId: string; artifactId: string}>): Promise<DurableDeliveryResponse>;
  completeDurableDeliveryManifest?(options: CompleteDurableDeliveryManifestOptions): Promise<DurableDeliveryManifestCompletionReceipt>;
  invalidateDurableDeliveryManifest?(options: InvalidateDurableDeliveryManifestOptions): Promise<DurableDeliveryManifestInvalidationReceipt>;
  recordDurableUserDisposition?(options: RecordDurableUserDispositionOptions): Promise<DurableUserDispositionCommandReceipt>;
  getArtifactDeliveryManifest(options: Readonly<{companyId: string; artifactId: string}>): Promise<ArtifactDeliveryManifestResponse>;
  downloadArtifactPackage(options: Readonly<{companyId: string; artifactId: string}>): Promise<Blob>;
  getOperations(options: CompanyScopeOptions): Promise<OperationsView>;
  listNotifications(options: CompanyScopeOptions): Promise<NotificationsView>;
  getCompanyFeedback(options: CompanyScopeOptions): Promise<CompanyFeedbackView>;
  storeGitHubFeedbackCredential(options: StoreGitHubCredentialOptions): Promise<GitHubCredentialReceipt>;
  deleteGitHubFeedbackCredential(options: DeleteGitHubCredentialOptions): Promise<GitHubCredentialReceipt>;
  registerGitHubFeedbackSource(options: RegisterGitHubFeedbackSourceOptions): Promise<GitHubFeedbackSourceCommandReceipt>;
  probeGitHubFeedbackSource(options: ProbeGitHubFeedbackSourceOptions): Promise<GitHubFeedbackProbeReceipt>;
  decideGitHubFeedbackSource(options: DecideGitHubFeedbackSourceOptions): Promise<GitHubFeedbackSourceCommandReceipt>;
  pollGitHubFeedbackSource(options: PollGitHubFeedbackSourceOptions): Promise<GitHubFeedbackPollReceipt>;
  setGitHubFeedbackCollectionPolicy(options: SetGitHubFeedbackCollectionPolicyOptions): Promise<GitHubFeedbackCollectionPolicyReceipt>;
  setGitHubFeedbackBacklogStatus(options: SetGitHubFeedbackBacklogStatusOptions): Promise<GitHubFeedbackBacklogStatusReceipt>;
  listCapabilityCatalog(options: CompanyScopeOptions): Promise<CapabilityCatalogView>;
  listDomainEvidence(options: CompanyScopeOptions): Promise<DomainEvidenceLedgerView>;
  setContentSourceAuthorization(options: SetContentSourceAuthorizationOptions): Promise<DomainContentSourceEventView>;
  setResearchSourceAuthorization(options: SetResearchSourceAuthorizationOptions): Promise<ResearchSourceView>;
  listResearchSources(options: ResearchSourcesQueryOptions): Promise<ReadonlyArray<ResearchSourceView>>;
  registerContentDraft(options: RegisterContentDraftOptions): Promise<DomainContentDraftView>;
  recordContentReview(options: RecordContentReviewOptions): Promise<DomainContentReviewView>;
  simulateContentPublication(options: SimulateContentPublicationOptions): Promise<DomainContentPublicationView>;
  recordContentCorrection(options: RecordContentCorrectionOptions): Promise<DomainContentCorrectionView>;
  recordContentFeedback(options: RecordContentFeedbackOptions): Promise<DomainContentFeedbackView>;
  runResearchSimulation(options: RunResearchSimulationOptions): Promise<ResearchSimulationRunView>;
  recordDomainEvidence(options: RecordDomainEvidenceOptions): Promise<DomainEvidenceRecordView>;
  recordDomainEvidenceReview(options: RecordDomainEvidenceReviewOptions): Promise<DomainEvidenceReviewRecordView>;
  recordDomainEvidenceSubstantiveAssessment(options: RecordDomainEvidenceSubstantiveAssessmentOptions): Promise<DomainEvidenceSubstantiveAssessmentRecordView>;
  recordDomainProfileQualification(options: RecordDomainProfileQualificationOptions): Promise<DomainProfileQualificationRecordView>;
  recordDomainEvidenceSubstantiveAssessment(options: RecordDomainEvidenceSubstantiveAssessmentOptions): Promise<DomainEvidenceSubstantiveAssessmentRecordView>;
  listDomainEvidenceArtifactPreviewEntries(options: ListDomainEvidenceArtifactPreviewEntriesOptions): Promise<DomainEvidenceArtifactPreviewManifestView>;
  getDomainEvidenceArtifactPreview(options: GetDomainEvidenceArtifactPreviewOptions): Promise<DomainEvidenceArtifactPreviewView>;
  importSkill(options: ImportSkillOptions): Promise<unknown>;
  importReadOnlySkillPackage(options: ImportReadOnlySkillPackageOptions): Promise<unknown>;
  importStdioMCPPackage(options: ImportStdioMCPPackageOptions): Promise<StdioMCPPackageRevisionView>;
  observeStdioMCPRuntime(options: ObserveStdioMCPRuntimeOptions): Promise<unknown>;
  observeStreamableHTTPMCPRuntime(options: ObserveStreamableHTTPMCPRuntimeOptions): Promise<unknown>;
  registerMCP(options: RegisterMCPOptions): Promise<unknown>;
  qualifyCapability(options: QualifyCapabilityOptions): Promise<unknown>;
  decideCapability(options: DecideCapabilityOptions): Promise<unknown>;
  reviewIncompleteCapabilityRevocation(options: ReviewCapabilityRevocationOptions): Promise<unknown>;
  approveStdioMCPRuntimeQualification(options: ApproveStdioMCPRuntimeQualificationOptions): Promise<unknown>;
  bindEmployeeCapability(options: BindEmployeeCapabilityOptions): Promise<unknown>;
  revokeEmployeeCapability(options: BindEmployeeCapabilityOptions): Promise<unknown>;
  configureNotificationRoute(options: ConfigureNotificationRouteOptions): Promise<CompanyCommandReceipt>;
  testNotification(options: TestNotificationOptions): Promise<Readonly<{intentId: string; deliveryId: string; adapter: string; state: string; requestId: string; accepted: true; acceptedAt: string}>>;
  getCompanyOverview(options: CompanyScopeOptions): Promise<CompanyOverviewView>;
  listDailyRoutines(options: DailyRoutineQueryOptions): Promise<ReadonlyArray<DailyRoutineView>>;
  getMemoryTaskStatus(options: MemoryTaskStatusQueryOptions): Promise<MemoryTaskStatusView>;
  listMemoryCorrections(options: MemoryCorrectionQueueQueryOptions): Promise<MemoryCorrectionQueueView>;
  proposeMemoryCorrection(options: ProposeMemoryCorrectionOptions): Promise<MemoryCorrectionCommandReceiptView>;
  reviewMemoryCorrection(options: ReviewMemoryCorrectionOptions): Promise<MemoryCorrectionCommandReceiptView>;
  getCompanyToolCallBudget(options: CompanyScopeOptions): Promise<CompanyToolCallBudgetView>;
  changeCompanyToolCallBudget(options: ChangeCompanyToolCallBudgetOptions): Promise<CompanyToolCallBudgetChangeReceiptView>;
  setCompanyToolCallClosingReserve(options: SetCompanyToolCallClosingReserveOptions): Promise<CompanyToolCallClosingReserveReceiptView>;
  listProblemToolCallBudgets(options: ProblemToolCallBudgetQueryOptions): Promise<ProblemToolCallBudgetListView>;
  listMissionToolCallBudgets(options: MissionToolCallBudgetQueryOptions): Promise<MissionToolCallBudgetListView>;
  changeMissionToolCallBudget(options: ChangeMissionToolCallBudgetOptions): Promise<MissionToolCallBudgetChangeReceiptView>;
  setMissionToolCallClosingReserve(options: SetMissionToolCallClosingReserveOptions): Promise<MissionToolCallClosingReserveReceiptView>;
  allocateProblemToolCalls(options: AllocateProblemToolCallsOptions): Promise<ProblemToolCallAllocationReceiptView>;
  allocateTaskToolCalls(options: AllocateTaskToolCallsOptions): Promise<TaskToolCallAllocationReceiptView>;
  closeTaskToolBudgetIncomplete(options: CloseTaskToolBudgetIncompleteOptions): Promise<TaskToolCallIncompleteClosureReceiptView>;
  setProblemToolCallClosingReserve(options: SetProblemToolCallClosingReserveOptions): Promise<ProblemToolCallClosingReserveReceiptView>;
  getMemoryTaskRevalidationPreview(options: MemoryTaskRevalidationPreviewOptions): Promise<MemoryTaskRevalidationPreviewView>;
  revalidateMemoryTask(options: RevalidateMemoryTaskOptions): Promise<MemoryTaskRevalidationReceipt>;
  listActivityEvents(options: ActivityQueryOptions): Promise<ActivityView>;
  listMissionInputs(options: MissionInputQueryOptions): Promise<ReadonlyArray<MissionInputView>>;
  getTaskInputManifest(options: TaskInputManifestQueryOptions): Promise<TaskInputManifestView>;
  listProjectEnvironments(options: CompanyScopeOptions): Promise<ReadonlyArray<ProjectEnvironmentRevisionView>>;
  decideProjectEnvironmentPolicy(options: EnvironmentPolicyDecisionOptions): Promise<EnvironmentPolicyDecisionReceipt>;
  decideProjectEnvironmentExecutorQualification(options: EnvironmentExecutorQualificationOptions): Promise<EnvironmentExecutorQualificationReceipt>;
  ensureProjectEnvironment(options: EnsureEnvironmentOptions): Promise<EnvironmentPreparationRunView>;
  listTaskJobRuns(options: TaskJobRunsQueryOptions): Promise<ReadonlyArray<JobRunView>>;
  listTaskCrossBackendHandovers(options: TaskCrossBackendHandoversQueryOptions): Promise<ReadonlyArray<CrossBackendHandoverView>>;
  createTaskEnvironmentHandover(options: CreateTaskEnvironmentHandoverOptions): Promise<CrossBackendHandoverView>;
  startTaskJobRun(options: StartTaskJobRunOptions): Promise<JobRunCommandReceipt>;
  stopTaskJobRun(options: StopTaskJobRunOptions): Promise<JobRunCommandReceipt>;
  getTaskJobLogs(options: TaskJobLogsQueryOptions): Promise<JobRunLogArtifactView>;
  createProjectJobBrowserSession(options: CreateProjectJobBrowserSessionOptions): Promise<ServiceBrowserSessionView>;
  uploadMissionInput(options: UploadMissionInputOptions): Promise<MissionInputCommandReceipt>;
  uploadMissionDirectoryInput(options: UploadMissionDirectoryInputOptions): Promise<MissionInputCommandReceipt>;
  createMission(options: CreateMissionOptions): Promise<MissionCommandReceipt>;
  createDailyRoutine(options: CreateDailyRoutineOptions): Promise<DailyRoutineCommandReceipt>;
  setDailyRoutineTaskInstruction(options: SetDailyRoutineTaskInstructionOptions): Promise<DailyRoutineCommandReceipt>;
  startMission(options: MissionCommandOptions): Promise<MissionCommandReceipt>;
  pauseMission(options: MissionCommandOptions): Promise<MissionCommandReceipt>;
  resumeMission(options: MissionCommandOptions): Promise<MissionCommandReceipt>;
  cancelMission(options: MissionCommandOptions): Promise<MissionCommandReceipt>;
  closeMission(options: MissionCloseoutOptions): Promise<MissionCommandReceipt>;
  setHumanInterventionState(options: SetHumanInterventionStateOptions): Promise<HumanInterventionCommandReceipt>;
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

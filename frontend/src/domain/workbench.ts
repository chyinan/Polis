// pattern: Functional Core

export type DataMode = 'real' | 'simulated' | 'unavailable';

export type MissionCommandType = 'mission.create' | 'mission.start' | 'mission.pause' | 'mission.resume' | 'mission.cancel' | 'mission.closeout';

export type MissionCommandResultState = 'draft' | 'active' | 'paused' | 'closing' | 'succeeded' | 'ended_not_met' | 'cancelled';

export type CompanyState = 'active' | 'archived';

export type EmployeeDraft = Readonly<{
  id: string;
  displayName: string;
  role: string;
  modelProfile: string;
}>;

export type FixedTeamCoverageAssignmentView = Readonly<{
  task_type: string;
  owner: string;
  eligible_independent_checkers: ReadonlyArray<string>;
  acceptance_path: string;
  qualification: 'unverified';
}>;

export type FixedTeamCoverageDraftView = Readonly<{
  document_version: string;
  design_kind: 'fixed_team_coverage_draft';
  execution_enabled: false;
  role_changes_at_runtime: false;
  template_requires_human_confirmation: true;
  employee_ids: ReadonlyArray<string>;
  coverage: ReadonlyArray<FixedTeamCoverageAssignmentView>;
  missing_path: string;
  checker_policy: string;
  trusted_baseline_mutable_by_workers: false;
  guarantees_semantic_independence: false;
}>;

export type FixedTeamCoverageRoleRevisionView = Readonly<{
  revision_sha256: string;
  owner_decision: 'installation_owner_confirmed_fixed_team_mapping';
  qualification: 'unverified';
  contract: FixedTeamCoverageDraftView;
}>;

export type CompanySummaryView = Readonly<{
  id: string;
  name: string;
  workspaceRoot: string;
  state: CompanyState;
  roster: ReadonlyArray<EmployeeDraft>;
  teamCoverageConfirmed?: boolean;
  teamCoverageConfirmationSha256?: string;
  teamCoverageConfirmedAt?: string;
  teamCoverageRoleRevision?: FixedTeamCoverageRoleRevisionView;
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

export type OperatorInstructionState = 'pending' | 'applied' | 'rejected' | 'needs_clarification';
export type OperatorInstructionOutcome = 'applied' | 'rejected' | 'needs_clarification';
export type OperatorInstructionResponseView = Readonly<{
  employeeId: string;
  outcome: OperatorInstructionOutcome;
  summary: string;
  respondedAt: string;
}>;

export type OperatorInstructionView = Readonly<{
  instructionId: string;
  missionId: string | null;
  taskId: string | null;
  employeeId: string | null;
  content: string;
  state: OperatorInstructionState;
  createdAt: string;
  responseOutcome: OperatorInstructionOutcome | null;
  responseSummary: string | null;
  respondedByEmployeeId: string | null;
  respondedAt: string | null;
  responses: ReadonlyArray<OperatorInstructionResponseView>;
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

export type ArtifactDeliveryManifestView = Readonly<{
  schemaVersion: 'polis-delivery-manifest@1';
  companyId: string;
  artifactId: string;
  taskId: string;
  content: Readonly<{fileName: string; contentType: string; byteSize: string; sha256: string}>;
  state: string;
  verdict: string;
  qualification: Readonly<{checkpointId: string; validationReceiptId: string; taskValidationBindingDigest: string; workspaceDigest: string; workspaceRevision: string; runnerRevision: string}>;
  createdAt: string;
}>;

export type ArtifactDeliveryManifestResponse = Readonly<{
  manifest: ArtifactDeliveryManifestView;
  manifestSha256: string;
}>;

export type DurableDeliverySectionState = 'available' | 'unavailable' | 'missing' | 'not_requested';
export type UserDispositionState = 'not_requested' | 'awaiting_feedback' | 'accepted' | 'changes_requested';

export type DurableDeliverySectionView = Readonly<{
  key: string;
  state: DurableDeliverySectionState;
  detail: string;
}>;

export type DurableDeliveryManifestView = Readonly<{
  schemaVersion: 'polis-durable-delivery-manifest@1';
  deliveryId: string;
  revision: string;
  companyId: string;
  missionId: string;
  taskId: string;
  artifactId: string;
  state: 'assembling' | 'ready' | 'invalidated' | 'withdrawn';
  artifact: Readonly<{fileName: 'artifact.bin'; byteSize: string; sha256: string}>;
  sections: ReadonlyArray<DurableDeliverySectionView>;
  createdAt: string;
}>;

export type DurableUserDispositionView = Readonly<{
  revision: string;
  manifestRevision: string;
  state: UserDispositionState;
  actor: string;
  reason: string;
  requestId: string;
  feedbackDeadline: string;
  createdAt: string;
}>;

export type DurableDeliveryResponse = Readonly<{
  manifest: DurableDeliveryManifestView;
  manifestSha256: string;
  userDisposition: DurableUserDispositionView;
}>;

export type ProjectEnvironmentPolicyManifestView = Readonly<{
  schemaVersion: 'project-environment-policy@1';
  profileId: 'windows-node-npm@1' | 'linux-node-npm@1';
  registryHosts: ReadonlyArray<string>;
  installPolicy: 'npm ci --ignore-scripts --no-audit --no-fund' | 'npm ci --ignore-scripts --no-audit --no-fund --offline';
  lifecycleScriptsPolicy: 'ignore';
  networkPolicy: 'registry_allowlist' | 'deny_all';
  timeoutMs: number;
  outputLimitBytes: number;
  services?: ReadonlyArray<ProjectServiceDefinitionView>;
}>;

export type ServiceProbeSpecView = Readonly<{
  bindAddress: '127.0.0.1' | '::1';
  port: number;
  path: string;
  expectedStatusCode: number;
  expectedBodySha256: string;
  timeoutMs: number;
  leaseDurationMs: number;
}>;

export type ProjectServiceDefinitionView = Readonly<{
  id: string;
  scriptPath: string;
  probe: ServiceProbeSpecView;
}>;

export type ProjectEnvironmentRevisionView = Readonly<{
  companyId: string;
  revisionId: string;
  missionId: string;
  sourceInputId: string;
  sourceInputRevision: string;
  projectRootRelative: string;
  sourceBindingStatus: 'bound' | 'unverified';
  profileId: 'windows-node-npm@1' | 'linux-node-npm@1';
  sourceRevisionSha256: string;
  packageJsonSha256: string;
  lockfileSha256: string;
  policySha256: string;
  policyManifest: ProjectEnvironmentPolicyManifestView | null;
  toolchainSha256: string;
  executorFingerprintSha256: string | null;
  hostFingerprintSha256: string | null;
  isolationPolicySha256: string | null;
  executorEvidenceSha256: string | null;
  executorEvidenceInputId: string | null;
  executorEvidenceInputRevision: string | null;
  policyDecision: 'unverified' | 'source_unverified' | 'not_approved' | 'approved' | 'revoked' | 'revocation_required';
  executorQualification: 'unqualified' | 'qualified' | 'expired' | 'revoked' | 'identity_stale';
  preparationState: 'unprepared' | 'blocked_policy' | 'blocked_source_unverified' | 'blocked_unqualified' | 'accepted' | 'starting' | 'running' | 'ready' | 'failed' | 'cancelled' | 'outcome_unknown';
  preparationReason: string;
  preparationRunId: string | null;
  createdAt: string;
}>;

export type EnvironmentPolicyDecisionReceipt = Readonly<{
  id: string;
  status: 'approved' | 'revoked';
}>;

export type EnvironmentExecutorQualificationReceipt = Readonly<{
  id: string;
  status: 'qualified' | 'revoked';
}>;

export type EnvironmentPreparationRunView = Readonly<{
  companyId: string;
  runId: string;
  revisionId: string;
  state: 'blocked_policy' | 'blocked_source_unverified' | 'blocked_unqualified' | 'accepted' | 'starting' | 'running' | 'ready' | 'failed' | 'cancelled' | 'outcome_unknown';
  reasonCode: string;
  requestId: string;
  createdAt: string;
}>;

export type ServiceEndpointView = Readonly<{
  generation: string;
  bindAddress: '127.0.0.1' | '::1' | 'localhost';
  port: string;
  readiness: 'not_ready' | 'ready' | 'unhealthy' | 'revoked';
  sourceRevisionSha256: string;
  healthcheckSha256: string;
  leaseExpiresAt: string | null;
}>;

export type JobRunView = Readonly<{
  companyId: string;
  jobId: string;
  taskId: string;
  sessionId: string;
  environmentRevisionId: string;
  handoverId: string;
  kind: 'batch' | 'service' | 'controlled_input';
  serviceId: string;
  state: 'accepted' | 'starting' | 'running' | 'exited' | 'failed' | 'cancelled' | 'outcome_unknown';
  readiness: 'not_applicable' | 'not_ready' | 'ready' | 'unhealthy';
  exitCode: number | null;
  reasonCode: string;
  stdoutOffset: string;
  stderrOffset: string;
  stdoutBytes: string;
  stderrBytes: string;
  logsTruncated: boolean;
  logGap: boolean;
  logManifestSha256: string | null;
  serviceEndpoint: ServiceEndpointView | null;
  createdAt: string;
  updatedAt: string;
}>;

export type CrossBackendHandoverView = Readonly<{
  companyId: string;
  handoverId: string;
  missionId: string;
  taskId: string;
  sourceJobId: string;
  sourceSessionId: string;
  sourceRuntimeIncarnation: string;
  sourceEnvironmentRevisionId: string;
  sourceProfileId: 'windows-node-npm@1' | 'linux-node-npm@1';
  targetEnvironmentRevisionId: string;
  targetProfileId: 'windows-node-npm@1' | 'linux-node-npm@1';
  projectSourceSha256: string;
  packageJsonSha256: string;
  lockfileSha256: string;
  workspaceDigest: string;
  workspaceRevision: number;
  taskInputManifestSha256: string;
  targetPolicySha256: string;
  targetToolchainSha256: string;
  requestId: string;
  recordSha256: string;
  createdAt: string;
}>;

export type JobRunCommandReceipt = Readonly<{
  companyId: string;
  jobId: string;
  taskId: string;
  kind: 'batch' | 'service' | 'controlled_input';
  serviceId: string;
  state: 'accepted' | 'starting' | 'running' | 'exited' | 'failed' | 'cancelled' | 'outcome_unknown';
  readiness: 'not_applicable' | 'not_ready' | 'ready' | 'unhealthy';
  reasonCode: string;
}>;

export type JobRunLogArtifactView = Readonly<{
  companyId: string;
  jobId: string;
  manifestSha256: string;
  content: string;
}>;

export type ServiceBrowserSessionView = Readonly<{
  url: string;
  expiresAt: string;
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
  safetyAlias: string;
  credentialRef: string;
  status: string;
  routeRevision: string;
  qualificationStatus: string;
  qualifiedUntil: string;
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

export type FeedbackSourceView = Readonly<{
  sourceId: string;
  provider: 'github';
  repositoryId: string;
  repository: string;
  profileRevision: string;
  state: string;
  permissionStatus: string;
  coverage: string;
  coverageReason: string;
  coveredThrough: string | null;
  lastScanAt: string | null;
  collectionEnabled: boolean;
  collectionIntervalSeconds: number;
  collectionRationale: string;
  collectionNextPollAt: string | null;
  collectionLastAttempt: string;
  collectionLastReasonCode: string;
}>;

export type GitHubFeedbackCollectionPolicyReceipt = Readonly<{
  companyId: string;
  sourceId: string;
  enabled: boolean;
  intervalSeconds: number;
  rationale: string;
  updatedAt: string;
  nextPollAt: string | null;
  lastAttempt: string;
  lastReasonCode: string;
  requestId: string;
  externalEnabled: boolean;
  schedulerEnabled: boolean;
}>;

export type FeedbackCommentView = Readonly<{
  commentId: string;
  revisionSha256: string;
  body: string;
  bodySha256: string;
  bodyTruncated: boolean;
  sourceUpdatedAt: string;
  htmlUrl: string;
}>;

export type GitHubFeedbackBacklogStatus = 'open' | 'needs_review' | 'triaging' | 'waiting' | 'handled' | 'archived';

export type GitHubFeedbackBacklogStatusReceipt = Readonly<{
  companyId: string;
  sourceId: string;
  providerItemId: string;
  issueNumber: number;
  revisionSha256: string;
  status: GitHubFeedbackBacklogStatus;
  rationale: string;
  eventId: string;
  requestId: string;
  remoteState: string;
  remoteUnchanged: true;
}>;

export type FeedbackIssueView = Readonly<{
  sourceId: string;
  providerItemId: string;
  issueNumber: string;
  revisionSha256: string;
  backlogStatus: GitHubFeedbackBacklogStatus;
  backlogReason: string;
  backlogUpdatedAt: string;
  title: string;
  titleTruncated: boolean;
  body: string;
  bodySha256: string;
  bodyTruncated: boolean;
  state: string;
  sourceUpdatedAt: string;
  observedAt: string;
  htmlUrl: string;
  commentCoverage: string;
  commentCoverageReason: string;
  commentCount: string;
  commentContextPartial: boolean;
  comments: ReadonlyArray<FeedbackCommentView>;
}>;

export type CompanyFeedbackView = Readonly<{
  companyId: string;
  sources: ReadonlyArray<FeedbackSourceView>;
  issues: ReadonlyArray<FeedbackIssueView>;
}>;

export type GitHubCredentialReceipt = Readonly<{
  credentialRef: 'default-readonly';
  stored: boolean;
}>;

export type GitHubFeedbackSourceCommandReceipt = Readonly<{
  companyId: string;
  sourceId: string;
  repositoryId: string;
  repository: string;
  profileRevision: string;
  filterRevision: string;
  configurationSha256: string;
  state: string;
  permissionStatus: string;
  permissionProbeSha256?: string;
  createdAt: string;
}>;

export type GitHubFeedbackProbeReceipt = Readonly<{
  companyId: string;
  sourceId: string;
  requestId: string;
  permissionStatus: string;
  coverage: string;
  coverageReason: string;
}>;

export type GitHubFeedbackPollReceipt = Readonly<{
  companyId: string;
  sourceId: string;
  requestId: string;
  scanId: string;
  coverage: string;
  coverageReason: string;
  coveredThrough: string | null;
  pageCount: number;
  itemCount: number;
  commentScanCount: number;
  commentCoverage: string;
  commentCoverageReason: string;
  replayed: boolean;
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
  createdAt: string;
}>;

export type StdioMCPPackageFileView = Readonly<{
  relativePath: string;
  mediaType: string;
  byteSize: number;
  contentSHA256: string;
}>;

export type StdioMCPPackageManifestView = Readonly<{
  schemaVersion: 'polis-controlled-stdio-mcp@1';
  name: string;
  serverName: string;
  serverVersion: string;
  command: string;
  entryPoint: string;
  args: ReadonlyArray<string>;
  files: ReadonlyArray<StdioMCPPackageFileView>;
}>;

export type StdioMCPPackageRevisionView = Readonly<{
  companyId: string;
  id: string;
  serverId: string;
  revision: string;
  manifestDigest: string;
  manifest: StdioMCPPackageManifestView;
  createdAt: string;
}>;

export type CapabilityQualificationView = Readonly<{
  companyId: string;
  qualificationId: string;
  capabilityKind: 'skill' | 'mcp';
  capabilityId: string;
  versionDigest: string;
  profile: 'read_only_skill@1' | 'stdio_mcp@1' | 'streamable_http_mcp_2026_07_28@1';
  status: 'metadata_verified' | 'needs_external_qualification' | 'failed';
  evidenceDigest: string;
  createdAt: string;
}>;

export type EmployeeCapabilityBindingView = Readonly<{
  companyId: string;
  eventId: string;
  employeeId: string;
  capabilityKind: 'skill' | 'mcp';
  capabilityId: string;
  versionDigest: string;
  qualificationId: string;
  qualificationStatus: 'metadata_verified' | 'needs_external_qualification' | 'failed';
  state: 'bound' | 'revoked';
  executionStatus: 'runtime_unqualified' | 'runtime_qualified_dispatch_unavailable' | 'runtime_qualified_dispatch_disabled' | 'runtime_qualified_dispatch_available';
  reason: string;
  createdAt: string;
}>;

export type StdioMCPRuntimeQualificationView = Readonly<{
  companyId: string;
  runtimeQualificationId: string;
  capabilityId: string;
  capabilityQualificationId: string;
  transport?: 'stdio' | 'streamable_http';
  endpoint?: string;
  versionDigest: string;
  descriptorDigest: string;
  runtimeProfile: 'polis-controlled-stdio-mcp-2026-07-28@1' | 'polis-streamable-http-mcp-2026-07-28@1';
  hostOs: 'windows' | 'remote';
  hostProfile: 'windows_appcontainer_deny_all@1' | 'https_public_dns_pinned@1';
  commandSha256: string;
  packageManifestSha256: string;
  serverName: string;
  serverVersion: string;
  protocolVersion: '2026-07-28';
  toolSchemaSha256: string;
  status: 'observed_unqualified' | 'qualified' | 'schema_drift' | 'revoked';
  evidenceDigest: string;
  createdAt: string;
}>;

export type CapabilityDecisionView = Readonly<{
  companyId: string;
  decisionId: string;
  capabilityKind: 'skill' | 'mcp';
  capabilityId: string;
  versionDigest: string;
  qualificationId: string;
  decision: 'approved' | 'revoked';
  rationale: string;
  actor: string;
  createdAt: string;
}>;

export type CapabilityRevocationSessionView = Readonly<{
  sessionId: string;
  employeeId: string;
  taskId: string;
  missionId: string;
  state: string;
  stateAtRevocation?: string;
  skillLoadCount: number;
  mcpCallCount: number;
  dispatchingMcpCallCount: number;
}>;

export type CapabilityRevocationMCPCallView = Readonly<{
  intentId: string;
  sessionId: string;
  employeeId: string;
  toolName: string;
  status: 'dispatching' | 'completed' | 'outcome_unknown';
  statusAtRevocation?: 'dispatching' | 'completed' | 'outcome_unknown';
  reasonCode?: string;
  createdAt: string;
}>;

export type CapabilityRevocationOwnerReviewView = Readonly<{
  disposition: 'acknowledged_unresolved';
  rationale: string;
  actor: 'installation-owner';
  reviewedAt: string;
}>;

export type CapabilityRevocationStatusView = Readonly<{
  companyId: string;
  revocationId: string;
  scope: 'capability' | 'employee';
  capabilityKind: 'skill' | 'mcp';
  capabilityId: string;
  versionDigest: string;
  qualificationId: string;
  employeeId?: string;
  reason: string;
  actor: string;
  acceptedAt: string;
  revocationAccepted: boolean;
  effectiveForNewDispatch: boolean;
  quiesced: boolean;
  sessionInventoryComplete: boolean;
  ownerReview?: CapabilityRevocationOwnerReviewView;
  affectedSessionCount: number;
  liveSessionCount: number;
  sessions: ReadonlyArray<CapabilityRevocationSessionView>;
  sessionsTruncated: boolean;
  mcpCallCount: number;
  dispatchingMcpCallCount: number;
  mcpCalls: ReadonlyArray<CapabilityRevocationMCPCallView>;
  mcpCallsTruncated: boolean;
}>;

export type CapabilityCatalogView = Readonly<{
  skills: ReadonlyArray<SkillRevisionView>;
  mcpServers: ReadonlyArray<MCPServerDefinitionView>;
  mcpPackages: ReadonlyArray<StdioMCPPackageRevisionView>;
  qualifications: ReadonlyArray<CapabilityQualificationView>;
  bindings: ReadonlyArray<EmployeeCapabilityBindingView>;
  decisions: ReadonlyArray<CapabilityDecisionView>;
  runtimeQualifications: ReadonlyArray<StdioMCPRuntimeQualificationView>;
  revocations: ReadonlyArray<CapabilityRevocationStatusView>;
  revocationsTruncated: boolean;
  runtimeObservationAvailable: boolean;
  streamableHttpRuntimeObservationAvailable?: boolean;
}>;

export type DomainEvidenceAreaView = 'quality' | 'intervention' | 'recovery' | 'cost' | 'organization_benefit';
export type DomainEvidenceReadinessStatus = 'incomplete' | 'ready_for_review';
export type DomainProfileQualificationDecisionView = 'qualified' | 'revoked';

export type DomainWorkflowProfileView = Readonly<{
  id: string;
  revision: string;
  domain: 'content_operations' | 'research_simulation';
  qualificationStatus: 'not_run' | DomainProfileQualificationDecisionView;
  executionEnabled: false;
  stages: ReadonlyArray<string>;
  requiredEvidence: ReadonlyArray<DomainEvidenceAreaView>;
}>;

export type DomainEvidenceItemView = Readonly<{
  area: DomainEvidenceAreaView;
  artifact: Readonly<{id: string; revision: number; sha256: string}>;
  methodSHA256: string;
  assessedByEmployeeId: string;
}>;

export type DomainEvidenceSubmissionView = Readonly<{
  profileId: string;
  profileRevision: string;
  evidence: ReadonlyArray<DomainEvidenceItemView>;
}>;

export type DomainEvidenceReviewOutcomeView = 'evidence_references_accepted' | 'evidence_references_rejected' | 'more_evidence_required';

export type DomainEvidenceAreaAssessmentOutcomeView = 'accepted' | 'rejected' | 'insufficient';
export type DomainEvidenceSubstantiveOutcomeView = 'evidence_accepted' | 'evidence_rejected' | 'more_evidence_required';

export type DomainEvidenceAreaAssessmentView = Readonly<{
  area: DomainEvidenceAreaView;
  outcome: DomainEvidenceAreaAssessmentOutcomeView;
  rationale: string;
}>;

export type DomainEvidencePreviewAttestationView = Readonly<{
  area: DomainEvidenceAreaView;
  relativePath: string;
  sourceDigest: string;
  contentDigest: string;
  mediaType: string;
}>;

export type DomainEvidenceReviewRecordView = Readonly<{
  companyId: string;
  reviewId: string;
  recordId: string;
  outcome: DomainEvidenceReviewOutcomeView;
  reviewerEmployeeId: string;
  rationale: string;
  reviewContractRevision: 0 | 1;
  previewedEvidence: ReadonlyArray<DomainEvidencePreviewAttestationView>;
  requestId: string;
  createdAt: string;
}>;

export type DomainEvidenceSubstantiveAssessmentRecordView = Readonly<{
  companyId: string;
  assessmentId: string;
  recordId: string;
  evidenceDigest: string;
  outcome: DomainEvidenceSubstantiveOutcomeView;
  reviewerEmployeeId: string;
  areaAssessments: ReadonlyArray<DomainEvidenceAreaAssessmentView>;
  previewedEvidence: ReadonlyArray<DomainEvidencePreviewAttestationView>;
  requestId: string;
  createdAt: string;
}>;

export type DomainEvidenceArtifactPreviewEntryView = Readonly<{
  relativePath: string;
  fileName: string;
  mediaType: string;
  byteSize: number;
  contentSHA256: string;
  previewable: boolean;
  reasonCode: string;
}>;

export type DomainEvidenceArtifactPreviewManifestView = Readonly<{
  companyId: string;
  recordId: string;
  area: DomainEvidenceAreaView;
  inputId: string;
  inputRevision: number;
  sourceDigest: string;
  entries: ReadonlyArray<DomainEvidenceArtifactPreviewEntryView>;
}>;

export type DomainEvidenceArtifactPreviewView = Readonly<{
  companyId: string;
  recordId: string;
  area: DomainEvidenceAreaView;
  relativePath: string;
  inputId: string;
  inputRevision: string;
  sourceDigest: string;
  contentDigest: string;
  fileName: string;
  mediaType: string;
  byteSize: string;
  content: Blob;
  textContent: string | null;
}>;

export type DomainEvidenceRecordView = Readonly<{
  companyId: string;
  recordId: string;
  profileId: string;
  profileRevision: string;
  readinessStatus: DomainEvidenceReadinessStatus;
  qualificationStatus: 'not_run';
  executionEnabled: false;
  evidenceDigest: string;
  submission: DomainEvidenceSubmissionView;
  review: DomainEvidenceReviewRecordView | null;
  assessment: DomainEvidenceSubstantiveAssessmentRecordView | null;
  reasonCodes: ReadonlyArray<string>;
  requestId: string;
  createdAt: string;
}>;

export type DomainProfileQualificationRecordView = Readonly<{
  companyId: string;
  eventId: string;
  profileId: string;
  profileRevision: string;
  decision: DomainProfileQualificationDecisionView;
  evidenceInputId: string;
  evidenceInputRevision: string;
  evidenceSha256: string;
  rationale: string;
  actor: 'local-owner';
  requestId: string;
  createdAt: string;
}>;

export type ResearchSimulationOutputView = Readonly<{
  schemaVersion: 'polis-research-simulation-output@1';
  algorithm: 'bootstrap-mean-difference@1';
  protocolRevision: number;
  datasetSha256: string;
  methodSha256: string;
  seed: string;
  controlDefinition: string;
  iterations: number;
  sampleSize: number;
  riskConsumedUnits: number;
  riskUnit: 'sample_draw';
  controlMean: number;
  treatmentMean: number;
  meanDifference: number;
  minimumDifference: number;
  maximumDifference: number;
}>;

export type ResearchSimulationRunView = Readonly<{
  companyId: string;
  runId: string;
  profileId: 'research-simulation-reference';
  profileRevision: 'research-simulation@1';
  protocolRevision: string;
  datasetInputId: string;
  datasetInputRevision: string;
  datasetSha256: string;
  methodInputId: string;
  methodInputRevision: string;
  methodSha256: string;
  seed: string;
  controlDefinition: string;
  riskUnit: 'sample_draw';
  riskBudgetUnits: string;
  riskConsumedUnits: string;
  outputSha256: string;
  output: ResearchSimulationOutputView;
  requestId: string;
  createdAt: string;
}>;

export type DomainContentSourceStateView = 'authorized' | 'revoked';
export type DomainContentClaimFindingView = 'verified' | 'inconclusive' | 'contradicted';
export type DomainContentReviewOutcomeView = 'accepted' | 'inconclusive' | 'rejected';

export type DomainContentSourceEventView = Readonly<{
  companyId: string;
  eventSeq: string;
  eventId: string;
  inputId: string;
  revision: string;
  sha256: string;
  state: DomainContentSourceStateView;
  rationale: string;
  requestId: string;
  createdAt: string;
}>;

export type DomainContentDraftView = Readonly<{
  companyId: string;
  draftId: string;
  draftInputId: string;
  draftRevision: string;
  draftSha256: string;
  writerEmployeeId: string;
  criticalClaims: ReadonlyArray<string>;
  constraintsPassed: boolean;
  requestId: string;
  createdAt: string;
}>;

export type DomainContentSourceReferenceView = Readonly<{inputId: string; revision: string; sha256: string}>;
export type DomainContentClaimReviewView = Readonly<{
  claimId: string;
  finding: DomainContentClaimFindingView;
  sources: ReadonlyArray<DomainContentSourceReferenceView>;
  limitation: string;
}>;
export type DomainContentReviewSubmissionView = Readonly<{
  draftRevision: string;
  checkerEmployeeId: string;
  humanSampled: boolean;
  claims: ReadonlyArray<DomainContentClaimReviewView>;
}>;
export type DomainContentSampleEvidenceView = Readonly<{
  draftRevision: string;
  draftSha256: string;
  plan: DomainContentSourceReferenceView;
  sampledClaimIds: ReadonlyArray<string>;
  sampledByEmployeeId: string;
}>;
export type DomainContentReviewView = Readonly<{
  companyId: string;
  reviewId: string;
  draftInputId: string;
  draftRevision: string;
  draftSha256: string;
  correctionId: string;
  checkerEmployeeId: string;
  outcome: DomainContentReviewOutcomeView;
  stale: boolean;
  review: DomainContentReviewSubmissionView;
  sample: DomainContentSampleEvidenceView;
  reasonCodes: ReadonlyArray<string>;
  requestId: string;
  createdAt: string;
}>;

export type DomainContentPublicationReceiptView = Readonly<{
  schemaVersion: 'polis-content-publication-simulation@1';
  mode: 'simulation';
  publicationId: string;
  reviewId: string;
  draftInputId: string;
  draftRevision: string;
  draftSha256: string;
  externalSideEffects: false;
}>;
export type DomainContentPublicationView = Readonly<{
  companyId: string;
  publicationId: string;
  reviewId: string;
  draftInputId: string;
  draftRevision: string;
  draftSha256: string;
  mode: 'simulation';
  externalSideEffects: false;
  receiptSha256: string;
  receipt: DomainContentPublicationReceiptView;
  requestId: string;
  createdAt: string;
}>;
export type DomainContentCorrectionView = Readonly<{
  companyId: string;
  correctionId: string;
  publicationId: string;
  correctionDraftInputId: string;
  correctionDraftRevision: string;
  correctionDraftSha256: string;
  rationale: string;
  state: 'review_required';
  requestId: string;
  createdAt: string;
}>;
export type DomainContentFeedbackCategoryView = 'positive' | 'negative' | 'mixed' | 'inconclusive' | 'correction_requested';
export type DomainContentFeedbackView = Readonly<{
  companyId: string;
  feedbackId: string;
  publicationId: string;
  category: DomainContentFeedbackCategoryView;
  note: string;
  state: 'recorded' | 'review_required';
  requestId: string;
  createdAt: string;
}>;

export type DomainEvidenceLedgerView = Readonly<{
  companyId: string;
  profiles: ReadonlyArray<DomainWorkflowProfileView>;
  submissions: ReadonlyArray<DomainEvidenceRecordView>;
  qualifications: ReadonlyArray<DomainProfileQualificationRecordView>;
  researchSimulationRuns: ReadonlyArray<ResearchSimulationRunView>;
  contentSourceEvents: ReadonlyArray<DomainContentSourceEventView>;
  contentDrafts: ReadonlyArray<DomainContentDraftView>;
  contentReviews: ReadonlyArray<DomainContentReviewView>;
  contentPublications: ReadonlyArray<DomainContentPublicationView>;
  contentCorrections: ReadonlyArray<DomainContentCorrectionView>;
  contentFeedback: ReadonlyArray<DomainContentFeedbackView>;
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

export type DailyRoutineCommandReceipt = Readonly<{
  commandId: string;
  commandType: 'routine.daily.create' | 'routine.daily.instruction';
  targetType: 'routine';
  targetId: string;
  requestId: string;
  accepted: true;
  acceptedAt: string;
  resultingState: 'active' | 'instruction_set';
}>;

export type CompanyCommandType = 'company.create' | 'company.update' | 'company.team_coverage.confirm' | 'company.archive' | 'runtime.settings.update' | 'notification.route.update';

export type CompanyCommandResultState = 'active' | 'archived' | 'restart_required' | 'configured' | 'unverified' | 'ready' | 'revoked';

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

export type MissionChangeRequestState = 'received' | 'queued' | 'considered' | 'applied' | 'declined' | 'superseded';

export type MissionChangeInputRevision = Readonly<{
  inputId: string;
  revision: number;
  contentDigest: string;
  state: 'usable' | 'partial' | 'unsupported';
}>;

export type MissionChangeTaskImpact = Readonly<{
  taskId: string;
  ownerEmployeeId: string;
  kind: string;
  state: string;
  generation: number;
  workspaceDigest: string | null;
  workspaceRevision: number | null;
}>;

export type MissionChangeArtifactImpact = Readonly<{
  artifactId: string;
  taskId: string;
  digest: string;
  verdict: string;
}>;

export type MissionChangeWorkerImpact = Readonly<{
  sessionId: string;
  taskId: string;
  employeeId: string;
  state: string;
}>;

export type MissionChangeJobImpact = Readonly<{
  jobId: string;
  taskId: string;
  state: string;
  readiness: string;
}>;

export type MissionChangeServiceEndpointImpact = Readonly<{
  jobId: string;
  generation: number;
  readiness: 'not_ready' | 'ready' | 'unhealthy' | 'revoked';
}>;

export type MissionChangeTakeoverLeaseImpact = Readonly<{
  leaseId: string;
  taskId: string;
  baseWorkspaceDigest: string;
  baseWorkspaceRevision: number;
}>;

export type MissionChangeTakeoverSnapshotImpact = Readonly<{
  leaseId: string;
  taskId: string;
  inputId: string;
  inputRevision: number;
  contentDigest: string;
  byteSize: number;
  humanEffortSeconds: number | null;
}>;

export type MissionChangeImpact = Readonly<{
  schemaVersion: 'polis-mission-change-impact@1';
  missionId: string;
  baseRequirementsSha256: string;
  inputRevisions: ReadonlyArray<MissionChangeInputRevision>;
  tasks: ReadonlyArray<MissionChangeTaskImpact>;
  artifacts: ReadonlyArray<MissionChangeArtifactImpact>;
  activeWorkerSessions: ReadonlyArray<MissionChangeWorkerImpact>;
  nonterminalJobRuns: ReadonlyArray<MissionChangeJobImpact>;
  activeServiceEndpoints: ReadonlyArray<MissionChangeServiceEndpointImpact>;
  activeTaskTakeoverLeases: ReadonlyArray<MissionChangeTakeoverLeaseImpact>;
  returnedHumanTakeoverSnapshots: ReadonlyArray<MissionChangeTakeoverSnapshotImpact>;
  naturalLanguageImpactStatus: 'not_assessed';
}>;

export type MissionChangeInputRevisionMap = Readonly<{
  origin: 'mission_input' | 'task_workspace' | 'human_takeover';
  sourceTaskId?: string;
  previousInputId: string;
  previousRevision: number;
  successorInputId: string;
  successorRevision: number;
  contentDigest: string;
}>;

export type MissionChangeRequestEvent = Readonly<{
  eventId: string;
  state: MissionChangeRequestState;
  impactRevision: number | null;
  successorMissionId: string | null;
  reasonCode: string;
  createdAt: string;
  inputRevisionMap: ReadonlyArray<MissionChangeInputRevisionMap>;
  planningAssessmentId?: string;
  planningAssessmentSha256?: string;
  planningRiskLevel?: 'low' | 'high' | 'uncertain';
}>;

export type MissionChangePlanningAssessmentView = Readonly<{
  schemaVersion: 'polis-mission-change-planning-assessment@1';
  assessmentId: string;
  revision: number;
  status: 'current' | 'stale';
  analysisBasisSha256: string;
  assessmentSha256: string;
  riskLevel: 'low' | 'high' | 'uncertain';
  summary: string;
  affectedTaskIds: ReadonlyArray<string>;
  unaffectedTaskIds: ReadonlyArray<string>;
  uncertainTaskIds: ReadonlyArray<string>;
  questions: ReadonlyArray<string>;
  recommendedControls: ReadonlyArray<string>;
  workerSessionId: string;
  workerTaskId: string;
  workerEpoch: number;
  createdAt: string;
}>;

export type MissionChangeRequestView = Readonly<{
  changeRequestId: string;
  missionId: string;
  clientRequestId: string;
  baseRequirementsSha256: string;
  changeSummary: string;
  proposedTitle: string;
  proposedGoal: string;
  proposedAcceptanceContract: AcceptanceContract | null;
  blockPreviousResults: boolean;
  state: MissionChangeRequestState;
  impactRevision: number;
  impactSha256: string;
  impact: MissionChangeImpact;
  planningAssessment: MissionChangePlanningAssessmentView | null;
  successorMissionId: string | null;
  inputRevisionMap: ReadonlyArray<MissionChangeInputRevisionMap>;
  createdAt: string;
  events: ReadonlyArray<MissionChangeRequestEvent>;
}>;

export type TaskTakeoverLeaseState = 'granted' | 'returned' | 'released';

export type TaskTakeoverDiffSummaryView = Readonly<{
  model: string;
  baseWorkspaceDigest: string;
  baseWorkspaceRevision: number;
  submittedContentDigest: string;
  baseBytes: number;
  submittedBytes: number;
  removedLines: number;
  addedLines: number;
  changed: boolean;
  baseWorkspaceTreeSha256?: string;
  submittedWorkspaceTreeSha256?: string;
  addedFiles?: ReadonlyArray<string>;
  modifiedFiles?: ReadonlyArray<string>;
  deletedFiles?: ReadonlyArray<string>;
}>;

export type TaskTakeoverLeaseEventView = Readonly<{
  eventId: string;
  state: TaskTakeoverLeaseState;
  snapshotInputId: string | null;
  snapshotRevision: number | null;
  snapshotDigest: string | null;
  snapshotBytes: number | null;
  humanEffortSeconds: number | null;
  diffSummary: TaskTakeoverDiffSummaryView | null;
  reasonCode: string;
  createdAt: string;
}>;

export type TaskTakeoverLeaseView = Readonly<{
  leaseId: string;
  missionId: string;
  taskId: string;
  clientRequestId: string;
  baseRequirementsSha256: string;
  baseWorkspaceDigest: string;
  baseWorkspaceRevision: number;
  workspaceTree?: TaskTakeoverWorkspaceTreeBindingView;
  state: TaskTakeoverLeaseState;
  snapshotInputId: string | null;
  snapshotRevision: number | null;
  snapshotDigest: string | null;
  snapshotBytes: number | null;
  humanEffortSeconds: number | null;
  diffSummary: TaskTakeoverDiffSummaryView | null;
  createdAt: string;
  events: ReadonlyArray<TaskTakeoverLeaseEventView>;
}>;

export type TaskTakeoverWorkspaceTreeBindingView = Readonly<{
  rootBindingId: string;
  revision: number;
  manifestSha256: string;
  fileCount: number;
  bytes: number;
}>;

export type TaskTakeoverWorkspaceManifestEntryView = Readonly<{
  relativePath: string;
  sha256: string;
  bytes: number;
  fileRevision: number;
  sourceRevision: number;
  contentType: 'text/utf-8';
}>;

export type TaskTakeoverWorkspaceManifestView = Readonly<{
  leaseId: string;
  missionId: string;
  taskId: string;
  workspaceTree: TaskTakeoverWorkspaceTreeBindingView;
  entries: ReadonlyArray<TaskTakeoverWorkspaceManifestEntryView>;
}>;

export type TaskTakeoverWorkspaceFileView = Readonly<{
  leaseId: string;
  missionId: string;
  taskId: string;
  manifestSha256: string;
  relativePath: string;
  sha256: string;
  bytes: number;
  fileRevision: number;
  workspaceRevision: number;
  contentType: 'text/utf-8';
  content: string;
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

export type EmployeeScheduleState = 'quiescing' | 'sleeping' | 'wake_pending' | 'admitted' | 'working' | 'paused' | 'waiting_quota';

export type EmployeeScheduleView = Readonly<{
  state: EmployeeScheduleState;
  workGeneration: string;
  checkedGeneration: string;
  nextDueAt: string | null;
  pauseReason: string;
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
  schedule: EmployeeScheduleView | null;
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
  workflowState?: 'open' | 'acknowledged';
  notificationState?: string;
}>;

export type HumanInterventionState = 'acknowledged' | 'resolved';

export type HumanInterventionCommandReceipt = Readonly<{
  commandId: string;
  commandType: `human_intervention.${HumanInterventionState}`;
  targetType: 'human_intervention';
  targetId: string;
  requestId: string;
  accepted: true;
  acceptedAt: string;
  resultingState: HumanInterventionState;
}>;

export type MissionCloseoutSummary = Readonly<{
  requestedOutcome: 'succeeded' | 'ended_not_met' | 'cancelled';
  rationale: string;
  acceptanceArtifactIds: ReadonlyArray<string>;
  requestId: string;
  openedAt: string;
  terminalOutcome: 'succeeded' | 'ended_not_met' | 'cancelled' | null;
  report?: Readonly<Record<string, unknown>>;
  finishedAt: string | null;
}>;

export type MissionSummary = Readonly<{
  missionId: string;
  title: string;
  goal: string;
  state: 'draft' | 'active' | 'paused' | 'closing' | 'succeeded' | 'ended_not_met' | 'cancelled';
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
  closeout?: MissionCloseoutSummary | null;
}>;

export type DailyRoutineCatchUpPolicy = 'skip' | 'coalesce_latest' | 'catch_up';

export type DailyRoutineView = Readonly<{
  routineId: string;
  missionId: string;
  employeeId: string;
  timezone: string;
  localTime: string;
  nextLogicalDay: string;
  catchUpPolicy: DailyRoutineCatchUpPolicy;
  maxCatchUp: number;
  nextDueAt: string | null;
  taskInstruction: string | null;
  needsInstructionOccurrences: number;
  linkedTaskCount: number;
}>;

export type MemoryTaskImpactView = Readonly<{
  dependencyId: string;
  recordId: string;
  recordRevision: number;
  replacementRevision: number;
  riskLevel: 'ordinary' | 'high' | 'critical';
  state: 'dirty' | 'frozen';
  correctionId: string;
  reason: string;
}>;

export type MemoryCorrectionQueueItemView = Readonly<{
  correctionId: string;
  recordId: string;
  recordKind: string;
  recordScope: string;
  missionId?: string;
  sensitivity: string;
  baseRevision: number;
  currentRevision: number;
  currentState: string;
  source: MemorySourceReferenceView;
  observedAt: string;
  proposedBy: string;
  proposedBySessionId?: string;
  proposedByTaskId?: string;
  proposedAt: string;
  state: 'proposed' | 'approved' | 'rejected' | 'stale' | 'revoked';
  reviewDecision?: string;
  reviewActor?: string;
  reviewSessionId?: string;
  reviewTaskId?: string;
  reviewSequence?: number;
}>;

export type MemoryCorrectionCommandReceiptView = Readonly<{
  id: string;
  status: string;
  revision?: number;
}>;

export type MemoryCorrectionQueueView = Readonly<{
  items: ReadonlyArray<MemoryCorrectionQueueItemView>;
  truncated: boolean;
}>;

export type ProblemToolCallBudgetState = 'pending' | 'unbounded' | 'available' | 'closing_reserved' | 'exhausted';
export type ProblemToolCallBudgetRejectionRoute = 'worker_admission' | 'worker_tool_call';
export type ProblemToolCallBudgetRejectionReason = 'session_limit' | 'task_limit' | 'problem_limit' | 'closing_reserve' | 'initial_closing_reserve';

export type TaskToolCallBudgetView = Readonly<{
  taskId: string;
  kind: string;
  toolCallLimit: number | null;
  toolCallsUsed: number;
  toolCallsRemaining: number;
  allocationRevision: number;
  allocationEligible: boolean;
  lastAllocationReason?: string;
  lastAllocatedAt?: string;
  budgetRejectionCount: number;
  lastRejectionReason?: ProblemToolCallBudgetRejectionReason;
  lastRejectionAt?: string;
  closedIncomplete: boolean;
  closureReason?: string;
  closedAt?: string;
  closureEligible: boolean;
}>;

export type ProblemToolCallBudgetView = Readonly<{
  problemKey: string;
  missionId: string;
  taskCount: number;
  workerSessionAttempts: number;
  toolCallLimit: number | null;
  toolCallsUsed: number;
  toolCallsRemaining: number;
  allocationRevision: number;
  closingReserveToolCalls: number;
  closingReserveRemaining: number;
  closingReserveRevision: number;
  state: ProblemToolCallBudgetState;
  lastAllocationReason?: string;
  lastAllocatedAt?: string;
  lastClosingReserveReason?: string;
  lastClosingReserveAt?: string;
  budgetRejectionCount: number;
  lastRejectionAt?: string;
  lastRejectionRoute?: ProblemToolCallBudgetRejectionRoute;
  lastRejectionReason?: ProblemToolCallBudgetRejectionReason;
  lastRejectionTaskId?: string;
  tasks: ReadonlyArray<TaskToolCallBudgetView>;
  tasksTruncated: boolean;
}>;

export type ProblemToolCallBudgetListView = Readonly<{
  items: ReadonlyArray<ProblemToolCallBudgetView>;
  truncated: boolean;
}>;

export type MissionToolCallBudgetState = 'pending' | 'available' | 'closing_reserved' | 'exhausted';
export type MissionToolCallBudgetRejectionReason = 'mission_budget_pending' | 'mission_limit' | 'mission_closing_reserve';
export type MissionToolCallBudgetRejectionRoute = 'worker_admission' | 'worker_tool_call';

export type MissionToolCallBudgetView = Readonly<{
  missionId: string;
  title: string;
  missionState: string;
  toolCallLimit: number | null;
  toolCallsUsed: number;
  toolCallsRemaining: number;
  closingReserveToolCalls: number;
  closingReserveRemaining: number;
  closingReserveRevision: number;
  revision: number;
  state: MissionToolCallBudgetState;
  allocationCount: number;
  lastReason?: string;
  lastAllocatedAt?: string;
  lastClosingReserveReason?: string;
  lastClosingReserveAt?: string;
  rejectionCount: number;
  lastRejectionAt?: string;
  lastRejectionRoute?: MissionToolCallBudgetRejectionRoute;
  lastRejectionReason?: MissionToolCallBudgetRejectionReason;
  lastRejectionTaskId?: string;
}>;

export type MissionToolCallBudgetListView = Readonly<{
  items: ReadonlyArray<MissionToolCallBudgetView>;
  truncated: boolean;
}>;

export type MissionToolCallBudgetChangeReceiptView = Readonly<{
  id: string;
  status: 'mission_budget_configured' | 'mission_budget_allocated';
  revision: number;
}>;

export type CompanyToolCallBudgetState = 'pending' | 'available' | 'closing_reserved' | 'exhausted';
export type CompanyToolCallBudgetRejectionRoute = 'worker_admission' | 'worker_tool_call';
export type CompanyToolCallBudgetRejectionReason = 'company_budget_pending' | 'company_limit' | 'company_closing_reserve';

export type CompanyToolCallBudgetView = Readonly<{
  companyId: string;
  toolCallLimit: number | null;
  toolCallsUsed: number;
  toolCallsRemaining: number;
  closingReserveToolCalls: number;
  closingReserveRemaining: number;
  closingReserveRevision: number;
  lastClosingReserveReason?: string;
  lastClosingReserveAt?: string;
  revision: number;
  state: CompanyToolCallBudgetState;
  allocationCount: number;
  lastReason?: string;
  lastAllocatedAt?: string;
  rejectionCount: number;
  lastRejectionAt?: string;
  lastRejectionRoute?: CompanyToolCallBudgetRejectionRoute;
  lastRejectionReason?: CompanyToolCallBudgetRejectionReason;
  lastRejectionTaskId?: string;
}>;

export type CompanyToolCallBudgetChangeReceiptView = Readonly<{
  id: string;
  status: 'company_budget_configured' | 'company_budget_allocated';
  revision: number;
}>;

export type CompanyToolCallClosingReserveReceiptView = Readonly<{
  id: string;
  status: 'company_closing_reserve_updated';
  revision: number;
}>;

export type MissionToolCallClosingReserveReceiptView = Readonly<{
  id: string;
  status: 'mission_closing_reserve_updated';
  revision: number;
}>;

export type ProblemToolCallAllocationReceiptView = Readonly<{
  id: string;
  status: 'allocated';
  revision: number;
}>;

export type ProblemToolCallClosingReserveReceiptView = Readonly<{
  id: string;
  status: 'reserve_updated';
  revision: number;
}>;

export type TaskToolCallAllocationReceiptView = Readonly<{
  id: string;
  status: 'task_budget_allocated';
  revision: number;
}>;

export type TaskToolCallIncompleteClosureReceiptView = Readonly<{
  id: string;
  status: 'closed_incomplete';
  revision: 1;
}>;

export type MemoryTaskStatusView = Readonly<{
  state: 'clear' | 'dirty' | 'frozen';
  impacts: ReadonlyArray<MemoryTaskImpactView>;
}>;

export type MemorySourceReferenceView = Readonly<{
  kind: string;
  id: string;
  revision: number;
  sha256: string;
}>;

export type MemoryTaskRevalidationPreviewView = Readonly<{
  taskId: string;
  taskState: string;
  taskGeneration: number;
  taskPlan: unknown;
  taskPlanSha256: string;
  missionState: string;
  dependencyId: string;
  dependencyState: string;
  correctionId: string;
  correctionSource: MemorySourceReferenceView;
  correctionObservedAt: string;
  correctionProposerReason: string;
  correctionReviewReason: string;
  previousRecordId: string;
  previousRevision: number;
  replacementRevision: number;
  replacementContent: string;
  replacementContentSha256: string;
  riskLevel: 'ordinary' | 'high' | 'critical';
  targetKind: string;
  targetId: string;
  targetRevision: number;
  targetSha256: string;
  stoppedSessionId: string;
  workspaceDigest: string;
  workspaceRevision: number;
  workspaceContent: string;
  otherImpacts: ReadonlyArray<MemoryTaskImpactView>;
  contextSha256: string;
}>;

export type MemoryTaskRevalidationReceipt = Readonly<{
  id: string;
  status: 'revalidated';
  revision: number;
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

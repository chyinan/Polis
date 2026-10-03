// pattern: Imperative Shell
package kernel

type Workspace struct {
	Digest, Content string
	Revision        int64
}
type ToolCallBudget struct {
	Limit                          int64 `json:"tool_call_limit"`
	Used                           int64 `json:"tool_calls_used"`
	Remaining                      int64 `json:"tool_calls_remaining"`
	CompanyLimit                   int64 `json:"company_tool_call_limit"`
	CompanyUsed                    int64 `json:"company_tool_calls_used"`
	CompanyRemaining               int64 `json:"company_tool_calls_remaining"`
	CompanyAvailable               int64 `json:"company_tool_calls_available"`
	CompanyLimitConfigured         bool  `json:"company_tool_call_limit_configured"`
	CompanyBudgetRevision          int64 `json:"company_tool_call_budget_revision"`
	CompanyClosingReserve          int64 `json:"company_tool_calls_reserved_for_closing"`
	CompanyClosingReserveRemaining int64 `json:"company_tool_calls_closing_reserve_remaining"`
	CompanyClosingReserveRevision  int64 `json:"company_tool_call_closing_reserve_revision"`
	CompanyClosingClass            bool  `json:"company_closing_task_class"`
	MissionLimit                   int64 `json:"mission_tool_call_limit"`
	MissionUsed                    int64 `json:"mission_tool_calls_used"`
	MissionRemaining               int64 `json:"mission_tool_calls_remaining"`
	MissionAvailable               int64 `json:"mission_tool_calls_available"`
	MissionLimitConfigured         bool  `json:"mission_tool_call_limit_configured"`
	MissionBudgetRevision          int64 `json:"mission_tool_call_budget_revision"`
	MissionClosingReserve          int64 `json:"mission_tool_calls_reserved_for_closing"`
	MissionClosingReserveRemaining int64 `json:"mission_tool_calls_closing_reserve_remaining"`
	MissionClosingReserveRevision  int64 `json:"mission_tool_call_closing_reserve_revision"`
	MissionClosingClass            bool  `json:"mission_closing_task_class"`
	TaskLimit                      int64 `json:"task_tool_call_limit"`
	TaskUsed                       int64 `json:"task_tool_calls_used"`
	TaskRemaining                  int64 `json:"task_tool_calls_remaining"`
	ProblemLimit                   int64 `json:"problem_tool_call_limit"`
	ProblemUsed                    int64 `json:"problem_tool_calls_used"`
	ProblemRemaining               int64 `json:"problem_tool_calls_remaining"`
	ProblemAvailable               int64 `json:"problem_tool_calls_available"`
	ProblemClosingReserve          int64 `json:"problem_tool_calls_reserved_for_closing"`
	ProblemClosingClass            bool  `json:"problem_closing_task_class"`
}

type ProblemLineage struct {
	ProblemKey              string `json:"problem_key"`
	TaskCount               int64  `json:"task_count"`
	WorkerSessionAttempts   int64  `json:"worker_session_attempts"`
	ToolCallLimit           int64  `json:"problem_tool_call_limit"`
	ToolCallsUsed           int64  `json:"tool_calls_used"`
	ToolCallsRemaining      int64  `json:"problem_tool_calls_remaining"`
	ClosingReserveToolCalls int64  `json:"closing_reserve_tool_calls"`
	ClosingReserveRemaining int64  `json:"closing_reserve_remaining"`
	ClosingReserveRevision  int64  `json:"closing_reserve_revision"`
}

type SkillReferenceSummary struct {
	RelativePath  string `json:"relativePath"`
	MediaType     string `json:"mediaType"`
	ByteSize      int64  `json:"byteSize"`
	ContentDigest string `json:"contentDigest"`
	Loadable      bool   `json:"loadable"`
}

type BoundSkillSummary struct {
	SkillID       string                  `json:"skillId"`
	PackageID     string                  `json:"packageId"`
	Revision      string                  `json:"revision"`
	DisplayName   string                  `json:"displayName"`
	Description   string                  `json:"description"`
	VersionDigest string                  `json:"versionDigest"`
	Profile       string                  `json:"profile"`
	References    []SkillReferenceSummary `json:"references"`
}
type HandoverBundle struct {
	EmployeeID                    string
	Task                          Task
	Workspace                     Workspace
	Obligations                   []Obligation
	Checkpoints                   []Checkpoint
	MemoryStatus                  MemoryTaskStatus               `json:"memory_status,omitempty"`
	MemoryContext                 []MemoryTaskDependencyContext  `json:"memory_context,omitempty"`
	DirectMessageTargets          []ProductDirectMessageTarget   `json:"direct_message_targets,omitempty"`
	DirectMessageTargetsTruncated bool                           `json:"direct_message_targets_truncated,omitempty"`
	SkillCatalog                  []BoundSkillSummary            `json:"skill_catalog,omitempty"`
	SkillCatalogTruncated         bool                           `json:"skill_catalog_truncated,omitempty"`
	SkillLoads                    []SkillLoadUse                 `json:"skill_loads,omitempty"`
	SkillLoadsTruncated           bool                           `json:"skill_loads_truncated,omitempty"`
	MCPToolSets                   []BoundStdioMCPToolSet         `json:"mcp_tool_sets,omitempty"`
	SharedArtifactReads           []SharedMissionArtifactReadUse `json:"shared_artifact_reads,omitempty"`
	SharedArtifactReadsTruncated  bool                           `json:"shared_artifact_reads_truncated,omitempty"`
	CompanySeq                    int64
	Contract                      string
	ToolBudget                    ToolCallBudget
	ProblemLineage                ProblemLineage `json:"problem_lineage"`
}
type Checkpoint struct {
	AcceptanceCheckerRevision          string   `json:"acceptance_checker_revision,omitempty"`
	TaskValidationBindingDigest        string   `json:"task_validation_binding_digest,omitempty"`
	ValidationStatus                   string   `json:"validation_status,omitempty"`
	CheckpointPolicyRevision           string   `json:"checkpoint_policy_revision,omitempty"`
	ArtifactEligibilityPolicyRevision  string   `json:"artifact_eligibility_policy_revision,omitempty"`
	ContractSupersessionPolicyRevision string   `json:"contract_supersession_policy_revision,omitempty"`
	FinalizationState                  string   `json:"finalization_state,omitempty"`
	Kind                               string   `json:"kind,omitempty"`
	Summary                            string   `json:"summary"`
	Facts                              []string `json:"facts"`
	Decisions                          []string `json:"decisions"`
	Rejected                           []string `json:"rejected"`
	EvidenceRefs                       []string `json:"evidence_refs"`
	WorkspaceDigest                    string   `json:"workspace_digest,omitempty"`
	WorkspaceRevision                  int64    `json:"workspace_revision,omitempty"`
	ContractRevisionID                 string   `json:"contract_revision_id,omitempty"`
	FailedChecks                       []string `json:"failed_checks,omitempty"`
	PendingMessageID                   string   `json:"pending_message_id,omitempty"`
	PendingMessageState                string   `json:"pending_message_state,omitempty"`
	PendingObligationID                string   `json:"pending_obligation_id,omitempty"`
	PendingObligationState             string   `json:"pending_obligation_state,omitempty"`
	SessionID                          string   `json:"session_id,omitempty"`
	Epoch                              int64    `json:"epoch,omitempty"`
	NextAction                         string   `json:"next_action,omitempty"`
}

const (
	CheckpointProgress  = "progress"
	CheckpointQualified = "qualified"
)

func (b Binding) SessionID() string                   { return b.session }
func (b Binding) EmployeeID() string                  { return b.employee }
func (b Binding) TaskID() string                      { return b.task }
func (b Binding) Epoch() int64                        { return b.epoch }
func (b Binding) Incarnation() string                 { return b.incarnation }
func (b Binding) WorkspaceDigest() string             { return b.workspaceDigest }
func (b Binding) WorkspaceRevision() int64            { return b.workspaceRevision }
func (b Binding) TaskValidationBindingDigest() string { return b.taskValidationBindingDigest }

func HasQualifiedCheckpoint(checkpoints []Checkpoint, workspace Workspace) bool {
	for _, checkpoint := range checkpoints {
		if checkpoint.Kind == CheckpointQualified && checkpoint.FinalizationState != "superseded_for_finalization" && checkpoint.WorkspaceDigest == workspace.Digest && checkpoint.WorkspaceRevision == workspace.Revision {
			return true
		}
	}
	return false
}

func HasQualifiedCheckpointForBinding(checkpoints []Checkpoint, workspace Workspace, bindingDigest string) bool {
	if bindingDigest == "" {
		return false
	}
	for _, checkpoint := range checkpoints {
		if checkpoint.Kind == CheckpointQualified && checkpoint.FinalizationState == "current" && checkpoint.WorkspaceDigest == workspace.Digest && checkpoint.WorkspaceRevision == workspace.Revision && checkpoint.TaskValidationBindingDigest == bindingDigest && checkpoint.ValidationStatus == "PASS" {
			return true
		}
	}
	return false
}

// pattern: Functional Core
package kernel

type FrontendStartingState struct {
	CompanyID                    string `json:"company_id"`
	MissionID                    string `json:"mission_id"`
	BackendTaskID                string `json:"backend_task_id"`
	FrontendTaskID               string `json:"frontend_task_id"`
	MessageID                    string `json:"message_id"`
	ObligationID                 string `json:"obligation_id"`
	ContractRevisionID           string `json:"contract_revision_id"`
	FrontendTaskState            string `json:"frontend_task_state"`
	MessageState                 string `json:"message_state"`
	ObligationState              string `json:"obligation_state"`
	ObligationOwner              string `json:"obligation_owner"`
	ContractState                string `json:"contract_state"`
	BackendSessionState          string `json:"backend_session_state"`
	FrontendSessionCount         int64  `json:"frontend_session_count"`
	LiveWriterCount              int64  `json:"live_writer_count"`
	EmployeeEpoch                int64  `json:"employee_epoch"`
	RuntimeIncarnation           string `json:"runtime_incarnation"`
	CurrentWriterSession         string `json:"current_writer_session,omitempty"`
	PlannerRelayCount            int64  `json:"planner_relay_count"`
	CompanySequence              int64  `json:"company_sequence"`
	EventCount                   int64  `json:"event_count"`
	WorkspaceRevision            int64  `json:"workspace_revision"`
	WorkspaceDigest              string `json:"workspace_digest"`
	BackendWorkspaceRevision     int64  `json:"backend_workspace_revision"`
	BackendWorkspaceDigest       string `json:"backend_workspace_digest"`
	BackendArtifactID            string `json:"backend_artifact_id"`
	BackendArtifactTaskID        string `json:"backend_artifact_task_id"`
	BackendArtifactAuthor        string `json:"backend_artifact_author"`
	BackendArtifactDigest        string `json:"backend_artifact_digest"`
	BackendArtifactState         string `json:"backend_artifact_state"`
	BackendArtifactVerdict       string `json:"backend_artifact_verdict"`
	BackendCheckpointID          string `json:"backend_checkpoint_id"`
	CheckpointWorkspaceRevision  int64  `json:"checkpoint_workspace_revision"`
	CheckpointWorkspaceDigest    string `json:"checkpoint_workspace_digest"`
	CheckpointContractRevisionID string `json:"checkpoint_contract_revision_id"`
}

func ValidateFrontendStartingState(state FrontendStartingState) []string {
	reasons := make([]string, 0)
	for name, value := range map[string]string{
		"company_id": state.CompanyID, "mission_id": state.MissionID, "backend_task_id": state.BackendTaskID,
		"frontend_task_id": state.FrontendTaskID, "message_id": state.MessageID, "obligation_id": state.ObligationID,
		"contract_revision_id": state.ContractRevisionID, "runtime_incarnation": state.RuntimeIncarnation, "workspace_digest": state.WorkspaceDigest, "backend_workspace_digest": state.BackendWorkspaceDigest,
	} {
		if value == "" {
			reasons = append(reasons, name+"_missing")
		}
	}
	if state.FrontendTaskState != "ready" {
		reasons = append(reasons, "frontend_task_not_ready")
	}
	if state.MessageState != "persisted" {
		reasons = append(reasons, "message_not_persisted")
	}
	if state.ObligationState != "pending" {
		reasons = append(reasons, "obligation_not_pending")
	}
	if state.ObligationOwner != "emp-frontend" {
		reasons = append(reasons, "obligation_owner_mismatch")
	}
	if state.MessageID != state.ObligationID {
		reasons = append(reasons, "message_obligation_mismatch")
	}
	if state.ContractState != "accepted" {
		reasons = append(reasons, "effective_contract_not_accepted")
	}
	if state.ContractRevisionID == "" {
		reasons = append(reasons, "effective_contract_missing")
	}
	if state.BackendSessionState != "stopped" {
		reasons = append(reasons, "backend_session_not_stopped")
	}
	if state.FrontendSessionCount != 0 {
		reasons = append(reasons, "frontend_sessions_expected_zero")
	}
	if state.LiveWriterCount != 0 {
		reasons = append(reasons, "frontend_live_writer_expected_zero")
	}
	if state.EmployeeEpoch <= 0 {
		reasons = append(reasons, "frontend_epoch_invalid")
	}
	if state.PlannerRelayCount != 0 {
		reasons = append(reasons, "planner_relay_expected_zero")
	}
	if state.WorkspaceRevision <= 0 {
		reasons = append(reasons, "frontend_workspace_revision_invalid")
	}
	if state.BackendWorkspaceRevision <= 0 {
		reasons = append(reasons, "backend_workspace_revision_invalid")
	}
	if state.BackendWorkspaceDigest == "" {
		reasons = append(reasons, "backend_workspace_digest_missing")
	}
	if state.BackendArtifactID == "" {
		reasons = append(reasons, "backend_artifact_missing")
	}
	if state.BackendArtifactTaskID != state.BackendTaskID {
		reasons = append(reasons, "backend_artifact_task_mismatch")
	}
	if state.BackendArtifactAuthor != "emp-backend" {
		reasons = append(reasons, "backend_artifact_author_mismatch")
	}
	if state.BackendArtifactState != "ready" {
		reasons = append(reasons, "backend_artifact_state_not_ready")
	}
	if state.BackendArtifactVerdict != "candidate" {
		reasons = append(reasons, "backend_artifact_verdict_not_candidate")
	}
	if state.BackendArtifactDigest == "" {
		reasons = append(reasons, "backend_artifact_digest_missing")
	}
	if state.BackendCheckpointID == "" {
		reasons = append(reasons, "backend_checkpoint_missing")
	}
	if state.CheckpointWorkspaceRevision != state.BackendWorkspaceRevision {
		reasons = append(reasons, "backend_checkpoint_workspace_revision_mismatch")
	}
	if state.CheckpointWorkspaceDigest != state.BackendWorkspaceDigest {
		reasons = append(reasons, "backend_checkpoint_workspace_digest_mismatch")
	}
	if state.CheckpointContractRevisionID != state.ContractRevisionID {
		reasons = append(reasons, "backend_checkpoint_contract_mismatch")
	}
	return reasons
}

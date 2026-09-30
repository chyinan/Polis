// pattern: Functional Core
package kernel

import "testing"

func cleanFrontendStartingStateForTest() FrontendStartingState {
	return FrontendStartingState{
		CompanyID: "company", MissionID: "mission", BackendTaskID: "backend-task", FrontendTaskID: "frontend-task",
		MessageID: "message", ObligationID: "message", ContractRevisionID: "contract",
		FrontendTaskState: "ready", MessageState: "persisted", ObligationState: "pending", ObligationOwner: "emp-frontend",
		ContractState: "accepted", BackendSessionState: "stopped", FrontendSessionCount: 0, LiveWriterCount: 0,
		EmployeeEpoch: 2, RuntimeIncarnation: "incarnation", PlannerRelayCount: 0, WorkspaceRevision: 1,
		WorkspaceDigest:          "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		BackendWorkspaceRevision: 1, BackendWorkspaceDigest: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		BackendArtifactID: "artifact", BackendArtifactTaskID: "backend-task", BackendArtifactAuthor: "emp-backend", BackendArtifactDigest: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef", BackendArtifactState: "ready", BackendArtifactVerdict: "candidate", BackendCheckpointID: "checkpoint", CheckpointWorkspaceRevision: 1, CheckpointWorkspaceDigest: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef", CheckpointContractRevisionID: "contract",
	}
}

func TestValidateFrontendStartingStateAcceptsCleanBaseline(t *testing.T) {
	if reasons := ValidateFrontendStartingState(cleanFrontendStartingStateForTest()); len(reasons) != 0 {
		t.Fatalf("clean starting state rejected: %v", reasons)
	}
}

func TestValidateFrontendStartingStateRejectsPersistedFrontendSession(t *testing.T) {
	state := cleanFrontendStartingStateForTest()
	state.FrontendSessionCount = 1
	state.LiveWriterCount = 1
	reasons := ValidateFrontendStartingState(state)
	if len(reasons) == 0 || reasons[0] != "frontend_sessions_expected_zero" {
		t.Fatalf("session mismatch was not actionable: %v", reasons)
	}
}

func TestValidateFrontendStartingStateRejectsPlannerRelay(t *testing.T) {
	state := cleanFrontendStartingStateForTest()
	state.PlannerRelayCount = 1
	reasons := ValidateFrontendStartingState(state)
	if len(reasons) == 0 || reasons[0] != "planner_relay_expected_zero" {
		t.Fatalf("planner relay mismatch was not actionable: %v", reasons)
	}
}

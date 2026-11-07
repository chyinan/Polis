// pattern: Functional Core
package probe

import "testing"

func TestBackendL2DiffAcceptsApprovedCheckpointEvidenceRefsEvolution(t *testing.T) {
	old := `[{"name":"polis_work_checkpoint","description":"same","inputSchema":{"type":"object","properties":{"evidence":{"type":"array"}},"required":["evidence"],"additionalProperties":false}}]`
	current := `[{"name":"polis_work_checkpoint","description":"same","inputSchema":{"type":"object","properties":{"evidence_refs":{"type":"array"}},"required":["evidence_refs"],"additionalProperties":false}}]`
	diff, err := compareBackendL2ToolRegistry([]byte(old), []byte(current))
	if err != nil {
		t.Fatal(err)
	}
	if !diff.Explained || len(diff.ApprovedChanges) != 1 || len(diff.UnexplainedChanges) != 0 || diff.ApprovedChanges[0].Path != "polis_work_checkpoint.inputSchema.properties.evidence -> evidence_refs" || diff.ApprovedChanges[0].OldRequired != "evidence" || diff.ApprovedChanges[0].NewRequired != "evidence_refs" {
		t.Fatalf("approved checkpoint evolution was not explained: %+v", diff)
	}
}

func TestBackendL2DiffRejectsUnrelatedProviderSurfaceDrift(t *testing.T) {
	old := `[{"name":"polis_work_checkpoint","description":"same","inputSchema":{"type":"object","properties":{"evidence":{"type":"array"}},"required":["evidence"],"additionalProperties":false}}]`
	current := `[{"name":"polis_work_checkpoint","description":"changed","inputSchema":{"type":"object","properties":{"evidence_refs":{"type":"array"}},"required":["evidence_refs"],"additionalProperties":false}}]`
	diff, err := compareBackendL2ToolRegistry([]byte(old), []byte(current))
	if err != nil {
		t.Fatal(err)
	}
	if diff.Explained || len(diff.UnexplainedChanges) == 0 {
		t.Fatalf("unrelated provider drift was accepted: %+v", diff)
	}
}

func TestBackendL2DiffAcceptsExplicitCollabSendTargetContract(t *testing.T) {
	old := `[{"name":"polis_collab_send","description":"old","inputSchema":{"type":"object","properties":{"to_task":{"type":"string"},"contract_revision_id":{"type":"string"},"body":{"type":"string"},"actionable":{"type":"boolean"}},"required":["actionable","body","contract_revision_id","to_task"],"additionalProperties":false}}]`
	current := `[{"name":"polis_collab_send","description":"Send a direct actionable collaboration message to the peer employee's current peer task. Use the explicit employee ID and task ID returned by work_current; to_task is not accepted.","inputSchema":{"type":"object","properties":{"to_employee_id":{"type":"string"},"to_task_id":{"type":"string"},"contract_revision_id":{"type":"string"},"body":{"type":"string"},"actionable":{"type":"boolean"}},"required":["actionable","body","contract_revision_id","to_employee_id","to_task_id"],"additionalProperties":false}}]`
	diff, err := compareBackendL2ToolRegistry([]byte(old), []byte(current))
	if err != nil {
		t.Fatal(err)
	}
	if !diff.Explained || len(diff.ApprovedChanges) != 1 || len(diff.UnexplainedChanges) != 0 || diff.ApprovedChanges[0].Tool != "polis_collab_send" {
		t.Fatalf("explicit collaboration target contract was not explained: %+v", diff)
	}
}

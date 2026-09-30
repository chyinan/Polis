// pattern: Imperative Shell
package kernel

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"polis/internal/core"
	"polis/internal/runner"
	"polis/internal/taskvalidation"
)

func TestProductCandidateFencesPostPublicationWorkspaceAndCheckpointWrites(t *testing.T) {
	dsn := os.Getenv("POLIS_R05B3_TEST_DSN")
	if dsn == "" {
		dsn = os.Getenv("POLIS_TEST_DSN")
	}
	if dsn == "" {
		t.Skip("dedicated PostgreSQL required")
	}
	ctx := context.Background()
	k, err := Open(ctx, dsn, t.TempDir())
	must(t, err)
	defer k.Close()

	scope, err := k.TXCreateCompany(ctx, "r05b3-finalizer-fence-company")
	must(t, err)
	acceptance := &taskvalidation.AcceptanceContract{Revision: taskvalidation.AcceptanceContractRevision, RequiredText: []string{"Task ID: {{task_id}}"}}
	mission, err := k.TXCreateMissionGoalWithAcceptance(ctx, scope, "candidate fencing", "ensure a published Artifact cannot be made stale", acceptance, "fence-create")
	must(t, err)
	_, err = k.TXStartMissionCommand(ctx, scope, mission.ID, "fence-start")
	must(t, err)
	task, err := k.TXPrepareProductTask(ctx, scope, mission.ID, "ensure a published Artifact cannot be made stale", "fence-prepare")
	must(t, err)
	binding, err := k.TXNewWorkerWithToolBudget(ctx, scope, task.ID, "offline-model/medium", 16)
	must(t, err)
	process, err := runner.Start(binding.SessionID(), []string{"/bin/sleep", "60"}, nil)
	must(t, err)
	stopped := false
	defer func() {
		if stopped {
			return
		}
		_ = k.TXBeginStop(context.Background(), binding)
		proof, stopErr := process.Stop()
		if stopErr == nil {
			_ = k.TXConfirmStopped(context.Background(), binding, proof)
		}
	}()
	must(t, k.TXAttachWorker(ctx, binding, process))
	must(t, k.TXValidateWorker(ctx, binding))
	must(t, k.TXActivateWorker(ctx, binding, "offline-model/medium"))

	tools := EmployeeTools{Kernel: k, Binding: binding, ProductSurface: true}
	handover, err := k.Handover(ctx, binding)
	must(t, err)
	content := strings.Join(handover.Task.ValidationBinding.Contract.RequiredText, "\n") + "\n"
	replaceArgs, err := json.Marshal(map[string]any{"expected_digest": handover.Workspace.Digest, "expected_revision": handover.Workspace.Revision, "content": content})
	must(t, err)
	write := tools.Call(ctx, "workspace_replace", "fence-write", replaceArgs)
	if write.Error != "" || write.Receipt == nil || write.Receipt.Revision != 2 {
		t.Fatalf("initial product workspace write failed: %+v", write)
	}

	check := tools.Call(ctx, "workspace_check", "fence-check", []byte(`{}`))
	if check.Error != "" || check.Receipt == nil {
		t.Fatalf("passing product check failed: %+v", check)
	}
	badCheckpoint := tools.Call(ctx, "work_checkpoint", "fence-bad-checkpoint", []byte(`{"kind":"qualified","summary":"bad","facts":["present"],"decisions":["publish"],"rejected":[],"evidence_refs":["prose is not a receipt"]}`))
	badRejection, ok := badCheckpoint.Data.(PeerToolRejection)
	if badCheckpoint.Error != core.Malformed.Error() || !ok || badRejection.FailingField != "evidence_refs[0]" || badRejection.ReasonCode != "invalid_field_type" {
		t.Fatalf("malformed product checkpoint did not expose field-level feedback: %+v", badCheckpoint)
	}
	submit := tools.Call(ctx, "task_submit", "fence-submit", []byte(`{}`))
	if submit.Error != "" || submit.Receipt == nil || submit.Receipt.Status != "candidate" {
		t.Fatalf("product task delivery failed: %+v", submit)
	}
	var delivery ProductDeliveryResult
	must(t, json.Unmarshal(mustJSON(t, submit.Data), &delivery))
	if delivery.CheckID != check.Receipt.ID || delivery.CheckpointID == "" || delivery.ArtifactID != submit.Receipt.ID {
		t.Fatalf("task_submit did not return deterministic delivery provenance: %+v", delivery)
	}

	postCandidateWrite, err := json.Marshal(map[string]any{"expected_digest": write.Receipt.ID, "expected_revision": write.Receipt.Revision, "content": content + "post-candidate mutation\n"})
	must(t, err)
	if result := tools.Call(ctx, "workspace_replace", "after-finalize-write", postCandidateWrite); result.Error != core.Denied.Error() {
		t.Fatalf("candidate Task accepted a post-publication workspace write: %+v", result)
	}
	if result := tools.Call(ctx, "workspace_check", "after-finalize-check", []byte(`{}`)); result.Error != core.Denied.Error() {
		t.Fatalf("candidate Task accepted a post-publication check: %+v", result)
	}
	if result := tools.Call(ctx, "work_checkpoint", "after-finalize-checkpoint", []byte(`{"kind":"progress","summary":"late","facts":["late"],"decisions":["late"],"rejected":[],"evidence_refs":[{"receipt_id":"late","proves":"workspace_check_pass"}],"next_action":"late"}`)); result.Error != core.Denied.Error() {
		t.Fatalf("candidate Task accepted a post-publication checkpoint: %+v", result)
	}

	var workspaceDigest string
	var workspaceRevision int64
	must(t, k.pool.QueryRow(ctx, "SELECT digest,revision FROM worker_workspaces WHERE company_id=$1 AND task_id=$2", scope.company, task.ID).Scan(&workspaceDigest, &workspaceRevision))
	if workspaceDigest != write.Receipt.ID || workspaceRevision != write.Receipt.Revision {
		t.Fatalf("post-publication calls changed the qualified workspace: digest=%s revision=%d", workspaceDigest, workspaceRevision)
	}
	var artifactDigest, taskState string
	must(t, k.pool.QueryRow(ctx, "SELECT a.digest,t.state FROM artifacts a JOIN tasks t ON t.company_id=a.company_id AND t.id=a.task_id WHERE a.company_id=$1 AND a.id=$2", scope.company, submit.Receipt.ID).Scan(&artifactDigest, &taskState))
	if artifactDigest != workspaceDigest || taskState != "candidate" {
		t.Fatalf("finalized Artifact/Task state drifted: artifact=%s workspace=%s task=%s", artifactDigest, workspaceDigest, taskState)
	}
	var qualifiedArtifact, qualifiedCheckpoint, qualifiedCheck, qualifiedSession, qualifiedBinding, qualifiedWorkspace, qualifiedRunner string
	var qualifiedRevision int64
	must(t, k.pool.QueryRow(ctx, `SELECT artifact_id,checkpoint_id,check_id,session_id,validation_binding_digest,workspace_digest,workspace_revision,runner_revision
FROM task_validation_artifact_qualifications WHERE company_id=$1 AND task_id=$2`, scope.company, task.ID).Scan(&qualifiedArtifact, &qualifiedCheckpoint, &qualifiedCheck, &qualifiedSession, &qualifiedBinding, &qualifiedWorkspace, &qualifiedRevision, &qualifiedRunner))
	if qualifiedArtifact != submit.Receipt.ID || qualifiedCheckpoint != delivery.CheckpointID || qualifiedCheck != check.Receipt.ID || qualifiedSession != binding.SessionID() || qualifiedBinding != handover.Task.ValidationBinding.ConfigurationDigest || qualifiedWorkspace != workspaceDigest || qualifiedRevision != write.Receipt.Revision || qualifiedRunner != taskvalidation.TextContainsAllRunnerRevision {
		t.Fatalf("Artifact qualification does not match the original validated candidate: artifact=%s checkpoint=%s check=%s session=%s binding=%s workspace=%s/%d runner=%s", qualifiedArtifact, qualifiedCheckpoint, qualifiedCheck, qualifiedSession, qualifiedBinding, qualifiedWorkspace, qualifiedRevision, qualifiedRunner)
	}
	must(t, k.TXBeginStop(ctx, binding))
	proof, err := process.Stop()
	must(t, err)
	must(t, k.TXConfirmStopped(ctx, binding, proof))
	stopped = true
}

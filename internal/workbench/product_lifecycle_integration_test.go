// pattern: Imperative Shell
package workbench_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	"polis/internal/kernel"
	"polis/internal/runner"
	"polis/internal/taskvalidation"
	"polis/internal/workbench"
)

func TestProductCandidateLifecycleProjectsThroughWorkbench(t *testing.T) {
	dsn := os.Getenv("POLIS_TEST_DSN")
	if dsn == "" {
		t.Skip("dedicated PostgreSQL required")
	}
	ctx := context.Background()
	k, err := kernel.Open(ctx, dsn, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer k.Close()
	companyID := fmt.Sprintf("r0-5b7-workbench-%d", time.Now().UnixNano())
	scope, err := k.TXCreateCompany(ctx, companyID)
	if err != nil {
		t.Fatal(err)
	}
	contract := &taskvalidation.AcceptanceContract{Revision: taskvalidation.AcceptanceContractRevision, RequiredText: []string{"Task ID: {{task_id}}"}}
	mission, err := k.TXCreateMissionGoalWithAcceptance(ctx, scope, "product lifecycle projection", "publish one candidate", contract, "r0-5b7-create")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = k.TXStartMissionCommand(ctx, scope, mission.ID, "r0-5b7-start"); err != nil {
		t.Fatal(err)
	}
	task, err := k.TXPrepareProductTask(ctx, scope, mission.ID, "publish one candidate", "r0-5b7-prepare")
	if err != nil {
		t.Fatal(err)
	}
	binding, err := k.TXNewWorkerWithToolBudget(ctx, scope, task.ID, "offline-model/medium", 16)
	if err != nil {
		t.Fatal(err)
	}
	process, err := runner.Start(binding.SessionID(), []string{"/bin/sh", "-c", "sleep 60"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer process.Stop()
	if err = k.TXAttachWorker(ctx, binding, process); err != nil {
		t.Fatal(err)
	}
	if err = k.TXValidateWorker(ctx, binding); err != nil {
		t.Fatal(err)
	}
	if err = k.TXActivateWorker(ctx, binding, "offline-model/medium"); err != nil {
		t.Fatal(err)
	}
	tools := kernel.EmployeeTools{Kernel: k, Binding: binding, ProductSurface: true}
	handover, err := k.Handover(ctx, binding)
	if err != nil {
		t.Fatal(err)
	}
	content := "Task ID: " + task.ID + "\n"
	writeArgs, _ := json.Marshal(map[string]any{"expected_digest": handover.Workspace.Digest, "expected_revision": handover.Workspace.Revision, "content": content})
	write := tools.Call(ctx, "workspace_replace", "r0-5b7-write", writeArgs)
	if write.Error != "" || write.Receipt == nil {
		t.Fatalf("workspace replace failed: %+v", write)
	}
	check := tools.Call(ctx, "workspace_check", "r0-5b7-check", []byte(`{}`))
	if check.Error != "" || check.Receipt == nil {
		t.Fatalf("workspace check failed: %+v", check)
	}
	checkpointArgs, _ := json.Marshal(map[string]any{"kind": "qualified", "summary": "candidate validated", "facts": []string{"Task ID criterion is present"}, "decisions": []string{"publish candidate"}, "rejected": []string{}, "evidence_refs": []map[string]string{{"receipt_id": check.Receipt.ID, "proves": "workspace_check_pass"}}})
	checkpoint := tools.Call(ctx, "work_checkpoint", "r0-5b7-checkpoint", checkpointArgs)
	if checkpoint.Error != "" || checkpoint.Receipt == nil {
		t.Fatalf("qualified checkpoint failed: %+v", checkpoint)
	}
	submit := tools.Call(ctx, "artifact_submit", "r0-5b7-submit", []byte(`{}`))
	if submit.Error != "" || submit.Receipt == nil || submit.Receipt.Status != "candidate" {
		t.Fatalf("artifact submit failed: %+v", submit)
	}
	store, err := workbench.NewPostgresReadStore(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	overview, err := store.GetCompanyOverview(ctx, companyID)
	if err != nil {
		t.Fatal(err)
	}
	var projected workbench.TaskSummary
	for _, item := range overview.Tasks {
		if item.TaskID == task.ID {
			projected = item
		}
	}
	if projected.State != "candidate" || projected.Acceptance != "candidate" || len(overview.Artifacts) != 1 || overview.Artifacts[0].ArtifactID != submit.Receipt.ID {
		t.Fatalf("Workbench projection lost the published lifecycle: task=%+v artifacts=%+v", projected, overview.Artifacts)
	}
	if len(overview.Checkpoints) != 1 {
		t.Fatalf("Workbench projected %d current checkpoints, want one: %+v", len(overview.Checkpoints), overview.Checkpoints)
	}
	selected := overview.Checkpoints[0]
	if selected.TaskID != task.ID || selected.Kind != "qualified" || selected.ArtifactID == nil || *selected.ArtifactID != submit.Receipt.ID {
		t.Fatalf("current checkpoint is not bound to the published Artifact: checkpoint=%+v artifact=%s", selected, submit.Receipt.ID)
	}
	if overview.Artifacts[0].CheckpointID == nil || *overview.Artifacts[0].CheckpointID != selected.CheckpointID {
		t.Fatalf("Workbench artifact/checkpoint relation is incoherent: artifact=%+v selected=%+v", overview.Artifacts[0], selected)
	}
}

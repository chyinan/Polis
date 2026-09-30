// pattern: Imperative Shell
package kernel

import (
	"context"
	"os"
	"runtime"
	"testing"

	"polis/internal/core"
	"polis/internal/intake"
	"polis/internal/runner"
	"polis/internal/taskvalidation"
)

func TestTakeoverLeaseFreezesWorkspaceAndReturnsHumanSnapshotToSuccessor(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("the owned process fixture requires Linux")
	}
	dsn := os.Getenv("POLIS_TEST_DSN")
	if dsn == "" {
		t.Skip("dedicated PostgreSQL required")
	}
	ctx := context.Background()
	k, err := Open(ctx, dsn, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer k.Close()
	scope, err := k.TXCreateCompany(ctx, "takeover-lease-"+newID())
	if err != nil {
		t.Fatal(err)
	}
	contract := &taskvalidation.AcceptanceContract{Revision: taskvalidation.AcceptanceContractRevision, RequiredText: []string{"Task summary:", "Implementation:", "Validation:"}}
	mission, err := k.TXCreateMissionGoalWithAcceptance(ctx, scope, "Human handover", "Preserve the approved implementation boundary.", contract, "takeover-mission-create-"+newID())
	if err != nil {
		t.Fatal(err)
	}
	missionDetails, err := k.MissionDetails(ctx, scope, mission.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = k.TXStartMissionCommand(ctx, scope, mission.ID, "takeover-mission-start-"+newID()); err != nil {
		t.Fatal(err)
	}
	inputBytes := []byte("# Requirements\nKeep the API response backwards compatible.\n")
	prepared, err := intake.PrepareUpload("requirements.md", "text/markdown", inputBytes)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = k.TXAddMissionInput(ctx, scope, mission.ID, "", "takeover-input-"+newID(), prepared, inputBytes); err != nil {
		t.Fatal(err)
	}
	task, err := k.TXPrepareProductTask(ctx, scope, mission.ID, missionDetails.Goal, "takeover-task-"+newID())
	if err != nil {
		t.Fatal(err)
	}
	worker, err := k.TXNewProductProviderWorkerWithToolBudget(ctx, scope, task.ID, "gpt-5.6-sol/medium", 4)
	if err != nil {
		t.Fatal(err)
	}
	process, err := runner.Start(worker.SessionID(), []string{"/bin/sleep", "120"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer process.Stop()
	if err = k.TXAttachWorker(ctx, worker, process); err != nil {
		t.Fatal(err)
	}
	if err = k.TXValidateWorker(ctx, worker); err != nil {
		t.Fatal(err)
	}
	if err = k.TXActivateWorker(ctx, worker, "gpt-5.6-sol/medium"); err != nil {
		t.Fatal(err)
	}
	if _, err = k.TXSetMissionPausedCommand(ctx, scope, mission.ID, true, "takeover-mission-pause-"+newID()); err != nil {
		t.Fatal(err)
	}
	if _, err = k.TXCreateTaskTakeoverLease(ctx, scope, mission.ID, task.ID, "takeover-while-worker-live-"+newID()); err == nil {
		t.Fatal("human takeover lease was granted before the live WorkerSession stopped")
	}
	if err = k.TXBeginStop(ctx, worker); err != nil {
		t.Fatal(err)
	}
	stopProof, err := process.Stop()
	if err != nil {
		t.Fatal(err)
	}
	if err = k.TXConfirmStopped(ctx, worker, stopProof); err != nil {
		t.Fatal(err)
	}
	lease, err := k.TXCreateTaskTakeoverLease(ctx, scope, mission.ID, task.ID, "takeover-lease-create-"+newID())
	if err != nil {
		t.Fatal(err)
	}
	if lease.State != "granted" || lease.BaseWorkspaceDigest == "" || lease.BaseWorkspaceRevision <= 0 || lease.TaskID != task.ID {
		t.Fatalf("takeover lease did not freeze the stopped workspace: %+v", lease)
	}
	if _, err = k.TXSubmitTaskTakeoverSnapshot(ctx, scope, mission.ID, lease.LeaseID, TaskTakeoverSnapshotInput{
		RequestID: "takeover-stale-" + newID(), BaseWorkspaceDigest: repeatDigest('f'), BaseWorkspaceRevision: lease.BaseWorkspaceRevision + 1,
		Content: "Human baseline must match the lease revision.", HumanEffortSeconds: 90,
	}); err != core.Conflict {
		t.Fatalf("stale human snapshot error=%v, want conflict", err)
	}

	humanContent := "# Human handover\nPreserve the existing response format and add the empty-input note.\n"
	returned, err := k.TXSubmitTaskTakeoverSnapshot(ctx, scope, mission.ID, lease.LeaseID, TaskTakeoverSnapshotInput{
		RequestID: "takeover-submit-" + newID(), BaseWorkspaceDigest: lease.BaseWorkspaceDigest, BaseWorkspaceRevision: lease.BaseWorkspaceRevision,
		Content: humanContent, HumanEffortSeconds: 90,
	})
	if err != nil {
		t.Fatal(err)
	}
	if returned.State != "returned" || returned.SnapshotInputID == nil || returned.SnapshotRevision == nil || returned.SnapshotDigest == nil || *returned.SnapshotDigest == "" || returned.HumanEffortSeconds == nil || *returned.HumanEffortSeconds != 90 {
		t.Fatalf("submitted human snapshot was not tied to a returned lease: %+v", returned)
	}
	var currentDigest string
	var currentRevision int64
	if err = k.pool.QueryRow(ctx, "SELECT digest,revision FROM worker_workspaces WHERE company_id=$1 AND task_id=$2", scope.company, task.ID).Scan(&currentDigest, &currentRevision); err != nil {
		t.Fatal(err)
	}
	if currentDigest != lease.BaseWorkspaceDigest || currentRevision != lease.BaseWorkspaceRevision {
		t.Fatalf("human handback wrote the stopped Worker workspace in place: digest=%s revision=%d", currentDigest, currentRevision)
	}
	change, err := k.TXCreateMissionChangeRequest(ctx, scope, mission.ID, MissionChangeRequestInput{
		ChangeSummary: "Apply the returned human workspace baseline.", ProposedAcceptanceContract: contract, BlockPreviousResults: true,
	}, "takeover-change-"+newID())
	if err != nil {
		t.Fatal(err)
	}
	considered, err := k.TXConsiderMissionChangeRequest(ctx, scope, mission.ID, change.ID, "takeover-change-consider-"+newID())
	if err != nil {
		t.Fatal(err)
	}
	applied, err := k.TXApplyMissionChangeRequest(ctx, scope, mission.ID, change.ID, considered.ImpactSHA256, "takeover-change-apply-"+newID())
	if err != nil {
		t.Fatal(err)
	}
	var mappedHandover bool
	for _, item := range applied.InputRevisionMap {
		mappedHandover = mappedHandover || (item.Origin == "human_takeover" && item.SourceTaskID == task.ID && item.ContentDigest == *returned.SnapshotDigest)
	}
	if !mappedHandover {
		t.Fatalf("successor Mission omitted human takeover provenance: %+v", applied.InputRevisionMap)
	}
	if _, err = k.TXStartMissionCommand(ctx, scope, *applied.SuccessorMissionID, "takeover-successor-start-"+newID()); err != nil {
		t.Fatal(err)
	}
	successorTask, err := k.TXPrepareProductTask(ctx, scope, *applied.SuccessorMissionID, missionDetails.Goal, "takeover-successor-task-"+newID())
	if err != nil {
		t.Fatal(err)
	}
	manifest, err := k.TaskInputManifest(ctx, scope, successorTask.ID)
	if err != nil {
		t.Fatal(err)
	}
	var humanSnapshotDelivered bool
	for _, input := range manifest.Manifest.CandidateInputs {
		humanSnapshotDelivered = humanSnapshotDelivered || input.ContentDigest == *returned.SnapshotDigest
	}
	if !humanSnapshotDelivered {
		t.Fatalf("returned human snapshot did not reach successor Task input manifest: %+v", manifest.Manifest.CandidateInputs)
	}
}

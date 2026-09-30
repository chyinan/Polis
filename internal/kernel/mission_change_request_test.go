// pattern: Imperative Shell
package kernel

import (
	"context"
	"os"
	"runtime"
	"strings"
	"testing"

	"polis/internal/core"
	"polis/internal/intake"
	"polis/internal/runner"
	"polis/internal/taskvalidation"
)

func TestMissionChangeRequestAppliesAsSuccessorOnlyAfterStoppedBoundary(t *testing.T) {
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
	scope, err := k.TXCreateCompany(ctx, "change-request-"+newID())
	if err != nil {
		t.Fatal(err)
	}
	baseContract := &taskvalidation.AcceptanceContract{Revision: taskvalidation.AcceptanceContractRevision, RequiredText: []string{"Task summary:", "Implementation:", "Validation:"}}
	created, err := k.TXCreateMissionGoalWithAcceptance(ctx, scope, "Change request source", "Return a non-empty list of records.", baseContract, "change-source-create-"+newID())
	if err != nil {
		t.Fatal(err)
	}
	if _, err = k.TXStartMissionCommand(ctx, scope, created.ID, "change-source-start-"+newID()); err != nil {
		t.Fatal(err)
	}
	inputBytes := []byte("# Requirements v1\nReturn records sorted by creation time.\n")
	preparedInput, err := intake.PrepareUpload("requirements.md", "text/markdown", inputBytes)
	if err != nil {
		t.Fatal(err)
	}
	sourceInput, err := k.TXAddMissionInput(ctx, scope, created.ID, "", "change-source-input-"+newID(), preparedInput, inputBytes)
	if err != nil {
		t.Fatal(err)
	}
	task, err := k.TXPrepareProductTask(ctx, scope, created.ID, "Return a non-empty list of records.", "change-source-task-"+newID())
	if err != nil {
		t.Fatal(err)
	}
	artifactID := "change-artifact-" + newID()
	if _, err = k.pool.Exec(ctx, `INSERT INTO artifacts(company_id,id,task_id,author,digest,bytes,state,verdict,contract)
VALUES($1,$2,$3,$4,$5,10,'ready','candidate',$6)`, scope.company, artifactID, task.ID, task.Owner, repeatDigest('a'), core.Contract); err != nil {
		t.Fatal(err)
	}
	binding, err := k.TXNewProductProviderWorkerWithToolBudget(ctx, scope, task.ID, "gpt-5.6-sol/medium", 4)
	if err != nil {
		t.Fatal(err)
	}
	process, err := runner.Start(binding.SessionID(), []string{"/bin/sleep", "120"}, nil)
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
	if err = k.TXActivateWorker(ctx, binding, "gpt-5.6-sol/medium"); err != nil {
		t.Fatal(err)
	}

	proposedContract := &taskvalidation.AcceptanceContract{Revision: taskvalidation.AcceptanceContractRevision, RequiredText: []string{"Task summary:", "Empty results:", "Validation:"}}
	change, err := k.TXCreateMissionChangeRequest(ctx, scope, created.ID, MissionChangeRequestInput{
		ChangeSummary: "Define the empty-dataset result explicitly.", ProposedGoal: "Return an empty list when the dataset has no records.",
		ProposedAcceptanceContract: proposedContract, BlockPreviousResults: true,
	}, "change-create-"+newID())
	if err != nil {
		t.Fatal(err)
	}
	var impactedProductTask bool
	for _, impactedTask := range change.Impact.Tasks {
		impactedProductTask = impactedProductTask || impactedTask.TaskID == task.ID
	}
	if change.State != "queued" || change.BaseRequirementsSHA256 == "" || change.ImpactSHA256 == "" || len(change.Impact.Tasks) != 2 || !impactedProductTask || len(change.Impact.Artifacts) != 1 || change.Impact.Artifacts[0].ArtifactID != artifactID {
		t.Fatalf("change request did not freeze its base and explicit impact: %+v", change)
	}
	var outputBlockCount int
	if err = k.pool.QueryRow(ctx, "SELECT count(*) FROM mission_change_request_output_blocks WHERE company_id=$1 AND change_request_id=$2 AND artifact_id=$3", scope.company, change.ID, artifactID).Scan(&outputBlockCount); err != nil || outputBlockCount != 1 {
		t.Fatalf("previous result block=(%d,%v), want one block", outputBlockCount, err)
	}
	if _, err = k.TXCreateMissionChangeRequest(ctx, scope, created.ID, MissionChangeRequestInput{ChangeSummary: "A second concurrent Mission change."}, "change-create-overlap-"+newID()); err == nil {
		t.Fatal("a second open formal change request was accepted for the same Mission")
	}
	if _, err = k.TXSetMissionPausedCommand(ctx, scope, created.ID, true, "change-source-pause-"+newID()); err != nil {
		t.Fatal(err)
	}
	if _, err = k.TXConsiderMissionChangeRequest(ctx, scope, created.ID, change.ID, "change-consider-live-writer-"+newID()); err == nil {
		t.Fatal("change request was considered while its prior WorkerSession still owned the Task")
	}
	if err = k.TXBeginStop(ctx, binding); err != nil {
		t.Fatal(err)
	}
	stopProof, err := process.Stop()
	if err != nil {
		t.Fatal(err)
	}
	if err = k.TXConfirmStopped(ctx, binding, stopProof); err != nil {
		t.Fatal(err)
	}
	considered, err := k.TXConsiderMissionChangeRequest(ctx, scope, created.ID, change.ID, "change-consider-"+newID())
	if err != nil {
		t.Fatal(err)
	}
	if considered.State != "considered" || len(considered.Impact.ActiveWorkerSessions) != 0 || considered.ImpactSHA256 == "" {
		t.Fatalf("stopped boundary was not recorded: %+v", considered)
	}
	if _, err = k.TXApplyMissionChangeRequest(ctx, scope, created.ID, change.ID, repeatDigest('f'), "change-apply-stale-"+newID()); err == nil {
		t.Fatal("a formal change with an unreviewed impact digest was applied")
	}
	applied, err := k.TXApplyMissionChangeRequest(ctx, scope, created.ID, change.ID, considered.ImpactSHA256, "change-apply-"+newID())
	if err != nil {
		t.Fatal(err)
	}
	if applied.State != "applied" || applied.SuccessorMissionID == nil || *applied.SuccessorMissionID == "" || *applied.SuccessorMissionID == created.ID {
		t.Fatalf("formal change did not create a successor Mission: %+v", applied)
	}
	var handoverSnapshotCopied bool
	for _, item := range applied.InputRevisionMap {
		handoverSnapshotCopied = handoverSnapshotCopied || (item.Origin == "task_workspace" && item.SourceTaskID == task.ID && item.ContentDigest == binding.WorkspaceDigest())
	}
	if len(applied.InputRevisionMap) != 2 || !handoverSnapshotCopied {
		t.Fatalf("successor input map did not preserve both source input and stopped workspace: %+v", applied.InputRevisionMap)
	}
	successorMissionID := *applied.SuccessorMissionID
	oldMission, err := k.MissionDetails(ctx, scope, created.ID)
	if err != nil || oldMission.State != "cancelled" {
		t.Fatalf("old Mission state=(%q,%v), want cancelled", oldMission.State, err)
	}
	oldTask, err := k.taskByID(ctx, scope, task.ID)
	if err != nil || oldTask.State != "cancelled" {
		t.Fatalf("old Task state=(%q,%v), want cancelled", oldTask.State, err)
	}
	successor, err := k.MissionDetails(ctx, scope, successorMissionID)
	if err != nil || successor.State != "draft" || successor.Goal != "Return an empty list when the dataset has no records." || successor.AcceptanceContract == nil || successor.AcceptanceContract.RequiredText[1] != "Empty results:" {
		t.Fatalf("successor Mission did not receive the approved requirement revision: %+v err=%v", successor, err)
	}
	var clonedInputID string
	var clonedRevision int64
	if err = k.pool.QueryRow(ctx, `SELECT input_id,revision FROM mission_inputs WHERE company_id=$1 AND mission_id=$2 AND content_digest=$3`, scope.company, successorMissionID, preparedInput.ContentDigest).Scan(&clonedInputID, &clonedRevision); err != nil {
		t.Fatal(err)
	}
	if clonedInputID == "" || clonedInputID == sourceInput.InputID || clonedRevision != 1 {
		t.Fatalf("successor input revision=(%q,%d), want a new immutable reference to the same bytes", clonedInputID, clonedRevision)
	}
	if _, err = k.TXStartMissionCommand(ctx, scope, successorMissionID, "change-successor-start-"+newID()); err != nil {
		t.Fatal(err)
	}
	newTask, err := k.TXPrepareProductTask(ctx, scope, successorMissionID, successor.Goal, "change-successor-task-"+newID())
	if err != nil {
		t.Fatal(err)
	}
	manifest, err := k.TaskInputManifest(ctx, scope, newTask.ID)
	var preservedRequirementInput, preservedWorkspaceInput bool
	for _, candidate := range manifest.Manifest.CandidateInputs {
		preservedRequirementInput = preservedRequirementInput || candidate.ContentDigest == preparedInput.ContentDigest
		preservedWorkspaceInput = preservedWorkspaceInput || candidate.ContentDigest == binding.WorkspaceDigest()
	}
	if err != nil || len(manifest.Manifest.CandidateInputs) != 2 || !preservedRequirementInput || !preservedWorkspaceInput {
		t.Fatalf("successor Task did not freeze the cloned input bytes: %+v err=%v", manifest, err)
	}
	var eventCount int
	if err = k.pool.QueryRow(ctx, `SELECT count(*) FROM mission_change_request_events WHERE company_id=$1 AND change_request_id=$2 AND state IN ('received','queued','considered','applied')`, scope.company, change.ID).Scan(&eventCount); err != nil || eventCount < 4 {
		t.Fatalf("change request decision history count=(%d,%v), want at least 4", eventCount, err)
	}
}

func repeatDigest(character byte) string {
	return strings.Repeat(string(character), 64)
}

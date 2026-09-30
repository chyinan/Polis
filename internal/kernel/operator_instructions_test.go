// pattern: Imperative Shell
package kernel

import (
	"context"
	"encoding/json"
	"os"
	"runtime"
	"strings"
	"testing"

	"polis/internal/core"
	"polis/internal/runner"
)

func TestOperatorInstructionIsPersistedForActiveMission(t *testing.T) {
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
	scope, err := k.TXCreateCompany(ctx, "r06-instruction-company")
	if err != nil {
		t.Fatal(err)
	}
	task, err := k.TXCreateProbe(ctx, scope, "mission-1")
	if err != nil {
		t.Fatal(err)
	}
	instruction, err := k.TXCreateOperatorInstruction(ctx, scope, OperatorInstructionInput{MissionID: task.Mission, EmployeeID: "emp-backend", Content: "Continue with the current validation feedback."}, "instruction-1")
	if err != nil {
		t.Fatal(err)
	}
	if instruction.State != "pending" || instruction.Content == "" {
		t.Fatalf("unexpected instruction: %+v", instruction)
	}
}

func TestOperatorGuidanceIsReadAndAnsweredByBoundWorker(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("the process fixture requires Linux")
	}
	dsn, goRoot := os.Getenv("POLIS_TEST_DSN"), os.Getenv("POLIS_GO_ROOT")
	if dsn == "" || goRoot == "" {
		t.Skip("dedicated PostgreSQL 18 and compiler required")
	}
	ctx := context.Background()
	k, err := Open(ctx, dsn, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer k.Close()
	scope, err := k.TXCreateCompany(ctx, "r1-guidance-"+newID())
	if err != nil {
		t.Fatal(err)
	}
	task, err := k.TXCreateProbe(ctx, scope, "guidance-mission-"+newID())
	if err != nil {
		t.Fatal(err)
	}
	binding, err := k.TXNewWorker(ctx, scope, task.ID, "gpt-5.6-sol/medium")
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
	if err = k.TXActivateWorker(ctx, binding, "sandbox-measured"); err != nil {
		t.Fatal(err)
	}
	_, err = k.TXCreateOperatorInstruction(ctx, scope, OperatorInstructionInput{MissionID: task.Mission, TaskID: task.ID, EmployeeID: binding.EmployeeID(), Content: "Keep the approved scope; prioritize the latest failing check."}, "guidance-create-"+newID())
	if err != nil {
		t.Fatal(err)
	}
	if _, err = k.TXCreateOperatorInstruction(ctx, scope, OperatorInstructionInput{MissionID: task.Mission, TaskID: task.ID, EmployeeID: "emp-frontend", Content: "This task is owned by another employee."}, "guidance-mismatched-target-"+newID()); err != core.OutOfScope {
		t.Fatalf("incompatible Employee/Task instruction target was accepted: %v", err)
	}
	if _, err = k.pool.Exec(ctx, "UPDATE employees SET enabled=false WHERE company_id=$1 AND id=$2", scope.company, binding.EmployeeID()); err != nil {
		t.Fatal(err)
	}
	if _, err = k.TXCreateOperatorInstruction(ctx, scope, OperatorInstructionInput{MissionID: task.Mission, TaskID: task.ID, Content: "The task owner is disabled."}, "guidance-disabled-owner-"+newID()); err != core.OutOfScope {
		t.Fatalf("task guidance was created for a disabled employee: %v", err)
	}
	if _, err = k.pool.Exec(ctx, "UPDATE employees SET enabled=true WHERE company_id=$1 AND id=$2", scope.company, binding.EmployeeID()); err != nil {
		t.Fatal(err)
	}
	if _, err = k.TXCreateOperatorInstruction(ctx, scope, OperatorInstructionInput{MissionID: task.Mission, EmployeeID: "emp-frontend", Content: "No task is assigned to this employee in the mission."}, "guidance-unassigned-employee-"+newID()); err != core.OutOfScope {
		t.Fatalf("mission guidance was created for an employee without a task in the mission: %v", err)
	}
	otherEmployeeInstruction, err := k.TXCreateOperatorInstruction(ctx, scope, OperatorInstructionInput{EmployeeID: "emp-frontend", Content: "Use the company-wide implementation note."}, "guidance-other-employee-"+newID())
	if err != nil {
		t.Fatal(err)
	}
	companyInstruction, err := k.TXCreateOperatorInstruction(ctx, scope, OperatorInstructionInput{Content: "Keep company work within its approved scope."}, "guidance-company-"+newID())
	if err != nil {
		t.Fatal(err)
	}
	baseTools := EmployeeTools{Kernel: k, Binding: binding, Checker: runner.Verifier{GoRoot: goRoot, Scratch: t.TempDir()}, Phase: "single", ProductSurface: true}
	if result := baseTools.Call(ctx, "guidance_read", "guidance-read-disabled-"+newID(), []byte(`{}`)); result.Error != core.Denied.Error() {
		t.Fatalf("guidance escaped its separately versioned surface: %+v", result)
	}
	tools := baseTools
	tools.GuidanceSurface = true
	if result := tools.Call(ctx, "guidance_read", "guidance-read-spoof-"+newID(), []byte(`{"employee_id":"emp-frontend"}`)); result.Error != core.Malformed.Error() {
		t.Fatalf("model-supplied guidance identity was accepted: %+v", result)
	}
	read := tools.Call(ctx, "guidance_read", "guidance-read-"+newID(), []byte(`{}`))
	if read.Error != "" {
		t.Fatalf("guidance read failed: %+v", read)
	}
	var inbox OperatorGuidanceInbox
	body, err := json.Marshal(read.Data)
	if err != nil || json.Unmarshal(body, &inbox) != nil || len(inbox.Items) != 2 {
		t.Fatalf("bound guidance inbox=%s, err=%v", body, err)
	}
	var targetedInstructionID string
	var companyInstructionVisible bool
	for _, item := range inbox.Items {
		if item.EmployeeID == binding.EmployeeID() && item.TaskID == task.ID && item.State == "pending" {
			targetedInstructionID = item.ID
		}
		if item.ID == companyInstruction.ID {
			companyInstructionVisible = true
		}
		if item.ID == otherEmployeeInstruction.ID {
			t.Fatalf("instruction targeted to another employee was visible: %+v", item)
		}
	}
	if targetedInstructionID == "" || !companyInstructionVisible || companyInstruction.MissionID != nil {
		t.Fatalf("scoped inbox omitted the employee or company instruction: %+v", inbox.Items)
	}
	boundedSummary := strings.Repeat("😀", 128)
	args, err := json.Marshal(map[string]string{"instruction_id": targetedInstructionID, "outcome": "applied", "summary": boundedSummary})
	if err != nil {
		t.Fatal(err)
	}
	spoofedArgs := []byte(`{"instruction_id":"` + targetedInstructionID + `","outcome":"applied","summary":"spoof","employee_id":"emp-frontend"}`)
	if result := tools.Call(ctx, "guidance_respond", "guidance-respond-spoof-"+newID(), spoofedArgs); result.Error != core.Malformed.Error() {
		t.Fatalf("model-supplied response identity was accepted: %+v", result)
	}
	callID := "guidance-respond-" + newID()
	response := tools.Call(ctx, "guidance_respond", callID, args)
	if response.Error != "" || response.Receipt == nil || response.Receipt.Status != "applied" {
		t.Fatalf("guidance response=%+v", response)
	}
	replay := tools.Call(ctx, "guidance_respond", callID, args)
	if replay.Error != "" || replay.Receipt == nil || replay.Receipt.ID != response.Receipt.ID {
		t.Fatalf("guidance response replay=%+v first=%+v", replay, response)
	}
	otherResponse, err := k.TXRespondToOperatorInstruction(ctx, binding, OperatorInstructionResponseInput{
		InstructionID: otherEmployeeInstruction.ID, Outcome: OperatorInstructionApplied, Summary: "Cross-employee response must be denied.",
	}, "guidance-cross-employee-"+newID())
	if err != core.OutOfScope || otherResponse.ID != "" {
		t.Fatalf("cross-employee guidance response=(%+v,%v), want out of scope", otherResponse, err)
	}
	duplicateResponse, duplicateErr := k.TXRespondToOperatorInstruction(ctx, binding, OperatorInstructionResponseInput{
		InstructionID: targetedInstructionID, Outcome: OperatorInstructionRejected, Summary: "A second response must conflict.",
	}, "guidance-second-response-"+newID())
	if duplicateErr == nil || duplicateResponse.ID != "" {
		t.Fatalf("second guidance response=(%+v,%v), want conflict", duplicateResponse, duplicateErr)
	}
	remaining := tools.Call(ctx, "guidance_read", "guidance-read-after-response-"+newID(), []byte(`{}`))
	remainingJSON, err := json.Marshal(remaining.Data)
	var pending OperatorGuidanceInbox
	if err != nil || json.Unmarshal(remainingJSON, &pending) != nil || len(pending.Items) != 1 || pending.Items[0].ID != companyInstruction.ID {
		t.Fatalf("answered guidance remained in inbox=%s err=%v", remainingJSON, err)
	}
	var state, summary string
	if err = k.pool.QueryRow(ctx, `SELECT i.state,r.summary FROM operator_instructions i JOIN operator_instruction_responses r ON r.company_id=i.company_id AND r.instruction_id=i.id WHERE i.company_id=$1 AND i.id=$2`, scope.company, targetedInstructionID).Scan(&state, &summary); err != nil {
		t.Fatal(err)
	}
	if state != "applied" || summary != boundedSummary {
		t.Fatalf("persisted guidance response=(%s,%q)", state, summary)
	}
	if _, err = k.pool.Exec(ctx, `UPDATE operator_instruction_responses SET summary='rewritten' WHERE company_id=$1 AND instruction_id=$2`, scope.company, targetedInstructionID); err == nil {
		t.Fatal("operator instruction response history was mutable")
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
	if _, err = k.TXCancelMission(ctx, scope, task.Mission, "guidance-cancel-"+newID()); err != nil {
		t.Fatal(err)
	}
	nextTask, err := k.TXCreateProbe(ctx, scope, "guidance-next-mission-"+newID())
	if err != nil {
		t.Fatal(err)
	}
	nextBinding, err := k.TXNewWorker(ctx, scope, nextTask.ID, "gpt-5.6-sol/medium")
	if err != nil {
		t.Fatal(err)
	}
	nextProcess, err := runner.Start(nextBinding.SessionID(), []string{"/bin/sleep", "120"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer nextProcess.Stop()
	if err = k.TXAttachWorker(ctx, nextBinding, nextProcess); err != nil {
		t.Fatal(err)
	}
	if err = k.TXValidateWorker(ctx, nextBinding); err != nil {
		t.Fatal(err)
	}
	if err = k.TXActivateWorker(ctx, nextBinding, "sandbox-measured-next-mission"); err != nil {
		t.Fatal(err)
	}
	nextInbox, err := k.ReadPendingOperatorInstructions(ctx, nextBinding)
	if err != nil || len(nextInbox.Items) != 1 || nextInbox.Items[0].ID != companyInstruction.ID || nextInbox.Items[0].MissionID != nil {
		t.Fatalf("company-wide guidance did not survive a Mission change or old Mission guidance leaked: inbox=%+v err=%v", nextInbox, err)
	}
	if _, err = k.TXRespondToOperatorInstruction(ctx, nextBinding, OperatorInstructionResponseInput{
		InstructionID: companyInstruction.ID, Outcome: OperatorInstructionApplied, Summary: "Applied in the second Mission.",
	}, "guidance-company-response-backend-"+newID()); err != nil {
		t.Fatal(err)
	}
	if err = k.TXBeginStop(ctx, nextBinding); err != nil {
		t.Fatal(err)
	}
	nextProof, err := nextProcess.Stop()
	if err != nil {
		t.Fatal(err)
	}
	if err = k.TXConfirmStopped(ctx, nextBinding, nextProof); err != nil {
		t.Fatal(err)
	}
	if _, err = k.TXCancelMission(ctx, scope, nextTask.Mission, "guidance-cancel-next-"+newID()); err != nil {
		t.Fatal(err)
	}
	peerFixture, err := k.TXCreatePeerFixture(ctx, scope, "guidance-frontend-mission-"+newID())
	if err != nil {
		t.Fatal(err)
	}
	frontendBinding, err := k.TXNewWorker(ctx, scope, peerFixture.Frontend.ID, "gpt-5.6-sol/medium")
	if err != nil {
		t.Fatal(err)
	}
	frontendProcess, err := runner.Start(frontendBinding.SessionID(), []string{"/bin/sleep", "120"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer frontendProcess.Stop()
	if err = k.TXAttachWorker(ctx, frontendBinding, frontendProcess); err != nil {
		t.Fatal(err)
	}
	if err = k.TXValidateWorker(ctx, frontendBinding); err != nil {
		t.Fatal(err)
	}
	if err = k.TXActivateWorker(ctx, frontendBinding, "sandbox-measured-frontend-mission"); err != nil {
		t.Fatal(err)
	}
	frontendInbox, err := k.ReadPendingOperatorInstructions(ctx, frontendBinding)
	if err != nil || len(frontendInbox.Items) != 2 {
		t.Fatalf("company-wide guidance was not independently delivered to the next Employee: inbox=%+v err=%v", frontendInbox, err)
	}
	var frontendTargetedGuidanceVisible, companyGuidanceVisible bool
	for _, item := range frontendInbox.Items {
		frontendTargetedGuidanceVisible = frontendTargetedGuidanceVisible || item.ID == otherEmployeeInstruction.ID
		companyGuidanceVisible = companyGuidanceVisible || item.ID == companyInstruction.ID
	}
	if !frontendTargetedGuidanceVisible || !companyGuidanceVisible {
		t.Fatalf("employee-targeted or broadcast company guidance missing from frontend inbox: %+v", frontendInbox.Items)
	}
	if _, err = k.TXRespondToOperatorInstruction(ctx, frontendBinding, OperatorInstructionResponseInput{
		InstructionID: otherEmployeeInstruction.ID, Outcome: OperatorInstructionApplied, Summary: "Applied only for this Employee.",
	}, "guidance-employee-company-response-frontend-"+newID()); err != nil {
		t.Fatal(err)
	}
	if _, err = k.TXRespondToOperatorInstruction(ctx, frontendBinding, OperatorInstructionResponseInput{
		InstructionID: companyInstruction.ID, Outcome: OperatorInstructionNeedsClarification, Summary: "Need the boundary for the approved scope clarified.",
	}, "guidance-company-response-frontend-"+newID()); err != nil {
		t.Fatal(err)
	}
	var companyGuidanceState string
	var responseCount int
	if err = k.pool.QueryRow(ctx, `SELECT i.state,count(r.response_id) FROM operator_instructions i JOIN operator_instruction_responses r ON r.company_id=i.company_id AND r.instruction_id=i.id WHERE i.company_id=$1 AND i.id=$2 GROUP BY i.state`, scope.company, companyInstruction.ID).Scan(&companyGuidanceState, &responseCount); err != nil {
		t.Fatal(err)
	}
	if companyGuidanceState != "pending" || responseCount != 2 {
		t.Fatalf("company-wide guidance closed before every recipient responded: state=%s responses=%d", companyGuidanceState, responseCount)
	}
}

// pattern: Imperative Shell
package workbench

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"testing"
	"time"

	"polis/internal/control"
	"polis/internal/kernel"
	"polis/internal/runner"
)

func TestOperatorInstructionResponseIsVisibleInWorkbench(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("the process fixture requires Linux")
	}
	dsn := os.Getenv("POLIS_TEST_DSN")
	if dsn == "" {
		t.Skip("dedicated PostgreSQL 18 required")
	}
	ctx := context.Background()
	k, err := kernel.Open(ctx, dsn, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer k.Close()
	companyID := fmt.Sprintf("r1-guidance-view-%d", time.Now().UnixNano())
	company, err := k.TXCreateCompany(ctx, companyID)
	if err != nil {
		t.Fatal(err)
	}
	task, err := k.TXCreateProbe(ctx, company, fmt.Sprintf("guidance-mission-%d", time.Now().UnixNano()))
	if err != nil {
		t.Fatal(err)
	}
	binding, err := k.TXNewWorker(ctx, company, task.ID, "gpt-5.6-sol/medium")
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
	instruction, err := control.NewService(k, nil).CreateOperatorInstruction(ctx, companyID, control.CreateOperatorInstructionRequest{
		Content:   "Keep the approved scope and prioritize the latest failing check.",
		RequestID: fmt.Sprintf("guidance-create-%d", time.Now().UnixNano()),
	})
	if err != nil {
		t.Fatal(err)
	}
	if !instruction.Accepted || instruction.CreatedAt == "" || instruction.MissionID != nil || instruction.TaskID != nil || instruction.EmployeeID != nil || instruction.ResponseOutcome != nil || instruction.ResponseSummary != nil || len(instruction.Responses) != 0 {
		t.Fatalf("operator guidance command returned an invalid receipt: %+v", instruction)
	}
	_, err = k.TXRespondToOperatorInstruction(ctx, binding, kernel.OperatorInstructionResponseInput{
		InstructionID: instruction.InstructionID, Outcome: kernel.OperatorInstructionNeedsClarification, Summary: "The referenced check is not available in the current environment.",
	}, fmt.Sprintf("guidance-respond-%d", time.Now().UnixNano()))
	if err != nil {
		t.Fatal(err)
	}
	store, err := NewPostgresReadStore(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	instructions, err := store.ListOperatorInstructions(ctx, companyID)
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range instructions {
		if item.InstructionID != instruction.InstructionID {
			continue
		}
		if item.MissionID != nil || item.State != "pending" || item.ResponseOutcome == nil || *item.ResponseOutcome != "needs_clarification" || item.ResponseSummary == nil || *item.ResponseSummary != "The referenced check is not available in the current environment." || item.RespondedByEmployeeID == nil || *item.RespondedByEmployeeID != binding.EmployeeID() || item.RespondedAt == nil || len(item.Responses) != 1 || item.Responses[0].Outcome != "needs_clarification" {
			t.Fatalf("Workbench projected an incomplete guidance response: %+v", item)
		}
		return
	}
	t.Fatalf("Workbench omitted persisted instruction response %s", instruction.InstructionID)
}

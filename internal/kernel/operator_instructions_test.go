// pattern: Imperative Shell
package kernel

import (
	"context"
	"os"
	"testing"
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
	if err = k.TXCreateMission(ctx, scope, "mission-1"); err != nil {
		t.Fatal(err)
	}
	if _, err = k.TXStartMissionCommand(ctx, scope, "mission-1", "start-1"); err != nil {
		t.Fatal(err)
	}
	instruction, err := k.TXCreateOperatorInstruction(ctx, scope, OperatorInstructionInput{MissionID: "mission-1", EmployeeID: "emp-backend", Content: "Continue with the current validation feedback."}, "instruction-1")
	if err != nil {
		t.Fatal(err)
	}
	if instruction.State != "pending" || instruction.Content == "" {
		t.Fatalf("unexpected instruction: %+v", instruction)
	}
}

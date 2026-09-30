// pattern: Imperative Shell
package workbench

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"polis/internal/core"
	"polis/internal/kernel"
)

func TestPostgresReadStoreProjectsEmployeeSchedule(t *testing.T) {
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
	companyID := fmt.Sprintf("employee-schedule-view-%d", time.Now().UnixNano())
	scope, err := k.TXCreateCompany(ctx, companyID)
	if err != nil {
		t.Fatal(err)
	}
	mission, err := k.TXCreateMissionGoal(ctx, scope, "Schedule view", "project persisted schedule facts", "schedule-view-mission")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = k.TXStartMissionCommand(ctx, scope, mission.ID, "schedule-view-start"); err != nil {
		t.Fatal(err)
	}
	store, err := NewPostgresReadStore(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	overview, err := store.GetCompanyOverview(ctx, companyID)
	if err != nil {
		t.Fatal(err)
	}
	for _, employee := range overview.Employees {
		if employee.EmployeeID != "emp-planning" {
			continue
		}
		if employee.Schedule == nil || employee.Schedule.State != "wake_pending" || employee.Schedule.WorkGeneration != "1" || employee.Schedule.CheckedGeneration != "0" {
			t.Fatalf("Workbench schedule=%+v, want wake_pending at generation 1/0", employee.Schedule)
		}
		return
	}
	t.Fatal("Workbench overview omitted the fixed planning employee")
}

func TestPostgresReadStoreProjectsMissionDailyRoutines(t *testing.T) {
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
	companyID := fmt.Sprintf("daily-routine-view-%d", time.Now().UnixNano())
	scope, err := k.TXCreateCompany(ctx, companyID)
	if err != nil {
		t.Fatal(err)
	}
	mission, err := k.TXCreateMissionGoal(ctx, scope, "Routine view", "project persisted daily Routines", "routine-view-mission")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = k.TXStartMissionCommand(ctx, scope, mission.ID, "routine-view-start"); err != nil {
		t.Fatal(err)
	}
	routineID := "routine-view-" + fmt.Sprint(time.Now().UnixNano())
	instruction := "Review the assigned queue and report one safe next step."
	if _, err = k.TXCreateDailyRoutine(ctx, scope, mission.ID, "emp-backend", core.DailyRoutineSchedule{
		RoutineID: routineID, TaskInstruction: instruction, Timezone: "UTC", LocalTime: "09:00",
		NextLogicalDay: "2026-09-29", CatchUpPolicy: core.RoutineCatchUpCoalesceLatest, MaxCatchUp: 1,
	}, "routine-view-create"); err != nil {
		t.Fatal(err)
	}
	if _, err = k.TXMaterializeDailyRoutine(ctx, scope, routineID, time.Date(2026, time.September, 30, 10, 0, 0, 0, time.UTC), "routine-view-materialize"); err != nil {
		t.Fatal(err)
	}
	store, err := NewPostgresReadStore(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	items, err := store.ListDailyRoutines(ctx, companyID, mission.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].RoutineID != routineID || items[0].TaskInstruction == nil || *items[0].TaskInstruction != instruction || items[0].LinkedTaskCount != 1 || items[0].NeedsInstructionOccurrences != 0 || items[0].NextLogicalDay != "2026-10-01" {
		t.Fatalf("Workbench daily Routines=%+v", items)
	}
}

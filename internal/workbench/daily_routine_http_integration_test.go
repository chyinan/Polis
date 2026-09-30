// pattern: Imperative Shell
package workbench

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"polis/internal/control"
	"polis/internal/core"
	"polis/internal/kernel"
)

func TestHTTPDailyRoutineCreateRepairAndReadback(t *testing.T) {
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
	companyID := fmt.Sprintf("routine-http-%d", time.Now().UnixNano())
	scope, err := k.TXCreateCompany(ctx, companyID)
	if err != nil {
		t.Fatal(err)
	}
	mission, err := k.TXCreateMissionGoal(ctx, scope, "Daily work", "complete the approved recurring check", "routine-http-mission")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = k.TXStartMissionCommand(ctx, scope, mission.ID, "routine-http-start"); err != nil {
		t.Fatal(err)
	}
	store, err := NewPostgresReadStore(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	handler := NewHandler(store, control.NewService(k, nil))
	routineID := "routine-http-" + fmt.Sprint(time.Now().UnixNano())
	createBody, err := json.Marshal(control.CreateDailyRoutineRequest{
		RoutineID: routineID, EmployeeID: "emp-backend", TaskInstruction: "Review the approved recurring check.",
		Timezone: "UTC", LocalTime: "00:00", NextLogicalDay: time.Now().UTC().AddDate(0, 0, -1).Format(time.DateOnly),
		CatchUpPolicy: string(core.RoutineCatchUpCoalesceLatest), MaxCatchUp: 1, RequestID: "routine-http-create",
	})
	if err != nil {
		t.Fatal(err)
	}
	createResponse := httptest.NewRecorder()
	createRequest := httptest.NewRequest(http.MethodPost, "/api/workbench/companies/"+companyID+"/missions/"+mission.ID+"/routines", bytes.NewReader(createBody))
	createRequest.Header.Set("Content-Type", "application/json")
	handler.ServeHTTP(createResponse, createRequest)
	if createResponse.Code != http.StatusAccepted {
		t.Fatalf("create Routine status=%d body=%s", createResponse.Code, createResponse.Body.String())
	}
	listRoutines := func() []DailyRoutineView {
		t.Helper()
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/workbench/companies/"+companyID+"/missions/"+mission.ID+"/routines", nil))
		if response.Code != http.StatusOK {
			t.Fatalf("list Routines status=%d body=%s", response.Code, response.Body.String())
		}
		var items []DailyRoutineView
		if err := json.NewDecoder(response.Body).Decode(&items); err != nil {
			t.Fatal(err)
		}
		return items
	}
	items := listRoutines()
	if len(items) != 1 || items[0].RoutineID != routineID || items[0].TaskInstruction == nil {
		t.Fatalf("created Routine readback=%+v", items)
	}
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if _, err = pool.Exec(ctx, `UPDATE routines SET task_instruction=NULL WHERE company_id=$1 AND id=$2`, companyID, routineID); err != nil {
		t.Fatal(err)
	}
	if _, err = k.TXMaterializeDailyRoutine(ctx, scope, routineID, time.Now().UTC(), "routine-http-materialize"); err != nil {
		t.Fatal(err)
	}
	items = listRoutines()
	if len(items) != 1 || items[0].NeedsInstructionOccurrences != 1 || items[0].TaskInstruction != nil {
		t.Fatalf("legacy Routine missing-instruction projection=%+v", items)
	}
	repairBody, err := json.Marshal(control.SetDailyRoutineTaskInstructionRequest{TaskInstruction: "Inspect current behavior and report the next safe step.", RequestID: "routine-http-repair"})
	if err != nil {
		t.Fatal(err)
	}
	repairResponse := httptest.NewRecorder()
	repairRequest := httptest.NewRequest(http.MethodPost, "/api/workbench/companies/"+companyID+"/missions/"+mission.ID+"/routines/"+routineID+"/instruction", bytes.NewReader(repairBody))
	repairRequest.Header.Set("Content-Type", "application/json")
	handler.ServeHTTP(repairResponse, repairRequest)
	if repairResponse.Code != http.StatusAccepted {
		t.Fatalf("repair Routine status=%d body=%s", repairResponse.Code, repairResponse.Body.String())
	}
	items = listRoutines()
	if len(items) != 1 || items[0].NeedsInstructionOccurrences != 0 || items[0].LinkedTaskCount != 1 || items[0].TaskInstruction == nil || *items[0].TaskInstruction != "Inspect current behavior and report the next safe step." {
		t.Fatalf("repaired Routine readback=%+v", items)
	}
}

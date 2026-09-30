// pattern: Imperative Shell
package control

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"polis/internal/core"
	"polis/internal/kernel"
	"polis/internal/provider"
)

type unavailableReadinessAdapter struct {
	startCalls int
}

func (a *unavailableReadinessAdapter) Mode() string { return "deterministic" }
func (a *unavailableReadinessAdapter) Readiness(context.Context) error {
	return errors.New("secret=never-store-this-token")
}
func (a *unavailableReadinessAdapter) ToolSurface() provider.ToolSurface {
	return provider.ToolSurface{}
}
func (a *unavailableReadinessAdapter) Start(context.Context, string, string) error {
	a.startCalls++
	return nil
}
func (*unavailableReadinessAdapter) Stop(context.Context, string, string) error { return nil }
func (*unavailableReadinessAdapter) Close()                                     {}

func TestMissionStartReadinessFailureCreatesSafeHumanInterventionAndOutbox(t *testing.T) {
	dsn := os.Getenv("POLIS_TEST_DSN")
	if dsn == "" {
		t.Skip("dedicated PostgreSQL required")
	}
	ctx := context.Background()
	runtime, err := kernel.Open(ctx, dsn, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Close()
	companyID := "start-intervention-" + fmt.Sprint(time.Now().UnixNano())
	scope, err := runtime.TXCreateCompany(ctx, companyID)
	if err != nil {
		t.Fatal(err)
	}
	mission, err := runtime.TXCreateMissionGoal(ctx, scope, "Readiness failure", "record a human action when runtime is unavailable", "start-intervention-mission")
	if err != nil {
		t.Fatal(err)
	}
	worker := &unavailableReadinessAdapter{}
	service := NewService(runtime, worker)
	_, err = service.StartMission(ctx, companyID, MissionCommandRequest{MissionID: mission.ID, RequestID: "start-intervention-request"})
	if !errors.Is(err, core.Conflict) {
		t.Fatalf("StartMission error = %v, want provider unavailable conflict", err)
	}
	if worker.startCalls != 0 {
		t.Fatalf("worker Start called %d times after readiness failure", worker.startCalls)
	}
	missionDetails, err := runtime.MissionDetails(ctx, scope, mission.ID)
	if err != nil {
		t.Fatal(err)
	}
	if missionDetails.State != "draft" {
		t.Fatalf("readiness failure activated Mission: %s", missionDetails.State)
	}
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	var interventionCount, intentCount int
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM human_interventions WHERE company_id=$1 AND mission_id=$2 AND state='open' AND reason_code='handover_required'`, companyID, mission.ID).Scan(&interventionCount); err != nil {
		t.Fatal(err)
	}
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM notification_intents WHERE company_id=$1 AND event_kind='human_intervention.open' AND state='pending'`, companyID).Scan(&intentCount); err != nil {
		t.Fatal(err)
	}
	if interventionCount != 1 || intentCount != 1 {
		t.Fatalf("human intervention/outbox counts = %d/%d", interventionCount, intentCount)
	}
	var payload string
	if err = pool.QueryRow(ctx, `SELECT payload::text FROM notification_intents WHERE company_id=$1 AND event_kind='human_intervention.open'`, companyID).Scan(&payload); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(payload, "never-store-this-token") || strings.Contains(payload, "secret=") {
		t.Fatalf("raw runtime error reached the notification outbox: %s", payload)
	}
}

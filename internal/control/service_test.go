// pattern: Imperative Shell
package control

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"polis/internal/kernel"
	"polis/internal/provider"
)

type recordingWorkerAdapter struct {
	started []string
	stopped []string
}

func TestCompanySummarySerializesEmptyRosterAsArray(t *testing.T) {
	summary := companySummary(kernel.CompanyDetails{ID: "company-1", Name: "Company One", WorkspaceRoot: "C:/workspace", State: "active"})
	raw, err := json.Marshal(summary)
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err = json.Unmarshal(raw, &decoded); err != nil {
		t.Fatal(err)
	}
	roster, ok := decoded["roster"].([]any)
	if !ok || roster == nil {
		t.Fatalf("roster JSON = %#v, want empty array", decoded["roster"])
	}
}

func (a *recordingWorkerAdapter) Mode() string                      { return "test" }
func (a *recordingWorkerAdapter) Readiness(context.Context) error   { return nil }
func (a *recordingWorkerAdapter) ToolSurface() provider.ToolSurface { return provider.ToolSurface{} }

func (a *recordingWorkerAdapter) Start(_ context.Context, companyID, missionID string) error {
	a.started = append(a.started, companyID+"/"+missionID)
	return nil
}

func (a *recordingWorkerAdapter) Stop(_ context.Context, companyID, missionID string) error {
	a.stopped = append(a.stopped, companyID+"/"+missionID)
	return nil
}

func (a *recordingWorkerAdapter) Close() {}

func TestServiceUsesWorkerAdapterForStartAndCancel(t *testing.T) {
	dsn := os.Getenv("POLIS_R05A_CONTROL_TEST_DSN")
	if dsn == "" {
		t.Skip("dedicated control-service PostgreSQL required")
	}
	ctx := context.Background()
	k, err := kernel.Open(ctx, dsn, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer k.Close()
	company := "r05a-control-company"
	_, err = k.TXCreateCompany(ctx, company)
	if err != nil {
		t.Fatal(err)
	}
	adapter := &recordingWorkerAdapter{}
	service := NewService(k, adapter)

	created, err := service.CreateMission(ctx, company, CreateMissionRequest{Title: "control goal", Goal: "control goal body", RequestID: "control-create"})
	if err != nil {
		t.Fatal(err)
	}
	started, err := service.StartMission(ctx, company, MissionCommandRequest{MissionID: created.TargetID, RequestID: "control-start"})
	if err != nil {
		t.Fatal(err)
	}
	if !started.Accepted || started.ResultingState != "active" {
		t.Fatalf("unexpected start command: %+v", started)
	}
	if len(adapter.started) != 1 {
		t.Fatalf("worker starts = %v, want one", adapter.started)
	}

	cancelled, err := service.CancelMission(ctx, company, MissionCommandRequest{MissionID: created.TargetID, RequestID: "control-cancel"})
	if err != nil {
		t.Fatal(err)
	}
	if !cancelled.Accepted || cancelled.ResultingState != "cancelled" {
		t.Fatalf("unexpected cancel command: %+v", cancelled)
	}
	if len(adapter.stopped) != 1 {
		t.Fatalf("worker stops = %v, want one", adapter.stopped)
	}
}

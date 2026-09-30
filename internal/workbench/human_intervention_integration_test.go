// pattern: Imperative Shell
package workbench

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"polis/internal/kernel"
)

func TestOpenHumanInterventionAppearsInWorkbenchAttention(t *testing.T) {
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
	suffix := fmt.Sprint(time.Now().UnixNano())
	companyID := "human-view-" + suffix
	scope, err := runtime.TXCreateCompany(ctx, companyID)
	if err != nil {
		t.Fatal(err)
	}
	mission, err := runtime.TXCreateMissionGoal(ctx, scope, "Runtime recovery", "verify administrator handover visibility", "human-view-mission-"+suffix)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = runtime.TXCreateHumanIntervention(ctx, scope, kernel.HumanInterventionInput{
		MissionID: mission.ID, ProblemKey: "provider-readiness-view", Severity: "high", ReasonCode: "handover_required",
		AffectedScope: "mission", ProtectionAction: "no_mutation", RequiredAction: "reauthorize_in_workbench",
	}, "human-view-intervention-"+suffix); err != nil {
		t.Fatal(err)
	}
	store, err := NewPostgresReadStore(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	view, err := store.GetCompanyOverview(ctx, companyID)
	if err != nil {
		t.Fatal(err)
	}
	var found *AttentionItem
	for index := range view.Attention {
		if view.Attention[index].Subject.Kind == "human_intervention" {
			found = &view.Attention[index]
			break
		}
	}
	if found == nil || found.Tone != "warning" || found.Subject.Label == "" || found.WorkflowState != "open" || found.NotificationState != "pending" || !strings.Contains(found.Description, "工作台") {
		t.Fatalf("human intervention is missing or unsafe in attention projection: %+v", view.Attention)
	}
}

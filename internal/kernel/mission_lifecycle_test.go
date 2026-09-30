// pattern: Imperative Shell
package kernel

import (
	"context"
	"os"
	"testing"

	"polis/internal/core"
)

func TestMissionPauseResumeCommandsRequireStateAndReplayIdempotently(t *testing.T) {
	dsn := os.Getenv("POLIS_TEST_DSN")
	if dsn == "" {
		t.Skip("dedicated PostgreSQL required")
	}
	ctx := context.Background()
	runtime, err := Open(ctx, dsn, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Close()
	scope, err := runtime.TXCreateCompany(ctx, "mission-lifecycle-"+newID())
	if err != nil {
		t.Fatal(err)
	}
	mission, err := runtime.TXCreateMissionGoal(ctx, scope, "Pause and resume", "verify formal mission lifecycle commands", "lifecycle-create")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = runtime.TXSetMissionPausedCommand(ctx, scope, mission.ID, true, "lifecycle-pause-before-start"); err == nil {
		t.Fatal("paused a draft Mission")
	}
	if _, err = runtime.TXStartMissionCommand(ctx, scope, mission.ID, "lifecycle-start"); err != nil {
		t.Fatal(err)
	}
	paused, err := runtime.TXSetMissionPausedCommand(ctx, scope, mission.ID, true, "lifecycle-pause")
	if err != nil {
		t.Fatal(err)
	}
	if paused.Status != "paused" {
		t.Fatalf("pause receipt=%+v", paused)
	}
	replayed, found, err := runtime.LookupMissionPauseCommand(ctx, scope, mission.ID, true, "lifecycle-pause")
	if err != nil || !found || replayed != paused {
		t.Fatalf("pause replay receipt=%+v found=%t err=%v", replayed, found, err)
	}
	if _, err = runtime.TXSetMissionPausedCommand(ctx, scope, mission.ID, true, "lifecycle-pause-duplicate"); err == nil {
		t.Fatal("accepted a second state transition to pause")
	}
	resumed, err := runtime.TXSetMissionPausedCommand(ctx, scope, mission.ID, false, "lifecycle-resume")
	if err != nil || resumed.Status != "active" {
		t.Fatalf("resume receipt=%+v err=%v", resumed, err)
	}
	if _, err = runtime.TXSetMissionPausedCommand(ctx, scope, mission.ID, false, "lifecycle-resume-duplicate"); err == nil {
		t.Fatal("accepted a second state transition to resume")
	}
	if _, _, err = runtime.LookupMissionPauseCommand(ctx, scope, mission.ID, false, "lifecycle-pause"); err == nil || err == core.Malformed {
		t.Fatalf("reused a pause request id for resume without a conflict: %v", err)
	}
}

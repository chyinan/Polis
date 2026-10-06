// pattern: Functional Core
package kernel

import (
	"context"
	"errors"
	"testing"

	"polis/internal/core"
)

func TestValidateProductTaskEnvironmentEnsureScopeRequiresCurrentMissionAndTask(t *testing.T) {
	binding := Binding{scope: Scope{company: "company"}, task: "task", session: "session"}
	if err := validateProductTaskEnvironmentEnsureScope(binding, "mission", "mission", "task"); err != nil {
		t.Fatalf("current Mission/Task was rejected: %v", err)
	}
	for name, input := range map[string]struct {
		revisionMission string
		currentMission  string
		currentTask     string
	}{
		"different mission": {revisionMission: "other-mission", currentMission: "mission", currentTask: "task"},
		"different task":    {revisionMission: "mission", currentMission: "mission", currentTask: "other-task"},
		"missing session":   {revisionMission: "mission", currentMission: "mission", currentTask: "task"},
	} {
		t.Run(name, func(t *testing.T) {
			caseBinding := binding
			if name == "missing session" {
				caseBinding.session = ""
			}
			if err := validateProductTaskEnvironmentEnsureScope(caseBinding, input.revisionMission, input.currentMission, input.currentTask); !errors.Is(err, core.OutOfScope) {
				t.Fatalf("scope error=%v, want %v", err, core.OutOfScope)
			}
		})
	}
}

func TestEmployeeToolsEnvironmentEnsureRequiresTheIsolatedWritableSurface(t *testing.T) {
	_, err := (EmployeeTools{}).call(context.Background(), "environment_ensure", "budget-key", []byte(`{"revision_id":"revision"}`))
	if !errors.Is(err, core.Denied) {
		t.Fatalf("environment ensure without isolated surface returned %v, want %v", err, core.Denied)
	}
}

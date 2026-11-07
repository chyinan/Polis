// pattern: Functional Core
package kernel

import (
	"errors"
	"testing"

	"polis/internal/core"
	"polis/internal/taskvalidation"
)

func TestTaskIsProductProviderExecutableUsesPersistedKindAndAssignee(t *testing.T) {
	tests := []struct {
		name string
		task Task
		want bool
	}{
		{
			name: "compat backend task",
			task: Task{Kind: core.TaskKindCompat, Owner: core.EmployeeBackendID},
			want: true,
		},
		{
			name: "planning bootstrap task",
			task: Task{Kind: core.TaskKindBootstrapPlan, Owner: core.EmployeePlanningID},
			want: false,
		},
		{
			name: "compat task assigned to another employee",
			task: Task{Kind: core.TaskKindCompat, Owner: core.EmployeePlanningID},
			want: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := test.task.IsProductProviderExecutable(); got != test.want {
				t.Fatalf("Task.IsProductProviderExecutable() = %t, want %t", got, test.want)
			}
		})
	}
}

func TestProductTaskValidationBindingMustMatchTaskMissionAndDigest(t *testing.T) {
	contract := &taskvalidation.AcceptanceContract{
		Revision:     taskvalidation.AcceptanceContractRevision,
		RequiredText: []string{"Mission ID: mission", "Task ID: task"},
	}
	binding, err := taskvalidation.Bind("task", "mission", contract)
	if err != nil {
		t.Fatal(err)
	}
	task := Task{ID: "task", Mission: "mission", Kind: core.TaskKindCompat, Owner: core.EmployeeBackendID, ValidationBinding: binding}
	if !task.HasValidProductValidationBinding() {
		t.Fatal("valid public TaskValidationBinding was rejected")
	}

	changedTask := task
	changedTask.ID = "other-task"
	changedMission := task
	changedMission.Mission = "other-mission"
	changedDigest := task
	changedDigest.ValidationBinding = &taskvalidation.Binding{
		TaskID: binding.TaskID, MissionID: binding.MissionID, AcceptanceRevision: binding.AcceptanceRevision,
		RunnerKind: binding.RunnerKind, RunnerRevision: binding.RunnerRevision,
		ConfigurationDigest: "0000000000000000000000000000000000000000000000000000000000000000", Contract: binding.Contract,
	}
	withoutBinding := task
	withoutBinding.ValidationBinding = nil
	for name, candidate := range map[string]Task{
		"different task":      changedTask,
		"different mission":   changedMission,
		"configuration drift": changedDigest,
		"missing binding":     withoutBinding,
	} {
		t.Run(name, func(t *testing.T) {
			if candidate.HasValidProductValidationBinding() {
				t.Fatalf("TaskValidationBinding was accepted for invalid %s case", name)
			}
		})
	}
}

func TestProductWorkspaceCASRequiresSha256DigestAndPositiveRevision(t *testing.T) {
	if !IsValidProductWorkspaceCAS("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", 1) {
		t.Fatal("valid product workspace CAS snapshot was rejected")
	}
	for _, test := range []struct {
		name     string
		digest   string
		revision int64
	}{
		{name: "missing digest", digest: "", revision: 1},
		{name: "non-hex digest", digest: "zzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzz", revision: 1},
		{name: "zero revision", digest: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", revision: 0},
	} {
		t.Run(test.name, func(t *testing.T) {
			if IsValidProductWorkspaceCAS(test.digest, test.revision) {
				t.Fatalf("invalid workspace CAS snapshot accepted: digest=%q revision=%d", test.digest, test.revision)
			}
		})
	}
}

func TestSelectSingleProductProviderTaskIgnoresMissionControlTask(t *testing.T) {
	bootstrap := Task{ID: "plan", Mission: "mission", Owner: core.EmployeePlanningID, Kind: core.TaskKindBootstrapPlan, State: "completed"}
	compat := Task{ID: "work", Mission: "mission", Owner: core.EmployeeBackendID, Kind: core.TaskKindCompat, State: "ready"}

	selected, err := SelectSingleProductProviderTask([]Task{bootstrap, compat})
	if err != nil {
		t.Fatal(err)
	}
	if selected.ID != compat.ID {
		t.Fatalf("selected Task = %q, want provider-executable Task %q", selected.ID, compat.ID)
	}
}

func TestSelectSingleProductProviderTaskRejectsMultipleExecutableTasks(t *testing.T) {
	first := Task{ID: "work-a", Mission: "mission", Owner: core.EmployeeBackendID, Kind: core.TaskKindCompat, State: "ready"}
	second := Task{ID: "work-b", Mission: "mission", Owner: core.EmployeeBackendID, Kind: core.TaskKindCompat, State: "ready"}
	if _, err := SelectSingleProductProviderTask([]Task{first, second}); !errors.Is(err, core.Conflict) {
		t.Fatalf("multiple provider-executable Tasks error = %v, want %s", err, core.Conflict)
	}
}

func TestSelectSingleProductProviderTaskRejectsEmptyFanout(t *testing.T) {
	if _, err := SelectSingleProductProviderTask([]Task{{ID: "plan", Kind: core.TaskKindBootstrapPlan, Owner: core.EmployeePlanningID}}); !errors.Is(err, core.Denied) {
		t.Fatalf("empty provider-executable Task set error = %v, want %s", err, core.Denied)
	}
}

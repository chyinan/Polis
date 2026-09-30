// pattern: Functional Core
package core

import "testing"

func TestProductProviderExecutableTaskRequiresCompatKindAndBackendAssignee(t *testing.T) {
	tests := []struct {
		name  string
		kind  TaskKind
		owner string
		want  bool
	}{
		{name: "bootstrap planning task", kind: TaskKindBootstrapPlan, owner: EmployeePlanningID, want: false},
		{name: "product employee task", kind: TaskKindCompat, owner: EmployeeBackendID, want: true},
		{name: "compat assigned to planner", kind: TaskKindCompat, owner: EmployeePlanningID, want: false},
		{name: "arithmetic task", kind: TaskKindCompute, owner: EmployeeBackendID, want: false},
		{name: "peer backend task is outside product surface", kind: TaskKindPeerBackend, owner: EmployeeBackendID, want: false},
		{name: "unknown task kind", kind: TaskKind("unknown"), owner: EmployeeBackendID, want: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := IsProductProviderExecutableTask(test.kind, test.owner); got != test.want {
				t.Fatalf("IsProductProviderExecutableTask(%q, %q) = %t, want %t", test.kind, test.owner, got, test.want)
			}
		})
	}
}

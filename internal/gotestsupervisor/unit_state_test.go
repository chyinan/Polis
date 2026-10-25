// pattern: Functional Core
package gotestsupervisor

import "testing"

func TestQueuedInactiveUnitIsNotATerminalExecution(t *testing.T) {
	if unitTerminal(map[string]string{"ActiveState": "inactive", "InvocationID": ""}) {
		t.Fatal("queued unit was treated as a completed launch")
	}
	if !unitTerminal(map[string]string{"ActiveState": "inactive", "InvocationID": "observed"}) {
		t.Fatal("observed terminal invocation was not collectable")
	}
	if unitTerminal(map[string]string{"ActiveState": "active", "InvocationID": "observed"}) {
		t.Fatal("running invocation was treated as terminal")
	}
}

func TestUnitResultFallbackRequiresSuccessfulTerminalState(t *testing.T) {
	tests := []struct {
		name       string
		properties map[string]string
		want       bool
	}{
		{
			name: "inactive invocation result",
			properties: map[string]string{
				"ActiveState": "inactive", "InvocationID": "", "Result": "success", "ExecMainStatus": "0",
			},
			want: true,
		},
		{
			name: "active unit waits for an invocation result",
			properties: map[string]string{
				"ActiveState": "active", "InvocationID": "", "Result": "success", "ExecMainStatus": "0",
			},
			want: false,
		},
		{
			name: "nonzero systemd result is rejected",
			properties: map[string]string{
				"ActiveState": "failed", "InvocationID": "", "Result": "exit-code", "ExecMainStatus": "1",
			},
			want: false,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := unitResultMayBeReady(test.properties); got != test.want {
				t.Fatalf("unitResultMayBeReady() = %t, want %t", got, test.want)
			}
		})
	}
}

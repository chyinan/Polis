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

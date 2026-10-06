// pattern: Functional Core
package spec

import "testing"

func TestTaskRevisionDecisionTransitionUsesExplicitMonotonicLifecycle(t *testing.T) {
	cases := []struct {
		name, current, decision, want string
	}{
		{name: "proposed approved", current: "", decision: string(TaskRevisionDecisionApproved), want: string(TaskRevisionStateApproved)},
		{name: "proposed rejected", current: string(TaskRevisionStateProposed), decision: string(TaskRevisionDecisionRejected), want: string(TaskRevisionStateRejected)},
		{name: "approved revoked", current: string(TaskRevisionStateApproved), decision: string(TaskRevisionDecisionRevoked), want: string(TaskRevisionStateRevoked)},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			if got, err := TaskRevisionDecisionTransition(test.current, FixedTeamTaskRevisionDecision(test.decision)); err != nil || got != test.want {
				t.Fatalf("transition(%q,%q)=(%q,%v), want %q", test.current, test.decision, got, err, test.want)
			}
		})
	}
}

func TestTaskRevisionDecisionTransitionRejectsResurrectionAndInvalidStates(t *testing.T) {
	cases := []struct {
		name, current, decision string
	}{
		{name: "rejected cannot approve", current: string(TaskRevisionStateRejected), decision: string(TaskRevisionDecisionApproved)},
		{name: "rejected cannot revoke", current: string(TaskRevisionStateRejected), decision: string(TaskRevisionDecisionRevoked)},
		{name: "proposed cannot revoke", current: string(TaskRevisionStateProposed), decision: string(TaskRevisionDecisionRevoked)},
		{name: "revoked cannot approve", current: string(TaskRevisionStateRevoked), decision: string(TaskRevisionDecisionApproved)},
		{name: "unknown state", current: "candidate", decision: string(TaskRevisionDecisionApproved)},
		{name: "unknown decision", current: string(TaskRevisionStateProposed), decision: "qualified"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			if _, err := TaskRevisionDecisionTransition(test.current, FixedTeamTaskRevisionDecision(test.decision)); err == nil {
				t.Fatalf("transition(%q,%q) unexpectedly succeeded", test.current, test.decision)
			}
		})
	}
}

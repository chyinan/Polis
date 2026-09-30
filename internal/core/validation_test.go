// pattern: Functional Core
package core

import (
	"errors"
	"testing"
)

func TestCandidateRequiresExactContractAndIndependentChecker(t *testing.T) {
	for _, tc := range []struct {
		name, author, reviewer, content string
		valid                           bool
	}{
		{"valid", "emp-backend", "emp-review", "Polis R0: 2 + 3 = 5\n", true},
		{"wrong sum", "emp-backend", "emp-review", "Polis R0: 2 + 3 = 6\n", false},
		{"self approval", "emp-backend", "emp-backend", "Polis R0: 2 + 3 = 5\n", false},
		{"empty", "emp-backend", "emp-review", "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := CheckCandidate(tc.author, tc.reviewer, []byte(tc.content)); (got == nil) != tc.valid {
				t.Fatalf("CheckCandidate = %v, want valid %v", got, tc.valid)
			}
		})
	}
}

func TestConflictErrorKeepsMachineCodeAndHumanState(t *testing.T) {
	err := ConflictError{Reason: "mission is already active", CurrentState: "active"}
	if !errors.Is(err, Conflict) || err.Error() != "mission is already active" || err.CurrentState != "active" {
		t.Fatalf("conflict error = %v/%q, want conflict with active state", err, err.CurrentState)
	}
}

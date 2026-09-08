// pattern: Functional Core
package core

import "testing"

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

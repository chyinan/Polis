// pattern: Functional Core
package core

import (
	"bytes"
	"errors"
	"fmt"
	"regexp"
)

type Code string

func (c Code) Error() string { return string(c) }

const (
	Malformed              Code = "MALFORMED_INPUT"
	TooLarge               Code = "INPUT_TOO_LARGE"
	OutOfScope             Code = "OUT_OF_SCOPE"
	StaleEpoch             Code = "STALE_EPOCH"
	Conflict               Code = "REVISION_CONFLICT"
	Denied                 Code = "POLICY_DENIED"
	Integrity              Code = "ARTIFACT_INTEGRITY"
	ToolCallBudgetExceeded Code = "TOOL_CALL_BUDGET_EXHAUSTED"
)
const Contract = "r0-arithmetic@1"
const MaxContent = 4096

type ConflictError struct {
	Reason       string
	CurrentState string
}

func (e ConflictError) Error() string {
	if e.Reason != "" {
		return e.Reason
	}
	return fmt.Sprintf("command conflicts with current state %q", e.CurrentState)
}

func (e ConflictError) Unwrap() error { return Conflict }

var idPattern = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,80}$`)

func ValidID(id string) bool { return idPattern.MatchString(id) }

// CheckCandidate is the frozen R0 checker, never supplied by a worker.
func CheckCandidate(author, reviewer string, content []byte) error {
	if author == reviewer || reviewer != "emp-review" {
		return Denied
	}
	if !bytes.Equal(content, []byte("Polis R0: 2 + 3 = 5\n")) {
		return errors.New("acceptance_failed: fixed arithmetic contract not satisfied")
	}
	return nil
}

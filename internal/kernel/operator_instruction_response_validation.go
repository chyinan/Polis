// pattern: Functional Core
package kernel

import (
	"strings"
	"unicode/utf8"

	"polis/internal/core"
)

type OperatorInstructionOutcome string

const (
	OperatorInstructionApplied            OperatorInstructionOutcome = "applied"
	OperatorInstructionRejected           OperatorInstructionOutcome = "rejected"
	OperatorInstructionNeedsClarification OperatorInstructionOutcome = "needs_clarification"
)

func normalizeOperatorInstructionResponse(input OperatorInstructionResponseInput) (OperatorInstructionResponseInput, error) {
	input.InstructionID = strings.TrimSpace(input.InstructionID)
	input.Summary = strings.TrimSpace(input.Summary)
	if !core.ValidID(input.InstructionID) || utf8.RuneCountInString(input.Summary) < 1 || utf8.RuneCountInString(input.Summary) > 128 || len(input.Summary) > 512 {
		return OperatorInstructionResponseInput{}, core.Malformed
	}
	switch input.Outcome {
	case OperatorInstructionApplied, OperatorInstructionRejected, OperatorInstructionNeedsClarification:
	default:
		return OperatorInstructionResponseInput{}, core.Malformed
	}
	return input, nil
}

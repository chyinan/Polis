// pattern: Functional Core
package kernel

import (
	"strings"
	"testing"
)

func TestNormalizeOperatorInstructionResponseUsesUnicodeCharacterAndByteBounds(t *testing.T) {
	validEmojiSummary := strings.Repeat("😀", 128)
	response, err := normalizeOperatorInstructionResponse(OperatorInstructionResponseInput{
		InstructionID: "instruction-1", Outcome: OperatorInstructionApplied, Summary: validEmojiSummary,
	})
	if err != nil || response.Summary != validEmojiSummary || len(response.Summary) != 512 {
		t.Fatalf("128 four-byte Unicode characters were rejected: bytes=%d err=%v", len(response.Summary), err)
	}
	if _, err = normalizeOperatorInstructionResponse(OperatorInstructionResponseInput{
		InstructionID: "instruction-1", Outcome: OperatorInstructionApplied, Summary: strings.Repeat("界", 129),
	}); err == nil {
		t.Fatal("129 Unicode characters exceeded the shared summary length contract")
	}
	if _, err = normalizeOperatorInstructionResponse(OperatorInstructionResponseInput{
		InstructionID: "instruction-1", Outcome: OperatorInstructionApplied, Summary: " \t\n ",
	}); err == nil {
		t.Fatal("whitespace-only response summary was accepted")
	}
}

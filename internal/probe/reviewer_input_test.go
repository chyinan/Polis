// pattern: Functional Core
package probe

import "testing"

func TestReviewerInputAccountingUsesByteBasedTokenEstimate(t *testing.T) {
	components := estimateReviewerInputContext("abcd", "12345678", "prompt", []string{"tool-result"})
	if len(components) != 4 {
		t.Fatalf("unexpected component count: %d", len(components))
	}
	if components[0].Bytes != 4 || components[0].EstimatedTokens != 1 {
		t.Fatalf("developer accounting mismatch: %+v", components[0])
	}
	if components[1].Bytes != 8 || components[1].EstimatedTokens != 2 {
		t.Fatalf("schema accounting mismatch: %+v", components[1])
	}
	if components[3].Source == "" || components[3].EstimatedTokens != 3 {
		t.Fatalf("tool result accounting mismatch: %+v", components[3])
	}
}

// pattern: Functional Core
package probe

import "testing"

func TestH3OrderingRequiresReviewerFreezeBeforeHiddenVerifier(t *testing.T) {
	if err := validateH3Ordering([]string{"reviewer_session_end", "review_record_frozen", "hidden_verifier", "runtime_isolation", "composite_assessment"}); err != nil {
		t.Fatal(err)
	}
	if err := validateH3Ordering([]string{"reviewer_session_end", "hidden_verifier", "review_record_frozen", "runtime_isolation", "composite_assessment"}); err == nil {
		t.Fatal("hidden verifier was allowed before frozen reviewer record")
	}
}

func TestH3CompositeCannotPassWithoutReviewerVerdict(t *testing.T) {
	if got := h3CompositeVerdict("", "passed", "passed"); got != "inconclusive" {
		t.Fatalf("missing reviewer verdict became %s", got)
	}
	if got := h3CompositeVerdict("passed", "failed", "passed"); got != "inconclusive" {
		t.Fatalf("hidden verifier failure became %s", got)
	}
	if got := h3CompositeVerdict("passed", "passed", "passed"); got != "passed" {
		t.Fatalf("all passing inputs became %s", got)
	}
}

// pattern: Functional Core
package probe

import "testing"

func TestH1CompositePreservesReviewerDisagreement(t *testing.T) {
	if got := h1Composite("passed", "failed", "passed"); got.Composite != "FAILED" || got.ReviewerQuality != "FAIL" {
		t.Fatalf("hidden verifier disagreement was collapsed: %+v", got)
	}
	if got := h1Composite("failed", "passed", "passed"); got.Composite != "FAILED" || got.ReviewerQuality != "FAIL_DISAGREEMENT" {
		t.Fatalf("reviewer disagreement was not preserved: %+v", got)
	}
	if got := h1Composite("passed", "passed", "passed"); got.Composite != "PASSED" || got.ReviewerQuality != "PASS" {
		t.Fatalf("all-pass composite was not accepted: %+v", got)
	}
}

func TestH1ReviewerIsolationRejectsSubjectWriterActivity(t *testing.T) {
	if h1ReviewerIsolation([]string{"work_current", "context_read", "workspace_read", "review_submit"}, []byte("reviewer only")) != true {
		t.Fatal("valid read-only review was rejected")
	}
	if h1ReviewerIsolation([]string{"work_current", "workspace_replace", "review_submit"}, []byte("reviewer only")) {
		t.Fatal("subject writer activity passed reviewer isolation")
	}
}

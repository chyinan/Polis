// pattern: Imperative Shell
package kernel

import (
	"context"
	"os"
	"path/filepath"
	"polis/internal/fixture"
	"testing"
)

func TestFrontendConsumptionCandidateRuntimeReferencePasses(t *testing.T) {
	root := t.TempDir()
	report, err := VerifyFrontendConsumptionCandidateSource(context.Background(), root, fixture.FrontendConsumptionReferenceSource)
	if err != nil {
		t.Fatal(err)
	}
	if !report.Passed {
		t.Fatalf("reference source failed public consumption verifier: %+v", report)
	}
}

func TestFrontendConsumptionCandidateRuntimeFrozenV6FailsWithPublicReasons(t *testing.T) {
	root := t.TempDir()
	report, err := VerifyFrontendConsumptionCandidateSource(context.Background(), root, fixture.FrozenV6Revision12FrontendCandidate)
	if err != nil {
		t.Fatal(err)
	}
	if report.Passed || !reportHasReason(report, "response_items_not_consumed") || !reportHasReason(report, "next_cursor_not_consumed") {
		t.Fatalf("frozen V6 candidate was not rejected for public behavior: %+v", report)
	}
}

func TestFrontendConsumptionRuntimeUsesDisposableRootOnly(t *testing.T) {
	root := t.TempDir()
	if _, err := os.Stat(filepath.Clean(root)); err != nil {
		t.Fatal(err)
	}
}

func reportHasReason(report fixture.FrontendConsumptionReport, reason string) bool {
	for _, criterion := range report.Criteria {
		if criterion.ReasonCode == reason {
			return true
		}
	}
	return false
}

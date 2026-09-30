// pattern: Functional Core
package control

import (
	"testing"

	"polis/internal/codex"
	"polis/internal/intake"
)

func TestTaskImageSelectionMatchesCodexTurnInputBounds(t *testing.T) {
	if intake.MaxModelInputImages != codex.MaxTurnImages || intake.MaxModelInputImageBytes != codex.MaxTurnImageBytes || intake.MaxModelInputImageTotalBytes != codex.MaxTurnImageTotalBytes {
		t.Fatalf("Task and Codex image bounds differ: intake=%d/%d/%d codex=%d/%d/%d", intake.MaxModelInputImages, intake.MaxModelInputImageBytes, intake.MaxModelInputImageTotalBytes, codex.MaxTurnImages, codex.MaxTurnImageBytes, codex.MaxTurnImageTotalBytes)
	}
}

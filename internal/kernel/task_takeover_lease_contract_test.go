// pattern: Functional Core
package kernel

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"

	"polis/internal/core"
)

func TestTaskTakeoverDiffSummaryBindsBothWorkspaceVersions(t *testing.T) {
	base := []byte("alpha\nbeta\n")
	baseDigest := testContentDigest(base)
	result, err := taskTakeoverDiffSummary(baseDigest, 7, base, []byte("alpha\nbeta\nhuman\n"))
	if err != nil {
		t.Fatal(err)
	}
	if result.Model != "single_hunk_line_summary@1" || result.BaseWorkspaceRevision != 7 || result.BaseBytes != len(base) || result.SubmittedBytes == 0 || result.RemovedLines != 0 || result.AddedLines != 1 || !result.Changed || result.SubmittedContentDigest == baseDigest {
		t.Fatalf("bounded human diff summary=%+v", result)
	}
}

func TestTaskTakeoverDiffSummaryRejectsStaleOrNoopSnapshots(t *testing.T) {
	base := []byte("current workspace\n")
	digest := testContentDigest(base)
	var err error
	if _, err = taskTakeoverDiffSummary(strings.Repeat("f", 64), 1, base, []byte("new workspace\n")); err != core.Integrity {
		t.Fatalf("stale base digest error=%v, want integrity", err)
	}
	if _, err = taskTakeoverDiffSummary(digest, 1, base, base); err == nil {
		t.Fatal("unchanged human snapshot was accepted")
	}
	if validTaskTakeoverHumanEffortSeconds(86400) == false || validTaskTakeoverHumanEffortSeconds(-1) || validTaskTakeoverHumanEffortSeconds(86401) {
		t.Fatal("human effort bounds do not enforce 0 through 86400 seconds")
	}
}

func testContentDigest(content []byte) string {
	digest := sha256.Sum256(content)
	return hex.EncodeToString(digest[:])
}

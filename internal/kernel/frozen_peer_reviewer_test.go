// pattern: Imperative Shell
package kernel

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestFrozenPeerReviewerPersistsOnlyIndependentReviewRecord(t *testing.T) {
	recordPath := filepath.Join(t.TempDir(), "review-record.json")
	tools := &FrozenPeerReviewerTools{Evidence: FrozenPeerReviewEvidence{
		SubjectRevision: "subject", ContractRevisionID: "contract", MessageID: "message", ObligationID: "obligation",
		BackendArtifactID: "backend-artifact", FrontendArtifactID: "frontend-artifact", BackendDigest: "backend-digest", FrontendDigest: "frontend-digest",
		BackendContent: "backend", FrontendContent: "frontend", TaskInput: "review", AllowedRefs: []string{"subject", "contract"}, ReviewRecordPath: recordPath,
	}}
	ctx := context.Background()
	for _, name := range []string{"work_current", "context_read", "workspace_read"} {
		result := tools.Call(ctx, name, "call-"+name, []byte(`{}`))
		if result.Error != "" {
			t.Fatalf("read-only reviewer tool %s failed: %+v", name, result)
		}
	}
	submission := ReviewerSubmission{Verdict: "passed", Findings: []string{"public contract and frozen candidates are consistent"}, Evidence: []string{"subject", "contract"}, Confidence: "high", Limitations: []string{"review is limited to the frozen package"}}
	raw, err := json.Marshal(submission)
	if err != nil {
		t.Fatal(err)
	}
	result := tools.Call(ctx, "review_submit", "review-submit", raw)
	if result.Error != "" || result.Receipt == nil {
		t.Fatalf("formal review_submit failed: %+v", result)
	}
	if _, err := os.Stat(recordPath); err != nil {
		t.Fatal(err)
	}
	replay := tools.Call(ctx, "review_submit", "review-submit-replay", raw)
	if replay.Error != "" || replay.Receipt == nil || replay.Receipt.ID != result.Receipt.ID {
		t.Fatalf("review submit was not idempotent: first=%+v replay=%+v", result, replay)
	}
	if denied := tools.Call(ctx, "workspace_replace", "forbidden", []byte(`{}`)); denied.Error == "" {
		t.Fatal("frozen reviewer exposed subject writer")
	}
}

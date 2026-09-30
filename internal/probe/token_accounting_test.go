// pattern: Imperative Shell
package probe

import (
	"os"
	"path/filepath"
	"polis/internal/codex"
	"runtime"
	"testing"
)

func TestContinuation4TokenAccountingIsObservableWithoutOptimization(t *testing.T) {
	_, file, _, _ := runtime.Caller(0)
	root := filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
	path := filepath.Join(root, "evidence", "development", "r0.3a-real-backend-employee", "continuation-4", "8bfd303beafe6fefd25337a2bd1ef77d", "protocol.jsonl")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	accounting, err := AccountProtocolJSONL(raw, continuation4ObservedUsage())
	if err != nil {
		t.Fatal(err)
	}
	if accounting.ProtocolBytes != int64(len(raw)) || accounting.RepeatedWorkspacePayloadBytes <= 0 || accounting.RepeatedToolResultBytes <= 0 || accounting.AccumulatedConversationBytes <= 0 || accounting.LastUncachedInputTokens != 38106 {
		t.Fatalf("unexpected continuation-4 accounting: %+v", accounting)
	}
}

func continuation4ObservedUsage() (usage codex.TokenUsage) {
	usage.Last.TotalTokens = 465971
	usage.Last.InputTokens = 461018
	usage.Last.CachedInputTokens = 422912
	usage.Last.OutputTokens = 4953
	usage.Last.ReasoningOutputTokens = 2383
	return usage
}

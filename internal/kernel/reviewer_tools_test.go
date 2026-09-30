// pattern: Imperative Shell
package kernel

import (
	"context"
	"polis/internal/core"
	"testing"
)

func TestReviewerToolsRejectCandidateWritesBeforeKernelAccess(t *testing.T) {
	tools := ReviewerTools{}
	for _, name := range []string{"workspace_replace", "artifact_submit"} {
		result := tools.Call(context.Background(), name, "write", []byte(`{}`))
		if result.Error != string(core.Denied) {
			t.Fatalf("reviewer write %s returned %q", name, result.Error)
		}
	}
}

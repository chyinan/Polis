// pattern: Functional Core
package provider

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestSummarizeProductSurfaceDiagnosticProtocolCapturesOneExactZeroToolTurn(t *testing.T) {
	tools, err := json.Marshal(ProductToolSurface().Tools)
	if err != nil {
		t.Fatal(err)
	}
	lines := []string{
		`{"time":"2026-09-17T01:00:00Z","direction":"send","data":{"id":1,"method":"initialize","params":{}}}`,
		`{"time":"2026-09-17T01:00:00.010Z","direction":"receive","data":{"id":1,"result":{"userAgent":"codex-cli 0.154.0-alpha.6.2"}}}`,
		`{"time":"2026-09-17T01:00:00.020Z","direction":"send","data":{"id":2,"method":"thread/start","params":{"dynamicTools":` + string(tools) + `}}}`,
		`{"time":"2026-09-17T01:00:00.030Z","direction":"receive","data":{"id":2,"result":{"thread":{"id":"thread-1"}}}}`,
		`{"time":"2026-09-17T01:00:00.100Z","direction":"send","data":{"id":3,"method":"turn/start","params":{"threadId":"thread-1"}}}`,
		`{"time":"2026-09-17T01:00:00.350Z","direction":"receive","data":{"method":"item/agentMessage/delta","params":{"delta":"POLIS_PRODUCT_SURFACE_V4_CANARY_OK"}}}`,
		`{"time":"2026-09-17T01:00:00.500Z","direction":"receive","data":{"method":"turn/completed","params":{"turn":{"status":"completed"}}}}`,
	}
	summary, err := SummarizeProductSurfaceDiagnosticProtocol([]byte(strings.Join(lines, "\n") + "\n"))
	if err != nil {
		t.Fatal(err)
	}
	if summary.InitializeRequestCount != 1 || summary.InitializeResponseCount != 1 || summary.ThreadStartRequestCount != 1 || summary.ThreadStartResponseCount != 1 || summary.RegisteredToolCount != 7 || summary.RegisteredToolsManifestDigest != ProductToolSurface().ManifestDigest || summary.TurnStartCount != 1 || summary.TurnCompletedCount != 1 || summary.ToolCallEventCount != 0 {
		t.Fatalf("unexpected diagnostic protocol summary: %+v", summary)
	}
	if summary.FirstOutput != ProductSurfaceDiagnosticCanaryOutput || summary.FirstOutputAt.IsZero() || summary.TurnStartAt.IsZero() || summary.TurnCompletedAt.IsZero() {
		t.Fatalf("diagnostic output/terminal evidence missing: %+v", summary)
	}
	if got := summary.FirstOutputAt.Sub(summary.TurnStartAt); got != 250*time.Millisecond {
		t.Fatalf("time to first output=%s want 250ms", got)
	}
}

func TestSummarizeProductSurfaceDiagnosticProtocolCountsAttemptedToolCalls(t *testing.T) {
	lines := []string{
		`{"time":"2026-09-17T01:00:00Z","direction":"receive","data":{"method":"item/tool/call","params":{"tool":"polis_workspace_read"}}}`,
	}
	summary, err := SummarizeProductSurfaceDiagnosticProtocol([]byte(strings.Join(lines, "\n")))
	if err != nil {
		t.Fatal(err)
	}
	if summary.ToolCallEventCount != 1 {
		t.Fatalf("tool call events=%d want=1", summary.ToolCallEventCount)
	}
}

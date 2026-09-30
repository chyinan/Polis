// pattern: Imperative Shell
package control

import (
	"context"
	"testing"

	"polis/internal/kernel"
	"polis/internal/provider"
)

func TestRealProviderAdapterRejectsUnqualifiedLive2RuntimeBeforeMissionStart(t *testing.T) {
	runtime := provider.NewFakeRuntime(provider.FakeRuntimeConfig{
		Model: "gpt-5.6-luna", Effort: "medium", Purpose: provider.Live2AuthorizationPurpose,
		ExecutionEnvelope: "stale-runtime-envelope@1", ToolCallLimit: 16,
	})
	adapter, err := NewRealProviderWorkerAdapter(new(kernel.Kernel), runtime)
	if err != nil {
		t.Fatal(err)
	}
	if err = adapter.Readiness(context.Background()); err == nil {
		t.Fatal("RealProviderWorkerAdapter accepted a stale LIVE_2 runtime envelope")
	}
}

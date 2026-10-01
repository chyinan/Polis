// pattern: Imperative Shell
package main

import (
	"context"
	"testing"

	"polis/internal/kernel"
	"polis/internal/provider"
)

func TestBuildWorkerAdapterAllowsAutoDispatchOnlyForFakeProductSurfaceV7(t *testing.T) {
	t.Setenv("POLIS_WORKER_MODE", "real")
	t.Setenv("POLIS_PROVIDER_TRANSPORT", "fake")
	t.Setenv("POLIS_OFFLINE_DIRECT_MESSAGING_ENABLED", "1")
	t.Setenv("POLIS_AUTO_WORKER_DISPATCH_ENABLED", "1")
	t.Setenv("POLIS_CONTROLLED_MCP_TOOL_SURFACE", "")
	t.Setenv("POLIS_CONTROLLED_MCP_TOOL_SURFACE_V2", "")
	t.Setenv("POLIS_PROVIDER_PURPOSE", "")

	adapter, err := buildWorkerAdapter(&kernel.Kernel{})
	if err != nil {
		t.Fatal(err)
	}
	defer adapter.Close()
	if adapter.ToolSurface().ManifestDigest != provider.ProductDirectMessagingToolSurface().ManifestDigest {
		t.Fatalf("selected surface digest=%s, want Fake @7 digest %s", adapter.ToolSurface().ManifestDigest, provider.ProductDirectMessagingToolSurface().ManifestDigest)
	}
	if err = adapter.Readiness(context.Background()); err != nil {
		t.Fatalf("Fake @7 readiness: %v", err)
	}
	gate, ok := adapter.(interface {
		AutomaticProductDispatchReadiness(context.Context) error
	})
	if !ok {
		t.Fatal("selected adapter does not expose the automatic-dispatch readiness gate")
	}
	if err = gate.AutomaticProductDispatchReadiness(context.Background()); err != nil {
		t.Fatalf("Fake @7 automatic-dispatch readiness: %v", err)
	}
}

func TestBuildWorkerAdapterRejectsAutoDispatchWithoutOfflineFakeSurface(t *testing.T) {
	t.Setenv("POLIS_WORKER_MODE", "real")
	t.Setenv("POLIS_PROVIDER_TRANSPORT", "fake")
	t.Setenv("POLIS_OFFLINE_DIRECT_MESSAGING_ENABLED", "")
	t.Setenv("POLIS_AUTO_WORKER_DISPATCH_ENABLED", "1")
	if _, err := buildWorkerAdapter(&kernel.Kernel{}); err == nil {
		t.Fatal("automatic dispatch without the explicit offline Fake @7 surface was accepted")
	}

	t.Setenv("POLIS_OFFLINE_DIRECT_MESSAGING_ENABLED", "1")
	t.Setenv("POLIS_PROVIDER_TRANSPORT", "codex")
	if _, err := buildWorkerAdapter(&kernel.Kernel{}); err == nil {
		t.Fatal("automatic dispatch with the real provider transport was accepted")
	}
}

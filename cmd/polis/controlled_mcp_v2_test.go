// pattern: Imperative Shell
package main

import (
	"context"
	"testing"

	"polis/internal/kernel"
	"polis/internal/provider"
)

func TestBuildWorkerAdapterSelectsOfflineMCPV2WithoutAppContainer(t *testing.T) {
	t.Setenv("POLIS_WORKER_MODE", "real")
	t.Setenv("POLIS_PROVIDER_TRANSPORT", "fake")
	t.Setenv("POLIS_CONTROLLED_MCP_TOOL_SURFACE", "")
	t.Setenv("POLIS_CONTROLLED_MCP_TOOL_SURFACE_V2", "1")
	t.Setenv("POLIS_MCP_APP_CONTAINER_ID", "")
	t.Setenv("POLIS_MCP_STREAMABLE_HTTP_ENABLED", "")

	adapter, err := buildWorkerAdapter(&kernel.Kernel{})
	if err != nil {
		t.Fatal(err)
	}
	defer adapter.Close()
	if adapter.ToolSurface().ManifestDigest != provider.ProductControlledMCPToolSurfaceV2().ManifestDigest {
		t.Fatalf("selected MCP v2 tool surface=%+v", adapter.ToolSurface())
	}
	if err = adapter.Readiness(context.Background()); err != nil {
		t.Fatalf("offline MCP v2 Worker readiness without AppContainer: %v", err)
	}
}

func TestBuildWorkerAdapterRejectsMixedOrRealMCPV2Surface(t *testing.T) {
	t.Setenv("POLIS_WORKER_MODE", "real")
	t.Setenv("POLIS_PROVIDER_TRANSPORT", "fake")
	t.Setenv("POLIS_CONTROLLED_MCP_TOOL_SURFACE", "1")
	t.Setenv("POLIS_CONTROLLED_MCP_TOOL_SURFACE_V2", "1")
	if _, err := buildWorkerAdapter(&kernel.Kernel{}); err == nil {
		t.Fatal("mixed MCP tool-surface versions were accepted")
	}

	t.Setenv("POLIS_CONTROLLED_MCP_TOOL_SURFACE", "")
	t.Setenv("POLIS_PROVIDER_TRANSPORT", "codex")
	if _, err := buildWorkerAdapter(&kernel.Kernel{}); err == nil {
		t.Fatal("unqualified real-provider MCP v2 surface was accepted")
	}
}

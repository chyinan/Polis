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

func TestBuildWorkerAdapterEnablesProjectJobsOnlyOnIsolatedFakeSurface(t *testing.T) {
	t.Setenv("POLIS_WORKER_MODE", "real")
	t.Setenv("POLIS_PROVIDER_TRANSPORT", "fake")
	t.Setenv("POLIS_PROVIDER_PURPOSE", "")
	t.Setenv("POLIS_OFFLINE_PROJECT_JOBS_ENABLED", "1")
	for _, name := range []string{
		"POLIS_OFFLINE_DIRECT_MESSAGING_ENABLED", "POLIS_OFFLINE_SHARED_ARTIFACTS_ENABLED",
		"POLIS_OFFLINE_SKILL_DIRECTORY_ENABLED", "POLIS_OFFLINE_WORKSPACE_TREE_ENABLED",
		"POLIS_OFFLINE_MISSION_CHANGE_ASSESSMENT_ENABLED", "POLIS_OFFLINE_ENVIRONMENT_STATUS_ENABLED",
		"POLIS_AUTO_WORKER_DISPATCH_ENABLED", "POLIS_CONTROLLED_MCP_TOOL_SURFACE", "POLIS_CONTROLLED_MCP_TOOL_SURFACE_V2",
	} {
		t.Setenv(name, "")
	}
	adapter, err := buildWorkerAdapter(&kernel.Kernel{})
	if err != nil {
		t.Fatal(err)
	}
	defer adapter.Close()
	if adapter.ToolSurface().ManifestDigest != provider.ProductProjectJobsToolSurface().ManifestDigest {
		t.Fatalf("selected surface digest=%s, want isolated Fake @20 digest %s", adapter.ToolSurface().ManifestDigest, provider.ProductProjectJobsToolSurface().ManifestDigest)
	}
	if err = adapter.Readiness(context.Background()); err != nil {
		t.Fatalf("Fake @20 readiness: %v", err)
	}

	t.Setenv("POLIS_PROVIDER_TRANSPORT", "codex")
	if _, err = buildWorkerAdapter(&kernel.Kernel{}); err == nil {
		t.Fatal("project-job Worker tools were accepted with a real Provider transport")
	}
	t.Setenv("POLIS_PROVIDER_TRANSPORT", "fake")
	t.Setenv("POLIS_OFFLINE_DIRECT_MESSAGING_ENABLED", "1")
	if _, err = buildWorkerAdapter(&kernel.Kernel{}); err == nil {
		t.Fatal("project-job and direct-messaging tool surfaces were combined")
	}
}

func TestBuildWorkerAdapterEnablesEnvironmentEnsureOnlyOnIsolatedFakeSurface(t *testing.T) {
	t.Setenv("POLIS_WORKER_MODE", "real")
	t.Setenv("POLIS_PROVIDER_TRANSPORT", "fake")
	t.Setenv("POLIS_PROVIDER_PURPOSE", "")
	t.Setenv("POLIS_OFFLINE_ENVIRONMENT_ENSURE_ENABLED", "1")
	for _, name := range []string{
		"POLIS_OFFLINE_DIRECT_MESSAGING_ENABLED", "POLIS_OFFLINE_SHARED_ARTIFACTS_ENABLED",
		"POLIS_OFFLINE_SKILL_DIRECTORY_ENABLED", "POLIS_OFFLINE_WORKSPACE_TREE_ENABLED",
		"POLIS_OFFLINE_MISSION_CHANGE_ASSESSMENT_ENABLED", "POLIS_OFFLINE_ENVIRONMENT_STATUS_ENABLED",
		"POLIS_OFFLINE_PROJECT_JOBS_ENABLED", "POLIS_AUTO_WORKER_DISPATCH_ENABLED",
		"POLIS_CONTROLLED_MCP_TOOL_SURFACE", "POLIS_CONTROLLED_MCP_TOOL_SURFACE_V2",
	} {
		t.Setenv(name, "")
	}
	adapter, err := buildWorkerAdapter(&kernel.Kernel{})
	if err != nil {
		t.Fatal(err)
	}
	defer adapter.Close()
	if adapter.ToolSurface().ManifestDigest != provider.ProductEnvironmentEnsureToolSurface().ManifestDigest {
		t.Fatalf("selected surface digest=%s, want Fake @16 digest %s", adapter.ToolSurface().ManifestDigest, provider.ProductEnvironmentEnsureToolSurface().ManifestDigest)
	}
	if err = adapter.Readiness(context.Background()); err != nil {
		t.Fatalf("Fake @16 readiness: %v", err)
	}

	t.Setenv("POLIS_PROVIDER_TRANSPORT", "codex")
	if _, err = buildWorkerAdapter(&kernel.Kernel{}); err == nil {
		t.Fatal("environment ensure Worker tools were accepted with a real Provider transport")
	}
	t.Setenv("POLIS_PROVIDER_TRANSPORT", "fake")
	t.Setenv("POLIS_OFFLINE_PROJECT_JOBS_ENABLED", "1")
	if _, err = buildWorkerAdapter(&kernel.Kernel{}); err == nil {
		t.Fatal("environment ensure and project JobRun tool surfaces were combined")
	}
}

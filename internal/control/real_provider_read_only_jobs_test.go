// pattern: Functional Core
package control

import (
	"context"
	"testing"

	"polis/internal/provider"
)

func TestReadOnlyJobsFakeSurfacePassesAdapterReadinessWithoutRealProvider(t *testing.T) {
	runtime := provider.NewFakeRuntime(provider.FakeRuntimeConfig{ReadOnlyJobsSurface: true})
	adapter, err := NewRealProviderWorkerLaunchQualificationAdapter(runtime)
	if err != nil {
		t.Fatal(err)
	}
	if err := adapter.Readiness(context.Background()); err != nil {
		t.Fatalf("fake read-only jobs adapter readiness: %v", err)
	}
}

func TestBorrowerLeaseFakeSurfacePassesAdapterReadinessWithoutRealProvider(t *testing.T) {
	runtime := provider.NewFakeRuntime(provider.FakeRuntimeConfig{BorrowerLeaseSurface: true})
	adapter, err := NewRealProviderWorkerLaunchQualificationAdapter(runtime)
	if err != nil {
		t.Fatal(err)
	}
	if err := adapter.Readiness(context.Background()); err != nil {
		t.Fatalf("fake borrower lease adapter readiness: %v", err)
	}
}

func TestBrowserRunFakeSurfacePassesAdapterReadinessWithoutRealProvider(t *testing.T) {
	runtime := provider.NewFakeRuntime(provider.FakeRuntimeConfig{BrowserRunSurface: true})
	adapter, err := NewRealProviderWorkerLaunchQualificationAdapter(runtime)
	if err != nil {
		t.Fatal(err)
	}
	if err := adapter.Readiness(context.Background()); err != nil {
		t.Fatalf("fake BrowserRun adapter readiness: %v", err)
	}
}

func TestEnvironmentEnsureFakeSurfacePassesAdapterReadinessWithoutRealProvider(t *testing.T) {
	runtime := provider.NewFakeRuntime(provider.FakeRuntimeConfig{EnvironmentEnsureSurface: true})
	adapter, err := NewRealProviderWorkerLaunchQualificationAdapter(runtime)
	if err != nil {
		t.Fatal(err)
	}
	if err := adapter.Readiness(context.Background()); err != nil {
		t.Fatalf("fake environment ensure adapter readiness: %v", err)
	}
}

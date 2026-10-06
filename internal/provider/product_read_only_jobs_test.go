// pattern: Functional Core
package provider

import (
	"context"
	"testing"
)

func TestProductBorrowerLeaseSurfaceIsFakeOnlyAndBounded(t *testing.T) {
	runtime := NewFakeRuntime(FakeRuntimeConfig{BorrowerLeaseSurface: true})
	surface := ProductBorrowerLeaseToolSurface()
	profile := runtime.ExecutionProfile()
	if surface.ToolCount != 12 || profile.ToolSurfaceQualification != ProductBorrowerLeaseToolSurfaceQualification {
		t.Fatalf("borrower lease surface/profile=%+v/%+v, want 12/@17", surface, profile)
	}
	if err := runtime.Readiness(context.Background()); err != nil {
		t.Fatalf("explicit borrower lease Fake Runtime readiness: %v", err)
	}
	authorization := validProviderExecutionAuthorization("offline-model", "fake", profile.Purpose, profile.ExecutionEnvelope)
	authorization.ToolSurfaceDigest = surface.ManifestDigest
	authorization.ToolSurfaceQualification = profile.ToolSurfaceQualification
	authorization.ToolCount = surface.ToolCount
	authorization.AggregateSchemaBytes = surface.AggregateSchemaBytes
	authorization.AggregateSchemaDigest = surface.AggregateSchemaDigest
	authorization.ExactSurfaceExecutionFingerprint = profile.ExactSurfaceExecutionFingerprint
	authorization.ProductProviderL2Fingerprint = profile.ProductProviderL2Fingerprint
	if err := ValidateRuntimeExecutionAuthorization(authorization); err != nil {
		t.Fatalf("exact fake borrower lease authorization was rejected: %v", err)
	}
	realAuthorization := authorization
	realAuthorization.ProviderMode = "real"
	if err := ValidateRuntimeExecutionAuthorization(realAuthorization); err == nil {
		t.Fatal("real provider accepted fake-only borrower lease surface")
	}
	if err := ValidateOfflineFakeBorrowerLeaseSurface("real", profile, surface); err == nil {
		t.Fatal("borrower lease surface passed the non-fake validator")
	}
}

func TestProductReadOnlyJobsSurfaceIsFakeOnlyAndTaskBound(t *testing.T) {
	runtime := NewFakeRuntime(FakeRuntimeConfig{ReadOnlyJobsSurface: true})
	surface := ProductReadOnlyJobsToolSurface()
	profile := runtime.ExecutionProfile()
	if surface.ToolCount != 9 {
		t.Fatalf("read-only jobs tool count=%d, want 9", surface.ToolCount)
	}
	if err := runtime.Readiness(context.Background()); err != nil {
		t.Fatalf("explicit read-only jobs Fake Runtime readiness: %v", err)
	}
	authorization := validProviderExecutionAuthorization("offline-model", "fake", profile.Purpose, profile.ExecutionEnvelope)
	authorization.ToolSurfaceDigest = surface.ManifestDigest
	authorization.ToolSurfaceQualification = profile.ToolSurfaceQualification
	authorization.ToolCount = surface.ToolCount
	authorization.AggregateSchemaBytes = surface.AggregateSchemaBytes
	authorization.AggregateSchemaDigest = surface.AggregateSchemaDigest
	authorization.ExactSurfaceExecutionFingerprint = profile.ExactSurfaceExecutionFingerprint
	authorization.ProductProviderL2Fingerprint = profile.ProductProviderL2Fingerprint
	if err := ValidateRuntimeExecutionAuthorization(authorization); err != nil {
		t.Fatalf("exact fake read-only jobs authorization was rejected: %v", err)
	}
	realAuthorization := authorization
	realAuthorization.ProviderMode = "real"
	if err := ValidateRuntimeExecutionAuthorization(realAuthorization); err == nil {
		t.Fatal("real provider accepted the fake-only read-only jobs surface")
	}
	if err := ValidateOfflineFakeReadOnlyJobsSurface("real", profile, surface); err == nil {
		t.Fatal("read-only jobs surface passed the non-fake validator")
	}
}

func TestProductEnvironmentEnsureSurfaceIsFakeOnlyAndIncludesStatus(t *testing.T) {
	runtime := NewFakeRuntime(FakeRuntimeConfig{EnvironmentEnsureSurface: true})
	surface := ProductEnvironmentEnsureToolSurface()
	profile := runtime.ExecutionProfile()
	if surface.ToolCount != 9 || profile.ToolSurfaceQualification != ProductEnvironmentEnsureToolSurfaceQualification {
		t.Fatalf("environment ensure surface/profile=%+v/%+v, want 9/@16", surface, profile)
	}
	if err := runtime.Readiness(context.Background()); err != nil {
		t.Fatalf("explicit environment ensure Fake Runtime readiness: %v", err)
	}
	authorization := validProviderExecutionAuthorization("offline-model", "fake", profile.Purpose, profile.ExecutionEnvelope)
	authorization.ToolSurfaceDigest = surface.ManifestDigest
	authorization.ToolSurfaceQualification = profile.ToolSurfaceQualification
	authorization.ToolCount = surface.ToolCount
	authorization.AggregateSchemaBytes = surface.AggregateSchemaBytes
	authorization.AggregateSchemaDigest = surface.AggregateSchemaDigest
	authorization.ExactSurfaceExecutionFingerprint = profile.ExactSurfaceExecutionFingerprint
	authorization.ProductProviderL2Fingerprint = profile.ProductProviderL2Fingerprint
	if err := ValidateRuntimeExecutionAuthorization(authorization); err != nil {
		t.Fatalf("exact fake environment ensure authorization was rejected: %v", err)
	}
	if err := ValidateRuntimeExecutionAuthorization(func() ExecutionAuthorization { copy := authorization; copy.ProviderMode = "real"; return copy }()); err == nil {
		t.Fatal("real provider accepted the fake-only environment ensure surface")
	}
}

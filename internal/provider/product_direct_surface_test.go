// pattern: Functional Core
package provider

import (
	"context"
	"testing"
)

func TestProductDirectMessagingSurfaceIsPinnedAndFakeOnly(t *testing.T) {
	runtime := NewFakeRuntime(FakeRuntimeConfig{DirectMessagingSurface: true})
	surface := ProductDirectMessagingToolSurface()
	profile := runtime.ExecutionProfile()
	if surface.ManifestDigest != ProductDirectMessagingManifestDigest || surface.AggregateSchemaBytes != ProductDirectMessagingSchemaBytes || surface.AggregateSchemaDigest != ProductDirectMessagingSchemaDigest {
		t.Fatalf("direct-message manifest/schema drifted: count=%d manifest=%s schema_bytes=%d schema=%s", surface.ToolCount, surface.ManifestDigest, surface.AggregateSchemaBytes, surface.AggregateSchemaDigest)
	}
	if err := runtime.Readiness(context.Background()); err != nil {
		t.Fatalf("explicit direct-message Fake Runtime readiness: %v", err)
	}
	if surface.ToolCount != 12 || profile.ToolSurfaceQualification != "polis-product-tool-surface@7" || profile.Purpose != OfflineDirectMessagingToolSurfacePurpose {
		t.Fatalf("surface/profile not pinned to the direct-message fake profile: surface=%+v profile=%+v", surface, profile)
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
		t.Fatalf("exact fake direct-message authorization was rejected: %v", err)
	}
	if err := ValidateOfflineFakeProductDirectMessagingSurface("real", profile, surface); err == nil {
		t.Fatal("the direct-message surface passed the real-provider gate")
	}
	realAuthorization := authorization
	realAuthorization.ProviderMode = "real"
	if err := ValidateRuntimeExecutionAuthorization(realAuthorization); err == nil {
		t.Fatal("a real provider accepted the unqualified direct-message surface")
	}
	changed := authorization
	changed.ToolSurfaceDigest = ProductToolSurface().ManifestDigest
	if err := ValidateRuntimeExecutionAuthorization(changed); err == nil {
		t.Fatal("fake direct-message authorization accepted a mismatched base-surface digest")
	}
	if err := NewFakeRuntime(FakeRuntimeConfig{DirectMessagingSurface: true, ReadOnlySkillSurface: true}).Readiness(context.Background()); err == nil {
		t.Fatal("mixed direct-message and Skill surfaces were accepted")
	}
}

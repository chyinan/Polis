// pattern: Functional Core
package provider

import "testing"

func TestCurrentProductSurfaceDiagnosticContractIsV4(t *testing.T) {
	if ProductToolSurfaceQualification != "polis-product-tool-surface@4" {
		t.Fatalf("surface qualification=%q want polis-product-tool-surface@4", ProductToolSurfaceQualification)
	}
	if ProductSurfaceDiagnosticPurpose != "PRODUCT_SURFACE_V4_LIVE_QUALIFICATION" {
		t.Fatalf("diagnostic purpose=%q want PRODUCT_SURFACE_V4_LIVE_QUALIFICATION", ProductSurfaceDiagnosticPurpose)
	}
	if ProductSurfaceDiagnosticInstructionRevision != "product-surface-v4-diagnostic-instructions@1" {
		t.Fatalf("instruction revision=%q want product-surface-v4-diagnostic-instructions@1", ProductSurfaceDiagnosticInstructionRevision)
	}
	if ProductSurfaceDiagnosticCanaryOutput != "POLIS_PRODUCT_SURFACE_V4_CANARY_OK" {
		t.Fatalf("canary output=%q want POLIS_PRODUCT_SURFACE_V4_CANARY_OK", ProductSurfaceDiagnosticCanaryOutput)
	}
	if ProductSurfaceDiagnosticPrompt != "Respond with exactly POLIS_PRODUCT_SURFACE_V4_CANARY_OK." {
		t.Fatalf("diagnostic prompt=%q", ProductSurfaceDiagnosticPrompt)
	}
}

func TestProductSurfaceDiagnosticAttempt2UsesFreshPurpose(t *testing.T) {
	base := productSurfaceDiagnosticAuthorizationFixture(t)
	authorization, err := NewProductSurfaceDiagnosticAuthorizationForPurpose(
		"r0.5b12-live-canary-1",
		"r05b12-product-surface-v4-canary",
		base.ExecutionIdentity,
		base.OfflineQualificationDigest,
		base.IssuedAt,
		ProductSurfaceDiagnosticAttempt2Purpose,
	)
	if err != nil {
		t.Fatal(err)
	}
	if authorization.Purpose != ProductSurfaceDiagnosticAttempt2Purpose {
		t.Fatalf("purpose=%q want %q", authorization.Purpose, ProductSurfaceDiagnosticAttempt2Purpose)
	}
	if err = ValidateProductSurfaceDiagnosticAuthorization(authorization, authorization); err != nil {
		t.Fatal(err)
	}
}

func TestProductSurfaceDiagnosticAttempt3UsesFreshPurpose(t *testing.T) {
	base := productSurfaceDiagnosticAuthorizationFixture(t)
	authorization, err := NewProductSurfaceDiagnosticAuthorizationForPurpose(
		"r0.5b14-live-canary-1",
		"r05b14-product-surface-v4-canary",
		base.ExecutionIdentity,
		base.OfflineQualificationDigest,
		base.IssuedAt,
		ProductSurfaceDiagnosticAttempt3Purpose,
	)
	if err != nil {
		t.Fatal(err)
	}
	if authorization.Purpose != ProductSurfaceDiagnosticAttempt3Purpose {
		t.Fatalf("purpose=%q want %q", authorization.Purpose, ProductSurfaceDiagnosticAttempt3Purpose)
	}
	if err = ValidateProductSurfaceDiagnosticAuthorization(authorization, authorization); err != nil {
		t.Fatal(err)
	}
}

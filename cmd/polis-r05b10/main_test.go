// pattern: Functional Core
package main

import (
	"path/filepath"
	"testing"

	"polis/internal/provider"
)

func TestR05B10RunnerBindsCurrentV4SurfaceAndSingleDiagnostic(t *testing.T) {
	if evidenceRootDefault != "evidence/development/r0.5b10-product-tool-surface-v4-live-qualification" {
		t.Fatalf("evidence root=%q", evidenceRootDefault)
	}
	if diagnosticID != "r0.5b10-live-canary-1" || diagnosticSessionID != "r05b10-product-surface-v4-canary" {
		t.Fatalf("diagnostic identity=%q/%q", diagnosticID, diagnosticSessionID)
	}
	if qualificationField != "product_surface_v4_live_l2" || providerSmokeEligibilityField != "eligible_for_future_product_sample" {
		t.Fatalf("qualification fields=%q/%q", qualificationField, providerSmokeEligibilityField)
	}
	surface := currentSurface()
	if err := validateExactSurface(surface); err != nil {
		t.Fatal(err)
	}
	if err := validateB9FrozenSurface(surface); err != nil {
		t.Fatal(err)
	}
	if provider.ProductToolSurfaceQualification != "polis-product-tool-surface@4" {
		t.Fatalf("provider surface qualification=%q", provider.ProductToolSurfaceQualification)
	}
}

func TestR05B10RunnerAllowsExplicitAttempt2DiagnosticIdentity(t *testing.T) {
	t.Setenv("POLIS_PRODUCT_SURFACE_V4_DIAGNOSTIC_ID", "r0.5b12-live-canary-1")
	t.Setenv("POLIS_PRODUCT_SURFACE_V4_DIAGNOSTIC_SESSION", "r05b12-product-surface-v4-canary")
	t.Setenv("POLIS_PRODUCT_SURFACE_V4_DIAGNOSTIC_PURPOSE", provider.ProductSurfaceDiagnosticAttempt2Purpose)
	t.Setenv("POLIS_PRODUCT_SURFACE_V4_OFFLINE_TEMP_PREFIX", "polis-r05b12-pg")
	t.Setenv("POLIS_PRODUCT_SURFACE_V4_DATABASE_PREFIX", "polis_r0_r05b12_")

	if got := diagnosticIDForRun(); got != "r0.5b12-live-canary-1" {
		t.Fatalf("diagnostic ID=%q", got)
	}
	if got := diagnosticSessionIDForRun(); got != "r05b12-product-surface-v4-canary" {
		t.Fatalf("diagnostic session=%q", got)
	}
	if got := diagnosticPurposeForRun(); got != provider.ProductSurfaceDiagnosticAttempt2Purpose {
		t.Fatalf("diagnostic purpose=%q", got)
	}
	if got := offlineTempPrefixForRun(); got != "polis-r05b12-pg" {
		t.Fatalf("offline temp prefix=%q", got)
	}
	if got := offlineDatabasePrefixForRun(); got != "polis_r0_r05b12_" {
		t.Fatalf("offline database prefix=%q", got)
	}
}

func TestR05B10RunnerOnlyAllowsKnownB11PreflightHomeReuse(t *testing.T) {
	t.Setenv("POLIS_PRODUCT_SURFACE_V4_REUSE_B11_PREFLIGHT_SESSION", "true")
	wantRoot := filepath.Join(".runtime", "windows", "r0.5b11-provider-runtime-version-binding-hardening")
	if !allowQualifiedB11PreflightSessionReuse(wantRoot, "r05b11-provider-preflight") {
		t.Fatal("known B11 preflight home was not recognized")
	}
	if allowQualifiedB11PreflightSessionReuse(filepath.Join(".runtime", "windows", "other"), "r05b11-provider-preflight") {
		t.Fatal("unrelated runtime root was accepted for preflight home reuse")
	}
	if allowQualifiedB11PreflightSessionReuse(wantRoot, "r05b12-product-surface-v4-canary") {
		t.Fatal("new diagnostic session was accepted for preflight home reuse")
	}
}

func TestR05B10RunnerBindsControlledRuntimeManifest(t *testing.T) {
	t.Setenv("POLIS_PROVIDER_RUNTIME_MANIFEST", `C:\controlled-runtime\runtime-manifest.json`)
	t.Setenv("POLIS_PRODUCT_SURFACE_V4_RUNTIME_ROOT", t.TempDir())

	config, err := loadDiagnosticConfig(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if config.RuntimeManifestPath != `C:\controlled-runtime\runtime-manifest.json` {
		t.Fatalf("runtime manifest=%q", config.RuntimeManifestPath)
	}
}

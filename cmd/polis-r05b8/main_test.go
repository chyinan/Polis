// pattern: Functional Core
package main

import "testing"

func TestR05B8RunnerBindsCurrentSurfaceAndSingleDiagnostic(t *testing.T) {
	if evidenceRootDefault != "evidence/development/r0.5b8-product-tool-surface-v3-live-qualification" {
		t.Fatalf("evidence root=%q", evidenceRootDefault)
	}
	if diagnosticID != "r0.5b8-live-canary-1" || diagnosticSessionID != "r05b8-product-surface-v3-canary" {
		t.Fatalf("diagnostic identity=%q/%q", diagnosticID, diagnosticSessionID)
	}
	if qualificationField != "product_surface_v3_live_l2" || providerSmokeEligibilityField != "eligible_for_future_product_sample" {
		t.Fatalf("qualification fields=%q/%q", qualificationField, providerSmokeEligibilityField)
	}
}

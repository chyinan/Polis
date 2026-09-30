// pattern: Imperative Shell
package probe

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"polis/internal/kernel"
	"strings"
)

type FrontendActivationPreflightReport struct {
	Status                         string                        `json:"status"`
	Reasons                        []string                      `json:"reasons,omitempty"`
	StartingState                  kernel.FrontendStartingState  `json:"starting_state"`
	RecoveryAnchors                kernel.PeerRecoveryAnchors    `json:"recovery_anchors"`
	AnchorProbe                    string                        `json:"anchor_probe"`
	CASReport                      kernel.CASSourceBindingReport `json:"cas_report"`
	BaselineBinding                kernel.RuntimeCASBinding      `json:"baseline_binding"`
	RuntimeBinding                 kernel.RuntimeCASBinding      `json:"runtime_binding"`
	BaselineManifestSHA256         string                        `json:"baseline_manifest_sha256,omitempty"`
	ExpectedBaselineManifestSHA256 string                        `json:"expected_baseline_manifest_sha256,omitempty"`
	Mutation                       bool                          `json:"mutation"`
}

func ValidateFrontendActivationCASBinding(baseline, runtime kernel.RuntimeCASBinding, report kernel.CASSourceBindingReport) []string {
	reasons := make([]string, 0)
	if err := runtime.ValidateSyntax(); err != nil {
		reasons = append(reasons, "runtime_cas_binding_invalid")
	}
	if err := baseline.ValidateSyntax(); err != nil {
		reasons = append(reasons, "baseline_cas_binding_missing")
	}
	if !runtimeCASBindingsEquivalent(baseline, runtime) {
		reasons = append(reasons, "baseline_cas_binding_mismatch")
	}
	if err := runtime.ValidateReport(report); err != nil {
		reasons = append(reasons, "runtime_cas_binding_report_mismatch")
	}
	return reasons
}

func runtimeCASBindingsEquivalent(baseline, runtime kernel.RuntimeCASBinding) bool {
	return canonicalCASRootIdentity(baseline.CanonicalRoot) == canonicalCASRootIdentity(runtime.CanonicalRoot) &&
		baseline.LayoutRevision == runtime.LayoutRevision &&
		baseline.CompanyNamespace == runtime.CompanyNamespace &&
		baseline.RequiredBlobInventoryDigest == runtime.RequiredBlobInventoryDigest &&
		baseline.RequiredBlobCount == runtime.RequiredBlobCount
}

func canonicalCASRootIdentity(path string) string {
	value := filepath.ToSlash(strings.ReplaceAll(strings.TrimSpace(path), "\\", "/"))
	value = strings.TrimRight(value, "/")
	lower := strings.ToLower(value)
	for _, prefix := range []string{"//wsl.localhost/ubuntu-22.04", "//wsl$/ubuntu-22.04"} {
		if strings.HasPrefix(lower, prefix) {
			value = value[len(prefix):]
			break
		}
	}
	if strings.HasPrefix(value, "/mnt/") && len(value) > len("/mnt/x/") {
		value = "/" + string(value[5]) + value[6:]
	}
	return strings.ToLower(filepath.ToSlash(value))
}

func FrontendActivationPreflight(ctx context.Context, dsn, baselineManifestPath, expectedBaselineManifestSHA256 string, binding kernel.RuntimeCASBinding, required []kernel.CASRequiredBlob) (FrontendActivationPreflightReport, error) {
	report := FrontendActivationPreflightReport{Status: "FRONTEND_ACTIVATION_PREFLIGHT_FAILED", RuntimeBinding: binding, Mutation: false}
	report.ExpectedBaselineManifestSHA256 = expectedBaselineManifestSHA256
	if err := binding.ValidateSyntax(); err != nil {
		report.Reasons = append(report.Reasons, "runtime_cas_binding_invalid")
		return report, err
	}
	state, err := kernel.ProbeFrontendStartingState(ctx, dsn)
	report.StartingState = state
	if err != nil {
		report.Reasons = append(report.Reasons, "starting_state_probe_unavailable")
		return report, err
	}
	report.Reasons = append(report.Reasons, kernel.ValidateFrontendStartingState(state)...)
	if len(report.Reasons) == 0 {
		anchors, anchorErr := kernel.ReadOnlyPeerRecoveryAnchors(ctx, dsn, state.CompanyID)
		report.RecoveryAnchors = anchors
		if anchorErr != nil {
			report.AnchorProbe = "FAILED"
			report.Reasons = append(report.Reasons, "recovery_anchor_probe_failed")
			return report, anchorErr
		}
		report.AnchorProbe = "PASSED"
	}
	raw, err := os.ReadFile(baselineManifestPath)
	if err != nil {
		report.Reasons = append(report.Reasons, "baseline_manifest_unavailable")
		return report, err
	}
	actualManifestSHA256 := sha256.Sum256(raw)
	report.BaselineManifestSHA256 = fmt.Sprintf("%x", actualManifestSHA256[:])
	if !validSHA256Binding(expectedBaselineManifestSHA256) {
		report.Reasons = append(report.Reasons, "baseline_manifest_hash_missing_or_invalid")
		return report, errors.New("baseline manifest hash binding is missing or invalid")
	}
	if !strings.EqualFold(report.BaselineManifestSHA256, expectedBaselineManifestSHA256) {
		report.Reasons = append(report.Reasons, "baseline_manifest_hash_mismatch")
		return report, fmt.Errorf("baseline manifest hash mismatch: got %s", report.BaselineManifestSHA256)
	}
	var baseline struct {
		RuntimeCASBinding kernel.RuntimeCASBinding `json:"runtime_cas_binding"`
	}
	if err := json.Unmarshal(raw, &baseline); err != nil || baseline.RuntimeCASBinding.ValidateSyntax() != nil {
		report.Reasons = append(report.Reasons, "baseline_cas_binding_missing")
		return report, errors.New("baseline manifest has no valid runtime CAS binding")
	}
	report.BaselineBinding = baseline.RuntimeCASBinding
	casReport, err := kernel.ValidateCASSourceBinding(binding.CanonicalRoot, required)
	report.CASReport = casReport
	if err != nil {
		report.Reasons = append(report.Reasons, "cas_binding_validation_failed")
		return report, err
	}
	report.Reasons = append(report.Reasons, ValidateFrontendActivationCASBinding(baseline.RuntimeCASBinding, binding, casReport)...)
	if len(report.Reasons) != 0 {
		return report, fmt.Errorf("frontend activation preflight rejected: %v", report.Reasons)
	}
	report.Status = "FRONTEND_ACTIVATION_PREFLIGHT_PASSED"
	return report, nil
}

func validSHA256Binding(value string) bool {
	if len(value) != sha256.Size*2 {
		return false
	}
	for _, r := range value {
		if !(r >= '0' && r <= '9') && !(r >= 'a' && r <= 'f') && !(r >= 'A' && r <= 'F') {
			return false
		}
	}
	return true
}

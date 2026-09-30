// pattern: Functional Core
package probe

import (
	"crypto/sha256"
	"encoding/hex"
	"polis/internal/kernel"
	"strings"
	"testing"
)

func TestValidateFrontendActivationCASBindingRejectsRootMismatch(t *testing.T) {
	report := kernel.CASSourceBindingReport{CanonicalRoot: "/baseline/blobs", SentinelKind: "directory", LayoutRevision: kernel.CASLayoutRevision, CompanyNamespace: "company", RequiredBlobCount: 1, InventoryDigest: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"}
	baseline := kernel.RuntimeCASBinding{CanonicalRoot: "/baseline/blobs", LayoutRevision: kernel.CASLayoutRevision, CompanyNamespace: "company", RequiredBlobInventoryDigest: report.InventoryDigest, RequiredBlobCount: 1}
	runtime := baseline
	runtime.CanonicalRoot = "/baseline/cas"
	reasons := ValidateFrontendActivationCASBinding(baseline, runtime, report)
	if len(reasons) != 2 || reasons[0] != "baseline_cas_binding_mismatch" || reasons[1] != "runtime_cas_binding_report_mismatch" {
		t.Fatalf("unexpected CAS binding reasons: %v", reasons)
	}
}

func TestFrontendActivationPreflightBaselineManifestHashBinding(t *testing.T) {
	content := []byte(`{"schema_version":"r03a-frontend-execution-baseline-manifest@1"}`)
	digest := sha256.Sum256(content)
	want := hex.EncodeToString(digest[:])
	if !validSHA256Binding(want) {
		t.Fatal("valid SHA-256 binding was rejected")
	}
	for _, invalid := range []string{"", "abc", "zz" + want[2:], want[:63]} {
		if validSHA256Binding(invalid) {
			t.Fatalf("invalid SHA-256 binding accepted: %q", invalid)
		}
	}
}

func TestFrontendActivationPreflightCASBindingRequiresExactManifestBinding(t *testing.T) {
	report := kernel.CASSourceBindingReport{CanonicalRoot: "/baseline/blobs", SentinelKind: "directory", LayoutRevision: kernel.CASLayoutRevision, CompanyNamespace: "company", RequiredBlobCount: 1, InventoryDigest: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"}
	binding := kernel.RuntimeCASBinding{CanonicalRoot: report.CanonicalRoot, LayoutRevision: report.LayoutRevision, CompanyNamespace: report.CompanyNamespace, RequiredBlobInventoryDigest: report.InventoryDigest, RequiredBlobCount: report.RequiredBlobCount}
	if reasons := ValidateFrontendActivationCASBinding(binding, binding, report); len(reasons) != 0 {
		t.Fatalf("exact CAS binding rejected: %v", reasons)
	}
	wrong := binding
	wrong.RequiredBlobInventoryDigest = "abcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcd"
	if reasons := ValidateFrontendActivationCASBinding(binding, wrong, report); len(reasons) != 2 || reasons[0] != "baseline_cas_binding_mismatch" || reasons[1] != "runtime_cas_binding_report_mismatch" {
		t.Fatalf("wrong CAS binding reasons=%v", reasons)
	}
}

func TestRuntimeCASBindingAcceptsExplicitWindowsWSLPathIdentity(t *testing.T) {
	baseline := kernel.RuntimeCASBinding{CanonicalRoot: "/home/chyinan/.local/state/polis-recovery/baseline/blobs", LayoutRevision: kernel.CASLayoutRevision, CompanyNamespace: "company", RequiredBlobInventoryDigest: strings.Repeat("a", 64), RequiredBlobCount: 4}
	runtime := baseline
	runtime.CanonicalRoot = `\\wsl.localhost\Ubuntu-22.04\home\chyinan\.local\state\polis-recovery\baseline\blobs`
	report := kernel.CASSourceBindingReport{CanonicalRoot: runtime.CanonicalRoot, LayoutRevision: runtime.LayoutRevision, CompanyNamespace: runtime.CompanyNamespace, RequiredBlobCount: runtime.RequiredBlobCount, InventoryDigest: runtime.RequiredBlobInventoryDigest}
	if reasons := ValidateFrontendActivationCASBinding(baseline, runtime, report); len(reasons) != 0 {
		t.Fatalf("equivalent WSL/Windows CAS roots rejected: %v", reasons)
	}
}

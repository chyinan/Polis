// pattern: Imperative Shell
package control

import (
	"os"
	"path/filepath"
	"testing"
)

func TestProviderAuthReadinessReportsMissingCredentialPath(t *testing.T) {
	t.Setenv("POLIS_PROVIDER_AUTH_FILE", "")
	if got := providerAuthReadiness("codex"); got != "missing" {
		t.Fatalf("provider auth readiness=%q, want missing", got)
	}
}

func TestProviderAuthReadinessReportsConfiguredWithoutExposingCredentials(t *testing.T) {
	path := filepath.Join(t.TempDir(), "auth.json")
	if err := os.WriteFile(path, []byte(`{"auth_mode":"chatgpt","tokens":{"access_token":"fixture-secret","refresh_token":"fixture-refresh"}}`), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("POLIS_PROVIDER_AUTH_FILE", path)
	if got := providerAuthReadiness("codex"); got != "configured" {
		t.Fatalf("provider auth readiness=%q, want configured", got)
	}
	if got := providerAuthReadiness("fake"); got != "not_required" {
		t.Fatalf("fake provider auth readiness=%q, want not_required", got)
	}
}

func TestProviderAuthReadinessRejectsMalformedCredentialWithoutLeakingContents(t *testing.T) {
	path := filepath.Join(t.TempDir(), "auth.json")
	if err := os.WriteFile(path, []byte(`{"auth_mode":"chatgpt","tokens":{"access_token":"secret-material"`), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("POLIS_PROVIDER_AUTH_FILE", path)
	if got := providerAuthReadiness("codex"); got != "invalid" {
		t.Fatalf("provider auth readiness=%q, want invalid", got)
	}
}

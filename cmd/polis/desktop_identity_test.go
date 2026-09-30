package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"polis/db"
	"polis/internal/desktop"
)

func TestDesktopIdentityUsesTheExecutableFileContents(t *testing.T) {
	path := filepath.Join(t.TempDir(), "polis.exe")
	contents := []byte("selected Polis backend bytes")
	if err := os.WriteFile(path, contents, 0o600); err != nil {
		t.Fatal(err)
	}

	identity, err := identifyDesktopExecutable(path)
	if err != nil {
		t.Fatalf("identify executable: %v", err)
	}
	expected := sha256.Sum256(contents)
	if identity.Service != "polis_backend" || identity.ExecutableSHA256 != hex.EncodeToString(expected[:]) {
		t.Fatalf("identity=%+v, expected executable SHA-256 %x", identity, expected)
	}
}

func TestDesktopIdentityEndpointRequiresTheSessionTokenAndReportsTheRunningImage(t *testing.T) {
	handler := desktop.Middleware("desktop-session", desktopIdentityHandler())
	unauthorized := httptest.NewRecorder()
	unauthorizedRequest := httptest.NewRequest(http.MethodGet, "/api/desktop/identity", nil)
	handler.ServeHTTP(unauthorized, unauthorizedRequest)
	if unauthorized.Code != http.StatusUnauthorized {
		t.Fatalf("missing token status=%d, want %d", unauthorized.Code, http.StatusUnauthorized)
	}

	authorized := httptest.NewRecorder()
	authorizedRequest := httptest.NewRequest(http.MethodGet, "/api/desktop/identity", nil)
	authorizedRequest.Header.Set("X-Polis-Desktop-Token", "desktop-session")
	handler.ServeHTTP(authorized, authorizedRequest)
	if authorized.Code != http.StatusOK {
		t.Fatalf("authorized status=%d body=%s", authorized.Code, authorized.Body.String())
	}
	var identity desktopExecutableIdentity
	if err := json.Unmarshal(authorized.Body.Bytes(), &identity); err != nil {
		t.Fatalf("decode identity: %v", err)
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	contents, err := os.ReadFile(executable)
	if err != nil {
		t.Fatal(err)
	}
	want := sha256.Sum256(contents)
	if identity.Service != "polis_backend" || identity.ExecutableSHA256 != hex.EncodeToString(want[:]) {
		t.Fatalf("identity=%+v, want running executable hash %x", identity, want)
	}
	if identity.MigrationManifestSHA256 != db.MigrationManifestSHA256() {
		t.Fatalf("migration manifest=%q, want %q", identity.MigrationManifestSHA256, db.MigrationManifestSHA256())
	}
}

func TestDesktopSidecarInfoIsMachineReadableAndCarriesEmbeddedMigrationManifest(t *testing.T) {
	var output bytes.Buffer
	if err := writeDesktopSidecarInfo(&output); err != nil {
		t.Fatalf("write sidecar info command report: %v", err)
	}
	var decoded map[string]string
	if err := json.Unmarshal(output.Bytes(), &decoded); err != nil {
		t.Fatalf("decode machine-readable sidecar info: %v", err)
	}
	if decoded["service"] != "polis_backend" || decoded["migration_manifest_sha256"] != db.MigrationManifestSHA256() {
		t.Fatalf("sidecar info=%s, want embedded manifest digest %s", output.Bytes(), db.MigrationManifestSHA256())
	}
}

func TestDesktopIdentityRejectsMissingAndNonRegularExecutables(t *testing.T) {
	if _, err := identifyDesktopExecutable(filepath.Join(t.TempDir(), "missing.exe")); err == nil {
		t.Fatal("missing executable was accepted")
	}
	if _, err := identifyDesktopExecutable(t.TempDir()); err == nil {
		t.Fatal("directory was accepted as an executable")
	}
}

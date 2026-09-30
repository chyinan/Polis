// pattern: Functional Core
package codex

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestParseAuthMaterialSeparatesIdentityAndCredentialRevision(t *testing.T) {
	raw := []byte(`{"auth_mode":"chatgpt","tokens":{"id_token":"` + testJWT(t, map[string]any{"iss": "issuer", "sub": "subject", "auth_provider": "openai"}) + `"},"last_refresh":"first"}`)
	material, err := ParseAuthMaterial(raw, "mounted_codex_auth_file")
	if err != nil {
		t.Fatal(err)
	}
	if material.SourceClass != "mounted_codex_auth_file" || material.IdentityFingerprintStatus != "available" || material.IdentityFingerprint == "unavailable" {
		t.Fatalf("identity material = %+v", material)
	}
	expected := sha256.Sum256(raw)
	if material.CredentialRevisionFingerprint != hex.EncodeToString(expected[:]) || material.CredentialRevisionFingerprintStatus != "available" {
		t.Fatalf("credential revision = %+v, want raw file SHA-256", material)
	}

	rotated := []byte(`{"auth_mode":"chatgpt","tokens":{"id_token":"` + testJWT(t, map[string]any{"iss": "issuer", "sub": "subject", "auth_provider": "openai"}) + `"},"last_refresh":"second"}`)
	rotatedMaterial, err := ParseAuthMaterial(rotated, "mounted_codex_auth_file")
	if err != nil {
		t.Fatal(err)
	}
	if rotatedMaterial.IdentityFingerprint != material.IdentityFingerprint {
		t.Fatal("stable identity changed when only credential metadata changed")
	}
	if rotatedMaterial.CredentialRevisionFingerprint == material.CredentialRevisionFingerprint {
		t.Fatal("credential revision did not change with auth material")
	}
}

func TestParseAuthMaterialMarksMissingIdentityUnavailable(t *testing.T) {
	raw := []byte(`{"auth_mode":"chatgpt","tokens":{"refresh_token":"opaque"}}`)
	material, err := ParseAuthMaterial(raw, "mounted_codex_auth_file")
	if err != nil {
		t.Fatal(err)
	}
	if material.IdentityFingerprintStatus != "unavailable" || material.IdentityFingerprint != "unavailable" {
		t.Fatalf("missing identity was not marked unavailable: %+v", material)
	}
	if material.CredentialRevisionFingerprintStatus != "available" {
		t.Fatalf("credential revision should remain available: %+v", material)
	}
}

func TestCanonicalManifestV2SeparatesAuthChangesAndStaleReasons(t *testing.T) {
	combination := ExecutionCombination{
		CodexVersion: "0.153.4", BinarySHA256: "binary", Model: "gpt-5.6-luna", Effort: "medium", RuntimeProfile: "wsl-linux-amd64", SandboxClass: "read-only", ProxyConfigDigest: "proxy", AuthSourceClass: "mounted_codex_auth_file", CodeModeHostSHA256: "host", CapabilityDigest: "cap", NativeProtocolDigest: "native",
	}
	base := AuthFingerprintManifest{
		AuthSourceClass:                                "mounted_codex_auth_file",
		AuthIdentityFingerprintSchemaVersion:           AuthIdentityFingerprintSchemaVersion,
		AuthIdentityFingerprint:                        "identity-a",
		AuthIdentityFingerprintStatus:                  "available",
		AuthCredentialRevisionFingerprintSchemaVersion: AuthCredentialRevisionFingerprintSchemaVersion,
		AuthCredentialRevisionFingerprint:              "revision-a",
		AuthCredentialRevisionFingerprintStatus:        "available",
	}
	identityChanged := base
	identityChanged.AuthIdentityFingerprint = "identity-b"
	revisionChanged := base
	revisionChanged.AuthCredentialRevisionFingerprint = "revision-b"
	baseDigest := combination.CanonicalManifestV2Digest(base)
	if baseDigest == combination.CanonicalManifestV2Digest(identityChanged) || baseDigest == combination.CanonicalManifestV2Digest(revisionChanged) {
		t.Fatal("v2 manifest ignored an auth-layer change")
	}
	comparison := CompareAuthManifests(base, identityChanged)
	if comparison.Identity != AuthComparisonDifferent || comparison.ReasonCode != AuthIdentityChangedReasonCode {
		t.Fatalf("identity stale classification = %+v", comparison)
	}
	comparison = CompareAuthManifests(base, revisionChanged)
	if comparison.Identity != AuthComparisonSame || comparison.CredentialRevision != AuthComparisonDifferent || comparison.ReasonCode != AuthCredentialRevisionChangedReasonCode {
		t.Fatalf("credential stale classification = %+v", comparison)
	}
}

func TestEvaluateBusinessGateV2UsesDistinctAuthStaleReasons(t *testing.T) {
	combination := ExecutionCombination{
		CodexVersion: "0.153.4", BinarySHA256: "binary", Model: "gpt-5.6-luna", Effort: "medium", RuntimeProfile: "wsl-linux-amd64", SandboxClass: "read-only", ProxyConfigDigest: "proxy", AuthSourceClass: "mounted_codex_auth_file", CodeModeHostSHA256: "host", CapabilityDigest: "cap", NativeProtocolDigest: "native",
	}
	auth := AuthFingerprintManifest{
		AuthSourceClass: "mounted_codex_auth_file", AuthIdentityFingerprintSchemaVersion: AuthIdentityFingerprintSchemaVersion, AuthIdentityFingerprint: strings.Repeat("a", 64), AuthIdentityFingerprintStatus: "available", AuthCredentialRevisionFingerprintSchemaVersion: AuthCredentialRevisionFingerprintSchemaVersion, AuthCredentialRevisionFingerprint: strings.Repeat("c", 64), AuthCredentialRevisionFingerprintStatus: "available",
	}
	record := QualificationRecordV2{Layer: QualificationL1BaseTransport, Status: QualificationQualified, EvidenceResult: EvidencePassed, FingerprintSchemaVersion: CanonicalManifestFingerprintSchemaVersionV2, CanonicalManifestDigest: combination.CanonicalManifestV2Digest(auth), Combination: combination, Auth: auth, CreatedAt: time.Unix(1, 0)}
	identityChanged := auth
	identityChanged.AuthIdentityFingerprint = strings.Repeat("b", 64)
	decision := EvaluateBusinessGateV2(time.Unix(2, 0), combination, identityChanged, []QualificationRecordV2{record})
	if decision.Status != QualificationStale || decision.Reason != AuthIdentityChangedReasonCode {
		t.Fatalf("identity stale decision = %+v", decision)
	}
	revisionChanged := auth
	revisionChanged.AuthCredentialRevisionFingerprint = strings.Repeat("d", 64)
	decision = EvaluateBusinessGateV2(time.Unix(2, 0), combination, revisionChanged, []QualificationRecordV2{record})
	if decision.Status != QualificationStale || decision.Reason != AuthCredentialRevisionChangedReasonCode {
		t.Fatalf("credential stale decision = %+v", decision)
	}
}

func TestLegacyT9AuthFieldIsReconciledAsCredentialRevision(t *testing.T) {
	historical := AuthFingerprintManifest{
		AuthSourceClass:                                "mounted_codex_auth_file",
		AuthIdentityFingerprintSchemaVersion:           AuthIdentityFingerprintSchemaVersion,
		AuthIdentityFingerprint:                        "unavailable",
		AuthIdentityFingerprintStatus:                  "unavailable",
		AuthCredentialRevisionFingerprintSchemaVersion: AuthCredentialRevisionFingerprintSchemaVersion,
		AuthCredentialRevisionFingerprint:              strings.Repeat("a", 64),
		AuthCredentialRevisionFingerprintStatus:        "available",
	}
	current := historical
	current.AuthIdentityFingerprint = strings.Repeat("b", 64)
	current.AuthIdentityFingerprintStatus = "available"
	current.AuthCredentialRevisionFingerprint = strings.Repeat("c", 64)
	comparison := CompareAuthManifests(historical, current)
	if comparison.IncomparableFingerprintTypes {
		t.Fatal("same credential-revision schema was marked incomparable")
	}
	if comparison.Identity != AuthComparisonNotReconstructable {
		t.Fatalf("identity comparison = %q", comparison.Identity)
	}
	if comparison.CredentialRevision != AuthComparisonDifferent || comparison.ReasonCode != AuthCredentialRevisionChangedReasonCode {
		t.Fatalf("credential comparison = %+v", comparison)
	}
}

func testJWT(t *testing.T, claims map[string]any) string {
	t.Helper()
	encode := func(value any) string {
		raw, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		return base64.RawURLEncoding.EncodeToString(raw)
	}
	return encode(map[string]string{"alg": "none", "typ": "JWT"}) + "." + encode(claims) + ".signature"
}

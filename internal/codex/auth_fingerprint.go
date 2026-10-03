// pattern: Functional Core
package codex

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"time"
	"unicode"
)

const (
	// CanonicalManifestFingerprintSchemaVersionV2 is opt-in. The v1 manifest
	// and its historical digests retain their original meaning.
	CanonicalManifestFingerprintSchemaVersionV2    = "canonical-manifest-v2"
	AuthIdentityFingerprintSchemaVersion           = "auth-identity-v1"
	AuthCredentialRevisionFingerprintSchemaVersion = "auth-credential-revision-v1"
	AuthIdentityChangedReasonCode                  = "auth_identity_changed"
	AuthCredentialRevisionChangedReasonCode        = "auth_credential_revision_changed"
	AuthIdentityNotReconstructableReasonCode       = "auth_identity_not_reconstructable"
	AuthFingerprintTypesIncomparableReasonCode     = "auth_fingerprint_types_incomparable"
)

type AuthMaterial struct {
	SourceClass                                string `json:"auth_source_class"`
	IdentityFingerprintSchemaVersion           string `json:"auth_identity_fingerprint_schema_version"`
	IdentityFingerprint                        string `json:"auth_identity_fingerprint"`
	IdentityFingerprintStatus                  string `json:"auth_identity_fingerprint_status"`
	CredentialRevisionFingerprintSchemaVersion string `json:"auth_credential_revision_fingerprint_schema_version"`
	CredentialRevisionFingerprint              string `json:"auth_credential_revision_fingerprint"`
	CredentialRevisionFingerprintStatus        string `json:"auth_credential_revision_fingerprint_status"`
}

type AuthFingerprintManifest struct {
	AuthSourceClass                                string `json:"auth_source_class"`
	AuthIdentityFingerprintSchemaVersion           string `json:"auth_identity_fingerprint_schema_version"`
	AuthIdentityFingerprint                        string `json:"auth_identity_fingerprint"`
	AuthIdentityFingerprintStatus                  string `json:"auth_identity_fingerprint_status"`
	AuthCredentialRevisionFingerprintSchemaVersion string `json:"auth_credential_revision_fingerprint_schema_version"`
	AuthCredentialRevisionFingerprint              string `json:"auth_credential_revision_fingerprint"`
	AuthCredentialRevisionFingerprintStatus        string `json:"auth_credential_revision_fingerprint_status"`
}

type AuthComparisonStatus string

const (
	AuthComparisonSame               AuthComparisonStatus = "SAME"
	AuthComparisonDifferent          AuthComparisonStatus = "DIFFERENT"
	AuthComparisonNotRecorded        AuthComparisonStatus = "NOT_RECORDED"
	AuthComparisonNotReconstructable AuthComparisonStatus = "NOT_RECONSTRUCTABLE"
	AuthComparisonIncomparable       AuthComparisonStatus = "INCOMPARABLE"
)

type AuthManifestComparison struct {
	SourceClass                  AuthComparisonStatus `json:"source_class"`
	Identity                     AuthComparisonStatus `json:"identity"`
	CredentialRevision           AuthComparisonStatus `json:"credential_revision"`
	ReasonCode                   string               `json:"reason_code,omitempty"`
	ReasonCodes                  []string             `json:"reason_codes,omitempty"`
	StrictExperimentComparable   bool                 `json:"strict_experiment_comparable"`
	IncomparableFingerprintTypes bool                 `json:"incomparable_fingerprint_types"`
}

// ParseAuthMaterial computes two intentionally different fingerprints. The
// credential revision is SHA-256 over the exact auth-file bytes. The identity
// fingerprint is SHA-256 over compact JSON containing only issuer, subject and
// optional auth-provider claims from a locally parsed id_token; claim values
// never leave this function.
func ParseAuthMaterial(raw []byte, sourceClass string) (AuthMaterial, error) {
	if sourceClass == "" {
		return AuthMaterial{}, errors.New("auth source class is required")
	}
	revision := sha256.Sum256(raw)
	material := AuthMaterial{
		SourceClass:                                sourceClass,
		IdentityFingerprintSchemaVersion:           AuthIdentityFingerprintSchemaVersion,
		IdentityFingerprint:                        "unavailable",
		IdentityFingerprintStatus:                  "unavailable",
		CredentialRevisionFingerprintSchemaVersion: AuthCredentialRevisionFingerprintSchemaVersion,
		CredentialRevisionFingerprint:              hex.EncodeToString(revision[:]),
		CredentialRevisionFingerprintStatus:        "available",
	}

	var document struct {
		Tokens struct {
			IDToken string `json:"id_token"`
		} `json:"tokens"`
	}
	if json.Unmarshal(raw, &document) != nil || document.Tokens.IDToken == "" {
		return material, nil
	}
	claims, ok := parseJWTClaims(document.Tokens.IDToken)
	if !ok {
		return material, nil
	}
	issuer, issuerOK := jsonStringClaim(claims, "iss")
	subject, subjectOK := jsonStringClaim(claims, "sub")
	if !issuerOK || !subjectOK || issuer == "" || subject == "" {
		return material, nil
	}
	authProvider, _ := jsonStringClaim(claims, "auth_provider")
	identityBytes, err := json.Marshal(struct {
		SchemaVersion string `json:"schema_version"`
		Issuer        string `json:"issuer"`
		Subject       string `json:"subject"`
		AuthProvider  string `json:"auth_provider,omitempty"`
	}{AuthIdentityFingerprintSchemaVersion, issuer, subject, authProvider})
	if err != nil {
		return material, nil
	}
	identity := sha256.Sum256(identityBytes)
	material.IdentityFingerprint = hex.EncodeToString(identity[:])
	material.IdentityFingerprintStatus = "available"
	return material, nil
}

// ParseChatGPTAccountIDFingerprint extracts Codex's selected ChatGPT account
// identifier from its auth file and immediately hashes it into an opaque,
// provider-scoped key. The raw account identifier is never returned to callers.
// This is an account locator only; it does not establish billing semantics.
func ParseChatGPTAccountIDFingerprint(raw []byte) (fingerprint, status, reasonCode string) {
	var document struct {
		AuthMode string `json:"auth_mode"`
		Tokens   struct {
			AccountID string `json:"account_id"`
			IDToken   string `json:"id_token"`
		} `json:"tokens"`
	}
	if json.Unmarshal(raw, &document) != nil {
		return "", "unavailable", "auth_file_unparseable"
	}
	if document.AuthMode != "chatgpt" {
		return "", "unsupported", "auth_mode_account_id_not_supported"
	}
	accountID := document.Tokens.AccountID
	if accountID == "" && document.Tokens.IDToken != "" {
		if claims, ok := parseJWTClaims(document.Tokens.IDToken); ok {
			if rawAuth, exists := claims["https://api.openai.com/auth"]; exists {
				var authClaims map[string]json.RawMessage
				if json.Unmarshal(rawAuth, &authClaims) == nil {
					accountID, _ = jsonStringClaim(authClaims, "chatgpt_account_id")
				}
			}
		}
	}
	if accountID == "" || strings.TrimSpace(accountID) != accountID || len(accountID) > 512 {
		return "", "unavailable", "chatgpt_account_id_not_reconstructable"
	}
	for _, char := range accountID {
		if unicode.IsControl(char) {
			return "", "unavailable", "chatgpt_account_id_not_reconstructable"
		}
	}
	identityBytes, err := json.Marshal(struct {
		SchemaVersion string `json:"schema_version"`
		Provider      string `json:"provider"`
		AccountID     string `json:"account_id"`
	}{"codex-chatgpt-account-id@1", "codex_chatgpt", accountID})
	if err != nil {
		return "", "unavailable", "chatgpt_account_id_not_reconstructable"
	}
	fingerprintBytes := sha256.Sum256(identityBytes)
	return hex.EncodeToString(fingerprintBytes[:]), "available", ""
}

func (m AuthFingerprintManifest) Validate() error {
	if m.AuthSourceClass == "" {
		return errors.New("auth source class is required")
	}
	if m.AuthIdentityFingerprintSchemaVersion != AuthIdentityFingerprintSchemaVersion {
		return errors.New("unsupported auth identity fingerprint schema")
	}
	if m.AuthIdentityFingerprintStatus == "unavailable" {
		if m.AuthIdentityFingerprint != "unavailable" {
			return errors.New("unavailable auth identity must use unavailable value")
		}
	} else if m.AuthIdentityFingerprintStatus != "available" || !isSHA256Hex(m.AuthIdentityFingerprint) {
		return errors.New("invalid auth identity fingerprint")
	}
	if m.AuthCredentialRevisionFingerprintSchemaVersion != AuthCredentialRevisionFingerprintSchemaVersion || m.AuthCredentialRevisionFingerprintStatus != "available" || !isSHA256Hex(m.AuthCredentialRevisionFingerprint) {
		return errors.New("invalid auth credential revision fingerprint")
	}
	return nil
}

func (m AuthMaterial) Manifest() AuthFingerprintManifest {
	return AuthFingerprintManifest{
		AuthSourceClass:                                m.SourceClass,
		AuthIdentityFingerprintSchemaVersion:           m.IdentityFingerprintSchemaVersion,
		AuthIdentityFingerprint:                        m.IdentityFingerprint,
		AuthIdentityFingerprintStatus:                  m.IdentityFingerprintStatus,
		AuthCredentialRevisionFingerprintSchemaVersion: m.CredentialRevisionFingerprintSchemaVersion,
		AuthCredentialRevisionFingerprint:              m.CredentialRevisionFingerprint,
		AuthCredentialRevisionFingerprintStatus:        m.CredentialRevisionFingerprintStatus,
	}
}

func CompareAuthManifests(historical, current AuthFingerprintManifest) AuthManifestComparison {
	comparison := AuthManifestComparison{
		SourceClass:        compareText(historical.AuthSourceClass, current.AuthSourceClass),
		Identity:           compareFingerprint(historical.AuthIdentityFingerprintSchemaVersion, historical.AuthIdentityFingerprint, historical.AuthIdentityFingerprintStatus, current.AuthIdentityFingerprintSchemaVersion, current.AuthIdentityFingerprint, current.AuthIdentityFingerprintStatus, AuthIdentityFingerprintSchemaVersion),
		CredentialRevision: compareFingerprint(historical.AuthCredentialRevisionFingerprintSchemaVersion, historical.AuthCredentialRevisionFingerprint, historical.AuthCredentialRevisionFingerprintStatus, current.AuthCredentialRevisionFingerprintSchemaVersion, current.AuthCredentialRevisionFingerprint, current.AuthCredentialRevisionFingerprintStatus, AuthCredentialRevisionFingerprintSchemaVersion),
	}
	comparison.IncomparableFingerprintTypes = comparison.Identity == AuthComparisonIncomparable || comparison.CredentialRevision == AuthComparisonIncomparable
	if comparison.Identity == AuthComparisonDifferent {
		comparison.ReasonCodes = append(comparison.ReasonCodes, AuthIdentityChangedReasonCode)
	}
	if comparison.CredentialRevision == AuthComparisonDifferent {
		comparison.ReasonCodes = append(comparison.ReasonCodes, AuthCredentialRevisionChangedReasonCode)
	}
	if comparison.Identity == AuthComparisonNotReconstructable {
		comparison.ReasonCodes = append(comparison.ReasonCodes, AuthIdentityNotReconstructableReasonCode)
	}
	if comparison.IncomparableFingerprintTypes {
		comparison.ReasonCodes = append(comparison.ReasonCodes, AuthFingerprintTypesIncomparableReasonCode)
	}
	if len(comparison.ReasonCodes) > 0 {
		comparison.ReasonCode = comparison.ReasonCodes[0]
	}
	comparison.StrictExperimentComparable = comparison.SourceClass == AuthComparisonSame && comparison.Identity == AuthComparisonSame && comparison.CredentialRevision == AuthComparisonSame
	return comparison
}

type QualificationFingerprintV2 struct {
	FingerprintSchemaVersion string `json:"fingerprint_schema_version"`
	CanonicalManifestDigest  string `json:"canonical_manifest_digest"`
}

func (f QualificationFingerprintV2) Validate() error {
	if f.FingerprintSchemaVersion != CanonicalManifestFingerprintSchemaVersionV2 || !isSHA256Hex(f.CanonicalManifestDigest) {
		return errors.New("invalid canonical-manifest-v2 fingerprint")
	}
	return nil
}

type CanonicalManifestV2 struct {
	FingerprintSchemaVersion string                  `json:"fingerprint_schema_version"`
	Combination              ExecutionCombination    `json:"combination"`
	Auth                     AuthFingerprintManifest `json:"auth"`
}

func (m CanonicalManifestV2) Validate() error {
	if m.FingerprintSchemaVersion != CanonicalManifestFingerprintSchemaVersionV2 {
		return errors.New("unsupported canonical-manifest-v2 schema")
	}
	if err := m.Combination.Validate(); err != nil {
		return err
	}
	return m.Auth.Validate()
}

func (c ExecutionCombination) CanonicalManifestV2Digest(auth AuthFingerprintManifest) string {
	manifest := CanonicalManifestV2{FingerprintSchemaVersion: CanonicalManifestFingerprintSchemaVersionV2, Combination: c, Auth: auth}
	raw, _ := json.Marshal(manifest)
	h := sha256.Sum256(raw)
	return hex.EncodeToString(h[:])
}

func (c ExecutionCombination) CurrentFingerprintV2(auth AuthFingerprintManifest) QualificationFingerprintV2 {
	return QualificationFingerprintV2{FingerprintSchemaVersion: CanonicalManifestFingerprintSchemaVersionV2, CanonicalManifestDigest: c.CanonicalManifestV2Digest(auth)}
}

type QualificationRecordV2 struct {
	Layer                    QualificationLayer      `json:"layer"`
	Status                   QualificationStatus     `json:"status"`
	SchedulingDecision       string                  `json:"scheduling_decision"`
	EvidenceResult           EvidenceResult          `json:"evidence_result"`
	FingerprintSchemaVersion string                  `json:"fingerprint_schema_version"`
	CanonicalManifestDigest  string                  `json:"canonical_manifest_digest"`
	Combination              ExecutionCombination    `json:"combination"`
	Auth                     AuthFingerprintManifest `json:"auth"`
	Reason                   string                  `json:"reason"`
	CreatedAt                time.Time               `json:"created_at"`
	ExpiresAt                time.Time               `json:"expires_at,omitempty"`
}

func EvaluateBusinessGateV2(now time.Time, current ExecutionCombination, auth AuthFingerprintManifest, records []QualificationRecordV2) GateDecision {
	key := current.CanonicalManifestV2Digest(auth)
	decision := GateDecision{Layer: QualificationL1BaseTransport, Status: QualificationUnverified, EvidenceResult: EvidenceNotRun, ExecutionKey: key, Reason: "no qualification record"}
	if err := auth.Validate(); err != nil {
		decision.Reason = "invalid auth manifest: " + err.Error()
		return decision
	}
	for _, record := range records {
		if record.Layer != QualificationL1BaseTransport || !reflect.DeepEqual(record.Combination, current) {
			continue
		}
		comparison := CompareAuthManifests(record.Auth, auth)
		if comparison.Identity == AuthComparisonDifferent {
			decision.Status = QualificationStale
			decision.Reason = AuthIdentityChangedReasonCode
			return decision
		}
		if comparison.CredentialRevision == AuthComparisonDifferent {
			decision.Status = QualificationStale
			decision.Reason = AuthCredentialRevisionChangedReasonCode
			return decision
		}
		if comparison.Identity != AuthComparisonSame {
			decision.Status = QualificationStale
			decision.Reason = AuthIdentityNotReconstructableReasonCode
			return decision
		}
		if record.FingerprintSchemaVersion != CanonicalManifestFingerprintSchemaVersionV2 || record.CanonicalManifestDigest != key {
			decision.Status = QualificationStale
			decision.Reason = "canonical_manifest_changed"
			return decision
		}
		if !record.ExpiresAt.IsZero() && !now.Before(record.ExpiresAt) {
			decision.Status = QualificationStale
			decision.Reason = "qualification_freshness_expired"
			return decision
		}
		decision.Status, decision.EvidenceResult, decision.Reason = record.Status, record.EvidenceResult, record.Reason
		if record.Status != QualificationQualified || record.EvidenceResult != EvidencePassed {
			return decision
		}
		decision.Allowed = true
		return decision
	}
	return decision
}

func compareText(old, current string) AuthComparisonStatus {
	if old == "" || current == "" {
		return AuthComparisonNotRecorded
	}
	if old == current {
		return AuthComparisonSame
	}
	return AuthComparisonDifferent
}

func compareFingerprint(oldSchema, oldValue, oldStatus, currentSchema, currentValue, currentStatus, expectedSchema string) AuthComparisonStatus {
	if oldSchema == "" || oldValue == "" || oldStatus == "" || currentSchema == "" || currentValue == "" || currentStatus == "" {
		return AuthComparisonNotRecorded
	}
	if oldSchema != expectedSchema || currentSchema != expectedSchema {
		return AuthComparisonIncomparable
	}
	if oldStatus == "unavailable" || oldValue == "unavailable" || currentStatus == "unavailable" || currentValue == "unavailable" {
		return AuthComparisonNotReconstructable
	}
	if oldStatus != "available" || currentStatus != "available" {
		return AuthComparisonIncomparable
	}
	if oldValue == currentValue {
		return AuthComparisonSame
	}
	return AuthComparisonDifferent
}

func parseJWTClaims(token string) (map[string]json.RawMessage, bool) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, false
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		payload, err = base64.URLEncoding.DecodeString(parts[1])
	}
	if err != nil {
		return nil, false
	}
	var claims map[string]json.RawMessage
	if json.Unmarshal(payload, &claims) != nil {
		return nil, false
	}
	return claims, true
}

func jsonStringClaim(claims map[string]json.RawMessage, name string) (string, bool) {
	raw, ok := claims[name]
	if !ok {
		return "", false
	}
	var value string
	if json.Unmarshal(raw, &value) != nil {
		return "", false
	}
	return value, true
}

func isSHA256Hex(value string) bool {
	if len(value) != sha256.Size*2 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

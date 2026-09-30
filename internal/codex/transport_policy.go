// pattern: Functional Core
package codex

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"reflect"
	"time"
)

const (
	// CanonicalManifestFingerprintSchemaVersionV3 adds explicit provider
	// transport policy. v1 and v2 remain unchanged and retain their meanings.
	CanonicalManifestFingerprintSchemaVersionV3 = "canonical-manifest-v3"
	ProviderTransportPolicyExplicitlyDisabled   = "explicitly_disabled"
	ProviderTransportPolicyNativeDefault        = "native_default"
	ProviderTransportPolicyChangedReasonCode    = "provider_transport_policy_changed"
)

type TransportPolicyManifest struct {
	ProviderTransportPolicy string `json:"provider_transport_policy"`
	WebSocketPolicy         string `json:"websocket_policy"`
}

func (p TransportPolicyManifest) Validate() error {
	if !validTransportPolicy(p.ProviderTransportPolicy) || !validTransportPolicy(p.WebSocketPolicy) {
		return errors.New("invalid provider transport policy")
	}
	return nil
}

type CanonicalManifestV3 struct {
	FingerprintSchemaVersion string                  `json:"fingerprint_schema_version"`
	Combination              ExecutionCombination    `json:"combination"`
	Auth                     AuthFingerprintManifest `json:"auth"`
	Transport                TransportPolicyManifest `json:"transport"`
}

func (m CanonicalManifestV3) Validate() error {
	if m.FingerprintSchemaVersion != CanonicalManifestFingerprintSchemaVersionV3 {
		return errors.New("unsupported canonical-manifest-v3 schema")
	}
	if err := m.Combination.Validate(); err != nil {
		return err
	}
	if err := m.Auth.Validate(); err != nil {
		return err
	}
	return m.Transport.Validate()
}

type QualificationFingerprintV3 struct {
	FingerprintSchemaVersion string `json:"fingerprint_schema_version"`
	CanonicalManifestDigest  string `json:"canonical_manifest_digest"`
}

func (f QualificationFingerprintV3) Validate() error {
	if f.FingerprintSchemaVersion != CanonicalManifestFingerprintSchemaVersionV3 || !isSHA256Hex(f.CanonicalManifestDigest) {
		return errors.New("invalid canonical-manifest-v3 fingerprint")
	}
	return nil
}

func (c ExecutionCombination) CanonicalManifestV3Digest(auth AuthFingerprintManifest, transport TransportPolicyManifest) string {
	manifest := CanonicalManifestV3{FingerprintSchemaVersion: CanonicalManifestFingerprintSchemaVersionV3, Combination: c, Auth: auth, Transport: transport}
	raw, _ := json.Marshal(manifest)
	digest := sha256.Sum256(raw)
	return hex.EncodeToString(digest[:])
}

func (c ExecutionCombination) CurrentFingerprintV3(auth AuthFingerprintManifest, transport TransportPolicyManifest) QualificationFingerprintV3 {
	return QualificationFingerprintV3{FingerprintSchemaVersion: CanonicalManifestFingerprintSchemaVersionV3, CanonicalManifestDigest: c.CanonicalManifestV3Digest(auth, transport)}
}

type QualificationRecordV3 struct {
	Layer                    QualificationLayer      `json:"layer"`
	Status                   QualificationStatus     `json:"status"`
	SchedulingDecision       string                  `json:"scheduling_decision"`
	EvidenceResult           EvidenceResult          `json:"evidence_result"`
	FingerprintSchemaVersion string                  `json:"fingerprint_schema_version"`
	CanonicalManifestDigest  string                  `json:"canonical_manifest_digest"`
	Combination              ExecutionCombination    `json:"combination"`
	Auth                     AuthFingerprintManifest `json:"auth"`
	Transport                TransportPolicyManifest `json:"transport"`
	Reason                   string                  `json:"reason"`
	CreatedAt                time.Time               `json:"created_at"`
	ExpiresAt                time.Time               `json:"expires_at,omitempty"`
}

func EvaluateBusinessGateV3(now time.Time, current ExecutionCombination, auth AuthFingerprintManifest, transport TransportPolicyManifest, records []QualificationRecordV3) GateDecision {
	key := current.CanonicalManifestV3Digest(auth, transport)
	decision := GateDecision{Layer: QualificationL1BaseTransport, Status: QualificationUnverified, EvidenceResult: EvidenceNotRun, ExecutionKey: key, Reason: "no qualification record"}
	if err := auth.Validate(); err != nil {
		decision.Reason = "invalid auth manifest: " + err.Error()
		return decision
	}
	if err := transport.Validate(); err != nil {
		decision.Reason = "invalid transport manifest: " + err.Error()
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
		if record.Transport != transport {
			decision.Status = QualificationStale
			decision.Reason = ProviderTransportPolicyChangedReasonCode
			return decision
		}
		if record.FingerprintSchemaVersion != CanonicalManifestFingerprintSchemaVersionV3 || record.CanonicalManifestDigest != key {
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

func validTransportPolicy(value string) bool {
	return value == ProviderTransportPolicyExplicitlyDisabled || value == ProviderTransportPolicyNativeDefault
}

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
	// V4 adds the effective network namespace policy without changing V1/V2/V3
	// canonical meanings or recomputing their historical fingerprints.
	CanonicalManifestFingerprintSchemaVersionV4 = "canonical-manifest-v4"
	NetworkNamespacePolicyChangedReasonCode     = "network_namespace_policy_changed"
)

type NetworkNamespacePolicy string

const (
	NetworkNamespacePolicySharedHostNetwork        NetworkNamespacePolicy = "shared_host_network"
	NetworkNamespacePolicyIsolatedNetworkNamespace NetworkNamespacePolicy = "isolated_network_namespace"
)

func (p NetworkNamespacePolicy) Validate() error {
	if p != NetworkNamespacePolicySharedHostNetwork && p != NetworkNamespacePolicyIsolatedNetworkNamespace {
		return errors.New("invalid network namespace policy")
	}
	return nil
}

type CanonicalManifestV4 struct {
	FingerprintSchemaVersion string                  `json:"fingerprint_schema_version"`
	Combination              ExecutionCombination    `json:"combination"`
	Auth                     AuthFingerprintManifest `json:"auth"`
	Transport                TransportPolicyManifest `json:"transport"`
	NetworkNamespacePolicy   NetworkNamespacePolicy  `json:"network_namespace_policy"`
}

func (m CanonicalManifestV4) Validate() error {
	if m.FingerprintSchemaVersion != CanonicalManifestFingerprintSchemaVersionV4 {
		return errors.New("unsupported canonical-manifest-v4 schema")
	}
	if err := m.Combination.Validate(); err != nil {
		return err
	}
	if err := m.Auth.Validate(); err != nil {
		return err
	}
	if err := m.Transport.Validate(); err != nil {
		return err
	}
	return m.NetworkNamespacePolicy.Validate()
}

type QualificationFingerprintV4 struct {
	FingerprintSchemaVersion string `json:"fingerprint_schema_version"`
	CanonicalManifestDigest  string `json:"canonical_manifest_digest"`
}

func (f QualificationFingerprintV4) Validate() error {
	if f.FingerprintSchemaVersion != CanonicalManifestFingerprintSchemaVersionV4 || !isSHA256Hex(f.CanonicalManifestDigest) {
		return errors.New("invalid canonical-manifest-v4 fingerprint")
	}
	return nil
}

func (c ExecutionCombination) CanonicalManifestV4Digest(auth AuthFingerprintManifest, transport TransportPolicyManifest, network NetworkNamespacePolicy) string {
	manifest := CanonicalManifestV4{FingerprintSchemaVersion: CanonicalManifestFingerprintSchemaVersionV4, Combination: c, Auth: auth, Transport: transport, NetworkNamespacePolicy: network}
	raw, _ := json.Marshal(manifest)
	digest := sha256.Sum256(raw)
	return hex.EncodeToString(digest[:])
}

func (c ExecutionCombination) CurrentFingerprintV4(auth AuthFingerprintManifest, transport TransportPolicyManifest, network NetworkNamespacePolicy) QualificationFingerprintV4 {
	return QualificationFingerprintV4{FingerprintSchemaVersion: CanonicalManifestFingerprintSchemaVersionV4, CanonicalManifestDigest: c.CanonicalManifestV4Digest(auth, transport, network)}
}

type QualificationRecordV4 struct {
	Layer                    QualificationLayer      `json:"layer"`
	Status                   QualificationStatus     `json:"status"`
	SchedulingDecision       string                  `json:"scheduling_decision"`
	EvidenceResult           EvidenceResult          `json:"evidence_result"`
	FingerprintSchemaVersion string                  `json:"fingerprint_schema_version"`
	CanonicalManifestDigest  string                  `json:"canonical_manifest_digest"`
	Combination              ExecutionCombination    `json:"combination"`
	Auth                     AuthFingerprintManifest `json:"auth"`
	Transport                TransportPolicyManifest `json:"transport"`
	NetworkNamespacePolicy   NetworkNamespacePolicy  `json:"network_namespace_policy"`
	Reason                   string                  `json:"reason"`
	CreatedAt                time.Time               `json:"created_at"`
	ExpiresAt                time.Time               `json:"expires_at,omitempty"`
}

func EvaluateBusinessGateV4(now time.Time, current ExecutionCombination, auth AuthFingerprintManifest, transport TransportPolicyManifest, network NetworkNamespacePolicy, records []QualificationRecordV4) GateDecision {
	key := current.CanonicalManifestV4Digest(auth, transport, network)
	decision := GateDecision{Layer: QualificationL1BaseTransport, Status: QualificationUnverified, EvidenceResult: EvidenceNotRun, ExecutionKey: key, Reason: "no qualification record"}
	if err := current.Validate(); err != nil {
		decision.Reason = "invalid execution combination: " + err.Error()
		return decision
	}
	if err := auth.Validate(); err != nil {
		decision.Reason = "invalid auth manifest: " + err.Error()
		return decision
	}
	if err := transport.Validate(); err != nil {
		decision.Reason = "invalid transport manifest: " + err.Error()
		return decision
	}
	if err := network.Validate(); err != nil {
		decision.Reason = "invalid network namespace policy: " + err.Error()
		return decision
	}
	for _, record := range records {
		if record.Layer != QualificationL1BaseTransport || !reflect.DeepEqual(record.Combination, current) {
			continue
		}
		comparison := CompareAuthManifests(record.Auth, auth)
		if comparison.Identity == AuthComparisonDifferent {
			decision.Status, decision.Reason = QualificationStale, AuthIdentityChangedReasonCode
			return decision
		}
		if comparison.CredentialRevision == AuthComparisonDifferent {
			decision.Status, decision.Reason = QualificationStale, AuthCredentialRevisionChangedReasonCode
			return decision
		}
		if comparison.Identity != AuthComparisonSame {
			decision.Status, decision.Reason = QualificationStale, AuthIdentityNotReconstructableReasonCode
			return decision
		}
		if record.Transport != transport {
			decision.Status, decision.Reason = QualificationStale, ProviderTransportPolicyChangedReasonCode
			return decision
		}
		if record.NetworkNamespacePolicy != network {
			decision.Status, decision.Reason = QualificationStale, NetworkNamespacePolicyChangedReasonCode
			return decision
		}
		if record.FingerprintSchemaVersion != CanonicalManifestFingerprintSchemaVersionV4 || record.CanonicalManifestDigest != key {
			decision.Status, decision.Reason = QualificationStale, "canonical_manifest_changed"
			return decision
		}
		if !record.ExpiresAt.IsZero() && !now.Before(record.ExpiresAt) {
			decision.Status, decision.Reason = QualificationStale, "qualification_freshness_expired"
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

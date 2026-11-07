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
	// V5 adds config-home identity and normalized effective-config digests. V4
	// and earlier canonical meanings remain immutable.
	CanonicalManifestFingerprintSchemaVersionV5 = "canonical-manifest-v5"
	CodexHomeProfileChangedReasonCode           = "codex_home_profile_changed"
	EffectiveConfigChangedReasonCode            = "effective_config_changed"
	EffectiveTransportConfigChangedReasonCode   = "effective_transport_config_changed"
)

type CodexHomeProfile string

const (
	CodexHomeProfileWindowsWorking    CodexHomeProfile = "windows_working_user"
	CodexHomeProfileWindowsDiagnostic CodexHomeProfile = "windows_native_diagnostic"
	CodexHomeProfilePolisIsolated     CodexHomeProfile = "polis_wsl_bwrap_isolated"
)

func (p CodexHomeProfile) Validate() error {
	if p != CodexHomeProfileWindowsWorking && p != CodexHomeProfileWindowsDiagnostic && p != CodexHomeProfilePolisIsolated {
		return errors.New("invalid Codex HOME profile")
	}
	return nil
}

type CanonicalManifestV5 struct {
	FingerprintSchemaVersion       string                  `json:"fingerprint_schema_version"`
	Combination                    ExecutionCombination    `json:"combination"`
	Auth                           AuthFingerprintManifest `json:"auth"`
	Transport                      TransportPolicyManifest `json:"transport"`
	NetworkNamespacePolicy         NetworkNamespacePolicy  `json:"network_namespace_policy"`
	CodexHomeProfile               CodexHomeProfile        `json:"codex_home_profile"`
	EffectiveConfigDigest          string                  `json:"effective_config_digest"`
	EffectiveTransportConfigDigest string                  `json:"effective_transport_config_digest"`
}

func (m CanonicalManifestV5) Validate() error {
	if m.FingerprintSchemaVersion != CanonicalManifestFingerprintSchemaVersionV5 {
		return errors.New("unsupported canonical-manifest-v5 schema")
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
	if err := m.NetworkNamespacePolicy.Validate(); err != nil {
		return err
	}
	if err := m.CodexHomeProfile.Validate(); err != nil {
		return err
	}
	if !isSHA256Hex(m.EffectiveConfigDigest) || !isSHA256Hex(m.EffectiveTransportConfigDigest) {
		return errors.New("invalid effective config digest")
	}
	return nil
}

type QualificationFingerprintV5 struct {
	FingerprintSchemaVersion string `json:"fingerprint_schema_version"`
	CanonicalManifestDigest  string `json:"canonical_manifest_digest"`
}

func (f QualificationFingerprintV5) Validate() error {
	if f.FingerprintSchemaVersion != CanonicalManifestFingerprintSchemaVersionV5 || !isSHA256Hex(f.CanonicalManifestDigest) {
		return errors.New("invalid canonical-manifest-v5 fingerprint")
	}
	return nil
}

func (c ExecutionCombination) CanonicalManifestV5Digest(auth AuthFingerprintManifest, transport TransportPolicyManifest, network NetworkNamespacePolicy, homeProfile CodexHomeProfile, effectiveConfigDigest, effectiveTransportConfigDigest string) string {
	manifest := CanonicalManifestV5{FingerprintSchemaVersion: CanonicalManifestFingerprintSchemaVersionV5, Combination: c, Auth: auth, Transport: transport, NetworkNamespacePolicy: network, CodexHomeProfile: homeProfile, EffectiveConfigDigest: effectiveConfigDigest, EffectiveTransportConfigDigest: effectiveTransportConfigDigest}
	raw, _ := json.Marshal(manifest)
	digest := sha256.Sum256(raw)
	return hex.EncodeToString(digest[:])
}

func (c ExecutionCombination) CurrentFingerprintV5(auth AuthFingerprintManifest, transport TransportPolicyManifest, network NetworkNamespacePolicy, homeProfile CodexHomeProfile, effectiveConfigDigest, effectiveTransportConfigDigest string) QualificationFingerprintV5 {
	return QualificationFingerprintV5{FingerprintSchemaVersion: CanonicalManifestFingerprintSchemaVersionV5, CanonicalManifestDigest: c.CanonicalManifestV5Digest(auth, transport, network, homeProfile, effectiveConfigDigest, effectiveTransportConfigDigest)}
}

type QualificationRecordV5 struct {
	Layer                          QualificationLayer      `json:"layer"`
	Status                         QualificationStatus     `json:"status"`
	SchedulingDecision             string                  `json:"scheduling_decision"`
	EvidenceResult                 EvidenceResult          `json:"evidence_result"`
	FingerprintSchemaVersion       string                  `json:"fingerprint_schema_version"`
	CanonicalManifestDigest        string                  `json:"canonical_manifest_digest"`
	Combination                    ExecutionCombination    `json:"combination"`
	Auth                           AuthFingerprintManifest `json:"auth"`
	Transport                      TransportPolicyManifest `json:"transport"`
	NetworkNamespacePolicy         NetworkNamespacePolicy  `json:"network_namespace_policy"`
	CodexHomeProfile               CodexHomeProfile        `json:"codex_home_profile"`
	EffectiveConfigDigest          string                  `json:"effective_config_digest"`
	EffectiveTransportConfigDigest string                  `json:"effective_transport_config_digest"`
	Reason                         string                  `json:"reason"`
	CreatedAt                      time.Time               `json:"created_at"`
	ExpiresAt                      time.Time               `json:"expires_at,omitempty"`
}

func EvaluateBusinessGateV5(now time.Time, current ExecutionCombination, auth AuthFingerprintManifest, transport TransportPolicyManifest, network NetworkNamespacePolicy, homeProfile CodexHomeProfile, effectiveConfigDigest, effectiveTransportConfigDigest string, records []QualificationRecordV5) GateDecision {
	key := current.CanonicalManifestV5Digest(auth, transport, network, homeProfile, effectiveConfigDigest, effectiveTransportConfigDigest)
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
	if err := homeProfile.Validate(); err != nil || !isSHA256Hex(effectiveConfigDigest) || !isSHA256Hex(effectiveTransportConfigDigest) {
		decision.Reason = "invalid effective config identity"
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
		if record.CodexHomeProfile != homeProfile {
			decision.Status, decision.Reason = QualificationStale, CodexHomeProfileChangedReasonCode
			return decision
		}
		if record.EffectiveConfigDigest != effectiveConfigDigest {
			decision.Status, decision.Reason = QualificationStale, EffectiveConfigChangedReasonCode
			return decision
		}
		if record.EffectiveTransportConfigDigest != effectiveTransportConfigDigest {
			decision.Status, decision.Reason = QualificationStale, EffectiveTransportConfigChangedReasonCode
			return decision
		}
		if record.FingerprintSchemaVersion != CanonicalManifestFingerprintSchemaVersionV5 || record.CanonicalManifestDigest != key {
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

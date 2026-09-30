// pattern: Functional Core
package codex

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
)

const CanonicalManifestFingerprintSchemaVersionV6 = "canonical-manifest-v6"

type RuntimeNetworkPolicy string

const (
	RuntimeNetworkPolicySharedHostNetwork RuntimeNetworkPolicy = "shared_host_network"
	RuntimeNetworkPolicyIsolatedNamespace RuntimeNetworkPolicy = "isolated_network_namespace"
	RuntimeNetworkPolicyNativeOSNetwork   RuntimeNetworkPolicy = "native_os_network"
)

func (p RuntimeNetworkPolicy) Validate() error {
	if p != RuntimeNetworkPolicySharedHostNetwork && p != RuntimeNetworkPolicyIsolatedNamespace && p != RuntimeNetworkPolicyNativeOSNetwork {
		return errors.New("invalid runtime network policy")
	}
	return nil
}

type RuntimeExecutionEnvelope struct {
	Platform            string               `json:"platform"`
	Architecture        string               `json:"architecture"`
	RuntimeProfile      string               `json:"runtime_profile"`
	FilesystemIsolation string               `json:"filesystem_isolation"`
	ProcessIsolation    string               `json:"process_isolation"`
	NetworkPolicy       RuntimeNetworkPolicy `json:"network_policy"`
	CWDRole             string               `json:"cwd_role"`
	LaunchMechanism     string               `json:"launch_mechanism"`
	StdioMode           string               `json:"stdio_mode"`
}

func (e RuntimeExecutionEnvelope) Validate() error {
	if e.Platform == "" || e.Architecture == "" || e.RuntimeProfile == "" || e.FilesystemIsolation == "" || e.ProcessIsolation == "" || e.CWDRole == "" || e.LaunchMechanism == "" || e.StdioMode == "" {
		return errors.New("runtime execution envelope is incomplete")
	}
	return e.NetworkPolicy.Validate()
}

type CanonicalManifestV6 struct {
	FingerprintSchemaVersion       string                   `json:"fingerprint_schema_version"`
	Combination                    ExecutionCombination     `json:"combination"`
	Auth                           AuthFingerprintManifest  `json:"auth"`
	Transport                      TransportPolicyManifest  `json:"transport"`
	Runtime                        RuntimeExecutionEnvelope `json:"runtime"`
	CodexHomeProfile               CodexHomeProfile         `json:"codex_home_profile"`
	EffectiveConfigDigest          string                   `json:"effective_config_digest"`
	EffectiveTransportConfigDigest string                   `json:"effective_transport_config_digest"`
}

func (m CanonicalManifestV6) Validate() error {
	if m.FingerprintSchemaVersion != CanonicalManifestFingerprintSchemaVersionV6 {
		return errors.New("unsupported canonical-manifest-v6 schema")
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
	if err := m.Runtime.Validate(); err != nil {
		return err
	}
	if err := m.CodexHomeProfile.Validate(); err != nil {
		return err
	}
	if !isSHA256Hex(m.EffectiveConfigDigest) || !isSHA256Hex(m.EffectiveTransportConfigDigest) {
		return errors.New("invalid V6 effective config digest")
	}
	return nil
}

type QualificationFingerprintV6 struct {
	FingerprintSchemaVersion string `json:"fingerprint_schema_version"`
	CanonicalManifestDigest  string `json:"canonical_manifest_digest"`
}

func (f QualificationFingerprintV6) Validate() error {
	if f.FingerprintSchemaVersion != CanonicalManifestFingerprintSchemaVersionV6 || !isSHA256Hex(f.CanonicalManifestDigest) {
		return errors.New("invalid canonical-manifest-v6 fingerprint")
	}
	return nil
}

func (c ExecutionCombination) CanonicalManifestV6Digest(auth AuthFingerprintManifest, transport TransportPolicyManifest, runtime RuntimeExecutionEnvelope, homeProfile CodexHomeProfile, effectiveConfigDigest, effectiveTransportConfigDigest string) string {
	manifest := CanonicalManifestV6{FingerprintSchemaVersion: CanonicalManifestFingerprintSchemaVersionV6, Combination: c, Auth: auth, Transport: transport, Runtime: runtime, CodexHomeProfile: homeProfile, EffectiveConfigDigest: effectiveConfigDigest, EffectiveTransportConfigDigest: effectiveTransportConfigDigest}
	raw, _ := json.Marshal(manifest)
	digest := sha256.Sum256(raw)
	return hex.EncodeToString(digest[:])
}

func (c ExecutionCombination) CurrentFingerprintV6(auth AuthFingerprintManifest, transport TransportPolicyManifest, runtime RuntimeExecutionEnvelope, homeProfile CodexHomeProfile, effectiveConfigDigest, effectiveTransportConfigDigest string) QualificationFingerprintV6 {
	return QualificationFingerprintV6{FingerprintSchemaVersion: CanonicalManifestFingerprintSchemaVersionV6, CanonicalManifestDigest: c.CanonicalManifestV6Digest(auth, transport, runtime, homeProfile, effectiveConfigDigest, effectiveTransportConfigDigest)}
}

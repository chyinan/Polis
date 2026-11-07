// pattern: Functional Core
package codex

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
)

const CanonicalManifestFingerprintSchemaVersionV7 = "canonical-manifest-v7"

type ToolSurfaceEntry struct {
	Name                string `json:"name"`
	SchemaDigest        string `json:"schema_digest"`
	SchemaBytes         int    `json:"schema_bytes"`
	DescriptionDigest   string `json:"description_digest"`
	BindingIdentity     string `json:"binding_identity"`
	AuthorizationClass  string `json:"authorization_class"`
	RegistrationOrdinal int    `json:"registration_ordinal"`
}

type ToolSurfaceManifest struct {
	CheckpointPolicyRevision           string             `json:"checkpoint_policy_revision,omitempty"`
	ArtifactEligibilityPolicyRevision  string             `json:"artifact_eligibility_policy_revision,omitempty"`
	ContractSupersessionPolicyRevision string             `json:"contract_supersession_policy_revision,omitempty"`
	AcceptanceCheckerRevision          string             `json:"acceptance_checker_revision,omitempty"`
	SurfaceID                          string             `json:"surface_id"`
	ToolCount                          int                `json:"tool_count"`
	AggregateManifestDigest            string             `json:"aggregate_manifest_digest"`
	AggregateSchemaDigest              string             `json:"aggregate_schema_digest"`
	AggregateSchemaBytes               int                `json:"aggregate_schema_bytes"`
	ThreadStartPayloadDigest           string             `json:"thread_start_payload_digest"`
	ThreadStartPayloadBytes            int                `json:"thread_start_payload_bytes"`
	Tools                              []ToolSurfaceEntry `json:"tools"`
	BusinessWritePolicy                string             `json:"business_write_policy"`
}

func (m ToolSurfaceManifest) Validate() error {
	if m.SurfaceID == "" || m.ToolCount <= 0 || m.ToolCount != len(m.Tools) {
		return errors.New("invalid tool surface cardinality")
	}
	if !isSHA256Hex(m.AggregateManifestDigest) || !isSHA256Hex(m.AggregateSchemaDigest) || m.AggregateSchemaBytes <= 0 || !isSHA256Hex(m.ThreadStartPayloadDigest) || m.ThreadStartPayloadBytes <= 0 || m.BusinessWritePolicy == "" {
		return errors.New("invalid tool surface aggregate")
	}
	seenNames := map[string]bool{}
	seenOrdinals := map[int]bool{}
	for _, tool := range m.Tools {
		if tool.Name == "" || seenNames[tool.Name] || tool.SchemaBytes <= 0 || !isSHA256Hex(tool.SchemaDigest) || tool.DescriptionDigest != "" && !isSHA256Hex(tool.DescriptionDigest) || tool.BindingIdentity == "" || tool.AuthorizationClass == "" || tool.RegistrationOrdinal < 1 || tool.RegistrationOrdinal > m.ToolCount || seenOrdinals[tool.RegistrationOrdinal] {
			return errors.New("invalid tool surface entry")
		}
		seenNames[tool.Name] = true
		seenOrdinals[tool.RegistrationOrdinal] = true
	}
	for ordinal := 1; ordinal <= m.ToolCount; ordinal++ {
		if !seenOrdinals[ordinal] {
			return errors.New("tool surface registration ordinals are not contiguous")
		}
	}
	return nil
}

type CanonicalManifestV7 struct {
	FingerprintSchemaVersion string              `json:"fingerprint_schema_version"`
	Base                     CanonicalManifestV6 `json:"base_manifest"`
	ToolSurface              ToolSurfaceManifest `json:"tool_surface"`
}

func (m CanonicalManifestV7) Validate() error {
	if m.FingerprintSchemaVersion != CanonicalManifestFingerprintSchemaVersionV7 {
		return errors.New("unsupported canonical-manifest-v7 schema")
	}
	if err := m.Base.Validate(); err != nil {
		return err
	}
	return m.ToolSurface.Validate()
}

type QualificationFingerprintV7 struct {
	FingerprintSchemaVersion string `json:"fingerprint_schema_version"`
	CanonicalManifestDigest  string `json:"canonical_manifest_digest"`
}

func (f QualificationFingerprintV7) Validate() error {
	if f.FingerprintSchemaVersion != CanonicalManifestFingerprintSchemaVersionV7 || !isSHA256Hex(f.CanonicalManifestDigest) {
		return errors.New("invalid canonical-manifest-v7 fingerprint")
	}
	return nil
}

func (c ExecutionCombination) CanonicalManifestV7Digest(manifest CanonicalManifestV7) string {
	raw, _ := json.Marshal(manifest)
	digest := sha256.Sum256(raw)
	return hex.EncodeToString(digest[:])
}

func (c ExecutionCombination) CurrentFingerprintV7(manifest CanonicalManifestV7) QualificationFingerprintV7 {
	return QualificationFingerprintV7{FingerprintSchemaVersion: CanonicalManifestFingerprintSchemaVersionV7, CanonicalManifestDigest: c.CanonicalManifestV7Digest(manifest)}
}

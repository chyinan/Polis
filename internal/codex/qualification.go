// pattern: Functional Core
package codex

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sort"
	"time"
)

type QualificationLayer string

const (
	QualificationL0NativeProcess QualificationLayer = "L0_native_process"
	QualificationL1BaseTransport QualificationLayer = "L1_base_provider_transport"
	QualificationL2ToolSurface   QualificationLayer = "L2_dynamic_tool_surface"
	QualificationL3Business      QualificationLayer = "L3_employee_business"
)

type QualificationStatus string

const (
	QualificationUnverified   QualificationStatus = "unverified"
	QualificationQualified    QualificationStatus = "qualified"
	QualificationUnqualified  QualificationStatus = "unqualified"
	QualificationStale        QualificationStatus = "stale"
	QualificationInconclusive QualificationStatus = "inconclusive"
)

type EvidenceResult string

const (
	EvidencePassed       EvidenceResult = "passed"
	EvidenceFailed       EvidenceResult = "failed"
	EvidenceInconclusive EvidenceResult = "inconclusive"
	EvidenceNotRun       EvidenceResult = "not_run"
)

const (
	// LegacyFingerprintSchemaVersion identifies the pre-T10 JSON-serialization
	// hash used by historical qualification evidence, including T7.
	LegacyFingerprintSchemaVersion = "legacy-json-serialization-v0"
	// CanonicalManifestFingerprintSchemaVersion is explicit so a legacy hash
	// cannot be silently treated as a digest from the current schema.
	CanonicalManifestFingerprintSchemaVersion = "canonical-manifest-v1"
)

type QualificationFingerprint struct {
	FingerprintSchemaVersion string `json:"fingerprint_schema_version"`
	CanonicalManifestDigest  string `json:"canonical_manifest_digest"`
}

func (f QualificationFingerprint) Validate() error {
	if f.FingerprintSchemaVersion != CanonicalManifestFingerprintSchemaVersion {
		return errors.New("unsupported qualification fingerprint schema")
	}
	if len(f.CanonicalManifestDigest) != sha256.Size*2 {
		return errors.New("invalid canonical manifest digest")
	}
	if _, err := hex.DecodeString(f.CanonicalManifestDigest); err != nil {
		return errors.New("invalid canonical manifest digest")
	}
	return nil
}

func (f QualificationFingerprint) Matches(c ExecutionCombination) bool {
	return f.Validate() == nil && f.CanonicalManifestDigest == c.CanonicalManifestDigest()
}

// ExecutionCombination excludes business tool-surface identity deliberately.
// Tool qualification is a separate L2 record, so a T5 change cannot silently
// turn an L1 transport result into an L3 business permission.
type ExecutionCombination struct {
	CodexVersion         string `json:"codex_version"`
	BinarySHA256         string `json:"binary_sha256"`
	Model                string `json:"model"`
	Effort               string `json:"effort"`
	RuntimeProfile       string `json:"runtime_profile"`
	SandboxClass         string `json:"sandbox_class"`
	ProxyConfigDigest    string `json:"proxy_config_digest"`
	AuthSourceClass      string `json:"auth_source_class"`
	CodeModeHostSHA256   string `json:"code_mode_host_sha256"`
	CapabilityDigest     string `json:"capability_digest"`
	NativeProtocolDigest string `json:"native_protocol_digest"`
}

func (c ExecutionCombination) Validate() error {
	values := []string{c.CodexVersion, c.BinarySHA256, c.Model, c.Effort, c.RuntimeProfile, c.SandboxClass, c.ProxyConfigDigest, c.AuthSourceClass, c.CodeModeHostSHA256, c.CapabilityDigest, c.NativeProtocolDigest}
	for _, value := range values {
		if value == "" {
			return errors.New("execution combination has an empty qualification field")
		}
	}
	return nil
}

func (c ExecutionCombination) Fingerprint() string {
	return c.LegacyFingerprint()
}

// LegacyFingerprint preserves the historical T7/T9 serialization exactly.
// New qualification manifests must use CurrentFingerprint instead.
func (c ExecutionCombination) LegacyFingerprint() string {
	raw, _ := json.Marshal(c)
	h := sha256.Sum256(raw)
	return hex.EncodeToString(h[:])
}

func (c ExecutionCombination) CanonicalManifestDigest() string {
	manifest := struct {
		FingerprintSchemaVersion string               `json:"fingerprint_schema_version"`
		Combination              ExecutionCombination `json:"combination"`
	}{
		FingerprintSchemaVersion: CanonicalManifestFingerprintSchemaVersion,
		Combination:              c,
	}
	raw, _ := json.Marshal(manifest)
	h := sha256.Sum256(raw)
	return hex.EncodeToString(h[:])
}

func (c ExecutionCombination) CurrentFingerprint() QualificationFingerprint {
	return QualificationFingerprint{
		FingerprintSchemaVersion: CanonicalManifestFingerprintSchemaVersion,
		CanonicalManifestDigest:  c.CanonicalManifestDigest(),
	}
}

type QualificationRecord struct {
	Layer                    QualificationLayer   `json:"layer"`
	Status                   QualificationStatus  `json:"status"`
	EvidenceResult           EvidenceResult       `json:"evidence_result"`
	ExecutionKey             string               `json:"execution_key"`
	FingerprintSchemaVersion string               `json:"fingerprint_schema_version,omitempty"`
	CanonicalManifestDigest  string               `json:"canonical_manifest_digest,omitempty"`
	Combination              ExecutionCombination `json:"combination"`
	ToolSurfaceDigest        string               `json:"tool_surface_digest,omitempty"`
	EvidenceRef              string               `json:"evidence_ref"`
	Reason                   string               `json:"reason"`
	CreatedAt                time.Time            `json:"created_at"`
	ExpiresAt                time.Time            `json:"expires_at,omitempty"`
}

type GateDecision struct {
	Allowed        bool                `json:"allowed"`
	Layer          QualificationLayer  `json:"layer"`
	Status         QualificationStatus `json:"status"`
	EvidenceResult EvidenceResult      `json:"evidence_result"`
	ExecutionKey   string              `json:"execution_key"`
	Reason         string              `json:"reason"`
}

func EvaluateBusinessGate(now time.Time, current ExecutionCombination, records []QualificationRecord) GateDecision {
	return evaluateQualificationGate(now, QualificationL1BaseTransport, current, "", records)
}

func EvaluateToolSurfaceGate(now time.Time, current ExecutionCombination, toolSurfaceDigest string, records []QualificationRecord) GateDecision {
	return evaluateQualificationGate(now, QualificationL2ToolSurface, current, toolSurfaceDigest, records)
}

func evaluateQualificationGate(now time.Time, layer QualificationLayer, current ExecutionCombination, toolSurfaceDigest string, records []QualificationRecord) GateDecision {
	key := current.Fingerprint()
	decision := GateDecision{Layer: layer, Status: QualificationUnverified, EvidenceResult: EvidenceNotRun, ExecutionKey: key, Reason: "no qualification record"}
	if err := current.Validate(); err != nil {
		decision.Reason = "invalid execution combination: " + err.Error()
		return decision
	}
	matching := make([]QualificationRecord, 0)
	layerRecords := make([]QualificationRecord, 0)
	currentFingerprint := current.CurrentFingerprint()
	for _, record := range records {
		if record.Layer != layer {
			continue
		}
		layerRecords = append(layerRecords, record)
		legacyMatch := record.FingerprintSchemaVersion == "" && record.CanonicalManifestDigest == "" && record.ExecutionKey == key
		currentMatch := record.FingerprintSchemaVersion == currentFingerprint.FingerprintSchemaVersion && record.CanonicalManifestDigest == currentFingerprint.CanonicalManifestDigest
		if (legacyMatch || currentMatch) && (layer != QualificationL2ToolSurface || record.ToolSurfaceDigest == toolSurfaceDigest) {
			matching = append(matching, record)
		}
	}
	if len(matching) == 0 {
		if len(layerRecords) > 0 {
			decision.Status = QualificationStale
			decision.Reason = "qualification record belongs to a different execution combination or tool surface"
		}
		return decision
	}
	sort.SliceStable(matching, func(i, j int) bool { return matching[i].CreatedAt.After(matching[j].CreatedAt) })
	record := matching[0]
	decision.Status, decision.EvidenceResult, decision.Reason = record.Status, record.EvidenceResult, record.Reason
	if !record.ExpiresAt.IsZero() && !now.Before(record.ExpiresAt) {
		decision.Allowed = false
		decision.Status = QualificationStale
		decision.Reason = "qualification freshness deadline expired"
		return decision
	}
	if record.Status != QualificationQualified {
		return decision
	}
	if record.EvidenceResult != EvidencePassed {
		decision.Allowed = false
		decision.Status = QualificationInconclusive
		decision.Reason = "qualified scheduling state lacks passed evidence"
		return decision
	}
	decision.Allowed = true
	decision.Status = QualificationQualified
	decision.Reason = "exact execution combination is qualified"
	return decision
}

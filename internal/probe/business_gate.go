// pattern: Imperative Shell
package probe

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"polis/internal/codex"
	"reflect"
	"time"
)

type qualificationEnvelope struct {
	Record      codex.QualificationRecord  `json:"record"`
	Combination codex.ExecutionCombination `json:"combination"`
}

// RequireBusinessQualification is the only shell entry used before a real
// business allowance. It reads immutable qualification evidence and delegates
// all status/key logic to the pure codex gate.
func RequireBusinessQualification(recordPath, manifestPath string) error {
	if recordPath == "" || manifestPath == "" {
		return errors.New("business execution gate blocked: qualification record and current execution manifest are required")
	}
	recordRaw, e := os.ReadFile(recordPath)
	if e != nil {
		return fmt.Errorf("business execution gate blocked: read qualification record: %w", e)
	}
	manifestRaw, e := os.ReadFile(manifestPath)
	if e != nil {
		return fmt.Errorf("business execution gate blocked: read execution manifest: %w", e)
	}
	var record codex.QualificationRecord
	if e = json.Unmarshal(recordRaw, &record); e != nil {
		return fmt.Errorf("business execution gate blocked: invalid qualification record: %w", e)
	}
	var manifest qualificationEnvelope
	if e = json.Unmarshal(manifestRaw, &manifest); e != nil {
		return fmt.Errorf("business execution gate blocked: invalid execution manifest: %w", e)
	}
	if manifest.Combination.Fingerprint() != record.ExecutionKey {
		return errors.New("business execution gate blocked: current execution combination does not match qualification key")
	}
	decision := codex.EvaluateBusinessGate(time.Now(), manifest.Combination, []codex.QualificationRecord{record})
	if !decision.Allowed {
		return fmt.Errorf("business execution gate blocked: status=%s evidence=%s reason=%s", decision.Status, decision.EvidenceResult, decision.Reason)
	}
	return nil
}

type R03AT2QualificationReport struct {
	Qualification                   string    `json:"qualification"`
	ExecutionFingerprint            string    `json:"execution_fingerprint"`
	CodexVersion                    string    `json:"codex_version"`
	Model                           string    `json:"model"`
	Effort                          string    `json:"effort"`
	CapabilityDigest                string    `json:"capability_digest"`
	ToolManifestDigest              string    `json:"tool_manifest_digest"`
	ToolCount                       int       `json:"tool_count"`
	AuthSourceClass                 string    `json:"auth_source_class"`
	AuthIdentityFingerprint         string    `json:"auth_identity_fingerprint"`
	AuthCredentialRevision          string    `json:"auth_credential_revision_fingerprint"`
	BehavioralContractQualification string    `json:"behavioral_contract_qualification"`
	RevalidatedAt                   time.Time `json:"revalidated_at"`
}

// RequireR03AT2Qualification revalidates the frozen Windows-native T21B
// combination immediately before business allowance/worker creation. The
// diagnostic manifest remains the authority for the qualified envelope; the
// business prompt is intentionally not substituted into that manifest.
func RequireR03AT2Qualification(cfg R03AT2Config) (R03AT2QualificationReport, error) {
	var report R03AT2QualificationReport
	if cfg.ExecutionManifestPath == "" || cfg.ExecutionConfigPath == "" || cfg.QualificationPath == "" || cfg.Binary == "" || cfg.CodeModeHost == "" || cfg.AuthFile == "" || cfg.SelectedConfigPath == "" || cfg.ExecutionFingerprint == "" {
		return report, errors.New("R0.3A real Backend qualification requires complete revalidation inputs")
	}
	var sealed struct {
		Qualification            string                           `json:"qualification"`
		FingerprintSchemaVersion string                           `json:"fingerprint_schema_version"`
		CanonicalManifest        codex.CanonicalManifestV7        `json:"canonical_manifest"`
		CanonicalManifestDigest  string                           `json:"canonical_manifest_digest"`
		QualificationFingerprint codex.QualificationFingerprintV7 `json:"qualification_fingerprint"`
	}
	if err := readT11JSON(cfg.ExecutionManifestPath, &sealed); err != nil {
		return report, fmt.Errorf("business execution gate blocked: read T21B execution manifest: %w", err)
	}
	var manifestHeader struct {
		SchemaVersion string `json:"schema_version"`
	}
	if raw, err := os.ReadFile(cfg.ExecutionManifestPath); err == nil && json.Unmarshal(raw, &manifestHeader) == nil && manifestHeader.SchemaVersion == r03aCurrentBinaryL2ManifestSchema {
		return requireCurrentBinaryRevisedL2Qualification(cfg)
	}
	var execution t21ExecutionConfig
	if err := readT11JSON(cfg.ExecutionConfigPath, &execution); err != nil {
		return report, fmt.Errorf("business execution gate blocked: read T21B execution config: %w", err)
	}
	var qualification struct {
		Status                  string `json:"status"`
		EligibleForRealBackend  bool   `json:"eligible_for_real_backend"`
		CanonicalManifestDigest string `json:"canonical_manifest_digest"`
	}
	if err := readT11JSON(cfg.QualificationPath, &qualification); err != nil {
		return report, fmt.Errorf("business execution gate blocked: read T21B qualification: %w", err)
	}
	if sealed.Qualification != "R0.3A-T21B" || sealed.FingerprintSchemaVersion != codex.CanonicalManifestFingerprintSchemaVersionV7 || qualification.Status != "PASSED" || !qualification.EligibleForRealBackend {
		return report, errors.New("business execution gate blocked: T21B qualification is not eligible")
	}
	if err := sealed.CanonicalManifest.Validate(); err != nil {
		return report, fmt.Errorf("business execution gate blocked: invalid T21B manifest: %w", err)
	}
	expectedFingerprint := sealed.CanonicalManifest.Base.Combination.CurrentFingerprintV7(sealed.CanonicalManifest)
	if sealed.CanonicalManifestDigest != expectedFingerprint.CanonicalManifestDigest || sealed.QualificationFingerprint != expectedFingerprint || qualification.CanonicalManifestDigest != expectedFingerprint.CanonicalManifestDigest || cfg.ExecutionFingerprint != expectedFingerprint.CanonicalManifestDigest {
		return report, errors.New("business execution gate blocked: current execution fingerprint is stale or mismatched")
	}
	if execution.QualificationFingerprint != expectedFingerprint || execution.ToolManifestDigest != sealed.CanonicalManifest.ToolSurface.AggregateManifestDigest || execution.ToolSurface.ToolCount != sealed.CanonicalManifest.ToolSurface.ToolCount {
		return report, errors.New("business execution gate blocked: T21B execution config is stale")
	}
	currentSurface, _, _, err := buildT21ToolSurface()
	if err != nil {
		return report, fmt.Errorf("business execution gate blocked: build formal peer tool surface: %w", err)
	}
	if !sameR03AT2ToolSurface(currentSurface, sealed.CanonicalManifest.ToolSurface) {
		return report, errors.New("business execution gate blocked: formal PeerBackendTools surface drifted")
	}
	binaryDigest, err := sha256File(cfg.Binary)
	if err != nil || binaryDigest != sealed.CanonicalManifest.Base.Combination.BinarySHA256 {
		return report, errors.New("business execution gate blocked: Windows Codex binary drifted")
	}
	helperDigest, err := sha256File(cfg.CodeModeHost)
	if err != nil || helperDigest != sealed.CanonicalManifest.Base.Combination.CodeModeHostSHA256 {
		return report, errors.New("business execution gate blocked: Windows code-mode host drifted")
	}
	selectedConfigDigest, err := sha256File(cfg.SelectedConfigPath)
	if err != nil || selectedConfigDigest != execution.SelectedConfigRawSHA256 {
		return report, errors.New("business execution gate blocked: selected config drifted")
	}
	authRaw, err := os.ReadFile(cfg.AuthFile)
	if err != nil {
		return report, fmt.Errorf("business execution gate blocked: read auth: %w", err)
	}
	authMaterial, err := codex.ParseAuthMaterial(authRaw, sealed.CanonicalManifest.Base.Auth.AuthSourceClass)
	if err != nil || !reflect.DeepEqual(authMaterial.Manifest(), sealed.CanonicalManifest.Base.Auth) {
		return report, errors.New("business execution gate blocked: auth identity or credential revision drifted")
	}
	if cfg.ExpectedNativeVersion != "" && cfg.ExpectedNativeVersion != sealed.CanonicalManifest.Base.Combination.CodexVersion {
		return report, errors.New("business execution gate blocked: expected Codex version drifted")
	}
	report = R03AT2QualificationReport{Qualification: sealed.Qualification, ExecutionFingerprint: expectedFingerprint.CanonicalManifestDigest, CodexVersion: sealed.CanonicalManifest.Base.Combination.CodexVersion, Model: sealed.CanonicalManifest.Base.Combination.Model, Effort: sealed.CanonicalManifest.Base.Combination.Effort, CapabilityDigest: sealed.CanonicalManifest.Base.Combination.CapabilityDigest, ToolManifestDigest: currentSurface.AggregateManifestDigest, ToolCount: currentSurface.ToolCount, AuthSourceClass: authMaterial.SourceClass, AuthIdentityFingerprint: authMaterial.IdentityFingerprint, AuthCredentialRevision: authMaterial.CredentialRevisionFingerprint, RevalidatedAt: time.Now().UTC()}
	return report, nil
}

func sameR03AT2ToolSurface(current, expected codex.ToolSurfaceManifest) bool {
	if current.CheckpointPolicyRevision != expected.CheckpointPolicyRevision || current.ArtifactEligibilityPolicyRevision != expected.ArtifactEligibilityPolicyRevision || current.ContractSupersessionPolicyRevision != expected.ContractSupersessionPolicyRevision || current.AcceptanceCheckerRevision != expected.AcceptanceCheckerRevision {
		return false
	}
	if current.SurfaceID != expected.SurfaceID || current.ToolCount != expected.ToolCount || current.AggregateManifestDigest != expected.AggregateManifestDigest || current.AggregateSchemaDigest != expected.AggregateSchemaDigest || current.AggregateSchemaBytes != expected.AggregateSchemaBytes || current.BusinessWritePolicy != expected.BusinessWritePolicy || len(current.Tools) != len(expected.Tools) {
		return false
	}
	for i := range current.Tools {
		if current.Tools[i] != expected.Tools[i] {
			return false
		}
	}
	return true
}

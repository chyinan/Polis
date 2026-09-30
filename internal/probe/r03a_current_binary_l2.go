// pattern: Imperative Shell
package probe

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	"polis/internal/codex"
)

const (
	r03aCurrentBinaryL2Qualification  = "R0.3A-CURRENT-BINARY-REVISED-11-TOOL-L2"
	r03aCurrentBinaryL2BindingSchema  = "r03a-current-binary-revised-11-tool-l2-binding-v1"
	r03aCurrentBinaryL2ManifestSchema = "r03a-current-binary-revised-11-tool-l2-manifest-v1"
)

type currentBinaryL2Binding struct {
	SchemaVersion                      string `json:"schema_version"`
	L1Fingerprint                      string `json:"l1_fingerprint"`
	ControlledRuntimeManifestDigest    string `json:"controlled_runtime_manifest_digest"`
	CodexVersion                       string `json:"codex_version"`
	CodexBinarySHA256                  string `json:"codex_binary_sha256"`
	CodeModeHostSHA256                 string `json:"code_mode_host_sha256"`
	ToolManifestDigest                 string `json:"tool_manifest_digest"`
	AggregateSchemaDigest              string `json:"aggregate_schema_digest"`
	AggregateSchemaBytes               int    `json:"aggregate_schema_bytes"`
	ToolCount                          int    `json:"tool_count"`
	CheckpointPolicyRevision           string `json:"checkpoint_policy_revision"`
	ArtifactEligibilityPolicyRevision  string `json:"artifact_eligibility_policy_revision"`
	ContractSupersessionPolicyRevision string `json:"contract_supersession_policy_revision"`
	AcceptanceCheckerRevision          string `json:"acceptance_checker_revision"`
}

func (b currentBinaryL2Binding) Validate() error {
	if b.SchemaVersion != r03aCurrentBinaryL2BindingSchema || !validCurrentBinaryDigest(b.L1Fingerprint) || !validCurrentBinaryDigest(b.ControlledRuntimeManifestDigest) || b.CodexVersion == "" || !validCurrentBinaryDigest(b.CodexBinarySHA256) || !validCurrentBinaryDigest(b.CodeModeHostSHA256) || !validCurrentBinaryDigest(b.ToolManifestDigest) || !validCurrentBinaryDigest(b.AggregateSchemaDigest) || b.AggregateSchemaBytes <= 0 || b.ToolCount != 11 {
		return errors.New("invalid current-binary L2 binding")
	}
	if b.CheckpointPolicyRevision == "" || b.ArtifactEligibilityPolicyRevision == "" || b.ContractSupersessionPolicyRevision == "" || b.AcceptanceCheckerRevision == "" {
		return errors.New("current-binary L2 binding is missing a policy revision")
	}
	return nil
}

func (b currentBinaryL2Binding) Fingerprint() (string, error) {
	if err := b.Validate(); err != nil {
		return "", err
	}
	raw, err := json.Marshal(b)
	if err != nil {
		return "", err
	}
	return digest(raw), nil
}

type currentBinaryL1Evidence struct {
	Status                          string `json:"status"`
	CurrentBinaryWindowsL1          string `json:"current_binary_windows_l1"`
	ExecutionFingerprint            string `json:"execution_fingerprint"`
	ControlledRuntimeManifestDigest string `json:"controlled_runtime_manifest_digest"`
	CodexVersion                    string `json:"codex_version"`
	CodexBinarySHA256               string `json:"codex_binary_sha256"`
	CodeModeHostSHA256              string `json:"code_mode_host_sha256"`
	AuthIdentityFingerprint         string `json:"auth_identity_fingerprint"`
	AuthCredentialRevision          string `json:"auth_credential_revision_fingerprint"`
	AuthSourceClass                 string `json:"auth_source_class"`
	EffectiveConfigDigest           string `json:"effective_config_digest"`
	EffectiveTransportConfigDigest  string `json:"effective_transport_config_digest"`
	SelectedConfigRawSHA256         string `json:"selected_config_raw_sha256"`
	Model                           string `json:"model"`
	Effort                          string `json:"effort"`
	RuntimeProfile                  string `json:"runtime_profile"`
	ProviderTransportPolicy         string `json:"provider_transport_policy"`
	DynamicToolCount                int    `json:"dynamic_tool_count"`
	MediumStarted                   int    `json:"medium_started"`
	ProviderEgress                  int    `json:"provider_egress"`
	UnresolvedTransportState        bool   `json:"unresolved_transport_state"`
}

type currentBinaryL2ExecutionManifest struct {
	SchemaVersion                   string                    `json:"schema_version"`
	Qualification                   string                    `json:"qualification"`
	CanonicalManifest               codex.CanonicalManifestV7 `json:"canonical_manifest"`
	CanonicalManifestDigest         string                    `json:"canonical_manifest_digest"`
	ExecutionFingerprint            string                    `json:"execution_fingerprint"`
	L1Fingerprint                   string                    `json:"l1_fingerprint"`
	ControlledRuntimeManifestDigest string                    `json:"controlled_runtime_manifest_digest"`
	ToolSurface                     codex.ToolSurfaceManifest `json:"tool_surface"`
	ToolManifestDigest              string                    `json:"tool_manifest_digest"`
	PolicyRevisions                 map[string]string         `json:"policy_revisions"`
	Binding                         currentBinaryL2Binding    `json:"binding"`
}

type currentBinaryL2LiveQualification struct {
	Qualification                string `json:"qualification"`
	Status                       string `json:"status"`
	ExecutionFingerprint         string `json:"execution_fingerprint"`
	CurrentBinaryRevised11ToolL2 string `json:"current_binary_revised_11_tool_l2"`
	EligibleForRevisedBackendRun bool   `json:"eligible_for_revised_backend_run"`
}

func requireCurrentBinaryRevisedL2Qualification(cfg R03AT2Config) (R03AT2QualificationReport, error) {
	var report R03AT2QualificationReport
	var manifest currentBinaryL2ExecutionManifest
	if err := readT11JSON(cfg.ExecutionManifestPath, &manifest); err != nil {
		return report, fmt.Errorf("business execution gate blocked: read current-binary L2 manifest: %w", err)
	}
	if manifest.SchemaVersion != r03aCurrentBinaryL2ManifestSchema || manifest.Qualification != r03aCurrentBinaryL2Qualification || manifest.ExecutionFingerprint != cfg.ExecutionFingerprint || !validCurrentBinaryDigest(manifest.ExecutionFingerprint) || manifest.L1Fingerprint == "" || manifest.ControlledRuntimeManifestDigest == "" || manifest.ToolManifestDigest == "" {
		return report, errors.New("business execution gate blocked: current-binary L2 manifest is stale or mismatched")
	}
	if err := manifest.CanonicalManifest.Validate(); err != nil {
		return report, fmt.Errorf("business execution gate blocked: invalid current-binary L2 canonical manifest: %w", err)
	}
	expectedManifestDigest := manifest.CanonicalManifest.Base.Combination.CurrentFingerprintV7(manifest.CanonicalManifest).CanonicalManifestDigest
	if manifest.CanonicalManifestDigest != expectedManifestDigest {
		return report, errors.New("business execution gate blocked: current-binary L2 canonical manifest digest mismatch")
	}
	bindingFingerprint, err := manifest.Binding.Fingerprint()
	if err != nil || bindingFingerprint != manifest.ExecutionFingerprint {
		return report, errors.New("business execution gate blocked: current-binary L2 authorization binding mismatch")
	}

	var l1 currentBinaryL1Evidence
	if cfg.CurrentL1EvidencePath == "" || readT11JSON(filepath.Join(cfg.CurrentL1EvidencePath, "result.json"), &l1) != nil || l1.Status != "passed" || l1.CurrentBinaryWindowsL1 != "QUALIFIED" || l1.ExecutionFingerprint != manifest.L1Fingerprint || l1.ControlledRuntimeManifestDigest != manifest.ControlledRuntimeManifestDigest || l1.CodexBinarySHA256 != manifest.CanonicalManifest.Base.Combination.BinarySHA256 || l1.CodeModeHostSHA256 != manifest.CanonicalManifest.Base.Combination.CodeModeHostSHA256 || l1.Model != "gpt-5.6-luna" || l1.Effort != "medium" || l1.RuntimeProfile != "windows-native" || l1.ProviderTransportPolicy != "native_default" || l1.DynamicToolCount != 0 || l1.MediumStarted != 1 || l1.ProviderEgress != 1 || l1.UnresolvedTransportState {
		return report, errors.New("business execution gate blocked: current-binary Windows L1 evidence is stale or mismatched")
	}

	surface, tools, _, err := buildT21ToolSurface()
	if err != nil {
		return report, fmt.Errorf("business execution gate blocked: build formal peer tool surface: %w", err)
	}
	payloadRaw, err := json.Marshal(t21NormalizedThreadPayload(tools, r03aT21DeveloperInstruction))
	if err != nil {
		return report, fmt.Errorf("business execution gate blocked: serialize formal peer tool payload: %w", err)
	}
	surface.ThreadStartPayloadDigest = digest(payloadRaw)
	surface.ThreadStartPayloadBytes = len(payloadRaw)
	if !sameR03AT2ToolSurface(surface, manifest.ToolSurface) || manifest.ToolSurface.ThreadStartPayloadDigest != surface.ThreadStartPayloadDigest || manifest.ToolSurface.ThreadStartPayloadBytes != surface.ThreadStartPayloadBytes || manifest.ToolSurface.AggregateManifestDigest != manifest.ToolManifestDigest {
		return report, errors.New("business execution gate blocked: formal current PeerBackendTools surface drifted")
	}
	if !reflect.DeepEqual(manifest.CanonicalManifest.ToolSurface, manifest.ToolSurface) {
		return report, errors.New("business execution gate blocked: canonical and registered current tool surfaces differ")
	}
	if manifest.ToolSurface.BusinessWritePolicy != "diagnostic_denied" {
		return report, errors.New("business execution gate blocked: current L2 write policy is not diagnostic_denied")
	}
	if manifest.PolicyRevisions["checkpoint_policy_revision"] != surface.CheckpointPolicyRevision || manifest.PolicyRevisions["artifact_eligibility_policy_revision"] != surface.ArtifactEligibilityPolicyRevision || manifest.PolicyRevisions["contract_supersession_policy_revision"] != surface.ContractSupersessionPolicyRevision || manifest.PolicyRevisions["acceptance_checker_revision"] != surface.AcceptanceCheckerRevision {
		return report, errors.New("business execution gate blocked: current L2 policy revision binding drifted")
	}

	var live currentBinaryL2LiveQualification
	if err := readT11JSON(cfg.QualificationPath, &live); err != nil {
		return report, fmt.Errorf("business execution gate blocked: read current-binary L2 live qualification: %w", err)
	}
	if live.Qualification != r03aCurrentBinaryL2Qualification || live.Status != "passed" || live.ExecutionFingerprint != manifest.ExecutionFingerprint || live.CurrentBinaryRevised11ToolL2 != "QUALIFIED" || !live.EligibleForRevisedBackendRun {
		return report, errors.New("business execution gate blocked: current-binary revised L2 live qualification is not eligible")
	}

	binaryDigest, err := sha256File(cfg.Binary)
	if err != nil || binaryDigest != manifest.CanonicalManifest.Base.Combination.BinarySHA256 {
		return report, errors.New("business execution gate blocked: current controlled Codex binary drifted")
	}
	helperDigest, err := sha256File(cfg.CodeModeHost)
	if err != nil || helperDigest != manifest.CanonicalManifest.Base.Combination.CodeModeHostSHA256 {
		return report, errors.New("business execution gate blocked: current controlled code-mode host drifted")
	}
	selectedConfigDigest, err := sha256File(cfg.SelectedConfigPath)
	if err != nil || selectedConfigDigest != l1.SelectedConfigRawSHA256 {
		return report, errors.New("business execution gate blocked: selected config drifted from current L1")
	}
	authRaw, err := os.ReadFile(cfg.AuthFile)
	if err != nil {
		return report, fmt.Errorf("business execution gate blocked: read current auth: %w", err)
	}
	authMaterial, err := codex.ParseAuthMaterial(authRaw, manifest.CanonicalManifest.Base.Auth.AuthSourceClass)
	if err != nil || !reflect.DeepEqual(authMaterial.Manifest(), manifest.CanonicalManifest.Base.Auth) {
		return report, errors.New("business execution gate blocked: current auth identity or credential revision drifted")
	}
	if cfg.ExpectedNativeVersion != "" && cfg.ExpectedNativeVersion != manifest.CanonicalManifest.Base.Combination.CodexVersion {
		return report, errors.New("business execution gate blocked: current Codex version expectation drifted")
	}
	return R03AT2QualificationReport{
		Qualification:                   manifest.Qualification,
		ExecutionFingerprint:            manifest.ExecutionFingerprint,
		CodexVersion:                    manifest.CanonicalManifest.Base.Combination.CodexVersion,
		Model:                           manifest.CanonicalManifest.Base.Combination.Model,
		Effort:                          manifest.CanonicalManifest.Base.Combination.Effort,
		CapabilityDigest:                manifest.CanonicalManifest.Base.Combination.CapabilityDigest,
		ToolManifestDigest:              manifest.ToolSurface.AggregateManifestDigest,
		ToolCount:                       manifest.ToolSurface.ToolCount,
		AuthSourceClass:                 authMaterial.SourceClass,
		AuthIdentityFingerprint:         authMaterial.IdentityFingerprint,
		AuthCredentialRevision:          authMaterial.CredentialRevisionFingerprint,
		BehavioralContractQualification: "passed",
		RevalidatedAt:                   time.Now().UTC(),
	}, nil
}

func RecordR03ACurrentBinaryL2Offline(evidence, l1Evidence, t21dEvidence string) (map[string]any, error) {
	if evidence == "" || l1Evidence == "" || t21dEvidence == "" {
		return nil, errors.New("current-binary L2 qualification paths are required")
	}
	if entries, err := os.ReadDir(evidence); err == nil && len(entries) != 0 {
		return nil, errors.New("current-binary L2 evidence path is not fresh; refusing retry/reset")
	} else if err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	var l1 currentBinaryL1Evidence
	if err := readT11JSON(filepath.Join(l1Evidence, "result.json"), &l1); err != nil {
		return nil, fmt.Errorf("read current-binary L1 result: %w", err)
	}
	if l1.Status != "passed" || l1.CurrentBinaryWindowsL1 != "QUALIFIED" || !validCurrentBinaryDigest(l1.ExecutionFingerprint) || l1.ControlledRuntimeManifestDigest == "" || l1.CodexVersion == "" || !validCurrentBinaryDigest(l1.CodexBinarySHA256) || !validCurrentBinaryDigest(l1.CodeModeHostSHA256) || l1.AuthIdentityFingerprint == "" || !validCurrentBinaryDigest(l1.AuthCredentialRevision) || !validCurrentBinaryDigest(l1.EffectiveConfigDigest) || !validCurrentBinaryDigest(l1.EffectiveTransportConfigDigest) || l1.Model != "gpt-5.6-luna" || l1.Effort != "medium" || l1.RuntimeProfile != "windows-native" || l1.ProviderTransportPolicy != "native_default" || l1.DynamicToolCount != 0 || l1.MediumStarted != 1 || l1.ProviderEgress != 1 || l1.UnresolvedTransportState {
		return nil, errors.New("current-binary Windows L1 is not sealed as qualified")
	}

	var t21d struct {
		CanonicalManifest codex.CanonicalManifestV7 `json:"canonical_manifest"`
		ToolSurface       codex.ToolSurfaceManifest `json:"tool_surface"`
	}
	if err := readT11JSON(filepath.Join(t21dEvidence, "execution-manifest.json"), &t21d); err != nil {
		return nil, fmt.Errorf("read immutable T21D manifest: %w", err)
	}
	if err := t21d.CanonicalManifest.Validate(); err != nil {
		return nil, fmt.Errorf("immutable T21D manifest invalid: %w", err)
	}
	if err := t21d.ToolSurface.Validate(); err != nil {
		return nil, fmt.Errorf("immutable T21D tool surface invalid: %w", err)
	}

	surface, tools, toolRaw, err := buildT21ToolSurface()
	if err != nil {
		return nil, err
	}
	payloadRaw, err := json.Marshal(t21NormalizedThreadPayload(tools, r03aT21DeveloperInstruction))
	if err != nil {
		return nil, fmt.Errorf("current-binary L2 thread/start serialization: %w", err)
	}
	surface.ThreadStartPayloadDigest = digest(payloadRaw)
	surface.ThreadStartPayloadBytes = len(payloadRaw)
	if err := surface.Validate(); err != nil {
		return nil, fmt.Errorf("current formal PeerBackendTools surface invalid: %w", err)
	}
	if !reflect.DeepEqual(surface, t21d.ToolSurface) {
		return nil, errors.New("current formal 11-tool surface or policy revisions drifted from T21D without approval")
	}

	base := t21d.CanonicalManifest.Base
	versionToken := strings.TrimPrefix(l1.CodexVersion, "codex-cli ")
	base.Combination.CodexVersion = versionToken
	base.Combination.BinarySHA256 = l1.CodexBinarySHA256
	base.Combination.CodeModeHostSHA256 = l1.CodeModeHostSHA256
	base.Combination.Model = l1.Model
	base.Combination.Effort = l1.Effort
	base.Combination.CapabilityDigest = currentBinaryL2CapabilityDigest(l1.ExecutionFingerprint, surface)
	if base.Auth.AuthIdentityFingerprint != l1.AuthIdentityFingerprint || base.Auth.AuthCredentialRevisionFingerprint != l1.AuthCredentialRevision || base.EffectiveConfigDigest != l1.EffectiveConfigDigest || base.EffectiveTransportConfigDigest != l1.EffectiveTransportConfigDigest {
		return nil, errors.New("current-binary L1 auth/config factors do not match the T21D base")
	}
	manifest := codex.CanonicalManifestV7{FingerprintSchemaVersion: codex.CanonicalManifestFingerprintSchemaVersionV7, Base: base, ToolSurface: surface}
	if err := manifest.Validate(); err != nil {
		return nil, fmt.Errorf("current-binary L2 canonical manifest invalid: %w", err)
	}
	v7Fingerprint := manifest.Base.Combination.CurrentFingerprintV7(manifest)

	binding := currentBinaryL2Binding{
		SchemaVersion:                      r03aCurrentBinaryL2BindingSchema,
		L1Fingerprint:                      l1.ExecutionFingerprint,
		ControlledRuntimeManifestDigest:    l1.ControlledRuntimeManifestDigest,
		CodexVersion:                       l1.CodexVersion,
		CodexBinarySHA256:                  l1.CodexBinarySHA256,
		CodeModeHostSHA256:                 l1.CodeModeHostSHA256,
		ToolManifestDigest:                 surface.AggregateManifestDigest,
		AggregateSchemaDigest:              surface.AggregateSchemaDigest,
		AggregateSchemaBytes:               surface.AggregateSchemaBytes,
		ToolCount:                          surface.ToolCount,
		CheckpointPolicyRevision:           surface.CheckpointPolicyRevision,
		ArtifactEligibilityPolicyRevision:  surface.ArtifactEligibilityPolicyRevision,
		ContractSupersessionPolicyRevision: surface.ContractSupersessionPolicyRevision,
		AcceptanceCheckerRevision:          surface.AcceptanceCheckerRevision,
	}
	executionFingerprint, err := binding.Fingerprint()
	if err != nil {
		return nil, err
	}
	policyRevisions := map[string]string{
		"checkpoint_policy_revision":            surface.CheckpointPolicyRevision,
		"artifact_eligibility_policy_revision":  surface.ArtifactEligibilityPolicyRevision,
		"contract_supersession_policy_revision": surface.ContractSupersessionPolicyRevision,
		"acceptance_checker_revision":           surface.AcceptanceCheckerRevision,
	}
	diff := map[string]any{
		"reference":                         "R0.3A-T21D",
		"surface_exact_match":               true,
		"unexplained_schema_drift":          []any{},
		"unexplained_handler_drift":         []any{},
		"unexplained_binding_drift":         []any{},
		"unexplained_policy_drift":          []any{},
		"approved_factors":                  []string{"current_binary_l1_fingerprint", "controlled_runtime_manifest_digest", "current_codex_binary_sha256", "current_code_mode_host_sha256", "current_codex_version"},
		"policy_revisions_bound_explicitly": true,
		"historical_t21d_modified":          false,
	}
	out := map[string]any{
		"qualification":                      r03aCurrentBinaryL2Qualification,
		"status":                             "PASSED",
		"current_binary_l1":                  "QUALIFIED",
		"tool_count":                         surface.ToolCount,
		"tool_names":                         t21ToolNames(surface),
		"tool_manifest_digest":               surface.AggregateManifestDigest,
		"aggregate_schema_digest":            surface.AggregateSchemaDigest,
		"aggregate_schema_bytes":             surface.AggregateSchemaBytes,
		"thread_start_payload_digest":        surface.ThreadStartPayloadDigest,
		"thread_start_payload_bytes":         surface.ThreadStartPayloadBytes,
		"handler_binding_digest":             digest(toolRaw),
		"policy_revisions":                   policyRevisions,
		"canonical_manifest_digest":          v7Fingerprint.CanonicalManifestDigest,
		"execution_fingerprint":              executionFingerprint,
		"l1_fingerprint":                     l1.ExecutionFingerprint,
		"controlled_runtime_manifest_digest": l1.ControlledRuntimeManifestDigest,
		"codex_version":                      l1.CodexVersion,
		"codex_binary_sha256":                l1.CodexBinarySHA256,
		"code_mode_host_sha256":              l1.CodeModeHostSHA256,
		"model":                              l1.Model,
		"effort":                             l1.Effort,
		"dynamic_tool_count":                 surface.ToolCount,
		"business_write_policy":              surface.BusinessWritePolicy,
		"semantic_diff":                      diff,
		"unrelated_drift":                    false,
		"medium":                             0,
		"high":                               0,
		"provider_egress":                    0,
		"historical_evidence_modified":       false,
		"generated_at":                       time.Now().UTC(),
	}

	if err := os.MkdirAll(evidence, 0700); err != nil {
		return nil, err
	}
	if err := os.WriteFile(filepath.Join(evidence, "tool-registry.json"), append(toolRaw, '\n'), 0600); err != nil {
		return nil, err
	}
	if err := writeJSON(filepath.Join(evidence, "tool-surface.json"), map[string]any{"schema_version": "canonical-tool-manifest-v1", "source": "codex.PeerBackendTools", "surface": surface, "tool_names": t21ToolNames(surface), "tools": json.RawMessage(toolRaw)}); err != nil {
		return nil, err
	}
	if err := writeJSON(filepath.Join(evidence, "execution-manifest.json"), map[string]any{
		"schema_version":                      r03aCurrentBinaryL2ManifestSchema,
		"qualification":                       r03aCurrentBinaryL2Qualification,
		"fingerprint_schema_version":          codex.CanonicalManifestFingerprintSchemaVersionV7,
		"canonical_manifest":                  manifest,
		"canonical_manifest_digest":           v7Fingerprint.CanonicalManifestDigest,
		"execution_fingerprint":               executionFingerprint,
		"l1_fingerprint":                      l1.ExecutionFingerprint,
		"controlled_runtime_manifest_digest":  l1.ControlledRuntimeManifestDigest,
		"tool_surface":                        surface,
		"tool_manifest_digest":                surface.AggregateManifestDigest,
		"tool_names":                          t21ToolNames(surface),
		"policy_revisions":                    policyRevisions,
		"binding":                             binding,
		"thread_start_payload_canonical_json": string(payloadRaw),
		"thread_start_payload_digest":         surface.ThreadStartPayloadDigest,
		"thread_start_payload_bytes":          surface.ThreadStartPayloadBytes,
		"historical_t21d_modified":            false,
	}); err != nil {
		return nil, err
	}
	if err := writeJSON(filepath.Join(evidence, "semantic-diff.json"), diff); err != nil {
		return nil, err
	}
	if err := writeJSON(filepath.Join(evidence, "preflight.json"), map[string]any{
		"qualification":                      r03aCurrentBinaryL2Qualification,
		"status":                             "offline_preflight_passed",
		"passed":                             true,
		"current_binary_l1":                  "QUALIFIED",
		"l1_fingerprint":                     l1.ExecutionFingerprint,
		"controlled_runtime_manifest_digest": l1.ControlledRuntimeManifestDigest,
		"tool_count":                         surface.ToolCount,
		"tool_manifest_digest":               surface.AggregateManifestDigest,
		"aggregate_schema_digest":            surface.AggregateSchemaDigest,
		"aggregate_schema_bytes":             surface.AggregateSchemaBytes,
		"handler_binding_digest":             digest(toolRaw),
		"policy_revisions":                   policyRevisions,
		"t21d_surface_exact_match":           true,
		"unexplained_drift":                  false,
		"medium":                             0,
		"high":                               0,
		"provider_egress":                    0,
		"historical_evidence_modified":       false,
	}); err != nil {
		return nil, err
	}
	if err := writeJSON(filepath.Join(evidence, "offline-result.json"), out); err != nil {
		return nil, err
	}
	return out, nil
}

func VerifyR03ACurrentBinaryL2Offline(evidence string) error {
	var result struct {
		Qualification         string `json:"qualification"`
		Status                string `json:"status"`
		CurrentBinaryL1       string `json:"current_binary_l1"`
		ToolCount             int    `json:"tool_count"`
		ToolManifestDigest    string `json:"tool_manifest_digest"`
		AggregateSchemaDigest string `json:"aggregate_schema_digest"`
		AggregateSchemaBytes  int    `json:"aggregate_schema_bytes"`
		ExecutionFingerprint  string `json:"execution_fingerprint"`
		Medium                int    `json:"medium"`
		High                  int    `json:"high"`
		ProviderEgress        int    `json:"provider_egress"`
		UnrelatedDrift        bool   `json:"unrelated_drift"`
	}
	if err := readT11JSON(filepath.Join(evidence, "offline-result.json"), &result); err != nil {
		return err
	}
	if result.Qualification != r03aCurrentBinaryL2Qualification || result.Status != "PASSED" || result.CurrentBinaryL1 != "QUALIFIED" || result.ToolCount != 11 || !validCurrentBinaryDigest(result.ToolManifestDigest) || !validCurrentBinaryDigest(result.AggregateSchemaDigest) || result.AggregateSchemaBytes <= 0 || !validCurrentBinaryDigest(result.ExecutionFingerprint) || result.Medium != 0 || result.High != 0 || result.ProviderEgress != 0 || result.UnrelatedDrift {
		return errors.New("current-binary L2 offline qualification is not passed")
	}
	var manifest struct {
		ExecutionFingerprint string                    `json:"execution_fingerprint"`
		ToolSurface          codex.ToolSurfaceManifest `json:"tool_surface"`
	}
	if err := readT11JSON(filepath.Join(evidence, "execution-manifest.json"), &manifest); err != nil {
		return err
	}
	if manifest.ExecutionFingerprint != result.ExecutionFingerprint || manifest.ToolSurface.ToolCount != 11 || manifest.ToolSurface.AggregateManifestDigest != result.ToolManifestDigest || manifest.ToolSurface.AggregateSchemaDigest != result.AggregateSchemaDigest || manifest.ToolSurface.AggregateSchemaBytes != result.AggregateSchemaBytes || manifest.ToolSurface.Validate() != nil {
		return errors.New("current-binary L2 offline manifest is inconsistent")
	}
	return nil
}

func currentBinaryL2CapabilityDigest(l1Fingerprint string, surface codex.ToolSurfaceManifest) string {
	raw := []byte("r03a-current-binary-revised-l2-capability-v1\x00" + l1Fingerprint + "\x00" + surface.AggregateManifestDigest + "\x00" + surface.AggregateSchemaDigest + "\x00" + fmt.Sprint(surface.ToolCount))
	digestValue := sha256.Sum256(raw)
	return hex.EncodeToString(digestValue[:])
}

func validCurrentBinaryDigest(value string) bool {
	if len(value) != sha256.Size*2 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

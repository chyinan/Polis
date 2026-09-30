// pattern: Imperative Shell
package probe

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"polis/internal/codex"
	"polis/internal/core"
	"reflect"
	"time"
)

const (
	r03aFrontendL2Qualification   = "R0.3A-CURRENT-BINARY-REVISED-FRONTEND-L2"
	r03aFrontendL2ManifestSchema  = "r03a-current-binary-revised-frontend-l2-manifest-v1"
	r03aFrontendL2SurfaceID       = "peer_frontend"
	r03aFrontendL2PolicyWriteMode = "diagnostic_denied"
)

var r03aFrontendToolBindings = map[string]string{
	"work_current":       "PeerEmployeeTools.call->Handover",
	"context_read":       "PeerEmployeeTools.call->Handover",
	"workspace_read":     "PeerEmployeeTools.call->Workspace",
	"collab_inbox":       "PeerEmployeeTools.call->PeerInbox",
	"contract_read":      "PeerEmployeeTools.call->PeerContractRead",
	"collab_ack":         "PeerEmployeeTools.call->TXPeerAck",
	"collab_apply":       "PeerEmployeeTools.call->TXPeerApply",
	"workspace_replace":  "PeerEmployeeTools.call->TXReplace",
	"workspace_check":    "PeerEmployeeTools.call->workspace.check",
	"work_checkpoint":    "PeerEmployeeTools.call->TXCheckpoint",
	"artifact_submit":    "PeerEmployeeTools.call->TXSubmit",
	"obligation_resolve": "PeerEmployeeTools.call->TXPeerResolve",
}

type r03aFrontendL2OldSurface struct {
	Surface codex.ToolSurfaceManifest `json:"surface"`
	Tools   []any                     `json:"tools"`
}

func buildR03AFrontendToolSurface() (codex.ToolSurfaceManifest, []any, []byte, error) {
	return buildPeerToolSurface(codex.PeerFrontendTools(), 12, r03aFrontendL2SurfaceID, r03aFrontendToolBindings, r03aFrontendL2PolicyWriteMode)
}

func loadHandoverV4FrontendToolSurface(protocolPath string) (codex.ToolSurfaceManifest, []any, []byte, error) {
	file, err := os.Open(protocolPath)
	if err != nil {
		return codex.ToolSurfaceManifest{}, nil, nil, fmt.Errorf("open handover-v4 protocol: %w", err)
	}
	defer file.Close()
	var rawTools json.RawMessage
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var entry struct {
			Direction string `json:"direction"`
			Data      struct {
				Method string          `json:"method"`
				Params json.RawMessage `json:"params"`
			} `json:"data"`
		}
		if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil || entry.Direction != "send" || entry.Data.Method != "thread/start" {
			continue
		}
		var params struct {
			DynamicTools json.RawMessage `json:"dynamicTools"`
		}
		if err := json.Unmarshal(entry.Data.Params, &params); err != nil || len(params.DynamicTools) == 0 {
			continue
		}
		rawTools = append([]byte(nil), params.DynamicTools...)
		break
	}
	if err := scanner.Err(); err != nil {
		return codex.ToolSurfaceManifest{}, nil, nil, err
	}
	if len(rawTools) == 0 {
		return codex.ToolSurfaceManifest{}, nil, nil, errors.New("handover-v4 protocol has no Frontend thread/start tool surface")
	}
	var tools []any
	if err := json.Unmarshal(rawTools, &tools); err != nil {
		return codex.ToolSurfaceManifest{}, nil, nil, fmt.Errorf("decode handover-v4 Frontend tool surface: %w", err)
	}
	surface, tools, canonical, err := buildPeerToolSurface(tools, 12, r03aFrontendL2SurfaceID, r03aFrontendToolBindings, "historical_handover_v4")
	if err != nil {
		return codex.ToolSurfaceManifest{}, nil, nil, err
	}
	// Historical provider payloads did not carry the current local policy
	// revision fields; the schema/description diff is the authority here.
	surface.CheckpointPolicyRevision = ""
	surface.ArtifactEligibilityPolicyRevision = ""
	surface.ContractSupersessionPolicyRevision = ""
	surface.AcceptanceCheckerRevision = ""
	return surface, tools, canonical, nil
}

func compareR03AFrontendL2Surface(old, current codex.ToolSurfaceManifest) map[string]any {
	diff := make([]map[string]any, 0)
	unexplained := make([]map[string]any, 0)
	if old.ToolCount != current.ToolCount || len(old.Tools) != len(current.Tools) {
		unexplained = append(unexplained, map[string]any{"kind": "registration", "reason": "tool count changed"})
	}
	for index := 0; index < len(old.Tools) && index < len(current.Tools); index++ {
		previous, actual := old.Tools[index], current.Tools[index]
		if previous.Name != actual.Name || previous.BindingIdentity != actual.BindingIdentity || previous.AuthorizationClass != actual.AuthorizationClass || previous.RegistrationOrdinal != actual.RegistrationOrdinal {
			unexplained = append(unexplained, map[string]any{"kind": "binding_or_registration", "ordinal": index + 1, "old": previous, "current": actual})
			continue
		}
		if previous.SchemaDigest == actual.SchemaDigest && previous.SchemaBytes == actual.SchemaBytes && previous.DescriptionDigest == actual.DescriptionDigest {
			continue
		}
		change := map[string]any{"tool": actual.Name, "old_schema_digest": previous.SchemaDigest, "current_schema_digest": actual.SchemaDigest, "old_schema_bytes": previous.SchemaBytes, "current_schema_bytes": actual.SchemaBytes, "old_description_digest": previous.DescriptionDigest, "current_description_digest": actual.DescriptionDigest}
		if actual.Name == "polis_collab_apply" || actual.Name == "polis_work_checkpoint" {
			change["approved_scope"] = "behavioral_contract_hardening"
			diff = append(diff, change)
		} else {
			change["approved_scope"] = false
			unexplained = append(unexplained, change)
		}
	}
	return map[string]any{
		"old_surface_id":        old.SurfaceID,
		"current_surface_id":    current.SurfaceID,
		"old_tool_count":        old.ToolCount,
		"current_tool_count":    current.ToolCount,
		"approved_changes":      diff,
		"unexplained_drift":     unexplained,
		"surface_exact_names":   old.ToolCount == current.ToolCount && len(unexplained) == 0,
		"approved_change_count": len(diff),
	}
}

func currentFrontendL2CapabilityDigest(l1Fingerprint string, surface codex.ToolSurfaceManifest) string {
	return currentBinaryL2CapabilityDigest(l1Fingerprint, surface)
}

func RecordR03AFrontendL2Offline(evidence, l1Evidence, baseL2Evidence, handoverProtocol string) (map[string]any, error) {
	if evidence == "" || l1Evidence == "" || baseL2Evidence == "" || handoverProtocol == "" {
		return nil, errors.New("Frontend L2 offline qualification paths are required")
	}
	if entries, err := os.ReadDir(evidence); err == nil && len(entries) != 0 {
		return nil, errors.New("Frontend L2 evidence path is not fresh; refusing overwrite")
	} else if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	var l1 currentBinaryL1Evidence
	if err := readT11JSON(filepath.Join(l1Evidence, "result.json"), &l1); err != nil {
		return nil, fmt.Errorf("read current Windows L1 evidence: %w", err)
	}
	if l1.Status != "passed" || l1.CurrentBinaryWindowsL1 != "QUALIFIED" || !validCurrentBinaryDigest(l1.ExecutionFingerprint) || l1.Model != "gpt-5.6-luna" || l1.Effort != "medium" || l1.RuntimeProfile != "windows-native" || l1.ProviderTransportPolicy != "native_default" || l1.DynamicToolCount != 0 || l1.MediumStarted != 1 || l1.ProviderEgress != 1 || l1.UnresolvedTransportState {
		return nil, errors.New("current Windows L1 evidence is not sealed as qualified")
	}
	var base struct {
		CanonicalManifest codex.CanonicalManifestV7 `json:"canonical_manifest"`
	}
	if err := readT11JSON(filepath.Join(baseL2Evidence, "execution-manifest.json"), &base); err != nil {
		return nil, fmt.Errorf("read current-binary L2 base manifest: %w", err)
	}
	if err := base.CanonicalManifest.Validate(); err != nil {
		return nil, fmt.Errorf("current-binary L2 base manifest invalid: %w", err)
	}
	if base.CanonicalManifest.Base.Combination.BinarySHA256 != l1.CodexBinarySHA256 || base.CanonicalManifest.Base.Combination.CodeModeHostSHA256 != l1.CodeModeHostSHA256 || base.CanonicalManifest.Base.Auth.AuthIdentityFingerprint != l1.AuthIdentityFingerprint || base.CanonicalManifest.Base.Auth.AuthCredentialRevisionFingerprint != l1.AuthCredentialRevision || base.CanonicalManifest.Base.EffectiveConfigDigest != l1.EffectiveConfigDigest || base.CanonicalManifest.Base.EffectiveTransportConfigDigest != l1.EffectiveTransportConfigDigest {
		return nil, errors.New("current-binary L2 base factors do not match current Windows L1")
	}
	oldSurface, _, _, err := loadHandoverV4FrontendToolSurface(handoverProtocol)
	if err != nil {
		return nil, err
	}
	currentSurface, tools, toolRaw, err := buildR03AFrontendToolSurface()
	if err != nil {
		return nil, err
	}
	payload, err := json.Marshal(t21NormalizedThreadPayload(tools, r03aT21DeveloperInstruction))
	if err != nil {
		return nil, err
	}
	currentSurface.ThreadStartPayloadDigest = digest(payload)
	currentSurface.ThreadStartPayloadBytes = len(payload)
	if err := currentSurface.Validate(); err != nil {
		return nil, fmt.Errorf("current Frontend tool surface invalid: %w", err)
	}
	diff := compareR03AFrontendL2Surface(oldSurface, currentSurface)
	approvedChanges, _ := diff["approved_changes"].([]map[string]any)
	unexplained, _ := diff["unexplained_drift"].([]map[string]any)
	if currentSurface.ToolCount != 12 || len(approvedChanges) != 2 || len(unexplained) != 0 {
		return nil, errors.New("current Frontend surface has unexplained or incomplete hardening drift")
	}
	manifestBase := base.CanonicalManifest.Base
	manifestBase.Combination.CapabilityDigest = currentFrontendL2CapabilityDigest(l1.ExecutionFingerprint, currentSurface)
	manifest := codex.CanonicalManifestV7{FingerprintSchemaVersion: codex.CanonicalManifestFingerprintSchemaVersionV7, Base: manifestBase, ToolSurface: currentSurface}
	if err := manifest.Validate(); err != nil {
		return nil, fmt.Errorf("current Frontend L2 canonical manifest invalid: %w", err)
	}
	fingerprint := manifest.Base.Combination.CurrentFingerprintV7(manifest)
	executionFingerprint := fingerprint.CanonicalManifestDigest
	policyRevisions := map[string]string{
		"checkpoint_policy_revision":            currentSurface.CheckpointPolicyRevision,
		"artifact_eligibility_policy_revision":  currentSurface.ArtifactEligibilityPolicyRevision,
		"contract_supersession_policy_revision": currentSurface.ContractSupersessionPolicyRevision,
		"acceptance_checker_revision":           currentSurface.AcceptanceCheckerRevision,
	}
	result := map[string]any{
		"qualification": r03aFrontendL2Qualification, "status": "PASSED", "surface_role": "peer_frontend", "current_binary_l1": "QUALIFIED",
		"tool_count": currentSurface.ToolCount, "tool_names": t21ToolNames(currentSurface), "tool_manifest_digest": currentSurface.AggregateManifestDigest,
		"aggregate_schema_digest": currentSurface.AggregateSchemaDigest, "aggregate_schema_bytes": currentSurface.AggregateSchemaBytes,
		"thread_start_payload_digest": currentSurface.ThreadStartPayloadDigest, "thread_start_payload_bytes": currentSurface.ThreadStartPayloadBytes,
		"handler_binding_digest": digest(toolRaw), "policy_revisions": policyRevisions, "canonical_manifest_digest": fingerprint.CanonicalManifestDigest,
		"execution_fingerprint": executionFingerprint, "l1_fingerprint": l1.ExecutionFingerprint, "controlled_runtime_manifest_digest": l1.ControlledRuntimeManifestDigest,
		"codex_version": l1.CodexVersion, "codex_binary_sha256": l1.CodexBinarySHA256, "code_mode_host_sha256": l1.CodeModeHostSHA256,
		"auth_source_class": l1.AuthSourceClass, "auth_identity_fingerprint": l1.AuthIdentityFingerprint, "auth_credential_revision_fingerprint": l1.AuthCredentialRevision,
		"effective_config_digest": l1.EffectiveConfigDigest, "effective_transport_config_digest": l1.EffectiveTransportConfigDigest,
		"provider_transport_policy": l1.ProviderTransportPolicy, "proxy_policy": "no_injected_proxy", "model": l1.Model, "effort": l1.Effort,
		"dynamic_tool_count": currentSurface.ToolCount, "business_write_policy": currentSurface.BusinessWritePolicy, "semantic_diff": diff,
		"unrelated_drift": false, "medium": 0, "high": 0, "provider_egress": 0, "historical_evidence_modified": false,
		"generated_at": time.Now().UTC(),
	}
	if err := os.MkdirAll(evidence, 0700); err != nil {
		return nil, err
	}
	if err := os.WriteFile(filepath.Join(evidence, "tool-registry.json"), append(toolRaw, '\n'), 0600); err != nil {
		return nil, err
	}
	if err := writeJSON(filepath.Join(evidence, "tool-surface.json"), map[string]any{"schema_version": "canonical-tool-manifest-v1", "source": "codex.PeerFrontendTools", "surface": currentSurface, "tool_names": t21ToolNames(currentSurface), "tools": json.RawMessage(toolRaw)}); err != nil {
		return nil, err
	}
	if err := writeJSON(filepath.Join(evidence, "semantic-diff.json"), diff); err != nil {
		return nil, err
	}
	if err := writeJSON(filepath.Join(evidence, "execution-manifest.json"), map[string]any{
		"schema_version": r03aFrontendL2ManifestSchema, "qualification": r03aFrontendL2Qualification,
		"fingerprint_schema_version": codex.CanonicalManifestFingerprintSchemaVersionV7, "canonical_manifest": manifest,
		"canonical_manifest_digest": fingerprint.CanonicalManifestDigest, "execution_fingerprint": executionFingerprint,
		"l1_fingerprint": l1.ExecutionFingerprint, "controlled_runtime_manifest_digest": l1.ControlledRuntimeManifestDigest,
		"tool_surface": currentSurface, "tool_manifest_digest": currentSurface.AggregateManifestDigest, "tool_names": t21ToolNames(currentSurface),
		"policy_revisions": policyRevisions, "thread_start_payload_canonical_json": string(payload), "thread_start_payload_digest": currentSurface.ThreadStartPayloadDigest,
		"thread_start_payload_bytes": currentSurface.ThreadStartPayloadBytes, "handler_binding_digest": digest(toolRaw), "historical_evidence_modified": false,
	}); err != nil {
		return nil, err
	}
	if err := writeJSON(filepath.Join(evidence, "preflight.json"), map[string]any{
		"qualification": r03aFrontendL2Qualification, "status": "offline_preflight_passed", "passed": true, "surface_role": "peer_frontend",
		"current_binary_l1": "QUALIFIED", "l1_fingerprint": l1.ExecutionFingerprint, "execution_fingerprint": executionFingerprint,
		"tool_count": currentSurface.ToolCount, "tool_manifest_digest": currentSurface.AggregateManifestDigest, "aggregate_schema_digest": currentSurface.AggregateSchemaDigest,
		"aggregate_schema_bytes": currentSurface.AggregateSchemaBytes, "handler_binding_digest": digest(toolRaw), "policy_revisions": policyRevisions,
		"approved_hardening_diff": true, "unexplained_drift": false, "medium": 0, "high": 0, "provider_egress": 0, "historical_evidence_modified": false,
	}); err != nil {
		return nil, err
	}
	if err := writeJSON(filepath.Join(evidence, "offline-result.json"), result); err != nil {
		return nil, err
	}
	return result, nil
}

func VerifyR03AFrontendL2Offline(evidence string) error {
	var result struct {
		Qualification         string `json:"qualification"`
		Status                string `json:"status"`
		SurfaceRole           string `json:"surface_role"`
		ToolCount             int    `json:"tool_count"`
		ToolManifestDigest    string `json:"tool_manifest_digest"`
		AggregateSchemaDigest string `json:"aggregate_schema_digest"`
		AggregateSchemaBytes  int    `json:"aggregate_schema_bytes"`
		ExecutionFingerprint  string `json:"execution_fingerprint"`
		Medium                int    `json:"medium"`
		High                  int    `json:"high"`
		ProviderEgress        int    `json:"provider_egress"`
	}
	if err := readT11JSON(filepath.Join(evidence, "offline-result.json"), &result); err != nil {
		return err
	}
	if result.Qualification != r03aFrontendL2Qualification || result.Status != "PASSED" || result.SurfaceRole != "peer_frontend" || result.ToolCount != 12 || !validCurrentBinaryDigest(result.ToolManifestDigest) || !validCurrentBinaryDigest(result.AggregateSchemaDigest) || result.AggregateSchemaBytes <= 0 || !validCurrentBinaryDigest(result.ExecutionFingerprint) || result.Medium != 0 || result.High != 0 || result.ProviderEgress != 0 {
		return errors.New("Frontend L2 offline qualification is not passed")
	}
	var manifest struct {
		SchemaVersion        string                    `json:"schema_version"`
		Qualification        string                    `json:"qualification"`
		ExecutionFingerprint string                    `json:"execution_fingerprint"`
		ToolSurface          codex.ToolSurfaceManifest `json:"tool_surface"`
	}
	if err := readT11JSON(filepath.Join(evidence, "execution-manifest.json"), &manifest); err != nil {
		return err
	}
	if manifest.SchemaVersion != r03aFrontendL2ManifestSchema || manifest.Qualification != r03aFrontendL2Qualification || manifest.ExecutionFingerprint != result.ExecutionFingerprint || manifest.ToolSurface.ToolCount != 12 || manifest.ToolSurface.AggregateManifestDigest != result.ToolManifestDigest || manifest.ToolSurface.AggregateSchemaDigest != result.AggregateSchemaDigest || manifest.ToolSurface.AggregateSchemaBytes != result.AggregateSchemaBytes || manifest.ToolSurface.Validate() != nil {
		return errors.New("Frontend L2 offline manifest is inconsistent")
	}
	return nil
}

func requireCurrentBinaryRevisedFrontendL2Qualification(cfg R03AT2Config) (R03AT2QualificationReport, error) {
	var report R03AT2QualificationReport
	var manifest struct {
		SchemaVersion                   string                    `json:"schema_version"`
		Qualification                   string                    `json:"qualification"`
		CanonicalManifest               codex.CanonicalManifestV7 `json:"canonical_manifest"`
		CanonicalManifestDigest         string                    `json:"canonical_manifest_digest"`
		ExecutionFingerprint            string                    `json:"execution_fingerprint"`
		L1Fingerprint                   string                    `json:"l1_fingerprint"`
		ControlledRuntimeManifestDigest string                    `json:"controlled_runtime_manifest_digest"`
		ToolSurface                     codex.ToolSurfaceManifest `json:"tool_surface"`
		ToolManifestDigest              string                    `json:"tool_manifest_digest"`
	}
	if err := readT11JSON(cfg.ExecutionManifestPath, &manifest); err != nil {
		return report, fmt.Errorf("Frontend execution gate blocked: read L2 manifest: %w", err)
	}
	if manifest.SchemaVersion != r03aFrontendL2ManifestSchema || manifest.Qualification != r03aFrontendL2Qualification || manifest.ExecutionFingerprint != cfg.ExecutionFingerprint || !validCurrentBinaryDigest(manifest.ExecutionFingerprint) || manifest.ToolSurface.ToolCount != 12 || manifest.ToolManifestDigest != manifest.ToolSurface.AggregateManifestDigest || manifest.ToolSurface.BusinessWritePolicy != r03aFrontendL2PolicyWriteMode {
		return report, errors.New("Frontend execution gate blocked: revised Frontend L2 manifest is stale or mismatched")
	}
	if err := manifest.CanonicalManifest.Validate(); err != nil {
		return report, fmt.Errorf("Frontend execution gate blocked: invalid Frontend L2 canonical manifest: %w", err)
	}
	expectedFingerprint := manifest.CanonicalManifest.Base.Combination.CurrentFingerprintV7(manifest.CanonicalManifest)
	if manifest.CanonicalManifestDigest != expectedFingerprint.CanonicalManifestDigest {
		return report, errors.New("Frontend execution gate blocked: Frontend L2 canonical manifest digest mismatch")
	}
	if !reflect.DeepEqual(manifest.CanonicalManifest.ToolSurface, manifest.ToolSurface) {
		return report, errors.New("Frontend execution gate blocked: canonical and registered Frontend surfaces differ")
	}

	var l1 currentBinaryL1Evidence
	if cfg.CurrentL1EvidencePath == "" || readT11JSON(filepath.Join(cfg.CurrentL1EvidencePath, "result.json"), &l1) != nil || l1.Status != "passed" || l1.CurrentBinaryWindowsL1 != "QUALIFIED" || l1.ExecutionFingerprint != manifest.L1Fingerprint || l1.ControlledRuntimeManifestDigest != manifest.ControlledRuntimeManifestDigest || l1.CodexBinarySHA256 != manifest.CanonicalManifest.Base.Combination.BinarySHA256 || l1.CodeModeHostSHA256 != manifest.CanonicalManifest.Base.Combination.CodeModeHostSHA256 || l1.Model != "gpt-5.6-luna" || l1.Effort != "medium" || l1.RuntimeProfile != "windows-native" || l1.ProviderTransportPolicy != "native_default" || l1.DynamicToolCount != 0 || l1.MediumStarted != 1 || l1.ProviderEgress != 1 || l1.UnresolvedTransportState {
		return report, errors.New("Frontend execution gate blocked: current Windows L1 evidence is stale or mismatched")
	}
	currentSurface, tools, _, err := buildR03AFrontendToolSurface()
	if err != nil {
		return report, err
	}
	payload, err := json.Marshal(t21NormalizedThreadPayload(tools, r03aT21DeveloperInstruction))
	if err != nil {
		return report, err
	}
	currentSurface.ThreadStartPayloadDigest = digest(payload)
	currentSurface.ThreadStartPayloadBytes = len(payload)
	if !reflect.DeepEqual(currentSurface, manifest.ToolSurface) {
		return report, errors.New("Frontend execution gate blocked: formal PeerFrontendTools surface drifted")
	}

	var live struct {
		Qualification            string `json:"qualification"`
		Status                   string `json:"status"`
		ExecutionFingerprint     string `json:"execution_fingerprint"`
		CurrentFrontendRevisedL2 string `json:"current_frontend_revised_l2"`
		Eligible                 bool   `json:"eligible_for_revised_frontend_initial_live"`
		RegisteredToolCount      int    `json:"registered_tool_count"`
		ToolManifestDigest       string `json:"tool_manifest_digest"`
		AggregateSchemaDigest    string `json:"aggregate_schema_digest"`
		AggregateSchemaBytes     int    `json:"aggregate_schema_bytes"`
	}
	if err := readT11JSON(cfg.QualificationPath, &live); err != nil {
		return report, fmt.Errorf("Frontend execution gate blocked: read live L2 qualification: %w", err)
	}
	if live.Qualification != r03aFrontendL2Qualification || live.Status != "passed" || live.CurrentFrontendRevisedL2 != "QUALIFIED" || !live.Eligible || live.RegisteredToolCount != manifest.ToolSurface.ToolCount || live.ToolManifestDigest != manifest.ToolSurface.AggregateManifestDigest || live.AggregateSchemaDigest != manifest.ToolSurface.AggregateSchemaDigest || live.AggregateSchemaBytes != manifest.ToolSurface.AggregateSchemaBytes {
		return report, errors.New("Frontend execution gate blocked: revised Frontend L2 live qualification is not eligible")
	}
	if live.ExecutionFingerprint != manifest.ExecutionFingerprint {
		var feedback struct {
			Qualification       string            `json:"qualification"`
			Status              string            `json:"status"`
			RegistrationChanged bool              `json:"provider_visible_registration_changed"`
			PolicyRevisions     map[string]string `json:"policy_revisions"`
		}
		if cfg.CheckerFeedbackQualificationPath == "" || readT11JSON(cfg.CheckerFeedbackQualificationPath, &feedback) != nil || feedback.Qualification != "R0.3A-FRONTEND-CHECKER-FEEDBACK-L2" || feedback.Status != "PASSED" || feedback.RegistrationChanged || feedback.PolicyRevisions["acceptance_checker_revision"] != core.PeerAcceptanceCheckerRevision || feedback.PolicyRevisions["checkpoint_policy_revision"] != core.CheckpointPolicyRevision || feedback.PolicyRevisions["artifact_eligibility_policy_revision"] != core.ArtifactEligibilityPolicyRevision || feedback.PolicyRevisions["contract_supersession_policy_revision"] != core.PeerContractSupersessionPolicyRevision {
			return report, errors.New("Frontend execution gate blocked: checker feedback qualification is stale or missing for reused transport L2")
		}
	}

	binaryDigest, err := sha256File(cfg.Binary)
	if err != nil || binaryDigest != manifest.CanonicalManifest.Base.Combination.BinarySHA256 {
		return report, errors.New("Frontend execution gate blocked: controlled Codex binary drifted")
	}
	helperDigest, err := sha256File(cfg.CodeModeHost)
	if err != nil || helperDigest != manifest.CanonicalManifest.Base.Combination.CodeModeHostSHA256 {
		return report, errors.New("Frontend execution gate blocked: controlled code-mode host drifted")
	}
	selectedConfigDigest, err := sha256File(cfg.SelectedConfigPath)
	if err != nil || selectedConfigDigest != l1.SelectedConfigRawSHA256 {
		return report, errors.New("Frontend execution gate blocked: selected config drifted from current L1")
	}
	authRaw, err := os.ReadFile(cfg.AuthFile)
	if err != nil {
		return report, fmt.Errorf("Frontend execution gate blocked: read current auth: %w", err)
	}
	authMaterial, err := codex.ParseAuthMaterial(authRaw, manifest.CanonicalManifest.Base.Auth.AuthSourceClass)
	if err != nil || !reflect.DeepEqual(authMaterial.Manifest(), manifest.CanonicalManifest.Base.Auth) {
		return report, errors.New("Frontend execution gate blocked: auth identity or credential revision drifted")
	}
	if cfg.ExpectedNativeVersion != "" && cfg.ExpectedNativeVersion != manifest.CanonicalManifest.Base.Combination.CodexVersion {
		return report, errors.New("Frontend execution gate blocked: Codex version expectation drifted")
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

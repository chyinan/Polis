// pattern: Imperative Shell
package probe

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	"polis/internal/codex"
	"polis/internal/fixture"
)

type BackendL2SchemaChange struct {
	Tool        string `json:"tool"`
	Path        string `json:"path"`
	OldRequired string `json:"old_required"`
	NewRequired string `json:"new_required"`
	OldBytes    int    `json:"old_schema_bytes"`
	NewBytes    int    `json:"new_schema_bytes"`
	Reason      string `json:"reason"`
}

type BackendL2SurfaceDiff struct {
	Explained          bool                    `json:"explained"`
	OldToolCount       int                     `json:"old_tool_count"`
	CurrentToolCount   int                     `json:"current_tool_count"`
	ApprovedChanges    []BackendL2SchemaChange `json:"approved_changes"`
	UnexplainedChanges []string                `json:"unexplained_changes"`
}

type BackendL2RequalificationConfig struct {
	Evidence                string
	OldEvidence             string
	L1Evidence              string
	AcceptanceQualification string
	Binary                  string
	CodeModeHost            string
	AuthFile                string
	SelectedConfig          string
	RuntimeManifest         string
}

func compareBackendL2ToolRegistry(oldRaw, currentRaw []byte) (BackendL2SurfaceDiff, error) {
	var oldTools, currentTools []map[string]any
	if err := json.Unmarshal(oldRaw, &oldTools); err != nil {
		return BackendL2SurfaceDiff{}, fmt.Errorf("old Backend tool registry is invalid: %w", err)
	}
	if err := json.Unmarshal(currentRaw, &currentTools); err != nil {
		return BackendL2SurfaceDiff{}, fmt.Errorf("current Backend tool registry is invalid: %w", err)
	}
	diff := BackendL2SurfaceDiff{OldToolCount: len(oldTools), CurrentToolCount: len(currentTools)}
	if len(oldTools) != len(currentTools) {
		diff.UnexplainedChanges = append(diff.UnexplainedChanges, "tool_count")
	}
	limit := len(oldTools)
	if len(currentTools) < limit {
		limit = len(currentTools)
	}
	for i := 0; i < limit; i++ {
		oldName, _ := oldTools[i]["name"].(string)
		currentName, _ := currentTools[i]["name"].(string)
		if oldName != currentName {
			diff.UnexplainedChanges = append(diff.UnexplainedChanges, fmt.Sprintf("tools[%d].name", i+1))
			continue
		}
		oldDescription, _ := oldTools[i]["description"].(string)
		currentDescription, _ := currentTools[i]["description"].(string)
		oldSchema, err := canonicalJSONValue(oldTools[i]["inputSchema"])
		if err != nil {
			return diff, err
		}
		currentSchema, err := canonicalJSONValue(currentTools[i]["inputSchema"])
		if err != nil {
			return diff, err
		}
		if string(oldSchema) == string(currentSchema) {
			if oldDescription != currentDescription {
				diff.UnexplainedChanges = append(diff.UnexplainedChanges, oldName+".description")
			}
			continue
		}
		if oldName == "polis_work_checkpoint" {
			change, approved := approvedCheckpointEvidenceRefsChange(oldTools[i]["inputSchema"], currentTools[i]["inputSchema"])
			approved = approved && oldDescription == currentDescription
			if approved {
				diff.ApprovedChanges = append(diff.ApprovedChanges, change)
				continue
			}
		}
		if oldName == "polis_collab_send" {
			change, approved := approvedCollabSendTargetChange(oldTools[i]["inputSchema"], currentTools[i]["inputSchema"], currentDescription)
			if approved {
				diff.ApprovedChanges = append(diff.ApprovedChanges, change)
				continue
			}
		}
		diff.UnexplainedChanges = append(diff.UnexplainedChanges, oldName+".inputSchema")
	}
	if len(oldTools) != len(currentTools) {
		for i := limit; i < len(oldTools); i++ {
			diff.UnexplainedChanges = append(diff.UnexplainedChanges, fmt.Sprintf("old_tools[%d]", i+1))
		}
		for i := limit; i < len(currentTools); i++ {
			diff.UnexplainedChanges = append(diff.UnexplainedChanges, fmt.Sprintf("current_tools[%d]", i+1))
		}
	}
	diff.Explained = len(diff.UnexplainedChanges) == 0
	return diff, nil
}

func approvedCollabSendTargetChange(oldRaw, currentRaw any, currentDescription string) (BackendL2SchemaChange, bool) {
	oldSchema, oldOK := oldRaw.(map[string]any)
	currentSchema, currentOK := currentRaw.(map[string]any)
	if !oldOK || !currentOK || currentDescription != "Send a direct actionable collaboration message to the peer employee's current peer task. Use the explicit employee ID and task ID returned by work_current; to_task is not accepted." {
		return BackendL2SchemaChange{}, false
	}
	oldProperties, oldOK := oldSchema["properties"].(map[string]any)
	currentProperties, currentOK := currentSchema["properties"].(map[string]any)
	if !oldOK || !currentOK || oldProperties["to_task"] == nil || currentProperties["to_employee_id"] == nil || currentProperties["to_task_id"] == nil || currentProperties["to_task"] != nil {
		return BackendL2SchemaChange{}, false
	}
	for _, name := range []string{"contract_revision_id", "body", "actionable"} {
		if !reflect.DeepEqual(oldProperties[name], currentProperties[name]) {
			return BackendL2SchemaChange{}, false
		}
	}
	oldRequired, oldOK := stringSliceValue(oldSchema["required"])
	currentRequired, currentOK := stringSliceValue(currentSchema["required"])
	if !oldOK || !currentOK || !containsExact(oldRequired, "to_task") || !containsExact(currentRequired, "to_employee_id") || !containsExact(currentRequired, "to_task_id") || containsExact(currentRequired, "to_task") {
		return BackendL2SchemaChange{}, false
	}
	return BackendL2SchemaChange{
		Tool:        "polis_collab_send",
		Path:        "polis_collab_send.inputSchema.properties.to_task -> to_employee_id + to_task_id",
		OldRequired: "to_task",
		NewRequired: "to_employee_id,to_task_id",
		OldBytes:    len(mustJSONBytes(oldRaw)),
		NewBytes:    len(mustJSONBytes(currentRaw)),
		Reason:      "approved collaboration target hardening: employee and responsibility task IDs are explicit",
	}, true
}

func mustJSONBytes(value any) []byte {
	raw, _ := json.Marshal(value)
	return raw
}

func approvedCheckpointEvidenceRefsChange(oldRaw, currentRaw any) (BackendL2SchemaChange, bool) {
	oldSchema, oldOK := oldRaw.(map[string]any)
	currentSchema, currentOK := currentRaw.(map[string]any)
	if !oldOK || !currentOK {
		return BackendL2SchemaChange{}, false
	}
	oldProperties, oldOK := oldSchema["properties"].(map[string]any)
	currentProperties, currentOK := currentSchema["properties"].(map[string]any)
	if !oldOK || !currentOK {
		return BackendL2SchemaChange{}, false
	}
	oldEvidence, oldPresent := oldProperties["evidence"]
	currentEvidenceRefs, currentPresent := currentProperties["evidence_refs"]
	if !oldPresent || currentPresent == false || oldEvidence == nil || currentEvidenceRefs == nil {
		return BackendL2SchemaChange{}, false
	}
	oldCopy := cloneStringMap(oldSchema)
	currentCopy := cloneStringMap(currentSchema)
	oldCopyProperties, oldOK := oldCopy["properties"].(map[string]any)
	currentCopyProperties, currentOK := currentCopy["properties"].(map[string]any)
	if !oldOK || !currentOK {
		return BackendL2SchemaChange{}, false
	}
	delete(oldCopyProperties, "evidence")
	delete(currentCopyProperties, "evidence_refs")
	oldCopy["properties"], currentCopy["properties"] = oldCopyProperties, currentCopyProperties
	oldRequired, oldOK := stringSliceValue(oldCopy["required"])
	currentRequired, currentOK := stringSliceValue(currentCopy["required"])
	if !oldOK || !currentOK || !containsExact(oldRequired, "evidence") || !containsExact(currentRequired, "evidence_refs") {
		return BackendL2SchemaChange{}, false
	}
	oldCopy["required"], currentCopy["required"] = removeString(oldRequired, "evidence"), removeString(currentRequired, "evidence_refs")
	if !reflect.DeepEqual(oldCopy, currentCopy) {
		return BackendL2SchemaChange{}, false
	}
	oldBytes, _ := json.Marshal(oldRaw)
	currentBytes, _ := json.Marshal(currentRaw)
	return BackendL2SchemaChange{Tool: "polis_work_checkpoint", Path: "polis_work_checkpoint.inputSchema.properties.evidence -> evidence_refs", OldRequired: "evidence", NewRequired: "evidence_refs", OldBytes: len(oldBytes), NewBytes: len(currentBytes), Reason: "approved checkpoint/behavioral hardening: receipt references replace free-text evidence"}, true
}

func RecordR03ABackendL2Requalification(cfg BackendL2RequalificationConfig) (map[string]any, error) {
	if cfg.Evidence == "" || cfg.OldEvidence == "" || cfg.L1Evidence == "" || cfg.AcceptanceQualification == "" || cfg.Binary == "" || cfg.CodeModeHost == "" || cfg.AuthFile == "" || cfg.SelectedConfig == "" || cfg.RuntimeManifest == "" {
		return nil, errors.New("Backend L2 requalification paths are incomplete")
	}
	if entries, err := os.ReadDir(cfg.Evidence); err == nil && len(entries) != 0 {
		return nil, errors.New("Backend L2 requalification evidence path is not fresh")
	} else if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	acceptance, err := loadR03APaginationV3Acceptance(cfg.AcceptanceQualification)
	if err != nil {
		return nil, err
	}
	var l1 currentBinaryL1Evidence
	if err := readT11JSON(filepath.Join(cfg.L1Evidence, "result.json"), &l1); err != nil {
		return nil, err
	}
	if l1.Status != "passed" || l1.CurrentBinaryWindowsL1 != "QUALIFIED" {
		return nil, errors.New("current Windows L1 is not qualified")
	}
	var oldManifest currentBinaryL2ExecutionManifest
	if err := readT11JSON(filepath.Join(cfg.OldEvidence, "execution-manifest.json"), &oldManifest); err != nil {
		return nil, err
	}
	oldRegistry, err := os.ReadFile(filepath.Join(cfg.OldEvidence, "tool-registry.json"))
	if err != nil {
		return nil, err
	}
	surface, tools, currentRegistry, err := buildT21ToolSurface()
	if err != nil {
		return nil, err
	}
	payload, err := json.Marshal(t21NormalizedThreadPayload(tools, r03aT21DeveloperInstruction))
	if err != nil {
		return nil, err
	}
	surface.ThreadStartPayloadDigest = digest(payload)
	surface.ThreadStartPayloadBytes = len(payload)
	diff, err := compareBackendL2ToolRegistry(oldRegistry, currentRegistry)
	if err != nil {
		return nil, err
	}
	if !diff.Explained {
		return nil, fmt.Errorf("Backend L2 offline qualification denied: unexplained provider-visible drift: %s", strings.Join(diff.UnexplainedChanges, ","))
	}
	base := oldManifest.CanonicalManifest.Base
	base.Combination.CodexVersion = strings.TrimPrefix(l1.CodexVersion, "codex-cli ")
	base.Combination.BinarySHA256 = l1.CodexBinarySHA256
	base.Combination.CodeModeHostSHA256 = l1.CodeModeHostSHA256
	base.Combination.Model = l1.Model
	base.Combination.Effort = l1.Effort
	base.Combination.CapabilityDigest = currentBinaryL2CapabilityDigest(l1.ExecutionFingerprint, surface)
	manifest := codex.CanonicalManifestV7{FingerprintSchemaVersion: codex.CanonicalManifestFingerprintSchemaVersionV7, Base: base, ToolSurface: surface}
	if err := manifest.Validate(); err != nil {
		return nil, fmt.Errorf("new Backend L2 canonical manifest invalid: %w", err)
	}
	canonicalDigest := manifest.Base.Combination.CurrentFingerprintV7(manifest).CanonicalManifestDigest
	binding := currentBinaryL2Binding{SchemaVersion: r03aCurrentBinaryL2BindingSchema, L1Fingerprint: l1.ExecutionFingerprint, ControlledRuntimeManifestDigest: l1.ControlledRuntimeManifestDigest, CodexVersion: l1.CodexVersion, CodexBinarySHA256: l1.CodexBinarySHA256, CodeModeHostSHA256: l1.CodeModeHostSHA256, ToolManifestDigest: surface.AggregateManifestDigest, AggregateSchemaDigest: surface.AggregateSchemaDigest, AggregateSchemaBytes: surface.AggregateSchemaBytes, ToolCount: surface.ToolCount, CheckpointPolicyRevision: surface.CheckpointPolicyRevision, ArtifactEligibilityPolicyRevision: surface.ArtifactEligibilityPolicyRevision, ContractSupersessionPolicyRevision: surface.ContractSupersessionPolicyRevision, AcceptanceCheckerRevision: surface.AcceptanceCheckerRevision}
	executionFingerprint, err := binding.Fingerprint()
	if err != nil {
		return nil, err
	}
	policy := map[string]string{"acceptance_checker_revision": surface.AcceptanceCheckerRevision, "artifact_eligibility_policy_revision": surface.ArtifactEligibilityPolicyRevision, "checkpoint_policy_revision": surface.CheckpointPolicyRevision, "contract_supersession_policy_revision": surface.ContractSupersessionPolicyRevision}
	result := map[string]any{"qualification": r03aCurrentBinaryL2Qualification, "status": "PASSED", "current_binary_l1": "QUALIFIED", "tool_count": surface.ToolCount, "tool_names": t21ToolNames(surface), "tool_manifest_digest": surface.AggregateManifestDigest, "aggregate_schema_digest": surface.AggregateSchemaDigest, "aggregate_schema_bytes": surface.AggregateSchemaBytes, "thread_start_payload_digest": surface.ThreadStartPayloadDigest, "thread_start_payload_bytes": surface.ThreadStartPayloadBytes, "handler_binding_digest": digest(currentRegistry), "policy_revisions": policy, "canonical_manifest_digest": canonicalDigest, "execution_fingerprint": executionFingerprint, "l1_fingerprint": l1.ExecutionFingerprint, "controlled_runtime_manifest_digest": l1.ControlledRuntimeManifestDigest, "codex_version": l1.CodexVersion, "codex_binary_sha256": l1.CodexBinarySHA256, "code_mode_host_sha256": l1.CodeModeHostSHA256, "model": l1.Model, "effort": l1.Effort, "dynamic_tool_count": surface.ToolCount, "business_write_policy": surface.BusinessWritePolicy, "semantic_diff": diff, "unrelated_drift": false, "business_acceptance": map[string]any{"qualification": acceptance, "public_contract_revision": fixture.PeerPaginationContractRevision, "implementation_binding_contract": fixture.BackendBindingContractRevisionV2, "behavior_verifier_revision": fixture.PeerPaginationBehaviorVerifierRevisionV2, "factor_boundary": "business_acceptance_separate_from_provider_tool_surface"}, "medium": 0, "high": 0, "provider_egress": 0, "historical_evidence_modified": false, "generated_at": time.Now().UTC()}
	if err := os.MkdirAll(cfg.Evidence, 0700); err != nil {
		return nil, err
	}
	if err := os.WriteFile(filepath.Join(cfg.Evidence, "tool-registry.json"), append(currentRegistry, '\n'), 0600); err != nil {
		return nil, err
	}
	if err := writeJSON(filepath.Join(cfg.Evidence, "tool-surface.json"), map[string]any{"schema_version": "canonical-tool-manifest-v1", "source": "codex.PeerBackendTools", "surface": surface, "tool_names": t21ToolNames(surface), "tools": json.RawMessage(currentRegistry)}); err != nil {
		return nil, err
	}
	if err := writeJSON(filepath.Join(cfg.Evidence, "execution-manifest.json"), map[string]any{"schema_version": r03aCurrentBinaryL2ManifestSchema, "qualification": r03aCurrentBinaryL2Qualification, "fingerprint_schema_version": codex.CanonicalManifestFingerprintSchemaVersionV7, "canonical_manifest": manifest, "canonical_manifest_digest": canonicalDigest, "execution_fingerprint": executionFingerprint, "l1_fingerprint": l1.ExecutionFingerprint, "controlled_runtime_manifest_digest": l1.ControlledRuntimeManifestDigest, "tool_surface": surface, "tool_manifest_digest": surface.AggregateManifestDigest, "tool_names": t21ToolNames(surface), "policy_revisions": policy, "binding": binding, "thread_start_payload_canonical_json": string(payload), "thread_start_payload_digest": surface.ThreadStartPayloadDigest, "thread_start_payload_bytes": surface.ThreadStartPayloadBytes, "historical_previous_l2_modified": false}); err != nil {
		return nil, err
	}
	if err := writeJSON(filepath.Join(cfg.Evidence, "semantic-diff.json"), diff); err != nil {
		return nil, err
	}
	if err := writeJSON(filepath.Join(cfg.Evidence, "preflight.json"), map[string]any{"qualification": r03aCurrentBinaryL2Qualification, "status": "offline_preflight_passed", "passed": true, "current_binary_l1": "QUALIFIED", "l1_fingerprint": l1.ExecutionFingerprint, "tool_count": surface.ToolCount, "tool_manifest_digest": surface.AggregateManifestDigest, "aggregate_schema_digest": surface.AggregateSchemaDigest, "aggregate_schema_bytes": surface.AggregateSchemaBytes, "policy_revisions": policy, "business_acceptance_contract": fixture.PeerPaginationContractRevision, "business_acceptance_binding": fixture.BackendBindingContractRevisionV2, "business_acceptance_verifier": fixture.PeerPaginationBehaviorVerifierRevisionV2, "provider_visible_drift_explained": true, "unexplained_drift": false, "medium": 0, "high": 0, "provider_egress": 0, "historical_evidence_modified": false}); err != nil {
		return nil, err
	}
	if err := writeJSON(filepath.Join(cfg.Evidence, "offline-result.json"), result); err != nil {
		return nil, err
	}
	return result, nil
}

func VerifyR03ABackendL2Requalification(evidence string) error {
	var result struct {
		Status                string               `json:"status"`
		Qualification         string               `json:"qualification"`
		ExecutionFingerprint  string               `json:"execution_fingerprint"`
		ToolManifestDigest    string               `json:"tool_manifest_digest"`
		AggregateSchemaDigest string               `json:"aggregate_schema_digest"`
		AggregateSchemaBytes  int                  `json:"aggregate_schema_bytes"`
		ToolCount             int                  `json:"tool_count"`
		SemanticDiff          BackendL2SurfaceDiff `json:"semantic_diff"`
	}
	if err := readT11JSON(filepath.Join(evidence, "offline-result.json"), &result); err != nil {
		return err
	}
	if result.Status != "PASSED" || result.Qualification != r03aCurrentBinaryL2Qualification || result.ExecutionFingerprint == "" || result.ToolCount != 11 || result.ToolManifestDigest == "" || result.AggregateSchemaDigest == "" || result.AggregateSchemaBytes <= 0 || !result.SemanticDiff.Explained || len(result.SemanticDiff.UnexplainedChanges) != 0 {
		return errors.New("Backend L2 requalification result is incomplete")
	}
	return nil
}

func canonicalJSONValue(value any) ([]byte, error) {
	return json.Marshal(value)
}

func cloneStringMap(value map[string]any) map[string]any {
	raw, _ := json.Marshal(value)
	var clone map[string]any
	_ = json.Unmarshal(raw, &clone)
	return clone
}

func stringSliceValue(value any) ([]string, bool) {
	list, ok := value.([]any)
	if !ok {
		return nil, false
	}
	result := make([]string, 0, len(list))
	for _, item := range list {
		text, ok := item.(string)
		if !ok {
			return nil, false
		}
		result = append(result, text)
	}
	return result, true
}

func containsExact(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}

func removeString(values []string, unwanted string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		if value != unwanted {
			result = append(result, value)
		}
	}
	return result
}

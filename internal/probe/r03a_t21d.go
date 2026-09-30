// pattern: Imperative Shell
package probe

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"polis/internal/codex"
	"polis/internal/core"
	"time"
)

type R03AT21DOfflineResult struct {
	Qualification         string                           `json:"qualification"`
	Status                string                           `json:"status"`
	ToolCount             int                              `json:"tool_count"`
	ToolManifestDigest    string                           `json:"tool_manifest_digest"`
	AggregateSchemaDigest string                           `json:"aggregate_schema_digest"`
	AggregateSchemaBytes  int                              `json:"aggregate_schema_bytes"`
	BindingDigest         string                           `json:"handler_binding_digest"`
	PolicyRevisions       map[string]string                `json:"policy_revisions"`
	SemanticDiff          []map[string]any                 `json:"semantic_diff"`
	UnrelatedDrift        bool                             `json:"unrelated_drift"`
	ExecutionFingerprint  codex.QualificationFingerprintV7 `json:"execution_fingerprint"`
	Medium                int                              `json:"medium"`
	High                  int                              `json:"high"`
	ProviderEgress        int                              `json:"provider_egress"`
	GeneratedAt           time.Time                        `json:"generated_at"`
}

func RecordR03AT21DOfflineQualification(evidence, parentEvidence, t21cEvidence string) (R03AT21DOfflineResult, error) {
	var out R03AT21DOfflineResult
	if evidence == "" || parentEvidence == "" || t21cEvidence == "" {
		return out, errors.New("T21D offline qualification paths are required")
	}
	if _, err := os.Stat(filepath.Join(evidence, "offline-result.json")); err == nil {
		return out, errors.New("T21D offline evidence already exists; refusing overwrite")
	}
	var parent t21ExecutionConfig
	if err := readT11JSON(filepath.Join(parentEvidence, "execution-config.json"), &parent); err != nil {
		return out, fmt.Errorf("read immutable T21B execution config: %w", err)
	}
	var old struct {
		Surface codex.ToolSurfaceManifest `json:"surface"`
	}
	if err := readT11JSON(filepath.Join(t21cEvidence, "revised-manifest.json"), &old); err != nil {
		return out, fmt.Errorf("read immutable T21C revised manifest: %w", err)
	}
	surface, tools, toolRaw, err := buildT21ToolSurface()
	if err != nil {
		return out, err
	}
	payload, err := json.Marshal(t21NormalizedThreadPayload(tools, r03aT21DeveloperInstruction))
	if err != nil {
		return out, err
	}
	surface.ThreadStartPayloadDigest = digest(payload)
	surface.ThreadStartPayloadBytes = len(payload)
	if surface.ToolCount != 11 || surface.Validate() != nil {
		return out, errors.New("current formal PeerBackendTools is not valid exact-11")
	}
	if old.Surface.ToolCount != surface.ToolCount || len(old.Surface.Tools) != len(surface.Tools) {
		return out, errors.New("T21C to T21D diff contains unapproved tool surface drift")
	}
	for i := range surface.Tools {
		if !approvedT21DToolEntry(old.Surface.Tools[i], surface.Tools[i]) {
			return out, fmt.Errorf("T21C to T21D diff contains unapproved per-tool drift at ordinal %d", i+1)
		}
	}
	diff := []map[string]any{}
	for _, field := range []struct {
		name    string
		old     string
		current string
	}{
		{"checkpoint_policy_revision", old.Surface.CheckpointPolicyRevision, surface.CheckpointPolicyRevision},
		{"artifact_eligibility_policy_revision", old.Surface.ArtifactEligibilityPolicyRevision, surface.ArtifactEligibilityPolicyRevision},
		{"contract_supersession_policy_revision", old.Surface.ContractSupersessionPolicyRevision, surface.ContractSupersessionPolicyRevision},
		{"acceptance_checker_revision", old.Surface.AcceptanceCheckerRevision, surface.AcceptanceCheckerRevision},
	} {
		if field.old != field.current {
			diff = append(diff, map[string]any{"field": field.name, "old": field.old, "current": field.current, "approved_scope": true})
		}
	}
	for index, entry := range surface.Tools {
		if entry.Name != "polis_collab_apply" && entry.Name != "polis_work_checkpoint" && entry.Name != "polis_collab_send" {
			continue
		}
		if old.Surface.Tools[index].SchemaDigest != entry.SchemaDigest || old.Surface.Tools[index].SchemaBytes != entry.SchemaBytes || old.Surface.Tools[index].DescriptionDigest != entry.DescriptionDigest {
			diff = append(diff, map[string]any{"field": entry.Name + "_interaction_contract", "old_schema_digest": old.Surface.Tools[index].SchemaDigest, "current_schema_digest": entry.SchemaDigest, "old_schema_bytes": old.Surface.Tools[index].SchemaBytes, "current_schema_bytes": entry.SchemaBytes, "approved_scope": true})
		}
	}
	if len(diff) != 6 {
		return out, fmt.Errorf("T21D expected four policy and two current interaction-contract differences, got %d", len(diff))
	}
	revised := parent.CanonicalManifest
	revised.ToolSurface = surface
	revised.Base.Combination.CapabilityDigest = t21CapabilityDigest(parent.CanonicalManifest.Base.Combination.CapabilityDigest, surface)
	fingerprint := revised.Base.Combination.CurrentFingerprintV7(revised)
	bindingRaw, err := json.Marshal(surface.Tools)
	if err != nil {
		return out, err
	}
	out = R03AT21DOfflineResult{
		Qualification: "R0.3A-T21D", Status: "PASSED", ToolCount: surface.ToolCount,
		ToolManifestDigest: surface.AggregateManifestDigest, AggregateSchemaDigest: surface.AggregateSchemaDigest,
		AggregateSchemaBytes: surface.AggregateSchemaBytes, BindingDigest: digest(bindingRaw),
		PolicyRevisions: map[string]string{
			"checkpoint_policy_revision":            surface.CheckpointPolicyRevision,
			"artifact_eligibility_policy_revision":  surface.ArtifactEligibilityPolicyRevision,
			"contract_supersession_policy_revision": surface.ContractSupersessionPolicyRevision,
			"acceptance_checker_revision":           surface.AcceptanceCheckerRevision,
		},
		SemanticDiff: diff, UnrelatedDrift: false, ExecutionFingerprint: fingerprint,
		Medium: 0, High: 0, ProviderEgress: 0, GeneratedAt: time.Now().UTC(),
	}
	if err := os.MkdirAll(evidence, 0700); err != nil {
		return out, err
	}
	if err := os.WriteFile(filepath.Join(evidence, "tool-registry.json"), append(toolRaw, '\n'), 0600); err != nil {
		return out, err
	}
	if err := writeJSON(filepath.Join(evidence, "tool-surface.json"), map[string]any{"schema_version": "canonical-tool-manifest-v1", "source": "codex.PeerBackendTools", "surface": surface, "tool_names": t21ToolNames(surface), "tools": json.RawMessage(toolRaw)}); err != nil {
		return out, err
	}
	revisedConfig := parent
	revisedConfig.CanonicalManifest, revisedConfig.QualificationFingerprint, revisedConfig.ToolSurface = revised, fingerprint, surface
	revisedConfig.ToolNames, revisedConfig.ToolManifestDigest = t21ToolNames(surface), surface.AggregateManifestDigest
	revisedConfig.AggregateSchemaDigest, revisedConfig.AggregateSchemaBytes = surface.AggregateSchemaDigest, surface.AggregateSchemaBytes
	revisedConfig.ThreadStartPayloadCanonicalJSON, revisedConfig.ThreadStartPayloadDigest, revisedConfig.ThreadStartPayloadBytes = string(payload), surface.ThreadStartPayloadDigest, surface.ThreadStartPayloadBytes
	revisedConfig.ThreadStartDeveloperInstruction, revisedConfig.PromptSemantics = r03aT21DeveloperInstruction, r03aT21PromptSemantics
	if err := writeJSON(filepath.Join(evidence, "execution-config.json"), revisedConfig); err != nil {
		return out, err
	}
	if err := writeJSON(filepath.Join(evidence, "execution-manifest.json"), map[string]any{"qualification": "R0.3A-T21D", "fingerprint_schema_version": codex.CanonicalManifestFingerprintSchemaVersionV7, "canonical_manifest": revised, "canonical_manifest_digest": fingerprint.CanonicalManifestDigest, "qualification_fingerprint": fingerprint, "tool_surface": surface, "tool_manifest_digest": surface.AggregateManifestDigest, "policy_revisions": out.PolicyRevisions, "historical_parent": "R0.3A-T21B"}); err != nil {
		return out, err
	}
	if err := writeJSON(filepath.Join(evidence, "preflight.json"), map[string]any{"qualification": "R0.3A-T21D", "status": "offline_preflight_passed", "passed": true, "t21_fingerprint": fingerprint.CanonicalManifestDigest, "tool_manifest_digest": surface.AggregateManifestDigest, "aggregate_schema_digest": surface.AggregateSchemaDigest, "aggregate_schema_bytes": surface.AggregateSchemaBytes, "tool_count": surface.ToolCount, "policy_revisions": out.PolicyRevisions, "medium": 0, "high": 0, "provider_egress": 0}); err != nil {
		return out, err
	}
	if err := writeJSON(filepath.Join(evidence, "semantic-diff.json"), diff); err != nil {
		return out, err
	}
	if err := writeJSON(filepath.Join(evidence, "offline-result.json"), out); err != nil {
		return out, err
	}
	return out, nil
}

func VerifyR03AT21DOfflineQualification(evidence string) error {
	var record R03AT21DOfflineResult
	if err := readT11JSON(filepath.Join(evidence, "offline-result.json"), &record); err != nil {
		return err
	}
	if record.Qualification != "R0.3A-T21D" || record.Status != "PASSED" || record.ToolCount != 11 || record.UnrelatedDrift || record.Medium != 0 || record.High != 0 || record.ProviderEgress != 0 || len(record.SemanticDiff) != 6 {
		return errors.New("T21D offline qualification record is not passed")
	}
	current, _, _, err := buildT21ToolSurface()
	if err != nil {
		return err
	}
	if record.ToolManifestDigest != current.AggregateManifestDigest || record.AggregateSchemaDigest != current.AggregateSchemaDigest || record.AggregateSchemaBytes != current.AggregateSchemaBytes || current.CheckpointPolicyRevision != core.CheckpointPolicyRevision || current.ArtifactEligibilityPolicyRevision != core.ArtifactEligibilityPolicyRevision || current.ContractSupersessionPolicyRevision != core.PeerContractSupersessionPolicyRevision || current.AcceptanceCheckerRevision != core.PeerAcceptanceCheckerRevision {
		return errors.New("T21D offline manifest is stale")
	}
	return nil
}

func approvedT21DToolEntry(old, current codex.ToolSurfaceEntry) bool {
	if old.Name != current.Name || old.BindingIdentity != current.BindingIdentity || old.AuthorizationClass != current.AuthorizationClass || old.RegistrationOrdinal != current.RegistrationOrdinal {
		return false
	}
	if current.Name == "polis_collab_apply" || current.Name == "polis_work_checkpoint" || current.Name == "polis_collab_send" {
		return true
	}
	return old == current
}

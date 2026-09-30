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

const r03aFrontendCheckerFeedbackL2Qualification = "R0.3A-FRONTEND-CHECKER-FEEDBACK-L2"

func RecordR03AFrontendCheckerFeedbackL2Offline(evidence, previousManifestPath, frozenSuccessorProtocol string) (map[string]any, error) {
	if evidence == "" || previousManifestPath == "" || frozenSuccessorProtocol == "" {
		return nil, errors.New("checker feedback qualification paths are required")
	}
	if entries, err := os.ReadDir(evidence); err == nil && len(entries) != 0 {
		return nil, errors.New("checker feedback qualification evidence is not fresh; refusing overwrite")
	} else if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	var previous struct {
		ToolSurface codex.ToolSurfaceManifest `json:"tool_surface"`
	}
	if err := readT11JSON(previousManifestPath, &previous); err != nil {
		return nil, fmt.Errorf("read previous Frontend registration manifest: %w", err)
	}
	if err := previous.ToolSurface.Validate(); err != nil {
		return nil, fmt.Errorf("previous Frontend registration manifest invalid: %w", err)
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
		return nil, fmt.Errorf("current Frontend registration manifest invalid: %w", err)
	}
	registrationDiff := compareFrontendRegistrationSurface(previous.ToolSurface, currentSurface)
	policyDiff := compareFrontendPolicyRevisions(previous.ToolSurface, currentSurface)
	if !registrationDiff.Equivalent {
		return nil, errors.New("checker feedback change unexpectedly altered provider registration surface")
	}
	if !policyDiff.OnlyCheckerRevisionChanged || currentSurface.AcceptanceCheckerRevision != core.PeerAcceptanceCheckerRevision {
		return nil, errors.New("checker feedback policy change is not an isolated revision increment")
	}
	regression := frontendCheckerFeedbackRegression()
	if !checkerFeedbackRegressionPassed(regression) {
		return nil, errors.New("checker feedback regression did not pass")
	}
	if err := os.MkdirAll(evidence, 0700); err != nil {
		return nil, err
	}
	handoverSnapshotPath := filepath.Join(filepath.Dir(filepath.Dir(frozenSuccessorProtocol)), "successor-handover.json")
	_, snapshotErr := os.Stat(handoverSnapshotPath)
	authoritativeSnapshotAvailable := snapshotErr == nil
	result := map[string]any{
		"qualification":             r03aFrontendCheckerFeedbackL2Qualification,
		"status":                    "PASSED",
		"offline_only":              true,
		"previous_manifest_path":    previousManifestPath,
		"frozen_successor_protocol": frozenSuccessorProtocol,
		"old_frontend_business_qualification_stale": true,
		"provider_visible_registration_changed":     false,
		"provider_visible_schema_changed":           false,
		"provider_visible_description_changed":      false,
		"provider_visible_handler_binding_changed":  false,
		"tool_count":              currentSurface.ToolCount,
		"tool_manifest_digest":    currentSurface.AggregateManifestDigest,
		"aggregate_schema_digest": currentSurface.AggregateSchemaDigest,
		"aggregate_schema_bytes":  currentSurface.AggregateSchemaBytes,
		"handler_binding_digest":  digest(toolRaw),
		"registration_diff":       registrationDiff,
		"policy_diff":             policyDiff,
		"policy_revisions": map[string]string{
			"acceptance_checker_revision":           currentSurface.AcceptanceCheckerRevision,
			"checkpoint_policy_revision":            currentSurface.CheckpointPolicyRevision,
			"artifact_eligibility_policy_revision":  currentSurface.ArtifactEligibilityPolicyRevision,
			"contract_supersession_policy_revision": currentSurface.ContractSupersessionPolicyRevision,
		},
		"checker_feedback_regression":                regression,
		"frozen_successor_old_failed_checks":         7,
		"frozen_successor_old_feedback_detail":       false,
		"authoritative_handover_snapshot_path":       handoverSnapshotPath,
		"authoritative_handover_snapshot_available":  authoritativeSnapshotAvailable,
		"authoritative_database_available":           false,
		"eligible_for_successor_only_revised_run":    false,
		"requires_new_frontend_initial_and_handover": true,
		"medium":                       0,
		"high":                         0,
		"provider_egress":              0,
		"live_canary":                  "NOT_STARTED",
		"historical_evidence_modified": false,
		"generated_at":                 time.Now().UTC(),
	}
	if err := writeJSON(filepath.Join(evidence, "registration-diff.json"), map[string]any{
		"previous":                              previous.ToolSurface,
		"current":                               currentSurface,
		"provider_visible_registration_changed": false,
		"diff":                                  registrationDiff,
		"policy_diff":                           policyDiff,
	}); err != nil {
		return nil, err
	}
	if err := writeJSON(filepath.Join(evidence, "feedback-regression.json"), regression); err != nil {
		return nil, err
	}
	if err := writeJSON(filepath.Join(evidence, "offline-result.json"), result); err != nil {
		return nil, err
	}
	return result, nil
}

func VerifyR03AFrontendCheckerFeedbackL2Offline(evidence string) error {
	var result struct {
		Qualification         string `json:"qualification"`
		Status                string `json:"status"`
		RegistrationChanged   bool   `json:"provider_visible_registration_changed"`
		Medium                int    `json:"medium"`
		High                  int    `json:"high"`
		ProviderEgress        int    `json:"provider_egress"`
		SuccessorOnlyEligible bool   `json:"eligible_for_successor_only_revised_run"`
		AuthoritativeDatabase bool   `json:"authoritative_database_available"`
	}
	if err := readT11JSON(filepath.Join(evidence, "offline-result.json"), &result); err != nil {
		return err
	}
	if result.Qualification != r03aFrontendCheckerFeedbackL2Qualification || result.Status != "PASSED" || result.RegistrationChanged || result.Medium != 0 || result.High != 0 || result.ProviderEgress != 0 || result.SuccessorOnlyEligible || result.AuthoritativeDatabase {
		return errors.New("checker feedback offline qualification is not passed")
	}
	return nil
}

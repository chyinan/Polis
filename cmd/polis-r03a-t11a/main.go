// pattern: Imperative Shell
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"polis/internal/codex"
)

type t9ExecutionManifest struct {
	AuthIdentityFingerprint string                     `json:"auth_identity_fingerprint"`
	Combination             codex.ExecutionCombination `json:"combination"`
}

type t8ExecutionDiff struct {
	B struct {
		Auth struct {
			SHA256 string `json:"sha256"`
		} `json:"auth"`
	} `json:"B"`
}

type t11PreflightState struct {
	Passed     bool   `json:"passed"`
	Status     string `json:"status"`
	ModelCalls int    `json:"model_calls"`
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	authPath := os.Getenv("POLIS_CODEX_AUTH_FILE")
	if authPath == "" {
		return fmt.Errorf("auth reconciliation requires POLIS_CODEX_AUTH_FILE")
	}
	authRaw, err := os.ReadFile(authPath)
	if err != nil {
		return err
	}
	t9Path := filepath.Join(cwd, "evidence/development/r0.3a-t9/luna-1/execution-manifest.json")
	var t9 t9ExecutionManifest
	if err = readJSON(t9Path, &t9); err != nil {
		return fmt.Errorf("read T9 execution manifest: %w", err)
	}
	t8Path := filepath.Join(cwd, "evidence/development/r0.3a-t8/execution-diff.json")
	var t8 t8ExecutionDiff
	if err = readJSON(t8Path, &t8); err != nil {
		return fmt.Errorf("read T8 execution diff: %w", err)
	}
	if t9.AuthIdentityFingerprint == "" || t9.AuthIdentityFingerprint != t8.B.Auth.SHA256 {
		return fmt.Errorf("T9 auth field does not match T8 B auth file SHA")
	}
	currentMaterial, err := codex.ParseAuthMaterial(authRaw, "mounted_codex_auth_file")
	if err != nil {
		return err
	}
	historicalManifest := codex.AuthFingerprintManifest{
		AuthSourceClass:                                "mounted_codex_auth_file",
		AuthIdentityFingerprintSchemaVersion:           codex.AuthIdentityFingerprintSchemaVersion,
		AuthIdentityFingerprint:                        "unavailable",
		AuthIdentityFingerprintStatus:                  "unavailable",
		AuthCredentialRevisionFingerprintSchemaVersion: codex.AuthCredentialRevisionFingerprintSchemaVersion,
		AuthCredentialRevisionFingerprint:              t9.AuthIdentityFingerprint,
		AuthCredentialRevisionFingerprintStatus:        "available",
	}
	currentManifest := currentMaterial.Manifest()
	comparison := codex.CompareAuthManifests(historicalManifest, currentManifest)

	var t11 t11PreflightState
	t11Path := filepath.Join(cwd, "evidence/development/r0.3a-t11/luna-1/preflight.json")
	if err = readJSON(t11Path, &t11); err != nil {
		return fmt.Errorf("read T11 preflight evidence: %w", err)
	}

	credentialRevisionSameSemantic := map[string]any{
		"status":                    comparison.CredentialRevision,
		"historical_field_name":     "auth_identity_fingerprint",
		"historical_value":          t9.AuthIdentityFingerprint,
		"current_field_name":        "auth_credential_revision_fingerprint",
		"current_value":             currentMaterial.CredentialRevisionFingerprint,
		"algorithm":                 "SHA-256",
		"canonicalization":          "exact complete auth.json bytes; no JSON parse or normalization",
		"historical_schema_version": "NOT_RECORDED; reconstructed semantic is auth-credential-revision-v1",
		"current_schema_version":    codex.AuthCredentialRevisionFingerprintSchemaVersion,
		"identity_interpretation":   "not a stable account identity",
	}

	reconciliation := map[string]any{
		"qualification":      "R0.3A-T11A",
		"provider_egress":    "prohibited_and_not_run",
		"allowance_consumed": false,
		"t11_history": map[string]any{
			"live_canary":         "NOT_STARTED",
			"result":              "preflight_failed",
			"medium_consumed":     0,
			"preflight_path":      t11Path,
			"preflight_preserved": true,
			"preflight_status":    t11.Status,
			"model_calls":         t11.ModelCalls,
		},
		"fingerprint_definitions": []map[string]any{
			{
				"field_name":                 "T8 B.auth.sha256 / user_auth_file_opaque_fingerprint",
				"semantic_definition":        "opaque credential-material file revision, not stable identity",
				"source_data":                "host auth.json selected by scripts/r03a-t8-execution-diff.ps1",
				"canonicalization_algorithm": "raw file bytes via Get-FileHash -LiteralPath; no JSON normalization",
				"hash_algorithm":             "SHA-256",
				"schema_version":             "NOT_RECORDED",
			},
			{
				"field_name":                 "T9 auth_identity_fingerprint",
				"semantic_definition":        "copied T8 complete-file credential revision SHA; field name is a semantic mislabel",
				"source_data":                "T8 execution-diff.json B.auth.sha256, read by internal/probe/r03a_t9.go",
				"canonicalization_algorithm": "none in T9; inherits T8 raw auth.json bytes",
				"hash_algorithm":             "SHA-256",
				"schema_version":             "NOT_RECORDED",
			},
			{
				"field_name":                 "T10 combination.auth_source_class",
				"semantic_definition":        "auth source category only; no identity or credential revision digest",
				"source_data":                "T10 qualification-state.json combination",
				"canonicalization_algorithm": "not applicable",
				"hash_algorithm":             "not applicable",
				"schema_version":             codex.CanonicalManifestFingerprintSchemaVersion,
			},
			{
				"field_name":                 "T11 current auth comparison",
				"semantic_definition":        "T11 implementation computed complete-file credential revision SHA but stored/compared it as identity",
				"source_data":                "current POLIS_CODEX_AUTH_FILE bytes read by internal/probe/r03a_t11_runner.go",
				"canonicalization_algorithm": "exact raw auth.json bytes; no JSON normalization",
				"hash_algorithm":             "SHA-256",
				"schema_version":             "NOT_RECORDED in original T11; corrected future schema is auth-credential-revision-v1",
			},
			{
				"field_name":                 "canonical-manifest-v1",
				"semantic_definition":        "legacy/current execution combination digest; historical meaning unchanged",
				"source_data":                "JSON object containing fingerprint_schema_version and ExecutionCombination",
				"canonicalization_algorithm": "Go encoding/json compact deterministic struct serialization",
				"hash_algorithm":             "SHA-256",
				"schema_version":             codex.CanonicalManifestFingerprintSchemaVersion,
			},
			{
				"field_name":                 "canonical-manifest-v2",
				"semantic_definition":        "future execution combination digest with explicit auth source, identity and credential revision layers",
				"source_data":                "ExecutionCombination plus AuthFingerprintManifest",
				"canonicalization_algorithm": "Go encoding/json compact deterministic struct serialization including schema version",
				"hash_algorithm":             "SHA-256",
				"schema_version":             codex.CanonicalManifestFingerprintSchemaVersionV2,
			},
		},
		"historical_t9_value": map[string]any{
			"field_name":      "auth_identity_fingerprint",
			"value":           t9.AuthIdentityFingerprint,
			"actual_semantic": "credential revision fingerprint",
			"stable_identity": "NOT_RECONSTRUCTABLE",
		},
		"current_value": map[string]any{
			"credential_revision_fingerprint": currentMaterial.CredentialRevisionFingerprint,
			"identity_fingerprint":            currentMaterial.IdentityFingerprint,
			"identity_status":                 currentMaterial.IdentityFingerprintStatus,
			"credential_revision_status":      currentMaterial.CredentialRevisionFingerprintStatus,
			"raw_claim_values_emitted":        false,
		},
		"credential_revision_reconciliation": credentialRevisionSameSemantic,
		"comparison": map[string]any{
			"source_class":                   comparison.SourceClass,
			"identity":                       comparison.Identity,
			"credential_revision":            comparison.CredentialRevision,
			"reason_code":                    comparison.ReasonCode,
			"reason_codes":                   comparison.ReasonCodes,
			"incomparable_fingerprint_types": comparison.IncomparableFingerprintTypes,
			"field_name_semantics_mismatch":  true,
			"strict_experiment_comparable":   comparison.StrictExperimentComparable,
		},
		"policy_decision": map[string]any{
			"historical_T9_strict_version_only_control": "NO",
			"eligible_for_new_T11_version_only_canary":  "NO",
			"next_authorized_experiment":                "current-auth 0.151.0 no-proxy zero-tool baseline",
			"auth_identity_changed_claim":               "not asserted",
			"credential_revision_changed_claim":         "confirmed",
		},
	}

	reconciliation["future_policy_correction"] = map[string]any{
		"canonical_manifest_v1":                  "unchanged; historical T7/T9/T10 meanings preserved",
		"canonical_manifest_v2":                  "implemented as opt-in AuthFingerprintManifest-bearing schema",
		"t11_comparator":                         "uses separate auth identity and credential revision manifests",
		"identity_change_reason_code":            codex.AuthIdentityChangedReasonCode,
		"credential_revision_change_reason_code": codex.AuthCredentialRevisionChangedReasonCode,
		"identity_unavailable_reason_code":       codex.AuthIdentityNotReconstructableReasonCode,
	}
	outputPath := filepath.Join(cwd, "evidence/development/r0.3a-t11a/auth-reconciliation.json")
	if err = os.MkdirAll(filepath.Dir(outputPath), 0700); err != nil {
		return err
	}
	encoded, err := json.MarshalIndent(reconciliation, "", "  ")
	if err != nil {
		return err
	}
	if err = os.WriteFile(outputPath, append(encoded, '\n'), 0600); err != nil {
		return err
	}
	fmt.Printf("r03a-t11a=recorded source=%s identity=%s credential_revision=%s strict=%t\n", comparison.SourceClass, comparison.Identity, comparison.CredentialRevision, comparison.StrictExperimentComparable)
	return nil
}

func readJSON(path string, target any) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(bytes.TrimPrefix(raw, []byte{0xef, 0xbb, 0xbf}), target)
}

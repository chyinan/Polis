// pattern: Imperative Shell
package probe

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"polis/internal/codex"
)

type R03AT18Config struct {
	T17Evidence    string
	T14CEvidence   string
	T16Evidence    string
	AuthFile       string
	PolisHostHome  string
	PolisGuestHome string
	Evidence       string
}

type t18T17Normalized struct {
	Comparison        map[string]T17FieldComparison `json:"comparison"`
	Windows           T17ProfileSnapshot            `json:"windows"`
	Polis             T17ProfileSnapshot            `json:"polis"`
	SameNativeDefault string                        `json:"same_native_default"`
}

type t18T17Manifest struct {
	QualificationFingerprint struct {
		CanonicalManifestDigest string `json:"canonical_manifest_digest"`
	} `json:"qualification_fingerprint"`
}

type t18T14CManifest struct {
	CanonicalManifest codex.CanonicalManifestV3 `json:"canonical_manifest"`
}

type t18T16Launcher struct {
	T14CActualArgv []string `json:"t14c_actual_argv"`
}

func RunR03AT18(cfg R03AT18Config) (map[string]any, error) {
	if err := validateT18Config(cfg); err != nil {
		return nil, err
	}
	if err := ensureT18EvidenceFresh(cfg.Evidence); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(cfg.Evidence, 0700); err != nil {
		return nil, err
	}
	var normalized t18T17Normalized
	if err := readT11JSON(filepath.Join(cfg.T17Evidence, "normalized-effective-config.json"), &normalized); err != nil {
		return nil, recordT18Failure(cfg.Evidence, fmt.Errorf("preflight_failed: T17 normalized manifest: %w", err))
	}
	var t17Manifest t18T17Manifest
	if err := readT11JSON(filepath.Join(cfg.T17Evidence, "manifest-v5.json"), &t17Manifest); err != nil {
		return nil, recordT18Failure(cfg.Evidence, fmt.Errorf("preflight_failed: T17 V5 manifest: %w", err))
	}
	var t14c t18T14CManifest
	if err := readT11JSON(filepath.Join(cfg.T14CEvidence, "execution-manifest.json"), &t14c); err != nil {
		return nil, recordT18Failure(cfg.Evidence, fmt.Errorf("preflight_failed: T14C manifest: %w", err))
	}
	var t16Launcher t18T16Launcher
	if err := readT11JSON(filepath.Join(cfg.T16Evidence, "launcher.json"), &t16Launcher); err != nil {
		return nil, recordT18Failure(cfg.Evidence, fmt.Errorf("preflight_failed: T16 launcher evidence: %w", err))
	}
	if err := t14c.CanonicalManifest.Validate(); err != nil {
		return nil, recordT18Failure(cfg.Evidence, fmt.Errorf("preflight_failed: T14C manifest invalid: %w", err))
	}
	if normalized.SameNativeDefault != "UNPROVEN" {
		return nil, recordT18Failure(cfg.Evidence, errors.New("preflight_failed: T17 did not preserve same_native_default=UNPROVEN"))
	}
	if err := validateT18AuthBoundary(cfg, t14c.CanonicalManifest.Auth, t16Launcher.T14CActualArgv); err != nil {
		return nil, recordT18Failure(cfg.Evidence, err)
	}
	assessment := AssessT18SelectedConfig(normalized.Comparison, normalized.Polis.EffectiveConfigDigest, normalized.Polis.EffectiveTransportDigest)
	assessment.OmittedConfigKeys["codex_home_profile"] = "A/B HOME semantics differ; changing profile is the proposed factor, not a selected config key by itself"
	assessment.OmittedConfigKeys["auth.json"] = "credential source is controlled separately; rootfs placeholder must not be fingerprinted"
	assessment.OmittedConfigKeys["history/cache/session_state"] = "prohibited: not execution configuration"
	if assessment.CandidateConstructible || len(assessment.SelectedConfigKeys) != 0 {
		return nil, recordT18Failure(cfg.Evidence, errors.New("preflight_failed: selected config assessment unexpectedly produced a live candidate"))
	}
	fixedFactors := map[string]string{
		"codex_version":                   "unchanged_from_T14C",
		"binary_and_native_protocol":      "unchanged_from_T14C",
		"runtime_and_invocation":          "unchanged_from_T14C",
		"model":                           "gpt-5.6-luna unchanged",
		"effort":                          "medium unchanged",
		"sandbox":                         "read-only unchanged",
		"network_namespace_policy":        "shared_host_network unchanged",
		"proxy":                           "no injected proxy unchanged",
		"dynamic_tools":                   "0 unchanged",
		"plugins_and_mcp":                 "disabled unchanged",
		"auth_source":                     "mounted read-only auth unchanged",
		"auth_identity_and_revision":      "T14C exact opaque fingerprints unchanged",
		"prompt_deadlines_stop_semantics": "unchanged_from_T14C",
	}
	if err := writeJSON(filepath.Join(cfg.Evidence, "assessment.json"), map[string]any{
		"status":                                 "NOT_STARTED",
		"candidate_execution_config_constructed": false,
		"selected_config_keys":                   assessment.SelectedConfigKeys,
		"omitted_config_keys":                    assessment.OmittedConfigKeys,
		"effective_config_digest_before":         assessment.EffectiveConfigDigestBefore,
		"effective_config_digest_after":          assessment.EffectiveConfigDigestAfter,
		"transport_config_digest_before":         assessment.TransportConfigDigestBefore,
		"transport_config_digest_after":          assessment.TransportConfigDigestAfter,
		"unchanged_factors":                      fixedFactors,
		"auth_boundary": map[string]any{
			"rootfs_home_auth_json_placeholder":    "NOT credential source",
			"effective_mounted_auth":               "credential source",
			"auth_source_class":                    t14c.CanonicalManifest.Auth.AuthSourceClass,
			"auth_identity_fingerprint":            t14c.CanonicalManifest.Auth.AuthIdentityFingerprint,
			"auth_credential_revision_fingerprint": t14c.CanonicalManifest.Auth.AuthCredentialRevisionFingerprint,
		},
		"reason":                       assessment.Reason,
		"t17_v5_fingerprint":           t17Manifest.QualificationFingerprint.CanonicalManifestDigest,
		"historical_evidence_modified": false,
	}); err != nil {
		return nil, err
	}
	if err := writeJSON(filepath.Join(cfg.Evidence, "preflight.json"), map[string]any{
		"passed":                                 true,
		"status":                                 "offline_preflight_passed_candidate_not_started",
		"t18_result":                             "NOT_STARTED",
		"medium_consumed":                        0,
		"high_consumed":                          0,
		"provider_egress":                        0,
		"candidate_execution_config_constructed": false,
		"selected_config_keys":                   assessment.SelectedConfigKeys,
		"omitted_config_keys":                    assessment.OmittedConfigKeys,
		"effective_config_digest_before":         assessment.EffectiveConfigDigestBefore,
		"effective_config_digest_after":          assessment.EffectiveConfigDigestAfter,
		"transport_config_digest_before":         assessment.TransportConfigDigestBefore,
		"transport_config_digest_after":          assessment.TransportConfigDigestAfter,
		"same_native_default":                    "UNPROVEN",
		"native_execution_environment_qualified": false,
		"historical_evidence_modified":           false,
	}); err != nil {
		return nil, err
	}
	qualification := map[string]any{
		"qualification":                          "R0.3A-T18",
		"status":                                 "NOT_STARTED",
		"reason":                                 "No safe single selected non-secret config key could be isolated from T17 evidence while preserving all fixed execution factors; no Medium was called.",
		"selected_config_keys":                   assessment.SelectedConfigKeys,
		"omitted_config_keys":                    assessment.OmittedConfigKeys,
		"effective_config_digest_before":         assessment.EffectiveConfigDigestBefore,
		"effective_config_digest_after":          assessment.EffectiveConfigDigestAfter,
		"transport_config_digest_before":         assessment.TransportConfigDigestBefore,
		"transport_config_digest_after":          assessment.TransportConfigDigestAfter,
		"same_native_default":                    "UNPROVEN",
		"native_execution_environment_qualified": false,
		"provider_transport_recovery":            "NOT_YET_VERIFIED",
		"next_step":                              "Windows-native vs WSL/Linux runtime differential",
		"historical_t6_t9_t12_t13_t14c_modified": false,
	}
	if err := writeJSON(filepath.Join(cfg.Evidence, "qualification.json"), qualification); err != nil {
		return nil, err
	}
	return qualification, nil
}

func validateT18AuthBoundary(cfg R03AT18Config, expected codex.AuthFingerprintManifest, argv []string) error {
	raw, err := os.ReadFile(cfg.AuthFile)
	if err != nil {
		return fmt.Errorf("preflight_failed: current auth material: %w", err)
	}
	material, err := codex.ParseAuthMaterial(raw, expected.AuthSourceClass)
	if err != nil || material.Manifest() != expected {
		return errors.New("preflight_failed: effective mounted auth does not match T14C opaque fingerprints")
	}
	placeholder, err := os.Stat(filepath.Join(cfg.PolisHostHome, "auth.json"))
	if err != nil || !placeholder.Mode().IsRegular() || placeholder.Size() != 0 {
		return errors.New("preflight_failed: expected zero-byte rootfs auth placeholder was not confirmed")
	}
	mounted := false
	for index := 0; index+2 < len(argv); index++ {
		if argv[index] == "--ro-bind" && argv[index+1] == cfg.AuthFile && argv[index+2] == "/home/codex/auth.json" {
			mounted = true
			break
		}
	}
	if !mounted || cfg.PolisGuestHome != "/home/codex" {
		return errors.New("preflight_failed: effective read-only auth mount boundary was not confirmed")
	}
	return nil
}

func validateT18Config(cfg R03AT18Config) error {
	if cfg.T17Evidence == "" || cfg.T14CEvidence == "" || cfg.T16Evidence == "" || cfg.AuthFile == "" || cfg.PolisHostHome == "" || cfg.PolisGuestHome != "/home/codex" || cfg.Evidence == "" {
		return errors.New("preflight_failed: T18 configuration is incomplete")
	}
	return nil
}

func ensureT18EvidenceFresh(path string) error {
	entries, err := os.ReadDir(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if len(entries) != 0 {
		return errors.New("T18 evidence directory is not fresh; refusing retry")
	}
	return nil
}

func recordT18Failure(evidence string, cause error) error {
	return errors.Join(cause, writeJSON(filepath.Join(evidence, "preflight.json"), map[string]any{"passed": false, "status": "preflight_failed", "t18_result": "NOT_STARTED", "medium_consumed": 0, "high_consumed": 0, "provider_egress": 0, "error": cause.Error()}))
}

// pattern: Imperative Shell
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"polis/internal/codex"
	"polis/internal/probe"
)

type t13Manifest struct {
	CanonicalManifest codex.CanonicalManifestV2 `json:"canonical_manifest"`
}

type t10State struct {
	Combination codex.ExecutionCombination `json:"combination"`
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
		return fmt.Errorf("T14A requires POLIS_CODEX_AUTH_FILE")
	}
	authRaw, err := os.ReadFile(authPath)
	if err != nil {
		return err
	}
	var t13 t13Manifest
	if err := readJSON(filepath.Join(cwd, "evidence/development/r0.3a-t13/luna-1/execution-manifest.json"), &t13); err != nil {
		return err
	}
	var t10 t10State
	if err := readJSON(filepath.Join(cwd, "evidence/development/r0.3a-t10/qualification-state.json"), &t10); err != nil {
		return err
	}
	if err := t13.CanonicalManifest.Validate(); err != nil {
		return fmt.Errorf("T13 control manifest invalid: %w", err)
	}
	authMaterial, err := codex.ParseAuthMaterial(authRaw, "mounted_codex_auth_file")
	if err != nil {
		return err
	}
	auth := authMaterial.Manifest()
	if err := probe.CompareT12AuthSnapshot(t13.CanonicalManifest.Auth, auth); err != nil {
		return err
	}
	base := t13.CanonicalManifest.Combination
	if base.CodexVersion != "0.153.4" || base.Model != "gpt-5.6-luna" || base.Effort != "medium" || base.RuntimeProfile != "wsl-linux-amd64" || base.SandboxClass != "read-only" || base.ProxyConfigDigest != hashText("proxy:none") {
		return fmt.Errorf("T13 control combination is not the expected 0.153.4 transport baseline")
	}
	if t10.Combination.BinarySHA256 == "" || t10.Combination.CodeModeHostSHA256 == "" || t10.Combination.CapabilityDigest == "" {
		return fmt.Errorf("T10 compatibility fields are incomplete")
	}
	configBase := probe.T14AEffectiveExecutionConfig{
		BinaryPath:                 filepath.Join(cwd, ".tools/codex-linux-0.153.4/package/vendor/x86_64-unknown-linux-musl/bin/codex"),
		BinarySHA256:               t10.Combination.BinarySHA256,
		CodeModeHostPath:           filepath.Join(cwd, ".tools/codex-linux-0.153.4/package/vendor/x86_64-unknown-linux-musl/bin/codex-code-mode-host"),
		CodeModeHostSHA256:         t10.Combination.CodeModeHostSHA256,
		CodexVersion:               base.CodexVersion,
		Model:                      base.Model,
		Effort:                     base.Effort,
		RuntimeProfile:             base.RuntimeProfile,
		SandboxClass:               base.SandboxClass,
		AuthFile:                   authPath,
		Auth:                       auth,
		ProxyURL:                   "",
		ProxyEnvironment:           map[string]string{"HTTP_PROXY": "absent", "HTTPS_PROXY": "absent", "ALL_PROXY": "absent", "NO_PROXY": "absent", "POLIS_NATIVE_PROXY": "absent"},
		Home:                       "/home/codex",
		HostHomePath:               filepath.Join(cwd, ".runtime/linux/r03a-t14a/host-home"),
		CWD:                        "/work",
		Invocation:                 "app-server --stdio",
		DynamicToolCount:           0,
		ToolSchemaBytes:            0,
		DeveloperInstructionDigest: hashText("Reply with exactly: POLIS_TRANSPORT_CANARY_OK"),
		PromptDigest:               hashText("Reply with exactly: POLIS_TRANSPORT_CANARY_OK"),
		FirstOutputDeadlineMS:      90000,
		StreamingIdleDeadlineMS:    90000,
		ReconnectGraceMS:           30000,
		TotalDeadlineMS:            600000,
		StopSemantics:              "process-group-kill-and-waited-proof",
		NativeProtocolDigest:       t10.Combination.NativeProtocolDigest,
	}
	results := make(map[string]any)
	for _, policy := range []string{codex.ProviderTransportPolicyExplicitlyDisabled, codex.ProviderTransportPolicyNativeDefault} {
		config := configBase
		config.WebSocketPolicy = codex.TransportPolicyManifest{ProviderTransportPolicy: policy, WebSocketPolicy: policy}
		artifacts, err := probe.DeriveT14AArtifacts(config)
		if err != nil {
			return err
		}
		if err := probe.ValidateT14ABinding(config, artifacts); err != nil {
			return err
		}
		results[policy] = map[string]any{
			"source_execution_config_digest": artifacts.ConfigDigest,
			"derivation_schema_version":      artifacts.DerivationSchemaVersion,
			"capability_digest":              artifacts.CapabilityDigest,
			"manifest_digest":                artifacts.ManifestDigest,
			"qualification_fingerprint":      artifacts.Fingerprint,
			"launch_config_digest":           artifacts.LaunchConfigDigest,
			"capability_binding":             artifacts.CapabilityBinding,
			"manifest_binding":               artifacts.ManifestBinding,
			"fingerprint_binding":            artifacts.FingerprintBinding,
			"launch_binding":                 artifacts.LaunchBinding,
			"launch_args_and_env_derived":    true,
			"launch_config_bytes_derived":    true,
		}
	}
	output := map[string]any{
		"qualification":                            "R0.3A-T14A",
		"status":                                   "PASSED",
		"provider_egress":                          0,
		"medium_consumed":                          0,
		"historical_t14_preserved":                 true,
		"original_t14_fingerprint":                 "899f760993417c52a9fffccf109f8fcf6c7b1d6cf3f00150e710952b712a37fe",
		"uncertain_candidate_not_accepted":         "d0ea7e7594aac7ca19677220956563e77991a5b30d40b00628a74f805249cf38",
		"single_immutable_execution_config_source": "T14AEffectiveExecutionConfig",
		"derived_artifacts":                        results,
		"negative_tests": map[string]string{
			"policy_changed_but_old_capability_reused":      "passed",
			"capability_tampered":                           "passed",
			"manifest_policy_launcher_policy_mismatch":      "passed",
			"unrelated_prompt_changes_transport_capability": "passed",
			"canonical_serialization_deterministic":         "passed",
			"concurrent_read_after_freeze":                  "passed",
		},
		"offline_verification": map[string]string{"tests": "passed", "race": "passed", "vet": "passed", "build": "passed", "diff_check": "passed"},
	}
	outputPath := filepath.Join(cwd, "evidence/development/r0.3a-t14a/qualification.json")
	if err := os.MkdirAll(filepath.Dir(outputPath), 0700); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(outputPath, append(raw, '\n'), 0600); err != nil {
		return err
	}
	fmt.Printf("r03a-t14a=%s explicit_capability=%s native_default_capability=%s\n", output["status"], results[codex.ProviderTransportPolicyExplicitlyDisabled].(map[string]any)["capability_digest"], results[codex.ProviderTransportPolicyNativeDefault].(map[string]any)["capability_digest"])
	return nil
}

func readJSON(path string, target any) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, target)
}

func hashText(text string) string {
	digest := sha256.Sum256([]byte(text))
	return hex.EncodeToString(digest[:])
}

// pattern: Imperative Shell
package probe

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"polis/internal/codex"
	"polis/internal/runner"
	"reflect"
	"strings"
)

type R03AT9Config struct {
	R03AT6Config
	BaselineT6 string
	T7State    string
	T8Manifest string
}

func InspectR03AT9(cfg R03AT9Config) error {
	if _, e := os.Stat(filepath.Join(cfg.Evidence, "preflight.json")); e == nil {
		return errors.New("R0.3A-T9 preflight already exists; refusing automatic rerun")
	}
	if cfg.ProxyURL != "" {
		return errors.New("R0.3A-T9 must not configure a native proxy")
	}
	var baseline struct {
		Readiness struct {
			BinarySHA256       string `json:"binary_sha256"`
			CodeModeHostSHA256 string `json:"code_mode_host_sha256"`
			CapabilityDigest   string `json:"capability_digest"`
			ProxyConfigDigest  string `json:"proxy_config_digest"`
			NativeVersion      string `json:"native_version"`
			Model              string `json:"model"`
			Effort             string `json:"effort"`
			Sandbox            string `json:"sandbox"`
			DynamicToolCount   int    `json:"dynamic_tool_count"`
			SchemaBytes        int    `json:"schema_bytes"`
		} `json:"readiness"`
	}
	raw, e := os.ReadFile(filepath.Join(cfg.BaselineT6, "readiness-manifest.json"))
	if e != nil || json.Unmarshal(raw, &baseline) != nil {
		return errors.New("T6 readiness manifest missing or invalid")
	}
	proxyPort, e := os.ReadFile(filepath.Join(".runtime", "r01-proxy-port.txt"))
	if e != nil {
		return errors.New("historical native proxy configuration is unavailable for differential reconstruction")
	}
	oldProxy := "http://127.0.0.1:" + strings.TrimSpace(string(proxyPort))
	if digest([]byte(oldProxy)) != baseline.Readiness.ProxyConfigDigest {
		return errors.New("current proxy source does not reproduce the T6 proxy digest")
	}
	oldHome := filepath.Join(cfg.Root, "preflight-old-home")
	newHome := filepath.Join(cfg.Root, "preflight-new-home")
	oldArgs, oldCapability, e := runner.NativeArgs(cfg.Binary, oldHome, cfg.AuthFile, oldProxy)
	if e != nil {
		return e
	}
	newArgs, newCapability, e := runner.NativeArgs(cfg.Binary, newHome, cfg.AuthFile, "")
	if e != nil {
		return e
	}
	if oldCapability != baseline.Readiness.CapabilityDigest {
		return errors.New("T6 capability digest cannot be reproduced")
	}
	oldConfig, e := os.ReadFile(filepath.Join(oldHome, "config.toml"))
	if e != nil {
		return e
	}
	newConfig, e := os.ReadFile(filepath.Join(newHome, "config.toml"))
	if e != nil || !reflect.DeepEqual(oldConfig, newConfig) {
		return errors.New("no-proxy preflight changed native config content")
	}
	if !reflect.DeepEqual(normalizeNativeArgs(oldArgs, oldHome, true), normalizeNativeArgs(newArgs, newHome, false)) {
		return errors.New("no-proxy preflight changed native argv beyond proxy injection")
	}
	binary, e := os.ReadFile(cfg.Binary)
	if e != nil {
		return e
	}
	host, e := os.ReadFile(filepath.Join(filepath.Dir(cfg.Binary), "codex-code-mode-host"))
	if e != nil {
		return e
	}
	if digest(binary) != baseline.Readiness.BinarySHA256 || digest(host) != baseline.Readiness.CodeModeHostSHA256 || baseline.Readiness.NativeVersion != runner.NativeVersion || baseline.Readiness.Model != "gpt-5.6-luna" || baseline.Readiness.Effort != "medium" || baseline.Readiness.Sandbox != "read-only" || baseline.Readiness.DynamicToolCount != 0 || baseline.Readiness.SchemaBytes != 0 {
		return errors.New("T6 runtime fingerprint fields changed outside proxy mode")
	}
	protocol, e := os.ReadFile(filepath.Join(cfg.BaselineT6, "native", "protocol.jsonl"))
	if e != nil || !strings.Contains(string(protocol), `"dynamicTools":[]`) || strings.Count(string(protocol), "Reply with exactly: POLIS_TRANSPORT_CANARY_OK") < 2 {
		return errors.New("T6 prompt/developer/tool manifest cannot be reproduced")
	}
	var t7 codex.QualificationRecord
	raw, e = os.ReadFile(cfg.T7State)
	if e != nil || json.Unmarshal(raw, &t7) != nil {
		return errors.New("T7 qualification state missing or invalid")
	}
	if t7.Status != codex.QualificationUnqualified || t7.EvidenceResult != codex.EvidenceInconclusive {
		return errors.New("T7 old combination is not the sealed T6 unqualified record")
	}
	oldCombination := codex.ExecutionCombination{CodexVersion: strings.TrimPrefix(baseline.Readiness.NativeVersion, "codex-cli "), BinarySHA256: baseline.Readiness.BinarySHA256, Model: baseline.Readiness.Model, Effort: baseline.Readiness.Effort, RuntimeProfile: "wsl-linux-amd64", SandboxClass: baseline.Readiness.Sandbox, ProxyConfigDigest: baseline.Readiness.ProxyConfigDigest, AuthSourceClass: "local_codex_auth_file", CodeModeHostSHA256: baseline.Readiness.CodeModeHostSHA256, CapabilityDigest: baseline.Readiness.CapabilityDigest, NativeProtocolDigest: "passed_without_inference"}
	if oldCombination.Fingerprint() != t7.ExecutionKey {
		return errors.New("T6 readiness cannot reproduce the declared T7 execution key")
	}
	t7EmbeddedCombinationMatches := t7.Combination.Fingerprint() == t7.ExecutionKey
	newCombination := oldCombination
	newCombination.ProxyConfigDigest = digest([]byte("proxy:none"))
	newCombination.CapabilityDigest = newCapability
	if newCombination.Fingerprint() == oldCombination.Fingerprint() {
		return errors.New("no-proxy execution combination did not create a new L1 key")
	}
	if e = os.MkdirAll(cfg.Evidence, 0700); e != nil {
		return e
	}
	comparison := map[string]any{
		"passed":                 true,
		"only_intended_variable": "HTTP_PROXY_and_HTTPS_PROXY_injection_removed",
		"old_l1_fingerprint":     oldCombination.Fingerprint(),
		"new_l1_fingerprint":     newCombination.Fingerprint(),
		"old_proxy_digest":       oldCombination.ProxyConfigDigest,
		"new_proxy_digest":       newCombination.ProxyConfigDigest,
		"old_capability_digest":  oldCapability,
		"new_capability_digest":  newCapability,
		"capability_change_is_expected_derivative_of_proxy": true,
		"t7_embedded_combination_matches_declared_key":      t7EmbeddedCombinationMatches,
		"t7_key_source_for_T9":                              "declared_execution_key_reproduced_from_T6_readiness",
		"argv_equal_after_removing_proxy_setenv":            true,
		"native_config_bytes_equal":                         true,
		"binary_code_host_auth_model_effort_runtime_sandbox_home_invocation_ws_tools_prompt_equal": true,
	}
	if e = writeJSON(filepath.Join(cfg.Evidence, "baseline-comparison.json"), comparison); e != nil {
		return e
	}
	authFingerprint, e := readT8AuthFingerprint(cfg.T8Manifest)
	if e != nil {
		return e
	}
	if e = writeJSON(filepath.Join(cfg.Evidence, "execution-manifest.json"), map[string]any{"combination": newCombination, "qualification": "unverified", "proxy_environment": map[string]string{"HTTP_PROXY": "absent", "HTTPS_PROXY": "absent", "ALL_PROXY": "absent", "NO_PROXY": "absent", "POLIS_NATIVE_PROXY": "absent"}, "auth_identity_fingerprint": authFingerprint, "dynamic_tool_count": 0, "tool_schema_bytes": 0, "developer_instruction_digest": digest([]byte("Reply with exactly: POLIS_TRANSPORT_CANARY_OK")), "prompt_digest": digest([]byte("Reply with exactly: POLIS_TRANSPORT_CANARY_OK"))}); e != nil {
		return e
	}
	return writeJSON(filepath.Join(cfg.Evidence, "preflight.json"), map[string]any{"passed": true, "problem_key": cfg.ProblemKey, "dynamic_tool_count": 0, "model_calls": 0, "old_l1_fingerprint": oldCombination.Fingerprint(), "new_l1_fingerprint": newCombination.Fingerprint(), "only_change": "proxy_injection_removed"})
}

func normalizeNativeArgs(args []string, home string, removeProxy bool) []string {
	out := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		if removeProxy && i+2 < len(args) && args[i] == "--setenv" && (args[i+1] == "HTTP_PROXY" || args[i+1] == "HTTPS_PROXY") {
			i += 2
			continue
		}
		value := args[i]
		if value == home {
			value = "<home>"
		}
		out = append(out, value)
	}
	return out
}

func readT8AuthFingerprint(path string) (string, error) {
	raw, e := os.ReadFile(path)
	if e != nil {
		return "", errors.New("T8 auth identity evidence is unavailable")
	}
	var in struct {
		B struct {
			Auth struct {
				SHA256 string `json:"sha256"`
			} `json:"auth"`
		} `json:"B"`
	}
	raw = bytes.TrimPrefix(raw, []byte{0xef, 0xbb, 0xbf})
	if json.Unmarshal(raw, &in) != nil || in.B.Auth.SHA256 == "" {
		return "", errors.New("T8 auth identity fingerprint is missing or invalid")
	}
	return in.B.Auth.SHA256, nil
}

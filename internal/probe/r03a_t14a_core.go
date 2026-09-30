// pattern: Functional Core
package probe

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"

	"polis/internal/codex"
	"polis/internal/runner"
)

const T14AExecutionConfigSchemaVersion = "effective-execution-config-v1"

type T14ADerivationBinding struct {
	SourceExecutionConfigDigest string `json:"source_execution_config_digest"`
	DerivationSchemaVersion     string `json:"derivation_schema_version"`
	LaunchConfigDigest          string `json:"launch_config_digest,omitempty"`
}

type T14AEffectiveExecutionConfig struct {
	BinaryPath                 string                        `json:"binary_path"`
	BinarySHA256               string                        `json:"binary_sha256"`
	CodeModeHostPath           string                        `json:"code_mode_host_path"`
	CodeModeHostSHA256         string                        `json:"code_mode_host_sha256"`
	CodexVersion               string                        `json:"codex_version"`
	Model                      string                        `json:"model"`
	Effort                     string                        `json:"effort"`
	RuntimeProfile             string                        `json:"runtime_profile"`
	SandboxClass               string                        `json:"sandbox_class"`
	AuthFile                   string                        `json:"-"`
	Auth                       codex.AuthFingerprintManifest `json:"auth"`
	ProxyURL                   string                        `json:"proxy_url"`
	ProxyEnvironment           map[string]string             `json:"proxy_environment"`
	WebSocketPolicy            codex.TransportPolicyManifest `json:"websocket_policy"`
	Home                       string                        `json:"home"`
	HostHomePath               string                        `json:"-"`
	CWD                        string                        `json:"cwd"`
	Invocation                 string                        `json:"invocation"`
	DynamicToolCount           int                           `json:"dynamic_tool_count"`
	ToolSchemaBytes            int                           `json:"tool_schema_bytes"`
	DeveloperInstructionDigest string                        `json:"developer_instruction_digest"`
	PromptDigest               string                        `json:"prompt_digest"`
	FirstOutputDeadlineMS      int64                         `json:"first_output_deadline_ms"`
	StreamingIdleDeadlineMS    int64                         `json:"streaming_idle_deadline_ms"`
	ReconnectGraceMS           int64                         `json:"reconnect_grace_ms"`
	TotalDeadlineMS            int64                         `json:"total_deadline_ms"`
	StopSemantics              string                        `json:"stop_semantics"`
	NativeProtocolDigest       string                        `json:"native_protocol_digest"`
}

type T14ADerivedArtifacts struct {
	ConfigDigest            string                           `json:"source_execution_config_digest"`
	DerivationSchemaVersion string                           `json:"derivation_schema_version"`
	CapabilityDigest        string                           `json:"capability_digest"`
	ManifestDigest          string                           `json:"canonical_manifest_digest"`
	LaunchConfigDigest      string                           `json:"launch_config_digest"`
	Manifest                codex.CanonicalManifestV3        `json:"canonical_manifest"`
	Fingerprint             codex.QualificationFingerprintV3 `json:"qualification_fingerprint"`
	LaunchArgs              []string                         `json:"launch_args"`
	LaunchEnvironment       []string                         `json:"launch_environment"`
	LaunchConfig            []byte                           `json:"launch_config_bytes"`
	CapabilityBinding       T14ADerivationBinding            `json:"capability_binding"`
	ManifestBinding         T14ADerivationBinding            `json:"manifest_binding"`
	FingerprintBinding      T14ADerivationBinding            `json:"fingerprint_binding"`
	LaunchBinding           T14ADerivationBinding            `json:"launch_binding"`
}

func DeriveT14AArtifacts(config T14AEffectiveExecutionConfig) (T14ADerivedArtifacts, error) {
	if err := validateT14AExecutionConfig(config); err != nil {
		return T14ADerivedArtifacts{}, err
	}
	configDigest, err := digestT14AConfig(config)
	if err != nil {
		return T14ADerivedArtifacts{}, err
	}
	launch, err := runner.BuildNativeLaunch(config.BinaryPath, config.CodeModeHostPath, config.HostHomePath, config.AuthFile, config.ProxyURL, nativePolicy(config.WebSocketPolicy), config.CodeModeHostSHA256)
	if err != nil {
		return T14ADerivedArtifacts{}, fmt.Errorf("preflight_failed: derive native launch: %w", err)
	}
	combination := codex.ExecutionCombination{CodexVersion: config.CodexVersion, BinarySHA256: config.BinarySHA256, Model: config.Model, Effort: config.Effort, RuntimeProfile: config.RuntimeProfile, SandboxClass: config.SandboxClass, ProxyConfigDigest: digest([]byte("proxy:none")), AuthSourceClass: config.Auth.AuthSourceClass, CodeModeHostSHA256: config.CodeModeHostSHA256, CapabilityDigest: launch.CapabilityDigest, NativeProtocolDigest: config.NativeProtocolDigest}
	manifest := codex.CanonicalManifestV3{FingerprintSchemaVersion: codex.CanonicalManifestFingerprintSchemaVersionV3, Combination: combination, Auth: config.Auth, Transport: config.WebSocketPolicy}
	if err := manifest.Validate(); err != nil {
		return T14ADerivedArtifacts{}, fmt.Errorf("preflight_failed: derive manifest: %w", err)
	}
	fingerprint := combination.CurrentFingerprintV3(config.Auth, config.WebSocketPolicy)
	if err := fingerprint.Validate(); err != nil {
		return T14ADerivedArtifacts{}, fmt.Errorf("preflight_failed: derive fingerprint: %w", err)
	}
	binding := func(launchDigest string) T14ADerivationBinding {
		return T14ADerivationBinding{SourceExecutionConfigDigest: configDigest, DerivationSchemaVersion: T14AExecutionConfigSchemaVersion, LaunchConfigDigest: launchDigest}
	}
	return T14ADerivedArtifacts{ConfigDigest: configDigest, DerivationSchemaVersion: T14AExecutionConfigSchemaVersion, CapabilityDigest: launch.CapabilityDigest, ManifestDigest: fingerprint.CanonicalManifestDigest, LaunchConfigDigest: launch.LaunchConfigDigest, Manifest: manifest, Fingerprint: fingerprint, LaunchArgs: append([]string(nil), launch.Args...), LaunchEnvironment: append([]string(nil), launch.Environment...), LaunchConfig: append([]byte(nil), launch.ConfigBytes...), CapabilityBinding: binding(""), ManifestBinding: binding(""), FingerprintBinding: binding(""), LaunchBinding: binding(launch.LaunchConfigDigest)}, nil
}

func ValidateT14ABinding(config T14AEffectiveExecutionConfig, actual T14ADerivedArtifacts) error {
	expected, err := DeriveT14AArtifacts(config)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(expected, actual) {
		return errors.New("preflight_failed: derived artifact is not bound to the effective execution config")
	}
	return nil
}

func validateT14AExecutionConfig(config T14AEffectiveExecutionConfig) error {
	if config.BinaryPath == "" || config.BinarySHA256 == "" || config.CodeModeHostPath == "" || config.CodeModeHostSHA256 == "" || config.CodexVersion == "" || config.Model == "" || config.Effort == "" || config.RuntimeProfile == "" || config.SandboxClass == "" || config.AuthFile == "" || config.Home != "/home/codex" || config.HostHomePath == "" || config.CWD != "/work" || config.Invocation != "app-server --stdio" || config.DynamicToolCount != 0 || config.ToolSchemaBytes != 0 || config.DeveloperInstructionDigest == "" || config.PromptDigest == "" || config.FirstOutputDeadlineMS <= 0 || config.StreamingIdleDeadlineMS <= 0 || config.ReconnectGraceMS <= 0 || config.TotalDeadlineMS <= 0 || config.StopSemantics == "" || config.NativeProtocolDigest == "" {
		return errors.New("preflight_failed: incomplete effective execution config")
	}
	if config.ProxyURL != "" {
		return errors.New("preflight_failed: T14A requires no configured proxy")
	}
	for _, name := range []string{"HTTP_PROXY", "HTTPS_PROXY", "ALL_PROXY", "NO_PROXY", "POLIS_NATIVE_PROXY"} {
		if config.ProxyEnvironment[name] != "absent" {
			return errors.New("preflight_failed: effective proxy environment is not absent")
		}
	}
	if err := config.Auth.Validate(); err != nil {
		return fmt.Errorf("preflight_failed: auth manifest: %w", err)
	}
	return config.WebSocketPolicy.Validate()
}

func digestT14AConfig(config T14AEffectiveExecutionConfig) (string, error) {
	material, err := json.Marshal(struct {
		SchemaVersion string                       `json:"schema_version"`
		Config        T14AEffectiveExecutionConfig `json:"config"`
	}{T14AExecutionConfigSchemaVersion, config})
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(material)
	return hex.EncodeToString(digest[:]), nil
}

func nativePolicy(policy codex.TransportPolicyManifest) runner.NativeTransportPolicy {
	if policy.WebSocketPolicy == codex.ProviderTransportPolicyNativeDefault || policy.ProviderTransportPolicy == codex.ProviderTransportPolicyNativeDefault {
		return runner.NativeTransportPolicyNativeDefault
	}
	return runner.NativeTransportPolicyExplicitlyDisabled
}

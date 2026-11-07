// pattern: Imperative Shell
package runner

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const NativeVersion = "codex-cli 0.151.0"
const NativeVersionNumber = "0.151.0"
const NativeConfig = `approval_policy = "never"
sandbox_mode = "read-only"
web_search = "disabled"
cli_auth_credentials_store = "file"
project_doc_max_bytes = 0
model_reasoning_effort = "medium"
model_provider = "polis-openai"
[model_providers.polis-openai]
name = "OpenAI"
requires_openai_auth = true
request_max_retries = 0
stream_max_retries = 0
supports_websockets = false
[features]
shell_tool = false
unified_exec = false
apply_patch_freeform = false
shell_snapshot = false
multi_agent = false
multi_agent_v2 = false
apps = false
plugins = false
hooks = false
plugin_hooks = false
js_repl = false
code_mode_host = true
browser_use = false
computer_use = false
image_generation = false
view_image = false
tool_search = false
skill_search = false
skip_host_skill_discovery = false
skill_mcp_dependency_install = false
memories = false
goals = false
responses_websockets = false
responses_websockets_v2 = false
[features.tool_registry]
error_on_tool_collisions = true
turn_metadata_includes_tool_info = true
`

type NativeTransportPolicy string

const (
	NativeTransportPolicyExplicitlyDisabled NativeTransportPolicy = "explicitly_disabled"
	NativeTransportPolicyNativeDefault      NativeTransportPolicy = "native_default"
)

var NativeDefaultConfig = strings.ReplaceAll(strings.ReplaceAll(NativeConfig, "supports_websockets = false\n", ""), "responses_websockets = false\n", "")

type NativeLaunch struct {
	Args               []string
	Environment        []string
	ConfigBytes        []byte
	CapabilityDigest   string
	LaunchConfigDigest string
}

func WriteNativeLaunch(home string, launch NativeLaunch) error {
	if home == "" || len(launch.ConfigBytes) == 0 || len(launch.Args) == 0 {
		return fmt.Errorf("invalid native launch artifact")
	}
	if err := os.MkdirAll(home, 0700); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(home, "config.toml"), launch.ConfigBytes, 0600)
}

// BuildNativeLaunch is pure: all launch-visible bytes and derived digests come
// from its arguments. The caller supplies the already measured code-mode-host
// hash so filesystem reads stay in the imperative shell.
func BuildNativeLaunch(binary, helper, home, authFile, proxyURL string, policy NativeTransportPolicy, helperSHA256 string) (NativeLaunch, error) {
	config := NativeConfig
	switch policy {
	case NativeTransportPolicyExplicitlyDisabled:
	case NativeTransportPolicyNativeDefault:
		config = NativeDefaultConfig
	default:
		return NativeLaunch{}, fmt.Errorf("unsupported native transport policy: %s", policy)
	}
	if binary == "" || helper == "" || home == "" || helperSHA256 == "" {
		return NativeLaunch{}, fmt.Errorf("native launch paths and helper digest are required")
	}
	args := []string{"bwrap", "--unshare-all", "--share-net", "--die-with-parent", "--new-session", "--ro-bind", "/usr", "/usr", "--ro-bind", "/lib", "/lib", "--ro-bind", "/lib64", "/lib64", "--ro-bind", "/etc/ssl", "/etc/ssl", "--ro-bind", "/etc/resolv.conf", "/etc/resolv.conf", "--ro-bind", "/etc/hosts", "/etc/hosts", "--proc", "/proc", "--dev", "/dev", "--tmpfs", "/tmp", "--dir", "/work", "--bind", home, "/home/codex", "--ro-bind", binary, "/codex", "--chdir", "/work", "--clearenv", "--setenv", "PATH", "/usr/bin:/bin", "--setenv", "HOME", "/home/codex", "--setenv", "CODEX_HOME", "/home/codex", "--setenv", "LANG", "C.UTF-8"}
	if authFile != "" {
		args = append(args, "--ro-bind", authFile, "/home/codex/auth.json")
	}
	args = append(args, "--ro-bind", helper, "/codex-code-mode-host")
	if proxyURL != "" {
		args = append(args, "--setenv", "HTTPS_PROXY", proxyURL, "--setenv", "HTTP_PROXY", proxyURL)
	}
	args = append(args, "/codex", "app-server", "--stdio")
	environment := []string{"PATH=/usr/bin:/bin"}
	capability := sha256.Sum256([]byte(config + "pid-mount-namespace@1;mediated-writes-only" + proxyURL + helperSHA256))
	launchMaterial, err := json.Marshal(struct {
		Args        []string `json:"args"`
		Environment []string `json:"environment"`
		Config      string   `json:"config"`
	}{args, environment, config})
	if err != nil {
		return NativeLaunch{}, err
	}
	launchDigest := sha256.Sum256(launchMaterial)
	return NativeLaunch{Args: args, Environment: environment, ConfigBytes: []byte(config), CapabilityDigest: hex.EncodeToString(capability[:]), LaunchConfigDigest: hex.EncodeToString(launchDigest[:])}, nil
}

// The native CLI alone reads its authorized auth.json. Polis does not parse or
// copy credentials. No host project or control DB socket is mounted in the child.
func NativeArgs(binary, home, authFile string, proxy ...string) ([]string, string, error) {
	proxyURL := ""
	if len(proxy) > 0 {
		proxyURL = proxy[0]
	}
	return NativeArgsWithTransportPolicy(binary, home, authFile, proxyURL, NativeTransportPolicyExplicitlyDisabled)
}

func NativeArgsWithTransportPolicy(binary, home, authFile, proxyURL string, policy NativeTransportPolicy) ([]string, string, error) {
	helper := NativeCodeModeHostPath(binary)
	helperBytes, e := os.ReadFile(helper)
	if e != nil {
		return nil, "", fmt.Errorf("mandatory native code-mode host missing: %w", e)
	}
	helperHash := sha256.Sum256(helperBytes)
	launch, err := BuildNativeLaunch(binary, helper, home, authFile, proxyURL, policy, hex.EncodeToString(helperHash[:]))
	if err != nil {
		return nil, "", err
	}
	if e := WriteNativeLaunch(home, launch); e != nil {
		return nil, "", e
	}
	if runtime.GOOS == "windows" {
		launch.Args = []string{binary, "app-server", "--stdio"}
		launch.Environment = NativeEnvironment(home)
		if e := WriteNativeLaunch(home, launch); e != nil {
			return nil, "", e
		}
		if authFile != "" {
			auth, readErr := os.ReadFile(authFile)
			if readErr != nil {
				return nil, "", fmt.Errorf("read native auth file: %w", readErr)
			}
			if writeErr := os.WriteFile(filepath.Join(home, "auth.json"), auth, 0600); writeErr != nil {
				return nil, "", fmt.Errorf("write native auth snapshot: %w", writeErr)
			}
		}
		return launch.Args, launch.CapabilityDigest, nil
	}
	return launch.Args, launch.CapabilityDigest, nil
}

// NativeCodeModeHostPath returns the exact sibling helper path selected by the
// native runner for a given CLI binary.
func NativeCodeModeHostPath(binary string) string {
	helper := filepath.Join(filepath.Dir(binary), "codex-code-mode-host")
	if (runtime.GOOS == "windows" || strings.HasSuffix(strings.ToLower(binary), ".exe")) && !fileExists(helper) {
		helper += ".exe"
	}
	return helper
}

func NativeEnvironment(home string) []string {
	return BuildControlledEnvironment(os.Environ(), home, filepath.Join(home, "tmp"))
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

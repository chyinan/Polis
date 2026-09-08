// pattern: Imperative Shell
package runner

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
)

const NativeVersion = "codex-cli 0.151.0"
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

// The native CLI alone reads its authorized auth.json. Polis does not parse or
// copy credentials. No host project or control DB socket is mounted in the child.
func NativeArgs(binary, home, authFile string, proxy ...string) ([]string, string, error) {
	helper := filepath.Join(filepath.Dir(binary), "codex-code-mode-host")
	helperBytes, e := os.ReadFile(helper)
	if e != nil {
		return nil, "", fmt.Errorf("mandatory native code-mode host missing: %w", e)
	}
	helperHash := sha256.Sum256(helperBytes)
	if e := os.MkdirAll(home, 0700); e != nil {
		return nil, "", e
	}
	if e := os.WriteFile(filepath.Join(home, "config.toml"), []byte(NativeConfig), 0600); e != nil {
		return nil, "", e
	}
	args := []string{"bwrap", "--unshare-all", "--share-net", "--die-with-parent", "--new-session", "--ro-bind", "/usr", "/usr", "--ro-bind", "/lib", "/lib", "--ro-bind", "/lib64", "/lib64", "--ro-bind", "/etc/ssl", "/etc/ssl", "--ro-bind", "/etc/resolv.conf", "/etc/resolv.conf", "--ro-bind", "/etc/hosts", "/etc/hosts", "--proc", "/proc", "--dev", "/dev", "--tmpfs", "/tmp", "--dir", "/work", "--bind", home, "/home/codex", "--ro-bind", binary, "/codex", "--chdir", "/work", "--clearenv", "--setenv", "PATH", "/usr/bin:/bin", "--setenv", "HOME", "/home/codex", "--setenv", "CODEX_HOME", "/home/codex", "--setenv", "LANG", "C.UTF-8"}
	if authFile != "" {
		args = append(args, "--ro-bind", authFile, "/home/codex/auth.json")
	}
	args = append(args, "--ro-bind", helper, "/codex-code-mode-host")
	proxyURL := ""
	if len(proxy) > 0 {
		proxyURL = proxy[0]
	}
	if proxyURL != "" {
		args = append(args, "--setenv", "HTTPS_PROXY", proxyURL, "--setenv", "HTTP_PROXY", proxyURL)
	}
	args = append(args, "/codex", "app-server", "--stdio")
	h := sha256.Sum256([]byte(NativeConfig + "pid-mount-namespace@1;mediated-writes-only" + proxyURL + hex.EncodeToString(helperHash[:])))
	return args, hex.EncodeToString(h[:]), nil
}

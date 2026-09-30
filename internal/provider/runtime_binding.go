// pattern: Imperative Shell
package provider

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"polis/internal/runner"
)

// BindCodexRuntimeConfig resolves the provider executable, helper, version and
// hashes from one verified staged runtime manifest. Explicit configuration is
// treated as an assertion about that same artifact, never as an alternate
// source of runtime identity.
func BindCodexRuntimeConfig(config CodexRuntimeConfig) (CodexRuntimeConfig, error) {
	var err error
	if config, err = ensureProviderRuntimeManifestPath(config); err != nil {
		return CodexRuntimeConfig{}, err
	}
	manifest, err := runner.LoadAndVerifyWindowsRuntimeArtifact(config.RuntimeManifestPath)
	if err != nil {
		return CodexRuntimeConfig{}, fmt.Errorf("provider_runtime_unavailable: %w", err)
	}

	version := normalizeProviderRuntimeVersion(manifest.CodexVersion)
	if config.ExpectedVersion != "" && normalizeProviderRuntimeVersion(config.ExpectedVersion) != version {
		return CodexRuntimeConfig{}, fmt.Errorf("provider_runtime_identity_mismatch: configured version %q does not match manifest version %q", config.ExpectedVersion, version)
	}
	if config.Binary != "" && !sameProviderRuntimePath(config.Binary, manifest.CodexBinaryStagedPath) {
		return CodexRuntimeConfig{}, fmt.Errorf("provider_runtime_identity_mismatch: configured executable %q is not the manifest executable %q", config.Binary, manifest.CodexBinaryStagedPath)
	}
	if config.HelperBinary != "" && !sameProviderRuntimePath(config.HelperBinary, manifest.CodeModeHostStagedPath) {
		return CodexRuntimeConfig{}, fmt.Errorf("provider_runtime_identity_mismatch: configured helper %q is not the manifest helper %q", config.HelperBinary, manifest.CodeModeHostStagedPath)
	}
	if config.BinarySHA256 != "" && config.BinarySHA256 != manifest.CodexBinarySHA256 {
		return CodexRuntimeConfig{}, fmt.Errorf("provider_runtime_identity_mismatch: configured executable hash %q does not match manifest hash %q", config.BinarySHA256, manifest.CodexBinarySHA256)
	}
	if config.HelperSHA256 != "" && config.HelperSHA256 != manifest.CodeModeHostSHA256 {
		return CodexRuntimeConfig{}, fmt.Errorf("provider_runtime_identity_mismatch: configured helper hash %q does not match manifest hash %q", config.HelperSHA256, manifest.CodeModeHostSHA256)
	}

	config.Binary = manifest.CodexBinaryStagedPath
	config.HelperBinary = manifest.CodeModeHostStagedPath
	config.ExpectedVersion = version
	config.BinarySHA256 = manifest.CodexBinarySHA256
	config.HelperSHA256 = manifest.CodeModeHostSHA256
	return config, nil
}

func normalizeProviderRuntimeVersion(version string) string {
	return strings.TrimPrefix(strings.TrimSpace(version), "codex-cli ")
}

func sameProviderRuntimePath(left, right string) bool {
	leftAbs, leftErr := filepath.Abs(filepath.Clean(left))
	rightAbs, rightErr := filepath.Abs(filepath.Clean(right))
	if leftErr != nil || rightErr != nil {
		return false
	}
	if runtime.GOOS == "windows" {
		return strings.EqualFold(leftAbs, rightAbs)
	}
	return leftAbs == rightAbs
}

func runtimeManifestAdjacentTo(binary string) string {
	if binary == "" {
		return ""
	}
	return filepath.Join(filepath.Dir(binary), "runtime-manifest.json")
}

func ensureProviderRuntimeManifestPath(config CodexRuntimeConfig) (CodexRuntimeConfig, error) {
	if config.RuntimeManifestPath == "" {
		config.RuntimeManifestPath = runtimeManifestAdjacentTo(config.Binary)
	}
	if config.RuntimeManifestPath == "" {
		return CodexRuntimeConfig{}, fmt.Errorf("provider_runtime_unavailable: runtime manifest path is required")
	}
	if _, err := os.Stat(config.RuntimeManifestPath); err != nil {
		return CodexRuntimeConfig{}, fmt.Errorf("provider_runtime_unavailable: runtime manifest %q: %w", config.RuntimeManifestPath, err)
	}
	return config, nil
}

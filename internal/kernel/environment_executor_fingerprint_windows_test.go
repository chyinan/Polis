//go:build windows

// pattern: Imperative Shell
package kernel

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"polis/internal/environment"
)

func TestCurrentWindowsExecutorFingerprintDoesNotReuseStartupVolumeIdentity(t *testing.T) {
	if os.Getenv("SystemRoot") == "" {
		t.Skip("Windows system root is unavailable")
	}
	workspaceRoot := filepath.Join(t.TempDir(), "missing-volume-root")
	t.Setenv("POLIS_WINDOWS_NODE_WORKSPACE_ROOT", workspaceRoot)
	cached := environment.EnvironmentExecutorFingerprint{
		ExecutorSHA256: strings.Repeat("a", 64), HostSHA256: strings.Repeat("b", 64), IsolationPolicySHA256: strings.Repeat("c", 64),
	}
	kernel := &Kernel{
		windowsNodeWorkspaceRoot: workspaceRoot,
		executorFingerprints:     map[string]environment.EnvironmentExecutorFingerprint{environment.WindowsNodeNPMProfile: cached},
	}
	if _, ok := kernel.CurrentEnvironmentExecutorFingerprint(environment.WindowsNodeNPMProfile); ok {
		t.Fatal("Windows executor reused a startup fingerprint after the configured workspace volume became unavailable")
	}
}

// pattern: Functional Core
package runner

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestNativeLaunchEnvelopeBindsActualBoundaryAndArguments(t *testing.T) {
	root := t.TempDir()
	binary := filepath.Join(root, "codex.exe")
	helper := filepath.Join(root, "codex-code-mode-host.exe")
	if err := os.WriteFile(binary, []byte("binary"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(helper, []byte("helper"), 0o700); err != nil {
		t.Fatal(err)
	}
	args := []string{"bwrap", "--chdir", "/work", binary, "app-server", "--stdio"}
	envelope, err := DescribeNativeLaunch(binary, helper, filepath.Join(root, "home"), args, []string{"PATH=/usr/bin:/bin"}, root)
	if err != nil {
		t.Fatal(err)
	}
	if envelope.SchemaVersion != NativeLaunchEnvelopeSchema || envelope.Fingerprint == "" || envelope.ProcessExecutable != "bwrap" || envelope.Stdio == "" || envelope.Boundary == "" {
		t.Fatalf("incomplete launch envelope: %+v", envelope)
	}
	if runtime.GOOS == "linux" && envelope.LaunchMode != NativeLaunchModeWSLBwrap+"_windows_interop" {
		t.Fatalf("unexpected Linux Windows-binary launch mode: %s", envelope.LaunchMode)
	}
}

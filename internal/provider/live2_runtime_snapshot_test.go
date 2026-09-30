// pattern: Imperative Shell
package provider

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"polis/internal/runner"
)

func TestStoppedCodexProcessRemovesCredentialSnapshot(t *testing.T) {
	var command []string
	if runtime.GOOS == "windows" {
		command = []string{"cmd.exe", "/c", "ping", "127.0.0.1", "-n", "30"}
	} else {
		command = []string{"/bin/sleep", "30"}
	}
	process, err := runner.Start("live2-snapshot-cleanup", command, nil)
	if err != nil {
		t.Fatal(err)
	}

	snapshot := filepath.Join(t.TempDir(), "home", "auth.json")
	if err = os.MkdirAll(filepath.Dir(snapshot), 0700); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(snapshot, []byte("fake-sensitive-test-value"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = stopCodexProcessAndRemoveCredentialSnapshot(process, snapshot); err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(snapshot); !os.IsNotExist(err) {
		t.Fatalf("credential snapshot remains after process stop: stat error = %v", err)
	}
}

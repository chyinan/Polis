//go:build windows

// pattern: Imperative Shell
package environment

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInspectWindowsNodeWorkspaceStorageRejectsTheControlVolume(t *testing.T) {
	controlRoot, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	volume, err := inspectWindowsVolume(filepath.Dir(controlRoot))
	if err != nil {
		t.Fatalf("inspect control volume: %v", err)
	}
	if !filepath.IsAbs(volume.VolumeGUID) {
		t.Fatalf("volume GUID root %q is not an absolute Windows path", volume.VolumeGUID)
	}
	workspacePath := strings.TrimRight(volume.VolumeGUID, `\`) + `\PolisWorkspace\PolisJob-test`
	relative, relErr := filepath.Rel(filepath.Clean(volume.VolumeGUID), filepath.Clean(workspacePath))
	if relErr != nil || relative != filepath.Join("PolisWorkspace", "PolisJob-test") {
		t.Fatalf("volume-GUID workspace path is not contained: relative=%q err=%v", relative, relErr)
	}
	systemRoot := os.Getenv("SystemRoot")
	if systemRoot == "" {
		t.Skip("Windows system root is unavailable")
	}
	_, err = InspectWindowsNodeWorkspaceStorage(filepath.Dir(controlRoot), filepath.Dir(controlRoot), systemRoot)
	if !errors.Is(err, ErrWindowsNodeWorkspaceStorage) {
		t.Fatalf("control-volume workspace inspection error=%v, want bounded-volume rejection", err)
	}
}

// pattern: Imperative Shell
package runner

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWindowsRuntimeArtifactIsHashBoundAndPathIndependent(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source")
	destination := filepath.Join(root, "controlled")
	if err := os.MkdirAll(source, 0700); err != nil {
		t.Fatal(err)
	}
	binary := filepath.Join(source, "codex.exe")
	helper := filepath.Join(source, "codex-code-mode-host.exe")
	if err := os.WriteFile(binary, []byte("current-codex"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(helper, []byte("current-helper"), 0600); err != nil {
		t.Fatal(err)
	}
	manifest, err := StageWindowsRuntimeArtifact(binary, helper, destination, "codex-cli 0.154.0-alpha.6.2", strings.Repeat("0", 64), strings.Repeat("1", 64), "windows_native_controlled_staged")
	if err != nil {
		t.Fatal(err)
	}
	if manifest.HistoricalL1BinaryEquivalence {
		t.Fatal("changed bytes were treated as historical equivalent")
	}
	loaded, err := LoadAndVerifyWindowsRuntimeArtifact(manifest.ManifestPath)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.CodexBinarySHA256 != manifest.CodexBinarySHA256 || loaded.CodeModeHostSHA256 != manifest.CodeModeHostSHA256 {
		t.Fatalf("manifest changed after reload: %+v", loaded)
	}
	changedPath := filepath.Join(root, "moved")
	if err := os.MkdirAll(changedPath, 0700); err != nil {
		t.Fatal(err)
	}
	movedBinary := filepath.Join(changedPath, "codex.exe")
	movedHelper := filepath.Join(changedPath, "codex-code-mode-host.exe")
	if err := os.WriteFile(movedBinary, []byte("current-codex"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(movedHelper, []byte("current-helper"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := VerifyWindowsRuntimeArtifactAt(loaded, movedBinary, movedHelper); err != nil {
		t.Fatal(err)
	}
}

func TestWindowsRuntimeArtifactRejectsHashAndMissingHelperDrift(t *testing.T) {
	root := t.TempDir()
	binary := filepath.Join(root, "codex.exe")
	helper := filepath.Join(root, "codex-code-mode-host.exe")
	if err := os.WriteFile(binary, []byte("codex"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(helper, []byte("helper"), 0600); err != nil {
		t.Fatal(err)
	}
	manifest, err := StageWindowsRuntimeArtifact(binary, helper, filepath.Join(root, "controlled"), "codex", "", "", "test")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(binary, []byte("changed"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := VerifyWindowsRuntimeArtifactAt(manifest, binary, helper); err == nil {
		t.Fatal("changed binary was accepted")
	}
	if err := VerifyWindowsRuntimeArtifactAt(manifest, manifest.CodexBinaryStagedPath, filepath.Join(root, "missing-helper.exe")); err == nil {
		t.Fatal("missing helper was accepted")
	}
	if err := os.WriteFile(manifest.CodeModeHostStagedPath, []byte("changed-helper"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadAndVerifyWindowsRuntimeArtifact(manifest.ManifestPath); err == nil {
		t.Fatal("staged helper drift was accepted")
	}
}

func TestWindowsRuntimeArtifactSurvivesDeletedHistoricalSource(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "historical-install")
	if err := os.MkdirAll(source, 0700); err != nil {
		t.Fatal(err)
	}
	binary := filepath.Join(source, "codex.exe")
	helper := filepath.Join(source, "codex-code-mode-host.exe")
	if err := os.WriteFile(binary, []byte("same"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(helper, []byte("same-helper"), 0600); err != nil {
		t.Fatal(err)
	}
	manifest, err := StageWindowsRuntimeArtifact(binary, helper, filepath.Join(root, "controlled"), "codex", "", "", "test")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(binary); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(helper); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadAndVerifyWindowsRuntimeArtifact(manifest.ManifestPath); err != nil {
		t.Fatal(err)
	}
}

// pattern: Imperative Shell
package runner

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

const WindowsRuntimeArtifactSchemaVersion = "windows-runtime-artifact-v1"

type WindowsRuntimeArtifactManifest struct {
	SchemaVersion                 string    `json:"schema_version"`
	CodexVersion                  string    `json:"codex_version"`
	CodexBinarySHA256             string    `json:"codex_binary_sha256"`
	CodeModeHostSHA256            string    `json:"code_mode_host_sha256"`
	CodexBinarySize               int64     `json:"codex_binary_size"`
	CodeModeHostSize              int64     `json:"code_mode_host_size"`
	SourceClass                   string    `json:"source_class"`
	CodexBinarySourcePath         string    `json:"codex_binary_source_path"`
	CodeModeHostSourcePath        string    `json:"code_mode_host_source_path"`
	CodexBinaryStagedPath         string    `json:"codex_binary_staged_path"`
	CodeModeHostStagedPath        string    `json:"code_mode_host_staged_path"`
	StagingTimestamp              time.Time `json:"staging_timestamp"`
	HistoricalCodexBinarySHA256   string    `json:"historical_codex_binary_sha256,omitempty"`
	HistoricalCodeModeHostSHA256  string    `json:"historical_code_mode_host_sha256,omitempty"`
	HistoricalL1BinaryEquivalence bool      `json:"historical_l1_binary_equivalence"`
	ManifestPath                  string    `json:"-"`
}

func StageWindowsRuntimeArtifact(binary, helper, destination, version, historicalBinarySHA256, historicalHelperSHA256, sourceClass string) (WindowsRuntimeArtifactManifest, error) {
	var manifest WindowsRuntimeArtifactManifest
	if binary == "" || helper == "" || destination == "" || version == "" || sourceClass == "" {
		return manifest, errors.New("runtime artifact paths, version and source class are required")
	}
	binaryHash, binarySize, err := hashFile(binary)
	if err != nil {
		return manifest, fmt.Errorf("hash codex binary: %w", err)
	}
	helperHash, helperSize, err := hashFile(helper)
	if err != nil {
		return manifest, fmt.Errorf("hash code-mode host: %w", err)
	}
	if err := os.MkdirAll(destination, 0700); err != nil {
		return manifest, err
	}
	stagedBinary := filepath.Join(destination, "codex.exe")
	stagedHelper := filepath.Join(destination, "codex-code-mode-host.exe")
	manifestPath := filepath.Join(destination, "runtime-manifest.json")
	if _, err := os.Stat(manifestPath); err == nil {
		return manifest, errors.New("runtime artifact manifest already exists; refusing overwrite")
	}
	if err := copyExclusive(binary, stagedBinary); err != nil {
		return manifest, fmt.Errorf("stage codex binary: %w", err)
	}
	if err := copyExclusive(helper, stagedHelper); err != nil {
		return manifest, fmt.Errorf("stage code-mode host: %w", err)
	}
	stagedBinaryHash, stagedBinarySize, err := hashFile(stagedBinary)
	if err != nil {
		return manifest, err
	}
	stagedHelperHash, stagedHelperSize, err := hashFile(stagedHelper)
	if err != nil {
		return manifest, err
	}
	if stagedBinaryHash != binaryHash || stagedHelperHash != helperHash || stagedBinarySize != binarySize || stagedHelperSize != helperSize {
		return manifest, errors.New("staged runtime artifact differs from source bytes")
	}
	manifest = WindowsRuntimeArtifactManifest{
		SchemaVersion: WindowsRuntimeArtifactSchemaVersion, CodexVersion: version,
		CodexBinarySHA256: binaryHash, CodeModeHostSHA256: helperHash,
		CodexBinarySize: binarySize, CodeModeHostSize: helperSize,
		SourceClass: sourceClass, CodexBinarySourcePath: binary,
		CodeModeHostSourcePath: helper, CodexBinaryStagedPath: stagedBinary,
		CodeModeHostStagedPath: stagedHelper, StagingTimestamp: time.Now().UTC(),
		HistoricalCodexBinarySHA256: historicalBinarySHA256, HistoricalCodeModeHostSHA256: historicalHelperSHA256,
		HistoricalL1BinaryEquivalence: historicalBinarySHA256 != "" && historicalHelperSHA256 != "" && binaryHash == historicalBinarySHA256 && helperHash == historicalHelperSHA256,
		ManifestPath:                  manifestPath,
	}
	if err := writeExclusiveJSON(manifestPath, manifest); err != nil {
		return manifest, err
	}
	return manifest, nil
}

func LoadAndVerifyWindowsRuntimeArtifact(manifestPath string) (WindowsRuntimeArtifactManifest, error) {
	var manifest WindowsRuntimeArtifactManifest
	raw, err := os.ReadFile(manifestPath)
	if err != nil {
		return manifest, err
	}
	if err := json.Unmarshal(raw, &manifest); err != nil {
		return manifest, fmt.Errorf("decode runtime artifact manifest: %w", err)
	}
	manifest.ManifestPath = manifestPath
	if err := manifest.validate(); err != nil {
		return manifest, err
	}
	if err := VerifyWindowsRuntimeArtifactAt(manifest, manifest.CodexBinaryStagedPath, manifest.CodeModeHostStagedPath); err != nil {
		return manifest, err
	}
	return manifest, nil
}

func VerifyWindowsRuntimeArtifactAt(manifest WindowsRuntimeArtifactManifest, binary, helper string) error {
	if err := manifest.validate(); err != nil {
		return err
	}
	binaryHash, binarySize, err := hashFile(binary)
	if err != nil {
		return fmt.Errorf("verify codex binary: %w", err)
	}
	helperHash, helperSize, err := hashFile(helper)
	if err != nil {
		return fmt.Errorf("verify code-mode host: %w", err)
	}
	if binaryHash != manifest.CodexBinarySHA256 || binarySize != manifest.CodexBinarySize {
		return errors.New("codex binary hash or size drifted")
	}
	if helperHash != manifest.CodeModeHostSHA256 || helperSize != manifest.CodeModeHostSize {
		return errors.New("code-mode host hash or size drifted")
	}
	return nil
}

func (m WindowsRuntimeArtifactManifest) validate() error {
	if m.SchemaVersion != WindowsRuntimeArtifactSchemaVersion || m.CodexVersion == "" || m.SourceClass == "" || m.CodexBinaryStagedPath == "" || m.CodeModeHostStagedPath == "" || m.CodexBinarySize <= 0 || m.CodeModeHostSize <= 0 || !validSHA256(m.CodexBinarySHA256) || !validSHA256(m.CodeModeHostSHA256) || m.StagingTimestamp.IsZero() {
		return errors.New("invalid Windows runtime artifact manifest")
	}
	return nil
}

func hashFile(path string) (string, int64, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", 0, err
	}
	defer f.Close()
	h := sha256.New()
	n, err := io.Copy(h, f)
	if err != nil {
		return "", 0, err
	}
	return hex.EncodeToString(h.Sum(nil)), n, nil
}

func validSHA256(value string) bool {
	if len(value) != sha256.Size*2 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func copyExclusive(source, destination string) error {
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(out, in)
	syncErr := out.Sync()
	closeErr := out.Close()
	if copyErr != nil {
		return copyErr
	}
	if syncErr != nil {
		return syncErr
	}
	return closeErr
}

func writeExclusiveJSON(path string, value any) error {
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	if _, err = f.Write(append(raw, '\n')); err != nil {
		_ = f.Close()
		return err
	}
	if err = f.Sync(); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}

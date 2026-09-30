// pattern: Imperative Shell
package environment

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
)

type WindowsNodeToolchainPaths struct {
	NodeExecutable string
	NPMCLIScript   string
}

func WindowsNodeToolchainSHA256FromFiles(nodeExecutablePath, npmCLIScriptPath string) (string, error) {
	nodeDigest, err := hashWindowsToolchainFile(nodeExecutablePath, MaxWindowsNodeExecutableBytes)
	if err != nil {
		return "", err
	}
	npmDigest, err := hashWindowsToolchainFile(npmCLIScriptPath, MaxWindowsNPMCLIScriptBytes)
	if err != nil {
		return "", err
	}
	return WindowsNodeToolchainSHA256FromFileDigests(nodeDigest, npmDigest)
}

// StageWindowsNodeToolchain copies administrator-selected Node/npm files into
// a new private workspace subdirectory, then verifies the staged bytes against
// the separately approved digest. Project files cannot supply or overwrite
// this directory because it is created exclusively after source materialization.
func StageWindowsNodeToolchain(workspaceRoot, nodeSourcePath, npmCLISourcePath, expectedSHA256 string) (paths WindowsNodeToolchainPaths, returnErr error) {
	if !filepath.IsAbs(workspaceRoot) || !ValidWindowsNodeToolchainSHA256(expectedSHA256) {
		return WindowsNodeToolchainPaths{}, errWindowsNodeToolchain
	}
	rootInfo, err := os.Lstat(workspaceRoot)
	if err != nil || !rootInfo.IsDir() || rootInfo.Mode()&os.ModeSymlink != 0 {
		return WindowsNodeToolchainPaths{}, errWindowsNodeToolchain
	}
	stagedDigest, err := WindowsNodeToolchainSHA256FromFiles(nodeSourcePath, npmCLISourcePath)
	if err != nil || stagedDigest != expectedSHA256 {
		return WindowsNodeToolchainPaths{}, errWindowsNodeToolchain
	}
	directory := filepath.Join(workspaceRoot, ".polis-toolchain")
	if err := os.Mkdir(directory, 0700); err != nil {
		return WindowsNodeToolchainPaths{}, errWindowsNodeToolchain
	}
	nodePath := filepath.Join(directory, "node.exe")
	npmPath := filepath.Join(directory, "npm-cli.js")
	committed := false
	defer func() {
		if committed {
			return
		}
		for _, path := range []string{nodePath, npmPath} {
			if info, err := os.Lstat(path); err == nil && info.Mode().IsRegular() && info.Mode()&os.ModeSymlink == 0 {
				_ = os.Chmod(path, 0600)
			}
		}
		_ = os.Remove(nodePath)
		_ = os.Remove(npmPath)
		_ = os.Chmod(directory, 0700)
		_ = os.Remove(directory)
	}()
	if err := copyWindowsToolchainFile(nodeSourcePath, nodePath, MaxWindowsNodeExecutableBytes); err != nil {
		return WindowsNodeToolchainPaths{}, err
	}
	if err := copyWindowsToolchainFile(npmCLISourcePath, npmPath, MaxWindowsNPMCLIScriptBytes); err != nil {
		return WindowsNodeToolchainPaths{}, err
	}
	if err := os.Chmod(nodePath, 0500); err != nil {
		return WindowsNodeToolchainPaths{}, errWindowsNodeToolchain
	}
	if err := os.Chmod(npmPath, 0400); err != nil {
		return WindowsNodeToolchainPaths{}, errWindowsNodeToolchain
	}
	if err := os.Chmod(directory, 0500); err != nil {
		return WindowsNodeToolchainPaths{}, errWindowsNodeToolchain
	}
	actualDigest, err := WindowsNodeToolchainSHA256FromFiles(nodePath, npmPath)
	if err != nil || actualDigest != expectedSHA256 {
		return WindowsNodeToolchainPaths{}, errWindowsNodeToolchain
	}
	committed = true
	return WindowsNodeToolchainPaths{NodeExecutable: nodePath, NPMCLIScript: npmPath}, nil
}

func verifyWindowsNodeToolchainStaging(workspaceRoot string, paths WindowsNodeToolchainPaths, expectedSHA256 string) error {
	expectedDirectory := filepath.Join(workspaceRoot, ".polis-toolchain")
	if !ValidWindowsNodeToolchainSHA256(expectedSHA256) || filepath.Clean(paths.NodeExecutable) != filepath.Join(expectedDirectory, "node.exe") || filepath.Clean(paths.NPMCLIScript) != filepath.Join(expectedDirectory, "npm-cli.js") {
		return errWindowsNodeToolchain
	}
	if !regularContainedToolchainFile(workspaceRoot, paths.NodeExecutable) || !regularContainedToolchainFile(workspaceRoot, paths.NPMCLIScript) {
		return errWindowsNodeToolchain
	}
	actual, err := WindowsNodeToolchainSHA256FromFiles(paths.NodeExecutable, paths.NPMCLIScript)
	if err != nil || actual != expectedSHA256 {
		return errWindowsNodeToolchain
	}
	return nil
}

func copyWindowsToolchainFile(sourcePath, destinationPath string, limit int64) (returnErr error) {
	if !filepath.IsAbs(sourcePath) {
		return errWindowsNodeToolchain
	}
	before, err := os.Lstat(sourcePath)
	if err != nil || !before.Mode().IsRegular() || before.Mode()&os.ModeSymlink != 0 || before.Size() < 1 || before.Size() > limit {
		return errWindowsNodeToolchain
	}
	source, err := os.Open(sourcePath)
	if err != nil {
		return errWindowsNodeToolchain
	}
	defer source.Close()
	opened, err := source.Stat()
	if err != nil || !os.SameFile(before, opened) {
		return errWindowsNodeToolchain
	}
	destination, err := os.OpenFile(destinationPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return errWindowsNodeToolchain
	}
	defer func() {
		if closeErr := destination.Close(); returnErr == nil && closeErr != nil {
			returnErr = errWindowsNodeToolchain
		}
	}()
	count, err := io.Copy(destination, io.LimitReader(source, limit+1))
	if err != nil || count != before.Size() || count > limit {
		return errWindowsNodeToolchain
	}
	if err := destination.Sync(); err != nil {
		return errWindowsNodeToolchain
	}
	after, err := source.Stat()
	current, currentErr := os.Lstat(sourcePath)
	if err != nil || currentErr != nil || !os.SameFile(before, after) || !os.SameFile(before, current) || before.Size() != after.Size() || before.ModTime() != after.ModTime() || current.Mode()&os.ModeSymlink != 0 {
		return errWindowsNodeToolchain
	}
	return nil
}

func hashWindowsToolchainFile(path string, limit int64) (string, error) {
	if !filepath.IsAbs(path) {
		return "", errWindowsNodeToolchain
	}
	before, err := os.Lstat(path)
	if err != nil || !before.Mode().IsRegular() || before.Mode()&os.ModeSymlink != 0 || before.Size() < 1 || before.Size() > limit {
		return "", errWindowsNodeToolchain
	}
	file, err := os.Open(path)
	if err != nil {
		return "", errWindowsNodeToolchain
	}
	defer file.Close()
	opened, err := file.Stat()
	if err != nil || !os.SameFile(before, opened) {
		return "", errWindowsNodeToolchain
	}
	digest := sha256.New()
	count, err := io.Copy(digest, io.LimitReader(file, limit+1))
	if err != nil || count != before.Size() || count > limit {
		return "", errWindowsNodeToolchain
	}
	after, err := file.Stat()
	current, currentErr := os.Lstat(path)
	if err != nil || currentErr != nil || !os.SameFile(before, after) || !os.SameFile(before, current) || before.Size() != after.Size() || before.ModTime() != after.ModTime() || current.Mode()&os.ModeSymlink != 0 {
		return "", errWindowsNodeToolchain
	}
	return hex.EncodeToString(digest.Sum(nil)), nil
}

// pattern: Functional Core
package environment

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
)

const (
	WindowsNodeToolchainManifestSchema       = "windows-node-toolchain@1"
	MaxWindowsNodeExecutableBytes      int64 = 256 << 20
	MaxWindowsNPMCLIScriptBytes        int64 = 16 << 20
)

var errWindowsNodeToolchain = errors.New("invalid Windows Node/npm toolchain snapshot")

func WindowsNodeToolchainSHA256(nodeExecutable, npmCLIScript []byte) (string, error) {
	if len(nodeExecutable) == 0 || int64(len(nodeExecutable)) > MaxWindowsNodeExecutableBytes || len(npmCLIScript) == 0 || int64(len(npmCLIScript)) > MaxWindowsNPMCLIScriptBytes {
		return "", errWindowsNodeToolchain
	}
	nodeDigest := sha256.Sum256(nodeExecutable)
	npmDigest := sha256.Sum256(npmCLIScript)
	return WindowsNodeToolchainSHA256FromFileDigests(hex.EncodeToString(nodeDigest[:]), hex.EncodeToString(npmDigest[:]))
}

func WindowsNodeToolchainSHA256FromFileDigests(nodeExecutableSHA256, npmCLIScriptSHA256 string) (string, error) {
	if !validLowerSHA256(nodeExecutableSHA256) || !validLowerSHA256(npmCLIScriptSHA256) {
		return "", errWindowsNodeToolchain
	}
	manifest, err := json.Marshal(struct {
		SchemaVersion        string `json:"schemaVersion"`
		NodeExecutableSHA256 string `json:"nodeExecutableSha256"`
		NPMCLIScriptSHA256   string `json:"npmCliScriptSha256"`
	}{WindowsNodeToolchainManifestSchema, nodeExecutableSHA256, npmCLIScriptSHA256})
	if err != nil {
		return "", errWindowsNodeToolchain
	}
	digest := sha256.Sum256(manifest)
	return hex.EncodeToString(digest[:]), nil
}

func ValidWindowsNodeToolchainSHA256(value string) bool {
	return ValidNodeToolchainSHA256(value)
}

func ValidNodeToolchainSHA256(value string) bool {
	return validLowerSHA256(value)
}

func validLowerSHA256(value string) bool {
	if len(value) != 64 || value != strings.ToLower(value) {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

// pattern: Imperative Shell
package environment

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const maxExecutorBinaryFingerprintBytes = 512 << 20

type EnvironmentExecutorFingerprint struct {
	ExecutorSHA256        string
	HostSHA256            string
	IsolationPolicySHA256 string
}

func CurrentEnvironmentExecutorFingerprint(profileID string) (EnvironmentExecutorFingerprint, error) {
	_, supported := IsolationProfileForNodeProfile(profileID)
	if !supported {
		return EnvironmentExecutorFingerprint{}, errors.New("node environment profile has no executor isolation policy")
	}

	executablePath, err := os.Executable()
	if err != nil {
		return EnvironmentExecutorFingerprint{}, errors.New("current executor binary path is unavailable")
	}
	executable, err := os.Open(executablePath)
	if err != nil {
		return EnvironmentExecutorFingerprint{}, errors.New("current executor binary cannot be opened")
	}
	defer executable.Close()
	info, err := executable.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() <= 0 || info.Size() > maxExecutorBinaryFingerprintBytes {
		return EnvironmentExecutorFingerprint{}, errors.New("current executor binary metadata is invalid")
	}
	executorHash := sha256.New()
	bytesRead, err := io.Copy(executorHash, io.LimitReader(executable, maxExecutorBinaryFingerprintBytes+1))
	if err != nil || bytesRead != info.Size() {
		return EnvironmentExecutorFingerprint{}, errors.New("current executor binary fingerprint could not be read completely")
	}

	hostIdentity, err := currentHostBuildIdentity()
	if err != nil || strings.TrimSpace(hostIdentity) == "" {
		return EnvironmentExecutorFingerprint{}, errors.New("current host build identity is unavailable")
	}
	policyDigest, _ := IsolationPolicyFingerprint(profileID)
	if profileID == WindowsNodeNPMProfile && runtime.GOOS == "windows" {
		policyDigest, err = CurrentWindowsNodeWorkspacePolicyFingerprint(os.Getenv("POLIS_WINDOWS_NODE_WORKSPACE_ROOT"))
		if err != nil {
			return EnvironmentExecutorFingerprint{}, err
		}
	}
	return EnvironmentExecutorFingerprint{
		ExecutorSHA256:        hex.EncodeToString(executorHash.Sum(nil)),
		HostSHA256:            fingerprintSHA256("polis-host-build@1", hostIdentity),
		IsolationPolicySHA256: policyDigest,
	}, nil
}

func CurrentWindowsNodeWorkspacePolicyFingerprint(workspaceRoot string) (string, error) {
	policyDigest, ok := IsolationPolicyFingerprint(WindowsNodeNPMProfile)
	if !ok {
		return "", errors.New("Windows Node workspace policy is unavailable")
	}
	executablePath, err := os.Executable()
	if err != nil || !filepath.IsAbs(executablePath) {
		return "", errors.New("current executor binary path is unavailable")
	}
	storage, err := InspectWindowsNodeWorkspaceStorage(workspaceRoot, filepath.Dir(executablePath), os.Getenv("SystemRoot"))
	if err != nil {
		return "", errors.New("current Windows workspace volume identity is unavailable")
	}
	fingerprint, err := WindowsWorkspaceStorageFingerprint(policyDigest, storage)
	if err != nil {
		return "", errors.New("current Windows workspace volume fingerprint is invalid")
	}
	return fingerprint, nil
}

func IsolationPolicyFingerprint(profileID string) (string, bool) {
	isolationProfile, supported := IsolationProfileForNodeProfile(profileID)
	if !supported {
		return "", false
	}
	policyRevision := ""
	switch profileID {
	case WindowsNodeNPMProfile:
		policyRevision = "windows-appcontainer-node-policy@5"
	case LinuxNodeNPMProfile:
		policyRevision = "linux-bwrap-node-policy@2"
	default:
		return "", false
	}
	return fingerprintSHA256("polis-executor-isolation-policy@1", strings.Join([]string{profileID, isolationProfile, policyRevision}, "\x00")), true
}

func validEnvironmentFingerprintDigest(value string) bool {
	if len(value) != 64 {
		return false
	}
	for _, character := range value {
		if !((character >= '0' && character <= '9') || (character >= 'a' && character <= 'f')) {
			return false
		}
	}
	return true
}

func fingerprintSHA256(domain, value string) string {
	encoded := sha256.Sum256([]byte(fmt.Sprintf("%s\x00%s", domain, value)))
	return hex.EncodeToString(encoded[:])
}

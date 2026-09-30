// pattern: Functional Core
package environment

import (
	"errors"
	"strconv"
	"strings"
)

const (
	WindowsNodeWorkspaceVolumeMinBytes   uint64 = 1 << 30
	WindowsNodeWorkspaceVolumeLimitBytes uint64 = 16 << 30
)

var ErrWindowsNodeWorkspaceStorage = errors.New("Windows Node workspace storage is not a dedicated bounded NTFS volume")

type WindowsNodeWorkspaceStorageInfo struct {
	WorkspaceRoot  string
	VolumeRoot     string
	VolumeGUID     string
	VolumeSerial   uint64
	VolumeLabel    string
	FileSystem     string
	TotalBytes     uint64
	AvailableBytes uint64
}

type WindowsNodeWorkspaceStorageConfig struct {
	WorkspaceRoot                 string
	ControlRoot                   string
	SystemRoot                    string
	ExpectedIsolationPolicySHA256 string
}

// ValidateWindowsNodeWorkspaceStorageInfo accepts only a bounded NTFS volume
// that is distinct from both the control process and Windows system volumes.
func ValidateWindowsNodeWorkspaceStorageInfo(info WindowsNodeWorkspaceStorageInfo, controlVolumeSerial, systemVolumeSerial uint64) error {
	if !isWindowsAbsolutePath(info.WorkspaceRoot) || !isWindowsAbsolutePath(info.VolumeRoot) || !validWindowsVolumeGUID(info.VolumeGUID) || !windowsPathDirectChild(info.VolumeRoot, info.WorkspaceRoot) {
		return ErrWindowsNodeWorkspaceStorage
	}
	if info.VolumeSerial == 0 || info.VolumeSerial == controlVolumeSerial || info.VolumeSerial == systemVolumeSerial {
		return ErrWindowsNodeWorkspaceStorage
	}
	if !strings.EqualFold(info.VolumeLabel, "POLIS_WORKSPACE") || !strings.EqualFold(info.FileSystem, "NTFS") || info.TotalBytes < WindowsNodeWorkspaceVolumeMinBytes || info.TotalBytes > WindowsNodeWorkspaceVolumeLimitBytes {
		return ErrWindowsNodeWorkspaceStorage
	}
	if info.AvailableBytes == 0 || info.AvailableBytes > info.TotalBytes {
		return ErrWindowsNodeWorkspaceStorage
	}
	return nil
}

func WindowsWorkspaceStorageFingerprint(policyDigest string, info WindowsNodeWorkspaceStorageInfo) (string, error) {
	if !validEnvironmentFingerprintDigest(policyDigest) || !isWindowsAbsolutePath(info.WorkspaceRoot) || !isWindowsAbsolutePath(info.VolumeRoot) || !validWindowsVolumeGUID(info.VolumeGUID) ||
		!windowsPathDirectChild(info.VolumeRoot, info.WorkspaceRoot) || info.VolumeSerial == 0 || !strings.EqualFold(info.VolumeLabel, "POLIS_WORKSPACE") ||
		!strings.EqualFold(info.FileSystem, "NTFS") || info.TotalBytes < WindowsNodeWorkspaceVolumeMinBytes || info.TotalBytes > WindowsNodeWorkspaceVolumeLimitBytes ||
		info.AvailableBytes == 0 || info.AvailableBytes > info.TotalBytes {
		return "", ErrWindowsNodeWorkspaceStorage
	}
	identity := strings.Join([]string{
		policyDigest,
		strings.ToLower(info.WorkspaceRoot),
		strings.ToLower(info.VolumeRoot),
		strings.ToLower(info.VolumeGUID),
		strconv.FormatUint(info.VolumeSerial, 10),
		strings.ToUpper(info.VolumeLabel),
		strings.ToUpper(info.FileSystem),
		strconv.FormatUint(info.TotalBytes, 10),
	}, "\x00")
	return fingerprintSHA256("polis-windows-workspace-storage-binding@1", identity), nil
}

func validWindowsVolumeGUID(value string) bool {
	const prefix = `\\?\Volume{`
	if !strings.HasPrefix(strings.ToLower(value), strings.ToLower(prefix)) || !strings.HasSuffix(value, `}\`) {
		return false
	}
	guid := value[len(prefix) : len(value)-2]
	if len(guid) != 36 {
		return false
	}
	for index, character := range guid {
		if index == 8 || index == 13 || index == 18 || index == 23 {
			if character != '-' {
				return false
			}
			continue
		}
		if !((character >= '0' && character <= '9') || (character >= 'a' && character <= 'f') || (character >= 'A' && character <= 'F')) {
			return false
		}
	}
	return true
}

func WindowsWorkspacePreparationFingerprintMatches(authorized, current string) bool {
	return validEnvironmentFingerprintDigest(authorized) && validEnvironmentFingerprintDigest(current) && authorized == current
}

func isWindowsAbsolutePath(value string) bool {
	return len(value) >= 3 && ((value[0] >= 'A' && value[0] <= 'Z') || (value[0] >= 'a' && value[0] <= 'z')) && value[1] == ':' && value[2] == '\\' && !strings.ContainsRune(value, '/')
}

func windowsPathDirectChild(root, target string) bool {
	root = strings.TrimRight(root, `\`)
	target = strings.TrimRight(target, `\`)
	if len(root) == 2 && root[1] == ':' {
		root += `\`
	}
	if len(target) <= len(root) || !strings.EqualFold(target[:len(root)], root) {
		return false
	}
	childName := target[len(root):]
	return childName != "" && !strings.ContainsRune(childName, '\\')
}

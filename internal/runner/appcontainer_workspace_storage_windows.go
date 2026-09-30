//go:build windows

// pattern: Imperative Shell
package runner

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	windowsAppContainerWorkspaceMinBytes      uint64 = 1 << 30
	windowsAppContainerWorkspaceLimitBytes    uint64 = 16 << 30
	windowsAppContainerWorkspaceLabel                = "POLIS_WORKSPACE"
	windowsAppContainerWorkspaceRights               = "0x001201BF"
	windowsAppContainerWorkspaceChildRights          = "0x001301BF"
	windowsAppContainerProfileWriteDenyRights        = "0x00010156"
	maxWindowsAppContainerProfileEntries             = 1024
)

type windowsWorkspaceVolume struct {
	root       string
	guidRoot   string
	serial     uint32
	label      string
	fileSystem string
	total      uint64
	available  uint64
}

func prepareWindowsBoundedAppContainerWorkspace(binding AppContainerWorkspaceStorageBinding, profileName string, packageSID *windows.SID) (string, error) {
	if binding.Root == "" || strings.TrimSpace(binding.Root) != binding.Root || !filepath.IsAbs(binding.Root) || !filepath.IsAbs(binding.ControlRoot) || !filepath.IsAbs(binding.SystemRoot) || profileName == "" || packageSID == nil {
		return "", errors.New("bounded AppContainer workspace binding is incomplete")
	}
	if err := requireWindowsNonLinkDirectory(binding.Root); err != nil {
		return "", err
	}
	workspaceVolume, err := inspectWindowsWorkspaceVolume(binding.Root)
	if err != nil {
		return "", err
	}
	controlVolume, err := inspectWindowsWorkspaceVolume(binding.ControlRoot)
	if err != nil {
		return "", errors.New("control process volume identity is unavailable")
	}
	systemVolume, err := inspectWindowsWorkspaceVolume(binding.SystemRoot)
	if err != nil {
		return "", errors.New("Windows system volume identity is unavailable")
	}
	if workspaceVolume.serial == 0 || workspaceVolume.serial == controlVolume.serial || workspaceVolume.serial == systemVolume.serial ||
		!strings.EqualFold(workspaceVolume.label, windowsAppContainerWorkspaceLabel) || !strings.EqualFold(workspaceVolume.fileSystem, "NTFS") ||
		workspaceVolume.total < windowsAppContainerWorkspaceMinBytes || workspaceVolume.total > windowsAppContainerWorkspaceLimitBytes ||
		workspaceVolume.available == 0 || workspaceVolume.available > workspaceVolume.total {
		return "", errors.New("Windows Node workspace volume does not meet the bounded NTFS storage policy")
	}
	volumeRelative, err := filepath.Rel(filepath.Clean(workspaceVolume.root), filepath.Clean(binding.Root))
	if err != nil || volumeRelative == "." || volumeRelative == ".." || strings.Contains(volumeRelative, string(filepath.Separator)) || filepath.IsAbs(volumeRelative) {
		return "", errors.New("Windows Node workspace root must be a direct child of its volume mount")
	}
	if binding.ExpectedVolumeRoot == "" || !strings.EqualFold(filepath.Clean(binding.ExpectedVolumeRoot), filepath.Clean(workspaceVolume.root)) ||
		!strings.EqualFold(binding.ExpectedVolumeGUID, workspaceVolume.guidRoot) ||
		binding.ExpectedVolumeSerial != uint64(workspaceVolume.serial) || !strings.EqualFold(binding.ExpectedVolumeLabel, workspaceVolume.label) ||
		!strings.EqualFold(binding.ExpectedFileSystem, workspaceVolume.fileSystem) || binding.ExpectedTotalBytes != workspaceVolume.total {
		return "", errors.New("Windows Node workspace volume changed after qualification")
	}
	workspaceRoot := strings.TrimRight(workspaceVolume.guidRoot, `\`) + `\` + volumeRelative + `\` + profileName
	if _, err = os.Lstat(workspaceRoot); err == nil {
		return "", errors.New("bounded AppContainer workspace path already exists")
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", errors.New("bounded AppContainer workspace path could not be inspected")
	}
	if err = os.Mkdir(workspaceRoot, 0700); err != nil {
		return "", errors.New("bounded AppContainer workspace could not be created")
	}
	if err = grantWindowsAppContainerWorkspaceAccess(workspaceRoot, packageSID); err != nil {
		cleanupErr := retryWorkspaceDirectoryCleanup(workspaceRoot, os.RemoveAll)
		if cleanupErr != nil {
			return workspaceRoot, errors.Join(err, cleanupErr)
		}
		return "", err
	}
	return workspaceRoot, nil
}

func cleanupWindowsManagedWorkspaceRoot(root string, managed bool) error {
	if !managed {
		return nil
	}
	return retryWorkspaceDirectoryCleanup(root, os.RemoveAll)
}

func requireWindowsNonLinkDirectory(path string) error {
	if _, err := filepath.Abs(path); err != nil {
		return errors.New("Windows workspace volume root must be absolute")
	}
	cleaned := filepath.Clean(path)
	info, err := os.Lstat(cleaned)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return errors.New("Windows workspace volume root must be an existing non-link directory")
	}
	return nil
}

func inspectWindowsWorkspaceVolume(path string) (windowsWorkspaceVolume, error) {
	pathPointer, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return windowsWorkspaceVolume{}, errors.New("Windows workspace volume path is invalid")
	}
	volumePath := make([]uint16, 32768)
	if err = windows.GetVolumePathName(pathPointer, &volumePath[0], uint32(len(volumePath))); err != nil {
		return windowsWorkspaceVolume{}, errors.New("Windows workspace volume mount path is unavailable")
	}
	root := windows.UTF16ToString(volumePath)
	rootPointer, err := windows.UTF16PtrFromString(root)
	if err != nil {
		return windowsWorkspaceVolume{}, errors.New("Windows workspace volume mount path is invalid")
	}
	volumeGUID := make([]uint16, 64)
	if err = windows.GetVolumeNameForVolumeMountPoint(rootPointer, &volumeGUID[0], uint32(len(volumeGUID))); err != nil {
		return windowsWorkspaceVolume{}, errors.New("Windows workspace volume GUID is unavailable")
	}
	volumeName := make([]uint16, 261)
	fileSystemName := make([]uint16, 261)
	var serial uint32
	if err = windows.GetVolumeInformation(rootPointer, &volumeName[0], uint32(len(volumeName)), &serial, nil, nil, &fileSystemName[0], uint32(len(fileSystemName))); err != nil {
		return windowsWorkspaceVolume{}, errors.New("Windows workspace volume identity is unavailable")
	}
	var available, total, totalFree uint64
	procedure := windows.NewLazySystemDLL("kernel32.dll").NewProc("GetDiskFreeSpaceExW")
	if err = procedure.Find(); err != nil {
		return windowsWorkspaceVolume{}, errors.New("Windows workspace volume capacity query is unavailable")
	}
	result, _, callErr := procedure.Call(uintptr(unsafe.Pointer(rootPointer)), uintptr(unsafe.Pointer(&available)), uintptr(unsafe.Pointer(&total)), uintptr(unsafe.Pointer(&totalFree)))
	if result == 0 {
		return windowsWorkspaceVolume{}, errors.Join(errors.New("Windows workspace volume capacity query failed"), callErr)
	}
	return windowsWorkspaceVolume{
		root: root, guidRoot: windows.UTF16ToString(volumeGUID), serial: serial, label: windows.UTF16ToString(volumeName), fileSystem: windows.UTF16ToString(fileSystemName),
		total: total, available: available,
	}, nil
}

func grantWindowsAppContainerWorkspaceAccess(root string, packageSID *windows.SID) error {
	token, err := windows.OpenCurrentProcessToken()
	if err != nil {
		return errors.New("Windows workspace owner identity is unavailable")
	}
	defer token.Close()
	user, err := token.GetTokenUser()
	if err != nil || user == nil || user.User.Sid == nil {
		return errors.New("Windows workspace owner SID is unavailable")
	}
	sddl := fmt.Sprintf("D:P(A;OICI;FA;;;SY)(A;OICI;FA;;;BA)(A;OICI;FA;;;%s)(A;;%s;;;%s)(A;OICIIO;%s;;;%s)",
		user.User.Sid.String(), windowsAppContainerWorkspaceRights, packageSID.String(), windowsAppContainerWorkspaceChildRights, packageSID.String())
	descriptor, err := windows.SecurityDescriptorFromString(sddl)
	if err != nil {
		return errors.New("Windows workspace ACL could not be constructed")
	}
	dacl, _, err := descriptor.DACL()
	if err != nil || dacl == nil {
		return errors.New("Windows workspace ACL is incomplete")
	}
	if err = windows.SetNamedSecurityInfo(root, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION, nil, nil, dacl, nil); err != nil {
		return errors.New("Windows workspace ACL could not be applied")
	}
	return nil
}

func restrictWindowsAppContainerDefaultProfileWrites(profileRoot string, packageSID *windows.SID) error {
	if profileRoot == "" || packageSID == nil {
		return errors.New("AppContainer default profile restriction input is invalid")
	}
	token, err := windows.OpenCurrentProcessToken()
	if err != nil {
		return errors.New("Windows workspace owner identity is unavailable")
	}
	defer token.Close()
	user, err := token.GetTokenUser()
	if err != nil || user == nil || user.User.Sid == nil {
		return errors.New("Windows workspace owner SID is unavailable")
	}
	sddl := fmt.Sprintf("D:P(D;OICI;%s;;;%s)(A;OICI;FRFX;;;%s)(A;OICI;FA;;;SY)(A;OICI;FA;;;BA)(A;OICI;FA;;;%s)",
		windowsAppContainerProfileWriteDenyRights, packageSID.String(), packageSID.String(), user.User.Sid.String())
	descriptor, err := windows.SecurityDescriptorFromString(sddl)
	if err != nil {
		return errors.New("AppContainer default profile ACL could not be constructed")
	}
	dacl, _, err := descriptor.DACL()
	if err != nil || dacl == nil {
		return errors.New("AppContainer default profile ACL is incomplete")
	}
	entries := 0
	return filepath.WalkDir(profileRoot, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		entries++
		if entries > maxWindowsAppContainerProfileEntries {
			return errors.New("AppContainer default profile contains too many entries")
		}
		info, statErr := os.Lstat(path)
		if statErr != nil || info.Mode()&os.ModeSymlink != 0 || !entry.IsDir() && !info.Mode().IsRegular() {
			return errors.New("AppContainer default profile contains an unsupported filesystem entry")
		}
		if aclErr := windows.SetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION, nil, nil, dacl, nil); aclErr != nil {
			return fmt.Errorf("AppContainer default profile ACL update failed at %q: %w", path, aclErr)
		}
		return nil
	})
}

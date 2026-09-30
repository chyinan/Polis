//go:build windows

// pattern: Imperative Shell
package environment

import (
	"errors"
	"os"
	"unsafe"

	"golang.org/x/sys/windows"
)

// InspectWindowsNodeWorkspaceStorage reads the actual volume backing a
// pre-provisioned workspace root. The caller must still bind this root to the
// AppContainer before any project files are materialized.
func InspectWindowsNodeWorkspaceStorage(workspaceRoot, controlRoot, systemRoot string) (WindowsNodeWorkspaceStorageInfo, error) {
	info, err := inspectWindowsVolume(workspaceRoot)
	if err != nil {
		return WindowsNodeWorkspaceStorageInfo{}, errors.Join(ErrWindowsNodeWorkspaceStorage, err)
	}
	control, err := inspectWindowsVolume(controlRoot)
	if err != nil {
		return WindowsNodeWorkspaceStorageInfo{}, errors.Join(ErrWindowsNodeWorkspaceStorage, err)
	}
	system, err := inspectWindowsVolume(systemRoot)
	if err != nil {
		return WindowsNodeWorkspaceStorageInfo{}, errors.Join(ErrWindowsNodeWorkspaceStorage, err)
	}
	info.WorkspaceRoot = workspaceRoot
	if err = ValidateWindowsNodeWorkspaceStorageInfo(info, control.VolumeSerial, system.VolumeSerial); err != nil {
		return WindowsNodeWorkspaceStorageInfo{}, err
	}
	return info, nil
}

func inspectWindowsVolume(path string) (WindowsNodeWorkspaceStorageInfo, error) {
	if path == "" {
		return WindowsNodeWorkspaceStorageInfo{}, errors.New("Windows volume path is empty")
	}
	if details, err := os.Lstat(path); err != nil || !details.IsDir() || details.Mode()&os.ModeSymlink != 0 {
		return WindowsNodeWorkspaceStorageInfo{}, errors.New("Windows volume probe path must be an existing non-link directory")
	}
	pathPointer, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return WindowsNodeWorkspaceStorageInfo{}, errors.New("Windows volume path is invalid")
	}
	volumePath := make([]uint16, 32768)
	if err = windows.GetVolumePathName(pathPointer, &volumePath[0], uint32(len(volumePath))); err != nil {
		return WindowsNodeWorkspaceStorageInfo{}, errors.New("Windows volume mount path is unavailable")
	}
	volumeRoot := windows.UTF16ToString(volumePath)
	volumePointer, err := windows.UTF16PtrFromString(volumeRoot)
	if err != nil {
		return WindowsNodeWorkspaceStorageInfo{}, errors.New("Windows volume mount path is invalid")
	}
	volumeGUID := make([]uint16, 64)
	if err = windows.GetVolumeNameForVolumeMountPoint(volumePointer, &volumeGUID[0], uint32(len(volumeGUID))); err != nil {
		return WindowsNodeWorkspaceStorageInfo{}, errors.New("Windows volume GUID is unavailable")
	}
	var serial uint32
	volumeName := make([]uint16, 261)
	fileSystem := make([]uint16, 261)
	if err = windows.GetVolumeInformation(volumePointer, &volumeName[0], uint32(len(volumeName)), &serial, nil, nil, &fileSystem[0], uint32(len(fileSystem))); err != nil {
		return WindowsNodeWorkspaceStorageInfo{}, errors.New("Windows volume filesystem identity is unavailable")
	}
	var available, total, totalFree uint64
	procedure := windows.NewLazySystemDLL("kernel32.dll").NewProc("GetDiskFreeSpaceExW")
	if err = procedure.Find(); err != nil {
		return WindowsNodeWorkspaceStorageInfo{}, errors.New("Windows volume capacity query is unavailable")
	}
	result, _, callErr := procedure.Call(
		uintptr(unsafe.Pointer(volumePointer)),
		uintptr(unsafe.Pointer(&available)),
		uintptr(unsafe.Pointer(&total)),
		uintptr(unsafe.Pointer(&totalFree)),
	)
	if result == 0 {
		return WindowsNodeWorkspaceStorageInfo{}, errors.Join(errors.New("Windows volume capacity query failed"), callErr)
	}
	return WindowsNodeWorkspaceStorageInfo{
		VolumeRoot: volumeRoot, VolumeGUID: windows.UTF16ToString(volumeGUID), VolumeSerial: uint64(serial), VolumeLabel: windows.UTF16ToString(volumeName), FileSystem: windows.UTF16ToString(fileSystem),
		TotalBytes: total, AvailableBytes: available,
	}, nil
}

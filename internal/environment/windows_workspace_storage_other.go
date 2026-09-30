//go:build !windows

// pattern: Imperative Shell
package environment

import "errors"

func InspectWindowsNodeWorkspaceStorage(string, string, string) (WindowsNodeWorkspaceStorageInfo, error) {
	return WindowsNodeWorkspaceStorageInfo{}, errors.Join(ErrWindowsNodeWorkspaceStorage, errors.New("Windows volume inspection is unavailable on this platform"))
}

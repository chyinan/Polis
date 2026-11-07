//go:build windows

// pattern: Functional Core

package kernel

import (
	"errors"
	"syscall"
)

func isParentDirectorySyncUnsupported(err error) bool {
	return errors.Is(err, syscall.ERROR_ACCESS_DENIED)
}

func parentDirectorySyncUnsupportedAllowed() bool { return true }

func parentDirectorySyncContract() string {
	return "strong_file_sync_atomic_rename_parent_directory_sync_unsupported_not_unix_equivalent"
}

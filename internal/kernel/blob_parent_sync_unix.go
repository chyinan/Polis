//go:build !windows

// pattern: Functional Core

package kernel

func isParentDirectorySyncUnsupported(error) bool { return false }

func parentDirectorySyncUnsupportedAllowed() bool { return false }

func parentDirectorySyncContract() string {
	return "strong_file_sync_atomic_rename_parent_directory_fsync"
}

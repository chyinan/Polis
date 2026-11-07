//go:build windows

// pattern: Imperative Shell

package kernel

import (
	"errors"
	"os"
	"testing"
)

func TestWindowsParentDirectorySyncIsPlatformQualified(t *testing.T) {
	root, err := os.OpenRoot(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	err = syncParentDirectory(root)
	if err != nil && !errors.Is(err, ErrParentDirectorySyncUnsupported) {
		t.Fatalf("unexpected Windows parent directory sync error: %v", err)
	}
	if parentDirectorySyncContract() != "strong_file_sync_atomic_rename_parent_directory_sync_unsupported_not_unix_equivalent" {
		t.Fatalf("Windows durability contract changed unexpectedly: %s", parentDirectorySyncContract())
	}
}

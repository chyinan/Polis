//go:build !windows

// pattern: Imperative Shell

package kernel

import (
	"errors"
	"os"
	"testing"
)

func TestUnixParentDirectorySyncUnsupportedSentinelRemainsFatal(t *testing.T) {
	root := openBlobTestRoot(t)
	content := []byte("unix-parent-sync-failure")
	err := finalizeBlob(root, ".stage-unix-parent", blobTestDigest(content), content, blobFinalizeOperations{
		syncFile:   func(*os.File) error { return nil },
		rename:     func(root *os.Root, from, to string) error { return root.Rename(from, to) },
		syncParent: func(*os.Root) error { return ErrParentDirectorySyncUnsupported },
	})
	if !errors.Is(err, ErrParentDirectorySyncUnsupported) {
		t.Fatalf("Unix parent-directory unsupported sentinel was ignored: %v", err)
	}
}

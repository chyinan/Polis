// pattern: Imperative Shell

package kernel

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"polis/internal/core"
	"testing"
)

func TestFinalizeBlobPropagatesFileSyncFailureAndCleansStage(t *testing.T) {
	root := openBlobTestRoot(t)
	content := []byte("file-sync-fixture")
	digest := blobTestDigest(content)
	fileSyncErr := errors.New("injected file sync failure")
	renameCalled := false
	err := finalizeBlob(root, ".stage-file-sync", digest, content, blobFinalizeOperations{
		syncFile:   func(*os.File) error { return fileSyncErr },
		rename:     func(*os.Root, string, string) error { renameCalled = true; return nil },
		syncParent: func(*os.Root) error { return nil },
	})
	if !errors.Is(err, fileSyncErr) {
		t.Fatalf("file sync failure was not propagated: %v", err)
	}
	if renameCalled {
		t.Fatal("rename ran after file sync failure")
	}
	assertBlobStageAbsent(t, root, ".stage-file-sync")
}

func TestFinalizeBlobPropagatesRenameFailureAndCleansStage(t *testing.T) {
	root := openBlobTestRoot(t)
	content := []byte("rename-fixture")
	digest := blobTestDigest(content)
	renameErr := errors.New("injected rename failure")
	err := finalizeBlob(root, ".stage-rename", digest, content, blobFinalizeOperations{
		syncFile:   func(*os.File) error { return nil },
		rename:     func(*os.Root, string, string) error { return renameErr },
		syncParent: func(*os.Root) error { return nil },
	})
	if !errors.Is(err, renameErr) {
		t.Fatalf("rename failure was not propagated: %v", err)
	}
	assertBlobStageAbsent(t, root, ".stage-rename")
}

func TestFinalizeBlobPropagatesParentDirectorySyncFailure(t *testing.T) {
	root := openBlobTestRoot(t)
	content := []byte("parent-sync-fixture")
	digest := blobTestDigest(content)
	parentSyncErr := errors.New("injected parent directory sync failure")
	err := finalizeBlob(root, ".stage-parent-sync", digest, content, blobFinalizeOperations{
		syncFile:   func(*os.File) error { return nil },
		rename:     func(root *os.Root, from, to string) error { return root.Rename(from, to) },
		syncParent: func(*os.Root) error { return parentSyncErr },
	})
	if !errors.Is(err, parentSyncErr) {
		t.Fatalf("parent directory sync failure was not propagated: %v", err)
	}
	assertBlobStageAbsent(t, root, ".stage-parent-sync")
	if _, err := root.Open(filepath.Base(digest)); err != nil {
		t.Fatalf("final blob was not atomically finalized before parent sync failure: %v", err)
	}
}

func TestParentDirectorySyncContractIsExplicit(t *testing.T) {
	if parentDirectorySyncContract() == "" {
		t.Fatal("parent directory sync contract is not documented by the platform implementation")
	}
}

func TestRunBlobDurabilitySmokeUsesRealFilesystemAndCleansOwnedTree(t *testing.T) {
	root := filepath.Join(t.TempDir(), "blob-smoke")
	report, err := RunBlobDurabilitySmoke(root)
	if err != nil {
		t.Fatal(err)
	}
	if report.Status != "passed" || !report.FinalBlobPresent || !report.ContentHashCorrect || report.StageFiles != 0 || report.Cleanup != "owned_company_directory_removed" {
		t.Fatalf("unexpected blob durability report: %+v", report)
	}
}

func TestBlobRootRejectsTraversalCompany(t *testing.T) {
	rootPath := t.TempDir()
	if _, err := blobDir(rootPath, "../outside"); !errors.Is(err, core.Malformed) {
		t.Fatalf("traversal company was not rejected: %v", err)
	}
	root, err := os.OpenRoot(rootPath)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	if _, err := root.Open("../outside"); err == nil {
		t.Fatal("os.Root allowed traversal outside its root")
	}
	if _, err := os.Stat(filepath.Join(rootPath, "..", "outside")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("traversal created or exposed an outside path: %v", err)
	}
}

func TestReadBlobBoundedRejectsOversizedCASBeforeRead(t *testing.T) {
	root := t.TempDir()
	companyID := "bounded-blob-company"
	content := []byte("six-bytes")
	digest, err := putBlob(root, companyID, content)
	if err != nil {
		t.Fatal(err)
	}
	got, err := readBlobBounded(root, companyID, digest, int64(len(content)))
	if err != nil || string(got) != string(content) {
		t.Fatalf("bounded CAS read=%q err=%v", got, err)
	}
	if _, err = readBlobBounded(root, companyID, digest, int64(len(content)-1)); !errors.Is(err, core.Integrity) {
		t.Fatalf("oversized CAS content error=%v, want integrity rejection", err)
	}
	if _, err = readBlobBounded(root, companyID, digest, 0); !errors.Is(err, core.Malformed) {
		t.Fatalf("invalid CAS size limit error=%v, want malformed input", err)
	}
}

func TestReadBlobBoundedAllowsAnExplicitLimitAboveLegacyContentSize(t *testing.T) {
	root := t.TempDir()
	companyID := "bounded-large-report-company"
	content := bytes.Repeat([]byte("r"), core.MaxContent+1)
	digest, err := putBlob(root, companyID, content)
	if err != nil {
		t.Fatal(err)
	}
	got, err := readBlobBounded(root, companyID, digest, 64<<10)
	if err != nil || !bytes.Equal(got, content) {
		t.Fatalf("explicit bounded CAS read length=%d err=%v, want %d bytes", len(got), err, len(content))
	}
}

func openBlobTestRoot(t *testing.T) *os.Root {
	t.Helper()
	root, err := os.OpenRoot(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { root.Close() })
	return root
}

func blobTestDigest(content []byte) string {
	digest := sha256.Sum256(content)
	return hex.EncodeToString(digest[:])
}

func assertBlobStageAbsent(t *testing.T, root *os.Root, name string) {
	t.Helper()
	if _, err := root.Open(name); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("stage file %q was not cleaned: %v", name, err)
	}
}

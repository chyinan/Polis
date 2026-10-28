package intake

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPrepareReadOnlyRootSnapshotCreatesBoundedPrivateDirectoryPackage(t *testing.T) {
	rootPath := t.TempDir()
	if err := os.MkdirAll(filepath.Join(rootPath, "src", "internal"), 0750); err != nil {
		t.Fatal(err)
	}
	for name, content := range map[string]string{
		"README.md":                     "# private workspace\n",
		filepath.Join("src", "main.go"): "package main\n",
		filepath.Join("src", "internal", "config.json"): "{\"mode\":\"review\"}\n",
	} {
		if err := os.WriteFile(filepath.Join(rootPath, name), []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}
	root, err := os.OpenRoot(rootPath)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()

	snapshot, err := PrepareReadOnlyRootSnapshot(root)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.RootName != readOnlyRootSnapshotName || snapshot.Upload.SourceKind != "directory_snapshot" || snapshot.Upload.State != StateUsable {
		t.Fatalf("read-only root snapshot metadata=%+v", snapshot.Upload)
	}
	files, err := ExtractVerifiedDirectoryFiles(snapshot.Archive)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 3 || !containsInputFileText(files, "workspace/README.md", "# private workspace") ||
		!containsInputFileText(files, "workspace/src/main.go", "package main") ||
		!containsInputFileText(files, "workspace/src/internal/config.json", "review") {
		t.Fatalf("root snapshot files=%+v", files)
	}
	for _, file := range files {
		if strings.Contains(file.RelativePath, rootPath) || filepath.IsAbs(file.RelativePath) {
			t.Fatalf("host path leaked into snapshot path %q", file.RelativePath)
		}
	}
	if err = VerifyPreparedUpload(snapshot.Upload, snapshot.Archive); err != nil {
		t.Fatalf("read-only root package did not canonically verify: %v", err)
	}
}

func TestPrepareReadOnlyRootSnapshotRejectsSymlinksAndMissingRootHandles(t *testing.T) {
	if _, err := PrepareReadOnlyRootSnapshot(nil); uploadReason(err) != "host_root_unavailable" {
		t.Fatalf("missing root error=%v, want host_root_unavailable", err)
	}
	parent := t.TempDir()
	rootPath := filepath.Join(parent, "root")
	if err := os.Mkdir(rootPath, 0750); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(parent, "outside.txt")
	if err := os.WriteFile(outside, []byte("outside"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(rootPath, "link.txt")); err != nil {
		t.Skipf("host does not permit symlink test setup: %v", err)
	}
	root, err := os.OpenRoot(rootPath)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	if _, err = PrepareReadOnlyRootSnapshot(root); uploadReason(err) != "host_root_link_unsupported" {
		t.Fatalf("symlink root entry error=%v, want host_root_link_unsupported", err)
	}
}

// pattern: Functional Core/Imperative Shell
package gotestprofile

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"polis/internal/fixture"
)

func TestMaterializeFixtureCopiesOnlyFrozenInputsReadOnly(t *testing.T) {
	content := []byte("package formatter\nfunc Render(v float64) string { return \"0\" }\n")
	files := []SnapshotFile{fixtureSnapshotFile("formatter.go", content)}
	if runtime.GOOS != "linux" {
		if _, err := MaterializeFixture(t.TempDir(), files); !errors.Is(err, ErrMaterializeFailed) {
			t.Fatalf("non-Linux host must refuse materialization, got %v", err)
		}
		return
	}
	parent, err := os.MkdirTemp("/tmp", "polis-go-profile-test-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(parent) })
	jobDirectory := filepath.Join(parent, "job-source")
	if err := os.Mkdir(jobDirectory, 0o700); err != nil {
		t.Fatal(err)
	}
	result, err := MaterializeFixture(jobDirectory, files)
	if err != nil {
		t.Fatal(err)
	}
	if result.CandidateFiles != 1 || result.CandidateBytes != int64(len(content)) || result.CandidateFilesSHA256 == "" || result.TrustedTestSHA256 != FixtureTestManifestSHA256() {
		t.Fatalf("unexpected materialization receipt: %+v", result)
	}
	for name, expected := range map[string][]byte{
		"formatter.go":   content,
		"go.mod":         []byte(fixtureGoMod),
		"frozen_test.go": nil, // checked below against the trusted manifest input
	} {
		got, readErr := os.ReadFile(filepath.Join(jobDirectory, name))
		if readErr != nil {
			t.Fatal(readErr)
		}
		if name == "frozen_test.go" {
			if len(got) == 0 {
				t.Fatal("trusted baseline tests were not materialized")
			}
		} else if string(got) != string(expected) {
			t.Fatalf("materialized %s changed", name)
		}
		info, statErr := os.Stat(filepath.Join(jobDirectory, name))
		if statErr != nil {
			t.Fatalf("materialized source %s cannot be stat'ed: %v", name, statErr)
		}
		if info.Mode().Perm()&0o222 != 0 {
			t.Fatalf("materialized source %s is writable: mode=%v", name, info.Mode())
		}
	}
	if info, statErr := os.Stat(jobDirectory); statErr != nil {
		t.Fatalf("materialized source directory cannot be stat'ed: %v", statErr)
	} else if info.Mode().Perm()&0o222 != 0 {
		t.Fatalf("materialized source directory is writable: mode=%v", info.Mode())
	}
	if _, statErr := os.Lstat(filepath.Join(jobDirectory, ".git")); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("unexpected .git path in job source tree: %v", statErr)
	}
}

func TestValidateFixtureSnapshotFilesReturnsBoundedDigest(t *testing.T) {
	files := []SnapshotFile{fixtureSnapshotFile("formatter.go", []byte("package formatter\n"))}
	digest, size, err := ValidateFixtureSnapshotFiles(files)
	if err != nil {
		t.Fatal(err)
	}
	if digest != digestSnapshotFiles(files) || size != int64(len(files[0].Content)) {
		t.Fatalf("snapshot validation returned digest/size %s/%d", digest, size)
	}
}

func TestMaterializeFixtureRejectsUnsafeOrTamperedSnapshots(t *testing.T) {
	base := fixtureSnapshotFile("formatter.go", []byte("package formatter\n"))
	for name, mutate := range map[string]func(*SnapshotFile){
		"wrong digest":          func(file *SnapshotFile) { file.Digest = string(make([]byte, 64)) },
		"symlink entry":         func(file *SnapshotFile) { file.EntryType = "symlink" },
		"alternate module file": func(file *SnapshotFile) { file.RelativePath = "go.mod" },
		"nested file":           func(file *SnapshotFile) { file.RelativePath = "nested_test.go" },
		"git pointer":           func(file *SnapshotFile) { file.RelativePath = ".git" },
	} {
		t.Run(name, func(t *testing.T) {
			candidate := base
			mutate(&candidate)
			if _, _, _, err := validateFixtureTree([]SnapshotFile{candidate}); !errors.Is(err, ErrInvalidFixtureTree) {
				t.Fatalf("unsafe snapshot was accepted: %v", err)
			}
		})
	}
	if _, _, _, err := validateFixtureTree([]SnapshotFile{base, base}); !errors.Is(err, ErrInvalidFixtureTree) {
		t.Fatalf("duplicate path was accepted: %v", err)
	}
}

func TestMaterializeFixtureAcceptsOnlyExactTrustedProtectedFiles(t *testing.T) {
	files := []SnapshotFile{
		fixtureSnapshotFile("formatter.go", []byte("package formatter\nfunc Render(v float64) string { return \"0\" }\n")),
		fixtureSnapshotFile("go.mod", []byte(fixtureGoMod)),
		fixtureSnapshotFile("frozen_test.go", []byte(fixture.Tests("full"))),
	}
	if _, _, _, err := validateFixtureTree(files); err != nil {
		t.Fatalf("exact frozen protected files were rejected: %v", err)
	}
	if runtime.GOOS == "linux" {
		parent, err := os.MkdirTemp("/tmp", "polis-go-protected-profile-test-")
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.RemoveAll(parent) })
		jobDirectory := filepath.Join(parent, "job-source")
		if err = os.Mkdir(jobDirectory, 0o700); err != nil {
			t.Fatal(err)
		}
		if _, err = MaterializeFixture(jobDirectory, files); err != nil {
			t.Fatalf("exact protected files could not be materialized: %v", err)
		}
		for name, expected := range map[string]string{"go.mod": fixtureGoMod, "frozen_test.go": fixture.Tests("full")} {
			actual, readErr := os.ReadFile(filepath.Join(jobDirectory, name))
			if readErr != nil || string(actual) != expected {
				t.Fatalf("trusted %s changed during materialization: %v", name, readErr)
			}
		}
	}
	for _, path := range []string{"go.mod", "frozen_test.go"} {
		candidate := append([]SnapshotFile(nil), files...)
		for index := range candidate {
			if candidate[index].RelativePath == path {
				candidate[index] = fixtureSnapshotFile(path, []byte("package altered\n"))
			}
		}
		if _, _, _, err := validateFixtureTree(candidate); !errors.Is(err, ErrInvalidFixtureTree) {
			t.Fatalf("modified protected file %s was accepted: %v", path, err)
		}
	}
}

func fixtureSnapshotFile(path string, content []byte) SnapshotFile {
	digest := sha256.Sum256(content)
	return SnapshotFile{
		RelativePath: path, Digest: hex.EncodeToString(digest[:]), Bytes: int64(len(content)),
		ContentType: "text/utf-8", EntryType: "regular", Content: append([]byte(nil), content...),
	}
}

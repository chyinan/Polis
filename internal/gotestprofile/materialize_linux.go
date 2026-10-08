//go:build linux

// pattern: Imperative Shell
package gotestprofile

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"syscall"

	"polis/internal/fixture"
)

// MaterializeFixture copies verified CAS contents into an already-created,
// empty, private job-owned directory. The caller must create that directory
// inside the quota-controlled scratch mount and must not execute anything
// until host resource controls are attached to the complete job cgroup.
func MaterializeFixture(directory string, files []SnapshotFile) (MaterializedFixture, error) {
	byPath, candidateDigest, candidateBytes, err := validateFixtureTree(files)
	if err != nil {
		return MaterializedFixture{}, err
	}
	absolute, err := filepath.Abs(directory)
	if err != nil {
		return MaterializedFixture{}, fmt.Errorf("%w: resolve job directory: %v", ErrMaterializeFailed, err)
	}
	info, err := os.Lstat(absolute)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm()&0o077 != 0 {
		mode := os.FileMode(0)
		if info != nil {
			mode = info.Mode()
		}
		return MaterializedFixture{}, fmt.Errorf("%w: job directory must be a private real directory (mode=%v): %v", ErrMaterializeFailed, mode, err)
	}
	rootFD, err := syscall.Open(absolute, syscall.O_RDONLY|syscall.O_DIRECTORY|syscall.O_NOFOLLOW|syscall.O_CLOEXEC, 0)
	if err != nil {
		return MaterializedFixture{}, fmt.Errorf("%w: open job directory: %v", ErrMaterializeFailed, err)
	}
	root := os.NewFile(uintptr(rootFD), absolute)
	if root == nil {
		_ = syscall.Close(rootFD)
		return MaterializedFixture{}, fmt.Errorf("%w: wrap job directory", ErrMaterializeFailed)
	}
	defer root.Close()
	entries, err := root.ReadDir(1)
	if err != nil && !errors.Is(err, io.EOF) {
		return MaterializedFixture{}, fmt.Errorf("%w: inspect job directory: %v", ErrMaterializeFailed, err)
	}
	if len(entries) != 0 {
		return MaterializedFixture{}, fmt.Errorf("%w: job directory is not empty", ErrMaterializeFailed)
	}
	// Rewind the directory descriptor after the emptiness check. ReadDir may
	// advance the shared directory offset used by subsequent operations.
	if _, err = syscall.Seek(rootFD, 0, 0); err != nil {
		return MaterializedFixture{}, fmt.Errorf("%w: reset job directory cursor: %v", ErrMaterializeFailed, err)
	}
	trusted := map[string][]byte{
		"go.mod":         []byte(fixtureGoMod),
		"frozen_test.go": []byte(fixture.Tests("full")),
	}
	for path, content := range trusted {
		if err = writeFixtureFile(rootFD, path, content); err != nil {
			return MaterializedFixture{}, err
		}
	}
	for path, file := range byPath {
		if path == "go.mod" || path == "frozen_test.go" {
			continue
		}
		if err = writeFixtureFile(rootFD, path, file.Content); err != nil {
			return MaterializedFixture{}, err
		}
	}
	if err = syscall.Fsync(rootFD); err != nil {
		return MaterializedFixture{}, fmt.Errorf("%w: flush job directory: %v", ErrMaterializeFailed, err)
	}
	if err = syscall.Fchmod(rootFD, 0o555); err != nil {
		return MaterializedFixture{}, fmt.Errorf("%w: make job source read-only: %v", ErrMaterializeFailed, err)
	}
	return MaterializedFixture{
		Directory: absolute, CandidateFilesSHA256: candidateDigest, CandidateFiles: len(files),
		CandidateBytes: candidateBytes, TrustedTestSHA256: FixtureTestManifestSHA256(),
	}, nil
}

func writeFixtureFile(rootFD int, name string, content []byte) error {
	fd, err := syscall.Openat(rootFD, name, syscall.O_WRONLY|syscall.O_CREAT|syscall.O_EXCL|syscall.O_NOFOLLOW|syscall.O_CLOEXEC, 0o444)
	if err != nil {
		return fmt.Errorf("%w: create fixture input: %v", ErrMaterializeFailed, err)
	}
	file := os.NewFile(uintptr(fd), name)
	if file == nil {
		_ = syscall.Close(fd)
		return fmt.Errorf("%w: wrap fixture input", ErrMaterializeFailed)
	}
	if _, err = file.Write(content); err != nil {
		_ = file.Close()
		return fmt.Errorf("%w: write fixture input: %v", ErrMaterializeFailed, err)
	}
	if err = file.Sync(); err != nil {
		_ = file.Close()
		return fmt.Errorf("%w: flush fixture input: %v", ErrMaterializeFailed, err)
	}
	if err = file.Close(); err != nil {
		return fmt.Errorf("%w: close fixture input: %v", ErrMaterializeFailed, err)
	}
	return nil
}

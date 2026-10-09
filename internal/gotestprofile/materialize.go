// pattern: Functional Core/Imperative Shell
package gotestprofile

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"polis/internal/fixture"
	"polis/internal/workspacepath"
)

var (
	ErrInvalidFixtureTree = errors.New("invalid candidate tree for the fixed Go fixture")
	ErrMaterializeFailed  = errors.New("could not materialize the fixed Go fixture")
)

const (
	maxFixtureFiles      = 512
	maxFixtureFileBytes  = 2 << 20
	maxFixtureTotalBytes = 16 << 20
)

// SnapshotFile is an immutable control-plane reference to one CAS object.
// Content must come from the digest-addressed CAS reader, never from a path
// supplied by the candidate or the source repository.
type SnapshotFile struct {
	RelativePath string
	Digest       string
	Bytes        int64
	ContentType  string
	EntryType    string
	Content      []byte
}

// MaterializedFixture describes only a data directory. It is not evidence
// that the Go toolchain or candidate code was executed.
type MaterializedFixture struct {
	Directory            string
	CandidateFilesSHA256 string
	CandidateFiles       int
	CandidateBytes       int64
	TrustedTestSHA256    string
}

func ValidateFixtureSnapshotFiles(files []SnapshotFile) (string, int64, error) {
	_, digest, totalBytes, err := validateFixtureTree(files)
	return digest, totalBytes, err
}

func validateFixtureTree(files []SnapshotFile) (map[string]SnapshotFile, string, int64, error) {
	if len(files) == 0 || len(files) > maxFixtureFiles {
		return nil, "", 0, ErrInvalidFixtureTree
	}
	byPath := make(map[string]SnapshotFile, len(files))
	var total int64
	for _, file := range files {
		path := file.RelativePath
		if !validateFixtureSourcePolicy(path, file.Content) {
			return nil, "", 0, ErrInvalidFixtureTree
		}
		if !workspacepath.ValidRelativePath(path) || strings.Contains(path, "\\") || strings.HasPrefix(path, ".") ||
			strings.Contains(path, "/.git") || path == ".git" || file.EntryType != "regular" || file.ContentType != "text/utf-8" ||
			file.Bytes <= 0 || file.Bytes > maxFixtureFileBytes || int64(len(file.Content)) != file.Bytes || !utf8.Valid(file.Content) || !validDigest(file.Digest) || digestHex(file.Content) != file.Digest {
			return nil, "", 0, ErrInvalidFixtureTree
		}
		if _, exists := byPath[path]; exists {
			return nil, "", 0, ErrInvalidFixtureTree
		}
		switch path {
		case "formatter.go":
		case "go.mod":
			if !bytes.Equal(file.Content, []byte(fixtureGoMod)) {
				return nil, "", 0, ErrInvalidFixtureTree
			}
		case "frozen_test.go":
			if !bytes.Equal(file.Content, []byte(fixture.Tests("full"))) {
				return nil, "", 0, ErrInvalidFixtureTree
			}
		default:
			if !strings.HasSuffix(path, "_test.go") || strings.Contains(path, "/") {
				return nil, "", 0, ErrInvalidFixtureTree
			}
		}
		total += file.Bytes
		if total > maxFixtureTotalBytes {
			return nil, "", 0, ErrInvalidFixtureTree
		}
		byPath[path] = file
	}
	if _, exists := byPath["formatter.go"]; !exists {
		return nil, "", 0, ErrInvalidFixtureTree
	}
	return byPath, digestSnapshotFiles(files), total, nil
}

func validDigest(value string) bool {
	if len(value) != sha256.Size*2 || strings.ToLower(value) != value {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func digestHex(content []byte) string {
	digest := sha256.Sum256(content)
	return hex.EncodeToString(digest[:])
}

func digestSnapshotFiles(files []SnapshotFile) string {
	ordered := append([]SnapshotFile(nil), files...)
	for i := 1; i < len(ordered); i++ {
		for j := i; j > 0 && ordered[j].RelativePath < ordered[j-1].RelativePath; j-- {
			ordered[j], ordered[j-1] = ordered[j-1], ordered[j]
		}
	}
	var canonical strings.Builder
	for _, file := range ordered {
		fmt.Fprintf(&canonical, "%s\x00%s\x00%d\n", file.RelativePath, file.Digest, file.Bytes)
	}
	return digestHex([]byte(canonical.String()))
}

// InventoryDigest shares the immutable path/digest/size identity with patch
// delivery. Callers still validate file bytes and their own snapshot policy.
func InventoryDigest(files []SnapshotFile) string { return digestSnapshotFiles(files) }

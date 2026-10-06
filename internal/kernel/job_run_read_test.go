// pattern: Imperative Shell
package kernel

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"polis/internal/core"
)

func TestReadVerifiedJobRunLogArtifactAllowsThePersistedLogBound(t *testing.T) {
	root := t.TempDir()
	companyID := "job-log-company"
	content := bytes.Repeat([]byte("l"), 8192)
	digest, err := putBlob(root, companyID, content)
	if err != nil {
		t.Fatal(err)
	}
	artifact, err := readVerifiedJobRunLogArtifact(root, companyID, "job-1", digest)
	if err != nil {
		t.Fatalf("large JobRun log read error: %v", err)
	}
	if artifact.CompanyID != companyID || artifact.JobID != "job-1" || artifact.ManifestSHA256 != digest || !bytes.Equal(artifact.Content, content) {
		t.Fatalf("large JobRun log artifact=%+v, want digest-bound content", artifact)
	}
}

func TestReadVerifiedJobRunLogArtifactRejectsMissingOrTamperedManifest(t *testing.T) {
	root := t.TempDir()
	companyID := "job-log-integrity-company"
	missingDigest := strings.Repeat("a", 64)
	if _, err := readVerifiedJobRunLogArtifact(root, companyID, "job-1", missingDigest); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("missing JobRun log error=%v, want not-exist", err)
	}
	content := []byte("original log")
	digest, err := putBlob(root, companyID, content)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.Chmod(filepath.Join(root, companyID, digest), 0600); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(root, companyID, digest), []byte("tampered log"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = readVerifiedJobRunLogArtifact(root, companyID, "job-1", digest); !errors.Is(err, core.Integrity) {
		t.Fatalf("tampered JobRun log error=%v, want integrity", err)
	}
}

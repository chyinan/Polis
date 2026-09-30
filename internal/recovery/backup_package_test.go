// pattern: Imperative Shell
package recovery

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestVerifyRecoveryBackupPackageBindsDumpAndCompleteCASInventory(t *testing.T) {
	root := makeRecoveryBackupFixture(t)
	report, err := VerifyRecoveryBackupPackage(root)
	if err != nil || report.Status != "PASSED" || report.VerifiedBlobCount != 1 || report.DatabaseName != "polis_source" || report.SchemaVersion != 40 {
		t.Fatalf("verification=%+v err=%v", report, err)
	}
	if err = os.WriteFile(filepath.Join(root, "cas", "company-1", digestBytes([]byte("unmanifested"))), []byte("unmanifested"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err = VerifyRecoveryBackupPackage(root); err == nil {
		t.Fatal("unmanifested CAS file was accepted")
	}
}

func TestVerifyRecoveryBackupPackageRejectsTamperedContentAndCredentialClaims(t *testing.T) {
	root := makeRecoveryBackupFixture(t)
	if err := os.WriteFile(filepath.Join(root, RecoveryBackupDatabaseName), []byte("changed"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := VerifyRecoveryBackupPackage(root); err == nil {
		t.Fatal("modified database dump was accepted")
	}

	root = makeRecoveryBackupFixture(t)
	manifestPath := filepath.Join(root, RecoveryBackupManifestName)
	var manifest RecoveryBackupManifest
	raw, err := os.ReadFile(manifestPath)
	if err != nil || json.Unmarshal(raw, &manifest) != nil {
		t.Fatalf("read test manifest: %v", err)
	}
	manifest.CredentialsIncluded = true
	raw, err = json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(manifestPath, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := VerifyRecoveryBackupPackage(root); err == nil {
		t.Fatal("package claiming included credentials was accepted")
	}
}

func TestRecoveryBackupManifestKeepsCASRowsInSeparateHashedManifest(t *testing.T) {
	manifest := RecoveryBackupManifest{
		SchemaVersion: RecoveryBackupPackageSchema, Status: "COMPLETE", GenerationID: stringsRepeat("a", 32), CreatedAt: time.Now().UTC().Format(time.RFC3339Nano),
		Database: RecoveryBackupDatabase{FileName: RecoveryBackupDatabaseName, ByteSize: 1, SHA256: stringsRepeat("b", 64), DatabaseName: "db", DatabaseOwner: "backup-owner", RuntimeRoleName: "polis_runtime", PostgresMajor: 18, SchemaVersion: 40, Extensions: []RecoveryBackupExtension{}},
		CAS:      RecoveryBackupCAS{ManifestFileName: RecoveryBackupCASName, ManifestSHA256: stringsRepeat("c", 64), BlobCount: 2, TotalBytes: 2},
	}
	if err := ValidateRecoveryBackupManifest(manifest); err != nil {
		t.Fatalf("empty declaration validation failed: %v", err)
	}
}

func makeRecoveryBackupFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "cas", "company-1"), 0o700); err != nil {
		t.Fatal(err)
	}
	dump := []byte("postgres custom-format fixture")
	if err := os.WriteFile(filepath.Join(root, RecoveryBackupDatabaseName), dump, 0o600); err != nil {
		t.Fatal(err)
	}
	blob := []byte("mission input bytes")
	blobDigest := digestBytes(blob)
	if err := os.WriteFile(filepath.Join(root, "cas", "company-1", blobDigest), blob, 0o600); err != nil {
		t.Fatal(err)
	}
	entries := []RecoveryBackupCASEntry{{CompanyID: "company-1", SHA256: blobDigest, ByteSize: int64(len(blob))}}
	casRaw, err := json.Marshal(struct {
		SchemaVersion string                   `json:"schemaVersion"`
		Entries       []RecoveryBackupCASEntry `json:"entries"`
	}{RecoveryBackupPackageSchema, entries})
	if err != nil {
		t.Fatal(err)
	}
	casDigest := sha256.Sum256(casRaw)
	if err = os.WriteFile(filepath.Join(root, RecoveryBackupCASName), casRaw, 0o600); err != nil {
		t.Fatal(err)
	}
	dumpDigest := sha256.Sum256(dump)
	manifest := RecoveryBackupManifest{
		SchemaVersion: RecoveryBackupPackageSchema, Status: "COMPLETE", GenerationID: stringsRepeat("a", 32), CreatedAt: time.Now().UTC().Format(time.RFC3339Nano),
		Database: RecoveryBackupDatabase{FileName: RecoveryBackupDatabaseName, ByteSize: int64(len(dump)), SHA256: hex.EncodeToString(dumpDigest[:]), DatabaseName: "polis_source", DatabaseOwner: "backup-owner", RuntimeRoleName: "polis_runtime", PostgresMajor: 18, SchemaVersion: 40, Extensions: []RecoveryBackupExtension{}},
		CAS:      RecoveryBackupCAS{ManifestFileName: RecoveryBackupCASName, ManifestSHA256: hex.EncodeToString(casDigest[:]), BlobCount: len(entries), TotalBytes: int64(len(blob))},
	}
	manifestRaw, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	manifestDigest := sha256.Sum256(manifestRaw)
	if err = os.WriteFile(filepath.Join(root, RecoveryBackupManifestName), manifestRaw, 0o600); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(root, RecoveryBackupCompleteName), []byte(hex.EncodeToString(manifestDigest[:])+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return root
}

func digestBytes(content []byte) string {
	digest := sha256.Sum256(content)
	return hex.EncodeToString(digest[:])
}

func stringsRepeat(value string, count int) string {
	result := ""
	for range count {
		result += value
	}
	return result
}

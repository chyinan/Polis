package workbench

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"testing"
)

func TestBuildArtifactDeliveryPackageBindsArtifactAndQualification(t *testing.T) {
	content := []byte("qualified artifact bytes")
	contentHash := sha256.Sum256(content)
	manifest := ArtifactDeliveryManifestView{
		SchemaVersion: ArtifactDeliveryManifestSchema,
		CompanyID:     "company-1",
		ArtifactID:    "artifact-1",
		TaskID:        "task-1",
		Content:       ArtifactDeliveryContent{FileName: "artifact.bin", ContentType: "application/octet-stream", ByteSize: "24", SHA256: hex.EncodeToString(contentHash[:])},
		State:         "ready",
		Verdict:       "passed",
		Qualification: ArtifactDeliveryQualification{
			CheckpointID: "checkpoint-1", ValidationReceiptID: "receipt-1",
			TaskValidationBindingDigest: string(repeatByte('a', 64)), WorkspaceDigest: string(repeatByte('b', 64)),
			WorkspaceRevision: "7", RunnerRevision: "runner-test@1",
		},
		CreatedAt: "2026-09-24T00:00:00Z",
	}
	packaged, err := buildArtifactDeliveryPackage(manifest, content)
	if err != nil {
		t.Fatal(err)
	}
	packageHash := sha256.Sum256(packaged.Archive)
	if packaged.PackageSHA256 != hex.EncodeToString(packageHash[:]) {
		t.Fatal("package digest does not bind the emitted ZIP bytes")
	}
	archive, err := zip.NewReader(bytes.NewReader(packaged.Archive), int64(len(packaged.Archive)))
	if err != nil {
		t.Fatal(err)
	}
	if len(archive.File) != 3 {
		t.Fatalf("delivery archive entries = %d, want 3", len(archive.File))
	}
	entries := map[string][]byte{}
	for _, entry := range archive.File {
		if entry.Flags&0x0008 != 0 {
			t.Fatalf("delivery archive entry %q requires a data descriptor unsupported by the browser verifier", entry.Name)
		}
		reader, openErr := entry.Open()
		if openErr != nil {
			t.Fatal(openErr)
		}
		data, readErr := io.ReadAll(reader)
		_ = reader.Close()
		if readErr != nil {
			t.Fatal(readErr)
		}
		entries[entry.Name] = data
	}
	if !bytes.Equal(entries["artifact.bin"], content) || len(entries["manifest.json"]) == 0 {
		t.Fatal("delivery ZIP does not contain its bound artifact and manifest")
	}
	wantSums := packaged.ManifestSHA256 + "  manifest.json\n" + manifest.Content.SHA256 + "  artifact.bin\n"
	if string(entries["SHA256SUMS"]) != wantSums {
		t.Fatalf("checksum file = %q, want %q", entries["SHA256SUMS"], wantSums)
	}
}

func TestBuildArtifactDeliveryPackageRejectsContentDrift(t *testing.T) {
	content := []byte("artifact")
	hash := sha256.Sum256(content)
	manifest := ArtifactDeliveryManifestView{
		SchemaVersion: ArtifactDeliveryManifestSchema, CompanyID: "company-1", ArtifactID: "artifact-1", TaskID: "task-1",
		Content: ArtifactDeliveryContent{FileName: "artifact.bin", ContentType: "application/octet-stream", ByteSize: "8", SHA256: hex.EncodeToString(hash[:])},
		State:   "ready", Verdict: "passed",
		Qualification: ArtifactDeliveryQualification{CheckpointID: "checkpoint-1", ValidationReceiptID: "receipt-1", TaskValidationBindingDigest: string(repeatByte('a', 64)), WorkspaceDigest: string(repeatByte('b', 64)), WorkspaceRevision: "1", RunnerRevision: "runner-test@1"},
		CreatedAt:     "2026-09-24T00:00:00Z",
	}
	if _, err := buildArtifactDeliveryPackage(manifest, []byte("changed!")); err == nil {
		t.Fatal("artifact content drift was accepted")
	}
}

func repeatByte(value byte, count int) []byte {
	result := make([]byte, count)
	for index := range result {
		result[index] = value
	}
	return result
}

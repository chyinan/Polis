// pattern: Functional Core
package workbench

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"hash/crc32"
	"strconv"
	"time"
)

const ArtifactDeliveryManifestSchema = "polis-delivery-manifest@1"

type ArtifactDeliveryQualification struct {
	CheckpointID                string `json:"checkpointId"`
	ValidationReceiptID         string `json:"validationReceiptId"`
	TaskValidationBindingDigest string `json:"taskValidationBindingDigest"`
	WorkspaceDigest             string `json:"workspaceDigest"`
	WorkspaceRevision           string `json:"workspaceRevision"`
	RunnerRevision              string `json:"runnerRevision"`
}

type ArtifactDeliveryContent struct {
	FileName    string `json:"fileName"`
	ContentType string `json:"contentType"`
	ByteSize    string `json:"byteSize"`
	SHA256      string `json:"sha256"`
}

type ArtifactDeliveryManifestView struct {
	SchemaVersion string                        `json:"schemaVersion"`
	CompanyID     string                        `json:"companyId"`
	ArtifactID    string                        `json:"artifactId"`
	TaskID        string                        `json:"taskId"`
	Content       ArtifactDeliveryContent       `json:"content"`
	State         string                        `json:"state"`
	Verdict       string                        `json:"verdict"`
	Qualification ArtifactDeliveryQualification `json:"qualification"`
	CreatedAt     string                        `json:"createdAt"`
}

type ArtifactDeliveryManifestResponse struct {
	Manifest       ArtifactDeliveryManifestView `json:"manifest"`
	ManifestSHA256 string                       `json:"manifestSha256"`
}

type ArtifactDeliveryPackage struct {
	Manifest       ArtifactDeliveryManifestView
	ManifestSHA256 string
	PackageSHA256  string
	Archive        []byte
}

func canonicalArtifactDeliveryManifest(manifest ArtifactDeliveryManifestView) ([]byte, string, error) {
	byteSize, byteSizeErr := strconv.ParseInt(manifest.Content.ByteSize, 10, 64)
	workspaceRevision, revisionErr := strconv.ParseInt(manifest.Qualification.WorkspaceRevision, 10, 64)
	_, timestampErr := time.Parse(time.RFC3339Nano, manifest.CreatedAt)
	if manifest.SchemaVersion != ArtifactDeliveryManifestSchema || manifest.CompanyID == "" || manifest.ArtifactID == "" || manifest.TaskID == "" || manifest.Content.FileName != "artifact.bin" || manifest.Content.ContentType != "application/octet-stream" || byteSizeErr != nil || byteSize <= 0 || manifest.State != "ready" || !validArtifactVerdict(manifest.Verdict) || !validDeliverySHA256(manifest.Content.SHA256) || manifest.Qualification.CheckpointID == "" || manifest.Qualification.ValidationReceiptID == "" || !validDeliverySHA256(manifest.Qualification.TaskValidationBindingDigest) || !validDeliverySHA256(manifest.Qualification.WorkspaceDigest) || revisionErr != nil || workspaceRevision <= 0 || manifest.Qualification.RunnerRevision == "" || timestampErr != nil {
		return nil, "", errors.New("delivery manifest is incomplete or invalid")
	}
	encoded, err := json.Marshal(manifest)
	if err != nil {
		return nil, "", err
	}
	hash := sha256.Sum256(encoded)
	return encoded, hex.EncodeToString(hash[:]), nil
}

func validArtifactVerdict(value string) bool {
	switch value {
	case "candidate", "passed", "failed", "invalidated":
		return true
	default:
		return false
	}
}

func buildArtifactDeliveryPackage(manifest ArtifactDeliveryManifestView, content []byte) (ArtifactDeliveryPackage, error) {
	encodedManifest, manifestDigest, err := canonicalArtifactDeliveryManifest(manifest)
	if err != nil {
		return ArtifactDeliveryPackage{}, err
	}
	if strconv.FormatInt(int64(len(content)), 10) != manifest.Content.ByteSize {
		return ArtifactDeliveryPackage{}, errors.New("artifact content size does not match its delivery manifest")
	}
	contentHash := sha256.Sum256(content)
	if hex.EncodeToString(contentHash[:]) != manifest.Content.SHA256 {
		return ArtifactDeliveryPackage{}, errors.New("artifact content digest does not match its delivery manifest")
	}
	var archive bytes.Buffer
	writer := zip.NewWriter(&archive)
	writeEntry := func(name string, data []byte) error {
		header := &zip.FileHeader{Name: name, Method: zip.Store, UncompressedSize64: uint64(len(data)), CompressedSize64: uint64(len(data))}
		header.Flags = 0
		header.CRC32 = crc32.ChecksumIEEE(data)
		header.SetModTime(time.Date(1980, 1, 1, 0, 0, 0, 0, time.UTC))
		header.SetMode(0o644)
		entry, createErr := writer.CreateRaw(header)
		if createErr != nil {
			return createErr
		}
		_, writeErr := entry.Write(data)
		return writeErr
	}
	if err = writeEntry("artifact.bin", content); err == nil {
		err = writeEntry("manifest.json", encodedManifest)
	}
	if err == nil {
		checksumFile := fmt.Sprintf("%s  manifest.json\n%s  artifact.bin\n", manifestDigest, manifest.Content.SHA256)
		err = writeEntry("SHA256SUMS", []byte(checksumFile))
	}
	if closeErr := writer.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return ArtifactDeliveryPackage{}, err
	}
	archiveBytes := archive.Bytes()
	packageHash := sha256.Sum256(archiveBytes)
	return ArtifactDeliveryPackage{Manifest: manifest, ManifestSHA256: manifestDigest, PackageSHA256: hex.EncodeToString(packageHash[:]), Archive: archiveBytes}, nil
}

func validDeliverySHA256(value string) bool {
	if len(value) != 64 {
		return false
	}
	decoded, err := hex.DecodeString(value)
	return err == nil && hex.EncodeToString(decoded) == value
}

// pattern: Functional Core
package intake

import (
	"crypto/sha256"
	"encoding/hex"
	"path"
	"strings"
)

type DomainEvidencePreview struct {
	FileName      string
	MediaType     string
	Content       []byte
	ContentSHA256 string
}

type DomainEvidencePreviewEntry struct {
	RelativePath  string
	FileName      string
	MediaType     string
	ByteSize      int64
	ContentSHA256 string
	Previewable   bool
	ReasonCode    string
}

func ListDomainEvidencePreviewEntries(sourceKind, mediaType, displayName string, content []byte) ([]DomainEvidencePreviewEntry, error) {
	files, err := domainEvidencePreviewSourceFiles(sourceKind, mediaType, displayName, content)
	if err != nil {
		return nil, err
	}
	entries := make([]DomainEvidencePreviewEntry, 0, len(files))
	for _, file := range files {
		previewMediaType := domainEvidencePreviewMemberMediaType(sourceKind, file.RelativePath, file.MediaType)
		entry := DomainEvidencePreviewEntry{
			RelativePath:  file.RelativePath,
			FileName:      path.Base(file.RelativePath),
			MediaType:     previewMediaType,
			ByteSize:      int64(len(file.Content)),
			ContentSHA256: evidencePreviewDigest(file.Content),
		}
		if !domainEvidencePreviewMetadataOnly(file.RelativePath) {
			if preview, previewErr := prepareDomainEvidencePreviewContent(file.RelativePath, previewMediaType, file.Content); previewErr == nil {
				entry.FileName = preview.FileName
				entry.ContentSHA256 = preview.ContentSHA256
				entry.Previewable = true
				entries = append(entries, entry)
				continue
			}
		}
		entry.ReasonCode = "evidence_preview_unsupported"
		entries = append(entries, entry)
	}
	return entries, nil
}

func PrepareDomainEvidencePreview(sourceKind, mediaType, displayName string, content []byte, relativePath string) (DomainEvidencePreview, error) {
	files, err := domainEvidencePreviewSourceFiles(sourceKind, mediaType, displayName, content)
	if err != nil {
		return DomainEvidencePreview{}, err
	}
	for _, file := range files {
		if file.RelativePath != relativePath {
			continue
		}
		if domainEvidencePreviewMetadataOnly(file.RelativePath) {
			return DomainEvidencePreview{}, &UploadError{ReasonCode: "evidence_preview_unsupported"}
		}
		previewMediaType := domainEvidencePreviewMemberMediaType(sourceKind, file.RelativePath, file.MediaType)
		return prepareDomainEvidencePreviewContent(file.RelativePath, previewMediaType, file.Content)
	}
	return DomainEvidencePreview{}, &UploadError{ReasonCode: "evidence_preview_unsupported"}
}

func domainEvidencePreviewMetadataOnly(relativePath string) bool {
	return IsDomainEvidencePreviewMetadataPath(relativePath)
}

// IsDomainEvidencePreviewMetadataPath reserves package metadata across all source kinds.
func IsDomainEvidencePreviewMetadataPath(relativePath string) bool {
	return relativePath == pdfSnapshotRecordPath || path.Base(relativePath) == ".polis-git-source.json"
}

func domainEvidencePreviewSourceFiles(sourceKind, mediaType, displayName string, content []byte) ([]DirectoryInputFile, error) {
	if len(content) == 0 || len(content) > MaxUploadBytes {
		return nil, &UploadError{ReasonCode: "evidence_preview_unsupported"}
	}
	canonicalMediaType, err := normalizeMediaType(mediaType)
	if err != nil {
		return nil, &UploadError{ReasonCode: "evidence_preview_unsupported"}
	}
	if sourceKind == "upload" {
		filename, nameErr := safeDisplayName(displayName)
		if nameErr != nil {
			return nil, &UploadError{ReasonCode: "evidence_preview_unsupported"}
		}
		return []DirectoryInputFile{{RelativePath: filename, MediaType: canonicalMediaType, Content: content}}, nil
	}
	if sourceKindMediaTypeInvalid(sourceKind, canonicalMediaType) {
		return nil, &UploadError{ReasonCode: "evidence_preview_unsupported"}
	}
	files, err := ExtractVerifiedInputArchive(sourceKind, content)
	if err != nil || len(files) == 0 {
		return nil, &UploadError{ReasonCode: "evidence_preview_unsupported"}
	}
	return files, nil
}

func sourceKindMediaTypeInvalid(sourceKind, mediaType string) bool {
	switch sourceKind {
	case "directory_snapshot", "git_snapshot", "pdf_snapshot":
		return mediaType != "application/gzip"
	case "zip_snapshot":
		return mediaType != "application/zip"
	default:
		return true
	}
}

func domainEvidencePreviewMemberMediaType(sourceKind, relativePath, mediaType string) string {
	if sourceKind == "pdf_snapshot" && relativePath == pdfSnapshotOriginalPath && mediaType == "application/octet-stream" {
		return "application/pdf"
	}
	return mediaType
}

func prepareDomainEvidencePreviewContent(relativePath, mediaType string, content []byte) (DomainEvidencePreview, error) {
	if len(content) == 0 || len(content) > MaxUploadBytes {
		return DomainEvidencePreview{}, &UploadError{ReasonCode: "evidence_preview_unsupported"}
	}
	if mediaType == "application/pdf" {
		return DomainEvidencePreview{}, &UploadError{ReasonCode: "evidence_preview_unsupported"}
	}
	filename := path.Base(relativePath)
	prepared, err := prepareUploadFile(filename, mediaType, content)
	if err != nil || prepared.MediaType != mediaType || (prepared.State != StateUsable && prepared.State != StatePartial) {
		return DomainEvidencePreview{}, &UploadError{ReasonCode: "evidence_preview_unsupported"}
	}
	filename = prepared.DisplayName
	if !evidencePreviewMediaTypeAllowed(mediaType) {
		return DomainEvidencePreview{}, &UploadError{ReasonCode: "evidence_preview_unsupported"}
	}
	copyOfContent := append([]byte(nil), content...)
	return DomainEvidencePreview{FileName: filename, MediaType: mediaType, Content: copyOfContent, ContentSHA256: evidencePreviewDigest(copyOfContent)}, nil
}

func evidencePreviewDigest(content []byte) string {
	digest := sha256.Sum256(content)
	return hex.EncodeToString(digest[:])
}

func evidencePreviewMediaTypeAllowed(mediaType string) bool {
	switch strings.ToLower(strings.TrimSpace(mediaType)) {
	case "text/plain", "text/markdown", "text/csv", "application/json", "image/png", "image/jpeg":
		return true
	default:
		return false
	}
}

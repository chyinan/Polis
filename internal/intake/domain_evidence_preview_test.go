// pattern: Functional Core
package intake

import (
	"bytes"
	"errors"
	"testing"
)

func TestPrepareDomainEvidencePreviewCopiesValidatedTextBytes(t *testing.T) {
	content := []byte("R3 evidence: reviewers must see this text safely")
	preview, err := PrepareDomainEvidencePreview("upload", "text/markdown", "quality.md", content, "quality.md")
	if err != nil {
		t.Fatal(err)
	}
	if preview.FileName != "quality.md" || preview.MediaType != "text/markdown" || !bytes.Equal(preview.Content, content) {
		t.Fatalf("preview does not preserve the validated source: %+v", preview)
	}
	content[0] = 'x'
	if !bytes.Equal(preview.Content, []byte("R3 evidence: reviewers must see this text safely")) {
		t.Fatal("preview content aliases the caller's mutable input")
	}
}

func TestPrepareDomainEvidencePreviewUsesStaticExtractedPDFText(t *testing.T) {
	original := testTextPDF("reviewable PDF evidence")
	prepared, stored, err := PrepareMissionInput("quality.pdf", "application/pdf", original)
	if err != nil {
		t.Fatal(err)
	}
	if prepared.SourceKind != "pdf_snapshot" {
		t.Fatalf("PDF source kind=%q", prepared.SourceKind)
	}
	entries, err := ListDomainEvidencePreviewEntries(prepared.SourceKind, prepared.MediaType, prepared.DisplayName, stored)
	if err != nil || len(entries) != 3 {
		t.Fatalf("PDF preview entries=(%+v,%v)", entries, err)
	}
	var originalEntry, extractedTextEntry, metadataEntry *DomainEvidencePreviewEntry
	for index := range entries {
		entry := &entries[index]
		switch entry.RelativePath {
		case pdfSnapshotOriginalPath:
			originalEntry = entry
		case pdfSnapshotTextPath:
			extractedTextEntry = entry
		case pdfSnapshotRecordPath:
			metadataEntry = entry
		}
	}
	if originalEntry == nil || originalEntry.Previewable || extractedTextEntry == nil || !extractedTextEntry.Previewable || metadataEntry == nil || metadataEntry.Previewable {
		t.Fatalf("PDF preview manifest did not distinguish extracted content from original/metadata: %+v", entries)
	}
	if _, err = PrepareDomainEvidencePreview(prepared.SourceKind, prepared.MediaType, prepared.DisplayName, stored, pdfSnapshotOriginalPath); err == nil {
		t.Fatal("preview exposed an active PDF document inside the application")
	}
	preview, err := PrepareDomainEvidencePreview(prepared.SourceKind, prepared.MediaType, prepared.DisplayName, stored, pdfSnapshotTextPath)
	if err != nil || preview.FileName != "extracted.txt" || preview.MediaType != "text/plain" || string(preview.Content) != "reviewable PDF evidence" {
		t.Fatalf("PDF extracted-text preview=(%+v,%v)", preview, err)
	}
}

func TestPrepareDomainEvidencePreviewRejectsUnsafeAndArchiveSources(t *testing.T) {
	cases := []struct {
		name      string
		source    string
		mediaType string
		filename  string
		content   []byte
	}{
		{name: "directory archive", source: "directory_snapshot", mediaType: "application/gzip", filename: "evidence.tar.gz", content: []byte("archive")},
		{name: "zip archive", source: "zip_snapshot", mediaType: "application/zip", filename: "evidence.zip", content: []byte("archive")},
		{name: "git snapshot", source: "git_snapshot", mediaType: "application/gzip", filename: "evidence.tar.gz", content: []byte("archive")},
		{name: "active HTML", source: "upload", mediaType: "text/html", filename: "evidence.html", content: []byte("<script>alert(1)</script>")},
		{name: "invalid UTF-8 text", source: "upload", mediaType: "text/plain", filename: "evidence.txt", content: []byte{0xff, 0xfe}},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := PrepareDomainEvidencePreview(testCase.source, testCase.mediaType, testCase.filename, testCase.content, testCase.filename)
			var uploadError *UploadError
			if !errors.As(err, &uploadError) || uploadError.ReasonCode != "evidence_preview_unsupported" {
				t.Fatalf("preview error=%v, want evidence_preview_unsupported", err)
			}
		})
	}
}

func TestDomainEvidencePreviewListsVerifiedDirectoryAndZIPMembers(t *testing.T) {
	directory, err := PrepareDirectorySnapshot([]DirectoryInputFile{
		{RelativePath: "bundle/report.md", MediaType: "text/markdown", Content: []byte("report body")},
		{RelativePath: "bundle/diagram.svg", MediaType: "image/svg+xml", Content: []byte("<svg></svg>")},
	})
	if err != nil {
		t.Fatal(err)
	}
	directoryEntries, err := ListDomainEvidencePreviewEntries("directory_snapshot", directory.Upload.MediaType, directory.Upload.DisplayName, directory.Archive)
	if err != nil || len(directoryEntries) != 2 {
		t.Fatalf("directory preview entries=(%+v,%v)", directoryEntries, err)
	}
	var reportEntry, svgEntry *DomainEvidencePreviewEntry
	for index := range directoryEntries {
		entry := &directoryEntries[index]
		switch entry.RelativePath {
		case "bundle/report.md":
			reportEntry = entry
		case "bundle/diagram.svg":
			svgEntry = entry
		}
	}
	if reportEntry == nil || !reportEntry.Previewable || svgEntry == nil || svgEntry.Previewable || svgEntry.ReasonCode != "evidence_preview_unsupported" {
		t.Fatalf("unsafe media type was not excluded from the preview manifest: %+v", directoryEntries)
	}
	preview, err := PrepareDomainEvidencePreview("directory_snapshot", directory.Upload.MediaType, directory.Upload.DisplayName, directory.Archive, reportEntry.RelativePath)
	if err != nil || string(preview.Content) != "report body" {
		t.Fatalf("directory member preview=(%+v,%v)", preview, err)
	}
	if _, err = PrepareDomainEvidencePreview("directory_snapshot", directory.Upload.MediaType, directory.Upload.DisplayName, directory.Archive, "../outside.txt"); err == nil {
		t.Fatal("directory preview accepted a path outside the verified manifest")
	}

	zipBytes := zipEntriesForTest(t, []zipTestEntry{{name: "repo/README.md", content: "zip report"}})
	zipUpload, storedZIP, err := PrepareMissionInput("source.zip", "application/zip", zipBytes)
	if err != nil {
		t.Fatal(err)
	}
	zipEntries, err := ListDomainEvidencePreviewEntries(zipUpload.SourceKind, zipUpload.MediaType, zipUpload.DisplayName, storedZIP)
	if err != nil || len(zipEntries) != 1 || !zipEntries[0].Previewable || zipEntries[0].RelativePath != "repo/README.md" {
		t.Fatalf("ZIP preview entries=(%+v,%v)", zipEntries, err)
	}
}

func TestDomainEvidencePreviewExcludesReservedMetadataAcrossArchiveKinds(t *testing.T) {
	reservedPaths := []string{"pdf/extraction.json", "pdf/.polis-git-source.json"}
	directoryFiles := make([]DirectoryInputFile, 0, len(reservedPaths))
	zipFiles := make([]zipTestEntry, 0, len(reservedPaths))
	for _, relativePath := range reservedPaths {
		content := []byte(`{"metadata":true}`)
		directoryFiles = append(directoryFiles, DirectoryInputFile{RelativePath: relativePath, MediaType: "application/json", Content: content})
		zipFiles = append(zipFiles, zipTestEntry{name: relativePath, content: string(content)})
	}
	directory, err := PrepareDirectorySnapshot(directoryFiles)
	if err != nil {
		t.Fatal(err)
	}
	zipBytes := zipEntriesForTest(t, zipFiles)
	zipUpload, storedZIP, err := PrepareMissionInput("evidence.zip", "application/zip", zipBytes)
	if err != nil {
		t.Fatal(err)
	}
	sources := []struct {
		name, kind, mediaType, displayName string
		content                            []byte
	}{
		{name: "directory", kind: "directory_snapshot", mediaType: directory.Upload.MediaType, displayName: directory.Upload.DisplayName, content: directory.Archive},
		{name: "zip", kind: zipUpload.SourceKind, mediaType: zipUpload.MediaType, displayName: zipUpload.DisplayName, content: storedZIP},
	}
	for _, source := range sources {
		t.Run(source.name, func(t *testing.T) {
			entries, listErr := ListDomainEvidencePreviewEntries(source.kind, source.mediaType, source.displayName, source.content)
			if listErr != nil {
				t.Fatal(listErr)
			}
			for _, relativePath := range reservedPaths {
				var found *DomainEvidencePreviewEntry
				for index := range entries {
					if entries[index].RelativePath == relativePath {
						found = &entries[index]
						break
					}
				}
				if found == nil || found.Previewable || found.ReasonCode != "evidence_preview_unsupported" {
					t.Fatalf("reserved metadata path %q was not listed as unavailable: %+v", relativePath, found)
				}
				if _, previewErr := PrepareDomainEvidencePreview(source.kind, source.mediaType, source.displayName, source.content, relativePath); previewErr == nil {
					t.Fatalf("reserved metadata path %q was previewable from %s", relativePath, source.name)
				}
			}
		})
	}
}

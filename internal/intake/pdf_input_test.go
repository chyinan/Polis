package intake

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/giraffesyo/pdf/pdftest"
)

func TestPrepareMissionInputPackagesPDFAndDeliversExtractedTextToWorker(t *testing.T) {
	const sentinel = "PDF_WORKER_INPUT_SENTINEL"
	source := testTextPDF(sentinel)
	prepared, stored, err := PrepareMissionInput("brief.pdf", "application/pdf", source)
	if err != nil {
		t.Fatal(err)
	}
	if prepared.SourceKind != "pdf_snapshot" || prepared.State != StatePartial || prepared.DisplayName != "brief.pdf" || prepared.MediaType != "application/gzip" {
		t.Fatalf("PDF input metadata = %+v", prepared)
	}
	if bytes.Equal(stored, source) || int64(len(stored)) != prepared.ByteSize || digestForTest(string(stored)) != prepared.ContentDigest {
		t.Fatal("PDF source was not stored as a digest-bound extraction package")
	}
	if err = VerifyPreparedUpload(prepared, stored); err != nil {
		t.Fatalf("stored PDF package did not verify: %v", err)
	}

	files, err := ExtractVerifiedInputArchive(prepared.SourceKind, stored)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 3 {
		t.Fatalf("PDF package files = %d, want source, extraction record, and text: %+v", len(files), files)
	}
	if !containsInputFile(files, "pdf/original.pdf", source) || !containsInputFileText(files, "pdf/extracted.txt", sentinel) {
		t.Fatalf("PDF package did not preserve its source and extracted text: %+v", files)
	}

	manifest, manifestDigest, err := PrepareModelInputManifest("company-1", "mission-1", "task-pdf", []MissionInputReference{{
		InputID: "pdf-input", Revision: 1, RequestID: "pdf-request", SourceKind: prepared.SourceKind,
		DisplayName: prepared.DisplayName, MediaType: prepared.MediaType, ByteSize: prepared.ByteSize,
		ContentDigest: prepared.ContentDigest, State: prepared.State,
	}})
	if err != nil {
		t.Fatal(err)
	}
	workerContext, err := PrepareModelInputContext(manifest, manifestDigest, map[string][]byte{"pdf-input": stored})
	if err != nil {
		t.Fatal(err)
	}
	if len(workerContext.Inputs) != 2 || workerContext.Inputs[0].RelativePath != "pdf/extracted.txt" || !strings.Contains(workerContext.Inputs[0].Text, sentinel) {
		t.Fatalf("PDF extracted text did not reach bounded Worker context: %+v", workerContext)
	}
	if len(workerContext.Excluded) != 1 || workerContext.Excluded[0].RelativePath != "pdf/original.pdf" || workerContext.Excluded[0].Reason != "representation_not_supported" {
		t.Fatalf("PDF original was not explicitly excluded from text context: %+v", workerContext.Excluded)
	}
}

func TestPrepareMissionInputRejectsMalformedAndOversizedPDFs(t *testing.T) {
	if _, _, err := PrepareMissionInput("broken.pdf", "application/pdf", []byte("not a PDF")); uploadReason(err) != "pdf_invalid" {
		t.Fatalf("malformed PDF error = %v, want pdf_invalid", err)
	}
	if _, _, err := PrepareMissionInput("brief.pdf", "image/png", testTextPDF("x")); uploadReason(err) != "media_type_mismatch" {
		t.Fatalf("mislabeled PDF error = %v, want media_type_mismatch", err)
	}
	if _, _, err := PrepareMissionInput("large.pdf", "application/pdf", bytes.Repeat([]byte{'x'}, MaxPDFSourceBytes+1)); uploadReason(err) != "pdf_source_too_large" {
		t.Fatalf("oversized PDF error = %v, want pdf_source_too_large", err)
	}
}

func TestPrepareMissionInputKeepsScannedPDFPartialWithoutInventingOCRText(t *testing.T) {
	source := pdftest.Build(1,
		pdftest.Catalog(2),
		pdftest.Pages(3),
		pdftest.Page(2, 4, "<< >>"),
		pdftest.Stream("", "q Q"),
	)
	prepared, stored, err := PrepareMissionInput("scan.pdf", "application/pdf", source)
	if err != nil {
		t.Fatal(err)
	}
	if prepared.State != StatePartial {
		t.Fatalf("scanned PDF state = %q, want partial", prepared.State)
	}
	files, err := ExtractVerifiedInputArchive(prepared.SourceKind, stored)
	if err != nil {
		t.Fatal(err)
	}
	if containsInputFile(files, "pdf/extracted.txt", nil) {
		t.Fatal("image-only PDF unexpectedly produced an extracted text file")
	}
	if !containsInputFile(files, "pdf/original.pdf", source) || !containsInputFileText(files, "pdf/extraction.json", "no_text") {
		t.Fatalf("scanned PDF source or extraction status missing: %+v", files)
	}
}

func TestPrepareMissionInputCapsPDFTextAndRecordsTruncation(t *testing.T) {
	longText := strings.Repeat("x", MaxModelInputFileBytes+2048)
	prepared, stored, err := PrepareMissionInput("long.pdf", "application/pdf", testTextPDF(longText))
	if err != nil {
		t.Fatal(err)
	}
	files, err := ExtractVerifiedInputArchive(prepared.SourceKind, stored)
	if err != nil {
		t.Fatal(err)
	}
	text := findInputFile(files, "pdf/extracted.txt")
	if len(text) > MaxModelInputFileBytes {
		t.Fatalf("PDF extracted text size = %d, want <= %d", len(text), MaxModelInputFileBytes)
	}
	var record PDFExtractionRecord
	if err = json.Unmarshal(findInputFile(files, "pdf/extraction.json"), &record); err != nil {
		t.Fatal(err)
	}
	if !record.TextTruncated || record.Status != "text_truncated" {
		t.Fatalf("PDF text truncation record = %+v", record)
	}
}

func testTextPDF(value string) []byte {
	value = strings.ReplaceAll(value, "\\", "\\\\")
	value = strings.ReplaceAll(value, "(", "\\(")
	value = strings.ReplaceAll(value, ")", "\\)")
	contents := fmt.Sprintf("BT /F1 12 Tf 72 720 Td (%s) Tj ET", value)
	return pdftest.Build(1,
		pdftest.Catalog(2),
		pdftest.Pages(3),
		pdftest.Page(2, 4, "<< /Font << /F1 5 0 R >> >>"),
		pdftest.Stream("", contents),
		pdftest.Helvetica(),
	)
}

func containsInputFile(files []DirectoryInputFile, name string, content []byte) bool {
	for _, file := range files {
		if file.RelativePath == name && bytes.Equal(file.Content, content) {
			return true
		}
	}
	return false
}

func containsInputFileText(files []DirectoryInputFile, name, text string) bool {
	for _, file := range files {
		if file.RelativePath == name && strings.Contains(string(file.Content), text) {
			return true
		}
	}
	return false
}

func findInputFile(files []DirectoryInputFile, name string) []byte {
	for _, file := range files {
		if file.RelativePath == name {
			return file.Content
		}
	}
	return nil
}

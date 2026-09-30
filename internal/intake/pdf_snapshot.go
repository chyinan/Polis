// pattern: Functional Core
package intake

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"path"
	"sort"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	pdf "github.com/giraffesyo/pdf"
)

const (
	MaxPDFSourceBytes = 6 << 20
	MaxPDFPages       = 40

	pdfSnapshotRoot         = "pdf"
	pdfSnapshotOriginalPath = "pdf/original.pdf"
	pdfSnapshotTextPath     = "pdf/extracted.txt"
	pdfSnapshotRecordPath   = "pdf/extraction.json"
	pdfExtractionSchema     = "polis-pdf-extraction@1"
	pdfExtractionParser     = "github.com/giraffesyo/pdf@v0.7.0"
	maxPDFExtractDuration   = 8 * time.Second
	maxPDFStreamBytes       = 1 << 20
	maxPDFOperatorsPerPage  = 20_000
	maxPDFGlyphsPerPage     = 16_384
	maxPDFFormDepth         = 8
	maxPDFImagesPerPage     = 64
	maxPDFImageBytesPerPage = 1 << 20
	maxPDFImagePixels       = 4_000_000
)

type PDFExtractionRecord struct {
	SchemaVersion       string   `json:"schemaVersion"`
	Parser              string   `json:"parser"`
	Status              string   `json:"status"`
	SourceSHA256        string   `json:"sourceSha256"`
	ExtractedTextSHA256 string   `json:"extractedTextSha256,omitempty"`
	PageCount           int      `json:"pageCount"`
	PagesProcessed      int      `json:"pagesProcessed"`
	TextBytes           int      `json:"textBytes"`
	TextTruncated       bool     `json:"textTruncated"`
	PageLimitReached    bool     `json:"pageLimitReached"`
	ImagePages          int      `json:"imagePages"`
	WarningCodes        []string `json:"warningCodes"`
}

func preparePDFMissionInput(filename, declaredMediaType string, content []byte) (PreparedUpload, []byte, error) {
	displayName, err := safeDisplayName(filename)
	if err != nil {
		return PreparedUpload{}, nil, err
	}
	if len(content) == 0 {
		return PreparedUpload{}, nil, &UploadError{ReasonCode: "empty_file"}
	}
	if len(content) > MaxUploadBytes {
		return PreparedUpload{}, nil, &UploadError{ReasonCode: "upload_too_large"}
	}
	if len(content) > MaxPDFSourceBytes {
		return PreparedUpload{}, nil, &UploadError{ReasonCode: "pdf_source_too_large"}
	}
	declared, err := normalizeMediaType(declaredMediaType)
	if err != nil {
		return PreparedUpload{}, nil, err
	}
	if !compatibleDeclaredMediaType(declared, "application/pdf", "application/octet-stream") {
		return PreparedUpload{}, nil, &UploadError{ReasonCode: "media_type_mismatch"}
	}

	text, record, err := extractPDFText(content)
	if err != nil {
		reason := "pdf_invalid"
		if errors.Is(err, context.DeadlineExceeded) {
			reason = "pdf_processing_limit"
		}
		return PreparedUpload{}, nil, &UploadError{ReasonCode: reason}
	}
	files := []DirectoryInputFile{
		{RelativePath: pdfSnapshotOriginalPath, MediaType: "application/pdf", Content: content},
	}
	recordBytes, err := json.Marshal(record)
	if err != nil {
		return PreparedUpload{}, nil, &UploadError{ReasonCode: "pdf_metadata_invalid"}
	}
	files = append(files, DirectoryInputFile{RelativePath: pdfSnapshotRecordPath, MediaType: "application/json", Content: recordBytes})
	if len(text) > 0 {
		files = append(files, DirectoryInputFile{RelativePath: pdfSnapshotTextPath, MediaType: "text/plain", Content: []byte(text)})
	}
	snapshot, err := PrepareDirectorySnapshot(files)
	if err != nil {
		return PreparedUpload{}, nil, err
	}
	prepared := snapshot.Upload
	prepared.SourceKind = "pdf_snapshot"
	prepared.DisplayName = displayName
	prepared.State = StatePartial
	return prepared, snapshot.Archive, nil
}

func extractPDFText(content []byte) (text string, record PDFExtractionRecord, resultErr error) {
	defer func() {
		if recover() != nil {
			text = ""
			record = PDFExtractionRecord{}
			resultErr = errors.New("PDF parser failed")
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), maxPDFExtractDuration)
	defer cancel()
	var output strings.Builder
	warnings := make(map[string]struct{})
	imagePages := 0
	pagesProcessed := 0
	textTruncated := false
	options := pdf.Options{
		Pages:       []pdf.PageRange{{First: 1, Last: MaxPDFPages}},
		Concurrency: 1,
		Layout:      pdf.LayoutOptions{Mode: pdf.LayoutContentOrder},
		Limits: pdf.Limits{
			MaxStreamBytes: maxPDFStreamBytes, MaxOperatorsPerPage: maxPDFOperatorsPerPage,
			MaxGlyphsPerPage: maxPDFGlyphsPerPage, MaxFormDepth: maxPDFFormDepth,
			MaxImagesPerPage: maxPDFImagesPerPage, MaxImageBytesPerPage: maxPDFImageBytesPerPage,
			MaxImagePixels: maxPDFImagePixels,
		},
	}
	document, err := pdf.ExtractPages(ctx, bytes.NewReader(content), int64(len(content)), options, func(page pdf.Page) error {
		pagesProcessed++
		if page.ImageCount > 0 {
			imagePages++
		}
		for _, warning := range page.Warnings {
			warnings[string(warning.Code)] = struct{}{}
		}
		pageText := normalizePDFText(page.Text())
		if pageText == "" {
			return nil
		}
		separator := ""
		if output.Len() > 0 {
			separator = "\n\n"
		}
		if !appendBoundedPDFText(&output, separator, MaxModelInputFileBytes) || !appendBoundedPDFText(&output, pageText, MaxModelInputFileBytes) {
			textTruncated = true
		}
		return nil
	})
	if err != nil {
		return "", PDFExtractionRecord{}, err
	}
	if document == nil || document.PageCount < 1 {
		return "", PDFExtractionRecord{}, errors.New("PDF has no pages")
	}
	for _, warning := range document.Warnings {
		warnings[string(warning.Code)] = struct{}{}
	}
	warningCodes := make([]string, 0, len(warnings))
	for code := range warnings {
		warningCodes = append(warningCodes, code)
	}
	sort.Strings(warningCodes)
	for _, code := range warningCodes {
		if code == string(pdf.WarningWorkLimit) || code == string(pdf.WarningStreamLimit) {
			textTruncated = true
		}
	}
	pageLimitReached := document.PageCount > pagesProcessed
	text = output.String()
	sourceDigest := sha256.Sum256(content)
	record = PDFExtractionRecord{
		SchemaVersion: pdfExtractionSchema, Parser: pdfExtractionParser, Status: "text_extracted",
		SourceSHA256: hex.EncodeToString(sourceDigest[:]), PageCount: document.PageCount,
		PagesProcessed: pagesProcessed, TextBytes: len(text), TextTruncated: textTruncated,
		PageLimitReached: pageLimitReached, ImagePages: imagePages, WarningCodes: warningCodes,
	}
	if len(text) == 0 {
		record.Status = "no_text"
	} else if textTruncated || pageLimitReached {
		record.Status = "text_truncated"
	} else if imagePages > 0 || len(warningCodes) > 0 {
		record.Status = "partial_text"
	}
	if len(text) > 0 {
		textDigest := sha256.Sum256([]byte(text))
		record.ExtractedTextSHA256 = hex.EncodeToString(textDigest[:])
	}
	return text, record, nil
}

func appendBoundedPDFText(output *strings.Builder, value string, maxBytes int) bool {
	complete := true
	for _, char := range value {
		if unicode.IsControl(char) && char != '\n' && char != '\r' && char != '\t' {
			continue
		}
		if char == '\r' {
			char = '\n'
		}
		encoded := string(char)
		if output.Len()+len(encoded) > maxBytes {
			complete = false
			break
		}
		output.WriteString(encoded)
	}
	return complete
}

func normalizePDFText(value string) string {
	if !utf8.ValidString(value) {
		value = strings.ToValidUTF8(value, "�")
	}
	return strings.TrimSpace(value)
}

func verifyPDFSnapshot(prepared PreparedUpload, content []byte) (PreparedUpload, error) {
	files, err := ExtractVerifiedDirectoryFiles(content)
	if err != nil {
		return PreparedUpload{}, err
	}
	var source []byte
	for _, file := range files {
		if file.RelativePath == pdfSnapshotOriginalPath {
			source = file.Content
			break
		}
	}
	if len(source) == 0 {
		return PreparedUpload{}, &UploadError{ReasonCode: "pdf_snapshot_invalid"}
	}
	verified, rebuilt, err := preparePDFMissionInput(prepared.DisplayName, "application/pdf", source)
	if err != nil {
		return PreparedUpload{}, err
	}
	if !bytes.Equal(rebuilt, content) {
		return PreparedUpload{}, &UploadError{ReasonCode: "pdf_snapshot_noncanonical"}
	}
	if path.Ext(strings.ToLower(prepared.DisplayName)) != ".pdf" {
		return PreparedUpload{}, &UploadError{ReasonCode: "pdf_snapshot_invalid"}
	}
	return verified, nil
}

// pattern: Functional Core
package intake

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/png"
	"math"
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

	pdfSnapshotRoot          = "pdf"
	pdfSnapshotOriginalPath  = "pdf/original.pdf"
	pdfSnapshotTextPath      = "pdf/extracted.txt"
	pdfSnapshotRecordPath    = "pdf/extraction.json"
	pdfExtractionSchemaV1    = "polis-pdf-extraction@1"
	pdfExtractionSchemaV2    = "polis-pdf-extraction@2"
	pdfExtractionSchema      = "polis-pdf-extraction@3"
	pdfExtractionParser      = "github.com/giraffesyo/pdf@v0.7.0"
	pdfImageRepresentationV1 = "polis-pdf-embedded-image@1"
	pdfImageRepresentation   = "polis-pdf-embedded-image@2"
	pdfAppearanceSemantics   = "intrinsic_embedded_image_object"
	pdfImageNumberMeaning    = "1_based_page_images_array_ordinal_not_pdf_object_id"
	pdfAppearanceLimitations = "soft_masks_transparency_color_key_masks_and_rendering_intent_may_not_be_applied"
	maxPDFExtractDuration    = 8 * time.Second
	maxPDFStreamBytes        = 1 << 20
	maxPDFOperatorsPerPage   = 20_000
	maxPDFGlyphsPerPage      = 16_384
	maxPDFFormDepth          = 8
	maxPDFImagesPerPage      = 64
	maxPDFImageBytesPerPage  = 1 << 20
	maxPDFImagePixels        = 1_000_000
	maxPDFVisualImages       = 8
	maxPDFVisualImageBytes   = 256 << 10
	maxPDFVisualTotalBytes   = 960 << 10
)

type PDFExtractionRecord struct {
	SchemaVersion         string         `json:"schemaVersion"`
	Parser                string         `json:"parser"`
	Status                string         `json:"status"`
	SourceSHA256          string         `json:"sourceSha256"`
	ExtractedTextSHA256   string         `json:"extractedTextSha256,omitempty"`
	PageCount             int            `json:"pageCount"`
	PagesProcessed        int            `json:"pagesProcessed"`
	TextBytes             int            `json:"textBytes"`
	TextTruncated         bool           `json:"textTruncated"`
	PageLimitReached      bool           `json:"pageLimitReached"`
	ImagePages            int            `json:"imagePages"`
	ImageRepresentation   string         `json:"imageRepresentation,omitempty"`
	AppearanceSemantics   string         `json:"appearanceSemantics,omitempty"`
	AppearanceLimitations string         `json:"appearanceLimitations,omitempty"`
	ImageNumberMeaning    string         `json:"imageNumberMeaning,omitempty"`
	PageImages            []PDFPageImage `json:"pageImages,omitempty"`
	PageImagesTruncated   bool           `json:"pageImagesTruncated,omitempty"`
	WarningCodes          []string       `json:"warningCodes"`
}

// PDFPageImage binds an intrinsic decoded PDF image object to the page and
// placement where the parser reports it. ImageNumber is its 1-based ordinal
// in the parser's Page.Images slice, not a source PDF object identifier. This
// does not represent a raster of the full page or its complete visible effect.
type PDFPageImage struct {
	PageNumber    int             `json:"pageNumber"`
	ImageNumber   int             `json:"imageNumber"`
	RelativePath  string          `json:"relativePath"`
	Width         int             `json:"width"`
	Height        int             `json:"height"`
	ContentDigest string          `json:"contentDigest"`
	Placement     [4]PDFPagePoint `json:"placement"`
	MediaBox      PDFPageRect     `json:"mediaBox"`
	CropBox       PDFPageRect     `json:"cropBox"`
	Rotation      int             `json:"rotation"`
}

type PDFPagePoint struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

type PDFPageRect struct {
	MinX float64 `json:"minX"`
	MinY float64 `json:"minY"`
	MaxX float64 `json:"maxX"`
	MaxY float64 `json:"maxY"`
}

var errPDFVisualImageTooLarge = errors.New("PDF embedded image representation exceeds its byte bound")

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

	text, record, pageImages, err := extractPDFRepresentation(content)
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
	files = append(files, pageImages...)
	return packagePDFSnapshot(displayName, files)
}

func prepareLegacyPDFMissionInput(filename string, source []byte) (PreparedUpload, []byte, error) {
	displayName, err := safeDisplayName(filename)
	if err != nil {
		return PreparedUpload{}, nil, err
	}
	text, record, err := extractPDFText(source)
	if err != nil {
		return PreparedUpload{}, nil, err
	}
	files := []DirectoryInputFile{{RelativePath: pdfSnapshotOriginalPath, MediaType: "application/pdf", Content: source}}
	recordBytes, err := json.Marshal(record)
	if err != nil {
		return PreparedUpload{}, nil, &UploadError{ReasonCode: "pdf_metadata_invalid"}
	}
	files = append(files, DirectoryInputFile{RelativePath: pdfSnapshotRecordPath, MediaType: "application/json", Content: recordBytes})
	if len(text) > 0 {
		files = append(files, DirectoryInputFile{RelativePath: pdfSnapshotTextPath, MediaType: "text/plain", Content: []byte(text)})
	}
	return packagePDFSnapshot(displayName, files)
}

func preparePDFMissionInputV2(filename string, source []byte) (PreparedUpload, []byte, error) {
	displayName, err := safeDisplayName(filename)
	if err != nil {
		return PreparedUpload{}, nil, err
	}
	if len(source) == 0 || len(source) > MaxPDFSourceBytes {
		return PreparedUpload{}, nil, &UploadError{ReasonCode: "pdf_source_too_large"}
	}
	text, record, pageImages, err := extractPDFRepresentationV2(source)
	if err != nil {
		return PreparedUpload{}, nil, err
	}
	files := []DirectoryInputFile{{RelativePath: pdfSnapshotOriginalPath, MediaType: "application/pdf", Content: source}}
	recordBytes, err := json.Marshal(record)
	if err != nil {
		return PreparedUpload{}, nil, &UploadError{ReasonCode: "pdf_metadata_invalid"}
	}
	files = append(files, DirectoryInputFile{RelativePath: pdfSnapshotRecordPath, MediaType: "application/json", Content: recordBytes})
	if len(text) > 0 {
		files = append(files, DirectoryInputFile{RelativePath: pdfSnapshotTextPath, MediaType: "text/plain", Content: []byte(text)})
	}
	files = append(files, pageImages...)
	return packagePDFSnapshot(displayName, files)
}

func packagePDFSnapshot(displayName string, files []DirectoryInputFile) (PreparedUpload, []byte, error) {
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
	text, record, _, resultErr = extractPDF(content, false, pdfExtractionSchemaV1, false)
	return text, record, resultErr
}

func extractPDFRepresentation(content []byte) (text string, record PDFExtractionRecord, files []DirectoryInputFile, resultErr error) {
	return extractPDF(content, true, pdfExtractionSchema, false)
}

func extractPDFRepresentationV2(content []byte) (text string, record PDFExtractionRecord, files []DirectoryInputFile, resultErr error) {
	return extractPDF(content, true, pdfExtractionSchemaV2, true)
}

func extractPDF(content []byte, includeImages bool, schemaVersion string, legacyV2 bool) (text string, record PDFExtractionRecord, files []DirectoryInputFile, resultErr error) {
	defer func() {
		if recover() != nil {
			text = ""
			record = PDFExtractionRecord{}
			files = nil
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
	pageImages := make([]PDFPageImage, 0, maxPDFVisualImages)
	imageFiles := make([]DirectoryInputFile, 0, maxPDFVisualImages)
	visualAttempts := 0
	visualBytes := 0
	visualTruncated := false
	options := pdf.Options{
		Pages:         []pdf.PageRange{{First: 1, Last: MaxPDFPages}},
		Concurrency:   1,
		Layout:        pdf.LayoutOptions{Mode: pdf.LayoutContentOrder},
		IncludeImages: includeImages,
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
		if includeImages {
			if !legacyV2 && page.ImageCount > len(page.Images) {
				visualTruncated = true
				warnings["image_representation_omitted"] = struct{}{}
			}
			for imageIndex, embedded := range page.Images {
				if visualAttempts >= maxPDFVisualImages {
					visualTruncated = true
					break
				}
				visualAttempts++
				if err := ctx.Err(); err != nil {
					return err
				}
				var encoded []byte
				var decodeErr error
				if legacyV2 {
					encoded, decodeErr = encodePDFEmbeddedImageV2(embedded)
				} else {
					encoded, decodeErr = encodePDFEmbeddedImage(embedded)
				}
				if err := ctx.Err(); err != nil {
					return err
				}
				if decodeErr != nil || len(encoded) == 0 || len(encoded) > maxPDFVisualImageBytes || visualBytes+len(encoded) > maxPDFVisualTotalBytes {
					visualTruncated = true
					warnings["image_representation_omitted"] = struct{}{}
					continue
				}
				relativePath := fmt.Sprintf("pdf/images/page-%04d-image-%02d.png", page.Number, imageIndex+1)
				digest := sha256.Sum256(encoded)
				metadata := PDFPageImage{
					PageNumber: page.Number, ImageNumber: imageIndex + 1, RelativePath: relativePath,
					Width: embedded.Width, Height: embedded.Height, ContentDigest: hex.EncodeToString(digest[:]),
					MediaBox: pdfPageRect(page.MediaBox), CropBox: pdfPageRect(page.CropBox), Rotation: page.Rotation,
				}
				for pointIndex, point := range embedded.Quad() {
					metadata.Placement[pointIndex] = PDFPagePoint{X: point.X, Y: point.Y}
				}
				if !validPDFPageImageMetadata(metadata) {
					visualTruncated = true
					warnings["image_representation_omitted"] = struct{}{}
					continue
				}
				pageImages = append(pageImages, metadata)
				imageFiles = append(imageFiles, DirectoryInputFile{RelativePath: relativePath, MediaType: "image/png", Content: encoded})
				visualBytes += len(encoded)
			}
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
		return "", PDFExtractionRecord{}, nil, err
	}
	if document == nil || document.PageCount < 1 {
		return "", PDFExtractionRecord{}, nil, errors.New("PDF has no pages")
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
		SchemaVersion: schemaVersion, Parser: pdfExtractionParser, Status: "text_extracted",
		SourceSHA256: hex.EncodeToString(sourceDigest[:]), PageCount: document.PageCount,
		PagesProcessed: pagesProcessed, TextBytes: len(text), TextTruncated: textTruncated,
		PageLimitReached: pageLimitReached, ImagePages: imagePages, WarningCodes: warningCodes,
	}
	if includeImages {
		record.ImageRepresentation = pdfImageRepresentation
		if legacyV2 {
			record.ImageRepresentation = pdfImageRepresentationV1
		} else {
			record.AppearanceSemantics = pdfAppearanceSemantics
			record.AppearanceLimitations = pdfAppearanceLimitations
			record.ImageNumberMeaning = pdfImageNumberMeaning
		}
		record.PageImages = pageImages
		record.PageImagesTruncated = visualTruncated || pageLimitReached
		files = imageFiles
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
	return text, record, files, nil
}

type pdfImageLimitBuffer struct {
	bytes.Buffer
	limit int
}

func (buffer *pdfImageLimitBuffer) Write(content []byte) (int, error) {
	if len(content) > buffer.limit-buffer.Len() {
		return 0, errPDFVisualImageTooLarge
	}
	return buffer.Buffer.Write(content)
}

func encodePDFEmbeddedImage(embedded pdf.Image) ([]byte, error) {
	if embedded.ImageMask {
		return nil, errors.New("PDF stencil image masks are unsupported")
	}
	switch embedded.Filter {
	case "":
		if !pdfRawSamplesHaveExactLength(embedded) {
			return nil, errors.New("PDF raw image samples are truncated or oversized")
		}
	case "DCTDecode":
	default:
		// The pinned parser pads short CCITT streams and raw samples. Other
		// filters are not admitted here until their completeness behavior is
		// independently verified.
		return nil, errors.New("PDF image filter is not supported by the strict visual path")
	}
	if embedded.Width < 1 || embedded.Height < 1 || embedded.Width > maxPDFImagePixels/embedded.Height {
		return nil, errors.New("PDF embedded image dimensions exceed their bound")
	}
	decoded, err := embedded.Decode()
	if err != nil {
		return nil, err
	}
	bounds := decoded.Bounds()
	if bounds.Dx() != embedded.Width || bounds.Dy() != embedded.Height || bounds.Dx() < 1 || bounds.Dy() < 1 || bounds.Dx() > maxPDFImagePixels/bounds.Dy() {
		return nil, errors.New("PDF embedded image decoded dimensions differ")
	}
	output := &pdfImageLimitBuffer{limit: maxPDFVisualImageBytes}
	if err = png.Encode(output, decoded); err != nil {
		return nil, err
	}
	return append([]byte(nil), output.Bytes()...), nil
}

// encodePDFEmbeddedImageV2 reproduces the pre-Slice267 decoder behavior only
// so immutable extraction@2 snapshots can still be verified byte-for-byte.
func encodePDFEmbeddedImageV2(embedded pdf.Image) ([]byte, error) {
	if embedded.Width < 1 || embedded.Height < 1 || embedded.Width > maxPDFImagePixels/embedded.Height {
		return nil, errors.New("PDF embedded image dimensions exceed their bound")
	}
	decoded, err := embedded.Decode()
	if err != nil {
		return nil, err
	}
	bounds := decoded.Bounds()
	if bounds.Dx() != embedded.Width || bounds.Dy() != embedded.Height || bounds.Dx() < 1 || bounds.Dy() < 1 || bounds.Dx() > maxPDFImagePixels/bounds.Dy() {
		return nil, errors.New("PDF embedded image decoded dimensions differ")
	}
	output := &pdfImageLimitBuffer{limit: maxPDFVisualImageBytes}
	if err = png.Encode(output, decoded); err != nil {
		return nil, err
	}
	return append([]byte(nil), output.Bytes()...), nil
}

func pdfRawSamplesHaveExactLength(embedded pdf.Image) bool {
	if embedded.Width < 1 || embedded.Height < 1 || embedded.Components < 1 || embedded.Width > maxPDFImagePixels/embedded.Height {
		return false
	}
	switch embedded.BitsPerComponent {
	case 1, 2, 4, 8, 16:
	default:
		return false
	}
	rowBits := uint64(embedded.Width)
	if rowBits > ^uint64(0)/uint64(embedded.Components) {
		return false
	}
	rowBits *= uint64(embedded.Components)
	if rowBits > ^uint64(0)/uint64(embedded.BitsPerComponent) {
		return false
	}
	rowBits *= uint64(embedded.BitsPerComponent)
	rowBytes := rowBits / 8
	if rowBits%8 != 0 {
		rowBytes++
	}
	if rowBytes > ^uint64(0)/uint64(embedded.Height) {
		return false
	}
	expectedBytes := rowBytes * uint64(embedded.Height)
	return expectedBytes == uint64(len(embedded.Data))
}

func pdfPageRect(value pdf.Rect) PDFPageRect {
	return PDFPageRect{MinX: value.MinX, MinY: value.MinY, MaxX: value.MaxX, MaxY: value.MaxY}
}

func validPDFPageImageMetadata(metadata PDFPageImage) bool {
	if metadata.PageNumber < 1 || metadata.PageNumber > MaxPDFPages || metadata.ImageNumber < 1 || metadata.ImageNumber > maxPDFImagesPerPage || metadata.RelativePath != fmt.Sprintf("pdf/images/page-%04d-image-%02d.png", metadata.PageNumber, metadata.ImageNumber) || metadata.Width < 1 || metadata.Height < 1 || metadata.Width > maxPDFImagePixels/metadata.Height || !validSHA256Digest(metadata.ContentDigest) || metadata.Rotation < 0 || metadata.Rotation >= 360 {
		return false
	}
	for _, point := range metadata.Placement {
		if math.IsNaN(point.X) || math.IsInf(point.X, 0) || math.IsNaN(point.Y) || math.IsInf(point.Y, 0) {
			return false
		}
	}
	for _, value := range []float64{metadata.MediaBox.MinX, metadata.MediaBox.MinY, metadata.MediaBox.MaxX, metadata.MediaBox.MaxY, metadata.CropBox.MinX, metadata.CropBox.MinY, metadata.CropBox.MaxX, metadata.CropBox.MaxY} {
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return false
		}
	}
	return true
}

func pdfImageMetadataByPath(files []DirectoryInputFile) (map[string]PDFPageImage, error) {
	byPath := make(map[string]DirectoryInputFile, len(files))
	var recordBytes []byte
	for _, file := range files {
		byPath[file.RelativePath] = file
		if file.RelativePath == pdfSnapshotRecordPath {
			recordBytes = file.Content
		}
	}
	if len(recordBytes) == 0 {
		return nil, errors.New("PDF snapshot extraction record is missing")
	}
	var record PDFExtractionRecord
	if err := json.Unmarshal(recordBytes, &record); err != nil || record.SchemaVersion != pdfExtractionSchema && record.SchemaVersion != pdfExtractionSchemaV2 && record.SchemaVersion != pdfExtractionSchemaV1 || record.PageCount < 1 || record.PagesProcessed < 1 || record.PagesProcessed > MaxPDFPages || record.PagesProcessed > record.PageCount || !validSHA256Digest(record.SourceSHA256) {
		return nil, errors.New("PDF snapshot extraction record is invalid")
	}
	original, exists := byPath[pdfSnapshotOriginalPath]
	if !exists || sha256Digest(original.Content) != record.SourceSHA256 {
		return nil, errors.New("PDF snapshot source differs from its extraction record")
	}
	switch record.SchemaVersion {
	case pdfExtractionSchemaV1:
		if record.ImageRepresentation != "" || record.AppearanceSemantics != "" || record.AppearanceLimitations != "" || record.ImageNumberMeaning != "" || len(record.PageImages) != 0 {
			return nil, errors.New("legacy PDF snapshot has unexpected visual metadata")
		}
	case pdfExtractionSchemaV2:
		if record.ImageRepresentation != pdfImageRepresentationV1 || record.AppearanceSemantics != "" || record.AppearanceLimitations != "" || record.ImageNumberMeaning != "" {
			return nil, errors.New("PDF extraction@2 image representation version is invalid")
		}
	case pdfExtractionSchema:
		if record.ImageRepresentation != pdfImageRepresentation || record.AppearanceSemantics != pdfAppearanceSemantics || record.AppearanceLimitations != pdfAppearanceLimitations || record.ImageNumberMeaning != pdfImageNumberMeaning {
			return nil, errors.New("PDF extraction@3 image semantics are invalid")
		}
	default:
		return nil, errors.New("PDF extraction schema is not supported")
	}
	if len(record.PageImages) > maxPDFVisualImages {
		return nil, errors.New("PDF snapshot image representation version is invalid")
	}
	result := make(map[string]PDFPageImage, len(record.PageImages))
	seenPages := make(map[[2]int]struct{}, len(record.PageImages))
	totalBytes := 0
	for _, metadata := range record.PageImages {
		if !validPDFPageImageMetadata(metadata) || metadata.PageNumber > record.PagesProcessed {
			return nil, errors.New("PDF page image metadata is invalid")
		}
		if _, duplicate := seenPages[[2]int{metadata.PageNumber, metadata.ImageNumber}]; duplicate {
			return nil, errors.New("PDF page image number is duplicated")
		}
		seenPages[[2]int{metadata.PageNumber, metadata.ImageNumber}] = struct{}{}
		file, exists := byPath[metadata.RelativePath]
		if !exists || file.MediaType != "image/png" || sha256Digest(file.Content) != metadata.ContentDigest || len(file.Content) > maxPDFVisualImageBytes {
			return nil, errors.New("PDF page image bytes differ from their extraction metadata")
		}
		totalBytes += len(file.Content)
		if totalBytes > maxPDFVisualTotalBytes {
			return nil, errors.New("PDF page images exceed their total byte bound")
		}
		config, format, decodeErr := image.DecodeConfig(bytes.NewReader(file.Content))
		if decodeErr != nil || format != "png" || config.Width != metadata.Width || config.Height != metadata.Height {
			return nil, errors.New("PDF page image dimensions differ from their extraction metadata")
		}
		result[metadata.RelativePath] = metadata
	}
	for _, file := range files {
		if strings.HasPrefix(file.RelativePath, "pdf/images/") {
			if _, ok := result[file.RelativePath]; !ok {
				return nil, errors.New("PDF snapshot contains an unrecorded page image")
			}
		}
	}
	return result, nil
}

func pdfSnapshotUsesCurrentImageSemantics(files []DirectoryInputFile) bool {
	for _, file := range files {
		if file.RelativePath != pdfSnapshotRecordPath {
			continue
		}
		var record PDFExtractionRecord
		if json.Unmarshal(file.Content, &record) != nil {
			return false
		}
		return record.SchemaVersion == pdfExtractionSchema &&
			record.ImageRepresentation == pdfImageRepresentation &&
			record.AppearanceSemantics == pdfAppearanceSemantics &&
			record.AppearanceLimitations == pdfAppearanceLimitations &&
			record.ImageNumberMeaning == pdfImageNumberMeaning
	}
	return false
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
	var schemaVersion string
	for _, file := range files {
		if file.RelativePath == pdfSnapshotOriginalPath {
			source = file.Content
		}
		if file.RelativePath == pdfSnapshotRecordPath {
			var record PDFExtractionRecord
			if json.Unmarshal(file.Content, &record) != nil {
				return PreparedUpload{}, &UploadError{ReasonCode: "pdf_snapshot_invalid"}
			}
			schemaVersion = record.SchemaVersion
		}
	}
	if len(source) == 0 {
		return PreparedUpload{}, &UploadError{ReasonCode: "pdf_snapshot_invalid"}
	}
	var verified PreparedUpload
	var rebuilt []byte
	switch schemaVersion {
	case pdfExtractionSchemaV1:
		verified, rebuilt, err = prepareLegacyPDFMissionInput(prepared.DisplayName, source)
	case pdfExtractionSchemaV2:
		verified, rebuilt, err = preparePDFMissionInputV2(prepared.DisplayName, source)
	case pdfExtractionSchema:
		verified, rebuilt, err = preparePDFMissionInput(prepared.DisplayName, "application/pdf", source)
	default:
		return PreparedUpload{}, &UploadError{ReasonCode: "pdf_snapshot_invalid"}
	}
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

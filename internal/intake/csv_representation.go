// pattern: Functional Core
package intake

import (
	"bytes"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"unicode/utf8"
)

const (
	MaxModelCSVInputs        = 4
	MaxCSVPreviewRows        = 8
	MaxCSVPreviewBytes       = 8 << 10
	MaxCSVHeaderBytes        = 8 << 10
	MaxCSVHeaderFieldBytes   = 256
	MaxCSVRangeRows          = 100
	MaxCSVRangeResponseBytes = 128 << 10
)

// CSVColumnSummary carries a bounded display name and a non-authoritative
// inferred type. NameTruncated means the exact header field remains available
// through a range read; the raw CSV bytes are never rewritten.
type CSVColumnSummary struct {
	Index         int    `json:"index"`
	Name          string `json:"name"`
	HeaderMissing bool   `json:"headerMissing,omitempty"`
	NameTruncated bool   `json:"nameTruncated,omitempty"`
	InferredType  string `json:"inferredType"`
}

type CSVTableSummary struct {
	Reference        ModelInputManifestEntry `json:"reference"`
	ManifestDigest   string                  `json:"manifestDigest"`
	SourceDigest     string                  `json:"sourceDigest"`
	Delimiter        string                  `json:"delimiter"`
	Encoding         string                  `json:"encoding"`
	HeaderRow        int                     `json:"headerRow"`
	RowCount         int                     `json:"rowCount"`
	ColumnCount      int                     `json:"columnCount"`
	TypeSampleRows   int                     `json:"typeSampleRows"`
	HeaderDigest     string                  `json:"headerDigest"`
	HeadersTruncated bool                    `json:"headersTruncated,omitempty"`
	Columns          []CSVColumnSummary      `json:"columns"`
	Preview          CSVRowRange             `json:"preview"`
}

// CSVRowRange uses one-based data-row numbers. StartRow=0 is reserved for a
// read of the exact header record. Fields remain raw cell strings; formula-like
// values are neither evaluated nor rewritten.
type CSVRowRange struct {
	ReadReference  string     `json:"readReference,omitempty"`
	InputID        string     `json:"inputId"`
	Revision       int64      `json:"revision"`
	ManifestDigest string     `json:"manifestDigest"`
	SourceDigest   string     `json:"sourceDigest"`
	StartRow       int        `json:"startRow"`
	Rows           [][]string `json:"rows"`
	ReturnedRows   int        `json:"returnedRows"`
	NextRow        int        `json:"nextRow,omitempty"`
	Truncated      bool       `json:"truncated,omitempty"`
	RangeDigest    string     `json:"rangeDigest"`
}

func ProviderCSVInputEligible(reference ModelInputManifestEntry) bool {
	return reference.SourceKind == "upload" && reference.State == StateUsable && reference.MediaType == "text/csv" &&
		reference.ByteSize > 0 && reference.ByteSize <= MaxUploadBytes
}

// PrepareCSVTableSummary scans the validated original and returns only bounded
// metadata and an initial row range. SourceDigest always refers to the exact
// immutable upload bytes, including a BOM when present.
func PrepareCSVTableSummary(reference ModelInputManifestEntry, manifestDigest string, content []byte) (CSVTableSummary, error) {
	if !ProviderCSVInputEligible(reference) || !validModelInputDigest(manifestDigest) ||
		int64(len(content)) != reference.ByteSize || sha256Digest(content) != reference.ContentDigest || !utf8.Valid(content) {
		return CSVTableSummary{}, errors.New("CSV bytes do not match the frozen input revision")
	}
	encoding := "utf-8"
	if bytes.HasPrefix(content, []byte{0xef, 0xbb, 0xbf}) {
		encoding = "utf-8-bom"
	}
	reader := newBoundedCSVReader(content)
	reader.ReuseRecord = true
	header, err := reader.Read()
	if err != nil || len(header) == 0 || len(header) > maxCSVColumns {
		return CSVTableSummary{}, errors.New("CSV header cannot be read")
	}
	headerCopy := append([]string(nil), header...)
	headerBytes, err := json.Marshal(headerCopy)
	if err != nil {
		return CSVTableSummary{}, err
	}
	summary := CSVTableSummary{
		Reference: reference, ManifestDigest: manifestDigest, SourceDigest: reference.ContentDigest,
		Delimiter: ",", Encoding: encoding, HeaderRow: 1,
		HeaderDigest: sha256Digest(headerBytes),
	}
	headerBudget := MaxCSVHeaderBytes
	headerNameTruncated := make([]bool, len(headerCopy))
	columnTypes := make([]string, len(headerCopy))
	for index := range columnTypes {
		columnTypes[index] = "unknown"
	}
	for index, name := range headerCopy {
		shown := name
		truncated := false
		if len(shown) > MaxCSVHeaderFieldBytes {
			shown = shown[:MaxCSVHeaderFieldBytes]
			for !utf8.ValidString(shown) {
				shown = shown[:len(shown)-1]
			}
			truncated = true
		}
		if len(shown) > headerBudget {
			shown = ""
			truncated = true
		}
		headerBudget -= len(shown)
		summary.HeadersTruncated = summary.HeadersTruncated || truncated
		headerNameTruncated[index] = truncated
		headerCopy[index] = shown
	}
	preview := CSVRowRange{InputID: reference.InputID, Revision: reference.Revision, ManifestDigest: manifestDigest,
		SourceDigest: reference.ContentDigest, StartRow: 1, Rows: make([][]string, 0, MaxCSVPreviewRows)}
	previewBytes := 0
	dataRow := 0
	columnCount := len(header)
	for {
		fields, readErr := reader.Read()
		if readErr == io.EOF {
			break
		}
		if readErr != nil || len(fields) == 0 || len(fields) > maxCSVColumns {
			return CSVTableSummary{}, errors.New("CSV rows exceed the frozen table bounds")
		}
		dataRow++
		if len(fields) > columnCount {
			columnCount = len(fields)
			for len(columnTypes) < columnCount {
				columnTypes = append(columnTypes, "unknown")
				headerCopy = append(headerCopy, "")
				headerNameTruncated = append(headerNameTruncated, false)
			}
		}
		if dataRow <= 100 {
			for column, field := range fields {
				columnTypes[column] = inferCSVType(columnTypes[column], field)
			}
		}
		if len(preview.Rows) < MaxCSVPreviewRows && !preview.Truncated {
			rowCopy := append([]string(nil), fields...)
			encoded, marshalErr := json.Marshal(rowCopy)
			if marshalErr != nil {
				return CSVTableSummary{}, marshalErr
			}
			if previewBytes+len(encoded) > MaxCSVPreviewBytes {
				preview.Truncated = true
				preview.NextRow = dataRow
			} else {
				previewBytes += len(encoded)
				preview.Rows = append(preview.Rows, rowCopy)
			}
		}
		if len(preview.Rows) == MaxCSVPreviewRows && !preview.Truncated {
			preview.Truncated = true
			preview.NextRow = dataRow + 1
		}
		if dataRow >= 100 {
			break
		}
	}
	summary.RowCount = countCSVDataRows(content)
	if summary.RowCount < 0 {
		return CSVTableSummary{}, errors.New("CSV row count could not be verified")
	}
	summary.TypeSampleRows = summary.RowCount
	if summary.TypeSampleRows > 100 {
		summary.TypeSampleRows = 100
	}
	summary.ColumnCount = columnCount
	summary.Columns = make([]CSVColumnSummary, columnCount)
	for index := range summary.Columns {
		name := ""
		truncated := false
		headerMissing := index >= len(header)
		if index < len(headerCopy) {
			name = headerCopy[index]
			truncated = headerNameTruncated[index]
		}
		inferred := "unknown"
		if index < len(columnTypes) {
			inferred = columnTypes[index]
		}
		summary.Columns[index] = CSVColumnSummary{Index: index + 1, Name: name, HeaderMissing: headerMissing, NameTruncated: truncated, InferredType: inferred}
	}
	preview.ReturnedRows = len(preview.Rows)
	preview.RangeDigest = csvRangeDigest(preview)
	summary.Preview = preview
	return summary, nil
}

// ReadCSVRowRange returns a bounded, immutable-revision-bound logical row
// range. startRow=0 selects the header; positive rows select data rows starting
// at one. A response never exceeds MaxCSVRangeRows or the configured byte cap.
func ReadCSVRowRange(reference ModelInputManifestEntry, manifestDigest string, content []byte, startRow, maxRows int) (CSVRowRange, error) {
	if !ProviderCSVInputEligible(reference) || !validModelInputDigest(manifestDigest) ||
		int64(len(content)) != reference.ByteSize || sha256Digest(content) != reference.ContentDigest || !validBoundedCSV(content) ||
		startRow < 0 || maxRows < 1 || maxRows > MaxCSVRangeRows {
		return CSVRowRange{}, errors.New("CSV range request does not match a bounded frozen source")
	}
	rowCount := countCSVDataRows(content)
	if rowCount < 0 || (startRow > 0 && startRow > rowCount) {
		return CSVRowRange{}, errors.New("CSV range starts beyond the available data rows")
	}
	reader := newBoundedCSVReader(content)
	reader.ReuseRecord = false
	if startRow == 0 {
		header, err := reader.Read()
		if err != nil {
			return CSVRowRange{}, errors.New("CSV header range is unavailable")
		}
		encoded, marshalErr := json.Marshal(header)
		if marshalErr != nil || len(encoded) > MaxCSVRangeResponseBytes {
			return CSVRowRange{}, errors.New("CSV header exceeds the bounded range response")
		}
		return makeCSVRange(reference, manifestDigest, 0, [][]string{append([]string(nil), header...)}, false, 0), nil
	}
	if _, err := reader.Read(); err != nil {
		return CSVRowRange{}, errors.New("CSV header range is unavailable")
	}
	for skipped := 1; skipped < startRow; skipped++ {
		if _, err := reader.Read(); err != nil {
			return CSVRowRange{}, errors.New("CSV range starts beyond the available rows")
		}
	}
	rows := make([][]string, 0, maxRows)
	usedBytes := 0
	truncated, nextRow := false, 0
	for len(rows) < maxRows {
		fields, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return CSVRowRange{}, errors.New("CSV range could not be parsed")
		}
		row := append([]string(nil), fields...)
		encoded, marshalErr := json.Marshal(row)
		if marshalErr != nil {
			return CSVRowRange{}, marshalErr
		}
		if usedBytes+len(encoded) > MaxCSVRangeResponseBytes {
			if len(rows) == 0 {
				return CSVRowRange{}, errors.New("single CSV row exceeds the bounded range response")
			}
			truncated, nextRow = true, startRow+len(rows)
			break
		}
		usedBytes += len(encoded)
		rows = append(rows, row)
	}
	if len(rows) == maxRows {
		if _, err := reader.Read(); err != io.EOF {
			truncated, nextRow = true, startRow+len(rows)
		}
	}
	return makeCSVRange(reference, manifestDigest, startRow, rows, truncated, nextRow), nil
}

func newBoundedCSVReader(content []byte) *csv.Reader {
	content = bytes.TrimPrefix(content, []byte{0xef, 0xbb, 0xbf})
	reader := csv.NewReader(bytes.NewReader(content))
	reader.FieldsPerRecord = -1
	reader.LazyQuotes = false
	reader.TrimLeadingSpace = false
	return reader
}

func countCSVDataRows(content []byte) int {
	reader := newBoundedCSVReader(content)
	count := 0
	for {
		_, err := reader.Read()
		if err == io.EOF {
			return count - 1
		}
		if err != nil {
			return -1
		}
		count++
		if count > maxCSVRecords {
			return -1
		}
	}
}

func inferCSVType(current, value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return current
	}
	if strings.EqualFold(value, "true") || strings.EqualFold(value, "false") {
		return mergeCSVType(current, "boolean")
	}
	if isCSVInteger(value) {
		return mergeCSVType(current, "integer")
	}
	if isCSVNumber(value) {
		return mergeCSVType(current, "number")
	}
	if strings.HasPrefix(value, "=") || strings.HasPrefix(value, "+") || strings.HasPrefix(value, "-") || strings.HasPrefix(value, "@") {
		return mergeCSVType(current, "text")
	}
	return mergeCSVType(current, "text")
}

func mergeCSVType(current, next string) string {
	if current == "unknown" || current == "empty" {
		return next
	}
	if current == next {
		return current
	}
	if current == "integer" && next == "number" || current == "number" && next == "integer" {
		return "number"
	}
	return "text"
}

func isCSVInteger(value string) bool {
	if value == "" {
		return false
	}
	start := 0
	if value[0] == '-' || value[0] == '+' {
		start = 1
	}
	if start == len(value) {
		return false
	}
	for _, r := range value[start:] {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func isCSVNumber(value string) bool {
	seenDigit, seenDot := false, false
	for index, r := range value {
		switch {
		case index == 0 && (r == '-' || r == '+'):
		case r >= '0' && r <= '9':
			seenDigit = true
		case r == '.' && !seenDot:
			seenDot = true
		default:
			return false
		}
	}
	return seenDigit && seenDot
}

func makeCSVRange(reference ModelInputManifestEntry, manifestDigest string, startRow int, rows [][]string, truncated bool, nextRow int) CSVRowRange {
	result := CSVRowRange{InputID: reference.InputID, Revision: reference.Revision, ManifestDigest: manifestDigest,
		SourceDigest: reference.ContentDigest, StartRow: startRow, Rows: rows, ReturnedRows: len(rows),
		NextRow: nextRow, Truncated: truncated}
	result.RangeDigest = csvRangeDigest(result)
	return result
}

func csvRangeDigest(result CSVRowRange) string {
	result.RangeDigest = ""
	encoded, _ := json.Marshal(result)
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:])
}

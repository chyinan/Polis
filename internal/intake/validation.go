// pattern: Functional Core
package intake

import (
	"bytes"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"mime"
	"path"
	"strings"
	"unicode"
	"unicode/utf8"
)

const MaxUploadBytes = 8 << 20

const (
	maxCSVRecords    = 100_000
	maxCSVColumns    = 256
	maxCSVFieldBytes = 64 << 10
)

type State string

const (
	StateUsable      State = "usable"
	StatePartial     State = "partial"
	StateUnsupported State = "unsupported"
	StateUploading   State = "uploading"
	StateStored      State = "stored"
	StateRejected    State = "rejected"
)

type PreparedUpload struct {
	SourceKind         string
	DisplayName        string
	MediaType          string
	ByteSize           int64
	ContentDigest      string
	State              State
	TextRepresentation string
	ImageWidth         int
	ImageHeight        int
}

type UploadError struct {
	ReasonCode string
}

func (e *UploadError) Error() string {
	return fmt.Sprintf("invalid input upload: %s", e.ReasonCode)
}

// PrepareUpload validates an upload and returns the metadata for the bytes that
// must be stored. For PDFs, the stored bytes are a canonical extraction
// package; callers that persist the upload must use PrepareMissionInput.
func PrepareUpload(filename, declaredMediaType string, content []byte) (PreparedUpload, error) {
	prepared, _, err := PrepareMissionInput(filename, declaredMediaType, content)
	return prepared, err
}

// PrepareMissionInput returns upload metadata together with the exact bytes to
// persist. Most inputs preserve their submitted bytes. PDFs are wrapped with
// their original bytes, bounded extracted text, and extraction metadata.
func PrepareMissionInput(filename, declaredMediaType string, content []byte) (PreparedUpload, []byte, error) {
	if strings.EqualFold(path.Ext(strings.ReplaceAll(filename, "\\", "/")), ".pdf") {
		return preparePDFMissionInput(filename, declaredMediaType, content)
	}
	prepared, err := prepareUploadFile(filename, declaredMediaType, content)
	if err != nil {
		return PreparedUpload{}, nil, err
	}
	return prepared, content, nil
}

// prepareUploadFile validates one original file without transforming it. It
// is used for members of directory and ZIP snapshots, where nested PDFs remain
// preserved source files rather than triggering another parser pass.
func prepareUploadFile(filename, declaredMediaType string, content []byte) (PreparedUpload, error) {
	displayName, err := safeDisplayName(filename)
	if err != nil {
		return PreparedUpload{}, err
	}
	if len(content) == 0 {
		return PreparedUpload{}, &UploadError{ReasonCode: "empty_file"}
	}
	if len(content) > MaxUploadBytes {
		return PreparedUpload{}, &UploadError{ReasonCode: "upload_too_large"}
	}

	declared, err := normalizeMediaType(declaredMediaType)
	if err != nil {
		return PreparedUpload{}, err
	}
	sum := sha256.Sum256(content)
	prepared := PreparedUpload{
		SourceKind:    "upload",
		DisplayName:   displayName,
		MediaType:     "application/octet-stream",
		ByteSize:      int64(len(content)),
		ContentDigest: hex.EncodeToString(sum[:]),
		State:         StateUnsupported,
	}

	extension := strings.ToLower(path.Ext(displayName))
	if extension == ".zip" {
		if declared != "application/x-zip-compressed" && !compatibleDeclaredMediaType(declared, "application/zip", "application/octet-stream") {
			return PreparedUpload{}, &UploadError{ReasonCode: "media_type_mismatch"}
		}
		files, err := ExtractVerifiedZIPFiles(content)
		if err != nil {
			return PreparedUpload{}, err
		}
		usableTextFiles := 0
		allFilesSupported := true
		for _, file := range files {
			if isProviderTextMediaType(file.MediaType) {
				usableTextFiles++
			} else {
				allFilesSupported = false
			}
		}
		prepared.SourceKind = "zip_snapshot"
		prepared.MediaType = "application/zip"
		prepared.TextRepresentation = ""
		switch {
		case usableTextFiles == 0:
			prepared.State = StateUnsupported
		case allFilesSupported:
			prepared.State = StateUsable
		default:
			prepared.State = StatePartial
		}
		return prepared, nil
	}
	if extension == ".png" || extension == ".jpg" || extension == ".jpeg" {
		config, format, decodeErr := image.DecodeConfig(bytes.NewReader(content))
		if decodeErr != nil || config.Width <= 0 || config.Height <= 0 || int64(config.Width)*int64(config.Height) > maxImagePixels {
			return PreparedUpload{}, &UploadError{ReasonCode: "image_invalid"}
		}
		expectedFormat := "png"
		expectedMediaType := "image/png"
		if extension == ".jpg" || extension == ".jpeg" {
			expectedFormat = "jpeg"
			expectedMediaType = "image/jpeg"
		}
		if format != expectedFormat || !compatibleDeclaredMediaType(declared, expectedMediaType, "application/octet-stream") {
			return PreparedUpload{}, &UploadError{ReasonCode: "media_type_mismatch"}
		}
		prepared.MediaType = expectedMediaType
		prepared.State = StatePartial
		prepared.ImageWidth = config.Width
		prepared.ImageHeight = config.Height
		return prepared, nil
	}

	canonicalMediaType, supported := textMediaType(extension)
	if !supported {
		return prepared, nil
	}
	if !compatibleDeclaredMediaType(declared, canonicalMediaType, "text/plain") {
		return PreparedUpload{}, &UploadError{ReasonCode: "media_type_mismatch"}
	}
	if !utf8.Valid(content) || bytes.IndexByte(content, 0) >= 0 {
		return PreparedUpload{}, &UploadError{ReasonCode: "text_encoding_invalid"}
	}
	if canonicalMediaType == "text/csv" && !validBoundedCSV(content) {
		return PreparedUpload{}, &UploadError{ReasonCode: "csv_invalid"}
	}
	prepared.MediaType = canonicalMediaType
	prepared.State = StateUsable
	prepared.TextRepresentation = string(content)
	return prepared, nil
}

const maxImagePixels int64 = 40_000_000

func safeDisplayName(filename string) (string, error) {
	normalized := strings.ReplaceAll(filename, "\\", "/")
	displayName := path.Base(normalized)
	if strings.TrimSpace(displayName) == "" || displayName == "." || displayName == ".." || len(displayName) > 255 || strings.IndexFunc(displayName, unicode.IsControl) >= 0 {
		return "", &UploadError{ReasonCode: "filename_invalid"}
	}
	return displayName, nil
}

func normalizeMediaType(value string) (string, error) {
	if strings.TrimSpace(value) == "" {
		return "", nil
	}
	mediaType, _, err := mime.ParseMediaType(strings.TrimSpace(value))
	if err != nil {
		return "", &UploadError{ReasonCode: "media_type_invalid"}
	}
	return strings.ToLower(mediaType), nil
}

func compatibleDeclaredMediaType(declared, canonical, generic string) bool {
	return declared == "" || declared == "application/octet-stream" || declared == generic || declared == canonical || (strings.HasPrefix(canonical, "text/") && declared == "text/plain")
}

func validBoundedCSV(content []byte) bool {
	content = bytes.TrimPrefix(content, []byte{0xef, 0xbb, 0xbf})
	reader := csv.NewReader(bytes.NewReader(content))
	reader.FieldsPerRecord = -1
	reader.ReuseRecord = true
	records := 0
	for {
		fields, err := reader.Read()
		if err == io.EOF {
			return records > 0
		}
		if err != nil || len(fields) == 0 || len(fields) > maxCSVColumns {
			return false
		}
		records++
		if records > maxCSVRecords {
			return false
		}
		for _, field := range fields {
			if len(field) > maxCSVFieldBytes {
				return false
			}
		}
	}
}

func textMediaType(extension string) (string, bool) {
	switch extension {
	case ".md":
		return "text/markdown", true
	case ".json":
		return "application/json", true
	case ".csv":
		return "text/csv", true
	case ".txt", ".go", ".ts", ".tsx", ".js", ".jsx", ".html", ".css", ".yaml", ".yml", ".toml", ".sql", ".rs", ".py", ".sh", ".ps1":
		return "text/plain", true
	default:
		return "", false
	}
}

func MissionAllowsInput(state string) bool {
	return state == "draft" || state == "active" || state == "paused"
}

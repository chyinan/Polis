// pattern: Functional Core
package intake

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"io"
	"strings"
	"testing"
)

func TestPrepareUploadReturnsUsableUTF8TextAndContentDigest(t *testing.T) {
	content := []byte("# Goal\nImplement the approved feature.\n")
	got, err := PrepareUpload(`C:\Users\alice\mission\goal.md`, "text/markdown", content)
	if err != nil {
		t.Fatalf("PrepareUpload() error = %v", err)
	}
	sum := sha256.Sum256(content)
	if got.DisplayName != "goal.md" || got.MediaType != "text/markdown" || got.State != StateUsable {
		t.Fatalf("unexpected upload metadata: %+v", got)
	}
	if got.ContentDigest != hex.EncodeToString(sum[:]) || got.TextRepresentation != string(content) {
		t.Fatalf("unexpected stored representation: %+v", got)
	}
}

func TestPrepareUploadAcceptsGenericBrowserMediaTypeForRecognizedText(t *testing.T) {
	content := []byte("# Goal\nValid UTF-8 text.\n")
	got, err := PrepareUpload("goal.md", "application/octet-stream", content)
	if err != nil {
		t.Fatalf("PrepareUpload() error = %v", err)
	}
	if got.State != StateUsable || got.MediaType != "text/markdown" || got.TextRepresentation != string(content) {
		t.Fatalf("unexpected text upload: %+v", got)
	}
}

func TestPrepareUploadValidatesCSVAsBoundedTextInput(t *testing.T) {
	content := []byte("name,value\r\n\"two, words\",7\r\nnotes,\"a line\nbreak\"\r\n")
	got, err := PrepareUpload("metrics.csv", "text/csv", content)
	if err != nil {
		t.Fatal(err)
	}
	if got.State != StateUsable || got.MediaType != "text/csv" || got.TextRepresentation != string(content) {
		t.Fatalf("valid CSV representation = %+v", got)
	}
	if _, err := PrepareUpload("broken.csv", "text/csv", []byte("name,value\n\"unclosed,1\n")); uploadReason(err) != "csv_invalid" {
		t.Fatalf("malformed CSV error = %v, want csv_invalid", err)
	}
}

func TestPrepareUploadRejectsCSVColumnAndFieldBounds(t *testing.T) {
	tooManyColumns := strings.Repeat("x,", maxCSVColumns) + "x\n"
	tooLargeField := strings.Repeat("x", maxCSVFieldBytes+1) + ",value\n"
	for name, content := range map[string]string{"columns": tooManyColumns, "field": tooLargeField} {
		t.Run(name, func(t *testing.T) {
			if _, err := PrepareUpload("bounded.csv", "text/csv", []byte(content)); uploadReason(err) != "csv_invalid" {
				t.Fatalf("CSV %s bound error = %v, want csv_invalid", name, err)
			}
		})
	}
}

func TestPrepareUploadKeepsValidatedImageAsPartialInput(t *testing.T) {
	var encoded bytes.Buffer
	source := image.NewRGBA(image.Rect(0, 0, 2, 3))
	source.Set(0, 0, color.RGBA{R: 255, A: 255})
	if err := png.Encode(&encoded, source); err != nil {
		t.Fatal(err)
	}

	got, err := PrepareUpload("screen.png", "image/png", encoded.Bytes())
	if err != nil {
		t.Fatalf("PrepareUpload() error = %v", err)
	}
	if got.State != StatePartial || got.MediaType != "image/png" || got.ImageWidth != 2 || got.ImageHeight != 3 {
		t.Fatalf("unexpected image representation: %+v", got)
	}
	if got.TextRepresentation != "" {
		t.Fatalf("image bytes were mislabeled as text: %+v", got)
	}
}

func TestPrepareUploadRejectsMislabeledOrInvalidText(t *testing.T) {
	var encoded bytes.Buffer
	if err := jpeg.Encode(&encoded, image.NewRGBA(image.Rect(0, 0, 1, 1)), nil); err != nil {
		t.Fatal(err)
	}
	if _, err := PrepareUpload("photo.jpg", "image/png", encoded.Bytes()); uploadReason(err) != "media_type_mismatch" {
		t.Fatalf("mislabeled image error = %v", err)
	}
	if _, err := PrepareUpload("notes.md", "text/markdown", []byte{0xff, 0xfe}); uploadReason(err) != "text_encoding_invalid" {
		t.Fatalf("invalid UTF-8 text error = %v", err)
	}
}

func TestPrepareUploadRejectsEmptyOversizedAndInvalidNames(t *testing.T) {
	for _, test := range []struct {
		name, filename string
		content        []byte
		want           string
	}{
		{name: "empty", filename: "empty.txt", content: nil, want: "empty_file"},
		{name: "oversized", filename: "large.txt", content: bytes.Repeat([]byte{'x'}, MaxUploadBytes+1), want: "upload_too_large"},
		{name: "missing name", filename: "  ", content: []byte("text"), want: "filename_invalid"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := PrepareUpload(test.filename, "text/plain", test.content); uploadReason(err) != test.want {
				t.Fatalf("PrepareUpload() error = %v, want reason %q", err, test.want)
			}
		})
	}
}

func TestPrepareUploadMarksOtherFormatsUnsupportedWithoutLosingDigest(t *testing.T) {
	content := zipForTest(t, map[string]string{
		"repo/README.md": "project notes",
		"repo/image.bin": "opaque bytes",
	})
	got, err := PrepareUpload("archive.zip", "application/zip", content)
	if err != nil {
		t.Fatalf("PrepareUpload() error = %v", err)
	}
	sum := sha256.Sum256(content)
	if got.SourceKind != "zip_snapshot" || got.MediaType != "application/zip" || got.State != StatePartial || got.ContentDigest != hex.EncodeToString(sum[:]) || got.TextRepresentation != "" {
		t.Fatalf("unexpected ZIP source metadata: %+v", got)
	}
}

func TestPrepareUploadAcceptsWindowsZIPMediaType(t *testing.T) {
	content := zipForTest(t, map[string]string{"repo/README.md": "project text"})
	got, err := PrepareUpload("project.zip", "application/x-zip-compressed", content)
	if err != nil {
		t.Fatal(err)
	}
	if got.SourceKind != "zip_snapshot" || got.State != StateUsable || got.MediaType != "application/zip" {
		t.Fatalf("Windows ZIP upload metadata = %+v", got)
	}
}

func zipForTest(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	for name, content := range files {
		entry, err := writer.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = io.WriteString(entry, content); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}

func TestMissionStateAllowsInputOnlyWhileEditable(t *testing.T) {
	for state, want := range map[string]bool{
		"draft": true, "active": true, "paused": true,
		"succeeded": false, "cancelled": false, "": false,
	} {
		if got := MissionAllowsInput(state); got != want {
			t.Errorf("MissionAllowsInput(%q) = %t, want %t", state, got, want)
		}
	}
}

func uploadReason(err error) string {
	if err == nil {
		return ""
	}
	uploadErr, ok := err.(*UploadError)
	if !ok {
		return "unexpected_error_type"
	}
	return uploadErr.ReasonCode
}

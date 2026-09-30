// pattern: Imperative Shell
package workbench

import (
	"bytes"
	"errors"
	"mime/multipart"
	"net/http/httptest"
	"testing"

	"polis/internal/intake"
)

func TestParseMissionDirectoryUploadPairsRelativePathsWithFiles(t *testing.T) {
	body, contentType := directoryMultipart(t, "requestId", "directory-1", "inputId", "source-tree", "paths", "project/README.md", "file", "README.md", []byte("# Project\n"))
	request := httptest.NewRequest("POST", "/", body)
	request.Header.Set("Content-Type", contentType)
	parsedRequest, files, err := parseMissionDirectoryUpload(httptest.NewRecorder(), request, "mission-1")
	if err != nil {
		t.Fatal(err)
	}
	if parsedRequest.MissionID != "mission-1" || parsedRequest.RequestID != "directory-1" || parsedRequest.InputID != "source-tree" {
		t.Fatalf("parsed request = %+v", parsedRequest)
	}
	if len(files) != 1 || files[0].RelativePath != "project/README.md" || string(files[0].Content) != "# Project\n" {
		t.Fatalf("parsed files = %+v", files)
	}
	if _, err = intake.PrepareDirectorySnapshot(files); err != nil {
		t.Fatalf("parsed directory should validate: %v", err)
	}
}

func TestParseMissionDirectoryUploadRejectsUnpairedPathsAndUnknownFields(t *testing.T) {
	for _, fields := range [][]any{
		{"requestId", "directory-2", "paths", "project/README.md", "paths", "other/file.txt", "file", "README.md", []byte("# Project\n")},
		{"requestId", "directory-3", "paths", "project/README.md", "inputId", "tree-1", "extra", "unexpected", "file", "README.md", []byte("x")},
	} {
		body, contentType := directoryMultipart(t, fields...)
		request := httptest.NewRequest("POST", "/", body)
		request.Header.Set("Content-Type", contentType)
		if _, _, err := parseMissionDirectoryUpload(httptest.NewRecorder(), request, "mission-1"); err == nil {
			t.Fatalf("accepted malformed directory fields: %v", fields)
		}
	}
}

func TestParseMissionDirectoryUploadBoundsRequestBody(t *testing.T) {
	content := bytes.Repeat([]byte("x"), intake.MaxDirectoryBytes+1)
	body, contentType := directoryMultipart(t, "requestId", "directory-large", "paths", "project/large.txt", "file", "large.txt", content)
	request := httptest.NewRequest("POST", "/", body)
	request.Header.Set("Content-Type", contentType)
	_, _, err := parseMissionDirectoryUpload(httptest.NewRecorder(), request, "mission-1")
	if !errors.Is(err, errMissionDirectoryInputTooLarge) {
		t.Fatalf("oversized directory error = %v", err)
	}
}

func directoryMultipart(t *testing.T, fields ...any) (*bytes.Buffer, string) {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	for index := 0; index < len(fields); {
		kind := fields[index].(string)
		index++
		switch kind {
		case "file":
			name := fields[index].(string)
			content := fields[index+1].([]byte)
			index += 2
			part, err := writer.CreateFormFile("files", name)
			if err != nil {
				t.Fatal(err)
			}
			if _, err = part.Write(content); err != nil {
				t.Fatal(err)
			}
		case "requestId", "inputId", "paths", "extra":
			value := fields[index].(string)
			index++
			if err := writer.WriteField(kind, value); err != nil {
				t.Fatal(err)
			}
		default:
			t.Fatalf("unknown test multipart field %q", kind)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return &body, writer.FormDataContentType()
}

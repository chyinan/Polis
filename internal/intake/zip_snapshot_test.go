// pattern: Functional Core
package intake

import (
	"archive/zip"
	"bytes"
	"os"
	"testing"
)

func TestPrepareUploadRejectsUnsafeZIPEntries(t *testing.T) {
	for _, test := range []struct {
		name    string
		entries []zipTestEntry
		want    string
	}{
		{name: "path traversal", entries: []zipTestEntry{{name: "../outside.txt", content: "escape"}}, want: "zip_path_invalid"},
		{name: "absolute path", entries: []zipTestEntry{{name: "/outside.txt", content: "escape"}}, want: "zip_path_invalid"},
		{name: "duplicate path", entries: []zipTestEntry{{name: "repo/readme.md", content: "one"}, {name: "repo/README.md", content: "two"}}, want: "zip_path_duplicate"},
		{name: "symlink", entries: []zipTestEntry{{name: "repo/link", content: "target", mode: os.ModeSymlink | 0o777}}, want: "zip_entry_type_unsupported"},
		{name: "nested archive", entries: []zipTestEntry{{name: "repo/inner.zip", content: "nested"}}, want: "zip_nested_archive_unsupported"},
		{name: "expanded size", entries: []zipTestEntry{{name: "repo/large.txt", content: string(bytes.Repeat([]byte{'x'}, MaxDirectoryBytes+1))}}, want: "zip_expanded_size_exceeded"},
	} {
		t.Run(test.name, func(t *testing.T) {
			archive := zipEntriesForTest(t, test.entries)
			if _, err := PrepareUpload("source.zip", "application/zip", archive); uploadReason(err) != test.want {
				t.Fatalf("unsafe ZIP error = %v, want reason %q", err, test.want)
			}
		})
	}
}

type zipTestEntry struct {
	name    string
	content string
	mode    os.FileMode
}

func zipEntriesForTest(t *testing.T, entries []zipTestEntry) []byte {
	t.Helper()
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	for _, item := range entries {
		header := &zip.FileHeader{Name: item.name, Method: zip.Deflate}
		if item.mode != 0 {
			header.SetMode(item.mode)
		}
		entry, err := writer.CreateHeader(header)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = entry.Write([]byte(item.content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}

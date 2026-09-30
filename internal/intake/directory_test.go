// pattern: Functional Core
package intake

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io"
	"testing"
)

func TestPrepareDirectorySnapshotBuildsDeterministicManifestAndArchive(t *testing.T) {
	files := []DirectoryInputFile{
		{RelativePath: "project/src/main.go", MediaType: "text/plain", Content: []byte("package main\n")},
		{RelativePath: "project/README.md", MediaType: "text/markdown", Content: []byte("# Project\n")},
	}
	first, err := PrepareDirectorySnapshot(files)
	if err != nil {
		t.Fatal(err)
	}
	second, err := PrepareDirectorySnapshot([]DirectoryInputFile{files[1], files[0]})
	if err != nil {
		t.Fatal(err)
	}
	if first.Upload.SourceKind != "directory_snapshot" || first.Upload.State != StateUsable || first.RootName != "project" {
		t.Fatalf("unexpected prepared directory: %+v", first.Upload)
	}
	if first.Upload.ContentDigest != second.Upload.ContentDigest || !bytes.Equal(first.Archive, second.Archive) {
		t.Fatal("directory snapshot is not deterministic")
	}
	if first.Upload.ByteSize != int64(len(first.Archive)) || first.Upload.ByteSize > MaxUploadBytes {
		t.Fatalf("archive size is not bounded and self-consistent: %d", first.Upload.ByteSize)
	}
	var manifest DirectoryManifest
	if err = json.Unmarshal(first.Manifest, &manifest); err != nil {
		t.Fatal(err)
	}
	if manifest.SchemaVersion != "polis-directory-snapshot@1" || len(manifest.Files) != 2 || manifest.Files[0].RelativePath != "project/README.md" {
		t.Fatalf("unexpected directory manifest: %+v", manifest)
	}
	if err = VerifyPreparedUpload(first.Upload, first.Archive); err != nil {
		t.Fatalf("generated directory archive did not verify: %v", err)
	}

	reader, err := gzip.NewReader(bytes.NewReader(first.Archive))
	if err != nil {
		t.Fatal(err)
	}
	tarReader := tar.NewReader(reader)
	seen := map[string]string{}
	for {
		header, nextErr := tarReader.Next()
		if nextErr == io.EOF {
			break
		}
		if nextErr != nil {
			t.Fatal(nextErr)
		}
		body, readErr := io.ReadAll(tarReader)
		if readErr != nil {
			t.Fatal(readErr)
		}
		seen[header.Name] = string(body)
	}
	if err = reader.Close(); err != nil {
		t.Fatal(err)
	}
	if seen["project/src/main.go"] != "package main\n" || seen["project/README.md"] != "# Project\n" {
		t.Fatalf("archive did not preserve selected files: %#v", seen)
	}
}

func TestPrepareDirectorySnapshotRejectsUnsafeOrAmbiguousPaths(t *testing.T) {
	unsafe := []string{"../escape.txt", "/root/file.txt", "C:/root/file.txt", `project\file.txt`, "project/../file.txt", "project/CON.txt", "project/a."}
	for _, name := range unsafe {
		t.Run(name, func(t *testing.T) {
			_, err := PrepareDirectorySnapshot([]DirectoryInputFile{{RelativePath: name, MediaType: "text/plain", Content: []byte("x")}})
			if err == nil {
				t.Fatalf("accepted unsafe directory path %q", name)
			}
		})
	}
	for _, files := range [][]DirectoryInputFile{
		{{RelativePath: "project/a.txt", MediaType: "text/plain", Content: []byte("a")}, {RelativePath: "project/A.txt", MediaType: "text/plain", Content: []byte("b")}},
		{{RelativePath: "project/a", MediaType: "text/plain", Content: []byte("a")}, {RelativePath: "project/a/b.txt", MediaType: "text/plain", Content: []byte("b")}},
		{{RelativePath: "project/a.txt", MediaType: "text/plain", Content: []byte("a")}, {RelativePath: "other/b.txt", MediaType: "text/plain", Content: []byte("b")}},
	} {
		if _, err := PrepareDirectorySnapshot(files); err == nil {
			t.Fatalf("accepted ambiguous paths: %+v", files)
		}
	}
}

func TestPrepareDirectorySnapshotPreservesNestedPDFWithoutParsingIt(t *testing.T) {
	snapshot, err := PrepareDirectorySnapshot([]DirectoryInputFile{
		{RelativePath: "project/source.pdf", MediaType: "application/pdf", Content: []byte("not parsed inside a directory snapshot")},
		{RelativePath: "project/README.md", MediaType: "text/markdown", Content: []byte("project notes")},
	})
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Upload.State != StatePartial {
		t.Fatalf("directory snapshot with nested PDF state = %q, want partial", snapshot.Upload.State)
	}
	files, err := ExtractVerifiedDirectoryFiles(snapshot.Archive)
	if err != nil {
		t.Fatal(err)
	}
	if !containsInputFile(files, "project/source.pdf", []byte("not parsed inside a directory snapshot")) {
		t.Fatal("nested PDF source bytes were not preserved")
	}
}

func TestPrepareDirectorySnapshotBoundsFileCountAndAggregateBytes(t *testing.T) {
	tooMany := make([]DirectoryInputFile, MaxDirectoryFiles+1)
	for index := range tooMany {
		tooMany[index] = DirectoryInputFile{RelativePath: "project/file" + string(rune('a'+index%26)) + ".txt", MediaType: "text/plain", Content: []byte("x")}
	}
	if _, err := PrepareDirectorySnapshot(tooMany); err == nil {
		t.Fatal("accepted more than the directory file limit")
	}
	large := []DirectoryInputFile{{RelativePath: "project/large.txt", MediaType: "text/plain", Content: bytes.Repeat([]byte("x"), MaxDirectoryBytes+1)}}
	if _, err := PrepareDirectorySnapshot(large); err == nil {
		t.Fatal("accepted an oversized directory snapshot")
	}
}

func TestPrepareDirectorySnapshotMarksMixedImageOrUnsupportedContentPartial(t *testing.T) {
	imagePNG := []byte{137, 80, 78, 71, 13, 10, 26, 10, 0, 0, 0, 13, 73, 72, 68, 82, 0, 0, 0, 1, 0, 0, 0, 1, 8, 6, 0, 0, 0, 31, 21, 196, 137, 0, 0, 0, 11, 73, 68, 65, 84, 120, 156, 99, 96, 0, 2, 0, 0, 5, 0, 1, 167, 38, 129, 36, 0, 0, 0, 0, 73, 69, 78, 68, 174, 66, 96, 130}
	prepared, err := PrepareDirectorySnapshot([]DirectoryInputFile{
		{RelativePath: "project/README.md", MediaType: "text/markdown", Content: []byte("# Project\n")},
		{RelativePath: "project/image.png", MediaType: "image/png", Content: imagePNG},
		{RelativePath: "project/archive.bin", MediaType: "application/octet-stream", Content: []byte{0, 1, 2}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if prepared.Upload.State != StatePartial {
		t.Fatalf("mixed directory state = %q, want partial", prepared.Upload.State)
	}
}

func TestVerifyPreparedUploadRejectsCorruptDirectoryArchive(t *testing.T) {
	prepared, err := PrepareDirectorySnapshot([]DirectoryInputFile{{RelativePath: "project/a.md", MediaType: "text/markdown", Content: []byte("# A")}})
	if err != nil {
		t.Fatal(err)
	}
	corrupt := append([]byte(nil), prepared.Archive...)
	corrupt[len(corrupt)/2] ^= 1
	if err = VerifyPreparedUpload(prepared.Upload, corrupt); err == nil {
		t.Fatal("accepted a corrupt directory archive")
	}
}

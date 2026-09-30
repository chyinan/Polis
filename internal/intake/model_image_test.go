// pattern: Functional Core
package intake

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"strings"
	"testing"
)

func TestPrepareModelInputContextIncludesValidatedImageBytesAndReceiptMetadata(t *testing.T) {
	imageBytes := testModelPNG(t)
	manifest, digest, err := PrepareModelInputManifest("company-image", "mission-image", "task-image", []MissionInputReference{{
		InputID: "image-input", Revision: 1, RequestID: "image-upload", SourceKind: "upload", DisplayName: "screen.png", MediaType: "image/png",
		ByteSize: int64(len(imageBytes)), ContentDigest: sha256Digest(imageBytes), State: StatePartial,
	}})
	if err != nil {
		t.Fatal(err)
	}
	if len(manifest.CandidateInputs) != 1 || len(manifest.ExcludedInputs) != 0 {
		t.Fatalf("validated image is not a Task manifest candidate: %+v", manifest)
	}
	prepared, err := PrepareModelInputContext(manifest, digest, map[string][]byte{"image-input": imageBytes})
	if err != nil {
		t.Fatal(err)
	}
	if len(prepared.Images) != 1 || len(prepared.Inputs) != 0 || !bytes.Equal(prepared.Images[0].Content, imageBytes) {
		t.Fatalf("image was not retained as a vision payload: %+v", prepared)
	}
	if prepared.Images[0].ContentDigest != sha256Digest(imageBytes) || prepared.Images[0].MediaType != "image/png" {
		t.Fatalf("image payload lost source identity: %+v", prepared.Images[0])
	}
	if !strings.Contains(prepared.PromptSection, "screen.png") || !strings.Contains(prepared.PromptSection, "image/png") || !strings.Contains(prepared.PromptSection, sha256Digest(imageBytes)) {
		t.Fatalf("image metadata is absent from the Worker context: %s", prepared.PromptSection)
	}
	if _, err = PrepareModelInputContext(manifest, digest, map[string][]byte{"image-input": []byte("not a PNG")}); err == nil {
		t.Fatal("image delivery accepted bytes that did not match the frozen digest")
	}
}

func TestPrepareModelInputContextRejectsMalformedImageRepresentation(t *testing.T) {
	malformed := []byte("not image bytes")
	manifest, digest, err := PrepareModelInputManifest("company-image", "mission-image", "task-image-invalid", []MissionInputReference{{
		InputID: "image-input", Revision: 1, RequestID: "image-upload-invalid", SourceKind: "upload", DisplayName: "broken.png", MediaType: "image/png",
		ByteSize: int64(len(malformed)), ContentDigest: sha256Digest(malformed), State: StatePartial,
	}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = PrepareModelInputContext(manifest, digest, map[string][]byte{"image-input": malformed}); err == nil {
		t.Fatal("image context accepted malformed pixels/metadata")
	}
}

func TestDirectoryImageSelectionIsIncludedAndReceiptVerifiable(t *testing.T) {
	imageBytes := testModelPNG(t)
	directory, err := PrepareDirectorySnapshot([]DirectoryInputFile{
		{RelativePath: "project/README.md", MediaType: "text/markdown", Content: []byte("The screenshot accompanies these instructions.")},
		{RelativePath: "project/screen.png", MediaType: "image/png", Content: imageBytes},
		{RelativePath: "project/data.bin", MediaType: "application/octet-stream", Content: []byte("opaque")},
	})
	if err != nil {
		t.Fatal(err)
	}
	manifest, digest, err := PrepareModelInputManifest("company-image", "mission-image", "task-directory-image", []MissionInputReference{{
		InputID: "directory-input", Revision: 1, RequestID: "directory-upload", SourceKind: "directory_snapshot", DisplayName: directory.RootName,
		MediaType: directory.Upload.MediaType, ByteSize: directory.Upload.ByteSize, ContentDigest: directory.Upload.ContentDigest, State: directory.Upload.State,
	}})
	if err != nil {
		t.Fatal(err)
	}
	prepared, err := PrepareModelInputContext(manifest, digest, map[string][]byte{"directory-input": directory.Archive})
	if err != nil {
		t.Fatal(err)
	}
	if len(prepared.Inputs) != 1 || len(prepared.Images) != 1 || len(prepared.Excluded) != 1 || prepared.Images[0].RelativePath != "project/screen.png" {
		t.Fatalf("directory image context=%+v", prepared)
	}
	included := make([]ModelInputDeliveryRef, 0, len(prepared.Inputs)+len(prepared.Images))
	for _, item := range prepared.Inputs {
		included = append(included, ModelInputDeliveryRef{InputID: item.Reference.InputID, RelativePath: item.RelativePath, MediaType: item.MediaType, ByteSize: item.ByteSize, ContentDigest: item.ContentDigest})
	}
	for _, item := range prepared.Images {
		included = append(included, ModelInputDeliveryRef{InputID: item.Reference.InputID, RelativePath: item.RelativePath, MediaType: item.MediaType, ByteSize: item.ByteSize, ContentDigest: item.ContentDigest})
	}
	files, err := ExtractVerifiedInputArchive(directory.Upload.SourceKind, directory.Archive)
	if err != nil {
		t.Fatal(err)
	}
	archiveFiles := make([]DirectoryInputFile, len(files))
	for index, file := range files {
		archiveFiles[index] = DirectoryInputFile{RelativePath: file.RelativePath, MediaType: file.MediaType, Content: file.Content}
	}
	if err = VerifyModelInputDeliverySelection(manifest, included, prepared.Excluded, map[string][]DirectoryInputFile{"directory-input": archiveFiles}); err != nil {
		t.Fatalf("directory image delivery receipt did not verify: %v", err)
	}
}

func testModelPNG(t *testing.T) []byte {
	t.Helper()
	imageValue := image.NewRGBA(image.Rect(0, 0, 2, 2))
	imageValue.Set(0, 0, color.RGBA{R: 255, A: 255})
	var buffer bytes.Buffer
	if err := png.Encode(&buffer, imageValue); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}

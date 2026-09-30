// pattern: Functional Core
package intake

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path"
	"sort"
	"strings"
	"time"
	"unicode"
)

const (
	DirectorySnapshotSchema          = "polis-directory-snapshot@1"
	MaxDirectoryFiles                = 250
	MaxDirectoryBytes                = 7 << 20
	MaxDirectoryManifest             = 256 << 10
	MaxDirectoryArchiveExpandedBytes = MaxDirectoryBytes + MaxDirectoryManifest + (MaxDirectoryFiles+2)*512 + MaxDirectoryFiles*2048
	DirectoryManifestName            = ".polis-input-manifest.json"
)

type DirectoryInputFile struct {
	RelativePath string
	MediaType    string
	Content      []byte
}

type DirectoryManifestEntry struct {
	RelativePath  string `json:"relativePath"`
	MediaType     string `json:"mediaType"`
	ByteSize      int64  `json:"byteSize"`
	ContentDigest string `json:"contentDigest"`
	State         State  `json:"state"`
}

type DirectoryManifest struct {
	SchemaVersion string                   `json:"schemaVersion"`
	RootName      string                   `json:"rootName"`
	Files         []DirectoryManifestEntry `json:"files"`
}

type PreparedDirectorySnapshot struct {
	RootName string
	Manifest []byte
	Archive  []byte
	Upload   PreparedUpload
}

func PrepareDirectorySnapshot(files []DirectoryInputFile) (PreparedDirectorySnapshot, error) {
	if len(files) == 0 || len(files) > MaxDirectoryFiles {
		return PreparedDirectorySnapshot{}, &UploadError{ReasonCode: "directory_file_count_invalid"}
	}
	preparedFiles := make([]DirectoryInputFile, len(files))
	copy(preparedFiles, files)
	sort.Slice(preparedFiles, func(i, j int) bool { return preparedFiles[i].RelativePath < preparedFiles[j].RelativePath })

	entries := make([]DirectoryManifestEntry, 0, len(preparedFiles))
	paths := make(map[string]struct{}, len(preparedFiles))
	rootName := ""
	totalBytes := int64(0)
	state := StateUsable
	for index, file := range preparedFiles {
		relativePath, fileRoot, pathErr := safeRelativeDirectoryPath(file.RelativePath)
		if pathErr != nil {
			return PreparedDirectorySnapshot{}, pathErr
		}
		if rootName == "" {
			rootName = fileRoot
		} else if rootName != fileRoot {
			return PreparedDirectorySnapshot{}, &UploadError{ReasonCode: "directory_root_mismatch"}
		}
		folded := strings.ToLower(relativePath)
		if _, duplicate := paths[folded]; duplicate {
			return PreparedDirectorySnapshot{}, &UploadError{ReasonCode: "directory_path_collision"}
		}
		for prior := range paths {
			if strings.HasPrefix(folded, prior+"/") || strings.HasPrefix(prior, folded+"/") {
				return PreparedDirectorySnapshot{}, &UploadError{ReasonCode: "directory_file_parent_collision"}
			}
		}
		paths[folded] = struct{}{}
		if len(file.Content) == 0 {
			return PreparedDirectorySnapshot{}, &UploadError{ReasonCode: "empty_file"}
		}
		if int64(len(file.Content)) > MaxUploadBytes {
			return PreparedDirectorySnapshot{}, &UploadError{ReasonCode: "upload_too_large"}
		}
		totalBytes += int64(len(file.Content))
		if totalBytes > MaxDirectoryBytes {
			return PreparedDirectorySnapshot{}, &UploadError{ReasonCode: "directory_too_large"}
		}
		prepared, err := prepareUploadFile(path.Base(relativePath), file.MediaType, file.Content)
		if err != nil {
			return PreparedDirectorySnapshot{}, err
		}
		if prepared.State != StateUsable {
			state = StatePartial
		}
		entries = append(entries, DirectoryManifestEntry{
			RelativePath: relativePath, MediaType: prepared.MediaType, ByteSize: prepared.ByteSize,
			ContentDigest: prepared.ContentDigest, State: prepared.State,
		})
		preparedFiles[index].RelativePath = relativePath
	}
	manifest := DirectoryManifest{SchemaVersion: DirectorySnapshotSchema, RootName: rootName, Files: entries}
	manifestBytes, err := json.Marshal(manifest)
	if err != nil {
		return PreparedDirectorySnapshot{}, err
	}
	if len(manifestBytes) > MaxDirectoryManifest {
		return PreparedDirectorySnapshot{}, &UploadError{ReasonCode: "directory_manifest_too_large"}
	}
	archiveBytes, err := buildDirectoryArchive(rootName, manifestBytes, preparedFiles)
	if err != nil {
		return PreparedDirectorySnapshot{}, err
	}
	if len(archiveBytes) == 0 || len(archiveBytes) > MaxUploadBytes {
		return PreparedDirectorySnapshot{}, &UploadError{ReasonCode: "directory_archive_too_large"}
	}
	digest := sha256.Sum256(archiveBytes)
	return PreparedDirectorySnapshot{
		RootName: rootName,
		Manifest: manifestBytes,
		Archive:  archiveBytes,
		Upload: PreparedUpload{
			SourceKind: "directory_snapshot", DisplayName: rootName, MediaType: "application/gzip",
			ByteSize: int64(len(archiveBytes)), ContentDigest: hex.EncodeToString(digest[:]), State: state,
		},
	}, nil
}

func VerifyPreparedUpload(prepared PreparedUpload, content []byte) error {
	var verified PreparedUpload
	var err error
	switch prepared.SourceKind {
	case "upload":
		verified, err = PrepareUpload(prepared.DisplayName, prepared.MediaType, content)
		if err != nil {
			return err
		}
	case "directory_snapshot":
		snapshot, err := verifyDirectoryArchive(content)
		if err != nil {
			return err
		}
		verified = snapshot.Upload
	case "zip_snapshot":
		verified, err = prepareUploadFile(prepared.DisplayName, "application/zip", content)
		if err != nil {
			return err
		}
	case "pdf_snapshot":
		verified, err = verifyPDFSnapshot(prepared, content)
		if err != nil {
			return err
		}
	case "git_snapshot":
		verifiedSnapshot, err := VerifyGitSnapshot(content)
		if err != nil {
			return err
		}
		verified = verifiedSnapshot.Upload
	default:
		return &UploadError{ReasonCode: "source_kind_invalid"}
	}
	if verified != prepared {
		return &UploadError{ReasonCode: "prepared_upload_mismatch"}
	}
	return nil
}

func safeRelativeDirectoryPath(value string) (string, string, error) {
	if value == "" || len(value) > 1024 || strings.Contains(value, "\\") || strings.HasPrefix(value, "/") || strings.ContainsRune(value, '\x00') {
		return "", "", &UploadError{ReasonCode: "directory_path_invalid"}
	}
	clean := path.Clean(value)
	if clean != value || clean == "." || strings.HasPrefix(clean, "../") || path.IsAbs(clean) {
		return "", "", &UploadError{ReasonCode: "directory_path_invalid"}
	}
	parts := strings.Split(clean, "/")
	if len(parts) < 2 {
		return "", "", &UploadError{ReasonCode: "directory_path_missing_root"}
	}
	for _, part := range parts {
		if part == "" || part == "." || part == ".." || strings.HasSuffix(part, ".") || strings.HasSuffix(part, " ") || strings.IndexFunc(part, unicode.IsControl) >= 0 || strings.ContainsAny(part, `<>:"|?*`) {
			return "", "", &UploadError{ReasonCode: "directory_path_invalid"}
		}
		deviceBase := strings.ToUpper(strings.TrimSuffix(part, path.Ext(part)))
		if deviceBase == "CON" || deviceBase == "PRN" || deviceBase == "AUX" || deviceBase == "NUL" || isWindowsNumberedDevice(deviceBase) {
			return "", "", &UploadError{ReasonCode: "directory_path_reserved"}
		}
	}
	return clean, parts[0], nil
}

func isWindowsNumberedDevice(value string) bool {
	if len(value) != 4 || (value[:3] != "COM" && value[:3] != "LPT") {
		return false
	}
	return value[3] >= '1' && value[3] <= '9'
}

func buildDirectoryArchive(rootName string, manifest []byte, files []DirectoryInputFile) ([]byte, error) {
	var output bytes.Buffer
	gzipWriter := gzip.NewWriter(&output)
	gzipWriter.Header.ModTime = time.Unix(0, 0).UTC()
	gzipWriter.Header.OS = 255
	tarWriter := tar.NewWriter(gzipWriter)
	writeEntry := func(name string, content []byte) error {
		header := &tar.Header{
			Name: name, Mode: 0o644, Size: int64(len(content)), ModTime: time.Unix(0, 0).UTC(),
			Typeflag: tar.TypeReg, Format: tar.FormatPAX,
		}
		if err := tarWriter.WriteHeader(header); err != nil {
			return err
		}
		_, err := tarWriter.Write(content)
		return err
	}
	if err := writeEntry(DirectoryManifestName, manifest); err != nil {
		_ = tarWriter.Close()
		_ = gzipWriter.Close()
		return nil, err
	}
	for _, file := range files {
		if _, _, err := safeRelativeDirectoryPath(file.RelativePath); err != nil {
			_ = tarWriter.Close()
			_ = gzipWriter.Close()
			return nil, err
		}
		if err := writeEntry(file.RelativePath, file.Content); err != nil {
			_ = tarWriter.Close()
			_ = gzipWriter.Close()
			return nil, err
		}
	}
	if err := tarWriter.Close(); err != nil {
		_ = gzipWriter.Close()
		return nil, err
	}
	if err := gzipWriter.Close(); err != nil {
		return nil, err
	}
	_ = rootName // rootName is bound in the manifest and checked during archive verification.
	return output.Bytes(), nil
}

func verifyDirectoryArchive(content []byte) (PreparedDirectorySnapshot, error) {
	if len(content) == 0 || len(content) > MaxUploadBytes {
		return PreparedDirectorySnapshot{}, &UploadError{ReasonCode: "directory_archive_size_invalid"}
	}
	gzipReader, err := gzip.NewReader(bytes.NewReader(content))
	if err != nil {
		return PreparedDirectorySnapshot{}, &UploadError{ReasonCode: "directory_archive_invalid"}
	}
	defer gzipReader.Close()
	tarReader := tar.NewReader(io.LimitReader(gzipReader, MaxDirectoryArchiveExpandedBytes))
	files := make([]DirectoryInputFile, 0, MaxDirectoryFiles)
	var manifest DirectoryManifest
	manifestRead := false
	seen := make(map[string]struct{}, MaxDirectoryFiles+1)
	totalBytes := int64(0)
	for {
		header, nextErr := tarReader.Next()
		if errors.Is(nextErr, io.EOF) {
			break
		}
		if nextErr != nil {
			return PreparedDirectorySnapshot{}, &UploadError{ReasonCode: "directory_archive_invalid"}
		}
		if header.Typeflag != tar.TypeReg || header.Size < 0 {
			return PreparedDirectorySnapshot{}, &UploadError{ReasonCode: "directory_entry_type_invalid"}
		}
		if _, duplicate := seen[header.Name]; duplicate {
			return PreparedDirectorySnapshot{}, &UploadError{ReasonCode: "directory_path_collision"}
		}
		seen[header.Name] = struct{}{}
		if header.Name == DirectoryManifestName {
			if manifestRead || len(files) != 0 || header.Size <= 0 || header.Size > MaxDirectoryManifest {
				return PreparedDirectorySnapshot{}, &UploadError{ReasonCode: "directory_manifest_invalid"}
			}
			manifestBytes, readErr := io.ReadAll(io.LimitReader(tarReader, header.Size+1))
			if readErr != nil || int64(len(manifestBytes)) != header.Size {
				return PreparedDirectorySnapshot{}, &UploadError{ReasonCode: "directory_manifest_invalid"}
			}
			decoder := json.NewDecoder(bytes.NewReader(manifestBytes))
			decoder.DisallowUnknownFields()
			if decoder.Decode(&manifest) != nil || manifest.SchemaVersion != DirectorySnapshotSchema || len(manifest.Files) == 0 || len(manifest.Files) > MaxDirectoryFiles {
				return PreparedDirectorySnapshot{}, &UploadError{ReasonCode: "directory_manifest_invalid"}
			}
			manifestRead = true
			continue
		}
		if !manifestRead || len(files) >= MaxDirectoryFiles {
			return PreparedDirectorySnapshot{}, &UploadError{ReasonCode: "directory_archive_order_invalid"}
		}
		clean, root, pathErr := safeRelativeDirectoryPath(header.Name)
		if pathErr != nil || root != manifest.RootName || clean != header.Name || header.Size == 0 || header.Size > MaxUploadBytes {
			return PreparedDirectorySnapshot{}, &UploadError{ReasonCode: "directory_path_invalid"}
		}
		body, readErr := io.ReadAll(io.LimitReader(tarReader, header.Size+1))
		if readErr != nil || int64(len(body)) != header.Size {
			return PreparedDirectorySnapshot{}, &UploadError{ReasonCode: "directory_entry_size_invalid"}
		}
		totalBytes += int64(len(body))
		if totalBytes > MaxDirectoryBytes {
			return PreparedDirectorySnapshot{}, &UploadError{ReasonCode: "directory_too_large"}
		}
		files = append(files, DirectoryInputFile{RelativePath: clean, Content: body})
	}
	if !manifestRead || len(files) == 0 || len(files) != len(manifest.Files) {
		return PreparedDirectorySnapshot{}, &UploadError{ReasonCode: "directory_manifest_mismatch"}
	}
	for index := range files {
		entry := manifest.Files[index]
		if entry.RelativePath != files[index].RelativePath || entry.ByteSize != int64(len(files[index].Content)) {
			return PreparedDirectorySnapshot{}, &UploadError{ReasonCode: "directory_manifest_mismatch"}
		}
		files[index].MediaType = entry.MediaType
	}
	verified, err := PrepareDirectorySnapshot(files)
	if err != nil {
		return PreparedDirectorySnapshot{}, err
	}
	if verified.RootName != manifest.RootName || !bytes.Equal(verified.Manifest, manifestBytesFromContent(content)) || !bytes.Equal(verified.Archive, content) {
		return PreparedDirectorySnapshot{}, fmt.Errorf("%w: directory snapshot is not canonical", &UploadError{ReasonCode: "directory_archive_noncanonical"})
	}
	return verified, nil
}

func manifestBytesFromContent(content []byte) []byte {
	gzipReader, err := gzip.NewReader(bytes.NewReader(content))
	if err != nil {
		return nil
	}
	defer gzipReader.Close()
	tarReader := tar.NewReader(gzipReader)
	header, err := tarReader.Next()
	if err != nil || header.Name != DirectoryManifestName {
		return nil
	}
	manifest, err := io.ReadAll(io.LimitReader(tarReader, MaxDirectoryManifest+1))
	if err != nil || len(manifest) > MaxDirectoryManifest {
		return nil
	}
	return manifest
}

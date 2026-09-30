// pattern: Functional Core
package intake

import (
	"archive/zip"
	"bytes"
	"errors"
	"io"
	"path"
	"sort"
	"strings"
	"unicode"
)

const (
	MaxZIPEntries   = 512
	MaxZIPFiles     = MaxDirectoryFiles
	MaxZIPPathBytes = 1024
)

func isInputArchiveSource(sourceKind string) bool {
	return sourceKind == "directory_snapshot" || sourceKind == "zip_snapshot" || sourceKind == "pdf_snapshot" || sourceKind == "git_snapshot"
}

func IsInputArchiveSource(sourceKind string) bool {
	return isInputArchiveSource(sourceKind)
}

func ExtractVerifiedInputArchive(sourceKind string, content []byte) ([]DirectoryInputFile, error) {
	switch sourceKind {
	case "directory_snapshot":
		return ExtractVerifiedDirectoryFiles(content)
	case "zip_snapshot":
		return ExtractVerifiedZIPFiles(content)
	case "pdf_snapshot":
		return ExtractVerifiedDirectoryFiles(content)
	case "git_snapshot":
		return ExtractVerifiedGitSnapshotFiles(content)
	default:
		return nil, &UploadError{ReasonCode: "source_kind_invalid"}
	}
}

// ExtractVerifiedZIPFiles returns a bounded, path-checked view of a ZIP source.
// It never writes entries to the host filesystem.
func ExtractVerifiedZIPFiles(content []byte) ([]DirectoryInputFile, error) {
	if len(content) == 0 || len(content) > MaxUploadBytes {
		return nil, &UploadError{ReasonCode: "zip_archive_size_invalid"}
	}
	reader, err := zip.NewReader(bytes.NewReader(content), int64(len(content)))
	if err != nil || len(reader.File) == 0 {
		return nil, &UploadError{ReasonCode: "zip_archive_invalid"}
	}
	if len(reader.File) > MaxZIPEntries {
		return nil, &UploadError{ReasonCode: "zip_entry_count_exceeded"}
	}
	seen := make(map[string]bool, len(reader.File))
	filesByPath := make(map[string]struct{}, len(reader.File))
	files := make([]DirectoryInputFile, 0, len(reader.File))
	var expandedBytes uint64
	for _, entry := range reader.File {
		isDirectory := strings.HasSuffix(entry.Name, "/")
		entryPath := entry.Name
		if isDirectory {
			entryPath = strings.TrimSuffix(entryPath, "/")
		}
		cleanPath, err := safeZIPRelativePath(entryPath)
		if err != nil || cleanPath != entryPath {
			return nil, &UploadError{ReasonCode: "zip_path_invalid"}
		}
		key := strings.ToLower(cleanPath)
		if _, exists := seen[key]; exists {
			return nil, &UploadError{ReasonCode: "zip_path_duplicate"}
		}
		seen[key] = isDirectory
		mode := entry.Mode()
		if (isDirectory && mode.Type() != 0 && !mode.IsDir()) || (!isDirectory && !mode.IsRegular()) {
			return nil, &UploadError{ReasonCode: "zip_entry_type_unsupported"}
		}
		if isDirectory {
			if _, isFile := filesByPath[key]; isFile {
				return nil, &UploadError{ReasonCode: "zip_path_duplicate"}
			}
			continue
		}
		for parent := path.Dir(cleanPath); parent != "."; parent = path.Dir(parent) {
			if _, isFile := filesByPath[strings.ToLower(parent)]; isFile {
				return nil, &UploadError{ReasonCode: "zip_path_duplicate"}
			}
		}
		prefix := key + "/"
		for existing := range filesByPath {
			if strings.HasPrefix(existing, prefix) {
				return nil, &UploadError{ReasonCode: "zip_path_duplicate"}
			}
		}
		if nestedArchivePath(cleanPath) {
			return nil, &UploadError{ReasonCode: "zip_nested_archive_unsupported"}
		}
		if len(files) >= MaxZIPFiles {
			return nil, &UploadError{ReasonCode: "zip_file_count_exceeded"}
		}
		if entry.UncompressedSize64 == 0 {
			return nil, &UploadError{ReasonCode: "zip_empty_file_unsupported"}
		}
		if entry.UncompressedSize64 > MaxDirectoryBytes || expandedBytes+entry.UncompressedSize64 > MaxDirectoryBytes {
			return nil, &UploadError{ReasonCode: "zip_expanded_size_exceeded"}
		}
		expandedBytes += entry.UncompressedSize64
		opened, openErr := entry.Open()
		if openErr != nil {
			return nil, &UploadError{ReasonCode: "zip_archive_invalid"}
		}
		body, readErr := io.ReadAll(io.LimitReader(opened, int64(entry.UncompressedSize64)+1))
		closeErr := opened.Close()
		if readErr != nil || closeErr != nil || uint64(len(body)) != entry.UncompressedSize64 {
			return nil, &UploadError{ReasonCode: "zip_archive_invalid"}
		}
		verified, verifyErr := prepareUploadFile(path.Base(cleanPath), "application/octet-stream", body)
		mediaType := "application/octet-stream"
		if verifyErr == nil {
			mediaType = verified.MediaType
		}
		filesByPath[key] = struct{}{}
		files = append(files, DirectoryInputFile{RelativePath: cleanPath, MediaType: mediaType, Content: body})
	}
	if len(files) == 0 {
		return nil, &UploadError{ReasonCode: "zip_no_files"}
	}
	sort.Slice(files, func(i, j int) bool { return files[i].RelativePath < files[j].RelativePath })
	return files, nil
}

func safeZIPRelativePath(value string) (string, error) {
	if value == "" || len(value) > MaxZIPPathBytes || strings.ContainsAny(value, `\\:`) || strings.HasPrefix(value, "/") || strings.ContainsRune(value, '\x00') {
		return "", errors.New("unsafe ZIP path")
	}
	clean := path.Clean(value)
	if clean != value || clean == "." || strings.HasPrefix(clean, "../") || path.IsAbs(clean) {
		return "", errors.New("unsafe ZIP path")
	}
	for _, part := range strings.Split(clean, "/") {
		if part == "" || part == "." || part == ".." || strings.HasSuffix(part, ".") || strings.HasSuffix(part, " ") || strings.IndexFunc(part, unicode.IsControl) >= 0 || strings.ContainsAny(part, `<>:"|?*`) {
			return "", errors.New("unsafe ZIP path")
		}
		deviceBase := strings.ToUpper(strings.TrimSuffix(part, path.Ext(part)))
		if deviceBase == "CON" || deviceBase == "PRN" || deviceBase == "AUX" || deviceBase == "NUL" || isWindowsNumberedDevice(deviceBase) {
			return "", errors.New("reserved ZIP path")
		}
	}
	return clean, nil
}

func nestedArchivePath(relativePath string) bool {
	switch strings.ToLower(path.Ext(relativePath)) {
	case ".zip", ".7z", ".rar", ".tar", ".gz", ".tgz", ".bz2", ".xz", ".zst":
		return true
	default:
		return false
	}
}

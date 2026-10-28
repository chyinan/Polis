// pattern: Imperative Shell
package intake

import (
	"io"
	"io/fs"
	"mime"
	"os"
	"path"
	"strings"
)

const readOnlyRootSnapshotName = "workspace"

// PrepareReadOnlyRootSnapshot converts an already-authorized OS root handle
// into the existing bounded immutable directory snapshot format. It never
// accepts or returns a host path, and it never writes to the root. Callers must
// obtain the *os.Root from a separately owner-authorized adapter. This source
// walker rejects observed symlinks and non-regular entries; native TOCTOU,
// mount-boundary and filesystem-identity qualification is still required
// before wiring it to a live Company root.
func PrepareReadOnlyRootSnapshot(root *os.Root) (PreparedDirectorySnapshot, error) {
	if root == nil {
		return PreparedDirectorySnapshot{}, &UploadError{ReasonCode: "host_root_unavailable"}
	}
	rootInfo, err := root.Stat(".")
	if err != nil || !rootInfo.IsDir() {
		return PreparedDirectorySnapshot{}, &UploadError{ReasonCode: "host_root_invalid"}
	}
	directories := map[string]fs.FileInfo{".": rootInfo}
	files := make([]DirectoryInputFile, 0, 32)
	totalBytes := int64(0)
	err = fs.WalkDir(root.FS(), ".", func(relativePath string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return &UploadError{ReasonCode: "host_root_scan_failed"}
		}
		if relativePath == "." {
			return nil
		}
		if entry.Type()&fs.ModeSymlink != 0 {
			return &UploadError{ReasonCode: "host_root_link_unsupported"}
		}
		info, infoErr := entry.Info()
		if infoErr != nil || info.Mode()&fs.ModeSymlink != 0 {
			return &UploadError{ReasonCode: "host_root_scan_failed"}
		}
		if info.IsDir() {
			directories[relativePath] = info
			return nil
		}
		if !info.Mode().IsRegular() {
			return &UploadError{ReasonCode: "host_root_entry_unsupported"}
		}
		if !safeRelativeRootFilePath(relativePath) || info.Size() <= 0 || info.Size() > MaxUploadBytes {
			return &UploadError{ReasonCode: "host_root_file_invalid"}
		}
		if len(files) >= MaxDirectoryFiles || totalBytes+info.Size() > MaxDirectoryBytes {
			return &UploadError{ReasonCode: "host_root_snapshot_too_large"}
		}
		content, readErr := readStableRootFile(root, relativePath, info, directories)
		if readErr != nil {
			return readErr
		}
		mediaType, supportedText := textMediaType(strings.ToLower(path.Ext(relativePath)))
		if !supportedText {
			mediaType = mime.TypeByExtension(path.Ext(relativePath))
			if mediaType == "" {
				mediaType = "application/octet-stream"
			}
		}
		files = append(files, DirectoryInputFile{
			RelativePath: path.Join(readOnlyRootSnapshotName, relativePath),
			MediaType:    mediaType,
			Content:      content,
		})
		totalBytes += int64(len(content))
		return nil
	})
	if err != nil {
		return PreparedDirectorySnapshot{}, err
	}
	if len(files) == 0 {
		return PreparedDirectorySnapshot{}, &UploadError{ReasonCode: "host_root_empty"}
	}
	return PrepareDirectorySnapshot(files)
}

func safeRelativeRootFilePath(value string) bool {
	if value == "" || strings.Contains(value, "\\") || strings.HasPrefix(value, "/") {
		return false
	}
	clean := path.Clean(value)
	if clean != value || clean == "." || strings.HasPrefix(clean, "../") || path.IsAbs(clean) {
		return false
	}
	return true
}

func readStableRootFile(root *os.Root, relativePath string, expected fs.FileInfo, directories map[string]fs.FileInfo) ([]byte, error) {
	parentPath := path.Dir(relativePath)
	parentRoot, closeParent, err := openRootDirectoryChain(root, parentPath, directories)
	if err != nil {
		return nil, err
	}
	if closeParent {
		defer parentRoot.Close()
	}
	baseName := path.Base(relativePath)
	before, err := parentRoot.Lstat(baseName)
	if err != nil || !before.Mode().IsRegular() || !os.SameFile(expected, before) {
		return nil, &UploadError{ReasonCode: "host_root_file_changed"}
	}
	file, err := parentRoot.Open(baseName)
	if err != nil {
		return nil, &UploadError{ReasonCode: "host_root_file_read_failed"}
	}
	defer file.Close()
	opened, err := file.Stat()
	if err != nil || !opened.Mode().IsRegular() || !os.SameFile(before, opened) || opened.Size() != expected.Size() {
		return nil, &UploadError{ReasonCode: "host_root_file_changed"}
	}
	content, err := io.ReadAll(io.LimitReader(file, MaxUploadBytes+1))
	if err != nil || len(content) == 0 || int64(len(content)) != expected.Size() || int64(len(content)) > MaxUploadBytes {
		return nil, &UploadError{ReasonCode: "host_root_file_changed"}
	}
	openedAfterRead, err := file.Stat()
	if err != nil || !os.SameFile(opened, openedAfterRead) || openedAfterRead.Size() != expected.Size() {
		return nil, &UploadError{ReasonCode: "host_root_file_changed"}
	}
	after, err := parentRoot.Lstat(baseName)
	if err != nil || !after.Mode().IsRegular() || !os.SameFile(openedAfterRead, after) {
		return nil, &UploadError{ReasonCode: "host_root_file_changed"}
	}
	return content, nil
}

func openRootDirectoryChain(root *os.Root, relativePath string, directories map[string]fs.FileInfo) (*os.Root, bool, error) {
	if relativePath == "." {
		return root, false, nil
	}
	current := root
	owned := false
	prefix := ""
	for _, component := range strings.Split(relativePath, "/") {
		if component == "" || component == "." || component == ".." {
			if owned {
				_ = current.Close()
			}
			return nil, false, &UploadError{ReasonCode: "host_root_file_invalid"}
		}
		if prefix == "" {
			prefix = component
		} else {
			prefix = path.Join(prefix, component)
		}
		expected, exists := directories[prefix]
		before, statErr := current.Lstat(component)
		if !exists || statErr != nil || !before.IsDir() || before.Mode()&fs.ModeSymlink != 0 || !os.SameFile(expected, before) {
			if owned {
				_ = current.Close()
			}
			return nil, false, &UploadError{ReasonCode: "host_root_directory_changed"}
		}
		next, openErr := current.OpenRoot(component)
		if openErr != nil {
			if owned {
				_ = current.Close()
			}
			return nil, false, &UploadError{ReasonCode: "host_root_directory_changed"}
		}
		opened, openedErr := next.Stat(".")
		after, afterErr := current.Lstat(component)
		if openedErr != nil || !opened.IsDir() || !os.SameFile(expected, opened) || afterErr != nil || after.Mode()&fs.ModeSymlink != 0 || !os.SameFile(expected, after) {
			_ = next.Close()
			if owned {
				_ = current.Close()
			}
			return nil, false, &UploadError{ReasonCode: "host_root_directory_changed"}
		}
		if owned {
			_ = current.Close()
		}
		current, owned = next, true
	}
	return current, owned, nil
}

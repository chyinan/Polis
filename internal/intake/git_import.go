// pattern: Imperative Shell
package intake

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"unicode/utf8"
)

const (
	maxGitTreeOutputBytes  = 4 << 20
	maxGitErrorOutputBytes = 16 << 10
)

var errGitCommandOutputLimit = errors.New("git command output exceeded its bound")

// ImportGitCommit reads a full commit already present in a local working
// repository. It never fetches, checks out, writes to the source repository,
// follows filters, or reads credential helpers.
func ImportGitCommit(ctx context.Context, repositoryPath, commitSelector string) (PreparedGitSnapshot, error) {
	if !validGitObjectID(strings.ToLower(commitSelector)) {
		return PreparedGitSnapshot{}, &UploadError{ReasonCode: "git_commit_selector_invalid"}
	}
	if ctx == nil {
		return PreparedGitSnapshot{}, &UploadError{ReasonCode: "git_import_context_invalid"}
	}
	requestedRoot, err := filepath.Abs(repositoryPath)
	if err != nil || strings.TrimSpace(repositoryPath) == "" {
		return PreparedGitSnapshot{}, &UploadError{ReasonCode: "git_repository_path_invalid"}
	}
	requestedRoot, err = filepath.EvalSymlinks(filepath.Clean(requestedRoot))
	if err != nil {
		return PreparedGitSnapshot{}, &UploadError{ReasonCode: "git_repository_path_invalid"}
	}
	info, err := os.Stat(requestedRoot)
	if err != nil || !info.IsDir() {
		return PreparedGitSnapshot{}, &UploadError{ReasonCode: "git_repository_path_invalid"}
	}
	inside, err := runLocalGit(ctx, requestedRoot, 16, "rev-parse", "--is-inside-work-tree")
	if err != nil || strings.TrimSpace(string(inside)) != "true" {
		return PreparedGitSnapshot{}, &UploadError{ReasonCode: "git_repository_invalid"}
	}
	topLevel, err := runLocalGit(ctx, requestedRoot, 4<<10, "rev-parse", "--show-toplevel")
	if err != nil {
		return PreparedGitSnapshot{}, err
	}
	repositoryRoot, err := filepath.EvalSymlinks(filepath.Clean(strings.TrimSpace(string(topLevel))))
	if err != nil || !sameLocalPath(repositoryRoot, requestedRoot) {
		return PreparedGitSnapshot{}, &UploadError{ReasonCode: "git_repository_root_required"}
	}
	rootName, err := safeDisplayName(filepath.Base(repositoryRoot))
	if err != nil || !validGitSnapshotRoot(rootName) {
		return PreparedGitSnapshot{}, &UploadError{ReasonCode: "git_repository_name_invalid"}
	}

	commitID := strings.ToLower(commitSelector)
	objectType, err := runLocalGit(ctx, repositoryRoot, 64, "cat-file", "-t", commitID)
	if err != nil || strings.TrimSpace(string(objectType)) != "commit" {
		return PreparedGitSnapshot{}, &UploadError{ReasonCode: "git_commit_not_available_locally"}
	}
	resolvedCommit, err := runLocalGit(ctx, repositoryRoot, 128, "rev-parse", "--verify", "--end-of-options", commitID+"^{commit}")
	if err != nil || strings.TrimSpace(strings.ToLower(string(resolvedCommit))) != commitID {
		return PreparedGitSnapshot{}, &UploadError{ReasonCode: "git_commit_selector_invalid"}
	}
	treeOutput, err := runLocalGit(ctx, repositoryRoot, 128, "rev-parse", "--verify", "--end-of-options", commitID+"^{tree}")
	if err != nil {
		return PreparedGitSnapshot{}, &UploadError{ReasonCode: "git_commit_tree_unavailable"}
	}
	treeID := strings.TrimSpace(strings.ToLower(string(treeOutput)))
	if !validGitObjectID(treeID) {
		return PreparedGitSnapshot{}, &UploadError{ReasonCode: "git_commit_tree_invalid"}
	}

	treeOutputBytes, err := runLocalGit(ctx, repositoryRoot, maxGitTreeOutputBytes, "ls-tree", "-r", "-z", "--long", "--full-tree", commitID)
	if err != nil {
		return PreparedGitSnapshot{}, err
	}
	entries, err := parseGitTreeEntries(treeOutputBytes)
	if err != nil {
		return PreparedGitSnapshot{}, err
	}
	if len(entries) > MaxDirectoryFiles {
		return PreparedGitSnapshot{}, &UploadError{ReasonCode: "git_snapshot_file_count_exceeded"}
	}
	pathSet := make(map[string]struct{}, len(entries))
	files := make([]DirectoryInputFile, 0, len(entries))
	exclusions := make([]GitSnapshotExclusion, 0)
	totalBytes := int64(0)
	for _, entry := range entries {
		relativePath, err := safeGitRepositoryPath(rootName, entry.path)
		if err != nil {
			return PreparedGitSnapshot{}, err
		}
		if err = checkGitSnapshotPath(pathSet, relativePath); err != nil {
			return PreparedGitSnapshot{}, err
		}
		switch {
		case entry.mode == "160000" && entry.objectType == "commit":
			exclusions = append(exclusions, GitSnapshotExclusion{RelativePath: relativePath, Reason: "submodule_content_not_fetched"})
			continue
		case entry.mode == "120000":
			exclusions = append(exclusions, GitSnapshotExclusion{RelativePath: relativePath, Reason: "symlink_not_followed"})
			continue
		case (entry.mode != "100644" && entry.mode != "100755") || entry.objectType != "blob":
			exclusions = append(exclusions, GitSnapshotExclusion{RelativePath: relativePath, Reason: "file_representation_invalid"})
			continue
		}
		if entry.size <= 0 {
			exclusions = append(exclusions, GitSnapshotExclusion{RelativePath: relativePath, Reason: "empty_file_unsupported"})
			continue
		}
		if entry.size > MaxUploadBytes {
			exclusions = append(exclusions, GitSnapshotExclusion{RelativePath: relativePath, Reason: "source_file_size_limit"})
			continue
		}
		if totalBytes+entry.size > MaxDirectoryBytes-(16<<10) || len(files) >= MaxDirectoryFiles-1 {
			reason := "source_total_size_limit"
			if len(files) >= MaxDirectoryFiles-1 {
				reason = "source_file_count_limit"
			}
			exclusions = append(exclusions, GitSnapshotExclusion{RelativePath: relativePath, Reason: reason})
			continue
		}
		content, readErr := runLocalGit(ctx, repositoryRoot, int(entry.size)+1, "cat-file", "blob", entry.objectID)
		if readErr != nil || int64(len(content)) != entry.size {
			return PreparedGitSnapshot{}, &UploadError{ReasonCode: "git_snapshot_blob_read_failed"}
		}
		if isGitLFSPointer(content) {
			exclusions = append(exclusions, GitSnapshotExclusion{RelativePath: relativePath, Reason: "git_lfs_payload_not_fetched"})
			continue
		}
		mediaType := "application/octet-stream"
		if _, supported := textMediaType(strings.ToLower(path.Ext(entry.path))); supported {
			mediaType = "text/plain"
		} else if isGitImageExtension(entry.path) {
			mediaType = "application/octet-stream"
		}
		prepared, prepareErr := prepareUploadFile(path.Base(entry.path), mediaType, content)
		if prepareErr != nil {
			exclusions = append(exclusions, GitSnapshotExclusion{RelativePath: relativePath, Reason: "file_representation_invalid"})
			continue
		}
		files = append(files, DirectoryInputFile{RelativePath: relativePath, MediaType: prepared.MediaType, Content: content})
		totalBytes += entry.size
	}
	repositoryIdentity := sha256Hex([]byte(filepath.Clean(repositoryRoot)))
	return PrepareGitSnapshot(rootName, repositoryIdentity, commitID, treeID, "not_included_by_policy", files, exclusions)
}

type gitTreeEntry struct {
	mode       string
	objectType string
	objectID   string
	size       int64
	path       string
}

func parseGitTreeEntries(content []byte) ([]gitTreeEntry, error) {
	if len(content) == 0 {
		return nil, nil
	}
	if content[len(content)-1] != 0 {
		return nil, &UploadError{ReasonCode: "git_tree_listing_invalid"}
	}
	records := bytes.Split(content[:len(content)-1], []byte{0})
	if len(records) > MaxDirectoryFiles {
		return nil, &UploadError{ReasonCode: "git_snapshot_file_count_exceeded"}
	}
	entries := make([]gitTreeEntry, 0, len(records))
	for _, record := range records {
		separator := bytes.IndexByte(record, '\t')
		if separator < 0 || !utf8.Valid(record[separator+1:]) {
			return nil, &UploadError{ReasonCode: "git_tree_listing_invalid"}
		}
		fields := strings.Fields(string(record[:separator]))
		if len(fields) != 4 || !validGitObjectID(strings.ToLower(fields[2])) {
			return nil, &UploadError{ReasonCode: "git_tree_listing_invalid"}
		}
		size := int64(-1)
		if fields[3] != "-" {
			parsed, err := strconv.ParseInt(fields[3], 10, 64)
			if err != nil || parsed < 0 {
				return nil, &UploadError{ReasonCode: "git_tree_listing_invalid"}
			}
			size = parsed
		}
		entries = append(entries, gitTreeEntry{mode: fields[0], objectType: fields[1], objectID: strings.ToLower(fields[2]), size: size, path: string(record[separator+1:])})
	}
	return entries, nil
}

func safeGitRepositoryPath(rootName, repositoryRelativePath string) (string, error) {
	if repositoryRelativePath == "" || !utf8.ValidString(repositoryRelativePath) || strings.Contains(repositoryRelativePath, "\\") {
		return "", &UploadError{ReasonCode: "git_repository_path_unsupported"}
	}
	value := path.Join(rootName, repositoryRelativePath)
	clean, root, err := safeRelativeDirectoryPath(value)
	if err != nil || clean != value || root != rootName {
		return "", &UploadError{ReasonCode: "git_repository_path_unsupported"}
	}
	return clean, nil
}

func isGitLFSPointer(content []byte) bool {
	return bytes.HasPrefix(content, []byte("version https://git-lfs.github.com/spec/v1\n")) || bytes.HasPrefix(content, []byte("version https://git-lfs.github.com/spec/v1\r\n"))
}

func isGitImageExtension(filename string) bool {
	switch strings.ToLower(path.Ext(filename)) {
	case ".png", ".jpg", ".jpeg":
		return true
	default:
		return false
	}
}

func runLocalGit(ctx context.Context, repositoryRoot string, outputLimit int, arguments ...string) ([]byte, error) {
	baseArgs := []string{
		"-c", "core.hooksPath=" + os.DevNull,
		"-c", "credential.helper=",
		"-c", "core.fsmonitor=false",
		"-c", "filter.lfs.smudge=",
		"-c", "filter.lfs.process=",
		"-c", "filter.lfs.required=false",
		"-c", "protocol.file.allow=never",
		"-C", repositoryRoot,
	}
	command := exec.CommandContext(ctx, "git", append(baseArgs, arguments...)...)
	command.Env = localGitEnvironment()
	stdout := boundedGitBuffer{limit: outputLimit}
	stderr := boundedGitBuffer{limit: maxGitErrorOutputBytes}
	command.Stdout = &stdout
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		if errors.Is(err, errGitCommandOutputLimit) || errors.Is(stdout.err, errGitCommandOutputLimit) || errors.Is(stderr.err, errGitCommandOutputLimit) {
			return nil, &UploadError{ReasonCode: "git_command_output_too_large"}
		}
		if ctx.Err() != nil {
			return nil, &UploadError{ReasonCode: "git_import_cancelled"}
		}
		return nil, &UploadError{ReasonCode: "git_snapshot_git_command_failed"}
	}
	return stdout.data, nil
}

type boundedGitBuffer struct {
	data  []byte
	limit int
	err   error
}

func (buffer *boundedGitBuffer) Write(content []byte) (int, error) {
	if len(content) > buffer.limit-len(buffer.data) {
		buffer.err = errGitCommandOutputLimit
		return 0, buffer.err
	}
	buffer.data = append(buffer.data, content...)
	return len(content), nil
}

func localGitEnvironment() []string {
	environment := []string{
		"GIT_CONFIG_NOSYSTEM=1",
		"GIT_CONFIG_GLOBAL=" + os.DevNull,
		"GIT_TERMINAL_PROMPT=0",
		"GIT_OPTIONAL_LOCKS=0",
		"GIT_NO_REPLACE_OBJECTS=1",
		"GIT_NO_LAZY_FETCH=1",
		"GIT_ATTR_NOSYSTEM=1",
	}
	for _, name := range []string{"PATH", "SystemRoot", "WINDIR", "TEMP", "TMP"} {
		if value := os.Getenv(name); value != "" {
			environment = append(environment, name+"="+value)
		}
	}
	return environment
}

func sameLocalPath(left, right string) bool {
	left = filepath.Clean(left)
	right = filepath.Clean(right)
	if filepath.Separator == '\\' {
		return strings.EqualFold(left, right)
	}
	return left == right
}

func sha256Hex(content []byte) string {
	sum := sha256.Sum256(content)
	return fmt.Sprintf("%x", sum[:])
}

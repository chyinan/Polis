// pattern: Functional Core
package intake

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"path"
	"sort"
	"strings"
	"time"
)

const (
	GitSnapshotSchema         = "polis-git-snapshot@1"
	GitSnapshotManifestName   = ".polis-git-input-manifest.json"
	GitSnapshotSourceNotePath = ".polis-git-source.json"
	maxGitSnapshotSourceNote  = MaxModelInputFileBytes
)

type GitSnapshotExclusion struct {
	RelativePath string `json:"relativePath"`
	Reason       string `json:"reason"`
}

type GitSnapshotOmissionSummary struct {
	Reason string `json:"reason"`
	Count  int    `json:"count"`
}

type GitSnapshotSourceNote struct {
	SchemaVersion            string                       `json:"schemaVersion"`
	RepositoryIdentity       string                       `json:"repositoryIdentity"`
	CommitID                 string                       `json:"commitId"`
	TreeID                   string                       `json:"treeId"`
	SelectedRef              string                       `json:"selectedRef"`
	WorkingTreeStatus        string                       `json:"workingTreeStatus"`
	WorkingTreePolicy        string                       `json:"workingTreePolicy"`
	NetworkFetch             string                       `json:"networkFetch"`
	SubmodulesNotFetched     int                          `json:"submodulesNotFetched"`
	LFSPayloadsNotFetched    int                          `json:"lfsPayloadsNotFetched"`
	Omissions                []GitSnapshotOmissionSummary `json:"omissions"`
	OmissionExamples         []GitSnapshotExclusion       `json:"omissionExamples"`
	OmissionDetailsTruncated bool                         `json:"omissionDetailsTruncated"`
}

type GitSnapshotManifest struct {
	SchemaVersion      string                   `json:"schemaVersion"`
	RootName           string                   `json:"rootName"`
	RepositoryIdentity string                   `json:"repositoryIdentity"`
	CommitID           string                   `json:"commitId"`
	TreeID             string                   `json:"treeId"`
	WorkingTreeStatus  string                   `json:"workingTreeStatus"`
	Files              []DirectoryManifestEntry `json:"files"`
	Exclusions         []GitSnapshotExclusion   `json:"exclusions"`
}

type PreparedGitSnapshot struct {
	RootName string
	Manifest []byte
	Archive  []byte
	Upload   PreparedUpload
}

// PrepareGitSnapshot builds a canonical archive for the exact selected commit.
// The repository path is never included; RepositoryIdentity is already a
// one-way digest computed by the local importer.
func PrepareGitSnapshot(rootName, repositoryIdentity, commitID, treeID, workingTreeStatus string, files []DirectoryInputFile, exclusions []GitSnapshotExclusion) (PreparedGitSnapshot, error) {
	if !validGitSnapshotRoot(rootName) || !validSHA256Digest(repositoryIdentity) || !validGitObjectID(commitID) || !validGitObjectID(treeID) || !validGitWorkingTreeStatus(workingTreeStatus) || len(files) > MaxDirectoryFiles-1 || len(exclusions) > MaxDirectoryFiles {
		return PreparedGitSnapshot{}, &UploadError{ReasonCode: "git_snapshot_metadata_invalid"}
	}
	orderedFiles := append([]DirectoryInputFile(nil), files...)
	sort.Slice(orderedFiles, func(i, j int) bool { return orderedFiles[i].RelativePath < orderedFiles[j].RelativePath })
	orderedExclusions := append([]GitSnapshotExclusion(nil), exclusions...)
	sort.Slice(orderedExclusions, func(i, j int) bool {
		if orderedExclusions[i].RelativePath != orderedExclusions[j].RelativePath {
			return orderedExclusions[i].RelativePath < orderedExclusions[j].RelativePath
		}
		return orderedExclusions[i].Reason < orderedExclusions[j].Reason
	})

	seen := make(map[string]struct{}, len(orderedFiles)+len(orderedExclusions)+1)
	entries := make([]DirectoryManifestEntry, 0, len(orderedFiles)+1)
	archiveFiles := make([]DirectoryInputFile, 0, len(orderedFiles)+1)
	totalBytes := int64(0)
	state := StateUsable
	for _, file := range orderedFiles {
		clean, root, err := safeRelativeDirectoryPath(file.RelativePath)
		if clean == rootName+"/"+GitSnapshotSourceNotePath {
			return PreparedGitSnapshot{}, &UploadError{ReasonCode: "git_snapshot_reserved_path"}
		}
		if err != nil || clean != file.RelativePath || root != rootName {
			return PreparedGitSnapshot{}, &UploadError{ReasonCode: "git_snapshot_path_invalid"}
		}
		if err = checkGitSnapshotPath(seen, clean); err != nil {
			return PreparedGitSnapshot{}, err
		}
		if len(file.Content) == 0 {
			return PreparedGitSnapshot{}, &UploadError{ReasonCode: "empty_file"}
		}
		prepared, prepareErr := prepareUploadFile(path.Base(clean), file.MediaType, file.Content)
		if prepareErr != nil {
			return PreparedGitSnapshot{}, &UploadError{ReasonCode: "git_snapshot_file_invalid"}
		}
		totalBytes += int64(len(file.Content))
		if totalBytes > MaxDirectoryBytes {
			return PreparedGitSnapshot{}, &UploadError{ReasonCode: "git_snapshot_too_large"}
		}
		if prepared.State != StateUsable {
			state = StatePartial
		}
		entries = append(entries, DirectoryManifestEntry{
			RelativePath: clean, MediaType: prepared.MediaType, ByteSize: prepared.ByteSize,
			ContentDigest: prepared.ContentDigest, State: prepared.State,
		})
		archiveFiles = append(archiveFiles, DirectoryInputFile{RelativePath: clean, MediaType: prepared.MediaType, Content: append([]byte(nil), file.Content...)})
	}
	for _, exclusion := range orderedExclusions {
		clean, root, err := safeRelativeDirectoryPath(exclusion.RelativePath)
		if err != nil || clean != exclusion.RelativePath || root != rootName || !validGitExclusionReason(exclusion.Reason) {
			return PreparedGitSnapshot{}, &UploadError{ReasonCode: "git_snapshot_exclusion_invalid"}
		}
		if err = checkGitSnapshotPath(seen, clean); err != nil {
			return PreparedGitSnapshot{}, err
		}
	}
	if len(orderedFiles) == 0 && len(orderedExclusions) == 0 {
		return PreparedGitSnapshot{}, &UploadError{ReasonCode: "git_snapshot_empty"}
	}
	if workingTreeStatus != "clean" || len(orderedExclusions) > 0 {
		state = StatePartial
	}

	note := buildGitSnapshotSourceNote(repositoryIdentity, commitID, treeID, workingTreeStatus, orderedExclusions)
	noteBytes, err := json.Marshal(note)
	if err != nil {
		return PreparedGitSnapshot{}, err
	}
	if len(noteBytes) == 0 || len(noteBytes) > maxGitSnapshotSourceNote {
		return PreparedGitSnapshot{}, &UploadError{ReasonCode: "git_snapshot_source_note_too_large"}
	}
	totalBytes += int64(len(noteBytes))
	if totalBytes > MaxDirectoryBytes {
		return PreparedGitSnapshot{}, &UploadError{ReasonCode: "git_snapshot_too_large"}
	}
	notePath := rootName + "/" + GitSnapshotSourceNotePath
	if err = checkGitSnapshotPath(seen, notePath); err != nil {
		return PreparedGitSnapshot{}, &UploadError{ReasonCode: "git_snapshot_reserved_path"}
	}
	noteUpload, err := prepareUploadFile(GitSnapshotSourceNotePath, "application/json", noteBytes)
	if err != nil {
		return PreparedGitSnapshot{}, &UploadError{ReasonCode: "git_snapshot_source_note_invalid"}
	}
	entries = append(entries, DirectoryManifestEntry{RelativePath: notePath, MediaType: noteUpload.MediaType, ByteSize: noteUpload.ByteSize, ContentDigest: noteUpload.ContentDigest, State: noteUpload.State})
	archiveFiles = append(archiveFiles, DirectoryInputFile{RelativePath: notePath, MediaType: noteUpload.MediaType, Content: noteBytes})
	sort.Slice(entries, func(i, j int) bool { return entries[i].RelativePath < entries[j].RelativePath })
	sort.Slice(archiveFiles, func(i, j int) bool { return archiveFiles[i].RelativePath < archiveFiles[j].RelativePath })

	manifest := GitSnapshotManifest{
		SchemaVersion: GitSnapshotSchema, RootName: rootName, RepositoryIdentity: repositoryIdentity,
		CommitID: commitID, TreeID: treeID, WorkingTreeStatus: workingTreeStatus,
		Files: entries, Exclusions: orderedExclusions,
	}
	manifestBytes, err := json.Marshal(manifest)
	if err != nil {
		return PreparedGitSnapshot{}, err
	}
	if len(manifestBytes) == 0 || len(manifestBytes) > MaxDirectoryManifest {
		return PreparedGitSnapshot{}, &UploadError{ReasonCode: "git_snapshot_manifest_too_large"}
	}
	archive, err := buildGitSnapshotArchive(manifestBytes, archiveFiles)
	if err != nil {
		return PreparedGitSnapshot{}, err
	}
	if len(archive) == 0 || len(archive) > MaxUploadBytes {
		return PreparedGitSnapshot{}, &UploadError{ReasonCode: "git_snapshot_archive_too_large"}
	}
	digest := sha256.Sum256(archive)
	return PreparedGitSnapshot{
		RootName: rootName, Manifest: manifestBytes, Archive: archive,
		Upload: PreparedUpload{
			SourceKind: "git_snapshot", DisplayName: rootName + "@" + commitID[:12], MediaType: "application/gzip",
			ByteSize: int64(len(archive)), ContentDigest: hex.EncodeToString(digest[:]), State: state,
		},
	}, nil
}

func VerifyGitSnapshot(content []byte) (PreparedGitSnapshot, error) {
	manifest, files, err := readGitSnapshotArchive(content)
	if err != nil {
		return PreparedGitSnapshot{}, err
	}
	sourceFiles := make([]DirectoryInputFile, 0, len(files)-1)
	for _, file := range files {
		if file.RelativePath == manifest.RootName+"/"+GitSnapshotSourceNotePath {
			continue
		}
		sourceFiles = append(sourceFiles, file)
	}
	verified, err := PrepareGitSnapshot(manifest.RootName, manifest.RepositoryIdentity, manifest.CommitID, manifest.TreeID, manifest.WorkingTreeStatus, sourceFiles, manifest.Exclusions)
	if err != nil {
		return PreparedGitSnapshot{}, err
	}
	if !bytes.Equal(verified.Archive, content) {
		return PreparedGitSnapshot{}, &UploadError{ReasonCode: "git_snapshot_noncanonical"}
	}
	return verified, nil
}

func ExtractVerifiedGitSnapshotFiles(content []byte) ([]DirectoryInputFile, error) {
	if _, err := VerifyGitSnapshot(content); err != nil {
		return nil, err
	}
	_, files, err := readGitSnapshotArchive(content)
	if err != nil {
		return nil, err
	}
	return files, nil
}

func readGitSnapshotArchive(content []byte) (GitSnapshotManifest, []DirectoryInputFile, error) {
	if len(content) == 0 || len(content) > MaxUploadBytes {
		return GitSnapshotManifest{}, nil, &UploadError{ReasonCode: "git_snapshot_archive_size_invalid"}
	}
	gzipReader, err := gzip.NewReader(bytes.NewReader(content))
	if err != nil {
		return GitSnapshotManifest{}, nil, &UploadError{ReasonCode: "git_snapshot_archive_invalid"}
	}
	defer gzipReader.Close()
	tarReader := tar.NewReader(io.LimitReader(gzipReader, MaxDirectoryArchiveExpandedBytes))
	manifestHeader, err := tarReader.Next()
	if err != nil || manifestHeader.Name != GitSnapshotManifestName || manifestHeader.Typeflag != tar.TypeReg || manifestHeader.Size <= 0 || manifestHeader.Size > MaxDirectoryManifest {
		return GitSnapshotManifest{}, nil, &UploadError{ReasonCode: "git_snapshot_manifest_invalid"}
	}
	manifestBytes, err := io.ReadAll(io.LimitReader(tarReader, manifestHeader.Size+1))
	if err != nil || int64(len(manifestBytes)) != manifestHeader.Size {
		return GitSnapshotManifest{}, nil, &UploadError{ReasonCode: "git_snapshot_manifest_invalid"}
	}
	var manifest GitSnapshotManifest
	decoder := json.NewDecoder(bytes.NewReader(manifestBytes))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&manifest) != nil || manifest.SchemaVersion != GitSnapshotSchema || !validGitSnapshotRoot(manifest.RootName) || !validSHA256Digest(manifest.RepositoryIdentity) || !validGitObjectID(manifest.CommitID) || !validGitObjectID(manifest.TreeID) || !validGitWorkingTreeStatus(manifest.WorkingTreeStatus) || len(manifest.Files) == 0 || len(manifest.Files) > MaxDirectoryFiles {
		return GitSnapshotManifest{}, nil, &UploadError{ReasonCode: "git_snapshot_manifest_invalid"}
	}
	var trailing any
	if decoder.Decode(&trailing) != io.EOF {
		return GitSnapshotManifest{}, nil, &UploadError{ReasonCode: "git_snapshot_manifest_invalid"}
	}
	files := make([]DirectoryInputFile, 0, len(manifest.Files))
	seen := make(map[string]struct{}, len(manifest.Files))
	totalBytes := int64(0)
	for _, entry := range manifest.Files {
		clean, root, pathErr := safeRelativeDirectoryPath(entry.RelativePath)
		if pathErr != nil || clean != entry.RelativePath || root != manifest.RootName || !validSHA256Digest(entry.ContentDigest) || entry.ByteSize <= 0 || entry.ByteSize > MaxUploadBytes || entry.State != StateUsable && entry.State != StateUnsupported && entry.State != StatePartial {
			return GitSnapshotManifest{}, nil, &UploadError{ReasonCode: "git_snapshot_file_manifest_invalid"}
		}
		if pathErr = checkGitSnapshotPath(seen, clean); pathErr != nil {
			return GitSnapshotManifest{}, nil, pathErr
		}
		header, nextErr := tarReader.Next()
		if nextErr != nil || header.Name != clean || header.Typeflag != tar.TypeReg || header.Size != entry.ByteSize {
			return GitSnapshotManifest{}, nil, &UploadError{ReasonCode: "git_snapshot_file_order_invalid"}
		}
		body, readErr := io.ReadAll(io.LimitReader(tarReader, entry.ByteSize+1))
		if readErr != nil || int64(len(body)) != entry.ByteSize || sha256Digest(body) != entry.ContentDigest {
			return GitSnapshotManifest{}, nil, &UploadError{ReasonCode: "git_snapshot_file_digest_mismatch"}
		}
		verifiedUpload, verifyErr := prepareUploadFile(path.Base(clean), entry.MediaType, body)
		if verifyErr != nil || verifiedUpload.MediaType != entry.MediaType || verifiedUpload.State != entry.State {
			return GitSnapshotManifest{}, nil, &UploadError{ReasonCode: "git_snapshot_file_representation_invalid"}
		}
		totalBytes += int64(len(body))
		if totalBytes > MaxDirectoryBytes {
			return GitSnapshotManifest{}, nil, &UploadError{ReasonCode: "git_snapshot_too_large"}
		}
		files = append(files, DirectoryInputFile{RelativePath: clean, MediaType: entry.MediaType, Content: body})
	}
	if _, nextErr := tarReader.Next(); nextErr != io.EOF {
		return GitSnapshotManifest{}, nil, &UploadError{ReasonCode: "git_snapshot_trailing_entry"}
	}
	if len(files) == 0 || files[len(files)-1].RelativePath == "" {
		return GitSnapshotManifest{}, nil, &UploadError{ReasonCode: "git_snapshot_empty"}
	}
	if err = validateGitSnapshotManifestFiles(manifest, files); err != nil {
		return GitSnapshotManifest{}, nil, err
	}
	return manifest, files, nil
}

func validateGitSnapshotManifestFiles(manifest GitSnapshotManifest, files []DirectoryInputFile) error {
	if len(files) != len(manifest.Files) {
		return &UploadError{ReasonCode: "git_snapshot_manifest_mismatch"}
	}
	for index, file := range files {
		entry := manifest.Files[index]
		if entry.RelativePath != file.RelativePath || entry.ByteSize != int64(len(file.Content)) || entry.ContentDigest != sha256Digest(file.Content) {
			return &UploadError{ReasonCode: "git_snapshot_manifest_mismatch"}
		}
	}
	return nil
}

func buildGitSnapshotArchive(manifest []byte, files []DirectoryInputFile) ([]byte, error) {
	var output bytes.Buffer
	gzipWriter := gzip.NewWriter(&output)
	gzipWriter.Header.ModTime = time.Unix(0, 0).UTC()
	gzipWriter.Header.OS = 255
	tarWriter := tar.NewWriter(gzipWriter)
	writeEntry := func(name string, content []byte) error {
		header := &tar.Header{Name: name, Mode: 0o644, Size: int64(len(content)), ModTime: time.Unix(0, 0).UTC(), Typeflag: tar.TypeReg, Format: tar.FormatPAX}
		if err := tarWriter.WriteHeader(header); err != nil {
			return err
		}
		_, err := tarWriter.Write(content)
		return err
	}
	if err := writeEntry(GitSnapshotManifestName, manifest); err != nil {
		_ = tarWriter.Close()
		_ = gzipWriter.Close()
		return nil, err
	}
	for _, file := range files {
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
	return output.Bytes(), nil
}

func buildGitSnapshotSourceNote(repositoryIdentity, commitID, treeID, workingTreeStatus string, exclusions []GitSnapshotExclusion) GitSnapshotSourceNote {
	counts := make(map[string]int)
	for _, exclusion := range exclusions {
		counts[exclusion.Reason]++
	}
	reasons := make([]string, 0, len(counts))
	for reason := range counts {
		reasons = append(reasons, reason)
	}
	sort.Strings(reasons)
	omissions := make([]GitSnapshotOmissionSummary, 0, len(reasons))
	submodules, lfs := 0, 0
	for _, reason := range reasons {
		omissions = append(omissions, GitSnapshotOmissionSummary{Reason: reason, Count: counts[reason]})
		if reason == "submodule_content_not_fetched" {
			submodules += counts[reason]
		}
		if reason == "git_lfs_payload_not_fetched" {
			lfs += counts[reason]
		}
	}
	examples := append([]GitSnapshotExclusion(nil), exclusions...)
	if len(examples) > 12 {
		examples = examples[:12]
	}
	note := GitSnapshotSourceNote{
		SchemaVersion: GitSnapshotSchema, RepositoryIdentity: repositoryIdentity, CommitID: commitID,
		TreeID: treeID, SelectedRef: commitID, WorkingTreeStatus: workingTreeStatus,
		WorkingTreePolicy:    "working-tree files are not read or included; add a separate directory snapshot for local changes",
		NetworkFetch:         "disabled",
		SubmodulesNotFetched: submodules, LFSPayloadsNotFetched: lfs, Omissions: omissions, OmissionExamples: examples,
		OmissionDetailsTruncated: len(exclusions) > len(examples),
	}
	for len(note.OmissionExamples) > 0 {
		encoded, err := json.Marshal(note)
		if err == nil && len(encoded) <= maxGitSnapshotSourceNote {
			return note
		}
		note.OmissionExamples = note.OmissionExamples[:len(note.OmissionExamples)-1]
		note.OmissionDetailsTruncated = true
	}
	return note
}

func checkGitSnapshotPath(seen map[string]struct{}, relativePath string) error {
	folded := strings.ToLower(relativePath)
	if _, exists := seen[folded]; exists {
		return &UploadError{ReasonCode: "git_snapshot_path_collision"}
	}
	for prior := range seen {
		if strings.HasPrefix(folded, prior+"/") || strings.HasPrefix(prior, folded+"/") {
			return &UploadError{ReasonCode: "git_snapshot_path_collision"}
		}
	}
	seen[folded] = struct{}{}
	return nil
}

func validGitSnapshotRoot(value string) bool {
	if value == "" || len(value) > 120 || strings.Contains(value, "/") || strings.Contains(value, "\\") {
		return false
	}
	_, root, err := safeRelativeDirectoryPath(value + "/snapshot.txt")
	return err == nil && root == value
}

func validGitObjectID(value string) bool {
	if len(value) != 40 && len(value) != 64 {
		return false
	}
	for _, character := range value {
		if !((character >= '0' && character <= '9') || (character >= 'a' && character <= 'f')) {
			return false
		}
	}
	return true
}

func validGitWorkingTreeStatus(value string) bool {
	return value == "not_included_by_policy"
}

func validGitExclusionReason(value string) bool {
	switch value {
	case "submodule_content_not_fetched", "git_lfs_payload_not_fetched", "symlink_not_followed", "source_file_size_limit", "source_total_size_limit", "source_file_count_limit", "empty_file_unsupported", "file_representation_invalid":
		return true
	default:
		return false
	}
}

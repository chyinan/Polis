// pattern: Functional Core
package kernel

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"polis/internal/core"
	"polis/internal/intake"
	"polis/internal/workspacepath"
)

type workspaceTreeGitFile struct {
	RelativePath string
	Digest       string
	Content      []byte
}

type workspaceTreeGitImport struct {
	Manifest            intake.GitSnapshotManifest
	InputManifestSHA256 string
	Files               []workspaceTreeGitFile
}

func (k *Kernel) loadInitialGitWorkspaceImportTX(ctx context.Context, tx pgx.Tx, companyID, missionID, taskID string) (*workspaceTreeGitImport, bool, error) {
	var inputManifestSHA256 string
	var encodedManifest []byte
	err := tx.QueryRow(ctx, `SELECT manifest_digest,manifest FROM task_input_manifests WHERE company_id=$1 AND mission_id=$2 AND task_id=$3`, companyID, missionID, taskID).Scan(&inputManifestSHA256, &encodedManifest)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	var inputManifest intake.ModelInputManifest
	if json.Unmarshal(encodedManifest, &inputManifest) != nil || intake.VerifyModelInputManifest(inputManifest, inputManifestSHA256) != nil ||
		inputManifest.CompanyID != companyID || inputManifest.MissionID != missionID || inputManifest.TaskID != taskID {
		return nil, false, core.Integrity
	}
	var selected *intake.ModelInputManifestEntry
	for index := range inputManifest.CandidateInputs {
		entry := &inputManifest.CandidateInputs[index]
		if entry.SourceKind != "git_snapshot" {
			continue
		}
		if selected != nil {
			return nil, false, core.ConflictError{Reason: "Task has multiple Git base commits; an explicit base selection is required", CurrentState: "ambiguous_git_base"}
		}
		selected = entry
	}
	if selected == nil {
		return nil, false, nil
	}
	var sourceKind, contentDigest, inputState string
	var byteSize int64
	if err = tx.QueryRow(ctx, `SELECT source_kind,content_digest,state,byte_size FROM mission_inputs
WHERE company_id=$1 AND mission_id=$2 AND input_id=$3 AND revision=$4`, companyID, missionID, selected.InputID, selected.Revision).Scan(&sourceKind, &contentDigest, &inputState, &byteSize); err != nil {
		return nil, false, core.Integrity
	}
	if sourceKind != "git_snapshot" || contentDigest != selected.ContentDigest || inputState != string(selected.State) || byteSize != selected.ByteSize || byteSize < 1 || byteSize > intake.MaxUploadBytes {
		return nil, false, core.Integrity
	}
	archive, err := readBlobBounded(k.root, companyID, contentDigest, intake.MaxUploadBytes)
	if err != nil || int64(len(archive)) != byteSize || casDigest(archive) != contentDigest {
		return nil, false, core.Integrity
	}
	manifest, files, err := workspaceTreeFilesFromGitSnapshot(archive)
	if err != nil {
		return nil, false, core.Denied
	}
	return &workspaceTreeGitImport{Manifest: manifest, InputManifestSHA256: inputManifestSHA256, Files: files}, true, nil
}

func (k *Kernel) verifyWorkspaceTreeGitSourceTX(ctx context.Context, tx pgx.Tx, companyID string, manifest workspaceTreeManifest) error {
	if manifest.BaseCommitOID == "" && manifest.InputManifestSHA256 == "" {
		return nil
	}
	if !validWorkspaceGitOID(manifest.BaseCommitOID) || !validSHA256(manifest.InputManifestSHA256) {
		return core.Integrity
	}
	gitImport, exists, err := k.loadInitialGitWorkspaceImportTX(ctx, tx, companyID, manifest.MissionID, manifest.TaskID)
	if err != nil || !exists || gitImport.InputManifestSHA256 != manifest.InputManifestSHA256 || gitImport.Manifest.CommitID != manifest.BaseCommitOID {
		return core.Integrity
	}
	if manifest.Revision == 1 && !workspaceTreeEntriesMatchGitFiles(manifest.Entries, gitImport.Files) {
		return core.Integrity
	}
	return nil
}

func workspaceTreeEntriesMatchGitFiles(entries []WorkspaceTreeEntry, files []workspaceTreeGitFile) bool {
	if len(entries) != len(files) {
		return false
	}
	for index, entry := range entries {
		file := files[index]
		if entry.RelativePath != file.RelativePath || entry.Digest != file.Digest || entry.Bytes != int64(len(file.Content)) || entry.FileRevision != 1 || entry.ContentType != "text/utf-8" {
			return false
		}
	}
	return true
}

func workspaceTreeFilesFromGitSnapshot(archive []byte) (intake.GitSnapshotManifest, []workspaceTreeGitFile, error) {
	prepared, err := intake.VerifyGitSnapshot(archive)
	if err != nil || prepared.Upload.SourceKind != "git_snapshot" || prepared.Upload.ContentDigest != casDigest(archive) {
		return intake.GitSnapshotManifest{}, nil, fmt.Errorf("verified Git snapshot identity is invalid")
	}
	decoder := json.NewDecoder(bytes.NewReader(prepared.Manifest))
	decoder.DisallowUnknownFields()
	var manifest intake.GitSnapshotManifest
	if decoder.Decode(&manifest) != nil || manifest.CommitID == "" || manifest.TreeID == "" || len(manifest.Exclusions) != 0 {
		return intake.GitSnapshotManifest{}, nil, fmt.Errorf("Git snapshot is incomplete or its manifest is invalid")
	}
	var trailing any
	if err = decoder.Decode(&trailing); err != io.EOF {
		return intake.GitSnapshotManifest{}, nil, fmt.Errorf("Git snapshot manifest has trailing data")
	}
	extracted, err := intake.ExtractVerifiedGitSnapshotFiles(archive)
	if err != nil {
		return intake.GitSnapshotManifest{}, nil, fmt.Errorf("Git snapshot file verification failed")
	}
	byPath := make(map[string]intake.DirectoryInputFile, len(extracted))
	for _, file := range extracted {
		if _, exists := byPath[file.RelativePath]; exists {
			return intake.GitSnapshotManifest{}, nil, fmt.Errorf("Git snapshot contains a duplicate path")
		}
		byPath[file.RelativePath] = file
	}
	files := make([]workspaceTreeGitFile, 0, len(manifest.Files))
	var total int64
	for _, entry := range manifest.Files {
		if entry.RelativePath == manifest.RootName+"/"+intake.GitSnapshotSourceNotePath {
			continue
		}
		prefix := manifest.RootName + "/"
		if !strings.HasPrefix(entry.RelativePath, prefix) || entry.State != intake.StateUsable {
			return intake.GitSnapshotManifest{}, nil, fmt.Errorf("Git snapshot contains an unsupported source entry")
		}
		relativePath := strings.TrimPrefix(entry.RelativePath, prefix)
		if !workspacepath.ValidRelativePath(relativePath) || hasGitMetadataPathSegment(relativePath) {
			return intake.GitSnapshotManifest{}, nil, fmt.Errorf("Git snapshot path is not allowed in WorkspaceTree")
		}
		file, exists := byPath[entry.RelativePath]
		if !exists || int64(len(file.Content)) != entry.ByteSize || casDigest(file.Content) != entry.ContentDigest || !utf8.Valid(file.Content) || bytes.IndexByte(file.Content, 0) >= 0 || len(file.Content) > workspaceTreeMaxFileBytes {
			return intake.GitSnapshotManifest{}, nil, fmt.Errorf("Git snapshot file is not bounded UTF-8 text")
		}
		total += int64(len(file.Content))
		if total > workspaceTreeMaxTotalBytes || len(files) >= workspaceTreeMaxFiles {
			return intake.GitSnapshotManifest{}, nil, fmt.Errorf("Git snapshot exceeds WorkspaceTree bounds")
		}
		files = append(files, workspaceTreeGitFile{RelativePath: relativePath, Digest: entry.ContentDigest, Content: append([]byte(nil), file.Content...)})
	}
	if len(files) == 0 {
		return intake.GitSnapshotManifest{}, nil, fmt.Errorf("Git snapshot contains no supported source files")
	}
	for i := 1; i < len(files); i++ {
		for j := i; j > 0 && files[j].RelativePath < files[j-1].RelativePath; j-- {
			files[j], files[j-1] = files[j-1], files[j]
		}
	}
	return manifest, files, nil
}

func hasGitMetadataPathSegment(relativePath string) bool {
	for _, segment := range strings.Split(relativePath, "/") {
		if strings.EqualFold(segment, ".git") {
			return true
		}
	}
	return false
}

func validWorkspaceGitOID(value string) bool {
	if len(value) != 40 && len(value) != 64 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil && strings.ToLower(value) == value
}

// pattern: Functional Core
package workspacepatch

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"polis/internal/core"
	"polis/internal/gotestprofile"
	"polis/internal/workspacepath"
)

const (
	PatchBundleVersion   = "polis-workspace-patch@1"
	maxSnapshotFileCount = 512
	maxSnapshotFileBytes = 2 << 20
	maxSnapshotTotal     = 16 << 20
	maxPatchBundleBytes  = 24 << 20
)

var ErrInvalidPatch = errors.New("invalid workspace patch")

type SnapshotFile struct {
	RelativePath   string
	Digest         string
	Bytes          int64
	FileRevision   int64
	SourceRevision int64
	ContentType    string
	EntryType      string
	Content        []byte
}

type Snapshot struct {
	CompanyID           string
	MissionID           string
	TaskID              string
	SourceCommitOID     string
	InputManifestSHA256 string
	ArtifactID          string
	ManifestSHA256      string
	RootBindingID       string
	Revision            int64
	Files               []SnapshotFile
}

type GoTestEvidence struct {
	JobRunID                    string
	Status                      string
	CompanyID                   string
	MissionID                   string
	TaskID                      string
	BaseCommitOID               string
	InputManifestSHA256         string
	BaseSnapshotArtifactID      string
	BaseSnapshotSHA256          string
	BaseWorkspaceRevision       int64
	BaseFilesSHA256             string
	CandidateSnapshotArtifactID string
	CandidateSnapshotSHA256     string
	CandidateWorkspaceRevision  int64
	CandidateFilesSHA256        string
	JobBindingSHA256            string
	TaskValidationBindingSHA256 string
	ProfileRevision             string
	ProfileSHA256               string
	RunnerSHA256                string
	ToolchainSHA256             string
	CommandSHA256               string
	TestManifestSHA256          string
	ResultSHA256                string
}

type ChangePolicy struct {
	Manifest      gotestprofile.Manifest
	ProfileSHA256 string
}

type BuildInput struct {
	Base      Snapshot
	Candidate Snapshot
	Evidence  GoTestEvidence
	Policy    ChangePolicy
}

type Bundle struct {
	SchemaVersion               string          `json:"schemaVersion"`
	CompanyID                   string          `json:"companyId"`
	MissionID                   string          `json:"missionId"`
	TaskID                      string          `json:"taskId"`
	BaseCommitOID               string          `json:"baseCommitOid"`
	InputManifestSHA256         string          `json:"inputManifestSha256"`
	BaseSnapshotArtifactID      string          `json:"baseSnapshotArtifactId"`
	BaseSnapshotSHA256          string          `json:"baseSnapshotSha256"`
	BaseWorkspaceRevision       int64           `json:"baseWorkspaceRevision"`
	BaseFilesSHA256             string          `json:"baseFilesSha256"`
	CandidateSnapshotArtifactID string          `json:"candidateSnapshotArtifactId"`
	CandidateSnapshotSHA256     string          `json:"candidateSnapshotSha256"`
	CandidateWorkspaceRevision  int64           `json:"candidateWorkspaceRevision"`
	CandidateFilesSHA256        string          `json:"candidateFilesSha256"`
	JobBindingSHA256            string          `json:"jobBindingSha256"`
	TaskValidationBindingSHA256 string          `json:"taskValidationBindingSha256"`
	JobRunID                    string          `json:"jobRunId"`
	ProfileRevision             string          `json:"profileRevision"`
	ProfileSHA256               string          `json:"profileSha256"`
	RunnerSHA256                string          `json:"runnerSha256"`
	ToolchainSHA256             string          `json:"toolchainSha256"`
	CommandSHA256               string          `json:"commandSha256"`
	TestManifestSHA256          string          `json:"testManifestSha256"`
	TestResultSHA256            string          `json:"testResultSha256"`
	TestStatus                  string          `json:"testStatus"`
	CandidateEntries            []FileReference `json:"candidateEntries"`
	Files                       []FileDelta     `json:"files"`
	Diff                        string          `json:"diff"`
	DiffSHA256                  string          `json:"diffSha256"`
}

type FileReference struct {
	RelativePath   string `json:"relativePath"`
	SHA256         string `json:"sha256"`
	Bytes          int64  `json:"bytes"`
	FileRevision   int64  `json:"fileRevision"`
	SourceRevision int64  `json:"sourceRevision"`
	ContentType    string `json:"contentType"`
}

type FileDelta struct {
	RelativePath     string `json:"relativePath"`
	Action           string `json:"action"`
	BeforeSHA256     string `json:"beforeSha256,omitempty"`
	BeforeContentB64 string `json:"beforeContentBase64,omitempty"`
	AfterSHA256      string `json:"afterSha256,omitempty"`
	AfterContentB64  string `json:"afterContentBase64,omitempty"`
}

func Build(input BuildInput) ([]byte, error) {
	baseFiles, err := validateSnapshot(input.Base)
	if err != nil {
		return nil, err
	}
	candidateFiles, err := validateSnapshot(input.Candidate)
	if err != nil {
		return nil, err
	}
	if !validSnapshotPair(input.Base, input.Candidate) || !validTestEvidence(input.Evidence, input.Policy) || !evidenceMatchesSnapshots(input.Evidence, input.Base, input.Candidate) || !validChangePolicy(input.Policy) {
		return nil, ErrInvalidPatch
	}
	deltas := diffFiles(baseFiles, candidateFiles, input.Policy)
	if deltas == nil || len(deltas) == 0 {
		return nil, ErrInvalidPatch
	}
	baseReferences := fileReferences(baseFiles)
	candidateReferences := fileReferences(candidateFiles)
	diffText := formatPatchDiff(deltas)
	bundle := Bundle{
		SchemaVersion: PatchBundleVersion, CompanyID: input.Base.CompanyID, MissionID: input.Base.MissionID, TaskID: input.Base.TaskID,
		BaseCommitOID: input.Base.SourceCommitOID, InputManifestSHA256: input.Base.InputManifestSHA256,
		BaseSnapshotArtifactID: input.Base.ArtifactID, BaseSnapshotSHA256: input.Base.ManifestSHA256,
		BaseWorkspaceRevision: input.Base.Revision, BaseFilesSHA256: fileReferencesDigest(baseReferences),
		CandidateSnapshotArtifactID: input.Candidate.ArtifactID, CandidateSnapshotSHA256: input.Candidate.ManifestSHA256,
		CandidateWorkspaceRevision: input.Candidate.Revision, CandidateFilesSHA256: fileReferencesDigest(candidateReferences),
		JobBindingSHA256:            input.Evidence.JobBindingSHA256,
		TaskValidationBindingSHA256: input.Evidence.TaskValidationBindingSHA256, JobRunID: input.Evidence.JobRunID,
		ProfileRevision: input.Evidence.ProfileRevision, ProfileSHA256: input.Evidence.ProfileSHA256,
		RunnerSHA256: input.Evidence.RunnerSHA256, ToolchainSHA256: input.Evidence.ToolchainSHA256,
		CommandSHA256: input.Evidence.CommandSHA256, TestManifestSHA256: input.Evidence.TestManifestSHA256,
		TestResultSHA256: input.Evidence.ResultSHA256, TestStatus: input.Evidence.Status,
		CandidateEntries: candidateReferences, Files: deltas, Diff: diffText, DiffSHA256: digestBytes([]byte(diffText)),
	}
	raw, err := json.Marshal(bundle)
	if err != nil || len(raw) > maxPatchBundleBytes {
		return nil, ErrInvalidPatch
	}
	return raw, nil
}

func Parse(raw []byte) (Bundle, error) {
	if len(raw) == 0 || len(raw) > maxPatchBundleBytes {
		return Bundle{}, ErrInvalidPatch
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var bundle Bundle
	if decoder.Decode(&bundle) != nil {
		return Bundle{}, ErrInvalidPatch
	}
	var trailing any
	if decoder.Decode(&trailing) != io.EOF || !validBundle(bundle) {
		return Bundle{}, ErrInvalidPatch
	}
	canonical, err := json.Marshal(bundle)
	if err != nil || !bytes.Equal(canonical, raw) {
		return Bundle{}, ErrInvalidPatch
	}
	return bundle, nil
}

func Apply(base, candidate Snapshot, raw []byte, policy ChangePolicy) ([]SnapshotFile, error) {
	bundle, err := Parse(raw)
	if err != nil {
		return nil, err
	}
	baseFiles, err := validateSnapshot(base)
	if err != nil {
		return nil, err
	}
	candidateFiles, err := validateSnapshot(candidate)
	if err != nil {
		return nil, err
	}
	if !validSnapshotPair(base, candidate) || !validChangePolicy(policy) ||
		!validTestEvidence(evidenceFromBundle(bundle), policy) || !evidenceMatchesSnapshots(evidenceFromBundle(bundle), base, candidate) ||
		bundle.CompanyID != base.CompanyID || bundle.MissionID != base.MissionID || bundle.TaskID != base.TaskID ||
		bundle.BaseCommitOID != base.SourceCommitOID || bundle.InputManifestSHA256 != base.InputManifestSHA256 ||
		bundle.BaseSnapshotArtifactID != base.ArtifactID || bundle.BaseSnapshotSHA256 != base.ManifestSHA256 ||
		bundle.BaseWorkspaceRevision != base.Revision || bundle.BaseFilesSHA256 != fileReferencesDigest(fileReferences(baseFiles)) ||
		bundle.CandidateSnapshotArtifactID != candidate.ArtifactID || bundle.CandidateSnapshotSHA256 != candidate.ManifestSHA256 ||
		bundle.CandidateWorkspaceRevision != candidate.Revision || bundle.CandidateFilesSHA256 != fileReferencesDigest(fileReferences(candidateFiles)) ||
		bundle.ProfileRevision != policy.Manifest.ProfileRevision {
		return nil, ErrInvalidPatch
	}
	baseByPath := fileMap(baseFiles)
	for _, delta := range bundle.Files {
		current, exists := baseByPath[delta.RelativePath]
		if !changeAllowed(delta.RelativePath, exists, policy) {
			return nil, ErrInvalidPatch
		}
		if delta.Action == "add" {
			if exists || delta.BeforeSHA256 != "" || delta.BeforeContentB64 != "" {
				return nil, ErrInvalidPatch
			}
		} else if !exists || current.Digest != delta.BeforeSHA256 || !contentMatches(delta.BeforeContentB64, current.Content, current.Digest) {
			return nil, ErrInvalidPatch
		}
		switch delta.Action {
		case "add", "change":
			content, decodeErr := base64.RawStdEncoding.DecodeString(delta.AfterContentB64)
			if decodeErr != nil || digestBytes(content) != delta.AfterSHA256 {
				return nil, ErrInvalidPatch
			}
			if delta.Action == "change" {
				current.Content = content
				current.Digest = delta.AfterSHA256
				current.Bytes = int64(len(content))
				baseByPath[delta.RelativePath] = current
			} else {
				baseByPath[delta.RelativePath] = SnapshotFile{RelativePath: delta.RelativePath, Digest: delta.AfterSHA256, Bytes: int64(len(content)), ContentType: "text/utf-8", EntryType: "regular", Content: content}
			}
		case "delete":
			if delta.AfterSHA256 != "" || delta.AfterContentB64 != "" {
				return nil, ErrInvalidPatch
			}
			delete(baseByPath, delta.RelativePath)
		default:
			return nil, ErrInvalidPatch
		}
	}
	refs := make(map[string]FileReference, len(bundle.CandidateEntries))
	for _, reference := range bundle.CandidateEntries {
		refs[reference.RelativePath] = reference
	}
	result := make([]SnapshotFile, 0, len(baseByPath))
	for _, file := range baseByPath {
		reference, ok := refs[file.RelativePath]
		if !ok || reference.SHA256 != file.Digest || reference.Bytes != file.Bytes {
			return nil, ErrInvalidPatch
		}
		file.FileRevision = reference.FileRevision
		file.SourceRevision = reference.SourceRevision
		file.ContentType = reference.ContentType
		result = append(result, file)
	}
	result, err = validateSnapshotFiles(result)
	if err != nil || !sameFiles(result, candidateFiles) {
		return nil, ErrInvalidPatch
	}
	return result, nil
}

func validSnapshotPair(base, candidate Snapshot) bool {
	return core.ValidID(base.CompanyID) && core.ValidID(base.MissionID) && core.ValidID(base.TaskID) &&
		validGitObjectID(base.SourceCommitOID) && validSHA256(base.InputManifestSHA256) && core.ValidID(base.ArtifactID) &&
		validSHA256(base.ManifestSHA256) && core.ValidID(base.RootBindingID) && base.Revision > 0 &&
		candidate.CompanyID == base.CompanyID && candidate.MissionID == base.MissionID && candidate.TaskID == base.TaskID &&
		candidate.SourceCommitOID == base.SourceCommitOID && candidate.InputManifestSHA256 == base.InputManifestSHA256 &&
		core.ValidID(candidate.ArtifactID) && candidate.ArtifactID != base.ArtifactID && validSHA256(candidate.ManifestSHA256) &&
		candidate.RootBindingID == base.RootBindingID && candidate.Revision > base.Revision
}

func validTestEvidence(evidence GoTestEvidence, policy ChangePolicy) bool {
	_, profileSHA256, err := gotestprofile.Canonicalize(policy.Manifest)
	return err == nil && core.ValidID(evidence.JobRunID) && evidence.Status == "passed" &&
		core.ValidID(evidence.CompanyID) && core.ValidID(evidence.MissionID) && core.ValidID(evidence.TaskID) &&
		validGitObjectID(evidence.BaseCommitOID) && validSHA256(evidence.InputManifestSHA256) &&
		core.ValidID(evidence.BaseSnapshotArtifactID) && validSHA256(evidence.BaseSnapshotSHA256) && evidence.BaseWorkspaceRevision > 0 && validSHA256(evidence.BaseFilesSHA256) &&
		core.ValidID(evidence.CandidateSnapshotArtifactID) && validSHA256(evidence.CandidateSnapshotSHA256) && evidence.CandidateWorkspaceRevision > evidence.BaseWorkspaceRevision && validSHA256(evidence.CandidateFilesSHA256) &&
		validSHA256(evidence.TaskValidationBindingSHA256) && evidence.ProfileRevision == policy.Manifest.ProfileRevision &&
		evidence.ProfileSHA256 == profileSHA256 && evidence.ProfileSHA256 == policy.ProfileSHA256 &&
		evidence.RunnerSHA256 == policy.Manifest.RunnerSHA256 && evidence.ToolchainSHA256 == policy.Manifest.ToolchainSHA256 &&
		evidence.CommandSHA256 == gotestprofile.CommandSHA256(policy.Manifest) && evidence.TestManifestSHA256 == policy.Manifest.TestManifestSHA256 &&
		validSHA256(evidence.ResultSHA256) && evidence.JobBindingSHA256 == jobBindingSHA256(evidence, policy.Manifest)
}

func jobBindingSHA256(evidence GoTestEvidence, profile gotestprofile.Manifest) string {
	binding := gotestprofile.JobBinding{
		SchemaVersion: gotestprofile.JobBindingSchema, JobRunID: evidence.JobRunID,
		CompanyID: evidence.CompanyID, MissionID: evidence.MissionID, TaskID: evidence.TaskID,
		TaskValidationBindingSHA256: evidence.TaskValidationBindingSHA256,
		BaseCommitOID:               evidence.BaseCommitOID, InputManifestSHA256: evidence.InputManifestSHA256,
		BaseSnapshotArtifactID: evidence.BaseSnapshotArtifactID, BaseSnapshotSHA256: evidence.BaseSnapshotSHA256,
		BaseWorkspaceRevision: evidence.BaseWorkspaceRevision, BaseFilesSHA256: evidence.BaseFilesSHA256,
		CandidateSnapshotArtifactID: evidence.CandidateSnapshotArtifactID, CandidateSnapshotSHA256: evidence.CandidateSnapshotSHA256,
		CandidateWorkspaceRevision: evidence.CandidateWorkspaceRevision, CandidateFilesSHA256: evidence.CandidateFilesSHA256,
		ProfileSHA256: evidence.ProfileSHA256, Profile: profile,
	}
	_, bindingSHA256, err := gotestprofile.CanonicalizeJobBinding(binding)
	if err != nil {
		return ""
	}
	return bindingSHA256
}

func evidenceMatchesSnapshots(evidence GoTestEvidence, base, candidate Snapshot) bool {
	baseFiles, baseErr := validateSnapshot(base)
	candidateFiles, candidateErr := validateSnapshot(candidate)
	return baseErr == nil && candidateErr == nil &&
		evidence.CompanyID == base.CompanyID && evidence.MissionID == base.MissionID && evidence.TaskID == base.TaskID &&
		evidence.BaseCommitOID == base.SourceCommitOID && evidence.InputManifestSHA256 == base.InputManifestSHA256 &&
		evidence.BaseSnapshotArtifactID == base.ArtifactID && evidence.BaseSnapshotSHA256 == base.ManifestSHA256 && evidence.BaseWorkspaceRevision == base.Revision &&
		evidence.BaseFilesSHA256 == fileReferencesDigest(fileReferences(baseFiles)) &&
		evidence.CandidateSnapshotArtifactID == candidate.ArtifactID && evidence.CandidateSnapshotSHA256 == candidate.ManifestSHA256 && evidence.CandidateWorkspaceRevision == candidate.Revision &&
		evidence.CandidateFilesSHA256 == fileReferencesDigest(fileReferences(candidateFiles))
}

func evidenceFromBundle(bundle Bundle) GoTestEvidence {
	return GoTestEvidence{
		JobRunID: bundle.JobRunID, Status: bundle.TestStatus,
		CompanyID: bundle.CompanyID, MissionID: bundle.MissionID, TaskID: bundle.TaskID,
		BaseCommitOID: bundle.BaseCommitOID, InputManifestSHA256: bundle.InputManifestSHA256,
		BaseSnapshotArtifactID: bundle.BaseSnapshotArtifactID, BaseSnapshotSHA256: bundle.BaseSnapshotSHA256,
		BaseWorkspaceRevision: bundle.BaseWorkspaceRevision, BaseFilesSHA256: bundle.BaseFilesSHA256,
		CandidateSnapshotArtifactID: bundle.CandidateSnapshotArtifactID, CandidateSnapshotSHA256: bundle.CandidateSnapshotSHA256,
		CandidateWorkspaceRevision: bundle.CandidateWorkspaceRevision, CandidateFilesSHA256: bundle.CandidateFilesSHA256,
		JobBindingSHA256:            bundle.JobBindingSHA256,
		TaskValidationBindingSHA256: bundle.TaskValidationBindingSHA256,
		ProfileRevision:             bundle.ProfileRevision, ProfileSHA256: bundle.ProfileSHA256,
		RunnerSHA256: bundle.RunnerSHA256, ToolchainSHA256: bundle.ToolchainSHA256,
		CommandSHA256: bundle.CommandSHA256, TestManifestSHA256: bundle.TestManifestSHA256, ResultSHA256: bundle.TestResultSHA256,
	}
}

func validChangePolicy(policy ChangePolicy) bool {
	_, profileSHA256, err := gotestprofile.Canonicalize(policy.Manifest)
	return err == nil && policy.ProfileSHA256 == profileSHA256 && validSHA256(policy.ProfileSHA256)
}

func validateSnapshot(snapshot Snapshot) ([]SnapshotFile, error) {
	if len(snapshot.Files) == 0 || len(snapshot.Files) > maxSnapshotFileCount {
		return nil, ErrInvalidPatch
	}
	return validateSnapshotFiles(snapshot.Files)
}

func validateSnapshotFiles(files []SnapshotFile) ([]SnapshotFile, error) {
	if len(files) == 0 || len(files) > maxSnapshotFileCount {
		return nil, ErrInvalidPatch
	}
	result := append([]SnapshotFile(nil), files...)
	sort.Slice(result, func(i, j int) bool { return result[i].RelativePath < result[j].RelativePath })
	var total int64
	for index, file := range result {
		if !workspacepath.ValidRelativePath(file.RelativePath) || file.EntryType != "regular" || file.ContentType != "text/utf-8" ||
			file.Bytes != int64(len(file.Content)) || file.Bytes < 1 || file.Bytes > maxSnapshotFileBytes ||
			!utf8.Valid(file.Content) || digestBytes(file.Content) != file.Digest || file.FileRevision < 1 || file.SourceRevision < 1 {
			return nil, ErrInvalidPatch
		}
		if index > 0 && result[index-1].RelativePath == file.RelativePath {
			return nil, ErrInvalidPatch
		}
		total += file.Bytes
		if total > maxSnapshotTotal {
			return nil, ErrInvalidPatch
		}
	}
	return result, nil
}

func diffFiles(base, candidate []SnapshotFile, policy ChangePolicy) []FileDelta {
	baseByPath := fileMap(base)
	candidateByPath := fileMap(candidate)
	paths := make(map[string]struct{}, len(baseByPath)+len(candidateByPath))
	for path := range baseByPath {
		paths[path] = struct{}{}
	}
	for path := range candidateByPath {
		paths[path] = struct{}{}
	}
	ordered := make([]string, 0, len(paths))
	for path := range paths {
		ordered = append(ordered, path)
	}
	sort.Strings(ordered)
	deltas := make([]FileDelta, 0)
	for _, path := range ordered {
		oldFile, hadOld := baseByPath[path]
		newFile, hasNew := candidateByPath[path]
		if hadOld && hasNew && oldFile.Digest == newFile.Digest {
			continue
		}
		if !changeAllowed(path, hadOld, policy) {
			return nil
		}
		delta := FileDelta{RelativePath: path}
		if hadOld {
			delta.BeforeSHA256 = oldFile.Digest
			delta.BeforeContentB64 = base64.RawStdEncoding.EncodeToString(oldFile.Content)
		}
		switch {
		case !hadOld && hasNew:
			delta.Action = "add"
			delta.AfterSHA256 = newFile.Digest
			delta.AfterContentB64 = base64.RawStdEncoding.EncodeToString(newFile.Content)
		case hadOld && !hasNew:
			delta.Action = "delete"
		case hadOld && hasNew:
			delta.Action = "change"
			delta.AfterSHA256 = newFile.Digest
			delta.AfterContentB64 = base64.RawStdEncoding.EncodeToString(newFile.Content)
		default:
			return nil
		}
		deltas = append(deltas, delta)
	}
	return deltas
}

func changeAllowed(path string, existed bool, policy ChangePolicy) bool {
	if containsPath(policy.Manifest.ProtectedPaths, path) {
		return false
	}
	if containsPath(policy.Manifest.EditablePaths, path) {
		return true
	}
	return policy.Manifest.AllowNewTestFiles && !existed && strings.HasSuffix(path, "_test.go")
}

func containsPath(paths []string, wanted string) bool {
	for _, path := range paths {
		if path == wanted {
			return true
		}
	}
	return false
}

func fileMap(files []SnapshotFile) map[string]SnapshotFile {
	result := make(map[string]SnapshotFile, len(files))
	for _, file := range files {
		result[file.RelativePath] = file
	}
	return result
}

func fileReferences(files []SnapshotFile) []FileReference {
	result := make([]FileReference, len(files))
	for index, file := range files {
		result[index] = FileReference{RelativePath: file.RelativePath, SHA256: file.Digest, Bytes: file.Bytes, FileRevision: file.FileRevision, SourceRevision: file.SourceRevision, ContentType: file.ContentType}
	}
	return result
}

func fileReferencesDigest(files []FileReference) string {
	refs := make([]gotestprofile.SnapshotFile, len(files))
	for i, file := range files {
		refs[i] = gotestprofile.SnapshotFile{RelativePath: file.RelativePath, Digest: file.SHA256, Bytes: file.Bytes}
	}
	return gotestprofile.InventoryDigest(refs)
}

func sameFiles(left, right []SnapshotFile) bool {
	left = sortedFiles(left)
	right = sortedFiles(right)
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index].RelativePath != right[index].RelativePath || left[index].Digest != right[index].Digest || left[index].Bytes != right[index].Bytes ||
			left[index].FileRevision != right[index].FileRevision || left[index].SourceRevision != right[index].SourceRevision || left[index].ContentType != right[index].ContentType ||
			!bytes.Equal(left[index].Content, right[index].Content) {
			return false
		}
	}
	return true
}

func sortedFiles(files []SnapshotFile) []SnapshotFile {
	result := append([]SnapshotFile(nil), files...)
	sort.Slice(result, func(i, j int) bool { return result[i].RelativePath < result[j].RelativePath })
	return result
}

func formatPatchDiff(deltas []FileDelta) string {
	var output strings.Builder
	for _, delta := range deltas {
		oldLabel, newLabel := "a/"+delta.RelativePath, "b/"+delta.RelativePath
		if delta.Action == "add" {
			oldLabel = "/dev/null"
		}
		if delta.Action == "delete" {
			newLabel = "/dev/null"
		}
		output.WriteString("--- ")
		output.WriteString(oldLabel)
		output.WriteByte('\n')
		output.WriteString("+++ ")
		output.WriteString(newLabel)
		output.WriteByte('\n')
		var oldLines, newLines []string
		if delta.BeforeContentB64 != "" {
			content, _ := base64.RawStdEncoding.DecodeString(delta.BeforeContentB64)
			oldLines = displayLines(content)
		}
		if delta.AfterContentB64 != "" {
			content, _ := base64.RawStdEncoding.DecodeString(delta.AfterContentB64)
			newLines = displayLines(content)
		}
		output.WriteString("@@ -1,")
		output.WriteString(strconv.Itoa(len(oldLines)))
		output.WriteString(" +1,")
		output.WriteString(strconv.Itoa(len(newLines)))
		output.WriteString(" @@\n")
		for _, line := range oldLines {
			output.WriteByte('-')
			output.WriteString(line)
			output.WriteByte('\n')
		}
		for _, line := range newLines {
			output.WriteByte('+')
			output.WriteString(line)
			output.WriteByte('\n')
		}
	}
	return output.String()
}

func displayLines(content []byte) []string {
	if len(content) == 0 {
		return nil
	}
	return strings.Split(strings.TrimSuffix(string(content), "\n"), "\n")
}

func contentMatches(encoded string, expected []byte, digest string) bool {
	content, err := base64.RawStdEncoding.DecodeString(encoded)
	return err == nil && bytes.Equal(content, expected) && digestBytes(content) == digest
}

func validBundle(bundle Bundle) bool {
	if bundle.SchemaVersion != PatchBundleVersion || !core.ValidID(bundle.CompanyID) || !core.ValidID(bundle.MissionID) || !core.ValidID(bundle.TaskID) ||
		!validGitObjectID(bundle.BaseCommitOID) || !validSHA256(bundle.InputManifestSHA256) || !core.ValidID(bundle.BaseSnapshotArtifactID) ||
		!validSHA256(bundle.BaseSnapshotSHA256) || bundle.BaseWorkspaceRevision < 1 || !validSHA256(bundle.BaseFilesSHA256) ||
		!core.ValidID(bundle.CandidateSnapshotArtifactID) || !validSHA256(bundle.CandidateSnapshotSHA256) || bundle.CandidateWorkspaceRevision <= bundle.BaseWorkspaceRevision ||
		!validSHA256(bundle.CandidateFilesSHA256) || !validSHA256(bundle.JobBindingSHA256) || !validSHA256(bundle.TaskValidationBindingSHA256) || !core.ValidID(bundle.JobRunID) || bundle.ProfileRevision == "" ||
		!validSHA256(bundle.ProfileSHA256) || !validSHA256(bundle.RunnerSHA256) || !validSHA256(bundle.ToolchainSHA256) || !validSHA256(bundle.CommandSHA256) || !validSHA256(bundle.TestManifestSHA256) || !validSHA256(bundle.TestResultSHA256) ||
		bundle.TestStatus != "passed" || len(bundle.CandidateEntries) == 0 || len(bundle.CandidateEntries) > maxSnapshotFileCount || len(bundle.Files) == 0 || len(bundle.Files) > maxSnapshotFileCount ||
		!validSHA256(bundle.DiffSHA256) || digestBytes([]byte(bundle.Diff)) != bundle.DiffSHA256 || formatPatchDiff(bundle.Files) != bundle.Diff {
		return false
	}
	if fileReferencesDigest(bundle.CandidateEntries) != bundle.CandidateFilesSHA256 {
		return false
	}
	for index, entry := range bundle.CandidateEntries {
		if !workspacepath.ValidRelativePath(entry.RelativePath) || !validSHA256(entry.SHA256) || entry.Bytes < 1 || entry.Bytes > maxSnapshotFileBytes || entry.FileRevision < 1 || entry.SourceRevision < 1 || entry.ContentType != "text/utf-8" ||
			(index > 0 && bundle.CandidateEntries[index-1].RelativePath >= entry.RelativePath) {
			return false
		}
	}
	for index, delta := range bundle.Files {
		if !workspacepath.ValidRelativePath(delta.RelativePath) || (index > 0 && bundle.Files[index-1].RelativePath >= delta.RelativePath) ||
			(delta.BeforeSHA256 != "" && !validSHA256(delta.BeforeSHA256)) || (delta.AfterSHA256 != "" && !validSHA256(delta.AfterSHA256)) {
			return false
		}
		switch delta.Action {
		case "add", "change":
			if delta.Action == "change" && delta.BeforeContentB64 == "" || delta.Action == "add" && delta.BeforeContentB64 != "" {
				return false
			}
			if delta.BeforeContentB64 != "" {
				before, err := base64.RawStdEncoding.DecodeString(delta.BeforeContentB64)
				if err != nil || int64(len(before)) > maxSnapshotFileBytes || !utf8.Valid(before) || digestBytes(before) != delta.BeforeSHA256 {
					return false
				}
			}
			content, err := base64.RawStdEncoding.DecodeString(delta.AfterContentB64)
			if err != nil || int64(len(content)) > maxSnapshotFileBytes || !utf8.Valid(content) || digestBytes(content) != delta.AfterSHA256 {
				return false
			}
		case "delete":
			before, err := base64.RawStdEncoding.DecodeString(delta.BeforeContentB64)
			if err != nil || int64(len(before)) > maxSnapshotFileBytes || !utf8.Valid(before) || digestBytes(before) != delta.BeforeSHA256 || delta.AfterSHA256 != "" || delta.AfterContentB64 != "" || delta.BeforeSHA256 == "" {
				return false
			}
		default:
			return false
		}
	}
	return true
}

func validGitObjectID(value string) bool {
	if len(value) != 40 && len(value) != 64 {
		return false
	}
	return validLowerHex(value)
}

func validSHA256(value string) bool {
	return len(value) == sha256.Size*2 && validLowerHex(value)
}

func validLowerHex(value string) bool {
	if value != strings.ToLower(value) {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func digestBytes(value []byte) string {
	digest := sha256.Sum256(value)
	return hex.EncodeToString(digest[:])
}

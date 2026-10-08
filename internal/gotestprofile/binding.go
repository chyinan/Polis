// pattern: Functional Core
package gotestprofile

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"io"
	"regexp"
	"strings"
)

const JobBindingSchema = "polis-go-test-job-binding@1"

var jobBindingIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,80}$`)

// JobBinding is the immutable pre-execution identity for one fixed-profile
// JobRun. It freezes both snapshots before materialization or process start.
type JobBinding struct {
	SchemaVersion               string   `json:"schemaVersion"`
	JobRunID                    string   `json:"jobRunId"`
	CompanyID                   string   `json:"companyId"`
	MissionID                   string   `json:"missionId"`
	TaskID                      string   `json:"taskId"`
	TaskValidationBindingSHA256 string   `json:"taskValidationBindingSha256"`
	BaseCommitOID               string   `json:"baseCommitOid"`
	InputManifestSHA256         string   `json:"inputManifestSha256"`
	BaseSnapshotArtifactID      string   `json:"baseSnapshotArtifactId"`
	BaseSnapshotSHA256          string   `json:"baseSnapshotSha256"`
	BaseWorkspaceRevision       int64    `json:"baseWorkspaceRevision"`
	BaseFilesSHA256             string   `json:"baseFilesSha256"`
	CandidateSnapshotArtifactID string   `json:"candidateSnapshotArtifactId"`
	CandidateSnapshotSHA256     string   `json:"candidateSnapshotSha256"`
	CandidateWorkspaceRevision  int64    `json:"candidateWorkspaceRevision"`
	CandidateFilesSHA256        string   `json:"candidateFilesSha256"`
	ProfileSHA256               string   `json:"profileSha256"`
	Profile                     Manifest `json:"profile"`
}

func CanonicalizeJobBinding(binding JobBinding) ([]byte, string, error) {
	_, profileSHA256, err := Canonicalize(binding.Profile)
	if err != nil || binding.SchemaVersion != JobBindingSchema || !jobBindingIDPattern.MatchString(binding.JobRunID) ||
		!jobBindingIDPattern.MatchString(binding.CompanyID) || !jobBindingIDPattern.MatchString(binding.MissionID) || !jobBindingIDPattern.MatchString(binding.TaskID) ||
		!validSHA256(binding.TaskValidationBindingSHA256) || !validGitObjectID(binding.BaseCommitOID) || !validSHA256(binding.InputManifestSHA256) ||
		!jobBindingIDPattern.MatchString(binding.BaseSnapshotArtifactID) || !validSHA256(binding.BaseSnapshotSHA256) || binding.BaseWorkspaceRevision <= 0 || !validSHA256(binding.BaseFilesSHA256) ||
		!jobBindingIDPattern.MatchString(binding.CandidateSnapshotArtifactID) || binding.CandidateSnapshotArtifactID == binding.BaseSnapshotArtifactID || !validSHA256(binding.CandidateSnapshotSHA256) ||
		binding.CandidateWorkspaceRevision <= binding.BaseWorkspaceRevision || !validSHA256(binding.CandidateFilesSHA256) ||
		binding.ProfileSHA256 != profileSHA256 {
		return nil, "", ErrInvalidJobBinding
	}
	canonical, err := json.Marshal(binding)
	if err != nil {
		return nil, "", ErrInvalidJobBinding
	}
	return canonical, digest(canonical), nil
}

func ParseJobBinding(raw []byte, expectedSHA256 string) (JobBinding, string, error) {
	if !validSHA256(expectedSHA256) {
		return JobBinding{}, "", ErrInvalidJobBinding
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var binding JobBinding
	if decoder.Decode(&binding) != nil {
		return JobBinding{}, "", ErrInvalidJobBinding
	}
	var trailing any
	if decoder.Decode(&trailing) != io.EOF {
		return JobBinding{}, "", ErrInvalidJobBinding
	}
	canonical, digestValue, err := CanonicalizeJobBinding(binding)
	if err != nil || !bytes.Equal(canonical, raw) || digestValue != expectedSHA256 {
		return JobBinding{}, "", ErrInvalidJobBinding
	}
	return binding, digestValue, nil
}

func validGitObjectID(value string) bool {
	if (len(value) != 40 && len(value) != 64) || strings.ToLower(value) != value {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

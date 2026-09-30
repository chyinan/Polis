// pattern: Functional Core
package kernel

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
)

const (
	windowsNodeHandoverProfile = "windows-node-npm@1"
	linuxNodeHandoverProfile   = "linux-node-npm@1"
)

type CrossBackendHandoverRecord struct {
	CompanyID                 string `json:"companyId"`
	HandoverID                string `json:"handoverId"`
	MissionID                 string `json:"missionId"`
	TaskID                    string `json:"taskId"`
	SourceJobID               string `json:"sourceJobId"`
	SourceSessionID           string `json:"sourceSessionId"`
	SourceRuntimeIncarnation  string `json:"sourceRuntimeIncarnation"`
	SourceEnvironmentRevision string `json:"sourceEnvironmentRevisionId"`
	SourceProfileID           string `json:"sourceProfileId"`
	TargetEnvironmentRevision string `json:"targetEnvironmentRevisionId"`
	TargetProfileID           string `json:"targetProfileId"`
	ProjectSourceSHA256       string `json:"projectSourceSha256"`
	PackageJSONSHA256         string `json:"packageJsonSha256"`
	LockfileSHA256            string `json:"lockfileSha256"`
	WorkspaceDigest           string `json:"workspaceDigest"`
	WorkspaceRevision         int64  `json:"workspaceRevision"`
	TaskInputManifestSHA256   string `json:"taskInputManifestSha256"`
	TargetPolicySHA256        string `json:"targetPolicySha256"`
	TargetToolchainSHA256     string `json:"targetToolchainSha256"`
	RequestID                 string `json:"requestId"`
	RecordSHA256              string `json:"recordSha256"`
	CreatedAt                 string `json:"createdAt"`
}

type crossBackendHandoverDigestInput struct {
	CompanyID                 string `json:"companyId"`
	HandoverID                string `json:"handoverId"`
	MissionID                 string `json:"missionId"`
	TaskID                    string `json:"taskId"`
	SourceJobID               string `json:"sourceJobId"`
	SourceSessionID           string `json:"sourceSessionId"`
	SourceRuntimeIncarnation  string `json:"sourceRuntimeIncarnation"`
	SourceEnvironmentRevision string `json:"sourceEnvironmentRevisionId"`
	SourceProfileID           string `json:"sourceProfileId"`
	TargetEnvironmentRevision string `json:"targetEnvironmentRevisionId"`
	TargetProfileID           string `json:"targetProfileId"`
	ProjectSourceSHA256       string `json:"projectSourceSha256"`
	PackageJSONSHA256         string `json:"packageJsonSha256"`
	LockfileSHA256            string `json:"lockfileSha256"`
	WorkspaceDigest           string `json:"workspaceDigest"`
	WorkspaceRevision         int64  `json:"workspaceRevision"`
	TaskInputManifestSHA256   string `json:"taskInputManifestSha256"`
	TargetPolicySHA256        string `json:"targetPolicySha256"`
	TargetToolchainSHA256     string `json:"targetToolchainSha256"`
	RequestID                 string `json:"requestId"`
}

func supportedCrossBackendHandoverProfiles(sourceProfile, targetProfile string) bool {
	return sourceProfile == windowsNodeHandoverProfile && targetProfile == linuxNodeHandoverProfile ||
		sourceProfile == linuxNodeHandoverProfile && targetProfile == windowsNodeHandoverProfile
}

func VerifyCrossBackendHandoverRecord(record CrossBackendHandoverRecord) bool {
	return supportedCrossBackendHandoverProfiles(record.SourceProfileID, record.TargetProfileID) &&
		crossBackendHandoverRecordDigest(record) == record.RecordSHA256
}

func crossBackendHandoverRecordDigest(record CrossBackendHandoverRecord) string {
	content, _ := json.Marshal(crossBackendHandoverDigestInput{
		CompanyID: record.CompanyID, HandoverID: record.HandoverID, MissionID: record.MissionID, TaskID: record.TaskID,
		SourceJobID: record.SourceJobID, SourceSessionID: record.SourceSessionID,
		SourceRuntimeIncarnation: record.SourceRuntimeIncarnation, SourceEnvironmentRevision: record.SourceEnvironmentRevision,
		SourceProfileID: record.SourceProfileID, TargetEnvironmentRevision: record.TargetEnvironmentRevision,
		TargetProfileID: record.TargetProfileID, ProjectSourceSHA256: record.ProjectSourceSHA256,
		PackageJSONSHA256: record.PackageJSONSHA256, LockfileSHA256: record.LockfileSHA256,
		WorkspaceDigest: record.WorkspaceDigest, WorkspaceRevision: record.WorkspaceRevision,
		TaskInputManifestSHA256: record.TaskInputManifestSHA256, TargetPolicySHA256: record.TargetPolicySHA256,
		TargetToolchainSHA256: record.TargetToolchainSHA256, RequestID: record.RequestID,
	})
	digest := sha256.Sum256(content)
	return hex.EncodeToString(digest[:])
}

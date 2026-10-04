// pattern: Functional Core
package kernel

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"unicode/utf8"

	"polis/internal/core"
)

type TaskTakeoverSnapshotInput struct {
	RequestID             string
	BaseWorkspaceDigest   string
	BaseWorkspaceRevision int64
	Content               string
	HumanEffortSeconds    int64
}

type TaskTakeoverLeaseEvent struct {
	EventID            string                   `json:"eventId"`
	State              string                   `json:"state"`
	SnapshotInputID    *string                  `json:"snapshotInputId"`
	SnapshotRevision   *int64                   `json:"snapshotRevision"`
	SnapshotDigest     *string                  `json:"snapshotDigest"`
	SnapshotBytes      *int64                   `json:"snapshotBytes"`
	HumanEffortSeconds *int64                   `json:"humanEffortSeconds"`
	DiffSummary        *TaskTakeoverDiffSummary `json:"diffSummary"`
	ReasonCode         string                   `json:"reasonCode"`
	CreatedAt          string                   `json:"createdAt"`
}

type TaskTakeoverLease struct {
	LeaseID                string                            `json:"leaseId"`
	MissionID              string                            `json:"missionId"`
	TaskID                 string                            `json:"taskId"`
	ClientRequestID        string                            `json:"clientRequestId"`
	BaseRequirementsSHA256 string                            `json:"baseRequirementsSha256"`
	BaseWorkspaceDigest    string                            `json:"baseWorkspaceDigest"`
	BaseWorkspaceRevision  int64                             `json:"baseWorkspaceRevision"`
	WorkspaceTree          *TaskTakeoverWorkspaceTreeBinding `json:"workspaceTree,omitempty"`
	State                  string                            `json:"state"`
	SnapshotInputID        *string                           `json:"snapshotInputId"`
	SnapshotRevision       *int64                            `json:"snapshotRevision"`
	SnapshotDigest         *string                           `json:"snapshotDigest"`
	SnapshotBytes          *int64                            `json:"snapshotBytes"`
	HumanEffortSeconds     *int64                            `json:"humanEffortSeconds"`
	DiffSummary            *TaskTakeoverDiffSummary          `json:"diffSummary"`
	CreatedAt              string                            `json:"createdAt"`
	Events                 []TaskTakeoverLeaseEvent          `json:"events"`
}

type TaskTakeoverWorkspaceTreeBinding struct {
	RootBindingID  string `json:"rootBindingId"`
	Revision       int64  `json:"revision"`
	ManifestSHA256 string `json:"manifestSha256"`
	FileCount      int32  `json:"fileCount"`
	Bytes          int64  `json:"bytes"`
}

type TaskTakeoverDiffSummary struct {
	Model                  string `json:"model"`
	BaseWorkspaceDigest    string `json:"baseWorkspaceDigest"`
	BaseWorkspaceRevision  int64  `json:"baseWorkspaceRevision"`
	SubmittedContentDigest string `json:"submittedContentDigest"`
	BaseBytes              int    `json:"baseBytes"`
	SubmittedBytes         int    `json:"submittedBytes"`
	RemovedLines           int    `json:"removedLines"`
	AddedLines             int    `json:"addedLines"`
	Changed                bool   `json:"changed"`
}

func taskTakeoverDiffSummary(baseDigest string, baseRevision int64, baseContent, submitted []byte) (TaskTakeoverDiffSummary, error) {
	if !validTaskInputDigest(baseDigest) || baseRevision < 1 || len(baseContent) == 0 || !utf8.Valid(baseContent) || !utf8.Valid(submitted) {
		return TaskTakeoverDiffSummary{}, core.Malformed
	}
	if len(baseContent) > core.MaxContent || len(submitted) > core.MaxContent {
		return TaskTakeoverDiffSummary{}, core.TooLarge
	}
	if len(submitted) == 0 {
		return TaskTakeoverDiffSummary{}, core.Malformed
	}
	baseHash := sha256.Sum256(baseContent)
	if hex.EncodeToString(baseHash[:]) != baseDigest {
		return TaskTakeoverDiffSummary{}, core.Integrity
	}
	submittedHash := sha256.Sum256(submitted)
	submittedDigest := hex.EncodeToString(submittedHash[:])
	if submittedDigest == baseDigest {
		return TaskTakeoverDiffSummary{}, core.ConflictError{Reason: "human snapshot is unchanged from its frozen workspace", CurrentState: "no_changes"}
	}
	baseLines := diffLines(string(baseContent))
	submittedLines := diffLines(string(submitted))
	prefix := 0
	for prefix < len(baseLines) && prefix < len(submittedLines) && baseLines[prefix] == submittedLines[prefix] {
		prefix++
	}
	suffix := 0
	for suffix < len(baseLines)-prefix && suffix < len(submittedLines)-prefix &&
		baseLines[len(baseLines)-suffix-1] == submittedLines[len(submittedLines)-suffix-1] {
		suffix++
	}
	return TaskTakeoverDiffSummary{
		Model: "single_hunk_line_summary@1", BaseWorkspaceDigest: baseDigest, BaseWorkspaceRevision: baseRevision,
		SubmittedContentDigest: submittedDigest, BaseBytes: len(baseContent), SubmittedBytes: len(submitted),
		RemovedLines: len(baseLines) - prefix - suffix, AddedLines: len(submittedLines) - prefix - suffix, Changed: true,
	}, nil
}

func diffLines(content string) []string {
	return strings.Split(strings.TrimSuffix(content, "\n"), "\n")
}

func validTaskTakeoverHumanEffortSeconds(seconds int64) bool {
	return seconds >= 0 && seconds <= 86400
}

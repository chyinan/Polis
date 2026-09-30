// pattern: Functional Core
package recovery

import "fmt"

const (
	ArtifactReadyState       = "ready"
	ArtifactCandidateVerdict = "candidate"
)

type ArtifactWorkspaceContinuityInput struct {
	ArtifactID                  string
	ArtifactDigest              string
	ArtifactState               string
	ArtifactVerdict             string
	ArtifactWorkspaceTaskID     string
	ArtifactWorkspaceRevision   int64
	ArtifactWorkspaceDigest     string
	CurrentWorkspaceTaskID      string
	CurrentWorkspaceRevision    int64
	CurrentWorkspaceDigest      string
	CheckpointID                string
	CheckpointWorkspaceRevision int64
	CheckpointWorkspaceDigest   string
	CheckpointContractID        string
	ExpectedContractID          string
	CASArtifactDigest           string
	CASBlobPresent              bool
}

type ArtifactWorkspaceContinuityReport struct {
	ArtifactIdentity       bool `json:"artifact_identity"`
	ArtifactLifecycleState bool `json:"artifact_lifecycle_state"`
	ArtifactVerdict        bool `json:"artifact_verdict"`
	ArtifactDigest         bool `json:"artifact_digest"`
	ArtifactWorkspace      bool `json:"artifact_workspace_binding"`
	CurrentWorkspace       bool `json:"current_workspace_binding"`
	CheckpointWorkspace    bool `json:"checkpoint_workspace_binding"`
	CheckpointContract     bool `json:"checkpoint_contract_binding"`
	CASArtifact            bool `json:"cas_artifact_binding"`
	Passed                 bool `json:"passed"`
}

func ValidateArtifactWorkspaceContinuity(input ArtifactWorkspaceContinuityInput) (ArtifactWorkspaceContinuityReport, error) {
	report := ArtifactWorkspaceContinuityReport{
		ArtifactIdentity:       input.ArtifactID != "",
		ArtifactLifecycleState: input.ArtifactState == ArtifactReadyState,
		ArtifactVerdict:        input.ArtifactVerdict == ArtifactCandidateVerdict,
		ArtifactDigest:         input.ArtifactDigest != "",
		ArtifactWorkspace:      input.ArtifactWorkspaceTaskID != "" && input.ArtifactWorkspaceRevision > 0 && input.ArtifactWorkspaceDigest == input.ArtifactDigest,
		CurrentWorkspace:       input.CurrentWorkspaceTaskID == input.ArtifactWorkspaceTaskID && input.CurrentWorkspaceRevision == input.ArtifactWorkspaceRevision && input.CurrentWorkspaceDigest == input.ArtifactWorkspaceDigest,
		CheckpointWorkspace:    input.CheckpointID != "" && input.CheckpointWorkspaceRevision == input.ArtifactWorkspaceRevision && input.CheckpointWorkspaceDigest == input.ArtifactWorkspaceDigest,
		CheckpointContract:     input.CheckpointContractID != "" && input.CheckpointContractID == input.ExpectedContractID,
		CASArtifact:            input.CASBlobPresent && input.CASArtifactDigest == input.ArtifactDigest,
	}
	report.Passed = report.ArtifactIdentity && report.ArtifactLifecycleState && report.ArtifactVerdict && report.ArtifactDigest && report.ArtifactWorkspace && report.CurrentWorkspace && report.CheckpointWorkspace && report.CheckpointContract && report.CASArtifact
	if !report.Passed {
		return report, fmt.Errorf("artifact_workspace_continuity predicate failed")
	}
	return report, nil
}

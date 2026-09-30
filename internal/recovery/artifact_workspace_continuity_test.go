package recovery

import "testing"

func validArtifactWorkspaceContinuityInput() ArtifactWorkspaceContinuityInput {
	return ArtifactWorkspaceContinuityInput{
		ArtifactID: "artifact", ArtifactDigest: "digest", ArtifactState: "ready", ArtifactVerdict: "candidate",
		ArtifactWorkspaceTaskID: "backend-task", ArtifactWorkspaceRevision: 3, ArtifactWorkspaceDigest: "digest",
		CurrentWorkspaceTaskID: "backend-task", CurrentWorkspaceRevision: 3, CurrentWorkspaceDigest: "digest",
		CheckpointID: "checkpoint", CheckpointWorkspaceRevision: 3, CheckpointWorkspaceDigest: "digest",
		CheckpointContractID: "contract", ExpectedContractID: "contract", CASArtifactDigest: "digest", CASBlobPresent: true,
	}
}

func TestValidateArtifactWorkspaceContinuityAcceptsFormalReadyCandidateSemantics(t *testing.T) {
	report, err := ValidateArtifactWorkspaceContinuity(validArtifactWorkspaceContinuityInput())
	if err != nil || !report.Passed {
		t.Fatalf("valid continuity rejected: report=%+v err=%v", report, err)
	}
}

func TestValidateArtifactWorkspaceContinuityRejectsIndependentFailures(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*ArtifactWorkspaceContinuityInput)
	}{
		{"wrong state", func(i *ArtifactWorkspaceContinuityInput) { i.ArtifactState = "candidate" }},
		{"wrong verdict", func(i *ArtifactWorkspaceContinuityInput) { i.ArtifactVerdict = "passed" }},
		{"artifact digest mismatch", func(i *ArtifactWorkspaceContinuityInput) { i.ArtifactDigest = "other" }},
		{"workspace revision mismatch", func(i *ArtifactWorkspaceContinuityInput) { i.CurrentWorkspaceRevision = 2 }},
		{"checkpoint binding mismatch", func(i *ArtifactWorkspaceContinuityInput) { i.CheckpointContractID = "other" }},
		{"CAS digest mismatch", func(i *ArtifactWorkspaceContinuityInput) { i.CASArtifactDigest = "other" }},
		{"missing workspace identity", func(i *ArtifactWorkspaceContinuityInput) { i.CurrentWorkspaceTaskID = "missing" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			input := validArtifactWorkspaceContinuityInput()
			tc.mutate(&input)
			if _, err := ValidateArtifactWorkspaceContinuity(input); err == nil {
				t.Fatal("invalid continuity accepted")
			}
		})
	}
}

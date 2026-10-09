// pattern: Functional Core
package kernel

import (
	"polis/internal/core"
	"polis/internal/gotestprofile"
	"polis/internal/gotestsupervisor"
	"polis/internal/taskvalidation"
	"polis/internal/workspacepatch"
	"strings"
)

type goPatchBuildInput struct {
	record          goJobEvidenceRecord
	validation      taskvalidation.Binding
	base, candidate workspacepatch.Snapshot
}

func buildQualifiedGoPatch(in goPatchBuildInput) ([]byte, taskvalidation.Result, error) {
	b, r, v := in.record.binding, in.record.report, in.validation
	result := taskvalidation.Result{Status: taskvalidation.StatusValidatorUnavailable, ReasonCode: "go_execution_not_qualified"}
	if r.ExecutionKind != "real" || !r.ProcessStarted || !r.ExitObserved || !r.CleanupConfirmed || !r.QualificationEligible || r.Outcome != gotestprofile.OutcomePassed || gotestsupervisor.ValidateReport(b, r) != nil {
		return nil, result, core.Denied
	}
	if v.TaskID != b.TaskID || v.MissionID != b.MissionID || v.ConfigurationDigest != b.TaskValidationBindingSHA256 || taskvalidation.ConfigurationDigest(v) != v.ConfigurationDigest || v.RunnerKind != taskvalidation.FixedGoTestRunnerKind || v.RunnerRevision != b.Profile.ProfileRevision || v.Contract.GoTestProfileRevision != b.Profile.ProfileRevision || !validSHA256(in.record.resultSHA) {
		return nil, result, core.Integrity
	}
	var content strings.Builder
	for _, file := range in.candidate.Files {
		content.Write(file.Content)
		content.WriteByte('\n')
	}
	for _, required := range v.Contract.RequiredText {
		if !strings.Contains(content.String(), required) {
			result.Status = taskvalidation.StatusFail
			result.ReasonCode = "required_text_missing"
			return nil, result, core.Denied
		}
	}
	e := workspacepatch.GoTestEvidence{JobRunID: b.JobRunID, Status: r.Outcome, CompanyID: b.CompanyID, MissionID: b.MissionID, TaskID: b.TaskID, BaseCommitOID: b.BaseCommitOID, InputManifestSHA256: b.InputManifestSHA256, BaseSnapshotArtifactID: b.BaseSnapshotArtifactID, BaseSnapshotSHA256: b.BaseSnapshotSHA256, BaseWorkspaceRevision: b.BaseWorkspaceRevision, BaseFilesSHA256: b.BaseFilesSHA256, CandidateSnapshotArtifactID: b.CandidateSnapshotArtifactID, CandidateSnapshotSHA256: b.CandidateSnapshotSHA256, CandidateWorkspaceRevision: b.CandidateWorkspaceRevision, CandidateFilesSHA256: b.CandidateFilesSHA256, JobBindingSHA256: r.BindingSHA256, TaskValidationBindingSHA256: b.TaskValidationBindingSHA256, ProfileRevision: b.Profile.ProfileRevision, ProfileSHA256: b.ProfileSHA256, RunnerSHA256: b.Profile.RunnerSHA256, ToolchainSHA256: b.Profile.ToolchainSHA256, CommandSHA256: gotestprofile.CommandSHA256(b.Profile), TestManifestSHA256: b.Profile.TestManifestSHA256, ResultSHA256: in.record.resultSHA}
	policy := workspacepatch.ChangePolicy{Manifest: b.Profile, ProfileSHA256: b.ProfileSHA256}
	raw, err := workspacepatch.Build(workspacepatch.BuildInput{Base: in.base, Candidate: in.candidate, Evidence: e, Policy: policy})
	if err != nil {
		return nil, result, core.Integrity
	}
	if _, err = workspacepatch.Apply(in.base, in.candidate, raw, policy); err != nil {
		return nil, result, core.Integrity
	}
	result = taskvalidation.Result{Status: taskvalidation.StatusPass, ReasonCode: "fixed_go_tests_and_patch_replay_passed", Feedback: "Only the approved fixed Go test set, public literal criteria and exact patch replay were verified; no general correctness or malicious-code isolation claim.", TaskID: b.TaskID, MissionID: b.MissionID, AcceptanceRevision: v.AcceptanceRevision, RunnerKind: v.RunnerKind, RunnerRevision: v.RunnerRevision, ConfigurationDigest: v.ConfigurationDigest, WorkspaceDigest: b.CandidateSnapshotSHA256, WorkspaceRevision: b.CandidateWorkspaceRevision}
	return raw, result, nil
}

func patchSnapshot(record goTestSnapshotRecord, artifactID string) workspacepatch.Snapshot {
	m := record.Manifest
	out := workspacepatch.Snapshot{CompanyID: m.CompanyID, MissionID: m.MissionID, TaskID: m.TaskID, SourceCommitOID: m.BaseCommitOID, InputManifestSHA256: m.InputManifestSHA256, ArtifactID: artifactID, ManifestSHA256: record.ArtifactSHA256, RootBindingID: m.RootBindingID, Revision: m.Revision}
	for i, file := range record.Files {
		entry := m.Entries[i]
		out.Files = append(out.Files, workspacepatch.SnapshotFile{RelativePath: file.RelativePath, Digest: file.Digest, Bytes: file.Bytes, FileRevision: entry.FileRevision, SourceRevision: entry.SourceRevision, ContentType: file.ContentType, EntryType: "regular", Content: file.Content})
	}
	return out
}

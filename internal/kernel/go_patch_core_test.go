// pattern: Functional Core
package kernel

import (
	"bytes"
	"polis/internal/fixture"
	"polis/internal/gotestprofile"
	"polis/internal/gotestsupervisor"
	"polis/internal/taskvalidation"
	"polis/internal/workspacepatch"
	"strings"
	"testing"
)

func TestGoPatchRejectsUnexecutedOrSimulatedEvidence(t *testing.T) {
	for _, kind := range []string{"real", "simulation"} {
		input := goPatchBuildInput{record: goJobEvidenceRecord{report: gotestsupervisor.Report{ExecutionKind: kind, Outcome: gotestprofile.OutcomePassed, QualificationEligible: false}}}
		if _, _, err := buildQualifiedGoPatch(input); err == nil {
			t.Fatalf("%s no-execution result qualified a patch", kind)
		}
	}
}

func TestSyntheticGoPatchCoreBindsRealInventoryAndExactReplay(t *testing.T) {
	// Synthetic contract data only. This test executes no candidate and never
	// persists a JobRun, Artifact, checkpoint, or qualification receipt.
	profile := gotestprofile.FixtureManifest(strings.Repeat("f", 64))
	_, profileSHA, _ := gotestprofile.Canonicalize(profile)
	validation, err := taskvalidation.Bind("task", "mission", &taskvalidation.AcceptanceContract{Revision: taskvalidation.AcceptanceContractGoTestRevision, GoTestProfileRevision: gotestprofile.FixtureRevision, RequiredText: []string{"package formatter"}})
	if err != nil {
		t.Fatal(err)
	}
	makeSnapshot := func(revision int64, id, content string) (workspacepatch.Snapshot, string) {
		s := workspacepatch.Snapshot{CompanyID: "company", MissionID: "mission", TaskID: "task", SourceCommitOID: strings.Repeat("a", 40), InputManifestSHA256: strings.Repeat("b", 64), ArtifactID: id, ManifestSHA256: gotestsupervisor.SHA256([]byte(id)), RootBindingID: "workspace", Revision: revision}
		var inventory []gotestprofile.SnapshotFile
		for _, pair := range [][2]string{{"formatter.go", content}, {"go.mod", "module fixture\n\ngo 1.26\n"}, {"frozen_test.go", fixture.Tests("full")}} {
			data := []byte(pair[1])
			sha := gotestsupervisor.SHA256(data)
			s.Files = append(s.Files, workspacepatch.SnapshotFile{RelativePath: pair[0], Digest: sha, Bytes: int64(len(data)), FileRevision: revision, SourceRevision: revision, ContentType: "text/utf-8", EntryType: "regular", Content: data})
			inventory = append(inventory, gotestprofile.SnapshotFile{RelativePath: pair[0], Digest: sha, Bytes: int64(len(data)), ContentType: "text/utf-8", EntryType: "regular", Content: data})
		}
		sha, _, err := gotestprofile.ValidateFixtureSnapshotFiles(inventory)
		if err != nil {
			t.Fatal(err)
		}
		return s, sha
	}
	base, baseSHA := makeSnapshot(1, "base", fixture.Source)
	candidate, candidateSHA := makeSnapshot(2, "candidate", fixture.Source+"\n// approved candidate\n")
	b := gotestprofile.JobBinding{SchemaVersion: gotestprofile.JobBindingSchema, JobRunID: "job-synthetic", CompanyID: "company", MissionID: "mission", TaskID: "task", TaskValidationBindingSHA256: validation.ConfigurationDigest, BaseCommitOID: base.SourceCommitOID, InputManifestSHA256: base.InputManifestSHA256, BaseSnapshotArtifactID: base.ArtifactID, BaseSnapshotSHA256: base.ManifestSHA256, BaseWorkspaceRevision: 1, BaseFilesSHA256: baseSHA, CandidateSnapshotArtifactID: candidate.ArtifactID, CandidateSnapshotSHA256: candidate.ManifestSHA256, CandidateWorkspaceRevision: 2, CandidateFilesSHA256: candidateSHA, ProfileSHA256: profileSHA, Profile: profile}
	r := gotestsupervisor.NewUnstartedReport(b, "real", "exited", "fixed_test_set_passed", gotestsupervisor.HostCheck{Qualified: true, QualificationSHA256: strings.Repeat("c", 64), Limits: profile.Limits})
	var events bytes.Buffer
	for _, name := range []string{"TestFrozenCompatibility", "TestFrozenNeighborChange"} {
		events.WriteString(`{"Action":"pass","Package":"fixture","Test":"` + name + `"}` + "\n")
	}
	events.WriteString(`{"Action":"pass","Package":"fixture"}` + "\n")
	r.Stdout = gotestsupervisor.Log{Data: events.Bytes(), SHA256: gotestsupervisor.SHA256(events.Bytes())}
	zero := 0
	r.ProcessStarted = true
	r.ExitObserved = true
	r.ExitCode = &zero
	r.Outcome = "passed"
	r.QualificationEligible = true
	result := gotestprofile.ClassifyResult(gotestprofile.ExecutionFacts{ExitCodeKnown: true, ExitCode: 0, ProcessTreeReaped: true, CgroupControlsApplied: true, ScratchQuotaApplied: true, NetworkIsolationApplied: true, Stdout: events.Bytes()})
	r.TestResult = &result
	input := goPatchBuildInput{record: goJobEvidenceRecord{binding: b, report: r, resultSHA: gotestsupervisor.SHA256([]byte("synthetic envelope"))}, validation: *validation, base: base, candidate: candidate}
	raw, check, err := buildQualifiedGoPatch(input)
	if err != nil {
		t.Fatal(err)
	}
	if check.Status != taskvalidation.StatusPass || check.WorkspaceDigest != candidate.ManifestSHA256 || check.WorkspaceRevision != 2 {
		t.Fatalf("wrong acceptance binding: %+v", check)
	}
	if _, err = workspacepatch.Apply(base, candidate, raw, workspacepatch.ChangePolicy{Manifest: profile, ProfileSHA256: profileSHA}); err != nil {
		t.Fatal(err)
	}
	input.candidate.Revision++
	if _, _, err = buildQualifiedGoPatch(input); err == nil {
		t.Fatal("old receipt applied to a newer candidate")
	}
}

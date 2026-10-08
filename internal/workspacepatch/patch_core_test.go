// pattern: Functional Core
package workspacepatch

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"testing"
	"unicode/utf8"

	"polis/internal/gotestprofile"
)

func TestBuildAndApplyPatchReconstructsFrozenCandidate(t *testing.T) {
	base := patchSnapshot(1, "base-artifact", map[string]string{
		"formatter.go":   "package formatter\nfunc Render() string { return \"old\" }\n",
		"go.mod":         "module fixture\n\ngo 1.26\n",
		"frozen_test.go": "package formatter\n// trusted baseline\n",
	})
	candidate := patchSnapshot(2, "candidate-artifact", map[string]string{
		"formatter.go":      "package formatter\nfunc Render() string { return \"new\" }\n",
		"formatter_test.go": "package formatter\n// candidate regression test\n",
		"go.mod":            "module fixture\n\ngo 1.26\n",
		"frozen_test.go":    "package formatter\n// trusted baseline\n",
	})
	policy := fixturePatchPolicy()
	evidence := passingGoTestEvidence(base, candidate)

	encoded, err := Build(BuildInput{Base: base, Candidate: candidate, Evidence: evidence, Policy: policy})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	replayed, err := Apply(base, candidate, encoded, policy)
	if err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	if !samePatchFiles(replayed, candidate.Files) {
		t.Fatalf("applied files differ from frozen candidate: got=%+v want=%+v", replayed, candidate.Files)
	}
	bundle, err := Parse(encoded)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if bundle.JobRunID != evidence.JobRunID || bundle.BaseCommitOID != base.SourceCommitOID || bundle.CandidateSnapshotSHA256 != candidate.ManifestSHA256 {
		t.Fatalf("patch lost source/test binding: %+v", bundle)
	}
}

func TestBuildPatchRejectsFailedTestsAndProtectedChanges(t *testing.T) {
	base := patchSnapshot(1, "base-artifact", map[string]string{
		"formatter.go":   "package formatter\nfunc Render() string { return \"old\" }\n",
		"go.mod":         "module fixture\n\ngo 1.26\n",
		"frozen_test.go": "package formatter\n// trusted baseline\n",
	})
	candidate := patchSnapshot(2, "candidate-artifact", map[string]string{
		"formatter.go":   "package formatter\nfunc Render() string { return \"new\" }\n",
		"go.mod":         "module fixture\n\ngo 1.26\n",
		"frozen_test.go": "package formatter\n// trusted baseline\n",
	})
	evidence := passingGoTestEvidence(base, candidate)
	policy := fixturePatchPolicy()

	failedEvidence := evidence
	failedEvidence.Status = "failed"
	if _, err := Build(BuildInput{Base: base, Candidate: candidate, Evidence: failedEvidence, Policy: policy}); !errors.Is(err, ErrInvalidPatch) {
		t.Fatalf("failed Go tests error = %v, want invalid patch", err)
	}
	wrongSnapshotEvidence := evidence
	wrongSnapshotEvidence.CandidateSnapshotSHA256 = strings.Repeat("9", 64)
	if _, err := Build(BuildInput{Base: base, Candidate: candidate, Evidence: wrongSnapshotEvidence, Policy: policy}); !errors.Is(err, ErrInvalidPatch) {
		t.Fatalf("mismatched candidate test receipt error = %v, want invalid patch", err)
	}
	wrongProfile := policy
	wrongProfile.Manifest.Command = append([]string(nil), policy.Manifest.Command...)
	wrongProfile.Manifest.Command[1] = "build"
	if _, err := Build(BuildInput{Base: base, Candidate: candidate, Evidence: evidence, Policy: wrongProfile}); !errors.Is(err, ErrInvalidPatch) {
		t.Fatalf("mutated fixed profile error = %v, want invalid patch", err)
	}

	protectedCandidate := patchSnapshot(2, "candidate-artifact", map[string]string{
		"formatter.go":   "package formatter\nfunc Render() string { return \"new\" }\n",
		"go.mod":         "module attacker\n\ngo 1.26\n",
		"frozen_test.go": "package formatter\n// trusted baseline\n",
	})
	if _, err := Build(BuildInput{Base: base, Candidate: protectedCandidate, Evidence: passingGoTestEvidence(base, protectedCandidate), Policy: policy}); !errors.Is(err, ErrInvalidPatch) {
		t.Fatalf("protected module file change error = %v, want invalid patch", err)
	}
}

func TestBuildPatchRejectsUnsafeOrLinkedSnapshotEntries(t *testing.T) {
	base := patchSnapshot(1, "base-artifact", map[string]string{"formatter.go": "package formatter\n"})
	candidate := patchSnapshot(2, "candidate-artifact", map[string]string{"formatter.go": "package formatter\n// changed\n"})
	policy := fixturePatchPolicy()
	evidence := passingGoTestEvidence(base, candidate)

	unsafe := candidate
	unsafe.Files = []SnapshotFile{patchFile("../escape.go", "package formatter\n", 1, 1)}
	if _, err := Build(BuildInput{Base: base, Candidate: unsafe, Evidence: evidence, Policy: policy}); !errors.Is(err, ErrInvalidPatch) {
		t.Fatalf("path escape error = %v, want invalid patch", err)
	}

	linked := candidate
	linked.Files = []SnapshotFile{patchFile("formatter.go", "package formatter\n// changed\n", 1, 1)}
	linked.Files[0].EntryType = "symlink"
	if _, err := Build(BuildInput{Base: base, Candidate: linked, Evidence: evidence, Policy: policy}); !errors.Is(err, ErrInvalidPatch) {
		t.Fatalf("linked entry error = %v, want invalid patch", err)
	}
}

func TestApplyPatchRejectsChangedBaseAndCorruptPayload(t *testing.T) {
	base := patchSnapshot(1, "base-artifact", map[string]string{"formatter.go": "package formatter\nfunc Render() string { return \"old\" }\n"})
	candidate := patchSnapshot(2, "candidate-artifact", map[string]string{"formatter.go": "package formatter\nfunc Render() string { return \"new\" }\n"})
	policy := fixturePatchPolicy()
	encoded, err := Build(BuildInput{Base: base, Candidate: candidate, Evidence: passingGoTestEvidence(base, candidate), Policy: policy})
	if err != nil {
		t.Fatal(err)
	}
	changedBase := base
	changedBase.Files = []SnapshotFile{patchFile("formatter.go", "package formatter\nfunc Render() string { return \"tampered\" }\n", 1, 1)}
	if _, err = Apply(changedBase, candidate, encoded, policy); !errors.Is(err, ErrInvalidPatch) {
		t.Fatalf("changed base error = %v, want invalid patch", err)
	}
	parsed, err := Parse(encoded)
	if err != nil {
		t.Fatal(err)
	}
	parsed.Files[0].AfterSHA256 = strings.Repeat("0", 64)
	corrupt, err := json.Marshal(parsed)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = Apply(base, candidate, corrupt, policy); !errors.Is(err, ErrInvalidPatch) {
		t.Fatalf("corrupt patch error = %v, want invalid patch", err)
	}
}

func FuzzPatchRoundTrip(f *testing.F) {
	f.Add("before\n", "after\n")
	f.Add("alpha\nbeta\n", "alpha\ngamma\n")
	f.Fuzz(func(t *testing.T, before, after string) {
		if len(before) > 2048 || len(after) > 2048 || !utf8.ValidString(before) || !utf8.ValidString(after) || strings.ContainsRune(before, '\x00') || strings.ContainsRune(after, '\x00') || before == after {
			t.Skip()
		}
		base := patchSnapshot(1, "base-artifact", map[string]string{"formatter.go": "package formatter\n" + before})
		candidate := patchSnapshot(2, "candidate-artifact", map[string]string{"formatter.go": "package formatter\n" + after})
		encoded, err := Build(BuildInput{Base: base, Candidate: candidate, Evidence: passingGoTestEvidence(base, candidate), Policy: fixturePatchPolicy()})
		if err != nil {
			t.Fatalf("Build() error = %v", err)
		}
		replayed, err := Apply(base, candidate, encoded, fixturePatchPolicy())
		if err != nil {
			t.Fatalf("Apply() error = %v", err)
		}
		if !samePatchFiles(replayed, candidate.Files) {
			t.Fatal("patch application did not reconstruct the candidate")
		}
	})
}

func patchSnapshot(revision int64, artifactID string, contents map[string]string) Snapshot {
	files := make([]SnapshotFile, 0, len(contents))
	for path, content := range contents {
		files = append(files, patchFile(path, content, revision, 1))
	}
	return Snapshot{
		CompanyID: "company-1", MissionID: "mission-1", TaskID: "task-1", SourceCommitOID: strings.Repeat("a", 40),
		InputManifestSHA256: strings.Repeat("b", 64), ArtifactID: artifactID, ManifestSHA256: digestString(artifactID),
		RootBindingID: "root-1", Revision: revision, Files: files,
	}
}

func patchFile(path, content string, revision, sourceRevision int64) SnapshotFile {
	bytes := []byte(content)
	hash := sha256.Sum256(bytes)
	return SnapshotFile{
		RelativePath: path, Digest: hex.EncodeToString(hash[:]), Bytes: int64(len(bytes)),
		FileRevision: revision, SourceRevision: sourceRevision, ContentType: "text/utf-8", EntryType: "regular", Content: bytes,
	}
}

func passingGoTestEvidence(base, candidate Snapshot) GoTestEvidence {
	policy := fixturePatchPolicy()
	baseFiles, _ := validateSnapshot(base)
	candidateFiles, _ := validateSnapshot(candidate)
	evidence := GoTestEvidence{
		JobRunID: "go-job-1", Status: "passed", ProfileRevision: policy.Manifest.ProfileRevision,
		CompanyID: base.CompanyID, MissionID: base.MissionID, TaskID: base.TaskID,
		BaseCommitOID: base.SourceCommitOID, InputManifestSHA256: base.InputManifestSHA256,
		BaseSnapshotArtifactID: base.ArtifactID, BaseSnapshotSHA256: base.ManifestSHA256, BaseWorkspaceRevision: base.Revision,
		BaseFilesSHA256:             fileReferencesDigest(fileReferences(baseFiles)),
		CandidateSnapshotArtifactID: candidate.ArtifactID, CandidateSnapshotSHA256: candidate.ManifestSHA256,
		CandidateWorkspaceRevision: candidate.Revision, CandidateFilesSHA256: fileReferencesDigest(fileReferences(candidateFiles)),
		ProfileSHA256: policy.ProfileSHA256, TaskValidationBindingSHA256: strings.Repeat("1", 64),
		RunnerSHA256: policy.Manifest.RunnerSHA256, ToolchainSHA256: policy.Manifest.ToolchainSHA256,
		CommandSHA256: gotestprofile.CommandSHA256(policy.Manifest), TestManifestSHA256: policy.Manifest.TestManifestSHA256,
		ResultSHA256: strings.Repeat("f", 64),
	}
	evidence.JobBindingSHA256 = jobBindingSHA256(evidence, policy.Manifest)
	return evidence
}

func fixturePatchPolicy() ChangePolicy {
	manifest := gotestprofile.FixtureManifest(strings.Repeat("a", 64))
	_, profileSHA256, err := gotestprofile.Canonicalize(manifest)
	if err != nil {
		panic(err)
	}
	return ChangePolicy{Manifest: manifest, ProfileSHA256: profileSHA256}
}

func samePatchFiles(left, right []SnapshotFile) bool {
	if len(left) != len(right) {
		return false
	}
	copyLeft := append([]SnapshotFile(nil), left...)
	copyRight := append([]SnapshotFile(nil), right...)
	sort.Slice(copyLeft, func(i, j int) bool { return copyLeft[i].RelativePath < copyLeft[j].RelativePath })
	sort.Slice(copyRight, func(i, j int) bool { return copyRight[i].RelativePath < copyRight[j].RelativePath })
	for index := range copyLeft {
		if copyLeft[index].RelativePath != copyRight[index].RelativePath || copyLeft[index].Digest != copyRight[index].Digest || copyLeft[index].Bytes != copyRight[index].Bytes || !bytes.Equal(copyLeft[index].Content, copyRight[index].Content) {
			return false
		}
	}
	return true
}

func digestString(value string) string {
	hash := sha256.Sum256([]byte(value))
	return hex.EncodeToString(hash[:])
}

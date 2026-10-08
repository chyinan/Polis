// pattern: Functional Core
package gotestprofile

import (
	"bytes"
	"strings"
	"testing"
)

func TestGoTestJobBindingCanonicalRoundTripAndTamperRejection(t *testing.T) {
	binding := fixtureJobBinding()
	raw, digest, err := CanonicalizeJobBinding(binding)
	if err != nil {
		t.Fatal(err)
	}
	parsed, parsedDigest, err := ParseJobBinding(raw, digest)
	if err != nil {
		t.Fatal(err)
	}
	canonicalAgain, secondDigest, err := CanonicalizeJobBinding(parsed)
	if err != nil {
		t.Fatal(err)
	}
	if digest != parsedDigest || digest != secondDigest || !bytes.Equal(raw, canonicalAgain) {
		t.Fatalf("binding canonicalization changed: %s %s %s", digest, parsedDigest, secondDigest)
	}
	for name, mutate := range map[string]func(*JobBinding){
		"changed candidate digest": func(value *JobBinding) { value.CandidateSnapshotSHA256 = strings.Repeat("9", 64) },
		"changed source commit":    func(value *JobBinding) { value.BaseCommitOID = strings.Repeat("c", 40) },
		"changed profile command":  func(value *JobBinding) { value.Profile.Command[1] = "build" },
		"changed toolchain":        func(value *JobBinding) { value.Profile.ToolchainSHA256 = strings.Repeat("d", 64) },
	} {
		t.Run(name, func(t *testing.T) {
			candidate := fixtureJobBinding()
			candidate.Profile.Command = append([]string(nil), candidate.Profile.Command...)
			mutate(&candidate)
			mutated, _, canonicalErr := CanonicalizeJobBinding(candidate)
			if canonicalErr == nil {
				if _, _, err := ParseJobBinding(mutated, digest); err == nil {
					t.Fatal("tampered immutable Go JobRun binding matched the frozen digest")
				}
			}
		})
	}
}

func fixtureJobBinding() JobBinding {
	profile := FixtureManifest(strings.Repeat("a", 64))
	_, profileSHA256, err := Canonicalize(profile)
	if err != nil {
		panic(err)
	}
	return JobBinding{
		SchemaVersion: JobBindingSchema, JobRunID: "go-job-1", CompanyID: "company-1", MissionID: "mission-1", TaskID: "task-1",
		TaskValidationBindingSHA256: strings.Repeat("1", 64),
		BaseCommitOID:               strings.Repeat("a", 40), InputManifestSHA256: strings.Repeat("b", 64),
		BaseSnapshotArtifactID: "base-artifact", BaseSnapshotSHA256: strings.Repeat("c", 64), BaseWorkspaceRevision: 1, BaseFilesSHA256: strings.Repeat("d", 64),
		CandidateSnapshotArtifactID: "candidate-artifact", CandidateSnapshotSHA256: strings.Repeat("e", 64), CandidateWorkspaceRevision: 2, CandidateFilesSHA256: strings.Repeat("f", 64),
		ProfileSHA256: profileSHA256, Profile: profile,
	}
}

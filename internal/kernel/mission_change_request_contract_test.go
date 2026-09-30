// pattern: Functional Core
package kernel

import (
	"strings"
	"testing"

	"polis/internal/core"
	"polis/internal/taskvalidation"
)

func TestNormalizeMissionChangeRequestUsesCurrentGoalForOmittedFields(t *testing.T) {
	input, err := normalizeMissionChangeRequestInput(MissionChangeRequestInput{ChangeSummary: "  Change the empty result behavior.  "}, "Existing title", "Existing goal")
	if err != nil || input.ChangeSummary != "Change the empty result behavior." || input.ProposedTitle != "Existing title" || input.ProposedGoal != "Existing goal" {
		t.Fatalf("normalized request=%+v err=%v", input, err)
	}
	if _, err = normalizeMissionChangeRequestInput(MissionChangeRequestInput{ChangeSummary: " \t "}, "Existing title", "Existing goal"); err != core.Malformed {
		t.Fatalf("whitespace-only change summary error=%v, want malformed", err)
	}
}

func TestMissionChangeRequirementDigestBindsLatestInputRevisions(t *testing.T) {
	contract := &taskvalidation.AcceptanceContract{Revision: taskvalidation.AcceptanceContractRevision, RequiredText: []string{"Task summary:"}}
	first := []MissionChangeInputRevision{
		{InputID: "input-b", Revision: 2, ContentDigest: strings.Repeat("b", 64), State: "usable"},
		{InputID: "input-a", Revision: 1, ContentDigest: strings.Repeat("a", 64), State: "partial"},
	}
	second := []MissionChangeInputRevision{first[1], first[0]}
	firstDigest, err := missionChangeRequirementsDigest("mission-1", "title", "goal", contract, first)
	if err != nil {
		t.Fatal(err)
	}
	secondDigest, err := missionChangeRequirementsDigest("mission-1", "title", "goal", contract, second)
	if err != nil || firstDigest != secondDigest {
		t.Fatalf("digest depends on input query ordering: %s vs %s err=%v", firstDigest, secondDigest, err)
	}
	second[0].Revision++
	changedDigest, err := missionChangeRequirementsDigest("mission-1", "title", "goal", contract, second)
	if err != nil || changedDigest == firstDigest {
		t.Fatalf("new input revision did not invalidate base digest: %s vs %s err=%v", changedDigest, firstDigest, err)
	}
}

func TestMissionChangeImpactDigestAndLifecycleAreClosed(t *testing.T) {
	impact := MissionChangeImpact{
		SchemaVersion: MissionChangeImpactSchema, MissionID: "mission-1", BaseRequirementsSHA256: strings.Repeat("a", 64),
		InputRevisions: []MissionChangeInputRevision{}, Tasks: []MissionChangeTaskImpact{}, Artifacts: []MissionChangeArtifactImpact{},
		ActiveWorkerSessions: []MissionChangeWorkerImpact{}, NonterminalJobRuns: []MissionChangeJobImpact{}, ActiveServiceEndpoints: []MissionChangeServiceEndpointImpact{},
		ActiveTaskTakeoverLeases: []MissionChangeTakeoverLeaseImpact{}, ReturnedHumanTakeoverSnapshots: []MissionChangeTakeoverSnapshotImpact{},
		NaturalLanguageImpactStatus: "not_assessed",
	}
	firstDigest, err := missionChangeImpactDigest(impact)
	if err != nil {
		t.Fatal(err)
	}
	secondDigest, err := missionChangeImpactDigest(impact)
	if err != nil || firstDigest != secondDigest {
		t.Fatalf("impact digest is not stable: %s vs %s err=%v", firstDigest, secondDigest, err)
	}
	if _, err = missionChangeImpactDigest(MissionChangeImpact{SchemaVersion: "unknown"}); err != core.Malformed {
		t.Fatalf("unknown impact schema error=%v, want malformed", err)
	}
	if !missionChangeSafeBoundary(impact) {
		t.Fatal("empty impact should be a safe boundary")
	}
	unsafeImpacts := []MissionChangeImpact{
		{ActiveWorkerSessions: []MissionChangeWorkerImpact{{SessionID: "session-1"}}},
		{NonterminalJobRuns: []MissionChangeJobImpact{{JobID: "job-1"}}},
		{ActiveServiceEndpoints: []MissionChangeServiceEndpointImpact{{JobID: "job-1", Generation: 1, Readiness: "ready"}}},
		{InputRevisions: []MissionChangeInputRevision{{InputID: "input-1", State: "stored"}}},
	}
	for _, unsafeImpact := range unsafeImpacts {
		if missionChangeSafeBoundary(unsafeImpact) {
			t.Fatalf("unsafe boundary was accepted: %+v", unsafeImpact)
		}
	}
	for _, transition := range [][2]string{{"", "received"}, {"received", "queued"}, {"queued", "considered"}, {"considered", "applied"}, {"considered", "declined"}} {
		if !validMissionChangeRequestTransition(transition[0], transition[1]) {
			t.Fatalf("valid transition %q -> %q was rejected", transition[0], transition[1])
		}
	}
	for _, transition := range [][2]string{{"received", "applied"}, {"queued", "applied"}, {"applied", "considered"}, {"declined", "applied"}} {
		if validMissionChangeRequestTransition(transition[0], transition[1]) {
			t.Fatalf("invalid transition %q -> %q was accepted", transition[0], transition[1])
		}
	}
}

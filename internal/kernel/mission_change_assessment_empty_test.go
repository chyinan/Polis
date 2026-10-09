// pattern: Functional Core
package kernel

import "testing"

func TestPlanningAssessmentRetainsExplicitEmptyClassification(t *testing.T) {
	input := MissionChangePlanningAssessmentInput{ChangeRequestID: "change", AnalysisBasisSHA256: repeatDigest('a'), RiskLevel: "low", Summary: "Only the existing task is affected.", AffectedTaskIDs: []string{"task"}, UnaffectedTaskIDs: []string{}, UncertainTaskIDs: []string{}, Questions: []string{}, RecommendedControls: []string{}}
	impact := MissionChangeImpact{Tasks: []MissionChangeTaskImpact{{TaskID: "task"}}}
	got, err := normalizeMissionChangePlanningAssessment(input, impact)
	if err != nil || got.UnaffectedTaskIDs == nil || got.UncertainTaskIDs == nil {
		t.Fatalf("explicit empty classification lost: %+v %v", got, err)
	}
	input.UnaffectedTaskIDs = nil
	if _, err = normalizeMissionChangePlanningAssessment(input, impact); err == nil {
		t.Fatal("missing classification was accepted as explicit empty")
	}
}

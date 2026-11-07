// pattern: Functional Core
package probe

import "testing"

func TestAssessT18SelectedConfigRejectsNonSingleFactorCandidate(t *testing.T) {
	fields := map[string]T17FieldComparison{
		"model_reasoning_effort": {Name: "model_reasoning_effort", AClassification: T17ConfirmedDifferent, BClassification: T17ConfirmedDifferent, Comparison: T17ConfirmedDifferent},
		"sandbox_mode":           {Name: "sandbox_mode", AClassification: T17ConfirmedDifferent, BClassification: T17ConfirmedDifferent, Comparison: T17ConfirmedDifferent},
		"model_providers.polis-openai.supports_websockets": {Name: "model_providers.polis-openai.supports_websockets", AClassification: T17Unknown, BClassification: T17DefaultInB, Comparison: T17Unknown},
	}
	assessment := AssessT18SelectedConfig(fields, "before-config", "before-transport")
	if assessment.CandidateConstructible || len(assessment.SelectedConfigKeys) != 0 || assessment.EffectiveConfigDigestAfter != "before-config" || assessment.TransportConfigDigestAfter != "before-transport" {
		t.Fatalf("unsafe T18 candidate was accepted: %+v", assessment)
	}
	if assessment.OmittedConfigKeys["model_reasoning_effort"] == "" || assessment.OmittedConfigKeys["model_providers.polis-openai.supports_websockets"] == "" {
		t.Fatalf("omitted reasons incomplete: %+v", assessment.OmittedConfigKeys)
	}
}

func TestAssessT18ReportsFixedFactorsAsUnchanged(t *testing.T) {
	assessment := AssessT18SelectedConfig(map[string]T17FieldComparison{}, "a", "b")
	if assessment.CandidateConstructible || assessment.Reason == "" || len(assessment.UnchangedFactors) == 0 {
		t.Fatalf("empty selected config was not held as NOT_STARTED: %+v", assessment)
	}
}

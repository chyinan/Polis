// pattern: Functional Core

package control

import (
	"strings"
	"testing"
)

func TestResearchSimulationCommandRequestUsesCanonicalBoundedValues(t *testing.T) {
	valid := ResearchSimulationCommandRequest{
		DatasetInputID: "dataset-1", DatasetInputRevision: 1,
		MethodInputID: "method-1", MethodInputRevision: 1,
		Seed: "42", ControlDefinition: "control cohort", RiskBudgetUnits: 128, RequestID: "research-run-1",
	}
	tests := []struct {
		name   string
		mutate func(*ResearchSimulationCommandRequest)
		wantOK bool
	}{
		{name: "valid request", wantOK: true},
		{name: "noncanonical seed", mutate: func(request *ResearchSimulationCommandRequest) { request.Seed = "042" }},
		{name: "risk budget exceeds fixed simulation bound", mutate: func(request *ResearchSimulationCommandRequest) { request.RiskBudgetUnits = 5_120_001 }},
		{name: "risk budget is empty", mutate: func(request *ResearchSimulationCommandRequest) { request.RiskBudgetUnits = 0 }},
		{name: "blank control definition", mutate: func(request *ResearchSimulationCommandRequest) { request.ControlDefinition = "   " }},
		{name: "control definition exceeds rune bound", mutate: func(request *ResearchSimulationCommandRequest) {
			request.ControlDefinition = strings.Repeat("界", 513)
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := valid
			if test.mutate != nil {
				test.mutate(&request)
			}
			if got := validResearchSimulationCommandRequest("company-1", request); got != test.wantOK {
				t.Fatalf("validResearchSimulationCommandRequest()=%v, want %v", got, test.wantOK)
			}
		})
	}
}

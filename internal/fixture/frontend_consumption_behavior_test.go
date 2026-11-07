// pattern: Functional Core
package fixture

import "testing"

func TestFrontendConsumptionReferencePassesPublicBehavior(t *testing.T) {
	if _, err := ParseFrontendConsumptionContract(FrontendConsumptionContractV1); err != nil {
		t.Fatalf("public consumption contract is invalid: %v", err)
	}
	inputs := FrontendConsumptionProbeInputs()
	outputs := make([]FrontendConsumptionStepOutput, 0, len(inputs))
	for _, input := range inputs {
		output := FrontendConsumptionStepOutput{NextAction: FrontendNextAction{Kind: "stop"}}
		for _, item := range input.Response.Items {
			output.RenderedItems = append(output.RenderedItems, item)
		}
		if input.Response.NextCursor != nil {
			output.NextAction = FrontendNextAction{Kind: "request", Cursor: input.Response.NextCursor, Limit: input.RequestedLimit}
		}
		outputs = append(outputs, output)
	}
	report := VerifyFrontendConsumptionOutputsForRuntime(inputs, outputs)
	if !report.Passed {
		t.Fatalf("reference behavior rejected: %+v", report)
	}
}

func TestFrontendConsumptionNegativesArePubliclyDiscriminated(t *testing.T) {
	inputs := FrontendConsumptionProbeInputs()
	wrongID := make([]FrontendConsumptionStepOutput, len(inputs))
	for index := range wrongID {
		wrongID[index] = FrontendConsumptionStepOutput{NextAction: FrontendNextAction{Kind: "stop"}}
	}
	report := VerifyFrontendConsumptionOutputsForRuntime(inputs, wrongID)
	if report.Passed || !hasFrontendCriterion(report, "response_items_not_consumed") || !hasFrontendCriterion(report, "response_name_not_consumed") {
		t.Fatalf("wrong item consumption was not rejected: %+v", report)
	}

	wrongCursor := make([]FrontendConsumptionStepOutput, len(inputs))
	for index, input := range inputs {
		wrongCursor[index] = FrontendConsumptionStepOutput{RenderedItems: input.Response.Items, NextAction: FrontendNextAction{Kind: "request", Cursor: frontendStringPointer("wrong"), Limit: input.RequestedLimit}}
	}
	report = VerifyFrontendConsumptionOutputsForRuntime(inputs, wrongCursor)
	if report.Passed || !hasFrontendCriterion(report, "cursor_not_propagated") || !hasFrontendCriterion(report, "termination_not_observed") {
		t.Fatalf("wrong cursor/termination behavior was not rejected: %+v", report)
	}
}

func TestFrontendConsumptionNeighboringNegatives(t *testing.T) {
	inputs := FrontendConsumptionProbeInputs()
	valid := make([]FrontendConsumptionStepOutput, len(inputs))
	for index, input := range inputs {
		valid[index] = FrontendConsumptionStepOutput{RenderedItems: input.Response.Items}
		if input.Response.NextCursor == nil {
			valid[index].NextAction = FrontendNextAction{Kind: "stop"}
		} else {
			valid[index].NextAction = FrontendNextAction{Kind: "request", Cursor: input.Response.NextCursor, Limit: input.RequestedLimit}
		}
	}
	negatives := []struct {
		name   string
		mutate func([]FrontendConsumptionStepOutput, []FrontendConsumptionStepInput)
		reason string
	}{
		{"ignores id", func(outputs []FrontendConsumptionStepOutput, _ []FrontendConsumptionStepInput) {
			outputs[0].RenderedItems[0].ID = "wrong"
		}, "response_items_not_consumed"},
		{"ignores name", func(outputs []FrontendConsumptionStepOutput, _ []FrontendConsumptionStepInput) {
			outputs[0].RenderedItems[0].Name = "wrong"
		}, "response_name_not_consumed"},
		{"hardcodes cursor", func(outputs []FrontendConsumptionStepOutput, _ []FrontendConsumptionStepInput) {
			outputs[0].NextAction.Cursor = frontendStringPointer("hardcoded")
		}, "cursor_not_propagated"},
		{"ignores limit", func(outputs []FrontendConsumptionStepOutput, _ []FrontendConsumptionStepInput) {
			outputs[0].NextAction.Limit = 1
		}, "limit_not_propagated"},
		{"continues after null", func(outputs []FrontendConsumptionStepOutput, _ []FrontendConsumptionStepInput) {
			outputs[1].NextAction = FrontendNextAction{Kind: "request", Cursor: frontendStringPointer("unexpected"), Limit: 2}
		}, "termination_not_observed"},
	}
	for _, negative := range negatives {
		t.Run(negative.name, func(t *testing.T) {
			outputs := append([]FrontendConsumptionStepOutput(nil), valid...)
			for index := range outputs {
				outputs[index].RenderedItems = append([]FrontendConsumptionItem(nil), outputs[index].RenderedItems...)
			}
			negative.mutate(outputs, inputs)
			report := VerifyFrontendConsumptionOutputsForRuntime(inputs, outputs)
			if report.Passed || !hasFrontendCriterion(report, negative.reason) {
				t.Fatalf("negative behavior was not rejected: %+v", report)
			}
		})
	}
}

func hasFrontendCriterion(report FrontendConsumptionReport, reason string) bool {
	for _, criterion := range report.Criteria {
		if criterion.ReasonCode == reason {
			return true
		}
	}
	return false
}

// pattern: Imperative Shell
package kernel

import (
	"context"
	"encoding/json"
	"fmt"
	"polis/internal/fixture"
	"strconv"
	"strings"
)

func VerifyFrontendConsumptionCandidateSource(ctx context.Context, root, source string) (fixture.FrontendConsumptionReport, error) {
	binding := fixture.CheckFrontendPublicBindingV1(source)
	if !binding.Passed {
		return frontendConsumptionBindingFailure(binding), nil
	}
	inputs := fixture.FrontendConsumptionProbeInputs()
	rawInputs := make([]string, 0, len(inputs))
	for _, input := range inputs {
		raw, err := json.Marshal(input)
		if err != nil {
			return fixture.FrontendConsumptionReport{}, fmt.Errorf("encode frontend consumption input: %w", err)
		}
		rawInputs = append(rawInputs, string(raw))
	}
	lines, failed, err := runPaginationCandidate(ctx, root, source, "frontend", frontendConsumptionHarness(rawInputs))
	if err != nil {
		return fixture.FrontendConsumptionReport{}, err
	}
	if failed {
		return frontendConsumptionRuntimeFailure("candidate_execution_failed", "frontend candidate did not produce a step result"), nil
	}
	if len(lines) != len(inputs) {
		return frontendConsumptionRuntimeFailure("step_cardinality_mismatch", "frontend candidate did not produce one result for each public step"), nil
	}
	outputs := make([]fixture.FrontendConsumptionStepOutput, 0, len(lines))
	for _, line := range lines {
		var outputJSON string
		if err := json.Unmarshal([]byte(line), &outputJSON); err != nil {
			return frontendConsumptionRuntimeFailure("response_output_invalid", "frontend candidate output was not a JSON string result"), nil
		}
		var output fixture.FrontendConsumptionStepOutput
		if strings.TrimSpace(outputJSON) == "" || json.Unmarshal([]byte(outputJSON), &output) != nil {
			return frontendConsumptionRuntimeFailure("response_output_invalid", "frontend candidate result was not a structured public step result"), nil
		}
		outputs = append(outputs, output)
	}
	return VerifyFrontendConsumptionOutputs(inputs, outputs), nil
}

func VerifyFrontendConsumptionOutputs(inputs []fixture.FrontendConsumptionStepInput, outputs []fixture.FrontendConsumptionStepOutput) fixture.FrontendConsumptionReport {
	if len(inputs) != len(outputs) {
		return frontendConsumptionRuntimeFailure("step_cardinality_mismatch", "frontend candidate did not produce one result for each public step")
	}
	return verifyFrontendOutputs(inputs, outputs)
}

func frontendConsumptionHarness(inputs []string) string {
	quoted := make([]string, 0, len(inputs))
	for _, input := range inputs {
		quoted = append(quoted, strconv.Quote(input))
	}
	return "package main\n\nimport (\n\t\"encoding/json\"\n\t\"os\"\n)\n\nfunc main() {\n\tinputs := []string{" + strings.Join(quoted, ",") + "}\n\tencoder := json.NewEncoder(os.Stdout)\n\tfor _, input := range inputs {\n\t\t_ = encoder.Encode(ConsumeItems(input))\n\t}\n}\n"
}

func frontendConsumptionBindingFailure(binding fixture.FrontendBindingReport) fixture.FrontendConsumptionReport {
	return fixture.FrontendConsumptionReport{Passed: false, ContractRevision: fixture.FrontendConsumptionContractRevision, BindingRevision: fixture.FrontendBindingContractRevision, VerifierRevision: fixture.FrontendPaginationBehaviorVerifierRevision, Criteria: []fixture.FrontendConsumptionCriterion{{CriterionID: "public_binding", Passed: false, ReasonCode: binding.ReasonCode, Expected: "public ConsumeItems(step_json string) string binding", Actual: binding.ActionableSummary}}}
}

func frontendConsumptionRuntimeFailure(reason, summary string) fixture.FrontendConsumptionReport {
	return fixture.FrontendConsumptionReport{Passed: false, ContractRevision: fixture.FrontendConsumptionContractRevision, BindingRevision: fixture.FrontendBindingContractRevision, VerifierRevision: fixture.FrontendPaginationBehaviorVerifierRevision, Criteria: []fixture.FrontendConsumptionCriterion{{CriterionID: "runtime", Passed: false, ReasonCode: reason, Expected: "one structured step result per public step", Actual: summary}}}
}

func verifyFrontendOutputs(inputs []fixture.FrontendConsumptionStepInput, outputs []fixture.FrontendConsumptionStepOutput) fixture.FrontendConsumptionReport {
	return fixture.VerifyFrontendConsumptionOutputsForRuntime(inputs, outputs)
}

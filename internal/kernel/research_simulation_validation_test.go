// pattern: Functional Core

package kernel

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"testing"

	"polis/internal/domainworkflow"
)

func TestPersistedResearchSimulationOutputRechecksRiskEquation(t *testing.T) {
	output := domainworkflow.ResearchSimulationOutput{
		SchemaVersion: "polis-research-simulation-output@1", Algorithm: "bootstrap-mean-difference@1", ProtocolRevision: 1,
		DatasetSHA256: stringRepeat('a', 64), MethodSHA256: stringRepeat('b', 64), ControlDefinition: "control cohort", Seed: "42",
		Iterations: 16, SampleSize: 4, RiskConsumedUnits: 128, RiskUnit: "sample_draw",
		ControlMean: 2.5, TreatmentMean: 3.5, MeanDifference: 1, MinimumDifference: 0, MaximumDifference: 2,
	}
	record := ResearchSimulationRunRecord{
		CompanyID: "company-1", RunID: "research-run-1", ProfileID: "research-simulation-reference", ProfileRevision: "research-simulation@1",
		ProtocolRevision: "1", DatasetInputID: "dataset-1", DatasetInputRevision: "1", DatasetSHA256: output.DatasetSHA256,
		MethodInputID: "method-1", MethodInputRevision: "1", MethodSHA256: output.MethodSHA256, Seed: "42", ControlDefinition: output.ControlDefinition,
		RiskUnit: "sample_draw", RiskBudgetUnits: "128", RiskConsumedUnits: "128", OutputSHA256: "", Output: output,
	}
	canonical, err := json.Marshal(output)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(canonical)
	record.OutputSHA256 = hex.EncodeToString(digest[:])
	if !validPersistedResearchSimulationRunOutput(record, canonical) {
		t.Fatal("valid persisted simulation receipt was rejected")
	}

	underreported := record
	underreported.Output.RiskConsumedUnits = 1
	underreported.RiskBudgetUnits = "1"
	underreported.RiskConsumedUnits = "1"
	underreportedBytes, err := json.Marshal(underreported.Output)
	if err != nil {
		t.Fatal(err)
	}
	underreportedDigest := sha256.Sum256(underreportedBytes)
	underreported.OutputSHA256 = hex.EncodeToString(underreportedDigest[:])
	if validPersistedResearchSimulationRunOutput(underreported, underreportedBytes) {
		t.Fatal("underreported sample-draw receipt was accepted")
	}
}

func stringRepeat(value byte, count int) string {
	result := make([]byte, count)
	for index := range result {
		result[index] = value
	}
	return string(result)
}

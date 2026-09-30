// pattern: Functional Core
package domainworkflow

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"testing"
)

func TestRunResearchSimulationIsDeterministicAndSourceBound(t *testing.T) {
	dataset := []byte(`{"control":[1,2,3,4],"treatment":[2,3,4,5]}`)
	method, err := json.Marshal(ResearchSimulationMethod{SchemaVersion: "polis-research-method@1", Algorithm: "bootstrap-mean-difference@1", Iterations: 16, SampleSize: 4})
	if err != nil {
		t.Fatal(err)
	}
	datasetRef := researchSimulatorFixtureReference("dataset-input", 2, dataset)
	methodRef := researchSimulatorFixtureReference("method-input", 1, method)
	protocol := ResearchProtocol{
		Revision: 3, Dataset: datasetRef, MethodSHA256: methodRef.SHA256,
		ControlDefinition: "specified control cohort", RiskBudgetUnits: 128, RiskUnit: "sample_draw",
		Seed: 42, Mode: ExecutionSimulation,
	}
	authorized := []ContentSourceCatalogEntry{{CompanyID: "company-1", Reference: datasetRef}, {CompanyID: "company-1", Reference: methodRef}}
	first, decision := RunResearchSimulation("company-1", protocol, datasetRef, dataset, methodRef, method, authorized)
	if decision.Outcome != OutcomeAccepted {
		t.Fatalf("simulation decision=%+v", decision)
	}
	second, secondDecision := RunResearchSimulation("company-1", protocol, datasetRef, dataset, methodRef, method, authorized)
	if secondDecision.Outcome != OutcomeAccepted || first.OutputSHA256 != second.OutputSHA256 || !bytes.Equal(first.OutputBytes, second.OutputBytes) {
		t.Fatalf("same seed/input produced different outputs: first=%+v second=%+v decision=%+v", first, second, secondDecision)
	}
	if first.Output.RiskConsumedUnits != 128 || first.Output.Seed != "42" || first.Output.Algorithm != "bootstrap-mean-difference@1" {
		t.Fatalf("simulation receipt does not preserve the frozen protocol and actual budget use: %+v", first.Output)
	}
	if first.Output.ProtocolRevision != protocol.Revision || first.Output.DatasetSHA256 != datasetRef.SHA256 || first.Output.MethodSHA256 != methodRef.SHA256 || first.Output.RiskUnit != "sample_draw" {
		t.Fatalf("simulation receipt lost frozen source/control identity: %+v", first.Output)
	}
	if first.Output.MinimumDifference > first.Output.MaximumDifference {
		t.Fatalf("simulation difference bounds are reversed: %+v", first.Output)
	}
}

func TestRunResearchSimulationRejectsUntrustedSourcesAndUnboundedBudget(t *testing.T) {
	dataset := []byte(`{"control":[1,2,3],"treatment":[2,3,4]}`)
	method, err := json.Marshal(ResearchSimulationMethod{SchemaVersion: "polis-research-method@1", Algorithm: "bootstrap-mean-difference@1", Iterations: 20, SampleSize: 4})
	if err != nil {
		t.Fatal(err)
	}
	datasetRef := researchSimulatorFixtureReference("dataset-input", 1, dataset)
	methodRef := researchSimulatorFixtureReference("method-input", 1, method)
	protocol := ResearchProtocol{
		Revision: 1, Dataset: datasetRef, MethodSHA256: methodRef.SHA256,
		ControlDefinition: "specified control cohort", RiskBudgetUnits: 160, RiskUnit: "sample_draw",
		Seed: 7, Mode: ExecutionSimulation,
	}
	validCatalog := []ContentSourceCatalogEntry{{CompanyID: "company-1", Reference: datasetRef}, {CompanyID: "company-1", Reference: methodRef}}
	if _, decision := RunResearchSimulation("company-1", protocol, datasetRef, dataset, methodRef, method, []ContentSourceCatalogEntry{{CompanyID: "company-2", Reference: datasetRef}, {CompanyID: "company-1", Reference: methodRef}}); decision.Outcome != OutcomeRejected {
		t.Fatalf("cross-company dataset was accepted: %+v", decision)
	}
	protocol.RiskBudgetUnits = 159
	if _, decision := RunResearchSimulation("company-1", protocol, datasetRef, dataset, methodRef, method, validCatalog); decision.Outcome != OutcomeRejected {
		t.Fatalf("over-budget simulation was accepted: %+v", decision)
	}
	protocol.RiskBudgetUnits = 160
	if _, decision := RunResearchSimulation("company-1", protocol, datasetRef, append(dataset, ' '), methodRef, method, validCatalog); decision.Outcome != OutcomeRejected {
		t.Fatalf("changed dataset bytes were accepted: %+v", decision)
	}
	protocol.RiskUnit = "unbounded_custom_unit"
	if _, decision := RunResearchSimulation("company-1", protocol, datasetRef, dataset, methodRef, method, validCatalog); decision.Outcome != OutcomeRejected {
		t.Fatalf("unsupported budget unit was accepted: %+v", decision)
	}
}

func researchSimulatorFixtureReference(inputID string, revision int64, data []byte) SourceReference {
	digest := sha256.Sum256(data)
	return SourceReference{InputID: inputID, Revision: revision, SHA256: hex.EncodeToString(digest[:])}
}

// pattern: Functional Core
package domainworkflow

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"math"
	"sort"
	"strconv"
)

const (
	researchSimulationMethodSchema = "polis-research-method@1"
	researchSimulationAlgorithm    = "bootstrap-mean-difference@1"
	researchSimulationOutputSchema = "polis-research-simulation-output@1"
	researchSimulationRiskUnit     = "sample_draw"
	maxResearchDatasetBytes        = 1 << 20
	maxResearchMethodBytes         = 16 << 10
	maxResearchInputValues         = 10_000
	maxResearchSimulationRuns      = 10_000
	maxResearchSampleSize          = 256
)

const MaxResearchSimulationRiskUnits int64 = 5_120_000

type ResearchSimulationMethod struct {
	SchemaVersion string `json:"schemaVersion"`
	Algorithm     string `json:"algorithm"`
	Iterations    int64  `json:"iterations"`
	SampleSize    int64  `json:"sampleSize"`
}

type ResearchSimulationDataset struct {
	Control   []float64 `json:"control"`
	Treatment []float64 `json:"treatment"`
}

type ResearchSimulationOutput struct {
	SchemaVersion     string  `json:"schemaVersion"`
	Algorithm         string  `json:"algorithm"`
	ProtocolRevision  int64   `json:"protocolRevision"`
	DatasetSHA256     string  `json:"datasetSha256"`
	MethodSHA256      string  `json:"methodSha256"`
	ControlDefinition string  `json:"controlDefinition"`
	Seed              string  `json:"seed"`
	Iterations        int64   `json:"iterations"`
	SampleSize        int64   `json:"sampleSize"`
	RiskConsumedUnits int64   `json:"riskConsumedUnits"`
	RiskUnit          string  `json:"riskUnit"`
	ControlMean       float64 `json:"controlMean"`
	TreatmentMean     float64 `json:"treatmentMean"`
	MeanDifference    float64 `json:"meanDifference"`
	MinimumDifference float64 `json:"minimumDifference"`
	MaximumDifference float64 `json:"maximumDifference"`
}

type ResearchSimulationReceipt struct {
	Output       ResearchSimulationOutput
	OutputBytes  []byte
	OutputSHA256 string
}

// RunResearchSimulation runs a fixed, deterministic bootstrap mean-difference
// procedure. It accepts exact same-company MissionInput snapshots and never
// interprets data or method bytes as executable code. The result is a
// simulation artifact only; it does not infer a positive/negative research
// outcome or qualify a domain profile.
func RunResearchSimulation(companyID string, protocol ResearchProtocol, datasetReference SourceReference, datasetBytes []byte, methodReference SourceReference, methodBytes []byte, authorizedSources []ContentSourceCatalogEntry) (ResearchSimulationReceipt, Decision) {
	reasons := make([]string, 0)
	if !validEntityID(companyID) {
		reasons = append(reasons, "research_company_invalid")
	}
	if protocol.Mode != ExecutionSimulation {
		reasons = append(reasons, "research_live_execution_not_supported")
	}
	if protocol.Revision <= 0 || protocol.RiskBudgetUnits <= 0 || protocol.RiskBudgetUnits > MaxResearchSimulationRiskUnits || protocol.RiskUnit != researchSimulationRiskUnit || protocol.ControlDefinition == "" || !validSourceReference(protocol.Dataset) || !validDigest(protocol.MethodSHA256) {
		reasons = append(reasons, "research_protocol_invalid")
	}
	if !validSourceReference(datasetReference) || protocol.Dataset != datasetReference {
		reasons = append(reasons, "research_dataset_revision_mismatch")
	}
	if !validSourceReference(methodReference) || protocol.MethodSHA256 != methodReference.SHA256 {
		reasons = append(reasons, "research_method_revision_mismatch")
	}
	if len(datasetBytes) == 0 || len(datasetBytes) > maxResearchDatasetBytes || !matchesSourceDigest(datasetBytes, datasetReference.SHA256) {
		reasons = append(reasons, "research_dataset_bytes_mismatch")
	}
	if len(methodBytes) == 0 || len(methodBytes) > maxResearchMethodBytes || !matchesSourceDigest(methodBytes, methodReference.SHA256) {
		reasons = append(reasons, "research_method_bytes_mismatch")
	}
	allowedSources := make(map[string]map[int64]string, len(authorizedSources))
	for _, source := range authorizedSources {
		if source.CompanyID != companyID {
			reasons = append(reasons, "research_source_catalog_cross_company")
			continue
		}
		if !validSourceReference(source.Reference) {
			reasons = append(reasons, "research_source_catalog_invalid")
			continue
		}
		revisions := allowedSources[source.Reference.InputID]
		if revisions == nil {
			revisions = make(map[int64]string)
			allowedSources[source.Reference.InputID] = revisions
		}
		if priorDigest, exists := revisions[source.Reference.Revision]; exists && priorDigest != source.Reference.SHA256 {
			reasons = append(reasons, "research_source_catalog_conflict")
			continue
		}
		revisions[source.Reference.Revision] = source.Reference.SHA256
	}
	if reason := authorizedSourceReason(datasetReference, allowedSources, "research_dataset"); reason != "" {
		reasons = append(reasons, reason)
	}
	if reason := authorizedSourceReason(methodReference, allowedSources, "research_method_source"); reason != "" {
		reasons = append(reasons, reason)
	}
	if len(reasons) > 0 {
		sort.Strings(reasons)
		return ResearchSimulationReceipt{}, decision(OutcomeRejected, reasons)
	}

	var dataset ResearchSimulationDataset
	if !decodeSingleJSON(datasetBytes, &dataset) || !validResearchValues(dataset.Control) || !validResearchValues(dataset.Treatment) {
		return ResearchSimulationReceipt{}, decision(OutcomeRejected, []string{"research_dataset_format_invalid"})
	}
	var method ResearchSimulationMethod
	if !decodeSingleJSON(methodBytes, &method) || method.SchemaVersion != researchSimulationMethodSchema || method.Algorithm != researchSimulationAlgorithm ||
		method.Iterations <= 0 || method.Iterations > maxResearchSimulationRuns || method.SampleSize <= 0 || method.SampleSize > maxResearchSampleSize {
		return ResearchSimulationReceipt{}, decision(OutcomeRejected, []string{"research_method_spec_invalid"})
	}
	riskConsumed := 2 * method.Iterations * method.SampleSize
	if riskConsumed > protocol.RiskBudgetUnits {
		return ResearchSimulationReceipt{}, decision(OutcomeRejected, []string{"research_simulation_risk_budget_exceeded"})
	}

	rng := splitMix64{state: protocol.Seed}
	controlMean := mean(dataset.Control)
	treatmentMean := mean(dataset.Treatment)
	differences := make([]float64, method.Iterations)
	for iteration := int64(0); iteration < method.Iterations; iteration++ {
		differences[iteration] = sampleMean(&rng, dataset.Treatment, int(method.SampleSize)) - sampleMean(&rng, dataset.Control, int(method.SampleSize))
	}
	minDifference, maxDifference := differences[0], differences[0]
	differenceTotal := 0.0
	for _, difference := range differences {
		differenceTotal += difference
		if difference < minDifference {
			minDifference = difference
		}
		if difference > maxDifference {
			maxDifference = difference
		}
	}
	output := ResearchSimulationOutput{
		SchemaVersion: researchSimulationOutputSchema, Algorithm: method.Algorithm, ProtocolRevision: protocol.Revision,
		DatasetSHA256: datasetReference.SHA256, MethodSHA256: methodReference.SHA256, Seed: strconv.FormatUint(protocol.Seed, 10),
		ControlDefinition: protocol.ControlDefinition, Iterations: method.Iterations, SampleSize: method.SampleSize, RiskConsumedUnits: riskConsumed, RiskUnit: protocol.RiskUnit,
		ControlMean: controlMean, TreatmentMean: treatmentMean, MeanDifference: differenceTotal / float64(method.Iterations),
		MinimumDifference: minDifference, MaximumDifference: maxDifference,
	}
	outputBytes, err := json.Marshal(output)
	if err != nil {
		return ResearchSimulationReceipt{}, decision(OutcomeRejected, []string{"research_simulation_output_invalid"})
	}
	outputDigest := sha256.Sum256(outputBytes)
	return ResearchSimulationReceipt{
		Output: output, OutputBytes: outputBytes, OutputSHA256: hex.EncodeToString(outputDigest[:]),
	}, Decision{Outcome: OutcomeAccepted, ReasonCodes: []string{}}
}

func ValidResearchSimulationReference(reference SourceReference) bool {
	return validSourceReference(reference)
}

func ValidResearchSimulationReceipt(protocol ResearchProtocol, datasetReference, methodReference SourceReference, receipt ResearchSimulationReceipt) bool {
	if protocol.Mode != ExecutionSimulation || !validSourceReference(datasetReference) || !validSourceReference(methodReference) ||
		protocol.Dataset != datasetReference || protocol.MethodSHA256 != methodReference.SHA256 || protocol.RiskBudgetUnits <= 0 || protocol.RiskBudgetUnits > MaxResearchSimulationRiskUnits ||
		protocol.RiskUnit != researchSimulationRiskUnit || protocol.ControlDefinition == "" {
		return false
	}
	output := receipt.Output
	if output.SchemaVersion != researchSimulationOutputSchema || output.Algorithm != researchSimulationAlgorithm ||
		output.ProtocolRevision != protocol.Revision || output.DatasetSHA256 != datasetReference.SHA256 || output.MethodSHA256 != methodReference.SHA256 ||
		output.Seed != strconv.FormatUint(protocol.Seed, 10) || output.ControlDefinition != protocol.ControlDefinition || output.RiskUnit != protocol.RiskUnit ||
		output.Iterations <= 0 || output.Iterations > maxResearchSimulationRuns || output.SampleSize <= 0 || output.SampleSize > maxResearchSampleSize ||
		output.RiskConsumedUnits != 2*output.Iterations*output.SampleSize || output.RiskConsumedUnits > protocol.RiskBudgetUnits ||
		!finiteResearchNumber(output.ControlMean) || !finiteResearchNumber(output.TreatmentMean) || !finiteResearchNumber(output.MeanDifference) ||
		!finiteResearchNumber(output.MinimumDifference) || !finiteResearchNumber(output.MaximumDifference) || output.MinimumDifference > output.MaximumDifference {
		return false
	}
	outputBytes, err := json.Marshal(output)
	if err != nil || !bytes.Equal(outputBytes, receipt.OutputBytes) {
		return false
	}
	digest := sha256.Sum256(outputBytes)
	return hex.EncodeToString(digest[:]) == receipt.OutputSHA256
}

type splitMix64 struct{ state uint64 }

func (rng *splitMix64) next() uint64 {
	rng.state += 0x9e3779b97f4a7c15
	value := rng.state
	value = (value ^ (value >> 30)) * 0xbf58476d1ce4e5b9
	value = (value ^ (value >> 27)) * 0x94d049bb133111eb
	return value ^ (value >> 31)
}

func decodeSingleJSON(payload []byte, target any) bool {
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return false
	}
	var trailing any
	return decoder.Decode(&trailing) == io.EOF
}

func matchesSourceDigest(payload []byte, expected string) bool {
	if !validDigest(expected) {
		return false
	}
	digest := sha256.Sum256(payload)
	return hex.EncodeToString(digest[:]) == expected
}

func validResearchValues(values []float64) bool {
	if len(values) == 0 || len(values) > maxResearchInputValues {
		return false
	}
	for _, value := range values {
		if !finiteResearchNumber(value) || math.Abs(value) > 1e100 {
			return false
		}
	}
	return true
}

func finiteResearchNumber(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0)
}

func mean(values []float64) float64 {
	total := 0.0
	for _, value := range values {
		total += value
	}
	return total / float64(len(values))
}

func sampleMean(rng *splitMix64, values []float64, sampleSize int) float64 {
	total := 0.0
	for index := 0; index < sampleSize; index++ {
		total += values[rng.next()%uint64(len(values))]
	}
	return total / float64(sampleSize)
}

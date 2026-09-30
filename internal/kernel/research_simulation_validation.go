// pattern: Functional Core

package kernel

import (
	"strconv"

	"polis/internal/domainworkflow"
)

func validPersistedResearchSimulationRunOutput(record ResearchSimulationRunRecord, canonicalOutput []byte) bool {
	if record.ProfileID != domainworkflow.ResearchSimulationProfileID || record.ProfileRevision != domainworkflow.ResearchSimulationProfileRevision {
		return false
	}
	protocolRevision, protocolErr := strconv.ParseInt(record.ProtocolRevision, 10, 64)
	datasetRevision, datasetErr := strconv.ParseInt(record.DatasetInputRevision, 10, 64)
	methodRevision, methodErr := strconv.ParseInt(record.MethodInputRevision, 10, 64)
	seed, seedErr := strconv.ParseUint(record.Seed, 10, 64)
	riskBudget, budgetErr := strconv.ParseInt(record.RiskBudgetUnits, 10, 64)
	riskConsumed, consumedErr := strconv.ParseInt(record.RiskConsumedUnits, 10, 64)
	if protocolErr != nil || datasetErr != nil || methodErr != nil || seedErr != nil || budgetErr != nil || consumedErr != nil ||
		strconv.FormatUint(seed, 10) != record.Seed || record.Output.RiskConsumedUnits != riskConsumed {
		return false
	}
	dataset := domainworkflow.SourceReference{InputID: record.DatasetInputID, Revision: datasetRevision, SHA256: record.DatasetSHA256}
	method := domainworkflow.SourceReference{InputID: record.MethodInputID, Revision: methodRevision, SHA256: record.MethodSHA256}
	protocol := domainworkflow.ResearchProtocol{
		Revision: protocolRevision, Dataset: dataset, MethodSHA256: method.SHA256,
		ControlDefinition: record.ControlDefinition, RiskBudgetUnits: riskBudget, RiskUnit: record.RiskUnit,
		Seed: seed, Mode: domainworkflow.ExecutionSimulation,
	}
	return domainworkflow.ValidResearchSimulationReceipt(protocol, dataset, method, domainworkflow.ResearchSimulationReceipt{
		Output: record.Output, OutputBytes: canonicalOutput, OutputSHA256: record.OutputSHA256,
	})
}

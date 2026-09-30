// pattern: Imperative Shell
package control

import (
	"context"
	"strconv"
	"strings"

	"polis/internal/core"
	"polis/internal/domainworkflow"
	"polis/internal/kernel"
)

type ResearchSimulationCommandRequest struct {
	DatasetInputID       string `json:"datasetInputId"`
	DatasetInputRevision int64  `json:"datasetInputRevision,string"`
	MethodInputID        string `json:"methodInputId"`
	MethodInputRevision  int64  `json:"methodInputRevision,string"`
	Seed                 string `json:"seed"`
	ControlDefinition    string `json:"controlDefinition"`
	RiskBudgetUnits      int64  `json:"riskBudgetUnits"`
	RequestID            string `json:"requestId"`
}

type ResearchSimulationCommandService interface {
	RunResearchSimulation(context.Context, string, ResearchSimulationCommandRequest) (kernel.ResearchSimulationRunRecord, error)
}

func (s *Service) RunResearchSimulation(ctx context.Context, companyID string, request ResearchSimulationCommandRequest) (kernel.ResearchSimulationRunRecord, error) {
	if !validResearchSimulationCommandRequest(companyID, request) {
		return kernel.ResearchSimulationRunRecord{}, core.Malformed
	}
	releaseRequest, err := s.runtime.LockResearchSimulationRequest(ctx, companyID, request.RequestID)
	if err != nil {
		return kernel.ResearchSimulationRunRecord{}, err
	}
	defer releaseRequest()
	seed, _ := strconv.ParseUint(request.Seed, 10, 64)
	if prior, found, err := s.runtime.GetResearchSimulationRunByRequest(ctx, companyID, request.RequestID); err != nil {
		return kernel.ResearchSimulationRunRecord{}, err
	} else if found {
		if !sameResearchSimulationRequest(prior, request) {
			return kernel.ResearchSimulationRunRecord{}, core.Conflict
		}
		return prior, nil
	}
	dataset, err := s.runtime.ReadResearchSimulationInput(ctx, companyID, request.DatasetInputID, request.DatasetInputRevision, 1<<20)
	if err != nil {
		return kernel.ResearchSimulationRunRecord{}, err
	}
	method, err := s.runtime.ReadResearchSimulationInput(ctx, companyID, request.MethodInputID, request.MethodInputRevision, 16<<10)
	if err != nil {
		return kernel.ResearchSimulationRunRecord{}, err
	}
	protocol := domainworkflow.ResearchProtocol{
		Revision: 1, Dataset: dataset.Reference, MethodSHA256: method.Reference.SHA256,
		ControlDefinition: strings.TrimSpace(request.ControlDefinition), RiskBudgetUnits: request.RiskBudgetUnits,
		RiskUnit: "sample_draw", Seed: seed, Mode: domainworkflow.ExecutionSimulation,
	}
	authorizedSources := []domainworkflow.ContentSourceCatalogEntry{
		{CompanyID: companyID, Reference: dataset.Reference},
		{CompanyID: companyID, Reference: method.Reference},
	}
	receipt, decision := domainworkflow.RunResearchSimulation(companyID, protocol, dataset.Reference, dataset.Bytes, method.Reference, method.Bytes, authorizedSources)
	if decision.Outcome != domainworkflow.OutcomeAccepted {
		return kernel.ResearchSimulationRunRecord{}, core.Malformed
	}
	return s.runtime.TXRecordResearchSimulationRun(ctx, companyID, kernel.ResearchSimulationRunInput{
		RequestID: request.RequestID, Protocol: protocol, Dataset: dataset.Reference, Method: method.Reference, Receipt: receipt,
	})
}

func sameResearchSimulationRequest(record kernel.ResearchSimulationRunRecord, request ResearchSimulationCommandRequest) bool {
	seed, err := strconv.ParseUint(request.Seed, 10, 64)
	if err != nil {
		return false
	}
	return record.DatasetInputID == request.DatasetInputID && record.DatasetInputRevision == strconv.FormatInt(request.DatasetInputRevision, 10) &&
		record.MethodInputID == request.MethodInputID && record.MethodInputRevision == strconv.FormatInt(request.MethodInputRevision, 10) &&
		record.Seed == strconv.FormatUint(seed, 10) && record.ControlDefinition == strings.TrimSpace(request.ControlDefinition) &&
		record.RiskBudgetUnits == strconv.FormatInt(request.RiskBudgetUnits, 10)
}

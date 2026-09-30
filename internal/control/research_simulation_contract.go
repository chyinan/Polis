// pattern: Functional Core

package control

import (
	"strconv"
	"strings"
	"unicode/utf8"

	"polis/internal/core"
	"polis/internal/domainworkflow"
)

func validResearchSimulationCommandRequest(companyID string, request ResearchSimulationCommandRequest) bool {
	seed, seedErr := strconv.ParseUint(request.Seed, 10, 64)
	return core.ValidID(companyID) && core.ValidID(request.DatasetInputID) && request.DatasetInputRevision > 0 &&
		core.ValidID(request.MethodInputID) && request.MethodInputRevision > 0 && seedErr == nil &&
		request.Seed == strconv.FormatUint(seed, 10) && strings.TrimSpace(request.ControlDefinition) != "" &&
		utf8.RuneCountInString(strings.TrimSpace(request.ControlDefinition)) <= 512 &&
		request.RiskBudgetUnits > 0 && request.RiskBudgetUnits <= domainworkflow.MaxResearchSimulationRiskUnits &&
		validateRequestID(request.RequestID) == nil
}

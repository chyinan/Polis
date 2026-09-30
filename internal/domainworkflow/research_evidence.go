// pattern: Functional Core
package domainworkflow

import "sort"

// EvaluateResearchWithAuthorizedInputs binds the frozen dataset and method to
// exact revisions in the current Company's verified MissionInput catalog.
// This validates references and execution receipts; it does not independently
// establish that a research method is scientifically sound.
func EvaluateResearchWithAuthorizedInputs(companyID string, protocol ResearchProtocol, result ResearchResult, methodSource SourceReference, authorizedSources []ContentSourceCatalogEntry) Decision {
	base := EvaluateResearch(protocol, result)
	reasons := make([]string, 0, len(base.ReasonCodes))
	if base.Outcome == OutcomeRejected {
		reasons = append(reasons, base.ReasonCodes...)
	}
	if !validEntityID(companyID) {
		reasons = append(reasons, "research_company_invalid")
	}
	allowed := make(map[string]map[int64]string, len(authorizedSources))
	for _, source := range authorizedSources {
		if source.CompanyID != companyID {
			reasons = append(reasons, "research_source_catalog_cross_company")
			continue
		}
		if !validSourceReference(source.Reference) {
			reasons = append(reasons, "research_source_catalog_invalid")
			continue
		}
		revisions := allowed[source.Reference.InputID]
		if revisions == nil {
			revisions = make(map[int64]string)
			allowed[source.Reference.InputID] = revisions
		}
		if existingDigest, exists := revisions[source.Reference.Revision]; exists && existingDigest != source.Reference.SHA256 {
			reasons = append(reasons, "research_source_catalog_conflict")
			continue
		}
		revisions[source.Reference.Revision] = source.Reference.SHA256
	}
	if reason := authorizedSourceReason(protocol.Dataset, allowed, "research_dataset"); reason != "" {
		reasons = append(reasons, reason)
	}
	if !validSourceReference(methodSource) {
		reasons = append(reasons, "research_method_source_invalid")
	} else {
		if methodSource.SHA256 != protocol.MethodSHA256 {
			reasons = append(reasons, "research_method_source_digest_mismatch")
		}
		if reason := authorizedSourceReason(methodSource, allowed, "research_method"); reason != "" {
			reasons = append(reasons, reason)
		}
	}
	if len(reasons) > 0 {
		sort.Strings(reasons)
		return decision(OutcomeRejected, reasons)
	}
	if base.Outcome == OutcomeInconclusive {
		return base
	}
	return Decision{Outcome: OutcomeAccepted, ReasonCodes: []string{}}
}

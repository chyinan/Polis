// pattern: Functional Core
package domainworkflow

import "sort"

// ContentSampleEvidence binds a human-selected sample and its selection plan
// to one immutable draft revision. It does not imply a sampling-rate or
// content-quality threshold.
type ContentSampleEvidence struct {
	DraftRevision       int64           `json:"draftRevision,string"`
	DraftSHA256         string          `json:"draftSha256"`
	Plan                SourceReference `json:"plan"`
	SampledClaimIDs     []string        `json:"sampledClaimIds"`
	SampledByEmployeeID string          `json:"sampledByEmployeeId"`
}

type ContentSourceCatalogEntry struct {
	CompanyID string
	Reference SourceReference
}

func ValidContentSourceReference(reference SourceReference) bool {
	return validSourceReference(reference)
}

// EvaluateContentReviewWithEvidence adds same-company source binding and
// inspectable human-sample coverage to the base content review contract. The
// caller must populate authorizedSources from verified MissionInput/CAS rows;
// this function independently rejects entries from another company.
func EvaluateContentReviewWithEvidence(companyID string, draft ContentDraft, review ContentReview, authorizedSources []ContentSourceCatalogEntry, sample ContentSampleEvidence) Decision {
	base := EvaluateContentReview(draft, review)
	reasons := make([]string, 0, len(base.ReasonCodes))
	if base.Outcome == OutcomeRejected {
		reasons = append(reasons, base.ReasonCodes...)
	}
	if !validEntityID(companyID) {
		reasons = append(reasons, "content_company_invalid")
	}
	allowed := make(map[string]map[int64]string, len(authorizedSources))
	for _, source := range authorizedSources {
		if source.CompanyID != companyID {
			reasons = append(reasons, "content_source_catalog_cross_company")
			continue
		}
		if !validSourceReference(source.Reference) {
			reasons = append(reasons, "content_source_catalog_invalid")
			continue
		}
		revisions := allowed[source.Reference.InputID]
		if revisions == nil {
			revisions = make(map[int64]string)
			allowed[source.Reference.InputID] = revisions
		}
		if existingDigest, exists := revisions[source.Reference.Revision]; exists && existingDigest != source.Reference.SHA256 {
			reasons = append(reasons, "content_source_catalog_conflict")
			continue
		}
		revisions[source.Reference.Revision] = source.Reference.SHA256
	}

	if sample.DraftRevision != draft.Revision || sample.DraftSHA256 != draft.BodySHA256 {
		reasons = append(reasons, "content_sample_draft_mismatch")
	}
	if !validSourceReference(sample.Plan) {
		reasons = append(reasons, "content_sample_plan_invalid")
	} else if reason := authorizedSourceReason(sample.Plan, allowed, "content_sample_plan"); reason != "" {
		reasons = append(reasons, reason)
	}
	if !validEntityID(sample.SampledByEmployeeID) || sample.SampledByEmployeeID != review.CheckerEmployeeID || len(sample.SampledClaimIDs) == 0 {
		reasons = append(reasons, "content_sample_invalid")
	}
	claimIDs := make(map[string]struct{}, len(draft.CriticalClaims))
	for _, claimID := range draft.CriticalClaims {
		claimIDs[claimID] = struct{}{}
	}
	sampled := make(map[string]struct{}, len(sample.SampledClaimIDs))
	for _, claimID := range sample.SampledClaimIDs {
		if _, valid := claimIDs[claimID]; !valid {
			reasons = append(reasons, "content_sample_claim_unknown")
			continue
		}
		if _, duplicate := sampled[claimID]; duplicate {
			reasons = append(reasons, "content_sample_claim_duplicate")
		}
		sampled[claimID] = struct{}{}
	}
	for _, claim := range review.Claims {
		if claim.Finding != ClaimVerified {
			continue
		}
		for _, source := range claim.Sources {
			if reason := authorizedSourceReason(source, allowed, "content_source"); reason != "" {
				reasons = append(reasons, reason)
			}
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

func authorizedSourceReason(source SourceReference, allowed map[string]map[int64]string, reasonPrefix string) string {
	revisions := allowed[source.InputID]
	if revisions == nil {
		return reasonPrefix + "_not_authorized"
	}
	catalogDigest, exists := revisions[source.Revision]
	if !exists {
		return reasonPrefix + "_revision_unavailable"
	}
	if catalogDigest != source.SHA256 {
		return reasonPrefix + "_digest_mismatch"
	}
	return ""
}

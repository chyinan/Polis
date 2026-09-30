// pattern: Functional Core
package domainworkflow

import (
	"strings"
	"testing"
)

func TestContentReviewRequiresCurrentIndependentEvidence(t *testing.T) {
	draft := ContentDraft{
		Revision: 2, WriterEmployeeID: "writer-1", BodySHA256: repeatHash('a'),
		CriticalClaims: []string{"claim-1"}, ConstraintsPassed: true,
	}
	review := ContentReview{
		DraftRevision: 2, CheckerEmployeeID: "checker-1", HumanSampled: true,
		Claims: []ClaimReview{{ClaimID: "claim-1", Finding: ClaimVerified, Sources: []SourceReference{{InputID: "source-1", Revision: 3, SHA256: repeatHash('b')}}}},
	}
	if got := EvaluateContentReview(draft, review); got.Outcome != OutcomeAccepted {
		t.Fatalf("valid independent content review = %+v", got)
	}

	staleDraft := draft
	staleDraft.Revision++
	if got := EvaluateContentReview(staleDraft, review); got.Outcome != OutcomeRejected || !containsReason(got.ReasonCodes, "content_review_stale") {
		t.Fatalf("review survived a draft revision: %+v", got)
	}
	sameWriterReview := review
	sameWriterReview.CheckerEmployeeID = draft.WriterEmployeeID
	if got := EvaluateContentReview(draft, sameWriterReview); got.Outcome != OutcomeRejected || !containsReason(got.ReasonCodes, "content_review_not_independent") {
		t.Fatalf("author self-check was accepted: %+v", got)
	}
}

func TestContentReviewKeepsUnverifiedClaimsOutOfAcceptedDrafts(t *testing.T) {
	draft := ContentDraft{Revision: 1, WriterEmployeeID: "writer-1", BodySHA256: repeatHash('a'), CriticalClaims: []string{"claim-1"}, ConstraintsPassed: true}
	review := ContentReview{DraftRevision: 1, CheckerEmployeeID: "checker-1", HumanSampled: true, Claims: []ClaimReview{{ClaimID: "claim-1", Finding: ClaimVerified}}}
	if got := EvaluateContentReview(draft, review); got.Outcome != OutcomeRejected || !containsReason(got.ReasonCodes, "content_verified_claim_missing_source") {
		t.Fatalf("verified claim without a source was accepted: %+v", got)
	}
	review.Claims[0] = ClaimReview{ClaimID: "claim-1", Finding: ClaimInconclusive, Limitation: "source unavailable"}
	if got := EvaluateContentReview(draft, review); got.Outcome != OutcomeInconclusive {
		t.Fatalf("unavailable source was silently treated as verified: %+v", got)
	}
}

func TestContentReviewBindsAuthorizedSourcesAndHumanSampleToDraftRevision(t *testing.T) {
	draft := ContentDraft{
		Revision: 2, WriterEmployeeID: "writer-1", BodySHA256: repeatHash('a'),
		CriticalClaims: []string{"claim-1", "claim-2"}, ConstraintsPassed: true,
	}
	review := ContentReview{
		DraftRevision: 2, CheckerEmployeeID: "checker-1", HumanSampled: true,
		Claims: []ClaimReview{
			{ClaimID: "claim-1", Finding: ClaimVerified, Sources: []SourceReference{{InputID: "source-1", Revision: 3, SHA256: repeatHash('b')}}},
			{ClaimID: "claim-2", Finding: ClaimVerified, Sources: []SourceReference{{InputID: "source-2", Revision: 1, SHA256: repeatHash('c')}}},
		},
	}
	sources := []ContentSourceCatalogEntry{
		{CompanyID: "company-1", Reference: SourceReference{InputID: "source-1", Revision: 3, SHA256: repeatHash('b')}},
		{CompanyID: "company-1", Reference: SourceReference{InputID: "source-2", Revision: 1, SHA256: repeatHash('c')}},
		{CompanyID: "company-1", Reference: SourceReference{InputID: "sample-plan", Revision: 1, SHA256: repeatHash('d')}},
	}
	sample := ContentSampleEvidence{
		DraftRevision: 2, DraftSHA256: repeatHash('a'), Plan: SourceReference{InputID: "sample-plan", Revision: 1, SHA256: repeatHash('d')},
		SampledClaimIDs: []string{"claim-1"}, SampledByEmployeeID: "checker-1",
	}
	if got := EvaluateContentReviewWithEvidence("company-1", draft, review, sources, sample); got.Outcome != OutcomeAccepted {
		t.Fatalf("authorized sources and draft-bound human sample rejected: %+v", got)
	}

	wrongCompanyCatalog := append([]ContentSourceCatalogEntry(nil), sources...)
	wrongCompanyCatalog[0].CompanyID = "company-2"
	if got := EvaluateContentReviewWithEvidence("company-1", draft, review, wrongCompanyCatalog, sample); got.Outcome != OutcomeRejected || !containsReason(got.ReasonCodes, "content_source_not_authorized") {
		t.Fatalf("source outside the authorized company catalog was accepted: %+v", got)
	}
	wrongDigest := append([]ContentSourceCatalogEntry(nil), sources...)
	wrongDigest[0].Reference.SHA256 = repeatHash('e')
	if got := EvaluateContentReviewWithEvidence("company-1", draft, review, wrongDigest, sample); got.Outcome != OutcomeRejected || !containsReason(got.ReasonCodes, "content_source_digest_mismatch") {
		t.Fatalf("changed source digest was accepted: %+v", got)
	}
	staleSample := sample
	staleSample.DraftRevision++
	if got := EvaluateContentReviewWithEvidence("company-1", draft, review, sources, staleSample); got.Outcome != OutcomeRejected || !containsReason(got.ReasonCodes, "content_sample_draft_mismatch") {
		t.Fatalf("sample from another draft revision was accepted: %+v", got)
	}
	unauthorizedPlan := sample
	unauthorizedPlan.Plan.SHA256 = repeatHash('e')
	if got := EvaluateContentReviewWithEvidence("company-1", draft, review, sources, unauthorizedPlan); got.Outcome != OutcomeRejected || !containsReason(got.ReasonCodes, "content_sample_plan_digest_mismatch") {
		t.Fatalf("sample plan not matching its authorized MissionInput was accepted: %+v", got)
	}
	unknownSample := sample
	unknownSample.SampledClaimIDs = []string{"claim-outside-draft"}
	if got := EvaluateContentReviewWithEvidence("company-1", draft, review, sources, unknownSample); got.Outcome != OutcomeRejected || !containsReason(got.ReasonCodes, "content_sample_claim_unknown") {
		t.Fatalf("sample outside the current draft was accepted: %+v", got)
	}
	duplicateSample := sample
	duplicateSample.SampledClaimIDs = []string{"claim-1", "claim-1"}
	if got := EvaluateContentReviewWithEvidence("company-1", draft, review, sources, duplicateSample); got.Outcome != OutcomeRejected || !containsReason(got.ReasonCodes, "content_sample_claim_duplicate") {
		t.Fatalf("duplicate sampled claim was accepted: %+v", got)
	}
}

func TestContentEvidenceWrapperPreservesValidInconclusiveReview(t *testing.T) {
	draft := ContentDraft{Revision: 1, WriterEmployeeID: "writer-1", BodySHA256: repeatHash('a'), CriticalClaims: []string{"claim-1"}, ConstraintsPassed: true}
	review := ContentReview{DraftRevision: 1, CheckerEmployeeID: "checker-1", HumanSampled: true, Claims: []ClaimReview{{ClaimID: "claim-1", Finding: ClaimInconclusive, Limitation: "source is unavailable"}}}
	plan := SourceReference{InputID: "sample-plan", Revision: 1, SHA256: repeatHash('b')}
	sample := ContentSampleEvidence{DraftRevision: 1, DraftSHA256: draft.BodySHA256, Plan: plan, SampledClaimIDs: []string{"claim-1"}, SampledByEmployeeID: review.CheckerEmployeeID}
	decision := EvaluateContentReviewWithEvidence("company-1", draft, review, []ContentSourceCatalogEntry{{CompanyID: "company-1", Reference: plan}}, sample)
	if decision.Outcome != OutcomeInconclusive || !containsReason(decision.ReasonCodes, "content_claim_inconclusive") {
		t.Fatalf("valid inconclusive content review changed classification: %+v", decision)
	}
}

func TestResearchAcceptancePinsProtocolAndAcceptsNegativeSimulationResults(t *testing.T) {
	protocol := ResearchProtocol{
		Revision: 4, Dataset: SourceReference{InputID: "dataset-1", Revision: 2, SHA256: repeatHash('a')},
		MethodSHA256: repeatHash('b'), ControlDefinition: "compare against frozen baseline", RiskBudgetUnits: 100, RiskUnit: "simulation_units", Seed: 17, Mode: ExecutionSimulation,
	}
	result := ResearchResult{
		ProtocolRevision: 4, DatasetSHA256: repeatHash('a'), MethodSHA256: repeatHash('b'), Mode: ExecutionSimulation,
		Seed: 17, RiskConsumedUnits: 90, OutputSHA256: repeatHash('c'),
		Outcome: ResearchNegative, ResearcherEmployeeID: "researcher-1", EvaluatorEmployeeID: "evaluator-1", EvaluationSHA256: repeatHash('d'),
	}
	if got := EvaluateResearch(protocol, result); got.Outcome != OutcomeAccepted {
		t.Fatalf("complete negative result was not accepted as valid research: %+v", got)
	}
	changedDataset := result
	changedDataset.DatasetSHA256 = repeatHash('d')
	if got := EvaluateResearch(protocol, changedDataset); got.Outcome != OutcomeRejected || !containsReason(got.ReasonCodes, "research_dataset_revision_mismatch") {
		t.Fatalf("research result did not stay pinned to its dataset: %+v", got)
	}
	overBudget := result
	overBudget.RiskConsumedUnits = protocol.RiskBudgetUnits + 1
	if got := EvaluateResearch(protocol, overBudget); got.Outcome != OutcomeRejected || !containsReason(got.ReasonCodes, "research_risk_budget_exceeded") {
		t.Fatalf("simulation result exceeded its frozen risk budget: %+v", got)
	}
	changedSeed := result
	changedSeed.Seed++
	if got := EvaluateResearch(protocol, changedSeed); got.Outcome != OutcomeRejected || !containsReason(got.ReasonCodes, "research_seed_mismatch") {
		t.Fatalf("simulation result changed the frozen seed: %+v", got)
	}
	missingOutput := result
	missingOutput.OutputSHA256 = ""
	if got := EvaluateResearch(protocol, missingOutput); got.Outcome != OutcomeRejected || !containsReason(got.ReasonCodes, "research_simulation_output_missing") {
		t.Fatalf("simulation result without output digest was accepted: %+v", got)
	}
	live := protocol
	live.Mode = ExecutionLive
	if got := EvaluateResearch(live, result); got.Outcome != OutcomeRejected || !containsReason(got.ReasonCodes, "research_live_execution_not_supported") {
		t.Fatalf("live research execution was enabled: %+v", got)
	}
}

func TestResearchAcceptanceBindsDatasetAndMethodToCompanyInputs(t *testing.T) {
	protocol := ResearchProtocol{
		Revision: 1, Dataset: SourceReference{InputID: "dataset-1", Revision: 2, SHA256: repeatHash('a')},
		MethodSHA256: repeatHash('b'), ControlDefinition: "frozen control", RiskBudgetUnits: 20, RiskUnit: "simulation_units", Seed: 9, Mode: ExecutionSimulation,
	}
	result := ResearchResult{
		ProtocolRevision: 1, DatasetSHA256: repeatHash('a'), MethodSHA256: repeatHash('b'), Seed: 9,
		RiskConsumedUnits: 10, OutputSHA256: repeatHash('c'), Mode: ExecutionSimulation,
		Outcome: ResearchNegative, ResearcherEmployeeID: "researcher-1", EvaluatorEmployeeID: "evaluator-1", EvaluationSHA256: repeatHash('d'),
	}
	method := SourceReference{InputID: "method-1", Revision: 3, SHA256: repeatHash('b')}
	catalog := []ContentSourceCatalogEntry{
		{CompanyID: "company-1", Reference: protocol.Dataset},
		{CompanyID: "company-1", Reference: method},
	}
	if got := EvaluateResearchWithAuthorizedInputs("company-1", protocol, result, method, catalog); got.Outcome != OutcomeAccepted {
		t.Fatalf("same-company pinned research inputs rejected: %+v", got)
	}
	crossCompany := append([]ContentSourceCatalogEntry(nil), catalog...)
	crossCompany[0].CompanyID = "company-2"
	if got := EvaluateResearchWithAuthorizedInputs("company-1", protocol, result, method, crossCompany); got.Outcome != OutcomeRejected || !containsReason(got.ReasonCodes, "research_dataset_not_authorized") {
		t.Fatalf("foreign-company dataset was accepted: %+v", got)
	}
	methodMismatch := method
	methodMismatch.SHA256 = repeatHash('e')
	if got := EvaluateResearchWithAuthorizedInputs("company-1", protocol, result, methodMismatch, catalog); got.Outcome != OutcomeRejected || !containsReason(got.ReasonCodes, "research_method_source_digest_mismatch") {
		t.Fatalf("method source bytes not matching the frozen protocol were accepted: %+v", got)
	}
}

func TestResearchEvidenceWrapperPreservesValidInconclusiveResult(t *testing.T) {
	dataset := SourceReference{InputID: "dataset-1", Revision: 1, SHA256: repeatHash('a')}
	method := SourceReference{InputID: "method-1", Revision: 1, SHA256: repeatHash('b')}
	protocol := ResearchProtocol{Revision: 1, Dataset: dataset, MethodSHA256: method.SHA256, ControlDefinition: "frozen control", RiskBudgetUnits: 10, RiskUnit: "simulation_units", Seed: 4, Mode: ExecutionSimulation}
	result := ResearchResult{ProtocolRevision: 1, DatasetSHA256: dataset.SHA256, MethodSHA256: method.SHA256, Seed: 4, RiskConsumedUnits: 5, OutputSHA256: repeatHash('c'), Mode: ExecutionSimulation, Outcome: ResearchInconclusive, ResearcherEmployeeID: "researcher-1", EvaluatorEmployeeID: "evaluator-1", EvaluationSHA256: repeatHash('d')}
	catalog := []ContentSourceCatalogEntry{{CompanyID: "company-1", Reference: dataset}, {CompanyID: "company-1", Reference: method}}
	decision := EvaluateResearchWithAuthorizedInputs("company-1", protocol, result, method, catalog)
	if decision.Outcome != OutcomeInconclusive || !containsReason(decision.ReasonCodes, "research_result_inconclusive") {
		t.Fatalf("valid inconclusive research result changed classification: %+v", decision)
	}
}

func TestReferenceWorkflowsRemainUnqualifiedWithoutDomainEvidence(t *testing.T) {
	profiles := ReferenceWorkflows()
	if len(profiles) != 2 {
		t.Fatalf("reference workflow catalog has %d profiles", len(profiles))
	}
	for _, profile := range profiles {
		if profile.QualificationStatus != "not_run" || profile.ExecutionEnabled || profile.Revision == "" || len(profile.Stages) == 0 {
			t.Fatalf("domain profile claims unsupported qualification: %+v", profile)
		}
	}
}

func TestReferenceWorkflowEvidenceAreasAreDomainSpecific(t *testing.T) {
	profiles := ReferenceWorkflows()
	contentAreas := []DomainEvidenceArea{DomainEvidenceQuality, DomainEvidenceIntervention, DomainEvidenceRecovery, DomainEvidenceCost, DomainEvidenceOrganizationBenefit}
	researchAreas := []DomainEvidenceArea{DomainEvidenceQuality, DomainEvidenceRecovery, DomainEvidenceCost, DomainEvidenceOrganizationBenefit}
	for _, profile := range profiles {
		var expected []DomainEvidenceArea
		switch profile.Domain {
		case "content_operations":
			expected = contentAreas
		case "research_simulation":
			expected = researchAreas
		default:
			t.Fatalf("unexpected reference workflow domain %q", profile.Domain)
		}
		if !sameEvidenceAreas(profile.RequiredEvidence, expected) {
			t.Fatalf("domain %s evidence areas=%v, want %v", profile.Domain, profile.RequiredEvidence, expected)
		}
	}
}

func TestDomainEvidenceReadinessRequiresEveryProfileAreaWithoutQualifyingExecution(t *testing.T) {
	for _, profile := range ReferenceWorkflows() {
		if len(profile.RequiredEvidence) == 0 {
			t.Fatalf("profile %s has no acceptance evidence configuration", profile.ID)
		}
		submission := domainEvidenceSubmissionFor(profile)
		ready := EvaluateDomainEvidence(submission)
		if ready.Status != DomainEvidenceReadyForReview || ready.QualificationStatus != "not_run" || ready.ExecutionEnabled {
			t.Fatalf("complete evidence changed profile availability: %+v", ready)
		}
		empty := EvaluateDomainEvidence(DomainEvidenceSubmission{ProfileID: profile.ID, ProfileRevision: profile.Revision})
		if empty.Status != DomainEvidenceIncomplete || empty.ExecutionEnabled {
			t.Fatalf("empty evidence submission was not classified as incomplete: %+v", empty)
		}
		incomplete := submission
		incomplete.Evidence = append([]DomainEvidence(nil), submission.Evidence[:len(submission.Evidence)-1]...)
		missing := EvaluateDomainEvidence(incomplete)
		if missing.Status != DomainEvidenceIncomplete || !containsReason(missing.ReasonCodes, "domain_evidence_area_missing") || missing.ExecutionEnabled {
			t.Fatalf("missing evidence area was treated as ready: %+v", missing)
		}
	}
}

func sameEvidenceAreas(actual, expected []DomainEvidenceArea) bool {
	if len(actual) != len(expected) {
		return false
	}
	seen := make(map[DomainEvidenceArea]struct{}, len(actual))
	for _, area := range actual {
		if _, exists := seen[area]; exists {
			return false
		}
		seen[area] = struct{}{}
	}
	for _, area := range expected {
		if _, exists := seen[area]; !exists {
			return false
		}
	}
	return true
}

func TestDomainEvidenceRejectsUnknownProfilesAndDuplicateAreas(t *testing.T) {
	profile := ReferenceWorkflows()[0]
	submission := domainEvidenceSubmissionFor(profile)
	submission.Evidence = append(submission.Evidence, submission.Evidence[0])
	duplicate := EvaluateDomainEvidence(submission)
	if duplicate.Status != DomainEvidenceRejected || !containsReason(duplicate.ReasonCodes, "domain_evidence_area_duplicate") || duplicate.ExecutionEnabled {
		t.Fatalf("duplicate evidence area was accepted: %+v", duplicate)
	}
	submission.ProfileRevision = "content-operations@2"
	unknown := EvaluateDomainEvidence(submission)
	if unknown.Status != DomainEvidenceRejected || !containsReason(unknown.ReasonCodes, "domain_profile_unknown") || unknown.ExecutionEnabled {
		t.Fatalf("unknown workflow revision was accepted: %+v", unknown)
	}
}

func TestDomainEvidenceRejectsUnknownAreasAndMalformedReferences(t *testing.T) {
	profile := ReferenceWorkflows()[0]
	cases := []struct {
		name   string
		reason string
		mutate func(*DomainEvidenceSubmission)
	}{
		{name: "unknown area", reason: "domain_evidence_area_invalid", mutate: func(submission *DomainEvidenceSubmission) {
			submission.Evidence[0].Area = DomainEvidenceArea("invented")
		}},
		{name: "invalid artifact revision", reason: "domain_evidence_artifact_invalid", mutate: func(submission *DomainEvidenceSubmission) {
			submission.Evidence[0].Artifact.Revision = 0
		}},
		{name: "invalid artifact digest", reason: "domain_evidence_artifact_invalid", mutate: func(submission *DomainEvidenceSubmission) {
			submission.Evidence[0].Artifact.SHA256 = repeatHash('z')
		}},
		{name: "invalid assessment method", reason: "domain_evidence_method_invalid", mutate: func(submission *DomainEvidenceSubmission) {
			submission.Evidence[0].MethodSHA256 = "not-a-digest"
		}},
		{name: "invalid assessor", reason: "domain_evidence_assessor_invalid", mutate: func(submission *DomainEvidenceSubmission) {
			submission.Evidence[0].AssessedByEmployeeID = ""
		}},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			submission := domainEvidenceSubmissionFor(profile)
			testCase.mutate(&submission)
			got := EvaluateDomainEvidence(submission)
			if got.Status != DomainEvidenceRejected || !containsReason(got.ReasonCodes, testCase.reason) || got.ExecutionEnabled {
				t.Fatalf("malformed evidence returned %+v, want rejection reason %s", got, testCase.reason)
			}
		})
	}
}

func TestDomainEvidenceReviewRequiresIndependentReviewerAndReadySubmission(t *testing.T) {
	profile := ReferenceWorkflows()[0]
	submission := domainEvidenceSubmissionFor(profile)
	decision := DomainEvidenceReview{
		Outcome: DomainEvidenceReviewAccepted, ReviewerEmployeeID: "reviewer-1", Rationale: "references and method records were inspected",
		PreviewedEvidence: domainEvidencePreviewAttestationsFor(submission),
	}
	if reasons := ValidateDomainEvidenceReview(submission, decision); len(reasons) != 0 {
		t.Fatalf("valid independent evidence review rejected: %v", reasons)
	}

	decision.ReviewerEmployeeID = submission.Evidence[0].AssessedByEmployeeID
	if reasons := ValidateDomainEvidenceReview(submission, decision); !containsReason(reasons, "domain_evidence_reviewer_not_independent") {
		t.Fatalf("reviewer who assessed evidence was accepted: %v", reasons)
	}

	decision.ReviewerEmployeeID = "reviewer-1"
	incomplete := submission
	incomplete.Evidence = incomplete.Evidence[:len(incomplete.Evidence)-1]
	if reasons := ValidateDomainEvidenceReview(incomplete, decision); !containsReason(reasons, "domain_evidence_review_submission_not_ready") {
		t.Fatalf("incomplete evidence submission was reviewable: %v", reasons)
	}
}

func TestDomainEvidenceReviewRequiresBoundedRationaleAndKnownOutcome(t *testing.T) {
	submission := domainEvidenceSubmissionFor(ReferenceWorkflows()[1])
	review := DomainEvidenceReview{Outcome: DomainEvidenceReviewAccepted, ReviewerEmployeeID: "reviewer-1", Rationale: "reviewed", PreviewedEvidence: domainEvidencePreviewAttestationsFor(submission)}
	if reasons := ValidateDomainEvidenceReview(submission, review); len(reasons) != 0 {
		t.Fatalf("valid research evidence review rejected: %v", reasons)
	}
	review.Rationale = " \t "
	if reasons := ValidateDomainEvidenceReview(submission, review); !containsReason(reasons, "domain_evidence_review_rationale_invalid") {
		t.Fatalf("empty rationale accepted: %v", reasons)
	}
	review.Rationale = strings.Repeat("x", 2001)
	if reasons := ValidateDomainEvidenceReview(submission, review); !containsReason(reasons, "domain_evidence_review_rationale_invalid") {
		t.Fatalf("oversized rationale accepted: %v", reasons)
	}
	review.Rationale = "reviewed"
	review.Outcome = DomainEvidenceReviewOutcome("approved_for_execution")
	if reasons := ValidateDomainEvidenceReview(submission, review); !containsReason(reasons, "domain_evidence_review_outcome_invalid") {
		t.Fatalf("unknown review outcome accepted: %v", reasons)
	}
}

func TestDomainEvidenceReviewRequiresPreviewedBytesBoundToEveryEvidenceArea(t *testing.T) {
	submission := domainEvidenceSubmissionFor(ReferenceWorkflows()[0])
	review := DomainEvidenceReview{
		Outcome: DomainEvidenceReviewAccepted, ReviewerEmployeeID: "reviewer-1", Rationale: "reviewed source contents",
		PreviewedEvidence: domainEvidencePreviewAttestationsFor(submission),
	}
	if reasons := ValidateDomainEvidenceReview(submission, review); len(reasons) != 0 {
		t.Fatalf("complete preview attestations rejected: %v", reasons)
	}
	review.PreviewedEvidence = review.PreviewedEvidence[:len(review.PreviewedEvidence)-1]
	if reasons := ValidateDomainEvidenceReview(submission, review); !containsReason(reasons, "domain_evidence_review_preview_missing") {
		t.Fatalf("review missing an area's preview was accepted: %v", reasons)
	}
	review.PreviewedEvidence = domainEvidencePreviewAttestationsFor(submission)
	review.PreviewedEvidence[0].SourceDigest = repeatHash('z')
	if reasons := ValidateDomainEvidenceReview(submission, review); !containsReason(reasons, "domain_evidence_review_preview_source_mismatch") {
		t.Fatalf("preview with another source digest was accepted: %v", reasons)
	}
	review.PreviewedEvidence = domainEvidencePreviewAttestationsFor(submission)
	review.PreviewedEvidence[0].RelativePath = "../outside.txt"
	if reasons := ValidateDomainEvidenceReview(submission, review); !containsReason(reasons, "domain_evidence_review_preview_invalid") {
		t.Fatalf("preview with path traversal was accepted: %v", reasons)
	}
	review.PreviewedEvidence = domainEvidencePreviewAttestationsFor(submission)
	review.PreviewedEvidence[0].MediaType = "application/pdf"
	if reasons := ValidateDomainEvidenceReview(submission, review); !containsReason(reasons, "domain_evidence_review_preview_invalid") {
		t.Fatalf("active PDF preview attestation was accepted: %v", reasons)
	}
	for _, reservedPath := range []string{"pdf/extraction.json", "nested/.polis-git-source.json"} {
		review.PreviewedEvidence = domainEvidencePreviewAttestationsFor(submission)
		review.PreviewedEvidence[0].RelativePath = reservedPath
		if reasons := ValidateDomainEvidenceReview(submission, review); !containsReason(reasons, "domain_evidence_review_preview_invalid") {
			t.Fatalf("reserved metadata path %q was accepted for review: %v", reservedPath, reasons)
		}
	}
}

func TestDomainEvidenceSubstantiveReviewClassifiesEveryRequiredArea(t *testing.T) {
	for _, profile := range ReferenceWorkflows() {
		submission := domainEvidenceSubmissionFor(profile)
		review := DomainEvidenceSubstantiveReview{ReviewerEmployeeID: "independent-reviewer", PreviewedEvidence: domainEvidencePreviewAttestationsFor(submission)}
		for _, item := range submission.Evidence {
			review.AreaAssessments = append(review.AreaAssessments, DomainEvidenceAreaAssessment{Area: item.Area, Outcome: DomainAssessmentAccepted, Rationale: "domain evidence supports this area"})
		}
		result, reasons := EvaluateDomainEvidenceSubstantiveReview(submission, DomainEvidenceReviewAccepted, 1, review)
		if len(reasons) != 0 || result.Outcome != DomainEvidenceSubstantiveAccepted || len(result.AreaAssessments) != len(profile.RequiredEvidence) {
			t.Fatalf("complete %s substantive review = %+v reasons=%v", profile.Revision, result, reasons)
		}

		insufficient := review
		insufficient.AreaAssessments = append([]DomainEvidenceAreaAssessment(nil), review.AreaAssessments...)
		insufficient.AreaAssessments[0].Outcome = DomainAssessmentInsufficient
		result, reasons = EvaluateDomainEvidenceSubstantiveReview(submission, DomainEvidenceReviewAccepted, 1, insufficient)
		if len(reasons) != 0 || result.Outcome != DomainEvidenceSubstantiveNeedsMore {
			t.Fatalf("insufficient %s assessment = %+v reasons=%v", profile.Revision, result, reasons)
		}

		rejected := review
		rejected.AreaAssessments = append([]DomainEvidenceAreaAssessment(nil), review.AreaAssessments...)
		rejected.AreaAssessments[len(rejected.AreaAssessments)-1].Outcome = DomainAssessmentRejected
		result, reasons = EvaluateDomainEvidenceSubstantiveReview(submission, DomainEvidenceReviewAccepted, 1, rejected)
		if len(reasons) != 0 || result.Outcome != DomainEvidenceSubstantiveRejected {
			t.Fatalf("rejected %s assessment = %+v reasons=%v", profile.Revision, result, reasons)
		}
	}
}

func TestDomainEvidenceSubstantiveReviewRequiresAcceptedReferencesAndIndependentCoverage(t *testing.T) {
	profile := ReferenceWorkflows()[0]
	submission := domainEvidenceSubmissionFor(profile)
	review := DomainEvidenceSubstantiveReview{ReviewerEmployeeID: submission.Evidence[0].AssessedByEmployeeID, PreviewedEvidence: domainEvidencePreviewAttestationsFor(submission)}
	for _, item := range submission.Evidence {
		review.AreaAssessments = append(review.AreaAssessments, DomainEvidenceAreaAssessment{Area: item.Area, Outcome: DomainAssessmentAccepted, Rationale: "inspected"})
	}
	if _, reasons := EvaluateDomainEvidenceSubstantiveReview(submission, DomainEvidenceReviewAccepted, 1, review); !containsReason(reasons, "domain_evidence_substantive_reviewer_not_independent") {
		t.Fatalf("assessor self-review reasons = %v", reasons)
	}
	review.ReviewerEmployeeID = "independent-reviewer"
	if _, reasons := EvaluateDomainEvidenceSubstantiveReview(submission, DomainEvidenceReviewNeedsMore, 1, review); !containsReason(reasons, "domain_evidence_substantive_references_not_accepted") {
		t.Fatalf("unaccepted references reasons = %v", reasons)
	}
	if _, reasons := EvaluateDomainEvidenceSubstantiveReview(submission, DomainEvidenceReviewAccepted, 0, review); !containsReason(reasons, "domain_evidence_substantive_reference_review_unbound") {
		t.Fatalf("legacy reference review reasons = %v", reasons)
	}
	missing := review
	missing.AreaAssessments = append([]DomainEvidenceAreaAssessment(nil), review.AreaAssessments[1:]...)
	if _, reasons := EvaluateDomainEvidenceSubstantiveReview(submission, DomainEvidenceReviewAccepted, 1, missing); !containsReason(reasons, "domain_evidence_substantive_area_missing") {
		t.Fatalf("missing area assessment reasons = %v", reasons)
	}
}

func domainEvidencePreviewAttestationsFor(submission DomainEvidenceSubmission) []DomainEvidencePreviewAttestation {
	attestations := make([]DomainEvidencePreviewAttestation, 0, len(submission.Evidence))
	for _, evidence := range submission.Evidence {
		attestations = append(attestations, DomainEvidencePreviewAttestation{
			Area: evidence.Area, RelativePath: string(evidence.Area) + ".txt",
			SourceDigest: evidence.Artifact.SHA256, ContentDigest: repeatHash('e'), MediaType: "text/plain",
		})
	}
	return attestations
}

func domainEvidenceSubmissionFor(profile WorkflowProfile) DomainEvidenceSubmission {
	submission := DomainEvidenceSubmission{ProfileID: profile.ID, ProfileRevision: profile.Revision}
	for index, area := range profile.RequiredEvidence {
		submission.Evidence = append(submission.Evidence, DomainEvidence{
			Area:                 area,
			Artifact:             DomainEvidenceArtifact{ID: "evidence-" + string(rune('a'+index)), Revision: 1, SHA256: repeatHash(byte('a' + index%6))},
			MethodSHA256:         repeatHash('f'),
			AssessedByEmployeeID: "assessor-1",
		})
	}
	return submission
}

func repeatHash(char byte) string {
	result := make([]byte, 64)
	for index := range result {
		result[index] = char
	}
	return string(result)
}

func containsReason(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

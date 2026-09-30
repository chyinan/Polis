// pattern: Functional Core
package domainworkflow

import (
	"sort"
	"strings"
	"unicode/utf8"

	"polis/internal/intake"
)

type Outcome string

const (
	OutcomeAccepted     Outcome = "accepted"
	OutcomeRejected     Outcome = "rejected"
	OutcomeInconclusive Outcome = "inconclusive"
)

type SourceReference struct {
	InputID  string `json:"inputId"`
	Revision int64  `json:"revision,string"`
	SHA256   string `json:"sha256"`
}

type ClaimFinding string

const (
	ClaimVerified     ClaimFinding = "verified"
	ClaimInconclusive ClaimFinding = "inconclusive"
	ClaimContradicted ClaimFinding = "contradicted"
)

type ClaimReview struct {
	ClaimID    string            `json:"claimId"`
	Finding    ClaimFinding      `json:"finding"`
	Sources    []SourceReference `json:"sources"`
	Limitation string            `json:"limitation"`
}

type ContentDraft struct {
	Revision          int64    `json:"revision"`
	WriterEmployeeID  string   `json:"writerEmployeeId"`
	BodySHA256        string   `json:"bodySha256"`
	CriticalClaims    []string `json:"criticalClaims"`
	ConstraintsPassed bool     `json:"constraintsPassed"`
}

type ContentReview struct {
	DraftRevision     int64         `json:"draftRevision,string"`
	CheckerEmployeeID string        `json:"checkerEmployeeId"`
	HumanSampled      bool          `json:"humanSampled"`
	Claims            []ClaimReview `json:"claims"`
}

func ValidContentDraft(draft ContentDraft) bool {
	if draft.Revision <= 0 || !validDigest(draft.BodySHA256) || !validEntityID(draft.WriterEmployeeID) || len(draft.CriticalClaims) == 0 || len(draft.CriticalClaims) > 100 {
		return false
	}
	seen := make(map[string]struct{}, len(draft.CriticalClaims))
	for _, claimID := range draft.CriticalClaims {
		if !validEntityID(claimID) {
			return false
		}
		if _, duplicate := seen[claimID]; duplicate {
			return false
		}
		seen[claimID] = struct{}{}
	}
	return true
}

type Decision struct {
	Outcome     Outcome
	ReasonCodes []string
}

func EvaluateContentReview(draft ContentDraft, review ContentReview) Decision {
	reasons := make([]string, 0)
	if !ValidContentDraft(draft) {
		reasons = append(reasons, "content_draft_invalid")
	}
	if !draft.ConstraintsPassed {
		reasons = append(reasons, "content_constraints_not_passed")
	}
	if review.DraftRevision != draft.Revision {
		reasons = append(reasons, "content_review_stale")
	}
	if !validEntityID(review.CheckerEmployeeID) || review.CheckerEmployeeID == draft.WriterEmployeeID {
		reasons = append(reasons, "content_review_not_independent")
	}
	if !review.HumanSampled {
		reasons = append(reasons, "content_human_sample_missing")
	}
	if len(review.Claims) != len(draft.CriticalClaims) {
		reasons = append(reasons, "content_claim_reviews_incomplete")
	}
	claimIDs := make(map[string]struct{}, len(draft.CriticalClaims))
	for _, claimID := range draft.CriticalClaims {
		if !validEntityID(claimID) {
			reasons = append(reasons, "content_claim_id_invalid")
			continue
		}
		if _, exists := claimIDs[claimID]; exists {
			reasons = append(reasons, "content_claim_id_duplicate")
		}
		claimIDs[claimID] = struct{}{}
	}
	reviewed := make(map[string]ClaimReview, len(review.Claims))
	inconclusive := false
	for _, claim := range review.Claims {
		if !validEntityID(claim.ClaimID) {
			reasons = append(reasons, "content_claim_review_invalid")
			continue
		}
		if _, exists := reviewed[claim.ClaimID]; exists {
			reasons = append(reasons, "content_claim_review_duplicate")
		}
		reviewed[claim.ClaimID] = claim
	}
	for claimID := range claimIDs {
		claim, exists := reviewed[claimID]
		if !exists {
			reasons = append(reasons, "content_claim_review_missing")
			continue
		}
		switch claim.Finding {
		case ClaimVerified:
			if len(claim.Sources) == 0 {
				reasons = append(reasons, "content_verified_claim_missing_source")
			}
			for _, source := range claim.Sources {
				if !validSourceReference(source) {
					reasons = append(reasons, "content_source_reference_invalid")
				}
			}
		case ClaimInconclusive:
			if strings.TrimSpace(claim.Limitation) == "" {
				reasons = append(reasons, "content_inconclusive_limitation_missing")
			} else {
				inconclusive = true
			}
		case ClaimContradicted:
			reasons = append(reasons, "content_claim_contradicted")
		default:
			reasons = append(reasons, "content_claim_finding_invalid")
		}
	}
	if len(reasons) > 0 {
		return decision(OutcomeRejected, reasons)
	}
	if inconclusive {
		return Decision{Outcome: OutcomeInconclusive, ReasonCodes: []string{"content_claim_inconclusive"}}
	}
	return Decision{Outcome: OutcomeAccepted, ReasonCodes: []string{}}
}

type ExecutionMode string

const (
	ExecutionSimulation ExecutionMode = "simulation"
	ExecutionLive       ExecutionMode = "live"
)

type ResearchOutcome string

const (
	ResearchPositive     ResearchOutcome = "positive"
	ResearchNegative     ResearchOutcome = "negative"
	ResearchInconclusive ResearchOutcome = "inconclusive"
)

type ResearchProtocol struct {
	Revision          int64
	Dataset           SourceReference
	MethodSHA256      string
	ControlDefinition string
	RiskBudgetUnits   int64
	RiskUnit          string
	Seed              uint64
	Mode              ExecutionMode
}

type ResearchResult struct {
	ProtocolRevision     int64
	DatasetSHA256        string
	MethodSHA256         string
	Seed                 uint64
	RiskConsumedUnits    int64
	OutputSHA256         string
	Mode                 ExecutionMode
	Outcome              ResearchOutcome
	ResearcherEmployeeID string
	EvaluatorEmployeeID  string
	EvaluationSHA256     string
}

func EvaluateResearch(protocol ResearchProtocol, result ResearchResult) Decision {
	reasons := make([]string, 0)
	if protocol.Mode != ExecutionSimulation || result.Mode != ExecutionSimulation {
		reasons = append(reasons, "research_live_execution_not_supported")
	}
	if protocol.Revision <= 0 || !validSourceReference(protocol.Dataset) || !validDigest(protocol.MethodSHA256) || strings.TrimSpace(protocol.ControlDefinition) == "" || protocol.RiskBudgetUnits <= 0 || strings.TrimSpace(protocol.RiskUnit) == "" {
		reasons = append(reasons, "research_protocol_invalid")
	}
	if result.ProtocolRevision != protocol.Revision || result.DatasetSHA256 != protocol.Dataset.SHA256 {
		reasons = append(reasons, "research_dataset_revision_mismatch")
	}
	if result.MethodSHA256 != protocol.MethodSHA256 {
		reasons = append(reasons, "research_method_revision_mismatch")
	}
	if result.Seed != protocol.Seed {
		reasons = append(reasons, "research_seed_mismatch")
	}
	if result.RiskConsumedUnits <= 0 {
		reasons = append(reasons, "research_risk_consumption_invalid")
	} else if result.RiskConsumedUnits > protocol.RiskBudgetUnits {
		reasons = append(reasons, "research_risk_budget_exceeded")
	}
	if !validDigest(result.OutputSHA256) {
		reasons = append(reasons, "research_simulation_output_missing")
	}
	if !validEntityID(result.ResearcherEmployeeID) || !validEntityID(result.EvaluatorEmployeeID) || result.ResearcherEmployeeID == result.EvaluatorEmployeeID {
		reasons = append(reasons, "research_evaluation_not_independent")
	}
	if !validDigest(result.EvaluationSHA256) {
		reasons = append(reasons, "research_evaluation_evidence_missing")
	}
	if result.Outcome != ResearchPositive && result.Outcome != ResearchNegative && result.Outcome != ResearchInconclusive {
		reasons = append(reasons, "research_outcome_invalid")
	}
	if len(reasons) > 0 {
		return decision(OutcomeRejected, reasons)
	}
	if result.Outcome == ResearchInconclusive {
		return Decision{Outcome: OutcomeInconclusive, ReasonCodes: []string{"research_result_inconclusive"}}
	}
	return Decision{Outcome: OutcomeAccepted, ReasonCodes: []string{}}
}

type WorkflowProfile struct {
	ID                  string               `json:"id"`
	Revision            string               `json:"revision"`
	Domain              string               `json:"domain"`
	QualificationStatus string               `json:"qualificationStatus"`
	ExecutionEnabled    bool                 `json:"executionEnabled"`
	Stages              []string             `json:"stages"`
	RequiredEvidence    []DomainEvidenceArea `json:"requiredEvidence"`
}

const (
	ContentOperationsProfileID        = "content-operations-reference"
	ContentOperationsProfileRevision  = "content-operations@1"
	ResearchSimulationProfileID       = "research-simulation-reference"
	ResearchSimulationProfileRevision = "research-simulation@1"
)

type DomainEvidenceArea string

const (
	DomainEvidenceQuality             DomainEvidenceArea = "quality"
	DomainEvidenceIntervention        DomainEvidenceArea = "intervention"
	DomainEvidenceRecovery            DomainEvidenceArea = "recovery"
	DomainEvidenceCost                DomainEvidenceArea = "cost"
	DomainEvidenceOrganizationBenefit DomainEvidenceArea = "organization_benefit"
)

type DomainEvidenceArtifact struct {
	ID       string `json:"id"`
	Revision int64  `json:"revision"`
	SHA256   string `json:"sha256"`
}

type DomainEvidence struct {
	Area                 DomainEvidenceArea     `json:"area"`
	Artifact             DomainEvidenceArtifact `json:"artifact"`
	MethodSHA256         string                 `json:"methodSHA256"`
	AssessedByEmployeeID string                 `json:"assessedByEmployeeId"`
}

type DomainEvidenceSubmission struct {
	ProfileID       string           `json:"profileId"`
	ProfileRevision string           `json:"profileRevision"`
	Evidence        []DomainEvidence `json:"evidence"`
}

type DomainEvidenceStatus string

const (
	DomainEvidenceRejected       DomainEvidenceStatus = "rejected"
	DomainEvidenceIncomplete     DomainEvidenceStatus = "incomplete"
	DomainEvidenceReadyForReview DomainEvidenceStatus = "ready_for_review"
)

type DomainEvidenceReadiness struct {
	Status              DomainEvidenceStatus `json:"status"`
	QualificationStatus string               `json:"qualificationStatus"`
	ExecutionEnabled    bool                 `json:"executionEnabled"`
	ReasonCodes         []string             `json:"reasonCodes"`
}

type DomainEvidenceReviewOutcome string

const (
	DomainEvidenceReviewAccepted          DomainEvidenceReviewOutcome = "evidence_references_accepted"
	DomainEvidenceReviewRejected          DomainEvidenceReviewOutcome = "evidence_references_rejected"
	DomainEvidenceReviewNeedsMore         DomainEvidenceReviewOutcome = "more_evidence_required"
	maxDomainEvidenceReviewRationaleRunes                             = 2000
)

type DomainEvidenceReview struct {
	Outcome            DomainEvidenceReviewOutcome        `json:"outcome"`
	ReviewerEmployeeID string                             `json:"reviewerEmployeeId"`
	Rationale          string                             `json:"rationale"`
	PreviewedEvidence  []DomainEvidencePreviewAttestation `json:"previewedEvidence"`
}

type DomainEvidencePreviewAttestation struct {
	Area          DomainEvidenceArea `json:"area"`
	RelativePath  string             `json:"relativePath"`
	SourceDigest  string             `json:"sourceDigest"`
	ContentDigest string             `json:"contentDigest"`
	MediaType     string             `json:"mediaType"`
}

type DomainEvidenceAreaAssessmentOutcome string

const (
	DomainAssessmentAccepted     DomainEvidenceAreaAssessmentOutcome = "accepted"
	DomainAssessmentRejected     DomainEvidenceAreaAssessmentOutcome = "rejected"
	DomainAssessmentInsufficient DomainEvidenceAreaAssessmentOutcome = "insufficient"
)

type DomainEvidenceSubstantiveOutcome string

const (
	DomainEvidenceSubstantiveAccepted   DomainEvidenceSubstantiveOutcome = "evidence_accepted"
	DomainEvidenceSubstantiveRejected   DomainEvidenceSubstantiveOutcome = "evidence_rejected"
	DomainEvidenceSubstantiveNeedsMore  DomainEvidenceSubstantiveOutcome = "more_evidence_required"
	maxDomainEvidenceAreaRationaleRunes                                  = 1000
)

type DomainEvidenceAreaAssessment struct {
	Area      DomainEvidenceArea                  `json:"area"`
	Outcome   DomainEvidenceAreaAssessmentOutcome `json:"outcome"`
	Rationale string                              `json:"rationale"`
}

type DomainEvidenceSubstantiveReview struct {
	ReviewerEmployeeID string                             `json:"reviewerEmployeeId"`
	AreaAssessments    []DomainEvidenceAreaAssessment     `json:"areaAssessments"`
	PreviewedEvidence  []DomainEvidencePreviewAttestation `json:"previewedEvidence"`
}

type DomainEvidenceSubstantiveAssessment struct {
	Outcome            DomainEvidenceSubstantiveOutcome   `json:"outcome"`
	ReviewerEmployeeID string                             `json:"reviewerEmployeeId"`
	AreaAssessments    []DomainEvidenceAreaAssessment     `json:"areaAssessments"`
	PreviewedEvidence  []DomainEvidencePreviewAttestation `json:"previewedEvidence"`
}

// ValidateDomainEvidenceReview validates a reference review only. An accepted
// result does not qualify the domain profile or enable execution.
func ValidateDomainEvidenceReview(submission DomainEvidenceSubmission, review DomainEvidenceReview) []string {
	reasons := make([]string, 0)
	if EvaluateDomainEvidence(submission).Status != DomainEvidenceReadyForReview {
		reasons = append(reasons, "domain_evidence_review_submission_not_ready")
	}
	if !validEntityID(review.ReviewerEmployeeID) {
		reasons = append(reasons, "domain_evidence_reviewer_invalid")
	}
	if len(review.PreviewedEvidence) == 0 || len(review.PreviewedEvidence) > len(submission.Evidence)*250 {
		reasons = append(reasons, "domain_evidence_review_preview_missing")
	}
	sourceDigestByArea := make(map[DomainEvidenceArea]string, len(submission.Evidence))
	for _, evidence := range submission.Evidence {
		sourceDigestByArea[evidence.Area] = evidence.Artifact.SHA256
	}
	previewedByArea := make(map[DomainEvidenceArea]map[string]struct{}, len(submission.Evidence))
	for _, preview := range review.PreviewedEvidence {
		sourceDigest, areaExists := sourceDigestByArea[preview.Area]
		if !areaExists || !validEvidencePreviewPath(preview.RelativePath) || !validDigest(preview.ContentDigest) || !evidencePreviewMediaTypeValid(preview.MediaType) || intake.IsDomainEvidencePreviewMetadataPath(preview.RelativePath) {
			reasons = append(reasons, "domain_evidence_review_preview_invalid")
			continue
		}
		if preview.SourceDigest != sourceDigest {
			reasons = append(reasons, "domain_evidence_review_preview_source_mismatch")
		}
		paths := previewedByArea[preview.Area]
		if paths == nil {
			paths = make(map[string]struct{})
			previewedByArea[preview.Area] = paths
		}
		if _, exists := paths[preview.RelativePath]; exists {
			reasons = append(reasons, "domain_evidence_review_preview_duplicate")
		}
		paths[preview.RelativePath] = struct{}{}
	}
	for _, evidence := range submission.Evidence {
		if len(previewedByArea[evidence.Area]) == 0 {
			reasons = append(reasons, "domain_evidence_review_preview_missing")
		}
	}
	for _, evidence := range submission.Evidence {
		if review.ReviewerEmployeeID == evidence.AssessedByEmployeeID {
			reasons = append(reasons, "domain_evidence_reviewer_not_independent")
			break
		}
	}
	switch review.Outcome {
	case DomainEvidenceReviewAccepted, DomainEvidenceReviewRejected, DomainEvidenceReviewNeedsMore:
	default:
		reasons = append(reasons, "domain_evidence_review_outcome_invalid")
	}
	rationale := strings.TrimSpace(review.Rationale)
	if rationale == "" || utf8.RuneCountInString(rationale) > maxDomainEvidenceReviewRationaleRunes || strings.ContainsRune(rationale, '\x00') {
		reasons = append(reasons, "domain_evidence_review_rationale_invalid")
	}
	return reasons
}

// EvaluateDomainEvidenceSubstantiveReview records a human decision for one
// evidence submission. It does not change reference-profile qualification or
// execution availability.
func EvaluateDomainEvidenceSubstantiveReview(submission DomainEvidenceSubmission, referenceOutcome DomainEvidenceReviewOutcome, referenceReviewContractRevision int16, review DomainEvidenceSubstantiveReview) (DomainEvidenceSubstantiveAssessment, []string) {
	reasons := make([]string, 0)
	readiness := EvaluateDomainEvidence(submission)
	if readiness.Status != DomainEvidenceReadyForReview {
		reasons = append(reasons, "domain_evidence_substantive_submission_not_ready")
	}
	if referenceOutcome != DomainEvidenceReviewAccepted {
		reasons = append(reasons, "domain_evidence_substantive_references_not_accepted")
	}
	if referenceReviewContractRevision != 1 {
		reasons = append(reasons, "domain_evidence_substantive_reference_review_unbound")
	}
	if !validEntityID(review.ReviewerEmployeeID) {
		reasons = append(reasons, "domain_evidence_substantive_reviewer_invalid")
	}
	profile, exists := referenceProfile(submission.ProfileID, submission.ProfileRevision)
	if !exists {
		return DomainEvidenceSubstantiveAssessment{}, append(reasons, "domain_profile_unknown")
	}

	evidenceByArea := make(map[DomainEvidenceArea]DomainEvidence, len(submission.Evidence))
	for _, item := range submission.Evidence {
		evidenceByArea[item.Area] = item
		if item.AssessedByEmployeeID == review.ReviewerEmployeeID {
			reasons = append(reasons, "domain_evidence_substantive_reviewer_not_independent")
			break
		}
	}
	previewed := make(map[DomainEvidenceArea]map[string]struct{}, len(profile.RequiredEvidence))
	if len(review.PreviewedEvidence) == 0 || len(review.PreviewedEvidence) > len(profile.RequiredEvidence)*250 {
		reasons = append(reasons, "domain_evidence_substantive_preview_missing")
	}
	for _, preview := range review.PreviewedEvidence {
		evidence, areaExists := evidenceByArea[preview.Area]
		if !areaExists || !validEvidencePreviewPath(preview.RelativePath) || !validDigest(preview.ContentDigest) || !evidencePreviewMediaTypeValid(preview.MediaType) || intake.IsDomainEvidencePreviewMetadataPath(preview.RelativePath) {
			reasons = append(reasons, "domain_evidence_substantive_preview_invalid")
			continue
		}
		if preview.SourceDigest != evidence.Artifact.SHA256 {
			reasons = append(reasons, "domain_evidence_substantive_preview_source_mismatch")
		}
		paths := previewed[preview.Area]
		if paths == nil {
			paths = make(map[string]struct{})
			previewed[preview.Area] = paths
		}
		if _, duplicate := paths[preview.RelativePath]; duplicate {
			reasons = append(reasons, "domain_evidence_substantive_preview_duplicate")
		}
		paths[preview.RelativePath] = struct{}{}
	}
	for _, area := range profile.RequiredEvidence {
		if len(previewed[area]) == 0 {
			reasons = append(reasons, "domain_evidence_substantive_preview_missing")
		}
	}
	assessmentByArea := make(map[DomainEvidenceArea]DomainEvidenceAreaAssessment, len(review.AreaAssessments))
	for _, areaAssessment := range review.AreaAssessments {
		if _, areaExists := evidenceByArea[areaAssessment.Area]; !areaExists {
			reasons = append(reasons, "domain_evidence_substantive_area_invalid")
			continue
		}
		if _, duplicate := assessmentByArea[areaAssessment.Area]; duplicate {
			reasons = append(reasons, "domain_evidence_substantive_area_duplicate")
		}
		assessmentByArea[areaAssessment.Area] = areaAssessment
		switch areaAssessment.Outcome {
		case DomainAssessmentAccepted, DomainAssessmentRejected, DomainAssessmentInsufficient:
		default:
			reasons = append(reasons, "domain_evidence_substantive_outcome_invalid")
		}
		rationale := strings.TrimSpace(areaAssessment.Rationale)
		if rationale == "" || utf8.RuneCountInString(rationale) > maxDomainEvidenceAreaRationaleRunes || strings.ContainsRune(rationale, '\x00') {
			reasons = append(reasons, "domain_evidence_substantive_rationale_invalid")
		}
	}
	canonicalAssessments := make([]DomainEvidenceAreaAssessment, 0, len(profile.RequiredEvidence))
	for _, area := range profile.RequiredEvidence {
		assessment, areaExists := assessmentByArea[area]
		if !areaExists {
			reasons = append(reasons, "domain_evidence_substantive_area_missing")
			continue
		}
		canonicalAssessments = append(canonicalAssessments, assessment)
	}
	if len(reasons) != 0 {
		return DomainEvidenceSubstantiveAssessment{}, reasons
	}
	outcome := DomainEvidenceSubstantiveAccepted
	for _, assessment := range canonicalAssessments {
		if assessment.Outcome == DomainAssessmentRejected {
			outcome = DomainEvidenceSubstantiveRejected
			break
		}
		if assessment.Outcome == DomainAssessmentInsufficient {
			outcome = DomainEvidenceSubstantiveNeedsMore
		}
	}
	return DomainEvidenceSubstantiveAssessment{
		Outcome: outcome, ReviewerEmployeeID: review.ReviewerEmployeeID,
		AreaAssessments: canonicalAssessments, PreviewedEvidence: append([]DomainEvidencePreviewAttestation{}, review.PreviewedEvidence...),
	}, nil
}

func referenceProfile(profileID, profileRevision string) (WorkflowProfile, bool) {
	for _, profile := range ReferenceWorkflows() {
		if profile.ID == profileID && profile.Revision == profileRevision {
			return profile, true
		}
	}
	return WorkflowProfile{}, false
}

func validEvidencePreviewPath(value string) bool {
	if value == "" || len(value) > 1024 || strings.HasPrefix(value, "/") || strings.Contains(value, "\\") {
		return false
	}
	for _, segment := range strings.Split(value, "/") {
		if segment == "" || segment == "." || segment == ".." {
			return false
		}
	}
	return true
}

func evidencePreviewMediaTypeValid(mediaType string) bool {
	switch mediaType {
	case "text/plain", "text/markdown", "text/csv", "application/json", "image/png", "image/jpeg":
		return true
	default:
		return false
	}
}

func ReferenceWorkflows() []WorkflowProfile {
	return []WorkflowProfile{
		{
			ID: ContentOperationsProfileID, Revision: ContentOperationsProfileRevision, Domain: "content_operations",
			QualificationStatus: "not_run", ExecutionEnabled: false,
			Stages:           []string{"authorized_sources", "versioned_draft", "independent_fact_check", "human_sample", "simulated_publication", "correction_and_feedback"},
			RequiredEvidence: []DomainEvidenceArea{DomainEvidenceQuality, DomainEvidenceIntervention, DomainEvidenceRecovery, DomainEvidenceCost, DomainEvidenceOrganizationBenefit},
		},
		{
			ID: ResearchSimulationProfileID, Revision: ResearchSimulationProfileRevision, Domain: "research_simulation",
			QualificationStatus: "not_run", ExecutionEnabled: false,
			Stages:           []string{"pin_dataset", "preregister_protocol", "define_control_and_risk_budget", "run_simulation", "independent_evaluation", "retain_negative_result"},
			RequiredEvidence: []DomainEvidenceArea{DomainEvidenceQuality, DomainEvidenceRecovery, DomainEvidenceCost, DomainEvidenceOrganizationBenefit},
		},
	}
}

func EvaluateDomainEvidence(submission DomainEvidenceSubmission) DomainEvidenceReadiness {
	decision := DomainEvidenceReadiness{
		Status: DomainEvidenceRejected, QualificationStatus: "not_run", ExecutionEnabled: false,
		ReasonCodes: []string{},
	}
	var profile WorkflowProfile
	for _, candidate := range ReferenceWorkflows() {
		if candidate.ID == submission.ProfileID && candidate.Revision == submission.ProfileRevision {
			profile = candidate
			break
		}
	}
	if profile.ID == "" {
		decision.ReasonCodes = append(decision.ReasonCodes, "domain_profile_unknown")
		return decision
	}
	allowed := make(map[DomainEvidenceArea]struct{}, len(profile.RequiredEvidence))
	for _, area := range profile.RequiredEvidence {
		allowed[area] = struct{}{}
	}
	byArea := make(map[DomainEvidenceArea]DomainEvidence, len(submission.Evidence))
	for _, evidence := range submission.Evidence {
		if _, exists := allowed[evidence.Area]; !exists {
			decision.ReasonCodes = append(decision.ReasonCodes, "domain_evidence_area_invalid")
			continue
		}
		if _, exists := byArea[evidence.Area]; exists {
			decision.ReasonCodes = append(decision.ReasonCodes, "domain_evidence_area_duplicate")
		}
		byArea[evidence.Area] = evidence
		if !validEntityID(evidence.Artifact.ID) || evidence.Artifact.Revision <= 0 || !validDigest(evidence.Artifact.SHA256) {
			decision.ReasonCodes = append(decision.ReasonCodes, "domain_evidence_artifact_invalid")
		}
		if !validDigest(evidence.MethodSHA256) {
			decision.ReasonCodes = append(decision.ReasonCodes, "domain_evidence_method_invalid")
		}
		if !validEntityID(evidence.AssessedByEmployeeID) {
			decision.ReasonCodes = append(decision.ReasonCodes, "domain_evidence_assessor_invalid")
		}
	}
	missingCount := 0
	for _, area := range profile.RequiredEvidence {
		if _, exists := byArea[area]; !exists {
			decision.ReasonCodes = append(decision.ReasonCodes, "domain_evidence_area_missing")
			missingCount++
		}
	}
	if len(decision.ReasonCodes) > 0 {
		if missingCount > 0 && missingCount == len(decision.ReasonCodes) {
			decision.Status = DomainEvidenceIncomplete
		}
		return decision
	}
	decision.Status = DomainEvidenceReadyForReview
	decision.ReasonCodes = []string{"domain_evidence_requires_human_qualification_review"}
	return decision
}

func validSourceReference(source SourceReference) bool {
	return validEntityID(source.InputID) && source.Revision > 0 && validDigest(source.SHA256)
}

func validEntityID(value string) bool {
	if len(value) == 0 || len(value) > 80 {
		return false
	}
	for _, character := range value {
		if !((character >= 'a' && character <= 'z') || (character >= 'A' && character <= 'Z') || (character >= '0' && character <= '9') || character == '_' || character == '-') {
			return false
		}
	}
	return true
}

func validDigest(value string) bool {
	if len(value) != 64 {
		return false
	}
	for _, character := range value {
		if !((character >= '0' && character <= '9') || (character >= 'a' && character <= 'f')) {
			return false
		}
	}
	return true
}

func decision(outcome Outcome, reasons []string) Decision {
	sort.Strings(reasons)
	unique := make([]string, 0, len(reasons))
	for _, reason := range reasons {
		if len(unique) == 0 || unique[len(unique)-1] != reason {
			unique = append(unique, reason)
		}
	}
	return Decision{Outcome: outcome, ReasonCodes: unique}
}

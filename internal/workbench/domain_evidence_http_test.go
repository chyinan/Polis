// pattern: Imperative Shell
package workbench

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"polis/internal/control"
	"polis/internal/domainworkflow"
	"polis/internal/kernel"
)

type fakeDomainEvidenceService struct {
	fakeCommandService
	ledger                    kernel.DomainEvidenceLedger
	listedFor                 string
	submittedFor              string
	submitted                 control.DomainEvidenceCommandRequest
	reviewedFor               string
	reviewedID                string
	review                    control.DomainEvidenceReviewCommandRequest
	assessedFor               string
	assessedID                string
	assessment                control.DomainEvidenceSubstantiveAssessmentCommandRequest
	qualifiedFor              string
	qualifiedID               string
	qualification             control.DomainProfileQualificationCommandRequest
	previewedFor              string
	previewedID               string
	previewedArea             domainworkflow.DomainEvidenceArea
	previewedPath             string
	simulationFor             string
	simulationReq             control.ResearchSimulationCommandRequest
	contentSourceFor          string
	contentSource             control.DomainContentSourceAuthorizationCommandRequest
	contentDraftFor           string
	contentDraft              control.DomainContentDraftCommandRequest
	contentReviewFor          string
	contentReviewInputID      string
	contentReviewRevision     int64
	contentReview             control.DomainContentReviewCommandRequest
	contentPublicationFor     string
	contentPublicationRequest control.DomainContentPublicationCommandRequest
	contentCorrectionFor      string
	contentCorrectionRequest  control.DomainContentCorrectionCommandRequest
	contentFeedbackFor        string
	contentFeedbackRequest    control.DomainContentFeedbackCommandRequest
}

func (service *fakeDomainEvidenceService) SetContentSourceAuthorization(_ context.Context, companyID string, request control.DomainContentSourceAuthorizationCommandRequest) (kernel.DomainContentSourceEventRecord, error) {
	service.contentSourceFor, service.contentSource = companyID, request
	return kernel.DomainContentSourceEventRecord{CompanyID: companyID, EventID: "content-source-event-1", InputID: request.SourceInputID, Revision: "1", SHA256: strings.Repeat("a", 64), State: request.State, RequestID: request.RequestID}, nil
}

func (service *fakeDomainEvidenceService) RegisterContentDraft(_ context.Context, companyID string, request control.DomainContentDraftCommandRequest) (kernel.DomainContentDraftRecord, error) {
	service.contentDraftFor, service.contentDraft = companyID, request
	return kernel.DomainContentDraftRecord{CompanyID: companyID, DraftID: "content-draft-1", DraftInputID: request.DraftInputID, DraftRevision: "1", DraftSHA256: strings.Repeat("a", 64), WriterEmployeeID: request.WriterEmployeeID, CriticalClaims: request.CriticalClaims, ConstraintsPassed: request.ConstraintsPassed, RequestID: request.RequestID}, nil
}

func (service *fakeDomainEvidenceService) RecordContentReview(_ context.Context, companyID, draftInputID string, revision int64, request control.DomainContentReviewCommandRequest) (kernel.DomainContentReviewRecord, error) {
	service.contentReviewFor, service.contentReviewInputID, service.contentReviewRevision, service.contentReview = companyID, draftInputID, revision, request
	return kernel.DomainContentReviewRecord{CompanyID: companyID, ReviewID: "content-review-1", DraftInputID: draftInputID, DraftRevision: "1", DraftSHA256: strings.Repeat("a", 64), CheckerEmployeeID: request.Review.CheckerEmployeeID, Outcome: domainworkflow.OutcomeAccepted, Review: request.Review, Sample: request.Sample, ReasonCodes: []string{}, RequestID: request.RequestID}, nil
}

func (service *fakeDomainEvidenceService) SimulateContentPublication(_ context.Context, companyID string, request control.DomainContentPublicationCommandRequest) (kernel.DomainContentPublicationRecord, error) {
	service.contentPublicationFor, service.contentPublicationRequest = companyID, request
	return kernel.DomainContentPublicationRecord{CompanyID: companyID, PublicationID: "content-publication-1", ReviewID: request.ReviewID, Mode: "simulation", ExternalSideEffects: false, RequestID: request.RequestID}, nil
}

func (service *fakeDomainEvidenceService) RecordContentCorrection(_ context.Context, companyID string, request control.DomainContentCorrectionCommandRequest) (kernel.DomainContentCorrectionRecord, error) {
	service.contentCorrectionFor, service.contentCorrectionRequest = companyID, request
	return kernel.DomainContentCorrectionRecord{CompanyID: companyID, CorrectionID: "content-correction-1", PublicationID: request.PublicationID, State: "review_required", RequestID: request.RequestID}, nil
}

func (service *fakeDomainEvidenceService) RecordContentFeedback(_ context.Context, companyID string, request control.DomainContentFeedbackCommandRequest) (kernel.DomainContentFeedbackRecord, error) {
	service.contentFeedbackFor, service.contentFeedbackRequest = companyID, request
	return kernel.DomainContentFeedbackRecord{CompanyID: companyID, FeedbackID: "content-feedback-1", PublicationID: request.PublicationID, Category: request.Category, State: "review_required", RequestID: request.RequestID}, nil
}

func (service *fakeDomainEvidenceService) RunResearchSimulation(_ context.Context, companyID string, request control.ResearchSimulationCommandRequest) (kernel.ResearchSimulationRunRecord, error) {
	service.simulationFor = companyID
	service.simulationReq = request
	return kernel.ResearchSimulationRunRecord{
		CompanyID: companyID, RunID: "research-run-1", ProfileID: domainworkflow.ResearchSimulationProfileID,
		ProfileRevision: domainworkflow.ResearchSimulationProfileRevision, DatasetInputID: request.DatasetInputID,
		DatasetInputRevision: "1", DatasetSHA256: strings.Repeat("a", 64), MethodInputID: request.MethodInputID,
		MethodInputRevision: "1", MethodSHA256: strings.Repeat("b", 64), Seed: request.Seed,
		ControlDefinition: request.ControlDefinition, RiskUnit: "sample_draw", RiskBudgetUnits: "100", RiskConsumedUnits: "80",
		OutputSHA256: strings.Repeat("c", 64), RequestID: request.RequestID,
	}, nil
}

func (service *fakeDomainEvidenceService) ListDomainEvidenceArtifactPreviewEntries(_ context.Context, companyID, recordID string, area domainworkflow.DomainEvidenceArea) (kernel.DomainEvidenceArtifactPreviewManifest, error) {
	return kernel.DomainEvidenceArtifactPreviewManifest{
		CompanyID: companyID, RecordID: recordID, Area: area, InputID: "input-1", InputRevision: 1, SourceDigest: strings.Repeat("a", 64),
		Entries: []kernel.DomainEvidenceArtifactPreviewEntry{{RelativePath: "evidence.txt", FileName: "evidence.txt", MediaType: "text/plain", ByteSize: 12, ContentSHA256: strings.Repeat("b", 64), Previewable: true}},
	}, nil
}

func (service *fakeDomainEvidenceService) ReadDomainEvidenceArtifactPreview(_ context.Context, companyID, recordID string, area domainworkflow.DomainEvidenceArea, relativePath string) (kernel.DomainEvidenceArtifactPreview, error) {
	service.previewedFor = companyID
	service.previewedID = recordID
	service.previewedArea = area
	service.previewedPath = relativePath
	return kernel.DomainEvidenceArtifactPreview{
		CompanyID: companyID, RecordID: recordID, Area: area, InputID: "input-1", InputRevision: 1,
		SourceDigest: strings.Repeat("a", 64), FileName: "evidence.txt", MediaType: "text/plain",
		ContentSHA256: strings.Repeat("b", 64), Content: []byte("reviewable evidence text"),
	}, nil
}

func (service *fakeDomainEvidenceService) RecordDomainEvidenceReview(_ context.Context, companyID, recordID string, request control.DomainEvidenceReviewCommandRequest) (kernel.DomainEvidenceReviewRecord, error) {
	service.reviewedFor = companyID
	service.reviewedID = recordID
	service.review = request
	return kernel.DomainEvidenceReviewRecord{
		CompanyID: companyID, ReviewID: "domain-review-1", RecordID: recordID,
		Outcome: request.Outcome, ReviewerEmployeeID: request.ReviewerEmployeeID,
		Rationale: request.Rationale, ReviewContractRevision: 1, PreviewedEvidence: request.PreviewedEvidence, RequestID: request.RequestID,
	}, nil
}

func (service *fakeDomainEvidenceService) RecordDomainEvidenceSubstantiveAssessment(_ context.Context, companyID, recordID string, request control.DomainEvidenceSubstantiveAssessmentCommandRequest) (kernel.DomainEvidenceSubstantiveAssessmentRecord, error) {
	service.assessedFor = companyID
	service.assessedID = recordID
	service.assessment = request
	return kernel.DomainEvidenceSubstantiveAssessmentRecord{
		CompanyID: companyID, AssessmentID: "domain-substantive-1", RecordID: recordID, EvidenceDigest: strings.Repeat("a", 64),
		Outcome: domainworkflow.DomainEvidenceSubstantiveNeedsMore, ReviewerEmployeeID: request.ReviewerEmployeeID,
		AreaAssessments: request.AreaAssessments, PreviewedEvidence: request.PreviewedEvidence, RequestID: request.RequestID,
	}, nil
}

func (service *fakeDomainEvidenceService) RecordDomainProfileQualification(_ context.Context, companyID, profileID string, request control.DomainProfileQualificationCommandRequest) (kernel.DomainProfileQualificationRecord, error) {
	service.qualifiedFor = companyID
	service.qualifiedID = profileID
	service.qualification = request
	return kernel.DomainProfileQualificationRecord{
		CompanyID: companyID, EventID: "domain-profile-qualification-event", ProfileID: profileID,
		ProfileRevision: request.ProfileRevision, Decision: request.Decision,
		EvidenceInputID: request.EvidenceInputID, EvidenceInputRevision: 2, EvidenceSHA256: strings.Repeat("a", 64),
		Rationale: request.Rationale, Actor: "local-owner", RequestID: request.RequestID,
	}, nil
}

func (service *fakeDomainEvidenceService) ListDomainEvidence(_ context.Context, companyID string) (kernel.DomainEvidenceLedger, error) {
	service.listedFor = companyID
	return service.ledger, nil
}

func TestDomainEvidenceReviewRoutePersistsAnImmutableReferenceDecision(t *testing.T) {
	service := &fakeDomainEvidenceService{}
	handler := NewHandler(nil, service)
	request := control.DomainEvidenceReviewCommandRequest{
		DomainEvidenceReview: domainworkflow.DomainEvidenceReview{
			Outcome: domainworkflow.DomainEvidenceReviewNeedsMore, ReviewerEmployeeID: "emp-review", Rationale: "attach the recovery evidence log",
			PreviewedEvidence: []domainworkflow.DomainEvidencePreviewAttestation{{Area: domainworkflow.DomainEvidenceQuality, RelativePath: "quality.md", SourceDigest: strings.Repeat("a", 64), ContentDigest: strings.Repeat("b", 64), MediaType: "text/markdown"}},
		},
		RequestID: "domain-review-request-1",
	}
	body, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	reviewRequest := httptest.NewRequest(http.MethodPost, "/api/workbench/companies/company-1/domain-evidence/domain-evidence-record-1/review", bytes.NewReader(body))
	reviewRequest.Header.Set("Content-Type", "application/json")
	handler.ServeHTTP(response, reviewRequest)
	if response.Code != http.StatusAccepted || service.reviewedFor != "company-1" || service.reviewedID != "domain-evidence-record-1" || service.review.RequestID != request.RequestID {
		t.Fatalf("domain evidence review response=%d scope=%q record=%q request=%+v body=%s", response.Code, service.reviewedFor, service.reviewedID, service.review, response.Body.String())
	}
	var review kernel.DomainEvidenceReviewRecord
	if err = json.Unmarshal(response.Body.Bytes(), &review); err != nil || review.Outcome != request.Outcome || review.ReviewerEmployeeID != "emp-review" {
		t.Fatalf("review response=(%+v,%v)", review, err)
	}
}

func TestDomainEvidenceSubstantiveAssessmentRoutePersistsScopedDecision(t *testing.T) {
	service := &fakeDomainEvidenceService{}
	handler := NewHandler(nil, service)
	request := control.DomainEvidenceSubstantiveAssessmentCommandRequest{
		DomainEvidenceSubstantiveReview: domainworkflow.DomainEvidenceSubstantiveReview{
			ReviewerEmployeeID: "emp-review",
			AreaAssessments:    []domainworkflow.DomainEvidenceAreaAssessment{{Area: domainworkflow.DomainEvidenceQuality, Outcome: domainworkflow.DomainAssessmentInsufficient, Rationale: "attach the missing sample"}},
			PreviewedEvidence:  []domainworkflow.DomainEvidencePreviewAttestation{{Area: domainworkflow.DomainEvidenceQuality, RelativePath: "quality.md", SourceDigest: strings.Repeat("a", 64), ContentDigest: strings.Repeat("b", 64), MediaType: "text/markdown"}},
		},
		EvidenceDigest: strings.Repeat("a", 64),
		RequestID:      "substantive-assessment-request-1",
	}
	body, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	post := httptest.NewRequest(http.MethodPost, "/api/workbench/companies/company-1/domain-evidence/domain-evidence-record-1/assessment", bytes.NewReader(body))
	post.Header.Set("Content-Type", "application/json")
	handler.ServeHTTP(response, post)
	if response.Code != http.StatusAccepted || service.assessedFor != "company-1" || service.assessedID != "domain-evidence-record-1" || service.assessment.RequestID != request.RequestID {
		t.Fatalf("substantive assessment status=%d scope=%q record=%q request=%+v body=%s", response.Code, service.assessedFor, service.assessedID, service.assessment, response.Body.String())
	}
	parsed, ok := parsePath("/api/workbench/companies/company-1/domain-evidence/domain-evidence-record-1/assessment")
	if !ok || parsed.endpoint != "domain.evidence.assessment" || parsed.resourceID != "domain-evidence-record-1" {
		t.Fatalf("substantive assessment path=%+v ok=%v", parsed, ok)
	}
}

func TestDomainEvidencePreviewRouteReturnsVerifiedInlineBytes(t *testing.T) {
	service := &fakeDomainEvidenceService{}
	handler := NewHandler(nil, service)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/workbench/companies/company-1/domain-evidence/domain-evidence-record-1/evidence/quality/preview?path=reports%2Fquality.md", nil))
	if response.Code != http.StatusOK || service.previewedFor != "company-1" || service.previewedID != "domain-evidence-record-1" || service.previewedArea != domainworkflow.DomainEvidenceQuality || service.previewedPath != "reports/quality.md" {
		t.Fatalf("preview response=%d scope=%q record=%q area=%q path=%q body=%s", response.Code, service.previewedFor, service.previewedID, service.previewedArea, service.previewedPath, response.Body.String())
	}
	if response.Header().Get("Content-Type") != "text/plain" || response.Header().Get("X-Content-SHA256") != strings.Repeat("b", 64) || response.Header().Get("X-Source-SHA256") != strings.Repeat("a", 64) || response.Header().Get("X-Polis-Input-ID") != "input-1" || response.Header().Get("X-Polis-Input-Revision") != "1" || response.Header().Get("Cache-Control") != "no-store" || response.Header().Get("X-Content-Type-Options") != "nosniff" || response.Body.String() != "reviewable evidence text" {
		t.Fatalf("preview response headers/body are not bound to content: headers=%v body=%q", response.Header(), response.Body.String())
	}
}

func TestDomainEvidencePreviewEntriesRouteReturnsScopedManifest(t *testing.T) {
	service := &fakeDomainEvidenceService{}
	handler := NewHandler(nil, service)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/workbench/companies/company-1/domain-evidence/domain-evidence-record-1/evidence/quality/preview/entries", nil))
	if response.Code != http.StatusOK || response.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("preview entries response=%d headers=%v body=%s", response.Code, response.Header(), response.Body.String())
	}
	var manifest kernel.DomainEvidenceArtifactPreviewManifest
	if err := json.Unmarshal(response.Body.Bytes(), &manifest); err != nil || manifest.CompanyID != "company-1" || manifest.RecordID != "domain-evidence-record-1" || manifest.Area != domainworkflow.DomainEvidenceQuality || len(manifest.Entries) != 1 || !manifest.Entries[0].Previewable {
		t.Fatalf("preview entries manifest=(%+v,%v)", manifest, err)
	}
}

func (service *fakeDomainEvidenceService) RecordDomainEvidence(_ context.Context, companyID string, request control.DomainEvidenceCommandRequest) (kernel.DomainEvidenceRecord, error) {
	service.submittedFor = companyID
	service.submitted = request
	return kernel.DomainEvidenceRecord{
		CompanyID: companyID, RecordID: "domain-evidence-record-1", ProfileID: request.ProfileID,
		ProfileRevision: request.ProfileRevision, ReadinessStatus: domainworkflow.DomainEvidenceIncomplete,
		QualificationStatus: "not_run", ExecutionEnabled: false, Submission: request.DomainEvidenceSubmission,
		ReasonCodes: []string{"domain_evidence_area_missing"}, RequestID: request.RequestID,
	}, nil
}

func TestDomainEvidenceWorkbenchRoutesAreCompanyScopedAndNeverQualify(t *testing.T) {
	service := &fakeDomainEvidenceService{
		ledger: kernel.DomainEvidenceLedger{CompanyID: "company-1", Profiles: domainworkflow.ReferenceWorkflows(), Submissions: []kernel.DomainEvidenceRecord{}, Qualifications: []kernel.DomainProfileQualificationRecord{}, ResearchSimulationRuns: []kernel.ResearchSimulationRunRecord{}, ContentSourceEvents: []kernel.DomainContentSourceEventRecord{}, ContentDrafts: []kernel.DomainContentDraftRecord{}, ContentReviews: []kernel.DomainContentReviewRecord{}, ContentPublications: []kernel.DomainContentPublicationRecord{}, ContentCorrections: []kernel.DomainContentCorrectionRecord{}, ContentFeedback: []kernel.DomainContentFeedbackRecord{}},
	}
	handler := NewHandler(nil, service)
	get := httptest.NewRecorder()
	handler.ServeHTTP(get, httptest.NewRequest(http.MethodGet, "/api/workbench/companies/company-1/domain-workflows", nil))
	if get.Code != http.StatusOK || service.listedFor != "company-1" {
		t.Fatalf("domain workflow GET status=%d scope=%q body=%s", get.Code, service.listedFor, get.Body.String())
	}
	var ledger kernel.DomainEvidenceLedger
	if err := json.Unmarshal(get.Body.Bytes(), &ledger); err != nil || len(ledger.Profiles) != 2 {
		t.Fatalf("domain workflow ledger=(%+v,%v)", ledger, err)
	}
	request := control.DomainEvidenceCommandRequest{
		DomainEvidenceSubmission: domainworkflow.DomainEvidenceSubmission{ProfileID: "content-operations-reference", ProfileRevision: "content-operations@1"},
		RequestID:                "domain-evidence-request-1",
	}
	body, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	post := httptest.NewRecorder()
	postRequest := httptest.NewRequest(http.MethodPost, "/api/workbench/companies/company-1/domain-evidence", bytes.NewReader(body))
	postRequest.Header.Set("Content-Type", "application/json")
	handler.ServeHTTP(post, postRequest)
	if post.Code != http.StatusAccepted || service.submittedFor != "company-1" || service.submitted.RequestID != request.RequestID || service.submitted.ProfileID != request.ProfileID {
		t.Fatalf("domain evidence POST status=%d scope=%q request=%+v body=%s", post.Code, service.submittedFor, service.submitted, post.Body.String())
	}
	var record kernel.DomainEvidenceRecord
	if err = json.Unmarshal(post.Body.Bytes(), &record); err != nil || record.QualificationStatus != "not_run" || record.ExecutionEnabled {
		t.Fatalf("domain evidence response=(%+v,%v)", record, err)
	}
}

func TestResearchSimulationCommandRoutesAsACompanyScopedSimulation(t *testing.T) {
	service := &fakeDomainEvidenceService{}
	handler := NewHandler(nil, service)
	request := control.ResearchSimulationCommandRequest{
		DatasetInputID: "dataset-input", DatasetInputRevision: 2, MethodInputID: "method-input", MethodInputRevision: 1,
		Seed: "42", ControlDefinition: "specified control cohort", RiskBudgetUnits: 100, RequestID: "research-simulation-1",
	}
	body, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	httpRequest := httptest.NewRequest(http.MethodPost, "/api/workbench/companies/company-1/domain-workflows/research-simulations", bytes.NewReader(body))
	httpRequest.Header.Set("Content-Type", "application/json")
	handler.ServeHTTP(response, httpRequest)
	if response.Code != http.StatusAccepted || service.simulationFor != "company-1" || service.simulationReq.DatasetInputID != request.DatasetInputID || service.simulationReq.RequestID != request.RequestID {
		t.Fatalf("research simulation POST status=%d company=%q request=%+v body=%s", response.Code, service.simulationFor, service.simulationReq, response.Body.String())
	}
}

func TestContentOperationCommandsAreCompanyScoped(t *testing.T) {
	service := &fakeDomainEvidenceService{}
	handler := NewHandler(nil, service)
	requests := []struct {
		path string
		body any
	}{
		{"content-sources", control.DomainContentSourceAuthorizationCommandRequest{SourceInputID: "source-1", SourceInputRevision: 2, SourceSHA256: strings.Repeat("a", 64), State: kernel.DomainContentSourceAuthorized, Rationale: "source reviewed", RequestID: "content-source-request-1"}},
		{"content-drafts", control.DomainContentDraftCommandRequest{DraftInputID: "draft-1", DraftInputRevision: 3, WriterEmployeeID: "emp-backend", CriticalClaims: []string{"claim-1"}, ConstraintsPassed: true, RequestID: "content-draft-request-1"}},
		{"content-reviews", struct {
			DraftInputID  string `json:"draftInputId"`
			DraftRevision int64  `json:"draftRevision,string"`
			control.DomainContentReviewCommandRequest
		}{DraftInputID: "draft-1", DraftRevision: 3, DomainContentReviewCommandRequest: control.DomainContentReviewCommandRequest{
			Review: domainworkflow.ContentReview{DraftRevision: 3, CheckerEmployeeID: "emp-review"}, RequestID: "content-review-request-1",
		}}},
		{"content-publications", control.DomainContentPublicationCommandRequest{ReviewID: "content-review-1", RequestID: "content-publication-request-1"}},
		{"content-corrections", control.DomainContentCorrectionCommandRequest{PublicationID: "content-publication-1", CorrectionDraftInputID: "draft-1", CorrectionDraftRevision: "4", Rationale: "correct the tracked statement", RequestID: "content-correction-request-1"}},
		{"content-feedback", control.DomainContentFeedbackCommandRequest{PublicationID: "content-publication-1", Category: kernel.DomainContentFeedbackCorrectionRequested, Note: "Please review the withdrawn source.", RequestID: "content-feedback-request-1"}},
	}
	for _, item := range requests {
		body, err := json.Marshal(item.body)
		if err != nil {
			t.Fatal(err)
		}
		response := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/api/workbench/companies/company-1/domain-workflows/"+item.path, bytes.NewReader(body))
		request.Header.Set("Content-Type", "application/json")
		handler.ServeHTTP(response, request)
		if response.Code != http.StatusAccepted {
			t.Fatalf("content operation %s response=%d body=%s", item.path, response.Code, response.Body.String())
		}
	}
	if service.contentSourceFor != "company-1" || service.contentDraftFor != "company-1" || service.contentReviewFor != "company-1" ||
		service.contentPublicationFor != "company-1" || service.contentCorrectionFor != "company-1" || service.contentFeedbackFor != "company-1" ||
		service.contentReviewInputID != "draft-1" || service.contentReviewRevision != 3 {
		t.Fatalf("content operation company scopes source=%q draft=%q review=%q publication=%q correction=%q feedback=%q input=%q revision=%d",
			service.contentSourceFor, service.contentDraftFor, service.contentReviewFor, service.contentPublicationFor, service.contentCorrectionFor, service.contentFeedbackFor, service.contentReviewInputID, service.contentReviewRevision)
	}
}

func TestDomainEvidenceProfileQualificationRouteBindsOwnerDecisionToProfileEvidence(t *testing.T) {
	service := &fakeDomainEvidenceService{}
	handler := NewHandler(nil, service)
	request := control.DomainProfileQualificationCommandRequest{
		Decision: kernel.DomainProfileQualificationQualified, ProfileRevision: "content-operations@1",
		EvidenceInputID: "domain-qualification-report", EvidenceInputRevision: "2",
		Rationale: "independently reviewed domain quality, intervention, recovery, cost, and organization evidence",
		RequestID: "domain-profile-qualification-1",
	}
	body, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	httpRequest := httptest.NewRequest(http.MethodPost, "/api/workbench/companies/company-1/domain-workflows/content-operations-reference/qualification", bytes.NewReader(body))
	httpRequest.Header.Set("Content-Type", "application/json")
	handler.ServeHTTP(response, httpRequest)
	if response.Code != http.StatusAccepted || service.qualifiedFor != "company-1" || service.qualifiedID != "content-operations-reference" || service.qualification.EvidenceInputID != request.EvidenceInputID || service.qualification.RequestID != request.RequestID {
		t.Fatalf("profile qualification response=%d company=%q profile=%q request=%+v body=%s", response.Code, service.qualifiedFor, service.qualifiedID, service.qualification, response.Body.String())
	}
	var decision kernel.DomainProfileQualificationRecord
	if err = json.Unmarshal(response.Body.Bytes(), &decision); err != nil || decision.Decision != kernel.DomainProfileQualificationQualified || decision.EvidenceInputRevision != 2 {
		t.Fatalf("profile qualification response record=(%+v,%v)", decision, err)
	}
}

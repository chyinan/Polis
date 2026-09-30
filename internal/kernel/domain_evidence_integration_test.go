// pattern: Imperative Shell
package kernel

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"polis/internal/core"
	"polis/internal/domainworkflow"
	"polis/internal/intake"
)

func TestDomainEvidenceSubmissionIsScopedAppendOnlyAndNeverQualifies(t *testing.T) {
	dsn := os.Getenv("POLIS_TEST_DSN")
	if dsn == "" {
		t.Skip("dedicated PostgreSQL required")
	}
	ctx := context.Background()
	k, err := Open(ctx, dsn, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer k.Close()
	companyID := fmt.Sprintf("domain-evidence-%d", time.Now().UnixNano())
	if _, err = k.TXCreateCompany(ctx, companyID); err != nil {
		t.Fatal(err)
	}
	mission, err := k.TXCreateMissionGoal(ctx, k.LocalScope(companyID), "R3 evidence fixture", "store company evidence artifacts", "domain-evidence-mission")
	if err != nil {
		t.Fatal(err)
	}
	profile := domainworkflow.ReferenceWorkflows()[0]
	submission := domainEvidenceFixture(profile)
	previewedEvidence := make([]domainworkflow.DomainEvidencePreviewAttestation, 0, len(submission.Evidence))
	for index := range submission.Evidence {
		requestID := fmt.Sprintf("domain-evidence-input-%d", index)
		fileName := fmt.Sprintf("evidence-%d.md", index)
		content := []byte(fmt.Sprintf("evidence area %s data", submission.Evidence[index].Area))
		prepared, stored, prepareErr := intake.PrepareMissionInput(fileName, "text/markdown", content)
		if prepareErr != nil {
			t.Fatal(prepareErr)
		}
		input, addErr := k.TXAddMissionInput(ctx, k.LocalScope(companyID), mission.ID, "", requestID, prepared, stored)
		if addErr != nil {
			t.Fatal(addErr)
		}
		submission.Evidence[index].Artifact = domainworkflow.DomainEvidenceArtifact{ID: input.InputID, Revision: input.Revision, SHA256: input.ContentDigest}
		previewedEvidence = append(previewedEvidence, domainworkflow.DomainEvidencePreviewAttestation{
			Area: submission.Evidence[index].Area, RelativePath: fileName, SourceDigest: input.ContentDigest, ContentDigest: input.ContentDigest, MediaType: "text/markdown",
		})
	}
	record, err := k.TXRecordDomainEvidence(ctx, companyID, submission, "domain-evidence-ready")
	if err != nil {
		t.Fatal(err)
	}
	if record.ReadinessStatus != domainworkflow.DomainEvidenceReadyForReview || record.QualificationStatus != "not_run" || record.ExecutionEnabled {
		t.Fatalf("ready record changed qualification or execution: %+v", record)
	}
	reviewRequest := DomainEvidenceReviewRequest{
		DomainEvidenceReview: domainworkflow.DomainEvidenceReview{
			Outcome: domainworkflow.DomainEvidenceReviewAccepted, ReviewerEmployeeID: "emp-review", Rationale: "references inspected; qualification remains separate",
			PreviewedEvidence: previewedEvidence,
		},
		RequestID: "domain-evidence-review-ready",
	}
	review, err := k.TXRecordDomainEvidenceReview(ctx, companyID, record.RecordID, reviewRequest)
	if err != nil || review.Outcome != domainworkflow.DomainEvidenceReviewAccepted || review.ReviewerEmployeeID != "emp-review" || review.ReviewContractRevision != 1 || !sameDomainEvidencePreviewAttestations(review.PreviewedEvidence, previewedEvidence) {
		t.Fatalf("independent reference review=(%+v,%v)", review, err)
	}
	replayedReview, err := k.TXRecordDomainEvidenceReview(ctx, companyID, record.RecordID, reviewRequest)
	if err != nil || replayedReview.ReviewID != review.ReviewID {
		t.Fatalf("idempotent review replay=(%+v,%v), want same review", replayedReview, err)
	}
	substantiveRequest := DomainEvidenceSubstantiveAssessmentRequest{
		DomainEvidenceSubstantiveReview: domainworkflow.DomainEvidenceSubstantiveReview{
			ReviewerEmployeeID: "emp-review", PreviewedEvidence: previewedEvidence,
		},
		EvidenceDigest: record.EvidenceDigest,
		RequestID:      "domain-evidence-substantive-accepted",
	}
	for _, item := range submission.Evidence {
		substantiveRequest.AreaAssessments = append(substantiveRequest.AreaAssessments, domainworkflow.DomainEvidenceAreaAssessment{
			Area: item.Area, Outcome: domainworkflow.DomainAssessmentAccepted, Rationale: "inspected and meets the scoped evidence criterion",
		})
	}
	substantive, err := k.TXRecordDomainEvidenceSubstantiveAssessment(ctx, companyID, record.RecordID, substantiveRequest)
	if err != nil || substantive.Outcome != domainworkflow.DomainEvidenceSubstantiveAccepted || len(substantive.AreaAssessments) != len(profile.RequiredEvidence) || substantive.EvidenceDigest != record.EvidenceDigest {
		t.Fatalf("substantive area review=(%+v,%v)", substantive, err)
	}
	replayedSubstantive, err := k.TXRecordDomainEvidenceSubstantiveAssessment(ctx, companyID, record.RecordID, substantiveRequest)
	if err != nil || replayedSubstantive.AssessmentID != substantive.AssessmentID {
		t.Fatalf("idempotent substantive replay=(%+v,%v), want same assessment", replayedSubstantive, err)
	}
	researchProfile := domainworkflow.ReferenceWorkflows()[1]
	researchSubmission := domainEvidenceFixture(researchProfile)
	researchPreviews := make([]domainworkflow.DomainEvidencePreviewAttestation, 0, len(researchSubmission.Evidence))
	for index := range researchSubmission.Evidence {
		fileName := fmt.Sprintf("research-evidence-%d.md", index)
		content := []byte(fmt.Sprintf("research evidence area %s data", researchSubmission.Evidence[index].Area))
		prepared, stored, prepareErr := intake.PrepareMissionInput(fileName, "text/markdown", content)
		if prepareErr != nil {
			t.Fatal(prepareErr)
		}
		input, addErr := k.TXAddMissionInput(ctx, k.LocalScope(companyID), mission.ID, "", fmt.Sprintf("research-evidence-input-%d", index), prepared, stored)
		if addErr != nil {
			t.Fatal(addErr)
		}
		researchSubmission.Evidence[index].Artifact = domainworkflow.DomainEvidenceArtifact{ID: input.InputID, Revision: input.Revision, SHA256: input.ContentDigest}
		researchPreviews = append(researchPreviews, domainworkflow.DomainEvidencePreviewAttestation{Area: researchSubmission.Evidence[index].Area, RelativePath: fileName, SourceDigest: input.ContentDigest, ContentDigest: input.ContentDigest, MediaType: "text/markdown"})
	}
	researchRecord, err := k.TXRecordDomainEvidence(ctx, companyID, researchSubmission, "research-domain-evidence-ready")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = k.TXRecordDomainEvidenceReview(ctx, companyID, researchRecord.RecordID, DomainEvidenceReviewRequest{
		DomainEvidenceReview: domainworkflow.DomainEvidenceReview{Outcome: domainworkflow.DomainEvidenceReviewAccepted, ReviewerEmployeeID: "emp-review", Rationale: "research evidence references inspected", PreviewedEvidence: researchPreviews},
		RequestID:            "research-domain-evidence-reference-review",
	}); err != nil {
		t.Fatal(err)
	}
	researchAssessment := DomainEvidenceSubstantiveAssessmentRequest{
		DomainEvidenceSubstantiveReview: domainworkflow.DomainEvidenceSubstantiveReview{ReviewerEmployeeID: "emp-review", PreviewedEvidence: researchPreviews},
		EvidenceDigest:                  researchRecord.EvidenceDigest, RequestID: "research-domain-evidence-substantive-review",
	}
	for _, item := range researchSubmission.Evidence {
		researchAssessment.AreaAssessments = append(researchAssessment.AreaAssessments, domainworkflow.DomainEvidenceAreaAssessment{Area: item.Area, Outcome: domainworkflow.DomainAssessmentAccepted, Rationale: "simulation evidence supports the scoped criterion"})
	}
	researchAssessmentRecord, err := k.TXRecordDomainEvidenceSubstantiveAssessment(ctx, companyID, researchRecord.RecordID, researchAssessment)
	if err != nil || researchAssessmentRecord.Outcome != domainworkflow.DomainEvidenceSubstantiveAccepted {
		t.Fatalf("research simulation evidence assessment=(%+v,%v)", researchAssessmentRecord, err)
	}
	withoutArea := substantiveRequest
	withoutArea.RequestID = "domain-evidence-substantive-missing-area"
	withoutArea.AreaAssessments = substantiveRequest.AreaAssessments[1:]
	if _, err = k.TXRecordDomainEvidenceSubstantiveAssessment(ctx, companyID, record.RecordID, withoutArea); !errors.Is(err, core.Conflict) {
		t.Fatalf("second substantive assessment error=%v, want immutable one-assessment conflict", err)
	}
	withoutPreview := reviewRequest
	withoutPreview.RequestID = "domain-evidence-review-without-preview"
	withoutPreview.PreviewedEvidence = nil
	if _, err = k.TXRecordDomainEvidenceReview(ctx, companyID, record.RecordID, withoutPreview); !errors.Is(err, core.Malformed) {
		t.Fatalf("review without a preview attestation error=%v, want rejection", err)
	}
	wrongPreviewDigest := reviewRequest
	wrongPreviewDigest.RequestID = "domain-evidence-review-wrong-preview-digest"
	wrongPreviewDigest.PreviewedEvidence = append([]domainworkflow.DomainEvidencePreviewAttestation(nil), previewedEvidence...)
	wrongPreviewDigest.PreviewedEvidence[0].ContentDigest = strings.Repeat("c", 64)
	if _, err = k.TXRecordDomainEvidenceReview(ctx, companyID, record.RecordID, wrongPreviewDigest); !errors.Is(err, core.Integrity) {
		t.Fatalf("review with a forged preview digest error=%v, want integrity rejection", err)
	}
	if _, err = k.TXRecordDomainEvidenceReview(ctx, companyID, record.RecordID, DomainEvidenceReviewRequest{
		DomainEvidenceReview: domainworkflow.DomainEvidenceReview{Outcome: domainworkflow.DomainEvidenceReviewAccepted, ReviewerEmployeeID: "emp-planning", Rationale: "review role is required", PreviewedEvidence: previewedEvidence},
		RequestID:            "domain-evidence-review-wrong-role",
	}); !errors.Is(err, core.Denied) {
		t.Fatalf("non-review role decision error=%v, want role rejection", err)
	}
	if _, err = k.TXRecordDomainEvidenceReview(ctx, companyID, record.RecordID, DomainEvidenceReviewRequest{
		DomainEvidenceReview: domainworkflow.DomainEvidenceReview{Outcome: domainworkflow.DomainEvidenceReviewRejected, ReviewerEmployeeID: "emp-review", Rationale: "second decision", PreviewedEvidence: previewedEvidence},
		RequestID:            "domain-evidence-review-second",
	}); !errors.Is(err, core.Conflict) {
		t.Fatalf("second review decision error=%v, want immutable one-decision conflict", err)
	}
	if _, err = k.TXRecordDomainEvidenceReview(ctx, companyID, record.RecordID, DomainEvidenceReviewRequest{
		DomainEvidenceReview: domainworkflow.DomainEvidenceReview{Outcome: domainworkflow.DomainEvidenceReviewAccepted, ReviewerEmployeeID: submission.Evidence[0].AssessedByEmployeeID, Rationale: "self review", PreviewedEvidence: previewedEvidence},
		RequestID:            "domain-evidence-review-self",
	}); !errors.Is(err, core.Malformed) {
		t.Fatalf("self-reviewed evidence error=%v, want independent reviewer rejection", err)
	}
	replayed, err := k.TXRecordDomainEvidence(ctx, companyID, submission, "domain-evidence-ready")
	if err != nil || replayed.RecordID != record.RecordID {
		t.Fatalf("idempotent replay=(%+v,%v), want same record", replayed, err)
	}
	incomplete := domainworkflow.DomainEvidenceSubmission{ProfileID: profile.ID, ProfileRevision: profile.Revision}
	incompleteRecord, err := k.TXRecordDomainEvidence(ctx, companyID, incomplete, "domain-evidence-incomplete")
	if err != nil || incompleteRecord.ReadinessStatus != domainworkflow.DomainEvidenceIncomplete {
		t.Fatalf("incomplete submission=(%+v,%v)", incompleteRecord, err)
	}
	if incompleteRecord.Submission.Evidence == nil {
		t.Fatal("incomplete evidence was serialized as null instead of an empty array")
	}
	if _, err = k.pool.Exec(ctx, `INSERT INTO domain_workflow_evidence_reviews(company_id,review_id,record_id,outcome,reviewer_employee_id,rationale,request_id)
VALUES($1,'domain-review-incomplete', $2,'evidence_references_accepted','emp-review','must stay blocked','domain-review-incomplete')`, companyID, incompleteRecord.RecordID); err == nil {
		t.Fatal("database trigger accepted a review for an incomplete submission")
	}
	if _, err = k.pool.Exec(ctx, `INSERT INTO domain_workflow_evidence_reviews(company_id,review_id,record_id,outcome,reviewer_employee_id,rationale,request_id)
VALUES($1,'domain-review-self-assessed', $2,'evidence_references_accepted','emp-backend','must stay blocked','domain-review-self-assessed')`, companyID, record.RecordID); err == nil {
		t.Fatal("database trigger accepted a reviewer who assessed the evidence")
	}
	if _, err = k.pool.Exec(ctx, `INSERT INTO domain_workflow_evidence_reviews(company_id,review_id,record_id,outcome,reviewer_employee_id,rationale,request_id)
VALUES($1,'domain-review-wrong-role', $2,'evidence_references_accepted','emp-planning','must stay blocked','domain-review-wrong-role')`, companyID, record.RecordID); err == nil {
		t.Fatal("database trigger accepted a non-review role")
	}
	if _, err = k.pool.Exec(ctx, `INSERT INTO domain_workflow_evidence_reviews(company_id,review_id,record_id,outcome,reviewer_employee_id,rationale,request_id)
VALUES($1,'domain-review-no-previews', $2,'evidence_references_accepted','emp-review','must stay blocked','domain-review-no-previews')`, companyID, record.RecordID); err == nil {
		t.Fatal("database trigger accepted a decision without preview attestations")
	}
	emptyArrayReplay := incomplete
	emptyArrayReplay.Evidence = []domainworkflow.DomainEvidence{}
	replayedEmpty, err := k.TXRecordDomainEvidence(ctx, companyID, emptyArrayReplay, "domain-evidence-incomplete")
	if err != nil || replayedEmpty.RecordID != incompleteRecord.RecordID {
		t.Fatalf("nil/empty evidence replay=(%+v,%v), want same normalized record", replayedEmpty, err)
	}
	ledger, err := k.ListDomainEvidence(ctx, companyID)
	if err != nil || len(ledger.Profiles) != 2 || len(ledger.Submissions) != 3 {
		t.Fatalf("company evidence ledger=(%+v,%v), want two profiles and three submissions", ledger, err)
	}
	var persistedReady *DomainEvidenceRecord
	for index := range ledger.Submissions {
		if ledger.Submissions[index].RecordID == record.RecordID {
			persistedReady = &ledger.Submissions[index]
			break
		}
	}
	if persistedReady == nil || persistedReady.QualificationStatus != "not_run" || persistedReady.ExecutionEnabled || persistedReady.Review == nil || persistedReady.Review.ReviewID != review.ReviewID || persistedReady.Review.ReviewContractRevision != 1 || !sameDomainEvidencePreviewAttestations(persistedReady.Review.PreviewedEvidence, previewedEvidence) || persistedReady.Assessment == nil || persistedReady.Assessment.AssessmentID != substantive.AssessmentID || persistedReady.Assessment.Outcome != domainworkflow.DomainEvidenceSubstantiveAccepted {
		t.Fatalf("persisted readiness was projected as qualified or lost a reference/substantive decision: %+v", persistedReady)
	}
	otherCompanyID := companyID + "-other"
	if _, err = k.TXCreateCompany(ctx, otherCompanyID); err != nil {
		t.Fatal(err)
	}
	otherLedger, err := k.ListDomainEvidence(ctx, otherCompanyID)
	if err != nil || len(otherLedger.Submissions) != 0 {
		t.Fatalf("cross-company evidence read=(%+v,%v), want no submissions", otherLedger, err)
	}
	if _, err = k.pool.Exec(ctx, `UPDATE domain_workflow_evidence_submissions SET readiness_status='incomplete' WHERE company_id=$1 AND record_id=$2`, companyID, record.RecordID); err == nil {
		t.Fatal("append-only evidence record accepted an update")
	}
	if _, err = k.pool.Exec(ctx, `UPDATE domain_workflow_evidence_items SET input_sha256=$3 WHERE company_id=$1 AND record_id=$2`, companyID, record.RecordID, strings.Repeat("c", 64)); err == nil {
		t.Fatal("append-only evidence item accepted an update")
	}
	if _, err = k.pool.Exec(ctx, `UPDATE domain_workflow_evidence_reviews SET rationale='rewritten' WHERE company_id=$1 AND review_id=$2`, companyID, review.ReviewID); err == nil {
		t.Fatal("append-only review accepted an update")
	}
	if _, err = k.pool.Exec(ctx, `UPDATE domain_workflow_substantive_assessments SET outcome='evidence_rejected' WHERE company_id=$1 AND assessment_id=$2`, companyID, substantive.AssessmentID); err == nil {
		t.Fatal("append-only substantive assessment accepted an update")
	}
	if _, err = k.pool.Exec(ctx, `INSERT INTO domain_workflow_substantive_assessments(company_id,assessment_id,record_id,evidence_digest,outcome,reviewer_employee_id,area_assessments,previewed_evidence,request_id)
VALUES($1,'substantive-incomplete',$2,$3,'more_evidence_required','emp-review','[]'::jsonb,'[]'::jsonb,'substantive-incomplete')`, companyID, incompleteRecord.RecordID, incompleteRecord.EvidenceDigest); err == nil {
		t.Fatal("database trigger accepted a substantive decision without accepted reference review and previews")
	}
	if _, err = k.pool.Exec(ctx, `INSERT INTO domain_workflow_evidence_submissions(company_id,record_id,profile_id,profile_revision,readiness_status,qualification_status,execution_enabled,evidence_digest,evidence,reason_codes,request_id)
VALUES($1,'malformed-domain-record',$2,$3,'incomplete','not_run',false,$4,$5,'[]'::jsonb,'malformed-domain-request')`, companyID, profile.ID, profile.Revision, strings.Repeat("d", 64), []byte(`{"profileRevision":"content-operations@1","evidence":null}`)); err == nil {
		t.Fatal("database accepted evidence JSON with no profile ID and a null evidence array")
	}
	selfAssessed := submission
	selfAssessed.Evidence = append([]domainworkflow.DomainEvidence(nil), submission.Evidence...)
	selfAssessed.Evidence[0].AssessedByEmployeeID = "emp-review"
	selfAssessedRecord, err := k.TXRecordDomainEvidence(ctx, companyID, selfAssessed, "domain-evidence-self-assessed-source")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = k.pool.Exec(ctx, `INSERT INTO domain_workflow_evidence_reviews(company_id,review_id,record_id,outcome,reviewer_employee_id,rationale,previewed_evidence,review_contract_revision,request_id)
VALUES($1,'domain-review-self-assessor-role', $2,'evidence_references_accepted','emp-review','must stay blocked','[]'::jsonb,1,'domain-review-self-assessor-role')`, companyID, selfAssessedRecord.RecordID); err == nil {
		t.Fatal("database trigger accepted a reviewer who also assessed evidence")
	}
	metadataRecord, err := k.TXRecordDomainEvidence(ctx, companyID, submission, "domain-evidence-metadata-review-trigger")
	if err != nil {
		t.Fatal(err)
	}
	for index, relativePath := range []string{"pdf/extraction.json", "nested/.polis-git-source.json"} {
		metadataAttestations := append([]domainworkflow.DomainEvidencePreviewAttestation(nil), previewedEvidence...)
		metadataAttestations[0].RelativePath = relativePath
		metadataJSON, marshalErr := json.Marshal(metadataAttestations)
		if marshalErr != nil {
			t.Fatal(marshalErr)
		}
		reviewID := fmt.Sprintf("domain-review-metadata-path-%d", index)
		_, insertErr := k.pool.Exec(ctx, `INSERT INTO domain_workflow_evidence_reviews(company_id,review_id,record_id,outcome,reviewer_employee_id,rationale,previewed_evidence,review_contract_revision,request_id)
VALUES($1,$2,$3,'evidence_references_accepted','emp-review','metadata must remain unavailable',$4,1,$2)`, companyID, reviewID, metadataRecord.RecordID, metadataJSON)
		if insertErr == nil {
			t.Fatalf("database trigger accepted reserved metadata path %q", relativePath)
		}
	}
	unboundArtifact := domainEvidenceFixture(profile)
	unboundArtifact.Evidence[0].Artifact.ID = "missing-evidence-input"
	if _, err = k.TXRecordDomainEvidence(ctx, companyID, unboundArtifact, "domain-evidence-unbound"); !errors.Is(err, core.OutOfScope) {
		t.Fatalf("unbound evidence artifact error=%v, want company-scoped input rejection", err)
	}
	unknown := submission
	unknown.ProfileRevision = "content-operations@unknown"
	if _, err = k.TXRecordDomainEvidence(ctx, companyID, unknown, "domain-evidence-unknown"); err == nil {
		t.Fatal("unknown profile revision was persisted")
	}
}

func domainEvidenceFixture(profile domainworkflow.WorkflowProfile) domainworkflow.DomainEvidenceSubmission {
	submission := domainworkflow.DomainEvidenceSubmission{ProfileID: profile.ID, ProfileRevision: profile.Revision}
	for index, area := range profile.RequiredEvidence {
		submission.Evidence = append(submission.Evidence, domainworkflow.DomainEvidence{
			Area: area,
			Artifact: domainworkflow.DomainEvidenceArtifact{
				ID: fmt.Sprintf("domain-evidence-artifact-%d", index), Revision: 1, SHA256: strings.Repeat("a", 64),
			},
			MethodSHA256: strings.Repeat("b", 64), AssessedByEmployeeID: "emp-backend",
		})
	}
	return submission
}

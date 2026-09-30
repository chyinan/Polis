package kernel

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"polis/internal/domainworkflow"
	"polis/internal/intake"
)

func TestDomainEvidenceProfileQualificationIsCompanyScopedAndNeverEnablesExecution(t *testing.T) {
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
	companyID := fmt.Sprintf("domain-profile-qualification-%d", time.Now().UnixNano())
	scope, err := k.TXCreateCompany(ctx, companyID)
	if err != nil {
		t.Fatal(err)
	}
	mission, err := k.TXCreateMissionGoal(ctx, scope, "R3 qualification sample", "bind profile qualification to reviewed domain evidence", "domain-profile-qualification-mission")
	if err != nil {
		t.Fatal(err)
	}
	profile := domainworkflow.ReferenceWorkflows()[0]
	submission := domainworkflow.DomainEvidenceSubmission{ProfileID: profile.ID, ProfileRevision: profile.Revision}
	for index, area := range profile.RequiredEvidence {
		content := []byte(fmt.Sprintf("sample evidence for area %s", area))
		prepared, stored, prepareErr := intake.PrepareMissionInput(fmt.Sprintf("qualification-%d.md", index), "text/markdown", content)
		if prepareErr != nil {
			t.Fatal(prepareErr)
		}
		input, addErr := k.TXAddMissionInput(ctx, scope, mission.ID, "", fmt.Sprintf("domain-profile-qualification-input-%d", index), prepared, stored)
		if addErr != nil {
			t.Fatal(addErr)
		}
		submission.Evidence = append(submission.Evidence, domainworkflow.DomainEvidence{
			Area: area, Artifact: domainworkflow.DomainEvidenceArtifact{ID: input.InputID, Revision: input.Revision, SHA256: input.ContentDigest},
			MethodSHA256: strings.Repeat("a", 64), AssessedByEmployeeID: "emp-backend",
		})
	}
	record, err := k.TXRecordDomainEvidence(ctx, companyID, submission, "domain-profile-qualification-submit")
	if err != nil {
		t.Fatal(err)
	}
	previewed := make([]domainworkflow.DomainEvidencePreviewAttestation, 0, len(submission.Evidence))
	for _, item := range submission.Evidence {
		manifest, manifestErr := k.ListDomainEvidenceArtifactPreviewEntries(ctx, companyID, record.RecordID, item.Area)
		if manifestErr != nil || len(manifest.Entries) != 1 || !manifest.Entries[0].Previewable {
			t.Fatalf("area %s preview manifest=%+v err=%v", item.Area, manifest, manifestErr)
		}
		preview, previewErr := k.ReadDomainEvidenceArtifactPreview(ctx, companyID, record.RecordID, item.Area, manifest.Entries[0].RelativePath)
		if previewErr != nil {
			t.Fatal(previewErr)
		}
		previewed = append(previewed, domainworkflow.DomainEvidencePreviewAttestation{
			Area: item.Area, RelativePath: preview.RelativePath, SourceDigest: preview.SourceDigest,
			ContentDigest: preview.ContentSHA256, MediaType: preview.MediaType,
		})
	}
	review, err := k.TXRecordDomainEvidenceReview(ctx, companyID, record.RecordID, DomainEvidenceReviewRequest{
		DomainEvidenceReview: domainworkflow.DomainEvidenceReview{
			Outcome: domainworkflow.DomainEvidenceReviewAccepted, ReviewerEmployeeID: "emp-review",
			Rationale: "all profile evidence references inspected", PreviewedEvidence: previewed,
		},
		RequestID: "domain-profile-qualification-review",
	})
	if err != nil || review.Outcome != domainworkflow.DomainEvidenceReviewAccepted {
		t.Fatalf("domain evidence reference review=%+v err=%v", review, err)
	}
	assessmentRequest := DomainEvidenceSubstantiveAssessmentRequest{
		DomainEvidenceSubstantiveReview: domainworkflow.DomainEvidenceSubstantiveReview{ReviewerEmployeeID: "emp-review", PreviewedEvidence: previewed},
		EvidenceDigest:                  record.EvidenceDigest, RequestID: "domain-profile-qualification-assessment",
	}
	for _, item := range submission.Evidence {
		assessmentRequest.AreaAssessments = append(assessmentRequest.AreaAssessments, domainworkflow.DomainEvidenceAreaAssessment{
			Area: item.Area, Outcome: domainworkflow.DomainAssessmentAccepted, Rationale: "sample satisfies the scoped review criterion",
		})
	}
	assessment, err := k.TXRecordDomainEvidenceSubstantiveAssessment(ctx, companyID, record.RecordID, assessmentRequest)
	if err != nil || assessment.Outcome != domainworkflow.DomainEvidenceSubstantiveAccepted {
		t.Fatalf("domain substantive assessment=%+v err=%v", assessment, err)
	}
	qualificationEvidence := domainworkflow.DomainProfileQualificationEvidence{
		SchemaVersion: domainworkflow.DomainProfileQualificationEvidenceSchema, ProfileID: profile.ID, ProfileRevision: profile.Revision,
		Areas: make([]domainworkflow.DomainProfileQualificationAreaEvidence, 0, len(profile.RequiredEvidence)),
	}
	for _, area := range profile.RequiredEvidence {
		qualificationEvidence.Areas = append(qualificationEvidence.Areas, domainworkflow.DomainProfileQualificationAreaEvidence{
			Area: area, Cases: []domainworkflow.DomainProfileQualificationCaseReference{{
				RecordID: record.RecordID, EvidenceDigest: record.EvidenceDigest, AssessmentID: assessment.AssessmentID,
			}},
		})
	}
	invalidQualificationEvidence := qualificationEvidence
	invalidQualificationEvidence.Areas = append([]domainworkflow.DomainProfileQualificationAreaEvidence(nil), qualificationEvidence.Areas...)
	invalidQualificationEvidence.Areas[0].Cases = append([]domainworkflow.DomainProfileQualificationCaseReference(nil), qualificationEvidence.Areas[0].Cases...)
	invalidQualificationEvidence.Areas[0].Cases[0].EvidenceDigest = strings.Repeat("f", 64)
	invalidBytes, err := json.Marshal(invalidQualificationEvidence)
	if err != nil {
		t.Fatal(err)
	}
	invalidUpload, storedInvalid, err := intake.PrepareMissionInput("domain-profile-qualification-invalid.json", "application/json", invalidBytes)
	if err != nil {
		t.Fatal(err)
	}
	invalidInput, err := k.TXAddMissionInput(ctx, scope, mission.ID, "", "domain-profile-qualification-invalid-report", invalidUpload, storedInvalid)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = k.TXRecordDomainProfileQualification(ctx, companyID, DomainProfileQualificationInput{
		ProfileID: profile.ID, ProfileRevision: profile.Revision, Decision: DomainProfileQualificationQualified,
		EvidenceInputID: invalidInput.InputID, EvidenceInputRevision: invalidInput.Revision,
		Rationale: "invalid references must be rejected", RequestID: "domain-profile-qualification-invalid",
	}); err == nil {
		t.Fatal("qualification accepted a report with a mismatched case digest")
	}
	if _, err = k.TXRecordDomainProfileQualification(ctx, companyID, DomainProfileQualificationInput{
		ProfileID: profile.ID, ProfileRevision: profile.Revision, Decision: DomainProfileQualificationRevoked,
		Rationale: "there is no active qualification yet", RequestID: "domain-profile-qualification-revoke-before-qualify",
	}); err == nil {
		t.Fatal("profile qualification revocation was accepted before qualification")
	}
	qualificationBytes, err := json.Marshal(qualificationEvidence)
	if err != nil {
		t.Fatal(err)
	}
	qualificationUpload, storedQualification, err := intake.PrepareMissionInput("domain-profile-qualification.json", "application/json", qualificationBytes)
	if err != nil {
		t.Fatal(err)
	}
	qualificationInput, err := k.TXAddMissionInput(ctx, scope, mission.ID, "", "domain-profile-qualification-report", qualificationUpload, storedQualification)
	if err != nil {
		t.Fatal(err)
	}
	decision, err := k.TXRecordDomainProfileQualification(ctx, companyID, DomainProfileQualificationInput{
		ProfileID: profile.ID, ProfileRevision: profile.Revision, Decision: DomainProfileQualificationQualified,
		EvidenceInputID: qualificationInput.InputID, EvidenceInputRevision: qualificationInput.Revision,
		Rationale: "the independently reviewed evidence set covers the profile requirements", RequestID: "domain-profile-qualification-decide",
	})
	if err != nil || decision.Decision != DomainProfileQualificationQualified || decision.EvidenceSHA256 != qualificationInput.ContentDigest {
		t.Fatalf("profile qualification decision=%+v err=%v", decision, err)
	}
	ledger, err := k.ListDomainEvidence(ctx, companyID)
	if err != nil {
		t.Fatal(err)
	}
	if len(ledger.Qualifications) != 1 || ledger.Qualifications[0].EventID != decision.EventID {
		t.Fatalf("qualification history=%+v", ledger.Qualifications)
	}
	profileStatus := domainworkflow.WorkflowProfile{}
	for _, candidate := range ledger.Profiles {
		if candidate.ID == profile.ID {
			profileStatus = candidate
		}
	}
	if profileStatus.QualificationStatus != string(DomainProfileQualificationQualified) || profileStatus.ExecutionEnabled {
		t.Fatalf("profile status=%+v; evidence qualification must not enable an unimplemented runtime", profileStatus)
	}
	if _, err = k.TXRecordDomainProfileQualification(ctx, companyID, DomainProfileQualificationInput{
		ProfileID: profile.ID, ProfileRevision: profile.Revision, Decision: DomainProfileQualificationRevoked,
		Rationale: "revoke the current company profile qualification", RequestID: "domain-profile-qualification-revoke",
	}); err != nil {
		t.Fatal(err)
	}
	ledger, err = k.ListDomainEvidence(ctx, companyID)
	if err != nil {
		t.Fatal(err)
	}
	for _, candidate := range ledger.Profiles {
		if candidate.ID == profile.ID && (candidate.QualificationStatus != string(DomainProfileQualificationRevoked) || candidate.ExecutionEnabled) {
			t.Fatalf("revoked profile status=%+v", candidate)
		}
	}
	otherCompany := companyID + "-other"
	if _, err = k.TXCreateCompany(ctx, otherCompany); err != nil {
		t.Fatal(err)
	}
	if _, err = k.TXRecordDomainProfileQualification(ctx, otherCompany, DomainProfileQualificationInput{
		ProfileID: profile.ID, ProfileRevision: profile.Revision, Decision: DomainProfileQualificationQualified,
		EvidenceInputID: qualificationInput.InputID, EvidenceInputRevision: qualificationInput.Revision,
		Rationale: "cross-company report must be rejected", RequestID: "domain-profile-qualification-cross-company",
	}); err == nil {
		t.Fatal("cross-company profile qualification evidence was accepted")
	}
}

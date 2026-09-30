// pattern: Imperative Shell

package control

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"polis/internal/core"
	"polis/internal/domainworkflow"
	"polis/internal/intake"
	"polis/internal/kernel"
)

func TestContentOperationsBindAuthorizedSourcesAndVersionedDraftReview(t *testing.T) {
	dsn := os.Getenv("POLIS_TEST_DSN")
	if dsn == "" {
		t.Skip("dedicated PostgreSQL required")
	}
	ctx := context.Background()
	runtime, err := kernel.Open(ctx, dsn, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Close()
	companyID := fmt.Sprintf("r3-content-ops-%d", time.Now().UnixNano())
	if _, err = runtime.TXCreateCompany(ctx, companyID); err != nil {
		t.Fatal(err)
	}
	mission, err := runtime.TXCreateMissionGoal(ctx, runtime.LocalScope(companyID), "Content operation fixture", "review source-bound draft and sample claims", "r3-content-ops-mission")
	if err != nil {
		t.Fatal(err)
	}
	addInput := func(name, text, inputID, requestID string) kernel.MissionInputRevision {
		t.Helper()
		prepared, content, prepareErr := intake.PrepareMissionInput(name, "text/markdown", []byte(text))
		if prepareErr != nil {
			t.Fatal(prepareErr)
		}
		input, addErr := runtime.TXAddMissionInput(ctx, runtime.LocalScope(companyID), mission.ID, inputID, requestID, prepared, content)
		if addErr != nil {
			t.Fatal(addErr)
		}
		return input
	}
	source := addInput("source.md", "Synthetic source: the fixture value is 10.", "", "r3-content-source-input")
	plan := addInput("sample-plan.md", "Inspect every declared critical claim in this tiny fixture.", "", "r3-content-sample-plan-input")
	draftInput := addInput("draft.md", "The fixture value is 10.", "", "r3-content-draft-input-v1")
	service := &Service{runtime: runtime}

	for index, input := range []kernel.MissionInputRevision{source, plan} {
		_, err = service.SetContentSourceAuthorization(ctx, companyID, DomainContentSourceAuthorizationCommandRequest{
			SourceInputID: input.InputID, SourceInputRevision: input.Revision,
			SourceSHA256: input.ContentDigest,
			State:        kernel.DomainContentSourceAuthorized, Rationale: "approved for this local synthetic content fixture",
			RequestID: fmt.Sprintf("r3-content-source-authorize-%d", index),
		})
		if err != nil {
			t.Fatalf("authorize source input %s: %v", input.InputID, err)
		}
	}
	draft, err := service.RegisterContentDraft(ctx, companyID, DomainContentDraftCommandRequest{
		DraftInputID: draftInput.InputID, DraftInputRevision: draftInput.Revision, WriterEmployeeID: "emp-backend",
		CriticalClaims: []string{"claim-value"}, ConstraintsPassed: true, RequestID: "r3-content-draft-register-v1",
	})
	if err != nil || draft.DraftSHA256 != draftInput.ContentDigest || draft.DraftRevision != fmt.Sprint(draftInput.Revision) {
		t.Fatalf("register versioned content draft=%+v err=%v", draft, err)
	}
	contentReview := domainworkflow.ContentReview{
		DraftRevision: draftInput.Revision, CheckerEmployeeID: "emp-review", HumanSampled: true,
		Claims: []domainworkflow.ClaimReview{{ClaimID: "claim-value", Finding: domainworkflow.ClaimVerified, Sources: []domainworkflow.SourceReference{{InputID: source.InputID, Revision: source.Revision, SHA256: source.ContentDigest}}}},
	}
	sample := domainworkflow.ContentSampleEvidence{
		DraftRevision: draftInput.Revision, DraftSHA256: draft.DraftSHA256,
		Plan:            domainworkflow.SourceReference{InputID: plan.InputID, Revision: plan.Revision, SHA256: plan.ContentDigest},
		SampledClaimIDs: []string{"claim-value"}, SampledByEmployeeID: "emp-review",
	}
	review, err := service.RecordContentReview(ctx, companyID, draft.DraftInputID, draftInput.Revision, DomainContentReviewCommandRequest{
		Review: contentReview, Sample: sample, RequestID: "r3-content-review-v1",
	})
	if err != nil || review.Outcome != domainworkflow.OutcomeAccepted {
		t.Fatalf("record source-bound human content review=%+v err=%v", review, err)
	}
	replayed, err := service.RecordContentReview(ctx, companyID, draft.DraftInputID, draftInput.Revision, DomainContentReviewCommandRequest{
		Review: contentReview, Sample: sample, RequestID: "r3-content-review-v1",
	})
	if err != nil || replayed.ReviewID != review.ReviewID {
		t.Fatalf("content review replay=%+v err=%v", replayed, err)
	}
	publication, err := service.SimulateContentPublication(ctx, companyID, DomainContentPublicationCommandRequest{ReviewID: review.ReviewID, RequestID: "r3-content-publication-simulate"})
	if err != nil || publication.Mode != "simulation" || publication.ExternalSideEffects || publication.ReceiptSHA256 == "" {
		t.Fatalf("content simulated publication=%+v err=%v", publication, err)
	}
	replayedPublication, err := service.SimulateContentPublication(ctx, companyID, DomainContentPublicationCommandRequest{ReviewID: review.ReviewID, RequestID: "r3-content-publication-simulate"})
	if err != nil || replayedPublication.PublicationID != publication.PublicationID {
		t.Fatalf("content simulated publication replay=%+v err=%v", replayedPublication, err)
	}
	if _, err = service.SetContentSourceAuthorization(ctx, companyID, DomainContentSourceAuthorizationCommandRequest{
		SourceInputID: source.InputID, SourceInputRevision: source.Revision,
		SourceSHA256: source.ContentDigest,
		State:        kernel.DomainContentSourceRevoked, Rationale: "source approval withdrawn",
		RequestID: "r3-content-source-revoke",
	}); err != nil {
		t.Fatal(err)
	}
	draftV2Input := addInput("draft.md", "The fixture value is 10. The second revision is tracked.", draftInput.InputID, "r3-content-draft-input-v2")
	draftV2, err := service.RegisterContentDraft(ctx, companyID, DomainContentDraftCommandRequest{
		DraftInputID: draftV2Input.InputID, DraftInputRevision: draftV2Input.Revision, WriterEmployeeID: "emp-backend",
		CriticalClaims: []string{"claim-value"}, ConstraintsPassed: true, RequestID: "r3-content-draft-register-v2",
	})
	if err != nil || draftV2.DraftRevision != fmt.Sprint(draftV2Input.Revision) || draftV2Input.Revision <= draftInput.Revision {
		t.Fatalf("register next content draft revision=%+v err=%v", draftV2, err)
	}
	if _, err = service.SimulateContentPublication(ctx, companyID, DomainContentPublicationCommandRequest{ReviewID: review.ReviewID, RequestID: "r3-content-publication-stale-review"}); !errors.Is(err, core.Conflict) {
		t.Errorf("stale content review publication error=%v, want conflict", err)
	}
	correction, err := service.RecordContentCorrection(ctx, companyID, DomainContentCorrectionCommandRequest{
		PublicationID: publication.PublicationID, CorrectionDraftInputID: draftV2.DraftInputID, CorrectionDraftRevision: fmt.Sprint(draftV2Input.Revision),
		Rationale: "revise the content using the latest authorized evidence", RequestID: "r3-content-correction",
	})
	if err != nil || correction.State != "review_required" {
		t.Fatalf("content correction=%+v err=%v", correction, err)
	}
	feedback, err := service.RecordContentFeedback(ctx, companyID, DomainContentFeedbackCommandRequest{
		PublicationID: publication.PublicationID, Category: kernel.DomainContentFeedbackCorrectionRequested,
		Note: "The cited source was withdrawn after the simulation.", RequestID: "r3-content-feedback",
	})
	if err != nil || feedback.State != "review_required" || feedback.PublicationID != publication.PublicationID {
		t.Fatalf("content feedback=%+v err=%v", feedback, err)
	}
	staleSourceReview := contentReview
	staleSourceReview.DraftRevision = draftV2Input.Revision
	staleSample := sample
	staleSample.DraftRevision, staleSample.DraftSHA256 = draftV2Input.Revision, draftV2.DraftSHA256
	if _, err = service.RecordContentReview(ctx, companyID, draftV2.DraftInputID, draftV2Input.Revision, DomainContentReviewCommandRequest{
		Review: staleSourceReview, Sample: staleSample, RequestID: "r3-content-review-revoked-source",
	}); err == nil {
		t.Fatal("fact check used a source after its company authorization was revoked")
	}
	ledger, err := runtime.ListDomainEvidence(ctx, companyID)
	if err != nil || len(ledger.ContentSourceEvents) != 3 || len(ledger.ContentDrafts) != 2 || len(ledger.ContentReviews) != 1 || len(ledger.ContentPublications) != 1 || len(ledger.ContentCorrections) != 1 || len(ledger.ContentFeedback) != 1 {
		t.Fatalf("content operation ledger sources=%d drafts=%d reviews=%d publications=%d corrections=%d feedback=%d err=%v", len(ledger.ContentSourceEvents), len(ledger.ContentDrafts), len(ledger.ContentReviews), len(ledger.ContentPublications), len(ledger.ContentCorrections), len(ledger.ContentFeedback), err)
	}
	if !ledger.ContentReviews[0].Stale || ledger.ContentPublications[0].ExternalSideEffects || ledger.ContentCorrections[0].State != "review_required" || ledger.ContentFeedback[0].State != "review_required" {
		t.Fatalf("content operation lifecycle lost its stale/closed/review-required states: reviews=%+v publications=%+v corrections=%+v feedback=%+v", ledger.ContentReviews, ledger.ContentPublications, ledger.ContentCorrections, ledger.ContentFeedback)
	}
	for _, profile := range ledger.Profiles {
		if profile.ID == domainworkflow.ContentOperationsProfileID && (profile.QualificationStatus != "not_run" || profile.ExecutionEnabled) {
			t.Fatalf("content review changed profile qualification or execution: %+v", profile)
		}
	}
	if _, err = service.RegisterContentDraft(ctx, companyID, DomainContentDraftCommandRequest{
		DraftInputID: draftV2Input.InputID, DraftInputRevision: draftV2Input.Revision, WriterEmployeeID: "emp-backend",
		CriticalClaims: []string{"claim-value"}, ConstraintsPassed: true, RequestID: "r3-content-draft-other-request-same-version",
	}); !errors.Is(err, core.Conflict) {
		t.Fatalf("duplicate exact draft revision error=%v, want immutable conflict", err)
	}
}

func TestContentCorrectionRequiresANewReviewBeforeResimulation(t *testing.T) {
	dsn := os.Getenv("POLIS_TEST_DSN")
	if dsn == "" {
		t.Skip("dedicated PostgreSQL required")
	}
	ctx := context.Background()
	runtime, err := kernel.Open(ctx, dsn, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Close()
	companyID := fmt.Sprintf("r3-content-correction-%d", time.Now().UnixNano())
	if _, err = runtime.TXCreateCompany(ctx, companyID); err != nil {
		t.Fatal(err)
	}
	mission, err := runtime.TXCreateMissionGoal(ctx, runtime.LocalScope(companyID), "Content correction fixture", "require post-correction review", "r3-content-correction-mission")
	if err != nil {
		t.Fatal(err)
	}
	addInput := func(name, content, inputID, requestID string) kernel.MissionInputRevision {
		t.Helper()
		prepared, bytes, prepareErr := intake.PrepareMissionInput(name, "text/markdown", []byte(content))
		if prepareErr != nil {
			t.Fatal(prepareErr)
		}
		input, addErr := runtime.TXAddMissionInput(ctx, runtime.LocalScope(companyID), mission.ID, inputID, requestID, prepared, bytes)
		if addErr != nil {
			t.Fatal(addErr)
		}
		return input
	}
	source := addInput("source.md", "Synthetic source states the fixture value is 10.", "", "r3-correction-source")
	plan := addInput("sample-plan.md", "Inspect the claim after every correction.", "", "r3-correction-plan")
	service := &Service{runtime: runtime}
	for index, input := range []kernel.MissionInputRevision{source, plan} {
		if _, err = service.SetContentSourceAuthorization(ctx, companyID, DomainContentSourceAuthorizationCommandRequest{
			SourceInputID: input.InputID, SourceInputRevision: input.Revision, SourceSHA256: input.ContentDigest,
			State: kernel.DomainContentSourceAuthorized, Rationale: "approved local correction fixture",
			RequestID: fmt.Sprintf("r3-correction-authorize-%d", index),
		}); err != nil {
			t.Fatal(err)
		}
	}
	draft1Input := addInput("draft.md", "The fixture value is 10.", "", "r3-correction-draft1-input")
	draft1, err := service.RegisterContentDraft(ctx, companyID, DomainContentDraftCommandRequest{
		DraftInputID: draft1Input.InputID, DraftInputRevision: draft1Input.Revision, WriterEmployeeID: "emp-backend",
		CriticalClaims: []string{"claim-1"}, ConstraintsPassed: true, RequestID: "r3-correction-draft1-register",
	})
	if err != nil {
		t.Fatal(err)
	}
	makeReview := func(draft kernel.DomainContentDraftRecord, input kernel.MissionInputRevision, requestID string, correctionID string) kernel.DomainContentReviewRecord {
		t.Helper()
		review, reviewErr := service.RecordContentReview(ctx, companyID, draft.DraftInputID, input.Revision, DomainContentReviewCommandRequest{
			Review: domainworkflow.ContentReview{DraftRevision: input.Revision, CheckerEmployeeID: "emp-review", HumanSampled: true,
				Claims: []domainworkflow.ClaimReview{{ClaimID: "claim-1", Finding: domainworkflow.ClaimVerified, Sources: []domainworkflow.SourceReference{{InputID: source.InputID, Revision: source.Revision, SHA256: source.ContentDigest}}}}},
			Sample: domainworkflow.ContentSampleEvidence{DraftRevision: input.Revision, DraftSHA256: draft.DraftSHA256,
				Plan:            domainworkflow.SourceReference{InputID: plan.InputID, Revision: plan.Revision, SHA256: plan.ContentDigest},
				SampledClaimIDs: []string{"claim-1"}, SampledByEmployeeID: "emp-review"},
			CorrectionID: correctionID, RequestID: requestID,
		})
		if reviewErr != nil {
			t.Fatalf("record content fact-check %s: %v", requestID, reviewErr)
		}
		return review
	}
	review1 := makeReview(draft1, draft1Input, "r3-correction-review1", "")
	publication1, err := service.SimulateContentPublication(ctx, companyID, DomainContentPublicationCommandRequest{ReviewID: review1.ReviewID, RequestID: "r3-correction-publication1"})
	if err != nil {
		t.Fatal(err)
	}
	draft2Input := addInput("draft.md", "The corrected draft still records the fixture value as 10.", draft1Input.InputID, "r3-correction-draft2-input")
	draft2, err := service.RegisterContentDraft(ctx, companyID, DomainContentDraftCommandRequest{
		DraftInputID: draft2Input.InputID, DraftInputRevision: draft2Input.Revision, WriterEmployeeID: "emp-backend",
		CriticalClaims: []string{"claim-1"}, ConstraintsPassed: true, RequestID: "r3-correction-draft2-register",
	})
	if err != nil {
		t.Fatal(err)
	}
	draft3Input := addInput("draft.md", "The latest corrected draft still records the fixture value as 10.", draft1Input.InputID, "r3-correction-draft3-input")
	draft3, err := service.RegisterContentDraft(ctx, companyID, DomainContentDraftCommandRequest{
		DraftInputID: draft3Input.InputID, DraftInputRevision: draft3Input.Revision, WriterEmployeeID: "emp-backend",
		CriticalClaims: []string{"claim-1"}, ConstraintsPassed: true, RequestID: "r3-correction-draft3-register",
	})
	if err != nil {
		t.Fatal(err)
	}
	preCorrectionReview := makeReview(draft3, draft3Input, "r3-correction-review-before-correction", "")
	if _, err = service.RecordContentCorrection(ctx, companyID, DomainContentCorrectionCommandRequest{
		PublicationID: publication1.PublicationID, CorrectionDraftInputID: draft2.DraftInputID,
		CorrectionDraftRevision: draft2.DraftRevision, Rationale: "reject a superseded correction target", RequestID: "r3-correction-entry-superseded",
	}); !errors.Is(err, core.Conflict) {
		t.Fatalf("superseded correction target error=%v, want conflict", err)
	}
	correction, err := service.RecordContentCorrection(ctx, companyID, DomainContentCorrectionCommandRequest{
		PublicationID: publication1.PublicationID, CorrectionDraftInputID: draft3.DraftInputID,
		CorrectionDraftRevision: draft3.DraftRevision, Rationale: "link the corrected version to the prior simulation", RequestID: "r3-correction-entry",
	})
	if err != nil {
		t.Fatal(err)
	}
	directPool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer directPool.Close()
	invalidReceipt := kernel.DomainContentPublicationReceipt{
		SchemaVersion: "polis-content-publication-simulation@1", Mode: "simulation", PublicationID: "content-publication-unreviewed-correction",
		ReviewID: preCorrectionReview.ReviewID, DraftInputID: draft3.DraftInputID, DraftRevision: draft3.DraftRevision,
		DraftSHA256: draft3.DraftSHA256, ExternalSideEffects: false,
	}
	invalidReceiptBytes, err := json.Marshal(invalidReceipt)
	if err != nil {
		t.Fatal(err)
	}
	invalidReceiptDigest := sha256.Sum256(invalidReceiptBytes)
	_, err = directPool.Exec(ctx, `INSERT INTO domain_workflow_content_publication_simulations(
company_id,publication_id,review_id,draft_input_id,draft_revision,draft_sha256,mode,external_side_effects,receipt_sha256,receipt_json,request_id)
VALUES($1,$2,$3,$4,$5,$6,'simulation',false,$7,$8::jsonb,$9)`, companyID, invalidReceipt.PublicationID, invalidReceipt.ReviewID,
		invalidReceipt.DraftInputID, draft3Input.Revision, invalidReceipt.DraftSHA256, hex.EncodeToString(invalidReceiptDigest[:]), invalidReceiptBytes, "content-publication-unreviewed-correction")
	if err == nil {
		t.Fatal("database accepted a correction publication without a post-correction review")
	}
	if _, err = service.SimulateContentPublication(ctx, companyID, DomainContentPublicationCommandRequest{
		ReviewID: preCorrectionReview.ReviewID, RequestID: "r3-correction-block-pre-review",
	}); !errors.Is(err, core.Conflict) {
		t.Fatalf("pre-correction review publication error=%v, want review-required conflict", err)
	}
	postCorrectionReview := makeReview(draft3, draft3Input, "r3-correction-review-after-correction", correction.CorrectionID)
	if postCorrectionReview.CorrectionID != correction.CorrectionID || postCorrectionReview.Stale {
		t.Fatalf("post-correction review not bound/fresh: %+v", postCorrectionReview)
	}
	publication2, err := service.SimulateContentPublication(ctx, companyID, DomainContentPublicationCommandRequest{
		ReviewID: postCorrectionReview.ReviewID, RequestID: "r3-correction-publication2",
	})
	if err != nil || publication2.ExternalSideEffects {
		t.Fatalf("post-correction simulated publication=%+v err=%v", publication2, err)
	}
}

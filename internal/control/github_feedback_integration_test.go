// pattern: Imperative Shell
package control

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"polis/internal/core"
	githubfeedback "polis/internal/feedback/github"
	"polis/internal/kernel"
)

type fakeGitHubFeedbackProvider struct {
	repository    githubfeedback.Repository
	scanCalls     int
	commentCalls  int
	lastScanOpts  githubfeedback.ScanOptions
	lastCommentOp githubfeedback.CommentOptions
}

func (f *fakeGitHubFeedbackProvider) ScanIssues(_ context.Context, repository githubfeedback.Repository, options githubfeedback.ScanOptions) githubfeedback.ScanResult {
	f.scanCalls++
	f.repository = repository
	f.lastScanOpts = options
	probeHash := strings.Repeat("a", 64)
	if f.scanCalls == 1 {
		return githubfeedback.ScanResult{
			Repository: repository, CoverageCutoff: options.CoverageCutoff, ReadPermissionVerified: true,
			PermissionProbeSHA256: probeHash, Pages: []githubfeedback.PageRecord{{PageNumber: 1, ResponseSHA256: strings.Repeat("b", 64), ItemCount: 0}},
			Coverage: githubfeedback.CoverageComplete, CoveredThrough: &options.CoverageCutoff,
		}
	}
	body := "a bounded observed issue"
	bodyHash := sha256.Sum256([]byte(body))
	issue := githubfeedback.Issue{
		ID: 101, RepositoryID: repository.ID, PageNumber: 1, Number: 7, Title: "A local fake issue", Body: body,
		BodySHA256: hex.EncodeToString(bodyHash[:]), State: "open", UpdatedAt: options.CoverageCutoff.Add(-time.Minute).Format(time.RFC3339),
		HTMLURL: "https://github.com/acme/widget/issues/7", Untrusted: true,
	}
	return githubfeedback.ScanResult{
		Repository: repository, Since: options.Since, CoverageCutoff: options.CoverageCutoff, ReadPermissionVerified: true,
		PermissionProbeSHA256: probeHash, Issues: []githubfeedback.Issue{issue},
		Pages:    []githubfeedback.PageRecord{{PageNumber: 1, ResponseSHA256: strings.Repeat("c", 64), ItemCount: 1}},
		Coverage: githubfeedback.CoverageComplete, CoveredThrough: &options.CoverageCutoff,
	}
}

func (f *fakeGitHubFeedbackProvider) ReadIssueComments(_ context.Context, repository githubfeedback.Repository, issue githubfeedback.Issue, options githubfeedback.CommentOptions) githubfeedback.CommentResult {
	f.commentCalls++
	f.lastCommentOp = options
	body := "a bounded local fake comment"
	bodyHash := sha256.Sum256([]byte(body))
	return githubfeedback.CommentResult{
		Repository: repository, Issue: issue, Coverage: githubfeedback.CoverageComplete,
		Pages: []githubfeedback.PageRecord{{PageNumber: 1, ResponseSHA256: strings.Repeat("d", 64), ItemCount: 1}},
		Comments: []githubfeedback.Comment{{ID: 900, PageNumber: 1, Body: body, BodySHA256: hex.EncodeToString(bodyHash[:]),
			UpdatedAt: time.Now().UTC().Add(-time.Minute).Format(time.RFC3339), HTMLURL: "https://github.com/acme/widget/issues/7#issuecomment-900", Untrusted: true}},
	}
}

func TestGitHubFeedbackControlProbesApprovesPollsAndDoesNotCallUnapprovedSource(t *testing.T) {
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
	companyID := fmt.Sprintf("gh-control-%d", time.Now().UnixNano())
	if _, err = runtime.TXCreateCompany(ctx, companyID); err != nil {
		t.Fatal(err)
	}
	provider := &fakeGitHubFeedbackProvider{}
	service := &Service{runtime: runtime, githubFeedback: provider}
	source, err := service.RegisterGitHubFeedbackSource(ctx, companyID, RegisterGitHubFeedbackSourceRequest{
		RepositoryID: 1296269, Owner: "acme", Name: "widget", RequestID: "control-source-register",
	})
	if err != nil {
		t.Fatal(err)
	}
	if source.State != "draft" || source.PermissionStatus != "unverified" || source.Repository != "acme/widget" {
		t.Fatalf("registered source = %+v", source)
	}
	if _, err = (&Service{runtime: runtime}).ProbeGitHubFeedbackSource(ctx, companyID, source.SourceID, GitHubFeedbackProbeRequest{
		Rationale: "confirm that the default Control path stays disabled", RequestID: "control-source-probe-disabled",
	}); !errors.Is(err, ErrGitHubFeedbackUnavailable) {
		t.Fatalf("probe without injected token source error = %v, want unavailable", err)
	}
	if _, err = service.PollGitHubFeedbackSource(ctx, companyID, source.SourceID, GitHubFeedbackPollRequest{RequestID: "control-poll-before-approval"}); err != core.Denied {
		t.Fatalf("unapproved poll error = %v, want %s", err, core.Denied)
	}
	if provider.scanCalls != 0 || provider.commentCalls != 0 {
		t.Fatalf("unapproved source reached provider: scans=%d comments=%d", provider.scanCalls, provider.commentCalls)
	}
	probe, err := service.ProbeGitHubFeedbackSource(ctx, companyID, source.SourceID, GitHubFeedbackProbeRequest{
		Rationale: "verify only this selected read-only repository", RequestID: "control-source-probe",
	})
	if err != nil {
		t.Fatal(err)
	}
	if probe.PermissionStatus != "verified" || probe.Coverage != "complete" || provider.scanCalls != 1 {
		t.Fatalf("permission probe = %+v provider calls=%d", probe, provider.scanCalls)
	}
	probeReplay, err := service.ProbeGitHubFeedbackSource(ctx, companyID, source.SourceID, GitHubFeedbackProbeRequest{
		Rationale: "verify only this selected read-only repository", RequestID: "control-source-probe",
	})
	if err != nil || probeReplay.PermissionStatus != "verified" || probeReplay.Coverage != "replayed" || provider.scanCalls != 1 {
		t.Fatalf("permission probe replay repeated provider read: receipt=%+v calls=%d err=%v", probeReplay, provider.scanCalls, err)
	}
	approved, err := service.DecideGitHubFeedbackSource(ctx, companyID, source.SourceID, GitHubFeedbackDecisionRequest{
		Decision: "approved", Rationale: "approve the exact repository after its read probe", RequestID: "control-source-approve",
	})
	if err != nil {
		t.Fatal(err)
	}
	if approved.State != "approved" || approved.PermissionStatus != "verified" {
		t.Fatalf("approved source = %+v", approved)
	}
	poll, err := service.PollGitHubFeedbackSource(ctx, companyID, source.SourceID, GitHubFeedbackPollRequest{RequestID: "control-poll-approved"})
	if err != nil {
		t.Fatal(err)
	}
	if poll.Coverage != "complete" || poll.ItemCount != 1 || poll.PageCount != 1 || poll.CommentScanCount != 1 || poll.CommentCoverage != "complete" || poll.Replayed {
		t.Fatalf("bounded poll receipt = %+v", poll)
	}
	if provider.scanCalls != 2 || provider.commentCalls != 1 || provider.lastScanOpts.MaxPages != githubFeedbackPollMaxPages || provider.lastScanOpts.MaxItems != githubFeedbackPollMaxIssues || provider.lastCommentOp.MaxPages != 1 {
		t.Fatalf("provider bounds/call counts = scans %d comments %d scanOpts %+v commentOpts %+v", provider.scanCalls, provider.commentCalls, provider.lastScanOpts, provider.lastCommentOp)
	}
	if _, found, err := runtime.GetGitHubFeedbackScanByRequest(ctx, companyID, poll.RequestID); err != nil || !found {
		t.Fatalf("poll evidence lookup found=%t err=%v", found, err)
	}
	commentCount, partial, _, err := runtime.GitHubFeedbackCommentsForPollRequest(ctx, companyID, poll.RequestID)
	if err != nil || commentCount != 1 || partial {
		t.Fatalf("comment poll evidence count=%d partial=%t err=%v", commentCount, partial, err)
	}
	replay, err := service.PollGitHubFeedbackSource(ctx, companyID, source.SourceID, GitHubFeedbackPollRequest{RequestID: poll.RequestID})
	if err != nil {
		t.Fatal(err)
	}
	if !replay.Replayed || replay.ScanID != poll.ScanID || replay.CommentScanCount != 1 || replay.CommentCoverage != "complete" || provider.scanCalls != 2 || provider.commentCalls != 1 {
		t.Fatalf("poll replay repeated external reads or changed durable result: receipt=%+v scans=%d comments=%d", replay, provider.scanCalls, provider.commentCalls)
	}
	if _, err = service.DecideGitHubFeedbackSource(ctx, companyID, source.SourceID, GitHubFeedbackDecisionRequest{
		Decision: "revoked", Rationale: "revoke the selected source", RequestID: "control-source-revoke",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err = service.PollGitHubFeedbackSource(ctx, companyID, source.SourceID, GitHubFeedbackPollRequest{RequestID: "control-poll-after-revoke"}); err != core.Denied {
		t.Fatalf("revoked source poll error = %v, want %s", err, core.Denied)
	}
	if provider.scanCalls != 2 || provider.commentCalls != 1 {
		t.Fatalf("revoked source reached provider: scans=%d comments=%d", provider.scanCalls, provider.commentCalls)
	}
}

func TestGitHubFeedbackSchedulerRequiresCompanyPolicyAndDoesNotReplayReservedSlot(t *testing.T) {
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
	companyID := fmt.Sprintf("gh-schedule-%d", time.Now().UnixNano())
	if _, err = runtime.TXCreateCompany(ctx, companyID); err != nil {
		t.Fatal(err)
	}
	provider := &fakeGitHubFeedbackProvider{}
	service := &Service{runtime: runtime, githubFeedback: provider}
	database, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	processed, err := service.RunGitHubFeedbackSchedulerOnce(ctx, time.Now().UTC())
	if err != nil || processed != 0 || provider.scanCalls != 0 {
		t.Fatalf("default-off scheduler processed=%d calls=%d err=%v", processed, provider.scanCalls, err)
	}
	source, err := service.RegisterGitHubFeedbackSource(ctx, companyID, RegisterGitHubFeedbackSourceRequest{
		RepositoryID: 1296269, Owner: "acme", Name: "widget", RequestID: "schedule-source-register",
	})
	if err != nil {
		t.Fatal(err)
	}
	service.SetGitHubFeedbackSchedulerEnabled(true)
	policyRequest := GitHubFeedbackCollectionPolicyRequest{
		SourceID: source.SourceID, Enabled: true, IntervalSeconds: 900, Rationale: "explicit bounded company collection policy", RequestID: "schedule-policy-before-approval",
	}
	if _, err = service.SetGitHubFeedbackCollectionPolicy(ctx, companyID, policyRequest); !errors.Is(err, core.Denied) {
		t.Fatalf("policy enable without source approval error=%v, want %s", err, core.Denied)
	}
	if _, err = service.ProbeGitHubFeedbackSource(ctx, companyID, source.SourceID, GitHubFeedbackProbeRequest{
		Rationale: "verify selected read-only source", RequestID: "schedule-source-probe",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err = service.DecideGitHubFeedbackSource(ctx, companyID, source.SourceID, GitHubFeedbackDecisionRequest{
		Decision: "approved", Rationale: "approve source after verified probe", RequestID: "schedule-source-approve",
	}); err != nil {
		t.Fatal(err)
	}
	policyRequest.RequestID = "schedule-policy-enable"
	policy, err := service.SetGitHubFeedbackCollectionPolicy(ctx, companyID, policyRequest)
	if err != nil || !policy.Enabled || policy.IntervalSeconds != 900 || policy.ExternalEnabled != true || policy.SchedulerEnabled != true || policy.NextPollAt == nil {
		t.Fatalf("enabled collection policy=%+v err=%v", policy, err)
	}
	reservedAt := time.Now().UTC()
	if _, err = database.ExecContext(ctx, "UPDATE feedback_collection_schedule_state SET next_poll_at=$3 WHERE company_id=$1 AND source_id=$2", companyID, source.SourceID, reservedAt.Add(-time.Second)); err != nil {
		t.Fatal(err)
	}
	var priorAttemptCount int
	if err = database.QueryRowContext(ctx, "SELECT count(*) FROM feedback_collection_poll_attempts WHERE company_id=$1 AND source_id=$2", companyID, source.SourceID).Scan(&priorAttemptCount); err != nil || priorAttemptCount != 0 {
		t.Fatalf("fresh collection source already has %d poll attempts err=%v", priorAttemptCount, err)
	}
	processed, err = service.RunGitHubFeedbackSchedulerOnce(ctx, reservedAt)
	if err != nil || processed != 1 {
		t.Fatalf("scheduler poll processed=%d err=%v", processed, err)
	}
	if provider.scanCalls != 2 || provider.commentCalls != 1 {
		t.Fatalf("scheduled poll calls=%d comments=%d, want one scan and bounded comment read", provider.scanCalls, provider.commentCalls)
	}
	var attemptStatus string
	if err = database.QueryRowContext(ctx, "SELECT status FROM feedback_collection_poll_attempts WHERE company_id=$1 AND source_id=$2", companyID, source.SourceID).Scan(&attemptStatus); err != nil || attemptStatus != "completed" {
		t.Fatalf("scheduled attempt status=%q err=%v, want completed", attemptStatus, err)
	}
	processed, err = service.RunGitHubFeedbackSchedulerOnce(ctx, reservedAt)
	if err != nil || processed != 0 || provider.scanCalls != 2 || provider.commentCalls != 1 {
		t.Fatalf("scheduler replay processed=%d scans=%d comments=%d err=%v", processed, provider.scanCalls, provider.commentCalls, err)
	}
	unknownSlot := reservedAt.Add(3 * time.Hour)
	if _, err = database.ExecContext(ctx, "UPDATE feedback_collection_schedule_state SET next_poll_at=$3 WHERE company_id=$1 AND source_id=$2", companyID, source.SourceID, unknownSlot.Add(-time.Second)); err != nil {
		t.Fatal(err)
	}
	unknownReservations, err := runtime.TXReserveDueGitHubFeedbackPolls(ctx, unknownSlot, 1)
	if err != nil || len(unknownReservations) != 1 {
		t.Fatalf("reserve outcome-unknown slot=%+v err=%v", unknownReservations, err)
	}
	unknownRequestID := unknownReservations[0].RequestID
	claimed, err := runtime.TXClaimGitHubFeedbackPollDispatch(ctx, companyID, source.SourceID, unknownRequestID, unknownSlot)
	if err != nil || !claimed {
		t.Fatalf("claim dispatch before simulated crash claimed=%t err=%v", claimed, err)
	}
	processed, err = service.RunGitHubFeedbackSchedulerOnce(ctx, unknownSlot.Add(6*time.Minute))
	if err != nil || processed != 0 || provider.scanCalls != 2 {
		t.Fatalf("expired reserved slot replayed provider: processed=%d scans=%d err=%v", processed, provider.scanCalls, err)
	}
	if err = database.QueryRowContext(ctx, "SELECT status FROM feedback_collection_poll_attempts WHERE company_id=$1 AND source_id=$2 AND request_id=$3", companyID, source.SourceID, unknownRequestID).Scan(&attemptStatus); err != nil || attemptStatus != "outcome_unknown" {
		t.Fatalf("expired reservation status=%q err=%v, want outcome_unknown", attemptStatus, err)
	}
	nextSlotReservations, err := runtime.TXReserveDueGitHubFeedbackPolls(ctx, unknownSlot.Add(16*time.Minute), 1)
	if err != nil || len(nextSlotReservations) != 1 || nextSlotReservations[0].RequestID == unknownRequestID {
		t.Fatalf("next scheduled slot reservation=%+v err=%v, want a new non-replayed request ID", nextSlotReservations, err)
	}
	var taskCount int
	if err = database.QueryRowContext(ctx, "SELECT count(*) FROM tasks WHERE company_id=$1", companyID).Scan(&taskCount); err != nil || taskCount != 0 {
		t.Fatalf("scheduled GitHub collection created Tasks=%d err=%v", taskCount, err)
	}
	disable := policyRequest
	disable.Enabled = false
	disable.Rationale = "stop scheduled collection"
	disable.RequestID = "schedule-policy-disable"
	if policy, err = service.SetGitHubFeedbackCollectionPolicy(ctx, companyID, disable); err != nil || policy.Enabled || policy.NextPollAt != nil {
		t.Fatalf("disabled collection policy=%+v err=%v", policy, err)
	}
	claimed, err = runtime.TXClaimGitHubFeedbackPollDispatch(ctx, companyID, source.SourceID, nextSlotReservations[0].RequestID, time.Now().UTC())
	if err != nil || claimed || provider.scanCalls != 2 {
		t.Fatalf("dispatch after policy disable claimed=%t scans=%d err=%v", claimed, provider.scanCalls, err)
	}
	var disabledAttemptStatus, disabledAttemptReason string
	if err = database.QueryRowContext(ctx, "SELECT status,reason_code FROM feedback_collection_poll_attempts WHERE company_id=$1 AND source_id=$2 AND request_id=$3", companyID, source.SourceID, nextSlotReservations[0].RequestID).Scan(&disabledAttemptStatus, &disabledAttemptReason); err != nil || disabledAttemptStatus != "failed" || disabledAttemptReason != "policy_disabled" {
		t.Fatalf("policy-disabled reserved poll status=%q reason=%q err=%v", disabledAttemptStatus, disabledAttemptReason, err)
	}
	processed, err = service.RunGitHubFeedbackSchedulerOnce(ctx, time.Now().UTC())
	if err != nil || processed != 0 || provider.scanCalls != 2 {
		t.Fatalf("disabled scheduler processed=%d calls=%d err=%v", processed, provider.scanCalls, err)
	}
	reenable := policyRequest
	reenable.RequestID = "schedule-policy-reenable-before-revoke"
	if policy, err = service.SetGitHubFeedbackCollectionPolicy(ctx, companyID, reenable); err != nil || !policy.Enabled {
		t.Fatalf("re-enabled collection policy=%+v err=%v", policy, err)
	}
	revokedSlot := reservedAt.Add(12 * time.Hour)
	if _, err = database.ExecContext(ctx, "UPDATE feedback_collection_schedule_state SET next_poll_at=$3 WHERE company_id=$1 AND source_id=$2", companyID, source.SourceID, revokedSlot.Add(-time.Second)); err != nil {
		t.Fatal(err)
	}
	revokedReservations, err := runtime.TXReserveDueGitHubFeedbackPolls(ctx, revokedSlot, 1)
	if err != nil || len(revokedReservations) != 1 {
		t.Fatalf("reserve source before revocation=%+v err=%v", revokedReservations, err)
	}
	if _, err = service.DecideGitHubFeedbackSource(ctx, companyID, source.SourceID, GitHubFeedbackDecisionRequest{
		Decision: "revoked", Rationale: "revoke source before scheduled dispatch", RequestID: "schedule-source-revoke-before-dispatch",
	}); err != nil {
		t.Fatal(err)
	}
	claimed, err = runtime.TXClaimGitHubFeedbackPollDispatch(ctx, companyID, source.SourceID, revokedReservations[0].RequestID, revokedSlot)
	if err != nil || claimed || provider.scanCalls != 2 {
		t.Fatalf("dispatch after source revocation claimed=%t scans=%d err=%v", claimed, provider.scanCalls, err)
	}
	var revokedAttemptStatus, revokedAttemptReason string
	if err = database.QueryRowContext(ctx, "SELECT status,reason_code FROM feedback_collection_poll_attempts WHERE company_id=$1 AND source_id=$2 AND request_id=$3", companyID, source.SourceID, revokedReservations[0].RequestID).Scan(&revokedAttemptStatus, &revokedAttemptReason); err != nil || revokedAttemptStatus != "failed" || revokedAttemptReason != "source_not_authorized" {
		t.Fatalf("revoked-source poll status=%q reason=%q err=%v", revokedAttemptStatus, revokedAttemptReason, err)
	}
}

func TestGitHubFeedbackScheduledDispatchRejectsArchivedCompany(t *testing.T) {
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
	database, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	companyID := fmt.Sprintf("gh-schedule-archived-%d", time.Now().UnixNano())
	if _, err = runtime.TXCreateCompany(ctx, companyID); err != nil {
		t.Fatal(err)
	}
	provider := &fakeGitHubFeedbackProvider{}
	service := &Service{runtime: runtime, githubFeedback: provider}
	service.SetGitHubFeedbackSchedulerEnabled(true)
	source, err := service.RegisterGitHubFeedbackSource(ctx, companyID, RegisterGitHubFeedbackSourceRequest{
		RepositoryID: 1296269, Owner: "acme", Name: "widget", RequestID: "archived-schedule-register",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = service.ProbeGitHubFeedbackSource(ctx, companyID, source.SourceID, GitHubFeedbackProbeRequest{
		Rationale: "verify selected read-only source", RequestID: "archived-schedule-probe",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err = service.DecideGitHubFeedbackSource(ctx, companyID, source.SourceID, GitHubFeedbackDecisionRequest{
		Decision: "approved", Rationale: "approve selected source", RequestID: "archived-schedule-approve",
	}); err != nil {
		t.Fatal(err)
	}
	policy, err := service.SetGitHubFeedbackCollectionPolicy(ctx, companyID, GitHubFeedbackCollectionPolicyRequest{
		SourceID: source.SourceID, Enabled: true, IntervalSeconds: 900, Rationale: "bounded company collection", RequestID: "archived-schedule-policy",
	})
	if err != nil || !policy.Enabled {
		t.Fatalf("collection policy=%+v err=%v", policy, err)
	}
	reservedAt := time.Now().UTC()
	if _, err = database.ExecContext(ctx, "UPDATE feedback_collection_schedule_state SET next_poll_at=$3 WHERE company_id=$1 AND source_id=$2", companyID, source.SourceID, reservedAt.Add(-time.Second)); err != nil {
		t.Fatal(err)
	}
	reservations, err := runtime.TXReserveDueGitHubFeedbackPolls(ctx, reservedAt, 1)
	if err != nil || len(reservations) != 1 {
		t.Fatalf("reserve before company archive=%+v err=%v", reservations, err)
	}
	if _, err = database.ExecContext(ctx, "UPDATE companies SET state='archived' WHERE id=$1", companyID); err != nil {
		t.Fatal(err)
	}
	claimed, err := runtime.TXClaimGitHubFeedbackPollDispatch(ctx, companyID, source.SourceID, reservations[0].RequestID, reservedAt)
	if err != nil || claimed || provider.scanCalls != 1 {
		t.Fatalf("dispatch after company archive claimed=%t providerScans=%d err=%v", claimed, provider.scanCalls, err)
	}
	processed, err := service.RunGitHubFeedbackSchedulerOnce(ctx, reservedAt.Add(30*time.Minute))
	if err != nil || processed != 0 || provider.scanCalls != 1 {
		t.Fatalf("archived company scheduled poll processed=%d providerScans=%d err=%v", processed, provider.scanCalls, err)
	}
	var status, reason string
	if err = database.QueryRowContext(ctx, "SELECT status,reason_code FROM feedback_collection_poll_attempts WHERE company_id=$1 AND source_id=$2 AND request_id=$3", companyID, source.SourceID, reservations[0].RequestID).Scan(&status, &reason); err != nil || status != "failed" || reason != "company_not_active" {
		t.Fatalf("archived reservation status=%q reason=%q err=%v", status, reason, err)
	}
}

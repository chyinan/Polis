// pattern: Imperative Shell
package kernel

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"polis/internal/core"
	githubfeedback "polis/internal/feedback/github"
)

func TestGitHubFeedbackRequiresSourceApprovalAndOnlyCompleteScansAdvanceCursor(t *testing.T) {
	dsn := os.Getenv("POLIS_TEST_DSN")
	if dsn == "" {
		t.Skip("dedicated PostgreSQL required")
	}
	ctx := context.Background()
	runtime, err := Open(ctx, dsn, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Close()
	companyID := fmt.Sprintf("feedback-source-%d", time.Now().UnixNano())
	if _, err = runtime.TXCreateCompany(ctx, companyID); err != nil {
		t.Fatal(err)
	}
	repository := githubfeedback.Repository{ID: 1296269, Owner: "acme", Name: "widget"}
	source, err := runtime.TXRegisterGitHubFeedbackSource(ctx, companyID, GitHubFeedbackSourceInput{
		Repository: repository, RequestID: "feedback-source-register",
	})
	if err != nil {
		t.Fatal(err)
	}
	cutoff := time.Date(2026, 9, 24, 0, 0, 0, 0, time.UTC)
	complete := completeGitHubFeedbackScan(repository, cutoff, "first revision")
	if _, err = runtime.TXRecordGitHubFeedbackScan(ctx, companyID, source.SourceID, complete, "feedback-scan-before-approval"); !errors.Is(err, core.Denied) {
		t.Fatalf("unapproved feedback source scan error = %v, want %s", err, core.Denied)
	}
	if err = runtime.TXDecideGitHubFeedbackSource(ctx, companyID, source.SourceID, GitHubFeedbackSourceDecisionInput{
		Decision: "approved", Rationale: "must not approve before a permission probe", RequestID: "feedback-source-approve-too-early",
	}); !errors.Is(err, core.Denied) {
		t.Fatalf("feedback source approval without a permission probe error = %v, want %s", err, core.Denied)
	}
	probe := githubfeedback.ScanResult{
		Repository: repository, ReadPermissionVerified: true, PermissionProbeSHA256: strings.Repeat("a", 64),
	}
	if err = runtime.TXRecordGitHubFeedbackPermissionProbe(ctx, companyID, GitHubFeedbackPermissionInput{
		SourceID: source.SourceID, Probe: probe, Rationale: "verify selected repository read permission", RequestID: "feedback-source-probe",
	}); err != nil {
		t.Fatal(err)
	}
	if err = runtime.TXDecideGitHubFeedbackSource(ctx, companyID, source.SourceID, GitHubFeedbackSourceDecisionInput{
		Decision: "approved", Rationale: "approve only the selected read-only repository", RequestID: "feedback-source-approve",
	}); err != nil {
		t.Fatal(err)
	}
	first, err := runtime.TXRecordGitHubFeedbackScan(ctx, companyID, source.SourceID, complete, "feedback-scan-complete")
	if err != nil {
		t.Fatal(err)
	}
	if first.Coverage != string(githubfeedback.CoverageComplete) || first.CoveredThrough == nil || !first.CoveredThrough.Equal(cutoff) || first.ItemCount != 1 || first.PageCount != 1 {
		t.Fatalf("complete feedback scan record = %+v", first)
	}
	var observationCount, scanItemCount, pageCount int
	if err = runtime.pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM feedback_observations WHERE company_id=$1 AND source_id=$2),
(SELECT count(*) FROM feedback_scan_items WHERE company_id=$1 AND source_id=$2),
(SELECT count(*) FROM feedback_scan_pages WHERE company_id=$1 AND source_id=$2)`, companyID, source.SourceID).Scan(&observationCount, &scanItemCount, &pageCount); err != nil {
		t.Fatal(err)
	}
	if observationCount != 1 || scanItemCount != 1 || pageCount != 1 {
		t.Fatalf("durable feedback observation/page links = observations %d items %d pages %d", observationCount, scanItemCount, pageCount)
	}
	if _, err = runtime.pool.Exec(ctx, "UPDATE feedback_observations SET title='rewritten' WHERE company_id=$1 AND source_id=$2", companyID, source.SourceID); err == nil {
		t.Fatal("immutable feedback observation was modified")
	}
	commentBody := "bounded comment from the source"
	commentBodySHA := sha256.Sum256([]byte(commentBody))
	commentResult := githubfeedback.CommentResult{
		Repository: repository, Issue: complete.Issues[0], Coverage: githubfeedback.CoverageComplete,
		Pages:    []githubfeedback.PageRecord{{PageNumber: 1, ResponseSHA256: strings.Repeat("c", 64), ItemCount: 1}},
		Comments: []githubfeedback.Comment{{ID: 900, PageNumber: 1, Body: commentBody, BodySHA256: hex.EncodeToString(commentBodySHA[:]), UpdatedAt: cutoff.Add(-30 * time.Minute).Format(time.RFC3339), HTMLURL: "https://github.com/acme/widget/issues/7#issuecomment-900", Untrusted: true}},
	}
	commentScan, err := runtime.TXRecordGitHubFeedbackCommentScan(ctx, companyID, GitHubFeedbackCommentInput{
		SourceID: source.SourceID, Result: commentResult, RequestID: "feedback-comment-scan-complete",
	})
	if err != nil {
		t.Fatal(err)
	}
	if commentScan.Coverage != string(githubfeedback.CoverageComplete) || commentScan.CommentCount != 1 || commentScan.BodyBytes != len(commentBody) {
		t.Fatalf("durable comment context scan = %+v", commentScan)
	}
	var commentObservationCount, commentScanItemCount int
	if err = runtime.pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM feedback_comment_observations WHERE company_id=$1 AND source_id=$2),
(SELECT count(*) FROM feedback_comment_scan_items WHERE company_id=$1 AND source_id=$2)`, companyID, source.SourceID).Scan(&commentObservationCount, &commentScanItemCount); err != nil {
		t.Fatal(err)
	}
	if commentObservationCount != 1 || commentScanItemCount != 1 {
		t.Fatalf("durable feedback comment links = comments %d scan items %d", commentObservationCount, commentScanItemCount)
	}
	if _, err = runtime.pool.Exec(ctx, "UPDATE feedback_comment_observations SET body='rewritten' WHERE company_id=$1 AND source_id=$2", companyID, source.SourceID); err == nil {
		t.Fatal("immutable feedback comment observation was modified")
	}
	badPageCount := complete
	badPageCount.Pages = append([]githubfeedback.PageRecord(nil), complete.Pages...)
	badPageCount.Pages[0].ItemCount = 0
	if _, err = runtime.TXRecordGitHubFeedbackScan(ctx, companyID, source.SourceID, badPageCount, "feedback-scan-invalid-page-count"); !errors.Is(err, core.Malformed) {
		t.Fatalf("page/item inconsistency error = %v, want %s", err, core.Malformed)
	}
	partial := completeGitHubFeedbackScan(repository, cutoff.Add(time.Hour), "revised text")
	partial.Coverage = githubfeedback.CoveragePartial
	partial.CoverageReason = githubfeedback.ReasonRateLimited
	partial.CoveredThrough = nil
	second, err := runtime.TXRecordGitHubFeedbackScan(ctx, companyID, source.SourceID, partial, "feedback-scan-partial")
	if err != nil {
		t.Fatal(err)
	}
	if second.Coverage != string(githubfeedback.CoveragePartial) || second.CoveredThrough != nil {
		t.Fatalf("partial feedback scan record advanced coverage: %+v", second)
	}
	cursor, found, err := runtime.LatestGitHubFeedbackCursor(ctx, companyID, source.SourceID)
	if err != nil {
		t.Fatal(err)
	}
	if !found || !cursor.Equal(cutoff) {
		t.Fatalf("latest feedback cursor = %s found=%t, want %s", cursor, found, cutoff)
	}
	if err = runtime.pool.QueryRow(ctx, `SELECT count(*) FROM feedback_observations WHERE company_id=$1 AND source_id=$2`, companyID, source.SourceID).Scan(&observationCount); err != nil {
		t.Fatal(err)
	}
	if observationCount != 2 {
		t.Fatalf("changed issue revision was not appended: observations=%d", observationCount)
	}
}

func TestGitHubFeedbackBacklogKeepsInternalStateAndReopensOnSourceRevision(t *testing.T) {
	dsn := os.Getenv("POLIS_TEST_DSN")
	if dsn == "" {
		t.Skip("dedicated PostgreSQL required")
	}
	ctx := context.Background()
	runtime, err := Open(ctx, dsn, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Close()
	companyID := fmt.Sprintf("feedback-backlog-%d", time.Now().UnixNano())
	if _, err = runtime.TXCreateCompany(ctx, companyID); err != nil {
		t.Fatal(err)
	}
	repository := githubfeedback.Repository{ID: 1296269, Owner: "acme", Name: "widget"}
	source, err := runtime.TXRegisterGitHubFeedbackSource(ctx, companyID, GitHubFeedbackSourceInput{Repository: repository, RequestID: "backlog-source-register"})
	if err != nil {
		t.Fatal(err)
	}
	probe := githubfeedback.ScanResult{Repository: repository, ReadPermissionVerified: true, PermissionProbeSHA256: strings.Repeat("a", 64)}
	if err = runtime.TXRecordGitHubFeedbackPermissionProbe(ctx, companyID, GitHubFeedbackPermissionInput{SourceID: source.SourceID, Probe: probe, Rationale: "verify selected repository read permission", RequestID: "backlog-source-probe"}); err != nil {
		t.Fatal(err)
	}
	if err = runtime.TXDecideGitHubFeedbackSource(ctx, companyID, source.SourceID, GitHubFeedbackSourceDecisionInput{Decision: "approved", Rationale: "approve selected read-only repository", RequestID: "backlog-source-approve"}); err != nil {
		t.Fatal(err)
	}
	cutoff := time.Date(2026, 9, 24, 0, 0, 0, 0, time.UTC)
	firstScan := completeGitHubFeedbackScan(repository, cutoff, "new issue body")
	if _, err = runtime.TXRecordGitHubFeedbackScan(ctx, companyID, source.SourceID, firstScan, "backlog-scan-first"); err != nil {
		t.Fatal(err)
	}
	item, err := runtime.GetGitHubFeedbackBacklogItem(ctx, companyID, source.SourceID, 101)
	if err != nil || item.Status != "open" || item.RemoteState != "open" {
		t.Fatalf("new backlog projection=%+v err=%v", item, err)
	}
	staleDecision := GitHubFeedbackBacklogDecisionInput{SourceID: source.SourceID, ProviderItemID: 101, RevisionSHA256: strings.Repeat("f", 64), Status: "handled", Rationale: "stale decision must not cover new content", RequestID: "backlog-stale-decision"}
	if _, err = runtime.TXSetGitHubFeedbackBacklogStatus(ctx, companyID, staleDecision); !errors.Is(err, core.Conflict) {
		t.Fatalf("stale backlog decision error=%v, want %s", err, core.Conflict)
	}
	decision := GitHubFeedbackBacklogDecisionInput{SourceID: source.SourceID, ProviderItemID: 101, RevisionSHA256: item.RevisionSHA256, Status: "handled", Rationale: "triaged internally; remote issue remains unchanged", RequestID: "backlog-handle-first"}
	if _, err = runtime.TXSetGitHubFeedbackBacklogStatus(ctx, companyID, decision); err != nil {
		t.Fatal(err)
	}
	replayed, err := runtime.TXSetGitHubFeedbackBacklogStatus(ctx, companyID, decision)
	if err != nil || replayed.Status != "handled" {
		t.Fatalf("backlog decision replay=%+v err=%v", replayed, err)
	}
	updatedScan := completeGitHubFeedbackScan(repository, cutoff.Add(time.Hour), "corrected issue body")
	updatedScan.Issues[0].State = "closed"
	if _, err = runtime.TXRecordGitHubFeedbackScan(ctx, companyID, source.SourceID, updatedScan, "backlog-scan-updated"); err != nil {
		t.Fatal(err)
	}
	item, err = runtime.GetGitHubFeedbackBacklogItem(ctx, companyID, source.SourceID, 101)
	if err != nil || item.Status != "needs_review" || item.StatusReason != "source_revision_updated" || item.RemoteState != "closed" {
		t.Fatalf("updated source revision backlog=%+v err=%v", item, err)
	}
	staleRevisionDecision := decision
	staleRevisionDecision.RequestID = "backlog-stale-revision"
	if _, err = runtime.TXSetGitHubFeedbackBacklogStatus(ctx, companyID, staleRevisionDecision); !errors.Is(err, core.Conflict) {
		t.Fatalf("backlog action against an older issue revision error=%v, want %s", err, core.Conflict)
	}
	otherCompany := fmt.Sprintf("feedback-backlog-other-%d", time.Now().UnixNano())
	if _, err = runtime.TXCreateCompany(ctx, otherCompany); err != nil {
		t.Fatal(err)
	}
	if _, err = runtime.GetGitHubFeedbackBacklogItem(ctx, otherCompany, source.SourceID, 101); !errors.Is(err, core.OutOfScope) {
		t.Fatalf("cross-company backlog read error=%v, want %s", err, core.OutOfScope)
	}
}

func TestGitHubFeedbackScheduleRequiresCurrentKernelLeaseForReserveAndDispatch(t *testing.T) {
	dsn := os.Getenv("POLIS_TEST_DSN")
	if dsn == "" {
		t.Skip("dedicated PostgreSQL required")
	}
	ctx := context.Background()
	runtime, err := Open(ctx, dsn, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Close()
	companyID := fmt.Sprintf("feedback-lease-%d", time.Now().UnixNano())
	if _, err = runtime.TXCreateCompany(ctx, companyID); err != nil {
		t.Fatal(err)
	}
	repository := githubfeedback.Repository{ID: 1296269, Owner: "acme", Name: "widget"}
	source, err := runtime.TXRegisterGitHubFeedbackSource(ctx, companyID, GitHubFeedbackSourceInput{Repository: repository, RequestID: "lease-source-register"})
	if err != nil {
		t.Fatal(err)
	}
	probe := githubfeedback.ScanResult{Repository: repository, ReadPermissionVerified: true, PermissionProbeSHA256: strings.Repeat("a", 64)}
	if err = runtime.TXRecordGitHubFeedbackPermissionProbe(ctx, companyID, GitHubFeedbackPermissionInput{SourceID: source.SourceID, Probe: probe, Rationale: "verify selected read-only source", RequestID: "lease-source-probe"}); err != nil {
		t.Fatal(err)
	}
	if err = runtime.TXDecideGitHubFeedbackSource(ctx, companyID, source.SourceID, GitHubFeedbackSourceDecisionInput{Decision: "approved", Rationale: "approve after probe", RequestID: "lease-source-approve"}); err != nil {
		t.Fatal(err)
	}
	if _, err = runtime.TXSetGitHubFeedbackCollectionPolicy(ctx, companyID, GitHubFeedbackCollectionPolicyInput{SourceID: source.SourceID, Enabled: true, IntervalSeconds: 86400, Rationale: "bounded collection", RequestID: "lease-collection-policy"}); err != nil {
		t.Fatal(err)
	}
	reservedAt := time.Now().UTC()
	if _, err = runtime.pool.Exec(ctx, "UPDATE feedback_collection_schedule_state SET next_poll_at=$3 WHERE company_id=$1 AND source_id=$2", companyID, source.SourceID, reservedAt.Add(-time.Second)); err != nil {
		t.Fatal(err)
	}
	reservations, err := runtime.TXReserveDueGitHubFeedbackPolls(ctx, reservedAt, 1)
	if err != nil || len(reservations) != 1 {
		t.Fatalf("reservation before lease loss=%+v err=%v", reservations, err)
	}
	if err = runtime.lease.Conn().Close(ctx); err != nil {
		t.Fatal(err)
	}
	if claimed, claimErr := runtime.TXClaimGitHubFeedbackPollDispatch(ctx, companyID, source.SourceID, reservations[0].RequestID, reservedAt); !errors.Is(claimErr, core.StaleEpoch) || claimed {
		t.Fatalf("dispatch after lease loss claimed=%t err=%v, want stale epoch", claimed, claimErr)
	}
	var before int
	if err = runtime.pool.QueryRow(ctx, "SELECT count(*) FROM feedback_collection_poll_attempts WHERE company_id=$1 AND source_id=$2", companyID, source.SourceID).Scan(&before); err != nil {
		t.Fatal(err)
	}
	if _, err = runtime.pool.Exec(ctx, "UPDATE feedback_collection_schedule_state SET next_poll_at=$3 WHERE company_id=$1 AND source_id=$2", companyID, source.SourceID, reservedAt.Add(-time.Second)); err != nil {
		t.Fatal(err)
	}
	if _, err = runtime.TXReserveDueGitHubFeedbackPolls(ctx, reservedAt.Add(time.Second), 1); !errors.Is(err, core.StaleEpoch) {
		t.Fatalf("reserve after lease loss error=%v, want stale epoch", err)
	}
	var after int
	if err = runtime.pool.QueryRow(ctx, "SELECT count(*) FROM feedback_collection_poll_attempts WHERE company_id=$1 AND source_id=$2", companyID, source.SourceID).Scan(&after); err != nil || after != before {
		t.Fatalf("attempts changed after lost-lease reservation: before=%d after=%d err=%v", before, after, err)
	}
}

func completeGitHubFeedbackScan(repository githubfeedback.Repository, cutoff time.Time, body string) githubfeedback.ScanResult {
	bodySHA := sha256.Sum256([]byte(body))
	return githubfeedback.ScanResult{
		Repository: repository, CoverageCutoff: cutoff, ReadPermissionVerified: true, PermissionProbeSHA256: strings.Repeat("a", 64),
		Issues: []githubfeedback.Issue{{
			ID: 101, RepositoryID: repository.ID, PageNumber: 1, Number: 7, Title: "A bounded issue", Body: body,
			BodySHA256: hex.EncodeToString(bodySHA[:]), State: "open", UpdatedAt: cutoff.Add(-time.Hour).Format(time.RFC3339),
			HTMLURL: "https://github.com/acme/widget/issues/7", Untrusted: true,
		}},
		Pages:    []githubfeedback.PageRecord{{PageNumber: 1, ResponseSHA256: strings.Repeat("b", 64), ItemCount: 1}},
		Coverage: githubfeedback.CoverageComplete, CoveredThrough: &cutoff,
	}
}

// pattern: Imperative Shell
package workbench

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	githubfeedback "polis/internal/feedback/github"
	"polis/internal/kernel"
)

func TestPostgresReadStoreProjectsScopedBoundedGitHubFeedback(t *testing.T) {
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
	companyID := fmt.Sprintf("feedback-read-%d", time.Now().UnixNano())
	if _, err = runtime.TXCreateCompany(ctx, companyID); err != nil {
		t.Fatal(err)
	}
	repository := githubfeedback.Repository{ID: 1296269, Owner: "acme", Name: "widget"}
	source, err := runtime.TXRegisterGitHubFeedbackSource(ctx, companyID, kernel.GitHubFeedbackSourceInput{
		Repository: repository, RequestID: "read-source-register",
	})
	if err != nil {
		t.Fatal(err)
	}
	cutoff := time.Date(2026, 9, 24, 0, 0, 0, 0, time.UTC)
	permissionProbe := githubfeedback.ScanResult{
		Repository: repository, ReadPermissionVerified: true, PermissionProbeSHA256: strings.Repeat("a", 64),
	}
	if err = runtime.TXRecordGitHubFeedbackPermissionProbe(ctx, companyID, kernel.GitHubFeedbackPermissionInput{
		SourceID: source.SourceID, Probe: permissionProbe, Rationale: "verify selected read-only repository", RequestID: "read-source-probe",
	}); err != nil {
		t.Fatal(err)
	}
	if err = runtime.TXDecideGitHubFeedbackSource(ctx, companyID, source.SourceID, kernel.GitHubFeedbackSourceDecisionInput{
		Decision: "approved", Rationale: "approve selected repository", RequestID: "read-source-approve",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err = runtime.TXSetGitHubFeedbackCollectionPolicy(ctx, companyID, kernel.GitHubFeedbackCollectionPolicyInput{
		SourceID: source.SourceID, Enabled: true, IntervalSeconds: 86400, Rationale: "company authorized bounded read-only collection", RequestID: "read-source-collection-policy",
	}); err != nil {
		t.Fatal(err)
	}
	issueBody := strings.Repeat("界", 12_000)
	issueBodySHA := sha256.Sum256([]byte(issueBody))
	issue := githubfeedback.Issue{
		ID: 101, RepositoryID: repository.ID, PageNumber: 1, Number: 7, Title: strings.Repeat("界", 1_300),
		Body: issueBody, BodySHA256: hex.EncodeToString(issueBodySHA[:]), State: "open",
		UpdatedAt: cutoff.Add(-time.Hour).Format(time.RFC3339), HTMLURL: "https://github.com/acme/widget/issues/7", Untrusted: true,
	}
	issueScan := githubfeedback.ScanResult{
		Repository: repository, CoverageCutoff: cutoff, ReadPermissionVerified: true,
		PermissionProbeSHA256: strings.Repeat("a", 64), Issues: []githubfeedback.Issue{issue},
		Pages:    []githubfeedback.PageRecord{{PageNumber: 1, ResponseSHA256: strings.Repeat("b", 64), ItemCount: 1}},
		Coverage: githubfeedback.CoverageComplete, CoveredThrough: &cutoff,
	}
	if _, err = runtime.TXRecordGitHubFeedbackScan(ctx, companyID, source.SourceID, issueScan, "read-issue-scan"); err != nil {
		t.Fatal(err)
	}
	commentBody := strings.Repeat("注", 5_000)
	commentBodySHA := sha256.Sum256([]byte(commentBody))
	commentScan := githubfeedback.CommentResult{
		Repository: repository, Issue: issue, Coverage: githubfeedback.CoverageComplete,
		Pages: []githubfeedback.PageRecord{{PageNumber: 1, ResponseSHA256: strings.Repeat("c", 64), ItemCount: 1}},
		Comments: []githubfeedback.Comment{{
			ID: 900, PageNumber: 1, Body: commentBody, BodySHA256: hex.EncodeToString(commentBodySHA[:]),
			UpdatedAt: cutoff.Add(-30 * time.Minute).Format(time.RFC3339), HTMLURL: "https://github.com/acme/widget/issues/7#issuecomment-900", Untrusted: true,
		}},
	}
	if _, err = runtime.TXRecordGitHubFeedbackCommentScan(ctx, companyID, kernel.GitHubFeedbackCommentInput{
		SourceID: source.SourceID, Result: commentScan, RequestID: "read-comment-scan",
	}); err != nil {
		t.Fatal(err)
	}
	store, err := NewPostgresReadStore(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	view, err := store.GetCompanyFeedback(ctx, companyID)
	if err != nil {
		t.Fatal(err)
	}
	if len(view.Sources) != 1 || view.Sources[0].State != "approved" || view.Sources[0].PermissionStatus != "verified" || view.Sources[0].Coverage != "complete" || !view.Sources[0].CollectionEnabled || view.Sources[0].CollectionIntervalSeconds != 86400 || view.Sources[0].CollectionRationale == "" || view.Sources[0].CollectionNextPollAt == nil {
		t.Fatalf("feedback source projection = %+v", view.Sources)
	}
	if len(view.Issues) != 1 {
		t.Fatalf("feedback issue count = %d, want 1", len(view.Issues))
	}
	projected := view.Issues[0]
	if projected.SourceID != source.SourceID || projected.ProviderItemID != "101" || projected.IssueNumber != "7" || projected.BacklogStatus != "open" || projected.BacklogReason != "new_issue_observed" || projected.CommentCoverage != "complete" || projected.CommentCount != "1" || len(projected.Comments) != 1 || !projected.CommentContextPartial {
		t.Fatalf("feedback issue projection = %+v", projected)
	}
	if !projected.TitleTruncated || len([]byte(projected.Title)) > FeedbackIssueTitleViewBytes {
		t.Fatalf("projected issue title is not byte-bounded: bytes=%d truncated=%t", len([]byte(projected.Title)), projected.TitleTruncated)
	}
	if !projected.BodyTruncated || len([]byte(projected.Body)) > FeedbackIssueBodyViewBytes {
		t.Fatalf("projected issue body is not byte-bounded: bytes=%d truncated=%t", len([]byte(projected.Body)), projected.BodyTruncated)
	}
	if len(projected.Comments[0].Body) == 0 || !projected.Comments[0].BodyTruncated || len([]byte(projected.Comments[0].Body)) > FeedbackCommentBodyViewBytes {
		t.Fatalf("projected comment body is not byte-bounded: %+v bytes=%d", projected.Comments[0], len([]byte(projected.Comments[0].Body)))
	}
	otherCompany := fmt.Sprintf("feedback-read-other-%d", time.Now().UnixNano())
	if _, err = runtime.TXCreateCompany(ctx, otherCompany); err != nil {
		t.Fatal(err)
	}
	otherView, err := store.GetCompanyFeedback(ctx, otherCompany)
	if err != nil {
		t.Fatal(err)
	}
	if len(otherView.Sources) != 0 || len(otherView.Issues) != 0 {
		t.Fatalf("feedback projection crossed company scope: %+v", otherView)
	}
}

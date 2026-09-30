// pattern: Imperative Shell
package github

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

const testRepoID int64 = 1296269

func writeTestRepositoryMetadata(response http.ResponseWriter, request *http.Request) bool {
	if request.URL.Path != "/repos/acme/widget" {
		return false
	}
	response.Header().Set("Content-Type", "application/json")
	_, _ = response.Write([]byte(`{"id":1296269,"full_name":"acme/widget"}`))
	return true
}

func TestScanIssuesFollowsBoundedLinkPaginationAndExcludesPullRequests(t *testing.T) {
	issueCalls := 0
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Accept") != "application/vnd.github+json" || request.Header.Get("X-GitHub-Api-Version") != SupportedAPIVersion {
			t.Errorf("GitHub request headers = %#v", request.Header)
		}
		if request.Header.Get("Authorization") != "Bearer test-token" {
			t.Errorf("authorization was not provided by the injected credential source: %q", request.Header.Get("Authorization"))
		}
		if writeTestRepositoryMetadata(response, request) {
			return
		}
		issueCalls++
		if issueCalls == 1 {
			if request.URL.Path != "/repos/acme/widget/issues" || request.URL.Query().Get("per_page") != "100" || request.URL.Query().Get("state") != "all" {
				t.Errorf("initial issue request = %s?%s", request.URL.Path, request.URL.RawQuery)
			}
			response.Header().Set("Link", fmt.Sprintf("<%s/repos/acme/widget/issues?page=2&per_page=100&state=all&sort=updated&direction=asc&since=2026-09-23T00%%3A00%%3A00Z>; rel=\"next\"", server.URL))
			response.WriteHeader(http.StatusOK)
			_, _ = response.Write([]byte(`[{"id":10,"number":1,"title":"Bug","body":"untrusted issue body","state":"open","updated_at":"2026-09-20T10:00:00Z","html_url":"https://github.com/acme/widget/issues/1"},{"id":11,"number":2,"title":"PR","body":"ignore","state":"open","updated_at":"2026-09-20T11:00:00Z","pull_request":{"url":"https://api.github.com/repos/acme/widget/pulls/2"}}]`))
			return
		}
		if request.URL.Query().Get("page") != "2" {
			t.Errorf("next page URL was not propagated: %s", request.URL.String())
		}
		response.WriteHeader(http.StatusOK)
		_, _ = response.Write([]byte(`[{"id":12,"number":3,"title":"Another","body":null,"state":"closed","updated_at":"2026-09-21T10:00:00Z","html_url":"https://github.com/acme/widget/issues/3"}]`))
	}))
	defer server.Close()
	var credentialRepository Repository
	client := NewClient(server.Client(), func(_ context.Context, repository Repository) (string, error) {
		credentialRepository = repository
		return "test-token", nil
	})
	client.baseURL = server.URL
	cutoff := time.Date(2026, 9, 24, 0, 0, 0, 0, time.UTC)
	result := client.ScanIssues(context.Background(), Repository{ID: testRepoID, Owner: "acme", Name: "widget"}, ScanOptions{
		Since: cutoff.Add(-24 * time.Hour), CoverageCutoff: cutoff, MaxPages: 3, MaxItems: 100,
	})
	if result.Coverage != CoverageComplete || result.CoverageReason != "" || result.CoveredThrough == nil || !result.CoveredThrough.Equal(cutoff) || issueCalls != 2 {
		t.Fatalf("issue scan coverage = %+v, issue calls=%d", result, issueCalls)
	}
	if credentialRepository.ID != testRepoID || credentialRepository.Owner != "acme" || credentialRepository.Name != "widget" {
		t.Fatalf("token source was not scoped to the selected repository: %+v", credentialRepository)
	}
	if len(result.Issues) != 2 || result.Issues[0].ID != 10 || result.Issues[0].PageNumber != 1 || result.Issues[1].ID != 12 || result.Issues[1].PageNumber != 2 || result.Issues[0].BodySHA256 == "" || !result.Issues[0].Untrusted || len(result.Pages) != 2 || result.Pages[0].PageNumber != 1 || result.Pages[1].PageNumber != 2 {
		t.Fatalf("issue scan items/pages = %+v / %+v", result.Issues, result.Pages)
	}
}

func TestScanIssuesKeepsCompletedPagesButDoesNotAdvanceCoverageOnRateLimit(t *testing.T) {
	issueCalls := 0
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if writeTestRepositoryMetadata(response, request) {
			return
		}
		issueCalls++
		if issueCalls == 1 {
			response.Header().Set("Link", fmt.Sprintf("<%s/repos/acme/widget/issues?page=2&per_page=100&state=all&sort=updated&direction=asc>; rel=\"next\"", server.URL))
			_, _ = response.Write([]byte(`[{"id":10,"number":1,"title":"Bug","body":"body","state":"open","updated_at":"2026-09-20T10:00:00Z"}]`))
			return
		}
		response.Header().Set("Retry-After", "45")
		response.Header().Set("X-RateLimit-Remaining", "0")
		response.WriteHeader(http.StatusTooManyRequests)
		_, _ = response.Write([]byte(`{"message":"rate limit exceeded"}`))
	}))
	defer server.Close()
	client := NewClient(server.Client(), nil)
	client.baseURL = server.URL
	result := client.ScanIssues(context.Background(), Repository{ID: testRepoID, Owner: "acme", Name: "widget"}, ScanOptions{MaxPages: 4, MaxItems: 100})
	if result.Coverage != CoveragePartial || result.CoverageReason != ReasonRateLimited || result.CoveredThrough != nil || result.RetryAfterSeconds != 45 || len(result.Pages) != 1 || len(result.Issues) != 1 {
		t.Fatalf("rate-limited partial coverage = %+v", result)
	}
}

func TestScanIssuesClassifiesHeaderlessSecondaryRateLimitForBoundedBackoff(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if writeTestRepositoryMetadata(response, request) {
			return
		}
		response.WriteHeader(http.StatusForbidden)
		_, _ = response.Write([]byte(`{"message":"You have exceeded a secondary rate limit."}`))
	}))
	defer server.Close()
	client := NewClient(server.Client(), nil)
	client.baseURL = server.URL
	result := client.ScanIssues(context.Background(), Repository{ID: testRepoID, Owner: "acme", Name: "widget"}, ScanOptions{MaxPages: 1, MaxItems: 10})
	if result.Coverage != CoveragePartial || result.CoverageReason != ReasonRateLimited || result.RetryAfterSeconds < 60 {
		t.Fatalf("secondary rate limit was not classified for backoff: %+v", result)
	}
}

func TestScanIssuesRejectsCrossHostPaginationLinks(t *testing.T) {
	issueCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if writeTestRepositoryMetadata(response, request) {
			return
		}
		issueCalls++
		response.Header().Set("Link", "<https://attacker.example/steal>; rel=\"next\"")
		_, _ = response.Write([]byte(`[]`))
	}))
	defer server.Close()
	client := NewClient(server.Client(), nil)
	client.baseURL = server.URL
	result := client.ScanIssues(context.Background(), Repository{ID: testRepoID, Owner: "acme", Name: "widget"}, ScanOptions{MaxPages: 2, MaxItems: 100})
	if result.Coverage != CoveragePartial || result.CoverageReason != ReasonPaginationTargetInvalid || result.CoveredThrough != nil || issueCalls != 1 {
		t.Fatalf("untrusted pagination target result = %+v, issue calls=%d", result, issueCalls)
	}
}

func TestScanIssuesDeduplicatesOverlappingUpdatedWindow(t *testing.T) {
	issueCalls := 0
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if writeTestRepositoryMetadata(response, request) {
			return
		}
		issueCalls++
		if issueCalls == 1 {
			response.Header().Set("Link", fmt.Sprintf("<%s/repos/acme/widget/issues?page=2&per_page=100&state=all&sort=updated&direction=asc>; rel=\"next\"", server.URL))
			_, _ = response.Write([]byte(`[{"id":10,"number":1,"title":"Bug","body":"old body","state":"open","updated_at":"2026-09-20T10:00:00Z"}]`))
			return
		}
		_, _ = response.Write([]byte(`[{"id":10,"number":1,"title":"Bug updated","body":"corrected body","state":"open","updated_at":"2026-09-22T10:00:00Z"}]`))
	}))
	defer server.Close()
	client := NewClient(server.Client(), nil)
	client.baseURL = server.URL
	result := client.ScanIssues(context.Background(), Repository{ID: testRepoID, Owner: "acme", Name: "widget"}, ScanOptions{MaxPages: 2, MaxItems: 10})
	if result.Coverage != CoverageComplete || len(result.Issues) != 1 || result.Issues[0].PageNumber != 2 || result.Issues[0].Title != "Bug updated" || result.Issues[0].Body != "corrected body" {
		t.Fatalf("overlapping issue observations were not deduplicated to latest: %+v", result)
	}
}

func TestScanIssuesPageLimitLeavesCoveragePartial(t *testing.T) {
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if writeTestRepositoryMetadata(response, request) {
			return
		}
		response.Header().Set("Link", fmt.Sprintf("<%s/repos/acme/widget/issues?page=2&per_page=100&state=all&sort=updated&direction=asc>; rel=\"next\"", server.URL))
		_, _ = response.Write([]byte(`[]`))
	}))
	defer server.Close()
	client := NewClient(server.Client(), nil)
	client.baseURL = server.URL
	result := client.ScanIssues(context.Background(), Repository{ID: testRepoID, Owner: "acme", Name: "widget"}, ScanOptions{MaxPages: 1, MaxItems: 10})
	if result.Coverage != CoveragePartial || result.CoverageReason != ReasonPageLimit || result.CoveredThrough != nil || result.NextPageURL == "" {
		t.Fatalf("page-limited scan incorrectly advanced coverage: %+v", result)
	}
}

func TestScanIssuesDoesNotReturnObservationsFromAnUncommittedOverLimitPage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if writeTestRepositoryMetadata(response, request) {
			return
		}
		_, _ = response.Write([]byte(`[{"id":10,"number":1,"title":"First","body":"one","state":"open","updated_at":"2026-09-20T10:00:00Z"},{"id":11,"number":2,"title":"Second","body":"two","state":"open","updated_at":"2026-09-20T11:00:00Z"}]`))
	}))
	defer server.Close()
	client := NewClient(server.Client(), nil)
	client.baseURL = server.URL
	result := client.ScanIssues(context.Background(), Repository{ID: testRepoID, Owner: "acme", Name: "widget"}, ScanOptions{MaxPages: 1, MaxItems: 1})
	if result.Coverage != CoveragePartial || result.CoverageReason != ReasonItemLimit || result.CoveredThrough != nil || len(result.Issues) != 0 || len(result.Pages) != 0 || result.NextPageURL == "" {
		t.Fatalf("partial page escaped scan transaction boundary: %+v", result)
	}
}

func TestScanIssuesRejectsPaginationLinkThatChangesFilterOrSkipsPage(t *testing.T) {
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if writeTestRepositoryMetadata(response, request) {
			return
		}
		response.Header().Set("Link", fmt.Sprintf("<%s/repos/acme/widget/issues?page=9&per_page=100&state=closed&sort=updated&direction=asc>; rel=\"next\"", server.URL))
		_, _ = response.Write([]byte(`[]`))
	}))
	defer server.Close()
	client := NewClient(server.Client(), nil)
	client.baseURL = server.URL
	result := client.ScanIssues(context.Background(), Repository{ID: testRepoID, Owner: "acme", Name: "widget"}, ScanOptions{MaxPages: 3, MaxItems: 10})
	if result.Coverage != CoveragePartial || result.CoverageReason != ReasonPaginationTargetInvalid || result.CoveredThrough != nil {
		t.Fatalf("modified pagination query advanced coverage: %+v", result)
	}
}

func TestScanIssuesRejectsCanonicalPaginationAliasForAnotherRepositoryID(t *testing.T) {
	issueCalls := 0
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if writeTestRepositoryMetadata(response, request) {
			return
		}
		issueCalls++
		response.Header().Set("Link", fmt.Sprintf("<%s/repositories/555/issues?page=2&per_page=100&state=all&sort=updated&direction=asc>; rel=\"next\"", server.URL))
		_, _ = response.Write([]byte(`[]`))
	}))
	defer server.Close()
	client := NewClient(server.Client(), nil)
	client.baseURL = server.URL
	result := client.ScanIssues(context.Background(), Repository{ID: testRepoID, Owner: "acme", Name: "widget"}, ScanOptions{MaxPages: 2, MaxItems: 10})
	if result.Coverage != CoveragePartial || result.CoverageReason != ReasonPaginationTargetInvalid || result.CoveredThrough != nil || issueCalls != 1 {
		t.Fatalf("repository-ID-changing page link was accepted: %+v, issue calls=%d", result, issueCalls)
	}
}

func TestReadIssueCommentsBoundsTextAndKeepsCommentRevision(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if writeTestRepositoryMetadata(response, request) {
			return
		}
		if request.URL.Path == "/repos/acme/widget/issues" {
			_, _ = response.Write([]byte(`[{"id":10,"number":7,"title":"Bug","body":"body","state":"open","updated_at":"2026-09-20T10:00:00Z"}]`))
			return
		}
		if request.URL.Path == "/repos/acme/widget/issues/7" {
			_, _ = response.Write([]byte(`{"id":10,"number":7,"title":"Bug","body":"body","state":"open","updated_at":"2026-09-20T10:00:00Z"}`))
			return
		}
		if request.URL.Path != "/repos/acme/widget/issues/7/comments" || request.URL.Query().Get("per_page") != "100" {
			t.Errorf("comment request = %s?%s", request.URL.Path, request.URL.RawQuery)
		}
		_, _ = response.Write([]byte(`[{"id":99,"body":"0123456789abcdef","updated_at":"2026-09-22T12:00:00Z","html_url":"https://github.com/acme/widget/issues/7#issuecomment-99"}]`))
	}))
	defer server.Close()
	client := NewClient(server.Client(), nil)
	client.baseURL = server.URL
	repository := Repository{ID: testRepoID, Owner: "acme", Name: "widget"}
	issues := client.ScanIssues(context.Background(), repository, ScanOptions{MaxPages: 1, MaxItems: 10})
	if issues.Coverage != CoverageComplete || len(issues.Issues) != 1 {
		t.Fatalf("issue scan before comments = %+v", issues)
	}
	result := client.ReadIssueComments(context.Background(), repository, issues.Issues[0], CommentOptions{MaxPages: 2, MaxComments: 10, MaxBodyBytes: 8})
	if result.Coverage != CoveragePartial || result.CoverageReason != ReasonCommentContextLimit || len(result.Comments) != 1 {
		t.Fatalf("comment result = %+v", result)
	}
	comment := result.Comments[0]
	if comment.ID != 99 || comment.PageNumber != 1 || comment.UpdatedAt != "2026-09-22T12:00:00Z" || !comment.Untrusted || !comment.BodyTruncated || len(comment.Body) != 8 || comment.BodySHA256 == "" {
		t.Fatalf("bounded comment revision = %+v", comment)
	}
}

func TestReadIssueCommentsRejectsIssueFromAnotherBoundRepository(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		calls++
		_, _ = response.Write([]byte(`[]`))
	}))
	defer server.Close()
	client := NewClient(server.Client(), nil)
	client.baseURL = server.URL
	repository := Repository{ID: testRepoID, Owner: "acme", Name: "widget"}
	result := client.ReadIssueComments(context.Background(), repository, Issue{ID: 10, RepositoryID: 777, Number: 7}, CommentOptions{MaxPages: 1, MaxComments: 10})
	if result.CoverageReason != ReasonIssueNotBound || calls != 0 {
		t.Fatalf("comment read crossed repository binding: %+v, requests=%d", result, calls)
	}
}

func TestReadIssueCommentsRechecksAndRejectsPullRequests(t *testing.T) {
	commentCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if writeTestRepositoryMetadata(response, request) {
			return
		}
		if request.URL.Path == "/repos/acme/widget/issues/8" {
			_, _ = response.Write([]byte(`{"id":11,"number":8,"title":"PR","state":"open","updated_at":"2026-09-20T10:00:00Z","pull_request":{"url":"https://api.github.com/repos/acme/widget/pulls/8"}}`))
			return
		}
		if strings.HasSuffix(request.URL.Path, "/comments") {
			commentCalls++
		}
		_, _ = response.Write([]byte(`[]`))
	}))
	defer server.Close()
	client := NewClient(server.Client(), nil)
	client.baseURL = server.URL
	result := client.ReadIssueComments(context.Background(), Repository{ID: testRepoID, Owner: "acme", Name: "widget"}, Issue{ID: 11, RepositoryID: testRepoID, Number: 8}, CommentOptions{MaxPages: 1, MaxComments: 10})
	if result.Coverage != CoveragePartial || result.CoverageReason != ReasonPullRequestExcluded || commentCalls != 0 {
		t.Fatalf("PR comment context was not denied: %+v, comment requests=%d", result, commentCalls)
	}
}

func TestReadIssueCommentsRejectsIssueRevisionDriftBeforeComments(t *testing.T) {
	commentCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if writeTestRepositoryMetadata(response, request) {
			return
		}
		if request.URL.Path == "/repos/acme/widget/issues/8" {
			_, _ = response.Write([]byte(`{"id":11,"number":8,"title":"PR now issue","state":"open","updated_at":"2026-09-22T10:00:00Z"}`))
			return
		}
		if strings.HasSuffix(request.URL.Path, "/comments") {
			commentCalls++
		}
		_, _ = response.Write([]byte(`[]`))
	}))
	defer server.Close()
	client := NewClient(server.Client(), nil)
	client.baseURL = server.URL
	result := client.ReadIssueComments(context.Background(), Repository{ID: testRepoID, Owner: "acme", Name: "widget"}, Issue{
		ID: 11, RepositoryID: testRepoID, Number: 8, UpdatedAt: "2026-09-20T10:00:00Z",
	}, CommentOptions{MaxPages: 1, MaxComments: 10})
	if result.Coverage != CoveragePartial || result.CoverageReason != ReasonIssueRevisionChanged || commentCalls != 0 {
		t.Fatalf("comment read mixed a stale issue revision: %+v, comment requests=%d", result, commentCalls)
	}
}

func TestScanIssuesClassifiesUnauthorizedAndMalformedRepository(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		calls++
		response.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()
	client := NewClient(server.Client(), nil)
	client.baseURL = server.URL
	badRepository := client.ScanIssues(context.Background(), Repository{ID: testRepoID, Owner: "../acme", Name: "widget"}, ScanOptions{MaxPages: 1, MaxItems: 1})
	if badRepository.CoverageReason != ReasonRepositoryInvalid || calls != 0 {
		t.Fatalf("malformed repository request = %+v, calls=%d", badRepository, calls)
	}
	unauthorized := client.ScanIssues(context.Background(), Repository{ID: testRepoID, Owner: "acme", Name: "widget"}, ScanOptions{MaxPages: 1, MaxItems: 1})
	if unauthorized.Coverage != CoveragePartial || unauthorized.CoverageReason != ReasonUnauthorized || unauthorized.CoveredThrough != nil {
		t.Fatalf("unauthorized source coverage = %+v", unauthorized)
	}
	if strings.Contains(unauthorized.CoverageReason, "token") {
		t.Fatalf("credential detail leaked through reason code: %+v", unauthorized)
	}
}

func TestScanIssuesVerifiesImmutableRepositoryIdentityBeforeReadingIssues(t *testing.T) {
	issueCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/repos/acme/widget" {
			_, _ = response.Write([]byte(`{"id":999,"full_name":"acme/widget"}`))
			return
		}
		issueCalls++
		_, _ = response.Write([]byte(`[]`))
	}))
	defer server.Close()
	client := NewClient(server.Client(), nil)
	client.baseURL = server.URL
	result := client.ScanIssues(context.Background(), Repository{ID: testRepoID, Owner: "acme", Name: "widget"}, ScanOptions{MaxPages: 2, MaxItems: 10})
	if result.Coverage != CoveragePartial || result.CoverageReason != ReasonRepositoryIdentityMismatch || result.CoveredThrough != nil || issueCalls != 0 {
		t.Fatalf("repository identity drift did not stop collection: %+v, issue calls=%d", result, issueCalls)
	}
}

func TestScanIssuesTruncatesLargeIssueBodyButKeepsOriginalFingerprint(t *testing.T) {
	body := strings.Repeat("a", maxIssueBodyBytes+17)
	title := strings.Repeat("t", maxIssueTitleBytes+9)
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if writeTestRepositoryMetadata(response, request) {
			return
		}
		_, _ = fmt.Fprintf(response, `[{"id":10,"number":1,"title":%q,"body":%q,"state":"open","updated_at":"2026-09-20T10:00:00Z"}]`, title, body)
	}))
	defer server.Close()
	client := NewClient(server.Client(), nil)
	client.baseURL = server.URL
	result := client.ScanIssues(context.Background(), Repository{ID: testRepoID, Owner: "acme", Name: "widget"}, ScanOptions{MaxPages: 1, MaxItems: 10})
	want := sha256.Sum256([]byte(body))
	if result.Coverage != CoverageComplete || len(result.Issues) != 1 || len(result.Issues[0].Body) != maxIssueBodyBytes || !result.Issues[0].BodyTruncated || result.Issues[0].BodySHA256 != hex.EncodeToString(want[:]) || len(result.Issues[0].Title) != maxIssueTitleBytes || !result.Issues[0].TitleTruncated {
		t.Fatalf("bounded issue body lost its original fingerprint: %+v", result)
	}
}

// pattern: Imperative Shell
package github

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	SupportedAPIVersion = "2026-03-10"
	ProfileRevision     = "github-issues-readonly@1"
	FilterRevision      = "github-issues-overlap-updated-asc@1"
)

const (
	defaultMaxPages     = 5
	defaultMaxIssues    = 500
	defaultMaxComments  = 100
	defaultMaxBodyBytes = 64 << 10
	maxAllowedPages     = 20
	maxAllowedIssues    = 2000
	maxAllowedComments  = 500
	maxAllowedBodyBytes = 1 << 20
	maxResponseBytes    = 2 << 20
	maxIssueBodyBytes   = 64 << 10
	maxIssueTitleBytes  = 4096
)

type TokenSource func(context.Context, Repository) (string, error)

type Repository struct {
	ID    int64
	Owner string
	Name  string
}

type ScanOptions struct {
	Since          time.Time
	CoverageCutoff time.Time
	MaxPages       int
	MaxItems       int
}

type CommentOptions struct {
	Since        time.Time
	MaxPages     int
	MaxComments  int
	MaxBodyBytes int
}

type Coverage string

const (
	CoverageComplete Coverage = "complete"
	CoveragePartial  Coverage = "partial"
)

const (
	ReasonRateLimited                = "github_rate_limited"
	ReasonPaginationTargetInvalid    = "github_pagination_target_invalid"
	ReasonRepositoryInvalid          = "github_repository_invalid"
	ReasonRepositoryIdentityMismatch = "github_repository_identity_mismatch"
	ReasonUnauthorized               = "github_unauthorized"
	ReasonForbidden                  = "github_forbidden"
	ReasonNotFoundOrHidden           = "github_not_found_or_hidden"
	ReasonUpstreamUnavailable        = "github_upstream_unavailable"
	ReasonInvalidResponse            = "github_invalid_response"
	ReasonNetworkUnavailable         = "github_network_unavailable"
	ReasonTokenUnavailable           = "github_token_unavailable"
	ReasonPageLimit                  = "github_page_limit"
	ReasonItemLimit                  = "github_item_limit"
	ReasonCommentLimit               = "github_comment_limit"
	ReasonCommentContextLimit        = "github_comment_context_limit"
	ReasonPaginationCycle            = "github_pagination_cycle"
	ReasonResponseTooLarge           = "github_response_too_large"
	ReasonIssueNotBound              = "github_issue_not_bound_to_repository"
	ReasonPullRequestExcluded        = "github_pull_request_excluded"
	ReasonIssueRevisionChanged       = "github_issue_revision_changed"
)

type Issue struct {
	ID             int64
	RepositoryID   int64
	PageNumber     int
	Number         int
	Title          string
	TitleTruncated bool
	Body           string
	BodySHA256     string
	BodyTruncated  bool
	State          string
	UpdatedAt      string
	HTMLURL        string
	Untrusted      bool
}

type Comment struct {
	ID            int64
	PageNumber    int
	Body          string
	BodySHA256    string
	BodyTruncated bool
	UpdatedAt     string
	HTMLURL       string
	Untrusted     bool
}

type PageRecord struct {
	PageNumber     int
	RequestURL     string
	ResponseSHA256 string
	ETag           string
	ItemCount      int
}

type ScanResult struct {
	Repository             Repository
	Since                  time.Time
	CoverageCutoff         time.Time
	ReadPermissionVerified bool
	PermissionProbeSHA256  string
	Issues                 []Issue
	Pages                  []PageRecord
	Coverage               Coverage
	CoverageReason         string
	CoveredThrough         *time.Time
	NextPageURL            string
	RetryAfterSeconds      int
}

type CommentResult struct {
	Repository        Repository
	Issue             Issue
	Comments          []Comment
	Pages             []PageRecord
	Coverage          Coverage
	CoverageReason    string
	NextPageURL       string
	RetryAfterSeconds int
}

type Client struct {
	httpClient  *http.Client
	tokenSource TokenSource
	baseURL     string
}

type issueWire struct {
	ID          int64           `json:"id"`
	Number      int             `json:"number"`
	Title       string          `json:"title"`
	Body        *string         `json:"body"`
	State       string          `json:"state"`
	UpdatedAt   string          `json:"updated_at"`
	HTMLURL     string          `json:"html_url"`
	PullRequest json.RawMessage `json:"pull_request"`
}

type commentWire struct {
	ID        int64   `json:"id"`
	Body      *string `json:"body"`
	UpdatedAt string  `json:"updated_at"`
	HTMLURL   string  `json:"html_url"`
}

type repositoryWire struct {
	ID       int64  `json:"id"`
	FullName string `json:"full_name"`
}

type pageResponse struct {
	header     http.Header
	body       []byte
	reason     string
	retryAfter int
}

var (
	githubRepoPart = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]{0,99}$`)
)

func NewClient(httpClient *http.Client, tokenSource TokenSource) *Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 20 * time.Second}
	} else {
		cloned := *httpClient
		if cloned.Timeout == 0 {
			cloned.Timeout = 20 * time.Second
		}
		httpClient = &cloned
	}
	httpClient.CheckRedirect = func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }
	return &Client{httpClient: httpClient, tokenSource: tokenSource, baseURL: "https://api.github.com"}
}

func (c *Client) ScanIssues(ctx context.Context, repository Repository, options ScanOptions) ScanResult {
	cutoff := options.CoverageCutoff.UTC()
	if cutoff.IsZero() {
		cutoff = time.Now().UTC()
	}
	result := ScanResult{Repository: repository, Since: options.Since.UTC(), CoverageCutoff: cutoff, Issues: []Issue{}, Pages: []PageRecord{}, Coverage: CoveragePartial}
	if !validRepository(repository) {
		result.CoverageReason = ReasonRepositoryInvalid
		return result
	}
	if reason, retryAfter := c.verifyRepository(ctx, repository); reason != "" {
		result.CoverageReason = reason
		result.RetryAfterSeconds = retryAfter
		return result
	}
	maxPages, maxItems, valid := normalizeScanLimits(options.MaxPages, options.MaxItems)
	if !valid {
		result.CoverageReason = ReasonItemLimit
		return result
	}
	currentURL, err := c.issueListURL(repository, options.Since)
	if err != nil {
		result.CoverageReason = ReasonRepositoryInvalid
		return result
	}
	seenURLs := make(map[string]struct{}, maxPages)
	issueIndex := make(map[int64]int, maxItems)
	for pageNumber := 0; pageNumber < maxPages; pageNumber++ {
		if _, exists := seenURLs[currentURL]; exists {
			result.CoverageReason = ReasonPaginationCycle
			result.NextPageURL = currentURL
			return result
		}
		seenURLs[currentURL] = struct{}{}
		fetched := c.fetchPage(ctx, repository, currentURL)
		if fetched.reason != "" {
			result.CoverageReason = fetched.reason
			result.NextPageURL = currentURL
			result.RetryAfterSeconds = fetched.retryAfter
			return result
		}
		var wireItems []issueWire
		if err := decodeJSONArray(fetched.body, &wireItems); err != nil || len(wireItems) > 100 {
			result.CoverageReason = ReasonInvalidResponse
			result.NextPageURL = currentURL
			return result
		}
		if pageNumber == 0 {
			result.ReadPermissionVerified = true
			result.PermissionProbeSHA256 = sha256Hex(fetched.body)
		}
		pageIssues := make([]Issue, 0, len(wireItems))
		for _, item := range wireItems {
			if isPullRequest(item.PullRequest) {
				continue
			}
			issue, ok := normalizeIssue(item)
			if !ok {
				result.CoverageReason = ReasonInvalidResponse
				result.NextPageURL = currentURL
				return result
			}
			issue.RepositoryID = repository.ID
			issue.PageNumber = pageNumber + 1
			pageIssues = append(pageIssues, issue)
		}
		if wouldExceedIssueLimit(result.Issues, issueIndex, pageIssues, maxItems) {
			result.CoverageReason = ReasonItemLimit
			result.NextPageURL = currentURL
			return result
		}
		appendBoundedIssues(&result.Issues, issueIndex, pageIssues, maxItems)
		result.Pages = append(result.Pages, PageRecord{PageNumber: pageNumber + 1, RequestURL: currentURL, ResponseSHA256: sha256Hex(fetched.body), ETag: fetched.header.Get("ETag"), ItemCount: len(wireItems)})
		next, exists, err := nextPageURL(fetched.header.Get("Link"), c.baseURL, repository, 0, pageNumber+1, options.Since)
		if err != nil {
			result.CoverageReason = ReasonPaginationTargetInvalid
			return result
		}
		if !exists {
			result.Coverage = CoverageComplete
			result.CoverageReason = ""
			result.CoveredThrough = &cutoff
			return result
		}
		if pageNumber+1 >= maxPages {
			result.CoverageReason = ReasonPageLimit
			result.NextPageURL = next
			return result
		}
		currentURL = next
	}
	result.CoverageReason = ReasonPageLimit
	result.NextPageURL = currentURL
	return result
}

func (c *Client) ReadIssueComments(ctx context.Context, repository Repository, issue Issue, options CommentOptions) (result CommentResult) {
	result = CommentResult{Repository: repository, Issue: issue, Comments: []Comment{}, Pages: []PageRecord{}, Coverage: CoveragePartial}
	if !validRepository(repository) {
		result.CoverageReason = ReasonRepositoryInvalid
		return result
	}
	if issue.Number <= 0 || issue.ID <= 0 || issue.RepositoryID != repository.ID {
		result.CoverageReason = ReasonIssueNotBound
		return result
	}
	if reason, retryAfter := c.verifyRepository(ctx, repository); reason != "" {
		result.CoverageReason = reason
		result.RetryAfterSeconds = retryAfter
		return result
	}
	if reason, retryAfter := c.verifyIssue(ctx, repository, issue); reason != "" {
		result.CoverageReason = reason
		result.RetryAfterSeconds = retryAfter
		return result
	}
	maxPages, maxComments, maxBytes, valid := normalizeCommentLimits(options)
	if !valid {
		result.CoverageReason = ReasonCommentLimit
		return result
	}
	defer func() {
		if truncateCommentBodies(&result.Comments, maxBytes) && result.Coverage == CoverageComplete {
			result.Coverage = CoveragePartial
			result.CoverageReason = ReasonCommentContextLimit
		}
	}()
	currentURL, err := c.issueCommentsURL(repository, issue.Number, options.Since)
	if err != nil {
		result.CoverageReason = ReasonRepositoryInvalid
		return result
	}
	seenURLs := make(map[string]struct{}, maxPages)
	commentIndex := make(map[int64]int, maxComments)
	for pageNumber := 0; pageNumber < maxPages; pageNumber++ {
		if _, exists := seenURLs[currentURL]; exists {
			result.CoverageReason = ReasonPaginationCycle
			result.NextPageURL = currentURL
			return result
		}
		seenURLs[currentURL] = struct{}{}
		fetched := c.fetchPage(ctx, repository, currentURL)
		if fetched.reason != "" {
			result.CoverageReason = fetched.reason
			result.NextPageURL = currentURL
			result.RetryAfterSeconds = fetched.retryAfter
			return result
		}
		var wireItems []commentWire
		if err := decodeJSONArray(fetched.body, &wireItems); err != nil || len(wireItems) > 100 {
			result.CoverageReason = ReasonInvalidResponse
			result.NextPageURL = currentURL
			return result
		}
		pageComments := make([]Comment, 0, len(wireItems))
		for _, item := range wireItems {
			comment, ok := normalizeComment(item)
			if !ok {
				result.CoverageReason = ReasonInvalidResponse
				result.NextPageURL = currentURL
				return result
			}
			comment.PageNumber = pageNumber + 1
			pageComments = append(pageComments, comment)
		}
		if wouldExceedCommentLimit(result.Comments, commentIndex, pageComments, maxComments) {
			result.CoverageReason = ReasonCommentLimit
			result.NextPageURL = currentURL
			return result
		}
		appendBoundedComments(&result.Comments, commentIndex, pageComments)
		result.Pages = append(result.Pages, PageRecord{PageNumber: pageNumber + 1, RequestURL: currentURL, ResponseSHA256: sha256Hex(fetched.body), ETag: fetched.header.Get("ETag"), ItemCount: len(wireItems)})
		next, exists, err := nextPageURL(fetched.header.Get("Link"), c.baseURL, repository, issue.Number, pageNumber+1, options.Since)
		if err != nil {
			result.CoverageReason = ReasonPaginationTargetInvalid
			return result
		}
		if !exists {
			result.Coverage = CoverageComplete
			result.CoverageReason = ""
			return result
		}
		if pageNumber+1 >= maxPages {
			result.CoverageReason = ReasonPageLimit
			result.NextPageURL = next
			return result
		}
		currentURL = next
	}
	result.CoverageReason = ReasonPageLimit
	result.NextPageURL = currentURL
	return result
}

func (c *Client) issueListURL(repository Repository, since time.Time) (string, error) {
	endpoint, err := c.repoEndpoint(repository, "issues")
	if err != nil {
		return "", err
	}
	query := endpoint.Query()
	query.Set("state", "all")
	query.Set("sort", "updated")
	query.Set("direction", "asc")
	query.Set("per_page", "100")
	if !since.IsZero() {
		query.Set("since", since.UTC().Format(time.RFC3339))
	}
	endpoint.RawQuery = query.Encode()
	return endpoint.String(), nil
}

func (c *Client) issueCommentsURL(repository Repository, issueNumber int, since time.Time) (string, error) {
	endpoint, err := c.repoEndpoint(repository, fmt.Sprintf("issues/%d/comments", issueNumber))
	if err != nil {
		return "", err
	}
	query := endpoint.Query()
	query.Set("sort", "updated")
	query.Set("direction", "asc")
	query.Set("per_page", "100")
	if !since.IsZero() {
		query.Set("since", since.UTC().Format(time.RFC3339))
	}
	endpoint.RawQuery = query.Encode()
	return endpoint.String(), nil
}

func (c *Client) repoEndpoint(repository Repository, suffix string) (*url.URL, error) {
	if !validRepository(repository) {
		return nil, errors.New("invalid repository")
	}
	base, err := url.Parse(strings.TrimRight(c.baseURL, "/"))
	if err != nil || (base.Scheme != "https" && base.Scheme != "http") || base.Host == "" || base.User != nil || base.RawQuery != "" || base.Fragment != "" {
		return nil, errors.New("invalid API base URL")
	}
	base.Path = strings.TrimRight(base.Path, "/") + "/repos/" + repository.Owner + "/" + repository.Name
	if suffix = strings.Trim(suffix, "/"); suffix != "" {
		base.Path += "/" + suffix
	}
	return base, nil
}

func (c *Client) verifyRepository(ctx context.Context, repository Repository) (string, int) {
	endpoint, err := c.repoEndpoint(repository, "")
	if err != nil {
		return ReasonRepositoryInvalid, 0
	}
	response := c.fetchPage(ctx, repository, endpoint.String())
	if response.reason != "" {
		return response.reason, response.retryAfter
	}
	var actual repositoryWire
	if err := json.Unmarshal(response.body, &actual); err != nil || actual.ID <= 0 || actual.FullName == "" {
		return ReasonInvalidResponse, 0
	}
	expectedName := repository.Owner + "/" + repository.Name
	if actual.ID != repository.ID || !strings.EqualFold(actual.FullName, expectedName) {
		return ReasonRepositoryIdentityMismatch, 0
	}
	return "", 0
}

func (c *Client) verifyIssue(ctx context.Context, repository Repository, expected Issue) (string, int) {
	endpoint, err := c.repoEndpoint(repository, fmt.Sprintf("issues/%d", expected.Number))
	if err != nil {
		return ReasonRepositoryInvalid, 0
	}
	response := c.fetchPage(ctx, repository, endpoint.String())
	if response.reason != "" {
		return response.reason, response.retryAfter
	}
	var actual issueWire
	if err := json.Unmarshal(response.body, &actual); err != nil || actual.ID <= 0 || actual.Number <= 0 {
		return ReasonInvalidResponse, 0
	}
	if isPullRequest(actual.PullRequest) {
		return ReasonPullRequestExcluded, 0
	}
	if actual.ID != expected.ID || actual.Number != expected.Number {
		return ReasonIssueNotBound, 0
	}
	if actual.UpdatedAt != expected.UpdatedAt {
		return ReasonIssueRevisionChanged, 0
	}
	return "", 0
}

func (c *Client) fetchPage(ctx context.Context, repository Repository, requestURL string) pageResponse {
	parsed, err := url.Parse(requestURL)
	if err != nil {
		return pageResponse{reason: ReasonPaginationTargetInvalid}
	}
	base, err := url.Parse(c.baseURL)
	if err != nil || parsed.Scheme != base.Scheme || !strings.EqualFold(parsed.Host, base.Host) || parsed.User != nil || parsed.Fragment != "" {
		return pageResponse{reason: ReasonPaginationTargetInvalid}
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return pageResponse{reason: ReasonPaginationTargetInvalid}
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("X-GitHub-Api-Version", SupportedAPIVersion)
	request.Header.Set("User-Agent", "Polis-read-only-feedback/1")
	if c.tokenSource != nil {
		token, tokenErr := c.tokenSource(ctx, repository)
		if tokenErr != nil || !validToken(token) {
			return pageResponse{reason: ReasonTokenUnavailable}
		}
		request.Header.Set("Authorization", "Bearer "+token)
	}
	response, err := c.httpClient.Do(request)
	if err != nil {
		return pageResponse{reason: ReasonNetworkUnavailable}
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, maxResponseBytes+1))
	if err != nil {
		return pageResponse{reason: ReasonNetworkUnavailable}
	}
	if len(body) > maxResponseBytes {
		return pageResponse{header: response.Header.Clone(), reason: ReasonResponseTooLarge}
	}
	result := pageResponse{header: response.Header.Clone(), body: body}
	if response.StatusCode != http.StatusOK {
		result.reason = classifyStatus(response.StatusCode, response.Header, body)
		result.retryAfter = retryAfterSeconds(response.StatusCode, response.Header, body)
	}
	return result
}

func nextPageURL(linkHeader, baseURL string, repository Repository, issueNumber, currentPage int, since time.Time) (string, bool, error) {
	for _, link := range strings.Split(linkHeader, ",") {
		parts := strings.Split(link, ";")
		if len(parts) < 2 {
			continue
		}
		rel := ""
		for _, parameter := range parts[1:] {
			key, value, found := strings.Cut(strings.TrimSpace(parameter), "=")
			if found && strings.EqualFold(key, "rel") {
				rel = strings.Trim(value, `"`)
				break
			}
		}
		if rel != "next" {
			continue
		}
		target := strings.TrimSpace(parts[0])
		if len(target) < 3 || target[0] != '<' || target[len(target)-1] != '>' {
			return "", false, errors.New("malformed next link")
		}
		reference, err := url.Parse(target[1 : len(target)-1])
		if err != nil {
			return "", false, err
		}
		base, err := url.Parse(baseURL)
		if err != nil {
			return "", false, err
		}
		resolved := base.ResolveReference(reference)
		if resolved.Scheme != base.Scheme || !strings.EqualFold(resolved.Host, base.Host) || resolved.User != nil || resolved.Fragment != "" || !allowedPaginationPath(resolved.EscapedPath(), repository, issueNumber) || !validNextQuery(resolved.Query(), issueNumber, currentPage+1, since) {
			return "", false, errors.New("next link escaped the approved repository API path")
		}
		return resolved.String(), true, nil
	}
	return "", false, nil
}

func validNextQuery(query url.Values, issueNumber, expectedPage int, since time.Time) bool {
	allowed := map[string]struct{}{"page": {}, "per_page": {}, "since": {}, "sort": {}, "direction": {}}
	if issueNumber == 0 {
		allowed["state"] = struct{}{}
	}
	for key := range query {
		if _, exists := allowed[key]; !exists {
			return false
		}
	}
	page, err := strconv.Atoi(query.Get("page"))
	if err != nil || page != expectedPage || query.Get("per_page") != "100" || query.Get("sort") != "updated" || query.Get("direction") != "asc" {
		return false
	}
	if issueNumber == 0 && query.Get("state") != "all" {
		return false
	}
	if since.IsZero() {
		return query.Get("since") == ""
	}
	return query.Get("since") == since.UTC().Format(time.RFC3339)
}

func allowedPaginationPath(pathValue string, repository Repository, issueNumber int) bool {
	owner := url.PathEscape(repository.Owner)
	repo := url.PathEscape(repository.Name)
	issuesPath := "/repos/" + owner + "/" + repo + "/issues"
	canonicalRepositoryPath := "/repositories/" + strconv.FormatInt(repository.ID, 10) + "/issues"
	if issueNumber == 0 {
		return pathValue == issuesPath || pathValue == canonicalRepositoryPath
	}
	commentPath := issuesPath + "/" + strconv.Itoa(issueNumber) + "/comments"
	canonicalCommentPath := canonicalRepositoryPath + "/" + strconv.Itoa(issueNumber) + "/comments"
	return pathValue == commentPath || pathValue == canonicalCommentPath
}

func decodeJSONArray(body []byte, target any) error {
	if len(body) == 0 || body[0] != '[' {
		return errors.New("response is not a JSON array")
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return errors.New("response has trailing JSON")
	}
	return nil
}

func normalizeIssue(wire issueWire) (Issue, bool) {
	if wire.ID <= 0 || wire.Number <= 0 || wire.Title == "" || (wire.State != "open" && wire.State != "closed") || !validTimestamp(wire.UpdatedAt) {
		return Issue{}, false
	}
	body := ""
	if wire.Body != nil {
		body = *wire.Body
	}
	hash := sha256Hex([]byte(body))
	body, truncated := truncateUTF8(body, maxIssueBodyBytes)
	title, titleTruncated := truncateUTF8(wire.Title, maxIssueTitleBytes)
	return Issue{ID: wire.ID, Number: wire.Number, Title: title, TitleTruncated: titleTruncated, Body: body, BodySHA256: hash, BodyTruncated: truncated, State: wire.State, UpdatedAt: wire.UpdatedAt, HTMLURL: safeIssueURL(wire.HTMLURL), Untrusted: true}, true
}

func normalizeComment(wire commentWire) (Comment, bool) {
	if wire.ID <= 0 || !validTimestamp(wire.UpdatedAt) {
		return Comment{}, false
	}
	body := ""
	if wire.Body != nil {
		body = *wire.Body
	}
	return Comment{ID: wire.ID, Body: body, BodySHA256: sha256Hex([]byte(body)), UpdatedAt: wire.UpdatedAt, HTMLURL: safeIssueURL(wire.HTMLURL), Untrusted: true}, true
}

func validRepository(repository Repository) bool {
	return repository.ID > 0 && validRepoPart(repository.Owner) && validRepoPart(repository.Name)
}

func ValidRepository(repository Repository) bool {
	return validRepository(repository)
}

func SourceConfigurationDigest(repository Repository) (string, error) {
	if !validRepository(repository) {
		return "", errors.New("invalid feedback repository")
	}
	configuration := struct {
		Provider        string `json:"provider"`
		ProfileRevision string `json:"profileRevision"`
		FilterRevision  string `json:"filterRevision"`
		RepositoryID    int64  `json:"repositoryId"`
		Owner           string `json:"owner"`
		Name            string `json:"name"`
	}{"github", ProfileRevision, FilterRevision, repository.ID, strings.ToLower(repository.Owner), strings.ToLower(repository.Name)}
	encoded, err := json.Marshal(configuration)
	if err != nil {
		return "", err
	}
	return sha256Hex(encoded), nil
}

func validRepoPart(value string) bool {
	return value != "" && value != "." && value != ".." && githubRepoPart.MatchString(value)
}

func validToken(token string) bool {
	return token != "" && len(token) <= 8192 && !strings.ContainsAny(token, "\r\n")
}

func isPullRequest(raw json.RawMessage) bool {
	trimmed := bytes.TrimSpace(raw)
	return len(trimmed) > 0 && !bytes.Equal(trimmed, []byte("null"))
}

func sha256Hex(value []byte) string {
	sum := sha256.Sum256(value)
	return hex.EncodeToString(sum[:])
}

func validTimestamp(value string) bool {
	if value == "" {
		return false
	}
	_, err := time.Parse(time.RFC3339, value)
	return err == nil
}

func safeIssueURL(value string) string {
	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme != "https" || !strings.EqualFold(parsed.Host, "github.com") || parsed.User != nil {
		return ""
	}
	return parsed.String()
}

func truncateUTF8(value string, maxBytes int) (string, bool) {
	if maxBytes <= 0 {
		return "", value != ""
	}
	if len(value) <= maxBytes {
		return value, false
	}
	end := maxBytes
	for end > 0 && !utf8.ValidString(value[:end]) {
		end--
	}
	return value[:end], true
}

func normalizeScanLimits(maxPages, maxItems int) (int, int, bool) {
	if maxPages == 0 {
		maxPages = defaultMaxPages
	}
	if maxItems == 0 {
		maxItems = defaultMaxIssues
	}
	return maxPages, maxItems, maxPages > 0 && maxPages <= maxAllowedPages && maxItems > 0 && maxItems <= maxAllowedIssues
}

func normalizeCommentLimits(options CommentOptions) (int, int, int, bool) {
	maxPages, maxComments, maxBytes := options.MaxPages, options.MaxComments, options.MaxBodyBytes
	if maxPages == 0 {
		maxPages = defaultMaxPages
	}
	if maxComments == 0 {
		maxComments = defaultMaxComments
	}
	if maxBytes == 0 {
		maxBytes = defaultMaxBodyBytes
	}
	return maxPages, maxComments, maxBytes, maxPages > 0 && maxPages <= maxAllowedPages && maxComments > 0 && maxComments <= maxAllowedComments && maxBytes > 0 && maxBytes <= maxAllowedBodyBytes
}

func wouldExceedIssueLimit(existing []Issue, indexes map[int64]int, next []Issue, limit int) bool {
	count := len(existing)
	for _, issue := range next {
		if _, exists := indexes[issue.ID]; !exists {
			count++
			if count > limit {
				return true
			}
		}
	}
	return false
}

func appendBoundedIssues(existing *[]Issue, indexes map[int64]int, next []Issue, limit int) {
	for _, issue := range next {
		if index, exists := indexes[issue.ID]; exists {
			if issue.UpdatedAt >= (*existing)[index].UpdatedAt {
				(*existing)[index] = issue
			}
			continue
		}
		if len(*existing) >= limit {
			return
		}
		indexes[issue.ID] = len(*existing)
		*existing = append(*existing, issue)
	}
}

func wouldExceedCommentLimit(existing []Comment, indexes map[int64]int, next []Comment, limit int) bool {
	count := len(existing)
	for _, comment := range next {
		if _, exists := indexes[comment.ID]; !exists {
			count++
			if count > limit {
				return true
			}
		}
	}
	return false
}

func appendBoundedComments(existing *[]Comment, indexes map[int64]int, next []Comment) {
	for _, comment := range next {
		if index, exists := indexes[comment.ID]; exists {
			if comment.UpdatedAt >= (*existing)[index].UpdatedAt {
				(*existing)[index] = comment
			}
			continue
		}
		indexes[comment.ID] = len(*existing)
		*existing = append(*existing, comment)
	}
}

func truncateCommentBodies(comments *[]Comment, maxBytes int) bool {
	sort.SliceStable(*comments, func(i, j int) bool {
		if (*comments)[i].UpdatedAt != (*comments)[j].UpdatedAt {
			return (*comments)[i].UpdatedAt < (*comments)[j].UpdatedAt
		}
		return (*comments)[i].ID < (*comments)[j].ID
	})
	remaining := maxBytes
	truncatedAny := false
	for index := range *comments {
		comment := &(*comments)[index]
		body, truncated := truncateUTF8(comment.Body, remaining)
		comment.Body = body
		comment.BodyTruncated = comment.BodyTruncated || truncated
		remaining -= len(body)
		truncatedAny = truncatedAny || truncated
	}
	return truncatedAny
}

func classifyStatus(status int, headers http.Header, body []byte) string {
	switch status {
	case http.StatusUnauthorized:
		return ReasonUnauthorized
	case http.StatusForbidden, http.StatusTooManyRequests:
		message := strings.ToLower(string(body))
		if headers.Get("Retry-After") != "" || headers.Get("X-RateLimit-Remaining") == "0" || status == http.StatusTooManyRequests || strings.Contains(message, "rate limit") {
			return ReasonRateLimited
		}
		return ReasonForbidden
	case http.StatusNotFound, http.StatusGone:
		return ReasonNotFoundOrHidden
	default:
		if status >= http.StatusInternalServerError {
			return ReasonUpstreamUnavailable
		}
		return ReasonUpstreamUnavailable
	}
}

func retryAfterSeconds(status int, headers http.Header, body []byte) int {
	seconds, err := strconv.Atoi(strings.TrimSpace(headers.Get("Retry-After")))
	if err == nil && seconds >= 0 {
		if seconds > 86400 {
			return 86400
		}
		return seconds
	}
	if retryAt, parseErr := http.ParseTime(headers.Get("Retry-After")); parseErr == nil {
		seconds = int(time.Until(retryAt).Seconds())
		if seconds < 0 {
			return 0
		}
		if seconds > 86400 {
			return 86400
		}
		return seconds
	}
	if headers.Get("X-RateLimit-Remaining") == "0" {
		reset, parseErr := strconv.ParseInt(headers.Get("X-RateLimit-Reset"), 10, 64)
		if parseErr == nil {
			remaining := int(time.Until(time.Unix(reset, 0)).Seconds())
			if remaining < 0 {
				return 0
			}
			if remaining > 86400 {
				return 86400
			}
			return remaining
		}
	}
	if status == http.StatusTooManyRequests || strings.Contains(strings.ToLower(string(body)), "rate limit") {
		return 60
	}
	return 0
}

// pattern: Imperative Shell
package control

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"polis/internal/core"
	githubfeedback "polis/internal/feedback/github"
	"polis/internal/kernel"
)

const (
	githubFeedbackOverlapWindow   = 24 * time.Hour
	githubFeedbackBootstrapWindow = 7 * 24 * time.Hour
	githubFeedbackPollTimeout     = 90 * time.Second
	githubFeedbackPollMaxPages    = 3
	githubFeedbackPollMaxIssues   = 300
	githubFeedbackCommentIssues   = 5
	GitHubFeedbackCredentialRef   = "default-readonly"
)

var ErrGitHubFeedbackUnavailable = errors.New("GitHub feedback provider is not configured")
var ErrGitHubFeedbackCredentialStoreUnavailable = githubfeedback.ErrProtectedGitHubCredentialStoreUnavailable

type GitHubFeedbackProvider interface {
	ScanIssues(context.Context, githubfeedback.Repository, githubfeedback.ScanOptions) githubfeedback.ScanResult
	ReadIssueComments(context.Context, githubfeedback.Repository, githubfeedback.Issue, githubfeedback.CommentOptions) githubfeedback.CommentResult
}

type GitHubFeedbackCommandService interface {
	RegisterGitHubFeedbackSource(context.Context, string, RegisterGitHubFeedbackSourceRequest) (GitHubFeedbackSourceView, error)
	ProbeGitHubFeedbackSource(context.Context, string, string, GitHubFeedbackProbeRequest) (GitHubFeedbackProbeReceipt, error)
	DecideGitHubFeedbackSource(context.Context, string, string, GitHubFeedbackDecisionRequest) (GitHubFeedbackSourceView, error)
	PollGitHubFeedbackSource(context.Context, string, string, GitHubFeedbackPollRequest) (GitHubFeedbackPollReceipt, error)
}

type GitHubFeedbackBacklogCommandService interface {
	SetGitHubFeedbackBacklogStatus(context.Context, string, GitHubFeedbackBacklogStatusRequest) (GitHubFeedbackBacklogStatusReceipt, error)
}

type GitHubFeedbackCollectionPolicyCommandService interface {
	SetGitHubFeedbackCollectionPolicy(context.Context, string, GitHubFeedbackCollectionPolicyRequest) (GitHubFeedbackCollectionPolicyReceipt, error)
}

type GitHubFeedbackCollectionPolicyRequest struct {
	SourceID        string `json:"sourceId"`
	Enabled         bool   `json:"enabled"`
	IntervalSeconds int    `json:"intervalSeconds"`
	Rationale       string `json:"rationale"`
	RequestID       string `json:"requestId"`
}

type GitHubFeedbackCollectionPolicyReceipt struct {
	CompanyID        string  `json:"companyId"`
	SourceID         string  `json:"sourceId"`
	Enabled          bool    `json:"enabled"`
	IntervalSeconds  int     `json:"intervalSeconds"`
	Rationale        string  `json:"rationale"`
	UpdatedAt        string  `json:"updatedAt"`
	NextPollAt       *string `json:"nextPollAt"`
	LastAttempt      string  `json:"lastAttempt"`
	LastReasonCode   string  `json:"lastReasonCode"`
	RequestID        string  `json:"requestId"`
	ExternalEnabled  bool    `json:"externalEnabled"`
	SchedulerEnabled bool    `json:"schedulerEnabled"`
}

type GitHubFeedbackBacklogStatusRequest struct {
	SourceID       string `json:"sourceId"`
	ProviderItemID int64  `json:"providerItemId,string"`
	RevisionSHA256 string `json:"revisionSha256"`
	Status         string `json:"status"`
	Rationale      string `json:"rationale"`
	RequestID      string `json:"requestId"`
}

type GitHubFeedbackBacklogStatusReceipt struct {
	CompanyID       string `json:"companyId"`
	SourceID        string `json:"sourceId"`
	ProviderItemID  int64  `json:"providerItemId,string"`
	IssueNumber     int    `json:"issueNumber"`
	RevisionSHA256  string `json:"revisionSha256"`
	Status          string `json:"status"`
	Rationale       string `json:"rationale"`
	EventID         string `json:"eventId"`
	RequestID       string `json:"requestId"`
	RemoteState     string `json:"remoteState"`
	RemoteUnchanged bool   `json:"remoteUnchanged"`
}

type GitHubFeedbackCredentialCommandService interface {
	StoreGitHubFeedbackCredential(context.Context, string) (GitHubFeedbackCredentialReceipt, error)
	DeleteGitHubFeedbackCredential(context.Context) (GitHubFeedbackCredentialReceipt, error)
}

type GitHubFeedbackCredentialReceipt struct {
	CredentialRef string `json:"credentialRef"`
	Stored        bool   `json:"stored"`
}

type RegisterGitHubFeedbackSourceRequest struct {
	SourceID     string `json:"sourceId"`
	RepositoryID int64  `json:"repositoryId"`
	Owner        string `json:"owner"`
	Name         string `json:"name"`
	RequestID    string `json:"requestId"`
}

type GitHubFeedbackProbeRequest struct {
	Rationale string `json:"rationale"`
	RequestID string `json:"requestId"`
}

type GitHubFeedbackDecisionRequest struct {
	Decision  string `json:"decision"`
	Rationale string `json:"rationale"`
	RequestID string `json:"requestId"`
}

type GitHubFeedbackPollRequest struct {
	RequestID string `json:"requestId"`
}

type GitHubFeedbackSourceView struct {
	CompanyID           string `json:"companyId"`
	SourceID            string `json:"sourceId"`
	RepositoryID        string `json:"repositoryId"`
	Repository          string `json:"repository"`
	ProfileRevision     string `json:"profileRevision"`
	FilterRevision      string `json:"filterRevision"`
	ConfigurationSHA256 string `json:"configurationSha256"`
	State               string `json:"state"`
	PermissionStatus    string `json:"permissionStatus"`
	PermissionProbeSHA  string `json:"permissionProbeSha256,omitempty"`
	CreatedAt           string `json:"createdAt"`
}

type GitHubFeedbackProbeReceipt struct {
	CompanyID        string `json:"companyId"`
	SourceID         string `json:"sourceId"`
	RequestID        string `json:"requestId"`
	PermissionStatus string `json:"permissionStatus"`
	Coverage         string `json:"coverage"`
	CoverageReason   string `json:"coverageReason"`
}

type GitHubFeedbackPollReceipt struct {
	CompanyID             string  `json:"companyId"`
	SourceID              string  `json:"sourceId"`
	RequestID             string  `json:"requestId"`
	ScanID                string  `json:"scanId"`
	Coverage              string  `json:"coverage"`
	CoverageReason        string  `json:"coverageReason"`
	CoveredThrough        *string `json:"coveredThrough"`
	PageCount             int     `json:"pageCount"`
	ItemCount             int     `json:"itemCount"`
	CommentScanCount      int     `json:"commentScanCount"`
	CommentCoverage       string  `json:"commentCoverage"`
	CommentCoverageReason string  `json:"commentCoverageReason"`
	Replayed              bool    `json:"replayed"`
}

// SetGitHubFeedbackTokenSource enables the fixed-host client. Callers must provide
// a source backed by protected storage; no token is persisted by Control.
func (s *Service) SetGitHubFeedbackTokenSource(source githubfeedback.TokenSource) {
	if source == nil {
		s.githubFeedback = nil
		return
	}
	s.githubFeedback = githubfeedback.NewClient(nil, source)
}

func (s *Service) SetGitHubFeedbackSchedulerEnabled(enabled bool) {
	if s != nil {
		s.githubFeedbackSchedulerEnabled = enabled
	}
}

func (s *Service) SetGitHubFeedbackCredentialStore(store githubfeedback.GitHubCredentialStore) {
	s.githubCredentials = store
}

func (s *Service) StoreGitHubFeedbackCredential(ctx context.Context, token string) (GitHubFeedbackCredentialReceipt, error) {
	if ctx.Err() != nil {
		return GitHubFeedbackCredentialReceipt{}, ctx.Err()
	}
	if !githubfeedback.ValidGitHubToken(token) {
		return GitHubFeedbackCredentialReceipt{}, core.Malformed
	}
	if s.githubCredentials == nil {
		return GitHubFeedbackCredentialReceipt{}, githubfeedback.ErrProtectedGitHubCredentialStoreUnavailable
	}
	if err := s.githubCredentials.StoreToken(GitHubFeedbackCredentialRef, token); err != nil {
		return GitHubFeedbackCredentialReceipt{}, githubfeedback.ErrProtectedGitHubCredentialStoreUnavailable
	}
	return GitHubFeedbackCredentialReceipt{CredentialRef: GitHubFeedbackCredentialRef, Stored: true}, nil
}

func (s *Service) DeleteGitHubFeedbackCredential(ctx context.Context) (GitHubFeedbackCredentialReceipt, error) {
	if ctx.Err() != nil {
		return GitHubFeedbackCredentialReceipt{}, ctx.Err()
	}
	if s.githubCredentials == nil {
		return GitHubFeedbackCredentialReceipt{}, githubfeedback.ErrProtectedGitHubCredentialStoreUnavailable
	}
	if err := s.githubCredentials.DeleteToken(GitHubFeedbackCredentialRef); err != nil {
		return GitHubFeedbackCredentialReceipt{}, githubfeedback.ErrProtectedGitHubCredentialStoreUnavailable
	}
	return GitHubFeedbackCredentialReceipt{CredentialRef: GitHubFeedbackCredentialRef, Stored: false}, nil
}

func (s *Service) RegisterGitHubFeedbackSource(ctx context.Context, companyID string, request RegisterGitHubFeedbackSourceRequest) (GitHubFeedbackSourceView, error) {
	if !core.ValidID(companyID) || validateRequestID(request.RequestID) != nil {
		return GitHubFeedbackSourceView{}, core.Malformed
	}
	source, err := s.runtime.TXRegisterGitHubFeedbackSource(ctx, companyID, kernel.GitHubFeedbackSourceInput{
		SourceID: request.SourceID, Repository: githubfeedback.Repository{ID: request.RepositoryID, Owner: request.Owner, Name: request.Name}, RequestID: request.RequestID,
	})
	if err != nil {
		return GitHubFeedbackSourceView{}, err
	}
	return feedbackSourceView(source), nil
}

func (s *Service) ProbeGitHubFeedbackSource(ctx context.Context, companyID, sourceID string, request GitHubFeedbackProbeRequest) (GitHubFeedbackProbeReceipt, error) {
	if !core.ValidID(companyID) || !core.ValidID(sourceID) || validateRequestID(request.RequestID) != nil || strings.TrimSpace(request.Rationale) == "" || len(request.Rationale) > 512 {
		return GitHubFeedbackProbeReceipt{}, core.Malformed
	}
	probeRequestID := derivedFeedbackProbeRequestID(request.RequestID)
	if previous, found, err := s.runtime.GetGitHubFeedbackSourceEventByRequest(ctx, companyID, probeRequestID); err != nil {
		return GitHubFeedbackProbeReceipt{}, err
	} else if found {
		if previous.SourceID != sourceID {
			return GitHubFeedbackProbeReceipt{}, core.Conflict
		}
		return GitHubFeedbackProbeReceipt{CompanyID: companyID, SourceID: sourceID, RequestID: request.RequestID, PermissionStatus: previous.PermissionStatus, Coverage: "replayed"}, nil
	}
	source, err := s.runtime.GetGitHubFeedbackSource(ctx, companyID, sourceID)
	if err != nil {
		return GitHubFeedbackProbeReceipt{}, err
	}
	if source.State == "revoked" {
		return GitHubFeedbackProbeReceipt{}, core.Denied
	}
	if s.githubFeedback == nil {
		return GitHubFeedbackProbeReceipt{}, ErrGitHubFeedbackUnavailable
	}
	if err = s.runtime.CheckRuntimeLease(ctx); err != nil {
		return GitHubFeedbackProbeReceipt{}, err
	}
	cutoff := time.Now().UTC()
	probe := s.githubFeedback.ScanIssues(ctx, source.Repository, githubfeedback.ScanOptions{CoverageCutoff: cutoff, MaxPages: 1, MaxItems: 100})
	if err = s.runtime.TXRecordGitHubFeedbackPermissionProbe(ctx, companyID, kernel.GitHubFeedbackPermissionInput{
		SourceID: sourceID, Probe: probe, Rationale: request.Rationale, RequestID: probeRequestID,
	}); err != nil {
		return GitHubFeedbackProbeReceipt{}, err
	}
	updated, err := s.runtime.GetGitHubFeedbackSource(ctx, companyID, sourceID)
	if err != nil {
		return GitHubFeedbackProbeReceipt{}, err
	}
	return GitHubFeedbackProbeReceipt{CompanyID: companyID, SourceID: sourceID, RequestID: request.RequestID, PermissionStatus: updated.PermissionStatus, Coverage: string(probe.Coverage), CoverageReason: probe.CoverageReason}, nil
}

func (s *Service) DecideGitHubFeedbackSource(ctx context.Context, companyID, sourceID string, request GitHubFeedbackDecisionRequest) (GitHubFeedbackSourceView, error) {
	if !core.ValidID(companyID) || !core.ValidID(sourceID) || validateRequestID(request.RequestID) != nil {
		return GitHubFeedbackSourceView{}, core.Malformed
	}
	if err := s.runtime.TXDecideGitHubFeedbackSource(ctx, companyID, sourceID, kernel.GitHubFeedbackSourceDecisionInput{
		Decision: request.Decision, Rationale: request.Rationale, RequestID: request.RequestID,
	}); err != nil {
		return GitHubFeedbackSourceView{}, err
	}
	source, err := s.runtime.GetGitHubFeedbackSource(ctx, companyID, sourceID)
	if err != nil {
		return GitHubFeedbackSourceView{}, err
	}
	return feedbackSourceView(source), nil
}

func (s *Service) PollGitHubFeedbackSource(ctx context.Context, companyID, sourceID string, request GitHubFeedbackPollRequest) (GitHubFeedbackPollReceipt, error) {
	if !core.ValidID(companyID) || !core.ValidID(sourceID) || validateRequestID(request.RequestID) != nil {
		return GitHubFeedbackPollReceipt{}, core.Malformed
	}
	if previous, found, err := s.runtime.GetGitHubFeedbackScanByRequest(ctx, companyID, request.RequestID); err != nil {
		return GitHubFeedbackPollReceipt{}, err
	} else if found {
		if previous.SourceID != sourceID {
			return GitHubFeedbackPollReceipt{}, core.Conflict
		}
		receipt := pollReceiptFromRecord(previous, request.RequestID, true)
		commentCount, commentsPartial, reason, summaryErr := s.runtime.GitHubFeedbackCommentsForPollRequest(ctx, companyID, request.RequestID)
		if summaryErr != nil {
			return GitHubFeedbackPollReceipt{}, summaryErr
		}
		receipt.CommentScanCount = commentCount
		switch {
		case commentCount == 0 && previous.ItemCount == 0:
			receipt.CommentCoverage = "not_read"
		case commentCount == 0:
			receipt.CommentCoverage = "unknown"
		case commentsPartial:
			receipt.CommentCoverage = "partial"
			receipt.CommentCoverageReason = reason
		default:
			receipt.CommentCoverage = "complete"
		}
		return receipt, nil
	}
	source, err := s.runtime.GetGitHubFeedbackSource(ctx, companyID, sourceID)
	if err != nil {
		return GitHubFeedbackPollReceipt{}, err
	}
	if source.State != "approved" || source.PermissionStatus != "verified" {
		return GitHubFeedbackPollReceipt{}, core.Denied
	}
	if s.githubFeedback == nil {
		return GitHubFeedbackPollReceipt{}, ErrGitHubFeedbackUnavailable
	}
	pollCtx, cancel := context.WithTimeout(ctx, githubFeedbackPollTimeout)
	defer cancel()
	if err = s.runtime.CheckRuntimeLease(pollCtx); err != nil {
		return GitHubFeedbackPollReceipt{}, err
	}
	cutoff := time.Now().UTC()
	since := cutoff.Add(-githubFeedbackBootstrapWindow)
	if cursor, found, cursorErr := s.runtime.LatestGitHubFeedbackCursor(pollCtx, companyID, sourceID); cursorErr != nil {
		return GitHubFeedbackPollReceipt{}, cursorErr
	} else if found {
		since = cursor.Add(-githubFeedbackOverlapWindow)
	}
	scan := s.githubFeedback.ScanIssues(pollCtx, source.Repository, githubfeedback.ScanOptions{
		Since: since, CoverageCutoff: cutoff, MaxPages: githubFeedbackPollMaxPages, MaxItems: githubFeedbackPollMaxIssues,
	})
	record, err := s.runtime.TXRecordGitHubFeedbackScan(pollCtx, companyID, sourceID, scan, request.RequestID)
	if err != nil {
		return GitHubFeedbackPollReceipt{}, err
	}
	receipt := pollReceiptFromRecord(record, request.RequestID, false)
	receipt.CommentCoverage = "not_read"
	if len(scan.Issues) == 0 {
		return receipt, nil
	}
	start := len(scan.Issues) - githubFeedbackCommentIssues
	if start < 0 {
		start = 0
	}
	commentCoverage := "complete"
	for _, issue := range scan.Issues[start:] {
		if err := pollCtx.Err(); err != nil {
			return receipt, err
		}
		comments := s.githubFeedback.ReadIssueComments(pollCtx, source.Repository, issue, githubfeedback.CommentOptions{
			Since: issueUpdatedAt(issue), MaxPages: 1, MaxComments: 100, MaxBodyBytes: 64 << 10,
		})
		if feedbackPermissionDenied(comments.CoverageReason) {
			probeID := derivedFeedbackRequestID(request.RequestID, issue.ID, "permission")
			_ = s.runtime.TXRecordGitHubFeedbackPermissionProbe(pollCtx, companyID, kernel.GitHubFeedbackPermissionInput{
				SourceID: sourceID, Probe: githubfeedback.ScanResult{Repository: source.Repository, Coverage: githubfeedback.CoveragePartial, CoverageReason: comments.CoverageReason},
				Rationale: "comment permission was denied during bounded read-only polling", RequestID: probeID,
			})
			commentCoverage = "partial"
			receipt.CommentCoverageReason = comments.CoverageReason
			break
		}
		commentRequestID := derivedFeedbackRequestID(request.RequestID, issue.ID, "comments")
		stored, recordErr := s.runtime.TXRecordGitHubFeedbackCommentScan(pollCtx, companyID, kernel.GitHubFeedbackCommentInput{
			SourceID: sourceID, Result: comments, RequestID: commentRequestID,
		})
		if recordErr != nil {
			return receipt, recordErr
		}
		receipt.CommentScanCount++
		if stored.Coverage != string(githubfeedback.CoverageComplete) {
			commentCoverage = "partial"
			if receipt.CommentCoverageReason == "" {
				receipt.CommentCoverageReason = stored.CoverageReason
			}
		}
	}
	receipt.CommentCoverage = commentCoverage
	return receipt, nil
}

func (s *Service) SetGitHubFeedbackBacklogStatus(ctx context.Context, companyID string, request GitHubFeedbackBacklogStatusRequest) (GitHubFeedbackBacklogStatusReceipt, error) {
	if !core.ValidID(companyID) || !core.ValidID(request.SourceID) || request.ProviderItemID <= 0 || len(request.RevisionSHA256) != 64 || validateRequestID(request.RequestID) != nil || strings.TrimSpace(request.Rationale) == "" || len(request.Rationale) > 512 {
		return GitHubFeedbackBacklogStatusReceipt{}, core.Malformed
	}
	request.Rationale = strings.TrimSpace(request.Rationale)
	event, err := s.runtime.TXSetGitHubFeedbackBacklogStatus(ctx, companyID, kernel.GitHubFeedbackBacklogDecisionInput{
		SourceID: request.SourceID, ProviderItemID: request.ProviderItemID, RevisionSHA256: request.RevisionSHA256, Status: request.Status,
		Rationale: request.Rationale, RequestID: request.RequestID,
	})
	if err != nil {
		return GitHubFeedbackBacklogStatusReceipt{}, err
	}
	item, err := s.runtime.GetGitHubFeedbackBacklogItem(ctx, companyID, request.SourceID, request.ProviderItemID)
	if err != nil {
		return GitHubFeedbackBacklogStatusReceipt{}, err
	}
	return GitHubFeedbackBacklogStatusReceipt{
		CompanyID: companyID, SourceID: request.SourceID, ProviderItemID: request.ProviderItemID,
		IssueNumber: item.IssueNumber, RevisionSHA256: event.RevisionSHA256, Status: event.Status,
		Rationale: event.Rationale, EventID: event.EventID, RequestID: request.RequestID, RemoteState: item.RemoteState, RemoteUnchanged: true,
	}, nil
}

func (s *Service) SetGitHubFeedbackCollectionPolicy(ctx context.Context, companyID string, request GitHubFeedbackCollectionPolicyRequest) (GitHubFeedbackCollectionPolicyReceipt, error) {
	if !core.ValidID(companyID) || !core.ValidID(request.SourceID) || validateRequestID(request.RequestID) != nil || len(strings.TrimSpace(request.Rationale)) == 0 || len(strings.TrimSpace(request.Rationale)) > 512 {
		return GitHubFeedbackCollectionPolicyReceipt{}, core.Malformed
	}
	policy, err := s.runtime.TXSetGitHubFeedbackCollectionPolicy(ctx, companyID, kernel.GitHubFeedbackCollectionPolicyInput{
		SourceID: request.SourceID, Enabled: request.Enabled, IntervalSeconds: request.IntervalSeconds,
		Rationale: request.Rationale, RequestID: request.RequestID,
	})
	if err != nil {
		return GitHubFeedbackCollectionPolicyReceipt{}, err
	}
	return feedbackCollectionPolicyReceipt(policy, request.RequestID, s.githubFeedback != nil, s.githubFeedbackSchedulerEnabled), nil
}

func feedbackCollectionPolicyReceipt(policy kernel.GitHubFeedbackCollectionPolicy, requestID string, externalEnabled, schedulerEnabled bool) GitHubFeedbackCollectionPolicyReceipt {
	return GitHubFeedbackCollectionPolicyReceipt{
		CompanyID: policy.CompanyID, SourceID: policy.SourceID, Enabled: policy.Enabled, IntervalSeconds: policy.IntervalSeconds,
		Rationale: policy.Rationale, UpdatedAt: policy.UpdatedAt, NextPollAt: policy.NextPollAt,
		LastAttempt: policy.LastAttempt, LastReasonCode: policy.LastReasonCode, RequestID: requestID, ExternalEnabled: externalEnabled, SchedulerEnabled: schedulerEnabled,
	}
}

func feedbackSourceView(source kernel.GitHubFeedbackSource) GitHubFeedbackSourceView {
	return GitHubFeedbackSourceView{
		CompanyID: source.CompanyID, SourceID: source.SourceID, RepositoryID: fmt.Sprint(source.Repository.ID),
		Repository: source.Repository.Owner + "/" + source.Repository.Name, ProfileRevision: source.ProfileRevision,
		FilterRevision: source.FilterRevision, ConfigurationSHA256: source.ConfigurationSHA256, State: source.State,
		PermissionStatus: source.PermissionStatus, PermissionProbeSHA: source.PermissionProbeSHA256, CreatedAt: source.CreatedAt,
	}
}

func pollReceiptFromRecord(record kernel.GitHubFeedbackScanRecord, requestID string, replayed bool) GitHubFeedbackPollReceipt {
	var coveredThrough *string
	if record.CoveredThrough != nil {
		value := record.CoveredThrough.UTC().Format(time.RFC3339Nano)
		coveredThrough = &value
	}
	return GitHubFeedbackPollReceipt{
		CompanyID: record.CompanyID, SourceID: record.SourceID, RequestID: requestID, ScanID: record.ScanID,
		Coverage: record.Coverage, CoverageReason: record.CoverageReason, CoveredThrough: coveredThrough,
		PageCount: record.PageCount, ItemCount: record.ItemCount, CommentCoverage: "not_started", Replayed: replayed,
	}
}

func derivedFeedbackRequestID(parent string, issueID int64, kind string) string {
	digest := sha256.Sum256([]byte(parent))
	return fmt.Sprintf("ghfb-%s-%d-%s", hex.EncodeToString(digest[:16]), issueID, kind)
}

func derivedFeedbackProbeRequestID(parent string) string {
	digest := sha256.Sum256([]byte(parent))
	return "ghprobe-" + hex.EncodeToString(digest[:16])
}

func issueUpdatedAt(issue githubfeedback.Issue) time.Time {
	parsed, err := time.Parse(time.RFC3339, issue.UpdatedAt)
	if err != nil {
		return time.Time{}
	}
	return parsed.UTC()
}

func feedbackPermissionDenied(reason string) bool {
	return reason == githubfeedback.ReasonUnauthorized || reason == githubfeedback.ReasonForbidden || reason == githubfeedback.ReasonNotFoundOrHidden
}

var _ GitHubFeedbackCommandService = (*Service)(nil)
var _ GitHubFeedbackBacklogCommandService = (*Service)(nil)
var _ GitHubFeedbackCollectionPolicyCommandService = (*Service)(nil)
var _ GitHubFeedbackCredentialCommandService = (*Service)(nil)

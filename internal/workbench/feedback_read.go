// pattern: Imperative Shell
package workbench

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5"
	"polis/internal/core"
)

const (
	FeedbackIssueViewLimit       = 20
	FeedbackCommentViewLimit     = 5
	FeedbackIssueTitleViewBytes  = 4 << 10
	FeedbackIssueTitleViewChars  = FeedbackIssueTitleViewBytes / 4
	FeedbackIssueBodyViewBytes   = 16 << 10
	FeedbackIssueBodyViewChars   = FeedbackIssueBodyViewBytes / 4
	FeedbackCommentBodyViewBytes = 4 << 10
	FeedbackCommentBodyViewChars = FeedbackCommentBodyViewBytes / 4
)

type CompanyFeedbackView struct {
	CompanyID string               `json:"companyId"`
	Sources   []FeedbackSourceView `json:"sources"`
	Issues    []FeedbackIssueView  `json:"issues"`
}

type FeedbackSourceView struct {
	SourceID                  string  `json:"sourceId"`
	Provider                  string  `json:"provider"`
	RepositoryID              string  `json:"repositoryId"`
	Repository                string  `json:"repository"`
	ProfileRevision           string  `json:"profileRevision"`
	State                     string  `json:"state"`
	PermissionStatus          string  `json:"permissionStatus"`
	Coverage                  string  `json:"coverage"`
	CoverageReason            string  `json:"coverageReason"`
	CoveredThrough            *string `json:"coveredThrough"`
	LastScanAt                *string `json:"lastScanAt"`
	CollectionEnabled         bool    `json:"collectionEnabled"`
	CollectionIntervalSeconds int     `json:"collectionIntervalSeconds"`
	CollectionRationale       string  `json:"collectionRationale"`
	CollectionNextPollAt      *string `json:"collectionNextPollAt"`
	CollectionLastAttempt     string  `json:"collectionLastAttempt"`
	CollectionLastReasonCode  string  `json:"collectionLastReasonCode"`
}

type FeedbackCommentView struct {
	CommentID       string `json:"commentId"`
	RevisionSHA256  string `json:"revisionSha256"`
	Body            string `json:"body"`
	BodySHA256      string `json:"bodySha256"`
	BodyTruncated   bool   `json:"bodyTruncated"`
	SourceUpdatedAt string `json:"sourceUpdatedAt"`
	HTMLURL         string `json:"htmlUrl"`
}

type FeedbackIssueView struct {
	SourceID              string                `json:"sourceId"`
	ProviderItemID        string                `json:"providerItemId"`
	IssueNumber           string                `json:"issueNumber"`
	RevisionSHA256        string                `json:"revisionSha256"`
	BacklogStatus         string                `json:"backlogStatus"`
	BacklogReason         string                `json:"backlogReason"`
	BacklogUpdatedAt      string                `json:"backlogUpdatedAt"`
	Title                 string                `json:"title"`
	TitleTruncated        bool                  `json:"titleTruncated"`
	Body                  string                `json:"body"`
	BodySHA256            string                `json:"bodySha256"`
	BodyTruncated         bool                  `json:"bodyTruncated"`
	State                 string                `json:"state"`
	SourceUpdatedAt       string                `json:"sourceUpdatedAt"`
	ObservedAt            string                `json:"observedAt"`
	HTMLURL               string                `json:"htmlUrl"`
	CommentCoverage       string                `json:"commentCoverage"`
	CommentCoverageReason string                `json:"commentCoverageReason"`
	CommentCount          string                `json:"commentCount"`
	CommentContextPartial bool                  `json:"commentContextPartial"`
	Comments              []FeedbackCommentView `json:"comments"`
}

type FeedbackReader interface {
	GetCompanyFeedback(ctx context.Context, companyID string) (CompanyFeedbackView, error)
}

func (s *PostgresReadStore) GetCompanyFeedback(ctx context.Context, companyID string) (CompanyFeedbackView, error) {
	if err := validateCompanyID(companyID); err != nil {
		return CompanyFeedbackView{}, err
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return CompanyFeedbackView{}, err
	}
	defer tx.Rollback(ctx)
	view := CompanyFeedbackView{CompanyID: companyID, Sources: []FeedbackSourceView{}, Issues: []FeedbackIssueView{}}
	sourceRows, err := tx.Query(ctx, `SELECT b.source_id,b.provider,b.repository_id::text,b.repository_owner || '/' || b.repository_name,b.profile_revision,
COALESCE(event.state,'draft'),COALESCE(event.permission_status,'unverified'),
COALESCE(scan.coverage_status,'never'),COALESCE(scan.coverage_reason,''),scan.covered_through::text,scan.created_at::text,
COALESCE(policy.enabled,false),COALESCE(policy.interval_seconds,0),COALESCE(policy.rationale,''),schedule.next_poll_at::text,
COALESCE(attempt.status,''),COALESCE(attempt.reason_code,'')
FROM feedback_source_bindings b
LEFT JOIN LATERAL (
 SELECT state,permission_status FROM feedback_source_events
 WHERE company_id=b.company_id AND source_id=b.source_id ORDER BY event_seq DESC LIMIT 1
) event ON true
LEFT JOIN LATERAL (
 SELECT coverage_status,coverage_reason,covered_through,created_at FROM feedback_scan_runs
 WHERE company_id=b.company_id AND source_id=b.source_id ORDER BY coverage_cutoff DESC,created_at DESC LIMIT 1
) scan ON true
LEFT JOIN LATERAL (
 SELECT enabled,interval_seconds,rationale FROM feedback_collection_policy_events
 WHERE company_id=b.company_id AND source_id=b.source_id ORDER BY event_seq DESC LIMIT 1
) policy ON true
LEFT JOIN feedback_collection_schedule_state schedule USING(company_id,source_id)
LEFT JOIN LATERAL (
 SELECT status,reason_code FROM feedback_collection_poll_attempts
 WHERE company_id=b.company_id AND source_id=b.source_id ORDER BY scheduled_for DESC LIMIT 1
) attempt ON true
WHERE b.company_id=$1
ORDER BY b.created_at DESC,b.source_id
LIMIT 50`, companyID)
	if err != nil {
		return CompanyFeedbackView{}, err
	}
	for sourceRows.Next() {
		var item FeedbackSourceView
		var coveredThrough, lastScan, collectionNextPoll sql.NullString
		if err = sourceRows.Scan(&item.SourceID, &item.Provider, &item.RepositoryID, &item.Repository, &item.ProfileRevision, &item.State, &item.PermissionStatus, &item.Coverage, &item.CoverageReason, &coveredThrough, &lastScan,
			&item.CollectionEnabled, &item.CollectionIntervalSeconds, &item.CollectionRationale, &collectionNextPoll, &item.CollectionLastAttempt, &item.CollectionLastReasonCode); err != nil {
			sourceRows.Close()
			return CompanyFeedbackView{}, err
		}
		if coveredThrough.Valid {
			item.CoveredThrough = &coveredThrough.String
		}
		if lastScan.Valid {
			item.LastScanAt = &lastScan.String
		}
		if collectionNextPoll.Valid {
			item.CollectionNextPollAt = &collectionNextPoll.String
		}
		view.Sources = append(view.Sources, item)
	}
	if err = sourceRows.Err(); err != nil {
		sourceRows.Close()
		return CompanyFeedbackView{}, err
	}
	sourceRows.Close()
	issueRows, err := tx.Query(ctx, `WITH latest AS (
 SELECT DISTINCT ON(source_id,provider_item_id) *
 FROM feedback_observations
 WHERE company_id=$1
 ORDER BY source_id,provider_item_id,source_updated_at DESC,observed_at DESC,revision_sha256 DESC
)
SELECT o.source_id,o.provider_item_id::text,o.issue_number::text,o.revision_sha256,
CASE WHEN backlog.event_seq IS NULL THEN 'open' WHEN backlog.revision_sha256<>o.revision_sha256 THEN 'needs_review' ELSE backlog.status END,
CASE WHEN backlog.event_seq IS NULL THEN 'new_issue_observed' WHEN backlog.revision_sha256<>o.revision_sha256 THEN 'source_revision_updated' ELSE backlog.reason_code END,
CASE WHEN backlog.event_seq IS NULL THEN o.observed_at::text ELSE backlog.created_at::text END,
 left(o.title,$2),o.title_truncated OR char_length(o.title)>$2,
 left(o.body,$3),o.body_sha256,o.body_truncated OR char_length(o.body)>$3,
o.issue_state,o.source_updated_at::text,o.observed_at::text,o.html_url,
COALESCE(scan.coverage_status,'not_read'),COALESCE(scan.coverage_reason,''),COALESCE(scan.comment_count,0)::text,
COALESCE(scan.coverage_status='partial' OR scan.body_truncated OR scan.comment_count>$5 OR comments.body_truncated,false),
COALESCE(comments.items,'[]'::jsonb)
FROM latest o
LEFT JOIN LATERAL (
 SELECT event_seq,revision_sha256,status,reason_code,created_at FROM feedback_backlog_events
 WHERE company_id=o.company_id AND source_id=o.source_id AND provider_item_id=o.provider_item_id
 ORDER BY event_seq DESC LIMIT 1
) backlog ON true
LEFT JOIN LATERAL (
 SELECT comment_scan_id,coverage_status,coverage_reason,comment_count,body_truncated
 FROM feedback_comment_scans
 WHERE company_id=o.company_id AND source_id=o.source_id AND provider_item_id=o.provider_item_id AND issue_revision_sha256=o.revision_sha256
 ORDER BY created_at DESC LIMIT 1
) scan ON true
LEFT JOIN LATERAL (
 SELECT COALESCE(jsonb_agg(jsonb_build_object(
  'commentId',c.provider_comment_id::text,'revisionSha256',c.revision_sha256,'body',left(c.body,$4),
  'bodySha256',c.body_sha256,'bodyTruncated',c.body_truncated OR char_length(c.body)>$4,
  'sourceUpdatedAt',c.source_updated_at::text,'htmlUrl',c.html_url
 ) ORDER BY c.source_updated_at DESC,c.provider_comment_id),'[]'::jsonb) AS items,
 COALESCE(bool_or(c.body_truncated OR char_length(c.body)>$4),false) AS body_truncated
 FROM (
  SELECT co.* FROM feedback_comment_scan_items item
  JOIN feedback_comment_observations co
    ON co.company_id=item.company_id AND co.source_id=item.source_id AND co.provider_item_id=item.provider_item_id
   AND co.issue_revision_sha256=item.issue_revision_sha256 AND co.provider_comment_id=item.provider_comment_id
   AND co.revision_sha256=item.comment_revision_sha256
  WHERE item.company_id=o.company_id AND item.source_id=o.source_id AND item.provider_item_id=o.provider_item_id
    AND item.issue_revision_sha256=o.revision_sha256 AND item.comment_scan_id=scan.comment_scan_id
  ORDER BY co.source_updated_at DESC,co.provider_comment_id
  LIMIT $5
 ) c
) comments ON scan.comment_scan_id IS NOT NULL
	ORDER BY o.source_updated_at DESC,o.source_id,o.issue_number DESC
	LIMIT $6`, companyID, FeedbackIssueTitleViewChars, FeedbackIssueBodyViewChars, FeedbackCommentBodyViewChars, FeedbackCommentViewLimit, FeedbackIssueViewLimit)
	if err != nil {
		return CompanyFeedbackView{}, err
	}
	for issueRows.Next() {
		var item FeedbackIssueView
		var commentsJSON []byte
		if err = issueRows.Scan(&item.SourceID, &item.ProviderItemID, &item.IssueNumber, &item.RevisionSHA256, &item.BacklogStatus, &item.BacklogReason, &item.BacklogUpdatedAt, &item.Title, &item.TitleTruncated, &item.Body, &item.BodySHA256, &item.BodyTruncated, &item.State, &item.SourceUpdatedAt, &item.ObservedAt, &item.HTMLURL, &item.CommentCoverage, &item.CommentCoverageReason, &item.CommentCount, &item.CommentContextPartial, &commentsJSON); err != nil {
			issueRows.Close()
			return CompanyFeedbackView{}, err
		}
		if err = json.Unmarshal(commentsJSON, &item.Comments); err != nil {
			issueRows.Close()
			return CompanyFeedbackView{}, core.Integrity
		}
		if item.Comments == nil {
			item.Comments = []FeedbackCommentView{}
		}
		view.Issues = append(view.Issues, item)
	}
	if err = issueRows.Err(); err != nil {
		issueRows.Close()
		return CompanyFeedbackView{}, err
	}
	issueRows.Close()
	if err = tx.Commit(ctx); err != nil {
		return CompanyFeedbackView{}, fmt.Errorf("commit company feedback snapshot: %w", err)
	}
	return view, nil
}

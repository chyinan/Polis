// pattern: Imperative Shell
package kernel

import (
	"context"
	"errors"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	"polis/internal/core"
)

const (
	GitHubFeedbackBacklogOpen        = "open"
	GitHubFeedbackBacklogNeedsReview = "needs_review"
	GitHubFeedbackBacklogTriaging    = "triaging"
	GitHubFeedbackBacklogWaiting     = "waiting"
	GitHubFeedbackBacklogHandled     = "handled"
	GitHubFeedbackBacklogArchived    = "archived"
)

type GitHubFeedbackBacklogItem struct {
	CompanyID       string
	SourceID        string
	ProviderItemID  int64
	IssueNumber     int
	RevisionSHA256  string
	RemoteState     string
	Status          string
	StatusReason    string
	StatusUpdatedAt string
}

type GitHubFeedbackBacklogDecisionInput struct {
	SourceID       string
	ProviderItemID int64
	RevisionSHA256 string
	Status         string
	Rationale      string
	RequestID      string
}

type GitHubFeedbackBacklogDecision struct {
	EventID        string
	CompanyID      string
	SourceID       string
	ProviderItemID int64
	RevisionSHA256 string
	Status         string
	Rationale      string
	CreatedAt      string
}

func (k *Kernel) TXSetGitHubFeedbackBacklogStatus(ctx context.Context, companyID string, input GitHubFeedbackBacklogDecisionInput) (GitHubFeedbackBacklogDecision, error) {
	if !core.ValidID(companyID) || !core.ValidID(input.SourceID) || input.ProviderItemID <= 0 || !validFeedbackSHA256(input.RevisionSHA256) || !validGitHubFeedbackBacklogStatus(input.Status) || !core.ValidID(input.RequestID) || strings.TrimSpace(input.Rationale) == "" || len(input.Rationale) > 512 {
		return GitHubFeedbackBacklogDecision{}, core.Malformed
	}
	input.Rationale = strings.TrimSpace(input.Rationale)
	eventID := stableCapabilityID("gh-backlog-event", companyID, input.RequestID)
	_, err := k.TXWrite(ctx, k.LocalScope(companyID), nil, input.RequestID, "feedback.github.backlog.decide", input, func(tx pgx.Tx) (Receipt, error) {
		if err := ensureEnvironmentCompanyWritable(ctx, tx, companyID); err != nil {
			return Receipt{}, err
		}
		var boundSourceID string
		err := tx.QueryRow(ctx, `SELECT source_id FROM feedback_source_bindings WHERE company_id=$1 AND source_id=$2 FOR UPDATE`, companyID, input.SourceID).Scan(&boundSourceID)
		if errors.Is(err, pgx.ErrNoRows) {
			return Receipt{}, core.OutOfScope
		}
		if err != nil {
			return Receipt{}, err
		}
		var revisionSHA256 string
		err = tx.QueryRow(ctx, `SELECT revision_sha256 FROM feedback_observations
WHERE company_id=$1 AND source_id=$2 AND provider_item_id=$3
ORDER BY source_updated_at DESC,observed_at DESC,revision_sha256 DESC LIMIT 1 FOR UPDATE`, companyID, input.SourceID, input.ProviderItemID).Scan(&revisionSHA256)
		if errors.Is(err, pgx.ErrNoRows) {
			return Receipt{}, core.OutOfScope
		}
		if err != nil {
			return Receipt{}, err
		}
		if revisionSHA256 != input.RevisionSHA256 {
			return Receipt{}, core.Conflict
		}
		_, err = tx.Exec(ctx, `INSERT INTO feedback_backlog_events(company_id,event_id,source_id,provider_item_id,revision_sha256,status,reason_code,rationale,actor,request_id)
VALUES($1,$2,$3,$4,$5,$6,'owner_decision',$7,'local-owner',$8)`, companyID, eventID, input.SourceID, input.ProviderItemID, revisionSHA256, input.Status, input.Rationale, input.RequestID)
		if err != nil {
			if isUniqueViolation(err) {
				return Receipt{}, core.Conflict
			}
			return Receipt{}, err
		}
		return Receipt{ID: eventID, Status: input.Status}, nil
	})
	if err != nil {
		return GitHubFeedbackBacklogDecision{}, err
	}
	var result GitHubFeedbackBacklogDecision
	err = k.pool.QueryRow(ctx, `SELECT event_id,company_id,source_id,provider_item_id,revision_sha256,status,rationale,created_at::text
FROM feedback_backlog_events WHERE company_id=$1 AND event_id=$2`, companyID, eventID).Scan(&result.EventID, &result.CompanyID, &result.SourceID, &result.ProviderItemID, &result.RevisionSHA256, &result.Status, &result.Rationale, &result.CreatedAt)
	return result, err
}

func (k *Kernel) GetGitHubFeedbackBacklogItem(ctx context.Context, companyID, sourceID string, providerItemID int64) (GitHubFeedbackBacklogItem, error) {
	if !core.ValidID(companyID) || !core.ValidID(sourceID) || providerItemID <= 0 {
		return GitHubFeedbackBacklogItem{}, core.Malformed
	}
	var item GitHubFeedbackBacklogItem
	var statusUpdatedAt string
	err := k.pool.QueryRow(ctx, `SELECT o.company_id,o.source_id,o.provider_item_id,o.issue_number,o.revision_sha256,o.issue_state,
CASE WHEN event.event_seq IS NULL THEN 'open' WHEN event.revision_sha256<>o.revision_sha256 THEN 'needs_review' ELSE event.status END,
CASE WHEN event.event_seq IS NULL THEN 'new_issue_observed' WHEN event.revision_sha256<>o.revision_sha256 THEN 'source_revision_updated' ELSE event.reason_code END,
CASE WHEN event.event_seq IS NULL THEN o.observed_at::text ELSE event.created_at::text END
FROM feedback_observations o
LEFT JOIN LATERAL (
 SELECT event_seq,revision_sha256,status,reason_code,created_at FROM feedback_backlog_events
 WHERE company_id=o.company_id AND source_id=o.source_id AND provider_item_id=o.provider_item_id
 ORDER BY event_seq DESC LIMIT 1
) event ON true
WHERE o.company_id=$1 AND o.source_id=$2 AND o.provider_item_id=$3
ORDER BY o.source_updated_at DESC,o.observed_at DESC,o.revision_sha256 DESC LIMIT 1`, companyID, sourceID, providerItemID).Scan(
		&item.CompanyID, &item.SourceID, &item.ProviderItemID, &item.IssueNumber, &item.RevisionSHA256, &item.RemoteState,
		&item.Status, &item.StatusReason, &statusUpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return GitHubFeedbackBacklogItem{}, core.OutOfScope
	}
	if err != nil {
		return GitHubFeedbackBacklogItem{}, err
	}
	item.StatusUpdatedAt = statusUpdatedAt
	return item, nil
}

func (k *Kernel) ensureGitHubFeedbackBacklogObservation(ctx context.Context, tx pgx.Tx, companyID, sourceID string, providerItemID int64) error {
	var revisionSHA256 string
	err := tx.QueryRow(ctx, `SELECT revision_sha256 FROM feedback_observations
WHERE company_id=$1 AND source_id=$2 AND provider_item_id=$3
ORDER BY source_updated_at DESC,observed_at DESC,revision_sha256 DESC LIMIT 1`, companyID, sourceID, providerItemID).Scan(&revisionSHA256)
	if errors.Is(err, pgx.ErrNoRows) {
		return core.Integrity
	}
	if err != nil {
		return err
	}
	var previousRevision string
	err = tx.QueryRow(ctx, `SELECT revision_sha256 FROM feedback_backlog_events
WHERE company_id=$1 AND source_id=$2 AND provider_item_id=$3
ORDER BY event_seq DESC LIMIT 1`, companyID, sourceID, providerItemID).Scan(&previousRevision)
	if err == nil && previousRevision == revisionSHA256 {
		return nil
	}
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	requestID := stableCapabilityID("gh-backlog-observed", companyID, sourceID, strconv.FormatInt(providerItemID, 10), revisionSHA256)
	eventID := stableCapabilityID("gh-backlog-event", companyID, requestID)
	status, reason := GitHubFeedbackBacklogOpen, "new_issue_observed"
	if err == nil {
		status, reason = GitHubFeedbackBacklogNeedsReview, "source_revision_updated"
	}
	_, err = tx.Exec(ctx, `INSERT INTO feedback_backlog_events(company_id,event_id,source_id,provider_item_id,revision_sha256,status,reason_code,rationale,actor,request_id)
VALUES($1,$2,$3,$4,$5,$6,$7,'','system',$8) ON CONFLICT(company_id,request_id) DO NOTHING`, companyID, eventID, sourceID, providerItemID, revisionSHA256, status, reason, requestID)
	return err
}

func validGitHubFeedbackBacklogStatus(status string) bool {
	switch status {
	case GitHubFeedbackBacklogOpen, GitHubFeedbackBacklogTriaging, GitHubFeedbackBacklogWaiting, GitHubFeedbackBacklogHandled, GitHubFeedbackBacklogArchived:
		return true
	default:
		return false
	}
}

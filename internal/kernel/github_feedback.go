// pattern: Imperative Shell
package kernel

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"regexp"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"polis/internal/core"
	githubfeedback "polis/internal/feedback/github"
)

const (
	GitHubFeedbackProfileRevision = githubfeedback.ProfileRevision
	GitHubFeedbackFilterRevision  = githubfeedback.FilterRevision
)

var githubFeedbackReasonPattern = regexp.MustCompile("^[a-z0-9_-]{1,96}$")

type GitHubFeedbackSource struct {
	CompanyID             string
	SourceID              string
	Repository            githubfeedback.Repository
	ProfileRevision       string
	FilterRevision        string
	ConfigurationSHA256   string
	State                 string
	PermissionStatus      string
	PermissionProbeSHA256 string
	CreatedAt             string
}

type GitHubFeedbackSourceInput struct {
	SourceID   string
	Repository githubfeedback.Repository
	RequestID  string
}

type GitHubFeedbackPermissionInput struct {
	SourceID  string
	Probe     githubfeedback.ScanResult
	Rationale string
	RequestID string
}

type GitHubFeedbackSourceDecisionInput struct {
	Decision  string
	Rationale string
	RequestID string
}

type GitHubFeedbackSourceEvent struct {
	CompanyID             string
	SourceID              string
	State                 string
	PermissionStatus      string
	PermissionProbeSHA256 string
}

type GitHubFeedbackScanRecord struct {
	CompanyID      string
	SourceID       string
	ScanID         string
	Coverage       string
	CoverageReason string
	CoveredThrough *time.Time
	PageCount      int
	ItemCount      int
	CreatedAt      string
}

type GitHubFeedbackCommentInput struct {
	SourceID  string
	Result    githubfeedback.CommentResult
	RequestID string
}

type GitHubFeedbackCommentScanRecord struct {
	CompanyID           string
	SourceID            string
	CommentScanID       string
	ProviderItemID      int64
	IssueRevisionSHA256 string
	Coverage            string
	CoverageReason      string
	PageCount           int
	CommentCount        int
	BodyBytes           int
	BodyTruncated       bool
	CreatedAt           string
}

func (k *Kernel) TXRegisterGitHubFeedbackSource(ctx context.Context, companyID string, input GitHubFeedbackSourceInput) (GitHubFeedbackSource, error) {
	if !core.ValidID(companyID) || !core.ValidID(input.RequestID) || !githubfeedback.ValidRepository(input.Repository) || (input.SourceID != "" && !core.ValidID(input.SourceID)) {
		return GitHubFeedbackSource{}, core.Malformed
	}
	if input.SourceID == "" {
		input.SourceID = stableCapabilityID("gh-source", companyID, input.RequestID)
	}
	configurationDigest, err := githubfeedback.SourceConfigurationDigest(input.Repository)
	if err != nil {
		return GitHubFeedbackSource{}, core.Malformed
	}
	_, err = k.TXWrite(ctx, k.LocalScope(companyID), nil, input.RequestID, "feedback.github.source.register", struct {
		SourceID            string
		Repository          githubfeedback.Repository
		ConfigurationSHA256 string
	}{input.SourceID, input.Repository, configurationDigest}, func(tx pgx.Tx) (Receipt, error) {
		if err := ensureEnvironmentCompanyWritable(ctx, tx, companyID); err != nil {
			return Receipt{}, err
		}
		_, err := tx.Exec(ctx, "INSERT INTO feedback_source_bindings(company_id,source_id,provider,repository_id,repository_owner,repository_name,profile_revision,filter_revision,configuration_sha256,created_by) VALUES($1,$2,'github',$3,$4,$5,$6,$7,$8,'local-owner')",
			companyID, input.SourceID, input.Repository.ID, input.Repository.Owner, input.Repository.Name, GitHubFeedbackProfileRevision, GitHubFeedbackFilterRevision, configurationDigest)
		if err != nil {
			if isUniqueViolation(err) {
				return Receipt{}, core.Conflict
			}
			return Receipt{}, err
		}
		if err := insertGitHubFeedbackSourceEvent(ctx, tx, companyID, input.SourceID, input.RequestID, "draft", "unverified", "", "registered explicitly selected repository"); err != nil {
			return Receipt{}, err
		}
		return Receipt{ID: input.SourceID, Status: "draft"}, nil
	})
	if err != nil {
		return GitHubFeedbackSource{}, err
	}
	return k.GetGitHubFeedbackSource(ctx, companyID, input.SourceID)
}

func (k *Kernel) GetGitHubFeedbackSource(ctx context.Context, companyID, sourceID string) (GitHubFeedbackSource, error) {
	if !core.ValidID(companyID) || !core.ValidID(sourceID) {
		return GitHubFeedbackSource{}, core.Malformed
	}
	var source GitHubFeedbackSource
	var probeSHA256 *string
	err := k.pool.QueryRow(ctx, "SELECT b.company_id,b.source_id,b.repository_id,b.repository_owner,b.repository_name,b.profile_revision,b.filter_revision,b.configuration_sha256,b.created_at::text,event.state,event.permission_status,event.permission_probe_sha256 FROM feedback_source_bindings b LEFT JOIN LATERAL (SELECT state,permission_status,permission_probe_sha256 FROM feedback_source_events WHERE company_id=b.company_id AND source_id=b.source_id ORDER BY event_seq DESC LIMIT 1) event ON true WHERE b.company_id=$1 AND b.source_id=$2", companyID, sourceID).Scan(
		&source.CompanyID, &source.SourceID, &source.Repository.ID, &source.Repository.Owner, &source.Repository.Name, &source.ProfileRevision, &source.FilterRevision, &source.ConfigurationSHA256, &source.CreatedAt, &source.State, &source.PermissionStatus, &probeSHA256)
	if errors.Is(err, pgx.ErrNoRows) {
		return GitHubFeedbackSource{}, core.OutOfScope
	}
	if probeSHA256 != nil {
		source.PermissionProbeSHA256 = *probeSHA256
	}
	return source, err
}

func (k *Kernel) GetGitHubFeedbackSourceEventByRequest(ctx context.Context, companyID, requestID string) (GitHubFeedbackSourceEvent, bool, error) {
	if !core.ValidID(companyID) || !core.ValidID(requestID) {
		return GitHubFeedbackSourceEvent{}, false, core.Malformed
	}
	var event GitHubFeedbackSourceEvent
	var probeSHA256 *string
	err := k.pool.QueryRow(ctx, "SELECT company_id,source_id,state,permission_status,permission_probe_sha256 FROM feedback_source_events WHERE company_id=$1 AND request_id=$2", companyID, requestID).Scan(
		&event.CompanyID, &event.SourceID, &event.State, &event.PermissionStatus, &probeSHA256)
	if errors.Is(err, pgx.ErrNoRows) {
		return GitHubFeedbackSourceEvent{}, false, nil
	}
	if err != nil {
		return GitHubFeedbackSourceEvent{}, false, err
	}
	if probeSHA256 != nil {
		event.PermissionProbeSHA256 = *probeSHA256
	}
	return event, true, nil
}

func (k *Kernel) TXRecordGitHubFeedbackPermissionProbe(ctx context.Context, companyID string, input GitHubFeedbackPermissionInput) error {
	if !core.ValidID(companyID) || !core.ValidID(input.SourceID) || !core.ValidID(input.RequestID) || strings.TrimSpace(input.Rationale) == "" || len(input.Rationale) > 512 {
		return core.Malformed
	}
	permissionStatus := "unknown"
	probeSHA256 := ""
	if input.Probe.ReadPermissionVerified && validFeedbackSHA256(input.Probe.PermissionProbeSHA256) {
		permissionStatus = "verified"
		probeSHA256 = input.Probe.PermissionProbeSHA256
	} else if input.Probe.CoverageReason == githubfeedback.ReasonUnauthorized || input.Probe.CoverageReason == githubfeedback.ReasonForbidden || input.Probe.CoverageReason == githubfeedback.ReasonNotFoundOrHidden {
		permissionStatus = "denied"
	}
	probeInputDigest := githubFeedbackScanDigest(input.Probe)
	_, err := k.TXWrite(ctx, k.LocalScope(companyID), nil, input.RequestID, "feedback.github.source.permission_probe", struct {
		SourceID         string
		Permission       string
		ProbeSHA256      string
		ProbeInputSHA256 string
		Rationale        string
	}{input.SourceID, permissionStatus, probeSHA256, probeInputDigest, strings.TrimSpace(input.Rationale)}, func(tx pgx.Tx) (Receipt, error) {
		if err := ensureEnvironmentCompanyWritable(ctx, tx, companyID); err != nil {
			return Receipt{}, err
		}
		var repository githubfeedback.Repository
		if err := tx.QueryRow(ctx, "SELECT repository_id,repository_owner,repository_name FROM feedback_source_bindings WHERE company_id=$1 AND source_id=$2 FOR UPDATE", companyID, input.SourceID).Scan(&repository.ID, &repository.Owner, &repository.Name); errors.Is(err, pgx.ErrNoRows) {
			return Receipt{}, core.OutOfScope
		} else if err != nil {
			return Receipt{}, err
		}
		if repository.ID != input.Probe.Repository.ID || !strings.EqualFold(repository.Owner, input.Probe.Repository.Owner) || !strings.EqualFold(repository.Name, input.Probe.Repository.Name) {
			return Receipt{}, core.Integrity
		}
		state, _, _, err := latestGitHubFeedbackSourceEvent(ctx, tx, companyID, input.SourceID)
		if err != nil {
			return Receipt{}, err
		}
		if state == "revoked" {
			return Receipt{}, core.Denied
		}
		eventID := stableCapabilityID("gh-source-event", companyID, input.RequestID)
		if err := insertGitHubFeedbackSourceEvent(ctx, tx, companyID, input.SourceID, input.RequestID, state, permissionStatus, probeSHA256, strings.TrimSpace(input.Rationale)); err != nil {
			return Receipt{}, err
		}
		return Receipt{ID: eventID, Status: permissionStatus}, nil
	})
	return err
}

func (k *Kernel) TXDecideGitHubFeedbackSource(ctx context.Context, companyID, sourceID string, input GitHubFeedbackSourceDecisionInput) error {
	if !core.ValidID(companyID) || !core.ValidID(sourceID) || !core.ValidID(input.RequestID) || strings.TrimSpace(input.Rationale) == "" || len(input.Rationale) > 512 {
		return core.Malformed
	}
	if input.Decision != "approved" && input.Decision != "paused" && input.Decision != "revoked" {
		return core.Malformed
	}
	_, err := k.TXWrite(ctx, k.LocalScope(companyID), nil, input.RequestID, "feedback.github.source.decide", struct {
		SourceID  string
		Decision  string
		Rationale string
	}{sourceID, input.Decision, strings.TrimSpace(input.Rationale)}, func(tx pgx.Tx) (Receipt, error) {
		if err := ensureEnvironmentCompanyWritable(ctx, tx, companyID); err != nil {
			return Receipt{}, err
		}
		state, permissionStatus, probeSHA256, err := latestGitHubFeedbackSourceEvent(ctx, tx, companyID, sourceID)
		if err != nil {
			return Receipt{}, err
		}
		switch input.Decision {
		case "approved":
			if state == "revoked" || state == "approved" || permissionStatus != "verified" || !validFeedbackSHA256(probeSHA256) {
				return Receipt{}, core.Denied
			}
		case "paused":
			if state != "approved" {
				return Receipt{}, core.Conflict
			}
		case "revoked":
			if state == "revoked" {
				return Receipt{}, core.Conflict
			}
		}
		eventID := stableCapabilityID("gh-source-event", companyID, input.RequestID)
		if err := insertGitHubFeedbackSourceEvent(ctx, tx, companyID, sourceID, input.RequestID, input.Decision, permissionStatus, probeSHA256, strings.TrimSpace(input.Rationale)); err != nil {
			return Receipt{}, err
		}
		return Receipt{ID: eventID, Status: input.Decision}, nil
	})
	return err
}

func (k *Kernel) TXRecordGitHubFeedbackScan(ctx context.Context, companyID, sourceID string, scan githubfeedback.ScanResult, requestID string) (GitHubFeedbackScanRecord, error) {
	if !core.ValidID(companyID) || !core.ValidID(sourceID) || !core.ValidID(requestID) {
		return GitHubFeedbackScanRecord{}, core.Malformed
	}
	if err := validateGitHubFeedbackScan(scan); err != nil {
		return GitHubFeedbackScanRecord{}, err
	}
	scanDigest := githubFeedbackScanDigest(scan)
	scanID := stableCapabilityID("gh-scan", companyID, requestID)
	_, err := k.TXWrite(ctx, k.LocalScope(companyID), nil, requestID, "feedback.github.scan.record", scanDigest, func(tx pgx.Tx) (Receipt, error) {
		if err := ensureEnvironmentCompanyWritable(ctx, tx, companyID); err != nil {
			return Receipt{}, err
		}
		var configurationSHA256 string
		if err := tx.QueryRow(ctx, "SELECT configuration_sha256 FROM feedback_source_bindings WHERE company_id=$1 AND source_id=$2 FOR UPDATE", companyID, sourceID).Scan(&configurationSHA256); errors.Is(err, pgx.ErrNoRows) {
			return Receipt{}, core.OutOfScope
		} else if err != nil {
			return Receipt{}, err
		}
		expectedDigest, digestErr := githubfeedback.SourceConfigurationDigest(scan.Repository)
		if digestErr != nil || expectedDigest != configurationSHA256 {
			return Receipt{}, core.Integrity
		}
		state, permissionStatus, _, err := latestGitHubFeedbackSourceEvent(ctx, tx, companyID, sourceID)
		if err != nil {
			return Receipt{}, err
		}
		if state != "approved" || permissionStatus != "verified" {
			return Receipt{}, core.Denied
		}
		var coveredThrough any
		if scan.CoveredThrough != nil {
			coveredThrough = scan.CoveredThrough.UTC()
		}
		var sinceCursor any
		if !scan.Since.IsZero() {
			sinceCursor = scan.Since.UTC()
		}
		if _, err := tx.Exec(ctx, "INSERT INTO feedback_scan_runs(company_id,scan_id,source_id,request_id,configuration_sha256,since_cursor,coverage_cutoff,coverage_status,coverage_reason,covered_through,page_count,item_count) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)",
			companyID, scanID, sourceID, requestID, configurationSHA256, sinceCursor, scan.CoverageCutoff.UTC(), string(scan.Coverage), scan.CoverageReason, coveredThrough, len(scan.Pages), len(scan.Issues)); err != nil {
			return Receipt{}, err
		}
		for _, page := range scan.Pages {
			if _, err := tx.Exec(ctx, "INSERT INTO feedback_scan_pages(company_id,source_id,scan_id,page_number,response_sha256,etag,item_count) VALUES($1,$2,$3,$4,$5,NULLIF($6,''),$7)",
				companyID, sourceID, scanID, page.PageNumber, page.ResponseSHA256, page.ETag, page.ItemCount); err != nil {
				return Receipt{}, err
			}
		}
		for _, issue := range scan.Issues {
			revisionSHA256 := githubFeedbackIssueRevisionDigest(issue)
			if _, err := tx.Exec(ctx, "INSERT INTO feedback_observations(company_id,source_id,provider_item_id,issue_number,revision_sha256,body_sha256,title,title_truncated,body,body_truncated,issue_state,source_updated_at,html_url,untrusted) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,NULLIF($13,''),true) ON CONFLICT DO NOTHING",
				companyID, sourceID, issue.ID, issue.Number, revisionSHA256, issue.BodySHA256, issue.Title, issue.TitleTruncated, issue.Body, issue.BodyTruncated, issue.State, issue.UpdatedAt, issue.HTMLURL); err != nil {
				return Receipt{}, err
			}
			if _, err := tx.Exec(ctx, "INSERT INTO feedback_scan_items(company_id,source_id,scan_id,page_number,provider_item_id,revision_sha256) VALUES($1,$2,$3,$4,$5,$6)",
				companyID, sourceID, scanID, issue.PageNumber, issue.ID, revisionSHA256); err != nil {
				return Receipt{}, err
			}
			if err := k.ensureGitHubFeedbackBacklogObservation(ctx, tx, companyID, sourceID, issue.ID); err != nil {
				return Receipt{}, err
			}
		}
		if !scan.ReadPermissionVerified && (scan.CoverageReason == githubfeedback.ReasonUnauthorized || scan.CoverageReason == githubfeedback.ReasonForbidden || scan.CoverageReason == githubfeedback.ReasonNotFoundOrHidden) {
			permissionRequestID := stableCapabilityID("gh-scan-permission", companyID, requestID)
			if err := insertGitHubFeedbackSourceEvent(ctx, tx, companyID, sourceID, permissionRequestID, state, "denied", "", "provider read permission is unavailable"); err != nil {
				return Receipt{}, err
			}
		}
		return Receipt{ID: scanID, Status: string(scan.Coverage)}, nil
	})
	if err != nil {
		return GitHubFeedbackScanRecord{}, err
	}
	return k.GetGitHubFeedbackScan(ctx, companyID, scanID)
}

func (k *Kernel) GetGitHubFeedbackScan(ctx context.Context, companyID, scanID string) (GitHubFeedbackScanRecord, error) {
	if !core.ValidID(companyID) || !core.ValidID(scanID) {
		return GitHubFeedbackScanRecord{}, core.Malformed
	}
	var record GitHubFeedbackScanRecord
	var coveredThrough pgtype.Timestamptz
	err := k.pool.QueryRow(ctx, "SELECT company_id,source_id,scan_id,coverage_status,coverage_reason,covered_through,page_count,item_count,created_at::text FROM feedback_scan_runs WHERE company_id=$1 AND scan_id=$2", companyID, scanID).Scan(&record.CompanyID, &record.SourceID, &record.ScanID, &record.Coverage, &record.CoverageReason, &coveredThrough, &record.PageCount, &record.ItemCount, &record.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return GitHubFeedbackScanRecord{}, core.OutOfScope
	}
	if coveredThrough.Valid {
		value := coveredThrough.Time.UTC()
		record.CoveredThrough = &value
	}
	return record, err
}

func (k *Kernel) GetGitHubFeedbackScanByRequest(ctx context.Context, companyID, requestID string) (GitHubFeedbackScanRecord, bool, error) {
	if !core.ValidID(companyID) || !core.ValidID(requestID) {
		return GitHubFeedbackScanRecord{}, false, core.Malformed
	}
	var record GitHubFeedbackScanRecord
	var coveredThrough pgtype.Timestamptz
	err := k.pool.QueryRow(ctx, "SELECT company_id,source_id,scan_id,coverage_status,coverage_reason,covered_through,page_count,item_count,created_at::text FROM feedback_scan_runs WHERE company_id=$1 AND request_id=$2", companyID, requestID).Scan(
		&record.CompanyID, &record.SourceID, &record.ScanID, &record.Coverage, &record.CoverageReason, &coveredThrough, &record.PageCount, &record.ItemCount, &record.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return GitHubFeedbackScanRecord{}, false, nil
	}
	if err != nil {
		return GitHubFeedbackScanRecord{}, false, err
	}
	if coveredThrough.Valid {
		value := coveredThrough.Time.UTC()
		record.CoveredThrough = &value
	}
	return record, true, nil
}

func (k *Kernel) GitHubFeedbackCommentsForPollRequest(ctx context.Context, companyID, pollRequestID string) (int, bool, string, error) {
	if !core.ValidID(companyID) || !core.ValidID(pollRequestID) {
		return 0, false, "", core.Malformed
	}
	parentDigest := sha256.Sum256([]byte(pollRequestID))
	prefix := "ghfb-" + hex.EncodeToString(parentDigest[:16]) + "-%"
	var count int
	var partial bool
	var reason string
	err := k.pool.QueryRow(ctx, `SELECT count(*),COALESCE(bool_or(coverage_status='partial'),false),
COALESCE(string_agg(DISTINCT NULLIF(coverage_reason,''),','),'')
FROM feedback_comment_scans WHERE company_id=$1 AND request_id LIKE $2`, companyID, prefix).Scan(&count, &partial, &reason)
	return count, partial, reason, err
}

func (k *Kernel) LatestGitHubFeedbackCursor(ctx context.Context, companyID, sourceID string) (time.Time, bool, error) {
	if !core.ValidID(companyID) || !core.ValidID(sourceID) {
		return time.Time{}, false, core.Malformed
	}
	var coveredThrough pgtype.Timestamptz
	err := k.pool.QueryRow(ctx, "SELECT covered_through FROM feedback_scan_runs WHERE company_id=$1 AND source_id=$2 AND coverage_status='complete' ORDER BY coverage_cutoff DESC,created_at DESC LIMIT 1", companyID, sourceID).Scan(&coveredThrough)
	if errors.Is(err, pgx.ErrNoRows) {
		return time.Time{}, false, nil
	}
	if err != nil || !coveredThrough.Valid {
		return time.Time{}, false, err
	}
	return coveredThrough.Time.UTC(), true, nil
}

func (k *Kernel) TXRecordGitHubFeedbackCommentScan(ctx context.Context, companyID string, input GitHubFeedbackCommentInput) (GitHubFeedbackCommentScanRecord, error) {
	if !core.ValidID(companyID) || !core.ValidID(input.SourceID) || !core.ValidID(input.RequestID) {
		return GitHubFeedbackCommentScanRecord{}, core.Malformed
	}
	if err := validateGitHubFeedbackCommentScan(input.Result); err != nil {
		return GitHubFeedbackCommentScanRecord{}, err
	}
	issueRevisionSHA256 := githubFeedbackIssueRevisionDigest(input.Result.Issue)
	inputDigest := githubFeedbackCommentScanDigest(input.Result)
	commentScanID := stableCapabilityID("gh-comments", companyID, input.RequestID)
	_, err := k.TXWrite(ctx, k.LocalScope(companyID), nil, input.RequestID, "feedback.github.comments.record", inputDigest, func(tx pgx.Tx) (Receipt, error) {
		if err := ensureEnvironmentCompanyWritable(ctx, tx, companyID); err != nil {
			return Receipt{}, err
		}
		var configurationSHA256 string
		if err := tx.QueryRow(ctx, "SELECT configuration_sha256 FROM feedback_source_bindings WHERE company_id=$1 AND source_id=$2 FOR UPDATE", companyID, input.SourceID).Scan(&configurationSHA256); errors.Is(err, pgx.ErrNoRows) {
			return Receipt{}, core.OutOfScope
		} else if err != nil {
			return Receipt{}, err
		}
		expectedDigest, digestErr := githubfeedback.SourceConfigurationDigest(input.Result.Repository)
		if digestErr != nil || expectedDigest != configurationSHA256 || input.Result.Issue.RepositoryID != input.Result.Repository.ID {
			return Receipt{}, core.Integrity
		}
		state, permissionStatus, _, err := latestGitHubFeedbackSourceEvent(ctx, tx, companyID, input.SourceID)
		if err != nil {
			return Receipt{}, err
		}
		if state != "approved" || permissionStatus != "verified" {
			return Receipt{}, core.Denied
		}
		var issueExists bool
		if err := tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM feedback_observations WHERE company_id=$1 AND source_id=$2 AND provider_item_id=$3 AND revision_sha256=$4)", companyID, input.SourceID, input.Result.Issue.ID, issueRevisionSHA256).Scan(&issueExists); err != nil {
			return Receipt{}, err
		}
		if !issueExists {
			return Receipt{}, core.OutOfScope
		}
		bodyBytes := 0
		bodyTruncated := false
		for _, comment := range input.Result.Comments {
			bodyBytes += len(comment.Body)
			bodyTruncated = bodyTruncated || comment.BodyTruncated
		}
		if _, err := tx.Exec(ctx, "INSERT INTO feedback_comment_scans(company_id,comment_scan_id,source_id,provider_item_id,issue_revision_sha256,request_id,coverage_status,coverage_reason,page_count,comment_count,body_bytes,body_truncated) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)",
			companyID, commentScanID, input.SourceID, input.Result.Issue.ID, issueRevisionSHA256, input.RequestID, string(input.Result.Coverage), input.Result.CoverageReason, len(input.Result.Pages), len(input.Result.Comments), bodyBytes, bodyTruncated); err != nil {
			return Receipt{}, err
		}
		for _, page := range input.Result.Pages {
			if _, err := tx.Exec(ctx, "INSERT INTO feedback_comment_pages(company_id,source_id,comment_scan_id,page_number,response_sha256,etag,item_count) VALUES($1,$2,$3,$4,$5,NULLIF($6,''),$7)",
				companyID, input.SourceID, commentScanID, page.PageNumber, page.ResponseSHA256, page.ETag, page.ItemCount); err != nil {
				return Receipt{}, err
			}
		}
		for _, comment := range input.Result.Comments {
			revisionSHA256 := githubFeedbackCommentRevisionDigest(comment)
			if _, err := tx.Exec(ctx, "INSERT INTO feedback_comment_observations(company_id,source_id,provider_item_id,issue_revision_sha256,provider_comment_id,revision_sha256,body_sha256,body,body_truncated,source_updated_at,html_url,untrusted) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,NULLIF($11,''),true) ON CONFLICT DO NOTHING",
				companyID, input.SourceID, input.Result.Issue.ID, issueRevisionSHA256, comment.ID, revisionSHA256, comment.BodySHA256, comment.Body, comment.BodyTruncated, comment.UpdatedAt, comment.HTMLURL); err != nil {
				return Receipt{}, err
			}
			if _, err := tx.Exec(ctx, "INSERT INTO feedback_comment_scan_items(company_id,source_id,comment_scan_id,page_number,provider_item_id,issue_revision_sha256,provider_comment_id,comment_revision_sha256) VALUES($1,$2,$3,$4,$5,$6,$7,$8)",
				companyID, input.SourceID, commentScanID, comment.PageNumber, input.Result.Issue.ID, issueRevisionSHA256, comment.ID, revisionSHA256); err != nil {
				return Receipt{}, err
			}
		}
		return Receipt{ID: commentScanID, Status: string(input.Result.Coverage)}, nil
	})
	if err != nil {
		return GitHubFeedbackCommentScanRecord{}, err
	}
	return k.GetGitHubFeedbackCommentScan(ctx, companyID, commentScanID)
}

func (k *Kernel) GetGitHubFeedbackCommentScan(ctx context.Context, companyID, scanID string) (GitHubFeedbackCommentScanRecord, error) {
	if !core.ValidID(companyID) || !core.ValidID(scanID) {
		return GitHubFeedbackCommentScanRecord{}, core.Malformed
	}
	var record GitHubFeedbackCommentScanRecord
	err := k.pool.QueryRow(ctx, "SELECT company_id,source_id,comment_scan_id,provider_item_id,issue_revision_sha256,coverage_status,coverage_reason,page_count,comment_count,body_bytes,body_truncated,created_at::text FROM feedback_comment_scans WHERE company_id=$1 AND comment_scan_id=$2", companyID, scanID).Scan(
		&record.CompanyID, &record.SourceID, &record.CommentScanID, &record.ProviderItemID, &record.IssueRevisionSHA256, &record.Coverage, &record.CoverageReason, &record.PageCount, &record.CommentCount, &record.BodyBytes, &record.BodyTruncated, &record.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return GitHubFeedbackCommentScanRecord{}, core.OutOfScope
	}
	return record, err
}

func latestGitHubFeedbackSourceEvent(ctx context.Context, tx pgx.Tx, companyID, sourceID string) (string, string, string, error) {
	var state, permissionStatus string
	var probeSHA256 *string
	err := tx.QueryRow(ctx, "SELECT state,permission_status,permission_probe_sha256 FROM feedback_source_events WHERE company_id=$1 AND source_id=$2 ORDER BY event_seq DESC LIMIT 1", companyID, sourceID).Scan(&state, &permissionStatus, &probeSHA256)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", "", "", core.OutOfScope
	}
	probe := ""
	if probeSHA256 != nil {
		probe = *probeSHA256
	}
	return state, permissionStatus, probe, err
}

func insertGitHubFeedbackSourceEvent(ctx context.Context, tx pgx.Tx, companyID, sourceID, requestID, state, permissionStatus, probeSHA256, rationale string) error {
	eventID := stableCapabilityID("gh-source-event", companyID, requestID)
	_, err := tx.Exec(ctx, "INSERT INTO feedback_source_events(company_id,event_id,source_id,state,permission_status,permission_probe_sha256,rationale,actor,request_id) VALUES($1,$2,$3,$4,$5,NULLIF($6,''),$7,'local-owner',$8)",
		companyID, eventID, sourceID, state, permissionStatus, probeSHA256, rationale, requestID)
	return err
}

func validateGitHubFeedbackScan(scan githubfeedback.ScanResult) error {
	if !githubfeedback.ValidRepository(scan.Repository) || scan.CoverageCutoff.IsZero() {
		return core.Malformed
	}
	if scan.Coverage != githubfeedback.CoverageComplete && scan.Coverage != githubfeedback.CoveragePartial {
		return core.Malformed
	}
	if scan.Coverage == githubfeedback.CoverageComplete {
		if scan.CoverageReason != "" || scan.CoveredThrough == nil || !scan.CoveredThrough.Equal(scan.CoverageCutoff) || len(scan.Pages) == 0 {
			return core.Malformed
		}
	} else if scan.CoverageReason == "" || scan.CoveredThrough != nil || !githubFeedbackReasonPattern.MatchString(scan.CoverageReason) {
		return core.Malformed
	}
	if len(scan.Pages) > 20 || len(scan.Issues) > 2000 {
		return core.TooLarge
	}
	pages := make(map[int]int, len(scan.Pages))
	pageItemTotal := 0
	for index, page := range scan.Pages {
		if page.PageNumber != index+1 || page.ItemCount < 0 || page.ItemCount > 100 || !validFeedbackSHA256(page.ResponseSHA256) || len(page.ETag) > 512 {
			return core.Malformed
		}
		pages[page.PageNumber] = page.ItemCount
		pageItemTotal += page.ItemCount
	}
	if len(scan.Issues) > pageItemTotal {
		return core.Malformed
	}
	seenIssues := make(map[int64]struct{}, len(scan.Issues))
	issueCountByPage := make(map[int]int, len(scan.Pages))
	for _, issue := range scan.Issues {
		if issue.ID <= 0 || issue.RepositoryID != scan.Repository.ID || issue.PageNumber <= 0 || issue.PageNumber > len(pages) || issue.Number <= 0 || issue.Title == "" || len(issue.Title) > 4096 || len(issue.Body) > 64<<10 || !validFeedbackSHA256(issue.BodySHA256) || !githubFeedbackState(issue.State) || !githubFeedbackTimestamp(issue.UpdatedAt) || !issue.Untrusted {
			return core.Malformed
		}
		if !issue.BodyTruncated && githubFeedbackSHA256([]byte(issue.Body)) != issue.BodySHA256 {
			return core.Integrity
		}
		if _, exists := seenIssues[issue.ID]; exists {
			return core.Malformed
		}
		seenIssues[issue.ID] = struct{}{}
		issueCountByPage[issue.PageNumber]++
		if issueCountByPage[issue.PageNumber] > pages[issue.PageNumber] {
			return core.Malformed
		}
	}
	return nil
}

func validateGitHubFeedbackCommentScan(result githubfeedback.CommentResult) error {
	if !githubfeedback.ValidRepository(result.Repository) || result.Issue.ID <= 0 || result.Issue.Number <= 0 || result.Issue.RepositoryID != result.Repository.ID || !result.Issue.Untrusted {
		return core.Malformed
	}
	if result.Coverage != githubfeedback.CoverageComplete && result.Coverage != githubfeedback.CoveragePartial {
		return core.Malformed
	}
	if result.Coverage == githubfeedback.CoverageComplete {
		if result.CoverageReason != "" || len(result.Pages) == 0 {
			return core.Malformed
		}
	} else if result.CoverageReason == "" || !githubFeedbackReasonPattern.MatchString(result.CoverageReason) {
		return core.Malformed
	}
	if len(result.Pages) > 20 || len(result.Comments) > 500 {
		return core.TooLarge
	}
	pages := make(map[int]int, len(result.Pages))
	pageItemTotal := 0
	for index, page := range result.Pages {
		if page.PageNumber != index+1 || page.ItemCount < 0 || page.ItemCount > 100 || !validFeedbackSHA256(page.ResponseSHA256) || len(page.ETag) > 512 {
			return core.Malformed
		}
		pages[page.PageNumber] = page.ItemCount
		pageItemTotal += page.ItemCount
	}
	if len(result.Comments) > pageItemTotal {
		return core.Malformed
	}
	seenIDs := make(map[int64]struct{}, len(result.Comments))
	pageCounts := make(map[int]int, len(result.Pages))
	totalBytes := 0
	for _, comment := range result.Comments {
		if comment.ID <= 0 || comment.PageNumber <= 0 || comment.PageNumber > len(pages) || len(comment.Body) > 1<<20 || !validFeedbackSHA256(comment.BodySHA256) || !githubFeedbackTimestamp(comment.UpdatedAt) || !comment.Untrusted {
			return core.Malformed
		}
		if !comment.BodyTruncated && githubFeedbackSHA256([]byte(comment.Body)) != comment.BodySHA256 {
			return core.Integrity
		}
		if _, exists := seenIDs[comment.ID]; exists {
			return core.Malformed
		}
		seenIDs[comment.ID] = struct{}{}
		pageCounts[comment.PageNumber]++
		if pageCounts[comment.PageNumber] > pages[comment.PageNumber] {
			return core.Malformed
		}
		totalBytes += len(comment.Body)
		if totalBytes > 1<<20 {
			return core.TooLarge
		}
	}
	return nil
}

func githubFeedbackScanDigest(scan githubfeedback.ScanResult) string {
	encoded, _ := json.Marshal(scan)
	return githubFeedbackSHA256(encoded)
}

func githubFeedbackIssueRevisionDigest(issue githubfeedback.Issue) string {
	encoded, _ := json.Marshal(struct {
		ProviderItemID int64
		IssueNumber    int
		Title          string
		TitleTruncated bool
		BodySHA256     string
		BodyTruncated  bool
		State          string
		UpdatedAt      string
	}{issue.ID, issue.Number, issue.Title, issue.TitleTruncated, issue.BodySHA256, issue.BodyTruncated, issue.State, issue.UpdatedAt})
	return githubFeedbackSHA256(encoded)
}

func githubFeedbackCommentRevisionDigest(comment githubfeedback.Comment) string {
	encoded, _ := json.Marshal(struct {
		ProviderCommentID int64
		BodySHA256        string
		BodyTruncated     bool
		UpdatedAt         string
	}{comment.ID, comment.BodySHA256, comment.BodyTruncated, comment.UpdatedAt})
	return githubFeedbackSHA256(encoded)
}

func githubFeedbackCommentScanDigest(result githubfeedback.CommentResult) string {
	encoded, _ := json.Marshal(result)
	return githubFeedbackSHA256(encoded)
}

func githubFeedbackSHA256(value []byte) string {
	sum := sha256.Sum256(value)
	return hex.EncodeToString(sum[:])
}

func validFeedbackSHA256(value string) bool {
	if len(value) != 64 {
		return false
	}
	for _, character := range value {
		if !((character >= '0' && character <= '9') || (character >= 'a' && character <= 'f')) {
			return false
		}
	}
	return true
}

func githubFeedbackState(value string) bool {
	return value == "open" || value == "closed"
}

func githubFeedbackTimestamp(value string) bool {
	_, err := time.Parse(time.RFC3339, value)
	return err == nil
}

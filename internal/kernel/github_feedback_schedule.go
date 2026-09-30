// pattern: Imperative Shell
package kernel

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"polis/internal/core"
)

const (
	minGitHubFeedbackCollectionIntervalSeconds = 15 * 60
	maxGitHubFeedbackCollectionIntervalSeconds = 24 * 60 * 60
	githubFeedbackPollReservationLease         = 5 * time.Minute
	maxGitHubFeedbackPollsPerTick              = 3
)

type GitHubFeedbackCollectionPolicy struct {
	CompanyID       string
	SourceID        string
	Enabled         bool
	IntervalSeconds int
	Rationale       string
	UpdatedAt       string
	NextPollAt      *string
	LastAttempt     string
	LastReasonCode  string
}

type GitHubFeedbackCollectionPolicyInput struct {
	SourceID        string
	Enabled         bool
	IntervalSeconds int
	Rationale       string
	RequestID       string
}

type GitHubFeedbackPollReservation struct {
	CompanyID    string
	SourceID     string
	RequestID    string
	ScheduledFor time.Time
	LeaseUntil   time.Time
}

type GitHubFeedbackPollAttemptResult struct {
	RequestID string
	Status    string
	Reason    string
}

func validGitHubFeedbackCollectionInterval(intervalSeconds int) bool {
	return intervalSeconds >= minGitHubFeedbackCollectionIntervalSeconds && intervalSeconds <= maxGitHubFeedbackCollectionIntervalSeconds
}

func (k *Kernel) TXSetGitHubFeedbackCollectionPolicy(ctx context.Context, companyID string, input GitHubFeedbackCollectionPolicyInput) (GitHubFeedbackCollectionPolicy, error) {
	input.Rationale = strings.TrimSpace(input.Rationale)
	if !core.ValidID(companyID) || !core.ValidID(input.SourceID) || !core.ValidID(input.RequestID) || !validGitHubFeedbackCollectionInterval(input.IntervalSeconds) || input.Rationale == "" || len(input.Rationale) > 512 {
		return GitHubFeedbackCollectionPolicy{}, core.Malformed
	}
	_, err := k.TXWrite(ctx, k.LocalScope(companyID), nil, input.RequestID, "feedback.github.collection.policy", input, func(tx pgx.Tx) (Receipt, error) {
		if err := ensureEnvironmentCompanyWritable(ctx, tx, companyID); err != nil {
			return Receipt{}, err
		}
		var state, permissionStatus string
		err := tx.QueryRow(ctx, `SELECT COALESCE(event.state,'draft'),COALESCE(event.permission_status,'unverified')
FROM feedback_source_bindings binding LEFT JOIN LATERAL (
 SELECT state,permission_status FROM feedback_source_events WHERE company_id=binding.company_id AND source_id=binding.source_id ORDER BY event_seq DESC LIMIT 1
) event ON true WHERE binding.company_id=$1 AND binding.source_id=$2 FOR UPDATE OF binding`, companyID, input.SourceID).Scan(&state, &permissionStatus)
		if errors.Is(err, pgx.ErrNoRows) {
			return Receipt{}, core.OutOfScope
		}
		if err != nil {
			return Receipt{}, err
		}
		if input.Enabled && (state != "approved" || permissionStatus != "verified") {
			return Receipt{}, core.Denied
		}
		var eventSeq int64
		if err := tx.QueryRow(ctx, "SELECT COALESCE(MAX(event_seq),0)+1 FROM feedback_collection_policy_events WHERE company_id=$1 AND source_id=$2", companyID, input.SourceID).Scan(&eventSeq); err != nil {
			return Receipt{}, err
		}
		eventID := stableCapabilityID("gh-collection-policy", companyID, input.SourceID, input.RequestID)
		now := time.Now().UTC()
		if _, err := tx.Exec(ctx, `INSERT INTO feedback_collection_policy_events(company_id,source_id,event_seq,event_id,request_id,enabled,interval_seconds,rationale,actor,created_at)
VALUES($1,$2,$3,$4,$5,$6,$7,$8,'local-owner',$9)`, companyID, input.SourceID, eventSeq, eventID, input.RequestID, input.Enabled, input.IntervalSeconds, input.Rationale, now); err != nil {
			return Receipt{}, err
		}
		var nextPollAt *time.Time
		if input.Enabled {
			next := now.Add(time.Duration(input.IntervalSeconds) * time.Second)
			nextPollAt = &next
		}
		if _, err := tx.Exec(ctx, `INSERT INTO feedback_collection_schedule_state(company_id,source_id,interval_seconds,next_poll_at,updated_at)
VALUES($1,$2,$3,$4,$5)
ON CONFLICT(company_id,source_id) DO UPDATE SET interval_seconds=EXCLUDED.interval_seconds,next_poll_at=EXCLUDED.next_poll_at,updated_at=EXCLUDED.updated_at`, companyID, input.SourceID, input.IntervalSeconds, nextPollAt, now); err != nil {
			return Receipt{}, err
		}
		return Receipt{ID: eventID, Status: map[bool]string{true: "enabled", false: "disabled"}[input.Enabled]}, nil
	})
	if err != nil {
		return GitHubFeedbackCollectionPolicy{}, err
	}
	return k.GetGitHubFeedbackCollectionPolicy(ctx, companyID, input.SourceID)
}

func (k *Kernel) GetGitHubFeedbackCollectionPolicy(ctx context.Context, companyID, sourceID string) (GitHubFeedbackCollectionPolicy, error) {
	if !core.ValidID(companyID) || !core.ValidID(sourceID) {
		return GitHubFeedbackCollectionPolicy{}, core.Malformed
	}
	policy := GitHubFeedbackCollectionPolicy{CompanyID: companyID, SourceID: sourceID}
	var nextPollAt *string
	err := k.pool.QueryRow(ctx, `SELECT COALESCE(event.enabled,false),COALESCE(event.interval_seconds,0),COALESCE(event.rationale,''),COALESCE(event.created_at::text,''),schedule.next_poll_at::text,
COALESCE(attempt.status,''),COALESCE(attempt.reason_code,'')
FROM feedback_source_bindings binding
LEFT JOIN LATERAL (
 SELECT enabled,interval_seconds,rationale,created_at FROM feedback_collection_policy_events
 WHERE company_id=binding.company_id AND source_id=binding.source_id ORDER BY event_seq DESC LIMIT 1
) event ON true
LEFT JOIN feedback_collection_schedule_state schedule USING(company_id,source_id)
LEFT JOIN LATERAL (
 SELECT status,reason_code FROM feedback_collection_poll_attempts
 WHERE company_id=binding.company_id AND source_id=binding.source_id ORDER BY scheduled_for DESC LIMIT 1
) attempt ON true
WHERE binding.company_id=$1 AND binding.source_id=$2`, companyID, sourceID).Scan(
		&policy.Enabled, &policy.IntervalSeconds, &policy.Rationale, &policy.UpdatedAt, &nextPollAt, &policy.LastAttempt, &policy.LastReasonCode)
	if errors.Is(err, pgx.ErrNoRows) {
		return GitHubFeedbackCollectionPolicy{}, core.OutOfScope
	}
	if err != nil {
		return GitHubFeedbackCollectionPolicy{}, err
	}
	policy.NextPollAt = nextPollAt
	return policy, nil
}

func (k *Kernel) TXReserveDueGitHubFeedbackPolls(ctx context.Context, now time.Time, limit int) ([]GitHubFeedbackPollReservation, error) {
	if limit < 1 || limit > maxGitHubFeedbackPollsPerTick {
		return nil, core.Malformed
	}
	cleanupTx, err := k.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer cleanupTx.Rollback(ctx)
	if err = k.checkRuntimeLease(ctx, cleanupTx); err != nil {
		return nil, err
	}
	if _, err = cleanupTx.Exec(ctx, `UPDATE feedback_collection_poll_attempts SET
status=CASE WHEN started_at IS NULL THEN 'failed' ELSE 'outcome_unknown' END,
lease_until=NULL,finished_at=$1,
reason_code=CASE WHEN started_at IS NULL THEN 'reservation_expired_before_dispatch' ELSE 'reservation_expired_after_dispatch' END
WHERE status IN ('reserved','started') AND lease_until<=$1`, now); err != nil {
		return nil, err
	}
	if err = cleanupTx.Commit(ctx); err != nil {
		return nil, err
	}
	tx, err := k.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	if err = k.checkRuntimeLease(ctx, tx); err != nil {
		return nil, err
	}
	rows, err := tx.Query(ctx, `SELECT schedule.company_id,schedule.source_id
FROM feedback_collection_schedule_state schedule
JOIN companies company ON company.id=schedule.company_id AND company.state='active'
JOIN LATERAL (
 SELECT enabled FROM feedback_collection_policy_events WHERE company_id=schedule.company_id AND source_id=schedule.source_id ORDER BY event_seq DESC LIMIT 1
) policy ON policy.enabled=true
JOIN LATERAL (
 SELECT state,permission_status FROM feedback_source_events WHERE company_id=schedule.company_id AND source_id=schedule.source_id ORDER BY event_seq DESC LIMIT 1
) source_state ON source_state.state='approved' AND source_state.permission_status='verified'
WHERE schedule.next_poll_at IS NOT NULL AND schedule.next_poll_at<=$1
ORDER BY schedule.next_poll_at,schedule.company_id,schedule.source_id LIMIT $2`, now, limit)
	if err != nil {
		return nil, err
	}
	type dueSource struct{ companyID, sourceID string }
	due := make([]dueSource, 0, limit)
	seenDue := make(map[string]struct{}, limit)
	for rows.Next() {
		var item dueSource
		if err := rows.Scan(&item.companyID, &item.sourceID); err != nil {
			rows.Close()
			return nil, err
		}
		key := item.companyID + "\x00" + item.sourceID
		if _, exists := seenDue[key]; exists {
			continue
		}
		seenDue[key] = struct{}{}
		due = append(due, item)
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()
	companySet := make(map[string]struct{}, len(due))
	companyIDs := make([]string, 0, len(due))
	for _, item := range due {
		if _, ok := companySet[item.companyID]; ok {
			continue
		}
		companySet[item.companyID] = struct{}{}
		companyIDs = append(companyIDs, item.companyID)
	}
	for index := 0; index < len(companyIDs); index++ {
		for other := index + 1; other < len(companyIDs); other++ {
			if companyIDs[other] < companyIDs[index] {
				companyIDs[index], companyIDs[other] = companyIDs[other], companyIDs[index]
			}
		}
	}
	for _, companyID := range companyIDs {
		if err = k.guard(ctx, tx, Scope{companyID}, nil); err != nil {
			return nil, err
		}
	}
	reservations := make([]GitHubFeedbackPollReservation, 0, len(due))
	for _, item := range due {
		var interval int
		var dueAt time.Time
		lockErr := tx.QueryRow(ctx, `SELECT schedule.interval_seconds,schedule.next_poll_at
FROM feedback_collection_schedule_state schedule
JOIN companies company ON company.id=schedule.company_id AND company.state='active'
JOIN feedback_source_bindings binding USING(company_id,source_id)
JOIN LATERAL (
 SELECT enabled FROM feedback_collection_policy_events WHERE company_id=schedule.company_id AND source_id=schedule.source_id ORDER BY event_seq DESC LIMIT 1
) policy ON policy.enabled=true
JOIN LATERAL (
 SELECT state,permission_status FROM feedback_source_events WHERE company_id=schedule.company_id AND source_id=schedule.source_id ORDER BY event_seq DESC LIMIT 1
) source_state ON source_state.state='approved' AND source_state.permission_status='verified'
WHERE schedule.company_id=$1 AND schedule.source_id=$2 AND schedule.next_poll_at<=$3
FOR UPDATE OF schedule,binding SKIP LOCKED`, item.companyID, item.sourceID, now).Scan(&interval, &dueAt)
		if errors.Is(lockErr, pgx.ErrNoRows) {
			continue
		}
		if lockErr != nil {
			return nil, lockErr
		}
		requestID := stableCapabilityID("gh-scheduled-poll", item.companyID, item.sourceID, dueAt.UTC().Format(time.RFC3339Nano))
		leaseUntil := now.Add(githubFeedbackPollReservationLease)
		if _, err = tx.Exec(ctx, `UPDATE feedback_collection_schedule_state SET next_poll_at=$3,updated_at=$2 WHERE company_id=$1 AND source_id=$4`, item.companyID, now, now.Add(time.Duration(interval)*time.Second), item.sourceID); err != nil {
			return nil, err
		}
		inserted, insertErr := tx.Exec(ctx, `INSERT INTO feedback_collection_poll_attempts(company_id,source_id,request_id,scheduled_for,status,lease_until)
VALUES($1,$2,$3,$4,'reserved',$5) ON CONFLICT DO NOTHING`, item.companyID, item.sourceID, requestID, dueAt, leaseUntil)
		if insertErr != nil {
			err = insertErr
			return nil, err
		}
		if inserted.RowsAffected() == 0 {
			continue
		}
		if err = appendEvent(ctx, tx, Scope{item.companyID}, "feedback.github.collection.poll_reserved", map[string]any{
			"source_id": item.sourceID, "request_id": requestID, "scheduled_for": dueAt.UTC().Format(time.RFC3339Nano), "lease_until": leaseUntil.UTC().Format(time.RFC3339Nano),
		}); err != nil {
			return nil, err
		}
		reservations = append(reservations, GitHubFeedbackPollReservation{CompanyID: item.companyID, SourceID: item.sourceID, RequestID: requestID, ScheduledFor: dueAt, LeaseUntil: leaseUntil})
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	return reservations, nil
}

func (k *Kernel) TXRecordGitHubFeedbackPollAttemptResult(ctx context.Context, companyID, sourceID string, result GitHubFeedbackPollAttemptResult) error {
	if !core.ValidID(companyID) || !core.ValidID(sourceID) || !core.ValidID(result.RequestID) || !validGitHubFeedbackPollAttemptStatus(result.Status) || !ValidGitHubFeedbackPollAttemptReason(result.Reason) {
		return core.Malformed
	}
	tx, err := k.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err = k.guard(ctx, tx, Scope{companyID}, nil); err != nil {
		return err
	}
	command, err := tx.Exec(ctx, `UPDATE feedback_collection_poll_attempts SET status=$4,lease_until=NULL,reason_code=$5,finished_at=clock_timestamp()
WHERE company_id=$1 AND source_id=$2 AND request_id=$3 AND status IN ('reserved','started')`, companyID, sourceID, result.RequestID, result.Status, result.Reason)
	if err != nil {
		return err
	}
	var status string
	err = tx.QueryRow(ctx, `SELECT status FROM feedback_collection_poll_attempts WHERE company_id=$1 AND source_id=$2 AND request_id=$3`, companyID, sourceID, result.RequestID).Scan(&status)
	if errors.Is(err, pgx.ErrNoRows) {
		return core.OutOfScope
	}
	if err != nil {
		return err
	}
	if status != result.Status {
		return core.Conflict
	}
	if command.RowsAffected() == 0 {
		return tx.Commit(ctx)
	}
	if err = appendEvent(ctx, tx, Scope{companyID}, "feedback.github.collection.poll_result", map[string]any{
		"source_id": sourceID, "request_id": result.RequestID, "status": result.Status, "reason_code": result.Reason,
	}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (k *Kernel) TXClaimGitHubFeedbackPollDispatch(ctx context.Context, companyID, sourceID, requestID string, now time.Time) (bool, error) {
	if !core.ValidID(companyID) || !core.ValidID(sourceID) || !core.ValidID(requestID) {
		return false, core.Malformed
	}
	tx, err := k.pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback(ctx)
	if err = k.guard(ctx, tx, Scope{companyID}, nil); err != nil {
		return false, err
	}
	var companyState string
	err = tx.QueryRow(ctx, "SELECT state FROM companies WHERE id=$1 FOR UPDATE", companyID).Scan(&companyState)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, core.OutOfScope
	}
	if err != nil {
		return false, err
	}
	var sourceState, permissionStatus string
	err = tx.QueryRow(ctx, `SELECT COALESCE(event.state,'draft'),COALESCE(event.permission_status,'unverified')
FROM feedback_source_bindings binding LEFT JOIN LATERAL (
 SELECT state,permission_status FROM feedback_source_events WHERE company_id=binding.company_id AND source_id=binding.source_id ORDER BY event_seq DESC LIMIT 1
) event ON true WHERE binding.company_id=$1 AND binding.source_id=$2 FOR UPDATE OF binding`, companyID, sourceID).Scan(&sourceState, &permissionStatus)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, core.OutOfScope
	}
	if err != nil {
		return false, err
	}
	var policyEnabled bool
	err = tx.QueryRow(ctx, `SELECT enabled FROM feedback_collection_policy_events WHERE company_id=$1 AND source_id=$2 ORDER BY event_seq DESC LIMIT 1`, companyID, sourceID).Scan(&policyEnabled)
	if errors.Is(err, pgx.ErrNoRows) {
		policyEnabled = false
	} else if err != nil {
		return false, err
	}
	var attemptStatus string
	var leaseUntil pgtype.Timestamptz
	err = tx.QueryRow(ctx, `SELECT status,lease_until FROM feedback_collection_poll_attempts
WHERE company_id=$1 AND source_id=$2 AND request_id=$3 FOR UPDATE`, companyID, sourceID, requestID).Scan(&attemptStatus, &leaseUntil)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, core.OutOfScope
	}
	if err != nil {
		return false, err
	}
	if attemptStatus != "reserved" {
		return false, tx.Commit(ctx)
	}
	failedReason := ""
	switch {
	case companyState != "active":
		failedReason = "company_not_active"
	case !leaseUntil.Valid || !leaseUntil.Time.After(now):
		failedReason = "reservation_expired_before_dispatch"
	case !policyEnabled:
		failedReason = "policy_disabled"
	case sourceState != "approved" || permissionStatus != "verified":
		failedReason = "source_not_authorized"
	}
	if failedReason != "" {
		if _, err = tx.Exec(ctx, `UPDATE feedback_collection_poll_attempts SET status='failed',lease_until=NULL,reason_code=$4,finished_at=$5
WHERE company_id=$1 AND source_id=$2 AND request_id=$3 AND status='reserved'`, companyID, sourceID, requestID, failedReason, now); err != nil {
			return false, err
		}
		if err = appendEvent(ctx, tx, Scope{companyID}, "feedback.github.collection.poll_not_dispatched", map[string]any{
			"source_id": sourceID, "request_id": requestID, "reason_code": failedReason,
		}); err != nil {
			return false, err
		}
		if err = tx.Commit(ctx); err != nil {
			return false, err
		}
		return false, nil
	}
	if _, err = tx.Exec(ctx, `UPDATE feedback_collection_poll_attempts SET status='started',started_at=$4
WHERE company_id=$1 AND source_id=$2 AND request_id=$3 AND status='reserved'`, companyID, sourceID, requestID, now); err != nil {
		return false, err
	}
	if err = appendEvent(ctx, tx, Scope{companyID}, "feedback.github.collection.poll_started", map[string]any{
		"source_id": sourceID, "request_id": requestID,
	}); err != nil {
		return false, err
	}
	if err = tx.Commit(ctx); err != nil {
		return false, err
	}
	return true, nil
}

func validGitHubFeedbackPollAttemptStatus(status string) bool {
	switch status {
	case "completed", "partial", "failed", "outcome_unknown":
		return true
	default:
		return false
	}
}

func ValidGitHubFeedbackPollAttemptReason(reason string) bool {
	return len(reason) <= 96 && (reason == "" || githubFeedbackReasonPattern.MatchString(reason))
}

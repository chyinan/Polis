// pattern: Imperative Shell
package control

import (
	"context"
	"errors"
	"strings"
	"time"

	"polis/internal/core"
	"polis/internal/kernel"
)

const githubFeedbackSchedulerTick = time.Minute

func (s *Service) RunGitHubFeedbackSchedulerOnce(ctx context.Context, now time.Time) (int, error) {
	if s == nil || !s.githubFeedbackSchedulerEnabled {
		return 0, nil
	}
	if s.runtime == nil || s.githubFeedback == nil {
		return 0, ErrGitHubFeedbackUnavailable
	}
	reservations, err := s.runtime.TXReserveDueGitHubFeedbackPolls(ctx, now.UTC(), maxScheduledGitHubFeedbackPollsPerTick)
	if err != nil {
		return 0, err
	}
	processed := 0
	var resultErr error
	for _, reservation := range reservations {
		if err := s.runScheduledGitHubFeedbackPoll(ctx, reservation); err != nil {
			resultErr = errors.Join(resultErr, err)
		}
		processed++
	}
	return processed, resultErr
}

const maxScheduledGitHubFeedbackPollsPerTick = 3

func (s *Service) runScheduledGitHubFeedbackPoll(ctx context.Context, reservation kernel.GitHubFeedbackPollReservation) error {
	claimed, err := s.runtime.TXClaimGitHubFeedbackPollDispatch(ctx, reservation.CompanyID, reservation.SourceID, reservation.RequestID, time.Now().UTC())
	if err != nil {
		return err
	}
	if !claimed {
		return nil
	}
	if err = ctx.Err(); err != nil {
		return s.finishScheduledGitHubFeedbackPoll(reservation, kernel.GitHubFeedbackPollAttemptResult{RequestID: reservation.RequestID, Status: "failed", Reason: "scheduler_stopped"}, err)
	}
	receipt, pollErr := s.PollGitHubFeedbackSource(ctx, reservation.CompanyID, reservation.SourceID, GitHubFeedbackPollRequest{RequestID: reservation.RequestID})
	result := classifyScheduledGitHubFeedbackPoll(receipt, pollErr)
	result.RequestID = reservation.RequestID
	return s.finishScheduledGitHubFeedbackPoll(reservation, result, pollErr)
}

func (s *Service) finishScheduledGitHubFeedbackPoll(reservation kernel.GitHubFeedbackPollReservation, result kernel.GitHubFeedbackPollAttemptResult, cause error) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := s.runtime.TXRecordGitHubFeedbackPollAttemptResult(ctx, reservation.CompanyID, reservation.SourceID, result); err != nil {
		return errors.Join(cause, err)
	}
	return cause
}

func classifyScheduledGitHubFeedbackPoll(receipt GitHubFeedbackPollReceipt, err error) kernel.GitHubFeedbackPollAttemptResult {
	result := kernel.GitHubFeedbackPollAttemptResult{RequestID: receipt.RequestID}
	if err != nil {
		result.Status = "outcome_unknown"
		result.Reason = "provider_outcome_unknown"
		if errors.Is(err, core.Denied) {
			result.Status = "failed"
			result.Reason = "source_not_authorized"
		} else if errors.Is(err, ErrGitHubFeedbackUnavailable) {
			result.Status = "failed"
			result.Reason = "provider_unavailable"
		}
		return result
	}
	if receipt.Coverage == "complete" && receipt.CommentCoverage != "partial" && receipt.CommentCoverage != "unknown" {
		result.Status = "completed"
		return result
	}
	result.Status = "partial"
	result.Reason = strings.TrimSpace(receipt.CoverageReason)
	if result.Reason == "" || !kernel.ValidGitHubFeedbackPollAttemptReason(result.Reason) {
		result.Reason = "coverage_partial"
	}
	return result
}

func (s *Service) RunGitHubFeedbackScheduler(ctx context.Context, reportError func(error)) {
	ticker := time.NewTicker(githubFeedbackSchedulerTick)
	defer ticker.Stop()
	for {
		if _, err := s.RunGitHubFeedbackSchedulerOnce(ctx, time.Now()); err != nil && reportError != nil {
			reportError(err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

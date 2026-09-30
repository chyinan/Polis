// pattern: Imperative Shell
package control

import (
	"context"
	"errors"
	"net/http"
	"time"

	"polis/internal/kernel"
	"polis/internal/qqnotify"
)

type QQNotificationSender interface {
	SenderSession(ctx context.Context) (qqnotify.SenderSessionInfo, error)
	SynchronizePolicy(ctx context.Context, plan kernel.HumanInterventionDeliveryPlan, session qqnotify.SenderSessionInfo) error
	Dispatch(ctx context.Context, plan kernel.HumanInterventionDeliveryPlan, session qqnotify.SenderSessionInfo) qqnotify.SendResult
}

type LoopbackQQNotificationSender struct {
	baseURL    string
	key        []byte
	httpClient *http.Client
}

const qqNotificationRetryPollInterval = 5 * time.Second
const qqNotificationRetryBatchSize = 50

func NewLoopbackQQNotificationSender(baseURL string, key []byte, httpClient *http.Client) (*LoopbackQQNotificationSender, error) {
	if len(key) < 32 {
		return nil, errors.New("notification sender IPC key is missing or too short")
	}
	if _, err := qqnotify.NewIPCClient(qqnotify.IPCClientConfig{
		BaseURL: baseURL, Key: key,
		Policy:     qqnotify.SenderPolicy{InstallationID: "bootstrap-install", SenderEpoch: "bootstrap-epoch"},
		HTTPClient: httpClient,
	}); err != nil {
		return nil, err
	}
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 15 * time.Second}
	}
	return &LoopbackQQNotificationSender{baseURL: baseURL, key: append([]byte(nil), key...), httpClient: httpClient}, nil
}

func (s *Service) SetQQNotificationSender(sender QQNotificationSender) {
	var retryContext context.Context
	var retryDone chan struct{}
	s.qqDispatchMu.Lock()
	s.qqSender = sender
	if sender != nil && s.qqRetryCancel == nil {
		retryContext, s.qqRetryCancel = context.WithCancel(context.Background())
		s.qqRetryDone = make(chan struct{})
		retryDone = s.qqRetryDone
	}
	s.qqDispatchMu.Unlock()
	if retryDone != nil {
		go s.runQQNotificationRetryLoop(retryContext, retryDone)
	}
}

func (s *Service) runQQNotificationRetryLoop(ctx context.Context, done chan struct{}) {
	defer close(done)
	ticker := time.NewTicker(qqNotificationRetryPollInterval)
	defer ticker.Stop()
	_ = s.DispatchDueHumanInterventionNotifications(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_ = s.DispatchDueHumanInterventionNotifications(ctx)
		}
	}
}

func (s *Service) DispatchDueHumanInterventionNotifications(ctx context.Context) error {
	s.qqDispatchMu.Lock()
	senderReady := s.qqSender != nil
	s.qqDispatchMu.Unlock()
	if !senderReady {
		return nil
	}
	if _, err := s.runtime.TXReconcileExpiredHumanInterventionSends(ctx); err != nil {
		return err
	}
	due, err := s.runtime.ListDueHumanInterventionRetries(ctx, qqNotificationRetryBatchSize)
	if err != nil {
		return err
	}
	var dispatchErr error
	for _, item := range due {
		if _, err = s.DispatchHumanIntervention(ctx, item.CompanyID, item.InterventionID); err != nil && dispatchErr == nil {
			dispatchErr = err
		}
	}
	return dispatchErr
}

func (s *Service) DispatchHumanIntervention(ctx context.Context, companyID, interventionID string) (qqnotify.SendResult, error) {
	s.qqDispatchMu.Lock()
	defer s.qqDispatchMu.Unlock()
	if s.qqSender == nil {
		return qqnotify.SendResult{Outcome: qqnotify.OutcomeRetryWait, ErrorCode: "sender_disabled", RetryAfterSeconds: 60}, errors.New("QQ sender is disabled")
	}
	session, err := s.qqSender.SenderSession(ctx)
	if err != nil {
		return qqnotify.SendResult{Outcome: qqnotify.OutcomeRetryWait, ErrorCode: "sender_unavailable", RetryAfterSeconds: 30}, err
	}
	plan, err := s.runtime.TXPrepareHumanInterventionDelivery(ctx, s.runtime.LocalScope(companyID), interventionID, session)
	if err != nil {
		return qqnotify.SendResult{Outcome: qqnotify.OutcomeRejected, ErrorCode: "notification_not_eligible"}, err
	}
	result := qqnotify.SendResult{Outcome: qqnotify.OutcomeRetryWait, ErrorCode: "sender_policy_unavailable", RetryAfterSeconds: 30}
	if err = s.qqSender.SynchronizePolicy(ctx, plan, session); err == nil {
		result = s.qqSender.Dispatch(ctx, plan, session)
	}
	if _, completeErr := s.runtime.TXCompleteHumanInterventionDelivery(ctx, s.runtime.LocalScope(companyID), plan, result); completeErr != nil {
		return result, completeErr
	}
	return result, err
}

func (s *LoopbackQQNotificationSender) SenderSession(ctx context.Context) (qqnotify.SenderSessionInfo, error) {
	return qqnotify.ReadSenderSession(ctx, s.baseURL, s.key, s.httpClient)
}

func (s *LoopbackQQNotificationSender) SynchronizePolicy(ctx context.Context, plan kernel.HumanInterventionDeliveryPlan, session qqnotify.SenderSessionInfo) error {
	policy := senderPolicyForPlan(plan, session)
	client, err := qqnotify.NewIPCClient(qqnotify.IPCClientConfig{BaseURL: s.baseURL, Key: s.key, Policy: policy, HTTPClient: s.httpClient})
	if err != nil {
		return err
	}
	return client.UpdatePolicy(ctx, policy)
}

func (s *LoopbackQQNotificationSender) Dispatch(ctx context.Context, plan kernel.HumanInterventionDeliveryPlan, session qqnotify.SenderSessionInfo) qqnotify.SendResult {
	current, err := s.SenderSession(ctx)
	if err != nil || current.InstallationID != plan.InstallationID || current.SenderEpoch != plan.SenderEpoch || current.InstallationID != session.InstallationID || current.SenderEpoch != session.SenderEpoch {
		return qqnotify.SendResult{Outcome: qqnotify.OutcomeRetryWait, ErrorCode: "sender_epoch_changed", RetryAfterSeconds: 30}
	}
	policy := senderPolicyForPlan(plan, current)
	client, err := qqnotify.NewIPCClient(qqnotify.IPCClientConfig{BaseURL: s.baseURL, Key: s.key, Policy: policy, HTTPClient: s.httpClient})
	if err != nil {
		return qqnotify.SendResult{Outcome: qqnotify.OutcomeRejected, ErrorCode: "sender_configuration_invalid"}
	}
	return client.SendIntervention(ctx, plan.Route, plan.Intervention, plan.Grant)
}

func senderPolicyForPlan(plan kernel.HumanInterventionDeliveryPlan, session qqnotify.SenderSessionInfo) qqnotify.SenderPolicy {
	return qqnotify.SenderPolicy{
		InstallationID: session.InstallationID,
		SenderEpoch:    session.SenderEpoch,
		RouteRevision:  plan.Route.Revision,
		TargetOpenID:   plan.Route.TargetOpenID,
		CredentialRef:  plan.CredentialRef,
		RouteEnabled:   plan.Route.Enabled,
		RouteStatus:    plan.Route.Status,
		QualifiedUntil: plan.Route.QualifiedUntil,
	}
}

// pattern: Imperative Shell
package control

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"polis/internal/kernel"
	"polis/internal/qqnotify"
)

type recordingQQNotificationSender struct {
	session          qqnotify.SenderSessionInfo
	synchronizeCalls int
	dispatchCalls    int
	lastPlan         kernel.HumanInterventionDeliveryPlan
	result           qqnotify.SendResult
	dispatchResults  chan qqnotify.SendResult
}

func (s *recordingQQNotificationSender) SenderSession(context.Context) (qqnotify.SenderSessionInfo, error) {
	return s.session, nil
}

func (s *recordingQQNotificationSender) SynchronizePolicy(_ context.Context, plan kernel.HumanInterventionDeliveryPlan, _ qqnotify.SenderSessionInfo) error {
	s.synchronizeCalls++
	s.lastPlan = plan
	return nil
}

func (s *recordingQQNotificationSender) Dispatch(_ context.Context, plan kernel.HumanInterventionDeliveryPlan, _ qqnotify.SenderSessionInfo) qqnotify.SendResult {
	s.dispatchCalls++
	s.lastPlan = plan
	if s.dispatchResults != nil {
		s.dispatchResults <- s.result
	}
	return s.result
}

func TestDispatchHumanInterventionRequiresQualifiedRouteAndPersistsFakeReceipt(t *testing.T) {
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
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	companyID := fmt.Sprintf("qq-sender-%d", time.Now().UnixNano())
	scope, err := runtime.TXCreateCompany(ctx, companyID)
	if err != nil {
		t.Fatal(err)
	}
	mission, err := runtime.TXCreateMissionGoal(ctx, scope, "QQ reminder", "exercise the fake sender adapter", "qq-sender-mission")
	if err != nil {
		t.Fatal(err)
	}
	intervention, err := runtime.TXCreateHumanIntervention(ctx, scope, kernel.HumanInterventionInput{
		MissionID: mission.ID, ProblemKey: fmt.Sprintf("qq-sender-%d", time.Now().UnixNano()), Severity: "high", ReasonCode: "handover_required",
		AffectedScope: "mission", ProtectionAction: "no_mutation", RequiredAction: "reauthorize_in_workbench",
	}, "qq-sender-intervention")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = runtime.TXConfigureNotificationRoute(ctx, scope, kernel.NotificationRouteInput{
		Adapter: "qq_official", Destination: "admin-openid", SafetyAlias: "Team A", CredentialRef: "default", Enabled: false,
	}, "qq-sender-route-draft"); err != nil {
		t.Fatal(err)
	}
	sender := &recordingQQNotificationSender{
		session:         qqnotify.SenderSessionInfo{InstallationID: "install-qq-test", SenderEpoch: "epoch-qq-test"},
		result:          qqnotify.SendResult{Outcome: qqnotify.OutcomeProviderAccepted, RemoteMessageID: "provider-message-1", HTTPStatus: 200},
		dispatchResults: make(chan qqnotify.SendResult, 4),
	}
	service := NewService(runtime, nil)
	defer service.Close()
	service.SetQQNotificationSender(sender)
	if _, err = service.DispatchHumanIntervention(ctx, companyID, intervention.ID); err == nil {
		t.Fatal("unqualified QQ route was accepted")
	}
	if sender.synchronizeCalls != 0 || sender.dispatchCalls != 0 {
		t.Fatalf("unqualified route reached sender: policy=%d sends=%d", sender.synchronizeCalls, sender.dispatchCalls)
	}

	var routeID string
	var routeRevision int64
	if err = pool.QueryRow(ctx, `SELECT id,route_revision FROM notification_routes WHERE company_id=$1 AND adapter='qq_official'`, companyID).Scan(&routeID, &routeRevision); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO qq_channel_qualifications(company_id,route_id,route_revision,target_openid,account_fingerprint,qualification_status,evidence_digest,qualified_until)
VALUES($1,$2,$3,'admin-openid',$4,'qualified',$5,$6)`, companyID, routeID, routeRevision, strings.Repeat("a", 64), strings.Repeat("b", 64), time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if _, err = runtime.TXConfigureNotificationRoute(ctx, scope, kernel.NotificationRouteInput{
		Adapter: "qq_official", Destination: "admin-openid", SafetyAlias: "Team A", CredentialRef: "default", Enabled: true,
	}, "qq-sender-route-enable"); err != nil {
		t.Fatal(err)
	}
	result, err := service.DispatchHumanIntervention(ctx, companyID, intervention.ID)
	if err != nil || result.Outcome != qqnotify.OutcomeProviderAccepted {
		t.Fatalf("fake dispatch result = %+v, err=%v", result, err)
	}
	if sender.synchronizeCalls != 1 || sender.dispatchCalls != 1 || sender.lastPlan.Route.CredentialRef != "default" || sender.lastPlan.Route.TargetOpenID != "admin-openid" {
		t.Fatalf("qualified dispatch plan/calls = %+v policy=%d sends=%d", sender.lastPlan, sender.synchronizeCalls, sender.dispatchCalls)
	}
	var deliveryState, remoteID string
	if err = pool.QueryRow(ctx, `SELECT state,remote_message_id FROM notification_deliveries WHERE company_id=$1 AND id=$2`, companyID, sender.lastPlan.DeliveryID).Scan(&deliveryState, &remoteID); err != nil {
		t.Fatal(err)
	}
	if deliveryState != "provider_accepted" || remoteID != "provider-message-1" {
		t.Fatalf("delivery receipt = state %q remote id %q", deliveryState, remoteID)
	}
	<-sender.dispatchResults

	secondIntervention, err := runtime.TXCreateHumanIntervention(ctx, scope, kernel.HumanInterventionInput{
		MissionID: mission.ID, ProblemKey: "qq-sender-retry", Severity: "high", ReasonCode: "handover_required",
		AffectedScope: "mission", ProtectionAction: "no_mutation", RequiredAction: "reauthorize_in_workbench",
	}, "qq-sender-retry-intervention")
	if err != nil {
		t.Fatal(err)
	}
	sender.result = qqnotify.SendResult{Outcome: qqnotify.OutcomeRetryWait, ErrorCode: "provider_rate_limited", RetryAfterSeconds: 60, HTTPStatus: 429}
	retryResult, err := service.DispatchHumanIntervention(ctx, companyID, secondIntervention.ID)
	if err != nil || retryResult.Outcome != qqnotify.OutcomeRetryWait {
		t.Fatalf("initial retryable result = %+v, err=%v", retryResult, err)
	}
	<-sender.dispatchResults
	service.Close()
	if _, err = pool.Exec(ctx, `UPDATE notification_deliveries SET retry_at=clock_timestamp()-interval '1 second' WHERE company_id=$1 AND id=$2`, companyID, sender.lastPlan.DeliveryID); err != nil {
		t.Fatal(err)
	}
	sender.result = qqnotify.SendResult{Outcome: qqnotify.OutcomeProviderAccepted, RemoteMessageID: "provider-message-retry", HTTPStatus: 200}
	restartedService := NewService(runtime, nil)
	defer restartedService.Close()
	restartedService.SetQQNotificationSender(sender)
	select {
	case retried := <-sender.dispatchResults:
		if retried.Outcome != qqnotify.OutcomeProviderAccepted {
			t.Fatalf("automatic retry result = %+v", retried)
		}
	case <-time.After(7 * time.Second):
		t.Fatal("persisted due notification was not retried by the dispatcher")
	}
	deadline := time.NewTimer(2 * time.Second)
	defer deadline.Stop()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		var persistedRemoteID *string
		if err = pool.QueryRow(ctx, `SELECT state,remote_message_id FROM notification_deliveries WHERE company_id=$1 AND intent_id=(SELECT id FROM notification_intents WHERE company_id=$1 AND intervention_id=$2) ORDER BY attempt_count DESC LIMIT 1`, companyID, secondIntervention.ID).Scan(&deliveryState, &persistedRemoteID); err != nil {
			t.Fatal(err)
		}
		if deliveryState == "provider_accepted" && persistedRemoteID != nil && *persistedRemoteID == "provider-message-retry" {
			break
		}
		select {
		case <-ticker.C:
		case <-deadline.C:
			t.Fatalf("bounded retry receipt was not persisted: state=%s remote_id=%v calls=%d", deliveryState, persistedRemoteID, sender.dispatchCalls)
		}
	}
	if sender.dispatchCalls != 3 {
		t.Fatalf("retry dispatch count = %d, want 3", sender.dispatchCalls)
	}
}

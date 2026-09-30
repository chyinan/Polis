// pattern: Imperative Shell
package kernel

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"testing"
	"time"

	"polis/internal/core"
	"polis/internal/qqnotify"
)

func TestHumanInterventionCreatesSafeDeduplicatedNotificationIntent(t *testing.T) {
	dsn := os.Getenv("POLIS_TEST_DSN")
	if dsn == "" {
		t.Skip("dedicated PostgreSQL required")
	}
	ctx := context.Background()
	runtime, err := Open(ctx, dsn, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Close()
	scope, err := runtime.TXCreateCompany(ctx, "human-intervention-"+newID())
	if err != nil {
		t.Fatal(err)
	}
	mission, err := runtime.TXCreateMissionGoal(ctx, scope, "Provider readiness", "record a human takeover need", "human-intervention-mission")
	if err != nil {
		t.Fatal(err)
	}
	input := HumanInterventionInput{
		MissionID: mission.ID, ProblemKey: "provider-readiness", Severity: "high", ReasonCode: "handover_required",
		AffectedScope: "mission", ProtectionAction: "no_mutation", RequiredAction: "reauthorize_in_workbench",
	}
	first, err := runtime.TXCreateHumanIntervention(ctx, scope, input, "human-intervention-open-1")
	if err != nil {
		t.Fatal(err)
	}
	if first.State != "open" || first.Occurrence != 1 || first.ID == "" || first.IncidentID == "" {
		t.Fatalf("human intervention = %+v", first)
	}
	second, err := runtime.TXCreateHumanIntervention(ctx, scope, input, "human-intervention-open-2")
	if err != nil {
		t.Fatal(err)
	}
	if second.ID != first.ID || second.Occurrence != first.Occurrence {
		t.Fatalf("open occurrence was not deduplicated: first=%+v second=%+v", first, second)
	}
	var intentCount int
	var intentState string
	var payload []byte
	if err = runtime.pool.QueryRow(ctx, `SELECT count(*),min(state),min(payload::text) FROM notification_intents WHERE company_id=$1 AND intervention_id=$2`, scope.company, first.ID).Scan(&intentCount, &intentState, &payload); err != nil {
		t.Fatal(err)
	}
	if intentCount != 1 || intentState != "pending" {
		t.Fatalf("notification intent count/state = %d/%s", intentCount, intentState)
	}
	var safePayload map[string]any
	if err = json.Unmarshal(payload, &safePayload); err != nil {
		t.Fatal(err)
	}
	if safePayload["reason_code"] != "handover_required" || safePayload["raw_error"] != nil || safePayload["client_secret"] != nil {
		t.Fatalf("outbox payload is not safe metadata: %s", payload)
	}
	var deliveryCount int
	if err = runtime.pool.QueryRow(ctx, "SELECT count(*) FROM notification_deliveries WHERE company_id=$1 AND intent_id IN (SELECT id FROM notification_intents WHERE company_id=$1 AND intervention_id=$2)", scope.company, first.ID).Scan(&deliveryCount); err != nil {
		t.Fatal(err)
	}
	if deliveryCount != 0 {
		t.Fatalf("creating an intervention unexpectedly dispatched %d deliveries", deliveryCount)
	}
	acknowledged, err := runtime.TXSetHumanInterventionState(ctx, scope, first.ID, "acknowledged", "human-intervention-ack")
	if err != nil {
		t.Fatal(err)
	}
	if acknowledged.State != "acknowledged" {
		t.Fatalf("human acknowledgement state = %q", acknowledged.State)
	}
	if err = runtime.pool.QueryRow(ctx, `SELECT state FROM notification_intents WHERE company_id=$1 AND intervention_id=$2`, scope.company, first.ID).Scan(&intentState); err != nil {
		t.Fatal(err)
	}
	if intentState != "superseded" {
		t.Fatalf("acknowledgement did not supersede the pending reminder: %s", intentState)
	}
}

func TestQQOutboxRequiresQualifiedRouteAndNeverRetriesUnknownOutcome(t *testing.T) {
	dsn := os.Getenv("POLIS_TEST_DSN")
	if dsn == "" {
		t.Skip("dedicated PostgreSQL required")
	}
	ctx := context.Background()
	runtime, err := Open(ctx, dsn, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Close()
	scope, err := runtime.TXCreateCompany(ctx, "qq-outbox-"+newID())
	if err != nil {
		t.Fatal(err)
	}
	mission, err := runtime.TXCreateMissionGoal(ctx, scope, "QQ outbox", "verify the fake qualified route boundary", "qq-outbox-mission-"+newID())
	if err != nil {
		t.Fatal(err)
	}
	routeDraft := NotificationRouteInput{Adapter: "qq_official", Destination: "admin-openid", SafetyAlias: "Team A", CredentialRef: "default", Enabled: false}
	if _, err = runtime.TXConfigureNotificationRoute(ctx, scope, routeDraft, "qq-outbox-route-draft"); err != nil {
		t.Fatal(err)
	}
	intervention, err := runtime.TXCreateHumanIntervention(ctx, scope, HumanInterventionInput{MissionID: mission.ID, ProblemKey: "qq-outbox-problem", Severity: "high", ReasonCode: "handover_required", AffectedScope: "mission", ProtectionAction: "no_mutation", RequiredAction: "reauthorize_in_workbench"}, "qq-outbox-intervention")
	if err != nil {
		t.Fatal(err)
	}
	senderSession := qqnotify.SenderSessionInfo{InstallationID: "install-1", SenderEpoch: "sender-epoch-1"}
	if _, err = runtime.TXPrepareHumanInterventionDelivery(ctx, scope, intervention.ID, senderSession); !errors.Is(err, core.Conflict) {
		t.Fatalf("unqualified route dispatch error = %v, want conflict", err)
	}
	if _, err = runtime.pool.Exec(ctx, `INSERT INTO qq_channel_qualifications(company_id,route_id,route_revision,target_openid,account_fingerprint,qualification_status,evidence_digest,qualified_until)
VALUES($1,$2,1,$3,$4,'qualified',$5,$6)`, scope.company, "default-qq_official", routeDraft.Destination, fingerprint("bot-account"), fingerprint("offline-proactive-qualification"), time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	qualified := routeDraft
	qualified.Enabled = true
	if _, err = runtime.TXConfigureNotificationRoute(ctx, scope, qualified, "qq-outbox-route-enable"); err != nil {
		t.Fatal(err)
	}
	plan, err := runtime.TXPrepareHumanInterventionDelivery(ctx, scope, intervention.ID, senderSession)
	if err != nil {
		t.Fatal(err)
	}
	if plan.AttemptCount != 1 || plan.Route.TargetOpenID != routeDraft.Destination || plan.Grant.MessageSequence != 1 || plan.MessageDigest == "" {
		t.Fatalf("prepared QQ notification plan = %+v", plan)
	}
	completed, err := runtime.TXCompleteHumanInterventionDelivery(ctx, scope, plan, qqnotify.SendResult{Outcome: qqnotify.OutcomeUnknown, ErrorCode: "send_receipt_unknown"})
	if err != nil || completed.Status != "outcome_unknown" {
		t.Fatalf("unknown-send completion = %+v err=%v", completed, err)
	}
	if _, err = runtime.TXPrepareHumanInterventionDelivery(ctx, scope, intervention.ID, senderSession); !errors.Is(err, core.Conflict) {
		t.Fatalf("outcome_unknown was automatically retried: %v", err)
	}
	var remoteID *string
	var state string
	if err = runtime.pool.QueryRow(ctx, `SELECT state,remote_message_id FROM notification_deliveries WHERE company_id=$1 AND id=$2`, scope.company, plan.DeliveryID).Scan(&state, &remoteID); err != nil {
		t.Fatal(err)
	}
	if state != "outcome_unknown" || remoteID != nil {
		t.Fatalf("delivery readback invented or lost a receipt: state=%s remote_id=%v", state, remoteID)
	}

	interrupted, err := runtime.TXCreateHumanIntervention(ctx, scope, HumanInterventionInput{
		MissionID: mission.ID, ProblemKey: "qq-outbox-interrupted", Severity: "high", ReasonCode: "handover_required",
		AffectedScope: "mission", ProtectionAction: "no_mutation", RequiredAction: "reauthorize_in_workbench",
	}, "qq-outbox-interrupted-intervention")
	if err != nil {
		t.Fatal(err)
	}
	interruptedPlan, err := runtime.TXPrepareHumanInterventionDelivery(ctx, scope, interrupted.ID, senderSession)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = runtime.pool.Exec(ctx, `UPDATE notification_deliveries SET expires_at=clock_timestamp()-interval '1 second' WHERE company_id=$1 AND id=$2`, scope.company, interruptedPlan.DeliveryID); err != nil {
		t.Fatal(err)
	}
	if reconciled, reconcileErr := runtime.TXReconcileExpiredHumanInterventionSends(ctx); reconcileErr != nil || reconciled != 1 {
		t.Fatalf("expired in-flight reconciliation count=%d err=%v, want one closed attempt", reconciled, reconcileErr)
	}
	if _, err = runtime.TXPrepareHumanInterventionDelivery(ctx, scope, interrupted.ID, senderSession); !errors.Is(err, core.Conflict) {
		t.Fatalf("reconciled in-flight attempt dispatch error = %v, want conflict", err)
	}
	if err = runtime.pool.QueryRow(ctx, `SELECT state FROM notification_deliveries WHERE company_id=$1 AND id=$2`, scope.company, interruptedPlan.DeliveryID).Scan(&state); err != nil {
		t.Fatal(err)
	}
	if state != "outcome_unknown" {
		t.Fatalf("expired in-flight attempt remained stuck in %q", state)
	}
}

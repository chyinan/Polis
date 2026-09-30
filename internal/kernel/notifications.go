// pattern: Imperative Shell
package kernel

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"polis/internal/core"
	"polis/internal/qqnotify"
)

func notificationEventKind(kind string) string {
	switch {
	case kind == "mission.cancel", kind == "operator.instruction.created":
		return kind
	case strings.Contains(kind, "provider") && (strings.Contains(kind, "failed") || strings.Contains(kind, "inconclusive")):
		return kind
	case strings.Contains(kind, "artifact"):
		return kind
	default:
		return ""
	}
}

func appendNotificationIntent(ctx context.Context, tx pgx.Tx, s Scope, kind string, payload []byte) error {
	var subjectID string
	var object map[string]any
	if json.Unmarshal(payload, &object) == nil {
		if value, ok := object["id"].(string); ok {
			subjectID = value
		}
	}
	if subjectID == "" {
		subjectID = kind
	}
	_, err := tx.Exec(ctx, `INSERT INTO notification_intents(company_id,id,event_kind,subject_id,payload,state) VALUES($1,$2,$3,$4,$5,'pending')`, s.company, newID(), kind, subjectID, payload)
	return err
}

type NotificationRouteInput struct {
	Adapter       string
	Destination   string
	SafetyAlias   string
	CredentialRef string
	Enabled       bool
}

type NotificationTestReceipt struct {
	IntentID   string
	DeliveryID string
	Adapter    string
	State      string
}

func (k *Kernel) TXConfigureNotificationRoute(ctx context.Context, s Scope, input NotificationRouteInput, key string) (Receipt, error) {
	if input.Adapter != "local" && input.Adapter != "webhook" && input.Adapter != "qq_official" || len(input.Destination) > 512 {
		return Receipt{}, core.Malformed
	}
	if input.Adapter == "qq_official" && ((input.Destination != "" && !qqnotify.ValidTargetOpenID(input.Destination)) || (input.SafetyAlias != "" && !qqnotify.ValidSafetyAlias(input.SafetyAlias)) || (input.Destination != "" && input.SafetyAlias == "") || (input.CredentialRef != "" && !qqnotify.ValidCredentialRef(input.CredentialRef)) || (input.Destination != "" && input.CredentialRef == "")) {
		return Receipt{}, core.Malformed
	}
	return k.TXWrite(ctx, s, nil, key, "notification.route.update", input, func(tx pgx.Tx) (Receipt, error) {
		var exists bool
		if err := tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM companies WHERE id=$1)", s.company).Scan(&exists); err != nil {
			return Receipt{}, err
		}
		if !exists {
			return Receipt{}, core.OutOfScope
		}
		routeID := "default-" + input.Adapter
		destination := strings.TrimSpace(input.Destination)
		routeRevision := int64(1)
		qualificationStatus := "unverified"
		status := "unverified"
		var currentDestination, currentStatus, currentSafetyAlias, currentCredentialRef string
		var currentEnabled bool
		var currentRevision int64
		err := tx.QueryRow(ctx, `SELECT destination,enabled,route_revision,qualification_status,status,safety_alias,credential_ref
FROM notification_routes WHERE company_id=$1 AND adapter=$2 FOR UPDATE`, s.company, input.Adapter).Scan(&currentDestination, &currentEnabled, &currentRevision, &qualificationStatus, &currentStatus, &currentSafetyAlias, &currentCredentialRef)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return Receipt{}, err
		}
		if err == nil {
			routeRevision = currentRevision
			if input.Adapter == "qq_official" {
				if destination != currentDestination || input.SafetyAlias != currentSafetyAlias || input.CredentialRef != currentCredentialRef {
					routeRevision++
					qualificationStatus = "unverified"
				}
				if currentEnabled && !input.Enabled {
					routeRevision++
					qualificationStatus = "revoked"
				}
			} else if destination != currentDestination || input.Enabled != currentEnabled {
				routeRevision++
			}
		} else {
			qualificationStatus = "unverified"
		}
		qualifiedUntil := any(nil)
		if input.Adapter == "qq_official" {
			if input.Enabled {
				var expiresAt time.Time
				err = tx.QueryRow(ctx, `SELECT qualified_until FROM qq_channel_qualifications
WHERE company_id=$1 AND route_id=$2 AND route_revision=$3 AND target_openid=$4
AND qualification_status='qualified' AND qualified_until>clock_timestamp()`, s.company, routeID, routeRevision, destination).Scan(&expiresAt)
				if errors.Is(err, pgx.ErrNoRows) {
					return Receipt{}, core.ConflictError{Reason: "QQ route has no current proactive C2C qualification", CurrentState: "unverified"}
				}
				if err != nil {
					return Receipt{}, err
				}
				qualificationStatus, status, qualifiedUntil = "qualified", "ready", expiresAt
			} else if destination != "" {
				status = "configured"
			} else {
				status = "unverified"
			}
		} else if input.Enabled {
			status = "configured"
		} else {
			status = "unverified"
		}
		_, err = tx.Exec(ctx, `INSERT INTO notification_routes(company_id,id,adapter,enabled,destination,status,route_revision,qualification_status,qualified_until,safety_alias,credential_ref)
VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
ON CONFLICT(company_id,adapter) DO UPDATE SET enabled=EXCLUDED.enabled,destination=EXCLUDED.destination,status=EXCLUDED.status,route_revision=EXCLUDED.route_revision,qualification_status=EXCLUDED.qualification_status,qualified_until=EXCLUDED.qualified_until,safety_alias=EXCLUDED.safety_alias,credential_ref=EXCLUDED.credential_ref,updated_at=clock_timestamp()`,
			s.company, routeID, input.Adapter, input.Enabled, destination, status, routeRevision, qualificationStatus, qualifiedUntil, strings.TrimSpace(input.SafetyAlias), strings.TrimSpace(input.CredentialRef))
		return Receipt{ID: routeID, Status: status, Revision: routeRevision}, err
	})
}

func (k *Kernel) TXTestNotification(ctx context.Context, s Scope, key string) (NotificationTestReceipt, error) {
	selectedAdapter := ""
	receipt, err := k.TXWrite(ctx, s, nil, key, "notification.test", key, func(tx pgx.Tx) (Receipt, error) {
		var routeID, adapter string
		var enabled bool
		if err := tx.QueryRow(ctx, `SELECT id,adapter,enabled FROM notification_routes WHERE company_id=$1 AND enabled ORDER BY id LIMIT 1`, s.company).Scan(&routeID, &adapter, &enabled); errors.Is(err, pgx.ErrNoRows) {
			return Receipt{}, core.ConflictError{Reason: "notification route is not enabled", CurrentState: "disabled"}
		} else if err != nil {
			return Receipt{}, err
		} else if !enabled {
			return Receipt{}, core.Conflict
		}
		selectedAdapter = adapter
		if adapter == "qq_official" {
			return Receipt{}, core.ConflictError{Reason: "QQ sender dispatch is unavailable until an authorized sender qualification is connected", CurrentState: "not_dispatched"}
		}
		intentID := newID()
		if _, err := tx.Exec(ctx, `INSERT INTO notification_intents(company_id,id,event_kind,subject_id,payload,state) VALUES($1,$2,'notification.test',$2,'{"test":true}'::jsonb,'pending')`, s.company, intentID); err != nil {
			return Receipt{}, err
		}
		state := "delivered"
		if adapter != "local" {
			state = "accepted"
		}
		deliveryID := intentID
		if _, err := tx.Exec(ctx, `INSERT INTO notification_deliveries(company_id,id,intent_id,adapter,state) VALUES($1,$2,$3,$4,$5)`, s.company, deliveryID, intentID, adapter, state); err != nil {
			return Receipt{}, err
		}
		if _, err := tx.Exec(ctx, `UPDATE notification_intents SET state=$3 WHERE company_id=$1 AND id=$2`, s.company, intentID, state); err != nil {
			return Receipt{}, err
		}
		return Receipt{ID: intentID, Status: state}, nil
	})
	if err != nil {
		return NotificationTestReceipt{}, err
	}
	return notificationTestReceipt(selectedAdapter, receipt.ID, receipt.ID, receipt.Status), nil
}

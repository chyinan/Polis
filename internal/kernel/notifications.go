// pattern: Imperative Shell
package kernel

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
	"polis/internal/core"
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
	Adapter     string
	Destination string
	Enabled     bool
}

type NotificationTestReceipt struct {
	IntentID   string
	DeliveryID string
	Adapter    string
	State      string
}

func (k *Kernel) TXConfigureNotificationRoute(ctx context.Context, s Scope, input NotificationRouteInput, key string) (Receipt, error) {
	if input.Adapter != "local" && input.Adapter != "webhook" || len(input.Destination) > 512 {
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
		_, err := tx.Exec(ctx, `INSERT INTO notification_routes(company_id,id,adapter,enabled,destination,status)
VALUES($1,$2,$3,$4,$5,CASE WHEN $4 THEN 'configured' ELSE 'unverified' END)
ON CONFLICT(company_id,adapter) DO UPDATE SET enabled=EXCLUDED.enabled,destination=EXCLUDED.destination,status=EXCLUDED.status,updated_at=clock_timestamp()`, s.company, "default-"+input.Adapter, input.Adapter, input.Enabled, strings.TrimSpace(input.Destination))
		return Receipt{ID: "default-" + input.Adapter, Status: "configured"}, err
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

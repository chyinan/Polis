// pattern: Imperative Shell
package kernel

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

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
	Config      json.RawMessage
}

type NotificationTestReceipt struct {
	IntentID   string
	DeliveryID string
	Adapter    string
	State      string
}

type notificationRouteRecord struct {
	ID          string
	Adapter     string
	Enabled     bool
	Destination string
	Status      string
	Config      json.RawMessage
}

func normalizeNotificationConfig(adapter string, config json.RawMessage) (json.RawMessage, error) {
	if len(config) == 0 || string(bytes.TrimSpace(config)) == "" || string(bytes.TrimSpace(config)) == "null" {
		config = json.RawMessage(`{}`)
	}
	var obj map[string]any
	if err := json.Unmarshal(config, &obj); err != nil {
		return nil, core.Malformed
	}
	switch adapter {
	case "local":
		return json.RawMessage(`{}`), nil
	case "webhook":
		// keep only non-secret metadata; secret presence tracked separately
		out := map[string]any{}
		if v, ok := obj["signatureHeader"].(string); ok && strings.TrimSpace(v) != "" {
			out["signatureHeader"] = strings.TrimSpace(v)
		}
		return json.Marshal(out)
	case "qq_official":
		appID, _ := obj["appId"].(string)
		targetKind, _ := obj["targetKind"].(string)
		appID = strings.TrimSpace(appID)
		targetKind = strings.TrimSpace(strings.ToLower(targetKind))
		if appID == "" || (targetKind != "user" && targetKind != "group") {
			return nil, core.Malformed
		}
		out := map[string]any{"appId": appID, "targetKind": targetKind}
		if sandbox, ok := obj["sandbox"].(bool); ok {
			out["sandbox"] = sandbox
		}
		return json.Marshal(out)
	default:
		return nil, core.Malformed
	}
}

func validateNotificationDestination(adapter, destination string) (string, error) {
	destination = strings.TrimSpace(destination)
	if len(destination) > 512 {
		return "", core.Malformed
	}
	switch adapter {
	case "local":
		if destination == "" {
			destination = "local://workbench"
		}
		return destination, nil
	case "webhook":
		u, err := url.Parse(destination)
		if err != nil || u == nil || strings.TrimSpace(u.Scheme) == "" || strings.TrimSpace(u.Host) == "" {
			return "", core.Malformed
		}
		scheme := strings.ToLower(u.Scheme)
		if scheme != "https" && scheme != "http" {
			return "", core.Malformed
		}
		return destination, nil
	case "qq_official":
		if destination == "" || strings.ContainsAny(destination, " \t\r\n") {
			return "", core.Malformed
		}
		return destination, nil
	default:
		return "", core.Malformed
	}
}

func (k *Kernel) TXConfigureNotificationRoute(ctx context.Context, s Scope, input NotificationRouteInput, key string) (Receipt, error) {
	if input.Adapter != "local" && input.Adapter != "webhook" && input.Adapter != "qq_official" {
		return Receipt{}, core.Malformed
	}
	destination, err := validateNotificationDestination(input.Adapter, input.Destination)
	if err != nil {
		return Receipt{}, err
	}
	config, err := normalizeNotificationConfig(input.Adapter, input.Config)
	if err != nil {
		return Receipt{}, err
	}
	status := "unverified"
	if input.Enabled {
		status = "configured"
	}
	return k.TXWrite(ctx, s, nil, key, "notification.route.update", map[string]any{
		"adapter": input.Adapter, "destination": destination, "enabled": input.Enabled, "config": json.RawMessage(config),
	}, func(tx pgx.Tx) (Receipt, error) {
		var exists bool
		if err := tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM companies WHERE id=$1)", s.company).Scan(&exists); err != nil {
			return Receipt{}, err
		}
		if !exists {
			return Receipt{}, core.OutOfScope
		}
		_, err := tx.Exec(ctx, `INSERT INTO notification_routes(company_id,id,adapter,enabled,destination,status,config)
VALUES($1,$2,$3,$4,$5,$6,$7::jsonb)
ON CONFLICT(company_id,adapter) DO UPDATE SET enabled=EXCLUDED.enabled,destination=EXCLUDED.destination,status=EXCLUDED.status,config=EXCLUDED.config,updated_at=clock_timestamp()`,
			s.company, "default-"+input.Adapter, input.Adapter, input.Enabled, destination, status, config)
		return Receipt{ID: "default-" + input.Adapter, Status: status}, err
	})
}

type NotificationDispatchReceipt struct {
	Intents    int
	Dispatched int
	Delivered  int
	Failed     int
}

func (k *Kernel) TXTestNotification(ctx context.Context, s Scope, key, adapter string) (NotificationTestReceipt, error) {
	route, err := k.loadEnabledNotificationRoute(ctx, s.company, adapter)
	if err != nil {
		return NotificationTestReceipt{}, err
	}
	payload := map[string]any{"test": true, "companyId": s.company, "adapter": route.Adapter, "destination": route.Destination, "createdAt": time.Now().UTC().Format(time.RFC3339Nano)}
	rawPayload, _ := json.Marshal(payload)
	state, errCode, sendErr := k.executeNotificationSend(ctx, s.company, route, "notification.test", rawPayload)
	intentState := state
	deliveryState := state
	var errCodePtr *string
	if errCode != "" {
		errCodePtr = &errCode
	}
	receipt, err := k.TXWrite(ctx, s, nil, key, "notification.test", map[string]any{"adapter": route.Adapter, "state": state, "errorCode": errCode}, func(tx pgx.Tx) (Receipt, error) {
		intentID := newID()
		if _, err := tx.Exec(ctx, `INSERT INTO notification_intents(company_id,id,event_kind,subject_id,payload,state) VALUES($1,$2,'notification.test',$2,$3::jsonb,$4)`, s.company, intentID, rawPayload, intentState); err != nil {
			return Receipt{}, err
		}
		deliveryID := intentID
		if _, err := tx.Exec(ctx, `INSERT INTO notification_deliveries(company_id,id,intent_id,adapter,state,error_code) VALUES($1,$2,$3,$4,$5,$6)`, s.company, deliveryID, intentID, route.Adapter, deliveryState, errCodePtr); err != nil {
			return Receipt{}, err
		}
		return Receipt{ID: intentID, Status: deliveryState}, nil
	})
	if err != nil {
		return NotificationTestReceipt{}, err
	}
	if sendErr != nil && deliveryState != "delivered" {
		// keep receipt state; error recorded in delivery
	}
	return notificationTestReceipt(route.Adapter, receipt.ID, receipt.ID, receipt.Status), nil
}

type pendingNotificationIntent struct {
	ID        string
	EventKind string
	Payload   []byte
}

func (k *Kernel) TXDispatchPendingNotifications(ctx context.Context, s Scope, adapter string, limit int) (NotificationDispatchReceipt, error) {
	if adapter != "" && adapter != "local" && adapter != "webhook" && adapter != "qq_official" {
		return NotificationDispatchReceipt{}, core.Malformed
	}
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	routes, err := k.loadEnabledNotificationRoutes(ctx, s.company, adapter)
	if err != nil {
		return NotificationDispatchReceipt{}, err
	}
	if len(routes) == 0 {
		return NotificationDispatchReceipt{}, core.ConflictError{Reason: "notification route is not enabled", CurrentState: "disabled"}
	}
	intents, err := k.listPendingNotificationIntents(ctx, s.company, limit)
	if err != nil {
		return NotificationDispatchReceipt{}, err
	}
	receipt := NotificationDispatchReceipt{Intents: len(intents)}
	for _, intent := range intents {
		handled := false
		for _, route := range routes {
			exists, exErr := k.notificationDeliveryExists(ctx, s.company, intent.ID, route.Adapter)
			if exErr != nil {
				return NotificationDispatchReceipt{}, exErr
			}
			if exists {
				handled = true
				continue
			}
			state, errCode, _ := k.executeNotificationSend(ctx, s.company, route, intent.EventKind, intent.Payload)
			if insErr := k.insertNotificationDeliveryResult(ctx, s.company, intent.ID, route.Adapter, state, errCode); insErr != nil {
				return NotificationDispatchReceipt{}, insErr
			}
			handled = true
			receipt.Dispatched++
			if state == "delivered" {
				receipt.Delivered++
			} else {
				receipt.Failed++
			}
		}
		if handled {
			if err := k.refreshNotificationIntentState(ctx, s.company, intent.ID); err != nil {
				return NotificationDispatchReceipt{}, err
			}
		}
	}
	return receipt, nil
}

func (k *Kernel) listPendingNotificationIntents(ctx context.Context, companyID string, limit int) ([]pendingNotificationIntent, error) {
	rows, err := k.pool.Query(ctx, `SELECT id,event_kind,payload FROM notification_intents WHERE company_id=$1 AND state='pending' ORDER BY created_at,id LIMIT $2`, companyID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]pendingNotificationIntent, 0)
	for rows.Next() {
		var item pendingNotificationIntent
		var payload []byte
		if err := rows.Scan(&item.ID, &item.EventKind, &payload); err != nil {
			return nil, err
		}
		item.Payload = payload
		items = append(items, item)
	}
	return items, rows.Err()
}

func (k *Kernel) notificationDeliveryExists(ctx context.Context, companyID, intentID, adapter string) (bool, error) {
	var exists bool
	err := k.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM notification_deliveries WHERE company_id=$1 AND intent_id=$2 AND adapter=$3)`, companyID, intentID, adapter).Scan(&exists)
	return exists, err
}

func (k *Kernel) insertNotificationDeliveryResult(ctx context.Context, companyID, intentID, adapter, state, errCode string) error {
	var errCodePtr *string
	if errCode != "" {
		errCodePtr = &errCode
	}
	_, err := k.pool.Exec(ctx, `INSERT INTO notification_deliveries(company_id,id,intent_id,adapter,state,error_code) VALUES($1,$2,$3,$4,$5,$6)
ON CONFLICT DO NOTHING`, companyID, newID(), intentID, adapter, state, errCodePtr)
	return err
}

func (k *Kernel) refreshNotificationIntentState(ctx context.Context, companyID, intentID string) error {
	var delivered, failed, total int
	if err := k.pool.QueryRow(ctx, `SELECT count(*) FILTER (WHERE state='delivered'), count(*) FILTER (WHERE state='failed' OR state='unknown'), count(*) FROM notification_deliveries WHERE company_id=$1 AND intent_id=$2`, companyID, intentID).Scan(&delivered, &failed, &total); err != nil {
		return err
	}
	state := "pending"
	switch {
	case delivered > 0:
		state = "delivered"
	case total > 0 && failed == total:
		state = "failed"
	case total > 0:
		state = "pending"
	}
	_, err := k.pool.Exec(ctx, `UPDATE notification_intents SET state=$3 WHERE company_id=$1 AND id=$2`, companyID, intentID, state)
	return err
}

func (k *Kernel) loadEnabledNotificationRoutes(ctx context.Context, companyID, adapter string) ([]notificationRouteRecord, error) {
	query := `SELECT id,adapter,enabled,destination,status,config FROM notification_routes WHERE company_id=$1 AND enabled`
	args := []any{companyID}
	if strings.TrimSpace(adapter) != "" {
		query += ` AND adapter=$2`
		args = append(args, strings.TrimSpace(adapter))
	}
	query += ` ORDER BY id`
	rows, err := k.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	routes := make([]notificationRouteRecord, 0)
	for rows.Next() {
		var route notificationRouteRecord
		if err := rows.Scan(&route.ID, &route.Adapter, &route.Enabled, &route.Destination, &route.Status, &route.Config); err != nil {
			return nil, err
		}
		routes = append(routes, route)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return routes, nil
}

func (k *Kernel) loadEnabledNotificationRoute(ctx context.Context, companyID, adapter string) (notificationRouteRecord, error) {
	routes, err := k.loadEnabledNotificationRoutes(ctx, companyID, adapter)
	if err != nil {
		return notificationRouteRecord{}, err
	}
	if len(routes) == 0 {
		return notificationRouteRecord{}, core.ConflictError{Reason: "notification route is not enabled", CurrentState: "disabled"}
	}
	return routes[0], nil
}

func (k *Kernel) executeNotificationSend(ctx context.Context, companyID string, route notificationRouteRecord, eventKind string, payload []byte) (state string, errorCode string, err error) {
	switch route.Adapter {
	case "local":
		return "delivered", "", nil
	case "webhook":
		return sendWebhookNotification(ctx, companyID, route, eventKind, payload, k)
	case "qq_official":
		return sendQQOfficialNotification(ctx, companyID, route, payload, k)
	default:
		return "failed", "ADAPTER_UNSUPPORTED", core.Malformed
	}
}

type webhookSendConfig struct {
	SignatureHeader string `json:"signatureHeader"`
}

func sendWebhookNotification(ctx context.Context, companyID string, route notificationRouteRecord, eventKind string, payload []byte, k *Kernel) (string, string, error) {
	var cfg webhookSendConfig
	_ = json.Unmarshal(route.Config, &cfg)
	body := map[string]any{
		"eventKind":   eventKind,
		"companyId":   companyID,
		"adapter":     "webhook",
		"destination": route.Destination,
		"payload":     json.RawMessage(payload),
		"sentAt":      time.Now().UTC().Format(time.RFC3339Nano),
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return "failed", "PAYLOAD_ENCODE_FAILED", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, route.Destination, bytes.NewReader(raw))
	if err != nil {
		return "failed", "REQUEST_INVALID", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Polis-Webhook/1.0")
	if k != nil && k.NotificationSecretConfigured(companyID, "webhook") {
		secret, secErr := k.ReadNotificationSecret(companyID, "webhook")
		if secErr == nil && secret != "" {
			ts := fmt.Sprintf("%d", time.Now().Unix())
			mac := hmac.New(sha256.New, []byte(secret))
			_, _ = mac.Write([]byte(ts + "."))
			_, _ = mac.Write(raw)
			sig := hex.EncodeToString(mac.Sum(nil))
			header := strings.TrimSpace(cfg.SignatureHeader)
			if header == "" {
				header = "X-Polis-Signature-256"
			}
			req.Header.Set(header, "v1="+sig)
			req.Header.Set("X-Polis-Timestamp", ts)
		}
	}
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "failed", "WEBHOOK_REQUEST_FAILED", err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return "delivered", "", nil
	}
	return "failed", fmt.Sprintf("WEBHOOK_HTTP_%d", resp.StatusCode), fmt.Errorf("webhook status %d", resp.StatusCode)
}

type qqRouteConfig struct {
	AppID      string `json:"appId"`
	TargetKind string `json:"targetKind"`
	Sandbox    bool   `json:"sandbox"`
}

type qqAccessTokenResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   string `json:"expires_in"`
	Code        int    `json:"code"`
	Message     string `json:"message"`
}

type qqSendResponse struct {
	ID      string `json:"id"`
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func sendQQOfficialNotification(ctx context.Context, companyID string, route notificationRouteRecord, payload []byte, k *Kernel) (string, string, error) {
	if k == nil || !k.NotificationSecretConfigured(companyID, "qq_official") {
		return "failed", "QQ_SECRET_MISSING", core.ConflictError{Reason: "qq app secret is not configured", CurrentState: "missing_secret"}
	}
	var cfg qqRouteConfig
	if err := json.Unmarshal(route.Config, &cfg); err != nil {
		return "failed", "QQ_CONFIG_INVALID", err
	}
	cfg.AppID = strings.TrimSpace(cfg.AppID)
	cfg.TargetKind = strings.ToLower(strings.TrimSpace(cfg.TargetKind))
	if cfg.AppID == "" || (cfg.TargetKind != "user" && cfg.TargetKind != "group") || strings.TrimSpace(route.Destination) == "" {
		return "failed", "QQ_CONFIG_INVALID", core.Malformed
	}
	secret, err := k.ReadNotificationSecret(companyID, "qq_official")
	if err != nil || strings.TrimSpace(secret) == "" {
		return "failed", "QQ_SECRET_MISSING", err
	}
	token, err := getQQAccessToken(ctx, cfg.AppID, secret)
	if err != nil {
		return "failed", "QQ_TOKEN_FAILED", err
	}
	content := "Polis 通知测试"
	var p map[string]any
	if json.Unmarshal(payload, &p) == nil {
		if v, ok := p["content"].(string); ok && strings.TrimSpace(v) != "" {
			content = v
		} else {
			content = fmt.Sprintf("Polis 通知：%s", string(payload))
			if len([]rune(content)) > 200 {
				content = string([]rune(content)[:200]) + "…"
			}
		}
	}
	base := "https://api.sgroup.qq.com"
	if cfg.Sandbox {
		base = "https://sandbox.api.sgroup.qq.com"
	}
	path := fmt.Sprintf("/v2/users/%s/messages", url.PathEscape(route.Destination))
	if cfg.TargetKind == "group" {
		path = fmt.Sprintf("/v2/groups/%s/messages", url.PathEscape(route.Destination))
	}
	msgBody, _ := json.Marshal(map[string]any{"content": content, "msg_type": 0})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+path, bytes.NewReader(msgBody))
	if err != nil {
		return "failed", "QQ_REQUEST_INVALID", err
	}
	req.Header.Set("Authorization", "QQBot "+token)
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "failed", "QQ_REQUEST_FAILED", err
	}
	defer resp.Body.Close()
	rawResp, _ := io.ReadAll(io.LimitReader(resp.Body, 8192))
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		var sr qqSendResponse
		_ = json.Unmarshal(rawResp, &sr)
		if sr.Code != 0 && strings.TrimSpace(sr.Message) != "" {
			return "failed", fmt.Sprintf("QQ_API_%d", sr.Code), fmt.Errorf("qq api error: %s", sr.Message)
		}
		return "delivered", "", nil
	}
	var sr qqSendResponse
	_ = json.Unmarshal(rawResp, &sr)
	if sr.Code != 0 {
		return "failed", fmt.Sprintf("QQ_API_%d", sr.Code), fmt.Errorf("qq api error %d", sr.Code)
	}
	return "failed", fmt.Sprintf("QQ_HTTP_%d", resp.StatusCode), fmt.Errorf("qq status %d", resp.StatusCode)
}

func getQQAccessToken(ctx context.Context, appID, clientSecret string) (string, error) {
	body, _ := json.Marshal(map[string]string{"appId": appID, "clientSecret": clientSecret})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://bots.qq.com/app/getAppAccessToken", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 8192))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("qq token status %d", resp.StatusCode)
	}
	var tr qqAccessTokenResponse
	if err := json.Unmarshal(raw, &tr); err != nil {
		return "", err
	}
	if strings.TrimSpace(tr.AccessToken) == "" {
		if tr.Code != 0 || strings.TrimSpace(tr.Message) != "" {
			return "", fmt.Errorf("qq token error: %s", tr.Message)
		}
		return "", errors.New("qq token missing")
	}
	return tr.AccessToken, nil
}

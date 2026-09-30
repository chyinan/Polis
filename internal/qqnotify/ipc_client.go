// pattern: Imperative Shell
package qqnotify

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptrace"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type IPCClientConfig struct {
	BaseURL    string
	Key        []byte
	Policy     SenderPolicy
	HTTPClient *http.Client
	Now        func() time.Time
}

type DeliveryGrant struct {
	PermitID        string
	DeliveryID      string
	MessageSequence int64
	ExpiresAt       time.Time
}

func ReadSenderSession(ctx context.Context, baseURL string, key []byte, httpClient *http.Client) (SenderSessionInfo, error) {
	validatedURL, err := validatedLoopbackURL(baseURL)
	if err != nil || len(key) < 32 {
		return SenderSessionInfo{}, errors.New("authenticated sender session configuration is invalid")
	}
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 5 * time.Second}
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, validatedURL+"/v1/session", nil)
	if err != nil {
		return SenderSessionInfo{}, err
	}
	response, err := httpClient.Do(request)
	if err != nil {
		return SenderSessionInfo{}, errors.New("sender session is unavailable")
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return SenderSessionInfo{}, errors.New("sender session was not confirmed")
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, 2049))
	if err != nil || len(body) > 2048 {
		return SenderSessionInfo{}, errors.New("sender session response is invalid")
	}
	decoder := json.NewDecoder(strings.NewReader(string(body)))
	decoder.DisallowUnknownFields()
	var info SenderSessionInfo
	if err = decoder.Decode(&info); err != nil || decoder.Decode(new(any)) != io.EOF || VerifySenderSession(key, info) != nil {
		return SenderSessionInfo{}, errors.New("sender session signature is invalid")
	}
	return info, nil
}

type IPCClient struct {
	baseURL    string
	key        []byte
	policy     SenderPolicy
	policyMu   sync.Mutex
	httpClient *http.Client
	now        func() time.Time
}

func NewIPCClient(config IPCClientConfig) (*IPCClient, error) {
	baseURL, err := validatedLoopbackURL(config.BaseURL)
	if err != nil {
		return nil, err
	}
	if len(config.Key) < 32 || !validShortID(config.Policy.InstallationID) || !validShortID(config.Policy.SenderEpoch) {
		return nil, errors.New("authenticated notification IPC client is not configured")
	}
	httpClient := config.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 15 * time.Second}
	}
	now := config.Now
	if now == nil {
		now = time.Now
	}
	return &IPCClient{baseURL: baseURL, key: append([]byte(nil), config.Key...), policy: config.Policy, httpClient: httpClient, now: now}, nil
}

func (c *IPCClient) SendIntervention(ctx context.Context, route Route, intervention Intervention, grant DeliveryGrant) SendResult {
	c.policyMu.Lock()
	defer c.policyMu.Unlock()
	now := c.now()
	if err := ValidateRoute(route, now); err != nil || route.Revision != c.policy.RouteRevision || route.TargetOpenID != c.policy.TargetOpenID || !c.policy.RouteEnabled || c.policy.RouteStatus != "ready" || !now.Before(c.policy.QualifiedUntil) {
		return SendResult{Outcome: OutcomeRejected, ErrorCode: "route_not_ready"}
	}
	message, err := BuildInterventionMessage(intervention)
	if err != nil || !validShortID(grant.PermitID) || !validShortID(grant.DeliveryID) || grant.MessageSequence <= 0 || !grant.ExpiresAt.After(now) {
		return SendResult{Outcome: OutcomeRejected, ErrorCode: "send_grant_invalid"}
	}
	expiresAt := grant.ExpiresAt
	if maxExpiry := now.Add(MaxPermitLifetime); expiresAt.After(maxExpiry) {
		expiresAt = maxExpiry
	}
	permit := SendPermit{
		Purpose: SendPurposeC2CText, PermitID: grant.PermitID, InstallationID: c.policy.InstallationID,
		SenderEpoch: c.policy.SenderEpoch, DeliveryID: grant.DeliveryID, RouteRevision: route.Revision,
		TargetOpenID: route.TargetOpenID, CredentialRef: route.CredentialRef, Message: message, MessageSequence: grant.MessageSequence,
		IssuedAt: now, ExpiresAt: expiresAt,
	}
	signed, err := SignSendPermit(c.key, permit)
	if err != nil {
		return SendResult{Outcome: OutcomeRejected, ErrorCode: "send_grant_invalid"}
	}
	encoded, err := json.Marshal(signed)
	if err != nil {
		return SendResult{Outcome: OutcomeRejected, ErrorCode: "send_grant_invalid"}
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v1/send", strings.NewReader(string(encoded)))
	if err != nil {
		return SendResult{Outcome: OutcomeRejected, ErrorCode: "ipc_request_invalid"}
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")
	var requestWritten atomic.Bool
	trace := &httptrace.ClientTrace{WroteRequest: func(info httptrace.WroteRequestInfo) {
		if info.Err == nil {
			requestWritten.Store(true)
		}
	}}
	response, err := c.httpClient.Do(request.WithContext(httptrace.WithClientTrace(request.Context(), trace)))
	if err != nil {
		if requestWritten.Load() {
			return SendResult{Outcome: OutcomeUnknown, ErrorCode: "sender_receipt_unknown"}
		}
		return SendResult{Outcome: OutcomeRetryWait, ErrorCode: "sender_unavailable", RetryAfterSeconds: 30}
	}
	defer response.Body.Close()
	responseBody, readErr := io.ReadAll(io.LimitReader(response.Body, maxPermitBodyBytes+1))
	if readErr != nil || len(responseBody) > maxPermitBodyBytes {
		return SendResult{Outcome: OutcomeUnknown, HTTPStatus: response.StatusCode, ErrorCode: "sender_receipt_unknown"}
	}
	if response.StatusCode >= 200 && response.StatusCode < 300 {
		var result SendResult
		if json.Unmarshal(responseBody, &result) != nil || !validOutcome(result.Outcome) {
			return SendResult{Outcome: OutcomeUnknown, HTTPStatus: response.StatusCode, ErrorCode: "sender_receipt_invalid"}
		}
		return result
	}
	switch response.StatusCode {
	case http.StatusBadRequest, http.StatusForbidden, http.StatusConflict:
		return SendResult{Outcome: OutcomeRejected, HTTPStatus: response.StatusCode, ErrorCode: "sender_permit_rejected"}
	case http.StatusServiceUnavailable:
		return SendResult{Outcome: OutcomeRetryWait, HTTPStatus: response.StatusCode, ErrorCode: "sender_unavailable", RetryAfterSeconds: 30}
	default:
		return SendResult{Outcome: OutcomeUnknown, HTTPStatus: response.StatusCode, ErrorCode: "sender_receipt_unknown"}
	}
}

func (c *IPCClient) UpdatePolicy(ctx context.Context, next SenderPolicy) error {
	c.policyMu.Lock()
	defer c.policyMu.Unlock()
	update, err := SignPolicyUpdate(c.key, next)
	if err != nil {
		return err
	}
	if err = VerifyPolicyUpdate(c.key, update, c.policy, c.now()); err != nil {
		return err
	}
	// Freeze new sends locally before asking polisd to cross its final gate.
	localFreeze := c.policy
	localFreeze.RouteEnabled = false
	localFreeze.RouteStatus = "revocation_pending"
	c.policy = localFreeze
	encoded, err := json.Marshal(update)
	if err != nil {
		return err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v1/policy", strings.NewReader(string(encoded)))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")
	response, err := c.httpClient.Do(request)
	if err != nil {
		return errors.New("sender policy update receipt is unknown; local sending remains frozen")
	}
	defer response.Body.Close()
	responseBody, readErr := io.ReadAll(io.LimitReader(response.Body, maxPermitBodyBytes+1))
	if readErr != nil || len(responseBody) > maxPermitBodyBytes || response.StatusCode != http.StatusOK {
		return errors.New("sender policy update was not confirmed; local sending remains frozen")
	}
	var receipt struct {
		Status        string `json:"status"`
		RouteRevision int64  `json:"routeRevision"`
		RouteEnabled  bool   `json:"routeEnabled"`
	}
	if json.Unmarshal(responseBody, &receipt) != nil || receipt.Status != "applied" || receipt.RouteRevision != next.RouteRevision || receipt.RouteEnabled != next.RouteEnabled {
		return errors.New("sender policy update receipt is ambiguous; local sending remains frozen")
	}
	c.policy = next
	return nil
}

func validatedLoopbackURL(value string) (string, error) {
	parsed, err := url.Parse(value)
	if err != nil || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return "", errors.New("notification sender IPC URL is invalid")
	}
	if parsed.Hostname() == "" || parsed.Port() == "" {
		return "", errors.New("notification sender IPC URL requires an explicit loopback port")
	}
	ip := net.ParseIP(parsed.Hostname())
	if ip == nil || !ip.IsLoopback() || parsed.Path != "" {
		return "", errors.New("notification sender IPC must use a loopback IP URL")
	}
	return strings.TrimRight(parsed.String(), "/"), nil
}

func validOutcome(value Outcome) bool {
	switch value {
	case OutcomeProviderAccepted, OutcomeRetryWait, OutcomeRejected, OutcomeUnknown:
		return true
	default:
		return false
	}
}

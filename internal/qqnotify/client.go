// pattern: Imperative Shell
package qqnotify

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"
)

const (
	DefaultAPIBase       = "https://api.sgroup.qq.com"
	DefaultTokenURL      = "https://bots.qq.com/app/getAppAccessToken"
	refreshTokenEarlyBy  = 60 * time.Second
	maxProviderBodyBytes = 64 << 10
)

type Config struct {
	AppID           string
	ClientSecret    string
	CredentialRef   string
	LoadCredentials func(context.Context, string) (Credentials, error)
	APIBase         string
	TokenURL        string
	HTTPClient      *http.Client
	Now             func() time.Time
}

type SendResult struct {
	Outcome           Outcome
	HTTPStatus        int
	RemoteMessageID   string
	ErrorCode         string
	RetryAfterSeconds int
}

type Client struct {
	appID           string
	clientSecret    string
	credentialRef   string
	loadCredentials func(context.Context, string) (Credentials, error)
	apiBase         string
	tokenURL        string
	httpClient      *http.Client
	now             func() time.Time

	mu                     sync.Mutex
	accessToken            string
	tokenExpiresAt         time.Time
	tokenCredentialRef     string
	tokenAppID             string
	tokenSecretFingerprint string
}

func NewClient(config Config) *Client {
	apiBase := strings.TrimRight(config.APIBase, "/")
	if apiBase == "" {
		apiBase = DefaultAPIBase
	}
	tokenURL := strings.TrimSpace(config.TokenURL)
	if tokenURL == "" {
		tokenURL = DefaultTokenURL
	}
	httpClient := config.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 12 * time.Second}
	}
	credentialRef := strings.TrimSpace(config.CredentialRef)
	if credentialRef == "" {
		credentialRef = "default"
	}
	now := config.Now
	if now == nil {
		now = time.Now
	}
	return &Client{appID: config.AppID, clientSecret: config.ClientSecret, credentialRef: credentialRef, loadCredentials: config.LoadCredentials, apiBase: apiBase, tokenURL: tokenURL, httpClient: httpClient, now: now}
}

func (c *Client) SendIntervention(ctx context.Context, route Route, intervention Intervention, messageSequence int64) SendResult {
	message, err := BuildInterventionMessage(intervention)
	if err != nil {
		return SendResult{Outcome: OutcomeRejected, ErrorCode: "intervention_not_allowlisted"}
	}
	return c.sendC2CText(ctx, route, message, messageSequence)
}

func (c *Client) SendSignedPermit(ctx context.Context, key []byte, signed SignedSendPermit, policy SenderPolicy) SendResult {
	if err := VerifySendPermit(key, signed, policy, c.now()); err != nil {
		return SendResult{Outcome: OutcomeRejected, ErrorCode: "permit_rejected"}
	}
	route := Route{
		Adapter: AdapterName, Enabled: policy.RouteEnabled, Status: policy.RouteStatus,
		TargetOpenID: policy.TargetOpenID, CredentialRef: policy.CredentialRef, Revision: policy.RouteRevision,
		QualifiedUntil: policy.QualifiedUntil, MaxAttempts: 1,
	}
	return c.sendC2CText(ctx, route, signed.Permit.Message, signed.Permit.MessageSequence)
}

func (c *Client) sendC2CText(ctx context.Context, route Route, message string, messageSequence int64) SendResult {
	if err := ValidateRoute(route, c.now()); err != nil {
		return SendResult{Outcome: OutcomeRejected, ErrorCode: "route_not_ready"}
	}
	if !validOutboundText(message) || messageSequence <= 0 {
		return SendResult{Outcome: OutcomeRejected, ErrorCode: "send_request_invalid"}
	}
	credentials := Credentials{AppID: c.appID, ClientSecret: c.clientSecret}
	if c.loadCredentials != nil {
		loaded, err := c.loadCredentials(ctx, route.CredentialRef)
		if err != nil {
			return SendResult{Outcome: OutcomeRetryWait, ErrorCode: "credential_store_unavailable", RetryAfterSeconds: 60}
		}
		credentials = loaded
	} else if route.CredentialRef != c.credentialRef {
		return SendResult{Outcome: OutcomeRejected, ErrorCode: "credential_reference_mismatch"}
	}
	if err := credentials.Validate(); err != nil {
		return SendResult{Outcome: OutcomeRejected, ErrorCode: "credentials_unavailable"}
	}
	token, tokenStatus, tokenErr := c.ensureToken(ctx, route.CredentialRef, credentials)
	if tokenErr != nil {
		if tokenStatus >= 400 && tokenStatus < 500 {
			return SendResult{Outcome: OutcomeRejected, HTTPStatus: tokenStatus, ErrorCode: "credentials_rejected"}
		}
		return SendResult{Outcome: OutcomeRetryWait, HTTPStatus: tokenStatus, ErrorCode: "token_unavailable", RetryAfterSeconds: 30}
	}

	requestBody, err := json.Marshal(struct {
		MessageType int    `json:"msg_type"`
		Sequence    int64  `json:"msg_seq"`
		Content     string `json:"content"`
	}{MessageType: 0, Sequence: messageSequence, Content: message})
	if err != nil {
		return SendResult{Outcome: OutcomeRejected, ErrorCode: "send_request_invalid"}
	}
	requestURL := c.apiBase + "/v2/users/" + url.PathEscape(route.TargetOpenID) + "/messages"
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, requestURL, strings.NewReader(string(requestBody)))
	if err != nil {
		return SendResult{Outcome: OutcomeRejected, ErrorCode: "send_request_invalid"}
	}
	request.Header.Set("Authorization", "QQBot "+token)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")
	response, err := c.httpClient.Do(request)
	if err != nil {
		return SendResult{Outcome: OutcomeUnknown, ErrorCode: "send_receipt_unknown"}
	}
	defer response.Body.Close()
	responseBody, readErr := io.ReadAll(io.LimitReader(response.Body, maxProviderBodyBytes+1))
	if readErr != nil || len(responseBody) > maxProviderBodyBytes {
		return SendResult{Outcome: OutcomeUnknown, HTTPStatus: response.StatusCode, ErrorCode: "send_receipt_unreadable"}
	}
	return classifySendResponse(response.StatusCode, response.Header, responseBody)
}

func (c *Client) ensureToken(ctx context.Context, credentialRef string, credentials Credentials) (string, int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	credentialFingerprint := digestText(credentials.ClientSecret)
	if c.tokenCredentialRef != credentialRef || c.tokenAppID != credentials.AppID || c.tokenSecretFingerprint != credentialFingerprint {
		c.accessToken = ""
		c.tokenExpiresAt = time.Time{}
	}
	if c.accessToken != "" && c.now().Before(c.tokenExpiresAt.Add(-refreshTokenEarlyBy)) {
		return c.accessToken, 0, nil
	}
	body, err := json.Marshal(struct {
		AppID        string `json:"appId"`
		ClientSecret string `json:"clientSecret"`
	}{AppID: credentials.AppID, ClientSecret: credentials.ClientSecret})
	if err != nil {
		return "", 0, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.tokenURL, strings.NewReader(string(body)))
	if err != nil {
		return "", 0, err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")
	response, err := c.httpClient.Do(request)
	if err != nil {
		return "", 0, err
	}
	defer response.Body.Close()
	responseBody, readErr := io.ReadAll(io.LimitReader(response.Body, maxProviderBodyBytes+1))
	if readErr != nil || len(responseBody) > maxProviderBodyBytes {
		return "", response.StatusCode, errors.New("token response unreadable")
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return "", response.StatusCode, fmt.Errorf("token endpoint returned HTTP %d", response.StatusCode)
	}
	var tokenResponse struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int64  `json:"expires_in"`
	}
	if json.Unmarshal(responseBody, &tokenResponse) != nil || tokenResponse.AccessToken == "" || tokenResponse.ExpiresIn <= 0 {
		return "", response.StatusCode, errors.New("token response is malformed")
	}
	c.accessToken = tokenResponse.AccessToken
	c.tokenExpiresAt = c.now().Add(time.Duration(tokenResponse.ExpiresIn) * time.Second)
	c.tokenCredentialRef = credentialRef
	c.tokenAppID = credentials.AppID
	c.tokenSecretFingerprint = credentialFingerprint
	return c.accessToken, response.StatusCode, nil
}

func classifySendResponse(status int, headers http.Header, body []byte) SendResult {
	if status == http.StatusTooManyRequests {
		return SendResult{Outcome: OutcomeRetryWait, HTTPStatus: status, ErrorCode: "rate_limited", RetryAfterSeconds: parseRetryAfter(headers.Get("Retry-After"))}
	}
	if status == http.StatusRequestTimeout || status == http.StatusTooEarly || status >= 500 {
		return SendResult{Outcome: OutcomeUnknown, HTTPStatus: status, ErrorCode: "send_receipt_unknown"}
	}
	if status < 200 || status >= 300 {
		return SendResult{Outcome: OutcomeRejected, HTTPStatus: status, ErrorCode: safeHTTPErrorCode(status)}
	}
	var receipt struct {
		ID   string `json:"id"`
		Code *int   `json:"code"`
	}
	if err := json.Unmarshal(body, &receipt); err != nil {
		return SendResult{Outcome: OutcomeUnknown, HTTPStatus: status, ErrorCode: "send_receipt_unknown"}
	}
	if receipt.Code != nil && *receipt.Code != 0 {
		return SendResult{Outcome: OutcomeRejected, HTTPStatus: status, ErrorCode: "provider_rejected"}
	}
	if receipt.ID == "" && receipt.Code == nil {
		return SendResult{Outcome: OutcomeUnknown, HTTPStatus: status, ErrorCode: "send_receipt_unknown"}
	}
	return SendResult{Outcome: OutcomeProviderAccepted, HTTPStatus: status, RemoteMessageID: receipt.ID}
}

func parseRetryAfter(value string) int {
	seconds, err := strconv.Atoi(strings.TrimSpace(value))
	if err == nil && seconds > 0 {
		if seconds > MaxRetryAfterSeconds {
			return MaxRetryAfterSeconds
		}
		return seconds
	}
	if when, parseErr := http.ParseTime(value); parseErr == nil {
		seconds = int(time.Until(when).Seconds())
		if seconds < 1 {
			return 1
		}
		if seconds > MaxRetryAfterSeconds {
			return MaxRetryAfterSeconds
		}
		return seconds
	}
	return 60
}

func safeHTTPErrorCode(status int) string {
	switch status {
	case http.StatusBadRequest:
		return "request_rejected"
	case http.StatusUnauthorized:
		return "credentials_rejected"
	case http.StatusForbidden:
		return "permission_rejected"
	case http.StatusNotFound:
		return "target_rejected"
	case http.StatusConflict:
		return "request_conflict"
	case http.StatusRequestEntityTooLarge:
		return "payload_rejected"
	default:
		return "provider_rejected"
	}
}

func validOutboundText(value string) bool {
	if strings.TrimSpace(value) == "" || len([]rune(value)) > MaxInterventionRunes || strings.Contains(value, "@all") || strings.Contains(value, "@everyone") {
		return false
	}
	for _, character := range value {
		if character == '\n' || character == '\t' {
			continue
		}
		if unicode.IsControl(character) {
			return false
		}
	}
	return true
}

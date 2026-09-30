// pattern: Imperative Shell
package qqnotify

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestBuildInterventionMessageUsesOnlyRedactedAllowlistedFields(t *testing.T) {
	message, err := BuildInterventionMessage(Intervention{
		CompanyAlias: "研发一组", IncidentID: "INC-123", Severity: "high", ReasonCode: "credential_unavailable",
		ProtectionAction: "execution_paused", RequiredAction: "reauthorize_in_workbench", ObservedAt: time.Date(2026, 9, 23, 10, 30, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(message, "INC-123") || !strings.Contains(message, "已暂停相关执行") || !strings.Contains(message, "检查并重新授权") {
		t.Fatalf("safe intervention message is incomplete: %q", message)
	}
	for _, forbidden := range []string{"password", "client_secret", "token", "shell", "同意", "继续", "取消使命"} {
		if strings.Contains(strings.ToLower(message), strings.ToLower(forbidden)) {
			t.Fatalf("message contains forbidden content %q: %s", forbidden, message)
		}
	}
}

func TestBuildInterventionMessageRejectsUnmappedReasonAndUnsafeAlias(t *testing.T) {
	valid := Intervention{CompanyAlias: "Team A", IncidentID: "INC-1", Severity: "high", ReasonCode: "credential_unavailable", ProtectionAction: "execution_paused", RequiredAction: "reauthorize_in_workbench", ObservedAt: time.Now()}
	invalid := valid
	invalid.ReasonCode = "raw_error: token=secret"
	if _, err := BuildInterventionMessage(invalid); err == nil {
		t.Fatal("accepted an unmapped reason code")
	}
	invalid = valid
	invalid.CompanyAlias = "Team\n@all"
	if _, err := BuildInterventionMessage(invalid); err == nil {
		t.Fatal("accepted an unsafe display alias")
	}
}

func TestValidateRouteRequiresEnabledQualifiedExplicitC2CTarget(t *testing.T) {
	now := time.Date(2026, 9, 23, 10, 0, 0, 0, time.UTC)
	route := Route{Adapter: "qq_official", Enabled: true, Status: "ready", TargetOpenID: "admin-openid", CredentialRef: "default", Revision: 2, QualifiedUntil: now.Add(time.Hour), MaxAttempts: 3}
	if err := ValidateRoute(route, now); err != nil {
		t.Fatal(err)
	}
	for name, change := range map[string]func(*Route){
		"disabled":      func(r *Route) { r.Enabled = false },
		"unqualified":   func(r *Route) { r.Status = "unverified" },
		"no-target":     func(r *Route) { r.TargetOpenID = "" },
		"expired":       func(r *Route) { r.QualifiedUntil = now },
		"other-adapter": func(r *Route) { r.Adapter = "webhook" },
		"unbounded":     func(r *Route) { r.MaxAttempts = 100 },
	} {
		t.Run(name, func(t *testing.T) {
			candidate := route
			change(&candidate)
			if err := ValidateRoute(candidate, now); err == nil {
				t.Fatal("accepted an invalid QQ route")
			}
		})
	}
}

func TestClientRefreshesTokenAndSendsOnlyAdminC2CText(t *testing.T) {
	var tokenCalls atomic.Int32
	var sendCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/token":
			tokenCalls.Add(1)
			var input map[string]string
			if err := json.NewDecoder(request.Body).Decode(&input); err != nil || input["appId"] != "test-app" || input["clientSecret"] != "test-secret" {
				t.Errorf("token request payload = %#v, err=%v", input, err)
			}
			response.Header().Set("Content-Type", "application/json")
			_, _ = response.Write([]byte(`{"access_token":"token-value","expires_in":7200}`))
		case "/v2/users/admin-openid/messages":
			sendCalls.Add(1)
			if request.Header.Get("Authorization") != "QQBot token-value" {
				t.Errorf("Authorization = %q", request.Header.Get("Authorization"))
			}
			var payload map[string]any
			if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
				t.Errorf("decode C2C body: %v", err)
			}
			if payload["msg_type"] != float64(0) || payload["content"] != "[handover] review required" || payload["msg_seq"].(float64) < 42 {
				t.Errorf("C2C payload = %#v", payload)
			}
			if _, hasGroup := payload["group_id"]; hasGroup {
				t.Error("C2C payload included a group target")
			}
			if _, hasReply := payload["msg_id"]; hasReply {
				t.Error("proactive notice included a reply message ID")
			}
			response.Header().Set("Content-Type", "application/json")
			_, _ = response.Write([]byte(`{"id":"remote-message-1"}`))
		default:
			http.NotFound(response, request)
		}
	}))
	defer server.Close()

	client := NewClient(Config{AppID: "test-app", ClientSecret: "test-secret", APIBase: server.URL, TokenURL: server.URL + "/token", HTTPClient: server.Client()})
	route := Route{Adapter: "qq_official", Enabled: true, Status: "ready", TargetOpenID: "admin-openid", CredentialRef: "default", Revision: 2, QualifiedUntil: time.Now().Add(time.Hour), MaxAttempts: 3}
	first := client.sendC2CText(context.Background(), route, "[handover] review required", 42)
	if first.Outcome != OutcomeProviderAccepted || first.RemoteMessageID != "remote-message-1" || first.HTTPStatus != http.StatusOK {
		t.Fatalf("send result = %+v", first)
	}
	client.mu.Lock()
	client.tokenExpiresAt = time.Now().Add(30 * time.Second)
	client.mu.Unlock()
	second := client.sendC2CText(context.Background(), route, "[handover] review required", 43)
	if second.Outcome != OutcomeProviderAccepted || tokenCalls.Load() != 2 || sendCalls.Load() != 2 {
		t.Fatalf("token refresh or second send mismatch: result=%+v token_calls=%d send_calls=%d", second, tokenCalls.Load(), sendCalls.Load())
	}
}

func TestClientClassifiesExplicitRejectRateLimitAndUnknownReceiptWithoutRetry(t *testing.T) {
	for _, testCase := range []struct {
		name        string
		status      int
		body        string
		retryAfter  string
		wantOutcome Outcome
		wantRetry   int
	}{
		{name: "permission-rejected", status: http.StatusForbidden, body: `{"code":403,"message":"forbidden"}`, wantOutcome: OutcomeRejected},
		{name: "rate-limited", status: http.StatusTooManyRequests, body: `{"code":429,"message":"slow down"}`, retryAfter: "17", wantOutcome: OutcomeRetryWait, wantRetry: 17},
		{name: "unknown-success-body", status: http.StatusOK, body: `not-json`, wantOutcome: OutcomeUnknown},
		{name: "ambiguous-server-error", status: http.StatusBadGateway, body: `{"message":"upstream reset"}`, wantOutcome: OutcomeUnknown},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			var sendCalls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
				if request.URL.Path == "/token" {
					_, _ = response.Write([]byte(`{"access_token":"token-value","expires_in":7200}`))
					return
				}
				sendCalls.Add(1)
				if testCase.retryAfter != "" {
					response.Header().Set("Retry-After", testCase.retryAfter)
				}
				response.WriteHeader(testCase.status)
				_, _ = response.Write([]byte(testCase.body))
			}))
			defer server.Close()
			client := NewClient(Config{AppID: "app", ClientSecret: "secret", APIBase: server.URL, TokenURL: server.URL + "/token", HTTPClient: server.Client()})
			route := Route{Adapter: "qq_official", Enabled: true, Status: "ready", TargetOpenID: "admin-openid", CredentialRef: "default", Revision: 1, QualifiedUntil: time.Now().Add(time.Hour), MaxAttempts: 3}
			result := client.sendC2CText(context.Background(), route, "safe body", 1)
			if result.Outcome != testCase.wantOutcome || result.RetryAfterSeconds != testCase.wantRetry || sendCalls.Load() != 1 {
				t.Fatalf("result=%+v, sends=%d", result, sendCalls.Load())
			}
		})
	}
}

func TestClientMarksLostSendReceiptOutcomeUnknownWithoutAutomaticRetry(t *testing.T) {
	var sendCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/token" {
			_, _ = response.Write([]byte(`{"access_token":"token-value","expires_in":7200}`))
			return
		}
		sendCalls.Add(1)
		connection, _, err := response.(http.Hijacker).Hijack()
		if err == nil {
			_ = connection.Close()
		}
	}))
	defer server.Close()
	client := NewClient(Config{AppID: "app", ClientSecret: "secret", APIBase: server.URL, TokenURL: server.URL + "/token", HTTPClient: server.Client()})
	route := Route{Adapter: "qq_official", Enabled: true, Status: "ready", TargetOpenID: "admin-openid", CredentialRef: "default", Revision: 1, QualifiedUntil: time.Now().Add(time.Hour), MaxAttempts: 3}
	result := client.sendC2CText(context.Background(), route, "safe body", 1)
	if result.Outcome != OutcomeUnknown || sendCalls.Load() != 1 {
		t.Fatalf("lost receipt result=%+v, sends=%d", result, sendCalls.Load())
	}
}

func TestParseRetryAfterClampsPlatformDelay(t *testing.T) {
	if got := parseRetryAfter("99999"); got != MaxRetryAfterSeconds {
		t.Fatalf("retry-after clamp = %d, want %d", got, MaxRetryAfterSeconds)
	}
	if got := parseRetryAfter(strconv.Itoa(MaxRetryAfterSeconds)); got != MaxRetryAfterSeconds {
		t.Fatalf("retry-after = %d", got)
	}
}

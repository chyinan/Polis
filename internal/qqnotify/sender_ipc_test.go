// pattern: Imperative Shell
package qqnotify

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

type fakePermitTransport struct {
	mu      sync.Mutex
	calls   int
	result  SendResult
	entered chan struct{}
	resume  chan struct{}
}

func (f *fakePermitTransport) SendSignedPermit(ctx context.Context, key []byte, signed SignedSendPermit, policy SenderPolicy) SendResult {
	f.mu.Lock()
	f.calls++
	f.mu.Unlock()
	if f.entered != nil {
		close(f.entered)
		select {
		case <-f.resume:
		case <-ctx.Done():
			return SendResult{Outcome: OutcomeUnknown, ErrorCode: "send_receipt_unknown"}
		}
	}
	return f.result
}

func (f *fakePermitTransport) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls
}

func TestSenderIPCAuthenticatesAndConsumesPermitOnce(t *testing.T) {
	now := time.Now().UTC()
	key := []byte(strings.Repeat("i", 32))
	transport := &fakePermitTransport{result: SendResult{Outcome: OutcomeProviderAccepted, HTTPStatus: 200, RemoteMessageID: "remote-1"}}
	server := newTestSenderIPC(t, key, testSenderPolicy(now), transport)
	permit := testSendPermit(now)
	signed, err := SignSendPermit(key, permit)
	if err != nil {
		t.Fatal(err)
	}
	response := sendPermitRequest(t, server, signed)
	if response.Code != http.StatusOK || transport.callCount() != 1 {
		t.Fatalf("first send status=%d calls=%d body=%s", response.Code, transport.callCount(), response.Body.String())
	}
	var result SendResult
	if err = json.NewDecoder(response.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	if result.Outcome != OutcomeProviderAccepted || result.RemoteMessageID != "remote-1" {
		t.Fatalf("sender result = %+v", result)
	}
	replay := sendPermitRequest(t, server, signed)
	if replay.Code != http.StatusConflict || transport.callCount() != 1 {
		t.Fatalf("replayed permit status=%d calls=%d", replay.Code, transport.callCount())
	}
}

func TestSenderIPCRejectsBadSignatureAndOldRouteAfterRevocation(t *testing.T) {
	now := time.Now().UTC()
	key := []byte(strings.Repeat("i", 32))
	transport := &fakePermitTransport{result: SendResult{Outcome: OutcomeProviderAccepted}}
	server := newTestSenderIPC(t, key, testSenderPolicy(now), transport)
	permit := testSendPermit(now)
	signed, err := SignSendPermit(key, permit)
	if err != nil {
		t.Fatal(err)
	}
	tampered := signed
	tampered.Permit.TargetOpenID = "other-admin"
	if response := sendPermitRequest(t, server, tampered); response.Code != http.StatusForbidden {
		t.Fatalf("bad signature status = %d", response.Code)
	}
	revoked := testSenderPolicy(now)
	revoked.RouteStatus = "revoked"
	revoked.RouteEnabled = false
	if err = server.UpdatePolicy(revoked); err != nil {
		t.Fatal(err)
	}
	if response := sendPermitRequest(t, server, signed); response.Code != http.StatusForbidden {
		t.Fatalf("old permit after revocation status = %d", response.Code)
	}
	if transport.callCount() != 0 {
		t.Fatalf("sender was invoked %d times after denied permits", transport.callCount())
	}
}

func TestSenderIPCRevocationWaitsForFinalDispatchGate(t *testing.T) {
	now := time.Now().UTC()
	key := []byte(strings.Repeat("i", 32))
	transport := &fakePermitTransport{result: SendResult{Outcome: OutcomeUnknown, ErrorCode: "send_receipt_unknown"}, entered: make(chan struct{}), resume: make(chan struct{})}
	server := newTestSenderIPC(t, key, testSenderPolicy(now), transport)
	signed, err := SignSendPermit(key, testSendPermit(now))
	if err != nil {
		t.Fatal(err)
	}
	responseChannel := make(chan *httptest.ResponseRecorder, 1)
	go func() { responseChannel <- sendPermitRequest(t, server, signed) }()
	<-transport.entered
	policy := testSenderPolicy(now)
	policy.RouteStatus = "revoked"
	policy.RouteEnabled = false
	updated := make(chan error, 1)
	go func() { updated <- server.UpdatePolicy(policy) }()
	select {
	case err = <-updated:
		t.Fatalf("revocation crossed an in-flight dispatch boundary early: %v", err)
	case <-time.After(20 * time.Millisecond):
	}
	close(transport.resume)
	response := <-responseChannel
	if response.Code != http.StatusOK {
		t.Fatalf("in-flight permit result status = %d", response.Code)
	}
	if err = <-updated; err != nil {
		t.Fatal(err)
	}
	if after := sendPermitRequest(t, server, signed); after.Code != http.StatusForbidden || transport.callCount() != 1 {
		t.Fatalf("permit after revocation status=%d calls=%d", after.Code, transport.callCount())
	}
}

func newTestSenderIPC(t *testing.T, key []byte, policy SenderPolicy, transport PermitTransport) *SenderIPCServer {
	t.Helper()
	server, err := NewSenderIPCServer(SenderIPCConfig{Key: key, Policy: policy, Guard: NewMemoryPermitGuard(16), Transport: transport})
	if err != nil {
		t.Fatal(err)
	}
	return server
}

func sendPermitRequest(t *testing.T, server *SenderIPCServer, permit SignedSendPermit) *httptest.ResponseRecorder {
	t.Helper()
	encoded, err := json.Marshal(permit)
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/v1/send", bytes.NewReader(encoded))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)
	return response
}

func testSenderPolicy(now time.Time) SenderPolicy {
	return SenderPolicy{InstallationID: "install-1", SenderEpoch: "epoch-1", RouteRevision: 7, TargetOpenID: "admin-openid", CredentialRef: "default", RouteEnabled: true, RouteStatus: "ready", QualifiedUntil: now.Add(time.Hour)}
}

func testSendPermit(now time.Time) SendPermit {
	return SendPermit{Purpose: SendPurposeC2CText, PermitID: "permit-1", InstallationID: "install-1", SenderEpoch: "epoch-1", DeliveryID: "delivery-1", RouteRevision: 7, TargetOpenID: "admin-openid", CredentialRef: "default", Message: "safe notice", MessageSequence: 9, IssuedAt: now, ExpiresAt: now.Add(20 * time.Second)}
}

func TestSenderIPCRejectsMalformedBodyWithoutTransport(t *testing.T) {
	key := []byte(strings.Repeat("i", 32))
	transport := &fakePermitTransport{result: SendResult{Outcome: OutcomeProviderAccepted}}
	server := newTestSenderIPC(t, key, testSenderPolicy(time.Now()), transport)
	request := httptest.NewRequest(http.MethodPost, "/v1/send", strings.NewReader(`{"permit":{},"unexpected":true}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)
	body, _ := io.ReadAll(response.Body)
	if response.Code != http.StatusBadRequest || transport.callCount() != 0 || len(body) == 0 {
		t.Fatalf("malformed request status=%d calls=%d body=%s", response.Code, transport.callCount(), string(body))
	}
}

func TestIPCClientAndPolisdFakeTransportCompleteOneSignedC2CPermit(t *testing.T) {
	now := time.Now().UTC()
	key := []byte(strings.Repeat("p", 32))
	policy := testSenderPolicy(now)
	transport := &fakePermitTransport{result: SendResult{Outcome: OutcomeProviderAccepted, HTTPStatus: http.StatusOK}}
	sender := newTestSenderIPC(t, key, policy, transport)
	httpServer := httptest.NewServer(sender.Handler())
	defer httpServer.Close()
	client, err := NewIPCClient(IPCClientConfig{BaseURL: httpServer.URL, Key: key, Policy: policy})
	if err != nil {
		t.Fatal(err)
	}
	route := Route{Adapter: AdapterName, Enabled: true, Status: "ready", TargetOpenID: "admin-openid", CredentialRef: "default", Revision: 7, QualifiedUntil: now.Add(time.Hour), MaxAttempts: 3}
	intervention := Intervention{CompanyAlias: "Polis Team", IncidentID: "INC-12", Severity: "high", ReasonCode: "credential_unavailable", ProtectionAction: "execution_paused", RequiredAction: "reauthorize_in_workbench", ObservedAt: now}
	result := client.SendIntervention(context.Background(), route, intervention, DeliveryGrant{PermitID: "permit-ipc-1", DeliveryID: "delivery-ipc-1", MessageSequence: 1, ExpiresAt: now.Add(20 * time.Second)})
	if result.Outcome != OutcomeProviderAccepted || transport.callCount() != 1 {
		t.Fatalf("IPC sender result=%+v, transport calls=%d", result, transport.callCount())
	}
	if _, err = NewIPCClient(IPCClientConfig{BaseURL: "http://example.com:8099", Key: key, Policy: policy}); err == nil {
		t.Fatal("accepted a non-loopback notification IPC URL")
	}
}

func TestAuthenticatedPolicyUpdateRevokesSenderAndLocallyFreezesDispatch(t *testing.T) {
	now := time.Now().UTC()
	key := []byte(strings.Repeat("r", 32))
	policy := testSenderPolicy(now)
	transport := &fakePermitTransport{result: SendResult{Outcome: OutcomeProviderAccepted}}
	sender := newTestSenderIPC(t, key, policy, transport)
	httpServer := httptest.NewServer(sender.Handler())
	defer httpServer.Close()
	client, err := NewIPCClient(IPCClientConfig{BaseURL: httpServer.URL, Key: key, Policy: policy})
	if err != nil {
		t.Fatal(err)
	}
	revoked := policy
	revoked.RouteEnabled = false
	revoked.RouteStatus = "revoked"
	if err = client.UpdatePolicy(context.Background(), revoked); err != nil {
		t.Fatal(err)
	}
	intervention := Intervention{CompanyAlias: "Polis Team", IncidentID: "INC-13", Severity: "high", ReasonCode: "credential_unavailable", ProtectionAction: "execution_paused", RequiredAction: "reauthorize_in_workbench", ObservedAt: now}
	result := client.SendIntervention(context.Background(), Route{Adapter: AdapterName, Enabled: true, Status: "ready", TargetOpenID: "admin-openid", CredentialRef: "default", Revision: 7, QualifiedUntil: now.Add(time.Hour), MaxAttempts: 3}, intervention, DeliveryGrant{PermitID: "permit-revoked", DeliveryID: "delivery-revoked", MessageSequence: 1, ExpiresAt: now.Add(20 * time.Second)})
	if result.Outcome != OutcomeRejected || result.ErrorCode != "route_not_ready" || transport.callCount() != 0 {
		t.Fatalf("revoked client still dispatched: result=%+v calls=%d", result, transport.callCount())
	}
}

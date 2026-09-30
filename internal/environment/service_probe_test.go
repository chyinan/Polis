// pattern: Imperative Shell
package environment

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"
	"time"
)

type serviceOwnerVerifierFunc func(context.Context, int, string, uint16) error

func (fn serviceOwnerVerifierFunc) VerifyServiceEndpointOwner(ctx context.Context, processID int, bindAddress string, port uint16) error {
	return fn(ctx, processID, bindAddress, port)
}

func (serviceOwnerVerifierFunc) VerifyServiceEndpointConnectionOwner(ctx context.Context, processID int, bindAddress string, port uint16, clientAddress string, clientPort uint16) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if processID <= 0 || (bindAddress != "127.0.0.1" && bindAddress != "::1") || port == 0 || clientPort == 0 || net.ParseIP(clientAddress) == nil || !net.ParseIP(clientAddress).IsLoopback() {
		return ErrServiceEndpointOwnerUnverified
	}
	return nil
}

type unverifiedConnectionOwner struct{ serviceOwnerVerifierFunc }

func (unverifiedConnectionOwner) VerifyServiceEndpointConnectionOwner(context.Context, int, string, uint16, string, uint16) error {
	return errors.New("established connection belongs to another process")
}

func TestProbeServiceEndpointBindsHealthResponseToOwnedLoopbackProcess(t *testing.T) {
	const processID = 4242
	body := []byte("ready\n")
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet || request.URL.Path != "/healthz" {
			t.Errorf("request=%s %s, want GET /healthz", request.Method, request.URL.Path)
		}
		response.WriteHeader(http.StatusOK)
		_, _ = response.Write(body)
	}))
	defer server.Close()
	spec := serviceProbeSpecForTest(t, server.URL, body)
	ownerChecks := 0
	probe, err := ProbeServiceEndpoint(context.Background(), processID, spec, serviceOwnerVerifierFunc(func(_ context.Context, gotPID int, address string, port uint16) error {
		ownerChecks++
		if gotPID != processID || address != spec.BindAddress || port != spec.Port {
			t.Fatalf("owner check=(%d,%s,%d), want=(%d,%s,%d)", gotPID, address, port, processID, spec.BindAddress, spec.Port)
		}
		return nil
	}))
	if err != nil {
		t.Fatal(err)
	}
	wantHash := digestServiceProbeBytes(body)
	if probe.Readiness != ServiceReady || probe.HTTPStatus != http.StatusOK || probe.ResponseSHA256 != wantHash || probe.ProcessID != processID || probe.HealthcheckSHA256 == "" || probe.LeaseExpiresAt.Before(time.Now()) {
		t.Fatalf("service probe evidence=%+v", probe)
	}
	if ownerChecks != 2 {
		t.Fatalf("process ownership checked %d times, want before and after the HTTP probe", ownerChecks)
	}
}

func TestProbeServiceEndpointNeverFollowsRedirects(t *testing.T) {
	redirectTargetCalls := 0
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		redirectTargetCalls++
		_, _ = w.Write([]byte("redirect target"))
	}))
	defer target.Close()
	redirect := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Redirect(w, &http.Request{}, target.URL, http.StatusFound)
	}))
	defer redirect.Close()
	spec := serviceProbeSpecForTest(t, redirect.URL, []byte("unused"))
	probe, err := ProbeServiceEndpoint(context.Background(), 7, spec, serviceOwnerVerifierFunc(func(context.Context, int, string, uint16) error { return nil }))
	if err != nil {
		t.Fatal(err)
	}
	if probe.Readiness != ServiceUnhealthy || probe.ReasonCode != "http_status_mismatch" || probe.HTTPStatus != http.StatusFound || probe.LeaseExpiresAt.IsZero() || redirectTargetCalls != 0 {
		t.Fatalf("redirect result=%+v target calls=%d", probe, redirectTargetCalls)
	}
}

func TestProbeServiceEndpointFailsClosedWithoutProcessOwnership(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { requests++; _, _ = w.Write([]byte("ready")) }))
	defer server.Close()
	spec := serviceProbeSpecForTest(t, server.URL, []byte("ready"))
	probe, err := ProbeServiceEndpoint(context.Background(), 11, spec, serviceOwnerVerifierFunc(func(context.Context, int, string, uint16) error {
		return ErrServiceEndpointOwnerUnverified
	}))
	if !errors.Is(err, ErrServiceEndpointOwnerUnverified) || probe.Readiness != ServiceUnhealthy || probe.ReasonCode != "endpoint_owner_unverified" || requests != 0 {
		t.Fatalf("unowned endpoint probe=(%+v,%v), HTTP requests=%d", probe, err, requests)
	}
}

func TestProbeServiceEndpointRejectsOwnerChangeDuringResponse(t *testing.T) {
	body := []byte("ready")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write(body) }))
	defer server.Close()
	spec := serviceProbeSpecForTest(t, server.URL, body)
	checks := 0
	probe, err := ProbeServiceEndpoint(context.Background(), 11, spec, serviceOwnerVerifierFunc(func(context.Context, int, string, uint16) error {
		checks++
		if checks == 2 {
			return errors.New("listener owner changed")
		}
		return nil
	}))
	if !errors.Is(err, ErrServiceEndpointOwnerUnverified) || probe.Readiness != ServiceUnhealthy || probe.ReasonCode != "endpoint_owner_changed" || checks != 2 {
		t.Fatalf("owner-change probe=(%+v,%v), checks=%d", probe, err, checks)
	}
}

func TestProbeServiceEndpointRejectsHTTPBeforeUnverifiedConnectionIsSent(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests++
		_, _ = w.Write([]byte("ready"))
	}))
	defer server.Close()
	spec := serviceProbeSpecForTest(t, server.URL, []byte("ready"))
	probe, err := ProbeServiceEndpoint(context.Background(), 11, spec, unverifiedConnectionOwner{serviceOwnerVerifierFunc(func(context.Context, int, string, uint16) error { return nil })})
	if !errors.Is(err, ErrServiceEndpointOwnerUnverified) || probe.Readiness != ServiceUnhealthy || requests != 0 {
		t.Fatalf("unverified accepted connection reached HTTP handler: result=(%+v,%v) requests=%d", probe, err, requests)
	}
}

func TestProbeServiceEndpointDoesNotBecomeReadyAfterOwnerCheckDeadline(t *testing.T) {
	body := []byte("ready")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write(body) }))
	defer server.Close()
	spec := serviceProbeSpecForTest(t, server.URL, body)
	spec.TimeoutMS = 100
	checks := 0
	probe, err := ProbeServiceEndpoint(context.Background(), 17, spec, serviceOwnerVerifierFunc(func(context.Context, int, string, uint16) error {
		checks++
		if checks == 2 {
			time.Sleep(150 * time.Millisecond) // Deliberately returns success after the probe context expired.
		}
		return nil
	}))
	if !errors.Is(err, context.DeadlineExceeded) || probe.Readiness == ServiceReady || !probe.LeaseExpiresAt.IsZero() {
		t.Fatalf("expired owner check produced readiness evidence=(%+v,%v)", probe, err)
	}
}

func TestProbeServiceEndpointRejectsNonLoopbackAndOversizedResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write(make([]byte, MaxServiceProbeBodyBytes+1)) }))
	defer server.Close()
	spec := serviceProbeSpecForTest(t, server.URL, []byte("unused"))
	nonLoopback := spec
	nonLoopback.BindAddress = "192.0.2.1"
	if _, err := ProbeServiceEndpoint(context.Background(), 1, nonLoopback, serviceOwnerVerifierFunc(func(context.Context, int, string, uint16) error { return nil })); err == nil {
		t.Fatal("non-loopback readiness endpoint was accepted")
	}
	probe, err := ProbeServiceEndpoint(context.Background(), 1, spec, serviceOwnerVerifierFunc(func(context.Context, int, string, uint16) error { return nil }))
	if err != nil || probe.Readiness != ServiceUnhealthy || probe.ReasonCode != "response_too_large" {
		t.Fatalf("oversized response result=(%+v,%v)", probe, err)
	}
}

func TestProbeServiceEndpointRejectsPathInjection(t *testing.T) {
	spec := ServiceProbeSpec{
		BindAddress: "127.0.0.1", Port: 8080, Path: "/healthz\r\nHost: example.com",
		ExpectedStatusCode: http.StatusOK, ExpectedBodySHA256: digestServiceProbeBytes([]byte("ok")),
		TimeoutMS: 1000, LeaseDurationMS: 30_000,
	}
	verifications := 0
	if _, err := ProbeServiceEndpoint(context.Background(), 7, spec, serviceOwnerVerifierFunc(func(context.Context, int, string, uint16) error {
		verifications++
		return nil
	})); !errors.Is(err, ErrInvalidServiceProbe) || verifications != 0 {
		t.Fatalf("path injection accepted: error=%v owner checks=%d", err, verifications)
	}
}

func TestProbeServiceEndpointBoundsTimeoutAndResponseDigest(t *testing.T) {
	body := []byte("expected")
	slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(300 * time.Millisecond)
		_, _ = w.Write(body)
	}))
	defer slow.Close()
	spec := serviceProbeSpecForTest(t, slow.URL, body)
	spec.TimeoutMS = 100
	probe, err := ProbeServiceEndpoint(context.Background(), 1, spec, serviceOwnerVerifierFunc(func(context.Context, int, string, uint16) error { return nil }))
	if (err != nil && !errors.Is(err, context.DeadlineExceeded)) || probe.Readiness != ServiceUnhealthy || probe.ReasonCode != "probe_timeout" {
		t.Fatalf("timeout result=(%+v,%v)", probe, err)
	}
	fast := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("different")) }))
	defer fast.Close()
	spec = serviceProbeSpecForTest(t, fast.URL, body)
	probe, err = ProbeServiceEndpoint(context.Background(), 1, spec, serviceOwnerVerifierFunc(func(context.Context, int, string, uint16) error { return nil }))
	if err != nil || probe.Readiness != ServiceUnhealthy || probe.ReasonCode != "response_digest_mismatch" {
		t.Fatalf("digest mismatch result=(%+v,%v)", probe, err)
	}
}

func serviceProbeSpecForTest(t *testing.T, serverURL string, expectedBody []byte) ServiceProbeSpec {
	t.Helper()
	parsed, err := url.Parse(serverURL)
	if err != nil {
		t.Fatal(err)
	}
	address, portText, err := net.SplitHostPort(parsed.Host)
	if err != nil {
		t.Fatal(err)
	}
	port, err := strconv.Atoi(portText)
	if err != nil {
		t.Fatal(err)
	}
	hash := sha256.Sum256(expectedBody)
	return ServiceProbeSpec{
		BindAddress: address, Port: uint16(port), Path: "/healthz", ExpectedStatusCode: http.StatusOK,
		ExpectedBodySHA256: hex.EncodeToString(hash[:]), TimeoutMS: 1000, LeaseDurationMS: 30_000,
	}
}

func TestServiceProbeSpecDigestIsCanonical(t *testing.T) {
	spec := ServiceProbeSpec{BindAddress: "127.0.0.1", Port: 8080, Path: "/healthz", ExpectedStatusCode: 200, ExpectedBodySHA256: digestServiceProbeBytes([]byte("ok")), TimeoutMS: 1000, LeaseDurationMS: 30_000}
	first, err := ServiceProbeSpecSHA256(spec)
	if err != nil {
		t.Fatal(err)
	}
	second, err := ServiceProbeSpecSHA256(spec)
	if err != nil || first != second || len(first) != 64 {
		t.Fatalf("canonical service probe digest=(%s,%v), second=%s", first, err, second)
	}
	if _, err = ServiceProbeSpecSHA256(ServiceProbeSpec{}); err == nil {
		t.Fatal("invalid empty service probe spec was accepted")
	}
}

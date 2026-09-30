// pattern: Functional Core
package control

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"polis/internal/environment"
	"polis/internal/runner"
)

type serviceBrowserOwnerVerifier struct {
	processID int
	address   string
	port      uint16
	checks    atomic.Int32
}

func (owner *serviceBrowserOwnerVerifier) VerifyServiceEndpointOwner(_ context.Context, processID int, address string, port uint16) error {
	owner.checks.Add(1)
	if processID != owner.processID || address != owner.address || port != owner.port {
		return environment.ErrServiceEndpointOwnerUnverified
	}
	return nil
}

func (owner *serviceBrowserOwnerVerifier) VerifyServiceEndpointConnectionOwner(_ context.Context, processID int, address string, port uint16, localAddress string, localPort uint16) error {
	owner.checks.Add(1)
	if processID != owner.processID || address != owner.address || port != owner.port || net.ParseIP(localAddress) == nil || !net.ParseIP(localAddress).IsLoopback() || localPort == 0 {
		return environment.ErrServiceEndpointOwnerUnverified
	}
	return nil
}

func TestServiceBrowserIngressUsesOneTimeTicketAndPinnedUpstream(t *testing.T) {
	var calls atomic.Int32
	var ingressCookieName string
	var service *httptest.Server
	service = httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		calls.Add(1)
		if request.Host != "127.0.0.1:"+strconv.Itoa(servicePort(service)) {
			t.Errorf("upstream host = %q", request.Host)
		}
		if request.URL.Path != "/app" {
			t.Errorf("upstream path = %q", request.URL.Path)
		}
		appCookie, cookieErr := request.Cookie("app_session")
		if cookieErr != nil || appCookie.Value != "keep" {
			t.Errorf("application cookie was not forwarded: cookie=%v error=%v", appCookie, cookieErr)
		}
		response.Header().Add("Set-Cookie", "app_result=kept; Domain=127.0.0.1; Path=/")
		if ingressCookieName != "" {
			response.Header().Add("Set-Cookie", ingressCookieName+"=overwrite; Path=/")
		}
		_, _ = io.WriteString(response, "browser-visible-app")
	}))
	defer service.Close()
	address, port := serviceAddress(t, service)
	owner := &serviceBrowserOwnerVerifier{processID: os.Getpid(), address: address, port: port}
	ingress, err := NewServiceBrowserIngress(os.Getpid(), environment.ServiceProbeSpec{
		BindAddress: address, Port: port, Path: "/health", ExpectedStatusCode: http.StatusOK,
		ExpectedBodySHA256: sha256Hex("ok"), TimeoutMS: 500, LeaseDurationMS: 5000,
	}, owner)
	if err != nil {
		t.Fatal(err)
	}
	ingressCookieName = ingress.cookieName
	t.Cleanup(func() { _ = ingress.Close() })

	session, err := ingress.CreateSession("browser-session-1")
	if err != nil {
		t.Fatal(err)
	}
	target, err := url.Parse(session.URL)
	if err != nil || target.Scheme != "http" || target.Host == "" || !strings.HasPrefix(target.Path, "/_polis/open/") {
		t.Fatalf("browser session URL = %+v, error=%v", target, err)
	}
	client := &http.Client{CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}
	unauthorized, err := client.Get(target.Scheme + "://" + target.Host + "/app")
	if err != nil {
		t.Fatal(err)
	}
	_ = unauthorized.Body.Close()
	if unauthorized.StatusCode != http.StatusUnauthorized {
		t.Fatalf("direct service request status=%d, want unauthorized", unauthorized.StatusCode)
	}
	openResponse, err := client.Get(session.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer openResponse.Body.Close()
	if openResponse.StatusCode != http.StatusOK || openResponse.Header.Get("Content-Type") != "text/html; charset=utf-8" || len(openResponse.Cookies()) != 1 {
		t.Fatalf("open response status=%d headers=%v", openResponse.StatusCode, openResponse.Header)
	}
	if openResponse.Header.Get("Referrer-Policy") != "no-referrer" || openResponse.Header.Get("Cache-Control") != "no-store" {
		t.Fatalf("ticket response headers were not protected: %v", openResponse.Header)
	}
	bootstrap, err := io.ReadAll(openResponse.Body)
	if err != nil || !strings.Contains(string(bootstrap), "location.replace('/')") {
		t.Fatalf("ticket bootstrap body=%q error=%v", bootstrap, err)
	}
	cookie := openResponse.Cookies()[0]
	if !cookie.HttpOnly || cookie.SameSite != http.SameSiteStrictMode || cookie.Path != "/" {
		t.Fatalf("browser session cookie = %+v", cookie)
	}
	secondOpen, err := client.Get(session.URL)
	if err != nil {
		t.Fatal(err)
	}
	_ = secondOpen.Body.Close()
	if secondOpen.StatusCode != http.StatusNotFound {
		t.Fatalf("reused one-time ticket status=%d, want not found", secondOpen.StatusCode)
	}
	if calls.Load() != 0 {
		t.Fatalf("one-time ticket was reused; upstream calls=%d", calls.Load())
	}
	request, _ := http.NewRequest(http.MethodGet, target.Scheme+"://"+target.Host+"/app", nil)
	request.AddCookie(cookie)
	request.AddCookie(&http.Cookie{Name: "app_session", Value: "keep"})
	request.Header.Set("Sec-Fetch-Site", "same-origin")
	appResponse, err := client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer appResponse.Body.Close()
	body, err := io.ReadAll(appResponse.Body)
	if err != nil || appResponse.StatusCode != http.StatusOK || string(body) != "browser-visible-app" {
		t.Fatalf("proxied browser response status=%d body=%q error=%v", appResponse.StatusCode, body, err)
	}
	if owner.checks.Load() < 3 {
		t.Fatalf("service process ownership was not checked around browser traffic: checks=%d", owner.checks.Load())
	}
	for _, responseCookie := range appResponse.Cookies() {
		if responseCookie.Name == ingressCookieName {
			t.Fatal("upstream replaced the ingress authorization cookie")
		}
		if responseCookie.Name == "app_result" && responseCookie.Domain != "" {
			t.Fatalf("upstream cookie domain escaped through the gateway: %+v", responseCookie)
		}
	}
	if calls.Load() != 1 {
		t.Fatalf("upstream calls = %d, want one", calls.Load())
	}
}

func TestServiceBrowserIngressRejectsCrossSiteMutationAndClosesSessions(t *testing.T) {
	service := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		response.WriteHeader(http.StatusNoContent)
	}))
	defer service.Close()
	address, port := serviceAddress(t, service)
	owner := &serviceBrowserOwnerVerifier{processID: os.Getpid(), address: address, port: port}
	ingress, err := NewServiceBrowserIngress(os.Getpid(), environment.ServiceProbeSpec{
		BindAddress: address, Port: port, Path: "/health", ExpectedStatusCode: http.StatusOK,
		ExpectedBodySHA256: sha256Hex("ok"), TimeoutMS: 500, LeaseDurationMS: 5000,
	}, owner)
	if err != nil {
		t.Fatal(err)
	}
	session, err := ingress.CreateSession("browser-session-2")
	if err != nil {
		_ = ingress.Close()
		t.Fatal(err)
	}
	client := &http.Client{CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}
	openResponse, err := client.Get(session.URL)
	if err != nil {
		_ = ingress.Close()
		t.Fatal(err)
	}
	cookie := openResponse.Cookies()[0]
	_ = openResponse.Body.Close()
	target, _ := url.Parse(session.URL)
	mutation, _ := http.NewRequest(http.MethodDelete, target.Scheme+"://"+target.Host+"/mutate", strings.NewReader("x"))
	mutation.AddCookie(cookie)
	mutation.Header.Set("Origin", "https://attacker.invalid")
	mutation.Header.Set("Sec-Fetch-Site", "same-origin")
	blocked, err := client.Do(mutation)
	if err != nil {
		t.Fatal(err)
	}
	_ = blocked.Body.Close()
	if blocked.StatusCode != http.StatusForbidden {
		t.Fatalf("cross-site mutation status = %d", blocked.StatusCode)
	}
	sameSiteFetch, _ := http.NewRequest(http.MethodGet, target.Scheme+"://"+target.Host+"/mutate-via-get", nil)
	sameSiteFetch.AddCookie(cookie)
	sameSiteFetch.Header.Set("Sec-Fetch-Site", "same-site")
	blockedRead, err := client.Do(sameSiteFetch)
	if err != nil {
		t.Fatal(err)
	}
	_ = blockedRead.Body.Close()
	if blockedRead.StatusCode != http.StatusForbidden {
		t.Fatalf("same-site cross-origin read status=%d, want forbidden", blockedRead.StatusCode)
	}
	if err = ingress.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err = ingress.CreateSession("browser-session-after-close"); err == nil {
		t.Fatal("closed ingress issued a new session")
	}
}

func TestServiceBrowserIngressRequestIDReplayReturnsSameSession(t *testing.T) {
	service := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(response, "ok")
	}))
	defer service.Close()
	address, port := serviceAddress(t, service)
	owner := &serviceBrowserOwnerVerifier{processID: os.Getpid(), address: address, port: port}
	ingress, err := NewServiceBrowserIngress(os.Getpid(), environment.ServiceProbeSpec{
		BindAddress: address, Port: port, Path: "/health", ExpectedStatusCode: http.StatusOK,
		ExpectedBodySHA256: sha256Hex("ok"), TimeoutMS: 500, LeaseDurationMS: 5000,
	}, owner)
	if err != nil {
		t.Fatal(err)
	}
	defer ingress.Close()
	first, err := ingress.CreateSession("browser-session-replay")
	if err != nil {
		t.Fatal(err)
	}
	replay, err := ingress.CreateSession("browser-session-replay")
	if err != nil || replay.URL != first.URL || !replay.ExpiresAt.Equal(first.ExpiresAt) {
		t.Fatalf("session replay=%+v first=%+v error=%v", replay, first, err)
	}
}

func TestServiceBrowserIngressRejectsOversizedServiceResponse(t *testing.T) {
	service := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		response.Header().Set("Content-Length", strconv.Itoa(serviceBrowserMaxResponseBytes+1))
		response.WriteHeader(http.StatusOK)
	}))
	defer service.Close()
	address, port := serviceAddress(t, service)
	owner := &serviceBrowserOwnerVerifier{processID: os.Getpid(), address: address, port: port}
	ingress, err := NewServiceBrowserIngress(os.Getpid(), environment.ServiceProbeSpec{
		BindAddress: address, Port: port, Path: "/health", ExpectedStatusCode: http.StatusOK,
		ExpectedBodySHA256: sha256Hex("ok"), TimeoutMS: 500, LeaseDurationMS: 5000,
	}, owner)
	if err != nil {
		t.Fatal(err)
	}
	defer ingress.Close()
	client := &http.Client{CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}
	openResponse, err := client.Get(mustBrowserSessionURL(t, ingress, "browser-session-size-limit"))
	if err != nil {
		t.Fatal(err)
	}
	cookie := openResponse.Cookies()[0]
	_ = openResponse.Body.Close()
	request, _ := http.NewRequest(http.MethodGet, ingress.origin+"/large", nil)
	request.AddCookie(cookie)
	request.Header.Set("Sec-Fetch-Site", "same-origin")
	response, err := client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if response.StatusCode != http.StatusBadGateway {
		t.Fatalf("oversized upstream response status=%d, want bad gateway", response.StatusCode)
	}
}

func TestServiceBrowserIngressRejectsUnknownLengthResponseAboveLimit(t *testing.T) {
	service := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		flusher, ok := response.(http.Flusher)
		if !ok {
			t.Fatal("test server response does not support flushing")
		}
		response.WriteHeader(http.StatusOK)
		flusher.Flush()
		_, _ = response.Write(make([]byte, serviceBrowserMaxResponseBytes+1))
	}))
	defer service.Close()
	address, port := serviceAddress(t, service)
	owner := &serviceBrowserOwnerVerifier{processID: os.Getpid(), address: address, port: port}
	ingress, err := NewServiceBrowserIngress(os.Getpid(), environment.ServiceProbeSpec{
		BindAddress: address, Port: port, Path: "/health", ExpectedStatusCode: http.StatusOK,
		ExpectedBodySHA256: sha256Hex("ok"), TimeoutMS: 500, LeaseDurationMS: 5000,
	}, owner)
	if err != nil {
		t.Fatal(err)
	}
	defer ingress.Close()
	client := &http.Client{CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}
	openResponse, err := client.Get(mustBrowserSessionURL(t, ingress, "browser-session-chunked-limit"))
	if err != nil {
		t.Fatal(err)
	}
	cookie := openResponse.Cookies()[0]
	_ = openResponse.Body.Close()
	request, _ := http.NewRequest(http.MethodGet, ingress.origin+"/chunked-large", nil)
	request.AddCookie(cookie)
	request.Header.Set("Sec-Fetch-Site", "same-origin")
	response, err := client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	body, readErr := io.ReadAll(response.Body)
	_ = response.Body.Close()
	if response.StatusCode != http.StatusBadGateway || len(body) > 1024 || readErr != nil {
		t.Fatalf("chunked oversized response status=%d bodyBytes=%d readError=%v", response.StatusCode, len(body), readErr)
	}
	if strings.Contains(string(body), fmt.Sprintf("%d", serviceBrowserMaxResponseBytes)) {
		t.Fatalf("chunked oversized response body should be a bounded gateway error: %q", body)
	}
}

func TestCloseProjectJobsRevokesBrowserIngressBeforeStoppingService(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(response, "ok")
	}))
	defer upstream.Close()
	address, port := serviceAddress(t, upstream)
	owner := &serviceBrowserOwnerVerifier{processID: os.Getpid(), address: address, port: port}
	ingress, err := NewServiceBrowserIngress(os.Getpid(), environment.ServiceProbeSpec{
		BindAddress: address, Port: port, Path: "/health", ExpectedStatusCode: http.StatusOK,
		ExpectedBodySHA256: sha256Hex("ok"), TimeoutMS: 500, LeaseDurationMS: 5000,
	}, owner)
	if err != nil {
		t.Fatal(err)
	}
	session, err := ingress.CreateSession("browser-session-shutdown")
	if err != nil {
		_ = ingress.Close()
		t.Fatal(err)
	}
	executor := &shutdownOrderingProjectJobExecutor{check: func() error {
		client := &http.Client{Timeout: time.Second, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}
		response, requestErr := client.Get(session.URL)
		if requestErr == nil {
			defer response.Body.Close()
			if response.StatusCode == http.StatusSeeOther {
				return errors.New("browser ingress session remained usable when JobRun stop began")
			}
		}
		return nil
	}}
	active := &activeProjectJob{
		companyID: "company-1", jobID: "service-job-1", process: &shutdownIngressTestProcess{},
		service: &activeProjectService{browserIngress: ingress}, cancel: func() {},
	}
	serviceControl := &Service{projectJobRunner: executor, projectJobs: map[string]*activeProjectJob{active.jobID: active}}
	if err = serviceControl.closeProjectJobs(); err != nil {
		t.Fatal(err)
	}
	if !executor.stopCalled || executor.stopError != nil || active.service.browserIngress != nil {
		t.Fatalf("shutdown stopCalled=%v stopError=%v ingress=%v", executor.stopCalled, executor.stopError, active.service.browserIngress)
	}
}

func TestCreateProjectJobBrowserSessionDeniedAfterShutdownBegins(t *testing.T) {
	active := &activeProjectJob{
		companyID: "company-1", jobID: "service-job-1", process: &shutdownIngressTestProcess{},
		service: &activeProjectService{ready: make(chan struct{})}, cancel: func() {},
	}
	close(active.service.ready)
	service := &Service{projectJobsClosed: true, projectJobs: map[string]*activeProjectJob{active.jobID: active}}
	if _, err := service.CreateProjectJobBrowserSession(context.Background(), active.companyID, active.jobID, CreateProjectJobBrowserSessionRequest{RequestID: "browser-session-race"}); err == nil {
		t.Fatal("browser session was issued after service shutdown began")
	}
}

type shutdownOrderingProjectJobExecutor struct {
	check      func() error
	stopCalled bool
	stopError  error
}

func (*shutdownOrderingProjectJobExecutor) SupportsProjectJobProfile(string) bool { return true }
func (*shutdownOrderingProjectJobExecutor) HasPreparedEnvironment(string) bool    { return true }
func (*shutdownOrderingProjectJobExecutor) LaunchProjectJob(context.Context, ProjectJobExecutionRequest) (runner.AppContainerProcess, error) {
	return nil, errors.New("unexpected project launch")
}
func (executor *shutdownOrderingProjectJobExecutor) StopProjectJob(context.Context, string, string) error {
	executor.stopCalled = true
	if executor.check != nil {
		executor.stopError = executor.check()
	}
	return executor.stopError
}
func (*shutdownOrderingProjectJobExecutor) ForgetProjectJob(string) {}

type shutdownIngressTestProcess struct{}

func (*shutdownIngressTestProcess) PID() int                          { return 321 }
func (*shutdownIngressTestProcess) Stdin() io.WriteCloser             { return nil }
func (*shutdownIngressTestProcess) Stdout() io.ReadCloser             { return nil }
func (*shutdownIngressTestProcess) Stderr() io.ReadCloser             { return nil }
func (*shutdownIngressTestProcess) Wait(context.Context) (int, error) { return 0, nil }
func (*shutdownIngressTestProcess) Stop() (runner.StopProof, error)   { return runner.StopProof{}, nil }

func serviceAddress(t *testing.T, server *httptest.Server) (string, uint16) {
	t.Helper()
	address, portText, err := net.SplitHostPort(strings.TrimPrefix(server.URL, "http://"))
	if err != nil {
		t.Fatal(err)
	}
	portValue, err := strconv.ParseUint(portText, 10, 16)
	if err != nil {
		t.Fatal(err)
	}
	return address, uint16(portValue)
}

func mustBrowserSessionURL(t *testing.T, ingress *ServiceBrowserIngress, requestID string) string {
	t.Helper()
	session, err := ingress.CreateSession(requestID)
	if err != nil {
		t.Fatal(err)
	}
	return session.URL
}

func servicePort(server *httptest.Server) int {
	_, port, _ := net.SplitHostPort(strings.TrimPrefix(server.URL, "http://"))
	value, _ := strconv.Atoi(port)
	return value
}

func sha256Hex(value string) string {
	digest := sha256.Sum256([]byte(value))
	return hex.EncodeToString(digest[:])
}

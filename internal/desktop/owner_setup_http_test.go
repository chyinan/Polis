package desktop

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestOwnerBootstrapRequiresLocalOriginAndLoopbackPeer(t *testing.T) {
	handler := MiddlewareWithRemoteOrigin("configured-desktop-token", "", OwnerSetupHandler(nil, false))
	cases := []struct {
		name       string
		origin     string
		remoteAddr string
		wantStatus int
	}{
		{name: "local workbench origin", origin: "http://localhost:4173", remoteAddr: "127.0.0.1:4321", wantStatus: http.StatusBadRequest},
		{name: "missing origin", remoteAddr: "127.0.0.1:4321", wantStatus: http.StatusForbidden},
		{name: "remote origin", origin: "https://remote.example", remoteAddr: "127.0.0.1:4321", wantStatus: http.StatusForbidden},
		{name: "non-loopback peer", origin: "http://localhost:4173", remoteAddr: "192.0.2.8:4321", wantStatus: http.StatusForbidden},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, "/api/installation/owner/bootstrap", strings.NewReader(`{"code":"code","password":"long enough password"}`))
			request.RemoteAddr = test.remoteAddr
			if test.origin != "" {
				request.Header.Set("Origin", test.origin)
			}
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != test.wantStatus {
				t.Fatalf("status=%d body=%s, want %d", response.Code, response.Body.String(), test.wantStatus)
			}
		})
	}
}

func TestOwnerSetupStatusDoesNotRequireDesktopServiceToken(t *testing.T) {
	handler := MiddlewareWithRemoteOrigin("configured-desktop-token", "", OwnerSetupHandler(nil, false))
	request := httptest.NewRequest(http.MethodGet, "/api/installation/owner/status", nil)
	request.RemoteAddr = "127.0.0.1:4321"
	request.Header.Set("Origin", "http://localhost:4173")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("status=%d body=%s, want handler reached without desktop token", response.Code, response.Body.String())
	}
}

func TestOwnerSessionStatusIsPublicAndReportsNoSession(t *testing.T) {
	handler := MiddlewareWithRemoteOrigin("configured-desktop-token", "", OwnerSetupHandler(nil, false))
	request := httptest.NewRequest(http.MethodGet, "/api/installation/owner/session", nil)
	request.Header.Set("Origin", "http://localhost:4173")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"authenticated":false`) {
		t.Fatalf("session status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestSharedDesktopTokenDoesNotCountAsOwnerLogin(t *testing.T) {
	handler := OwnerSetupHandler(nil, false)
	request := httptest.NewRequest(http.MethodGet, "/api/installation/owner/session", nil)
	request = request.WithContext(context.WithValue(request.Context(), installationOwnerContextKey{}, true))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"authenticated":false`) {
		t.Fatalf("owner session status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestOwnerLoginRequiresBrowserOrigin(t *testing.T) {
	handler := MiddlewareWithRemoteOrigin("configured-desktop-token", "", OwnerSetupHandler(nil, false))
	request := httptest.NewRequest(http.MethodPost, "/api/installation/owner/login", strings.NewReader(`{"password":"example password"}`))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusForbidden {
		t.Fatalf("login without origin status=%d body=%s", response.Code, response.Body.String())
	}
}

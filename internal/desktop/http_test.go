// pattern: Imperative Shell
package desktop

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMiddlewareRequiresTokenAndAllowsDesktopOrigin(t *testing.T) {
	handler := Middleware("session", http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		response.WriteHeader(http.StatusNoContent)
	}))

	unauthorized := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	unauthorized.Header.Set("Origin", "tauri://localhost")
	unauthorizedResponse := httptest.NewRecorder()
	handler.ServeHTTP(unauthorizedResponse, unauthorized)
	if unauthorizedResponse.Code != http.StatusUnauthorized {
		t.Fatalf("unauthorized status=%d", unauthorizedResponse.Code)
	}

	authorized := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	authorized.Header.Set("Origin", "tauri://localhost")
	authorized.Header.Set("X-Polis-Desktop-Token", "session")
	authorizedResponse := httptest.NewRecorder()
	handler.ServeHTTP(authorizedResponse, authorized)
	if authorizedResponse.Code != http.StatusNoContent || authorizedResponse.Header().Get("Access-Control-Allow-Origin") != "tauri://localhost" {
		t.Fatalf("authorized response=%d origin=%q", authorizedResponse.Code, authorizedResponse.Header().Get("Access-Control-Allow-Origin"))
	}
}

func TestMiddlewareSupportsEventSourceQueryTokenAndRejectsForeignOrigin(t *testing.T) {
	handler := Middleware("session", http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		response.WriteHeader(http.StatusNoContent)
	}))

	eventSource := httptest.NewRequest(http.MethodGet, "/stream?desktop_token=session", nil)
	eventSourceResponse := httptest.NewRecorder()
	handler.ServeHTTP(eventSourceResponse, eventSource)
	if eventSourceResponse.Code != http.StatusNoContent {
		t.Fatalf("event source status=%d", eventSourceResponse.Code)
	}

	foreign := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	foreign.Header.Set("Origin", "https://attacker.example")
	foreign.Header.Set("X-Polis-Desktop-Token", "session")
	foreignResponse := httptest.NewRecorder()
	handler.ServeHTTP(foreignResponse, foreign)
	if foreignResponse.Code != http.StatusForbidden {
		t.Fatalf("foreign origin status=%d", foreignResponse.Code)
	}
}

func TestMiddlewareRejectsForeignOriginWhenTokenless(t *testing.T) {
	handler := Middleware("", http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		response.WriteHeader(http.StatusNoContent)
	}))
	request := httptest.NewRequest(http.MethodPost, "/api/workbench/companies/company-1/missions", nil)
	request.Header.Set("Origin", "https://attacker.example")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusForbidden {
		t.Fatalf("tokenless foreign-origin status=%d, want %d", response.Code, http.StatusForbidden)
	}
}

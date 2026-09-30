// pattern: Imperative Shell
package desktop

import (
	"net/http"
	"net/http/httptest"
	"strings"
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

func TestRemoteMiddlewareRequiresExactOriginAndHeaderToken(t *testing.T) {
	remoteOrigin := "https://polis.example.com"
	handler := MiddlewareWithRemoteOrigin("session", remoteOrigin, http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		response.WriteHeader(http.StatusNoContent)
	}))
	for _, test := range []struct {
		name       string
		origin     string
		header     string
		queryToken string
		wantStatus int
	}{
		{name: "remote header token", origin: remoteOrigin, header: "session", wantStatus: http.StatusNoContent},
		{name: "remote token missing", origin: remoteOrigin, wantStatus: http.StatusUnauthorized},
		{name: "remote query token rejected", origin: remoteOrigin, queryToken: "session", wantStatus: http.StatusUnauthorized},
		{name: "remote query token rejected without origin", queryToken: "session", wantStatus: http.StatusUnauthorized},
		{name: "local EventSource query token still works", origin: "tauri://localhost", queryToken: "session", wantStatus: http.StatusNoContent},
		{name: "other remote origin rejected", origin: "https://attacker.example", header: "session", wantStatus: http.StatusForbidden},
	} {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, "/healthz", nil)
			if test.queryToken != "" {
				request.URL.RawQuery = "desktop_token=" + test.queryToken
			}
			request.Header.Set("Origin", test.origin)
			if test.header != "" {
				request.Header.Set("X-Polis-Desktop-Token", test.header)
			}
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != test.wantStatus {
				t.Fatalf("status=%d, want %d", response.Code, test.wantStatus)
			}
			if test.wantStatus == http.StatusNoContent && response.Header().Get("Access-Control-Allow-Origin") != test.origin {
				t.Fatalf("allowed origin=%q, want %q", response.Header().Get("Access-Control-Allow-Origin"), test.origin)
			}
		})
	}
}

func TestMiddlewareExposesAllDomainEvidencePreviewHeadersToDesktopOrigins(t *testing.T) {
	handler := Middleware("session", http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		response.WriteHeader(http.StatusNoContent)
	}))
	request := httptest.NewRequest(http.MethodGet, "/api/workbench/companies/company-1/domain-evidence/record-1/evidence/quality/preview", nil)
	request.Header.Set("Origin", "http://127.0.0.1:4173")
	request.Header.Set("X-Polis-Desktop-Token", "session")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusNoContent {
		t.Fatalf("preview CORS status=%d", response.Code)
	}
	exposedHeaders := strings.Split(response.Header().Get("Access-Control-Expose-Headers"), ",")
	exposed := make(map[string]bool, len(exposedHeaders))
	for _, header := range exposedHeaders {
		exposed[strings.TrimSpace(header)] = true
	}
	for _, header := range []string{"X-Source-SHA256", "X-Polis-Input-ID", "X-Polis-Input-Revision", "X-Polis-Preview-Filename"} {
		if !exposed[header] {
			t.Errorf("cross-origin desktop response does not expose %s", header)
		}
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

func TestTokenlessMiddlewareRejectsJobRunEndpointsEvenOnLoopback(t *testing.T) {
	handler := Middleware("", http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		response.WriteHeader(http.StatusNoContent)
	}))
	for _, test := range []struct{ method, path string }{
		{http.MethodGet, "/api/workbench/companies/company-1/tasks/task-1/jobs"},
		{http.MethodPost, "/api/workbench/companies/company-1/tasks/task-1/jobs"},
		{http.MethodPost, "/api/workbench/companies/company-1/jobs/job-1/stop"},
		{http.MethodGet, "/api/workbench/companies/company-1/jobs/job-1/logs"},
		{http.MethodPost, "/api/workbench/companies/company-1/jobs/job-1/browser-session"},
	} {
		request := httptest.NewRequest(test.method, test.path, nil)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != http.StatusUnauthorized {
			t.Errorf("tokenless %s %s status=%d, want %d", test.method, test.path, response.Code, http.StatusUnauthorized)
		}
	}
}

func TestTokenlessMiddlewareRejectsDomainEvidenceEndpointsEvenOnLoopback(t *testing.T) {
	handler := Middleware("", http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		response.WriteHeader(http.StatusNoContent)
	}))
	for _, test := range []struct{ method, path string }{
		{http.MethodGet, "/api/workbench/companies/company-1/domain-workflows"},
		{http.MethodPost, "/api/workbench/companies/company-1/domain-workflows/content-operations-reference/qualification"},
		{http.MethodPost, "/api/workbench/companies/company-1/domain-evidence"},
		{http.MethodPost, "/api/workbench/companies/company-1/domain-evidence/domain-evidence-record-1/review"},
		{http.MethodGet, "/api/workbench/companies/company-1/domain-evidence/domain-evidence-record-1/evidence/quality/preview"},
	} {
		request := httptest.NewRequest(test.method, test.path, nil)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != http.StatusUnauthorized {
			t.Errorf("tokenless %s %s status=%d, want %d", test.method, test.path, response.Code, http.StatusUnauthorized)
		}
	}
}

func TestTokenlessMiddlewareRejectsSkillImportWithoutSessionToken(t *testing.T) {
	handler := Middleware("", http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		response.WriteHeader(http.StatusAccepted)
	}))
	request := httptest.NewRequest(http.MethodPost, "/api/workbench/companies/company-1/capabilities/skills", strings.NewReader("multipart body"))
	request.Header.Set("Content-Type", "multipart/form-data; boundary=test")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("tokenless Skill import status=%d, want %d", response.Code, http.StatusUnauthorized)
	}
}

func TestTokenlessMiddlewareRejectsGitHubFeedbackBacklogMutation(t *testing.T) {
	called := false
	handler := Middleware("", http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		called = true
		response.WriteHeader(http.StatusNoContent)
	}))
	request := httptest.NewRequest(http.MethodPost, "/api/workbench/companies/company-1/feedback/backlog", strings.NewReader(`{}`))
	request.Header.Set("Origin", "http://localhost:4173")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("tokenless feedback backlog status=%d, want %d", response.Code, http.StatusUnauthorized)
	}
	if called {
		t.Fatal("tokenless feedback backlog mutation reached the handler")
	}
}

func TestTokenlessMiddlewareRejectsGitHubFeedbackCollectionPolicyMutation(t *testing.T) {
	called := false
	handler := Middleware("", http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		called = true
		response.WriteHeader(http.StatusNoContent)
	}))
	request := httptest.NewRequest(http.MethodPost, "/api/workbench/companies/company-1/feedback/collection-policy", strings.NewReader(`{}`))
	request.Header.Set("Origin", "http://localhost:4173")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized || called {
		t.Fatalf("tokenless collection policy status=%d handlerCalled=%t, want 401 before handler", response.Code, called)
	}
}

func TestTokenlessMiddlewareRejectsAllCompanyFeedbackRoutes(t *testing.T) {
	handler := Middleware("", http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		response.WriteHeader(http.StatusNoContent)
	}))
	for _, test := range []struct{ method, path string }{
		{http.MethodGet, "/api/workbench/companies/company-1/feedback"},
		{http.MethodPost, "/api/workbench/companies/company-1/feedback/sources"},
		{http.MethodPost, "/api/workbench/companies/company-1/feedback/sources/source-1/probe"},
		{http.MethodPost, "/api/workbench/companies/company-1/feedback/sources/source-1/decision"},
		{http.MethodPost, "/api/workbench/companies/company-1/feedback/sources/source-1/scan"},
		{http.MethodPost, "/api/workbench/companies/company-1/feedback/credentials"},
		{http.MethodPost, "/api/workbench/companies/company-1/feedback/credentials/delete"},
		{http.MethodPost, "/api/workbench/companies/company-1/feedback/backlog"},
		{http.MethodPost, "/api/workbench/companies/company-1/feedback/collection-policy"},
	} {
		request := httptest.NewRequest(test.method, test.path, strings.NewReader("{}"))
		request.Header.Set("Origin", "http://localhost:4173")
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != http.StatusUnauthorized {
			t.Errorf("tokenless %s %s status=%d, want %d", test.method, test.path, response.Code, http.StatusUnauthorized)
		}
	}
}

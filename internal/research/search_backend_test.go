// pattern: Imperative Shell
package research

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestHTTPJSONSearchBackendBindsEndpointAndRejectsCredentials(t *testing.T) {
	var received *http.Request
	backend := &HTTPJSONSearchBackend{
		Origin:   "https://example.test",
		Endpoint: "https://example.test/search",
		Client: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			received = request
			return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(`{"results":[{"url":"https://example.test/docs","title":"Docs","snippet":"bounded","source_time":"2026-10-07T00:00:00Z"}]}`)), Request: request}, nil
		}),
	}
	results, err := backend.Search(context.Background(), "release notes")
	if err != nil || len(results) != 1 || results[0].URL != "https://example.test/docs" {
		t.Fatalf("results=%+v err=%v", results, err)
	}
	if received == nil || received.URL.Query().Get("q") != "release notes" || received.Header.Get("Cookie") != "" || received.Header.Get("Authorization") != "" {
		t.Fatalf("request=%v headers=%v", received.URL, received.Header)
	}
}

func TestHTTPJSONSearchBackendRejectsRedirectAndUnknownFields(t *testing.T) {
	for name, response := range map[string]*http.Response{
		"redirect":      {StatusCode: http.StatusFound, Header: http.Header{"Location": []string{"https://other.test/"}}, Body: io.NopCloser(strings.NewReader(""))},
		"unknown field": {StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(`{"results":[],"extra":true}`))},
	} {
		t.Run(name, func(t *testing.T) {
			backend := &HTTPJSONSearchBackend{Origin: "https://example.test", Endpoint: "https://example.test/search", Client: roundTripFunc(func(request *http.Request) (*http.Response, error) {
				response.Request = request
				return response, nil
			})}
			if _, err := backend.Search(context.Background(), "query"); err == nil {
				t.Fatal("unsafe search response was accepted")
			}
		})
	}
}

type credentialHeaders struct {
	headers http.Header
}

func (provider credentialHeaders) Headers(_ context.Context, _ string) (http.Header, error) {
	return provider.headers, nil
}

func TestHTTPJSONSearchBackendUsesOpaqueCredentialReferenceAndRejectsCookieHeaders(t *testing.T) {
	backend := &HTTPJSONSearchBackend{Origin: "https://example.test", Endpoint: "https://example.test/search", CredentialRef: "search-token", Credentials: credentialHeaders{headers: http.Header{"Authorization": []string{"Bearer fixture"}}}, Client: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.Header.Get("Authorization") != "Bearer fixture" {
			t.Fatalf("authorization header=%q", request.Header.Get("Authorization"))
		}
		return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(`{"results":[{"url":"https://example.test/docs","title":"Docs","snippet":"bounded","source_time":"2026-10-07T00:00:00Z"}]}`)), Request: request}, nil
	})}
	if _, err := backend.Search(context.Background(), "query"); err != nil {
		t.Fatal(err)
	}
	backend.Credentials = credentialHeaders{headers: http.Header{"Cookie": []string{"secret=1"}}}
	if _, err := backend.Search(context.Background(), "query"); err == nil {
		t.Fatal("credential provider cookie header was accepted")
	}
}

// pattern: Imperative Shell
package research

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) Do(request *http.Request) (*http.Response, error) { return f(request) }

func TestFetcherUsesBoundedNoCredentialRequestAndReturnsDigest(t *testing.T) {
	var received *http.Request
	fetcher := &Fetcher{Client: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		received = request
		return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"text/html; charset=utf-8"}}, Body: io.NopCloser(strings.NewReader("fixture")), Request: request}, nil
	})}
	outcome, err := fetcher.Fetch(context.Background(), FetchPlan{Origin: "https://example.test", TargetURL: "https://example.test/docs", TimeoutMS: 5000, MaxBytes: 4096})
	if err != nil || outcome.BodySHA256 != DigestBytes([]byte("fixture")) {
		t.Fatalf("outcome=%+v err=%v", outcome, err)
	}
	if received == nil || received.Header.Get("Cookie") != "" || received.Header.Get("Authorization") != "" || received.Header.Get("Accept") == "" {
		t.Fatalf("request headers=%v", received.Header)
	}
}

func TestFetcherRejectsRedirectWithoutFollowingIt(t *testing.T) {
	calls := 0
	fetcher := &Fetcher{Client: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		calls++
		return &http.Response{StatusCode: http.StatusFound, Header: http.Header{"Location": []string{"https://other.test/"}}, Body: io.NopCloser(strings.NewReader("")), Request: request}, nil
	})}
	_, err := fetcher.Fetch(context.Background(), FetchPlan{Origin: "https://example.test", TargetURL: "https://example.test/docs", TimeoutMS: 5000, MaxBytes: 4096})
	if err == nil || calls != 1 {
		t.Fatalf("redirect err=%v calls=%d", err, calls)
	}
}

// pattern: Imperative Shell
package research

import (
	"context"
	"errors"
	"io"
	"net/http"
	"time"
)

type HTTPDoer interface {
	Do(*http.Request) (*http.Response, error)
}

type Fetcher struct {
	Client HTTPDoer
}

func NewFetcher() *Fetcher {
	return &Fetcher{Client: &http.Client{
		Transport: &http.Transport{
			Proxy:                  nil,
			DisableKeepAlives:      true,
			ForceAttemptHTTP2:      false,
			MaxResponseHeaderBytes: 32 << 10,
		},
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse },
	}}
}

func (fetcher *Fetcher) Fetch(ctx context.Context, plan FetchPlan) (FetchOutcome, error) {
	normalized, err := NormalizeFetchPlan(plan)
	if err != nil {
		return FetchOutcome{}, err
	}
	if fetcher == nil || fetcher.Client == nil {
		return FetchOutcome{}, errors.New("research fetch client is unavailable")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	requestCtx, cancel := context.WithTimeout(ctx, time.Duration(normalized.TimeoutMS)*time.Millisecond)
	defer cancel()
	request, err := http.NewRequestWithContext(requestCtx, http.MethodGet, normalized.TargetURL, nil)
	if err != nil {
		return FetchOutcome{}, err
	}
	request.Header.Set("Accept", "text/html, text/plain, application/json, application/xhtml+xml, application/xml")
	request.Header.Set("Cache-Control", "no-store")
	response, err := fetcher.Client.Do(request)
	if err != nil {
		return FetchOutcome{}, err
	}
	defer response.Body.Close()
	finalURL := normalized.TargetURL
	if response.Request != nil && response.Request.URL != nil {
		finalURL = response.Request.URL.String()
	}
	if response.StatusCode >= 300 && response.StatusCode < 400 {
		return FetchOutcome{State: "failed", ReasonCode: "research_redirect_denied", FinalURL: finalURL, StatusCode: response.StatusCode, RedirectCount: 1}, ErrFetchPolicyViolation
	}
	if response.ContentLength > normalized.MaxBytes {
		return FetchOutcome{State: "failed", ReasonCode: "research_response_too_large", FinalURL: finalURL, StatusCode: response.StatusCode}, ErrFetchPolicyViolation
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, normalized.MaxBytes+1))
	if err != nil {
		return FetchOutcome{}, err
	}
	outcome := FetchOutcome{
		State: "succeeded", ReasonCode: "research_fetch_succeeded", FinalURL: finalURL,
		StatusCode: response.StatusCode, ContentType: response.Header.Get("Content-Type"), Body: body,
		BodySHA256: DigestBytes(body),
	}
	if int64(len(body)) > normalized.MaxBytes {
		outcome.State, outcome.ReasonCode = "failed", "research_response_too_large"
		return outcome, ErrFetchPolicyViolation
	}
	if err := ValidateFetchOutcome(normalized, outcome); err != nil {
		return outcome, err
	}
	return outcome, nil
}

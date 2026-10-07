// pattern: Imperative Shell
package research

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type SearchBackend interface {
	Search(context.Context, string) ([]SearchCandidate, error)
}

const maxSearchResponseBytes int64 = 1 << 20

type HTTPJSONSearchBackend struct {
	Client    HTTPDoer
	Origin    string
	Endpoint  string
	TimeoutMS int
}

func (backend *HTTPJSONSearchBackend) Search(ctx context.Context, query string) ([]SearchCandidate, error) {
	if backend == nil || backend.Client == nil {
		return nil, errors.New("search backend client is unavailable")
	}
	if strings.TrimSpace(query) == "" {
		return nil, ErrSearchResultInvalid
	}
	if ctx == nil {
		ctx = context.Background()
	}
	origin, err := canonicalHTTPSOrigin(backend.Origin)
	if err != nil {
		return nil, ErrSearchResultInvalid
	}
	endpoint, err := canonicalHTTPSURL(backend.Endpoint)
	if err != nil || urlOrigin(endpoint) != origin {
		return nil, ErrSearchResultInvalid
	}
	parsed, err := url.Parse(endpoint)
	if err != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return nil, ErrSearchResultInvalid
	}
	values := parsed.Query()
	values.Set("q", query)
	parsed.RawQuery = values.Encode()
	timeoutMS := backend.TimeoutMS
	if timeoutMS == 0 {
		timeoutMS = 30000
	}
	requestCtx, cancel := context.WithTimeout(ctx, time.Duration(timeoutMS)*time.Millisecond)
	defer cancel()
	request, err := http.NewRequestWithContext(requestCtx, http.MethodGet, parsed.String(), nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Accept", "application/json")
	response, err := backend.Client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 || response.Request == nil || response.Request.URL == nil || urlOrigin(response.Request.URL.String()) != origin {
		return nil, ErrSearchResultInvalid
	}
	if response.ContentLength > maxSearchResponseBytes {
		return nil, ErrSearchResultInvalid
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, maxSearchResponseBytes+1))
	if err != nil || int64(len(body)) > maxSearchResponseBytes {
		return nil, ErrSearchResultInvalid
	}
	var envelope struct {
		Results []SearchCandidate `json:"results"`
	}
	decoder := json.NewDecoder(strings.NewReader(string(body)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&envelope); err != nil {
		return nil, ErrSearchResultInvalid
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return nil, ErrSearchResultInvalid
	}
	normalized, err := NormalizeSearchResults(origin, envelope.Results)
	if err != nil {
		return nil, err
	}
	candidates := make([]SearchCandidate, 0, len(normalized))
	for _, result := range normalized {
		candidates = append(candidates, SearchCandidate{URL: result.URL, Title: result.Title, Snippet: result.Snippet, SourceTime: result.SourceTime})
	}
	return candidates, nil
}

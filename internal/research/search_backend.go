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

type CredentialProvider interface {
	Headers(context.Context, string) (http.Header, error)
}

const maxSearchResponseBytes int64 = 1 << 20

type HTTPJSONSearchBackend struct {
	Client        HTTPDoer
	Origin        string
	Endpoint      string
	CredentialRef string
	Credentials   CredentialProvider
	TimeoutMS     int
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
	if backend.CredentialRef != "" {
		if backend.Credentials == nil {
			return nil, errors.New("search credential provider is unavailable")
		}
		headers, err := backend.Credentials.Headers(requestCtx, backend.CredentialRef)
		if err != nil {
			return nil, err
		}
		for name, values := range headers {
			lower := strings.ToLower(name)
			if lower == "cookie" || lower == "set-cookie" || lower == "proxy-authorization" || lower == "host" || lower == "connection" || lower == "content-length" {
				return nil, errors.New("search credential provider returned a forbidden header")
			}
			for _, value := range values {
				request.Header.Add(name, value)
			}
		}
	}
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

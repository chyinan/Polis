// pattern: Functional Core
package research

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

const (
	maxSearchResults      = 20
	maxSearchTitleBytes   = 512
	maxSearchSnippetBytes = 4096
)

var ErrSearchResultInvalid = errors.New("invalid research search result")

type SearchCandidate struct {
	URL        string `json:"url"`
	Title      string `json:"title"`
	Snippet    string `json:"snippet"`
	SourceTime string `json:"source_time"`
}

type SearchResult struct {
	Rank       int    `json:"rank"`
	URL        string `json:"url"`
	Title      string `json:"title"`
	Snippet    string `json:"snippet"`
	SourceTime string `json:"source_time"`
	Digest     string `json:"digest"`
}

func NormalizeSearchResults(origin string, candidates []SearchCandidate) ([]SearchResult, error) {
	normalizedOrigin, err := canonicalHTTPSOrigin(origin)
	if err != nil || len(candidates) == 0 || len(candidates) > maxSearchResults {
		return nil, ErrSearchResultInvalid
	}
	results := make([]SearchResult, 0, len(candidates))
	seen := make(map[string]struct{}, len(candidates))
	for index, candidate := range candidates {
		candidate.Title = strings.TrimSpace(candidate.Title)
		candidate.Snippet = strings.TrimSpace(candidate.Snippet)
		candidate.SourceTime = strings.TrimSpace(candidate.SourceTime)
		url, urlErr := canonicalHTTPSURL(candidate.URL)
		if urlErr != nil || urlOrigin(url) != normalizedOrigin || candidate.Title == "" || len(candidate.Title) > maxSearchTitleBytes || len(candidate.Snippet) > maxSearchSnippetBytes || candidate.SourceTime == "" {
			return nil, ErrSearchResultInvalid
		}
		if _, exists := seen[url]; exists {
			return nil, ErrSearchResultInvalid
		}
		if _, timeErr := time.Parse(time.RFC3339, candidate.SourceTime); timeErr != nil {
			return nil, ErrSearchResultInvalid
		}
		seen[url] = struct{}{}
		result := SearchResult{Rank: index + 1, URL: url, Title: candidate.Title, Snippet: candidate.Snippet, SourceTime: candidate.SourceTime}
		encoded, marshalErr := json.Marshal(result)
		if marshalErr != nil {
			return nil, ErrSearchResultInvalid
		}
		digest := sha256.Sum256(encoded)
		result.Digest = hex.EncodeToString(digest[:])
		results = append(results, result)
	}
	return results, nil
}

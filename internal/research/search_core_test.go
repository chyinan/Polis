// pattern: Functional Core
package research

import (
	"errors"
	"strings"
	"testing"
)

func TestNormalizeSearchResultsBindsSourceTimeAndExactOrigin(t *testing.T) {
	results, err := NormalizeSearchResults("https://example.test", []SearchCandidate{
		{URL: "https://Example.test/docs", Title: "Docs", Snippet: "bounded result", SourceTime: "2026-10-07T00:00:00Z"},
		{URL: "https://example.test/notes", Title: "Notes", Snippet: "second result", SourceTime: "2026-10-07T01:00:00Z"},
	})
	if err != nil || len(results) != 2 {
		t.Fatalf("results=%+v err=%v", results, err)
	}
	if results[0].Rank != 1 || results[0].URL != "https://example.test/docs" || results[0].Digest == "" || results[1].Rank != 2 {
		t.Fatalf("normalized results=%+v", results)
	}
}

func TestNormalizeSearchResultsRejectsMissingSourceAndCrossOriginData(t *testing.T) {
	base := SearchCandidate{URL: "https://example.test/docs", Title: "Docs", Snippet: "bounded result", SourceTime: "2026-10-07T00:00:00Z"}
	for name, mutate := range map[string]func(*SearchCandidate){
		"cross origin":  func(v *SearchCandidate) { v.URL = "https://other.test/docs" },
		"missing time":  func(v *SearchCandidate) { v.SourceTime = "" },
		"bad time":      func(v *SearchCandidate) { v.SourceTime = "yesterday" },
		"missing title": func(v *SearchCandidate) { v.Title = "" },
	} {
		t.Run(name, func(t *testing.T) {
			candidate := base
			mutate(&candidate)
			if _, err := NormalizeSearchResults("https://example.test", []SearchCandidate{candidate}); !errors.Is(err, ErrSearchResultInvalid) {
				t.Fatalf("candidate=%+v err=%v", candidate, err)
			}
		})
	}
}

func TestNormalizeSearchResultsRejectsDuplicatesAndUnboundedFields(t *testing.T) {
	duplicate := SearchCandidate{URL: "https://example.test/docs", Title: "Docs", Snippet: "bounded", SourceTime: "2026-10-07T00:00:00Z"}
	if _, err := NormalizeSearchResults("https://example.test", []SearchCandidate{duplicate, duplicate}); !errors.Is(err, ErrSearchResultInvalid) {
		t.Fatalf("duplicate err=%v", err)
	}
	large := duplicate
	large.Snippet = strings.Repeat("x", maxSearchSnippetBytes+1)
	if _, err := NormalizeSearchResults("https://example.test", []SearchCandidate{large}); !errors.Is(err, ErrSearchResultInvalid) {
		t.Fatalf("large result err=%v", err)
	}
}

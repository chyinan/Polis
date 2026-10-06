// pattern: Functional Core
package kernel

import (
	"errors"
	"testing"

	"polis/internal/core"
)

func TestNormalizeResearchOperationRequestSeparatesSearchAndFetch(t *testing.T) {
	search, err := normalizeResearchOperationRequest(ResearchOperationRequest{Kind: ResearchOperationKindSearch, Query: "  Polis release notes  "})
	if err != nil || search.Query != "Polis release notes" || search.TargetURL != "" {
		t.Fatalf("search=%+v err=%v", search, err)
	}
	fetch, err := normalizeResearchOperationRequest(ResearchOperationRequest{Kind: ResearchOperationKindFetch, TargetURL: "https://Example.test/path?q=1"})
	if err != nil || fetch.TargetURL != "https://example.test/path?q=1" {
		t.Fatalf("fetch=%+v err=%v", fetch, err)
	}
	for _, input := range []ResearchOperationRequest{
		{Kind: ResearchOperationKindSearch, Query: "query", TargetURL: "https://example.test"},
		{Kind: ResearchOperationKindFetch, TargetURL: "http://example.test"},
		{Kind: ResearchOperationKindFetch, TargetURL: "https://user:pass@example.test"},
		{Kind: ResearchOperationKindFetch, TargetURL: "https://example.test#fragment"},
	} {
		if _, err := normalizeResearchOperationRequest(input); !errors.Is(err, core.Malformed) && !errors.Is(err, core.Denied) {
			t.Fatalf("input %+v returned %v", input, err)
		}
	}
}

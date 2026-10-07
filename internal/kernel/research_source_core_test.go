// pattern: Functional Core
package kernel

import (
	"errors"
	"strings"
	"testing"

	"polis/internal/core"
)

func TestNormalizeResearchSourceRegistrationCanonicalizesHTTPSOrigin(t *testing.T) {
	registration, err := normalizeResearchSourceRegistration(ResearchSourceRegistration{
		SourceID:              "source-docs",
		MissionID:             "mission-1",
		Origin:                "https://Example.test:8443/",
		SearchEndpoint:        "https://example.test:8443/search",
		SearchRankingRevision: ResearchSearchRankingRevision,
		ProfileRevision:       ResearchSourceProfileRevision,
		IdentitySHA256:        strings.Repeat("a", 64),
		DataSHA256:            strings.Repeat("b", 64),
	})
	if err != nil {
		t.Fatal(err)
	}
	if registration.Origin != "https://example.test:8443" || registration.SearchEndpoint != "https://example.test:8443/search" || registration.ProfileRevision != ResearchSourceProfileRevision {
		t.Fatalf("normalized research source=%+v", registration)
	}
}

func TestNormalizeResearchSourceRegistrationRejectsUnsafeOrIncompleteBindings(t *testing.T) {
	base := ResearchSourceRegistration{
		SourceID:        "source-docs",
		MissionID:       "mission-1",
		Origin:          "https://example.test",
		ProfileRevision: ResearchSourceProfileRevision,
		IdentitySHA256:  strings.Repeat("a", 64),
		DataSHA256:      strings.Repeat("b", 64),
	}
	cases := map[string]ResearchSourceRegistration{
		"http":                   mutateResearchSource(base, func(v *ResearchSourceRegistration) { v.Origin = "http://example.test" }),
		"path":                   mutateResearchSource(base, func(v *ResearchSourceRegistration) { v.Origin = "https://example.test/docs" }),
		"query":                  mutateResearchSource(base, func(v *ResearchSourceRegistration) { v.Origin = "https://example.test?x=1" }),
		"credentials":            mutateResearchSource(base, func(v *ResearchSourceRegistration) { v.Origin = "https://user:pass@example.test" }),
		"identity":               mutateResearchSource(base, func(v *ResearchSourceRegistration) { v.IdentitySHA256 = "bad" }),
		"data":                   mutateResearchSource(base, func(v *ResearchSourceRegistration) { v.DataSHA256 = "bad" }),
		"profile":                mutateResearchSource(base, func(v *ResearchSourceRegistration) { v.ProfileRevision = "research-source@2" }),
		"search endpoint origin": mutateResearchSource(base, func(v *ResearchSourceRegistration) { v.SearchEndpoint = "https://other.test/search" }),
		"search endpoint query":  mutateResearchSource(base, func(v *ResearchSourceRegistration) { v.SearchEndpoint = "https://example.test/search?q=fixed" }),
	}
	for name, input := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := normalizeResearchSourceRegistration(input); !errors.Is(err, core.Malformed) && !errors.Is(err, core.Denied) {
				t.Fatalf("registration=%+v err=%v, want malformed or denied", input, err)
			}
		})
	}
}

func TestValidateResearchSourceOperationBindsMissionAndExactOrigin(t *testing.T) {
	registration, err := normalizeResearchSourceRegistration(ResearchSourceRegistration{
		SourceID:        "source-docs",
		MissionID:       "mission-1",
		Origin:          "https://example.test",
		ProfileRevision: ResearchSourceProfileRevision,
		IdentitySHA256:  strings.Repeat("a", 64),
		DataSHA256:      strings.Repeat("b", 64),
	})
	if err != nil {
		t.Fatal(err)
	}
	for name, test := range map[string]struct {
		mission string
		request ResearchOperationRequest
		want    error
	}{
		"same origin fetch":  {mission: "mission-1", request: ResearchOperationRequest{Kind: ResearchOperationKindFetch, SourceID: "source-docs", TargetURL: "https://example.test/docs"}},
		"same origin search": {mission: "mission-1", request: ResearchOperationRequest{Kind: ResearchOperationKindSearch, SourceID: "source-docs", Query: "release notes"}},
		"wrong mission":      {mission: "mission-2", request: ResearchOperationRequest{Kind: ResearchOperationKindSearch, SourceID: "source-docs", Query: "release notes"}, want: core.OutOfScope},
		"wrong source":       {mission: "mission-1", request: ResearchOperationRequest{Kind: ResearchOperationKindSearch, SourceID: "other-source", Query: "release notes"}, want: core.OutOfScope},
		"cross origin":       {mission: "mission-1", request: ResearchOperationRequest{Kind: ResearchOperationKindFetch, SourceID: "source-docs", TargetURL: "https://other.test/docs"}, want: core.Denied},
		"cross port":         {mission: "mission-1", request: ResearchOperationRequest{Kind: ResearchOperationKindFetch, SourceID: "source-docs", TargetURL: "https://example.test:8443/docs"}, want: core.Denied},
	} {
		t.Run(name, func(t *testing.T) {
			normalized, normalizeErr := normalizeResearchOperationRequest(test.request)
			if normalizeErr != nil {
				t.Fatal(normalizeErr)
			}
			err := validateResearchSourceOperation(registration, test.mission, normalized)
			if test.want == nil {
				if err != nil {
					t.Fatalf("binding returned %v", err)
				}
				return
			}
			if !errors.Is(err, test.want) {
				t.Fatalf("binding err=%v, want %v", err, test.want)
			}
		})
	}
}

func TestResearchSourceRegistrationDigestIsStable(t *testing.T) {
	registration := ResearchSourceRegistration{
		SourceID:        "source-docs",
		MissionID:       "mission-1",
		Origin:          "https://example.test",
		ProfileRevision: ResearchSourceProfileRevision,
		IdentitySHA256:  strings.Repeat("a", 64),
		DataSHA256:      strings.Repeat("b", 64),
	}
	first, err := researchSourceRegistrationDigest(registration)
	if err != nil {
		t.Fatal(err)
	}
	second, err := researchSourceRegistrationDigest(registration)
	if err != nil || first != second || len(first) != 64 {
		t.Fatalf("digest=%q second=%q err=%v", first, second, err)
	}
}

func mutateResearchSource(input ResearchSourceRegistration, mutate func(*ResearchSourceRegistration)) ResearchSourceRegistration {
	mutated := input
	mutate(&mutated)
	return mutated
}

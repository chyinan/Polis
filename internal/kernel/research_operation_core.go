// pattern: Functional Core
package kernel

import (
	"net/url"
	"strings"

	"polis/internal/core"
)

const (
	ResearchOperationUnavailableReason = "research_backend_unavailable"
	ResearchOperationKindSearch        = "search"
	ResearchOperationKindFetch         = "fetch"
	maxResearchOperationInputBytes     = 4096
)

type ResearchOperationRequest struct {
	Kind      string `json:"kind"`
	Query     string `json:"query,omitempty"`
	TargetURL string `json:"targetUrl,omitempty"`
}

func normalizeResearchOperationRequest(input ResearchOperationRequest) (ResearchOperationRequest, error) {
	input.Kind = strings.TrimSpace(input.Kind)
	input.Query = strings.TrimSpace(input.Query)
	input.TargetURL = strings.TrimSpace(input.TargetURL)
	if input.Kind != ResearchOperationKindSearch && input.Kind != ResearchOperationKindFetch {
		return ResearchOperationRequest{}, core.Malformed
	}
	if input.Kind == ResearchOperationKindSearch {
		if input.Query == "" || len(input.Query) > maxResearchOperationInputBytes || input.TargetURL != "" || containsResearchControl(input.Query) {
			return ResearchOperationRequest{}, core.Malformed
		}
		return input, nil
	}
	if input.Query != "" || len(input.TargetURL) > maxResearchOperationInputBytes || containsResearchControl(input.TargetURL) {
		return ResearchOperationRequest{}, core.Malformed
	}
	canonical, err := canonicalResearchFetchURL(input.TargetURL)
	if err != nil {
		return ResearchOperationRequest{}, err
	}
	input.TargetURL = canonical
	return input, nil
}

func canonicalResearchFetchURL(raw string) (string, error) {
	if raw == "" {
		return "", core.Malformed
	}
	u, err := url.ParseRequestURI(raw)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.Fragment != "" || u.Opaque != "" {
		return "", core.Denied
	}
	u.Scheme = "https"
	u.Host = strings.ToLower(u.Host)
	return u.String(), nil
}

func containsResearchControl(value string) bool {
	return strings.ContainsAny(value, "\r\n\x00")
}

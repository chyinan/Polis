// pattern: Functional Core
package kernel

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/url"
	"strconv"
	"strings"

	"polis/internal/core"
)

const ResearchSourceProfileRevision = "research-source@1"

type ResearchSourceRegistration struct {
	SourceID        string `json:"sourceId"`
	MissionID       string `json:"missionId"`
	Origin          string `json:"origin"`
	SearchEndpoint  string `json:"searchEndpoint,omitempty"`
	ProfileRevision string `json:"profileRevision"`
	IdentitySHA256  string `json:"identitySha256"`
	DataSHA256      string `json:"dataSha256"`
}

func normalizeResearchSourceRegistration(input ResearchSourceRegistration) (ResearchSourceRegistration, error) {
	input.SourceID = strings.TrimSpace(input.SourceID)
	input.MissionID = strings.TrimSpace(input.MissionID)
	input.Origin = strings.TrimSpace(input.Origin)
	input.SearchEndpoint = strings.TrimSpace(input.SearchEndpoint)
	input.ProfileRevision = strings.TrimSpace(input.ProfileRevision)
	input.IdentitySHA256 = strings.TrimSpace(input.IdentitySHA256)
	input.DataSHA256 = strings.TrimSpace(input.DataSHA256)
	if !core.ValidID(input.SourceID) || !core.ValidID(input.MissionID) || input.ProfileRevision != ResearchSourceProfileRevision ||
		!validCapabilityDigest(input.IdentitySHA256) || !validCapabilityDigest(input.DataSHA256) {
		return ResearchSourceRegistration{}, core.Malformed
	}
	origin, err := canonicalResearchSourceOrigin(input.Origin)
	if err != nil {
		return ResearchSourceRegistration{}, err
	}
	input.Origin = origin
	if input.SearchEndpoint != "" {
		endpoint, endpointErr := canonicalResearchSourceSearchEndpoint(input.SearchEndpoint, origin)
		if endpointErr != nil {
			return ResearchSourceRegistration{}, endpointErr
		}
		input.SearchEndpoint = endpoint
	}
	return input, nil
}

func canonicalResearchSourceSearchEndpoint(raw, origin string) (string, error) {
	if raw == "" || strings.ContainsAny(raw, "\r\n\t") {
		return "", core.Malformed
	}
	u, err := url.ParseRequestURI(raw)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Opaque != "" || u.Path == "" {
		return "", core.Denied
	}
	if canonicalResearchURLAuthority(u) != origin {
		return "", core.Denied
	}
	u.Scheme = "https"
	u.Host = canonicalResearchURLAuthority(u)[len("https://"):]
	return u.String(), nil
}

func canonicalResearchSourceOrigin(raw string) (string, error) {
	if raw == "" || strings.ContainsAny(raw, "\r\n\t") {
		return "", core.Malformed
	}
	u, err := url.ParseRequestURI(raw)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") || u.Opaque != "" {
		return "", core.Denied
	}
	return canonicalResearchURLAuthority(u), nil
}

func canonicalResearchURLAuthority(u *url.URL) string {
	hostname := strings.ToLower(u.Hostname())
	host := hostname
	if strings.Contains(hostname, ":") {
		host = "[" + hostname + "]"
	}
	port := u.Port()
	if port != "" {
		parsed, err := strconv.Atoi(port)
		if err == nil && parsed == 443 {
			port = ""
		}
	}
	if port != "" {
		host += ":" + port
	}
	return "https://" + host
}

func researchOriginForURL(raw string) (string, error) {
	parsed, err := url.ParseRequestURI(raw)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil || parsed.Fragment != "" || parsed.Opaque != "" {
		return "", core.Denied
	}
	return canonicalResearchURLAuthority(parsed), nil
}

func validateResearchSourceOperation(registration ResearchSourceRegistration, missionID string, request ResearchOperationRequest) error {
	normalized, err := normalizeResearchSourceRegistration(registration)
	if err != nil {
		return err
	}
	if !core.ValidID(missionID) || missionID != normalized.MissionID || request.SourceID != normalized.SourceID {
		return core.OutOfScope
	}
	if request.Kind != ResearchOperationKindFetch {
		return nil
	}
	origin, err := researchOriginForURL(request.TargetURL)
	if err != nil {
		return err
	}
	if origin != normalized.Origin {
		return core.Denied
	}
	return nil
}

func researchSourceRegistrationDigest(registration ResearchSourceRegistration) (string, error) {
	normalized, err := normalizeResearchSourceRegistration(registration)
	if err != nil {
		return "", err
	}
	encoded, err := json.Marshal(normalized)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:]), nil
}

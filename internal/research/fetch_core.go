// pattern: Functional Core
package research

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/url"
	"strconv"
	"strings"
)

const maxFetchBytes int64 = 8 << 20

var (
	ErrInvalidFetchPlan     = errors.New("invalid research fetch plan")
	ErrFetchPolicyViolation = errors.New("research fetch policy violation")
)

type FetchPlan struct {
	Origin    string `json:"origin"`
	TargetURL string `json:"target_url"`
	TimeoutMS int    `json:"timeout_ms"`
	MaxBytes  int64  `json:"max_bytes"`
}

type FetchOutcome struct {
	State         string `json:"state"`
	ReasonCode    string `json:"reason_code"`
	FinalURL      string `json:"final_url"`
	StatusCode    int    `json:"status_code"`
	ContentType   string `json:"content_type"`
	RedirectCount int    `json:"redirect_count"`
	Body          []byte `json:"body,omitempty"`
	BodySHA256    string `json:"body_sha256"`
}

func NormalizeFetchPlan(input FetchPlan) (FetchPlan, error) {
	if input.TimeoutMS < 1000 || input.TimeoutMS > 120000 || input.MaxBytes < 1 || input.MaxBytes > maxFetchBytes {
		return FetchPlan{}, ErrInvalidFetchPlan
	}
	origin, err := canonicalHTTPSOrigin(input.Origin)
	if err != nil {
		return FetchPlan{}, ErrInvalidFetchPlan
	}
	target, err := canonicalHTTPSURL(input.TargetURL)
	if err != nil || urlOrigin(target) != origin {
		return FetchPlan{}, ErrInvalidFetchPlan
	}
	input.Origin = origin
	input.TargetURL = target
	return input, nil
}

func ValidateFetchOutcome(plan FetchPlan, outcome FetchOutcome) error {
	normalized, err := NormalizeFetchPlan(plan)
	if err != nil || outcome.State != "succeeded" || outcome.ReasonCode != "research_fetch_succeeded" || outcome.StatusCode < 200 || outcome.StatusCode >= 300 || outcome.RedirectCount != 0 || outcome.FinalURL == "" || outcome.ContentType == "" || !validFetchContentType(outcome.ContentType) || int64(len(outcome.Body)) > normalized.MaxBytes || len(outcome.BodySHA256) != 64 || outcome.BodySHA256 != DigestBytes(outcome.Body) || urlOrigin(outcome.FinalURL) != normalized.Origin {
		return ErrFetchPolicyViolation
	}
	return nil
}

func DigestBytes(value []byte) string {
	digest := sha256.Sum256(value)
	return hex.EncodeToString(digest[:])
}

func validFetchContentType(value string) bool {
	mediaType := strings.ToLower(strings.TrimSpace(strings.SplitN(value, ";", 2)[0]))
	return strings.HasPrefix(mediaType, "text/") || mediaType == "application/json" || mediaType == "application/xhtml+xml" || mediaType == "application/xml"
}

func canonicalHTTPSOrigin(raw string) (string, error) {
	if raw == "" || strings.TrimSpace(raw) != raw || strings.ContainsAny(raw, "\r\n\t") {
		return "", ErrInvalidFetchPlan
	}
	u, err := url.ParseRequestURI(raw)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") || u.Opaque != "" {
		return "", ErrInvalidFetchPlan
	}
	return urlAuthority(u), nil
}

func canonicalHTTPSURL(raw string) (string, error) {
	if raw == "" || strings.ContainsAny(raw, "\r\n\t") {
		return "", ErrInvalidFetchPlan
	}
	u, err := url.ParseRequestURI(raw)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.Fragment != "" || u.Opaque != "" {
		return "", ErrInvalidFetchPlan
	}
	u.Scheme = "https"
	u.Host = urlAuthority(u)[len("https://"):]
	return u.String(), nil
}

func urlOrigin(raw string) string {
	u, err := url.ParseRequestURI(raw)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil {
		return ""
	}
	return urlAuthority(u)
}

func urlAuthority(u *url.URL) string {
	host := strings.ToLower(u.Hostname())
	if strings.Contains(host, ":") {
		host = "[" + host + "]"
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

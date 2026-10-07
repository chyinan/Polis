// pattern: Functional Core
package browser

import (
	"errors"
	"net/url"
	"strconv"
	"strings"
)

const (
	BrowserRunnerProtocol = "polis-browser-runner@1"
	maxBrowserTextBytes   = 64 << 10
	maxBrowserTitleBytes  = 1 << 10
)

var (
	ErrInvalidBrowserRunPlan  = errors.New("invalid browser run plan")
	ErrBrowserPolicyViolation = errors.New("browser policy violation")
)

type BrowserRunPlan struct {
	TargetOrigin     string `json:"target_origin"`
	TimeoutMS        int    `json:"timeout_ms"`
	MaxRequests      int    `json:"max_requests"`
	MaxResponseBytes int64  `json:"max_response_bytes"`
}

type BrowserRunOutcome struct {
	Protocol            string `json:"protocol"`
	State               string `json:"state"`
	ReasonCode          string `json:"reason_code"`
	FinalURL            string `json:"final_url"`
	Title               string `json:"title"`
	Text                string `json:"text"`
	RequestCount        int    `json:"request_count"`
	BlockedRequestCount int    `json:"blocked_request_count"`
	ResponseBytes       int64  `json:"response_bytes"`
	DownloadCount       int    `json:"download_count"`
	WebSocketCount      int    `json:"websocket_count"`
}

func NormalizeBrowserRunPlan(input BrowserRunPlan) (BrowserRunPlan, error) {
	if input.TimeoutMS < 1000 || input.TimeoutMS > 120000 || input.MaxRequests < 1 || input.MaxRequests > 512 || input.MaxResponseBytes < 1 || input.MaxResponseBytes > 8<<20 {
		return BrowserRunPlan{}, ErrInvalidBrowserRunPlan
	}
	origin, err := canonicalHTTPSOrigin(input.TargetOrigin)
	if err != nil {
		return BrowserRunPlan{}, ErrInvalidBrowserRunPlan
	}
	input.TargetOrigin = origin
	return input, nil
}

func ValidateBrowserRunOutcome(plan BrowserRunPlan, outcome BrowserRunOutcome) error {
	normalized, err := NormalizeBrowserRunPlan(plan)
	if err != nil || outcome.Protocol != BrowserRunnerProtocol || outcome.State != "succeeded" || outcome.ReasonCode != "browser_run_succeeded" {
		return ErrBrowserPolicyViolation
	}
	finalOrigin, err := browserURLOrigin(outcome.FinalURL)
	if err != nil || finalOrigin != normalized.TargetOrigin || outcome.RequestCount < 1 || outcome.RequestCount > normalized.MaxRequests || outcome.BlockedRequestCount < 0 || outcome.BlockedRequestCount > outcome.RequestCount || outcome.ResponseBytes < 0 || outcome.ResponseBytes > normalized.MaxResponseBytes || outcome.DownloadCount != 0 || outcome.WebSocketCount != 0 || len(outcome.Title) > maxBrowserTitleBytes || len(outcome.Text) > maxBrowserTextBytes {
		return ErrBrowserPolicyViolation
	}
	return nil
}

func canonicalHTTPSOrigin(raw string) (string, error) {
	if raw == "" || strings.TrimSpace(raw) != raw || strings.ContainsAny(raw, "\r\n\t") {
		return "", ErrInvalidBrowserRunPlan
	}
	u, err := url.ParseRequestURI(raw)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") || u.Opaque != "" {
		return "", ErrInvalidBrowserRunPlan
	}
	return browserURLAuthority(u), nil
}

func browserURLOrigin(raw string) (string, error) {
	if raw == "" || strings.ContainsAny(raw, "\r\n\t") {
		return "", ErrBrowserPolicyViolation
	}
	u, err := url.ParseRequestURI(raw)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.Fragment != "" || u.Opaque != "" {
		return "", ErrBrowserPolicyViolation
	}
	return browserURLAuthority(u), nil
}

func browserURLAuthority(u *url.URL) string {
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

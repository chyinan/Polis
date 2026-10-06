// pattern: Functional Core
package kernel

import (
	"net/url"
	"strconv"
	"strings"

	"polis/internal/core"
)

const (
	BrowserRunBlockedReason = "browser_runtime_unqualified"
	maxBrowserRunMetadata   = 128
)

type BrowserRunRequest struct {
	ServiceJobID         string `json:"serviceJobId"`
	ServiceGeneration    int    `json:"serviceGeneration"`
	TargetOrigin         string `json:"targetOrigin"`
	PlanSHA256           string `json:"planSha256"`
	BrowserBuild         string `json:"browserBuild"`
	ExecutionEnvironment string `json:"executionEnvironment"`
	InputRevision        string `json:"inputRevision"`
	ViewportWidth        int    `json:"viewportWidth"`
	ViewportHeight       int    `json:"viewportHeight"`
	Locale               string `json:"locale"`
	Timezone             string `json:"timezone"`
}

func normalizeBrowserRunRequest(input BrowserRunRequest) (BrowserRunRequest, error) {
	if !core.ValidID(input.ServiceJobID) || input.ServiceGeneration <= 0 || !validEnvironmentSHA256(input.PlanSHA256) ||
		!core.ValidID(input.InputRevision) || input.ViewportWidth < 1 || input.ViewportWidth > 4096 || input.ViewportHeight < 1 || input.ViewportHeight > 4096 {
		return BrowserRunRequest{}, core.Malformed
	}
	origin, err := canonicalBrowserRunOrigin(input.TargetOrigin)
	if err != nil {
		return BrowserRunRequest{}, err
	}
	if len(origin) > 256 {
		return BrowserRunRequest{}, core.TooLarge
	}
	for _, value := range []string{input.BrowserBuild, input.ExecutionEnvironment, input.Locale, input.Timezone} {
		if !boundedBrowserRunMetadata(value) {
			return BrowserRunRequest{}, core.Malformed
		}
	}
	input.TargetOrigin = origin
	input.BrowserBuild = strings.TrimSpace(input.BrowserBuild)
	input.ExecutionEnvironment = strings.TrimSpace(input.ExecutionEnvironment)
	input.Locale = strings.TrimSpace(input.Locale)
	input.Timezone = strings.TrimSpace(input.Timezone)
	return input, nil
}

func canonicalBrowserRunOrigin(raw string) (string, error) {
	if strings.TrimSpace(raw) != raw || raw == "" || strings.ContainsAny(raw, "\r\n\t") {
		return "", core.Malformed
	}
	u, err := url.ParseRequestURI(raw)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") || u.Opaque != "" {
		return "", core.Denied
	}
	hostname := strings.ToLower(u.Hostname())
	if hostname == "" {
		return "", core.Denied
	}
	port := u.Port()
	if port != "" {
		parsed, parseErr := strconv.Atoi(port)
		if parseErr != nil || parsed < 1 || parsed > 65535 || port == "443" {
			if parseErr != nil || parsed < 1 || parsed > 65535 {
				return "", core.Denied
			}
			port = ""
		}
	}
	host := hostname
	if strings.Contains(hostname, ":") {
		host = "[" + hostname + "]"
	}
	if port != "" {
		host += ":" + port
	}
	return "https://" + host, nil
}

func boundedBrowserRunMetadata(value string) bool {
	trimmed := strings.TrimSpace(value)
	return trimmed != "" && len(trimmed) <= maxBrowserRunMetadata && !strings.ContainsAny(trimmed, "\r\n\t")
}

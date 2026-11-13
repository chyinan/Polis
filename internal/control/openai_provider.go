package control

import (
	"net"
	"net/url"
	"strings"

	"polis/internal/core"
)

func normalizeOpenAIEndpoint(raw string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Host == "" || parsed.Opaque != "" {
		return "", core.Malformed
	}
	if parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", core.Malformed
	}
	host := parsed.Hostname()
	switch parsed.Scheme {
	case "https":
	case "http":
		if host != "localhost" && !isLoopbackHost(host) {
			return "", core.Malformed
		}
	default:
		return "", core.Malformed
	}
	parsed.Host = strings.TrimSuffix(parsed.Host, ".")
	parsed.Path = strings.TrimRight(parsed.Path, "/")
	return parsed.String(), nil
}

func validateOpenAIModel(model string) error {
	model = strings.TrimSpace(model)
	if model == "" || len(model) > 128 || strings.ContainsAny(model, " \t\r\n") {
		return core.Malformed
	}
	return nil
}

func normalizeDeepSeekEffort(effort string) (string, error) {
	switch strings.TrimSpace(effort) {
	case "none", "low", "high", "max":
		return strings.TrimSpace(effort), nil
	default:
		return "", core.Malformed
	}
}

func validateOpenAIAPIKey(key string) error {
	key = strings.TrimSpace(key)
	if len(key) < 8 || len(key) > 512 || strings.ContainsAny(key, " \t\r\n") {
		return core.Malformed
	}
	return nil
}

func isLoopbackHost(host string) bool {
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

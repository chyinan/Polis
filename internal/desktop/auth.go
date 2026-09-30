// pattern: Functional Core
package desktop

import (
	"fmt"
	"net"
	"net/url"
	"strings"
)

// TokenMatches performs the desktop-local authentication comparison without
// reading environment variables or touching the network. The caller owns the
// token lifetime and must never persist it as product authority.
func TokenMatches(expected, presented string) bool {
	expected = strings.TrimSpace(expected)
	presented = strings.TrimSpace(presented)
	return expected != "" && presented != "" && expected == presented
}

// AllowedOrigin keeps the desktop API boundary explicit. Empty Origin is
// accepted for native health probes and same-origin browser requests.
func AllowedOrigin(origin string) bool {
	switch strings.TrimSpace(origin) {
	case "", "tauri://localhost", "http://tauri.localhost", "http://localhost:4173", "http://127.0.0.1:4173":
		return true
	default:
		return false
	}
}

// PresentedToken accepts the header used by fetch and the short-lived query
// fallback required by native EventSource, which cannot set custom headers.
func PresentedToken(header, authorization, query string) string {
	if strings.TrimSpace(header) != "" {
		return header
	}
	if strings.HasPrefix(authorization, "Bearer ") {
		return strings.TrimPrefix(authorization, "Bearer ")
	}
	return query
}

// RequiresSessionTokenPath marks local endpoints that expose company content
// or trigger project execution side effects. They stay closed in tokenless
// browser mode even when the listener is loopback-only.
func RequiresSessionTokenPath(path string) bool {
	if !strings.HasPrefix(path, "/api/workbench/companies/") {
		return false
	}
	if strings.Contains(path, "/environments") {
		return true
	}
	if strings.Contains(path, "/takeover-leases") || strings.Contains(path, "/takeover-lease") {
		return true
	}
	if strings.Contains(path, "/tasks/") && (strings.HasSuffix(path, "/jobs") || strings.HasSuffix(path, "/workspace")) {
		return true
	}
	if strings.Contains(path, "/jobs/") && (strings.HasSuffix(path, "/logs") || strings.HasSuffix(path, "/stop") || strings.HasSuffix(path, "/browser-session")) {
		return true
	}
	if strings.Contains(path, "/artifacts/") && (strings.HasSuffix(path, "/manifest") || strings.HasSuffix(path, "/download")) {
		return true
	}
	if strings.HasSuffix(path, "/domain-workflows") || strings.Contains(path, "/domain-workflows/") || strings.Contains(path, "/domain-evidence/") || strings.HasSuffix(path, "/domain-evidence") {
		return true
	}
	if strings.HasSuffix(path, "/feedback") || strings.Contains(path, "/feedback/") {
		return true
	}
	return strings.Contains(path, "/capabilities/") && (strings.HasSuffix(path, "/skills") || strings.HasSuffix(path, "/mcp") || strings.HasSuffix(path, "/mcp-packages") || strings.HasSuffix(path, "/qualify") || strings.HasSuffix(path, "/decide") || strings.HasSuffix(path, "/bind") || strings.HasSuffix(path, "/unbind") || strings.HasSuffix(path, "/runtime-approve") || strings.HasSuffix(path, "/runtime-observe") || strings.HasSuffix(path, "/runtime-observe-http"))
}

// ValidateServerBinding keeps tokenless browser compatibility local-only. A
// non-loopback listener must have an application token before it can start.
func ValidateServerBinding(address, token string) error {
	if strings.TrimSpace(token) != "" {
		return nil
	}
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return fmt.Errorf("invalid workbench address: %w", err)
	}
	if host == "localhost" {
		return nil
	}
	ip := net.ParseIP(host)
	if ip != nil && ip.IsLoopback() {
		return nil
	}
	return fmt.Errorf("desktop session token is required for non-loopback workbench address")
}

func ValidateRemoteManagementConfig(address, token, origin string) error {
	if origin == "" {
		return nil
	}
	if strings.TrimSpace(token) == "" {
		return fmt.Errorf("remote Workbench origin requires a desktop session token")
	}
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return fmt.Errorf("invalid workbench address: %w", err)
	}
	if host != "localhost" {
		ip := net.ParseIP(host)
		if ip == nil || !ip.IsLoopback() {
			return fmt.Errorf("remote Workbench mode requires a loopback listener behind the configured reverse proxy")
		}
	}
	if !isValidRemoteWorkbenchOrigin(origin) {
		return fmt.Errorf("remote Workbench origin must be one exact canonical HTTPS origin")
	}
	return nil
}

func isValidRemoteWorkbenchOrigin(origin string) bool {
	if origin == "" || origin != strings.TrimSpace(origin) || strings.Contains(origin, "*") {
		return false
	}
	parsed, err := url.ParseRequestURI(origin)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil || parsed.Opaque != "" ||
		parsed.Path != "" || parsed.RawQuery != "" || parsed.Fragment != "" || parsed.Hostname() == "" {
		return false
	}
	return origin == "https://"+parsed.Host
}

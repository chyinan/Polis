// pattern: Functional Core
package desktop

import (
	"fmt"
	"net"
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

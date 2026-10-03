// pattern: Imperative Shell
package desktop

import (
	"context"
	"encoding/json"
	"net/http"
)

// Middleware applies the ephemeral desktop token and the local Origin policy.
func Middleware(token string, next http.Handler) http.Handler {
	return MiddlewareWithRemoteOrigin(token, "", next)
}

// MiddlewareWithRemoteOrigin reuses an explicitly configured HTTPS reverse-proxy origin.
// The proxy must authenticate users and inject the service token; this handler
// never creates a listener, tunnel, or remote qualification.
func MiddlewareWithRemoteOrigin(token, remoteOrigin string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		origin := request.Header.Get("Origin")
		remoteConfigured := remoteOrigin != "" && isValidRemoteWorkbenchOrigin(remoteOrigin) && token != ""
		remoteRequest := remoteConfigured && origin == remoteOrigin
		localOriginRequest := origin != "" && AllowedOrigin(origin)
		if !AllowedOrigin(origin) && !remoteRequest {
			writeError(response, http.StatusForbidden, "desktop origin is not allowed")
			return
		}
		if origin != "" {
			response.Header().Set("Access-Control-Allow-Origin", origin)
			response.Header().Set("Vary", "Origin")
			response.Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type, X-Request-ID, X-Polis-Desktop-Token, Authorization")
			response.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			response.Header().Set("Access-Control-Expose-Headers", "Content-Disposition, Content-Length, X-Content-SHA256, X-Polis-Manifest-SHA256, X-Source-SHA256, X-Polis-Input-ID, X-Polis-Input-Revision, X-Polis-Preview-Filename")
		}
		if request.Method == http.MethodOptions {
			response.WriteHeader(http.StatusNoContent)
			return
		}
		queryToken := request.URL.Query().Get("desktop_token")
		if remoteConfigured && !localOriginRequest {
			queryToken = ""
		}
		presented := PresentedToken(request.Header.Get("X-Polis-Desktop-Token"), request.Header.Get("Authorization"), queryToken)
		if (token != "" && !TokenMatches(token, presented)) || (token == "" && RequiresSessionTokenPath(request.URL.Path)) {
			writeError(response, http.StatusUnauthorized, "desktop session token is missing or invalid")
			return
		}
		if token != "" && TokenMatches(token, presented) {
			request = request.WithContext(context.WithValue(request.Context(), installationOwnerContextKey{}, true))
		}
		next.ServeHTTP(response, request)
	})
}

func writeError(response http.ResponseWriter, status int, message string) {
	response.Header().Set("Content-Type", "application/json")
	response.WriteHeader(status)
	_ = json.NewEncoder(response).Encode(map[string]string{"error": message})
}

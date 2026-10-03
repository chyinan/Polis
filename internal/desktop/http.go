// pattern: Imperative Shell
package desktop

import (
	"context"
	"encoding/json"
	"net/http"

	"polis/internal/installationauth"
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
			response.Header().Set("Access-Control-Allow-Credentials", "true")
			response.Header().Set("Vary", "Origin")
			response.Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type, X-Request-ID, X-Polis-Desktop-Token, X-Polis-CSRF-Token, Authorization")
			response.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			response.Header().Set("Access-Control-Expose-Headers", "Content-Disposition, Content-Length, X-Content-SHA256, X-Polis-Manifest-SHA256, X-Source-SHA256, X-Polis-Input-ID, X-Polis-Input-Revision, X-Polis-Preview-Filename")
		}
		if request.Method == http.MethodOptions {
			response.WriteHeader(http.StatusNoContent)
			return
		}
		ownerPublicRequest := request.URL.Path == "/api/installation/owner/status" ||
			request.URL.Path == "/api/installation/owner/bootstrap" ||
			request.URL.Path == "/api/installation/owner/login" ||
			request.URL.Path == "/api/installation/owner/session"
		queryToken := request.URL.Query().Get("desktop_token")
		if remoteConfigured && !localOriginRequest {
			queryToken = ""
		}
		presented := PresentedToken(request.Header.Get("X-Polis-Desktop-Token"), request.Header.Get("Authorization"), queryToken)
		desktopTokenAuthenticated := token != "" && TokenMatches(token, presented)
		ownerSessionAuthenticated := installationauth.IsAuthenticated(request.Context())
		if !ownerPublicRequest && !desktopTokenAuthenticated && !ownerSessionAuthenticated &&
			(token != "" || RequiresSessionTokenPath(request.URL.Path)) {
			writeError(response, http.StatusUnauthorized, "desktop session token is missing or invalid")
			return
		}
		if ownerSessionAuthenticated && !desktopTokenAuthenticated && request.Method != http.MethodGet && request.Method != http.MethodHead {
			csrfCookie, _ := request.Cookie(installationauth.OwnerCSRFCookieName)
			csrfValue := ""
			if csrfCookie != nil {
				csrfValue = csrfCookie.Value
			}
			if origin == "" || !installationauth.CSRFValid(request.Context(), csrfValue, request.Header.Get(installationauth.OwnerCSRFHeaderName)) {
				writeError(response, http.StatusForbidden, "owner request failed CSRF verification")
				return
			}
		}
		if desktopTokenAuthenticated {
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

// pattern: Imperative Shell
package desktop

import (
	"encoding/json"
	"net/http"
)

// Middleware applies the ephemeral desktop token when configured and always
// enforces the narrow local Origin policy. Tokenless browser mode remains
// loopback-only at server startup and cannot accept foreign-origin writes.
func Middleware(token string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		origin := request.Header.Get("Origin")
		if !AllowedOrigin(origin) {
			writeError(response, http.StatusForbidden, "desktop origin is not allowed")
			return
		}
		if origin != "" {
			response.Header().Set("Access-Control-Allow-Origin", origin)
			response.Header().Set("Vary", "Origin")
			response.Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type, X-Request-ID, X-Polis-Desktop-Token, Authorization")
			response.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		}
		if request.Method == http.MethodOptions {
			response.WriteHeader(http.StatusNoContent)
			return
		}
		presented := PresentedToken(request.Header.Get("X-Polis-Desktop-Token"), request.Header.Get("Authorization"), request.URL.Query().Get("desktop_token"))
		if token != "" && !TokenMatches(token, presented) {
			writeError(response, http.StatusUnauthorized, "desktop session token is missing or invalid")
			return
		}
		next.ServeHTTP(response, request)
	})
}

func writeError(response http.ResponseWriter, status int, message string) {
	response.Header().Set("Content-Type", "application/json")
	response.WriteHeader(status)
	_ = json.NewEncoder(response).Encode(map[string]string{"error": message})
}

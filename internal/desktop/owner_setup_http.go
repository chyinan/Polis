package desktop

import (
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"polis/internal/installationauth"
)

type ownerBootstrapRequest struct {
	Code     string `json:"code"`
	Password string `json:"password"`
}

type ownerLoginRequest struct {
	Password string `json:"password"`
}

// OwnerSetupHandler exposes local owner bootstrap and revocable owner sessions.
func OwnerSetupHandler(store *installationauth.Store, secureCookies bool) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		response.Header().Set("Cache-Control", "no-store")
		response.Header().Set("X-Content-Type-Options", "nosniff")
		switch request.URL.Path {
		case "/api/installation/owner/status":
			if !isLocalOwnerSetupStatusRequest(request) {
				writeError(response, http.StatusForbidden, "owner setup is available only from the local Workbench")
				return
			}
			if request.Method != http.MethodGet {
				response.Header().Set("Allow", http.MethodGet)
				writeError(response, http.StatusMethodNotAllowed, "method not allowed")
				return
			}
			initialized, err := store.OwnerInitialized(request.Context())
			if err != nil {
				writeError(response, http.StatusServiceUnavailable, "owner setup state is unavailable")
				return
			}
			writeOwnerSetupJSON(response, http.StatusOK, map[string]bool{"initialized": initialized})
		case "/api/installation/owner/bootstrap":
			if !isLocalOwnerSetupRequest(request) {
				writeError(response, http.StatusForbidden, "owner setup is available only from the local Workbench")
				return
			}
			serveOwnerBootstrap(store, response, request)
		case "/api/installation/owner/login":
			serveOwnerLogin(store, response, request, secureCookies)
		case "/api/installation/owner/session":
			serveOwnerSession(response, request)
		case "/api/installation/owner/logout":
			serveOwnerLogout(store, response, request, secureCookies)
		default:
			writeError(response, http.StatusNotFound, "owner setup route not found")
		}
	})
}

func serveOwnerBootstrap(store *installationauth.Store, response http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		response.Header().Set("Allow", http.MethodPost)
		writeError(response, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	request.Body = http.MaxBytesReader(response, request.Body, 4096)
	decoder := json.NewDecoder(request.Body)
	decoder.DisallowUnknownFields()
	var input ownerBootstrapRequest
	if err := decoder.Decode(&input); err != nil {
		writeError(response, http.StatusBadRequest, "invalid owner bootstrap request")
		return
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		writeError(response, http.StatusBadRequest, "invalid owner bootstrap request")
		return
	}
	password := []byte(input.Password)
	input.Password = ""
	defer clear(password)
	err := store.InitializeOwner(request.Context(), strings.TrimSpace(input.Code), password)
	if err == nil {
		writeOwnerSetupJSON(response, http.StatusCreated, map[string]bool{"initialized": true})
		return
	}
	switch {
	case errors.Is(err, installationauth.ErrBootstrapRateLimited):
		response.Header().Set("Retry-After", "900")
		writeError(response, http.StatusTooManyRequests, "owner bootstrap is temporarily rate limited")
	case errors.Is(err, installationauth.ErrOwnerAlreadyInitialized):
		writeError(response, http.StatusConflict, "installation owner is already initialized")
	case errors.Is(err, installationauth.ErrBootstrapUnavailable):
		writeError(response, http.StatusBadRequest, "owner bootstrap code is invalid or expired")
	case errors.Is(err, installationauth.ErrOwnerPasswordPolicy):
		writeError(response, http.StatusBadRequest, "owner password must contain 14 to 1024 bytes")
	default:
		writeError(response, http.StatusServiceUnavailable, "owner setup could not be completed")
	}
}

func isLocalOwnerSetupRequest(request *http.Request) bool {
	if request == nil {
		return false
	}
	origin := request.Header.Get("Origin")
	if origin == "" || !AllowedOrigin(origin) {
		return false
	}
	host, _, err := net.SplitHostPort(request.RemoteAddr)
	if err != nil {
		return false
	}
	address := net.ParseIP(host)
	return address != nil && address.IsLoopback()
}

func isLocalOwnerSetupStatusRequest(request *http.Request) bool {
	if request == nil {
		return false
	}
	origin := request.Header.Get("Origin")
	if origin != "" && !AllowedOrigin(origin) {
		return false
	}
	host, _, err := net.SplitHostPort(request.RemoteAddr)
	if err != nil {
		return false
	}
	address := net.ParseIP(host)
	return address != nil && address.IsLoopback()
}

func serveOwnerLogin(store *installationauth.Store, response http.ResponseWriter, request *http.Request, secureCookies bool) {
	if request.Method != http.MethodPost {
		response.Header().Set("Allow", http.MethodPost)
		writeError(response, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if request.Header.Get("Origin") == "" {
		writeError(response, http.StatusForbidden, "owner login requires an approved origin")
		return
	}
	request.Body = http.MaxBytesReader(response, request.Body, 2048)
	decoder := json.NewDecoder(request.Body)
	decoder.DisallowUnknownFields()
	var input ownerLoginRequest
	if err := decoder.Decode(&input); err != nil {
		writeError(response, http.StatusBadRequest, "invalid owner login request")
		return
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		writeError(response, http.StatusBadRequest, "invalid owner login request")
		return
	}
	password := []byte(input.Password)
	input.Password = ""
	defer clear(password)
	result, err := store.Login(request.Context(), password)
	if err != nil {
		switch {
		case errors.Is(err, installationauth.ErrOwnerLoginRateLimited):
			response.Header().Set("Retry-After", "900")
			writeError(response, http.StatusTooManyRequests, "owner login is temporarily rate limited")
		case errors.Is(err, installationauth.ErrInvalidOwnerCredentials), errors.Is(err, installationauth.ErrOwnerNotInitialized):
			writeError(response, http.StatusUnauthorized, "owner credentials are invalid")
		default:
			writeError(response, http.StatusServiceUnavailable, "owner login is unavailable")
		}
		return
	}
	maxAge := int(time.Until(result.ExpiresAt).Seconds())
	if maxAge < 1 {
		writeError(response, http.StatusServiceUnavailable, "owner login is unavailable")
		return
	}
	http.SetCookie(response, &http.Cookie{
		Name: installationauth.OwnerSessionCookieName, Value: result.SessionToken, Path: "/api",
		Expires: result.ExpiresAt, MaxAge: maxAge, HttpOnly: true, Secure: secureCookies, SameSite: http.SameSiteStrictMode,
	})
	http.SetCookie(response, &http.Cookie{
		Name: installationauth.OwnerCSRFCookieName, Value: result.CSRFToken, Path: "/",
		Expires: result.ExpiresAt, MaxAge: maxAge, Secure: secureCookies, SameSite: http.SameSiteStrictMode,
	})
	writeOwnerSetupJSON(response, http.StatusOK, map[string]any{"authenticated": true, "expiresAt": result.ExpiresAt})
}

func serveOwnerSession(response http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		response.Header().Set("Allow", http.MethodGet)
		writeError(response, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if !installationauth.IsAuthenticated(request.Context()) {
		writeOwnerSetupJSON(response, http.StatusOK, map[string]any{"authenticated": false, "expiresAt": nil})
		return
	}
	var expiresAt any
	if expiry, ok := installationauth.SessionExpiry(request.Context()); ok {
		expiresAt = expiry
	}
	writeOwnerSetupJSON(response, http.StatusOK, map[string]any{"authenticated": true, "expiresAt": expiresAt})
}

func serveOwnerLogout(store *installationauth.Store, response http.ResponseWriter, request *http.Request, secureCookies bool) {
	if request.Method != http.MethodPost {
		response.Header().Set("Allow", http.MethodPost)
		writeError(response, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if request.Header.Get("Origin") == "" {
		writeError(response, http.StatusForbidden, "owner logout requires an approved origin")
		return
	}
	if !installationauth.IsAuthenticated(request.Context()) {
		writeError(response, http.StatusUnauthorized, "installation owner authentication is required")
		return
	}
	if cookie, err := request.Cookie(installationauth.OwnerSessionCookieName); err == nil && cookie.Value != "" {
		if err = store.RevokeSession(request.Context(), cookie.Value); err != nil {
			writeError(response, http.StatusServiceUnavailable, "owner session could not be revoked")
			return
		}
	}
	for _, name := range []string{installationauth.OwnerSessionCookieName, installationauth.OwnerCSRFCookieName} {
		path := "/api"
		if name == installationauth.OwnerCSRFCookieName {
			path = "/"
		}
		http.SetCookie(response, &http.Cookie{Name: name, Value: "", Path: path, MaxAge: -1, Expires: time.Unix(1, 0), HttpOnly: name == installationauth.OwnerSessionCookieName, Secure: secureCookies, SameSite: http.SameSiteStrictMode})
	}
	writeOwnerSetupJSON(response, http.StatusOK, map[string]bool{"authenticated": false})
}

func writeOwnerSetupJSON(response http.ResponseWriter, status int, value any) {
	response.Header().Set("Content-Type", "application/json; charset=utf-8")
	response.WriteHeader(status)
	_ = json.NewEncoder(response).Encode(value)
}

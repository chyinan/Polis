package installationauth

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestCSRFValidRequiresMatchingTokenBoundToOwnerSession(t *testing.T) {
	token := "csrf-secret-value"
	digest := sha256.Sum256([]byte(token))
	ctx := context.WithValue(context.Background(), sessionContextKey{}, authenticatedOwnerSession{
		csrfSHA256: hex.EncodeToString(digest[:]),
		expiresAt:  time.Now().Add(time.Hour),
	})
	if !IsAuthenticated(ctx) || !CSRFValid(ctx, token, token) {
		t.Fatal("authenticated session did not accept its bound CSRF token")
	}
	if CSRFValid(ctx, token, "different-token") || CSRFValid(context.Background(), token, token) {
		t.Fatal("CSRF verification accepted a mismatched token or unauthenticated context")
	}
}

func TestSessionMiddlewareIgnoresMissingOrUnverifiableCookie(t *testing.T) {
	store := NewStore(nil)
	handler := store.SessionMiddleware(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if IsAuthenticated(request.Context()) {
			t.Fatal("invalid session cookie authenticated request")
		}
	}))
	request := httptest.NewRequest(http.MethodGet, "/api/workbench/installation/provider-accounts", nil)
	request.AddCookie(&http.Cookie{Name: OwnerSessionCookieName, Value: "invalid"})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("middleware response status=%d", response.Code)
	}
}

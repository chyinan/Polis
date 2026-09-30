// pattern: Functional Core
package github

import (
	"context"
	"errors"
	"strings"
	"testing"
)

type memoryGitHubCredentialStore struct {
	token string
	err   error
	loads int
}

func (s *memoryGitHubCredentialStore) StoreToken(string, string) error { return nil }
func (s *memoryGitHubCredentialStore) LoadToken(string) (string, error) {
	s.loads++
	return s.token, s.err
}
func (s *memoryGitHubCredentialStore) DeleteToken(string) error { return nil }

func TestProtectedGitHubTokenSourceLoadsOnlyForValidFixedRepository(t *testing.T) {
	store := &memoryGitHubCredentialStore{token: "github_pat_" + strings.Repeat("A", 40)}
	source := ProtectedGitHubTokenSource(store, "readonly")
	got, err := source(context.Background(), Repository{ID: 1296269, Owner: "acme", Name: "widget"})
	if err != nil || got != store.token || store.loads != 1 {
		t.Fatalf("protected token source result=%q loads=%d err=%v", got, store.loads, err)
	}
	if _, err = source(context.Background(), Repository{ID: 0, Owner: "../acme", Name: "widget"}); !errors.Is(err, ErrGitHubTokenUnavailable) || store.loads != 1 {
		t.Fatalf("invalid repository reached credential store: loads=%d err=%v", store.loads, err)
	}
}

func TestProtectedGitHubTokenSourceRedactsCredentialStoreFailures(t *testing.T) {
	store := &memoryGitHubCredentialStore{err: errors.New("disk error leaked_token_value")}
	source := ProtectedGitHubTokenSource(store, "readonly")
	_, err := source(context.Background(), Repository{ID: 1296269, Owner: "acme", Name: "widget"})
	if !errors.Is(err, ErrGitHubTokenUnavailable) || strings.Contains(err.Error(), "leaked_token_value") {
		t.Fatalf("credential failure was not safely classified: %v", err)
	}
}

func TestGitHubTokenAndCredentialReferenceValidation(t *testing.T) {
	valid := "ghp_" + strings.Repeat("a", 36)
	if !ValidGitHubToken(valid) || !ValidGitHubCredentialRef("default-readonly") {
		t.Fatal("valid token or credential reference was rejected")
	}
	for _, token := range []string{"", "short", "bad token with spaces", "ghp_" + strings.Repeat("x", 600)} {
		if ValidGitHubToken(token) {
			t.Fatalf("invalid token was accepted: %q", token)
		}
	}
	for _, reference := range []string{"", "../token", "with space", strings.Repeat("a", 65)} {
		if ValidGitHubCredentialRef(reference) {
			t.Fatalf("invalid credential reference was accepted: %q", reference)
		}
	}
}

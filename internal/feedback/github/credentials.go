// pattern: Functional Core
package github

import (
	"context"
	"errors"
	"strings"
)

var ErrGitHubTokenUnavailable = errors.New(ReasonTokenUnavailable)
var ErrProtectedGitHubCredentialStoreUnavailable = errors.New("protected GitHub credential storage is unavailable")

type GitHubCredentialStore interface {
	StoreToken(credentialRef, token string) error
	LoadToken(credentialRef string) (string, error)
	DeleteToken(credentialRef string) error
}

func ValidGitHubCredentialRef(value string) bool {
	if len(value) == 0 || len(value) > 64 {
		return false
	}
	for _, character := range value {
		if (character >= 'A' && character <= 'Z') || (character >= 'a' && character <= 'z') || (character >= '0' && character <= '9') || character == '_' || character == '-' {
			continue
		}
		return false
	}
	return true
}

func ValidGitHubToken(value string) bool {
	if len(value) < 20 || len(value) > 512 || value != strings.TrimSpace(value) {
		return false
	}
	for _, character := range value {
		if (character >= 'A' && character <= 'Z') || (character >= 'a' && character <= 'z') || (character >= '0' && character <= '9') || character == '_' {
			continue
		}
		return false
	}
	return true
}

// ProtectedGitHubTokenSource exposes a stored token only for a fixed,
// syntactically valid repository. Storage errors and invalid token contents
// become one redacted reason code.
func ProtectedGitHubTokenSource(store GitHubCredentialStore, credentialRef string) TokenSource {
	return func(ctx context.Context, repository Repository) (string, error) {
		if ctx.Err() != nil || store == nil || !ValidRepository(repository) || !ValidGitHubCredentialRef(credentialRef) {
			return "", ErrGitHubTokenUnavailable
		}
		token, err := store.LoadToken(credentialRef)
		if err != nil || !ValidGitHubToken(token) {
			return "", ErrGitHubTokenUnavailable
		}
		return token, nil
	}
}

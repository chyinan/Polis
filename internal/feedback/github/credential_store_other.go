// pattern: Imperative Shell
//go:build !windows && !linux

package github

func NewProtectedGitHubCredentialStore() (GitHubCredentialStore, error) {
	return nil, ErrProtectedGitHubCredentialStoreUnavailable
}

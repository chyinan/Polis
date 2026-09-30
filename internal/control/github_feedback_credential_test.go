// pattern: Imperative Shell
package control

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	githubfeedback "polis/internal/feedback/github"
)

type recordingGitHubCredentialStore struct {
	ref       string
	token     string
	deleteRef string
}

func (s *recordingGitHubCredentialStore) StoreToken(reference, token string) error {
	s.ref, s.token = reference, token
	return nil
}
func (s *recordingGitHubCredentialStore) LoadToken(string) (string, error) { return s.token, nil }
func (s *recordingGitHubCredentialStore) DeleteToken(reference string) error {
	s.deleteRef = reference
	s.token = ""
	return nil
}

func TestControlStoresAndDeletesGitHubCredentialWithoutReturningSecret(t *testing.T) {
	store := &recordingGitHubCredentialStore{}
	service := &Service{githubCredentials: store}
	token := "github_pat_" + strings.Repeat("A", 40)
	receipt, err := service.StoreGitHubFeedbackCredential(context.Background(), token)
	if err != nil || receipt.CredentialRef != GitHubFeedbackCredentialRef || !receipt.Stored || store.token != token {
		t.Fatalf("credential store receipt=%+v stored=%t err=%v", receipt, store.token == token, err)
	}
	encoded, err := json.Marshal(receipt)
	if err != nil || strings.Contains(string(encoded), token) {
		t.Fatalf("credential receipt leaked token: %s err=%v", encoded, err)
	}
	deleted, err := service.DeleteGitHubFeedbackCredential(context.Background())
	if err != nil || deleted.Stored || store.deleteRef != GitHubFeedbackCredentialRef || store.token != "" {
		t.Fatalf("credential delete receipt=%+v ref=%q tokenCleared=%t err=%v", deleted, store.deleteRef, store.token == "", err)
	}
}

func TestControlKeepsGitHubCredentialStoreUnavailableClosed(t *testing.T) {
	service := &Service{}
	if _, err := service.StoreGitHubFeedbackCredential(context.Background(), "github_pat_"+strings.Repeat("A", 40)); !errors.Is(err, githubfeedback.ErrProtectedGitHubCredentialStoreUnavailable) {
		t.Fatalf("credential store without protected backend error=%v", err)
	}
}

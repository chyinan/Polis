// pattern: Imperative Shell
//go:build windows

package github

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestProtectedGitHubCredentialStoreRoundTripAndRotation(t *testing.T) {
	root := t.TempDir()
	store := &protectedGitHubCredentialStore{root: root}
	first := "github_pat_" + strings.Repeat("A", 40)
	second := "github_pat_" + strings.Repeat("B", 40)
	if err := store.StoreToken("default-readonly", first); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "github-read-default-readonly.dpapi")
	protected, err := os.ReadFile(path)
	if err != nil || strings.Contains(string(protected), first) {
		t.Fatalf("credential file is missing or contains cleartext: err=%v", err)
	}
	if got, err := store.LoadToken("default-readonly"); err != nil || got != first {
		t.Fatalf("protected credential load = %q, err=%v", got, err)
	}
	if err = store.StoreToken("default-readonly", second); err != nil {
		t.Fatal(err)
	}
	if got, err := store.LoadToken("default-readonly"); err != nil || got != second {
		t.Fatalf("rotated protected credential load = %q, err=%v", got, err)
	}
	if err = store.DeleteToken("default-readonly"); err != nil {
		t.Fatal(err)
	}
	if _, err = store.LoadToken("default-readonly"); err == nil {
		t.Fatal("deleted protected credential remained readable")
	}
}

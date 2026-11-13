package kernel

import (
	"os"
	"path/filepath"
	"testing"
)

func TestOpenAICredentialFileStaysOutsideTheCompanyTree(t *testing.T) {
	root := t.TempDir()
	k := &Kernel{root: root}
	if err := k.WriteOpenAIAPIKey("company-1", "sk-test-value"); err != nil {
		t.Fatal(err)
	}
	if !k.OpenAICredentialConfigured("company-1") {
		t.Fatal("credential was not readable after write")
	}
	if k.OpenAICredentialConfigured("../company-1") {
		t.Fatal("credential path accepted a company id outside the id grammar")
	}
	path := filepath.Join(root, "credentials", "company-1", "openai-api-key")
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
	if err := k.ClearOpenAIAPIKey("company-1"); err != nil {
		t.Fatal(err)
	}
	if k.OpenAICredentialConfigured("company-1") {
		t.Fatal("credential remained after clear")
	}
}

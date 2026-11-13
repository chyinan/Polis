package kernel

import (
	"os"
	"path/filepath"
	"strings"

	"polis/internal/core"
)

func (k *Kernel) providerCredentialPath(companyID, filename string) (string, error) {
	if k == nil || strings.TrimSpace(k.root) == "" || !core.ValidID(companyID) || filename == "" || strings.Contains(filename, "/") || strings.Contains(filename, "\\") {
		return "", core.Malformed
	}
	return filepath.Join(k.root, "credentials", companyID, filename), nil
}

// WriteOpenAIAPIKey stores a provider credential outside the command receipt.
// The file is the only copy; callers must not put the key in a transaction payload.
func (k *Kernel) writeCredential(companyID, filename, key string) error {
	path, err := k.providerCredentialPath(companyID, filename)
	if err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(key), 0o600)
}

func (k *Kernel) clearCredential(companyID, filename string) error {
	path, err := k.providerCredentialPath(companyID, filename)
	if err != nil {
		return err
	}
	if err = os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func (k *Kernel) credentialConfigured(companyID, filename string) bool {
	path, err := k.providerCredentialPath(companyID, filename)
	if err != nil {
		return false
	}
	raw, err := os.ReadFile(path)
	return err == nil && strings.TrimSpace(string(raw)) != ""
}

func (k *Kernel) WriteOpenAIAPIKey(companyID, key string) error {
	return k.writeCredential(companyID, "openai-api-key", key)
}

func (k *Kernel) ClearOpenAIAPIKey(companyID string) error {
	return k.clearCredential(companyID, "openai-api-key")
}

func (k *Kernel) OpenAICredentialConfigured(companyID string) bool {
	return k.credentialConfigured(companyID, "openai-api-key")
}

func (k *Kernel) WriteDeepSeekAPIKey(companyID, key string) error {
	return k.writeCredential(companyID, "deepseek-api-key", key)
}

func (k *Kernel) ClearDeepSeekAPIKey(companyID string) error {
	return k.clearCredential(companyID, "deepseek-api-key")
}

func (k *Kernel) DeepSeekCredentialConfigured(companyID string) bool {
	return k.credentialConfigured(companyID, "deepseek-api-key")
}

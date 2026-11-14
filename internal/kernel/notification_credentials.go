package kernel

import (
	"os"
	"strings"

	"polis/internal/core"
)

func (k *Kernel) notificationSecretFilename(adapter string) (string, error) {
	switch adapter {
	case "webhook":
		return "notification-webhook-secret", nil
	case "qq_official":
		return "notification-qq-app-secret", nil
	default:
		return "", core.Malformed
	}
}

func (k *Kernel) WriteNotificationSecret(companyID, adapter, secret string) error {
	filename, err := k.notificationSecretFilename(adapter)
	if err != nil {
		return err
	}
	return k.writeCredential(companyID, filename, strings.TrimSpace(secret))
}

func (k *Kernel) ClearNotificationSecret(companyID, adapter string) error {
	filename, err := k.notificationSecretFilename(adapter)
	if err != nil {
		return err
	}
	return k.clearCredential(companyID, filename)
}

func (k *Kernel) NotificationSecretConfigured(companyID, adapter string) bool {
	filename, err := k.notificationSecretFilename(adapter)
	if err != nil {
		return false
	}
	return k.credentialConfigured(companyID, filename)
}

func (k *Kernel) ReadNotificationSecret(companyID, adapter string) (string, error) {
	filename, err := k.notificationSecretFilename(adapter)
	if err != nil {
		return "", err
	}
	path, err := k.providerCredentialPath(companyID, filename)
	if err != nil {
		return "", err
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	value := strings.TrimSpace(string(raw))
	if value == "" {
		return "", core.Malformed
	}
	return value, nil
}

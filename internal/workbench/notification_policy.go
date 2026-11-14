// pattern: Functional Core
package workbench

import (
	"os"
	"path/filepath"
	"strings"
)

func notificationDestinationForView(adapter, destination string) string {
	switch adapter {
	case "webhook", "qq_official":
		if destination != "" {
			return "[redacted]"
		}
	}
	return destination
}

func notificationSecretConfiguredInRoot(root, companyID, adapter string) bool {
	if strings.TrimSpace(root) == "" || strings.TrimSpace(companyID) == "" {
		return false
	}
	var filename string
	switch adapter {
	case "webhook":
		filename = "notification-webhook-secret"
	case "qq_official":
		filename = "notification-qq-app-secret"
	default:
		return false
	}
	path := filepath.Join(filepath.Clean(root), "credentials", companyID, filename)
	raw, err := os.ReadFile(path)
	return err == nil && strings.TrimSpace(string(raw)) != ""
}

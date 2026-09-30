// pattern: Functional Core
package workbench

import "strings"

func notificationDestinationForView(adapter, destination string) string {
	if adapter == "webhook" && destination != "" {
		return "[redacted]"
	}
	if adapter == "qq_official" {
		if destination == "" {
			return "not configured"
		}
		runes := []rune(destination)
		if len(runes) > 4 {
			destination = string(runes[len(runes)-4:])
		}
		return "admin C2C ••••" + strings.TrimSpace(destination)
	}
	return destination
}

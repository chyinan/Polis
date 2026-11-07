// pattern: Functional Core
package workbench

func notificationDestinationForView(adapter, destination string) string {
	if adapter == "webhook" && destination != "" {
		return "[redacted]"
	}
	return destination
}

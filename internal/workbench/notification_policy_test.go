// pattern: Functional Core
package workbench

import "testing"

func TestNotificationDestinationRedactsWebhookSecrets(t *testing.T) {
	if got := notificationDestinationForView("webhook", "https://hooks.example/send?token=secret"); got != "[redacted]" {
		t.Fatalf("webhook destination=%q, want [redacted]", got)
	}
	if got := notificationDestinationForView("local", "local://workbench"); got != "local://workbench" {
		t.Fatalf("local destination=%q", got)
	}
}

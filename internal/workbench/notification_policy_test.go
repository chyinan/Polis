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

func TestNotificationDestinationMasksQQAdminOpenID(t *testing.T) {
	if got := notificationDestinationForView("qq_official", "admin-openid-1234"); got != "admin C2C \u2022\u2022\u2022\u20221234" {
		t.Fatalf("QQ destination=%q, want masked admin target", got)
	}
	if got := notificationDestinationForView("qq_official", ""); got != "not configured" {
		t.Fatalf("empty QQ destination=%q", got)
	}
}

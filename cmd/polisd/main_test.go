// pattern: Imperative Shell
package main

import (
	"strings"
	"testing"
)

func TestQQNotificationSenderIsDisabledByDefault(t *testing.T) {
	t.Setenv("POLIS_QQ_ENABLED", "")
	if err := runNotificationSender("127.0.0.1:8099"); err == nil || !strings.Contains(err.Error(), "disabled") {
		t.Fatalf("default notification sender error = %v", err)
	}
}

func TestQQNotificationSenderRejectsNonLoopbackBeforeCredentialRead(t *testing.T) {
	t.Setenv("POLIS_QQ_ENABLED", "1")
	if err := runNotificationSender("0.0.0.0:8099"); err == nil || !strings.Contains(err.Error(), "loopback") {
		t.Fatalf("non-loopback sender error = %v", err)
	}
}

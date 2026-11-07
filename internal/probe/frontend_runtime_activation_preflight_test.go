// pattern: Functional Core
package probe

import "testing"

func TestValidateExecutionEnvelopeFingerprintRejectsMissingOrDrift(t *testing.T) {
	if err := ValidateExecutionEnvelopeFingerprint("", "fingerprint"); err == nil {
		t.Fatal("missing expected envelope fingerprint was accepted")
	}
	if err := ValidateExecutionEnvelopeFingerprint("expected", "actual"); err == nil {
		t.Fatal("drifted envelope fingerprint was accepted")
	}
	if err := ValidateExecutionEnvelopeFingerprint("expected", "expected"); err != nil {
		t.Fatalf("exact envelope fingerprint rejected: %v", err)
	}
}

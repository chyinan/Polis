// pattern: Functional Core
package probe

import "testing"

func TestCanaryProtocolSentinelRequiresAssistantOutput(t *testing.T) {
	raw := []byte("{\"time\":\"2026-09-09T00:00:00Z\",\"direction\":\"send\",\"data\":{\"method\":\"turn/start\",\"params\":{\"input\":[{\"text\":\"Reply with exactly: POLIS_TRANSPORT_CANARY_OK\"}]}}}\n")
	summary, err := analyzeCanaryProtocol(raw, "POLIS_TRANSPORT_CANARY_OK")
	if err != nil {
		t.Fatal(err)
	}
	if summary.SentinelMatch || summary.FirstValidOutput {
		t.Fatal("prompt text was incorrectly treated as assistant output")
	}
}

func TestCanaryProtocolExtractsAssistantOutputAndReconnectLifecycle(t *testing.T) {
	raw := []byte("{\"time\":\"2026-09-09T00:00:00Z\",\"direction\":\"receive\",\"data\":{\"method\":\"turn/started\",\"params\":{}}}\n" +
		"{\"time\":\"2026-09-09T00:00:00.100Z\",\"direction\":\"receive\",\"data\":{\"method\":\"error\",\"params\":{\"willRetry\":true,\"error\":{\"codexErrorInfo\":{\"responseStreamDisconnected\":{}}}}}}\n" +
		"{\"time\":\"2026-09-09T00:00:01Z\",\"direction\":\"receive\",\"data\":{\"method\":\"item/completed\",\"params\":{\"item\":{\"type\":\"agentMessage\",\"text\":\"POLIS_TRANSPORT_CANARY_OK\"}}}}\n" +
		"{\"time\":\"2026-09-09T00:00:02Z\",\"direction\":\"receive\",\"data\":{\"method\":\"turn/completed\",\"params\":{\"turn\":{\"status\":\"completed\"}}}}\n")
	summary, err := analyzeCanaryProtocol(raw, "POLIS_TRANSPORT_CANARY_OK")
	if err != nil {
		t.Fatal(err)
	}
	if !summary.SentinelMatch || !summary.FirstValidOutput || summary.FirstValidOutputDeltaMS != 1000 || summary.ReconnectCount != 1 || summary.RecoveryDeltaMS != 900 || summary.TerminalState != "completed" {
		t.Fatalf("incorrect canary summary: %+v", summary)
	}
}

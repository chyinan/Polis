// pattern: Imperative Shell
package probe

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDeriveT20DeltaOutputIgnoresCompletedItemText(t *testing.T) {
	path := filepath.Join(t.TempDir(), "provider-events.jsonl")
	contents := "{\"method\":\"item/agentMessage/delta\",\"text\":\"POLIS_\"}\n" +
		"{\"method\":\"item/agentMessage/delta\",\"text\":\"TRANSPORT_CANARY_OK\"}\n" +
		"{\"method\":\"item/completed\",\"text\":\"POLIS_TRANSPORT_CANARY_OK\"}\n"
	if err := os.WriteFile(path, []byte(contents), 0600); err != nil {
		t.Fatal(err)
	}
	output, recorded, err := deriveT20DeltaOutput(path)
	if err != nil {
		t.Fatal(err)
	}
	if !recorded || output != r03aT20Sentinel {
		t.Fatalf("unexpected derived output: recorded=%v output=%q", recorded, output)
	}
}

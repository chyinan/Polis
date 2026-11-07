// pattern: Imperative Shell
package probe

import (
	"os"
	"path/filepath"
	"testing"
)

func TestT21DOfflineQualificationUsesCurrentRegistryAndApprovedPolicyDiff(t *testing.T) {
	dir := t.TempDir()
	parent := filepath.Join("..", "..", "evidence", "development", "r0.3a-t21b")
	t21c := filepath.Join("..", "..", "evidence", "development", "r0.3a-t21c")
	record, err := RecordR03AT21DOfflineQualification(dir, parent, t21c)
	if err != nil {
		t.Fatal(err)
	}
	if err = VerifyR03AT21DOfflineQualification(dir); err != nil {
		t.Fatal(err)
	}
	if record.ToolCount != 11 || len(record.SemanticDiff) != 6 || record.UnrelatedDrift || record.Medium != 0 || record.High != 0 || record.ProviderEgress != 0 {
		t.Fatalf("unexpected T21D record: %+v", record)
	}
	for _, name := range []string{"tool-registry.json", "tool-surface.json", "execution-config.json", "execution-manifest.json", "preflight.json", "semantic-diff.json", "offline-result.json"} {
		if _, err = os.Stat(filepath.Join(dir, name)); err != nil {
			t.Fatal(err)
		}
	}
}

package probe

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadT8AuthFingerprintReadsPathBAuth(t *testing.T) {
	path := filepath.Join(t.TempDir(), "execution-diff.json")
	if err := os.WriteFile(path, append([]byte{0xef, 0xbb, 0xbf}, []byte(`{"B":{"auth":{"sha256":"opaque-auth-fingerprint"}}}`)...), 0600); err != nil {
		t.Fatal(err)
	}

	got, err := readT8AuthFingerprint(path)
	if err != nil {
		t.Fatal(err)
	}
	if got != "opaque-auth-fingerprint" {
		t.Fatalf("fingerprint = %q", got)
	}
}

func TestReadT8AuthFingerprintRejectsMissingEvidence(t *testing.T) {
	path := filepath.Join(t.TempDir(), "execution-diff.json")
	if err := os.WriteFile(path, []byte(`{"B":{"auth":{}}}`), 0600); err != nil {
		t.Fatal(err)
	}

	if _, err := readT8AuthFingerprint(path); err == nil {
		t.Fatal("expected missing fingerprint to fail preflight")
	}
}

// pattern: Imperative Shell
package control

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

func TestExecutableFileSHA256BindsExactRegularFileBytes(t *testing.T) {
	path := filepath.Join(t.TempDir(), "runner")
	content := []byte("trusted go runner build")
	if err := os.WriteFile(path, content, 0o700); err != nil {
		t.Fatal(err)
	}
	digest, err := executableFileSHA256(path)
	if err != nil {
		t.Fatal(err)
	}
	wantBytes := sha256.Sum256(content)
	if digest != hex.EncodeToString(wantBytes[:]) {
		t.Fatalf("executableFileSHA256() = %s, want exact executable digest", digest)
	}
	if err = os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if _, err = executableFileSHA256(path); err == nil {
		t.Fatal("missing executable path was accepted")
	}
}

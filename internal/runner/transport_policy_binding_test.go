// pattern: Functional Core
package runner

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNativeLaunchEnvelopeFingerprintChangesWithTransportPolicy(t *testing.T) {
	root := t.TempDir()
	binary := filepath.Join(root, "codex.exe")
	helper := filepath.Join(root, "helper.exe")
	if err := os.WriteFile(binary, []byte("binary"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(helper, []byte("helper"), 0o700); err != nil {
		t.Fatal(err)
	}
	envelope, err := DescribeNativeLaunch(binary, helper, filepath.Join(root, "home"), []string{binary, "app-server", "--stdio"}, nil, root)
	if err != nil {
		t.Fatal(err)
	}
	base := BindTransportPolicy(envelope, NativeTransportPolicyBinding{Revision: "r03a-transport-policy@1", ReconnectGraceMS: 30000})
	drifted := BindTransportPolicy(envelope, NativeTransportPolicyBinding{Revision: "r03a-transport-policy@test", ReconnectGraceMS: 30000})
	if base.Fingerprint == "" || base.Fingerprint == drifted.Fingerprint {
		t.Fatalf("transport policy did not affect envelope fingerprint: base=%s drifted=%s", base.Fingerprint, drifted.Fingerprint)
	}
}

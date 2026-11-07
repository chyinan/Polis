// pattern: Functional Core
package probe

import (
	"strings"
	"testing"

	"polis/internal/codex"
)

func TestCompareT12AuthSnapshotRequiresExactV2Match(t *testing.T) {
	base := t12TestAuthManifest()
	if err := CompareT12AuthSnapshot(base, base); err != nil {
		t.Fatalf("identical auth snapshot rejected: %v", err)
	}

	identityChanged := base
	identityChanged.AuthIdentityFingerprint = strings.Repeat("b", 64)
	if err := CompareT12AuthSnapshot(base, identityChanged); err == nil {
		t.Fatal("identity change was accepted before live invocation")
	}

	revisionChanged := base
	revisionChanged.AuthCredentialRevisionFingerprint = strings.Repeat("d", 64)
	if err := CompareT12AuthSnapshot(base, revisionChanged); err == nil {
		t.Fatal("credential revision change was accepted before live invocation")
	}
}

func t12TestAuthManifest() codex.AuthFingerprintManifest {
	return codex.AuthFingerprintManifest{
		AuthSourceClass:                                "mounted_codex_auth_file",
		AuthIdentityFingerprintSchemaVersion:           codex.AuthIdentityFingerprintSchemaVersion,
		AuthIdentityFingerprint:                        strings.Repeat("a", 64),
		AuthIdentityFingerprintStatus:                  "available",
		AuthCredentialRevisionFingerprintSchemaVersion: codex.AuthCredentialRevisionFingerprintSchemaVersion,
		AuthCredentialRevisionFingerprint:              strings.Repeat("c", 64),
		AuthCredentialRevisionFingerprintStatus:        "available",
	}
}

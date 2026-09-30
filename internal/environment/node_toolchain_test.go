// pattern: Functional Core
package environment

import (
	"strings"
	"testing"
)

func TestWindowsNodeToolchainDigestBindsNodeAndNPMBytes(t *testing.T) {
	first, err := WindowsNodeToolchainSHA256([]byte("trusted node"), []byte("trusted npm cli"))
	if err != nil || !ValidWindowsNodeToolchainSHA256(first) {
		t.Fatalf("toolchain digest=%q err=%v", first, err)
	}
	replay, err := WindowsNodeToolchainSHA256([]byte("trusted node"), []byte("trusted npm cli"))
	if err != nil || replay != first {
		t.Fatalf("same toolchain produced digest=%q err=%v, want %q", replay, err, first)
	}
	changedNode, err := WindowsNodeToolchainSHA256([]byte("changed node"), []byte("trusted npm cli"))
	if err != nil || changedNode == first {
		t.Fatalf("changed Node bytes retained digest %q (original %q), err=%v", changedNode, first, err)
	}
	changedNPM, err := WindowsNodeToolchainSHA256([]byte("trusted node"), []byte("changed npm cli"))
	if err != nil || changedNPM == first {
		t.Fatalf("changed npm bytes retained digest %q (original %q), err=%v", changedNPM, first, err)
	}
}

func TestWindowsNodeToolchainDigestRejectsMissingOrNonCanonicalInputs(t *testing.T) {
	if _, err := WindowsNodeToolchainSHA256(nil, []byte("npm")); err == nil {
		t.Fatal("missing Node executable bytes were accepted")
	}
	if _, err := WindowsNodeToolchainSHA256FromFileDigests(strings.Repeat("A", 64), strings.Repeat("0", 64)); err == nil {
		t.Fatal("malformed per-file hashes were accepted")
	}
}

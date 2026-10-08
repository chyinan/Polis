// pattern: Functional Core
package gotestprofile

import (
	"bytes"
	"strings"
	"testing"
)

func TestFixtureProfileCanonicalRoundTripAndDigest(t *testing.T) {
	profile := FixtureManifest(strings.Repeat("a", 64))
	canonical, digest, err := Canonicalize(profile)
	if err != nil {
		t.Fatalf("Canonicalize() error = %v", err)
	}
	parsed, parsedDigest, err := Parse(canonical)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	canonicalAgain, digestAgain, err := Canonicalize(parsed)
	if err != nil {
		t.Fatalf("Canonicalize(parsed) error = %v", err)
	}
	if digest != parsedDigest || digest != digestAgain || !bytes.Equal(canonical, canonicalAgain) {
		t.Fatalf("profile canonical round trip changed bytes or digest: %s %s %s", digest, parsedDigest, digestAgain)
	}
	if len(profile.Command) == 0 || profile.Command[0] != "/go/bin/go" || profile.NetworkPolicy != "deny_all" || profile.Limits.SwapBytes != 0 {
		t.Fatalf("fixture profile omitted fixed tool or isolation contract: %+v", profile)
	}
	if !containsString(profile.Environment, "GOFLAGS=") || !containsString(profile.Environment, "GOTOOLCHAIN=local") || !containsString(profile.Environment, "GOPROXY=off") {
		t.Fatalf("profile does not pin offline Go environment: %+v", profile.Environment)
	}
}

func TestFixtureProfileDefines4096ScratchEntriesAsInodes(t *testing.T) {
	canonical, _, err := Canonicalize(FixtureManifest(strings.Repeat("a", 64)))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(canonical), `"scratchInodes":4096`) || strings.Contains(string(canonical), `"scratchFiles"`) {
		t.Fatalf("4096 scratch limit must be encoded as a filesystem inode cap: %s", canonical)
	}
}

func TestFixtureProfileRejectsCommandScopeAndResourceLimitChanges(t *testing.T) {
	base := FixtureManifest(strings.Repeat("a", 64))
	tests := []struct {
		name   string
		mutate func(*Manifest)
	}{
		{name: "shell command", mutate: func(value *Manifest) { value.Command[0] = "/bin/sh" }},
		{name: "package scope", mutate: func(value *Manifest) { value.PackageScope = []string{"./..."} }},
		{name: "network enabled", mutate: func(value *Manifest) { value.NetworkPolicy = "allow" }},
		{name: "memory raised", mutate: func(value *Manifest) { value.Limits.MemoryBytes++ }},
		{name: "swap enabled", mutate: func(value *Manifest) { value.Limits.SwapBytes = 1 }},
		{name: "scratch capacity raised", mutate: func(value *Manifest) { value.Limits.ScratchBytes++ }},
		{name: "scratch inode cap raised", mutate: func(value *Manifest) { value.Limits.ScratchInodes++ }},
		{name: "output limit raised", mutate: func(value *Manifest) { value.Limits.StdoutBytes++ }},
		{name: "toolchain digest invalid", mutate: func(value *Manifest) { value.ToolchainSHA256 = "unverified" }},
		{name: "toolchain digest substituted", mutate: func(value *Manifest) { value.ToolchainSHA256 = strings.Repeat("c", 64) }},
		{name: "ambient flags injected", mutate: func(value *Manifest) { value.Environment[7] = "GOFLAGS=-mod=mod" }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			candidate := cloneManifest(base)
			test.mutate(&candidate)
			if _, _, err := Canonicalize(candidate); err == nil {
				t.Fatal("invalid fixed Go profile was accepted")
			}
		})
	}
}

func cloneManifest(value Manifest) Manifest {
	value.PackageScope = append([]string(nil), value.PackageScope...)
	value.Command = append([]string(nil), value.Command...)
	value.Environment = append([]string(nil), value.Environment...)
	value.EditablePaths = append([]string(nil), value.EditablePaths...)
	value.ProtectedPaths = append([]string(nil), value.ProtectedPaths...)
	return value
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

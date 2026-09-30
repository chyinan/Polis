// pattern: Functional Core
package probe

import "testing"

func TestValidateT14CNamespaceBindingsAcceptsHostSourcesAndGuestTargets(t *testing.T) {
	facts := []T14CBindFact{
		{HostPath: "/mnt/d/Programs/Polis/codex", GuestPath: "/codex", ExpectedType: T14CRegularFile, Exists: true, Readable: true},
		{HostPath: "/mnt/d/Programs/Polis/home", GuestPath: "/home/codex", ExpectedType: T14CDirectory, Exists: true, Readable: true},
	}
	if err := ValidateT14CNamespaceBindings(facts, "/mnt/d/Programs/Polis/home", "/home/codex"); err != nil {
		t.Fatalf("valid host/guest bindings rejected: %v", err)
	}
}

func TestValidateT14CNamespaceBindingsRejectsGuestPathAsHostSource(t *testing.T) {
	facts := []T14CBindFact{{HostPath: "/home/codex", GuestPath: "/home/codex", ExpectedType: T14CDirectory, Exists: true, Readable: true}}
	if err := ValidateT14CNamespaceBindings(facts, "/home/codex", "/home/codex"); err == nil {
		t.Fatal("guest-only path was accepted as a host bind source")
	}
}

func TestValidateT14CNamespaceBindingsRejectsMissingOrInvalidGuestTarget(t *testing.T) {
	facts := []T14CBindFact{{HostPath: "/mnt/d/Programs/Polis/codex", GuestPath: "codex", ExpectedType: T14CRegularFile, Exists: false, Readable: true}}
	if err := ValidateT14CNamespaceBindings(facts, "/mnt/d/Programs/Polis/home", "/home/codex"); err == nil {
		t.Fatal("invalid binding facts were accepted")
	}
}

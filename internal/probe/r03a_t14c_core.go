// pattern: Functional Core
package probe

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
)

const (
	T14CRegularFile = "regular_file"
	T14CDirectory   = "directory"
	t14CGuestHome   = "/home/codex"
)

// T14CBindFact is the immutable, namespace-labelled result of the imperative
// host filesystem preflight. ActualType and SourceNamespace are populated by
// the shell; keeping them in the fact makes the pure validator reject a
// mismatched or stale launch binding before bwrap can start.
type T14CBindFact struct {
	HostPath        string `json:"host_path"`
	GuestPath       string `json:"guest_path"`
	ExpectedType    string `json:"expected_type"`
	ActualType      string `json:"actual_type,omitempty"`
	SourceNamespace string `json:"source_namespace"`
	Exists          bool   `json:"exists"`
	Readable        bool   `json:"readable"`
}

func ValidateT14CNamespaceBindings(facts []T14CBindFact, hostHomePath, guestHomePath string) error {
	if hostHomePath == "" || guestHomePath != t14CGuestHome {
		return errors.New("preflight_failed: invalid T14C host/guest home declaration")
	}
	if filepath.Clean(hostHomePath) == filepath.Clean(guestHomePath) {
		return errors.New("preflight_failed: guest HOME was used as a host bind source")
	}
	if len(facts) == 0 {
		return errors.New("preflight_failed: no T14C bind sources were validated")
	}
	seenTargets := make(map[string]struct{}, len(facts))
	for _, fact := range facts {
		if fact.HostPath == "" || fact.GuestPath == "" || !filepath.IsAbs(fact.GuestPath) {
			return fmt.Errorf("preflight_failed: bind target is not an absolute guest path: %q", fact.GuestPath)
		}
		if fact.GuestPath == guestHomePath && filepath.Clean(fact.HostPath) == filepath.Clean(guestHomePath) {
			return errors.New("preflight_failed: /home/codex is a guest target, not a host source")
		}
		if fact.SourceNamespace != "" && fact.SourceNamespace != "host" {
			return fmt.Errorf("preflight_failed: bind source namespace is not host: %s", fact.SourceNamespace)
		}
		if !fact.Exists || !fact.Readable {
			return fmt.Errorf("preflight_failed: bind source is missing or unreadable: %s", fact.HostPath)
		}
		if fact.ExpectedType != T14CRegularFile && fact.ExpectedType != T14CDirectory {
			return fmt.Errorf("preflight_failed: unsupported bind source type %q", fact.ExpectedType)
		}
		if fact.ActualType != "" && fact.ActualType != fact.ExpectedType {
			return fmt.Errorf("preflight_failed: bind source type mismatch for %s: expected=%s actual=%s", fact.HostPath, fact.ExpectedType, fact.ActualType)
		}
		if _, exists := seenTargets[fact.GuestPath]; exists {
			return fmt.Errorf("preflight_failed: duplicate guest bind target: %s", fact.GuestPath)
		}
		seenTargets[fact.GuestPath] = struct{}{}
	}
	return nil
}

func t14CGuestTargetsAreAbsolute(targets []string) error {
	for _, target := range targets {
		if !strings.HasPrefix(target, "/") || filepath.Clean(target) != target {
			return fmt.Errorf("preflight_failed: invalid guest bind target: %q", target)
		}
	}
	return nil
}

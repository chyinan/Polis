//go:build !linux

// pattern: Imperative Shell
package gotestprofile

import "fmt"

// The fixed Go fixture is not executed or materialized on unsupported hosts.
// Windows remains a supported build target for Polis itself; this profile is
// explicitly a qualified Linux/WSL-only execution target.
func MaterializeFixture(string, []SnapshotFile) (MaterializedFixture, error) {
	return MaterializedFixture{}, fmt.Errorf("%w: fixed Go fixture requires Linux openat semantics", ErrMaterializeFailed)
}

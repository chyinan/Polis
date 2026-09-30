//go:build windows

// pattern: Functional Core
package runner

import "testing"

func TestPrepareWindowsBoundedWorkspaceRejectsMissingBinding(t *testing.T) {
	if _, err := prepareWindowsBoundedAppContainerWorkspace(AppContainerWorkspaceStorageBinding{}, "PolisJob-test", nil); err == nil {
		t.Fatal("missing bounded-volume binding was accepted")
	}
}

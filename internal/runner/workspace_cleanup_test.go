// pattern: Functional Core
package runner

import (
	"errors"
	"strings"
	"testing"
)

func TestRetryWorkspaceDirectoryCleanupRetriesTransientFailure(t *testing.T) {
	attempts := 0
	err := retryWorkspaceDirectoryCleanup(`W:\PolisWorkspace\PolisJob-test`, func(string) error {
		attempts++
		if attempts == 1 {
			return errors.New("temporary sharing violation")
		}
		return nil
	})
	if err != nil || attempts != 2 {
		t.Fatalf("cleanup err=%v attempts=%d, want success after retry", err, attempts)
	}
}

func TestRetryWorkspaceDirectoryCleanupReportsResidualPath(t *testing.T) {
	path := `W:\PolisWorkspace\PolisJob-test`
	err := retryWorkspaceDirectoryCleanup(path, func(string) error { return errors.New("sharing violation") })
	if err == nil || !strings.Contains(err.Error(), path) {
		t.Fatalf("persistent cleanup error=%v, want residual path %q", err, path)
	}
}

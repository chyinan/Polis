// pattern: Imperative Shell
package runner

import (
	"errors"
	"fmt"
	"os"
	"time"
)

const workspaceDirectoryCleanupAttempts = 3

func retryWorkspaceDirectoryCleanup(path string, remove func(string) error) error {
	if path == "" || remove == nil {
		return errors.New("bounded workspace cleanup input is invalid")
	}
	var lastErr error
	for attempt := 1; attempt <= workspaceDirectoryCleanupAttempts; attempt++ {
		lastErr = remove(path)
		if lastErr == nil || errors.Is(lastErr, os.ErrNotExist) {
			return nil
		}
		if attempt < workspaceDirectoryCleanupAttempts {
			time.Sleep(10 * time.Millisecond)
		}
	}
	return fmt.Errorf("bounded workspace cleanup remains pending at %s after %d attempts: %w", path, workspaceDirectoryCleanupAttempts, lastErr)
}

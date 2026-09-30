//go:build !windows

// pattern: Imperative Shell
package runner

import (
	"context"
	"errors"
)

func ReconcileWindowsProcessTree(context.Context, string) (WindowsProcessTreeStopProof, error) {
	return WindowsProcessTreeStopProof{}, errors.New("Windows Job Object reconciliation is unavailable on this host")
}

func ReconcileWindowsWorkerProcessTree(context.Context, string, int) (WindowsProcessTreeStopProof, error) {
	return WindowsProcessTreeStopProof{}, errors.New("Windows Worker Job Object reconciliation is unavailable on this host")
}

//go:build !linux

// pattern: Imperative Shell

package runner

import (
	"context"
	"errors"
)

func ReconcileLinuxWorkerProcessGroup(context.Context, string, int) (LinuxProcessGroupStopProof, error) {
	return LinuxProcessGroupStopProof{}, errors.New("Linux procfs process-group reconciliation is unavailable on this host")
}

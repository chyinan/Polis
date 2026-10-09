//go:build !linux

// pattern: Imperative Shell
package runner

import (
	"context"
	"errors"
	"polis/internal/gotestprofile"
)

type FixedGoResult struct {
	Facts                                 gotestprofile.ExecutionFacts
	Started, LaunchObserved, ExitObserved bool
	StartError                            error
}

func CheckFixedGoControls(string, string, gotestprofile.ResourceLimits) error {
	return errors.New("linux_host_required")
}
func RunFixedGo(context.Context, string, string, string, string, string, string, gotestprofile.Manifest) FixedGoResult {
	return FixedGoResult{StartError: errors.New("linux_host_required")}
}

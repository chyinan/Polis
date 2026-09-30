// pattern: Functional Core
package environment

import (
	"context"
	"errors"
	"io"
	"testing"

	"polis/internal/runner"
)

type processWithExitState struct{ exited bool }

func (*processWithExitState) PID() int                          { return 7 }
func (*processWithExitState) Stdin() io.WriteCloser             { return nil }
func (*processWithExitState) Stdout() io.ReadCloser             { return nil }
func (*processWithExitState) Stderr() io.ReadCloser             { return nil }
func (*processWithExitState) Wait(context.Context) (int, error) { return 0, nil }
func (*processWithExitState) Stop() (runner.StopProof, error)   { return runner.StopProof{}, nil }
func (process *processWithExitState) HasExited() bool           { return process.exited }

type processWithTreeCleanupWait struct {
	processWithExitState
	waited bool
	err    error
}

func (process *processWithTreeCleanupWait) WaitForTreeCleanup(context.Context) error {
	process.waited = true
	return process.err
}

type processWithoutExitState struct{}

func (*processWithoutExitState) PID() int                          { return 7 }
func (*processWithoutExitState) Stdin() io.WriteCloser             { return nil }
func (*processWithoutExitState) Stdout() io.ReadCloser             { return nil }
func (*processWithoutExitState) Stderr() io.ReadCloser             { return nil }
func (*processWithoutExitState) Wait(context.Context) (int, error) { return 0, nil }
func (*processWithoutExitState) Stop() (runner.StopProof, error)   { return runner.StopProof{}, nil }

type processWithCleanupFailure struct {
	processWithExitState
	err error
}

func (process *processWithCleanupFailure) Wait(context.Context) (int, error) {
	return 1, process.err
}

func TestTrackedEnvironmentProcessForwardsExitStateAndFailsClosedWithoutIt(t *testing.T) {
	inner := &processWithExitState{}
	tracked := &trackedEnvironmentProcess{AppContainerProcess: inner}
	if tracked.HasExited() {
		t.Fatal("live underlying process was projected exited")
	}
	inner.exited = true
	if !tracked.HasExited() {
		t.Fatal("underlying process exit state was hidden by the wrapper")
	}
	if !(&trackedEnvironmentProcess{AppContainerProcess: &processWithoutExitState{}}).HasExited() {
		t.Fatal("wrapper without an exit-state capability did not fail closed")
	}
}

func TestTrackedEnvironmentProcessWaitsForUnderlyingTreeCleanup(t *testing.T) {
	inner := &processWithTreeCleanupWait{}
	tracked := &trackedEnvironmentProcess{AppContainerProcess: inner}
	if err := tracked.WaitForTreeCleanup(context.Background()); err != nil || !inner.waited {
		t.Fatalf("tree cleanup wait err=%v waited=%t", err, inner.waited)
	}
	if err := (&trackedEnvironmentProcess{AppContainerProcess: &processWithExitState{}}).WaitForTreeCleanup(context.Background()); err == nil {
		t.Fatal("wrapper without a tree-cleanup contract did not fail closed")
	}
}

func TestTrackedEnvironmentProcessRetainsSnapshotAfterUnconfirmedTreeCleanup(t *testing.T) {
	cleanupErr := errors.New("process tree remains active")
	inner := &processWithCleanupFailure{err: cleanupErr}
	released := false
	tracked := &trackedEnvironmentProcess{AppContainerProcess: inner, onExit: func() { released = true }}
	if _, err := tracked.Wait(context.Background()); !errors.Is(err, cleanupErr) {
		t.Fatalf("cleanup failure was lost from Wait: %v", err)
	}
	if released {
		t.Fatal("snapshot ownership was released while process-tree cleanup was unconfirmed")
	}
}

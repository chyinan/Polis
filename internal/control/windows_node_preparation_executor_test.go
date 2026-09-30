// pattern: Functional Core
package control

import (
	"context"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"polis/internal/environment"
	"polis/internal/kernel"
	"polis/internal/runner"
)

func TestRedactRegistryProxyCredentialsFromPreparationLogs(t *testing.T) {
	input := []byte("npm ERR! request to http://lease-user:lease-password@127.0.0.1:43123 failed")
	got := string(redactRegistryProxyCredentials(input))
	if strings.Contains(got, "lease-user") || strings.Contains(got, "lease-password") {
		t.Fatalf("registry proxy credentials remained in logs: %q", got)
	}
	if !strings.Contains(got, "http://[redacted]@127.0.0.1:43123") {
		t.Fatalf("redacted proxy endpoint was not retained for diagnosis: %q", got)
	}
}

func TestWindowsPreparationExecutorRetainsSnapshotUntilShutdown(t *testing.T) {
	executor := &WindowsNodeNPMPreparationExecutor{}
	first := &environment.WindowsNodeAppContainerSnapshot{}
	second := &environment.WindowsNodeAppContainerSnapshot{}
	owner := retainedEnvironmentSnapshot{companyID: "company-1", runID: "run-1", snapshot: first}
	if err := executor.storePreparedSnapshot("environment-revision-1", owner); err != nil {
		t.Fatal(err)
	}
	if err := executor.storePreparedSnapshot("environment-revision-1", retainedEnvironmentSnapshot{companyID: "company-1", runID: "run-2", snapshot: second}); err == nil {
		t.Fatal("a second snapshot replaced the prepared environment for the same immutable revision")
	}
	retained, ok := executor.preparedSnapshot("environment-revision-1")
	if !ok || retained != first {
		t.Fatal("prepared environment snapshot was not retained by its immutable revision")
	}
	if err := executor.Close(); err != nil {
		t.Fatalf("close retained environment snapshots: %v", err)
	}
	if _, ok = executor.preparedSnapshot("environment-revision-1"); ok {
		t.Fatal("executor shutdown retained a closed environment snapshot")
	}
	if err := executor.storePreparedSnapshot("environment-revision-2", retainedEnvironmentSnapshot{companyID: "company-1", runID: "run-3", snapshot: &environment.WindowsNodeAppContainerSnapshot{}}); err == nil {
		t.Fatal("executor accepted a new snapshot after shutdown began")
	}
}

func TestWindowsPreparationExecutorTracksPendingSnapshotForLaterCleanup(t *testing.T) {
	executor := &WindowsNodeNPMPreparationExecutor{}
	snapshot := &environment.WindowsNodeAppContainerSnapshot{}
	if err := executor.storePendingCleanup(retainedEnvironmentSnapshot{companyID: "company-1", runID: "run-1", snapshot: snapshot}); err != nil {
		t.Fatal(err)
	}
	if len(executor.pendingCleanup) != 1 {
		t.Fatalf("pending cleanup count=%d, want 1", len(executor.pendingCleanup))
	}
	if err := executor.Close(); err != nil {
		t.Fatalf("close pending snapshot: %v", err)
	}
	if len(executor.pendingCleanup) != 0 {
		t.Fatal("executor shutdown did not release the pending cleanup reference")
	}
}

func TestWindowsPreparationExecutorCloseWaitsForActivePreparation(t *testing.T) {
	executor := &WindowsNodeNPMPreparationExecutor{}
	releaseSlot, finish, ok, _ := executor.beginPreparation(context.Background())
	if !ok {
		t.Fatal("open executor refused a preparation")
	}
	closed := make(chan error, 1)
	go func() { closed <- executor.Close() }()
	select {
	case err := <-closed:
		t.Fatalf("executor close returned before active preparation finished: %v", err)
	case <-time.After(20 * time.Millisecond):
	}
	releaseSlot()
	finish()
	select {
	case err := <-closed:
		if err != nil {
			t.Fatalf("close executor after preparation drain: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("executor close did not resume after preparation finished")
	}
	if _, _, accepted, _ := executor.beginPreparation(context.Background()); accepted {
		t.Fatal("closed executor accepted a new preparation")
	}
}

func TestWindowsPreparationExecutorBoundsConcurrentPreparations(t *testing.T) {
	executor := &WindowsNodeNPMPreparationExecutor{}
	firstReleaseSlot, firstFinish, ok, _ := executor.beginPreparation(context.Background())
	if !ok {
		t.Fatal("first preparation was rejected")
	}
	secondReleaseSlot, secondFinish, ok, _ := executor.beginPreparation(context.Background())
	if !ok {
		t.Fatal("second preparation was rejected before the configured limit")
	}
	ctx, cancel := context.WithCancel(context.Background())
	thirdResult := make(chan bool, 1)
	go func() {
		_, _, acquired, _ := executor.beginPreparation(ctx)
		thirdResult <- acquired
	}()
	cancel()
	select {
	case acquired := <-thirdResult:
		if acquired {
			t.Fatal("executor exceeded its concurrent preparation bound")
		}
	case <-time.After(time.Second):
		t.Fatal("cancelled queued preparation did not leave the wait queue")
	}
	firstReleaseSlot()
	firstFinish()
	secondReleaseSlot()
	secondFinish()
}

func TestWindowsPreparationExecutorRejectsFullBoundedQueue(t *testing.T) {
	executor := &WindowsNodeNPMPreparationExecutor{preparationQueue: make(chan struct{}, maxQueuedWindowsNodePreparations)}
	for range maxQueuedWindowsNodePreparations {
		executor.preparationQueue <- struct{}{}
	}
	loaderCalled := false
	result, err := executor.PrepareProjectEnvironment(context.Background(), "run-1", func(context.Context) (kernel.ProjectEnvironmentExecutionSnapshot, error) {
		loaderCalled = true
		return kernel.ProjectEnvironmentExecutionSnapshot{}, nil
	})
	if !errors.Is(err, errEnvironmentPreparationQueueFull) || result.ReasonCode != "environment_executor_busy" {
		t.Fatalf("full preparation queue result=%+v error=%v", result, err)
	}
	if loaderCalled {
		t.Fatal("full preparation queue loaded and retained the execution snapshot")
	}
}

func TestWindowsPreparationExecutorLoadsSnapshotAfterSlotAdmission(t *testing.T) {
	executor := &WindowsNodeNPMPreparationExecutor{}
	loadErr := errors.New("offline snapshot load fixture")
	loaderCalled := false
	result, err := executor.PrepareProjectEnvironment(context.Background(), "run-1", func(context.Context) (kernel.ProjectEnvironmentExecutionSnapshot, error) {
		loaderCalled = true
		return kernel.ProjectEnvironmentExecutionSnapshot{}, loadErr
	})
	if !loaderCalled || !errors.Is(err, loadErr) || result.ReasonCode != "environment_snapshot_unavailable" {
		t.Fatalf("admitted preparation did not load its snapshot: called=%v result=%+v error=%v", loaderCalled, result, err)
	}
}

func TestPreparationStreamCaptureAbortUnblocksPipeReader(t *testing.T) {
	reader, writer := io.Pipe()
	capture := capturePreparationStream(reader, 16)
	done := make(chan preparationStreamResult, 1)
	go func() { done <- capture.wait() }()
	capture.abort()
	select {
	case result := <-done:
		if result.err == nil {
			t.Fatal("aborted pipe capture did not report the closed reader")
		}
	case <-time.After(time.Second):
		t.Fatal("aborting a preparation log capture left its reader blocked")
	}
	_ = writer.Close()
}

func TestPendingCleanupMonitorsProcessUntilSnapshotCanClose(t *testing.T) {
	executor := &WindowsNodeNPMPreparationExecutor{}
	firstSlotRelease, firstFinish, acquired, _ := executor.beginPreparation(context.Background())
	if !acquired {
		t.Fatal("pending preparation failed to acquire a slot")
	}
	process := &deferredTestAppContainerProcess{started: make(chan struct{}), exit: make(chan struct{})}
	owner := retainedEnvironmentSnapshot{companyID: "company-1", runID: "run-1", snapshot: &environment.WindowsNodeAppContainerSnapshot{}, process: process, releasePreparationSlot: firstSlotRelease}
	if err := executor.storePendingCleanup(owner); err != nil {
		t.Fatal(err)
	}
	done := executor.monitorPendingProcess(owner)
	firstFinish()
	select {
	case <-process.started:
	case <-time.After(time.Second):
		t.Fatal("pending cleanup did not wait on the process handle")
	}
	secondSlotRelease, secondFinish, acquired, _ := executor.beginPreparation(context.Background())
	if !acquired {
		t.Fatal("second concurrent preparation slot was unavailable")
	}
	queuedCtx, cancelQueued := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancelQueued()
	if _, _, acquired, _ = executor.beginPreparation(queuedCtx); acquired {
		t.Fatal("pending process released its preparation slot before cleanup completed")
	}
	close(process.exit)
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("pending cleanup did not retry after the process exited")
	}
	if len(executor.pendingCleanup) != 0 {
		t.Fatal("pending snapshot remained after its process exited and cleanup succeeded")
	}
	thirdSlotRelease, thirdFinish, acquired, _ := executor.beginPreparation(context.Background())
	if !acquired {
		t.Fatal("pending process slot was not released after cleanup completed")
	}
	thirdSlotRelease()
	thirdFinish()
	secondSlotRelease()
	secondFinish()
}

type deferredTestAppContainerProcess struct {
	started     chan struct{}
	exit        chan struct{}
	startedOnce sync.Once
}

func (process *deferredTestAppContainerProcess) PID() int              { return 42 }
func (process *deferredTestAppContainerProcess) Stdin() io.WriteCloser { return nil }
func (process *deferredTestAppContainerProcess) Stdout() io.ReadCloser { return nil }
func (process *deferredTestAppContainerProcess) Stderr() io.ReadCloser { return nil }
func (process *deferredTestAppContainerProcess) Wait(ctx context.Context) (int, error) {
	process.startedOnce.Do(func() { close(process.started) })
	select {
	case <-process.exit:
		return 0, nil
	case <-ctx.Done():
		return 0, ctx.Err()
	}
}
func (*deferredTestAppContainerProcess) Stop() (runner.StopProof, error) {
	return runner.StopProof{}, errors.New("stop is not confirmed")
}

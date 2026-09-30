// pattern: Functional Core
package runner

import (
	"context"
	"errors"
	"io"
	"testing"
)

func TestServiceListenerProcessClosesNetworkLeaseAfterConfirmedExit(t *testing.T) {
	process := &serviceListenerTestProcess{id: "service-job-1", pid: 91, exitCode: 0}
	closeCalls := 0
	leased := newServiceListenerLeaseProcess(process, "service-job-1", func() error {
		closeCalls++
		return nil
	})
	code, err := leased.Wait(context.Background())
	if err != nil || code != 0 || closeCalls != 1 {
		t.Fatalf("Wait code=%d error=%v leaseCloseCalls=%d", code, err, closeCalls)
	}
}

func TestServiceListenerProcessKeepsNetworkLeaseUntilStopProof(t *testing.T) {
	process := &serviceListenerTestProcess{id: "service-job-1", pid: 92, waitErr: context.DeadlineExceeded}
	closeCalls := 0
	leased := newServiceListenerLeaseProcess(process, "service-job-1", func() error {
		closeCalls++
		return nil
	})
	if _, err := leased.Wait(context.Background()); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Wait error=%v, want unconfirmed timeout", err)
	}
	if closeCalls != 0 {
		t.Fatalf("network lease closed before a stop proof: calls=%d", closeCalls)
	}
	process.stopProof = StopProof{id: process.id, pid: process.pid, stopped: true}
	if _, err := leased.Stop(); err != nil {
		t.Fatal(err)
	}
	if closeCalls != 1 {
		t.Fatalf("network lease close calls=%d, want one after proof", closeCalls)
	}
}

func TestServiceListenerProcessConfirmsTreeAfterCallerCancellationBeforeClosingLease(t *testing.T) {
	process := &serviceListenerTestProcess{id: "service-job-1", pid: 95, waitErr: context.Canceled, cleanupErrSet: true}
	closeCalls := 0
	leased := newServiceListenerLeaseProcess(process, process.id, func() error {
		closeCalls++
		return nil
	})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := leased.Wait(ctx)
	if !errors.Is(err, context.Canceled) || errors.Is(err, ErrAppContainerProcessStopUnconfirmed) {
		t.Fatalf("cancelled Wait error=%v, want cancellation with confirmed cleanup", err)
	}
	stopped, ok := leased.(interface{ ProcessTreeStopped() bool })
	if closeCalls != 1 || !ok || !stopped.ProcessTreeStopped() {
		t.Fatalf("cancelled Wait cleanup closeCalls=%d treeStopped=%v", closeCalls, ok && stopped.ProcessTreeStopped())
	}
}

func TestServiceListenerProcessRetainsLeaseWhenCleanupFailsAndRetries(t *testing.T) {
	process := &serviceListenerTestProcess{id: "service-job-1", pid: 93, stopProof: StopProof{id: "service-job-1", pid: 93, stopped: true}}
	closeCalls := 0
	cleanupFailure := errors.New("WFP dynamic session remains open")
	leased := newServiceListenerLeaseProcess(process, process.id, func() error {
		closeCalls++
		if closeCalls == 1 {
			return cleanupFailure
		}
		return nil
	})
	if _, err := leased.Stop(); !errors.Is(err, cleanupFailure) {
		t.Fatalf("first lease close error=%v", err)
	}
	if _, err := leased.Stop(); err != nil || closeCalls != 2 {
		t.Fatalf("retry Stop error=%v closeCalls=%d", err, closeCalls)
	}
}

func TestServiceListenerProcessRejectsWrongStopProofWithoutClosingLease(t *testing.T) {
	process := &serviceListenerTestProcess{id: "service-job-1", pid: 94, stopProof: StopProof{id: "other-job", pid: 94, stopped: true}}
	closeCalls := 0
	leased := newServiceListenerLeaseProcess(process, process.id, func() error {
		closeCalls++
		return nil
	})
	if _, err := leased.Stop(); !errors.Is(err, ErrAppContainerProcessStopUnconfirmed) || closeCalls != 0 {
		t.Fatalf("wrong stop proof error=%v closeCalls=%d", err, closeCalls)
	}
}

type serviceListenerTestProcess struct {
	id            string
	pid           int
	exitCode      int
	waitErr       error
	cleanupErr    error
	cleanupErrSet bool
	stopProof     StopProof
}

func (process *serviceListenerTestProcess) PID() int      { return process.pid }
func (*serviceListenerTestProcess) Stdin() io.WriteCloser { return nil }
func (*serviceListenerTestProcess) Stdout() io.ReadCloser { return nil }
func (*serviceListenerTestProcess) Stderr() io.ReadCloser { return nil }
func (process *serviceListenerTestProcess) Wait(context.Context) (int, error) {
	return process.exitCode, process.waitErr
}
func (process *serviceListenerTestProcess) Stop() (StopProof, error) { return process.stopProof, nil }
func (process *serviceListenerTestProcess) WaitForTreeCleanup(context.Context) error {
	if process.cleanupErrSet {
		return process.cleanupErr
	}
	return process.waitErr
}
func (*serviceListenerTestProcess) HasExited() bool { return true }
func (process *serviceListenerTestProcess) ConfirmTreeStopped(proof WindowsProcessTreeStopProof) error {
	if !proof.For(process.id) || proof.pid != process.pid {
		return ErrAppContainerProcessStopUnconfirmed
	}
	process.stopProof = StopProof{id: process.id, pid: process.pid, stopped: true}
	return nil
}

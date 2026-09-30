// pattern: Imperative Shell
package runner

import (
	"context"
	"errors"
	"sync"
	"time"
)

type serviceListenerLeaseProcess struct {
	AppContainerProcess
	identity       string
	closeLease     func() error
	mu             sync.Mutex
	leaseClosed    bool
	processStopped bool
}

func newServiceListenerLeaseProcess(process AppContainerProcess, identity string, closeLease func() error) AppContainerProcess {
	return &serviceListenerLeaseProcess{AppContainerProcess: process, identity: identity, closeLease: closeLease}
}

func (process *serviceListenerLeaseProcess) Wait(ctx context.Context) (int, error) {
	if process == nil || process.AppContainerProcess == nil || ctx == nil {
		return 0, ErrAppContainerUnavailable
	}
	code, waitErr := process.AppContainerProcess.Wait(ctx)
	if ctx.Err() != nil {
		waitErr = errors.Join(waitErr, ctx.Err())
	}
	if waiter, ok := process.AppContainerProcess.(interface{ WaitForTreeCleanup(context.Context) error }); ok {
		cleanupContext, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if cleanupErr := waiter.WaitForTreeCleanup(cleanupContext); cleanupErr != nil {
			return code, errors.Join(waitErr, ErrAppContainerProcessStopUnconfirmed, cleanupErr)
		}
	} else {
		return code, errors.Join(waitErr, ErrAppContainerProcessStopUnconfirmed)
	}
	process.markProcessStopped()
	return code, errors.Join(waitErr, process.closeConfirmedLease())
}

func (process *serviceListenerLeaseProcess) Stop() (StopProof, error) {
	if process == nil || process.AppContainerProcess == nil || process.identity == "" {
		return StopProof{}, ErrAppContainerProcessStopUnconfirmed
	}
	proof, stopErr := process.AppContainerProcess.Stop()
	if stopErr != nil {
		return proof, errors.Join(stopErr, ErrAppContainerProcessStopUnconfirmed)
	}
	if !proof.For(process.identity) || proof.PID() != process.AppContainerProcess.PID() {
		return proof, ErrAppContainerProcessStopUnconfirmed
	}
	process.markProcessStopped()
	return proof, process.closeConfirmedLease()
}

func (process *serviceListenerLeaseProcess) ConfirmTreeStopped(proof WindowsProcessTreeStopProof) error {
	if process == nil || process.AppContainerProcess == nil || process.identity == "" || !proof.For(process.identity) || proof.pid != process.AppContainerProcess.PID() {
		return ErrAppContainerProcessStopUnconfirmed
	}
	confirmer, ok := process.AppContainerProcess.(interface {
		ConfirmTreeStopped(WindowsProcessTreeStopProof) error
	})
	if !ok {
		return ErrAppContainerProcessStopUnconfirmed
	}
	if err := confirmer.ConfirmTreeStopped(proof); err != nil {
		return errors.Join(ErrAppContainerProcessStopUnconfirmed, err)
	}
	process.markProcessStopped()
	return process.closeConfirmedLease()
}

func (process *serviceListenerLeaseProcess) WaitForTreeCleanup(ctx context.Context) error {
	if process == nil || process.AppContainerProcess == nil || ctx == nil {
		return ErrAppContainerProcessStopUnconfirmed
	}
	waiter, ok := process.AppContainerProcess.(interface{ WaitForTreeCleanup(context.Context) error })
	if !ok {
		return ErrAppContainerProcessStopUnconfirmed
	}
	if err := waiter.WaitForTreeCleanup(ctx); err != nil {
		return errors.Join(ErrAppContainerProcessStopUnconfirmed, err)
	}
	process.markProcessStopped()
	return process.closeConfirmedLease()
}

func (process *serviceListenerLeaseProcess) ProcessTreeStopped() bool {
	if process == nil {
		return false
	}
	process.mu.Lock()
	defer process.mu.Unlock()
	return process.processStopped
}

func (process *serviceListenerLeaseProcess) HasExited() bool {
	if process == nil || process.AppContainerProcess == nil {
		return true
	}
	if state, ok := process.AppContainerProcess.(interface{ HasExited() bool }); ok {
		return state.HasExited()
	}
	return false
}

func (process *serviceListenerLeaseProcess) closeConfirmedLease() error {
	process.mu.Lock()
	defer process.mu.Unlock()
	if process.leaseClosed {
		return nil
	}
	if process.closeLease == nil {
		return errors.New("service listener lease cleanup is unavailable")
	}
	if err := process.closeLease(); err != nil {
		return err
	}
	process.leaseClosed = true
	return nil
}

func (process *serviceListenerLeaseProcess) markProcessStopped() {
	process.mu.Lock()
	process.processStopped = true
	process.mu.Unlock()
}

var _ AppContainerProcess = (*serviceListenerLeaseProcess)(nil)

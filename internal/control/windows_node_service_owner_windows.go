//go:build windows

// pattern: Imperative Shell
package control

import (
	"context"
	"errors"

	"polis/internal/core"
	"polis/internal/environment"
	"polis/internal/runner"
)

func (executor *WindowsNodeNPMPreparationExecutor) VerifyServiceEndpointOwner(ctx context.Context, processID int, bindAddress string, port uint16) error {
	if err := executor.verifyTrackedLiveServiceProcess(ctx, processID); err != nil {
		return err
	}
	if err := runner.VerifyWindowsTCPListenerOwner(ctx, processID, bindAddress, port); err != nil {
		return errors.Join(environment.ErrServiceEndpointOwnerUnverified, err)
	}
	return executor.verifyTrackedLiveServiceProcess(ctx, processID)
}

func (executor *WindowsNodeNPMPreparationExecutor) VerifyServiceEndpointConnectionOwner(ctx context.Context, processID int, bindAddress string, port uint16, clientAddress string, clientPort uint16) error {
	if err := executor.verifyTrackedLiveServiceProcess(ctx, processID); err != nil {
		return err
	}
	if err := runner.VerifyWindowsTCPConnectionOwner(ctx, processID, bindAddress, port, clientAddress, clientPort); err != nil {
		return errors.Join(environment.ErrServiceEndpointOwnerUnverified, err)
	}
	return executor.verifyTrackedLiveServiceProcess(ctx, processID)
}

func (executor *WindowsNodeNPMPreparationExecutor) verifyTrackedLiveServiceProcess(ctx context.Context, processID int) error {
	if executor == nil || ctx == nil || processID <= 0 || ctx.Err() != nil {
		return environment.ErrServiceEndpointOwnerUnverified
	}
	executor.preparedMu.Lock()
	if executor.closed {
		executor.preparedMu.Unlock()
		return environment.ErrServiceEndpointOwnerUnverified
	}
	trackedProcess := false
	for _, job := range executor.jobProcesses {
		if job.process == nil || job.process.PID() != processID {
			continue
		}
		if exitState, ok := job.process.(interface{ HasExited() bool }); ok && exitState.HasExited() {
			executor.preparedMu.Unlock()
			return environment.ErrServiceEndpointOwnerUnverified
		}
		trackedProcess = true
		break
	}
	executor.preparedMu.Unlock()
	if !trackedProcess {
		return errors.Join(environment.ErrServiceEndpointOwnerUnverified, core.OutOfScope)
	}
	if err := ctx.Err(); err != nil {
		return errors.Join(environment.ErrServiceEndpointOwnerUnverified, err)
	}
	return nil
}

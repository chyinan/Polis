// pattern: Imperative Shell
package control

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"

	"polis/internal/environment"
	"polis/internal/kernel"
	"polis/internal/provider"
	"polis/internal/runner"
)

type DeterministicWorkerAdapter struct {
	runtime             *kernel.Kernel
	workerCgroupManager environment.LinuxWorkerCgroupManager
	executable          string
	mu                  sync.Mutex
	workers             map[string]deterministicWorker
}

type deterministicWorker struct {
	binding           kernel.Binding
	process           *runner.Process
	processStopped    bool
	stopProof         runner.StopProof
	hostReconcileOnly bool
}

func NewDeterministicWorkerAdapter(runtime *kernel.Kernel, workerCgroupManagers ...environment.LinuxWorkerCgroupManager) *DeterministicWorkerAdapter {
	executable, err := os.Executable()
	if err != nil {
		executable = ""
	}
	var workerCgroupManager environment.LinuxWorkerCgroupManager
	if len(workerCgroupManagers) > 0 {
		workerCgroupManager = workerCgroupManagers[0]
	}
	return &DeterministicWorkerAdapter{runtime: runtime, workerCgroupManager: workerCgroupManager, executable: executable, workers: make(map[string]deterministicWorker)}
}

func (a *DeterministicWorkerAdapter) Mode() string                    { return "deterministic" }
func (a *DeterministicWorkerAdapter) Readiness(context.Context) error { return nil }
func (a *DeterministicWorkerAdapter) ToolSurface() provider.ToolSurface {
	return provider.ToolSurface{}
}

func (a *DeterministicWorkerAdapter) Start(ctx context.Context, companyID, missionID string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if worker, exists := a.workers[companyID+"/"+missionID]; exists {
		if worker.hostReconcileOnly || worker.processStopped {
			return runner.ErrProcessTreeStopUnconfirmed
		}
		return nil
	}
	if a.executable == "" {
		return errors.New("deterministic worker executable is unavailable")
	}
	scope := a.runtime.LocalScope(companyID)
	task, err := a.runtime.MissionBootstrapTask(ctx, scope, missionID)
	if err != nil {
		return err
	}
	binding, err := a.runtime.TXNewWorker(ctx, scope, task.ID, "deterministic/fake")
	if err != nil {
		return err
	}
	if err = a.runtime.TXBindWorkerProcessHost(ctx, binding, runner.CurrentProcessContainmentMetadata()); err != nil {
		_ = a.runtime.TXFinalizeWorkerBeforeProcess(context.Background(), binding, "deterministic worker process containment could not be bound", "deterministic-containment-failed-"+binding.SessionID())
		return err
	}
	process, err := runner.Start(binding.SessionID(), []string{a.executable, "deterministic-worker"}, nil)
	if err != nil {
		var failure *runner.LaunchFailure
		if errors.As(err, &failure) && failure.ProcessCreated {
			if runner.CurrentProcessContainmentMetadata().HostOS == "windows" {
				if reconcileErr := a.runtime.ReconcileWorkerSession(context.Background(), companyID, binding.SessionID()); reconcileErr == nil {
					return err
				} else {
					a.workers[companyID+"/"+missionID] = deterministicWorker{binding: binding, hostReconcileOnly: true}
					return errors.Join(err, fmt.Errorf("deterministic worker process cleanup is unresolved: %w", reconcileErr))
				}
			}
			a.workers[companyID+"/"+missionID] = deterministicWorker{binding: binding, hostReconcileOnly: true}
			return errors.Join(err, runner.ErrProcessTreeStopUnconfirmed)
		} else if !errors.As(err, &failure) || !failure.ProcessCreated {
			_ = a.runtime.TXFinalizeWorkerBeforeProcess(context.Background(), binding, err.Error(), "deterministic-worker-start-failed-"+binding.SessionID())
		}
		return err
	}
	cleanup := func(startErr error) error {
		cleanupCtx := context.Background()
		worker := deterministicWorker{binding: binding, process: process}
		cleanupErr := a.runtime.TXAttachWorker(cleanupCtx, binding, process)
		if cleanupErr == nil {
			cleanupErr = a.runtime.TXBeginStop(cleanupCtx, binding)
		}
		if cleanupErr == nil {
			worker.stopProof, cleanupErr = process.Stop()
			if cleanupErr == nil {
				worker.processStopped = true
				cleanupErr = a.runtime.TXConfirmStopped(cleanupCtx, binding, worker.stopProof)
			}
		}
		if cleanupErr != nil {
			a.workers[companyID+"/"+missionID] = worker
			return errors.Join(startErr, fmt.Errorf("deterministic worker cleanup is unresolved: %w", cleanupErr))
		}
		return startErr
	}
	if err = a.runtime.TXAttachWorker(ctx, binding, process); err != nil {
		return cleanup(err)
	}
	if err = a.runtime.TXValidateWorker(ctx, binding); err != nil {
		return cleanup(err)
	}
	if err = a.runtime.TXActivateWorker(ctx, binding, "deterministic-worker@1"); err != nil {
		return cleanup(err)
	}
	a.workers[companyID+"/"+missionID] = deterministicWorker{binding: binding, process: process}
	return nil
}

func (a *DeterministicWorkerAdapter) Stop(ctx context.Context, companyID, missionID string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.stopWorkerLocked(ctx, companyID+"/"+missionID, "")
}

func (a *DeterministicWorkerAdapter) StopSession(ctx context.Context, companyID, sessionID string) error {
	a.mu.Lock()
	for key, worker := range a.workers {
		workerCompany, _, ok := splitWorkerKey(key)
		if ok && workerCompany == companyID && worker.binding.SessionID() == sessionID {
			err := a.stopWorkerLocked(ctx, key, sessionID)
			a.mu.Unlock()
			return err
		}
	}
	a.mu.Unlock()
	return a.runtime.ReconcileWorkerSession(ctx, companyID, sessionID, a.workerCgroupManager)
}

func (a *DeterministicWorkerAdapter) stopWorkerLocked(ctx context.Context, key, expectedSessionID string) error {
	worker, exists := a.workers[key]
	if !exists {
		return nil
	}
	if expectedSessionID != "" && worker.binding.SessionID() != expectedSessionID {
		return errors.New("deterministic WorkerSession owner changed during stop")
	}
	if worker.hostReconcileOnly {
		companyID, _, ok := splitWorkerKey(key)
		if !ok {
			return errors.New("deterministic WorkerSession owner key is invalid")
		}
		if err := a.runtime.ReconcileWorkerSession(ctx, companyID, worker.binding.SessionID(), a.workerCgroupManager); err != nil {
			return err
		}
		delete(a.workers, key)
		return nil
	}
	if worker.processStopped {
		if err := a.runtime.TXConfirmStopped(ctx, worker.binding, worker.stopProof); err != nil {
			return err
		}
		delete(a.workers, key)
		return nil
	}
	if err := a.runtime.TXAttachWorker(ctx, worker.binding, worker.process); err != nil {
		return err
	}
	if err := a.runtime.TXBeginStop(ctx, worker.binding); err != nil {
		return err
	}
	proof, err := worker.process.Stop()
	if err != nil {
		return err
	}
	worker.processStopped = true
	worker.stopProof = proof
	a.workers[key] = worker
	if err = a.runtime.TXConfirmStopped(ctx, worker.binding, proof); err != nil {
		return err
	}
	delete(a.workers, key)
	return nil
}

func (a *DeterministicWorkerAdapter) Close() {
	a.mu.Lock()
	keys := make([]string, 0, len(a.workers))
	for key := range a.workers {
		keys = append(keys, key)
	}
	a.mu.Unlock()
	for _, key := range keys {
		companyID, missionID, ok := splitWorkerKey(key)
		if ok {
			_ = a.Stop(context.Background(), companyID, missionID)
		}
	}
}

func splitWorkerKey(key string) (string, string, bool) {
	for index := len(key) - 1; index >= 0; index-- {
		if key[index] == '/' {
			return key[:index], key[index+1:], key[:index] != "" && key[index+1:] != ""
		}
	}
	return "", "", false
}

var _ WorkerAdapter = (*DeterministicWorkerAdapter)(nil)
var _ WorkerSessionStopper = (*DeterministicWorkerAdapter)(nil)

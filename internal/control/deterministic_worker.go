// pattern: Imperative Shell
package control

import (
	"context"
	"errors"
	"os"
	"sync"

	"polis/internal/kernel"
	"polis/internal/provider"
	"polis/internal/runner"
)

type DeterministicWorkerAdapter struct {
	runtime    *kernel.Kernel
	executable string
	mu         sync.Mutex
	workers    map[string]deterministicWorker
}

type deterministicWorker struct {
	binding kernel.Binding
	process *runner.Process
}

func NewDeterministicWorkerAdapter(runtime *kernel.Kernel) *DeterministicWorkerAdapter {
	executable, err := os.Executable()
	if err != nil {
		executable = ""
	}
	return &DeterministicWorkerAdapter{runtime: runtime, executable: executable, workers: make(map[string]deterministicWorker)}
}

func (a *DeterministicWorkerAdapter) Mode() string                    { return "deterministic" }
func (a *DeterministicWorkerAdapter) Readiness(context.Context) error { return nil }
func (a *DeterministicWorkerAdapter) ToolSurface() provider.ToolSurface {
	return provider.ToolSurface{}
}

func (a *DeterministicWorkerAdapter) Start(ctx context.Context, companyID, missionID string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if _, exists := a.workers[companyID+"/"+missionID]; exists {
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
	process, err := runner.Start(binding.SessionID(), []string{a.executable, "deterministic-worker"}, nil)
	if err != nil {
		return err
	}
	cleanup := func() {
		_, _ = process.Stop()
	}
	if err = a.runtime.TXAttachWorker(ctx, binding, process); err != nil {
		cleanup()
		return err
	}
	if err = a.runtime.TXValidateWorker(ctx, binding); err != nil {
		cleanup()
		return err
	}
	if err = a.runtime.TXActivateWorker(ctx, binding, "deterministic-worker@1"); err != nil {
		cleanup()
		return err
	}
	a.workers[companyID+"/"+missionID] = deterministicWorker{binding: binding, process: process}
	return nil
}

func (a *DeterministicWorkerAdapter) Stop(ctx context.Context, companyID, missionID string) error {
	a.mu.Lock()
	worker, exists := a.workers[companyID+"/"+missionID]
	if exists {
		delete(a.workers, companyID+"/"+missionID)
	}
	a.mu.Unlock()
	if !exists {
		return nil
	}
	if err := a.runtime.TXBeginStop(ctx, worker.binding); err != nil {
		return err
	}
	proof, err := worker.process.Stop()
	if err != nil {
		return err
	}
	return a.runtime.TXConfirmStopped(ctx, worker.binding, proof)
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

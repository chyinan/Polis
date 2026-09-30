//go:build windows

// pattern: Imperative Shell
package control

import (
	"context"
	"errors"
	"io"
	"net"
	"os"
	"testing"
	"time"

	"polis/internal/environment"
	"polis/internal/runner"
)

type serviceOwnerTestProcess struct {
	pid         int
	exited      bool
	waitEntered chan struct{}
	cleanupDone chan struct{}
}

func (process *serviceOwnerTestProcess) PID() int      { return process.pid }
func (*serviceOwnerTestProcess) Stdin() io.WriteCloser { return nil }
func (*serviceOwnerTestProcess) Stdout() io.ReadCloser { return nil }
func (*serviceOwnerTestProcess) Stderr() io.ReadCloser { return nil }
func (process *serviceOwnerTestProcess) Wait(ctx context.Context) (int, error) {
	return 0, nil
}
func (process *serviceOwnerTestProcess) WaitForTreeCleanup(ctx context.Context) error {
	if process.cleanupDone == nil {
		return nil
	}
	close(process.waitEntered)
	select {
	case <-process.cleanupDone:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
func (*serviceOwnerTestProcess) Stop() (runner.StopProof, error) { return runner.StopProof{}, nil }
func (process *serviceOwnerTestProcess) HasExited() bool         { return process.exited }

func TestWindowsNodeExecutorRequiresTrackedLiveProcessForServiceEndpoint(t *testing.T) {
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	port := uint16(listener.Addr().(*net.TCPAddr).Port)
	process := &serviceOwnerTestProcess{pid: os.Getpid()}
	executor := &WindowsNodeNPMPreparationExecutor{jobProcesses: map[string]retainedProjectJobProcess{
		"service-job": {companyID: "company-1", process: process},
	}}
	if err = executor.VerifyServiceEndpointOwner(context.Background(), process.pid, "127.0.0.1", port); err != nil {
		t.Fatalf("tracked live process listener was not accepted: %v", err)
	}
	process.exited = true
	if err = executor.VerifyServiceEndpointOwner(context.Background(), process.pid, "127.0.0.1", port); err != environment.ErrServiceEndpointOwnerUnverified {
		t.Fatalf("exited tracked process still owned a service endpoint: %v", err)
	}
}

func TestWindowsNodeExecutorVerifiesTheAcceptedServiceConnectionOwner(t *testing.T) {
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	accepted := make(chan net.Conn, 1)
	go func() {
		connection, acceptErr := listener.Accept()
		if acceptErr == nil {
			accepted <- connection
		}
	}()
	client, err := net.DialTimeout("tcp4", listener.Addr().String(), time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	var server net.Conn
	select {
	case server = <-accepted:
	case <-time.After(time.Second):
		t.Fatal("service test server did not accept the connection")
	}
	defer server.Close()
	local := client.LocalAddr().(*net.TCPAddr)
	remote := client.RemoteAddr().(*net.TCPAddr)
	process := &serviceOwnerTestProcess{pid: os.Getpid()}
	executor := &WindowsNodeNPMPreparationExecutor{jobProcesses: map[string]retainedProjectJobProcess{
		"service-job": {companyID: "company-1", process: process},
	}}
	if err = executor.VerifyServiceEndpointConnectionOwner(context.Background(), process.pid, "127.0.0.1", uint16(remote.Port), local.IP.String(), uint16(local.Port)); err != nil {
		t.Fatalf("tracked process did not own the accepted loopback connection: %v", err)
	}
	if err = executor.VerifyServiceEndpointConnectionOwner(context.Background(), process.pid+1000, "127.0.0.1", uint16(remote.Port), local.IP.String(), uint16(local.Port)); !errors.Is(err, environment.ErrServiceEndpointOwnerUnverified) {
		t.Fatalf("untracked process was accepted as connection owner: %v", err)
	}
}

func TestWindowsNodeExecutorReconcilesOnlyUnrestoredOrExitedJobs(t *testing.T) {
	executor := &WindowsNodeNPMPreparationExecutor{}
	if err := executor.ReconcileUnrestoredProjectJob("company-1", "job-unrestored", "revision-1"); err != nil {
		t.Fatalf("job without a process handle was not reconciled after restart: %v", err)
	}
	process := &serviceOwnerTestProcess{pid: os.Getpid()}
	executor.jobProcesses = map[string]retainedProjectJobProcess{
		"job-live": {companyID: "company-1", revisionID: "revision-1", process: process},
	}
	if err := executor.ReconcileUnrestoredProjectJob("company-1", "job-live", "revision-1"); err == nil {
		t.Fatal("live tracked process was reconciled as exited")
	}
	process.exited = true
	if err := executor.ReconcileUnrestoredProjectJob("company-1", "job-live", "revision-1"); err != nil {
		t.Fatalf("exited tracked process was not reconciled: %v", err)
	}
}

func TestWindowsNodeExecutorWaitsForJobObjectCleanupBeforeReconciliation(t *testing.T) {
	process := &serviceOwnerTestProcess{
		pid:         os.Getpid(),
		exited:      true,
		waitEntered: make(chan struct{}),
		cleanupDone: make(chan struct{}),
	}
	executor := &WindowsNodeNPMPreparationExecutor{jobProcesses: map[string]retainedProjectJobProcess{
		"job-cleanup": {companyID: "company-1", revisionID: "revision-1", process: process},
	}}
	result := make(chan error, 1)
	go func() {
		result <- executor.ReconcileUnrestoredProjectJob("company-1", "job-cleanup", "revision-1")
	}()
	select {
	case <-process.waitEntered:
	case <-time.After(time.Second):
		t.Fatal("reconciliation did not wait for Job Object cleanup")
	}
	select {
	case err := <-result:
		t.Fatalf("reconciliation returned before Job Object cleanup: %v", err)
	default:
	}
	close(process.cleanupDone)
	select {
	case err := <-result:
		if err != nil {
			t.Fatalf("reconciliation failed after Job Object cleanup: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("reconciliation did not finish after Job Object cleanup")
	}
}

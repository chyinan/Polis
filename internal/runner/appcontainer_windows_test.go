// pattern: Imperative Shell
//go:build windows

package runner

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

func TestWindowsAppContainerChildProbe(t *testing.T) {
	if os.Getenv("POLIS_APPCONTAINER_CHILD") != "1" {
		return
	}
	if os.Getenv("POLIS_APPCONTAINER_CHILD_MODE") == "hold" {
		time.Sleep(30 * time.Second)
		return
	}
	workspace := os.Getenv("POLIS_APPCONTAINER_WORKSPACE")
	inside, insideErr := os.ReadFile(filepath.Join(workspace, "inside.txt"))
	_, outsideReadErr := os.ReadFile(os.Getenv("POLIS_APPCONTAINER_OUTSIDE_FILE"))
	outsideWritePath := os.Getenv("POLIS_APPCONTAINER_OUTSIDE_WRITE")
	outsideWriteErr := os.WriteFile(outsideWritePath, []byte("escaped"), 0600)
	readonlyToolchainPath := os.Getenv("POLIS_APPCONTAINER_READONLY_TOOLCHAIN")
	readonlyToolchainDenied := true
	if readonlyToolchainPath != "" {
		readonlyToolchainDenied = os.WriteFile(readonlyToolchainPath, []byte("tampered"), 0600) != nil
	}
	connection, networkErr := net.DialTimeout("tcp", os.Getenv("POLIS_APPCONTAINER_LOOPBACK"), 500*time.Millisecond)
	if connection != nil {
		_ = connection.Close()
	}
	fmt.Printf("inside=%s outside_read_denied=%t outside_write_denied=%t readonly_toolchain_write_denied=%t network_denied=%t\n", strings.TrimSpace(string(inside)), outsideReadErr != nil, outsideWriteErr != nil, readonlyToolchainDenied, networkErr != nil)
	if insideErr != nil || string(inside) != "contained" || outsideReadErr == nil || outsideWriteErr == nil || networkErr == nil || (readonlyToolchainPath != "" && !readonlyToolchainDenied) {
		t.Fatalf("AppContainer boundary failed: insideErr=%v outsideReadErr=%v outsideWriteErr=%v readonlyToolchainDenied=%t networkErr=%v", insideErr, outsideReadErr, outsideWriteErr, readonlyToolchainDenied, networkErr)
	}
}

func TestWindowsAppContainerStopsTimedOutProcessTree(t *testing.T) {
	sandbox, err := NewAppContainerSandbox(fmt.Sprintf("runner-timeout-%d", time.Now().UnixNano()))
	if err != nil {
		t.Fatalf("create no-network AppContainer profile: %v", err)
	}
	defer func() {
		if closeErr := sandbox.Close(); closeErr != nil {
			t.Errorf("delete AppContainer profile: %v", closeErr)
		}
	}()
	workspace := filepath.Join(sandbox.WorkspaceRoot(), "workspace")
	if err = os.Mkdir(workspace, 0700); err != nil {
		t.Fatal(err)
	}
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	childExe := filepath.Join(workspace, "runner-test.exe")
	if err = copyAppContainerTestExecutable(self, childExe); err != nil {
		t.Fatal(err)
	}
	processID := "job-timeout-1"
	process, err := sandbox.Launch(AppContainerLaunchSpec{
		ID: processID, WorkspaceRoot: workspace, Executable: childExe,
		Argv:             []string{childExe, "-test.run=^TestWindowsAppContainerChildProbe$"},
		WorkingDirectory: workspace, NetworkPolicy: AppContainerNetworkDenyAll,
		Environment: append(BuildAppContainerEnvironment(sandbox.WorkspaceRoot(), os.Getenv("SystemRoot")), "POLIS_APPCONTAINER_CHILD=1", "POLIS_APPCONTAINER_CHILD_MODE=hold"),
	})
	if err != nil {
		t.Fatalf("launch AppContainer hold fixture: %v", err)
	}
	defer process.Stop()
	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()
	if _, err = process.Wait(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("AppContainer timeout result error=%v, want deadline exceeded", err)
	}
	proof, err := process.Stop()
	if err != nil || !proof.For(processID) {
		t.Fatalf("AppContainer process tree termination proof=%+v err=%v", proof, err)
	}
}

func TestWindowsAppContainerStopWaitsForTreeCleanupAfterRootExit(t *testing.T) {
	process := &windowsAppContainerProcess{id: "cleanup-race", pid: 123, exited: true, done: make(chan struct{})}
	started := make(chan struct{})
	result := make(chan error, 1)
	go func() {
		close(started)
		proof, err := process.Stop()
		if err == nil && !proof.For(process.id) {
			err = errors.New("Stop returned proof for a different process")
		}
		result <- err
	}()
	<-started
	select {
	case err := <-result:
		t.Fatalf("Stop returned before the process monitor closed the Job Object: %v", err)
	case <-time.After(25 * time.Millisecond):
	}
	close(process.done)
	select {
	case err := <-result:
		if err != nil {
			t.Fatalf("Stop failed after tree cleanup completed: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Stop did not finish after process tree cleanup")
	}
}

func TestWindowsAppContainerDoesNotProveStopWhenTreeCleanupIsUnknown(t *testing.T) {
	process := &windowsAppContainerProcess{
		id: "cleanup-unconfirmed", pid: 123, exited: true, cleanupErr: errors.New("active job members remain"), done: make(chan struct{}),
	}
	close(process.done)
	proof, err := process.Stop()
	if err == nil || proof.For(process.id) {
		t.Fatalf("unconfirmed process-tree cleanup produced proof=%+v err=%v", proof, err)
	}
	if err = process.WaitForTreeCleanup(context.Background()); err == nil {
		t.Fatal("unconfirmed process-tree cleanup was reported complete")
	}
}

func TestWindowsProjectJobObjectAppliesHardResourceLimits(t *testing.T) {
	identity := fmt.Sprintf("resource-limits-%d", time.Now().UnixNano())
	job, err := newWindowsNamedProcessJob(identity)
	if err != nil {
		t.Fatalf("create project Job Object: %v", err)
	}
	defer windows.CloseHandle(job)

	var extended windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION
	if err = windows.QueryInformationJobObject(job, windows.JobObjectExtendedLimitInformation, uintptr(unsafe.Pointer(&extended)), uint32(unsafe.Sizeof(extended)), nil); err != nil {
		t.Fatalf("query project Job Object limits: %v", err)
	}
	resourceLimits := DefaultWindowsJobResourceLimits()
	wantFlags := uint32(windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE |
		windows.JOB_OBJECT_LIMIT_ACTIVE_PROCESS |
		windows.JOB_OBJECT_LIMIT_PROCESS_MEMORY |
		windows.JOB_OBJECT_LIMIT_JOB_MEMORY)
	if extended.BasicLimitInformation.LimitFlags&wantFlags != wantFlags ||
		extended.BasicLimitInformation.ActiveProcessLimit != resourceLimits.MaxActiveProcesses ||
		uint64(extended.ProcessMemoryLimit) != resourceLimits.ProcessMemoryBytes ||
		uint64(extended.JobMemoryLimit) != resourceLimits.JobMemoryBytes {
		t.Fatalf("project Job Object extended limits=%+v, want resource policy %+v", extended, resourceLimits)
	}

	var cpu windowsJobObjectCPURateControlInformation
	if err = windows.QueryInformationJobObject(job, jobObjectCPUControlInformation, uintptr(unsafe.Pointer(&cpu)), uint32(unsafe.Sizeof(cpu)), nil); err != nil {
		t.Fatalf("query project Job Object CPU limit: %v", err)
	}
	wantCPURate, err := resourceLimits.CPURateControlValue()
	if err != nil || cpu.ControlFlags&(jobObjectCPUControlEnable|jobObjectCPUControlHardCap) != jobObjectCPUControlEnable|jobObjectCPUControlHardCap || cpu.CPURate != wantCPURate {
		t.Fatalf("project Job Object CPU limit=%+v err=%v, want hard cap %d", cpu, err, wantCPURate)
	}
}

func TestWindowsGenericJobObjectKeepsProviderWorkerResourcePolicy(t *testing.T) {
	identity := fmt.Sprintf("provider-baseline-%d", time.Now().UnixNano())
	job, err := newWindowsNamedJobObject(appContainerJobObjectName(identity))
	if err != nil {
		t.Fatalf("create generic Provider Worker Job Object: %v", err)
	}
	defer windows.CloseHandle(job)

	var extended windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION
	if err = windows.QueryInformationJobObject(job, windows.JobObjectExtendedLimitInformation, uintptr(unsafe.Pointer(&extended)), uint32(unsafe.Sizeof(extended)), nil); err != nil {
		t.Fatalf("query generic Job Object limits: %v", err)
	}
	if extended.BasicLimitInformation.LimitFlags&windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE == 0 ||
		extended.BasicLimitInformation.ActiveProcessLimit != 64 || extended.ProcessMemoryLimit != 0 || extended.JobMemoryLimit != 0 {
		t.Fatalf("generic Provider Worker Job Object limits changed: %+v", extended)
	}

	var cpu windowsJobObjectCPURateControlInformation
	if err = windows.QueryInformationJobObject(job, jobObjectCPUControlInformation, uintptr(unsafe.Pointer(&cpu)), uint32(unsafe.Sizeof(cpu)), nil); err != nil {
		t.Fatalf("query generic Job Object CPU control: %v", err)
	}
	if cpu.ControlFlags != 0 || cpu.CPURate != 0 {
		t.Fatalf("generic Provider Worker Job Object received project CPU limits: %+v", cpu)
	}
}

func TestWindowsAppContainerDeniesOutsideFilesAndLoopback(t *testing.T) {
	identity := fmt.Sprintf("runner-sandbox-%d", time.Now().UnixNano())
	sandbox, err := NewAppContainerSandbox(identity)
	if err != nil {
		t.Fatalf("create no-network AppContainer profile: %v", err)
	}
	defer func() {
		if closeErr := sandbox.Close(); closeErr != nil {
			t.Errorf("delete AppContainer profile: %v", closeErr)
		}
	}()
	workspace := filepath.Join(sandbox.WorkspaceRoot(), "workspace")
	if err = os.Mkdir(workspace, 0700); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(workspace, "inside.txt"), []byte("contained"), 0600); err != nil {
		t.Fatal(err)
	}
	toolchainDirectory := filepath.Join(workspace, ".polis-toolchain")
	if err = os.Mkdir(toolchainDirectory, 0700); err != nil {
		t.Fatal(err)
	}
	toolchainFile := filepath.Join(toolchainDirectory, "node.exe")
	if err = os.WriteFile(toolchainFile, []byte("trusted toolchain fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	toolchainDirectoryLease, err := sandbox.LockReadOnlyPath(toolchainDirectory)
	if err != nil {
		t.Fatalf("lock toolchain directory against AppContainer writes: %v", err)
	}
	defer toolchainDirectoryLease.Close()
	toolchainFileLease, err := sandbox.LockReadOnlyPath(toolchainFile)
	if err != nil {
		t.Fatalf("lock toolchain file against AppContainer writes: %v", err)
	}
	defer toolchainFileLease.Close()
	outsideRoot := t.TempDir()
	outsideFile := filepath.Join(outsideRoot, "secret.txt")
	if err = os.WriteFile(outsideFile, []byte("outside"), 0600); err != nil {
		t.Fatal(err)
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	childExe := filepath.Join(workspace, "runner-test.exe")
	if err = copyAppContainerTestExecutable(self, childExe); err != nil {
		t.Fatal(err)
	}
	executableLease, err := sandbox.LockReadOnlyPath(childExe)
	if err != nil {
		t.Fatalf("lock AppContainer executable read-only: %v", err)
	}
	defer executableLease.Close()
	systemRoot := os.Getenv("SystemRoot")
	launch := AppContainerLaunchSpec{
		ID: "job-" + identity, WorkspaceRoot: workspace, Executable: childExe,
		Argv:             []string{childExe, "-test.run=^TestWindowsAppContainerChildProbe$"},
		WorkingDirectory: workspace, NetworkPolicy: AppContainerNetworkDenyAll,
		Environment: append(BuildAppContainerEnvironment(sandbox.WorkspaceRoot(), systemRoot),
			"POLIS_APPCONTAINER_CHILD=1",
			"POLIS_APPCONTAINER_WORKSPACE="+workspace,
			"POLIS_APPCONTAINER_OUTSIDE_FILE="+outsideFile,
			"POLIS_APPCONTAINER_OUTSIDE_WRITE="+filepath.Join(outsideRoot, "written.txt"),
			"POLIS_APPCONTAINER_READONLY_TOOLCHAIN="+toolchainFile,
			"POLIS_APPCONTAINER_LOOPBACK="+listener.Addr().String(),
		),
	}
	process, err := sandbox.Launch(launch)
	if err != nil {
		t.Fatalf("launch no-network AppContainer: %v", err)
	}
	defer process.Stop()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if code, waitErr := process.Wait(ctx); waitErr != nil || code != 0 {
		stderr, _ := io.ReadAll(process.Stderr())
		stdout, _ := io.ReadAll(process.Stdout())
		t.Fatalf("AppContainer child failed: exit=%d err=%v stdout=%s stderr=%s", code, waitErr, stdout, stderr)
	}
	stdout, readErr := io.ReadAll(process.Stdout())
	if readErr != nil || !strings.Contains(string(stdout), "outside_read_denied=true") || !strings.Contains(string(stdout), "outside_write_denied=true") || !strings.Contains(string(stdout), "readonly_toolchain_write_denied=true") || !strings.Contains(string(stdout), "network_denied=true") {
		t.Fatalf("AppContainer boundary evidence missing: stdout=%q err=%v", stdout, readErr)
	}
	toolchainBytes, err := os.ReadFile(toolchainFile)
	if err != nil || string(toolchainBytes) != "trusted toolchain fixture" {
		t.Fatalf("AppContainer modified protected toolchain bytes: %q err=%v", toolchainBytes, err)
	}
	if _, statErr := os.Lstat(filepath.Join(outsideRoot, "written.txt")); !os.IsNotExist(statErr) {
		t.Fatalf("AppContainer wrote outside its workspace: %v", statErr)
	}
}

func copyAppContainerTestExecutable(source, destination string) error {
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0700)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(out, in)
	syncErr := out.Sync()
	closeErr := out.Close()
	if copyErr != nil {
		return copyErr
	}
	if syncErr != nil {
		return syncErr
	}
	return closeErr
}

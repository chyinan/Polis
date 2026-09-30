// pattern: Imperative Shell
//go:build windows

package runner

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"
	"unicode/utf16"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	procThreadAttributeSecurityCapabilities = 0x00020009
	procThreadAttributeJobList              = 0x0002000D
	jobObjectCPUControlEnable               = 0x00000001
	jobObjectCPUControlHardCap              = 0x00000004
	jobObjectCPUControlInformation          = 15
	jobObjectQueryAccess                    = 0x0004
	jobObjectTerminateAccess                = 0x0008
	appContainerAlreadyExistsHRESULT        = 0x800700b7
)

type windowsSecurityCapabilities struct {
	AppContainerSID *windows.SID
	Capabilities    *windows.SIDAndAttributes
	CapabilityCount uint32
	Reserved        uint32
}

type windowsAppContainerSandbox struct {
	mu                          sync.Mutex
	profileName                 string
	sid                         *windows.SID
	root                        string
	active                      int
	closing                     bool
	closed                      bool
	profileDeleted              bool
	networkPolicy               string
	registryProxyEndpoint       string
	networkCapabilitySID        *windows.SID
	registryNetworkLease        *windowsRegistryEgressFilterLease
	serviceListenerLease        *windowsServiceListenerWFPLease
	serviceListenerOwner        string
	loopbackEnabled             bool
	registryRevocationRequested bool
	registryPermitRevoked       bool
	managedWorkspaceRoot        bool
}

func newAppContainerBackend(identity string) (appContainerBackend, error) {
	return newWindowsAppContainerBackend(identity, AppContainerNetworkDenyAll, "", RegistryEgressPlan{})
}

func newRegistryEgressAppContainerBackend(identity, registryProxyEndpoint string, plan RegistryEgressPlan, storage AppContainerWorkspaceStorageBinding) (appContainerBackend, error) {
	return newWindowsAppContainerBackend(identity, AppContainerNetworkRegistryOnly, registryProxyEndpoint, plan, storage)
}

func newWindowsAppContainerBackend(identity, networkPolicy, registryProxyEndpoint string, plan RegistryEgressPlan, storageBindings ...AppContainerWorkspaceStorageBinding) (appContainerBackend, error) {
	if len(storageBindings) > 1 {
		return nil, errors.New("AppContainer workspace storage binding is ambiguous")
	}
	if networkPolicy != AppContainerNetworkDenyAll && networkPolicy != AppContainerNetworkRegistryOnly {
		return nil, errors.New("unsupported AppContainer network policy")
	}
	if networkPolicy == AppContainerNetworkDenyAll && registryProxyEndpoint != "" {
		return nil, errors.New("deny-all AppContainer cannot bind a registry proxy")
	}
	if networkPolicy == AppContainerNetworkRegistryOnly {
		expected, err := BuildRegistryEgressPlan(registryProxyEndpoint)
		if err != nil || !reflect.DeepEqual(expected, plan) {
			return nil, errInvalidRegistryEgressEndpoint
		}
	}
	var nonce [16]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return nil, errors.New("could not create AppContainer identity")
	}
	identityHash := digestString(identity)
	profileName := "PolisJob-" + identityHash[:16] + "-" + hex.EncodeToString(nonce[:])
	sid, err := createOrDeriveAppContainerSID(profileName)
	if err != nil {
		return nil, err
	}
	profileRoot, err := getAppContainerFolderPath(sid)
	if err != nil {
		partial := &windowsAppContainerSandbox{profileName: profileName, sid: sid}
		cleanupErr := partial.close()
		if cleanupErr != nil {
			return partial, errors.Join(err, fmt.Errorf("AppContainer profile cleanup remains pending: %w", cleanupErr))
		}
		return nil, err
	}
	info, statErr := os.Lstat(profileRoot)
	if statErr != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		partial := &windowsAppContainerSandbox{profileName: profileName, sid: sid, root: filepath.Clean(profileRoot)}
		cleanupErr := partial.close()
		if cleanupErr != nil {
			return partial, errors.Join(errors.New("AppContainer profile folder is unavailable"), cleanupErr)
		}
		return nil, errors.New("AppContainer profile folder is unavailable")
	}
	root := profileRoot
	managedWorkspaceRoot := false
	if len(storageBindings) == 1 {
		workspaceRoot, prepareErr := prepareWindowsBoundedAppContainerWorkspace(storageBindings[0], profileName, sid)
		if workspaceRoot != "" {
			root = workspaceRoot
			managedWorkspaceRoot = true
		}
		sandbox := &windowsAppContainerSandbox{
			profileName: profileName, sid: sid, root: filepath.Clean(root), managedWorkspaceRoot: managedWorkspaceRoot,
			networkPolicy: networkPolicy, registryProxyEndpoint: registryProxyEndpoint,
		}
		if prepareErr != nil {
			cleanupErr := sandbox.close()
			if cleanupErr != nil {
				return sandbox, errors.Join(prepareErr, fmt.Errorf("AppContainer construction cleanup remains pending: %w", cleanupErr))
			}
			return nil, prepareErr
		}
		root = workspaceRoot
	}
	sandbox := &windowsAppContainerSandbox{
		profileName: profileName, sid: sid, root: filepath.Clean(root),
		managedWorkspaceRoot: managedWorkspaceRoot,
		networkPolicy:        networkPolicy, registryProxyEndpoint: registryProxyEndpoint,
	}
	cleanupFailedConstruction := func(cause error) (appContainerBackend, error) {
		if cleanupErr := sandbox.close(); cleanupErr != nil {
			return sandbox, errors.Join(cause, fmt.Errorf("AppContainer construction cleanup remains pending: %w", cleanupErr))
		}
		return nil, cause
	}
	if managedWorkspaceRoot {
		if err = restrictWindowsAppContainerDefaultProfileWrites(profileRoot, sid); err != nil {
			return cleanupFailedConstruction(err)
		}
	}
	for _, directory := range []string{filepath.Join(root, "profile"), filepath.Join(root, "tmp")} {
		if err = os.MkdirAll(directory, 0700); err != nil {
			return cleanupFailedConstruction(errors.New("AppContainer private directories could not be prepared"))
		}
	}
	if networkPolicy == AppContainerNetworkRegistryOnly {
		sandbox.networkCapabilitySID, err = windows.CreateWellKnownSid(windows.WinCapabilityInternetClientSid)
		if err != nil {
			return cleanupFailedConstruction(errors.New("AppContainer internet client capability is unavailable"))
		}
		sandbox.registryNetworkLease, err = installWindowsRegistryEgressFilters(sid, plan)
		if err != nil {
			return cleanupFailedConstruction(err)
		}
		err = enableLoopbackSIDWithRollback(func(enabled bool) (bool, error) {
			return setAppContainerLoopbackSID(sid, enabled)
		})
		if err != nil {
			return cleanupFailedConstruction(fmt.Errorf("failed to enable AppContainer loopback: %w", err))
		}
		sandbox.loopbackEnabled = true
	}
	return sandbox, nil
}

func (s *windowsAppContainerSandbox) workspaceRoot() string { return s.root }

func (s *windowsAppContainerSandbox) lockReadOnlyPath(path string) (io.Closer, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed || s.closing || s.sid == nil || s.active != 0 || !pathWithinDirectory(s.root, path) {
		return nil, errors.New("AppContainer path cannot be locked while closed, active or outside its profile")
	}
	info, err := os.Lstat(path)
	if err != nil || info.Mode()&os.ModeSymlink != 0 || (!info.IsDir() && !info.Mode().IsRegular()) {
		return nil, errors.New("AppContainer locked path must be a regular file or directory without links")
	}
	if err = validateAppContainerChildPath(s.root, path, info.IsDir()); err != nil {
		return nil, err
	}
	access := uint32(windows.GENERIC_READ | windows.READ_CONTROL)
	flags := uint32(0)
	if info.IsDir() {
		flags = windows.FILE_FLAG_BACKUP_SEMANTICS
	} else {
		access |= windows.FILE_EXECUTE
	}
	pathPointer, err := windows.UTF16PtrFromString(filepath.Clean(path))
	if err != nil {
		return nil, errors.New("AppContainer lock path is invalid")
	}
	handle, err := windows.CreateFile(pathPointer, access, windows.FILE_SHARE_READ, nil, windows.OPEN_EXISTING, flags, 0)
	if err != nil {
		return nil, errors.New("Windows could not hold a read-only toolchain file lease")
	}
	return os.NewFile(uintptr(handle), filepath.Base(path)), nil
}

func (s *windowsAppContainerSandbox) launch(spec AppContainerLaunchSpec) (AppContainerProcess, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed || s.closing || s.sid == nil || spec.NetworkPolicy != s.networkPolicy || spec.RegistryProxyEndpoint != s.registryProxyEndpoint || !pathWithinDirectory(s.root, spec.WorkspaceRoot) {
		return nil, errors.New("AppContainer profile is closed or workspace is outside its private root")
	}
	if s.registryRevocationRequested && s.networkPolicy == AppContainerNetworkRegistryOnly {
		return nil, errors.New("AppContainer registry egress lease is revoked")
	}
	if err := validateAppContainerChildPath(s.root, spec.WorkspaceRoot, true); err != nil {
		return nil, err
	}
	if err := validateAppContainerWorkspace(spec); err != nil {
		return nil, err
	}
	if s.serviceListenerLease != nil {
		return nil, errors.New("AppContainer service-listener network lease is active or awaiting cleanup")
	}
	if spec.ServiceListener != nil {
		return s.launchServiceListener(spec)
	}
	s.active++
	process, err := startWindowsAppContainerProcess(spec, s.sid, s.networkCapabilitySID, s.releaseProcess)
	if err != nil {
		s.active--
		return nil, err
	}
	return process, nil
}

func (s *windowsAppContainerSandbox) launchServiceListener(spec AppContainerLaunchSpec) (AppContainerProcess, error) {
	if s.networkPolicy != AppContainerNetworkDenyAll || s.registryProxyEndpoint != "" || spec.NetworkPolicy != AppContainerNetworkDenyAll {
		return nil, errors.New("service listener AppContainer cannot use registry or general egress")
	}
	if s.active != 0 {
		return nil, errors.New("service listener requires exclusive use of the AppContainer profile")
	}
	plan, err := BuildAppContainerServiceListenerPlan(*spec.ServiceListener)
	if err != nil {
		return nil, err
	}
	lease, err := installWindowsAppContainerServiceListenerFilters(s.sid, plan)
	if err != nil {
		if lease != nil {
			s.serviceListenerLease, s.serviceListenerOwner = lease, spec.ID
		}
		return nil, err
	}
	capabilitySID, err := windows.CreateWellKnownSid(windows.WinCapabilityInternetClientServerSid)
	if err != nil {
		if closeErr := lease.Close(); closeErr != nil {
			s.serviceListenerLease, s.serviceListenerOwner = lease, spec.ID
			return nil, errors.Join(errors.New("AppContainer internet client/server capability is unavailable"), closeErr)
		}
		return nil, errors.New("AppContainer internet client/server capability is unavailable")
	}
	s.serviceListenerLease, s.serviceListenerOwner = lease, spec.ID
	s.active++
	process, launchErr := startWindowsAppContainerProcess(spec, s.sid, capabilitySID, s.releaseProcess)
	_ = windows.FreeSid(capabilitySID)
	if launchErr != nil {
		s.active--
		if closeErr := s.closeServiceListenerLeaseLocked(spec.ID, lease); closeErr != nil {
			return nil, errors.Join(launchErr, closeErr)
		}
		return nil, launchErr
	}
	return newServiceListenerLeaseProcess(process, spec.ID, func() error {
		s.mu.Lock()
		defer s.mu.Unlock()
		return s.closeServiceListenerLeaseLocked(spec.ID, lease)
	}), nil
}

func (s *windowsAppContainerSandbox) closeServiceListenerLeaseLocked(identity string, lease *windowsServiceListenerWFPLease) error {
	if s.serviceListenerLease != lease || s.serviceListenerOwner != identity {
		return nil
	}
	if err := lease.Close(); err != nil {
		return err
	}
	s.serviceListenerLease = nil
	s.serviceListenerOwner = ""
	return nil
}

func (s *windowsAppContainerSandbox) revokeRegistryEgress() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed || s.closing || s.sid == nil {
		return errors.New("AppContainer profile is closed")
	}
	if s.networkPolicy == AppContainerNetworkDenyAll && s.registryPermitRevoked {
		return nil
	}
	if s.networkPolicy != AppContainerNetworkRegistryOnly || s.registryNetworkLease == nil {
		return errors.New("AppContainer profile has no registry-only WFP lease")
	}
	s.registryRevocationRequested = true
	if s.active != 0 {
		return errors.New("AppContainer still has active process trees")
	}
	if !s.registryPermitRevoked {
		if err := s.registryNetworkLease.RevokePermit(); err != nil {
			return err
		}
		s.registryPermitRevoked = true
	}
	if s.loopbackEnabled {
		if _, err := setAppContainerLoopbackSID(s.sid, false); err != nil {
			return err
		}
		s.loopbackEnabled = false
	}
	s.networkCapabilitySID = nil
	s.networkPolicy = AppContainerNetworkDenyAll
	s.registryProxyEndpoint = ""
	return nil
}

func (s *windowsAppContainerSandbox) releaseProcess() {
	s.mu.Lock()
	if s.active > 0 {
		s.active--
	}
	s.mu.Unlock()
}

func (s *windowsAppContainerSandbox) close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil
	}
	if s.active != 0 {
		return errors.New("AppContainer still has active process trees")
	}
	s.closing = true
	if s.serviceListenerLease != nil {
		if err := s.closeServiceListenerLeaseLocked(s.serviceListenerOwner, s.serviceListenerLease); err != nil {
			return fmt.Errorf("service listener WFP cleanup remains pending: %w", err)
		}
	}
	if s.loopbackEnabled {
		if _, err := setAppContainerLoopbackSID(s.sid, false); err != nil {
			return err
		}
		s.loopbackEnabled = false
	}
	if !s.profileDeleted {
		if err := deleteAppContainerProfile(s.profileName); err != nil {
			return err
		}
		s.profileDeleted = true
	}
	if s.managedWorkspaceRoot {
		if err := cleanupWindowsManagedWorkspaceRoot(s.root, true); err != nil {
			return err
		}
		s.managedWorkspaceRoot = false
	}
	if s.registryNetworkLease != nil {
		if err := s.registryNetworkLease.Close(); err != nil {
			return err
		}
		s.registryNetworkLease = nil
	}
	if s.sid != nil {
		_ = windows.FreeSid(s.sid)
		s.sid = nil
	}
	if s.networkCapabilitySID != nil {
		_ = windows.FreeSid(s.networkCapabilitySID)
		s.networkCapabilitySID = nil
	}
	s.closed = true
	return nil
}

type windowsAppContainerProcess struct {
	mu         sync.Mutex
	stopMu     sync.Mutex
	id         string
	pid        int
	job        windows.Handle
	process    windows.Handle
	stdin      *os.File
	stdout     *os.File
	stderr     *os.File
	done       chan struct{}
	waitErr    error
	cleanupErr error
	exitCode   int
	exited     bool
	proof      StopProof
	onExit     func()
}

type windowsJobObjectCPURateControlInformation struct {
	ControlFlags uint32
	CPURate      uint32
}

func startWindowsAppContainerProcess(spec AppContainerLaunchSpec, sid, networkCapabilitySID *windows.SID, onExit func()) (*windowsAppContainerProcess, error) {
	job, err := newWindowsNamedProcessJob(spec.ID)
	if err != nil {
		return nil, errors.New("unable to create AppContainer process job")
	}
	var childStdin, parentStdin, parentStdout, childStdout, parentStderr, childStderr windows.Handle
	cleanupPipes := func() {
		for _, handle := range []windows.Handle{childStdin, parentStdin, parentStdout, childStdout, parentStderr, childStderr} {
			if handle != 0 {
				_ = windows.CloseHandle(handle)
			}
		}
	}
	attributes := &windows.SecurityAttributes{Length: uint32(unsafe.Sizeof(windows.SecurityAttributes{})), InheritHandle: 1}
	if err = windows.CreatePipe(&childStdin, &parentStdin, attributes, 0); err == nil {
		err = windows.CreatePipe(&parentStdout, &childStdout, attributes, 0)
	}
	if err == nil {
		err = windows.CreatePipe(&parentStderr, &childStderr, attributes, 0)
	}
	if err != nil {
		cleanupPipes()
		_ = windows.CloseHandle(job)
		return nil, errors.New("unable to create AppContainer standard streams")
	}
	for _, parentHandle := range []windows.Handle{parentStdin, parentStdout, parentStderr} {
		if err = windows.SetHandleInformation(parentHandle, windows.HANDLE_FLAG_INHERIT, 0); err != nil {
			cleanupPipes()
			_ = windows.CloseHandle(job)
			return nil, errors.New("unable to protect parent standard streams")
		}
	}
	attributeList, err := windows.NewProcThreadAttributeList(3)
	if err != nil {
		cleanupPipes()
		_ = windows.CloseHandle(job)
		return nil, errors.New("unable to initialize AppContainer process attributes")
	}
	defer attributeList.Delete()
	securityCapabilities := windowsSecurityCapabilities{AppContainerSID: sid}
	capabilities := make([]windows.SIDAndAttributes, 0, 1)
	if networkCapabilitySID != nil {
		capabilities = append(capabilities, windows.SIDAndAttributes{Sid: networkCapabilitySID, Attributes: windows.SE_GROUP_ENABLED})
		securityCapabilities.Capabilities = &capabilities[0]
		securityCapabilities.CapabilityCount = uint32(len(capabilities))
	}
	childHandles := []windows.Handle{childStdin, childStdout, childStderr}
	jobHandles := []windows.Handle{job}
	var pinner runtime.Pinner
	pinner.Pin(&securityCapabilities)
	pinner.Pin(&childHandles[0])
	pinner.Pin(&jobHandles[0])
	if len(capabilities) > 0 {
		pinner.Pin(&capabilities[0])
		pinner.Pin(networkCapabilitySID)
	}
	if err = attributeList.Update(procThreadAttributeSecurityCapabilities, unsafe.Pointer(&securityCapabilities), unsafe.Sizeof(securityCapabilities)); err == nil {
		err = attributeList.Update(windows.PROC_THREAD_ATTRIBUTE_HANDLE_LIST, unsafe.Pointer(&childHandles[0]), uintptr(len(childHandles))*unsafe.Sizeof(childHandles[0]))
	}
	if err == nil {
		err = attributeList.Update(procThreadAttributeJobList, unsafe.Pointer(&jobHandles[0]), unsafe.Sizeof(jobHandles[0]))
	}
	if err != nil {
		pinner.Unpin()
		cleanupPipes()
		_ = windows.CloseHandle(job)
		return nil, errors.New("unable to bind AppContainer security capabilities")
	}
	startup := windows.StartupInfoEx{}
	startup.StartupInfo.Cb = uint32(unsafe.Sizeof(startup))
	startup.StartupInfo.Flags = windows.STARTF_USESTDHANDLES | windows.STARTF_USESHOWWINDOW
	startup.StartupInfo.ShowWindow = windows.SW_HIDE
	startup.StartupInfo.StdInput = childStdin
	startup.StartupInfo.StdOutput = childStdout
	startup.StartupInfo.StdErr = childStderr
	startup.ProcThreadAttributeList = attributeList.List()
	environmentBlock, err := buildAppContainerEnvironmentBlock(spec.Environment)
	if err != nil {
		pinner.Unpin()
		cleanupPipes()
		_ = windows.CloseHandle(job)
		return nil, err
	}
	executable, _ := windows.UTF16PtrFromString(spec.Executable)
	workingDirectory, _ := windows.UTF16PtrFromString(spec.WorkingDirectory)
	commandLine, err := appContainerCommandLine(spec.Argv)
	if err != nil {
		pinner.Unpin()
		cleanupPipes()
		_ = windows.CloseHandle(job)
		return nil, err
	}
	commandLineUTF16, err := windows.UTF16PtrFromString(commandLine)
	if err != nil {
		pinner.Unpin()
		cleanupPipes()
		_ = windows.CloseHandle(job)
		return nil, errors.New("AppContainer command line is invalid")
	}
	var processInfo windows.ProcessInformation
	flags := uint32(windows.CREATE_SUSPENDED | windows.EXTENDED_STARTUPINFO_PRESENT | windows.CREATE_UNICODE_ENVIRONMENT | windows.CREATE_NO_WINDOW)
	if err = windows.CreateProcess(executable, commandLineUTF16, nil, nil, true, flags, &environmentBlock[0], workingDirectory, &startup.StartupInfo, &processInfo); err != nil {
		pinner.Unpin()
		cleanupPipes()
		_ = windows.CloseHandle(job)
		return nil, &LaunchFailure{Phase: LaunchPhaseProcessCreate, ReasonCode: LaunchReasonProcessNotCreated, SafeMessage: safeErrorMessage(err), Cause: err}
	}
	pinner.Unpin()
	runtime.KeepAlive(networkCapabilitySID)
	for _, handle := range []windows.Handle{childStdin, childStdout, childStderr} {
		_ = windows.CloseHandle(handle)
	}
	childStdin, childStdout, childStderr = 0, 0, 0
	previousSuspendCount, err := windows.ResumeThread(processInfo.Thread)
	_ = windows.CloseHandle(processInfo.Thread)
	if err != nil || previousSuspendCount != 1 {
		_ = windows.TerminateJobObject(job, 1)
		_, _ = windows.WaitForSingleObject(processInfo.Process, 3000)
		_ = windows.CloseHandle(processInfo.Process)
		cleanupPipes()
		_ = windows.CloseHandle(job)
		return nil, errors.New("unable to resume contained AppContainer process")
	}
	stdin := os.NewFile(uintptr(parentStdin), "appcontainer-stdin")
	stdout := os.NewFile(uintptr(parentStdout), "appcontainer-stdout")
	stderr := os.NewFile(uintptr(parentStderr), "appcontainer-stderr")
	parentStdin, parentStdout, parentStderr = 0, 0, 0
	process := &windowsAppContainerProcess{pid: int(processInfo.ProcessId), job: job, process: processInfo.Process, stdin: stdin, stdout: stdout, stderr: stderr, done: make(chan struct{}), id: spec.ID, onExit: onExit}
	go process.monitor()
	return process, nil
}

func appContainerJobObjectName(identity string) string {
	digest := sha256.Sum256([]byte(identity))
	return "Local\\Polis-AppContainer-" + hex.EncodeToString(digest[:])
}

func newWindowsNamedProcessJob(identity string) (windows.Handle, error) {
	if identity == "" {
		return 0, errors.New("process job identity is required")
	}
	limits := DefaultWindowsJobResourceLimits()
	return newWindowsNamedJobObjectWithLimits(appContainerJobObjectName(identity), &limits)
}

func newWindowsNamedJobObject(objectName string) (windows.Handle, error) {
	return newWindowsNamedJobObjectWithLimits(objectName, nil)
}

func newWindowsNamedJobObjectWithLimits(objectName string, resourceLimits *WindowsJobResourceLimits) (windows.Handle, error) {
	if objectName == "" {
		return 0, errors.New("process job name is required")
	}
	if resourceLimits != nil {
		if err := resourceLimits.Validate(); err != nil {
			return 0, err
		}
	}
	name, err := windows.UTF16PtrFromString(objectName)
	if err != nil {
		return 0, errors.New("process job name is invalid")
	}
	createJob := windows.NewLazySystemDLL("kernel32.dll").NewProc("CreateJobObjectW")
	if err = createJob.Find(); err != nil {
		return 0, errors.New("Windows Job Object creation is unavailable")
	}
	handle, _, callErr := createJob.Call(0, uintptr(unsafe.Pointer(name)))
	if handle == 0 {
		return 0, callErr
	}
	job := windows.Handle(handle)
	if errors.Is(callErr, windows.ERROR_ALREADY_EXISTS) {
		_ = windows.CloseHandle(job)
		return 0, errors.New("a Job Object already exists for this execution identity")
	}
	info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
	info.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE |
		windows.JOB_OBJECT_LIMIT_ACTIVE_PROCESS
	info.BasicLimitInformation.ActiveProcessLimit = 64
	if resourceLimits != nil {
		info.BasicLimitInformation.LimitFlags |= windows.JOB_OBJECT_LIMIT_PROCESS_MEMORY | windows.JOB_OBJECT_LIMIT_JOB_MEMORY
		info.BasicLimitInformation.ActiveProcessLimit = resourceLimits.MaxActiveProcesses
		info.ProcessMemoryLimit = uintptr(resourceLimits.ProcessMemoryBytes)
		info.JobMemoryLimit = uintptr(resourceLimits.JobMemoryBytes)
	}
	if _, err = windows.SetInformationJobObject(job, windows.JobObjectExtendedLimitInformation, uintptr(unsafe.Pointer(&info)), uint32(unsafe.Sizeof(info))); err != nil {
		_ = windows.CloseHandle(job)
		return 0, errors.New("unable to set process job limits")
	}
	if resourceLimits != nil {
		cpuRate, rateErr := resourceLimits.CPURateControlValue()
		if rateErr != nil {
			_ = windows.CloseHandle(job)
			return 0, rateErr
		}
		cpuLimits := windowsJobObjectCPURateControlInformation{
			ControlFlags: jobObjectCPUControlEnable | jobObjectCPUControlHardCap,
			CPURate:      cpuRate,
		}
		if _, err = windows.SetInformationJobObject(job, jobObjectCPUControlInformation, uintptr(unsafe.Pointer(&cpuLimits)), uint32(unsafe.Sizeof(cpuLimits))); err != nil {
			_ = windows.CloseHandle(job)
			return 0, errors.New("unable to set process job CPU rate limit")
		}
	}
	return job, nil
}

type windowsJobAccountingInformation struct {
	TotalUserTime             int64
	TotalKernelTime           int64
	ThisPeriodTotalUserTime   int64
	ThisPeriodTotalKernelTime int64
	TotalPageFaultCount       uint32
	TotalProcesses            uint32
	ActiveProcesses           uint32
	TotalTerminatedProcesses  uint32
}

func activeWindowsJobProcessCount(job windows.Handle) (uint32, error) {
	var accounting windowsJobAccountingInformation
	if err := windows.QueryInformationJobObject(job, windows.JobObjectBasicAccountingInformation, uintptr(unsafe.Pointer(&accounting)), uint32(unsafe.Sizeof(accounting)), nil); err != nil {
		return 0, err
	}
	return accounting.ActiveProcesses, nil
}

func terminateAndWaitWindowsProcessTree(ctx context.Context, job windows.Handle) error {
	if ctx == nil || job == 0 {
		return errors.New("Windows Job Object cleanup input is invalid")
	}
	active, err := activeWindowsJobProcessCount(job)
	if err != nil {
		return errors.New("Windows Job Object process count could not be read")
	}
	if active == 0 {
		return nil
	}
	if err = windows.TerminateJobObject(job, 1); err != nil {
		return errors.New("Windows Job Object could not be terminated")
	}
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()
	for {
		active, err = activeWindowsJobProcessCount(job)
		if err != nil {
			return errors.New("Windows Job Object stop could not be confirmed")
		}
		if active == 0 {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

// ReconcileWindowsProcessTree confirms that a persisted Windows process tree
// has no live members. If the previous control process crashed while the tree
// was active, this can finish termination and wait for the Job Object's active
// process count instead of trusting a potentially reused PID.
func ReconcileWindowsProcessTree(ctx context.Context, identity string) (WindowsProcessTreeStopProof, error) {
	if ctx == nil || identity == "" {
		return WindowsProcessTreeStopProof{}, errors.New("Windows process reconciliation identity is invalid")
	}
	return reconcileWindowsNamedProcessTree(ctx, appContainerJobObjectName(identity), identity, 0, "appcontainer")
}

func reconcileWindowsNamedProcessTree(ctx context.Context, objectName, identity string, pid int, scope string) (WindowsProcessTreeStopProof, error) {
	name, err := windows.UTF16PtrFromString(objectName)
	if err != nil {
		return WindowsProcessTreeStopProof{}, errors.New("Windows process reconciliation name is invalid")
	}
	openJob := windows.NewLazySystemDLL("kernel32.dll").NewProc("OpenJobObjectW")
	if err = openJob.Find(); err != nil {
		return WindowsProcessTreeStopProof{}, errors.New("Windows Job Object query is unavailable")
	}
	handle, _, callErr := openJob.Call(jobObjectQueryAccess|jobObjectTerminateAccess, 0, uintptr(unsafe.Pointer(name)))
	if handle == 0 {
		if errors.Is(callErr, windows.ERROR_FILE_NOT_FOUND) {
			return WindowsProcessTreeStopProof{identity: identity, pid: pid, scope: scope, stopped: true}, nil
		}
		return WindowsProcessTreeStopProof{}, fmt.Errorf("open persisted Windows Job Object: %w", callErr)
	}
	job := windows.Handle(handle)
	defer windows.CloseHandle(job)
	var limits windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION
	if err = windows.QueryInformationJobObject(job, windows.JobObjectExtendedLimitInformation, uintptr(unsafe.Pointer(&limits)), uint32(unsafe.Sizeof(limits)), nil); err != nil {
		return WindowsProcessTreeStopProof{}, errors.New("persisted Windows Job Object limits could not be verified")
	}
	expectedFlags := uint32(windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE | windows.JOB_OBJECT_LIMIT_ACTIVE_PROCESS)
	if limits.BasicLimitInformation.LimitFlags&expectedFlags != expectedFlags || limits.BasicLimitInformation.ActiveProcessLimit != 64 {
		return WindowsProcessTreeStopProof{}, errors.New("persisted Windows Job Object does not match the Polis process-tree policy")
	}
	if err = terminateAndWaitWindowsProcessTree(ctx, job); err != nil {
		return WindowsProcessTreeStopProof{}, err
	}
	return WindowsProcessTreeStopProof{identity: identity, pid: pid, scope: scope, stopped: true}, nil
}

func (p *windowsAppContainerProcess) monitor() {
	processHandle := p.process
	_, waitErr := windows.WaitForSingleObject(processHandle, windows.INFINITE)
	var exitCode uint32
	if waitErr == nil {
		waitErr = windows.GetExitCodeProcess(processHandle, &exitCode)
	}
	p.mu.Lock()
	jobHandle := p.job
	p.job = 0
	p.process = 0
	p.exited = true
	p.waitErr = waitErr
	p.exitCode = int(exitCode)
	p.mu.Unlock()
	processCloseErr := windows.CloseHandle(processHandle)
	cleanupCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	cleanupErr := errors.Join(processCloseErr, terminateAndWaitWindowsProcessTree(cleanupCtx, jobHandle))
	cancel()
	jobCloseErr := windows.CloseHandle(jobHandle)
	cleanupErr = errors.Join(cleanupErr, jobCloseErr)
	_ = p.stdin.Close()
	if cleanupErr == nil && p.onExit != nil {
		p.onExit()
	}
	p.mu.Lock()
	p.cleanupErr = cleanupErr
	p.mu.Unlock()
	close(p.done)
}

func (p *windowsAppContainerProcess) PID() int { return p.pid }
func (p *windowsAppContainerProcess) HasExited() bool {
	if p == nil {
		return true
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.exited || p.process == 0 {
		return true
	}
	state, err := windows.WaitForSingleObject(p.process, 0)
	return err != nil || state != uint32(windows.WAIT_TIMEOUT)
}

// WaitForTreeCleanup returns only after the monitor has confirmed that every
// member exited and closed the Job Object. Unlike Wait, a deadline does not
// initiate a second Stop attempt, so callers can keep recovery bounded.
func (p *windowsAppContainerProcess) WaitForTreeCleanup(ctx context.Context) error {
	if p == nil || p.done == nil {
		return errors.New("AppContainer process tree cleanup is unavailable")
	}
	select {
	case <-p.done:
		p.mu.Lock()
		defer p.mu.Unlock()
		if p.cleanupErr != nil {
			return errors.Join(ErrAppContainerProcessStopUnconfirmed, p.cleanupErr)
		}
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (p *windowsAppContainerProcess) Stdin() io.WriteCloser { return p.stdin }
func (p *windowsAppContainerProcess) Stdout() io.ReadCloser { return p.stdout }
func (p *windowsAppContainerProcess) Stderr() io.ReadCloser { return p.stderr }

func (p *windowsAppContainerProcess) Wait(ctx context.Context) (int, error) {
	select {
	case <-p.done:
		p.mu.Lock()
		defer p.mu.Unlock()
		return p.exitCode, errors.Join(p.waitErr, p.cleanupErr)
	case <-ctx.Done():
		_, stopErr := p.Stop()
		if stopErr != nil {
			return 0, stopErr
		}
		return 0, ctx.Err()
	}
}

func (p *windowsAppContainerProcess) Stop() (StopProof, error) {
	p.stopMu.Lock()
	defer p.stopMu.Unlock()
	if p.proof.stopped {
		return p.proof, nil
	}
	select {
	case <-p.done:
		return p.confirmedStopProof()
	default:
	}
	p.mu.Lock()
	if p.exited || p.job == 0 {
		p.mu.Unlock()
		// The monitor marks the root process exited before closing the Job
		// Object. Wait for done so the proof also covers descendant cleanup.
		select {
		case <-p.done:
			return p.confirmedStopProof()
		case <-time.After(5 * time.Second):
			return StopProof{}, errors.New("AppContainer process tree termination unconfirmed")
		}
	}
	terminateErr := windows.TerminateJobObject(p.job, 1)
	p.mu.Unlock()
	if terminateErr != nil {
		select {
		case <-p.done:
		default:
			return StopProof{}, errors.New("AppContainer process tree termination failed")
		}
	}
	select {
	case <-p.done:
		return p.confirmedStopProof()
	case <-time.After(5 * time.Second):
		return StopProof{}, errors.New("AppContainer process tree termination unconfirmed")
	}
}

func (p *windowsAppContainerProcess) confirmedStopProof() (StopProof, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.cleanupErr != nil {
		return StopProof{}, errors.Join(ErrAppContainerProcessStopUnconfirmed, p.cleanupErr)
	}
	p.proof = StopProof{id: p.id, pid: p.pid, stopped: true}
	return p.proof, nil
}

func (p *windowsAppContainerProcess) ConfirmTreeStopped(proof WindowsProcessTreeStopProof) error {
	if p == nil || !proof.For(p.id) {
		return ErrAppContainerProcessStopUnconfirmed
	}
	select {
	case <-p.done:
	default:
		return ErrAppContainerProcessStopUnconfirmed
	}
	p.mu.Lock()
	if p.cleanupErr == nil {
		p.mu.Unlock()
		return nil
	}
	p.cleanupErr = nil
	p.mu.Unlock()
	if p.onExit != nil {
		p.onExit()
	}
	return nil
}

func createOrDeriveAppContainerSID(profileName string) (*windows.SID, error) {
	profile, err := windows.UTF16PtrFromString(profileName)
	if err != nil {
		return nil, errors.New("AppContainer profile name is invalid")
	}
	displayName, _ := windows.UTF16PtrFromString("Polis isolated job")
	description, _ := windows.UTF16PtrFromString("No-network AppContainer for an isolated Polis job")
	userenv := windows.NewLazySystemDLL("userenv.dll")
	create := userenv.NewProc("CreateAppContainerProfile")
	var sid *windows.SID
	result, _, _ := create.Call(uintptr(unsafe.Pointer(profile)), uintptr(unsafe.Pointer(displayName)), uintptr(unsafe.Pointer(description)), 0, 0, uintptr(unsafe.Pointer(&sid)))
	if uint32(result) == 0 && sid != nil {
		return sid, nil
	}
	if uint32(result) != appContainerAlreadyExistsHRESULT {
		return nil, errors.New("Windows could not create the AppContainer profile")
	}
	derive := userenv.NewProc("DeriveAppContainerSidFromAppContainerName")
	result, _, _ = derive.Call(uintptr(unsafe.Pointer(profile)), uintptr(unsafe.Pointer(&sid)))
	if uint32(result) != 0 || sid == nil {
		return nil, errors.New("Windows could not derive the AppContainer identity")
	}
	return sid, nil
}

func getAppContainerFolderPath(sid *windows.SID) (string, error) {
	sidString, err := windows.UTF16PtrFromString(sid.String())
	if err != nil {
		return "", errors.New("AppContainer SID is invalid")
	}
	userenv := windows.NewLazySystemDLL("userenv.dll")
	getPath := userenv.NewProc("GetAppContainerFolderPath")
	var pathPointer *uint16
	result, _, _ := getPath.Call(uintptr(unsafe.Pointer(sidString)), uintptr(unsafe.Pointer(&pathPointer)))
	if uint32(result) != 0 || pathPointer == nil {
		return "", errors.New("Windows could not locate the AppContainer private folder")
	}
	defer windows.CoTaskMemFree(unsafe.Pointer(pathPointer))
	return windows.UTF16PtrToString(pathPointer), nil
}

func deleteAppContainerProfile(profileName string) error {
	profile, err := windows.UTF16PtrFromString(profileName)
	if err != nil {
		return errors.New("AppContainer profile name is invalid")
	}
	userenv := windows.NewLazySystemDLL("userenv.dll")
	remove := userenv.NewProc("DeleteAppContainerProfile")
	result, _, _ := remove.Call(uintptr(unsafe.Pointer(profile)))
	if uint32(result) != 0 {
		return errors.New("Windows could not delete the AppContainer profile")
	}
	return nil
}

func validateAppContainerWorkspace(spec AppContainerLaunchSpec) error {
	if err := validateAppContainerChildPath(spec.WorkspaceRoot, spec.WorkspaceRoot, true); err != nil {
		return err
	}
	if err := validateAppContainerChildPath(spec.WorkspaceRoot, spec.Executable, false); err != nil {
		return err
	}
	return validateAppContainerChildPath(spec.WorkspaceRoot, spec.WorkingDirectory, true)
}

func validateAppContainerChildPath(root, target string, requireDirectory bool) error {
	if !filepath.IsAbs(root) || !filepath.IsAbs(target) {
		return errors.New("AppContainer path is outside its workspace")
	}
	relative, err := filepath.Rel(filepath.Clean(root), filepath.Clean(target))
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) || filepath.IsAbs(relative) {
		return errors.New("AppContainer path could not be resolved")
	}
	rootInfo, err := os.Lstat(root)
	if err != nil || !rootInfo.IsDir() || rootInfo.Mode()&os.ModeSymlink != 0 {
		return errors.New("AppContainer workspace must be an existing non-link directory")
	}
	current := root
	if relative != "." {
		parts := strings.Split(relative, string(filepath.Separator))
		for index, part := range parts {
			current = filepath.Join(current, part)
			info, statErr := os.Lstat(current)
			if statErr != nil || info.Mode()&os.ModeSymlink != 0 {
				return errors.New("AppContainer path contains a missing or linked component")
			}
			last := index == len(parts)-1
			if !last && !info.IsDir() {
				return errors.New("AppContainer path contains a non-directory parent")
			}
			if last && requireDirectory && !info.IsDir() {
				return errors.New("AppContainer working directory must be a directory")
			}
			if last && !requireDirectory && !info.Mode().IsRegular() {
				return errors.New("AppContainer executable must be a regular file")
			}
		}
	}
	if relative == "." && !requireDirectory {
		return errors.New("AppContainer executable cannot be its workspace root")
	}
	return nil
}

func appContainerCommandLine(argv []string) (string, error) {
	quoted := make([]string, len(argv))
	for index, argument := range argv {
		if strings.ContainsRune(argument, '\x00') {
			return "", errors.New("AppContainer argument contains a null byte")
		}
		quoted[index] = syscall.EscapeArg(argument)
	}
	return strings.Join(quoted, " "), nil
}

func buildAppContainerEnvironmentBlock(environment []string) ([]uint16, error) {
	if err := ValidateAppContainerEnvironment(environment); err != nil {
		return nil, err
	}
	entries := append([]string(nil), environment...)
	sort.Slice(entries, func(i, j int) bool { return strings.ToUpper(entries[i]) < strings.ToUpper(entries[j]) })
	block := strings.Join(entries, "\x00") + "\x00\x00"
	return utf16.Encode([]rune(block)), nil
}

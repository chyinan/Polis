// pattern: Imperative Shell
package environment

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"

	"polis/internal/runner"
)

type WindowsNodeAppContainerSnapshot struct {
	mu              sync.Mutex
	sandbox         *runner.AppContainerSandbox
	materialized    MaterializedProject
	plan            NodeNPMProjectPlan
	toolchainSHA256 string
	toolchainPaths  WindowsNodeToolchainPaths
	toolchainLeases []io.Closer
	active          int
}

// PrepareWindowsNodeAppContainerSnapshot binds a verified immutable source
// snapshot to a new, network-denied AppContainer profile. It does not execute
// Node/npm and does not claim the separate registry-only install policy.
func PrepareWindowsNodeAppContainerSnapshot(identity, workspaceID string, files []ProjectSourceFile, plan NodeNPMProjectPlan, registryHosts []string) (*WindowsNodeAppContainerSnapshot, error) {
	if plan.ProfileID != WindowsNodeNPMProfile || plan.SourceKind != NodeSnapshotFilesSource {
		return nil, ErrEnvironmentMaterialization
	}
	if err := RevalidateNodeNPMProjectFiles(plan, files, registryHosts); err != nil {
		return nil, err
	}
	plan.RegistryHosts = append([]string(nil), plan.RegistryHosts...)
	sandbox, err := runner.NewAppContainerSandbox(identity)
	if err != nil {
		return nil, err
	}
	snapshot := &WindowsNodeAppContainerSnapshot{sandbox: sandbox, plan: plan}
	materialized, err := MaterializeNodeNPMProjectFiles(sandbox.WorkspaceRoot(), workspaceID, files, plan, registryHosts)
	if err != nil {
		return discardFailedAppContainerPreparation(snapshot, err)
	}
	snapshot.materialized = materialized
	return snapshot, nil
}

// PrepareWindowsNodeNPMInstallSnapshot stages the approved source in a
// registry-only AppContainer. Call RevokeRegistryEgress after npm ci exits and
// before starting any project JobRun. If cleanup itself fails, it returns the
// remaining snapshot with the error; callers must retry Close on that snapshot.
func PrepareWindowsNodeNPMInstallSnapshot(identity, workspaceID string, files []ProjectSourceFile, plan NodeNPMProjectPlan, registryHosts []string, toolchainSHA256, nodeSourcePath, npmCLISourcePath string, storage WindowsNodeWorkspaceStorageConfig) (*WindowsNodeAppContainerSnapshot, error) {
	if plan.ProfileID != WindowsNodeNPMProfile || plan.SourceKind != NodeSnapshotFilesSource || plan.InstallPolicy != WindowsNodeInstallPolicy {
		return nil, ErrEnvironmentMaterialization
	}
	if !ValidWindowsNodeToolchainSHA256(toolchainSHA256) {
		return nil, errWindowsNodeToolchain
	}
	sourceToolchainSHA256, err := WindowsNodeToolchainSHA256FromFiles(nodeSourcePath, npmCLISourcePath)
	if err != nil || sourceToolchainSHA256 != toolchainSHA256 {
		return nil, errWindowsNodeToolchain
	}
	if err := RevalidateNodeNPMProjectFiles(plan, files, registryHosts); err != nil {
		return nil, err
	}
	var storageInfo WindowsNodeWorkspaceStorageInfo
	if storage.WorkspaceRoot != "" {
		storageInfo, err = InspectWindowsNodeWorkspaceStorage(storage.WorkspaceRoot, storage.ControlRoot, storage.SystemRoot)
		if err != nil {
			return nil, err
		}
		if runtime.GOOS == "windows" {
			basePolicyFingerprint, ok := IsolationPolicyFingerprint(WindowsNodeNPMProfile)
			if !ok {
				return nil, ErrWindowsNodeWorkspaceStorage
			}
			currentStorageFingerprint, fingerprintErr := WindowsWorkspaceStorageFingerprint(basePolicyFingerprint, storageInfo)
			if fingerprintErr != nil || !WindowsWorkspacePreparationFingerprintMatches(storage.ExpectedIsolationPolicySHA256, currentStorageFingerprint) {
				return nil, errors.Join(ErrWindowsNodeWorkspaceStorage, fingerprintErr)
			}
		}
	} else if runtime.GOOS == "windows" {
		return nil, ErrWindowsNodeWorkspaceStorage
	}
	plan.RegistryHosts = append([]string(nil), plan.RegistryHosts...)
	sandbox, err := runner.NewWindowsNodeNPMInstallAppContainerSandbox(runner.RegistryNPMInstallSandboxOptions{
		Identity: identity, AllowedHosts: registryHosts,
		WorkspaceStorage: runner.AppContainerWorkspaceStorageBinding{
			Root: storage.WorkspaceRoot, ControlRoot: storage.ControlRoot, SystemRoot: storage.SystemRoot,
			ExpectedVolumeRoot: storageInfo.VolumeRoot, ExpectedVolumeGUID: storageInfo.VolumeGUID, ExpectedVolumeSerial: storageInfo.VolumeSerial,
			ExpectedVolumeLabel: storageInfo.VolumeLabel, ExpectedFileSystem: storageInfo.FileSystem, ExpectedTotalBytes: storageInfo.TotalBytes,
		},
	})
	if err != nil {
		if sandbox != nil {
			return discardFailedAppContainerPreparation(&WindowsNodeAppContainerSnapshot{sandbox: sandbox, plan: plan}, err)
		}
		return nil, err
	}
	snapshot := &WindowsNodeAppContainerSnapshot{sandbox: sandbox, plan: plan, toolchainSHA256: toolchainSHA256}
	materialized, err := MaterializeNodeNPMProjectFiles(sandbox.WorkspaceRoot(), workspaceID, files, plan, registryHosts)
	if err != nil {
		return discardFailedAppContainerPreparation(snapshot, err)
	}
	snapshot.materialized = materialized
	toolchainPaths, err := StageWindowsNodeToolchain(materialized.WorkspaceRoot, nodeSourcePath, npmCLISourcePath, toolchainSHA256)
	if err != nil {
		return discardFailedAppContainerPreparation(snapshot, err)
	}
	snapshot.toolchainPaths = toolchainPaths
	snapshot.toolchainLeases = make([]io.Closer, 0, 3)
	for _, path := range []string{filepath.Dir(toolchainPaths.NodeExecutable), toolchainPaths.NodeExecutable, toolchainPaths.NPMCLIScript} {
		lease, lockErr := sandbox.LockReadOnlyPath(path)
		if lockErr != nil {
			return discardFailedAppContainerPreparation(snapshot, lockErr)
		}
		snapshot.toolchainLeases = append(snapshot.toolchainLeases, lease)
	}
	return snapshot, nil
}

func discardFailedAppContainerPreparation(snapshot *WindowsNodeAppContainerSnapshot, cause error) (*WindowsNodeAppContainerSnapshot, error) {
	if snapshot == nil {
		return nil, cause
	}
	if cleanupErr := snapshot.Close(); cleanupErr != nil {
		return snapshot, errors.Join(cause, fmt.Errorf("AppContainer preparation cleanup remains pending: %w", cleanupErr))
	}
	return nil, cause
}

func (s *WindowsNodeAppContainerSnapshot) Launch(executable string, argv, extraEnvironment []string, systemRoot string) (runner.AppContainerProcess, error) {
	return s.LaunchWithIdentity(workspaceIDForAppContainer(s.materialized.WorkspaceRoot), executable, argv, extraEnvironment, systemRoot)
}

// LaunchWithIdentity binds the Windows Job Object to the durable owner ID of
// one execution, so recovery can verify or stop the exact process tree.
func (s *WindowsNodeAppContainerSnapshot) LaunchWithIdentity(identity, executable string, argv, extraEnvironment []string, systemRoot string) (runner.AppContainerProcess, error) {
	if s == nil {
		return nil, runner.ErrAppContainerUnavailable
	}
	if identity == "" {
		return nil, errors.New("AppContainer process identity is required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.sandbox == nil {
		return nil, runner.ErrAppContainerUnavailable
	}
	if s.sandbox.RegistryProxyEndpoint() != "" {
		return nil, errors.New("project commands require registry egress to be revoked first")
	}
	if s.toolchainSHA256 != "" {
		if err := verifyWindowsNodeToolchainStaging(s.materialized.WorkspaceRoot, s.toolchainPaths, s.toolchainSHA256); err != nil {
			return nil, errors.New("staged Node/npm toolchain no longer matches its approved digest")
		}
	}
	process, err := s.launch(identity, executable, argv, extraEnvironment, systemRoot, runner.AppContainerNetworkDenyAll, "", nil)
	if err != nil {
		return nil, err
	}
	return s.trackProcess(process, identity), nil
}

// LaunchNodeProjectScript starts a regular JavaScript file inside the prepared
// project using the pinned Node executable and the snapshot's deny-all profile.
func (s *WindowsNodeAppContainerSnapshot) LaunchNodeProjectScript(identity, scriptPath string, args []string, systemRoot string) (runner.AppContainerProcess, error) {
	if s == nil {
		return nil, runner.ErrAppContainerUnavailable
	}
	if identity == "" {
		return nil, errors.New("project JobRun identity is required")
	}
	relativeScript, err := NormalizeNodeProjectScriptPath(scriptPath)
	if err != nil {
		return nil, err
	}
	if err = ValidateNodeProjectScriptArgs(args); err != nil {
		return nil, err
	}
	if !ValidWindowsNodeToolchainSHA256(s.toolchainSHA256) {
		return nil, errors.New("project script execution requires a digest-bound Node toolchain")
	}
	scriptAbsolute := filepath.Join(s.materialized.ProjectRoot, filepath.FromSlash(relativeScript))
	if !regularContainedToolchainFile(s.materialized.ProjectRoot, scriptAbsolute) {
		return nil, errors.New("project script must be a regular file inside the prepared project")
	}
	argv := make([]string, 1, len(args)+1)
	argv[0] = scriptAbsolute
	argv = append(argv, args...)
	return s.LaunchWithIdentity(identity, s.toolchainPaths.NodeExecutable, argv, nil, systemRoot)
}

func (s *WindowsNodeAppContainerSnapshot) LaunchNodeProjectService(identity, scriptPath string, listener runner.AppContainerServiceListenerSpec, systemRoot string) (runner.AppContainerProcess, error) {
	if s == nil {
		return nil, runner.ErrAppContainerUnavailable
	}
	if identity == "" {
		return nil, errors.New("project service JobRun identity is required")
	}
	if _, err := runner.BuildAppContainerServiceListenerPlan(listener); err != nil {
		return nil, err
	}
	relativeScript, err := NormalizeNodeProjectScriptPath(scriptPath)
	if err != nil {
		return nil, err
	}
	if !ValidWindowsNodeToolchainSHA256(s.toolchainSHA256) {
		return nil, errors.New("project service execution requires a digest-bound Node toolchain")
	}
	scriptAbsolute := filepath.Join(s.materialized.ProjectRoot, filepath.FromSlash(relativeScript))
	if !regularContainedToolchainFile(s.materialized.ProjectRoot, scriptAbsolute) {
		return nil, errors.New("project service script must be a regular file inside the prepared project")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.sandbox == nil || s.sandbox.RegistryProxyEndpoint() != "" {
		return nil, errors.New("project service requires a ready deny-all AppContainer snapshot")
	}
	if err = verifyWindowsNodeToolchainStaging(s.materialized.WorkspaceRoot, s.toolchainPaths, s.toolchainSHA256); err != nil {
		return nil, errors.New("staged Node/npm toolchain no longer matches its approved digest")
	}
	argv := []string{scriptAbsolute}
	extraEnvironment := []string{"HOST=" + listener.BindAddress, "PORT=" + strconv.Itoa(int(listener.Port))}
	return s.launch(identity, s.toolchainPaths.NodeExecutable, argv, extraEnvironment, systemRoot, runner.AppContainerNetworkDenyAll, "", &listener)
}

func (s *WindowsNodeAppContainerSnapshot) StopNodeProjectProcess(process runner.AppContainerProcess, identity string) error {
	if s == nil || process == nil {
		return runner.ErrAppContainerUnavailable
	}
	if identity == "" {
		return runner.ErrAppContainerProcessStopUnconfirmed
	}
	s.mu.Lock()
	if s.sandbox == nil {
		s.mu.Unlock()
		return runner.ErrAppContainerUnavailable
	}
	s.mu.Unlock()
	proof, err := process.Stop()
	if err != nil {
		return errors.Join(err, runner.ErrAppContainerProcessStopUnconfirmed)
	}
	if !proof.For(identity) || proof.PID() != process.PID() {
		return runner.ErrAppContainerProcessStopUnconfirmed
	}
	return nil
}

func (s *WindowsNodeAppContainerSnapshot) launch(identity, executable string, argv, extraEnvironment []string, systemRoot, networkPolicy, registryEndpoint string, serviceListener *runner.AppContainerServiceListenerSpec) (runner.AppContainerProcess, error) {
	info, err := os.Lstat(executable)
	if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return nil, errors.New("AppContainer executable must be a staged regular file")
	}
	environment := append(runner.BuildAppContainerEnvironment(s.sandbox.WorkspaceRoot(), systemRoot), extraEnvironment...)
	return s.sandbox.Launch(runner.AppContainerLaunchSpec{
		ID:                    identity,
		WorkspaceRoot:         s.materialized.WorkspaceRoot,
		Executable:            filepath.Clean(executable),
		Argv:                  append([]string{filepath.Clean(executable)}, argv...),
		WorkingDirectory:      s.materialized.ProjectRoot,
		NetworkPolicy:         networkPolicy,
		RegistryProxyEndpoint: registryEndpoint,
		ServiceListener:       serviceListener,
		Environment:           environment,
	})
}

func (s *WindowsNodeAppContainerSnapshot) LaunchNPMInstall(systemRoot string) (runner.AppContainerProcess, error) {
	if s == nil {
		return nil, runner.ErrAppContainerUnavailable
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.sandbox == nil || s.sandbox.RegistryProxyEndpoint() == "" {
		return nil, errors.New("npm install requires an active registry-only AppContainer lease")
	}
	if !ValidWindowsNodeToolchainSHA256(s.toolchainSHA256) {
		return nil, errors.New("npm install requires a bound Node/npm toolchain digest")
	}
	nodeExecutable := s.toolchainPaths.NodeExecutable
	npmCLIPath := s.toolchainPaths.NPMCLIScript
	argv, err := s.plan.InstallArgs(nodeExecutable, npmCLIPath)
	if err != nil {
		return nil, err
	}
	if err = verifyWindowsNodeToolchainStaging(s.materialized.WorkspaceRoot, s.toolchainPaths, s.toolchainSHA256); err != nil {
		return nil, errors.New("staged Node/npm toolchain bytes do not match the approved digest")
	}
	identity := workspaceIDForAppContainer(s.materialized.WorkspaceRoot)
	process, err := s.launch(identity, nodeExecutable, argv[1:], nil, systemRoot, runner.AppContainerNetworkRegistryOnly, s.sandbox.RegistryProxyEndpoint(), nil)
	if err != nil {
		return nil, err
	}
	return s.trackProcess(process, identity), nil
}

func (s *WindowsNodeAppContainerSnapshot) RevokeRegistryEgress() error {
	if s == nil {
		return runner.ErrAppContainerUnavailable
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.sandbox == nil {
		return runner.ErrAppContainerUnavailable
	}
	return s.sandbox.RevokeRegistryEgress()
}

func (s *WindowsNodeAppContainerSnapshot) Close() error {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.sandbox == nil {
		if err := closeReadOnlyLeases(s.toolchainLeases); err != nil {
			return err
		}
		s.toolchainLeases = nil
		return nil
	}
	if s.active != 0 {
		return errors.New("AppContainer still has active process leases")
	}
	leaseErr := closeReadOnlyLeases(s.toolchainLeases)
	if allReadOnlyLeasesClosed(s.toolchainLeases) {
		s.toolchainLeases = nil
	} else {
		if leaseErr == nil {
			leaseErr = errors.New("AppContainer toolchain file leases remain open")
		}
		return leaseErr
	}
	sandboxErr := s.sandbox.Close()
	if sandboxErr == nil {
		s.sandbox = nil
	}
	return errors.Join(leaseErr, sandboxErr)
}

func (s *WindowsNodeAppContainerSnapshot) trackProcess(process runner.AppContainerProcess, identity string) runner.AppContainerProcess {
	s.active++
	return &trackedEnvironmentProcess{AppContainerProcess: process, id: identity, onExit: func() {
		s.mu.Lock()
		if s.active > 0 {
			s.active--
		}
		s.mu.Unlock()
	}}
}

func closeReadOnlyLeases(leases []io.Closer) error {
	var closeErrors []error
	for index := len(leases) - 1; index >= 0; index-- {
		if leases[index] == nil {
			continue
		}
		if err := leases[index].Close(); err != nil {
			if errors.Is(err, os.ErrClosed) {
				leases[index] = nil
				continue
			}
			closeErrors = append(closeErrors, err)
			continue
		}
		leases[index] = nil
	}
	return errors.Join(closeErrors...)
}

func allReadOnlyLeasesClosed(leases []io.Closer) bool {
	for _, lease := range leases {
		if lease != nil {
			return false
		}
	}
	return true
}

type trackedEnvironmentProcess struct {
	runner.AppContainerProcess
	id     string
	onExit func()
	once   sync.Once
}

func (p *trackedEnvironmentProcess) HasExited() bool {
	if p == nil || p.AppContainerProcess == nil {
		return true
	}
	state, supported := p.AppContainerProcess.(interface{ HasExited() bool })
	return !supported || state.HasExited()
}

func (p *trackedEnvironmentProcess) WaitForTreeCleanup(ctx context.Context) error {
	if p == nil || p.AppContainerProcess == nil {
		return errors.New("AppContainer process tree cleanup is unavailable")
	}
	waiter, supported := p.AppContainerProcess.(interface{ WaitForTreeCleanup(context.Context) error })
	if !supported {
		return errors.New("AppContainer process tree cleanup cannot be confirmed")
	}
	return waiter.WaitForTreeCleanup(ctx)
}

func (p *trackedEnvironmentProcess) ConfirmTreeStopped(proof runner.WindowsProcessTreeStopProof) error {
	if p == nil || p.AppContainerProcess == nil {
		return runner.ErrAppContainerProcessStopUnconfirmed
	}
	confirmer, supported := p.AppContainerProcess.(interface {
		ConfirmTreeStopped(runner.WindowsProcessTreeStopProof) error
	})
	if !supported {
		return runner.ErrAppContainerProcessStopUnconfirmed
	}
	if err := confirmer.ConfirmTreeStopped(proof); err != nil {
		return err
	}
	p.release()
	return nil
}

func (p *trackedEnvironmentProcess) release() {
	p.once.Do(p.onExit)
}

func (p *trackedEnvironmentProcess) Wait(ctx context.Context) (int, error) {
	code, err := p.AppContainerProcess.Wait(ctx)
	if ctx.Err() == nil {
		confirmedStopped := false
		if confirmer, ok := p.AppContainerProcess.(interface{ ProcessTreeStopped() bool }); ok {
			confirmedStopped = confirmer.ProcessTreeStopped()
		}
		if err == nil || confirmedStopped {
			p.release()
		}
		return code, err
	}
	proof, stopErr := p.AppContainerProcess.Stop()
	if stopErr != nil {
		return code, errors.Join(err, stopErr, runner.ErrAppContainerProcessStopUnconfirmed)
	}
	if !proof.For(p.id) || proof.PID() != p.AppContainerProcess.PID() {
		return code, errors.Join(err, runner.ErrAppContainerProcessStopUnconfirmed)
	}
	p.release()
	return code, err
}

func (p *trackedEnvironmentProcess) Stop() (runner.StopProof, error) {
	proof, err := p.AppContainerProcess.Stop()
	if err == nil && proof.For(p.id) && proof.PID() == p.AppContainerProcess.PID() {
		p.release()
	} else if err == nil {
		err = runner.ErrAppContainerProcessStopUnconfirmed
	}
	return proof, err
}

func workspaceIDForAppContainer(workspaceRoot string) string {
	return filepath.Base(filepath.Clean(workspaceRoot))
}

func regularContainedToolchainFile(root, target string) bool {
	if !filepath.IsAbs(root) || !filepath.IsAbs(target) {
		return false
	}
	rootInfo, rootErr := os.Lstat(root)
	if rootErr != nil || !rootInfo.IsDir() || rootInfo.Mode()&os.ModeSymlink != 0 {
		return false
	}
	relative, err := filepath.Rel(filepath.Clean(root), filepath.Clean(target))
	if err != nil || relative == "." || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) || filepath.IsAbs(relative) {
		return false
	}
	current := filepath.Clean(root)
	parts := strings.Split(relative, string(filepath.Separator))
	for index, part := range parts {
		current = filepath.Join(current, part)
		info, statErr := os.Lstat(current)
		if statErr != nil || info.Mode()&os.ModeSymlink != 0 {
			return false
		}
		if index < len(parts)-1 && !info.IsDir() {
			return false
		}
	}
	info, err := os.Lstat(current)
	return err == nil && info.Mode().IsRegular() && info.Mode()&os.ModeSymlink == 0
}

// pattern: Imperative Shell
package runner

import (
	"context"
	"errors"
	"path/filepath"
	"sync"
	"testing"
)

func TestAppContainerLaunchAndCloseSerializeProxyLeaseAccess(t *testing.T) {
	proxy, err := StartRegistryTunnelProxy(context.Background(), []string{"registry.npmjs.org"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	backend := &closingTestAppContainerBackend{root: t.TempDir()}
	sandbox := &AppContainerSandbox{
		backend: backend, networkPolicy: AppContainerNetworkRegistryOnly,
		registryProxyEndpoint: proxy.Addr().String(), registryProxy: proxy,
	}
	workspace := filepath.Join(backend.root, "workspace")
	executable := filepath.Join(workspace, "node.exe")
	launch := AppContainerLaunchSpec{
		ID: "job-1", WorkspaceRoot: workspace, Executable: executable,
		Argv: []string{executable, "script.js"}, WorkingDirectory: workspace,
		NetworkPolicy: AppContainerNetworkRegistryOnly, RegistryProxyEndpoint: proxy.Addr().String(),
		Environment: []string{
			"HOME=" + filepath.Join(workspace, "home"), "LOCALAPPDATA=" + workspace, "PATH=C:\\Windows\\System32",
			"SYSTEMROOT=C:\\Windows", "TEMP=" + filepath.Join(workspace, "tmp"),
			"TMP=" + filepath.Join(workspace, "tmp"), "USERPROFILE=" + filepath.Join(workspace, "home"),
		},
	}
	start := make(chan struct{})
	var workers sync.WaitGroup
	for range 16 {
		workers.Add(1)
		go func() {
			defer workers.Done()
			<-start
			for range 50 {
				_, _ = sandbox.Launch(launch)
			}
		}()
	}
	workers.Add(1)
	go func() {
		defer workers.Done()
		<-start
		_ = sandbox.Close()
	}()
	close(start)
	workers.Wait()
	if err = sandbox.Close(); err != nil {
		t.Fatalf("close sandbox after concurrent launches: %v", err)
	}
}

func TestRegistryProxyEndpointIsEmptyAfterClose(t *testing.T) {
	sandbox := &AppContainerSandbox{
		backend:       &closingTestAppContainerBackend{root: t.TempDir()},
		networkPolicy: AppContainerNetworkRegistryOnly, registryProxyEndpoint: "127.0.0.1:43123",
	}
	if err := sandbox.Close(); err != nil {
		t.Fatalf("close registry-only sandbox: %v", err)
	}
	if got := sandbox.RegistryProxyEndpoint(); got != "" {
		t.Fatalf("closed sandbox endpoint = %q, want empty", got)
	}
}

func TestAppContainerCloseStopsRegistryBrokerBeforeBackendCleanup(t *testing.T) {
	proxy, err := StartRegistryTunnelProxy(context.Background(), []string{"registry.npmjs.org"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	backendErr := errors.New("AppContainer cleanup failed")
	backend := &closingTestAppContainerBackend{root: t.TempDir(), closeErr: backendErr}
	sandbox := &AppContainerSandbox{
		backend: backend, networkPolicy: AppContainerNetworkRegistryOnly,
		registryProxyEndpoint: proxy.Addr().String(), registryProxy: proxy,
	}
	if err = sandbox.Close(); !errors.Is(err, backendErr) {
		t.Fatalf("sandbox close error=%v want backend failure", err)
	}
	if proxy.isActive() {
		t.Fatal("registry proxy stayed available after AppContainer cleanup failed")
	}
}

func TestRevokeRegistryEgressClosesBrokerAndReturnsOuterPolicyToDenyAll(t *testing.T) {
	proxy, err := StartRegistryTunnelProxy(context.Background(), []string{"registry.npmjs.org"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	backend := &revokingTestAppContainerBackend{root: t.TempDir()}
	sandbox := &AppContainerSandbox{
		backend: backend, networkPolicy: AppContainerNetworkRegistryOnly,
		registryProxyEndpoint: proxy.Addr().String(), registryProxy: proxy,
	}
	if err = sandbox.RevokeRegistryEgress(); err != nil {
		t.Fatalf("revoke registry egress: %v", err)
	}
	if backend.revocations != 1 || sandbox.networkPolicy != AppContainerNetworkDenyAll || sandbox.registryProxy != nil || sandbox.RegistryProxyEndpoint() != "" || proxy.URL() != "" {
		t.Fatalf("registry lease did not close into deny-all state: revocations=%d policy=%q endpoint=%q proxy=%v", backend.revocations, sandbox.networkPolicy, sandbox.registryProxyEndpoint, sandbox.registryProxy)
	}
	if err = sandbox.RevokeRegistryEgress(); err != nil {
		t.Fatalf("idempotent registry egress revocation: %v", err)
	}
}

func TestRevokeRegistryEgressFailureClosesBrokerBeforeRetryingWFP(t *testing.T) {
	proxy, err := StartRegistryTunnelProxy(context.Background(), []string{"registry.npmjs.org"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer proxy.Close()
	revokeErr := errors.New("WFP permit delete failed")
	backend := &revokingTestAppContainerBackend{root: t.TempDir(), revokeErr: revokeErr}
	sandbox := &AppContainerSandbox{
		backend: backend, networkPolicy: AppContainerNetworkRegistryOnly,
		registryProxyEndpoint: proxy.Addr().String(), registryProxy: proxy,
	}
	if err = sandbox.RevokeRegistryEgress(); !errors.Is(err, revokeErr) {
		t.Fatalf("registry egress revocation error = %v, want WFP failure", err)
	}
	if sandbox.registryProxy != proxy || sandbox.RegistryProxyEndpoint() == "" || proxy.isActive() || sandbox.networkPolicy != AppContainerNetworkRegistryOnly {
		t.Fatal("failed WFP revocation did not close the broker while retaining the lease for recovery")
	}
	_, _ = sandbox.Launch(registrySandboxLaunchSpec(backend.root, proxy.Addr().String()))
	if backend.launches != 0 {
		t.Fatal("failed registry revocation allowed another process to launch after the broker closed")
	}
	backend.revokeErr = nil
	if err = sandbox.RevokeRegistryEgress(); err != nil {
		t.Fatalf("retry WFP revocation after broker close: %v", err)
	}
	if backend.revocations != 2 || sandbox.networkPolicy != AppContainerNetworkDenyAll || sandbox.RegistryProxyEndpoint() != "" {
		t.Fatalf("WFP retry did not finish into deny-all: revocations=%d policy=%q endpoint=%q", backend.revocations, sandbox.networkPolicy, sandbox.RegistryProxyEndpoint())
	}
}

func TestRegistryOnlySandboxRejectsUnapprovedNPMRegistry(t *testing.T) {
	proxy, err := StartRegistryTunnelProxy(context.Background(), []string{"registry.npmjs.org"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer proxy.Close()
	backend := &revokingTestAppContainerBackend{root: t.TempDir()}
	sandbox := &AppContainerSandbox{
		backend: backend, networkPolicy: AppContainerNetworkRegistryOnly,
		registryProxyEndpoint: proxy.Addr().String(), registryProxy: proxy,
	}
	spec := registrySandboxLaunchSpec(backend.root, proxy.Addr().String())
	spec.Argv[6] = "--registry=https://packages.example.invalid/"
	if _, err = sandbox.Launch(spec); err == nil {
		t.Fatal("registry-only sandbox accepted an npm host outside its proxy lease")
	}
	if backend.launches != 0 {
		t.Fatal("unapproved npm registry reached the AppContainer backend")
	}
}

type closingTestAppContainerBackend struct {
	root     string
	closeErr error
}

type revokingTestAppContainerBackend struct {
	root        string
	revocations int
	launches    int
	revokeErr   error
}

func (backend *revokingTestAppContainerBackend) workspaceRoot() string { return backend.root }

func (backend *revokingTestAppContainerBackend) launch(AppContainerLaunchSpec) (AppContainerProcess, error) {
	backend.launches++
	return nil, errors.New("test backend does not launch processes")
}

func (backend *revokingTestAppContainerBackend) close() error { return nil }

func (backend *revokingTestAppContainerBackend) revokeRegistryEgress() error {
	backend.revocations++
	return backend.revokeErr
}

func registrySandboxLaunchSpec(root, endpoint string) AppContainerLaunchSpec {
	workspace := filepath.Join(root, "workspace")
	toolchain := filepath.Join(workspace, ".polis-toolchain")
	executable := filepath.Join(toolchain, "node.exe")
	return AppContainerLaunchSpec{
		ID: "job-1", WorkspaceRoot: workspace, Executable: executable,
		Argv: []string{executable, filepath.Join(toolchain, "npm-cli.js"), "ci", "--ignore-scripts", "--no-audit", "--no-fund", "--registry=https://registry.npmjs.org/"}, WorkingDirectory: workspace,
		NetworkPolicy: AppContainerNetworkRegistryOnly, RegistryProxyEndpoint: endpoint,
		Environment: []string{
			"HOME=" + filepath.Join(workspace, "home"), "LOCALAPPDATA=" + workspace, "PATH=C:\\Windows\\System32",
			"SYSTEMROOT=C:\\Windows", "TEMP=" + filepath.Join(workspace, "tmp"),
			"TMP=" + filepath.Join(workspace, "tmp"), "USERPROFILE=" + filepath.Join(workspace, "home"),
		},
	}
}

func (backend *closingTestAppContainerBackend) workspaceRoot() string { return backend.root }

func (backend *closingTestAppContainerBackend) launch(AppContainerLaunchSpec) (AppContainerProcess, error) {
	return nil, errors.New("test backend does not launch processes")
}

func (backend *closingTestAppContainerBackend) close() error { return backend.closeErr }

// pattern: Functional Core
package runner

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
)

const AppContainerNetworkDenyAll = "deny_all"

var ErrAppContainerUnavailable = errors.New("Windows AppContainer isolation is unavailable on this platform")
var ErrAppContainerProcessStopUnconfirmed = errors.New("Windows AppContainer process-tree stop is unconfirmed")
var appContainerEnvNamePattern = regexp.MustCompile(`^[A-Z_][A-Z0-9_]{0,127}$`)

type AppContainerLaunchSpec struct {
	ID                    string
	WorkspaceRoot         string
	Executable            string
	Argv                  []string
	WorkingDirectory      string
	NetworkPolicy         string
	RegistryProxyEndpoint string
	ServiceListener       *AppContainerServiceListenerSpec
	Environment           []string
}

type appContainerBackend interface {
	workspaceRoot() string
	launch(AppContainerLaunchSpec) (AppContainerProcess, error)
	close() error
}

type registryEgressRevoker interface {
	revokeRegistryEgress() error
}

type appContainerPathLocker interface {
	lockReadOnlyPath(string) (io.Closer, error)
}

type AppContainerSandbox struct {
	mu                          sync.Mutex
	backend                     appContainerBackend
	networkPolicy               string
	registryProxyEndpoint       string
	registryProxy               *RegistryTunnelProxy
	registryRevocationRequested bool
}

type RegistryNPMInstallSandboxOptions struct {
	Identity         string
	AllowedHosts     []string
	WorkspaceStorage AppContainerWorkspaceStorageBinding
}

type AppContainerWorkspaceStorageBinding struct {
	Root                 string
	ControlRoot          string
	SystemRoot           string
	ExpectedVolumeRoot   string
	ExpectedVolumeGUID   string
	ExpectedVolumeSerial uint64
	ExpectedVolumeLabel  string
	ExpectedFileSystem   string
	ExpectedTotalBytes   uint64
}

type AppContainerProcess interface {
	PID() int
	Stdin() io.WriteCloser
	Stdout() io.ReadCloser
	Stderr() io.ReadCloser
	Wait(context.Context) (int, error)
	Stop() (StopProof, error)
}

func NewAppContainerSandbox(identity string) (*AppContainerSandbox, error) {
	if !validAppContainerIdentity(identity) {
		return nil, errors.New("invalid AppContainer identity")
	}
	backend, err := newAppContainerBackend(identity)
	if err != nil {
		return nil, err
	}
	return &AppContainerSandbox{backend: backend, networkPolicy: AppContainerNetworkDenyAll}, nil
}

func NewWindowsNodeNPMInstallAppContainerSandbox(options RegistryNPMInstallSandboxOptions) (*AppContainerSandbox, error) {
	if !validAppContainerIdentity(options.Identity) {
		return nil, errors.New("invalid AppContainer identity")
	}
	proxy, err := StartRegistryTunnelProxy(context.Background(), options.AllowedHosts, nil)
	if err != nil {
		return nil, err
	}
	plan, err := BuildRegistryEgressPlan(proxy.Addr().String())
	if err != nil {
		_ = proxy.Close()
		return nil, err
	}
	endpoint := proxy.Addr().String()
	backend, err := newRegistryEgressAppContainerBackend(options.Identity, endpoint, plan, options.WorkspaceStorage)
	if err != nil {
		if backend != nil {
			partial := &AppContainerSandbox{
				backend: backend, networkPolicy: AppContainerNetworkRegistryOnly,
				registryProxyEndpoint: endpoint, registryProxy: proxy,
			}
			if cleanupErr := partial.Close(); cleanupErr != nil {
				return partial, errors.Join(err, cleanupErr)
			}
		} else {
			_ = proxy.Close()
		}
		return nil, err
	}
	return &AppContainerSandbox{
		backend: backend, networkPolicy: AppContainerNetworkRegistryOnly,
		registryProxyEndpoint: endpoint, registryProxy: proxy,
	}, nil
}

func (s *AppContainerSandbox) WorkspaceRoot() string {
	if s == nil || s.backend == nil {
		return ""
	}
	return s.backend.workspaceRoot()
}

// RegistryProxyEndpoint returns the uncredentialed loopback endpoint required
// in a registry-only launch spec. Authentication remains private to the
// sandbox and is injected into the child proxy environment by Launch.
func (s *AppContainerSandbox) RegistryProxyEndpoint() string {
	if s == nil {
		return ""
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.networkPolicy != AppContainerNetworkRegistryOnly || s.registryProxyEndpoint == "" {
		return ""
	}
	return s.registryProxyEndpoint
}

func (s *AppContainerSandbox) Launch(spec AppContainerLaunchSpec) (AppContainerProcess, error) {
	if s == nil {
		return nil, ErrAppContainerUnavailable
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.backend == nil {
		return nil, ErrAppContainerUnavailable
	}
	if err := ValidateAppContainerLaunchSpec(spec); err != nil {
		return nil, err
	}
	if spec.NetworkPolicy != s.networkPolicy || spec.RegistryProxyEndpoint != s.registryProxyEndpoint {
		return nil, errors.New("AppContainer launch network policy differs from its leased profile")
	}
	if s.networkPolicy == AppContainerNetworkRegistryOnly && s.registryRevocationRequested {
		return nil, errors.New("AppContainer registry egress revocation is pending")
	}
	if s.networkPolicy == AppContainerNetworkRegistryOnly && (s.registryProxy == nil || !s.registryProxy.isActive()) {
		return nil, errors.New("registry egress broker is not active for this AppContainer profile")
	}
	if s.networkPolicy == AppContainerNetworkRegistryOnly {
		registryHost, err := registryNPMInstallHost(spec)
		if err != nil {
			return nil, err
		}
		if _, allowed := s.registryProxy.hosts[registryHost]; !allowed {
			return nil, errors.New("npm install registry is outside this AppContainer lease")
		}
		environment, err := BuildRegistryProxyEnvironment(spec.Environment, s.registryProxy.URL())
		if err != nil {
			return nil, err
		}
		spec.Environment = environment
	}
	if !pathWithinDirectory(s.backend.workspaceRoot(), spec.WorkspaceRoot) {
		return nil, errors.New("AppContainer workspace is outside its private profile root")
	}
	return s.backend.launch(spec)
}

// RevokeRegistryEgress permanently removes the registry-only permit from this
// lease while retaining the AppContainer's default-block filters. It is used
// after dependency preparation and before project JobRuns.
func (s *AppContainerSandbox) RevokeRegistryEgress() error {
	if s == nil {
		return ErrAppContainerUnavailable
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.backend == nil {
		return ErrAppContainerUnavailable
	}
	if s.networkPolicy == AppContainerNetworkDenyAll && s.registryProxyEndpoint == "" {
		return nil
	}
	if s.networkPolicy != AppContainerNetworkRegistryOnly {
		return errors.New("AppContainer has no revocable registry egress lease")
	}
	s.registryRevocationRequested = true
	var brokerErr error
	if s.registryProxy != nil {
		brokerErr = s.registryProxy.Close()
	}
	revoker, ok := s.backend.(registryEgressRevoker)
	if !ok {
		return errors.Join(brokerErr, ErrAppContainerUnavailable)
	}
	if err := revoker.revokeRegistryEgress(); err != nil {
		return errors.Join(brokerErr, err)
	}
	var closeErr error
	if s.registryProxy != nil {
		closeErr = s.registryProxy.Close()
		s.registryProxy = nil
	}
	s.registryProxyEndpoint = ""
	s.networkPolicy = AppContainerNetworkDenyAll
	s.registryRevocationRequested = false
	return errors.Join(brokerErr, closeErr)
}

// LockReadOnlyPath holds a sharing-deny-write handle to one contained path.
// Keep the returned lease open for every process that must not alter the path.
func (s *AppContainerSandbox) LockReadOnlyPath(path string) (io.Closer, error) {
	if s == nil {
		return nil, ErrAppContainerUnavailable
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.backend == nil || !pathWithinDirectory(s.backend.workspaceRoot(), path) {
		return nil, errors.New("read-only AppContainer path is outside its private profile")
	}
	locker, ok := s.backend.(appContainerPathLocker)
	if !ok {
		return nil, ErrAppContainerUnavailable
	}
	return locker.lockReadOnlyPath(path)
}

func (s *AppContainerSandbox) Close() error {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.backend == nil {
		return nil
	}
	var brokerErr error
	if s.registryProxy != nil {
		brokerErr = s.registryProxy.Close()
	}
	if err := s.backend.close(); err != nil {
		return errors.Join(brokerErr, err)
	}
	if s.registryProxy != nil {
		if err := s.registryProxy.Close(); err != nil {
			return errors.Join(brokerErr, err)
		}
		s.registryProxy = nil
	}
	if s.networkPolicy == AppContainerNetworkRegistryOnly {
		s.registryProxyEndpoint = ""
		s.networkPolicy = AppContainerNetworkDenyAll
	}
	s.registryRevocationRequested = false
	return brokerErr
}

func ValidateAppContainerLaunchSpec(spec AppContainerLaunchSpec) error {
	if !validAppContainerIdentity(spec.ID) || !filepath.IsAbs(spec.WorkspaceRoot) || !filepath.IsAbs(spec.Executable) || !filepath.IsAbs(spec.WorkingDirectory) || len(spec.Argv) == 0 || len(spec.Argv) > 64 {
		return errors.New("AppContainer launch specification is incomplete or requests unsupported network access")
	}
	switch spec.NetworkPolicy {
	case AppContainerNetworkDenyAll:
		if spec.RegistryProxyEndpoint != "" {
			return errors.New("deny-all AppContainer launch must not bind a registry proxy")
		}
	case AppContainerNetworkRegistryOnly:
		if _, err := BuildRegistryEgressPlan(spec.RegistryProxyEndpoint); err != nil {
			return err
		}
		if err := ValidateRegistryNPMInstallEnvironment(spec.Environment); err != nil {
			return err
		}
		if _, err := registryNPMInstallHost(spec); err != nil {
			return err
		}
	default:
		return errors.New("AppContainer launch requests an unsupported network policy")
	}
	if spec.ServiceListener != nil {
		if spec.NetworkPolicy != AppContainerNetworkDenyAll || spec.RegistryProxyEndpoint != "" {
			return errors.New("service listener policy is incompatible with AppContainer egress")
		}
		if _, err := BuildAppContainerServiceListenerPlan(*spec.ServiceListener); err != nil {
			return err
		}
	}
	root := filepath.Clean(spec.WorkspaceRoot)
	if !pathWithinDirectory(root, spec.Executable) || !pathWithinDirectoryOrEqual(root, spec.WorkingDirectory) || filepath.Clean(spec.Argv[0]) != filepath.Clean(spec.Executable) {
		return errors.New("AppContainer executable and working directory must remain inside its workspace")
	}
	commandLength := 0
	for _, argument := range spec.Argv {
		if strings.ContainsRune(argument, '\x00') || len(argument) > 4096 {
			return errors.New("AppContainer argument exceeds the accepted bound")
		}
		commandLength += len(argument) + 1
	}
	if commandLength > 32760 {
		return errors.New("AppContainer command line exceeds the Windows bound")
	}
	return ValidateAppContainerEnvironment(spec.Environment)
}

func ValidateRegistryNPMInstallEnvironment(environment []string) error {
	allowed := map[string]struct{}{
		"HOME": {}, "LOCALAPPDATA": {}, "PATH": {}, "SYSTEMROOT": {}, "TEMP": {}, "TMP": {}, "USERPROFILE": {},
	}
	values := make(map[string]string, len(environment))
	for _, entry := range environment {
		name, value, ok := strings.Cut(entry, "=")
		if !ok {
			return errors.New("registry-only npm environment contains an invalid entry")
		}
		if _, exists := allowed[name]; !exists {
			return errors.New("registry-only npm environment contains an unsupported override")
		}
		values[name] = value
	}
	if len(values) != len(allowed) {
		return errors.New("registry-only npm environment must contain only the fixed AppContainer variables")
	}
	for name := range allowed {
		if values[name] == "" {
			return errors.New("registry-only npm environment is missing a fixed AppContainer variable")
		}
	}
	systemRoot := strings.TrimRight(values["SYSTEMROOT"], `\/`)
	wantPath := systemRoot + `\System32;` + systemRoot
	if systemRoot == "" || !strings.EqualFold(values["PATH"], wantPath) {
		return errors.New("registry-only npm PATH must contain only the pinned Windows system directories")
	}
	return nil
}

func registryNPMInstallHost(spec AppContainerLaunchSpec) (string, error) {
	if len(spec.Argv) != 7 || !strings.EqualFold(filepath.Base(spec.Executable), "node.exe") || !strings.EqualFold(filepath.Base(filepath.Dir(spec.Executable)), ".polis-toolchain") {
		return "", errors.New("registry-only AppContainer launch is reserved for the pinned npm ci installer")
	}
	expectedNPMCLI := filepath.Join(filepath.Dir(spec.Executable), "npm-cli.js")
	if !strings.EqualFold(filepath.Clean(spec.Argv[0]), filepath.Clean(spec.Executable)) || !strings.EqualFold(filepath.Clean(spec.Argv[1]), filepath.Clean(expectedNPMCLI)) || spec.Argv[2] != "ci" || spec.Argv[3] != "--ignore-scripts" || spec.Argv[4] != "--no-audit" || spec.Argv[5] != "--no-fund" || !strings.HasPrefix(spec.Argv[6], "--registry=") {
		return "", errors.New("registry-only AppContainer launch must use the fixed npm ci policy")
	}
	registryURL := strings.TrimPrefix(spec.Argv[6], "--registry=")
	parsed, err := url.Parse(registryURL)
	if err != nil || parsed.Scheme != "https" || parsed.Opaque != "" || parsed.User != nil || parsed.Port() != "" || parsed.Path != "/" || parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", errors.New("npm install registry URL is invalid")
	}
	host := parsed.Hostname()
	if !validRegistryProxyHostname(host) || (&url.URL{Scheme: "https", Host: host, Path: "/"}).String() != registryURL {
		return "", errors.New("npm install registry URL must be a canonical hostname")
	}
	return host, nil
}

func ValidateAppContainerEnvironment(environment []string) error {
	if len(environment) < 7 || len(environment) > 64 {
		return errors.New("AppContainer environment has an invalid entry count")
	}
	seen := make(map[string]struct{}, len(environment))
	totalEnvironmentBytes := 0
	for _, item := range environment {
		name, value, ok := strings.Cut(item, "=")
		if !ok || !appContainerEnvNamePattern.MatchString(name) || sensitiveEnvironmentName(name) || strings.ContainsRune(value, '\x00') || len(value) > 4096 {
			return errors.New("AppContainer environment contains an invalid or sensitive entry")
		}
		if _, duplicate := seen[name]; duplicate {
			return errors.New("AppContainer environment contains duplicate names")
		}
		seen[name] = struct{}{}
		totalEnvironmentBytes += len(item) + 1
	}
	if totalEnvironmentBytes > 32760 {
		return errors.New("AppContainer environment exceeds the Windows bound")
	}
	for _, required := range []string{"HOME", "LOCALAPPDATA", "PATH", "SYSTEMROOT", "TEMP", "TMP", "USERPROFILE"} {
		if _, exists := seen[required]; !exists || environmentValue(environment, required) == "" {
			return fmt.Errorf("AppContainer environment is missing %s", required)
		}
	}
	return nil
}

func BuildAppContainerEnvironment(appContainerRoot, systemRoot string) []string {
	root := strings.TrimRight(appContainerRoot, `\/`)
	profile := root + `\profile`
	temp := root + `\tmp`
	systemRoot = strings.TrimRight(systemRoot, `\/`)
	return []string{
		"HOME=" + profile,
		"LOCALAPPDATA=" + root,
		"PATH=" + systemRoot + `\System32;` + systemRoot,
		"SYSTEMROOT=" + systemRoot,
		"TEMP=" + temp,
		"TMP=" + temp,
		"USERPROFILE=" + profile,
	}
}

func validAppContainerIdentity(value string) bool {
	if value == "" || len(value) > 80 {
		return false
	}
	for _, character := range value {
		if (character >= 'a' && character <= 'z') || (character >= 'A' && character <= 'Z') || (character >= '0' && character <= '9') || character == '_' || character == '-' {
			continue
		}
		return false
	}
	return true
}

func pathWithinDirectory(root, target string) bool {
	if !filepath.IsAbs(root) || !filepath.IsAbs(target) {
		return false
	}
	relative, err := filepath.Rel(filepath.Clean(root), filepath.Clean(target))
	if err != nil || relative == "." || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) || filepath.IsAbs(relative) {
		return false
	}
	return true
}

func pathWithinDirectoryOrEqual(root, target string) bool {
	if !filepath.IsAbs(root) || !filepath.IsAbs(target) {
		return false
	}
	relative, err := filepath.Rel(filepath.Clean(root), filepath.Clean(target))
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) || filepath.IsAbs(relative) {
		return false
	}
	return true
}

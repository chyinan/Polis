// pattern: Imperative Shell
package mcpowner

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"polis/internal/mcptransport"
	"polis/internal/runner"
)

const (
	maxPinnedFileBytes                int64 = 512 << 20
	maxPinnedPackageBytes             int64 = 1 << 30
	maxStderrBytes                    int64 = 1 << 20
	maxProcessStopWait                      = 5 * time.Second
	defaultGracefulStop                     = 3 * time.Second
	WindowsAppContainerDenyAllProfile       = "windows_appcontainer_deny_all@1"
)

var (
	ErrSchemaApprovalRequired = errors.New("stdio MCP call requires a persisted approved tool-schema digest")
	ErrOwnerStopped           = errors.New("stdio MCP process owner is stopped")
	ErrOwnerExited            = errors.New("stdio MCP process exited before the owner was stopped")
	ErrStderrLimitExceeded    = errors.New("stdio MCP process exceeded the bounded stderr limit")
)

type PinnedFile struct {
	Path   string
	SHA256 string
}

type ProcessSpec struct {
	Launch                      runner.AppContainerLaunchSpec
	PinnedPackageRoot           string
	EntryPoint                  string
	PinnedFiles                 []PinnedFile
	SourcePackageRevisionID     string
	SourcePackageManifestSHA256 string
	CommandSHA256               string
	PackageManifestSHA256       string
	ExpectedServer              mcptransport.StdioServerIdentity
	ApprovedToolSchemaSHA256    string
	GracefulStopTimeout         time.Duration
}

// RuntimeObservation is opaque outside mcpowner. Only a cleanly stopped
// process launched through public Start can produce a valid observation.
type RuntimeObservation struct {
	processSpec ProcessSpec
	server      mcptransport.StdioServerIdentity
	tools       []mcptransport.StdioToolDefinition
	digest      string
	hostOS      string
	hostProfile string
	sealed      bool
}

func (observation RuntimeObservation) Valid() bool {
	return observation.sealed && observation.digest != "" && len(observation.tools) > 0 && observation.hostOS == "windows" && observation.hostProfile == WindowsAppContainerDenyAllProfile
}

func (observation RuntimeObservation) ProcessSpec() ProcessSpec {
	if !observation.Valid() {
		return ProcessSpec{}
	}
	return cloneProcessSpec(observation.processSpec)
}

func (observation RuntimeObservation) ServerIdentity() mcptransport.StdioServerIdentity {
	if !observation.Valid() {
		return mcptransport.StdioServerIdentity{}
	}
	return observation.server
}

func (observation RuntimeObservation) Tools() []mcptransport.StdioToolDefinition {
	if !observation.Valid() {
		return nil
	}
	return cloneTools(observation.tools)
}

func (observation RuntimeObservation) ToolSchemaSHA256() string {
	if !observation.Valid() {
		return ""
	}
	return observation.digest
}

func (observation RuntimeObservation) Host() (string, string) {
	if !observation.Valid() {
		return "", ""
	}
	return observation.hostOS, observation.hostProfile
}

// ownedProcess.Stop must return nil only after the whole owned process tree is
// stopped. appContainerLauncher enforces that with runner.StopProof.
type ownedProcess interface {
	Stdin() io.WriteCloser
	Stdout() io.ReadCloser
	Stderr() io.ReadCloser
	Wait(context.Context) (int, error)
	Stop() error
}

type processLauncher interface {
	Launch(context.Context, ProcessSpec) (ownedProcess, error)
}

type appContainerLauncher struct {
	Sandbox *runner.AppContainerSandbox
}

func (launch appContainerLauncher) Launch(ctx context.Context, spec ProcessSpec) (ownedProcess, error) {
	if ctx == nil {
		return nil, errors.New("stdio MCP launch context is required")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	spec = cloneProcessSpec(spec)
	if launch.Sandbox == nil {
		return nil, runner.ErrAppContainerUnavailable
	}
	if err := validateProcessSpec(spec); err != nil {
		return nil, err
	}
	if err := runner.ValidateAppContainerLaunchSpec(spec.Launch); err != nil {
		return nil, err
	}
	leases, err := lockAndVerifyPinnedFiles(launch.Sandbox, spec)
	if err != nil {
		if len(leases) > 0 {
			return &appContainerProcess{id: spec.Launch.ID, leases: leases}, err
		}
		return nil, err
	}
	if err = ctx.Err(); err != nil {
		remaining, closeErr := closeLeases(leases)
		if len(remaining) > 0 {
			return &appContainerProcess{id: spec.Launch.ID, leases: remaining}, errors.Join(err, closeErr)
		}
		return nil, errors.Join(err, closeErr)
	}
	process, err := launch.Sandbox.Launch(spec.Launch)
	if err != nil {
		owner := &appContainerProcess{id: spec.Launch.ID, leases: leases}
		stopErr := owner.Stop()
		if stopErr != nil {
			return owner, errors.Join(err, stopErr)
		}
		return nil, err
	}
	if process == nil {
		owner := &appContainerProcess{id: spec.Launch.ID, leases: leases}
		stopErr := owner.Stop()
		cause := errors.New("AppContainer returned no MCP process")
		if stopErr != nil {
			return owner, errors.Join(cause, stopErr)
		}
		return nil, cause
	}
	return &appContainerProcess{process: process, id: spec.Launch.ID, leases: leases}, nil
}

type appContainerProcess struct {
	mu      sync.Mutex
	process runner.AppContainerProcess
	id      string
	leases  []io.Closer
	stopped bool
}

func (process *appContainerProcess) Stdin() io.WriteCloser {
	if process.process == nil {
		return nil
	}
	return process.process.Stdin()
}
func (process *appContainerProcess) Stdout() io.ReadCloser {
	if process.process == nil {
		return nil
	}
	return process.process.Stdout()
}
func (process *appContainerProcess) Stderr() io.ReadCloser {
	if process.process == nil {
		return nil
	}
	return process.process.Stderr()
}
func (process *appContainerProcess) Wait(ctx context.Context) (int, error) {
	if process.process == nil {
		return 0, nil
	}
	return process.process.Wait(ctx)
}

func (process *appContainerProcess) Stop() error {
	process.mu.Lock()
	defer process.mu.Unlock()
	if !process.stopped {
		if process.process != nil {
			proof, err := process.process.Stop()
			if err != nil {
				return err
			}
			if !proof.For(process.id) {
				return errors.New("stdio MCP AppContainer stop proof did not match its process owner")
			}
		}
		process.stopped = true
	}
	var err error
	process.leases, err = closeLeases(process.leases)
	return err
}

type Owner struct {
	mu                    sync.Mutex
	stopMu                sync.Mutex
	process               ownedProcess
	client                *mcptransport.StdioClient
	stream                *processStream
	stderr                io.ReadCloser
	processSpec           ProcessSpec
	tools                 []mcptransport.StdioToolDefinition
	digest                string
	server                mcptransport.StdioServerIdentity
	approvedDigest        string
	gracefulStop          time.Duration
	processDone           chan struct{}
	stderrOverflow        chan struct{}
	exitCode              int
	exitErr               error
	stderrErr             error
	hostOS                string
	hostProfile           string
	ready                 bool
	stopping              bool
	stopped               bool
	stopClean             bool
	startedInAppContainer bool
	toolCallStarted       bool
	unexpectedExit        bool
}

func Start(ctx context.Context, sandbox *runner.AppContainerSandbox, spec ProcessSpec) (*Owner, error) {
	if sandbox == nil {
		return nil, runner.ErrAppContainerUnavailable
	}
	owner, err := startWithLauncher(ctx, appContainerLauncher{Sandbox: sandbox}, spec)
	if owner != nil && err == nil {
		owner.mu.Lock()
		owner.startedInAppContainer = true
		owner.hostOS = "windows"
		owner.hostProfile = WindowsAppContainerDenyAllProfile
		owner.mu.Unlock()
	}
	return owner, err
}

func startWithLauncher(ctx context.Context, launcher processLauncher, spec ProcessSpec) (*Owner, error) {
	if ctx == nil {
		return nil, errors.New("stdio MCP start context is required")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if launcher == nil {
		return nil, errors.New("stdio MCP process launcher is required")
	}
	if err := validateProcessSpec(spec); err != nil {
		return nil, err
	}
	process, launchErr := launcher.Launch(ctx, cloneProcessSpec(spec))
	if launchErr != nil {
		if process == nil {
			return nil, launchErr
		}
		owner := newOwner(process, process.Stderr(), spec)
		owner.gracefulStop = time.Millisecond
		stopErr := owner.Stop(context.Background())
		if stopErr != nil {
			return owner, errors.Join(launchErr, stopErr)
		}
		return nil, launchErr
	}
	if process == nil {
		return nil, errors.New("stdio MCP launcher returned no process")
	}
	stdin, stdout, stderr := process.Stdin(), process.Stdout(), process.Stderr()
	owner := newOwner(process, stderr, spec)
	if stdin == nil || stdout == nil || stderr == nil {
		owner.gracefulStop = time.Millisecond
		incompleteErr := errors.New("stdio MCP launcher returned incomplete process streams")
		stopErr := owner.Stop(context.Background())
		if stopErr != nil {
			return owner, errors.Join(incompleteErr, stopErr)
		}
		return nil, incompleteErr
	}
	stream := &processStream{stdin: stdin, stdout: stdout}
	client, err := mcptransport.NewStdioClient(stream)
	if err != nil {
		owner.stream = stream
		stopErr := owner.Stop(context.Background())
		return ownerIfStopUnresolved(owner, errors.Join(err, stream.Close()), stopErr)
	}
	gracefulStop := spec.GracefulStopTimeout
	if gracefulStop == 0 {
		gracefulStop = defaultGracefulStop
	}
	owner.client, owner.stream, owner.gracefulStop = client, stream, gracefulStop
	select {
	case <-owner.processDone:
		_ = client.Close()
	default:
	}

	identity, err := client.Discover(ctx)
	if err == nil && (identity.Name != spec.ExpectedServer.Name || identity.Version != spec.ExpectedServer.Version) {
		err = errors.New("stdio MCP server identity differs from the pinned definition")
	}
	if err == nil {
		owner.server = identity
		owner.tools, owner.digest, err = client.ListTools(ctx)
	}
	if err == nil && spec.ApprovedToolSchemaSHA256 != "" && owner.digest != spec.ApprovedToolSchemaSHA256 {
		err = &mcptransport.ToolSchemaDriftError{ObservedDigest: owner.digest}
	}
	if err != nil {
		stopErr := owner.Stop(context.Background())
		if stopErr != nil {
			return owner, errors.Join(err, stopErr)
		}
		return nil, err
	}
	owner.mu.Lock()
	select {
	case <-owner.processDone:
		err = errors.Join(ErrOwnerExited, owner.exitErr)
	case <-owner.stderrOverflow:
		err = errors.Join(ErrStderrLimitExceeded, owner.stderrErr)
	default:
		if owner.stderrErr != nil {
			err = owner.stderrErr
		} else {
			owner.ready = true
		}
	}
	owner.mu.Unlock()
	if err != nil {
		stopErr := owner.Stop(context.Background())
		if stopErr != nil {
			return owner, errors.Join(err, stopErr)
		}
		return nil, err
	}
	go owner.watchStderrOverflow()
	return owner, nil
}

func (owner *Owner) Tools() []mcptransport.StdioToolDefinition {
	if owner == nil {
		return nil
	}
	owner.mu.Lock()
	defer owner.mu.Unlock()
	return cloneTools(owner.tools)
}

func (owner *Owner) ToolSchemaSHA256() string {
	if owner == nil {
		return ""
	}
	owner.mu.Lock()
	defer owner.mu.Unlock()
	return owner.digest
}

func (owner *Owner) RuntimeObservation() (RuntimeObservation, error) {
	if owner == nil {
		return RuntimeObservation{}, errors.New("stdio MCP owner is required")
	}
	owner.mu.Lock()
	defer owner.mu.Unlock()
	if !owner.startedInAppContainer || !owner.stopped || !owner.stopClean || owner.exitCode != 0 || owner.exitErr != nil || owner.toolCallStarted || owner.unexpectedExit || owner.stderrErr != nil || owner.digest == "" || len(owner.tools) == 0 {
		return RuntimeObservation{}, errors.New("stdio MCP runtime observation requires a clean, unused AppContainer process")
	}
	spec := cloneProcessSpec(owner.processSpec)
	spec.ApprovedToolSchemaSHA256 = owner.digest
	return RuntimeObservation{
		processSpec: spec, server: owner.server, tools: cloneTools(owner.tools), digest: owner.digest,
		hostOS: owner.hostOS, hostProfile: owner.hostProfile, sealed: true,
	}, nil
}

func (owner *Owner) CallTool(ctx context.Context, name string, arguments []byte) (mcptransport.StdioToolResult, error) {
	if owner == nil || ctx == nil {
		return mcptransport.StdioToolResult{}, errors.New("stdio MCP owner and call context are required")
	}
	owner.mu.Lock()
	ready := owner.ready && !owner.stopping && !owner.stopped
	digest := owner.approvedDigest
	stderrErr := owner.stderrErr
	owner.mu.Unlock()
	if !ready {
		select {
		case <-owner.processDone:
			return mcptransport.StdioToolResult{}, errors.Join(ErrOwnerExited, owner.exitErr)
		case <-owner.stderrOverflow:
			return mcptransport.StdioToolResult{}, errors.Join(ErrStderrLimitExceeded, stderrErr)
		default:
			if stderrErr != nil {
				return mcptransport.StdioToolResult{}, stderrErr
			}
			return mcptransport.StdioToolResult{}, ErrOwnerStopped
		}
	}
	if digest == "" {
		return mcptransport.StdioToolResult{}, ErrSchemaApprovalRequired
	}
	owner.mu.Lock()
	owner.toolCallStarted = true
	owner.mu.Unlock()
	result, err := owner.client.CallTool(ctx, digest, name, append([]byte(nil), arguments...))
	if errors.Is(err, mcptransport.ErrStdioToolSchemaChanged) {
		stopErr := owner.Stop(context.Background())
		return mcptransport.StdioToolResult{}, errors.Join(err, stopErr)
	}
	select {
	case <-owner.processDone:
		owner.mu.Lock()
		owner.ready = false
		owner.mu.Unlock()
		if err == nil {
			err = ErrOwnerExited
		}
		return mcptransport.StdioToolResult{}, errors.Join(err, owner.exitErr)
	default:
	}
	if err != nil {
		return mcptransport.StdioToolResult{}, err
	}
	return result, nil
}

func (owner *Owner) Stop(_ context.Context) error {
	if owner == nil {
		return nil
	}
	owner.stopMu.Lock()
	defer owner.stopMu.Unlock()
	owner.mu.Lock()
	if owner.stopped {
		owner.mu.Unlock()
		return nil
	}
	owner.ready = false
	owner.stopping = true
	stderrErr := owner.stderrErr
	owner.mu.Unlock()

	var closeErr error
	if owner.client != nil {
		closeErr = owner.client.Close()
	}
	gracefulContext, cancel := context.WithTimeout(context.Background(), owner.gracefulStop)
	if stderrErr == nil {
		select {
		case <-owner.processDone:
			cancel()
		case <-gracefulContext.Done():
		case <-owner.stderrOverflow:
		}
	}
	cancel()
	if stopErr := owner.process.Stop(); stopErr != nil {
		owner.retainAfterStopFailure()
		return errors.Join(closeErr, stderrErr, stopErr)
	}
	select {
	case <-owner.processDone:
	case <-time.After(maxProcessStopWait):
		owner.retainAfterStopFailure()
		return errors.Join(closeErr, errors.New("stdio MCP process wait did not complete after confirmed stop"))
	}
	var stderrCloseErr error
	if owner.stderr != nil {
		stderrCloseErr = owner.stderr.Close()
	}
	select {
	case <-owner.stderrOverflow:
	default:
	}
	owner.mu.Lock()
	owner.stopping = false
	owner.stopped = true
	stderrErr = owner.stderrErr
	owner.stopClean = closeErr == nil && stderrCloseErr == nil && stderrErr == nil
	owner.mu.Unlock()
	return errors.Join(closeErr, stderrCloseErr, stderrErr)
}

func newOwner(process ownedProcess, stderr io.ReadCloser, spec ProcessSpec) *Owner {
	gracefulStop := spec.GracefulStopTimeout
	if gracefulStop == 0 {
		gracefulStop = defaultGracefulStop
	}
	owner := &Owner{
		process: process, stderr: stderr, processSpec: cloneProcessSpec(spec),
		approvedDigest: spec.ApprovedToolSchemaSHA256,
		gracefulStop:   gracefulStop,
		processDone:    make(chan struct{}), stderrOverflow: make(chan struct{}, 1),
	}
	go owner.waitForProcess()
	if stderr != nil {
		go owner.drainStderr(stderr)
	}
	return owner
}

func ownerIfStopUnresolved(owner *Owner, cause, stopErr error) (*Owner, error) {
	if stopErr != nil {
		return owner, errors.Join(cause, stopErr)
	}
	return nil, cause
}

func (owner *Owner) retainAfterStopFailure() {
	owner.mu.Lock()
	owner.stopping = false
	owner.ready = false
	owner.mu.Unlock()
}

func (owner *Owner) waitForProcess() {
	code, err := owner.process.Wait(context.Background())
	owner.mu.Lock()
	owner.exitCode, owner.exitErr = code, err
	if !owner.stopping && owner.ready {
		owner.unexpectedExit = true
	}
	owner.ready = false
	owner.mu.Unlock()
	close(owner.processDone)
	if owner.client != nil {
		_ = owner.client.Close()
	}
}

func (owner *Owner) drainStderr(stderr io.Reader) {
	buffer := make([]byte, 4096)
	var count int64
	for {
		read, err := stderr.Read(buffer)
		count += int64(read)
		if count > maxStderrBytes {
			owner.mu.Lock()
			owner.stderrErr = ErrStderrLimitExceeded
			owner.ready = false
			owner.mu.Unlock()
			select {
			case owner.stderrOverflow <- struct{}{}:
			default:
			}
			return
		}
		if err != nil {
			if !errors.Is(err, io.EOF) {
				owner.mu.Lock()
				owner.stderrErr = err
				owner.ready = false
				owner.mu.Unlock()
			}
			return
		}
	}
}

func (owner *Owner) watchStderrOverflow() {
	select {
	case <-owner.stderrOverflow:
		stopErr := owner.Stop(context.Background())
		owner.mu.Lock()
		owner.stderrErr = errors.Join(ErrStderrLimitExceeded, stopErr)
		owner.mu.Unlock()
	case <-owner.processDone:
	}
}

type processStream struct {
	mu     sync.Mutex
	stdin  io.WriteCloser
	stdout io.ReadCloser
	closed bool
}

func (stream *processStream) Read(buffer []byte) (int, error)  { return stream.stdout.Read(buffer) }
func (stream *processStream) Write(buffer []byte) (int, error) { return stream.stdin.Write(buffer) }

func (stream *processStream) Close() error {
	stream.mu.Lock()
	defer stream.mu.Unlock()
	if stream.closed {
		return nil
	}
	stream.closed = true
	return errors.Join(stream.stdin.Close(), stream.stdout.Close())
}

type packageSnapshot struct {
	directories []string
	files       []string
	totalBytes  int64
}

func lockAndVerifyPinnedFiles(sandbox *runner.AppContainerSandbox, spec ProcessSpec) ([]io.Closer, error) {
	leases := make([]io.Closer, 0, len(spec.PinnedFiles)+16)
	closeOnError := func(cause error) ([]io.Closer, error) {
		remaining, closeErr := closeLeases(leases)
		return remaining, errors.Join(cause, closeErr)
	}
	rootLease, err := sandbox.LockReadOnlyPath(spec.PinnedPackageRoot)
	if err != nil {
		return nil, err
	}
	leases = append(leases, rootLease)
	firstSnapshot, err := scanPinnedPackage(spec.PinnedPackageRoot)
	if err != nil {
		return closeOnError(err)
	}
	if err = validatePackageFileSet(spec.PinnedPackageRoot, firstSnapshot, spec.PinnedFiles); err != nil {
		return closeOnError(err)
	}
	for _, directory := range firstSnapshot.directories {
		lease, lockErr := sandbox.LockReadOnlyPath(directory)
		if lockErr != nil {
			return closeOnError(lockErr)
		}
		leases = append(leases, lease)
	}
	secondSnapshot, err := scanPinnedPackage(spec.PinnedPackageRoot)
	if err != nil {
		return closeOnError(err)
	}
	if !samePackageSnapshot(firstSnapshot, secondSnapshot) {
		return closeOnError(errors.New("stdio MCP package contents changed while directory leases were acquired"))
	}
	files := append([]PinnedFile(nil), spec.PinnedFiles...)
	sort.Slice(files, func(left, right int) bool {
		return strings.ToLower(files[left].Path) < strings.ToLower(files[right].Path)
	})
	for _, file := range files {
		lease, lockErr := sandbox.LockReadOnlyPath(file.Path)
		if lockErr != nil {
			return closeOnError(lockErr)
		}
		leases = append(leases, lease)
		digest, hashErr := hashPinnedFile(file.Path)
		if hashErr != nil {
			return closeOnError(hashErr)
		}
		if digest != file.SHA256 {
			return closeOnError(fmt.Errorf("stdio MCP pinned file digest mismatch: %s", filepath.Base(file.Path)))
		}
	}
	manifestDigest, err := ComputePackageManifestSHA256(spec.PinnedPackageRoot, files)
	if err != nil || manifestDigest != spec.PackageManifestSHA256 {
		return closeOnError(errors.Join(errors.New("stdio MCP package manifest differs from its persisted digest"), err))
	}
	return leases, nil
}

func validateCompletePackageManifest(root string, pinned []PinnedFile) error {
	snapshot, err := scanPinnedPackage(root)
	if err != nil {
		return err
	}
	if err = validatePackageFileSet(root, snapshot, pinned); err != nil {
		return err
	}
	for _, file := range pinned {
		digest, hashErr := hashPinnedFile(file.Path)
		if hashErr != nil {
			return hashErr
		}
		if digest != file.SHA256 {
			return fmt.Errorf("stdio MCP package file differs from its pinned digest: %s", filepath.Base(file.Path))
		}
	}
	return nil
}

func scanPinnedPackage(root string) (packageSnapshot, error) {
	info, err := os.Lstat(root)
	if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return packageSnapshot{}, errors.Join(errors.New("stdio MCP package root must be a real directory"), err)
	}
	snapshot := packageSnapshot{}
	err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if filepath.Clean(path) == filepath.Clean(root) {
			return nil
		}
		entryInfo, infoErr := os.Lstat(path)
		if infoErr != nil {
			return infoErr
		}
		if entry.Type()&os.ModeSymlink != 0 || entryInfo.Mode()&os.ModeSymlink != 0 {
			return errors.New("stdio MCP package must not contain symbolic links")
		}
		if entryInfo.IsDir() {
			snapshot.directories = append(snapshot.directories, filepath.Clean(path))
			return nil
		}
		if !entryInfo.Mode().IsRegular() || entryInfo.Size() < 0 || entryInfo.Size() > maxPinnedFileBytes || snapshot.totalBytes > maxPinnedPackageBytes-entryInfo.Size() {
			return errors.New("stdio MCP package contains a non-regular or oversized file")
		}
		snapshot.totalBytes += entryInfo.Size()
		snapshot.files = append(snapshot.files, filepath.Clean(path))
		if len(snapshot.files) > maxPinnedFiles {
			return errors.New("stdio MCP package contains too many files")
		}
		return nil
	})
	if err != nil {
		return packageSnapshot{}, err
	}
	sort.Slice(snapshot.directories, func(left, right int) bool {
		return strings.ToLower(snapshot.directories[left]) < strings.ToLower(snapshot.directories[right])
	})
	sort.Slice(snapshot.files, func(left, right int) bool {
		return strings.ToLower(snapshot.files[left]) < strings.ToLower(snapshot.files[right])
	})
	if len(snapshot.files) == 0 {
		return packageSnapshot{}, errors.New("stdio MCP package must contain at least one file")
	}
	return snapshot, nil
}

func validatePackageFileSet(root string, snapshot packageSnapshot, pinned []PinnedFile) error {
	if len(snapshot.files) != len(pinned) || len(pinned) == 0 || len(pinned) > maxPinnedFiles {
		return errors.New("stdio MCP pinned file list does not contain the complete package")
	}
	expected := make(map[string]struct{}, len(pinned))
	for _, file := range pinned {
		if !filepath.IsAbs(file.Path) || !pathWithin(root, file.Path) || !validSHA256(file.SHA256) {
			return errors.New("stdio MCP pinned file is outside its package root or has an invalid digest")
		}
		key := strings.ToLower(filepath.Clean(file.Path))
		if _, duplicate := expected[key]; duplicate {
			return errors.New("stdio MCP pinned file list contains duplicate paths")
		}
		expected[key] = struct{}{}
	}
	for _, path := range snapshot.files {
		if _, exists := expected[strings.ToLower(filepath.Clean(path))]; !exists {
			return errors.New("stdio MCP package contains a file absent from its pinned manifest")
		}
	}
	return nil
}

func samePackageSnapshot(left, right packageSnapshot) bool {
	if len(left.directories) != len(right.directories) || len(left.files) != len(right.files) || left.totalBytes != right.totalBytes {
		return false
	}
	for index := range left.directories {
		if !strings.EqualFold(left.directories[index], right.directories[index]) {
			return false
		}
	}
	for index := range left.files {
		if !strings.EqualFold(left.files[index], right.files[index]) {
			return false
		}
	}
	return true
}

func hashPinnedFile(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() || info.Size() < 0 || info.Size() > maxPinnedFileBytes {
		return "", errors.New("stdio MCP pinned file is not a bounded regular file")
	}
	hasher := sha256.New()
	read, err := io.CopyN(hasher, file, maxPinnedFileBytes+1)
	if err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}
	if read != info.Size() {
		return "", errors.New("stdio MCP pinned file changed while it was hashed")
	}
	return hex.EncodeToString(hasher.Sum(nil)), nil
}

func closeLeases(leases []io.Closer) ([]io.Closer, error) {
	var errs []error
	var remaining []io.Closer
	for index := len(leases) - 1; index >= 0; index-- {
		if leases[index] != nil {
			if err := leases[index].Close(); err != nil {
				errs = append(errs, err)
				remaining = append(remaining, leases[index])
			}
		}
	}
	return remaining, errors.Join(errs...)
}

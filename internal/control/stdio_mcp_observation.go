// pattern: Imperative Shell
package control

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"polis/internal/capabilitysource"
	"polis/internal/core"
	"polis/internal/kernel"
	"polis/internal/mcpowner"
	"polis/internal/runner"
)

const maxRetainedMCPObservationPackages = 128

type pendingMCPRuntimeObservation struct {
	owner *mcpowner.Owner
	root  string
}

type mcpRuntimeObservationPackage struct {
	companyID       string
	serverID        string
	packageRevision string
	root            string
}

type appContainerStdioMCPRuntimeObserver struct {
	runtime      *kernel.Kernel
	sandbox      *runner.AppContainerSandbox
	systemRoot   string
	closeSandbox bool
	instanceID   string
	mu           sync.Mutex
	closed       bool
	packages     map[string]mcpRuntimeObservationPackage
	pending      []pendingMCPRuntimeObservation
}

func NewAppContainerStdioMCPRuntimeObserver(runtimeStore *kernel.Kernel, sandbox *runner.AppContainerSandbox, systemRoot string, closeSandbox bool) (StdioMCPRuntimeObserver, error) {
	if runtime.GOOS != "windows" {
		return nil, fmt.Errorf("stdio MCP runtime observation is unavailable on %s", runtime.GOOS)
	}
	if runtimeStore == nil || sandbox == nil || !filepath.IsAbs(sandbox.WorkspaceRoot()) || !filepath.IsAbs(systemRoot) {
		return nil, errors.New("stdio MCP runtime observation configuration is incomplete")
	}
	instanceID, _, err := newMCPRuntimeObservationIDs()
	if err != nil {
		return nil, err
	}
	if err = runtimeStore.TXReconcileStdioMCPRuntimeObservationIntents(context.Background(), instanceID); err != nil {
		return nil, fmt.Errorf("failed to reconcile interrupted MCP observation intents: %w", err)
	}
	if err = runtimeStore.TXRevokeStdioMCPRuntimeQualificationsOutsideSandbox(context.Background(), sandbox.WorkspaceRoot(), instanceID); err != nil {
		return nil, fmt.Errorf("failed to reconcile stale MCP AppContainer observations: %w", err)
	}
	return &appContainerStdioMCPRuntimeObserver{
		runtime: runtimeStore, sandbox: sandbox, systemRoot: filepath.Clean(systemRoot), closeSandbox: closeSandbox,
		instanceID: instanceID, packages: make(map[string]mcpRuntimeObservationPackage),
	}, nil
}

func (observer *appContainerStdioMCPRuntimeObserver) Observe(ctx context.Context, companyID string, request ObserveStdioMCPRuntimeRequest) (result kernel.StdioMCPRuntimeQualification, observeErr error) {
	if observer == nil || ctx == nil || !core.ValidID(companyID) || !core.ValidID(request.ServerID) || !core.ValidID(request.PackageRevisionID) || !core.ValidID(request.CapabilityQualificationID) || !core.ValidID(request.RequestID) {
		return kernel.StdioMCPRuntimeQualification{}, core.Malformed
	}
	if err := ctx.Err(); err != nil {
		return kernel.StdioMCPRuntimeQualification{}, err
	}
	observer.mu.Lock()
	defer observer.mu.Unlock()
	if observer.closed {
		return kernel.StdioMCPRuntimeQualification{}, core.Conflict
	}
	lockedContext, releaseServerLock, err := observer.runtime.LockStdioMCPPackageServer(ctx, companyID, request.ServerID)
	if err != nil {
		return kernel.StdioMCPRuntimeQualification{}, err
	}
	defer releaseServerLock()
	ctx = lockedContext
	catalog, err := observer.runtime.ListCapabilityCatalog(ctx, companyID)
	if err != nil {
		return kernel.StdioMCPRuntimeQualification{}, err
	}
	_, packageRevision, manifest, err := selectStdioMCPRuntimeObservation(catalog, request.ServerID, request.PackageRevisionID, request.CapabilityQualificationID)
	if err != nil {
		return kernel.StdioMCPRuntimeQualification{}, core.Denied
	}
	reservation, err := observer.runtime.TXReserveStdioMCPRuntimeObservation(ctx, companyID, kernel.StdioMCPRuntimeObservationReservationInput{
		CapabilityID: request.ServerID, CapabilityQualificationID: request.CapabilityQualificationID,
		PackageRevisionID: packageRevision.ID, PackageManifestSHA256: packageRevision.ManifestDigest, RequestID: request.RequestID,
	})
	if err != nil {
		return kernel.StdioMCPRuntimeQualification{}, err
	}
	if reservation.Existing != nil {
		return *reservation.Existing, nil
	}
	if !reservation.Start {
		return kernel.StdioMCPRuntimeQualification{}, core.Conflict
	}
	completed := false
	defer func() {
		if completed {
			return
		}
		unknownContext, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if markErr := observer.runtime.TXMarkStdioMCPRuntimeObservationUnknown(unknownContext, companyID, request.RequestID, "observation did not produce a stop-confirmed runtime record"); markErr != nil {
			observeErr = errors.Join(observeErr, markErr)
		}
	}()
	files, err := observer.runtime.ReadStdioMCPPackageFiles(ctx, companyID, request.ServerID, request.PackageRevisionID)
	if err != nil {
		return kernel.StdioMCPRuntimeQualification{}, err
	}
	if err = capabilitysource.VerifyStdioMCPBundle(manifest, packageRevision.ManifestDigest, files); err != nil {
		return kernel.StdioMCPRuntimeQualification{}, core.Integrity
	}
	packageKey := companyID + "\x00" + packageRevision.ID
	packageItem := observer.packages[packageKey]
	packageRoot := packageItem.root
	if packageRoot == "" {
		if len(observer.packages) >= maxRetainedMCPObservationPackages {
			return kernel.StdioMCPRuntimeQualification{}, core.TooLarge
		}
		_, randomDirectory, idErr := newMCPRuntimeObservationIDs()
		if idErr != nil {
			return kernel.StdioMCPRuntimeQualification{}, idErr
		}
		packageRoot = filepath.Join(observer.sandbox.WorkspaceRoot(), randomDirectory)
		if _, err = materializeMCPRuntimePackage(observer.sandbox.WorkspaceRoot(), packageRoot, files); err != nil {
			return kernel.StdioMCPRuntimeQualification{}, err
		}
		observer.packages[packageKey] = mcpRuntimeObservationPackage{companyID: companyID, serverID: request.ServerID, packageRevision: packageRevision.ID, root: packageRoot}
	}
	launchID, _, err := newMCPRuntimeObservationIDs()
	if err != nil {
		return kernel.StdioMCPRuntimeQualification{}, err
	}
	spec, err := buildStdioMCPRuntimeProcessSpec(observer.sandbox.WorkspaceRoot(), packageRoot, observer.systemRoot, launchID, packageRevision.ID, packageRevision.ManifestDigest, manifest, files)
	if err != nil {
		return kernel.StdioMCPRuntimeQualification{}, err
	}
	owner, startErr := mcpowner.Start(ctx, observer.sandbox, spec)
	if owner == nil {
		return kernel.StdioMCPRuntimeQualification{}, startErr
	}
	if startErr != nil {
		if stopErr := owner.Stop(context.Background()); stopErr != nil {
			observer.pending = append(observer.pending, pendingMCPRuntimeObservation{owner: owner, root: packageRoot})
			return kernel.StdioMCPRuntimeQualification{}, errors.Join(startErr, stopErr)
		}
		return kernel.StdioMCPRuntimeQualification{}, startErr
	}
	if err = owner.Stop(context.Background()); err != nil {
		observer.pending = append(observer.pending, pendingMCPRuntimeObservation{owner: owner, root: packageRoot})
		return kernel.StdioMCPRuntimeQualification{}, err
	}
	observation, err := owner.RuntimeObservation()
	if err != nil {
		return kernel.StdioMCPRuntimeQualification{}, err
	}
	result, err = observer.runtime.TXRecordStdioMCPRuntimeQualification(ctx, companyID, kernel.StdioMCPRuntimeObservationInput{
		CapabilityID: request.ServerID, CapabilityQualificationID: request.CapabilityQualificationID,
		Observation: observation, RequestID: request.RequestID,
	})
	if err != nil {
		return kernel.StdioMCPRuntimeQualification{}, err
	}
	completed = true
	return result, nil
}

func (observer *appContainerStdioMCPRuntimeObserver) Close() error {
	if observer == nil {
		return nil
	}
	observer.mu.Lock()
	defer observer.mu.Unlock()
	observer.closed = true
	var closeErrors []error
	for _, packageItem := range observer.packages {
		closeContext, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		err := observer.runtime.TXRevokeStdioMCPRuntimeQualificationsForPackageRevision(closeContext, packageItem.companyID, packageItem.serverID, packageItem.packageRevision, observer.instanceID)
		cancel()
		if err != nil {
			closeErrors = append(closeErrors, err)
		}
	}
	for index := range observer.pending {
		pending := observer.pending[index]
		if err := pending.owner.Stop(context.Background()); err != nil {
			closeErrors = append(closeErrors, err)
			continue
		}
		if err := removeMCPRuntimeObservationPackage(observer.sandbox.WorkspaceRoot(), pending.root); err != nil {
			closeErrors = append(closeErrors, err)
		}
		observer.pending[index] = pendingMCPRuntimeObservation{}
	}
	if len(closeErrors) == 0 {
		for _, packageItem := range observer.packages {
			if err := removeMCPRuntimeObservationPackage(observer.sandbox.WorkspaceRoot(), packageItem.root); err != nil {
				closeErrors = append(closeErrors, err)
			}
		}
		if len(closeErrors) == 0 {
			observer.packages = make(map[string]mcpRuntimeObservationPackage)
		}
	}
	if observer.closeSandbox && len(closeErrors) == 0 {
		if err := observer.sandbox.Close(); err != nil {
			closeErrors = append(closeErrors, err)
		}
	}
	return errors.Join(closeErrors...)
}

func newMCPRuntimeObservationIDs() (string, string, error) {
	var random [16]byte
	if _, err := rand.Read(random[:]); err != nil {
		return "", "", errors.New("could not create a unique MCP observation workspace")
	}
	value := hex.EncodeToString(random[:])
	return "mcp-observe-" + value, "mcp-observation-" + value, nil
}

func materializeMCPRuntimePackage(workspaceRoot, packageRoot string, files []capabilitysource.StdioMCPBundleFile) ([]mcpowner.PinnedFile, error) {
	if !filepath.IsAbs(workspaceRoot) || !filepath.IsAbs(packageRoot) || !pathWithinMCPRuntimeRoot(workspaceRoot, packageRoot) || len(files) == 0 {
		return nil, core.Malformed
	}
	if err := os.Mkdir(packageRoot, 0700); err != nil {
		return nil, err
	}
	pinnedFiles, err := mcpRuntimePinnedFiles(packageRoot, files)
	if err != nil {
		_ = removeMCPRuntimeObservationPackage(workspaceRoot, packageRoot)
		return nil, err
	}
	for index, file := range files {
		if err = writeMCPRuntimePackageFile(packageRoot, file); err != nil {
			_ = removeMCPRuntimeObservationPackage(workspaceRoot, packageRoot)
			return nil, err
		}
		pinnedFiles[index].SHA256 = file.ContentSHA256
	}
	return pinnedFiles, nil
}

func mcpRuntimePinnedFiles(packageRoot string, files []capabilitysource.StdioMCPBundleFile) ([]mcpowner.PinnedFile, error) {
	pinned := make([]mcpowner.PinnedFile, 0, len(files))
	for _, file := range files {
		if file.RelativePath == "" || filepath.IsAbs(filepath.FromSlash(file.RelativePath)) {
			return nil, core.Malformed
		}
		path := filepath.Clean(filepath.Join(packageRoot, filepath.FromSlash(file.RelativePath)))
		if !pathWithinMCPRuntimeRoot(packageRoot, path) {
			return nil, core.Malformed
		}
		pinned = append(pinned, mcpowner.PinnedFile{Path: path, SHA256: file.ContentSHA256})
	}
	return pinned, nil
}

func writeMCPRuntimePackageFile(packageRoot string, file capabilitysource.StdioMCPBundleFile) error {
	target := filepath.Clean(filepath.Join(packageRoot, filepath.FromSlash(file.RelativePath)))
	if !pathWithinMCPRuntimeRoot(packageRoot, target) || len(file.Content) != int(file.ByteSize) || digestMCPRuntimeBytes(file.Content) != file.ContentSHA256 {
		return core.Integrity
	}
	parent := filepath.Dir(target)
	if err := ensureMCPRuntimeDirectories(packageRoot, parent); err != nil {
		return err
	}
	output, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	_, writeErr := output.Write(file.Content)
	syncErr := output.Sync()
	closeErr := output.Close()
	if err = errors.Join(writeErr, syncErr, closeErr); err != nil {
		return err
	}
	return nil
}

func ensureMCPRuntimeDirectories(root, target string) error {
	if !pathWithinMCPRuntimeRoot(root, target) && filepath.Clean(root) != filepath.Clean(target) {
		return core.Malformed
	}
	relative, err := filepath.Rel(filepath.Clean(root), filepath.Clean(target))
	if err != nil {
		return err
	}
	current := filepath.Clean(root)
	if relative == "." {
		return nil
	}
	for _, component := range strings.Split(relative, string(filepath.Separator)) {
		current = filepath.Join(current, component)
		if err = os.Mkdir(current, 0700); errors.Is(err, os.ErrExist) {
			info, statErr := os.Lstat(current)
			if statErr != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
				return core.Integrity
			}
		} else if err != nil {
			return err
		}
	}
	return nil
}

func digestMCPRuntimeBytes(content []byte) string {
	digest := sha256.Sum256(content)
	return hex.EncodeToString(digest[:])
}

func removeMCPRuntimeObservationPackage(workspaceRoot, packageRoot string) error {
	if !pathWithinMCPRuntimeRoot(workspaceRoot, packageRoot) {
		return core.Denied
	}
	if err := os.RemoveAll(packageRoot); err != nil {
		return err
	}
	if _, err := os.Lstat(packageRoot); !errors.Is(err, os.ErrNotExist) {
		return errors.Join(err, errors.New("MCP observation package removal could not be confirmed"))
	}
	return nil
}

var _ StdioMCPRuntimeObserver = (*appContainerStdioMCPRuntimeObserver)(nil)

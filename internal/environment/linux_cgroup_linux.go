//go:build linux

// pattern: Imperative Shell
package environment

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
)

const linuxCgroupV2FilesystemMagic = 0x63677270
const linuxCgroupDirectoryPrefix = "polis-node-"
const linuxCgroupOwnerDirectoryPrefix = "polis-owner-"
const linuxCgroupPathFDFlag = 0x200000
const linuxNodeRuntimeIdentityFile = ".polis-linux-node-instance"

type linuxNodeCgroupV2Manager struct {
	runtimeRoot            string
	root                   string
	identity               string
	limits                 LinuxNodeResourceLimits
	workerHostID           string
	workerRootID           string
	workerBootID           string
	workerIdentityErr      error
	workerBubblewrapPath   string
	workerBubblewrapSHA256 string
	workerSandboxErr       error
	runtimeLease           *os.File
	rootLease              *os.File
	closeMu                sync.Mutex
}

type linuxNodeResourceCgroup struct {
	mu             sync.Mutex
	path           string
	name           string
	bubblewrapPath string
	hostID         string
	rootID         string
	bootID         string
	fd             int
	removed        bool
}

func NewLinuxNodeCgroupV2Manager(runtimeRoot, root, identity string, limits LinuxNodeResourceLimits, workerBubblewrapPaths ...string) (LinuxNodeCgroupManager, error) {
	if len(workerBubblewrapPaths) > 1 {
		return nil, fmt.Errorf("%w: only one Worker bubblewrap executable may be configured", ErrLinuxNodeCgroupUnavailable)
	}
	workerBubblewrapPath := ""
	if len(workerBubblewrapPaths) == 1 {
		workerBubblewrapPath = workerBubblewrapPaths[0]
	}
	if err := limits.Validate(); err != nil {
		return nil, err
	}
	runtimeRoot = filepath.Clean(runtimeRoot)
	root = filepath.Clean(root)
	if !validLinuxNodeInstanceIdentity(identity) {
		return nil, fmt.Errorf("%w: Linux Node runtime identity is invalid", ErrLinuxNodeCgroupUnavailable)
	}
	if err := validateLinuxNodeCgroupRuntimeRoot(runtimeRoot, root); err != nil {
		return nil, err
	}
	if err := validateLinuxNodeCgroupRoot(root, limits); err != nil {
		return nil, err
	}
	runtimeLease, err := acquireLinuxNodeDirectoryLease(runtimeRoot, "Linux Node runtime root")
	if err != nil {
		return nil, err
	}
	if err = bindLinuxNodeRuntimeIdentity(runtimeRoot, identity); err != nil {
		_ = syscall.Flock(int(runtimeLease.Fd()), syscall.LOCK_UN)
		_ = runtimeLease.Close()
		return nil, err
	}
	rootLease, err := acquireLinuxNodeCgroupRootLease(root)
	if err != nil {
		_ = syscall.Flock(int(runtimeLease.Fd()), syscall.LOCK_UN)
		_ = runtimeLease.Close()
		return nil, err
	}
	hostID, rootID, bootID, workerIdentityErr := currentLinuxWorkerCgroupIdentity(runtimeRoot, root, identity)
	workerBubblewrapSHA256 := ""
	var workerSandboxErr error
	if workerBubblewrapPath == "" {
		workerSandboxErr = fmt.Errorf("%w: Linux Worker bubblewrap executable is not configured", ErrLinuxNodeCgroupUnavailable)
	} else if !linuxRegularExecutable(workerBubblewrapPath) {
		workerSandboxErr = fmt.Errorf("%w: Linux Worker bubblewrap executable is invalid", ErrLinuxNodeCgroupUnavailable)
	} else {
		var contentDigest, mode, owner string
		var ownerKnown bool
		contentDigest, mode, owner, ownerKnown, workerSandboxErr = hashLinuxRegularToolIdentity(workerBubblewrapPath, maxLinuxNodeToolExecutableSize)
		if workerSandboxErr == nil {
			workerBubblewrapSHA256, workerSandboxErr = linuxWorkerBubblewrapFingerprint(contentDigest, mode, owner, ownerKnown)
		}
	}
	if workerSandboxErr == nil {
		rootID, workerSandboxErr = linuxWorkerCgroupSandboxRootIdentity(rootID, workerBubblewrapPath, workerBubblewrapSHA256)
	}
	manager := &linuxNodeCgroupV2Manager{
		runtimeRoot: runtimeRoot, root: root, identity: identity, limits: limits,
		workerHostID: hostID, workerRootID: rootID, workerBootID: bootID, workerIdentityErr: workerIdentityErr,
		workerBubblewrapPath: workerBubblewrapPath, workerBubblewrapSHA256: workerBubblewrapSHA256, workerSandboxErr: workerSandboxErr,
		runtimeLease: runtimeLease, rootLease: rootLease,
	}
	if err := manager.ReconcileUnrestored(); err != nil {
		_ = manager.releaseRootLease()
		return nil, err
	}
	return manager, nil
}

func currentLinuxWorkerCgroupIdentity(runtimeRoot, cgroupRoot, instanceID string) (string, string, string, error) {
	readIdentity := func(path string) (string, error) {
		contents, err := os.ReadFile(path)
		if err != nil || len(contents) > 128 {
			return "", fmt.Errorf("%w: Linux Worker cgroup identity source is unavailable", ErrLinuxNodeCgroupUnavailable)
		}
		return strings.TrimSpace(string(contents)), nil
	}
	machineID, err := readIdentity("/etc/machine-id")
	if err != nil {
		return "", "", "", err
	}
	productID, err := readIdentity("/sys/class/dmi/id/product_uuid")
	if err != nil {
		return "", "", "", err
	}
	bootID, err := readIdentity("/proc/sys/kernel/random/boot_id")
	if err != nil {
		return "", "", "", err
	}
	rootInfo, err := os.Stat(cgroupRoot)
	if err != nil {
		return "", "", "", fmt.Errorf("%w: Worker cgroup root identity is unavailable", ErrLinuxNodeCgroupUnavailable)
	}
	rootStat, ok := rootInfo.Sys().(*syscall.Stat_t)
	if !ok {
		return "", "", "", fmt.Errorf("%w: Worker cgroup root identity is unavailable", ErrLinuxNodeCgroupUnavailable)
	}
	hostID, rootID, err := linuxWorkerCgroupIdentities(machineID, productID, bootID, runtimeRoot, cgroupRoot, instanceID, uint64(rootStat.Dev), rootStat.Ino)
	if err != nil {
		return "", "", "", err
	}
	return hostID, rootID, bootID, nil
}

func validLinuxNodeInstanceIdentity(identity string) bool {
	if len(identity) != 64 {
		return false
	}
	for _, character := range identity {
		if !((character >= '0' && character <= '9') || (character >= 'a' && character <= 'f')) {
			return false
		}
	}
	return true
}

func bindLinuxNodeRuntimeIdentity(runtimeRoot, identity string) error {
	if !validLinuxNodeInstanceIdentity(identity) || !filepath.IsAbs(runtimeRoot) || !linuxNoLinkAncestors(runtimeRoot) {
		return fmt.Errorf("%w: Linux Node runtime identity binding is invalid", ErrLinuxNodeCgroupUnavailable)
	}
	filePath := filepath.Join(runtimeRoot, linuxNodeRuntimeIdentityFile)
	file, err := os.OpenFile(filePath, os.O_RDWR|os.O_CREATE|os.O_EXCL|syscall.O_CLOEXEC|syscall.O_NOFOLLOW, 0600)
	created := err == nil
	if errors.Is(err, os.ErrExist) {
		file, err = os.OpenFile(filePath, os.O_RDWR|syscall.O_CLOEXEC|syscall.O_NOFOLLOW, 0600)
	}
	if err != nil {
		return fmt.Errorf("%w: Linux Node runtime identity marker cannot be opened", ErrLinuxNodeCgroupUnavailable)
	}
	defer file.Close()
	if err = file.Chmod(0600); err != nil {
		return fmt.Errorf("%w: Linux Node runtime identity marker permissions cannot be secured", ErrLinuxNodeCgroupUnavailable)
	}
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm() != 0600 {
		return fmt.Errorf("%w: Linux Node runtime identity marker is not a private regular file", ErrLinuxNodeCgroupUnavailable)
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || stat.Uid != uint32(os.Geteuid()) || stat.Nlink != 1 {
		return fmt.Errorf("%w: Linux Node runtime identity marker has an unexpected owner or link count", ErrLinuxNodeCgroupUnavailable)
	}
	if _, err = file.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("%w: Linux Node runtime identity marker cannot be read", ErrLinuxNodeCgroupUnavailable)
	}
	contents, err := io.ReadAll(file)
	if err != nil {
		return fmt.Errorf("%w: Linux Node runtime identity marker cannot be read", ErrLinuxNodeCgroupUnavailable)
	}
	want := identity + "\n"
	if len(contents) == 0 {
		if !created {
			return fmt.Errorf("%w: Linux Node runtime identity marker is empty after an interrupted initialization", ErrLinuxNodeCgroupUnavailable)
		}
		if _, err = file.WriteString(want); err != nil {
			return fmt.Errorf("%w: Linux Node runtime identity marker cannot be written", ErrLinuxNodeCgroupUnavailable)
		}
		if err = file.Sync(); err != nil {
			return fmt.Errorf("%w: Linux Node runtime identity marker cannot be synced", ErrLinuxNodeCgroupUnavailable)
		}
		runtimeDir, openErr := os.Open(runtimeRoot)
		if openErr != nil {
			return fmt.Errorf("%w: Linux Node runtime root cannot be opened to sync its identity marker", ErrLinuxNodeCgroupUnavailable)
		}
		syncErr := runtimeDir.Sync()
		closeErr := runtimeDir.Close()
		if err = errors.Join(syncErr, closeErr); err != nil {
			return fmt.Errorf("%w: Linux Node runtime identity marker directory entry cannot be synced", ErrLinuxNodeCgroupUnavailable)
		}
		return nil
	}
	if string(contents) != want {
		return fmt.Errorf("%w: Polis runtime root is already bound to a different database and blob-store instance", ErrLinuxNodeCgroupUnavailable)
	}
	return nil
}

func validateLinuxNodeCgroupRuntimeRoot(runtimeRoot, cgroupRoot string) error {
	if !filepath.IsAbs(runtimeRoot) || !filepath.IsAbs(cgroupRoot) || runtimeRoot == string(filepath.Separator) || cgroupRoot == string(filepath.Separator) || filepath.Clean(runtimeRoot) != runtimeRoot || filepath.Clean(cgroupRoot) != cgroupRoot || !linuxNoLinkAncestors(runtimeRoot) || !linuxNoLinkAncestors(cgroupRoot) {
		return fmt.Errorf("%w: Linux Node runtime and cgroup roots must be canonical directories", ErrLinuxNodeCgroupUnavailable)
	}
	info, err := os.Lstat(runtimeRoot)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("%w: Linux Node runtime root is missing or linked", ErrLinuxNodeCgroupUnavailable)
	}
	runtimeStat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || runtimeStat.Uid != uint32(os.Geteuid()) || info.Mode().Perm()&0022 != 0 {
		uid := uint32(^uint32(0))
		if ok {
			uid = runtimeStat.Uid
		}
		return fmt.Errorf("%w: Linux Node runtime root must be owned by the service user and not writable by other users (owner=%d service=%d mode=%04o)", ErrLinuxNodeCgroupUnavailable, uid, os.Geteuid(), info.Mode().Perm())
	}
	relative, err := filepath.Rel(runtimeRoot, cgroupRoot)
	if err != nil || relative == "." || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) || filepath.IsAbs(relative) {
		return fmt.Errorf("%w: delegated cgroup root must be a dedicated child of the Polis runtime root", ErrLinuxNodeCgroupUnavailable)
	}
	return nil
}

func acquireLinuxNodeCgroupRootLease(root string) (*os.File, error) {
	return acquireLinuxNodeDirectoryLease(root, "delegated cgroup root")
}

func acquireLinuxNodeDirectoryLease(root, description string) (*os.File, error) {
	pathInfo, err := os.Lstat(root)
	if err != nil || !pathInfo.IsDir() || pathInfo.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("%w: %s cannot be leased", ErrLinuxNodeCgroupUnavailable, description)
	}
	lease, err := os.OpenFile(root, os.O_RDONLY|syscall.O_DIRECTORY|syscall.O_CLOEXEC|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return nil, fmt.Errorf("%w: %s cannot be opened for an instance lease", ErrLinuxNodeCgroupUnavailable, description)
	}
	openedInfo, err := lease.Stat()
	if err != nil || !os.SameFile(pathInfo, openedInfo) {
		_ = lease.Close()
		return nil, fmt.Errorf("%w: %s changed while acquiring its instance lease", ErrLinuxNodeCgroupUnavailable, description)
	}
	if err = syscall.Flock(int(lease.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		_ = lease.Close()
		return nil, fmt.Errorf("%w: %s is already leased by another Polis instance", ErrLinuxNodeCgroupUnavailable, description)
	}
	return lease, nil
}

func (manager *linuxNodeCgroupV2Manager) RuntimeRootPath() string {
	if manager == nil {
		return ""
	}
	return manager.runtimeRoot
}

func (manager *linuxNodeCgroupV2Manager) CgroupRootPath() string {
	if manager == nil {
		return ""
	}
	return manager.root
}

func (manager *linuxNodeCgroupV2Manager) InstanceIdentity() string {
	if manager == nil {
		return ""
	}
	return manager.identity
}

func (manager *linuxNodeCgroupV2Manager) WorkerCgroupHostIdentity() string {
	if manager == nil {
		return ""
	}
	return manager.workerHostID
}

func (manager *linuxNodeCgroupV2Manager) WorkerCgroupRootIdentity() string {
	if manager == nil {
		return ""
	}
	return manager.workerRootID
}

func (manager *linuxNodeCgroupV2Manager) WorkerCgroupBootID() string {
	if manager == nil {
		return ""
	}
	return manager.workerBootID
}

func (manager *linuxNodeCgroupV2Manager) WorkerContainmentReady() error {
	if manager == nil {
		return ErrLinuxNodeCgroupUnavailable
	}
	if manager.workerIdentityErr != nil || manager.workerSandboxErr != nil || !validLinuxWorkerCgroupHostID(manager.workerHostID) || !validLinuxWorkerCgroupHostID(manager.workerRootID) || !validLinuxUUID(manager.workerBootID) {
		return fmt.Errorf("%w: Linux Worker cgroup host/root identity is unavailable", ErrLinuxNodeCgroupUnavailable)
	}
	if !linuxRegularExecutable(manager.workerBubblewrapPath) {
		return fmt.Errorf("%w: Linux Worker bubblewrap executable is unavailable", ErrLinuxNodeCgroupUnavailable)
	}
	contentDigest, mode, owner, ownerKnown, err := hashLinuxRegularToolIdentity(manager.workerBubblewrapPath, maxLinuxNodeToolExecutableSize)
	bubblewrapSHA256, fingerprintErr := linuxWorkerBubblewrapFingerprint(contentDigest, mode, owner, ownerKnown)
	if err != nil || fingerprintErr != nil || bubblewrapSHA256 != manager.workerBubblewrapSHA256 {
		return fmt.Errorf("%w: Linux Worker bubblewrap executable changed", ErrLinuxNodeCgroupUnavailable)
	}
	if os.Geteuid() == 0 {
		return fmt.Errorf("%w: Linux real Worker containment requires a non-root Polis service identity", ErrLinuxNodeCgroupUnavailable)
	}
	return nil
}

func (manager *linuxNodeCgroupV2Manager) Close() error {
	if manager == nil {
		return nil
	}
	manager.closeMu.Lock()
	defer manager.closeMu.Unlock()
	if manager.rootLease == nil {
		return nil
	}
	if err := validateLinuxNodeCgroupRoot(manager.root, manager.limits); err != nil {
		return err
	}
	if err := requireLinuxNodeCgroupRootOwnerOnly(manager.root, manager.identity); err != nil {
		return err
	}
	return manager.releaseRootLease()
}

func (manager *linuxNodeCgroupV2Manager) releaseRootLease() error {
	var closeErrors []error
	for _, lease := range []*os.File{manager.rootLease, manager.runtimeLease} {
		if lease == nil {
			continue
		}
		if lease == manager.rootLease {
			manager.rootLease = nil
		} else {
			manager.runtimeLease = nil
		}
		closeErrors = append(closeErrors, syscall.Flock(int(lease.Fd()), syscall.LOCK_UN), lease.Close())
	}
	return errors.Join(closeErrors...)
}

func validateLinuxNodeCgroupRoot(root string, limits LinuxNodeResourceLimits) error {
	info, err := os.Lstat(root)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("%w: delegated cgroup root is missing or linked", ErrLinuxNodeCgroupUnavailable)
	}
	rootStat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || rootStat.Uid != uint32(os.Geteuid()) || info.Mode().Perm()&0022 != 0 {
		return fmt.Errorf("%w: delegated cgroup root must be owned by the service user and not writable by other users", ErrLinuxNodeCgroupUnavailable)
	}
	var filesystem syscall.Statfs_t
	if err = syscall.Statfs(root, &filesystem); err != nil || uint64(filesystem.Type) != linuxCgroupV2FilesystemMagic {
		return fmt.Errorf("%w: resource root is not a cgroup v2 filesystem", ErrLinuxNodeCgroupUnavailable)
	}
	controllers, err := os.ReadFile(filepath.Join(root, "cgroup.controllers"))
	if err != nil || !hasLinuxCgroupControllers(string(controllers), "cpu", "memory", "pids") {
		return fmt.Errorf("%w: cpu, memory, and pids controllers are not available", ErrLinuxNodeCgroupUnavailable)
	}
	enabled, err := os.ReadFile(filepath.Join(root, "cgroup.subtree_control"))
	if err != nil || !hasLinuxCgroupControllers(string(enabled), "cpu", "memory", "pids") {
		return fmt.Errorf("%w: cpu, memory, and pids controllers are not delegated to the resource root", ErrLinuxNodeCgroupUnavailable)
	}
	processes, err := os.ReadFile(filepath.Join(root, "cgroup.procs"))
	if err != nil || strings.TrimSpace(string(processes)) != "" {
		return fmt.Errorf("%w: delegated cgroup root must not contain processes", ErrLinuxNodeCgroupUnavailable)
	}
	parentLimits := make([]string, 6)
	for index, name := range []string{"cpu.max", "memory.max", "memory.swap.max", "pids.max", "memory.oom.group", "cgroup.max.descendants"} {
		value, readErr := os.ReadFile(filepath.Join(root, name))
		if readErr != nil {
			return fmt.Errorf("%w: aggregate cgroup control %s is unavailable", ErrLinuxNodeCgroupUnavailable, name)
		}
		parentLimits[index] = strings.TrimSpace(string(value))
	}
	if err = ValidateLinuxNodeParentCgroupLimits(parentLimits[0], parentLimits[1], parentLimits[2], parentLimits[3], parentLimits[4], parentLimits[5], limits); err != nil {
		return err
	}
	return nil
}

func hasLinuxCgroupControllers(contents string, required ...string) bool {
	available := make(map[string]struct{})
	for _, controller := range strings.Fields(contents) {
		available[strings.TrimPrefix(controller, "+")] = struct{}{}
	}
	for _, controller := range required {
		if _, ok := available[controller]; !ok {
			return false
		}
	}
	return true
}

func requireEmptyLinuxNodeCgroupRoot(root string) error {
	entries, err := os.ReadDir(root)
	if err != nil {
		return fmt.Errorf("%w: delegated cgroup root cannot be read", ErrLinuxNodeCgroupUnavailable)
	}
	for _, entry := range entries {
		if entry.IsDir() {
			return fmt.Errorf("%w: delegated cgroup root must be empty; existing child %q needs operator review", ErrLinuxNodeCgroupUnavailable, entry.Name())
		}
	}
	return nil
}

func linuxNodeCgroupChildDirectories(root string) ([]string, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, fmt.Errorf("%w: delegated cgroup root cannot be read", ErrLinuxNodeCgroupUnavailable)
	}
	children := make([]string, 0)
	for _, entry := range entries {
		path := filepath.Join(root, entry.Name())
		info, statErr := os.Lstat(path)
		if statErr != nil {
			return nil, fmt.Errorf("%w: delegated cgroup child cannot be inspected", ErrLinuxNodeCgroupUnavailable)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return nil, fmt.Errorf("%w: linked delegated cgroup child is not owned", ErrLinuxNodeCgroupUnavailable)
		}
		if !info.IsDir() {
			continue
		}
		if validLinuxNodeCgroupOwnerDirectoryName(entry.Name()) {
			continue
		}
		if validLinuxWorkerCgroupDirectoryName(entry.Name()) {
			continue
		}
		if !validLinuxNodeCgroupDirectoryName(entry.Name()) {
			return nil, fmt.Errorf("%w: existing child %q is outside the Polis cgroup namespace", ErrLinuxNodeCgroupUnavailable, entry.Name())
		}
		children = append(children, path)
	}
	return children, nil
}

func linuxNodeCgroupOwnerDirectoryName(identity string) string {
	return linuxCgroupOwnerDirectoryPrefix + identity
}

func validLinuxNodeCgroupOwnerDirectoryName(name string) bool {
	if !strings.HasPrefix(name, linuxCgroupOwnerDirectoryPrefix) {
		return false
	}
	return validLinuxNodeInstanceIdentity(strings.TrimPrefix(name, linuxCgroupOwnerDirectoryPrefix))
}

func ensureLinuxNodeCgroupOwner(root, identity string) error {
	if !validLinuxNodeInstanceIdentity(identity) {
		return fmt.Errorf("%w: Linux Node cgroup owner identity is invalid", ErrLinuxNodeCgroupUnavailable)
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return fmt.Errorf("%w: delegated cgroup root cannot be read", ErrLinuxNodeCgroupUnavailable)
	}
	want := linuxNodeCgroupOwnerDirectoryName(identity)
	ownerFound := false
	workGroupsFound := false
	for _, entry := range entries {
		path := filepath.Join(root, entry.Name())
		info, statErr := os.Lstat(path)
		if statErr != nil || info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("%w: delegated cgroup child is linked or cannot be inspected", ErrLinuxNodeCgroupUnavailable)
		}
		if !info.IsDir() {
			continue
		}
		if validLinuxNodeCgroupDirectoryName(entry.Name()) || validLinuxWorkerCgroupDirectoryName(entry.Name()) {
			workGroupsFound = true
			continue
		}
		if validLinuxNodeCgroupOwnerDirectoryName(entry.Name()) {
			if entry.Name() != want || ownerFound {
				return fmt.Errorf("%w: delegated cgroup root belongs to a different or ambiguous Polis instance", ErrLinuxNodeCgroupUnavailable)
			}
			ownerFound = true
			continue
		}
		return fmt.Errorf("%w: existing cgroup child %q is outside the Polis namespace", ErrLinuxNodeCgroupUnavailable, entry.Name())
	}
	if ownerFound {
		return nil
	}
	if workGroupsFound {
		return fmt.Errorf("%w: existing process groups have no durable instance owner marker", ErrLinuxNodeCgroupUnavailable)
	}
	ownerPath := filepath.Join(root, want)
	if err = os.Mkdir(ownerPath, 0700); err != nil {
		return fmt.Errorf("%w: delegated cgroup root owner marker could not be created", ErrLinuxNodeCgroupUnavailable)
	}
	info, err := os.Lstat(ownerPath)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("%w: delegated cgroup root owner marker could not be confirmed", ErrLinuxNodeCgroupUnavailable)
	}
	return nil
}

func requireLinuxNodeCgroupRootOwnerOnly(root, identity string) error {
	entries, err := os.ReadDir(root)
	if err != nil {
		return fmt.Errorf("%w: delegated cgroup root cannot be read", ErrLinuxNodeCgroupUnavailable)
	}
	want := linuxNodeCgroupOwnerDirectoryName(identity)
	ownerFound := false
	for _, entry := range entries {
		path := filepath.Join(root, entry.Name())
		info, statErr := os.Lstat(path)
		if statErr != nil || info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("%w: delegated cgroup owner marker changed during cleanup", ErrLinuxNodeCgroupUnavailable)
		}
		if !info.IsDir() {
			continue
		}
		if entry.Name() != want || ownerFound {
			return fmt.Errorf("%w: delegated cgroup root still contains a process group or foreign child", ErrLinuxNodeCgroupUnavailable)
		}
		ownerFound = true
	}
	if !ownerFound {
		return fmt.Errorf("%w: delegated cgroup owner marker is missing", ErrLinuxNodeCgroupUnavailable)
	}
	return validateLinuxNodeCgroupOwnerEmpty(root, identity)
}

func requireLinuxNodeCgroupRootOwnerAndWorkerGroups(root, identity string) error {
	entries, err := os.ReadDir(root)
	if err != nil {
		return fmt.Errorf("%w: delegated cgroup root cannot be read", ErrLinuxNodeCgroupUnavailable)
	}
	want := linuxNodeCgroupOwnerDirectoryName(identity)
	ownerFound := false
	for _, entry := range entries {
		path := filepath.Join(root, entry.Name())
		info, statErr := os.Lstat(path)
		if statErr != nil || info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("%w: delegated cgroup child is linked or cannot be inspected", ErrLinuxNodeCgroupUnavailable)
		}
		if !info.IsDir() {
			continue
		}
		if validLinuxWorkerCgroupDirectoryName(entry.Name()) {
			continue
		}
		if entry.Name() != want || ownerFound {
			return fmt.Errorf("%w: delegated cgroup root contains a node job, unknown child, or foreign owner", ErrLinuxNodeCgroupUnavailable)
		}
		ownerFound = true
	}
	if !ownerFound {
		return fmt.Errorf("%w: delegated cgroup owner marker is missing", ErrLinuxNodeCgroupUnavailable)
	}
	return validateLinuxNodeCgroupOwnerEmpty(root, identity)
}

func validateLinuxNodeCgroupOwnerEmpty(root, identity string) error {
	if !validLinuxNodeInstanceIdentity(identity) {
		return fmt.Errorf("%w: Linux Node cgroup owner identity is invalid", ErrLinuxNodeCgroupUnavailable)
	}
	ownerPath := filepath.Join(root, linuxNodeCgroupOwnerDirectoryName(identity))
	info, err := os.Lstat(ownerPath)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("%w: delegated cgroup owner marker is missing or linked", ErrLinuxNodeCgroupUnavailable)
	}
	processes, err := os.ReadFile(filepath.Join(ownerPath, "cgroup.procs"))
	if err != nil || strings.TrimSpace(string(processes)) != "" {
		return fmt.Errorf("%w: delegated cgroup owner marker contains a process or cannot be verified", ErrLinuxNodeCgroupUnavailable)
	}
	return nil
}

func validLinuxNodeCgroupDirectoryName(name string) bool {
	if !strings.HasPrefix(name, linuxCgroupDirectoryPrefix) {
		return false
	}
	suffix := strings.TrimPrefix(name, linuxCgroupDirectoryPrefix)
	if len(suffix) != 24 {
		return false
	}
	for _, character := range suffix {
		if !((character >= '0' && character <= '9') || (character >= 'a' && character <= 'f')) {
			return false
		}
	}
	return true
}

func cleanupLinuxNodeCgroupChildren(root string, cleanup func(string) error) error {
	if cleanup == nil {
		return fmt.Errorf("%w: delegated cgroup cleanup is unavailable", ErrLinuxNodeCgroupCleanupUnconfirmed)
	}
	children, err := linuxNodeCgroupChildDirectories(root)
	if err != nil {
		return err
	}
	for _, path := range children {
		if err = cleanup(path); err != nil {
			return fmt.Errorf("%w: an owned child cgroup could not be emptied and removed", ErrLinuxNodeCgroupCleanupUnconfirmed)
		}
	}
	return nil
}

// ReconcileUnrestored removes Node JobRun groups left by a previous Polis
// process and preserves named Worker groups for database-bound reconciliation.
// Unknown names fail closed before any group is changed.
func (manager *linuxNodeCgroupV2Manager) ReconcileUnrestored() error {
	if manager == nil {
		return ErrLinuxNodeCgroupUnavailable
	}
	if err := validateLinuxNodeCgroupRoot(manager.root, manager.limits); err != nil {
		return err
	}
	if err := ensureLinuxNodeCgroupOwner(manager.root, manager.identity); err != nil {
		return err
	}
	if err := validateLinuxNodeCgroupOwnerEmpty(manager.root, manager.identity); err != nil {
		return err
	}
	if err := cleanupLinuxNodeCgroupChildren(manager.root, func(path string) error {
		return (&linuxNodeResourceCgroup{path: path, fd: -1}).Cleanup()
	}); err != nil {
		return err
	}
	return requireLinuxNodeCgroupRootOwnerAndWorkerGroups(manager.root, manager.identity)
}

func (manager *linuxNodeCgroupV2Manager) Create(identity string) (LinuxNodeResourceCgroup, error) {
	if manager == nil || identity == "" || len(identity) > 256 || strings.ContainsRune(identity, '\x00') {
		return nil, fmt.Errorf("%w: resource cgroup identity is invalid", ErrLinuxNodeCgroupUnavailable)
	}
	var random [12]byte
	if _, err := rand.Read(random[:]); err != nil {
		return nil, fmt.Errorf("%w: resource cgroup identity could not be generated", ErrLinuxNodeCgroupUnavailable)
	}
	return manager.createGroup(linuxCgroupDirectoryPrefix + hex.EncodeToString(random[:]))
}

func (manager *linuxNodeCgroupV2Manager) CreateWorker(sessionID string) (LinuxWorkerResourceCgroup, error) {
	if err := manager.WorkerContainmentReady(); err != nil {
		return nil, err
	}
	name, err := LinuxWorkerCgroupName(sessionID)
	if err != nil {
		return nil, err
	}
	return manager.createGroup(name)
}

func (manager *linuxNodeCgroupV2Manager) ReconcileWorker(sessionID string) (LinuxWorkerCgroupStopProof, error) {
	if err := manager.WorkerContainmentReady(); err != nil {
		return LinuxWorkerCgroupStopProof{}, err
	}
	name, err := LinuxWorkerCgroupName(sessionID)
	if err != nil {
		return LinuxWorkerCgroupStopProof{}, err
	}
	if manager == nil || !validLinuxWorkerCgroupDirectoryName(name) {
		return LinuxWorkerCgroupStopProof{}, ErrLinuxNodeCgroupUnavailable
	}
	if err := validateLinuxNodeCgroupRoot(manager.root, manager.limits); err != nil {
		return LinuxWorkerCgroupStopProof{}, err
	}
	path := filepath.Join(manager.root, name)
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return LinuxWorkerCgroupStopProof{sessionID: sessionID, name: name, hostID: manager.workerHostID, rootID: manager.workerRootID, bootID: manager.workerBootID, stopped: true}, nil
	}
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return LinuxWorkerCgroupStopProof{}, fmt.Errorf("%w: WorkerSession cgroup is linked or cannot be inspected", ErrLinuxNodeCgroupCleanupUnconfirmed)
	}
	if err = (&linuxNodeResourceCgroup{path: path, name: name, fd: -1}).Cleanup(); err != nil {
		return LinuxWorkerCgroupStopProof{}, err
	}
	return LinuxWorkerCgroupStopProof{sessionID: sessionID, name: name, hostID: manager.workerHostID, rootID: manager.workerRootID, bootID: manager.workerBootID, stopped: true, present: true}, nil
}

func (manager *linuxNodeCgroupV2Manager) ReconcileWorkerOrphans(knownSessionIDs []string) error {
	if manager == nil {
		return ErrLinuxNodeCgroupUnavailable
	}
	known := make(map[string]struct{}, len(knownSessionIDs))
	for _, sessionID := range knownSessionIDs {
		name, err := LinuxWorkerCgroupName(sessionID)
		if err != nil {
			return err
		}
		known[name] = struct{}{}
	}
	entries, err := os.ReadDir(manager.root)
	if err != nil {
		return fmt.Errorf("%w: delegated cgroup root cannot be read for Worker orphan cleanup", ErrLinuxNodeCgroupUnavailable)
	}
	for _, entry := range entries {
		path := filepath.Join(manager.root, entry.Name())
		info, statErr := os.Lstat(path)
		if statErr != nil || info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("%w: Worker cgroup entry is linked or cannot be inspected", ErrLinuxNodeCgroupUnavailable)
		}
		if !info.IsDir() || validLinuxNodeCgroupOwnerDirectoryName(entry.Name()) {
			continue
		}
		if validLinuxNodeCgroupDirectoryName(entry.Name()) {
			return fmt.Errorf("%w: a Node JobRun cgroup remained after Node reconciliation", ErrLinuxNodeCgroupCleanupUnconfirmed)
		}
		if !validLinuxWorkerCgroupDirectoryName(entry.Name()) {
			return fmt.Errorf("%w: cgroup entry %q is outside the Polis namespace", ErrLinuxNodeCgroupUnavailable, entry.Name())
		}
		if _, isKnown := known[entry.Name()]; isKnown {
			continue
		}
		events, readErr := os.ReadFile(filepath.Join(path, "cgroup.events"))
		if readErr != nil {
			return fmt.Errorf("%w: orphan Worker cgroup state cannot be read", ErrLinuxNodeCgroupCleanupUnconfirmed)
		}
		remove, dispositionErr := linuxWorkerOrphanShouldBeRemoved(entry.Name(), false, string(events))
		if errors.Is(dispositionErr, ErrLinuxWorkerCgroupOrphanPopulated) {
			return fmt.Errorf("%w: %s", ErrLinuxWorkerCgroupOrphanPopulated, entry.Name())
		}
		if dispositionErr != nil {
			return fmt.Errorf("%w: orphan Worker cgroup state is invalid", ErrLinuxNodeCgroupCleanupUnconfirmed)
		}
		if !remove {
			continue
		}
		if removeErr := os.Remove(path); removeErr != nil {
			return fmt.Errorf("%w: empty orphan Worker cgroup could not be removed", ErrLinuxNodeCgroupCleanupUnconfirmed)
		}
	}
	return nil
}

func (manager *linuxNodeCgroupV2Manager) createGroup(name string) (*linuxNodeResourceCgroup, error) {
	if manager == nil || (!validLinuxNodeCgroupDirectoryName(name) && !validLinuxWorkerCgroupDirectoryName(name)) {
		return nil, fmt.Errorf("%w: cgroup leaf name is invalid", ErrLinuxNodeCgroupUnavailable)
	}
	if err := validateLinuxNodeCgroupRoot(manager.root, manager.limits); err != nil {
		return nil, err
	}
	path := filepath.Join(manager.root, name)
	if err := os.Mkdir(path, 0700); err != nil {
		return nil, fmt.Errorf("%w: isolated resource cgroup could not be created", ErrLinuxNodeCgroupUnavailable)
	}
	created := true
	defer func() {
		if created {
			_ = os.Remove(path)
		}
	}()
	controls, err := manager.limits.CgroupV2Controls()
	if err != nil {
		return nil, err
	}
	if err = applyLinuxResourceCgroupControls(path, controls); err != nil {
		return nil, fmt.Errorf("%w: required cgroup controls could not be set and verified", ErrLinuxNodeCgroupUnavailable)
	}
	if _, err = os.Stat(filepath.Join(path, "cgroup.kill")); err != nil {
		return nil, fmt.Errorf("%w: cgroup.kill is required for bounded cleanup", ErrLinuxNodeCgroupUnavailable)
	}
	events, err := os.ReadFile(filepath.Join(path, "cgroup.events"))
	if err != nil {
		return nil, fmt.Errorf("%w: cgroup.events is required for recursive cleanup proof", ErrLinuxNodeCgroupUnavailable)
	}
	if populated, populationErr := parseLinuxCgroupPopulated(string(events)); populationErr != nil || populated {
		return nil, fmt.Errorf("%w: new resource cgroup is already populated", ErrLinuxNodeCgroupUnavailable)
	}
	fd, err := syscall.Open(path, linuxCgroupPathFDFlag|syscall.O_CLOEXEC, 0)
	if err != nil {
		return nil, fmt.Errorf("%w: resource cgroup could not be opened for atomic process placement", ErrLinuxNodeCgroupUnavailable)
	}
	created = false
	return &linuxNodeResourceCgroup{path: path, name: name, bubblewrapPath: manager.workerBubblewrapPath, hostID: manager.workerHostID, rootID: manager.workerRootID, bootID: manager.workerBootID, fd: fd}, nil
}

func applyLinuxResourceCgroupControls(path string, controls map[string]string) error {
	for _, name := range []string{"cpu.max", "memory.max", "memory.swap.max", "memory.oom.group", "pids.max", "cgroup.max.descendants"} {
		value, ok := controls[name]
		if !ok {
			return ErrLinuxNodeCgroupUnavailable
		}
		if err := writeLinuxCgroupControl(filepath.Join(path, name), value); err != nil {
			return err
		}
		observed, err := os.ReadFile(filepath.Join(path, name))
		if err != nil || strings.TrimSpace(string(observed)) != value {
			return ErrLinuxNodeCgroupUnavailable
		}
	}
	return nil
}

func (group *linuxNodeResourceCgroup) Name() string {
	if group == nil {
		return ""
	}
	return group.name
}

func (group *linuxNodeResourceCgroup) CgroupRootPath() string {
	if group == nil || group.path == "" {
		return ""
	}
	return filepath.Dir(group.path)
}

func (group *linuxNodeResourceCgroup) BubblewrapPath() string {
	if group == nil {
		return ""
	}
	return group.bubblewrapPath
}

func (group *linuxNodeResourceCgroup) HostIdentity() string {
	if group == nil {
		return ""
	}
	return group.hostID
}

func (group *linuxNodeResourceCgroup) RootIdentity() string {
	if group == nil {
		return ""
	}
	return group.rootID
}

func (group *linuxNodeResourceCgroup) BootID() string {
	if group == nil {
		return ""
	}
	return group.bootID
}

func writeLinuxCgroupControl(path, value string) error {
	fd, err := syscall.Open(path, syscall.O_WRONLY|syscall.O_CLOEXEC, 0)
	if err != nil {
		return err
	}
	defer syscall.Close(fd)
	written, err := syscall.Write(fd, []byte(value))
	if err != nil || written != len(value) {
		return errors.Join(err, errors.New("short cgroup control write"))
	}
	return nil
}

func (group *linuxNodeResourceCgroup) FileDescriptor() int {
	if group == nil {
		return -1
	}
	group.mu.Lock()
	defer group.mu.Unlock()
	if group.removed {
		return -1
	}
	return group.fd
}

func (group *linuxNodeResourceCgroup) Cleanup() error {
	if group == nil {
		return nil
	}
	group.mu.Lock()
	defer group.mu.Unlock()
	if group.removed {
		return nil
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		events, err := os.ReadFile(filepath.Join(group.path, "cgroup.events"))
		if err != nil {
			return fmt.Errorf("%w: resource cgroup population state cannot be read", ErrLinuxNodeCgroupCleanupUnconfirmed)
		}
		populated, parseErr := parseLinuxCgroupPopulated(string(events))
		if parseErr != nil {
			return fmt.Errorf("%w: resource cgroup population state is invalid", ErrLinuxNodeCgroupCleanupUnconfirmed)
		}
		if !populated {
			break
		}
		if err = writeLinuxCgroupControl(filepath.Join(group.path, "cgroup.kill"), "1"); err != nil {
			return fmt.Errorf("%w: remaining processes could not be fenced", ErrLinuxNodeCgroupCleanupUnconfirmed)
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("%w: resource cgroup did not become empty", ErrLinuxNodeCgroupCleanupUnconfirmed)
		}
		time.Sleep(10 * time.Millisecond)
	}
	if group.fd >= 0 {
		if err := syscall.Close(group.fd); err != nil && !errors.Is(err, syscall.EBADF) {
			return fmt.Errorf("%w: resource cgroup handle could not be closed", ErrLinuxNodeCgroupCleanupUnconfirmed)
		}
		group.fd = -1
	}
	if err := os.Remove(group.path); err != nil {
		return fmt.Errorf("%w: empty resource cgroup could not be removed", ErrLinuxNodeCgroupCleanupUnconfirmed)
	}
	group.removed = true
	return nil
}

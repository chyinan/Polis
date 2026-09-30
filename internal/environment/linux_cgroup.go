// pattern: Functional Core
package environment

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
)

var ErrLinuxNodeCgroupUnavailable = errors.New("Linux Node cgroup v2 resource limits are unavailable")
var ErrLinuxNodeCgroupCleanupUnconfirmed = errors.New("Linux Node cgroup cleanup is unconfirmed")
var ErrLinuxWorkerCgroupOrphanPopulated = errors.New("Linux Worker cgroup orphan is populated")

const linuxWorkerCgroupDirectoryPrefix = "polis-worker-"

func parseLinuxCgroupPopulated(contents string) (bool, error) {
	found := false
	populated := false
	for _, line := range strings.Split(contents, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 || fields[0] != "populated" {
			continue
		}
		if found || len(fields) != 2 || (fields[1] != "0" && fields[1] != "1") {
			return false, fmt.Errorf("%w: cgroup.events population field is malformed", ErrLinuxNodeCgroupUnavailable)
		}
		found = true
		populated = fields[1] == "1"
	}
	if !found {
		return false, fmt.Errorf("%w: cgroup.events population field is missing", ErrLinuxNodeCgroupUnavailable)
	}
	return populated, nil
}

func linuxWorkerOrphanShouldBeRemoved(name string, known bool, events string) (bool, error) {
	if !validLinuxWorkerCgroupDirectoryName(name) {
		return false, fmt.Errorf("%w: Worker cgroup leaf name is invalid", ErrLinuxNodeCgroupUnavailable)
	}
	if known {
		return false, nil
	}
	populated, err := parseLinuxCgroupPopulated(events)
	if err != nil {
		return false, err
	}
	if populated {
		return false, ErrLinuxWorkerCgroupOrphanPopulated
	}
	return true, nil
}

func linuxWorkerCgroupIdentities(machineID, productID, bootID, runtimeRoot, cgroupRoot, instanceID string, device, inode uint64) (string, string, error) {
	if !validLowerHexIdentity(machineID, 32) || !validLinuxUUID(productID) || !validLinuxUUID(bootID) ||
		!linuxIdentityHasNonzeroHex(machineID) || !linuxIdentityHasNonzeroHex(productID) || !linuxIdentityHasNonzeroHex(bootID) ||
		!filepath.IsAbs(runtimeRoot) || filepath.Clean(runtimeRoot) != runtimeRoot ||
		!filepath.IsAbs(cgroupRoot) || filepath.Clean(cgroupRoot) != cgroupRoot ||
		!validLinuxWorkerCgroupHostID(instanceID) || inode == 0 {
		return "", "", fmt.Errorf("%w: Linux Worker cgroup host identity inputs are invalid", ErrLinuxNodeCgroupUnavailable)
	}
	hostDigest := sha256.Sum256([]byte("polis-linux-worker-host@1\n" + strings.ToLower(machineID) + "\n" + strings.ToLower(productID)))
	hostID := hex.EncodeToString(hostDigest[:])
	rootContents := fmt.Sprintf("polis-linux-worker-cgroup-root@1\n%s\n%s\n%s\n%s\n%s\n%d\n%d", hostID, bootID, runtimeRoot, cgroupRoot, instanceID, device, inode)
	rootDigest := sha256.Sum256([]byte(rootContents))
	return hostID, hex.EncodeToString(rootDigest[:]), nil
}

func linuxIdentityHasNonzeroHex(value string) bool {
	for _, character := range value {
		if character != '0' && character != '-' {
			return true
		}
	}
	return false
}

func linuxWorkerCgroupSandboxRootIdentity(rootID, bubblewrapPath, bubblewrapSHA256 string) (string, error) {
	if !validLinuxWorkerCgroupHostID(rootID) || !filepath.IsAbs(bubblewrapPath) || filepath.Clean(bubblewrapPath) != bubblewrapPath || !validLinuxWorkerCgroupHostID(bubblewrapSHA256) {
		return "", fmt.Errorf("%w: Linux Worker sandbox identity is invalid", ErrLinuxNodeCgroupUnavailable)
	}
	digest := sha256.Sum256([]byte("polis-linux-worker-sandbox@1\n" + rootID + "\n" + bubblewrapPath + "\n" + bubblewrapSHA256))
	return hex.EncodeToString(digest[:]), nil
}

func linuxWorkerBubblewrapFingerprint(contentSHA256, mode, owner string, ownerKnown bool) (string, error) {
	if !validLinuxWorkerCgroupHostID(contentSHA256) || mode == "" || owner == "" || !ownerKnown {
		return "", fmt.Errorf("%w: Linux Worker bubblewrap fingerprint is incomplete", ErrLinuxNodeCgroupUnavailable)
	}
	digest := sha256.Sum256([]byte(fmt.Sprintf("linux-worker-bwrap@1\n%s\n%s\n%s", contentSHA256, mode, owner)))
	return hex.EncodeToString(digest[:]), nil
}

func validLowerHexIdentity(value string, length int) bool {
	if len(value) != length {
		return false
	}
	for _, character := range value {
		if !((character >= '0' && character <= '9') || (character >= 'a' && character <= 'f') || (character >= 'A' && character <= 'F')) {
			return false
		}
	}
	return true
}

func validLinuxUUID(value string) bool {
	if len(value) != 36 || value[8] != '-' || value[13] != '-' || value[18] != '-' || value[23] != '-' {
		return false
	}
	for index, character := range value {
		if index == 8 || index == 13 || index == 18 || index == 23 {
			continue
		}
		if !((character >= '0' && character <= '9') || (character >= 'a' && character <= 'f') || (character >= 'A' && character <= 'F')) {
			return false
		}
	}
	return true
}

func LinuxWorkerCgroupName(sessionID string) (string, error) {
	if len(sessionID) == 0 || len(sessionID) > 128 {
		return "", fmt.Errorf("%w: WorkerSession identity is invalid", ErrLinuxNodeCgroupUnavailable)
	}
	for _, character := range sessionID {
		if !((character >= 'a' && character <= 'z') || (character >= 'A' && character <= 'Z') ||
			(character >= '0' && character <= '9') || character == '-' || character == '_') {
			return "", fmt.Errorf("%w: WorkerSession identity is invalid", ErrLinuxNodeCgroupUnavailable)
		}
	}
	digest := sha256.Sum256([]byte(sessionID))
	return linuxWorkerCgroupDirectoryPrefix + hex.EncodeToString(digest[:12]), nil
}

func validLinuxWorkerCgroupDirectoryName(name string) bool {
	if !strings.HasPrefix(name, linuxWorkerCgroupDirectoryPrefix) {
		return false
	}
	suffix := strings.TrimPrefix(name, linuxWorkerCgroupDirectoryPrefix)
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

func validLinuxWorkerCgroupHostID(identity string) bool {
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

type LinuxNodeResourceLimits struct {
	CPUQuotaMicros             int64 `json:"cpuQuotaMicros"`
	CPUPeriodMicros            int64 `json:"cpuPeriodMicros"`
	MemoryMaxBytes             int64 `json:"memoryMaxBytes"`
	MaxProcesses               int64 `json:"maxProcesses"`
	AggregateCPUQuotaMicros    int64 `json:"aggregateCpuQuotaMicros"`
	AggregateCPUPeriodMicros   int64 `json:"aggregateCpuPeriodMicros"`
	AggregateMemoryMaxBytes    int64 `json:"aggregateMemoryMaxBytes"`
	AggregateMaxProcesses      int64 `json:"aggregateMaxProcesses"`
	AggregateMaxCgroupChildren int64 `json:"aggregateMaxCgroupChildren"`
}

func DefaultLinuxNodeResourceLimits() LinuxNodeResourceLimits {
	return LinuxNodeResourceLimits{
		CPUQuotaMicros: 200_000, CPUPeriodMicros: 100_000,
		MemoryMaxBytes: 2 << 30, MaxProcesses: 256,
		AggregateCPUQuotaMicros: 400_000, AggregateCPUPeriodMicros: 100_000,
		AggregateMemoryMaxBytes: 4 << 30, AggregateMaxProcesses: 512, AggregateMaxCgroupChildren: 32,
	}
}

func (limits LinuxNodeResourceLimits) Validate() error {
	if limits.CPUQuotaMicros < 1_000 || limits.CPUQuotaMicros > 10_000_000 || limits.CPUPeriodMicros < 1_000 || limits.CPUPeriodMicros > 1_000_000 || limits.MemoryMaxBytes < 64<<20 || limits.MemoryMaxBytes > 64<<30 || limits.MaxProcesses < 16 || limits.MaxProcesses > 16_384 || limits.AggregateCPUQuotaMicros < 1_000 || limits.AggregateCPUQuotaMicros > 20_000_000 || limits.AggregateCPUPeriodMicros < 1_000 || limits.AggregateCPUPeriodMicros > 1_000_000 || limits.AggregateCPUQuotaMicros*limits.CPUPeriodMicros < limits.CPUQuotaMicros*limits.AggregateCPUPeriodMicros || limits.AggregateMemoryMaxBytes < limits.MemoryMaxBytes || limits.AggregateMemoryMaxBytes > 256<<30 || limits.AggregateMaxProcesses < limits.MaxProcesses || limits.AggregateMaxProcesses > 65_536 || limits.AggregateMaxCgroupChildren < 1 || limits.AggregateMaxCgroupChildren > 4096 {
		return fmt.Errorf("%w: Linux Node resource limits are outside the supported bounds", ErrEnvironmentPlan)
	}
	return nil
}

func ValidateLinuxNodeParentCgroupLimits(cpuMax, memoryMax, memorySwapMax, pidsMax, oomGroup, maxDescendants string, limits LinuxNodeResourceLimits) error {
	if err := limits.Validate(); err != nil {
		return err
	}
	cpuValues := strings.Fields(cpuMax)
	if len(cpuValues) != 2 || cpuValues[0] == "max" {
		return fmt.Errorf("%w: parent cpu.max must be finite", ErrLinuxNodeCgroupUnavailable)
	}
	quota, quotaErr := strconv.ParseInt(cpuValues[0], 10, 64)
	period, periodErr := strconv.ParseInt(cpuValues[1], 10, 64)
	if quotaErr != nil || periodErr != nil || quota < 1_000 || quota > 20_000_000 || period < 1_000 || period > 1_000_000 || quota*limits.AggregateCPUPeriodMicros > limits.AggregateCPUQuotaMicros*period {
		return fmt.Errorf("%w: parent CPU ceiling exceeds the aggregate profile", ErrLinuxNodeCgroupUnavailable)
	}
	memory, err := parseFiniteCgroupLimit(memoryMax)
	if err != nil || memory > limits.AggregateMemoryMaxBytes {
		return fmt.Errorf("%w: parent memory ceiling exceeds the aggregate profile", ErrLinuxNodeCgroupUnavailable)
	}
	if strings.TrimSpace(memorySwapMax) != "0" {
		return fmt.Errorf("%w: parent swap must be disabled", ErrLinuxNodeCgroupUnavailable)
	}
	pids, err := parseFiniteCgroupLimit(pidsMax)
	if err != nil || pids > limits.AggregateMaxProcesses {
		return fmt.Errorf("%w: parent PID ceiling exceeds the aggregate profile", ErrLinuxNodeCgroupUnavailable)
	}
	if strings.TrimSpace(oomGroup) != "1" {
		return fmt.Errorf("%w: parent OOM handling must be group-scoped", ErrLinuxNodeCgroupUnavailable)
	}
	descendants, err := parseFiniteCgroupLimit(maxDescendants)
	if err != nil || descendants > limits.AggregateMaxCgroupChildren {
		return fmt.Errorf("%w: parent cgroup child count exceeds the aggregate profile", ErrLinuxNodeCgroupUnavailable)
	}
	return nil
}

func parseFiniteCgroupLimit(value string) (int64, error) {
	value = strings.TrimSpace(value)
	if value == "" || value == "max" {
		return 0, ErrLinuxNodeCgroupUnavailable
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil || parsed <= 0 {
		return 0, ErrLinuxNodeCgroupUnavailable
	}
	return parsed, nil
}

func (limits LinuxNodeResourceLimits) CgroupV2Controls() (map[string]string, error) {
	if err := limits.Validate(); err != nil {
		return nil, err
	}
	return map[string]string{
		"cpu.max":                fmt.Sprintf("%d %d", limits.CPUQuotaMicros, limits.CPUPeriodMicros),
		"memory.max":             fmt.Sprint(limits.MemoryMaxBytes),
		"memory.swap.max":        "0",
		"memory.oom.group":       "1",
		"pids.max":               fmt.Sprint(limits.MaxProcesses),
		"cgroup.max.descendants": "0",
	}, nil
}

type LinuxNodeResourceCgroup interface {
	FileDescriptor() int
	Cleanup() error
}

type LinuxWorkerResourceCgroup interface {
	LinuxNodeResourceCgroup
	Name() string
	CgroupRootPath() string
	BubblewrapPath() string
	HostIdentity() string
	RootIdentity() string
	BootID() string
}

type LinuxWorkerCgroupStopProof struct {
	sessionID string
	name      string
	hostID    string
	rootID    string
	bootID    string
	stopped   bool
	present   bool
}

func (proof LinuxWorkerCgroupStopProof) ForWorkerSession(sessionID, name string) bool {
	return proof.stopped && sessionID != "" && sessionID == proof.sessionID &&
		validLinuxWorkerCgroupDirectoryName(name) && name == proof.name &&
		validLinuxWorkerCgroupHostID(proof.hostID) && validLinuxWorkerCgroupHostID(proof.rootID) && validLinuxUUID(proof.bootID)
}

func (proof LinuxWorkerCgroupStopProof) WasPresent() bool {
	return proof.stopped && proof.present
}

func (proof LinuxWorkerCgroupStopProof) HostIdentity() string {
	if !proof.stopped {
		return ""
	}
	return proof.hostID
}

func (proof LinuxWorkerCgroupStopProof) RootIdentity() string {
	if !proof.stopped {
		return ""
	}
	return proof.rootID
}

func (proof LinuxWorkerCgroupStopProof) BootID() string {
	if !proof.stopped {
		return ""
	}
	return proof.bootID
}

type LinuxNodeCgroupManager interface {
	Create(identity string) (LinuxNodeResourceCgroup, error)
}

type LinuxNodeCgroupManagerIdentity interface {
	LinuxNodeCgroupManager
	RuntimeRootPath() string
	CgroupRootPath() string
	InstanceIdentity() string
}

type LinuxNodeCgroupManagerCloser interface {
	Close() error
}

type LinuxWorkerCgroupManager interface {
	LinuxNodeCgroupManager
	LinuxNodeCgroupManagerIdentity
	WorkerCgroupHostIdentity() string
	WorkerCgroupRootIdentity() string
	WorkerCgroupBootID() string
	WorkerContainmentReady() error
	CreateWorker(sessionID string) (LinuxWorkerResourceCgroup, error)
	ReconcileWorker(sessionID string) (LinuxWorkerCgroupStopProof, error)
	ReconcileWorkerOrphans(knownSessionIDs []string) error
}

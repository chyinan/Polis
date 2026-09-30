//go:build linux

// pattern: Functional Core
package environment

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLinuxDelegatedCgroupRootRefusesExistingGroupsWithoutRemovingThem(t *testing.T) {
	root := t.TempDir()
	existing := filepath.Join(root, "polis-node-active")
	if err := os.Mkdir(existing, 0700); err != nil {
		t.Fatal(err)
	}
	if err := requireEmptyLinuxNodeCgroupRoot(root); err == nil {
		t.Fatal("delegated cgroup root with an existing group was accepted")
	}
	if _, err := os.Stat(existing); err != nil {
		t.Fatalf("pre-existing cgroup group was changed or removed: %v", err)
	}
}

func TestLinuxNodeDefaultResourceLimitsFormatBoundedCgroupControls(t *testing.T) {
	limits := DefaultLinuxNodeResourceLimits()
	if err := limits.Validate(); err != nil {
		t.Fatal(err)
	}
	controls, err := limits.CgroupV2Controls()
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"cpu.max":                "200000 100000",
		"memory.max":             "2147483648",
		"memory.swap.max":        "0",
		"memory.oom.group":       "1",
		"pids.max":               "256",
		"cgroup.max.descendants": "0",
	}
	if len(controls) != len(want) {
		t.Fatalf("resource control count=%d, want=%d: %+v", len(controls), len(want), controls)
	}
	for name, value := range want {
		if controls[name] != value {
			t.Fatalf("control %s=%q, want %q", name, controls[name], value)
		}
	}
}

func TestLinuxResourceCgroupInitializationWritesAndVerifiesAllControls(t *testing.T) {
	controls, err := DefaultLinuxNodeResourceLimits().CgroupV2Controls()
	if err != nil {
		t.Fatal(err)
	}
	path := t.TempDir()
	for name := range controls {
		if err = os.WriteFile(filepath.Join(path, name), nil, 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err = applyLinuxResourceCgroupControls(path, controls); err != nil {
		t.Fatalf("apply controls: %v", err)
	}
	for name, want := range controls {
		got, readErr := os.ReadFile(filepath.Join(path, name))
		if readErr != nil || strings.TrimSpace(string(got)) != want {
			t.Fatalf("control %s=%q err=%v, want %q", name, got, readErr, want)
		}
	}

	delete(controls, "cgroup.max.descendants")
	if err = applyLinuxResourceCgroupControls(path, controls); err == nil {
		t.Fatal("missing descendant bound was accepted")
	}
}

func TestLinuxNodeResourceLimitsRejectInvalidValues(t *testing.T) {
	invalid := []LinuxNodeResourceLimits{
		{CPUQuotaMicros: 0, CPUPeriodMicros: 100000, MemoryMaxBytes: 1 << 30, MaxProcesses: 64},
		{CPUQuotaMicros: 200000, CPUPeriodMicros: 0, MemoryMaxBytes: 1 << 30, MaxProcesses: 64},
		{CPUQuotaMicros: 200000, CPUPeriodMicros: 100000, MemoryMaxBytes: 0, MaxProcesses: 64},
		{CPUQuotaMicros: 200000, CPUPeriodMicros: 100000, MemoryMaxBytes: 1 << 30, MaxProcesses: 0},
	}
	for index, limits := range invalid {
		if err := limits.Validate(); err == nil {
			t.Fatalf("invalid limits case %d was accepted: %+v", index, limits)
		}
	}
}

func TestLinuxNodeCgroupParentRequiresFiniteAggregateQuotas(t *testing.T) {
	limits := DefaultLinuxNodeResourceLimits()
	valid := []string{"400000 100000", "4294967296", "0", "512", "1", "32"}
	if err := ValidateLinuxNodeParentCgroupLimits(valid[0], valid[1], valid[2], valid[3], valid[4], valid[5], limits); err != nil {
		t.Fatalf("default aggregate cgroup limits rejected: %v", err)
	}
	belowCeiling := []string{"200000 100000", "2147483648", "0", "256", "1", "8"}
	if err := ValidateLinuxNodeParentCgroupLimits(belowCeiling[0], belowCeiling[1], belowCeiling[2], belowCeiling[3], belowCeiling[4], belowCeiling[5], limits); err != nil {
		t.Fatalf("stricter parent cgroup limits rejected: %v", err)
	}
	invalid := [][]string{
		{"max 100000", "4294967296", "0", "512", "1", "32"},
		{"500000 100000", "4294967296", "0", "512", "1", "32"},
		{"400000 100000", "max", "0", "512", "1", "32"},
		{"400000 100000", "4294967296", "0", "513", "1", "32"},
		{"400000 100000", "4294967296", "0", "512", "0", "32"},
		{"400000 100000", "4294967296", "0", "512", "1", "max"},
	}
	for index, values := range invalid {
		if err := ValidateLinuxNodeParentCgroupLimits(values[0], values[1], values[2], values[3], values[4], values[5], limits); err == nil {
			t.Fatalf("unbounded or over-limit parent case %d accepted: %v", index, values)
		}
	}
}

func TestLinuxCgroupManagerRejectsOrdinaryFilesystemBeforeCreatingAnything(t *testing.T) {
	runtimeRoot, err := os.MkdirTemp("/tmp", "polis-cgroup-runtime-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(runtimeRoot)
	root := filepath.Join(runtimeRoot, "cgroups")
	if err := os.Mkdir(root, 0700); err != nil {
		t.Fatal(err)
	}
	if _, err := NewLinuxNodeCgroupV2Manager(runtimeRoot, root, strings.Repeat("a", 64), DefaultLinuxNodeResourceLimits()); err == nil {
		t.Fatal("ordinary temporary filesystem was accepted as a delegated cgroup v2 root")
	}
}

func TestLinuxDelegatedCgroupRootRejectsExistingChildWithoutRemovingIt(t *testing.T) {
	root := t.TempDir()
	child := filepath.Join(root, "polis-node-existing")
	if err := os.Mkdir(child, 0700); err != nil {
		t.Fatal(err)
	}
	if err := requireEmptyLinuxNodeCgroupRoot(root); err == nil {
		t.Fatal("delegated resource root accepted an existing cgroup")
	}
	if _, err := os.Stat(child); err != nil {
		t.Fatalf("manager modified existing child cgroup: %v", err)
	}
}

func TestLinuxNodeCgroupRestartCleanupRemovesOnlyOwnedGroups(t *testing.T) {
	root := t.TempDir()
	groupNames := []string{"polis-node-" + strings.Repeat("a", 24), "polis-node-" + strings.Repeat("b", 24)}
	for _, name := range groupNames {
		if err := os.Mkdir(filepath.Join(root, name), 0700); err != nil {
			t.Fatal(err)
		}
	}
	var cleaned []string
	err := cleanupLinuxNodeCgroupChildren(root, func(path string) error {
		cleaned = append(cleaned, filepath.Base(path))
		return os.Remove(path)
	})
	if err != nil {
		t.Fatalf("reconcile owned cgroup directories: %v", err)
	}
	if len(cleaned) != len(groupNames) || len(cleaned) != 2 {
		t.Fatalf("cleaned groups=%v, want %v", cleaned, groupNames)
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 0 {
		t.Fatalf("managed cgroup root after reconciliation=%v error=%v", entries, err)
	}
}

func TestLinuxNodeCgroupRestartCleanupPreservesWorkerGroups(t *testing.T) {
	root := t.TempDir()
	nodeGroup := filepath.Join(root, "polis-node-"+strings.Repeat("a", 24))
	workerGroup := filepath.Join(root, "polis-worker-"+strings.Repeat("b", 24))
	for _, path := range []string{nodeGroup, workerGroup} {
		if err := os.Mkdir(path, 0700); err != nil {
			t.Fatal(err)
		}
	}
	var cleaned []string
	err := cleanupLinuxNodeCgroupChildren(root, func(path string) error {
		cleaned = append(cleaned, filepath.Base(path))
		return os.Remove(path)
	})
	if err != nil {
		t.Fatalf("reconcile Node cgroup groups: %v", err)
	}
	if len(cleaned) != 1 || cleaned[0] != filepath.Base(nodeGroup) {
		t.Fatalf("startup cleaned cgroups %v, want only Node group %q", cleaned, filepath.Base(nodeGroup))
	}
	if _, err := os.Stat(workerGroup); err != nil {
		t.Fatalf("startup removed WorkerSession cgroup before database reconciliation: %v", err)
	}
}

func TestLinuxWorkerOrphanReconciliationRemovesOnlyEmptyUnknownGroups(t *testing.T) {
	root := t.TempDir()
	knownSession := "worker-session-known"
	knownName, err := LinuxWorkerCgroupName(knownSession)
	if err != nil {
		t.Fatal(err)
	}
	knownPath := filepath.Join(root, knownName)
	populatedOrphan := filepath.Join(root, "polis-worker-"+strings.Repeat("e", 24))
	for _, path := range []string{knownPath, populatedOrphan} {
		if err = os.Mkdir(path, 0700); err != nil {
			t.Fatal(err)
		}
	}
	if err = os.WriteFile(filepath.Join(knownPath, "cgroup.events"), []byte("populated 1\nfrozen 0\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(populatedOrphan, "cgroup.events"), []byte("populated 1\nfrozen 0\n"), 0600); err != nil {
		t.Fatal(err)
	}
	manager := &linuxNodeCgroupV2Manager{root: root}
	if err = manager.ReconcileWorkerOrphans([]string{knownSession}); !errors.Is(err, ErrLinuxWorkerCgroupOrphanPopulated) {
		t.Fatalf("populated orphan Worker cgroup error=%v, want fail-closed orphan error", err)
	}
	if _, err = os.Stat(knownPath); err != nil {
		t.Fatalf("known unresolved Worker cgroup was removed: %v", err)
	}
	if _, err = os.Stat(populatedOrphan); err != nil {
		t.Fatalf("populated orphan Worker cgroup was changed: %v", err)
	}
}

func TestLinuxWorkerOrphanDispositionPreservesKnownAndRemovesOnlyEmpty(t *testing.T) {
	name := "polis-worker-" + strings.Repeat("f", 24)
	if remove, err := linuxWorkerOrphanShouldBeRemoved(name, true, "populated 1\nfrozen 0\n"); err != nil || remove {
		t.Fatalf("known Worker cgroup disposition remove=%t error=%v, want preserve", remove, err)
	}
	if remove, err := linuxWorkerOrphanShouldBeRemoved(name, false, "populated 0\nfrozen 0\n"); err != nil || !remove {
		t.Fatalf("empty orphan disposition remove=%t error=%v, want remove", remove, err)
	}
	if remove, err := linuxWorkerOrphanShouldBeRemoved(name, false, "populated 1\nfrozen 0\n"); !errors.Is(err, ErrLinuxWorkerCgroupOrphanPopulated) || remove {
		t.Fatalf("populated orphan disposition remove=%t error=%v, want fail closed", remove, err)
	}
}

func TestLinuxWorkerCgroupNameIsStableOpaqueAndRejectsInvalidSessionIDs(t *testing.T) {
	first, err := LinuxWorkerCgroupName("worker-session-123")
	if err != nil {
		t.Fatalf("derive Worker cgroup name: %v", err)
	}
	second, err := LinuxWorkerCgroupName("worker-session-123")
	if err != nil || first != second {
		t.Fatalf("Worker cgroup name is not stable: first=%q second=%q error=%v", first, second, err)
	}
	third, err := LinuxWorkerCgroupName("worker-session-456")
	if err != nil || first == third {
		t.Fatalf("distinct WorkerSessions share a cgroup name: first=%q third=%q error=%v", first, third, err)
	}
	if !validLinuxWorkerCgroupDirectoryName(first) || strings.Contains(first, "worker-session") {
		t.Fatalf("Worker cgroup name is not an opaque managed leaf: %q", first)
	}
	for _, invalid := range []string{"", "../outside", "worker session", strings.Repeat("x", 129)} {
		if name, nameErr := LinuxWorkerCgroupName(invalid); nameErr == nil || name != "" {
			t.Fatalf("invalid session ID %q produced cgroup name %q error=%v", invalid, name, nameErr)
		}
	}
}

func TestLinuxWorkerCgroupExposesConfiguredRootFromLeafPath(t *testing.T) {
	group := &linuxNodeResourceCgroup{path: "/srv/polis/runtime/cgroup/polis-worker-0123456789abcdef01234567"}
	if got, want := group.CgroupRootPath(), "/srv/polis/runtime/cgroup"; got != want {
		t.Fatalf("Worker cgroup root=%q, want %q", got, want)
	}
}

func TestLinuxWorkerCgroupIdentityBindsHostRootBootAndInstance(t *testing.T) {
	machineID := strings.Repeat("a", 32)
	productID := "12345678-1234-1234-1234-123456789abc"
	bootID := "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"
	instanceID := strings.Repeat("1", 64)
	hostID, rootID, err := linuxWorkerCgroupIdentities(machineID, productID, bootID, "/var/lib/polis", "/sys/fs/cgroup/polis", instanceID, 42, 99)
	if err != nil {
		t.Fatalf("compute Worker cgroup identity: %v", err)
	}
	if !validLinuxWorkerCgroupHostID(hostID) || !validLinuxWorkerCgroupHostID(rootID) {
		t.Fatalf("Worker cgroup identities are not canonical: host=%q root=%q", hostID, rootID)
	}
	if sameHost, sameRoot, sameErr := linuxWorkerCgroupIdentities(machineID, productID, bootID, "/var/lib/polis", "/sys/fs/cgroup/polis", instanceID, 42, 99); sameErr != nil || sameHost != hostID || sameRoot != rootID {
		t.Fatalf("Worker cgroup identity is not stable: host=%q root=%q error=%v", sameHost, sameRoot, sameErr)
	}
	otherHost, otherRoot, err := linuxWorkerCgroupIdentities(strings.Repeat("b", 32), productID, bootID, "/var/lib/polis", "/sys/fs/cgroup/polis", instanceID, 42, 99)
	if err != nil || otherHost == hostID || otherRoot == rootID {
		t.Fatalf("different host identity was not distinguished: host=%q root=%q error=%v", otherHost, otherRoot, err)
	}
	changedRoot, errRoot, err := linuxWorkerCgroupIdentities(machineID, productID, bootID, "/var/lib/polis", "/sys/fs/cgroup/other", instanceID, 42, 99)
	if err != nil || changedRoot != hostID || errRoot == rootID {
		t.Fatalf("changed cgroup root was not distinguished: host=%q root=%q error=%v", changedRoot, errRoot, err)
	}
	changedBootHost, changedBootRoot, err := linuxWorkerCgroupIdentities(machineID, productID, "bbbbbbbb-cccc-dddd-eeee-ffffffffffff", "/var/lib/polis", "/sys/fs/cgroup/polis", instanceID, 42, 99)
	if err != nil || changedBootHost != hostID || changedBootRoot == rootID {
		t.Fatalf("changed boot ID was not distinguished: host=%q root=%q error=%v", changedBootHost, changedBootRoot, err)
	}
	_, changedInstanceRoot, err := linuxWorkerCgroupIdentities(machineID, productID, bootID, "/var/lib/polis", "/sys/fs/cgroup/polis", strings.Repeat("2", 64), 42, 99)
	if err != nil || changedInstanceRoot == rootID {
		t.Fatalf("changed Polis instance identity was not distinguished: root=%q error=%v", changedInstanceRoot, err)
	}
}

func TestLinuxWorkerCgroupSandboxIdentityBindsExactBubblewrap(t *testing.T) {
	baseRoot := strings.Repeat("a", 64)
	first, err := linuxWorkerCgroupSandboxRootIdentity(baseRoot, "/usr/bin/bwrap", strings.Repeat("b", 64))
	if err != nil {
		t.Fatal(err)
	}
	same, err := linuxWorkerCgroupSandboxRootIdentity(baseRoot, "/usr/bin/bwrap", strings.Repeat("b", 64))
	if err != nil || first != same {
		t.Fatalf("sandbox identity is unstable: first=%q same=%q error=%v", first, same, err)
	}
	changedDigest, err := linuxWorkerCgroupSandboxRootIdentity(baseRoot, "/usr/bin/bwrap", strings.Repeat("c", 64))
	if err != nil || changedDigest == first {
		t.Fatalf("changed bubblewrap digest was not bound: got=%q error=%v", changedDigest, err)
	}
	changedPath, err := linuxWorkerCgroupSandboxRootIdentity(baseRoot, "/usr/local/bin/bwrap", strings.Repeat("b", 64))
	if err != nil || changedPath == first {
		t.Fatalf("changed bubblewrap path was not bound: got=%q error=%v", changedPath, err)
	}
}

func TestLinuxWorkerBubblewrapFingerprintBindsBytesModeAndOwner(t *testing.T) {
	first, err := linuxWorkerBubblewrapFingerprint(strings.Repeat("a", 64), "0755", "0", true)
	if err != nil {
		t.Fatal(err)
	}
	changedMode, err := linuxWorkerBubblewrapFingerprint(strings.Repeat("a", 64), "0700", "0", true)
	if err != nil || changedMode == first {
		t.Fatalf("changed bubblewrap mode was not bound: got=%q error=%v", changedMode, err)
	}
	changedOwner, err := linuxWorkerBubblewrapFingerprint(strings.Repeat("a", 64), "0755", "1000", true)
	if err != nil || changedOwner == first {
		t.Fatalf("changed bubblewrap owner was not bound: got=%q error=%v", changedOwner, err)
	}
	if _, err = linuxWorkerBubblewrapFingerprint(strings.Repeat("a", 64), "0755", "0", false); err == nil {
		t.Fatal("unknown bubblewrap ownership was accepted")
	}
}

func TestLinuxNodeCgroupManagerSupportsNamedWorkerReconciliation(t *testing.T) {
	var manager any = &linuxNodeCgroupV2Manager{}
	if _, ok := manager.(interface {
		CreateWorker(sessionID string) (LinuxWorkerResourceCgroup, error)
		ReconcileWorker(sessionID string) (LinuxWorkerCgroupStopProof, error)
	}); !ok {
		t.Fatal("Linux cgroup manager cannot create and reconcile a named WorkerSession group")
	}
}

func TestLinuxWorkerCgroupStopProofBindsExactSessionAndLeaf(t *testing.T) {
	sessionID := "worker-session-123"
	name, err := LinuxWorkerCgroupName(sessionID)
	if err != nil {
		t.Fatal(err)
	}
	proof := LinuxWorkerCgroupStopProof{
		sessionID: sessionID, name: name, hostID: strings.Repeat("a", 64),
		rootID: strings.Repeat("b", 64), bootID: "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee", stopped: true,
	}
	if !proof.ForWorkerSession(sessionID, name) {
		t.Fatal("exact WorkerSession cgroup proof was rejected")
	}
	if proof.ForWorkerSession("another-session", name) || proof.ForWorkerSession(sessionID, "polis-worker-"+strings.Repeat("0", 24)) {
		t.Fatal("WorkerSession cgroup proof was accepted for a different session or leaf")
	}
}

func TestLinuxCgroupEventsParserRequiresAUniquePopulationField(t *testing.T) {
	for _, test := range []struct {
		name      string
		contents  string
		populated bool
		wantError bool
	}{
		{name: "empty hierarchy", contents: "populated 0\nfrozen 0\n"},
		{name: "populated descendant", contents: "populated 1\nfrozen 0\n", populated: true},
		{name: "missing population", contents: "frozen 0\n", wantError: true},
		{name: "duplicate population", contents: "populated 0\npopulated 1\n", wantError: true},
		{name: "invalid population", contents: "populated many\n", wantError: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			populated, err := parseLinuxCgroupPopulated(test.contents)
			if test.wantError {
				if err == nil {
					t.Fatalf("malformed cgroup.events %q was accepted", test.contents)
				}
				return
			}
			if err != nil || populated != test.populated {
				t.Fatalf("cgroup population=%t error=%v, want %t", populated, err, test.populated)
			}
		})
	}
}

func TestLinuxNodeCgroupRestartCleanupRefusesUnknownAndLinkedGroups(t *testing.T) {
	for _, test := range []struct {
		name string
		make func(t *testing.T, root string) string
	}{
		{name: "unknown directory", make: func(t *testing.T, root string) string {
			path := filepath.Join(root, "operator-owned")
			if err := os.Mkdir(path, 0700); err != nil {
				t.Fatal(err)
			}
			return path
		}},
		{name: "linked managed directory", make: func(t *testing.T, root string) string {
			outside := t.TempDir()
			path := filepath.Join(root, "polis-node-"+strings.Repeat("c", 24))
			if err := os.Symlink(outside, path); err != nil {
				t.Fatal(err)
			}
			return path
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			unownedPath := test.make(t, root)
			cleanupCalls := 0
			err := cleanupLinuxNodeCgroupChildren(root, func(string) error {
				cleanupCalls++
				return nil
			})
			if !errors.Is(err, ErrLinuxNodeCgroupUnavailable) || cleanupCalls != 0 {
				t.Fatalf("unknown cgroup reconciliation error=%v cleanup calls=%d", err, cleanupCalls)
			}
			if _, statErr := os.Lstat(unownedPath); statErr != nil {
				t.Fatalf("refused cgroup entry was changed: %v", statErr)
			}
		})
	}
}

func TestLinuxNodeCgroupRuntimeRootMustContainDedicatedCgroupRoot(t *testing.T) {
	runtimeRoot, err := os.MkdirTemp("/tmp", "polis-cgroup-runtime-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(runtimeRoot)
	cgroupRoot := filepath.Join(runtimeRoot, "cgroups")
	if err := os.Mkdir(cgroupRoot, 0700); err != nil {
		t.Fatal(err)
	}
	if err := validateLinuxNodeCgroupRuntimeRoot(runtimeRoot, cgroupRoot); err != nil {
		t.Fatalf("dedicated cgroup child rejected: %v", err)
	}
	outside, err := os.MkdirTemp("/tmp", "polis-cgroup-outside-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(outside)
	for _, candidate := range []string{runtimeRoot, filepath.Join(outside, "cgroups")} {
		if candidate != filepath.Join(outside, "cgroups") {
			if err := validateLinuxNodeCgroupRuntimeRoot(runtimeRoot, candidate); err == nil {
				t.Fatalf("runtime root itself was accepted as cgroup root %q", candidate)
			}
			continue
		}
		if err := os.Mkdir(candidate, 0700); err != nil {
			t.Fatal(err)
		}
		if err := validateLinuxNodeCgroupRuntimeRoot(runtimeRoot, candidate); err == nil {
			t.Fatalf("outside cgroup root %q was accepted", candidate)
		}
	}
	link := filepath.Join(runtimeRoot, "linked-cgroups")
	if err := os.Symlink(cgroupRoot, link); err != nil {
		t.Fatal(err)
	}
	if err := validateLinuxNodeCgroupRuntimeRoot(runtimeRoot, link); err == nil {
		t.Fatal("linked cgroup root was accepted")
	}
}

func TestLinuxNodeCgroupRootLeaseRejectsConcurrentInstances(t *testing.T) {
	root, err := os.MkdirTemp("/tmp", "polis-cgroup-lease-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(root)
	first, err := acquireLinuxNodeCgroupRootLease(root)
	if err != nil {
		t.Fatalf("acquire first root lease: %v", err)
	}
	defer first.Close()
	if second, secondErr := acquireLinuxNodeCgroupRootLease(root); secondErr == nil {
		_ = second.Close()
		t.Fatal("second Polis instance acquired the same cgroup root")
	}
	if err = first.Close(); err != nil {
		t.Fatal(err)
	}
	third, err := acquireLinuxNodeCgroupRootLease(root)
	if err != nil {
		t.Fatalf("root lease stayed locked after the first owner closed: %v", err)
	}
	if err = third.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestLinuxNodeRuntimeRootIdentityPersistsAndRejectsOtherInstances(t *testing.T) {
	runtimeRoot, err := os.MkdirTemp("/tmp", "polis-cgroup-identity-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(runtimeRoot)
	identity := strings.Repeat("a", 64)
	if err = bindLinuxNodeRuntimeIdentity(runtimeRoot, identity); err != nil {
		t.Fatalf("bind first runtime identity: %v", err)
	}
	if err = bindLinuxNodeRuntimeIdentity(runtimeRoot, identity); err != nil {
		t.Fatalf("same runtime identity could not restart: %v", err)
	}
	if err = bindLinuxNodeRuntimeIdentity(runtimeRoot, strings.Repeat("b", 64)); err == nil {
		t.Fatal("runtime root accepted a different database instance identity")
	}
	info, err := os.Lstat(filepath.Join(runtimeRoot, linuxNodeRuntimeIdentityFile))
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm() != 0600 {
		t.Fatalf("runtime identity marker has unsafe metadata: info=%v error=%v", info, err)
	}
}

func TestLinuxNodeCgroupOwnerMarkerPersistsAcrossRuntimeAliases(t *testing.T) {
	root, err := os.MkdirTemp("/tmp", "polis-cgroup-owner-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(root)
	identity := strings.Repeat("c", 64)
	if err = ensureLinuxNodeCgroupOwner(root, identity); err != nil {
		t.Fatalf("create cgroup-root owner marker: %v", err)
	}
	if err = ensureLinuxNodeCgroupOwner(root, identity); err != nil {
		t.Fatalf("same instance could not resume its cgroup root: %v", err)
	}
	ownerPath := filepath.Join(root, linuxNodeCgroupOwnerDirectoryName(identity))
	procsPath := filepath.Join(ownerPath, "cgroup.procs")
	if err = os.WriteFile(procsPath, nil, 0600); err != nil {
		t.Fatal(err)
	}
	if err = validateLinuxNodeCgroupOwnerEmpty(root, identity); err != nil {
		t.Fatalf("empty owner cgroup rejected: %v", err)
	}
	if err = os.WriteFile(procsPath, []byte("1234\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err = validateLinuxNodeCgroupOwnerEmpty(root, identity); err == nil {
		t.Fatal("owner marker containing a process was accepted as metadata only")
	}
	if err = os.WriteFile(procsPath, nil, 0600); err != nil {
		t.Fatal(err)
	}
	groupPath := filepath.Join(root, linuxCgroupDirectoryPrefix+strings.Repeat("d", 24))
	if err = os.Mkdir(groupPath, 0700); err != nil {
		t.Fatal(err)
	}
	if err = ensureLinuxNodeCgroupOwner(root, strings.Repeat("e", 64)); err == nil {
		t.Fatal("different instance claimed a cgroup root with an existing owner marker")
	}
	if _, err = os.Stat(groupPath); err != nil {
		t.Fatalf("rejected instance changed the existing process group: %v", err)
	}
	children, err := linuxNodeCgroupChildDirectories(root)
	if err != nil || len(children) != 1 || children[0] != groupPath {
		t.Fatalf("work-group scan did not preserve the owner marker separately: children=%v error=%v", children, err)
	}
}

func TestLinuxNodeCgroupOwnerMarkerRefusesUnboundExistingGroups(t *testing.T) {
	root, err := os.MkdirTemp("/tmp", "polis-cgroup-unbound-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(root)
	groupPath := filepath.Join(root, linuxCgroupDirectoryPrefix+strings.Repeat("f", 24))
	if err = os.Mkdir(groupPath, 0700); err != nil {
		t.Fatal(err)
	}
	if err = ensureLinuxNodeCgroupOwner(root, strings.Repeat("1", 64)); err == nil {
		t.Fatal("owner marker was backfilled over an unbound existing process group")
	}
	if _, err = os.Stat(groupPath); err != nil {
		t.Fatalf("unbound process group was changed before owner proof: %v", err)
	}
}

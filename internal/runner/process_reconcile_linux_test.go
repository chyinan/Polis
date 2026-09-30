//go:build linux

// pattern: Imperative Shell

package runner

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestLinuxWorkerCgroupContainmentMetadataRequiresCanonicalLeaf(t *testing.T) {
	valid := `{"host_os":"linux","profile":"linux_worker_cgroup_v2@1","cgroup_id":"polis-worker-` + strings.Repeat("a", 24) + `","cgroup_host_id":"` + strings.Repeat("b", 64) + `","cgroup_root_id":"` + strings.Repeat("c", 64) + `","cgroup_boot_id":"aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"}`
	var metadata ProcessContainmentMetadata
	if err := json.Unmarshal([]byte(valid), &metadata); err != nil {
		t.Fatal(err)
	}
	if !metadata.Valid() {
		t.Fatalf("valid Worker cgroup containment metadata was rejected: %+v", metadata)
	}
	for _, invalid := range []string{
		`{"host_os":"linux","profile":"linux_worker_cgroup_v2@1"}`,
		`{"host_os":"linux","profile":"linux_worker_cgroup_v2@1","cgroup_id":"../outside"}`,
		`{"host_os":"linux","profile":"linux_worker_cgroup_v2@1","cgroup_id":"polis-worker-` + strings.Repeat("a", 24) + `","cgroup_host_id":"` + strings.Repeat("b", 64) + `","cgroup_root_id":"` + strings.Repeat("c", 64) + `"}`,
		`{"host_os":"windows","profile":"linux_worker_cgroup_v2@1","cgroup_id":"polis-worker-` + strings.Repeat("a", 24) + `","cgroup_host_id":"` + strings.Repeat("b", 64) + `","cgroup_root_id":"` + strings.Repeat("c", 64) + `","cgroup_boot_id":"aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"}`,
	} {
		var candidate ProcessContainmentMetadata
		if err := json.Unmarshal([]byte(invalid), &candidate); err != nil {
			t.Fatal(err)
		}
		if candidate.Valid() {
			t.Fatalf("invalid Worker cgroup metadata was accepted: %+v", candidate)
		}
	}
}

type workerCgroupStopFixture struct {
	processID    int
	name         string
	cleanupCalls int
}

func (fixture *workerCgroupStopFixture) FileDescriptor() int { return 1 }
func (fixture *workerCgroupStopFixture) Name() string        { return fixture.name }
func (*workerCgroupStopFixture) CgroupRootPath() string      { return "/sys/fs/cgroup/polis" }
func (*workerCgroupStopFixture) BubblewrapPath() string      { return "/usr/bin/bwrap" }
func (*workerCgroupStopFixture) HostIdentity() string        { return strings.Repeat("b", 64) }
func (*workerCgroupStopFixture) RootIdentity() string        { return strings.Repeat("c", 64) }
func (*workerCgroupStopFixture) BootID() string              { return "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee" }
func (fixture *workerCgroupStopFixture) Cleanup() error {
	fixture.cleanupCalls++
	err := syscall.Kill(-fixture.processID, syscall.SIGKILL)
	if errors.Is(err, syscall.ESRCH) {
		return nil
	}
	return err
}

func TestLinuxWorkerProcessStopRequiresCgroupCleanup(t *testing.T) {
	process, err := Start("linux-worker-cgroup-stop-test", []string{"/bin/sleep", "30"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	name := "polis-worker-" + strings.Repeat("c", 24)
	process.containmentProfile = "linux_worker_cgroup_v2@1"
	process.containmentGroupID = name
	process.containmentHostID = strings.Repeat("b", 64)
	process.containmentRootID = strings.Repeat("c", 64)
	process.containmentBootID = "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"
	group := &workerCgroupStopFixture{processID: process.PID(), name: name}
	process.workerCgroup = group
	proof, err := process.Stop()
	if err != nil {
		t.Fatalf("stop cgroup-contained Worker: %v", err)
	}
	if group.cleanupCalls != 1 || !proof.For(process.id) || !process.ContainmentMetadata().Valid() {
		t.Fatalf("Worker stop did not verify cgroup cleanup: calls=%d proof=%+v containment=%+v", group.cleanupCalls, proof, process.ContainmentMetadata())
	}
}

type invalidWorkerCgroupFixture struct{ cleanupCalls int }

func (fixture *invalidWorkerCgroupFixture) FileDescriptor() int { return -1 }
func (*invalidWorkerCgroupFixture) Name() string                { return "" }
func (*invalidWorkerCgroupFixture) CgroupRootPath() string      { return "" }
func (*invalidWorkerCgroupFixture) BubblewrapPath() string      { return "/usr/bin/bwrap" }
func (*invalidWorkerCgroupFixture) HostIdentity() string        { return strings.Repeat("b", 64) }
func (*invalidWorkerCgroupFixture) RootIdentity() string        { return strings.Repeat("c", 64) }
func (*invalidWorkerCgroupFixture) BootID() string              { return "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee" }
func (fixture *invalidWorkerCgroupFixture) Cleanup() error {
	fixture.cleanupCalls++
	return nil
}

func TestLinuxWorkerCgroupLaunchFailureCleansCreatedGroup(t *testing.T) {
	group := &invalidWorkerCgroupFixture{}
	if _, err := StartWithLaunchSpecInWorkerCgroup("worker-cgroup-invalid-test", ProcessLaunchSpec{}, group); err == nil {
		t.Fatal("invalid Worker cgroup was accepted")
	}
	if group.cleanupCalls != 1 {
		t.Fatalf("invalid Worker cgroup was not cleaned: calls=%d", group.cleanupCalls)
	}
}

func TestLinuxWorkerCgroupLaunchRequiresCgroupHiddenNativeSandbox(t *testing.T) {
	customCgroupRoot := "/srv/polis/runtime/cgroup"
	launch, err := BuildNativeWorkerLaunch("/opt/codex", "/opt/codex-code-mode-host", "/home/codex", "/home/codex/auth.json", "", NativeTransportPolicyNativeDefault, strings.Repeat("a", 64), "/usr/bin/bwrap")
	if err != nil {
		t.Fatal(err)
	}
	if !validLinuxWorkerCgroupSandboxArgv(launch.Args, customCgroupRoot, "/home/codex") {
		t.Fatalf("native Worker sandbox was rejected: %v", launch.Args)
	}
	unsafe := []string{"/bin/sh", "-c", "exec codex app-server"}
	if validLinuxWorkerCgroupSandboxArgv(unsafe, customCgroupRoot, "/home/codex") {
		t.Fatal("direct host Worker command was accepted for cgroup containment")
	}
	withHostCgroup := append([]string(nil), launch.Args...)
	separator := -1
	for index, argument := range withHostCgroup {
		if argument == "--" {
			separator = index
			break
		}
	}
	if separator < 0 {
		t.Fatal("native Worker sandbox has no command boundary")
	}
	withHostCgroup = append(withHostCgroup[:separator], append([]string{"--bind", "/sys/fs/cgroup", "/sys/fs/cgroup"}, withHostCgroup[separator:]...)...)
	if validLinuxWorkerCgroupSandboxArgv(withHostCgroup, customCgroupRoot, "/home/codex") {
		t.Fatal("native Worker sandbox exposed the host cgroup filesystem")
	}
	for _, source := range []string{
		"/sys/fs/cgroup/polis-worker-0123456789abcdef01234567/cgroup.procs",
		"/sys/fs/cgroup/polis-worker-0123456789abcdef01234567/cgroup.subtree_control",
		"/sys/fs/cgroup/../cgroup/polis-worker-0123456789abcdef01234567/cgroup.procs",
		"/sys/cgroup/polis-worker-0123456789abcdef01234567/cgroup.procs",
	} {
		for _, mountFlag := range []string{"--bind", "--bind-try", "--dev-bind", "--dev-bind-try", "--ro-bind", "--ro-bind-try"} {
			withNestedHostCgroup := append([]string(nil), launch.Args...)
			separator = -1
			for index, argument := range withNestedHostCgroup {
				if argument == "--" {
					separator = index
					break
				}
			}
			if separator < 0 {
				t.Fatal("native Worker sandbox has no command boundary")
			}
			withNestedHostCgroup = append(withNestedHostCgroup[:separator], append([]string{mountFlag, source, "/tmp/worker-cgroup-control"}, withNestedHostCgroup[separator:]...)...)
			if validLinuxWorkerCgroupSandboxArgv(withNestedHostCgroup, customCgroupRoot, "/home/codex") {
				t.Fatalf("native Worker sandbox accepted host cgroup control source %q with %s", source, mountFlag)
			}
		}
	}
	withConfiguredRootControl := append([]string(nil), launch.Args...)
	separator = -1
	for index, argument := range withConfiguredRootControl {
		if argument == "--" {
			separator = index
			break
		}
	}
	if separator < 0 {
		t.Fatal("native Worker sandbox has no command boundary")
	}
	withConfiguredRootControl = append(withConfiguredRootControl[:separator], append([]string{"--bind", customCgroupRoot + "/polis-worker-0123456789abcdef01234567/cgroup.procs", "/tmp/cgroup.procs"}, withConfiguredRootControl[separator:]...)...)
	if validLinuxWorkerCgroupSandboxArgv(withConfiguredRootControl, customCgroupRoot, "/home/codex") {
		t.Fatal("native Worker sandbox accepted a control file under the configured cgroup root")
	}
	aliasParent := t.TempDir()
	aliasedCgroupRoot := filepath.Join(aliasParent, "cgroup-root")
	leaf := filepath.Join(aliasedCgroupRoot, "polis-worker-0123456789abcdef01234567")
	if err = os.MkdirAll(leaf, 0700); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(leaf, "cgroup.procs"), []byte("1234\n"), 0600); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(aliasParent, "cg")
	if err = os.Symlink(aliasedCgroupRoot, alias); err != nil {
		t.Fatal(err)
	}
	withCgroupAlias := append([]string(nil), launch.Args...)
	separator = -1
	for index, argument := range withCgroupAlias {
		if argument == "--" {
			separator = index
			break
		}
	}
	if separator < 0 {
		t.Fatal("native Worker sandbox has no command boundary")
	}
	withCgroupAlias = append(withCgroupAlias[:separator], append([]string{"--bind", filepath.Join(alias, filepath.Base(leaf), "cgroup.procs"), "/tmp/cgroup.procs"}, withCgroupAlias[separator:]...)...)
	if validLinuxWorkerCgroupSandboxArgv(withCgroupAlias, aliasedCgroupRoot, "/home/codex") {
		t.Fatal("native Worker sandbox accepted a symlink alias to a host cgroup control file")
	}
	withAliasedHome := append([]string(nil), launch.Args...)
	for index := 0; index+2 < len(withAliasedHome); index++ {
		if withAliasedHome[index] == "--bind" && withAliasedHome[index+2] == "/home/codex" {
			withAliasedHome[index+1] = filepath.Join(alias, filepath.Base(leaf))
			break
		}
	}
	if validLinuxWorkerCgroupSandboxArgv(withAliasedHome, aliasedCgroupRoot, "/home/codex") {
		t.Fatal("native Worker sandbox accepted a cgroup symlink as its writable home bind")
	}
}

func TestReconcileLinuxProcessGroupRefusesStopWhileMemberRemains(t *testing.T) {
	procRoot := t.TempDir()
	writeFakeLinuxProcStat(t, procRoot, 4421, "codex (worker)", 4421)
	proof, err := reconcileLinuxProcessGroupAt(context.Background(), procRoot, "session-4421", 4421)
	if !errors.Is(err, ErrLinuxProcessGroupStopUnconfirmed) || proof.stopped {
		t.Fatalf("active process-group proof=%+v err=%v, want stop-unconfirmed", proof, err)
	}
}

func TestReconcileLinuxProcessGroupProvesEmptyGroupAndRejectsMalformedProcData(t *testing.T) {
	procRoot := t.TempDir()
	proof, err := reconcileLinuxProcessGroupAt(context.Background(), procRoot, "session-4421", 4421)
	if err != nil || !proof.ForWorkerSession("session-4421", 4421) {
		t.Fatalf("empty group proof=%+v err=%v", proof, err)
	}

	writeFakeLinuxProcStat(t, procRoot, 4422, "broken", 4422)
	if err := os.WriteFile(filepath.Join(procRoot, "4422", "stat"), []byte("malformed"), 0o600); err != nil {
		t.Fatal(err)
	}
	if proof, err = reconcileLinuxProcessGroupAt(context.Background(), procRoot, "session-4421", 4421); err == nil || proof.stopped {
		t.Fatalf("malformed proc data produced proof=%+v err=%v, want fail-closed error", proof, err)
	}
}

func TestReconcileLinuxWorkerProcessGroupWaitsForDescendants(t *testing.T) {
	processID := "linux-process-group-recovery-test"
	process, err := Start(processID, []string{"/bin/sh", "-c", "sleep 30 & wait"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if members, countErr := linuxProcessGroupMemberCount(context.Background(), "/proc", process.PID()); countErr == nil && members > 0 {
			_ = syscall.Kill(-process.PID(), syscall.SIGKILL)
		}
		_, _ = process.Stop()
	}()
	deadline := time.Now().Add(3 * time.Second)
	for {
		members, countErr := linuxProcessGroupMemberCount(context.Background(), "/proc", process.PID())
		if countErr != nil {
			t.Fatal(countErr)
		}
		if members >= 2 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("test child did not join process group; members=%d", members)
		}
		time.Sleep(10 * time.Millisecond)
	}

	if _, err = ReconcileLinuxWorkerProcessGroup(context.Background(), processID, process.PID()); !errors.Is(err, ErrLinuxProcessGroupStopUnconfirmed) {
		t.Fatalf("active worker process group error=%v, want stop-unconfirmed", err)
	}
	if _, err = process.Stop(); !errors.Is(err, ErrProcessTreeStopUnconfirmed) {
		t.Fatalf("stop proof with a live descendant=%v, want process-tree stop-unconfirmed", err)
	}
	if members, countErr := linuxProcessGroupMemberCount(context.Background(), "/proc", process.PID()); countErr != nil || members < 1 {
		t.Fatalf("test child process was not retained after leader stop: members=%d error=%v", members, countErr)
	}
	if err = syscall.Kill(-process.PID(), syscall.SIGKILL); err != nil && !errors.Is(err, syscall.ESRCH) {
		t.Fatalf("kill remaining test process group: %v", err)
	}
	deadline = time.Now().Add(3 * time.Second)
	var proof LinuxProcessGroupStopProof
	for {
		proof, err = ReconcileLinuxWorkerProcessGroup(context.Background(), processID, process.PID())
		if err == nil {
			break
		}
		if !errors.Is(err, ErrLinuxProcessGroupStopUnconfirmed) {
			t.Fatalf("stopped worker process group proof=%+v error=%v", proof, err)
		}
		if time.Now().After(deadline) {
			t.Fatalf("stopped worker process group was not reaped: proof=%+v error=%v", proof, err)
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !proof.ForWorkerSession(processID, process.PID()) {
		t.Fatalf("stopped worker process group proof=%+v does not match session", proof)
	}
}

func TestReconcileLinuxWorkerProcessGroupRejectsEmptyProcSnapshotWhileGroupExists(t *testing.T) {
	procRoot := t.TempDir()
	processID := "linux-process-group-proc-snapshot-race-test"
	process, err := Start(processID, []string{"/bin/sleep", "30"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if _, stopErr := process.Stop(); stopErr != nil && !errors.Is(stopErr, os.ErrProcessDone) {
			t.Errorf("stop test process: %v", stopErr)
		}
	}()

	proof, err := reconcileLinuxProcessGroupAt(context.Background(), procRoot, processID, process.PID())
	if proof.stopped || !errors.Is(err, ErrLinuxProcessGroupStopUnconfirmed) {
		t.Fatalf("empty proc snapshot produced proof=%+v error=%v, want kernel group probe to keep it unresolved", proof, err)
	}
}

func writeFakeLinuxProcStat(t *testing.T, root string, pid int, command string, processGroupID int) {
	t.Helper()
	processDirectory := filepath.Join(root, strconv.Itoa(pid))
	if err := os.MkdirAll(processDirectory, 0o700); err != nil {
		t.Fatal(err)
	}
	stat := fmt.Sprintf("%d (%s) S 1 %d 1 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0\n", pid, command, processGroupID)
	if err := os.WriteFile(filepath.Join(processDirectory, "stat"), []byte(stat), 0o600); err != nil {
		t.Fatal(err)
	}
}

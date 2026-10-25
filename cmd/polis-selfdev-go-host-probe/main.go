// Command polis-selfdev-go-host-probe runs trusted, synthetic host checks.
// It never reads or executes candidate project files.
package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const memoryLimitBytes = 2 << 30

func fail(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}

func cgroupDirectory() string {
	raw, err := os.ReadFile("/proc/self/cgroup")
	if err != nil {
		fail("cgroup_unavailable")
	}
	for _, line := range strings.Split(string(raw), "\n") {
		if strings.HasPrefix(line, "0::") {
			path := strings.TrimPrefix(line, "0::")
			if !strings.HasPrefix(path, "/system.slice/polis-self-dev-go-probe@probe-") || filepath.Clean(path) != path {
				fail("unexpected_probe_cgroup")
			}
			return filepath.Join("/sys/fs/cgroup", strings.TrimPrefix(path, "/"))
		}
	}
	fail("unified_cgroup_unavailable")
	return ""
}

func readCounter(path, key string) uint64 {
	raw, err := os.ReadFile(path)
	if err != nil {
		fail("cgroup_counter_unavailable:%s", filepath.Base(path))
	}
	for _, line := range strings.Split(string(raw), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && fields[0] == key {
			value, err := strconv.ParseUint(fields[1], 10, 64)
			if err == nil {
				return value
			}
		}
	}
	fail("cgroup_counter_missing:%s", key)
	return 0
}

func readValue(path string) string {
	raw, err := os.ReadFile(path)
	if err != nil {
		fail("cgroup_value_unavailable:%s", filepath.Base(path))
	}
	return strings.TrimSpace(string(raw))
}

func allocateBeyondLimit() {
	buffer := make([]byte, memoryLimitBytes+(128<<20))
	for index := 0; index < len(buffer); index += 4096 {
		buffer[index] = byte(index)
	}
	runtime.KeepAlive(buffer)
	// Reaching this line means the cgroup did not enforce its configured cap.
	time.Sleep(time.Second)
}

func verifyMemoryLimit() {
	cgroup := cgroupDirectory()
	if readValue(filepath.Join(cgroup, "memory.max")) != strconv.FormatInt(memoryLimitBytes, 10) || readValue(filepath.Join(cgroup, "memory.swap.max")) != "0" {
		fail("memory_or_swap_limit_mismatch")
	}
	events := filepath.Join(cgroup, "memory.events")
	maxBefore, oomBefore, killedBefore := readCounter(events, "max"), readCounter(events, "oom"), readCounter(events, "oom_kill")
	if readValue(filepath.Join(cgroup, "memory.swap.current")) != "0" {
		fail("swap_in_use_before_probe")
	}
	executable, err := os.Executable()
	if err != nil {
		fail("probe_executable_unavailable")
	}
	child := exec.Command(executable, "allocate-beyond-limit")
	child.Env = []string{"PATH=/usr/bin:/bin", "LANG=C", "GOMAXPROCS=1"}
	child.Stdout, child.Stderr = nil, nil
	if err = child.Run(); err == nil {
		fail("memory_allocation_exceeded_configured_limit")
	}
	maxAfter, oomAfter, killedAfter := readCounter(events, "max"), readCounter(events, "oom"), readCounter(events, "oom_kill")
	if maxAfter <= maxBefore || (oomAfter <= oomBefore && killedAfter <= killedBefore) {
		fail("memory_limit_not_observed_by_kernel")
	}
	if readValue(filepath.Join(cgroup, "memory.swap.current")) != "0" {
		fail("swap_was_used_under_memory_pressure")
	}
	fmt.Printf("memory_boundary=pass;max_events=%d;oom_events=%d;oom_kill_events=%d;swap_current=0\n", maxAfter-maxBefore, oomAfter-oomBefore, killedAfter-killedBefore)
}

func spawnPIDs() {
	children := make([]*exec.Cmd, 0, 300)
	stopChildren := func() {
		for _, child := range children {
			if child.Process != nil {
				_ = child.Process.Kill()
			}
		}
		for _, child := range children {
			_ = child.Wait()
		}
	}
	for index := 0; index < 300; index++ {
		child := exec.Command("/bin/sleep", "30")
		if err := child.Start(); err != nil {
			if !errors.Is(err, syscall.EAGAIN) || len(children) < 200 {
				stopChildren()
				fail("pids_probe_spawn_failed:%v", err)
			}
			spawned := len(children)
			stopChildren()
			fmt.Printf("pids_spawn_boundary=pass;spawned=%d;limit_hit=1\n", spawned)
			return
		}
		children = append(children, child)
	}
	stopChildren()
	fail("pids_limit_not_enforced")
}

func verifyPIDsLimit() {
	cgroup := cgroupDirectory()
	events := filepath.Join(cgroup, "pids.events")
	before := readCounter(events, "max")
	executable, err := os.Executable()
	if err != nil {
		fail("probe_executable_unavailable")
	}
	child := exec.Command(executable, "spawn-pids")
	child.Env = []string{"PATH=/usr/bin:/bin", "LANG=C", "GOMAXPROCS=1"}
	output, err := child.CombinedOutput()
	if err != nil || !strings.Contains(string(output), "pids_spawn_boundary=pass;") {
		fail("pids_probe_failed:%s", strings.TrimSpace(string(output)))
	}
	after := readCounter(events, "max")
	if after <= before {
		fail("pids_limit_not_observed_by_kernel")
	}
	fmt.Printf("pids_boundary=pass;max_events=%d;%s", after-before, strings.TrimPrefix(string(output), "pids_spawn_boundary=pass;"))
}

func mustBeAbsent(path string) {
	_, err := os.Lstat(path)
	if !errors.Is(err, os.ErrNotExist) {
		fail("host_path_visible:%s", path)
	}
}

func mustRejectWrite(path string) {
	if err := os.WriteFile(path, []byte("forbidden"), 0600); err == nil {
		_ = os.Remove(path)
		fail("read_only_path_writable:%s", path)
	}
}

func verifyPathAndNetworkBoundary() {
	input, err := os.ReadFile("/src/probe-input.txt")
	if err != nil || string(input) != "trusted host probe input\n" {
		fail("read_only_source_unavailable")
	}
	for _, path := range []string{
		"/etc/passwd",
		"/home/chyinan",
		"/mnt/d/Programs/Polis-cloud-main",
		"/run/polis-self-dev-go-job",
	} {
		mustBeAbsent(path)
	}
	for _, path := range []string{
		"/src/probe-write-check",
		"/go/probe-write-check",
		"/deps/gomodcache/probe-write-check",
	} {
		mustRejectWrite(path)
	}
	if _, err = os.Stat("/go/bin/go"); err != nil {
		fail("pinned_toolchain_mount_missing")
	}
	if info, err := os.Stat("/deps/gomodcache"); err != nil || !info.IsDir() {
		fail("dependency_mount_missing")
	}
	if err = os.WriteFile("/scratch/probe-write-check", []byte("scratch\n"), 0600); err != nil {
		fail("bounded_scratch_not_writable")
	}
	if err = os.Remove("/scratch/probe-write-check"); err != nil {
		fail("bounded_scratch_cleanup_failed")
	}
	if _, err = os.ReadFile("/src/escape-link"); !errors.Is(err, os.ErrNotExist) {
		fail("source_symlink_escaped_sandbox")
	}
	hostNetNS, err := os.ReadFile("/src/host-netns")
	if err != nil {
		fail("host_network_identity_missing")
	}
	currentNetNS, err := os.Readlink("/proc/self/ns/net")
	if err != nil || strings.TrimSpace(string(hostNetNS)) == currentNetNS {
		fail("network_namespace_not_isolated")
	}
	interfaces, err := os.ReadFile("/proc/net/dev")
	if err != nil {
		fail("sandbox_network_interfaces_unavailable")
	}
	seenLoopback := false
	for _, line := range strings.Split(string(interfaces), "\n") {
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		name := strings.TrimSpace(parts[0])
		if name == "lo" {
			seenLoopback = true
		} else {
			fail("unexpected_sandbox_network_interface:%s", name)
		}
	}
	if !seenLoopback {
		fail("sandbox_loopback_missing")
	}
	fmt.Println("path_boundary=pass;network_namespace=pass;network_interfaces=loopback_only")
}

func main() {
	if len(os.Args) == 2 && os.Args[1] == "--version" {
		fmt.Println("polis-selfdev-go-host-probe@1")
		return
	}
	if len(os.Args) != 2 {
		fail("usage: polis-selfdev-go-host-probe pids|memory|boundary|allocate-beyond-limit|spawn-pids")
	}
	switch os.Args[1] {
	case "pids":
		verifyPIDsLimit()
	case "memory":
		verifyMemoryLimit()
	case "boundary":
		verifyPathAndNetworkBoundary()
	case "allocate-beyond-limit":
		allocateBeyondLimit()
	case "spawn-pids":
		spawnPIDs()
	default:
		fail("unknown_probe_mode")
	}
}

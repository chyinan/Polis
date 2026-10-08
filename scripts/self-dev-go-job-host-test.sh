#!/usr/bin/env bash
set -euo pipefail
repo_root="$(cd "$(dirname "$0")/.." && pwd -P)"
host_tool="$repo_root/scripts/self-dev-go-job-host.sh"

fail() {
  printf 'FAIL: %s\n' "$1" >&2
  exit 1
}

if [ ! -x "$host_tool" ]; then
  fail "host preparation tool is missing or not executable"
fi

inspect_output="$(bash "$host_tool" inspect)"
for field in distribution kernel init_systemd cgroup2_mount available_controllers delegated_controllers cgroup_kill scratch_filesystem scratch_bytes_available scratch_inodes_available bwrap privilege_gaps; do
  if ! grep -q "^${field}=" <<<"$inspect_output"; then
    fail "inspect omitted ${field}"
  fi
done

prepare_output="$(bash "$host_tool" prepare)"
grep -q '^changes_applied=no$' <<<"$prepare_output" || fail "prepare must be plan-only by default"
grep -q '/etc/systemd/system/polis-self-dev-go-job@.service' <<<"$prepare_output" || fail "prepare omitted the fixed systemd job template path"
grep -q 'TemporaryFileSystem=/scratch' <<<"$prepare_output" || fail "prepare omitted the per-job scratch tmpfs"
grep -q 'cgroup.kill' <<<"$prepare_output" || fail "prepare omitted the cgroup.kill verification requirement"
grep -q 'nr_inodes=4096' <<<"$prepare_output" || fail "prepare omitted the scratch inode cap"

root_helper="$repo_root/scripts/self-dev-go-job-host-root.sh"
root_plan="$(bash "$root_helper" plan --target-uid "$(id -u)")"
grep -q '^action=plan_only$' <<<"$root_plan" || fail "root helper plan must be read-only"
grep -q '^root_required=yes$' <<<"$root_plan" || fail "root helper plan omitted its privilege boundary"
grep -q 'MemorySwapMax=0' <<<"$root_plan" || fail "root helper plan omitted the swap limit"
grep -q 'TasksMax=256' <<<"$root_plan" || fail "root helper plan omitted the task limit"
grep -q 'nr_inodes=4096' <<<"$root_plan" || fail "root helper plan omitted the inode limit"
grep -q 'cgroup_management=systemd-managed; no direct cgroupfs writes' <<<"$root_plan" || fail "root helper must not write cgroupfs directly"
if grep -q '/etc/sudoers' <<<"$root_plan"; then
  fail "root helper plan must not modify sudoers"
fi

if bash "$host_tool" cleanup '../all' >/dev/null 2>&1; then
  fail "cleanup accepted a path-like job identifier"
fi
if bash "$host_tool" cleanup 'job-not-created' >/dev/null 2>&1; then
  fail "cleanup accepted an unknown job identifier"
fi

if verify_output="$(bash "$host_tool" verify 2>&1)"; then
  fail "verify unexpectedly qualified the current unprepared host"
else
  status=$?
  if [ "$status" -ne 2 ]; then
    fail "verify returned ${status}; expected structured blocked status 2"
  fi
  grep -q '^qualification=blocked$' <<<"$verify_output" || fail "verify omitted blocked status"
  grep -q 'candidate_execution=not_run' <<<"$verify_output" || fail "verify did not state that candidate code was not run"
  grep -q '^trusted_bwrap_network_path_probe=pass$' <<<"$verify_output" || fail "trusted bwrap namespace/read-only probe did not pass"
  grep -q '^resource_probe=systemd_probe_unit_missing$' <<<"$verify_output" || fail "verify did not identify the exact missing host preparation"
fi

printf 'PASS: host CLI is read-only in inspect/prepare and fails closed on verify/cleanup inputs\n'

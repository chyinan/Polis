#!/usr/bin/env bash
set -euo pipefail
repo_root="$(cd "$(dirname "$0")/.." && pwd -P)"
host_tool="$repo_root/scripts/self-dev-go-job-host.sh"
host_probe="$repo_root/bin/polis-selfdev-go-host-probe"
unit_control="$repo_root/bin/polis-selfdev-go-unit-control"

fail() {
  printf 'FAIL: %s\n' "$1" >&2
  exit 1
}

if [ ! -x "$host_tool" ]; then
  fail "host preparation tool is missing or not executable"
fi
if [ ! -x "$host_probe" ] || [ "$("$host_probe" --version 2>/dev/null || true)" != 'polis-selfdev-go-host-probe@1' ]; then
  fail "trusted host qualification probe binary is missing"
fi
if [ ! -x "$unit_control" ] || [ "$("$unit_control" --protocol-info 2>/dev/null || true)" != 'polis-selfdev-go-unit-control@1' ]; then
  fail "fixed unit-control broker build is missing"
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
grep -q '^apply_ready=yes$' <<<"$prepare_output" || fail "prepare must verify the real built supervisor artifact"
grep -q '^supervisor_command_source=present$' <<<"$prepare_output" || fail "prepare omitted the supervisor production entry"
grep -q '^supervisor_binary=present$' <<<"$prepare_output" || fail "prepare did not consume the supervisor binary"
grep -q '^unit_control_binary=present$' <<<"$prepare_output" || fail "prepare omitted the fixed unit-control broker"
grep -q '^privileged_apply_authorized=no$' <<<"$prepare_output" || fail "building a binary must not authorize privileged installation"
"$repo_root/bin/polis-selfdev-go-job-supervisor" --help >/dev/null 2>&1 || fail "supervisor --help failed"
if "$repo_root/bin/polis-selfdev-go-job-supervisor" --job-id=../escape >/dev/null 2>&1; then fail "supervisor accepted an escaping job id"; fi

root_helper="$repo_root/scripts/self-dev-go-job-host-root.sh"
root_plan="$(bash "$root_helper" plan --target-uid "$(id -u)")"
grep -q '^action=plan_only$' <<<"$root_plan" || fail "root helper plan must be read-only"
grep -q '^root_required=yes$' <<<"$root_plan" || fail "root helper plan omitted its privilege boundary"
grep -q '^source_supervisor=/mnt/d/Programs/Polis-cloud-main/bin/polis-selfdev-go-job-supervisor$' <<<"$root_plan" || fail "root helper plan did not preserve the canonical WSL repository path"
grep -q '^source_unit_control=/mnt/d/Programs/Polis-cloud-main/bin/polis-selfdev-go-unit-control$' <<<"$root_plan" || fail "root helper plan omitted the fixed unit-control broker source"
grep -q '^job_unit_execstart=/usr/local/libexec/polis-selfdev-go-job-supervisor --job-id=%i --toolchain-root=/run/polis-self-dev-go-toolchain$' <<<"$root_plan" || fail "job unit plan did not use the qualified read-only toolchain stage"
grep -q '/usr/share/polkit-1/actions/org.polis.selfdev.go-job.policy' <<<"$root_plan" || fail "root helper plan omitted the custom pkexec actions"
grep -q '/etc/polkit-1/localauthority/50-local.d/50-polis-selfdev-go-unit-control.pkla' <<<"$root_plan" || fail "root helper plan omitted the exact local-authority grant"
grep -q 'polkit_scope=one pkexec action bound to the unit-control helper path' <<<"$root_plan" || fail "root helper plan omitted the path-bound authorization boundary"
grep -q '^source_unit_control=/mnt/d/Programs/Polis-cloud-main/bin/polis-selfdev-go-unit-control$' <<<"$root_plan" || fail "root helper plan omitted the canonical unit-control broker path"
grep -q 'MemorySwapMax=0' <<<"$root_plan" || fail "root helper plan omitted the swap limit"
grep -q 'TasksMax=256' <<<"$root_plan" || fail "root helper plan omitted the task limit"
grep -q 'probe_only=OOMPolicy=continue' <<<"$root_plan" || fail "root helper plan omitted probe-only OOM observation policy"
grep -q 'nr_inodes=4096' <<<"$root_plan" || fail "root helper plan omitted the inode limit"
grep -q '/scratch (empty root:root 0755 mountpoint; per-unit tmpfs overlays it)' <<<"$root_plan" || fail "root helper plan omitted the empty private-tmpfs mountpoint"
grep -q 'cgroup_management=systemd-managed; no direct cgroupfs writes' <<<"$root_plan" || fail "root helper must not write cgroupfs directly"
if grep -q 'org.freedesktop.systemd1.manage-units' <<<"$root_plan"; then
  fail "root helper plan must not grant generic systemd unit control"
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
  grep -q '^trusted_bwrap_boundary_probe=pass$' <<<"$verify_output" || fail "trusted bwrap path/network boundary probe did not pass"
  grep -Eq '^resource_probe=(systemd_probe_unit_missing|systemd_probe_unit_start_failed|probe_did_not_finish)$' <<<"$verify_output" || fail "verify did not identify the exact blocked host probe boundary"
fi

printf 'PASS: host CLI is read-only in inspect/prepare and fails closed on verify/cleanup inputs\n'

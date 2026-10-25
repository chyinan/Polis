#!/usr/bin/env bash
set -euo pipefail

readonly required_controllers='cpu memory pids'
readonly root_helper_name='self-dev-go-job-host-root.sh'
readonly unit_prefix='polis-self-dev-go-job@'
readonly probe_prefix='polis-self-dev-go-probe@'

usage() {
  cat <<'USAGE'
usage:
  self-dev-go-job-host.sh inspect
  self-dev-go-job-host.sh prepare
  self-dev-go-job-host.sh verify
  self-dev-go-job-host.sh cleanup <job-id>
  self-dev-go-job-host.sh probe-unit <probe-id>   # systemd probe unit only
USAGE
}

one_line() {
  tr '\n\t ' ',,,' | sed 's/,,*/,/g; s/^,//; s/,$//'
}

value_or() {
  local value="$1"
  local fallback="$2"
  if [[ -n "$value" ]]; then
    printf '%s' "$value"
  else
    printf '%s' "$fallback"
  fi
}

current_uid() {
  id -u
}

current_user_cgroup() {
  local uid="$1"
  local control_group
  control_group="$(systemctl --user show --property=ControlGroup --value 2>/dev/null || true)"
  control_group="${control_group//$'\n'/}"
  if [[ "$control_group" == "/user.slice/user-${uid}.slice/user@${uid}.service"* ]]; then
    printf '/sys/fs/cgroup%s' "$control_group"
  fi
}

inspect() {
  local uid user_cgroup root_mount user_mount user_available user_delegated
  local user_subtree system_cgroup system_available system_subtree
  local cgroup_kill cgroup_writable systemd_state init_name distro bwrap_version
  local scratch_target scratch_source scratch_fstype scratch_options scratch_bytes scratch_inodes gaps
  uid="$(current_uid)"
  distro="${WSL_DISTRO_NAME:-unknown}"
  init_name="$(ps -p 1 -o comm= 2>/dev/null | tr -d '[:space:]' || true)"
  systemd_state="$(systemctl --user is-system-running 2>/dev/null || true)"
  root_mount="$(findmnt -n -t cgroup2 -o TARGET,FSTYPE,OPTIONS 2>/dev/null | head -n 1 || true)"
  user_cgroup="$(current_user_cgroup "$uid")"
  user_mount=''
  user_available=''
  user_delegated=''
  user_subtree=''
  system_cgroup=''
  system_available=''
  system_subtree=''
  cgroup_kill='missing'
  cgroup_writable='no'
  if [[ -n "$user_cgroup" && -d "$user_cgroup" && ! -L "$user_cgroup" ]]; then
    user_available="$(cat "$user_cgroup/cgroup.controllers" 2>/dev/null || true)"
    user_subtree="$(cat "$user_cgroup/cgroup.subtree_control" 2>/dev/null || true)"
    user_delegated="$user_subtree"
    cgroup_writable='no'
    [[ -w "$user_cgroup" ]] && cgroup_writable='yes'
    if [[ -e "$user_cgroup/cgroup.kill" ]]; then
      cgroup_kill='present-not-writable'
      [[ -w "$user_cgroup/cgroup.kill" ]] && cgroup_kill='present-writable'
    fi
    user_mount="$user_cgroup"
  fi
  system_cgroup="$(systemctl show --property=ControlGroup --value system.slice 2>/dev/null || true)"
  if [[ "$system_cgroup" == /* && -d "/sys/fs/cgroup${system_cgroup}" && ! -L "/sys/fs/cgroup${system_cgroup}" ]]; then
    system_available="$(cat "/sys/fs/cgroup${system_cgroup}/cgroup.controllers" 2>/dev/null || true)"
    system_subtree="$(cat "/sys/fs/cgroup${system_cgroup}/cgroup.subtree_control" 2>/dev/null || true)"
  else
    system_cgroup=''
  fi
  bwrap_version="$(bwrap --version 2>/dev/null || true)"
  scratch_target='/tmp'
  scratch_source="$(findmnt -n -T "$scratch_target" -o SOURCE 2>/dev/null || true)"
  scratch_fstype="$(findmnt -n -T "$scratch_target" -o FSTYPE 2>/dev/null || true)"
  scratch_options="$(findmnt -n -T "$scratch_target" -o OPTIONS 2>/dev/null || true)"
  scratch_bytes="$(df -B1 --output=avail "$scratch_target" 2>/dev/null | tail -n 1 | tr -d ' ' || true)"
  scratch_inodes="$(df -i "$scratch_target" 2>/dev/null | awk 'NR==2 {print $4}' | tr -d ' ' || true)"
  gaps=()
  [[ "$init_name" == 'systemd' && "$systemd_state" == 'running' ]] || gaps+=(systemd_user_manager_unavailable)
  [[ "$root_mount" == *cgroup2* ]] || gaps+=(cgroup_v2_mount_unavailable)
  [[ " $user_available " == *" cpu "* ]] || gaps+=(cpu_controller_not_available_to_user_scope)
  for controller in $required_controllers; do
    [[ " $user_delegated " == *" $controller "* ]] || gaps+=("${controller}_controller_not_delegated")
  done
  [[ "$cgroup_writable" == 'yes' ]] || gaps+=(dedicated_user_cgroup_not_writable)
  [[ "$cgroup_kill" == 'present-writable' ]] || gaps+=(cgroup_kill_not_writable_in_user_scope)
  [[ "$scratch_fstype" == 'tmpfs' ]] || gaps+=(per_job_scratch_tmpfs_not_active)
  [[ "$bwrap_version" == 'bubblewrap 0.6.1' || -n "$bwrap_version" ]] || gaps+=(bubblewrap_unavailable)

  printf 'distribution=%s\n' "$distro"
  printf 'kernel=%s\n' "$(uname -srmo 2>/dev/null || echo unavailable)"
  printf 'init_systemd=%s/%s\n' "$(value_or "$init_name" unavailable)" "$(value_or "$systemd_state" unavailable)"
  printf 'cgroup2_mount=%s\n' "$(value_or "$root_mount" unavailable)"
  printf 'available_controllers=%s\n' "$(value_or "$( (cat /sys/fs/cgroup/cgroup.controllers 2>/dev/null || true) | one_line)" unavailable)"
  printf 'root_subtree_control=%s\n' "$(value_or "$( (cat /sys/fs/cgroup/cgroup.subtree_control 2>/dev/null || true) | one_line)" none)"
  printf 'system_slice=%s\n' "$(value_or "$system_cgroup" unavailable)"
  printf 'system_slice_available_controllers=%s\n' "$(value_or "$(printf '%s' "$system_available" | one_line)" unavailable)"
  printf 'system_slice_subtree_control=%s\n' "$(value_or "$(printf '%s' "$system_subtree" | one_line)" none)"
  printf 'user_cgroup=%s\n' "$(value_or "$user_mount" unavailable)"
  printf 'user_available_controllers=%s\n' "$(value_or "$(printf '%s' "$user_available" | one_line)" unavailable)"
  printf 'user_subtree_control=%s\n' "$(value_or "$(printf '%s' "$user_subtree" | one_line)" none)"
  printf 'delegated_controllers=%s\n' "$(value_or "$(printf '%s' "$user_delegated" | one_line)" none)"
  printf 'user_cgroup_writable=%s\n' "$cgroup_writable"
  printf 'cgroup_kill=%s\n' "$cgroup_kill"
  printf 'bwrap=%s\n' "$(value_or "$bwrap_version" unavailable)"
  printf 'scratch_target=%s\n' "$scratch_target"
  printf 'scratch_source=%s\n' "$(value_or "$scratch_source" unavailable)"
  printf 'scratch_filesystem=%s\n' "$(value_or "$scratch_fstype" unavailable)"
  printf 'scratch_options=%s\n' "$(value_or "$(printf '%s' "$scratch_options" | one_line)" unavailable)"
  printf 'scratch_bytes_available=%s\n' "$(value_or "$scratch_bytes" unavailable)"
  printf 'scratch_inodes_available=%s\n' "$(value_or "$scratch_inodes" unavailable)"
  printf 'scratch_quota=not_verified_for_job\n'
  printf 'probe_profile=polis-go-test-fixture@1\n'
  printf 'system_service_profile=not_prepared\n'
  printf 'privilege_gaps=%s\n' "$(IFS=,; echo "${gaps[*]:-none}")"
}

prepare() {
  local uid user helper repo_root supervisor_source supervisor_binary supervisor_command_source supervisor_binary_state host_probe_binary host_probe_state unit_control_binary unit_control_state apply_ready apply_command
  uid="$(current_uid)"
  user="$(id -un)"
  helper="$(cd "$(dirname "$0")" && pwd -P)/$root_helper_name"
  repo_root="$(cd "$(dirname "$0")/.." && pwd -P)"
  supervisor_source="$repo_root/cmd/polis-selfdev-go-job-supervisor/main.go"
  supervisor_binary="$repo_root/bin/polis-selfdev-go-job-supervisor"
  host_probe_binary="$repo_root/bin/polis-selfdev-go-host-probe"
  unit_control_binary="$repo_root/bin/polis-selfdev-go-unit-control"
  if [[ -f "$supervisor_source" && ! -L "$supervisor_source" ]]; then
    supervisor_command_source=present
  else
    supervisor_command_source=missing
  fi
  if [[ -f "$supervisor_binary" && -x "$supervisor_binary" && ! -L "$supervisor_binary" ]] && [[ "$(od -An -tx1 -N5 "$supervisor_binary" | tr -d ' \n')" == 7f454c4602 ]]; then
    supervisor_binary_state=present
  else
    supervisor_binary_state=missing
  fi
  if [[ -f "$host_probe_binary" && -x "$host_probe_binary" && ! -L "$host_probe_binary" ]] && [[ "$("$host_probe_binary" --version 2>/dev/null || true)" == 'polis-selfdev-go-host-probe@1' ]]; then
    host_probe_state=present
  else
    host_probe_state=missing
  fi
  if [[ -f "$unit_control_binary" && -x "$unit_control_binary" && ! -L "$unit_control_binary" ]] && [[ "$("$unit_control_binary" --protocol-info 2>/dev/null || true)" == 'polis-selfdev-go-unit-control@1' ]]; then
    unit_control_state=present
  else
    unit_control_state=missing
  fi
  if [[ "$supervisor_command_source" == present && "$supervisor_binary_state" == present && "$host_probe_state" == present && "$unit_control_state" == present ]] && timeout -k 1s 30s "$supervisor_binary" --verify-build "$supervisor_binary.identity.json" --source-root "$repo_root" >/dev/null 2>&1; then
    apply_ready=yes
    apply_command="sudo bash $helper apply --target-uid $uid --confirm"
  else
    apply_ready=no
    apply_command=not_available_until_supervisor_build
  fi
  cat <<PLAN
changes_applied=no
prepare_mode=plan_only
target_distribution=${WSL_DISTRO_NAME:-unknown}
target_uid=${uid}
target_user=${user}
root_required=yes
supervisor_source_path=${supervisor_source}
supervisor_command_source=${supervisor_command_source}
supervisor_binary_path=${supervisor_binary}
supervisor_binary=${supervisor_binary_state}
host_probe_binary=${host_probe_state}
host_probe_binary_sha256=$(if [[ "$host_probe_state" == present ]]; then sha256sum "$host_probe_binary" | awk '{print $1}'; else echo unavailable; fi)
unit_control_binary=${unit_control_state}
unit_control_binary_sha256=$(if [[ "$unit_control_state" == present ]]; then sha256sum "$unit_control_binary" | awk '{print $1}'; else echo unavailable; fi)
apply_ready=${apply_ready}
setup_plan_valid=${apply_ready}
supervisor_binary_built=${apply_ready}
privileged_apply_authorized=no
host_execution_profile_qualified=no
exact_files:
  /usr/local/libexec (create 0755 only if absent; retain the shared parent on cleanup)
qualified_toolchain=/run/polis-self-dev-go-toolchain (root:root, read-only, and content-bound by the boot receipt)
  /etc/systemd/system/polis-self-dev-go-job@.service
  /etc/systemd/system/polis-self-dev-go-probe@.service
  /usr/share/polkit-1/actions/org.polis.selfdev.go-job.policy
  /etc/polkit-1/localauthority/50-local.d/50-polis-selfdev-go-unit-control.pkla
  /usr/local/libexec/polis-selfdev-go-job-supervisor (built from this checkout)
  /usr/local/libexec/polis-selfdev-go-job-supervisor.identity.json
  /usr/local/libexec/polis-selfdev-go-unit-control (fixed pkexec custom-action helper)
  /etc/systemd/system/polis-self-dev-go-unit-control.sha256
  /etc/systemd/system/polis-self-dev-go-job-supervisor.sha256
  /run/polis-self-dev-go-job (0700; request/result handoff)
  /scratch (empty root:root 0755 per-unit tmpfs mountpoint; cleanup only by exact empty rmdir)
resource_settings:
  CPUQuota=200%; CPUQuotaPeriodSec=100ms
  MemoryMax=2147483648; MemorySwapMax=0
  TasksMax=256 (pids.max counts all tasks, including processes and threads)
  RuntimeMaxSec=120s; KillMode=control-group
  termination=systemd-owned per-job cgroup; verify cgroup.kill presence and prove all child PIDs gone after stop
  TemporaryFileSystem=/scratch:rw,size=1073741824,nr_inodes=4096,mode=0700,uid=${uid},gid=$(id -g),nodev,nosuid
  nr_inodes=4096 caps all filesystem inodes on this per-job tmpfs (files, directories and links), not just regular files
impact=only this WSL distribution's dedicated self-dev Go JobRun system units and their systemd-created per-job cgroups; no global WSL config, physical disk, firewall, sudoers, or running Polis change
polkit=one custom action bound to the unit-control helper path; helper allows read-only seal verification and fixed job-start/job-stop; no systemd manage-units grant
rollback=stop exact probe/job units, then run: sudo bash ${helper} cleanup --target-uid ${uid} --confirm
activation=systemd daemon-reload; actual controller and quota availability must still pass the probe before candidate execution
prerequisite_command=sudo install -d -m 0755 /usr/local/libexec (review absent directories first; require existing polkit action and local-authority directories)
apply_command=${apply_command}
verify_command=bash $(cd "$(dirname "$0")" && pwd -P)/self-dev-go-job-host.sh verify
PLAN
}

run_local_bwrap_probe() {
  local probe_root source_dir deps_dir scratch_dir repo_root toolchain_root token status
  probe_root="$(mktemp -d /tmp/polis-self-dev-go-host-probe.XXXXXX)" || return 1
  source_dir="$probe_root/source"
  deps_dir="$probe_root/deps"
  scratch_dir="$probe_root/scratch"
  repo_root="$(cd "$(dirname "$0")/.." && pwd -P)"
  toolchain_root="$repo_root/.tools/go"
  token="polis-self-dev-probe-$(date +%s)-$$"
  mkdir -m 0700 "$source_dir" "$deps_dir" "$scratch_dir"
  [[ -x "$repo_root/bin/polis-selfdev-go-host-probe" ]] || return 1
  cp "$repo_root/bin/polis-selfdev-go-host-probe" "$source_dir/polis-selfdev-go-host-probe"
  chmod 0555 "$source_dir/polis-selfdev-go-host-probe"
  printf 'trusted host probe input\n' > "$source_dir/probe-input.txt"
  readlink /proc/self/ns/net > "$source_dir/host-netns"
  ln -s /mnt/d/Programs/Polis-cloud-main/go.mod "$source_dir/escape-link"
  if timeout -k 1s 8s bwrap --die-with-parent --unshare-user --unshare-pid --unshare-net --unshare-ipc --unshare-uts --new-session --cap-drop ALL --clearenv \
    --ro-bind "$source_dir" /src --ro-bind "$toolchain_root" /go --ro-bind "$deps_dir" /deps/gomodcache --bind "$scratch_dir" /scratch \
    --proc /proc --dev /dev --chdir /src -- /src/polis-selfdev-go-host-probe boundary; then
    status=0
  else
    status=$?
  fi
  if [[ $status -ne 0 ]]; then
    rm -f -- "$source_dir/probe-input.txt" "$source_dir/host-netns" "$source_dir/escape-link" "$source_dir/polis-selfdev-go-host-probe" "$scratch_dir/probe-write-check"
    rmdir -- "$source_dir" "$deps_dir" "$scratch_dir" "$probe_root" 2>/dev/null || true
    printf 'trusted_bwrap_boundary_probe=failed:%s\n' "$status"
    return 1
  fi
  if timeout -k 1s 2s bwrap --die-with-parent --unshare-net --unshare-pid --ro-bind / / --dev /dev --proc /proc \
    -- /bin/bash -c 'exec -a "$1" /bin/sleep 30' _ "$token"; then
    status=0
  else
    status=$?
  fi
  if [[ $status -ne 124 && $status -ne 137 ]]; then
    rm -f -- "$source_dir/probe-input.txt" "$source_dir/host-netns" "$source_dir/escape-link" "$source_dir/polis-selfdev-go-host-probe" "$scratch_dir/probe-write-check"
    rmdir -- "$source_dir" "$deps_dir" "$scratch_dir" "$probe_root" 2>/dev/null || true
    printf 'trusted_pid_cleanup_probe=failed:%s\n' "$status"
    return 1
  fi
  if ps -eo args= | grep -F "$token" | grep -v grep >/dev/null; then
    rm -f -- "$source_dir/probe-input.txt" "$source_dir/host-netns" "$source_dir/escape-link" "$source_dir/polis-selfdev-go-host-probe" "$scratch_dir/probe-write-check"
    rmdir -- "$source_dir" "$deps_dir" "$scratch_dir" "$probe_root" 2>/dev/null || true
    printf 'trusted_pid_cleanup_probe=orphan_process\n'
    return 1
  fi
  rm -f -- "$source_dir/probe-input.txt" "$source_dir/host-netns" "$source_dir/escape-link" "$source_dir/polis-selfdev-go-host-probe" "$scratch_dir/probe-write-check"
  rmdir -- "$source_dir" "$deps_dir" "$scratch_dir" "$probe_root" || return 1
  printf 'trusted_bwrap_boundary_probe=pass\n'
  printf 'trusted_bwrap_pid_cleanup_probe=pass\n'
  return 0
}

probe_bundle_sha256() {
  local repo_root
  repo_root="$(cd "$(dirname "$0")/.." && pwd -P)"
  (
    cd "$repo_root"
    sha256sum \
      scripts/self-dev-go-job-host.sh \
      scripts/self-dev-go-job-host-root.sh \
      cmd/polis-selfdev-go-host-probe/main.go \
      bin/polis-selfdev-go-host-probe | LC_ALL=C sort | sha256sum | awk '{print $1}'
  )
}

verify_wallclock_limit() {
  local probe_id probe_unit run_dir state result runtime_limit cgroup_path cgroup_dir probe_pid started elapsed populated
  probe_id="probe-wall-$(date +%s)-$$"
  probe_unit="polis-self-dev-go-probe@${probe_id}.service"
  run_dir="/run/polis-self-dev-go-job-probes/${probe_id}"
  started="$(date +%s)"
  if ! systemctl start --no-block "$probe_unit" >/dev/null 2>&1; then
    printf 'wallclock_boundary=blocked;reason=systemd_start_failed\n'
    return 1
  fi
  for _ in $(seq 1 300); do
    [[ -f "$run_dir/probe.result" && -f "$run_dir/process.pid" ]] && break
    state="$(systemctl show --property=ActiveState --value "$probe_unit" 2>/dev/null || true)"
    [[ "$state" == inactive || "$state" == failed ]] && break
    sleep 0.1
  done
  if [[ ! -f "$run_dir/probe.result" || ! -f "$run_dir/process.pid" ]]; then
    timeout -k 1s 8s systemctl stop "$probe_unit" >/dev/null 2>&1 || true
    printf 'wallclock_boundary=blocked;reason=probe_not_started\n'
    return 1
  fi
  probe_pid="$(cat "$run_dir/process.pid")"
  runtime_limit="$(systemctl show --property=RuntimeMaxUSec --value "$probe_unit" 2>/dev/null || true)"
  cgroup_path="$(systemctl show --property=ControlGroup --value "$probe_unit" 2>/dev/null || true)"
  if [[ ! "$runtime_limit" =~ ^(120000000|2min|120s)$ || "$cgroup_path" != "/system.slice/${probe_unit}" || ! "$probe_pid" =~ ^[0-9]+$ ]]; then
    timeout -k 1s 8s systemctl stop "$probe_unit" >/dev/null 2>&1 || true
    printf 'wallclock_boundary=blocked;reason=unit_contract_mismatch\n'
    return 1
  fi
  state=active
  for _ in $(seq 1 135); do
    state="$(systemctl show --property=ActiveState --value "$probe_unit" 2>/dev/null || true)"
    [[ "$state" == failed || "$state" == inactive ]] && break
    sleep 1
  done
  result="$(systemctl show --property=Result --value "$probe_unit" 2>/dev/null || true)"
  elapsed=$(($(date +%s) - started))
  if [[ "$state" != failed || "$result" != timeout || "$elapsed" -lt 110 || "$elapsed" -gt 140 ]]; then
    timeout -k 1s 8s systemctl stop "$probe_unit" >/dev/null 2>&1 || true
    printf 'wallclock_boundary=blocked;reason=runtime_limit_not_enforced;result=%s;elapsed_seconds=%s\n' "$result" "$elapsed"
    return 1
  fi
  for _ in $(seq 1 50); do
    [[ ! -d "/proc/${probe_pid}" ]] && break
    sleep 0.1
  done
  if [[ -d "/proc/${probe_pid}" ]]; then
    printf 'wallclock_boundary=blocked;reason=timed_out_process_survived\n'
    return 1
  fi
  cgroup_dir="/sys/fs/cgroup${cgroup_path}"
  if [[ -d "$cgroup_dir" ]]; then
    populated="$(awk '$1 == "populated" {print $2}' "$cgroup_dir/cgroup.events" 2>/dev/null || true)"
    if [[ "$populated" != 0 ]]; then
      printf 'wallclock_boundary=blocked;reason=timed_out_cgroup_populated\n'
      return 1
    fi
  fi
  printf 'wallclock_boundary=pass;result=timeout;elapsed_seconds=%s;process_tree=empty\n' "$elapsed"
}

verify() {
  local inspect_output probe_unit probe_id state run_dir probe_pid probe_result runtime_limit cgroup_path cgroup_dir populated bundle_sha
  inspect_output="$(inspect)"
  printf '%s\n' "$inspect_output"
  local repo_root
  repo_root="$(cd "$(dirname "$0")/.." && pwd -P)"
  if [[ ! -x "$repo_root/bin/polis-selfdev-go-host-probe" || "$($repo_root/bin/polis-selfdev-go-host-probe --version 2>/dev/null || true)" != 'polis-selfdev-go-host-probe@1' ]]; then
    printf 'trusted_probe_failure=host_probe_binary_missing\nqualification=blocked\ncandidate_execution=not_run\n'
    return 2
  fi
  if ! run_local_bwrap_probe; then
    printf 'trusted_probe_failure=present\nqualification=blocked\ncandidate_execution=not_run\n'
    return 2
  fi
  if ! systemctl list-unit-files --no-legend 'polis-self-dev-go-probe@.service' 2>/dev/null | awk '{print $1}' | grep -Fxq 'polis-self-dev-go-probe@.service'; then
    printf 'resource_probe=systemd_probe_unit_missing\nqualification=blocked\ncandidate_execution=not_run\n'
    return 2
  fi
  probe_id="probe-$(date +%s)-$$"
  probe_unit="polis-self-dev-go-probe@${probe_id}.service"
  run_dir="/run/polis-self-dev-go-job-probes/${probe_id}"
  if ! systemctl start --no-block "$probe_unit" >/dev/null 2>&1; then
    printf 'resource_probe=systemd_probe_unit_start_failed\nqualification=blocked\ncandidate_execution=not_run\n'
    return 2
  fi
  for _ in $(seq 1 300); do
    [[ -f "$run_dir/probe.result" && -f "$run_dir/process.pid" ]] && break
    state="$(systemctl show --property=ActiveState --value "$probe_unit" 2>/dev/null || true)"
    [[ "$state" == inactive || "$state" == failed ]] && break
    sleep 0.1
  done
  if [[ ! -f "$run_dir/probe.result" || ! -f "$run_dir/process.pid" ]]; then
    timeout -k 1s 8s systemctl stop "$probe_unit" >/dev/null 2>&1 || true
    printf 'resource_probe=probe_did_not_finish\nqualification=blocked\ncandidate_execution=not_run\n'
    return 2
  fi
  probe_result="$(cat "$run_dir/probe.result")"
  probe_pid="$(cat "$run_dir/process.pid")"
  runtime_limit="$(systemctl show --property=RuntimeMaxUSec --value "$probe_unit" 2>/dev/null || true)"
  cgroup_path="$(systemctl show --property=ControlGroup --value "$probe_unit" 2>/dev/null || true)"
  printf '%s\n' "$probe_result"
  if [[ ! "$runtime_limit" =~ ^(120000000|2min|120s)$ || ! "$probe_pid" =~ ^[0-9]+$ || "$cgroup_path" != "/system.slice/${probe_unit}" ]]; then
    timeout -k 1s 8s systemctl stop "$probe_unit" >/dev/null 2>&1 || true
    printf 'resource_probe=unit_contract_mismatch\nqualification=blocked\ncandidate_execution=not_run\n'
    return 2
  fi
  if ! timeout -k 1s 8s systemctl stop "$probe_unit" >/dev/null 2>&1; then
    printf 'process_tree_cleanup=systemd_stop_unconfirmed\nqualification=blocked\ncandidate_execution=not_run\n'
    return 2
  fi
  for _ in $(seq 1 30); do
    [[ ! -d "/proc/${probe_pid}" ]] && break
    sleep 0.1
  done
  if [[ -d "/proc/${probe_pid}" ]]; then
    printf 'process_tree_cleanup=orphan_process\nqualification=blocked\ncandidate_execution=not_run\n'
    return 2
  fi
  cgroup_dir="/sys/fs/cgroup${cgroup_path}"
  if [[ -d "$cgroup_dir" ]]; then
    populated="$(awk '$1 == "populated" {print $2}' "$cgroup_dir/cgroup.events" 2>/dev/null || true)"
    if [[ "$populated" != 0 ]]; then
      printf 'process_tree_cleanup=cgroup_still_populated\nqualification=blocked\ncandidate_execution=not_run\n'
      return 2
    fi
  fi
  if ! grep -Fxq 'profile_revision=polis-go-test-fixture@1' <<<"$probe_result" ||
    ! grep -Fxq 'scratch_byte_boundary=pass' <<<"$probe_result" ||
    ! grep -q '^scratch_inode_boundary=pass;created=[0-9][0-9]*$' <<<"$probe_result" ||
    ! grep -Eq '^cpu_throttling=(pass|not_applicable_cpu_capacity_at_or_below_quota)$' <<<"$probe_result" ||
    ! grep -q '^pids_boundary=pass;max_events=[1-9][0-9]*;spawned=[0-9][0-9]*;limit_hit=1$' <<<"$probe_result" ||
    ! grep -q '^memory_boundary=pass;max_events=[1-9][0-9]*;oom_events=[0-9][0-9]*;oom_kill_events=[0-9][0-9]*;swap_current=0$' <<<"$probe_result" ||
    ! grep -Fxq 'candidate_execution=not_run' <<<"$probe_result" ||
    ! grep -Fxq 'probe_result=pass' <<<"$probe_result"; then
    printf 'qualification=blocked\ncandidate_execution=not_run\n'
    return 2
  fi
  if ! verify_wallclock_limit; then
    printf 'qualification=blocked\ncandidate_execution=not_run\n'
    return 2
  fi
  bundle_sha="$(probe_bundle_sha256)"
  [[ "$bundle_sha" =~ ^[a-f0-9]{64}$ ]] || { printf 'qualification=blocked\ncandidate_execution=not_run\n'; return 2; }
  printf 'path_boundary=pass\nnetwork_boundary=pass\nprocess_tree_cleanup=pass\nprobe_bundle_sha256=%s\nqualification=qualified\ncandidate_execution=not_run\n' "$bundle_sha"
}

cleanup_job() {
  local job_id unit state cgroup_path cgroup_dir populated
  job_id="$1"
  [[ "$job_id" =~ ^[a-zA-Z0-9_-]{1,80}$ ]] || { printf 'invalid job id\n' >&2; return 1; }
  unit="polis-self-dev-go-job@job-${job_id}.service"
  cgroup_path="$(systemctl show --property=ControlGroup --value "$unit" 2>/dev/null || true)"
  [[ "$cgroup_path" == "/system.slice/${unit}" ]] || { printf 'job unit is unknown or outside the fixed system slice\n' >&2; return 1; }
  cgroup_dir="/sys/fs/cgroup${cgroup_path}"
  timeout -k 1s 8s /usr/bin/pkexec --disable-internal-agent /usr/local/libexec/polis-selfdev-go-unit-control job-stop "job-${job_id}"
  state="$(systemctl show --property=ActiveState --value "$unit" 2>/dev/null || true)"
  [[ "$state" == inactive || -z "$state" ]] || { printf 'job unit did not stop: %s\n' "$state" >&2; return 1; }
  if [[ -d "$cgroup_dir" ]]; then
    populated="$(awk '$1 == "populated" {print $2}' "$cgroup_dir/cgroup.events" 2>/dev/null || true)"
    [[ "$populated" == 0 ]] || { printf 'job process cgroup remains populated\n' >&2; return 1; }
  fi
  printf 'cleanup=stopped_exact_unit\njob_id=%s\nprocess_tree=systemd_control_group_stopped\n' "$job_id"
}

probe_unit() {
  local probe_id current cg_dir run_dir cpu_max memory_max swap_max pids_max tmpfs_type tmpfs_options inode_total
  local cpu_before cpu_after inode_count index cpu_capacity memory_result pids_result
  local repo_root
  probe_id="${1:-}"
  [[ "$probe_id" =~ ^probe-[a-zA-Z0-9_-]{1,80}$ ]] || { printf 'invalid probe id\n' >&2; return 1; }
  [[ -n "${INVOCATION_ID:-}" ]] || { printf 'probe-unit must run from a systemd unit\n' >&2; return 1; }
  current="$(awk -F: '$1 == "0" {print $3}' /proc/self/cgroup)"
  cg_dir="/sys/fs/cgroup${current}"
  run_dir="/run/polis-self-dev-go-job-probes/${probe_id}"
  repo_root="$(cd "$(dirname "$0")/.." && pwd -P)"
  [[ -d "$run_dir" && ! -L "$run_dir" ]] || { printf 'probe runtime directory missing\n' >&2; return 1; }
  cpu_max="$(cat "$cg_dir/cpu.max" 2>/dev/null || echo unavailable)"
  memory_max="$(cat "$cg_dir/memory.max" 2>/dev/null || echo unavailable)"
  swap_max="$(cat "$cg_dir/memory.swap.max" 2>/dev/null || echo unavailable)"
  pids_max="$(cat "$cg_dir/pids.max" 2>/dev/null || echo unavailable)"
  tmpfs_type="$(findmnt -n -T /scratch -o FSTYPE 2>/dev/null || echo unavailable)"
  tmpfs_options="$(findmnt -n -T /scratch -o OPTIONS 2>/dev/null | one_line || echo unavailable)"
  inode_total="$(stat -f -c %c /scratch 2>/dev/null || echo unavailable)"
  printf 'probe_controls_observed=cpu.max:%s,memory.max:%s,memory.swap.max:%s,pids.max:%s,scratch:%s,inodes:%s\n' "$cpu_max" "$memory_max" "$swap_max" "$pids_max" "$tmpfs_options" "$inode_total"
  printf 'probe_scratch_mount=fstype:%s;mount=%s\n' "$tmpfs_type" "$(findmnt -n -T /scratch -o TARGET,SOURCE,FSTYPE,OPTIONS 2>/dev/null | one_line || echo unavailable)"
  grep ' /scratch ' /proc/self/mountinfo || true
  [[ "$cpu_max" == '200000 100000' && "$memory_max" == '2147483648' && "$swap_max" == '0' && "$pids_max" == '256' ]] || { printf 'probe_limit_contract=failed\n'; return 1; }
  [[ -e "$cg_dir/cgroup.kill" && "$tmpfs_type" == 'tmpfs' && "$inode_total" == '4096' && "$tmpfs_options" == *'nr_inodes=4096'* ]] || { printf 'probe_scratch_contract=failed\n'; return 1; }
  if [[ "$probe_id" == probe-wall-* ]]; then
    printf 'wallclock_probe=started\n' > "$run_dir/probe.result"
    printf '%s\n' "$$" > "$run_dir/process.pid"
    exec /bin/sleep 180
  fi
  printf 'profile_revision=%s\n' 'polis-go-test-fixture@1' > "$run_dir/probe.result"
  printf 'cpu_max=%s\n' "$cpu_max" >> "$run_dir/probe.result"
  printf 'memory_max=%s\n' "$memory_max" >> "$run_dir/probe.result"
  printf 'memory_swap_max=%s\n' "$swap_max" >> "$run_dir/probe.result"
  printf 'pids_max=%s\n' "$pids_max" >> "$run_dir/probe.result"
  printf 'cgroup_kill=present_systemd_managed\n' >> "$run_dir/probe.result"
  printf 'scratch_fstype=%s\n' "$tmpfs_type" >> "$run_dir/probe.result"
  printf 'scratch_options=%s\n' "$(printf '%s' "$tmpfs_options" | one_line)" >> "$run_dir/probe.result"
  printf 'scratch_total_inodes=%s\n' "$inode_total" >> "$run_dir/probe.result"

  memory_result="$("$repo_root/bin/polis-selfdev-go-host-probe" memory)" || { printf 'memory_boundary=failed\n'; return 1; }
  printf '%s\n' "$memory_result" >> "$run_dir/probe.result"
  printf '%s\n' "$memory_result"

  # Allocate one GiB inside the bounded per-job tmpfs; the next byte must fail.
  dd if=/dev/zero of=/scratch/byte-boundary bs=1048576 count=1024 status=none || { printf 'scratch_byte_boundary=failed_write\n'; return 1; }
  if dd if=/dev/zero of=/scratch/byte-boundary bs=1 count=1 oflag=append conv=notrunc status=none 2>/dev/null; then
    printf 'scratch_byte_boundary=failed_overflow_accepted\n'
    return 1
  fi
  rm -f -- /scratch/byte-boundary
  printf 'scratch_byte_boundary=pass\n' >> "$run_dir/probe.result"
  printf 'scratch_byte_boundary=pass\n'

  # The configured tmpfs inode cap counts files and directories, not just regular files.
  mkdir /scratch/inode-boundary || { printf 'scratch_inode_boundary=failed_mkdir\n'; return 1; }
  inode_count=0
  for ((index=1; index<=4096; index++)); do
    if : > "/scratch/inode-boundary/file-${index}" 2>/dev/null; then
      ((inode_count+=1))
    else
      break
    fi
  done
  [[ "$inode_count" -gt 4000 && "$inode_count" -lt 4096 ]] || { printf 'scratch_inode_boundary=failed_count;created=%s\n' "$inode_count"; return 1; }
  if : > /scratch/inode-boundary/over-limit 2>/dev/null; then printf 'scratch_inode_boundary=failed_overflow_accepted\n'; return 1; fi
  printf 'scratch_inode_boundary=pass;created=%s\n' "$inode_count" >> "$run_dir/probe.result"
  printf 'scratch_inode_boundary=pass;created=%s\n' "$inode_count"

  # CPU quota must produce a cgroup throttling event under bounded trusted load.
  cpu_capacity="$(nproc 2>/dev/null || echo 1)"
  if [[ "$cpu_capacity" =~ ^[0-9]+$ && "$cpu_capacity" -gt 2 ]]; then
    cpu_before="$(awk '$1 == "nr_throttled" {print $2}' "$cg_dir/cpu.stat")"
    for index in 1 2 3 4; do (while :; do :; done) &
    done
    sleep 1
    for loop_pid in $(jobs -p); do kill "$loop_pid" 2>/dev/null || true; done
    wait 2>/dev/null || true
    cpu_after="$(awk '$1 == "nr_throttled" {print $2}' "$cg_dir/cpu.stat")"
    [[ "$cpu_before" =~ ^[0-9]+$ && "$cpu_after" =~ ^[0-9]+$ && "$cpu_after" -gt "$cpu_before" ]] || { printf 'cpu_throttling=failed;before=%s;after=%s\n' "$cpu_before" "$cpu_after"; return 1; }
    printf 'cpu_throttling=pass\n' >> "$run_dir/probe.result"
    printf 'cpu_throttling=pass\n'
  else
    printf 'cpu_throttling=not_applicable_cpu_capacity_at_or_below_quota\n' >> "$run_dir/probe.result"
    printf 'cpu_throttling=not_applicable_cpu_capacity_at_or_below_quota\n'
  fi

  # A trusted child process hits the cgroup task limit, then reaps its own test children.
  pids_result="$("$repo_root/bin/polis-selfdev-go-host-probe" pids)" || { printf 'pids_boundary=failed\n'; return 1; }
  printf '%s\n' "$pids_result" >> "$run_dir/probe.result"
  printf '%s\n' "$pids_result"

  # Leave one trusted descendant alive; the outer verifier must stop the unit and prove it is gone.
  /bin/sleep 120 &
  printf '%s\n' "$!" > "$run_dir/process.pid"
  printf 'candidate_execution=not_run\nprobe_result=pass\n' >> "$run_dir/probe.result"
  wait "$!"
}

case "${1:-}" in
  inspect) inspect ;;
  prepare) prepare ;;
  verify) verify ;;
  cleanup)
    [[ $# -eq 2 ]] || { usage >&2; exit 2; }
    cleanup_job "$2"
    ;;
  probe-unit)
    [[ $# -eq 2 ]] || { usage >&2; exit 2; }
    probe_unit "$2"
    ;;
  --help|-h|help) usage ;;
  *) usage >&2; exit 2 ;;
esac

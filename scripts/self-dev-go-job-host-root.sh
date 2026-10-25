#!/usr/bin/env bash
set -euo pipefail

readonly marker='# POLIS_SELF_DEV_GO_JOB_HOST_V1'
readonly systemd_root=/etc/systemd/system
readonly polkit_root=/etc/polkit-1/rules.d
readonly polkit_action_root=/usr/share/polkit-1/actions
readonly polkit_pkla_root=/etc/polkit-1/localauthority/50-local.d
readonly qualified_toolchain_root=/run/polis-self-dev-go-toolchain

usage() {
  printf 'usage: %s plan --target-uid UID | apply --target-uid UID --confirm | cleanup --target-uid UID --confirm\n' "$0"
}

die() {
  printf 'ERROR: %s\n' "$1" >&2
  exit 1
}

action="${1:-}"
shift || true
target_uid=''
confirmed=0
while (($#)); do
  case "$1" in
    --target-uid)
      (($# >= 2)) || die '--target-uid requires a value'
      target_uid="$2"
      shift 2
      ;;
    --confirm)
      confirmed=1
      shift
      ;;
    --help|-h)
      usage
      exit 0
      ;;
    *)
      die "unknown argument: $1"
      ;;
  esac
done

[[ "$target_uid" =~ ^[1-9][0-9]{0,8}$ ]] || die 'target uid must be a positive decimal uid'
target_user="$(getent passwd "$target_uid" | cut -d: -f1)"
target_gid="$(getent passwd "$target_uid" | cut -d: -f4)"
[[ -n "$target_user" && "$target_gid" =~ ^[0-9]+$ ]] || die 'target uid is not a local WSL user'
[[ "$target_user" =~ ^[a-zA-Z_][a-zA-Z0-9_-]{0,31}$ ]] || die 'target user name is not safe for a systemd unit'
repo_root="$(cd "$(dirname "$0")/.." && pwd -P)"

job_unit="$systemd_root/polis-self-dev-go-job@.service"
probe_unit="$systemd_root/polis-self-dev-go-probe@.service"
legacy_polkit_rule="$polkit_root/49-polis-self-dev-go-job.rules"
unit_control_policy="$polkit_action_root/org.polis.selfdev.go-job.policy"
unit_control_pkla="$polkit_pkla_root/50-polis-selfdev-go-unit-control.pkla"
supervisor_binary="$repo_root/bin/polis-selfdev-go-job-supervisor"
supervisor_identity="$supervisor_binary.identity.json"
installed_binary=/usr/local/libexec/polis-selfdev-go-job-supervisor
installed_identity="$installed_binary.identity.json"
unit_control_binary="$repo_root/bin/polis-selfdev-go-unit-control"
installed_unit_control=/usr/local/libexec/polis-selfdev-go-unit-control
runtime_base=/run/polis-self-dev-go-job
scratch_mountpoint=/scratch
scratch_marker="$systemd_root/polis-self-dev-go-job-scratch.marker"
binary_sha_file="$systemd_root/polis-self-dev-go-job-supervisor.sha256"
unit_control_sha_file="$systemd_root/polis-self-dev-go-unit-control.sha256"
probe_script="$repo_root/scripts/self-dev-go-job-host.sh"

print_plan() {
  cat <<PLAN
action=plan_only
changes_applied=no
target_distribution=${WSL_DISTRO_NAME:-unknown}
target_user=${target_user}
target_uid=${target_uid}
scope=one WSL distribution; systemd creates one resource-controlled cgroup per exact JobRun unit
root_required=yes
files_to_add:
  /usr/local/libexec (0755 only if absent; retain the shared parent on cleanup)
  remove the prior ineffective rule: ${legacy_polkit_rule}
  ${job_unit}
  ${probe_unit}
  ${unit_control_policy}
  ${unit_control_pkla}
  ${unit_control_sha_file}
  ${binary_sha_file}
  ${installed_binary} (verified dedicated fixed-Go supervisor built from this checkout)
  ${installed_identity}
  ${installed_unit_control} (fixed pkexec custom-action broker; no shell)
  ${qualified_toolchain_root} (existing root-owned, read-only tree bound by the host qualification digest)
  ${runtime_base} (0700; owned by ${target_user}; bounded request/result handoff only)
  ${scratch_mountpoint} (empty root:root 0755 mountpoint; per-unit tmpfs overlays it)
  ${scratch_marker} (ownership marker for exact empty-directory rollback)
source_supervisor=${supervisor_binary}
source_identity=${supervisor_identity}
source_unit_control=${unit_control_binary}
job_unit_execstart=/usr/local/libexec/polis-selfdev-go-job-supervisor --job-id=%i --toolchain-root=${qualified_toolchain_root}
resource_contract:
  CPUQuota=200%; CPUQuotaPeriodSec=100ms
  MemoryMax=2147483648; MemorySwapMax=0
  TasksMax=256 (cgroup pids controller counts tasks, including threads)
  RuntimeMaxSec=120s; KillMode=control-group
  probe_only=OOMPolicy=continue keeps the trusted probe supervisor alive to observe the child OOM; job unit retains the default OOM policy
  TemporaryFileSystem=/scratch:rw,size=1073741824,nr_inodes=4096,mode=0700,uid=${target_uid},gid=${target_gid},nodev,nosuid
  stdout/stderr limits remain 512 KiB each in the trusted supervisor
cgroup_management=systemd-managed; no direct cgroupfs writes
polkit_scope=one pkexec action bound to the unit-control helper path for ${target_user}; helper allows read-only --verify-installation plus job-start/job-stop for canonical Job IDs
mount_scope=per-system-service private tmpfs over an empty host mountpoint; the host directory itself is not a mount; no physical device, global mount, /etc/wsl.conf, or .wslconfig change
rollback=stop exact unit instances; run this script cleanup --target-uid ${target_uid} --confirm; remove /scratch only by rmdir when its owner/mode are unchanged and it is empty; no recursive workspace or cgroup cleanup
activation=systemd daemon-reload; actual controller and quota availability must pass the trusted probe
  prerequisite_command=sudo install -d -m 0755 /usr/local/libexec (review absent directory first; action and local-authority directories must already exist)
parent_rollback=shared parent directories are retained; if created solely for this setup, remove only with rmdir after cleanup confirms they are empty
PLAN
}

require_marked_file() {
  local file="$1"
  [[ -f "$file" && ! -L "$file" ]] || die "refusing to remove missing or linked path: $file"
  grep -Fxq "$marker" "$file" || die "refusing to remove file without this tool's marker: $file"
}

if [[ "$action" == "plan" ]]; then
  print_plan
  exit 0
fi

[[ "$confirmed" -eq 1 ]] || die 'apply/cleanup requires the explicit --confirm flag'
[[ "$(id -u)" -eq 0 ]] || die 'run this helper as root only after reviewing its plan'
[[ "$(ps -p 1 -o comm= | tr -d '[:space:]')" == systemd ]] || die 'systemd must be PID 1 in the selected WSL distribution'

case "$action" in
  apply)
    [[ -f "$supervisor_binary" && -x "$supervisor_binary" && ! -L "$supervisor_binary" ]] || die "trusted supervisor build is missing: $supervisor_binary"
    [[ -f "$supervisor_identity" && ! -L "$supervisor_identity" ]] || die "supervisor build identity is missing"
    [[ "$(od -An -tx1 -N5 "$supervisor_binary" | tr -d ' \n')" == 7f454c4602 ]] || die "supervisor must be a Linux 64-bit ELF executable"
    timeout -k 1s 30s "$supervisor_binary" --verify-build "$supervisor_identity" --source-root "$repo_root" || die "supervisor platform/protocol/source identity mismatch; rebuild required"
    [[ -f "$probe_script" && ! -L "$probe_script" ]] || die "trusted probe source is missing or linked: $probe_script"
    [[ "$target_uid" == 1000 && "$target_user" == chyinan ]] || die 'unit-control policy is pinned to chyinan uid 1000'
    [[ -x /usr/bin/pkexec && -f /usr/bin/pkexec && ! -L /usr/bin/pkexec ]] || die 'pkexec is not installed at the approved path'
    [[ -d "$qualified_toolchain_root" && ! -L "$qualified_toolchain_root" && -x "$qualified_toolchain_root/bin/go" && ! -L "$qualified_toolchain_root/bin/go" ]] || die 'root-owned staged Go toolchain is missing'
    [[ "$(stat -c '%u:%g:%a' "$qualified_toolchain_root")" == '0:0:555' ]] || die 'staged Go toolchain root must be root-owned and read-only'
    [[ -f "$unit_control_binary" && -x "$unit_control_binary" && ! -L "$unit_control_binary" ]] || die "fixed unit-control build is missing: $unit_control_binary"
    [[ "$(od -An -tx1 -N5 "$unit_control_binary" | tr -d ' \n')" == 7f454c4602 ]] || die 'unit-control helper must be a Linux 64-bit ELF executable'
    [[ "$("$unit_control_binary" --protocol-info 2>/dev/null || true)" == polis-selfdev-go-unit-control@1 ]] || die 'unit-control protocol identity mismatch'
    paths=("$job_unit" "$probe_unit" "$legacy_polkit_rule" "$unit_control_policy" "$unit_control_pkla" "$unit_control_sha_file" "$binary_sha_file" "$installed_binary" "$installed_identity" "$installed_unit_control" "$runtime_base" "$scratch_marker" "$scratch_mountpoint")
    for path in "${paths[@]}"; do
      [[ ! -e "$path" && ! -L "$path" ]] || die "refusing to overwrite existing path: $path"
    done
    for path in "$systemd_root" "$polkit_action_root" "$polkit_pkla_root" /usr/local/libexec; do
      [[ -d "$path" && ! -L "$path" ]] || die "required system directory is missing, linked, or not a directory: $path"
    done
    created=()
    rollback_partial() {
      local path index status
      status=$?
      if [[ "$action" != apply || "${apply_complete:-0}" == 1 ]]; then
        return "$status"
      fi
      for ((index=${#created[@]}-1; index>=0; index--)); do
        path="${created[$index]}"
        if [[ "$path" == "$installed_binary" || "$path" == "$installed_identity" ]]; then
          if [[ -f "$binary_sha_file" && -f "$path" && ! -L "$path" ]]; then
            line=2; [[ "$path" == "$installed_identity" ]] && line=3
            expected="$(sed -n "${line}p" "$binary_sha_file")"
            actual="$(sha256sum "$path" | cut -d ' ' -f 1)"
            [[ "$expected" == "$actual" ]] && rm -f -- "$path"
          fi
        elif [[ "$path" == "$installed_unit_control" ]]; then
          if [[ -f "$unit_control_sha_file" && -f "$path" && ! -L "$path" ]]; then
            expected="$(sed -n '2p' "$unit_control_sha_file")"
            actual="$(sha256sum "$path" | cut -d ' ' -f 1)"
            [[ "$expected" == "$actual" ]] && rm -f -- "$path"
          fi
        elif [[ "$path" == "$runtime_base" ]]; then
          [[ -d "$path" && ! -L "$path" ]] && rmdir -- "$path" 2>/dev/null || true
        elif [[ "$path" == "$scratch_mountpoint" ]]; then
          [[ -d "$path" && ! -L "$path" ]] && rmdir -- "$path" 2>/dev/null || true
        elif [[ -f "$path" && ! -L "$path" ]] && grep -Fxq "$marker" "$path"; then
          rm -f -- "$path"
        fi
      done
      return "$status"
    }
    apply_complete=0
    trap rollback_partial EXIT
    mkdir -m 0755 "$scratch_mountpoint"
    created+=("$scratch_mountpoint")
    printf '%s\n%s\n' "$marker" "$scratch_mountpoint" > "$scratch_marker"
    chmod 0644 "$scratch_marker"
    created+=("$scratch_marker")
    printf '%s\n' "$marker" > "$job_unit"
    cat >> "$job_unit" <<UNIT
[Unit]
Description=Polis fixed Go test JobRun %i
After=local-fs.target

[Service]
Type=exec
User=${target_user}
Group=${target_gid}
Slice=system.slice
ExecStart=/usr/local/libexec/polis-selfdev-go-job-supervisor --job-id=%i --toolchain-root=${qualified_toolchain_root}
CPUAccounting=yes
MemoryAccounting=yes
TasksAccounting=yes
CPUQuota=200%
CPUQuotaPeriodSec=100ms
MemoryMax=2147483648
MemorySwapMax=0
TasksMax=256
RuntimeMaxSec=120s
TimeoutStopSec=1s
KillMode=control-group
TemporaryFileSystem=/scratch:rw,size=1073741824,nr_inodes=4096,mode=0700,uid=${target_uid},gid=${target_gid},nodev,nosuid
RuntimeDirectory=polis-self-dev-go-job/%i
RuntimeDirectoryMode=0700
RuntimeDirectoryPreserve=yes
ProtectSystem=strict
ProtectHome=read-only
ProtectKernelTunables=yes
ProtectKernelModules=yes
ProtectKernelLogs=yes
ProtectControlGroups=yes
PrivateDevices=yes
NoNewPrivileges=yes
RestrictSUIDSGID=yes
LockPersonality=yes
ReadWritePaths=/run/polis-self-dev-go-job/%i
StandardOutput=journal
StandardError=journal
UNIT
    chmod 0644 "$job_unit"
    created+=("$job_unit")
    printf '%s\n' "$marker" > "$probe_unit"
    cat >> "$probe_unit" <<UNIT
[Unit]
Description=Polis trusted Go job host probe %i

[Service]
Type=exec
User=${target_user}
Group=${target_gid}
Slice=system.slice
ExecStart=/bin/bash ${probe_script} probe-unit %i
CPUAccounting=yes
MemoryAccounting=yes
TasksAccounting=yes
CPUQuota=200%
CPUQuotaPeriodSec=100ms
MemoryMax=2147483648
MemorySwapMax=0
TasksMax=256
RuntimeMaxSec=120s
TimeoutStopSec=1s
KillMode=control-group
OOMPolicy=continue
TemporaryFileSystem=/scratch:rw,size=1073741824,nr_inodes=4096,mode=0700,uid=${target_uid},gid=${target_gid},nodev,nosuid
RuntimeDirectory=polis-self-dev-go-job-probes/%i
RuntimeDirectoryMode=0700
ProtectSystem=strict
ProtectHome=read-only
ProtectKernelTunables=yes
ProtectKernelModules=yes
ProtectKernelLogs=yes
ProtectControlGroups=yes
PrivateDevices=yes
NoNewPrivileges=yes
RestrictSUIDSGID=yes
LockPersonality=yes
ReadWritePaths=/run/polis-self-dev-go-job-probes/%i
StandardOutput=journal
StandardError=journal
UNIT
    chmod 0644 "$probe_unit"
    created+=("$probe_unit")
    printf '%s\n%s\n%s\n' "$marker" "$(sha256sum "$supervisor_binary" | cut -d ' ' -f 1)" "$(sha256sum "$supervisor_identity" | cut -d ' ' -f 1)" > "$binary_sha_file"
    chmod 0644 "$binary_sha_file"
    created+=("$binary_sha_file")
    install -m 0755 "$supervisor_binary" "$installed_binary"
    created+=("$installed_binary")
    install -m 0644 "$supervisor_identity" "$installed_identity"
    created+=("$installed_identity")
    install -d -m 0700 -o "$target_uid" -g "$target_gid" "$runtime_base"
    created+=("$runtime_base")
    cat > "$unit_control_policy" <<POLICY
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE policyconfig PUBLIC "-//freedesktop//DTD polkit Policy Configuration 1.0//EN" "http://www.freedesktop.org/standards/PolicyKit/1/policyconfig.dtd">
<!--
${marker}
-->
<policyconfig>
  <vendor>Polis</vendor>
  <action id="org.polis.selfdev.go-job.control">
    <description>Control a fixed Polis self-development Go JobRun</description>
    <message>Polis requires authorization to control a fixed Go JobRun.</message>
    <defaults><allow_any>no</allow_any><allow_inactive>no</allow_inactive><allow_active>no</allow_active></defaults>
    <annotate key="org.freedesktop.policykit.exec.path">${installed_unit_control}</annotate>
  </action>
</policyconfig>
POLICY
    chmod 0644 "$unit_control_policy"
    created+=("$unit_control_policy")
    cat > "$unit_control_pkla" <<PKLA
${marker}
[Polis self-dev fixed Go Job control]
Identity=unix-user:${target_user}
Action=org.polis.selfdev.go-job.control;
ResultAny=yes
PKLA
    chmod 0644 "$unit_control_pkla"
    created+=("$unit_control_pkla")
    control_action="$(pkaction --action-id org.polis.selfdev.go-job.control 2>/dev/null || true)"
    grep -Fxq 'org.polis.selfdev.go-job.control' <<<"$control_action" || die 'Polkit did not load the fixed unit-control action'
    printf '%s\n%s\n%s\n%s\n' "$marker" "$(sha256sum "$unit_control_binary" | cut -d ' ' -f 1)" "$(sha256sum "$unit_control_policy" | cut -d ' ' -f 1)" "$(sha256sum "$unit_control_pkla" | cut -d ' ' -f 1)" > "$unit_control_sha_file"
    chmod 0644 "$unit_control_sha_file"
    created+=("$unit_control_sha_file")
    install -m 0755 "$unit_control_binary" "$installed_unit_control"
    created+=("$installed_unit_control")
    systemctl daemon-reload
    systemctl restart polkit.service >/dev/null 2>&1 || true
    apply_complete=1
    trap - EXIT
    printf 'prepared=yes\nunit_templates=polis-self-dev-go-job@.service,polis-self-dev-go-probe@.service\nchanges_applied=yes\n'
    ;;
  cleanup)
    units="$(systemctl list-units --state=active --no-legend --plain 'polis-self-dev-go-*.service' 2>/dev/null || true)"
    if [[ -n "$units" ]]; then
      die 'stop all Polis self-dev Go job/probe units by exact instance before removing host configuration'
    fi
    files=("$job_unit" "$probe_unit" "$legacy_polkit_rule" "$unit_control_policy" "$unit_control_pkla")
    for path in "${files[@]}"; do
      [[ ! -e "$path" && ! -L "$path" ]] && continue
      require_marked_file "$path"
    done
    if [[ -e "$scratch_marker" || -L "$scratch_marker" ]]; then
      require_marked_file "$scratch_marker"
      [[ "$(sed -n '2p' "$scratch_marker")" == "$scratch_mountpoint" ]] || die 'refusing scratch marker for a different path'
    fi
    if [[ -e "$installed_binary" || -L "$installed_binary" ]]; then
      [[ -f "$installed_binary" && ! -L "$installed_binary" && -f "$binary_sha_file" && ! -L "$binary_sha_file" ]] || die "refusing unsafe installed supervisor path"
      grep -Fxq "$marker" "$binary_sha_file" || die "refusing to remove supervisor without matching marker"
      expected="$(sed -n '2p' "$binary_sha_file")"
      actual="$(sha256sum "$installed_binary" | cut -d ' ' -f 1)"
      [[ "$expected" == "$actual" ]] || die "refusing to remove supervisor whose bytes changed"
      if [[ -e "$installed_identity" || -L "$installed_identity" ]]; then
        [[ -f "$installed_identity" && ! -L "$installed_identity" ]] || die "refusing unsafe installed identity path"
        [[ "$(sha256sum "$installed_identity" | cut -d ' ' -f 1)" == "$(sed -n '3p' "$binary_sha_file")" ]] || die "refusing changed supervisor identity"
      fi
    elif [[ -e "$binary_sha_file" || -L "$binary_sha_file" ]]; then
      require_marked_file "$binary_sha_file"
    fi
    if [[ -e "$installed_unit_control" || -L "$installed_unit_control" ]]; then
      [[ -f "$installed_unit_control" && ! -L "$installed_unit_control" && -f "$unit_control_sha_file" && ! -L "$unit_control_sha_file" ]] || die 'refusing unsafe installed unit-control helper path'
      grep -Fxq "$marker" "$unit_control_sha_file" || die 'refusing to remove unit-control helper without matching seal'
      [[ "$(sha256sum "$installed_unit_control" | cut -d ' ' -f 1)" == "$(sed -n '2p' "$unit_control_sha_file")" ]] || die 'refusing changed unit-control helper'
      [[ "$(sha256sum "$unit_control_policy" | cut -d ' ' -f 1)" == "$(sed -n '3p' "$unit_control_sha_file")" ]] || die 'refusing changed unit-control action policy'
      [[ "$(sha256sum "$unit_control_pkla" | cut -d ' ' -f 1)" == "$(sed -n '4p' "$unit_control_sha_file")" ]] || die 'refusing changed unit-control local policy'
    elif [[ -e "$unit_control_sha_file" || -L "$unit_control_sha_file" ]]; then
      require_marked_file "$unit_control_sha_file"
    fi
    # Validate the complete exact removal set before changing any target.
    if [[ ( -e "$installed_identity" || -L "$installed_identity" ) && ! -e "$installed_binary" ]]; then
      die 'refusing an orphaned identity manifest without its verified binary'
    fi
    for path in "${files[@]}"; do
      [[ ! -e "$path" ]] || rm -f -- "$path"
    done
    rm -f -- "$installed_binary" "$installed_identity" "$binary_sha_file" "$installed_unit_control" "$unit_control_sha_file"
    systemctl daemon-reload
    systemctl restart polkit.service >/dev/null 2>&1 || true
    if [[ -f "$scratch_marker" && ! -L "$scratch_marker" ]]; then
      if [[ -d "$scratch_mountpoint" && ! -L "$scratch_mountpoint" ]]; then
        scratch_stat="$(stat -c '%u:%a' "$scratch_mountpoint")"
        if [[ "$scratch_stat" == '0:755' ]] && rmdir -- "$scratch_mountpoint" 2>/dev/null; then
          rm -f -- "$scratch_marker"
        else
          printf 'scratch_mountpoint_preserved=%s (not empty or owner/mode changed)\n' "$scratch_mountpoint"
        fi
      else
        printf 'scratch_mountpoint_preserved=%s (missing or not a directory)\n' "$scratch_mountpoint"
      fi
    fi
    if [[ -d "$runtime_base" && ! -L "$runtime_base" ]]; then
      rmdir -- "$runtime_base" 2>/dev/null || printf 'runtime_handoff_preserved=%s (contains job evidence)\n' "$runtime_base"
    fi
    printf 'cleaned=yes\nremoved_only_marked_paths=yes\n'
    ;;
  *)
    usage >&2
    exit 2
    ;;
esac

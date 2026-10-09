#!/usr/bin/env bash
set -euo pipefail

readonly marker='# POLIS_SELF_DEV_GO_JOB_HOST_V1'
readonly systemd_root=/etc/systemd/system
readonly polkit_root=/etc/polkit-1/rules.d

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
if command -v wslpath >/dev/null 2>&1; then
  repo_root="$(wslpath -a "$repo_root")"
fi

job_unit="$systemd_root/polis-self-dev-go-job@.service"
probe_unit="$systemd_root/polis-self-dev-go-probe@.service"
polkit_rule="$polkit_root/49-polis-self-dev-go-job.rules"
supervisor_binary="$repo_root/bin/polis-selfdev-go-job-supervisor"
supervisor_identity="$supervisor_binary.identity.json"
installed_binary=/usr/local/libexec/polis-selfdev-go-job-supervisor
installed_identity="$installed_binary.identity.json"
runtime_base=/run/polis-self-dev-go-job
binary_sha_file="$systemd_root/polis-self-dev-go-job-supervisor.sha256"
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
  /usr/local/libexec and /etc/polkit-1/rules.d (0755 parent directories only if absent)
  ${job_unit}
  ${probe_unit}
  ${polkit_rule}
  ${binary_sha_file}
  ${installed_binary} (verified dedicated fixed-Go supervisor built from this checkout)
  ${installed_identity}
  ${runtime_base} (0700; owned by ${target_user}; bounded request/result handoff only)
resource_contract:
  CPUQuota=200%; CPUQuotaPeriodSec=100ms
  MemoryMax=2147483648; MemorySwapMax=0
  TasksMax=256 (cgroup pids controller counts tasks, including threads)
  RuntimeMaxSec=120s; KillMode=control-group
  TemporaryFileSystem=/scratch:rw,size=1073741824,nr_inodes=4096,mode=0700,uid=${target_uid},gid=${target_gid},nodev,nosuid
  stdout/stderr limits remain 512 KiB each in the trusted supervisor
cgroup_management=systemd-managed; no direct cgroupfs writes
polkit_scope=only start/stop/kill on polis-self-dev-go-job@job-*.service and polis-self-dev-go-probe@probe-*.service for ${target_user}; no sudoers rule
mount_scope=per-system-service private tmpfs; no physical device, global mount, /etc/wsl.conf, or .wslconfig change
rollback=stop exact unit instances; run this script cleanup --target-uid ${target_uid} --confirm; no recursive workspace or cgroup cleanup
activation=systemd daemon-reload; actual controller and quota availability must pass the trusted probe
prerequisite_command=sudo install -d -m 0755 /usr/local/libexec /etc/polkit-1/rules.d (review absent directories first; helper refuses missing/linked parents)
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
    paths=("$job_unit" "$probe_unit" "$polkit_rule" "$binary_sha_file" "$installed_binary" "$installed_identity" "$runtime_base")
    for path in "${paths[@]}"; do
      [[ ! -e "$path" && ! -L "$path" ]] || die "refusing to overwrite existing path: $path"
    done
    for path in "$systemd_root" "$polkit_root" /usr/local/libexec; do
      [[ -d "$path" && ! -L "$path" ]] || die "required system directory is missing, linked, or not a directory: $path"
    done
    created=()
    rollback_partial() {
      local path index
      for ((index=${#created[@]}-1; index>=0; index--)); do
        path="${created[$index]}"
        if [[ "$path" == "$installed_binary" || "$path" == "$installed_identity" ]]; then
          if [[ -f "$binary_sha_file" && -f "$path" && ! -L "$path" ]]; then
            line=2; [[ "$path" == "$installed_identity" ]] && line=3
            expected="$(sed -n "${line}p" "$binary_sha_file")"
            actual="$(sha256sum "$path" | cut -d ' ' -f 1)"
            [[ "$expected" == "$actual" ]] && rm -f -- "$path"
          fi
        elif [[ "$path" == "$runtime_base" ]]; then
          [[ -d "$path" && ! -L "$path" ]] && rmdir -- "$path" 2>/dev/null || true
        elif [[ -f "$path" && ! -L "$path" ]] && grep -Fxq "$marker" "$path"; then
          rm -f -- "$path"
        fi
      done
    }
    trap rollback_partial ERR
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
ExecStart=/usr/local/libexec/polis-selfdev-go-job-supervisor --job-id=%i --toolchain-root=${repo_root}/.tools/go
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
ReadWritePaths=/scratch /run/polis-self-dev-go-job/%i
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
ReadWritePaths=/scratch /run/polis-self-dev-go-job-probes/%i
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
    cat > "$polkit_rule" <<POLKIT
/*
${marker}
*/
polkit.addRule(function(action, subject) {
  var unit = action.lookup("unit");
  var verb = action.lookup("verb");
  var allowedUnit = typeof unit === "string" &&
    /^polis-self-dev-go-(job@job-[A-Za-z0-9_-]{1,80}|probe@probe-[A-Za-z0-9_-]{1,80})\.service$/.test(unit);
  var allowedVerb = verb === "start" || verb === "stop" || verb === "kill";
  if (action.id === "org.freedesktop.systemd1.manage-units" && subject.user === "${target_user}" && allowedUnit && allowedVerb) {
    return polkit.Result.YES;
  }
});
POLKIT
    chmod 0644 "$polkit_rule"
    created+=("$polkit_rule")
    systemctl daemon-reload
    trap - ERR
    printf 'prepared=yes\nunit_templates=polis-self-dev-go-job@.service,polis-self-dev-go-probe@.service\nchanges_applied=yes\n'
    ;;
  cleanup)
    units="$(systemctl list-units --state=active --no-legend --plain 'polis-self-dev-go-*.service' 2>/dev/null || true)"
    if [[ -n "$units" ]]; then
      die 'stop all Polis self-dev Go job/probe units by exact instance before removing host configuration'
    fi
    files=("$job_unit" "$probe_unit" "$polkit_rule")
    for path in "${files[@]}"; do
      [[ ! -e "$path" && ! -L "$path" ]] && continue
      require_marked_file "$path"
    done
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
    # Validate the complete exact removal set before changing any target.
    if [[ ( -e "$installed_identity" || -L "$installed_identity" ) && ! -e "$installed_binary" ]]; then
      die 'refusing an orphaned identity manifest without its verified binary'
    fi
    for path in "${files[@]}"; do
      [[ ! -e "$path" ]] || rm -f -- "$path"
    done
    rm -f -- "$installed_binary" "$installed_identity" "$binary_sha_file"
    systemctl daemon-reload
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

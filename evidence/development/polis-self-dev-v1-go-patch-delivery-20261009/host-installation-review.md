# 宿主资格与真实 Go JobRun 状态（resource probe 与 Workbench E2E 通过）

基线 de9089bc。以下内容由当前 root helper 的 heredoc 展开。用户此前批准创建空 `/scratch` mountpoint，并继续完成窄 broker 与受限 host 计划。root-owned 只读 Go toolchain staged tree 和窄 `pkexec` broker 已 apply。2026-10-26 current supervisor `4e241c70…` 的 CPU/memory/swap/PIDs/scratch/timeout/network/path/process-tree probe 全部通过，并签发同 boot receipt。最新真实 Go JobRun、正式 Patch、Workbench diff、ready DeliveryManifest 与 owner disposition E2E 全部通过；DB 与测试 owner 均为 disposable fixture。

分支 `codex/req40-manifest-routing`；本次验证基线为 `1321c69aa9feef42e8096b439e053b6d9b6a5b80`。用户随后明确要求提交并推送，本 slice 已按该要求完成。
当前 root 安装 supervisor SHA256：`4e241c70eed64ee138769b5bf191eee18ee9fa167295a3f6a3bdfa47171c7fb3`。
当前 supervisor source identity：`6be9cdf873e84f7699938332fd79c8999a857a539ee23b9ce1addcc4989971ab`。
unit-control helper SHA256：`186bb99f606abbd2604480f1d2047936a7414ac97169f59977992af0853d7b89`。
历史 `76fe…`、`b07af7…`、`fadc2013…`、`a7a2a9b8…`、`9fb29904…` 仅作旧安装/构建记录；当前 boot 目标只认完整 `4e241c70…` SHA256。

```ini
privileged_apply_authorized = YES
privileged_changes_applied = YES (exact pkexec broker + /scratch mountpoint)
pkexec_custom_action = PASSED (invalid inputs rejected before systemctl)
host_resource_probe_for_current_runner = PASSED
root_owned_qualification_receipt = ISSUED (boot-bound; current probe bundle)
host_execution_profile_qualification = QUALIFIED
real_restricted_go_job = PASSED (job-cdbb806b0e578419a30b513d6d80a742)
formal_patch_delivery = PASSED (artifact 1edfbc093b55f7345cd14d71cacc4e26)
workbench_diff_and_disposition = PASSED (read-only diff; disposable installation-owner accepted fixture)
postgres = PASS (PostgreSQL 18.6 / Schema 134 / disposable socket-only DB)
provider_egress = 0
real_model_worker_started = 0
automatic_apply_merge_deploy = NO
commit_push = NOT_PERFORMED
```

API/控制面必须设置 `POLIS_GO_TOOLCHAIN_ROOT=/run/polis-self-dev-go-toolchain`。该 boot-scoped staged tree 是 `root:root 0555`，内容 SHA-256 manifest 与本次 root receipt 匹配；systemd Job template 同样固定绑定这个路径。对原 Windows 挂载路径的同一 tree inspect 耗时约 151 秒，超过 120 秒 Job unit 上限，因此不得让 unit 回退到 `.tools/go`。真正候选执行仍要求 Workbench 显式提交 `executeGoTest=true`。

## 身份与权限

supervisor 安装路径：`/usr/local/libexec/polis-selfdev-go-job-supervisor`。
当前 root 安装副本完整 SHA256：`4e241c70eed64ee138769b5bf191eee18ee9fa167295a3f6a3bdfa47171c7fb3`。其 identity manifest 的 source identity 是 `6be9cdf873e84f7699938332fd79c8999a857a539ee23b9ce1addcc4989971ab`。
协议 `polis-go-job-supervisor@1`，profile `polis-go-test-fixture@1`，Linux amd64。

| 路径 | 拥有者和权限 | 谁可修改 |
|---|---|---|
| /etc/systemd/system/polis-self-dev-go-job@.service | root，0644 | 管理员；ExecStart 固定使用 staged toolchain |
| /etc/systemd/system/polis-self-dev-go-probe@.service | root，0644 | 管理员 |
| /etc/polkit-1/rules.d/49-polis-self-dev-go-job.rules | 不存在 | 旧安装的无效规则已按 marker 删除 |
| /usr/local/libexec/polis-selfdev-go-job-supervisor | root，0755 | 当前 SHA `4e241c70…`; chyinan 可读、执行 |
| 同路径 .identity.json | root，0644 | 管理员 |
| /usr/local/libexec/polis-selfdev-go-unit-control | root，0755 | 固定 helper；只读 seal verifier + 两个 Job unit verbs |
| /usr/share/polkit-1/actions/org.polis.selfdev.go-job.policy | root，0644 | 单一 custom action 绑定固定 helper path |
| /etc/polkit-1/localauthority/50-local.d/50-polis-selfdev-go-unit-control.pkla | root，0644 | 只授权 `chyinan` 的 helper action |
| /etc/systemd/system/polis-self-dev-go-unit-control.sha256 | root，0644 | 绑定 helper、policy 和 pkla 摘要 |
| /etc/systemd/system/polis-self-dev-go-job-supervisor.sha256 | root，0644 | 管理员 |
| /run/polis-self-dev-go-job | chyinan:1000，0700 | 可信控制面用户；候选挂载不可见 |
| /run/polis-self-dev-go-toolchain | root:root，0555 | 本次 boot 的固定 Go toolchain 内容树；Job unit 只读使用，receipt 绑定其 tree digest |
| /scratch | root:root，0755 空目录 | 安装器创建；仅作 systemd 每 unit 私有 tmpfs 的目标，cleanup 仅在空目录且 owner/mode 未变时 rmdir |
| /etc/systemd/system/polis-self-dev-go-job-scratch.marker | root:root，0644 | 安装器 marker；仅用于 `/scratch` 的精确回滚 |
| workspace 中的安装脚本、探针、host-probe binary 和 .tools/go | 开发用户可修改 | chyinan；不属于已安装 root-owned 产物 |

当前可信合成探针：`bin/polis-selfdev-go-host-probe`，SHA256 `e0dce81c047aaf1e73630bae957d6f4c16dbe453549469205ecfff92eec3d0c7`。构建入口：`bash scripts/build-self-dev-go-host-probe.sh`。它不是候选程序，也不修改 supervisor 的已批准 SHA256。
当前 unit-control helper：`bin/polis-selfdev-go-unit-control`，SHA256 `be7c920fdae8226ea3831a1db898699e3097a11d2925153dda1ba20280150157`。它是受限授权 shim，不是候选程序。

旧 `.rules` 已删除。新方案使用路径绑定的单一 `pkexec` custom action。root-owned helper 提供只读 `--verify-installation` 来检查 root-only `.pkla` seal，以及 `job-start` / `job-stop` 两个固定动作；动作路径和 ID 均严格校验，不调用 shell，也不授予通用 `manage-units` 权限。之前的非法 ID/未知 verb 负例均在调用 systemctl 前被拒绝。Company/Task/JobRun 权限继续由现有 Go API 检查。

探针引用 workspace 中可修改的脚本和 host-probe binary。管理员签发 qualification receipt 前必须审阅实际源码、bundle digest 和运行证据，不能仅凭 stdout 的 PASS。`ProbeSHA256` 绑定以下材料的 SHA256 清单摘要：`self-dev-go-job-host.sh`、`self-dev-go-job-host-root.sh`、`cmd/polis-selfdev-go-host-probe/main.go`、生成的 `bin/polis-selfdev-go-host-probe`。root-owned receipt 还绑定本次 boot、installed runner、toolchain tree、bwrap 和完整限制值。

## Job unit

目标：`/etc/systemd/system/polis-self-dev-go-job@.service`。

```ini
# POLIS_SELF_DEV_GO_JOB_HOST_V1
[Unit]
Description=Polis fixed Go test JobRun %i
After=local-fs.target

[Service]
Type=exec
User=chyinan
Group=1000
Slice=system.slice
ExecStart=/usr/local/libexec/polis-selfdev-go-job-supervisor --job-id=%i --toolchain-root=/run/polis-self-dev-go-toolchain
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
TemporaryFileSystem=/scratch:rw,size=1073741824,nr_inodes=4096,mode=0700,uid=1000,gid=1000,nodev,nosuid
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
```

## Probe unit

目标：`/etc/systemd/system/polis-self-dev-go-probe@.service`。

```ini
# POLIS_SELF_DEV_GO_JOB_HOST_V1
[Unit]
Description=Polis trusted Go job host probe %i

[Service]
Type=exec
User=chyinan
Group=1000
Slice=system.slice
ExecStart=/bin/bash /mnt/d/Programs/Polis-cloud-main/scripts/self-dev-go-job-host.sh probe-unit %i
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
TemporaryFileSystem=/scratch:rw,size=1073741824,nr_inodes=4096,mode=0700,uid=1000,gid=1000,nodev,nosuid
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
```

## Polkit broker

旧的 `49-polis-self-dev-go-job.rules` 已按 marker 清理。当前使用单一、路径绑定的 `pkexec` custom action 和 root-owned `/usr/local/libexec/polis-selfdev-go-unit-control`。helper 仅接受 `job-start` / `job-stop` 与规范 JobRun ID，并自行构造固定 unit 名；不开放 `org.freedesktop.systemd1.manage-units`。

## 专用 cgroup、scratch 和变更范围

仅 Ubuntu-22.04 中这两个模板的实例。systemd 管理 `/sys/fs/cgroup/system.slice/polis-self-dev-go-job@job-<id>.service`，候选无写入控制器或迁出权限。没有全局 cgroup chmod、sudoers、WSL 配置、网络放行或物理块设备操作。

每个实例单独挂载 1 GiB tmpfs，4096 限制全部 inode（含目录、文件和链接）。保留 nodev/nosuid；Go 测试二进制需要执行权限，因此 scratch 不设 noexec。CPU 200%、memory 2 GiB、swap 0、pids 256、120 秒墙钟和每路日志 512 KiB 均保持不变。安装不启动作业，不证明限制已生效。

## 资格探针的实际行为检查

- CPU：在 systemd probe unit 内读取准确 `cpu.max`，并在可用 CPU capacity 大于配额时以受限负载确认 throttling 事件增加。
- Memory/swap：读取准确 `memory.max=2147483648` 和 `memory.swap.max=0`；可信 helper 在该 cgroup 内触页分配超过 2 GiB，要求内核 `memory.events` 的 limit/OOM 计数实际增加，并确认 swap current 保持 0。
- PIDs：在 probe cgroup 内创建超过 256 个进程，确认 `pids.events:max` 增加。
- Scratch：在 unit 的私有 tmpfs 上写满 1 GiB 后验证下一字节失败；创建文件/目录至 inode 上限并验证越界失败。
- 进程树：保留一个可信 sleep 后停止 probe unit，逐一确认 PID 消失且 cgroup `populated=0`。
- 墙钟：另起相同 `RuntimeMaxSec=120s` 的可信 probe 实例，让其 sleep 180 秒；要求 systemd 真实记录 `Result=timeout`、PID 消失、cgroup 为空，记录实际 elapsed time。
- 网络/路径：用与 `RunFixedGo` 相同的 Bubblewrap mount/namespace 边界运行静态、可信 host-probe helper；确认 network namespace 与宿主不同且仅 loopback、宿主工作区/`/etc`/handoff 不可见、source/toolchain/deps 只读、scratch 可写，并拒绝 source 中指向宿主路径的 symlink。

任一实际行为断言缺失或失败均保持 `qualification=blocked`，不得创建 receipt 或启动候选。配置值/配置文件存在本身不构成资格证据。

### 上一安装版本的资源实测（2026-10-25）

root-controlled verifier 实际启动同一 systemd probe template；unit 内的 `User=chyinan` 可信探针行为均通过：CPU `cpu.max=200000 100000` 且 `nr_throttled` 增加；memory `max/oom` 事件增加并实际 OOM-kill 超限子进程；swap max/current 均为 0；pids max 事件增加（观察到 248 个测试子进程后被拒绝）；私有 tmpfs 显示 `size=1048576k,nr_inodes=4096,nodev,nosuid`，实写 1 GiB 后下一字节失败，inode 越界失败；实际 systemd 120 秒超时 `elapsed_seconds=120` 且 cgroup/进程树回收；固定 Bubblewrap network namespace、loopback-only、路径边界通过。

完整输出：`host-qualification-20261025/qualification-probe-root-full.log`；其 `probe_bundle_sha256` 为 `a660311791bf222409414b0c168f6626103fb4e9166150b69450b172233343be`。该次运行绑定旧 supervisor `b07af7…`，不能替代新 supervisor `a7a2a9…` 的资格探针。

### broker 安装后的授权负例（2026-10-26）

新 supervisor `a7a2a9…`、unit-control helper、custom action 和精确 `.pkla` 已 apply。`pkexec job-start`、`job-stop` 的非法 JobRun ID，以及未知 verb 都到达 root helper 后返回 `64`；helper 在调用 systemctl 前拒绝请求。该测试未启动 unit，`pkexec-v5-active-units.log` 为空。`systemd1.manage-units` 的直接 Polkit 权限仍未开放。

新 broker 方案使用现有 Jammy `pkexec` 和 local-authority custom action，不依赖未生效的 `.rules`，也不授予宽泛 `.pkla`。root-only 0700 的 Polkit local-authority 目录不会为 `chyinan` 放宽权限；readiness 由固定 `pkexec --verify-installation` 只读核对 root-owned seal 和 `.pkla` digest。当前 probe 输出为 `host-qualification-20261025/qualification-probe-root-schema134-disposition-fix.log`，`probe_bundle_sha256=7412bd2ed5fe86d89de3d1e193b1e9d0a4ebefe1dda0af8c31ca4ca75951b7c4`。root receipt qualification SHA-256 为 `62e82a5882918d2fecacd296d5d30d6b28884c897f064f74dc43e5f712b0e9f4`，boot ID `89e82fea-58e1-44c3-a79d-cc96ec7d8fcd`。对 staged tree 的 `--inspect-host` 返回 `qualified=true`。

### 完整本地 E2E — 2026-10-26

- run: `selfdev-e2e-20261026T063547`; disposable PostgreSQL 18.6 / Schema 134, Unix socket only, migration/least-privilege/cleanup PASS。
- actual fixed Go JobRun `job-cdbb806b0e578419a30b513d6d80a742` 在受限 systemd unit 内执行完整冻结测试，`outcome=passed`, `qualificationEligible=true`, `cleanupConfirmed=true`; root report 与请求、binding、当前 receipt 完全匹配。
- 正式 `go_patch` Artifact `1edfbc093b55f7345cd14d71cacc4e26` 发布一次，replay-verified；DeliveryManifest revision 2 为 `ready`；临时 owner 会话记录 `accepted`。该接受决定仅属于合成 disposable fixture，不代表对任何业务 Artifact 的用户接受。
- Windows Playwright 走真实 Vite、`RealWorkbenchApi`、Go Workbench HTTP handler 和 CSRF-protected installation-owner session，截图 `go-patch-workbench-diff.png` / `go-patch-workbench-disposition.png`。Diff 是只读；没有 apply、merge、deploy 或外部 action。
- 完整记录见同名 run 目录、PG runtime `20261025T223550Z-71394`、systemd request/result sidecars 和 `browser-complete.json`。`provider_egress=0`, `real_model_worker_started=0`, `automatic_apply/merge/deploy=NO`。

WSL 退出后 `/run` handoff 和 staged toolchain 会消失；本次未加 tmpfiles 或全局常驻配置。最终 E2E 已在 boot `89e82fea-58e1-44c3-a79d-cc96ec7d8fcd`、API 使用 staged toolchain 的条件下完成。E2E request/result sidecars 已复制到 slice evidence。收尾只停止本次创建的精确 keepalive 进程。

## 安装与卸载入口

现有脚本：`scripts/self-dev-go-job-host-root.sh`。现有非提权入口：`scripts/self-dev-go-job-host.sh inspect|prepare|verify|cleanup`。可信合成探针构建：`bash scripts/build-self-dev-go-host-probe.sh`。

以下保留已批准的受限 apply/qualification 重复操作顺序；本次 slice 已按该顺序完成安装、完整 probe 和同 boot receipt：

```bash
cd /mnt/d/Programs/Polis-cloud-main
bash scripts/self-dev-go-job-host-root.sh plan --target-uid 1000
# 安装器创建获批的空 /scratch mountpoint；custom action 和 pkla 目录已由系统包提供。
sudo install -d -m 0755 /usr/local/libexec
sudo bash scripts/self-dev-go-job-host-root.sh apply --target-uid 1000 --confirm
# 以 root 运行可信 host probe；若源码、runner、toolchain、boot 或 probe bundle 变化，须重新完成并签发新 receipt 后才可运行候选。
sudo bash scripts/self-dev-go-job-host.sh verify
```

回滚先停止实际存在的精确实例，然后：

```bash
sudo bash scripts/self-dev-go-job-host-root.sh cleanup --target-uid 1000 --confirm
```

cleanup 验证 marker、完整 binary/identity digest 且无活动实例后，删除两份 unit、旧规则及 custom action/pkexec 安装文件，并执行 daemon-reload。`/scratch` 只由 marker 验证后按 owner/mode 检查并 `rmdir`；非空/有挂载时保留。只 rmdir 空的 runtime handoff；非空证据保留。root-owned `/run/polis-self-dev-go-toolchain` 不是 installer 所有物，不由 cleanup 删除；它随 WSL boot 自动消失。共享父目录、源码、.tools、业务 DB 和运行中的 Polis 不删除。管理员后续另行安装的 qualification receipt 不属于本次 apply 的删除集合，撤销时需单独审阅。

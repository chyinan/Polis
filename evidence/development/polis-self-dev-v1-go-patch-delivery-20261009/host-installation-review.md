# 现有宿主安装方案实物展开（待批准）

基线 de9089bc。以下内容由现有 root helper 的 heredoc 展开，未安装任何文件。B 线重建产物后需要重新核对完整 SHA256；旧摘要不授权替换新产物。

## 身份与权限

supervisor 安装路径：`/usr/local/libexec/polis-selfdev-go-job-supervisor`。
完整 SHA256：`76fe177464c6d2024a5059e5b17a2d1e55a28dd544c655b5eb864434a24911ac`。
协议 `polis-go-job-supervisor@1`，profile `polis-go-test-fixture@1`，Linux amd64。

| 路径 | 拥有者和权限 | 谁可修改 |
|---|---|---|
| /etc/systemd/system/polis-self-dev-go-job@.service | root，0644 | 管理员 |
| /etc/systemd/system/polis-self-dev-go-probe@.service | root，0644 | 管理员 |
| /etc/polkit-1/rules.d/49-polis-self-dev-go-job.rules | root，0644 | 管理员 |
| /usr/local/libexec/polis-selfdev-go-job-supervisor | root，0755 | 管理员；chyinan 可读、执行 |
| 同路径 .identity.json | root，0644 | 管理员 |
| /etc/systemd/system/polis-self-dev-go-job-supervisor.sha256 | root，0644 | 管理员 |
| /run/polis-self-dev-go-job | chyinan:1000，0700 | 可信控制面用户；候选挂载不可见 |
| workspace 中的安装脚本、探针和 .tools/go | 开发用户可修改 | chyinan；不属于已安装 root-owned 产物 |

polkit 授权 chyinan 操作所有符合下方名称的实例，不限于一个 Company；Company/Task 权限由 Go API 另行检查。它只允许 start/stop/kill，不允许修改配置、任意 systemd service、任意 shell 或 sudo。候选不见 systemctl、宿主 cgroupfs、凭据、数据库和 handoff。

探针引用 workspace 中可修改的脚本。管理员签发 qualification receipt 前必须审阅实际探针源码、摘要和运行证据，不能仅凭 stdout 的 PASS。root-owned receipt 还绑定本次 boot、runner、toolchain tree、bwrap 和 probe digest。本轮不签发 receipt。

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
ExecStart=/usr/local/libexec/polis-selfdev-go-job-supervisor --job-id=%i --toolchain-root=/mnt/d/Programs/Polis-cloud-main/.tools/go
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
ReadWritePaths=/scratch /run/polis-self-dev-go-job/%i
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
ReadWritePaths=/scratch /run/polis-self-dev-go-job-probes/%i
StandardOutput=journal
StandardError=journal
```

## Polkit rule

目标：`/etc/polkit-1/rules.d/49-polis-self-dev-go-job.rules`。

```javascript
/*
# POLIS_SELF_DEV_GO_JOB_HOST_V1
*/
polkit.addRule(function(action, subject) {
  var unit = action.lookup("unit");
  var verb = action.lookup("verb");
  var allowedUnit = typeof unit === "string" &&
    /^polis-self-dev-go-(job@job-[A-Za-z0-9_-]{1,80}|probe@probe-[A-Za-z0-9_-]{1,80})\.service$/.test(unit);
  var allowedVerb = verb === "start" || verb === "stop" || verb === "kill";
  if (action.id === "org.freedesktop.systemd1.manage-units" && subject.user === "chyinan" && allowedUnit && allowedVerb) {
    return polkit.Result.YES;
  }
});
```

## 专用 cgroup、scratch 和变更范围

仅 Ubuntu-22.04 中这两个模板的实例。systemd 管理 `/sys/fs/cgroup/system.slice/polis-self-dev-go-job@job-<id>.service`，候选无写入控制器或迁出权限。没有全局 cgroup chmod、sudoers、WSL 配置、网络放行或物理块设备操作。

每个实例单独挂载 1 GiB tmpfs，4096 限制全部 inode（含目录、文件和链接）。保留 nodev/nosuid；Go 测试二进制需要执行权限，因此 scratch 不设 noexec。CPU 200%、memory 2 GiB、swap 0、pids 256、120 秒墙钟和每路日志 512 KiB 均保持不变。安装不启动作业，不证明限制已生效。

## 安装与卸载入口

现有脚本：`scripts/self-dev-go-job-host-root.sh`。现有非提权入口：`scripts/self-dev-go-job-host.sh inspect|prepare|verify|cleanup`。

以下仅展示，未获批准，不执行：

```bash
cd /mnt/d/Programs/Polis-cloud-main
bash scripts/self-dev-go-job-host-root.sh plan --target-uid 1000
# 先核对目录实际状态。只在不存在时创建两个父目录。
sudo install -d -m 0755 /usr/local/libexec /etc/polkit-1/rules.d
sudo bash scripts/self-dev-go-job-host-root.sh apply --target-uid 1000 --confirm
# 仅在安装获批并完成后运行可信探针；不运行候选。
bash scripts/self-dev-go-job-host.sh verify
```

回滚先停止实际存在的精确实例，然后：

```bash
sudo bash scripts/self-dev-go-job-host-root.sh cleanup --target-uid 1000 --confirm
```

cleanup 验证 marker、完整 binary/identity digest 且无活动实例后，删除两份 unit、polkit rule、安装的 supervisor/identity 和 sha256 文件，并执行 daemon-reload。只 rmdir 空的 runtime handoff；非空证据保留。共享父目录、源码、.tools、业务 DB 和运行中的 Polis 不删除。管理员后续另行安装的 qualification receipt 不属于本次 apply 的删除集合，撤销时需单独审阅。

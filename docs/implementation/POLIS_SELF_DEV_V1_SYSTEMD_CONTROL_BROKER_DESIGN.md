# POLIS Self-Dev 固定 Job 控制代理设计

状态：`IMPLEMENTED_AND_QUALIFIED`。窄 `pkexec` broker、root-owned read-only toolchain stage、systemd terminal-result reconciliation fallback 已实现。宿主 probe/receipt 与 Schema 134 / PostgreSQL 18 / real Go JobRun / Patch / Workbench diff/disposition E2E 均通过。当前 supervisor SHA256 `4e241c70eed64ee138769b5bf191eee18ee9fa167295a3f6a3bdfa47171c7fb3`，source identity `6be9cdf873e84f7699938332fd79c8999a857a539ee23b9ce1addcc4989971ab`。E2E 仅使用 disposable fixture；本文件不是更广泛 R1–R3/provider qualification。

## 背景与成功条件

最初 Job API 直接调用 `/usr/bin/systemctl` 启停固定的 Go Job unit；该 WSL 的 Polkit `systemd1.manage-units` 不授权普通用户。该直接路径已由下述窄 `pkexec` helper 替换。

同一系统的 Jammy `polkitd` 为 `0.105-33ubuntu0.2`，apt 没有更新候选，并使用 `polkitd-pkla` local-authority backend。不能用 `.pkla` 按 unit 名和 verb 限权。宽泛授权 `manage-units` 不可接受。

当前 root-installed supervisor 在真实 systemd unit 内通过 CPU、memory/swap、pids、scratch 容量和 inode、进程树回收、墙钟、禁网及路径边界检查。当前完整输出为 `evidence/development/polis-self-dev-v1-go-patch-delivery-20261009/host-qualification-20261025/qualification-probe-root-schema134-disposition-fix.log`，当前 boot-bound root receipt 已签发。早期旧 supervisor probe 仍作为历史证据保留。

成功条件：

- Polis API 只能通过固定 helper 请求只读 seal 验证、固定 job unit `start` 和 `stop`。
- 授权由精确 Polkit custom action 限定，不授予通用 systemd 管理权。
- 未知 verb、任意 unit、路径型或非法 JobRun ID 均拒绝；只读验证也不调用 systemd。
- 启动结果仍由现有 `SystemdExecutor` 观察。若 systemd 快速结束并清空 `InvocationID`，只有在新 handoff 中存在绑定正确且完整验证的 runner receipt、systemd 报告成功终态并确认 cgroup 为空时，才完成当前 Run 的回收；回执不明仍保留 `outcome_unknown`，不会重启同一 JobRun。
- Host qualification、JobRun receipt、Patch、Workbench 和人工 disposition 语义不变。

## 采用方案

采用一次性 root-owned unit-control helper，并用 `pkexec` custom action 授权。它不是常驻 D-Bus daemon。`pkexec` 通过 Polkit D-Bus 请求授权；当前 local-authority 可以按 custom action ID 配置 `.pkla`。

当前系统已有 `/usr/bin/pkexec`，版本为 `0.105-33ubuntu0.2`。本方案不安装或升级系统包。

长驻 D-Bus broker 也能提供相同边界，但需要新增 root daemon、D-Bus 接口和 Go 客户端依赖。当前先采用一次性 helper，减少常驻服务和 IPC 协议。若 custom action 与 `.pkla` 在本机组合测试中不能按预期授权，就停止实现，不改用宽泛策略。

Polkit action XML 用 `org.freedesktop.policykit.exec.path` 将 action 绑定到固定 helper。官方也支持用 `org.freedesktop.policykit.exec.argv1` 区分首个参数，但 WSL 实测显示未知首参仍会进入 helper。本设计不依赖 argv1 做安全边界，改由 helper 严格校验完整参数。官方说明见 [pkexec reference](https://polkit.pages.freedesktop.org/polkit/pkexec.1.html)。

拟定 custom action：

| Action ID | 绑定路径 | helper 允许的操作 |
|---|---|---|
| `org.polis.selfdev.go-job.control` | 固定 helper path | 只允许只读 `--verify-installation`、`job-start` 和 `job-stop` |

`/etc/polkit-1/localauthority/50-local.d/` 中的 `.pkla` 只把这个 action 授予 `unix-user:chyinan`。不授予 `org.freedesktop.systemd1.manage-units`。Host qualification probe 继续由管理员执行，不向 Polis 用户开放 probe 启动权限。

policy XML 把一个 custom action 绑定到 helper path，并设默认拒绝。`.pkla` 只列这个自定义 ID，身份只列 `unix-user:chyinan`。

## 控制代理契约

新增 `cmd/polis-selfdev-go-unit-control`，安装为 root-owned `/usr/local/libexec/polis-selfdev-go-unit-control`。命令接受只读 seal 验证，或以下两个 systemd 操作：

```text
pkexec /usr/local/libexec/polis-selfdev-go-unit-control job-start <job-id>
pkexec /usr/local/libexec/polis-selfdev-go-unit-control job-stop <job-id>
```

helper 不接收完整 unit 名、systemctl 参数或 shell 文本。它验证参数数量和固定动作，再由 `job-id` 拼出 `polis-self-dev-go-job@<job-id>.service`。ID 必须通过与 `core.ValidID` 一致的字符边界，并具有 `job-` 前缀。

helper 使用固定 `/usr/bin/systemctl` 路径和固定参数数组，不调用 shell。`job-start` 只执行 `start --no-block`；`job-stop` 只执行 `stop`。每次操作有明确超时和有界输出。helper 不读写 CAS、数据库、候选源码或 qualification receipt。

安装器保存 helper 的 root-owned SHA256 seal。`SystemdExecutor` 在调用前检查 helper 路径、root owner、不可写权限和完整摘要。helper 只输出固定、有限的操作结果，不接收用户提供的可执行路径或 systemctl 参数。

`.pkexec` action 只授权固定 helper 的路径。它不限制命令行参数；helper 必须独立校验全部参数，不能把 Polkit action 当成输入验证。实测未知首参仍会进入 helper，因此 helper 必须在调用 systemctl 前拒绝未知 verb 和多余参数。API 的只读 preflight 还会调用 root helper 的 `--verify-installation` verb；该 verb 只比对 root-only `.pkla` 内容与 seal，不调用 systemctl。这样不必放宽 Jammy 的 `/etc/polkit-1/localauthority` 目录权限。Polis 的 Company/Task/JobRun 授权继续由现有 API 检查；OS helper 只约束可操作的 systemd unit 范围。

## 与现有执行路径的接线

只替换 `internal/gotestsupervisor/systemd.go` 中的 `systemctl start/stop` 调用：

- `SystemdExecutor.Run` 调用 `pkexec ... job-start <job-id>`。
- `SystemdExecutor.Stop` 调用 `pkexec ... job-stop <job-id>`。
- `systemctl show` 仍由非特权 API 进程只读调用。
- `Collect` 仍只观察已有 unit 和结果，不调用 start。
- JobRun 的一次性 claim、CAS handoff、InvocationID、result/log 验证和进程树回收均保持原样。

如果 `pkexec` 明确拒绝授权且未启动 helper，记录启动失败，不写 PASS。若 pkexec、helper 或 systemd 返回超时，沿用现有 reconcile/stop 边界；无法证明是否启动时必须记为 `outcome_unknown`。不得自动重放同一 RequestID。

## 安装范围与身份

安装器管理以下精确文件，并通过 marker/hash seal 限制 cleanup：

- `/usr/local/libexec/polis-selfdev-go-unit-control`，root:root 0755。
- `/usr/share/polkit-1/actions/org.polis.selfdev.go-job.policy`，root:root 0644。
- `/etc/polkit-1/localauthority/50-local.d/50-polis-selfdev-go-unit-control.pkla`，root:root 0644。
- broker SHA256 seal，root:root 0644。
- `/run/polis-self-dev-go-toolchain` 是本次 boot 的 root-owned 0555 staged tree；不是 installer 所有物，随 WSL 关闭自动消失。Job unit 固定用此路径，API 必须以同路径配置 `POLIS_GO_TOOLCHAIN_ROOT`。

清理时只删除带本工具 marker、且摘要匹配的精确文件。不得添加通用 `.pkla`、sudoers、cgroup 权限、全局挂载或 WSL 配置。

改动 `internal/gotestsupervisor/systemd.go` 会改变 supervisor source identity 和 build SHA256。当前 root runner SHA256 `4e241c70eed64ee138769b5bf191eee18ee9fa167295a3f6a3bdfa47171c7fb3`，source identity `6be9cdf873e84f7699938332fd79c8999a857a539ee23b9ce1addcc4989971ab`。已精确 apply 并绑定当前 host receipt。

## 验证边界

离线测试需要覆盖 helper 参数白名单、JobRun ID 边界、固定 systemctl argv、超时和输出上限、只读 `--verify-installation` seal 检查，以及 `pkexec` custom action 的 helper-path 绑定和 `.pkla` 身份范围。负例必须证明 helper 不能启动任意 unit，也不能接受额外参数。

安装前的授权测试不启动候选。用非法 ID 调用 helper，确认 `pkexec` 进入 helper，且 helper 在调用 systemctl 前拒绝请求。再测未知 verb、额外参数和 path-like ID，并确认没有 systemd unit 启动。此后追加 root-only readback seal 验证，最终 root-installed helper 及 `.pkla` digest 检查通过。完整宿主行为检查也已在当前 boot 重跑并签发 receipt。

安装后先用非候选请求验证 custom action 的授权和拒绝路径。由管理员在同一 WSL boot 重跑完整宿主行为探针，审阅 supervisor、broker、policy、toolchain、bwrap 和 probe bundle 摘要；当前 root-owned receipt 与 API/browser `executeGoTest=true` 全链均通过。

WSL `/run/polis-self-dev-go-job` 与 `/run/polis-self-dev-go-toolchain` 会在 WSL instance 退出后消失。E2E 必须在同一 boot/handoff 生命周期内运行；当前最终 PASS evidence 与系统d request/result sidecars 已复制到 slice evidence。窄 broker 与 host probe 不会自动启动候选；显式 `executeGoTest=true` 仍是必需门。

始终保持：

```ini
provider_egress = 0
real_model_worker_started = 0
automatic_merge = NO
automatic_deploy = NO
slice_status = PASSED_LOCAL_E2E
```

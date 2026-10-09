# 固定 Go supervisor 与 PostgreSQL 持久化：PARTIAL

本轮完成可构建的 supervisor、prepare 产物接线和最小 Go JobRun 结果持久化。完整数据库测试基线已修通。**整个离线代码交付切片仍未完成，宿主资格仍为 BLOCKED，候选项目代码未执行。**

## 开发身份与范围

唯一工作目录：`D:/Programs/Polis-cloud-main`。开始 HEAD 为 `3e333100a22ee01e38879a90a614434717e1b896`，接续 `803509f799bac854f24530a45d103e4b45cbc73c`；开始时 tracked worktree/index 干净，只有预存 reconciliation 未跟踪目录。未读取、修改或暂存该目录，未访问旧 checkout 的源码/数据。

源码 checkpoint：`cdfcd1695a8e179a9539c4c69acc7f99dbdf034b`，明确标注 PARTIAL。之后只补齐 PG fixture 隔离/管理员初始化顺序、客户端 smoke 的专用运行时配置、测试 harness 别名、命令记录格式及独立证据。生产 Go 源码及 supervisor 产物身份未漂移；最终测试覆盖这些补充后的状态。没有 reset、clean、stash、切换 checkout、改写历史、push 或发布版本。

历史 qualification/evidence 保持不变。本轮原始失败及其分类见 `pg-failure-inventory.md`：保留上轮 138 个失败父用例的原始错误，不把早期级联拒绝一概归因于旧 fixture。中间失败运行全部保留，最后的完整成功运行独立保存。

## 最终验证

| 检查 | 真实结果与证据 |
|---|---|
| Go tests | `go-tests-complete.jsonl`，exit 0；1617 pass、0 fail、235 skip。此模式没有 PG DSN，数据库测试的 skip 不作为 PG PASS。 |
| Go race | `go-race-complete.jsonl`，exit 0；1617 pass、0 fail、235 skip。 |
| Go vet | `go-vet-complete.log`，exit 0。 |
| Linux / Windows build | `linux-build-final.log` / `windows-build-final.log`，均 exit 0；Windows 是构建结果，不是 Windows executor 资格。 |
| 完整 PG-backed suite | `postgres-runtime/20261009T010709Z-313`，exit 0；1867 pass、0 fail、7 skip，含子用例；顶层 1254 pass、7 skip。没有数据库用例因 DSN/管理员连接缺失而跳过。 |
| migrations / schema | PostgreSQL 18.6，`schema_identity=18:132`；1–129 的文件和 hash 未改。129 实际执行，具体数据库缺陷通过前向 130–132 修复；不宣称旧 schema 129 的完整基线已通过。 |
| runtime role / grants | NOSUPERUSER、NOCREATEDB、NOCREATEROLE、NOINHERIT；SELECT/INSERT/UPDATE、sequence USAGE/SELECT；实测读写成功、DELETE 拒绝。没有扩大 grants。 |
| Bash / PowerShell | 最终 Bash 工具语法与报告生成器语法/实际运行通过。 |
| frontend，源码未改 | 157 tests、typecheck、lint、build 全部 exit 0；没有关闭规则或扩大 ignore。 |
| diff-check | 完整 staged diff 检查 exit 0；只修正新命令元数据的尾随空格，原始测试输出未重写。 |

完整 Go 命令：`bash scripts/go.sh test -p 1 -count=1 -json ./...`、`test -race -p 1 -count=1 -json ./...`、`vet ./...`。Linux/Windows 构建使用明确 GOOS/GOARCH 和 CGO_ENABLED=0。完整 PG 入口仍是 `bash scripts/self-dev-go-job-pg-test.sh`，不是另一套数据库基础设施。原始日志及测试清单见 `postgres-final-tests.csv`、`postgres-final-skips.json`、`postgres-run-inventory.json`。

曾有一次中间完整运行 exit 0，但 admin failure-injection 与 installed-client smoke 仍跳过；未将其作为最终数据库门禁。修正初始化顺序与专用客户端路径后，完整重跑得到以上最终结果。中间 vet 因 fixture 调整留下未用 import 而失败的记录也保留，最终已修复并重跑。

### 专用数据库身份

最终 template database：`polis_r0_self_dev_go_1791508029_313`；runtime role：`polis_self_dev_go_runtime_1791508029_313`。每个测试使用同一 cluster 内的独立 `polis_r0_case_*` 数据库；只通过私有 Unix socket 访问，TCP listener 关闭，CAS、临时目录和备份均为新建测试资源。失败注入使用该 case 的管理员连接，确实观察到预期数据库错误并验证事务回滚。真实 migrations、hash ledger、dump/restore 与客户端版本检查均执行。cluster 已正常停止并清理，原始日志保留；没有使用业务 DB 或历史 specimen。

7 个未运行用例分别是 Windows Chrome host、三个固定 Codex native binary 相关用例、两个依赖历史 R0.2 evidence 的用例，以及缺少 controlled-provider runtime manifest 的身份 probe。它们的具体名称/原因全部保留，不计入 PASS，也未为通过它们而启用真实 provider。

## Supervisor 与 prepare

正式入口：`cmd/polis-selfdev-go-job-supervisor`。构建入口：`bash scripts/build-self-dev-go-supervisor.sh`。实际产物：`D:/Programs/Polis-cloud-main/bin/polis-selfdev-go-job-supervisor`，Linux amd64、静态 ELF；不是 `bin/polis` 或 PATH 替代程序。

| 身份 | 值 |
|---|---|
| 协议 / profile | `polis-go-job-supervisor@1` / `polis-go-test-fixture@1` |
| runner SHA256 | `76fe177464c6d2024a5059e5b17a2d1e55a28dd544c655b5eb864434a24911ac` |
| production source identity | `f1ff9fd1b4b008afca1dc74c4e8ff23d527be06b3247323f77a82d7eda0550af` |
| 实际 Go | `go1.26.8 linux/amd64` |
| Go binary SHA256 | `d9a2fa19c7ef8b57f420012c21f49f235c46f08a68c12077d9c753dbb6ccdc34` |

`--help`、`--protocol-info`、`--verify-build`、真实冻结输入的 `--check-input`、只读 `--inspect --job-id=…` 均真实调用。prepare 核对 ELF、平台、协议、当前 production source stamp 和产物 digest，实际结果：

```ini
supervisor_binary_built = YES
setup_plan_valid = YES
privileged_apply_authorized = NO
host_execution_profile_qualified = NO
```

生产运行入口只接收固定 job ID 和批准的 toolchain root，从固定私有 runtime handoff 读取 canonical request。实现包含真实的宿主检查、只读 materializer、固定 bwrap argv、清洁环境、有界日志、监控/终止、整组回收确认及结构化结果；没有任意 shell 工具或回显成功的空壳。

执行前必须处于正确 systemd job cgroup，实际资源值和 tmpfs 边界匹配，并具备绑定本次 boot/binary/toolchain/bwrap/probe 的 root-owned qualification receipt。CPU 200%、memory 2 GiB、swap 0、pids 256、墙钟 120 秒、scratch 1 GiB/4096 inode、stdout/stderr 各 512 KiB 均不放宽。候选只看到只读源码/工具链/依赖和受限 scratch，不见宿主 cgroupfs、数据库、凭据、`.git` 或 runtime handoff。

以上执行代码已构建/测试，**不等于真实内核限制已经验证生效**。本轮只验证了不具备前提时确实 fail closed。

## Go JobRun 持久化

复用现有 JobRun 状态机、schema 129 的 binding/result 表和 CAS 日志 API：冻结 base/candidate → accepted intent → 实际 supervisor 预检 → blocked/failed/cancelled 结构化结果。生产预检路径没有成功派发候选的开关。

真实 PG 用例 `TestGoJobPersistsRealHostGateAndSimulatedStartFailure` 通过：

- 显式导入可信临时 Git commit，经正式授权 WorkspaceTree 修改和快照路径建立精确 revision/digest。
- 实际 supervisor 返回宿主拒绝，正确落库；没有候选启动、退出码或成功测试 receipt。
- 初始化/启动失败的事务用例明确标为 `executionKind=simulation`，不是实际执行资格。
- 相同报告重复提交只有一份结果/终态；不同终态冲突。
- 错误 candidate revision、另一 JobRun 的 receipt、伪造 no-start PASS 均拒绝。
- 有界日志及 digest 经真实 CAS 读回；Task qualification 数保持 0。

未启动与未观察退出的状态不会伪造成 exit code 0。launcher 与候选的启动/退出事实分开记录。没有向公共 API 开放上传任意“成功报告”的接口。

## 宿主与下一步授权

WSL Ubuntu-22.04，内核 `6.6.87.2-microsoft-standard-WSL2`，systemd 正常。bwrap 0.6.1 的可信禁网/只读/退出探针通过；专用 systemd profile 未准备，CPU 未委派到当前用户子树，`cgroup.kill` 不可写，1 GiB/4096 inode 的 job scratch 未生效，资格 receipt 未安装。因此 host qualification 为 BLOCKED。

精确安装路径、需批准的 root 命令、核验入口和回滚见 `host-preparation.md`。下一步是批准专用 unit/polkit/binary 安装并运行可信 probes；不是放开整个 cgroupfs、免密 sudo 或关闭全部 WSL。安装/构建成功也不能自动签发资格 receipt或执行候选。

## 仍余接线与状态

| 项目 | 状态 |
|---|---|
| source_integration | PARTIAL：本轮 supervisor/预检/终态持久化完成；成功派发与交付尚未接通。 |
| host_execution_profile_qualification | BLOCKED，未执行 privileged apply。 |
| postgres_migration_and_integration | PASS，18.6 / schema 132，完整重跑。 |
| real_go_job_execution | NOT_RUN。 |
| patch_delivery_and_review | NOT_RUN，仍需正式接线。 |
| frontend_lint | PASS，frontend 未改。 |
| provider_surface_impact | NONE；注册表与真实 @4 surface 未改，无 fake-only 扩展。 |

后续还需要：正式 systemd dispatch/请求交接/结果观察和恢复；合格 Go receipt 与 TaskValidationBinding/qualified checkpoint 的绑定；可信 Patch replay → CAS Artifact → DeliveryManifest/publication policy；Workbench bounded diff、日志、下载和 UserDisposition 的真实 API/浏览器审核链。本轮未把 workspace_snapshot 改名冒充 ready Patch，也未将测试 double 发布成 qualification。

首次真实模型自开发还需要独立开发 Worker surface/admission 的授权和资格，以及上述真实 Go/Patch 离线链通过；不能利用当前 @4 或创建真实 Worker 绕过这些门禁。本轮完成后停止，不自动进入该阶段。

```ini
self_dev_go_job_patch_offline_slice = PARTIAL
provider_egress = 0
real_model_worker_started = 0
candidate_project_execution = NOT_RUN
live_canary = NOT_RUN
automatic_merge = NO
automatic_deploy = NO
```

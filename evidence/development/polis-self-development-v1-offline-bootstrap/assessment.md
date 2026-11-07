# Polis 自开发 V1 离线启动评估

日期：2026-10-08

状态：`self_development_offline_bootstrap = BLOCKED`

## 结论

当前代码支持创建通用 Mission、执行产品 Task、保存候选 Artifact，并在 Workbench 预览 Artifact。代码没有自开发 Mission、授权仓库绑定、多文件源码工作区、项目 build/test 命令或人工接受/拒绝接口。

现有 WSL `bwrap` verifier 只运行固定 Go fixture。它以只读方式挂载候选目录并隔离网络，但没有 CPU、内存、进程数和磁盘配额。R0.1 报告也明确不声明一般恶意 Worker 隔离资格。因此本次不能执行 disposable 项目的真实代码测试来冒充安全闭环。

本次没有启动 Polis Mission、Codex Worker、Codex CLI、Skill 或 MCP，也没有读取 ambient Codex 凭据。`provider_egress = 0`。没有创建 clone/worktree，没有运行目标项目命令，没有生成 Patch Artifact。运行中的 Polis 和数据库未被本次操作访问或修改。

## 能力审计

| 能力 | 分类 | 已核验事实 |
|---|---|---|
| Workbench 创建开发 Mission | PARTIAL | Workbench 已能创建通用 Mission；创建请求只有标题、目标和验收条件，没有开发类型或仓库绑定。 |
| 授权 Git 仓库 | MISSING | 未找到仓库授权记录、授权 API、revoke 路径或 Mission 到仓库的绑定。 |
| 独立 clone 与多文件源码读写 | BLOCKED | Worker Workspace 当前是一个 `Content` 字符串，WorkBench 将其投影成 `workspace.txt`。没有 clone/worktree 或多文件操作；当前隔离环境也未获一般恶意代码执行资格。 |
| 受控 build/test 命令 | BLOCKED | 产品 `workspace_check` 运行任务验收检查，不运行项目 build/test。现有 `runner.Verifier` 针对固定单文件 Go fixture 和冻结测试，不是通用项目执行器；资源配额资格缺失。 |
| Codex 作为真实开发 Worker | PARTIAL | Codex Runtime、`RealProviderWorkerAdapter` 和 WorkerSession 路径存在；当前 Worker 工具面只支持单 blob 工作区，没有自开发工具与仓库绑定。新工具面还需独立资格核验。 |
| 加载已批准 Skill | MISSING | R0.8 仅实现 Skill 候选登记与读取。登记写入 `candidate`；没有批准、加载或执行流程。 |
| 使用已批准 MCP | MISSING | R0.8 仅登记 MCP 定义，状态为 `unverified`；没有批准、连接或调用流程。 |
| 保存 diff、测试日志和 Patch Artifact | PARTIAL | 现有交付事务能保存候选 Artifact，Workbench 能读取 Artifact 和检查记录；没有源码 diff、Patch 类型或项目测试日志归档。Artifact 当前受 1 至 4096 字节限制。 |
| Workbench 人工接受或拒绝 | MISSING | Workbench 能查看候选成果；任务命令只有创建、启动和取消，没有人工接受/拒绝候选 Artifact 的命令或状态。 |

## 可复用组件与实际边界

- Mission：Workbench `POST /api/workbench/companies/{companyId}/missions` → `control.Service.CreateMission` → `Kernel.TXCreateMissionGoalWithAcceptance`。
- Task 与交付：复用 Company、Mission、Task、WorkerSession、TaskValidationBinding、digest/revision CAS、确定性候选 Artifact 发布事务。
- Worker：Codex Runtime 与 `RealProviderWorkerAdapter` 已存在；当前只覆盖产品 Worker Task 与现有工具面。
- Workbench：现有 read store 可呈现 Mission、Task、检查收据和 Artifact 预览。
- Skill/MCP：复用 R0.8 Company 级候选目录；目录记录本身不授予执行能力。
- verifier：`internal/runner/verifier.go` 只运行固定 Go fixture，候选目录只读挂载，网络隔离，单次输出上限 32 KiB，外层时限 45 秒。它没有通用仓库读写或完整资源配额。
- Codex 进程的 WSL 启动路径将专用 HOME 绑定到 `/home/codex`，工作目录 `/work` 是空目录；该路径没有授权的 Polis 仓库挂载。

## 阻塞与安全边界

1. 增加 Company 级仓库授权记录，绑定规范化仓库身份、授权者、权限、基线 revision 和撤销状态；Mission Start 必须引用有效授权。
2. 建立独立候选源码副本。Worker 只能通过带 Task/session 身份、revision CAS 和路径白名单的工具读写该副本；不能提交任意主机路径。
3. 先资格化实际执行环境，再开放项目命令。资格记录至少要证明无外网、显式环境 allowlist、工具链只读、仅候选目录可写、CPU/内存/PID/文件大小/磁盘/时限上限、完整进程树回收，以及符号链接和路径穿越防护。
4. 冻结基线测试与验收策略，拒绝删除或修改基线测试来取得 PASS。检查结果必须来自真实命令退出状态和真实日志。
5. 新增由控制面生成的 diff、测试日志和 Patch Artifact；Workbench 提供只读审阅及明确的接受/拒绝记录。接受不能隐式 merge、push 或 deploy。
6. 为开发 Worker 绑定经过审查和批准的 Skill。MCP 可保持关闭，不是首条本地源码修复的前提。
7. Codex 真实执行必须先核验显式 credential binding、provider endpoint/runtime/model readiness，以及开发 Worker 和精确工具面的执行资格。禁止读取 ambient Codex 凭据。

本任务没有执行 build/test 或 Disposable Project E2E。原因是当前没有符合上述资源控制要求的通用执行边界；因此没有离线闭环通过证据，不能标记 `PASSED`。

## 资格记录与下一轮预算建议

`spec/design-v0.4.5/product/RELEASE_SCOPE.json` 将 REQ-38 标记为 `planned`、`not_implemented`、`not_run`。`spec/design-v0.4.5/tests/traceability.json` 将其映射到 WF-16 至 WF-20。`TEAM_COVERAGE.json` 是 `execution_enabled=false` 且资格为 `unverified` 的设计稿，不是执行资格证据。REQ-38 的外部检索/浏览器资格保持独立，本任务没有放宽或替代它。

首个真实 Codex 开发 Mission 的预算建议如下。此建议不是授权：

- 1 个 Mission，1 个开发 Task，1 个 provider WorkerSession，并发数 1。
- `gpt-5.6-luna/medium`，最多 1 个 turn、32 次工具调用、总计 10 分钟。
- 0 个 High turn，最多 1 次授权、1 次 reservation、预期 1 次 provider egress。
- 未知结果不自动重试，不增加预算，不另建 Worker；先冻结证据并结束该次 Mission。

首个任务建议从用户已确认、可稳定复现且影响范围只在 `internal/core` 的小型缺陷中选择。允许修改一个纯逻辑源文件和对应回归测试；保护基线测试，不改数据库、provider、权限规则或部署流程。本次只做源码审计，没有确认具体缺陷，因此不指定虚构的修复项。

## 文档状态说明

当前 dirty worktree 中的 `AGENTS.md`、`NEXT_SLICE.md` 和 `PROGRESS.md` 有未提交内容及过期/冲突交接。顶部记录分别仍指向 R0.6 或 R0.5B，后段记录 R0.8。本文按用户在本任务提供的范围和 R0.8 capability catalog 报告审计，没有改写这些既有文档。

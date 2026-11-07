# Polis 自开发 V1 离线启动

状态：BLOCKED。详见 `evidence/development/polis-self-development-v1-offline-bootstrap/assessment.md`。

## 现状

- Workbench 可创建通用 Mission；没有开发 Mission 类型或仓库授权绑定。
- Task Workspace 是一个 `Content` blob，不是多文件源码树。
- 产品 `workspace_check` 是验收检查，不是通用 build/test 执行器。
- Codex Worker Runtime 存在，但没有自开发仓库工具面。
- Skill/MCP 只能登记为候选/未验证定义。
- Artifact 可预览；没有 diff Patch Artifact 流程，也没有人类接受/拒绝动作。

## 隔离结论

现有 `runner.Verifier` 使用 WSL `bwrap`、禁网、只读候选目录和固定 Go fixture。没有 CPU、内存、PID、文件大小和磁盘配额。R0.1 报告明确不声明一般恶意 Worker 隔离资格。本次没有执行目标项目代码，`provider_egress = 0`，没有真实 Mission 或 E2E。

## 关键源码

- Mission：`internal/workbench/http.go`、`internal/control/service.go`、`internal/kernel/mission.go`
- 单 blob Workspace 与 CAS：`internal/kernel/worker.go`、`internal/kernel/employee_ops.go`、`db/migrations/00002_workers.sql`
- 当前验收与交付：`internal/kernel/product_validation.go`、`internal/kernel/product_delivery.go`
- Worker 工具面：`internal/codex/tools.go`
- 固定 verifier：`internal/runner/verifier.go`
- R0.8 Skill/MCP 候选目录：`internal/kernel/capability_catalog.go`、`db/migrations/00012_r07_capability_catalog.sql`
- REQ-38 范围状态：`spec/design-v0.4.5/product/RELEASE_SCOPE.json`

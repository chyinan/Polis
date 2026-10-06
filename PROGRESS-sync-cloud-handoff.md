# 进度追踪：同步云端最新代码并按 handoff 继续

> 创建时间：2026-10-06 | 状态：进行中

## 目标
同步 Polis 仓库的云端最新版本代码，并依据 handoff 文档继续当前工作。

## 成功标准
- 云端最新 `origin/main` 已落到独立干净 worktree，原本地工作安全保留。
- 读取当前 handoff/进度文档，确定 Slice 272 之后的真实未完成工作。
- 按项目约束继续实施必要工作，并完成与范围匹配的离线验证。

## 已读文件
- `AGENTS.md` — 云端当前 handoff、授权边界和提交/推送要求
- `docs/implementation/CLOUD_CODEX_HANDOFF.md` — Slice 272 当前续作指针
- `docs/implementation/NEXT_SLICE.md` — 切片状态与验证边界
- `docs/implementation/PROGRESS.md` — 历史进度与证据索引
- `C:\Users\chyinan\.agents\skills\coding-effectively\SKILL.md` — 编码原则与必需子技能
- `C:\Users\chyinan\.agents\skills\functional-core-imperative-shell\SKILL.md` — 代码文件分类及纯核心/副作用外壳边界
- `C:\Users\chyinan\.agents\skills\defense-in-depth\SKILL.md` — 跨边界分层校验规则
- `C:\Users\chyinan\.agents\skills\howto-develop-with-postgres\SKILL.md` — PostgreSQL 事务、命名和读写分离规则
- `C:\Users\chyinan\.agents\skills\test-driven-development\SKILL.md` — 先失败测试再实现
- `C:\Users\chyinan\.agents\skills\investigating-a-codebase\SKILL.md` — 入口/搜索/引用跟踪调查流程
- `C:\Users\chyinan\.agents\skills\verification-before-completion\SKILL.md` — 完成前必须有新鲜命令证据
- `C:\Users\chyinan\.agents\skills\requesting-code-review\SKILL.md` — 完成前代码审查循环
- `C:\Users\chyinan\.agents\skills\using-git-worktrees\SKILL.md` — 隔离 worktree 与干净基线验证

## 当前进度
REQ-40 ready Manifest 完成命令、owner/CSRF 路由、Mission 生命周期路由和 terminal backlog Schema 112 已实现；开始收集完整验证证据。

## 下一步
运行目标包编译/测试、前端构建与 diff/hash 校验，更新 Slice 273 handoff/evidence 文档，然后提交并推送分支。

## 发现的关键信息
- 远端默认分支是 `main`，最新提交为 `91b1520`（Slice 272）。
- 原工作区存在大量本地修改/未跟踪文件；完整 stash 因特殊文件失败，但原目录保持原样。
- 云端当前来源 Schema 111、记录运行时 Schema 108；真实 Worker/provider、MCP、GitHub、QQ 和生产操作未授权。
- `howto-functional-vs-imperative` 子技能路径未找到，已由 FCIS 技能直接覆盖其核心要求。
- 基线 `rtk bash scripts/go.sh test ./...` 已实际运行：既有失败位于 `internal/control` 9 项、`internal/kernel` 1 项、`internal/provider` 1 项，未涉及 REQ-40 当前代码改动；其余包通过。
- 为本次续作创建分支 `codex/req40-manifest-routing`。
- `internal/kernel` 的 REQ-40 目标测试、`internal/control`/`internal/workbench` 编译级测试与 `db` 迁移/hash 测试均已通过；PostgreSQL-backed completion/routing 尚未执行，因为当前没有专用 DSN。

## 执行计划
1. 以纯函数和失败测试定义完整 `ready` Manifest 的证据绑定与 canonical digest。
2. 增加追加式 Manifest 完成命令与 owner-authenticated Control 路由，不修改历史 revision。
3. 将 `changes_requested` 按 active/paused 与 terminal Mission 分别路由至 formal change request 或 Company backlog。
4. 运行目标测试、构建、diff/hash 校验，更新 handoff/evidence，提交并推送分支。

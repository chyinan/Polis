# 进度追踪：优先实现不依赖真实环境的剩余任务

> 创建时间：2026-10-06 | 状态：进行中

## 目标
从当前 R1–R3 开放需求中筛出不依赖真实主机、账号、Provider、MCP、WorkerSession 或安装者政策的代码任务，优先实现并验证。

## 成功标准
- 明确区分可离线实现、需要政策/授权、需要真实环境资格的任务。
- 至少完成一批范围明确的离线代码闭环，并有失败测试、通过测试和构建证据。
- 不启动真实 Worker/provider/MCP/GitHub/QQ 或业务账号，不把未运行场景标为通过。

## 已读文件
- `summary/req40-delivery-continuation.md` — Slice 273 当前交付完成与路由边界
- `docs/implementation/NEXT_SLICE.md` — 下一项包含 backlog read projection、deadline/expiry、closeout policy、backfill
- `docs/implementation/OPEN_QUALIFICATION_BLOCKERS.md` — 22 个开放 REQ 及其环境/政策阻塞
- `internal/workbench/artifact_delivery_lifecycle.go` — DurableDelivery read path and database-backed lifecycle projection
- `frontend/src/domain/workbench.ts` — Workbench durable delivery DTO types
- `frontend/src/domain/workbench-validation.ts` — runtime validation boundary for durable delivery responses
- `frontend/src/pages/SubpagePanels.tsx` — existing durable delivery presentation component
- `summary/req40-backlog-read-projection.md` — Slice 274 scope and invariant summary

## 当前进度
已完成需求筛选；REQ-40 Company backlog 只读投影已接入 Go read store、TypeScript validation 和 Workbench 展示，兼容旧 runtime 的空投影 fallback、Schema 114 索引和 scope 测试已补齐；legacy frontend response test 与 build 已通过。

## 下一步
提交并推送 Slice 274；保留前端测试文件中三个既有无关失败及 PostgreSQL runtime 未执行的记录。

## 发现的关键信息
- REQ-36/37/38 与部分 REQ-40 工作需要 owner policy、真实主机、账号或运行时迁移，当前不应猜测。
- Schema 113 已持久化 terminal `delivery_feedback_backlog_events`，但目前只有写入路径，没有 Workbench read projection。
- 新投影限制为当前 Artifact delivery、最多 32 条、只读 `open` 事件；不创建任务、不唤醒 Mission、不改变 terminal 状态。
- 当前 worktree 不需要真实 PostgreSQL/Worker/provider/browser，适合直接在 WSL 继续开发。

# Polis R1 Workbench：Current Backend → Workbench Entity Mapping

> Product Track 首轮映射（2026-09-14）。design pack 只提供产品意图；下表的事实来源以当前 repo 的 schema、Go 类型与已保存证据为准。

## Mapping

| UI entity | current backend source | available now? | mock needed? | API missing? | semantic caveats |
|---|---|---:|---:|---|---|
| `ViewMeta / Scope` | `companies.id`, `companies.company_seq`; `Kernel.Snapshot`; `C-WORKBENCH 40.3` 建议的 `schema_version/company_id/entity_revision/snapshot_cursor/observed_at/data_mode` | 部分 | 是 | `GET /api/workbench/companies/:companyId/overview` 与 activity snapshot | 公司作用域必须进入每个 query key；`company_seq` 是公司事件头，不是时间戳；fixture 明示 `simulated`。
| `CompanyOverview` | `companies`, `missions`, `tasks`, `employees`, `events`; `Kernel.Snapshot` 只读窄快照 | 部分 | 是 | overview 聚合 read API | 首屏回答使命、有效进展、阻塞/决策、资源口径；不要用总完成率或员工数量制造经营结论。
| `Mission` / `MissionDetail` | `missions.state`, `missions.contract`; `contract_revisions`; `Kernel.Snapshot.MissionState` | 部分 | 是 | mission detail read API | `Mission` 是有界目标/授权周期；`ContractRevision` 是版本化合同，不是 view/render state。
| `Employee logical identity` | `employees.id`, `employees.epoch`; sqlc `dbgen.Employee` | 是（身份/epoch） | 是（display name/role） | enriched employee detail read API | Employee 不是 session；ID 稳定，模型更换不能新建员工卡。
| `WorkerSession` | `worker_sessions.employee_id/task_id/generation/epoch/incarnation/profile/state/tool_call_limit/tool_calls_used`; `dbgen.WorkerSession` | 是（schema） | 是（view aggregation） | employee/session read API | session 是一次运行/接班代；epoch/incarnation 用于 fence；`active` 不自动等于业务进展。
| `Employee status dimensions` | `worker_sessions.state`; `worker_observations`; `status-catalog.json` | 部分 | 是 | status/freshness read model | sleeping、waiting、lost_contact、后台工具在途可并存；未知 enum 不得映射成 working/success。
| `Current Task` | `tasks.owner/kind/state/generation/plan`; `Kernel.Snapshot.Tasks` | 部分 | 是（title/summary/dependency/acceptance view） | task detail read API | task board 是查询投影；`candidate` 不是 `completed`，`completed` 也不等同外部交付接受。
| `Message` | `messages.sender/recipient/kind/body/evidence_id/delivery_state/contract_revision_id/task_revision` | 是（schema） | 是（redacted presentation） | message/causality read API | Message != Obligation；`acknowledged` 只表示消息生命周期节点，不等于 obligation fulfilled。
| `Obligation` | `obligations.owner/state/evidence_id/evidence_ref`; peer indexes | 是（schema） | 是（detail links） | obligation read API | obligation 是责任/承诺；读取或 ack 不关闭责任；`fulfilled` 需要证据 binding。
| `ContractRevision` | `contract_revisions.revision/state/digest/endpoint/schema/proposer/accepter` | 是（schema） | 是（display summary） | contract revision read API | accepted/superseded/proposed 要分开；修订版本必须与 Message、Checkpoint、Artifact 关联。
| `Workspace checkpoint` | `worker_checkpoints.session_id/digest/data`; `Checkpoint` Go type | 是（schema/type） | 是（presentation） | checkpoint/evidence read API | progress checkpoint != final acceptance；qualified checkpoint 受 workspace revision、contract/checker policy 约束。
| `Artifact / Delivery` | `artifacts.author/digest/bytes/state/verdict/contract`; `artifact_qualifications`; `integration_candidates`; `review_records` | 部分（窄 snapshot） | 是（delivery checklist） | artifact/delivery read API | `candidate/passed/failed/invalidated` 不能合并；artifact ready/candidate 不能单独宣称交付完成。
| `Handover` | `worker_sessions` epoch/incarnation + `HandoverBundle`; R0.3A handover evidence | 部分 | 是（timeline projection） | handover read API | successor 保留 EmployeeId 但使用新 session/epoch；旧 writer 必须被拒绝；fixture 不篡改历史证据。
| `Tool/model budget` | `worker_sessions.profile`, `tool_call_limit`, `tool_calls_used`; evidence usage blocks | 部分 | 是（reported/estimated/unavailable） | resource/budget read API | 工具调用预算不是 token/美元；sleeping 不等于零成本；未知不能显示为 0。
| `Qualification / evidence` | `worker_checks`, `artifact_qualifications`, `review_records`; `evidence/development/**` | 部分 | 是（safe links/labels） | evidence read API | evidence 是可追溯依据，不把静态/fixture/真实结果混为一类；历史 `INCONCLUSIVE` 不得改写为 PASS。
| `ActivityEvent` | `events.company_seq/kind/payload/observed`; domain receipts/worker state changes | schema 有原始事件 | 是（统一 UI 表示） | activity read API + SSE stream | raw protocol JSON 不直接 dump；事件只描述已接受事实；需要 actor/subject/summary/references 的 presentation DTO。
| `NeedsAttention / Incident` | derived from pending obligations, stale/lost session, failed/inconclusive review, unknown operation state | 没有单独表 | 是（首轮） | incident/decision read API | 不是所有未读消息；前端不另算优先级；等待/休眠不是红色故障。

## Fixture provenance

首轮 fixture 以保存的 `evidence/development/r0.3a-real-frontend-handover-revised-v2/result.json` 的公开锚点构造：

- company `r03a-t2-company-1789221294871371900`、mission `r03a-t2-mission-1789221294871371900`、company sequence `70`；
- backend task `8ac83ed6f11413d8a8d5e9925fe0c1c0` 与 frontend task `3243c57410762f2cd8b666e3ad35c8c9`；
- accepted ContractRevision rev3 `4f57c71061c10ab5a78f632d8db59321`；
- Message/Obligation `7d027b5a8fd2267a68b364e47f4635ca`，其消息生命周期与 obligation 状态分开展示；
- backend checkpoint/artifact 锚点 `b3869930af22ae19ae4353e321b45371` / `5bf77f15ee867b857f116faad0312c4f`；
- frontend initial epoch 3/session `a300088a9d514a9c85e7beff5399ace9`，successor epoch 4/session `ccd5906064dce787c3dc864f6eb8aa7b`；
- frontend checkpoint/artifact `93054ca68a451d75f38dc98c4d490c0b` / `a0191e5ce29a56eb347b421df39fb86e`，workspace revision `5`。

页面显示为 `data_mode=simulated`，并标注“基于已保存的 R0.3A 证据锚点”；它不表示当前数据库仍存在，也不把 evidence path 当成运行时 API。

## Required read API

本轮没有擅自新增 Go endpoint。后续 RealWorkbenchApi 最小需要：

1. `GET /api/workbench/companies/:companyId/overview` — 返回 scoped `CompanyOverviewView` 与 `ViewMeta`。
2. `GET /api/workbench/companies/:companyId/activity?snapshot_cursor=<opaque>&cursor=<opaque>&limit=<bounded>` — 返回 `{items, next_cursor, meta}`；初始 activity read 与 overview 使用同一 snapshot watermark，分页 cursor/limit 与 `r03a-pagination-contract@3` 的 request/response/termination 语义保持一致，但不要把 Product Track 页面当成该 acceptance contract 的实现。
3. `GET /api/workbench/companies/:companyId/stream?cursor=<opaque>` — 单个 scoped SSE stream；初始水位由 snapshot 提供，断流只显示 stale/reconnecting，不清空旧值。

为后续页面预留但本轮不实现：employee detail、task detail、mission detail、artifact/delivery detail、evidence 与 incident/decision read API。所有 endpoint 都必须逐请求鉴权，不把导航可见性当授权。

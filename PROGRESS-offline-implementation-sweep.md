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
- `summary/req40-delivery-completion-ui.md` — Slice 276 owner completion UI/API summary

## 当前进度
已完成需求筛选；REQ-40 Company backlog 只读投影、历史 Artifact backfill 管理路径和 assembling delivery completion UI/API 均已实现并通过目标测试/build。当前未发现可在不猜测 owner policy 的前提下继续推进的明确代码闭环。

## 下一步
Slice 274–276 已提交并推送；保留前端测试文件中三个既有无关失败及 PostgreSQL/runtime backfill 未执行的记录。下一步需要 owner policy 决策后再继续。

## 发现的关键信息
- REQ-36/37/38 与部分 REQ-40 工作需要 owner policy、真实主机、账号或运行时迁移，当前不应猜测。
- Schema 113 已持久化 terminal `delivery_feedback_backlog_events`，但目前只有写入路径，没有 Workbench read projection。
- 新投影限制为当前 Artifact delivery、最多 32 条、只读 `open` 事件；不创建任务、不唤醒 Mission、不改变 terminal 状态。
- 当前 worktree 不需要真实 PostgreSQL/Worker/provider/browser，适合直接在 WSL 继续开发。
# Latest checkpoint: Slice277 REQ-37 read-only JobRun surface

Slice277 adds the fake-only `polis-product-tool-surface@15` with `jobs_status` and `jobs_logs`. The Kernel revalidates Company/Task/WorkerSession scope in a repeatable-read transaction and verifies the persisted log-manifest digest. The qualified real-provider `@4` surface is unchanged; `jobs.start/stop`, process control and BorrowerLease remain policy/native-host gated. Targeted Codex, Provider, Kernel and Control tests pass. A broader package sweep was attempted but remains non-clean because of existing provider identity, MCP permit/fixture and mission-command environment failures; no Worker, process, database runtime, provider, browser, external account or frozen scenario ran. Evidence: `evidence/development/r1-r3-implementation-validation-20261006-slice-277-req37-read-only-jobs/verification.md`.

Review follow-up: JobRun log reads now use the explicit 2 MiB log bound, read-only CAS lookup does not create a missing Company directory, and tests cover large logs, missing/tampered manifests, cross-Company scope and the isolated-surface deny path.
# Latest checkpoint: Slice278 REQ-40 owner policy decisions

Selected and implemented a seven-day `awaiting_feedback` window for ready delivery completion, derived expiry that blocks later writes without implicit acceptance, and a Mission success closeout gate requiring explicit accepted dispositions for all acceptance Artifacts. `ended_not_met`/`cancelled` and historical backfill remain independent of user acceptance. Pre-Schema111 success closeout fails closed. Targeted Kernel policy tests and `build ./cmd/...` passed; no migration, runtime database, Worker/provider, browser, external account or frozen scenario ran. Evidence: `evidence/development/r1-r3-implementation-validation-20261006-slice-278-req40-owner-policy/verification.md`.
# Latest checkpoint: Slice279 REQ-36 fake-only Worker environment ensure

Added product surface @16 with bound `environment_ensure`. Exact revision IDs are checked against the current Mission, and WorkerSession/Task/Mission authorization is held in the same idempotent transaction and replay guard. No host command, package edit, registry/network selection, migration, Worker turn, provider, browser or scenario ran. Evidence: `evidence/development/r1-r3-implementation-validation-20261006-slice-279-req36-worker-environment-ensure/verification.md`.
# Latest checkpoint: Slice280 executor/browser qualification

Authorized local qualification passed the Windows runner test binary and service-browser ingress Go suite. Browser rendering did not qualify because Windows could not reach WSL's randomized loopback ingress and WSL Chromium installation was unavailable. Evidence: `evidence/development/r1-r3-implementation-validation-20261006-slice-280-executor-browser-qualification/verification.md`.
# Latest checkpoint: Slice281 REQ-40 DeliveryManifest invalidation/withdrawal

Added owner/CSRF exact-current invalidation/withdrawal revisions with canonical digest checks, stale/terminal rejection, owner reason persistence and no Artifact/Mission/Task side effect. Targeted Go/TypeScript tests and frontend build passed; one unrelated full validation test remains. Evidence: `evidence/development/r1-r3-implementation-validation-20261006-slice-281-req40-delivery-invalidation/verification.md`.
# Latest checkpoint: Slice282 REQ-36 DependencyChange proposal/approval

Added Schema115 append-only DependencyChange proposal and decision intent. The fixed envelope validates exact semver dependencies and base-policy registry hosts; owner approval does not run npm or generate a lockfile/environment revision. Targeted Go tests/build/diff checks pass; no migration/runtime database, npm, Worker, executor, provider, browser or scenario ran. Evidence: `evidence/development/r1-r3-implementation-validation-20261006-slice-282-req36-dependency-change-proposal/verification.md`.
# Latest checkpoint: Slice283 REQ-37 durable BorrowerLease lifecycle

Added Schema116 append-only BorrowerLease identities/events with same-Mission distinct-Task and exact WorkerSession scope, bounded TTL/idle, release/touch and endpoint/generation revoke paths. No service process, native host, provider, browser or scenario ran. Evidence: `evidence/development/r1-r3-implementation-validation-20261006-slice-283-req37-borrower-leases/verification.md`.
# Latest checkpoint: Slice284 REQ-37 BorrowerLease product surface and recovery hardening

Added fake-only product surface @17 with `jobs_borrow`, `jobs_touch` and `jobs_release`; lease operations remain same-Mission and exact Task+WorkerSession bound, while service start/stop stays outside the surface. Revoke helpers now close pgx row streams before writes, recovery revokes leases before worker reconciliation, owner/generation validity is checked on touch/release, and endpoint/idle bounds are persisted consistently. Targeted Kernel/provider/control/db tests and `build ./cmd/...` pass; no service process, native host, provider, browser or scenario ran. Evidence: `evidence/development/r1-r3-implementation-validation-20261006-slice-284-req37-borrower-lease-tools/verification.md`.
# Latest checkpoint: Slice285 REQ-38 BrowserRun control-plane foundation

Added Schema117 BrowserRun identities/events and fake-only @18 `browser_run`/`browser_results`. Requests are current Task/WorkerSession and same-Mission service-generation bound, canonicalize HTTPS origin metadata, and persist as `blocked/browser_runtime_unqualified`; no browser or network execution is exposed. Targeted Kernel/codex/provider/control/db tests and `build ./cmd/...` pass; no browser, service process, Provider, external account or scenario ran. Evidence: `evidence/development/r1-r3-implementation-validation-20261006-slice-285-req38-browser-run-control-plane/verification.md`.
# Latest checkpoint: Slice286 REQ-38 research/search/fetch unavailable control plane

Added Schema118 ResearchOperation identities/events and fake-only @19 `research_search`/`research_fetch`. Search/HTTPS fetch requests persist explicit `unavailable/research_backend_unavailable` with zero Provider egress and never call web, HTTP, MCP or browser backends. Targeted Kernel/codex/provider/control/db tests and `build ./cmd/...` pass; no external web, Provider, browser, account or scenario ran. Evidence: `evidence/development/r1-r3-implementation-validation-20261006-slice-286-req38-research-unavailable-control-plane/verification.md`.
# Latest checkpoint: Slice287 REQ-40 delivery revision/disposition history projection

Extended the read-only durable delivery projection with bounded Manifest revision and UserDisposition histories, binding every item to current Company/Artifact/Mission/Task, stored digest and known revision identities. Added frontend validation and read-only rendering; direct TypeScript/Vite builds and Workbench Go test pass. No database runtime, Worker/provider, browser, external account or scenario ran. Evidence: `evidence/development/r1-r3-implementation-validation-20261006-slice-287-req40-delivery-history-projection/verification.md`.
# Latest checkpoint: Slice288 REQ-40 delivery revision routes

Added Schema119 append-only delivery revision routes linking live-Mission `changes_requested` feedback to formal MissionChangeRequest, successor Mission creation and successor product Task preparation. No parallel current-Mission Task is created; terminal feedback remains backlog-only. Kernel/db tests, migration hash and `build ./cmd/...` pass; no runtime database, Worker/provider, browser, external account or scenario ran. Evidence: `evidence/development/r1-r3-implementation-validation-20261006-slice-288-req40-delivery-revision-routes/verification.md`.
# Latest checkpoint: Slice289 REQ-40 revision-route read projection

Added optional Schema119 `revisionRoutes` read projection with immutable Manifest/disposition joins, `changes_requested` validation, bounded latest-16 route events and read-only Workbench rendering. Direct TypeScript/Vite and Workbench Go checks pass; no database runtime, Worker/provider, browser, external account or scenario ran. Evidence: `evidence/development/r1-r3-implementation-validation-20261006-slice-289-req40-revision-route-projection/verification.md`.
# Latest checkpoint: Slice290 REQ-14 generic ActionIntent deny foundation

Added Schema120 generic ActionIntent append-only records/events with strict ResourceKey/digest/idempotency and exact Task/WorkerSession scope. Generic requests remain explicitly denied and no generic DispatchPermit/external action is issued. Kernel test and `build ./cmd/...` pass; no external action, Provider, browser, database runtime or scenario ran. Evidence: `evidence/development/r1-r3-implementation-validation-20261006-slice-290-req14-generic-action-intent-deny-foundation/verification.md`.
# Latest checkpoint: Slice291 REQ-02 semantic TaskKind bridge

Added descriptive fixed-team `task_type` to persisted TaskKind mapping. Unknown types remain unmapped and all admission results stay human-gated; no Worker/provider execution was enabled. Spec test and `build ./cmd/...` pass; no runtime database, owner action, Worker/provider, browser, account or scenario ran. Evidence: `evidence/development/r1-r3-implementation-validation-20261007-slice-291-req02-semantic-task-kind-bridge/verification.md`.
# Latest checkpoint: Slice292 REQ-02 per-employee RoleRevision persistence

Added Schema121 immutable per-employee RoleRevision rows written with fixed-team owner confirmation. Rows bind the reviewed matrix digest, role name, semantic task types and descriptive TaskKinds while remaining `unverified`; Worker admission and provider execution stay gated. Spec/db tests and `build ./cmd/...` pass; no runtime database, owner action, Worker/provider, browser, account or scenario ran. Evidence: `evidence/development/r1-r3-implementation-validation-20261007-slice-292-req02-role-revisions/verification.md`.
# Latest checkpoint: Slice293 REQ-02 RoleRevision projection

Added optional Schema121 `EmployeeSummary.roleRevision` projection with deterministic latest-revision selection and old-Schema null compatibility. Workbench test and `build ./cmd/...` pass; no runtime database, owner action, Worker/provider, browser, account or scenario ran. Evidence: `evidence/development/r1-r3-implementation-validation-20261007-slice-293-req02-role-revision-projection/verification.md`.
# Latest checkpoint: Slice294 RoleRevision frontend boundary

Tightened EmployeeSummary RoleRevision validation to null or lowercase SHA-256 and corrected a stale formal MissionChangeRequest fixture. Workbench validation tests pass 50/50; direct TypeScript/Vite builds and diff check pass. No backend/runtime/provider/browser/account/scenario action ran. Evidence: `evidence/development/r1-r3-implementation-validation-20261007-slice-294-role-revision-frontend-boundary/verification.md`.

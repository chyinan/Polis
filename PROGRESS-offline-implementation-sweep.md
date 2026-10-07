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

# Latest checkpoint: Slice295 RoleRevision detail projection

Extended optional EmployeeSummary RoleRevision readback with role name, semantic task types/kinds, owner decision and qualification metadata. Company/Employee scope, malformed detail rejection and old-Schema compatibility are enforced; frontend validation and builds pass. No database runtime, owner action, Worker/provider, browser, account or scenario ran. Evidence: evidence/development/r1-r3-implementation-validation-20261007-slice-295-req02-role-revision-detail-projection/verification.md.
# Latest checkpoint: Slice296 ActionIntent Operations projection

Added optional Schema120 generic ActionIntent denied-audit projection to Workbench Operations. It is Company-scoped, latest-event filtered before the bounded limit, digest-only and old-Schema compatible. Workbench Go test, frontend validation/build and diff check pass; no external action, Provider, browser, database runtime, account or scenario ran. Evidence: evidence/development/r1-r3-implementation-validation-20261007-slice-296-action-intent-operations-projection/verification.md.

# Latest checkpoint: Slice297 offline test-contract alignment

Aligned stale test fixtures with the current bounded mission-command input,
one-shot MCP dispatch-permit lifecycle, Streamable HTTP permit callback and
Codex identity-readiness prerequisite. The full Go package suite and command
build pass; no production behavior, database runtime, Worker/provider,
browser, external account, MCP endpoint or frozen scenario ran. Evidence:
`evidence/development/r1-r3-implementation-validation-20261007-slice-297-offline-test-contracts/verification.md`.

# Latest checkpoint: Slice298 REQ-02 semantic TaskRevision binding

Added a pure content-addressed binding from fixed-team RoleRevision to an
explicit semantic task_type/owner/TaskKind tuple. Mismatch reason codes are
deterministic and exact matches remain unverified/human-gated. Spec tests pass;
no durable revision, Worker/provider execution, database runtime, browser,
external account or scenario ran. Evidence:
`evidence/development/r1-r3-implementation-validation-20261007-slice-298-req02-task-revision-binding/verification.md`.

# Latest checkpoint: Slice299 REQ-02 durable semantic TaskRevision context

Schema122 now persists the exact product compat/Backend semantic binding at
Worker admission when available. The row is append-only, FK-bound to the Task
and RoleRevision, idempotent and read-back verified; it remains
unverified/human-gated and absence of Schema122 skips only the optional write
while the Schema121 admission gate remains required. Full Go tests,
build and diff check pass; no migration/runtime/Worker/provider/browser action
ran. Evidence:
`evidence/development/r1-r3-implementation-validation-20261007-slice-299-req02-task-semantic-revisions/verification.md`.

# Latest checkpoint: Slice300 REQ-02 TaskRevision owner-decision lifecycle

Added the Schema123 append-only owner decision ledger and pure monotonic state
machine for exact TaskRevision bindings. Only proposed→approved/rejected and
approved→revoked are allowed; decisions remain unverified and do not authorize
execution. Targeted DB/spec/control/workbench/kernel tests pass; no migration,
Worker/provider/browser/external account or scenario ran. Evidence:
`evidence/development/r1-r3-implementation-validation-20261007-slice-300-req02-task-revision-decisions/verification.md`.

# Latest checkpoint: Slice301 REQ-02 TaskRevision Workbench projection

TaskSummary now optionally shows the latest durable semantic TaskRevision and
owner decision. Scope, Task owner/kind, digest, qualification and decision
validation fail closed; old schemas remain compatible and the UI is read-only.
Frontend 146/146 tests, build, full Go tests and command build pass; no
migration/runtime/Worker/provider/browser/external account/scenario ran.
Evidence:
`evidence/development/r1-r3-implementation-validation-20261007-slice-301-req02-task-revision-projection/verification.md`.

# Latest checkpoint: Slice302 REQ-38 BrowserRun evidence binding

Succeeded BrowserRun reads now require a canonicalized/content-addressed,
same-scope evidence manifest whose real ready candidate/passed Artifact rows
match the references; malformed or cross-scope
success evidence fails closed. Full Go tests and command build pass; no browser,
network, Worker/provider, external account or scenario ran. Evidence:
`evidence/development/r1-r3-implementation-validation-20261007-slice-302-req38-operation-evidence-binding/verification.md`.

# Latest checkpoint: Slice303 REQ-38 ResearchOperation evidence binding

Succeeded ResearchOperation results now require the canonical evidence
envelope and same-scope real Artifact digest/state checks. The unavailable fake
search/fetch backend remains unchanged; full Go tests and command build pass,
with no retrieval/network/Worker/provider/browser action. Evidence:
`evidence/development/r1-r3-implementation-validation-20261007-slice-303-req38-research-evidence-binding/verification.md`.

# Latest checkpoint: Slice304 local browser requalification

Authorized Windows Chrome/WSL fixture requalification reached the fixed local
Workbench but refused the randomized WSL service ingress, reproducing Slice280.
No browser isolation/rendering pass was claimed; the fixture was cleaned up.
Evidence:
`evidence/development/r1-r3-implementation-validation-20261007-slice-304-local-browser-requalification/verification.md`.

# Latest checkpoint: Slice305 Windows browser fixture qualification

The service ingress fixture was cross-compiled and run on Windows beside
Windows Chrome. Rendering and same-origin fetch passed; effective WFP/profile,
packaged Node/npm and clean-VM qualification remain open. Evidence:
`evidence/development/r1-r3-implementation-validation-20261007-slice-305-windows-browser-fixture-qualification/verification.md`.

# Latest checkpoint: Slice306 Linux runner verifier qualification

The WSL2 Ubuntu-22.04 runner now executes the previously skipped
`POLIS_GO_ROOT` verifier with the existing `bwrap` and workspace Go root. The
focused verifier test, full `internal/runner` package, complete Go package
suite and `build ./cmd/...` pass. This removes the Linux verifier gap only;
Windows WFP/profile, packaged Node/npm, clean-VM,
browser isolation and Provider qualification remain open. Evidence:
`evidence/development/r1-r3-implementation-validation-20261007-slice-306-linux-runner-verifier-qualification/verification.md`.

# Latest checkpoint: Slice307 REQ-38 research source binding

Added Schema124 Mission-scoped source registrations, immutable authorization/
revocation events, exact-origin binding and the owner Workbench route. Fake
research @19 accepts optional `source_id`, but search/fetch remains explicit
unavailable with zero egress. Full Go tests and command build pass; no external
source, Worker, Provider, browser or scenario ran. Evidence:
`evidence/development/r1-r3-implementation-validation-20261007-slice-307-research-source-binding/verification.md`.

# Latest checkpoint: Slice308 BrowserRun runner boundary

Added the source-level Playwright runner protocol, policy core and Artifact-
bound BrowserRun success fence. It uses a fresh profile, minimal environment,
bounded output and explicit same-origin/download/WebSocket controls, but was
not launched and remains default-deny. Evidence:
`evidence/development/r1-r3-implementation-validation-20261007-slice-308-browser-runner-boundary/verification.md`.

# Latest checkpoint: Slice309 controlled research fetch executor

Added the source-level bounded fetch executor and ResearchOperation success
fence. Tests use an in-memory HTTP transport; no external request or product
retrieval ran. Search ranking, BrowserRun wiring and host/provider qualification
remain open. Evidence:
`evidence/development/r1-r3-implementation-validation-20261007-slice-309-research-fetch-executor/verification.md`.

# Latest checkpoint: Slice310 research search result contract

Added the pure same-origin search-result contract: max 20, required source
time, bounded fields, duplicate rejection and stable digest. No search backend
or network ran. Evidence:
`evidence/development/r1-r3-implementation-validation-20261007-slice-310-research-search-contract/verification.md`.

# Latest checkpoint: Slice311 executor product wiring

Wired optional BrowserRun/ResearchOperation executors through EmployeeTools and
the RealProviderWorkerAdapter; nil defaults keep both paths fail-closed and no
runtime adapter ran. Evidence:
`evidence/development/r1-r3-implementation-validation-20261007-slice-311-executor-product-wiring/verification.md`.

# Latest checkpoint: Slice312 concrete operation adapters

Added optional BrowserRun/research-fetch adapters and Schema125
`operation_evidence` Artifact storage. Adapters remain disabled; no browser,
HTTP, Worker, Provider or migration runtime ran. Evidence:
`evidence/development/r1-r3-implementation-validation-20261007-slice-312-concrete-operation-adapters/verification.md`.

Tightened EmployeeSummary RoleRevision validation to null or lowercase SHA-256 and corrected a stale formal MissionChangeRequest fixture. Workbench validation tests pass 50/50; direct TypeScript/Vite builds and diff check pass. No backend/runtime/provider/browser/account/scenario action ran. Evidence: `evidence/development/r1-r3-implementation-validation-20261007-slice-294-role-revision-frontend-boundary/verification.md`.

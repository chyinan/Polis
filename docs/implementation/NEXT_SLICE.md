# Current handoff - R0.15 MCP `tools.call` qualification gate (implemented, bounded, 2026-11-07)

Workspace checkpoint `1d625e31` captured R0.3a–R0.14 plan state. R0.15 planning was added in `7b732f85`/`11f2b740`, then implemented as a **stdio-first, deny-by-default, local/fake-MCP-tested** slice with **no external egress exercised**.

**Implemented (schema 16):** `db/migrations/00016_r15_mcp_tool_call.sql` adds `mcp_tool_call_qualifications` and `mcp_tool_call_receipts` (idempotent by company+employee+handle+idempotency key). Kernel gate requires schema 16 (`internal/kernel/kernel.go`). Core: `internal/kernel/tool_call_policy.go` (request validation, digests, deny-by-default gate, minimal schema-required/additionalProperties checks), `internal/kernel/mcp_stdio_call.go` (bounded stdio `initialize` + `tools/call`, restricted env, temp workdir, timeout, no ambient secrets beyond probe env), `internal/kernel/tool_call_tx.go` (`TXUpsertMCPToolCallQualification`, `TXListMCPToolCallQualifications`, `TXCallMCPTool` with idempotency/request-digest conflict handling and persisted receipts). Control/workbench: `capabilities.tools.call`, `capabilities.tools.call-qualification`, `capabilities.tools.call-qualifications` (`internal/control/*`, `internal/workbench/http.go`).

**Safety boundary:** R0.10 probe, R0.14 descriptor, and binding approval do **not** authorize calls. Calls require explicit approved call qualification for binding+definition+handle+descriptor digest+policy revision; effectful/unknown class requires authorization ref. Transport is stdio-only in this slice (`TRANSPORT_NOT_QUALIFIED` otherwise). No external MCP, OAuth, remote HTTP/Streamable HTTP, QQ, or provider turn was exercised.

**Verification:** `go test ./internal/kernel -run "TestToolCall|TestValidateToolCall"` passed (pure gate/validation/digest). `go build ./...` and targeted vet passed. `TestMCPToolCallGateAndExecution` (`internal/kernel/tool_call_integration_test.go`) **PASS** on a disposable temp PostgreSQL 18/schema 16 cluster (bundled `.tools/pg`, DB `polis_r0_15_toolcall_*`, DSN redacted): deny (`CALL_NOT_QUALIFIED`), explicit qualification, local fake stdio MCP success, idempotent replay, `DESCRIPTOR_DIGEST_MISMATCH`. No external egress. Evidence: `evidence/development/r0.15-tool-call-gate/disposable-pg-integration.md`.

**Next (separately authorized/bounded):** workbench UI surfacing for qualifications/receipts (read-only by default; **no auto-execution**), fuller JSON Schema argument validation, worker task/session/epoch depth if productized, effect-class/policy refinement, and only after explicit qualification/authorization: any non-local or remote MCP transport (Streamable HTTP/OAuth/network sandbox profiles remain unqualified). No external egress/call authorization is implied. Report: `docs/implementation/R0.15_TOOL_CALL_GATE_REPORT.md`. Plan/contracts: `docs/design-plans/R0_15_MCP_TOOL_CALL_QUALIFICATION_GATE.md`, `docs/implementation/R0.15_TOOL_CALL_GATE_DECISIONS.md`, `docs/implementation/R0.15_TOOL_CALL_CONTRACT.md`. Docs draft `docs/implementation/R0.15_SCHEMA16_MCP_TOOL_CALL_DRAFT.sql` is superseded by active migration 00016.

# Current handoff - R0.14 tool descriptor read (2026-10-31)

`tools.describe` returns the frozen description and input schema for one handle on an employee's approved binding. The body must already be in the stored probe receipt. A names-only receipt and a revoked definition return not found, and the read does not start a process. `toolsCalled` stays 0 and `callable` stays false. Schema remains 15. Report: `docs/implementation/R0.14_TOOL_DESCRIPTOR_REPORT.md`.

`TestVisibleToolDescriptorReadsCachedSchemaWithoutCalling` passed. The disposable PostgreSQL test covered the missing body, the frozen body, and the untouched command. The workbench showed `Ping the probe. · {"type":"object"}`. Do not start `scripts.run` or `tools.call` from this result.

# Current handoff - R0.13 tool summary list (2026-10-31)

`tools.list` returns cached tool names for MCP definitions on one employee's approved binding. A name appears only when the stored probe receipt passed and recorded `toolsCalled=0`. The response sets `callable=false` and does not return the command. Unverified and revoked definitions stay empty, and the list does not start a process. Schema remains 15. Report: `docs/implementation/R0.13_TOOL_SUMMARY_REPORT.md`.

`TestVisibleToolPageReadsCachedNamesWithoutCalling` passed. The disposable PostgreSQL test covered the unverified, cached, and revoked cases. The workbench showed `polis_probe_ping` as `memory · 未调用`. Do not start `scripts.run` or `tools.call` from this result.

# Current handoff - R0.12 skill summary list (2026-10-31)

`skills.list` returns the approved skills on one employee's approved binding. Each item is a summary: purpose, version, source reference, content digest, and `scriptsExecuted=false`. Bodies and reference bytes stay out of the page. A draft binding or a revoked revision yields an empty page. Pages are at most 32 items. Schema remains 15. Report: `docs/implementation/R0.12_SKILL_SUMMARY_REPORT.md`.

`TestVisibleSkillPageKeepsApprovedSummariesOnly` passed. The disposable PostgreSQL load test covered the approved, draft, and revoked lists. The workbench showed `colors · 1 · 未执行`. Do not start `scripts.run` or an MCP tool call from this result.

# Current handoff - R0.11 skill reference read (2026-10-31)

An already loaded Skill revision can be re-read by one snapshotted `references/*.md` name. The text and digest come from the import snapshot, `scriptsExecuted` stays false, and a `scripts/` path is rejected. A load row is required; a revoked revision can still be re-read from that row and cannot be newly loaded. Schema remains 15. Report: `docs/implementation/R0.11_SKILL_REFERENCE_REPORT.md`.

`TestSkillReferenceViewReadsFrozenTextWithoutExecution` passed. `TestSkillLoadPersistsApprovedBodyWithoutExecution` passed on a disposable `polis_r0_` database, and the workbench button displayed the frozen reference text. Do not start `scripts.run` or an MCP tool call from this result.

# Current handoff - R0.10 skill load (2026-10-31)

`skills.load` records that an active task received one approved Skill revision. The employee must own the task, the task must be `ready` or `working`, and an approved capability binding must name that revision. The response returns the stored body, the content digest, and reference names captured at import. `scriptsExecuted` stays false and the load grants no tool permission. Repeating the same task, employee, and revision returns the original load row. A passed MCP probe still does not authorize tool calls.

Schema 15 adds `skill_loads`. `TestSkillLoadPersistsApprovedBodyWithoutExecution` passed on a disposable `polis_r0_` database: the first load stored the body and digest, the repeat load returned that same row, and candidate, draft-binding, completed-task, and revoked revisions were denied. The workbench button was not clicked. Existing databases must migrate to schema 15 before `polis` will start. Do not start `scripts.run` or an MCP tool call from this result.

# Current handoff - R0.10 local capability probe (2026-10-31)

R0.10 can import one skill package from `POLIS_SKILL_LIBRARY_ROOT/skills/<package>/SKILL.md` and can probe a registered stdio MCP with initialize and tools/list. Schema 14 stores the probe receipt on `mcp_server_definitions.qualification`. A passed probe moves the definition to `candidate`. It does not approve calls, invoke tools, execute skill scripts, send QQ, or start a provider turn.

The local experiment parsed `brand-guidelines` from the operator skill library and probed `@modelcontextprotocol/server-memory@2.0.1`. Evidence: `evidence/development/r0.10-local-capability-probe/result.json`. The probe process was not network-sandboxed. Existing databases must migrate to schema 14 before `polis` will start.

Next product work is read-only `skills.load` of an approved revision into a task context. Do not treat a passed probe as permission to call MCP tools.

# Current handoff - R0.9 capability binding (2026-10-31)

R0.9 local manifest import and employee capability binding is implemented offline. Schema 13 adds `employee_capability_bindings`. A pasted `SKILL.md`, or `skills/<id>/SKILL.md` and `mcp/<id>.yaml` under `POLIS_SKILL_LIBRARY_ROOT`, can be registered after a strict metadata parse. Symlinks, secret frontmatter keys, and files outside that layout are rejected. Approval changes catalog status only. A binding may reference approved Skill revisions and non-revoked MCP definitions; its qualification stays `execution=unverified` and `connection=unprovisioned`.

No Skill script, MCP process, MCP endpoint, OAuth flow, QQ send, or provider turn is executed by this slice. PostgreSQL integration and browser smoke were not run. Existing databases must migrate to schema 13 before `polis` will start.

Next product work still needs a separate slice: read-only `skills.load` of an approved revision into a task context, or a separately authorized MCP connection qualification. Do not treat binding approval as permission to connect or execute.

# Current handoff - R0.6 DOGFOOD MVP FOUNDATION closeout (2026-09-21)

R0.6 is **COMPLETE_BY_COMPOSITE_EVIDENCE** for the local/deterministic dogfood baseline. A–I supporting surfaces passed targeted and disposable PostgreSQL validation; the existing frontend and RealWorkbenchApi browser smoke passed with API failures=0. Final report: `docs/implementation/R0.6_DOGFOOD_MVP_REPORT.md`; gap matrix: `docs/implementation/R0.6_DOGFOOD_GAP_MATRIX.md`; stage evidence is under `evidence/development/r0.6-*`.

External QQ/webhook notification delivery is intentionally **NOT_RUN / UNQUALIFIED** because no external account send is authorized. Real-provider business execution, LIVE_6, new canary, retry, High, successor and automatic next milestone are not authorized. Local deterministic notification intent/route/delivery readback is the only notification evidence.

Stop after R0.6 closeout. Any future milestone requires a new explicit scope and authorization.

# Historical handoff - R0.5B composite evidence closeout (2026-09-21)

R0.5B is **COMPLETE_BY_COMPOSITE_EVIDENCE**. LIVE_5 raw remains `FAILED` and immutable; B17 readback/projection hardening independently passed. The composite demonstrates the real-provider product path, deterministic delivery, checkpoint/Artifact readback and browser visibility. `provider_egress=0` for B17, @4 is current, B14 is `QUALIFIED_REUSABLE`, and provider identity drift is `NONE`.

There is no next slice. Do not run LIVE_6, a canary, a retry, another provider turn, or the next milestone. Closeout/adjudication: `evidence/development/r0.5b-composite-evidence-closeout/closeout.md`. B17 evidence remains under `evidence/development/r0.5b17-checkpoint-projection-hardening/`.

# Current handoff - R0.5B16 reservation-to-initialize bridge hardening (2026-09-21)

R0.5B16 is **PASSED offline/local**. The frozen LIVE_4 boundary is precisely recorded at `evidence/development/r0.5b16-reservation-to-initialize-bridge-hardening/preflight-v6/live4-boundary.json`: authorization, reservation, WorkerSession, process/PID passed; initialize construction/write/ACK were not determinable/observed. Root cause was a business `CodexRuntime.Start` session incorrectly wrapped as diagnostic-only, which rejected transport policy before initialize write. Business sessions now use the normal `codexSession`; diagnostic wrapping is limited to diagnostic-only runtimes.

The reservation-bearing local production-path preflight passed 3/3 with fresh Company/Mission/compat Task/TaskValidationBinding/WorkerSession/runtime cycles. Each reached initialize request/write/ACK, thread/start, exact @4 registration and clean stop; reservation created/closed=3/3, active=0, provider_egress=0, turn/start=0, orphan=0. Lifecycle evidence includes controller ownership phases and protocol initialize prepare/payload/write/ACK phases. B14 surface/runtime/launch/execution identities are unchanged and remain `QUALIFIED_REUSABLE`; new live product smoke remains eligibility-only. Detailed summary: `summary/r0-5b16-reservation-to-initialize-bridge.md`.

# Historical handoff - R0.5B14 product tool surface v4 live qualification (2026-09-20)

R0.5B14 is **PASSED**. The exact current surface is `polis-product-tool-surface@4` with 7 tools, manifest `60192ba130e504ad380fd524ec02ab536a8724723098f0d3fb91ecc0f3983ea4`, schema `2206` bytes / `5ee61b10d249a11614d176fda5c3df86f26e0b650c1074b705dc48b4405c35cc`, runtime `0.154.0-alpha.6.2`, launch envelope `dfa3a3d252b2b781e47f932ff350e5eb99f0d6ac0112c1153531248dee0df24d`, and exact-surface execution fingerprint `e736d8298f0920c6dd81796006b2486881647f0fdbc66bb111bbd2387a1e99ff`.

The single B14 diagnostic authorization `r0.5b14-live-canary-1` used `gpt-5.6-luna/medium`, consumed exactly one reservation and one provider egress, registered all 7 tools, returned `POLIS_PRODUCT_SURFACE_V4_CANARY_OK`, used zero tools/reconnects, completed and stopped cleanly, and produced zero business side effects. The new current provider-v4 L2 fingerprint is `59a4ac003fb0f49360382a707f7057edc03e0574ba328d0cf3a59eeab2726781`; `product_provider_surface_status=QUALIFIED` and `eligible_for_live_3=YES`.

Evidence/report: `evidence/development/r0.5b14-product-tool-surface-v4-live-qualification-attempt-3/qualification-report.md`. Post-canary freshness passed; duplicate reservation was denied. B10/B12/B13 evidence remains immutable. Do not run LIVE_3 automatically; it requires separate authorization.

# Historical handoff - R0.5B13 provider process launch reproducibility hardening (2026-09-20)

R0.5B13 is **PASSED offline/local**. Frozen B12's observed category is `PROCESS_CREATE_FAILED_NO_PID`; its dominant root cause is `REPORTING_CLASSIFICATION_DEFECT` because it recorded no PID, no process-start timestamp, no protocol messages, no OS error code and no safe error text, so the exact OS cause cannot be narrowed without rewriting immutable evidence. B11/B12 launch composition is now machine-diffed, and production launch uses one canonical process spec with structured phase/reason/OS-code/PID reporting.

The canonical launch envelope is `r03a-native-launch-envelope@2`, current launch envelope fingerprint `dfa3a3d252b2b781e47f932ff350e5eb99f0d6ac0112c1153531248dee0df24d`, and current @4 exact-surface execution fingerprint `e736d8298f0920c6dd81796006b2486881647f0fdbc66bb111bbd2387a1e99ff`. Runtime remains `0.154.0-alpha.6.2`, binary SHA `081e4de4be8e38fac6ed4d95e3b1a0b9f6d31c090ddc36e1696b349fe406f575`, helper SHA `fdf360c3a02adce2a29272357d61681bdddc2ab9828a5fa615c4528e4384a23e`. The @4 surface is unchanged: 7 tools, manifest `60192ba130e504ad380fd524ec02ab536a8724723098f0d3fb91ecc0f3983ea4`, schema `2206` bytes / `5ee61b10d249a11614d176fda5c3df86f26e0b650c1074b705dc48b4405c35cc`.

Three independent Windows local production-path cycles passed process start, initialize, exact 7-tool thread/start registration, clean stop and orphan-process=0; an additional `RealProviderWorkerAdapter` zero-turn handshake passed. Provider reservation=0 and provider egress=0. Evidence/report: `evidence/development/r0.5b13-provider-process-launch-reproducibility-hardening-v3/`; B12 evidence remains immutable. `eligible_for_new_v4_live_qualification=YES` is eligibility only; no live canary was run.

# Historical handoff - R0.5B11 provider runtime version binding hardening (2026-09-20)

R0.5B11 provider-runtime version binding hardening is **PASSED offline/local**. The B10 root cause was a provider-runtime binding defect: an environment-supplied expected `0.155.0-alpha.9.2` was not coupled to the staged executable actually launched. Production `polis serve` now resolves one verified runtime manifest and binds the executable/helper paths, exact SHA256 values and normalized semantic version from that artifact; conflicting version/hash/path values and PATH-selected executables fail closed before egress.

The authoritative current runtime is `0.154.0-alpha.6.2`, binary SHA `081e4de4be8e38fac6ed4d95e3b1a0b9f6d31c090ddc36e1696b349fe406f575`, helper SHA `fdf360c3a02adce2a29272357d61681bdddc2ab9828a5fa615c4528e4384a23e`, launch mode `windows_native_direct`, launch envelope `676052c13ae0380250c4fcb9dbdb3cd8794167e214e6b3766e6e38b4d68bdc51`. The local Windows preflight passed process start, initialize, thread/start and clean stop with no turn and provider egress `0`. New @4 exact-surface execution fingerprint: `1f18dd6677e3543992af97fe79fb88604980b418622e436100d0ddf76cd8314c`.

The @4 product surface is unchanged: 7 tools, manifest `60192ba130e504ad380fd524ec02ab536a8724723098f0d3fb91ecc0f3983ea4`, schema bytes `2206`, schema digest `5ee61b10d249a11614d176fda5c3df86f26e0b650c1074b705dc48b4405c35cc`. Offline regressions cover the B10 `0.155`/`0.154` mismatch, binary/helper hash mismatch, helper mismatch, missing executable and wrong-PATH cases. `provider_surface_changed=false`, `runtime_binding_changed=true`, `launch_envelope_changed=false`, `exact_surface_execution_fingerprint_changed=true`; reservation=0, provider egress=0, Medium=0, High=0. `eligible_for_new_v4_live_qualification=YES`; live qualification status=`STILL_REQUIRED`. Evidence: `evidence/development/r0.5b11-provider-runtime-version-binding-hardening/qualification-report.md`. B10 evidence is immutable.

R0.5B10 exact @4 surface recomputation and offline L2 are **PASSED**. The single authorized diagnostic canary is terminal **INCONCLUSIVE**: reservation `1`, provider egress `0`, process start succeeded, initialize failed closed on expected `0.155.0-alpha.9.2` versus actual provider userAgent `0.154.0-alpha.6.2`. No thread/start, turn, tool call, reconnect, business object or business mutation occurred. Clean stop and duplicate reservation denial passed. No retry is allowed. The @4 provider L2 fingerprint was not created; @4 remains current but `NOT_QUALIFIED` for future product samples until a separately authorized run with a compatible current runtime.

Evidence: `evidence/development/r0.5b10-product-tool-surface-v4-live-qualification/`; final report: `qualification-report.md`; exact @4 execution fingerprint: `d931a5faf01f72f23ad91fd16f812a5e2b240b967f57b7fe9ab1b718db76bf61`. The B10 runner root defect found after the terminal attempt was corrected in source, but no provider request was repeated.

R0.5B9 is **PASSED offline**. Product delivery now has the explicit `polis_task_submit({})` decision. The control plane derives the current Task/session/TaskValidationBinding/workspace/check receipt, persists a deterministic qualified checkpoint, stages/publishes the validated Artifact, records TaskValidationArtifactQualification and moves the Task to `candidate` in one final database transaction. Durable staging/CAS recovery and duplicate replay are covered; progress checkpoints remain independent. Structured public delivery reasons cover validation, stale workspace, fencing, Task state and staging/publication failures.

The current product surface is `polis-product-tool-surface@4`, 7 tools, manifest `60192ba130e504ad380fd524ec02ab536a8724723098f0d3fb91ecc0f3983ea4`, schema bytes `2206`, schema digest `5ee61b10d249a11614d176fda5c3df86f26e0b650c1074b705dc48b4405c35cc`. @3 is historical/stale. The @4 live L2 attempt is terminal `INCONCLUSIVE`; its evidence is recorded above. Frozen LIVE_2 remains raw/adjudicated FAILED and its evidence is unchanged. The frozen LIVE_2 workspace/binding counterfactual delivery passed offline.

R0.5B9 evidence: `evidence/development/r0.5b9-deterministic-delivery-transaction/`; summary: `summary/r0-5b9-deterministic-delivery-transaction.md`; qualification/report are in that directory. Full Go/race/vet, Linux/Windows build, frontend, disposable PostgreSQL/CAS, control/Workbench and offline browser smoke gates passed.

## Previous handoff - R0.5B7 checkpoint schema and error-feedback hardening (2026-09-20)

R0.5B7 is PASSED offline. The product checkpoint contract now accepts rejected: [] as the sole no-rejection form, typed receipt references, and field-level public errors. Validation, TaskValidationBinding, writer fencing, publication ordering and Artifact policy remain strict. LIVE_2 raw and forensic evidence remain frozen FAILED; no provider traffic occurred. The current surface is polis-product-tool-surface@3 with no live qualification. See evidence/development/r0.5b7-checkpoint-schema-error-feedback-hardening/.

## Previous handoff - R0.5B LIVE_2 (2026-09-17)

`r0_5b_real_provider_product_smoke_live_2 = FAILED`. One fresh Mission was created and started through the existing Workbench. Start produced the qualified two-row Task set (`bootstrap_plan` / `emp-planning` / `completed`, `compat` / `emp-backend`), exactly one provider-executable Task, one immutable TaskValidationBinding and one provider-bearing WorkerSession. One `gpt-5.6-luna/medium` authorization, reservation, provider egress and turn occurred on the unchanged `polis-product-tool-surface@2` surface.

The provider turn completed normally, the public workspace check passed, and the worker stopped, but no qualified checkpoint or Artifact was submitted. This is a business-task failure, not a runtime/transport uncertainty. No retry, successor, High turn, second Mission or duplicate reservation was made. After preserving terminal evidence, Workbench Stop/Cancel closed the Mission and cancelled the incomplete compat Task. Final Mission state is `cancelled`; no live WorkerSession/process, credential snapshot or service listener remains.

Post-attempt targeted/full Go verification, race, vet, Linux/Windows builds, frontend tests/typecheck/lint/build, database/read-model integrity and script syntax checks passed. Full result and evidence are under `evidence/development/r0.5b-real-provider-product-smoke/live-2/run/`; summary: `PROGRESS-r0-5b-real-provider-product-smoke-live-2.md`. Do not retry LIVE_2 or automatically expand to another task. Preserve R0.5A–B5 and LIVE_1 evidence; R0.5A–B5 semantics and B4 qualification remain unchanged.

## Previous handoff — R0.5B5 (historical)

`r0_5b5_product_task_cardinality_hardening = PASSED`. The LIVE_1 `SMOKE_TASK_CARDINALITY_ASSUMPTION_MISMATCH` remains frozen as `INCONCLUSIVE` before Start with zero provider authorization, reservation, egress, turn, and WorkerSession. The domain audit confirms Start's two rows have distinct roles: `bootstrap_plan` (`emp-planning`) is planning/control, while `compat` (`emp-backend`) is product employee work. The existing persisted `tasks.kind` is represented by `core.TaskKind`; the product provider-executable predicate selects only `compat` assigned to `emp-backend`.

Offline real product Start proof: Mission `active`; two Task rows (`bootstrap_plan=completed`, `compat=candidate`); exactly one provider-executable Task, one WorkerSession bound to it, one immutable TaskValidationBinding, and one passing product check. `FakeRuntime` observed one reserve-interface call, one start, and one logical turn; actual provider reservation/egress, live Medium, and live High were zero. Duplicate Start and failed-start replay did not repeat the fake reserve-interface call; malformed fan-out failed before worker/provider selection. Workbench continues to display both Task kinds; `workbench_change = NOT_REQUIRED`.

At B5 completion, `eligible_for_r0_5b_live_2 = YES` was eligibility only and required separate explicit authorization. The exact `polis-product-tool-surface@2` surface remains unchanged and B4 remains `QUALIFIED_REUSABLE`. B5 evidence: `evidence/development/r0.5b5-product-task-role-and-smoke-cardinality-hardening/`.

## Previous handoff — R0.5B4 (historical)

`r0_5b4_product_tool_surface_v2_live_qualification = PASSED`. Current surface `polis-product-tool-surface@2` has 7 tools, manifest `2b403fc0f3c9a1513473828e66203becb7ac5202f298f917034fd95929a9f3b9`, schema 1470 bytes / `8f2e1ee8ba8c8d456af9ce9839f62a854bfe7c9f5847613657ccd7f77a9da04f`. Product exact-surface execution fingerprint: `fecc17cac69231575c13d3e2d767e31560e60ebe8dc2fb20921b925ac8d95f08`. Product provider L2 fingerprint: `4c206409f827211cb9f7b18d37da5d096efb7f735c90eaed15e4692128d6a532`. The one authorized `gpt-5.6-luna/medium` diagnostic had one reservation and egress, the exact sentinel, zero tool calls/reconnects/business effects, a completed turn and clean stop, fresh post-canary surface, and an actual denied second reservation. Evidence: `evidence/development/r0.5b4-product-tool-surface-v2-live-qualification/qualification-report.md` and the complete B4 evidence directory.

At B4 completion, the next business smoke remained separately authorized. No Company/Mission/Task/WorkerSession, High, retry, successor, or multi-agent E2E was started in that B4 canary. B2 product surface is `STALE`; R0.3A surfaces are `HISTORICAL / NOT_REUSABLE`. Preserve R0.5A/B1/B2/B3 evidence. The R0.3A notes below are retained historical records.

# Historical pre-V3 planning (superseded)

The following older planning notes are retained for traceability; the current next slice is the new Backend sample below.

# Next slice: separately authorize a new V3 Backend sample

The new R0.3A pagination V3 Backend-only business subject is sealed at `evidence/development/r0.3a-pagination-v3-real-backend-v2/` with `backend_real_execution = FAILED`, one Medium, one provider egress, 48/48 tool calls, and terminal transport reconciliation. The model did not produce a qualified candidate: public runtime checks repeatedly rejected the candidate's function binding, and only a progress checkpoint was persisted. No artifact, authoritative snapshot, CAS recovery package or Frontend phase exists for this subject.

Do not retry this allowance, increase the 48-call limit, start Frontend, start High, or modify historical H1, continuation-7, V3 preflight, or L1/L2 evidence. Public binding hardening is now locally qualified, with provider registration unchanged and no live canary repeated. A future new business sample requires separate authorization, a new independent evidence/run path, the same formal ProblemKey, and simultaneous binding to `r03a-pagination-contract@3`, `r03a-backend-binding@1`, and `r03a-pagination-behavior@1`; it must not synthesize a starting state from this failed disposable database.

Current implementation facts:

- `r03a-pagination-contract@3` and `r03a-pagination-behavior@1` remain the public acceptance factors.
- `r03a-backend-binding@1` is now the independently qualified public source-level binding contract.
- Current Windows L1 and Backend exact-11-tool L2 remain qualified for a future independently authorized run.
- The Backend-only allowance was exact: `gpt-5.6-luna/medium`, `medium_limit=1`, `high_limit=0`, `concurrency=1`, `tool_call_limit=48`.
- PostgreSQL source database and process were cleaned and stopped; no Frontend, successor or Reviewer was started.

## Superseded historical notes

T20 Windows-native L1 is `PASSED` and qualified. T21's offline V7 exact-11-tool preflight passed, but its sole live runner invocation stopped before process start because PowerShell nested the parsed JSON array; T21 is `NOT_STARTED / preflight_failed`, with Medium=0 and provider egress=0. The current allowance is sealed by the no-retry rule; no L2, Backend or business execution starts automatically.

T21A repaired and locally qualified the Windows registry materialization boundary. The corrected runner's provider-before `-PreflightOnly` run expanded the formal Go registry into 11 exact records, passed schema/digest/order/binding validation, completed process/initialize/thread/start and confirmed process stop with zero provider egress. The next action is a new explicit T21B allowance for one Windows-native exact-11-tool Medium live canary; do not start it automatically.

T21B then used a fresh Windows-native exact-11-tool allowance and completed the live turn with 11 registered tools, first output at 7421 ms, zero reconnects, native usage, `turn/completed`, zero attempted tools and zero business side effects. It is `PASSED`, Windows-native L2 exact-11 is qualified, and real Backend execution is eligible under a separate authorization. The output sentinel did not match the requested literal and remains a separate instruction-following finding; no additional transport forensics are required unless a future authorization explicitly changes scope.

Local authorization binding hardening is complete. Before any real Backend worker starts, the allowance must exactly bind execution fingerprint, employee ID, ProblemKey/purpose, model/profile/effort and limits; empty/wrong/stale/drifted bindings are denied. T21B's empty allowance fingerprint remains unchanged historical evidence. The next action is a separately authorized real `emp-backend` execution; no T21C/T21D or additional transport/tool canary is planned.

R0.3A deterministic/FakeWorker/PostgreSQL qualification is complete and recorded in `evidence/development/r0.3a/qualification.md`. T2.1/T4/T5 local qualifications are `PASSED`; T7/T8 establish the current L1 gate and confirmed path divergences; T9 removed only Polis child HTTP(S) proxy injection and still reached the same pre-first-output reconnect failure. The first real run, T2/T3 Backend-only requalifications, T6 and T9 zero-tool canaries remain `INCONCLUSIVE`. No Frontend, successor, Reviewer or High was started.

No continuation or retry is authorized for the completed T3 boundary. Preserve `evidence/development/r0.3a-real/luna-1/`, `evidence/development/r0.3a-t2/luna-1/`, `evidence/development/r0.3a-t3/luna-1/`, and all R0.1/R0.2/H2 evidence; all three provider-unknown outcomes must remain `INCONCLUSIVE`. T2.1 only qualifies local transport policy and does not reopen any allowance.

The T9 allowance is consumed and sealed at one Medium turn; do not retry or reset it. Its no-proxy combination is `unqualified_for_business_execution` at L1. R0.3A-T10 then passed local native compatibility for the independent Codex/app-server 0.153.4 Linux binary. T12 established the current-auth 0.151.0 baseline and T13 reproduced the same Case B failure under current auth with 0.153.4.

T14A passed the offline effective-config/capability/manifest/launcher consistency qualification, including separate host/guest HOME binding. T14B's first live attempt then stopped before Medium because the old launch artifact used `/home/codex` as the host bind source; it remains `NOT_STARTED / preflight_failed` and was not retried. T14C then used the corrected binding and exact T13 auth identity/revision, but native-default still reproduced two pre-first-output reconnects and the first-output deadline; it is `INCONCLUSIVE` and remains unqualified for business execution. No L2, T15, Backend or business execution starts automatically.

T13 passed its version-only factor preflight but was `INCONCLUSIVE`: 0.153.4 also produced two pre-first-output reconnects and the first-output deadline. T14's first attempt was `NOT_STARTED / preflight_failed` before process start because its manifest retained the wrong policy-derived capability digest; the attempt consumed zero Medium and was not retried. T14C corrected the manifest and launch namespace binding, consumed its single authorized Medium, and was still `INCONCLUSIVE`; native-default is not sufficient for recovery of the current combination. T16 then qualified the actual bwrap network path as shared with WSL and showed host/guest localhost reachability, so the bwrap network hypothesis is not supported. T17 confirmed a separate A/B CODEX_HOME/config divergence and left `same_native_default=UNPROVEN`; T18 found no safe single selected config key and remained `NOT_STARTED` with zero allowance. T19 then passed the offline Windows-native vs WSL/Linux envelope qualification; the future Windows-native minimal live canary is eligible only under a new explicit allowance and remains unstarted. No L1, L2, Backend or business execution starts automatically.
# Current handoff - R0.7 DESKTOP DOGFOOD SHELL closeout (2026-09-22)

R0.7 desktop shell is **PASSED_WITH_EXPLICIT_UI_LIMITATION**. Use `docs/implementation/R0.7_DESKTOP_REPORT.md` for architecture, package and verification details. The packaged process smoke passed bundled PostgreSQL/Go startup and child cleanup; GUI visual E2E is not verified because the CUA helper failed inventory twice. Do not infer visual first-run/tray/Company evidence from process smoke.

External provider business egress, QQ/webhook sends, auto-update, startup-with-Windows, code signing and multi-Agent benchmark work remain out of scope.
# Current handoff - R0.8 CAPABILITY CATALOG FOUNDATION closeout (2026-09-22)

R0.8 first vertical is **PASSED_OFFLINE_AND_LOCAL**. Skill candidate and MCP definition metadata can be registered and read through the Company-scoped catalog. No scripts, dependencies, MCP processes/endpoints, OAuth or QQ sends are executed. Next capability work requires a separately bounded slice for controlled local manifest import and capability binding/qualification.

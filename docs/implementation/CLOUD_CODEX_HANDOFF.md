# Polis cloud Codex handoff

Updated: 2026-10-03

## Current continuation pointer

The active checkout has advanced beyond the historical Slice 102–104 notes below. The current implementation is Slice 139 / Schema 94; read `docs/implementation/NEXT_SLICE.md` for the latest completed slice and next step, and `docs/implementation/R1_R3_IMPLEMENTATION_COVERAGE.md` for the remaining approved scope. Slices 121–128 add immutable ProblemKey lineage, shared ProblemKey budgets, owner-authorized allocations, closing reserves, immutable rejection records, incomplete closeout and scoped retry visibility. Slices 129–131 establish Mission protocol-tool-call cap and reserve enforcement; Slice 133 adds the Company cap and Slice 134 its closing reserve. Slices 135–137 bind Codex auth-principal observations, and Slice 138 captures Codex's selected account locator for general WorkerSessions. Slice 139 records billing-mode and installation-owner prerequisites. ProviderAccount financial controls remain unimplemented. Evidence: `evidence/development/r1-r3-implementation-validation-20261003-slice-139-provider-account-billing-scope-audit/verification.md`.

## Latest completed slice (139, REQ-16 ProviderAccount billing-scope audit; no migration)

Official sources distinguish ChatGPT shared credits/allowances, Enterprise token-based USD billing and API Platform organization/project billing. Enterprise Codex Analytics requires a workspace-scoped Admin key. No such key is configured here; live workspace usage/cost validation remains external. Kernel has only Company scope, so shared ProviderAccount mutations need an installation-owner boundary before implementation. See Slice 139 evidence.

## Latest completed slice (138, REQ-16 Codex account locator snapshot; Schema 94)

For ChatGPT auth files, Codex `tokens.account_id` (or the supported ID-token account claim) is hashed and bound immutably to each WorkerSession before execution; unavailable/unsupported cases are explicit. The runtime rejects a changed available account locator after binding. This is the selected Codex account routing scope, not proven billing coverage or a financial budget key. Go build, Schema 1–94 hashes and diff check pass; tests, PostgreSQL migration/runtime and provider execution were not run. See Slice 138 evidence.

## Latest completed slice (137, REQ-16 general Codex auth-principal observation; no migration)

General Codex business runtimes now derive the auth-principal fingerprint when the mounted auth file contains reconstructable issuer/subject claims; otherwise identity remains unavailable. After an identity snapshot is pinned to a WorkerSession, the runtime rechecks an available fingerprint before reservation/start and stops if it changed. This does not prove ProviderAccount billing identity or measure spend. Go build, Schema 1–93 hashes and diff check pass; tests, PostgreSQL runtime and provider execution were not run. See Slice 137 evidence.

## Previous completed slice (136, REQ-16 provider auth identity snapshot; Schema 93)

Schema 93 stores an immutable provider auth-identity snapshot per WorkerSession before execution. The optional versioned runtime capability reports available/unavailable/unsupported; only the existing LIVE_2 Codex path reports a fingerprint. Capture or persistence errors finalize the unstarted WorkerSession. The fingerprint is an auth-principal hint rather than a ProviderAccount ID or financial accounting unit. Go provider/kernel/control/workbench/command build, migration hash checks through 93 and `git diff --check` pass. Tests, PostgreSQL migration/runtime and provider execution were not run. ProviderAccount identity semantics outside LIVE_2 and its request-liability lifecycle remain open. See Slice 136 evidence.

## Previous completed slice (135, REQ-16 ProviderAccount identity/liability binding audit; no migration)

The general provider runtime and execution authorization carry no ProviderAccount identity. The auth identity snapshot is limited to the Codex LIVE_2 credential path and is not exposed to ordinary Worker admission. Durable usage is session-scoped; no account-level request reservation, settlement or unknown-liability ledger exists. Financial ProviderAccount caps remain deferred until those foundations are implemented. No code/schema/build/test/database/provider execution was changed or run; `git diff --check` passes. See Slice 135 evidence.

## Previous completed slice (134, REQ-16 Company closing reserve; Schema 92)

Schema 92 adds append-only Company reserve revisions inside its total protocol-tool-call cap. Only Kernel-created `review` and `peer_review` Tasks can use protected calls; ordinary work preserves remaining reserve during admission, charging, Handover and provider turn clamping. Closing calls burn reserve and Company total atomically. Pending Companies may configure reserve first, but initial cap configuration must cover existing usage plus remaining reserve. Immutable Company denial snapshots include reserve state, and Settings exposes confirmed reasoned updates. Go package/command build, frontend production build, hashes through 92 and `git diff --check` pass; Vite reports the existing 810.97 kB minified bundle advisory. Tests, PostgreSQL migration/runtime, provider execution and live cost measurement were not run. This measures admitted protocol tool calls only. See Slice 134 evidence.

## Previous completed slice (133, REQ-16 Company protocol tool-call cap; Schema 91)

Schema 91 backfills Company usage from durable Task counters and leaves the cap pending until explicit local-owner configuration. Append-only allocation history permits configuration/increases; immutable Company admission/call rejection evidence binds cap, usage and revision. Worker admission checks Company before Mission and accepted protocol tool calls charge Company with Session, Task, ProblemKey and Mission in one transaction. Handover and provider turn clamping use the minimum remaining calls. Settings exposes a no-store projection and confirmed reasoned change form. All Companies need explicit configuration after migration before new admissions/calls proceed. This does not measure model requests, hidden retries, tokens, provider egress or USD. Go package/command build, frontend production build, migration hashes through 91 and diff check pass; the existing Vite >500 kB advisory remains. Tests, PostgreSQL migration/runtime and provider execution were not run. ProviderAccount identity/liability, hidden retries, token/money accounting and FT-42–45 qualification remain open. See Slice 133 evidence.

## Previous completed slice (132, REQ-16 Company/Provider budget identity audit; no migration)

Company ID is stable; `txWrite` locks Company before budget-related row locks; admitted tool calls use idempotency keys derived from session and provider call ID; Task counters support conservative Company usage backfill. This enables an explicitly labeled Company protocol-tool-call cap, not a model/token/USD cap. General WorkerSessions do not bind a ProviderAccount ID, and terminal token observations are not settled provider-account spend. Do not enforce a strict financial ProviderAccount cap until identity and liability accounting exist. See the Slice 132 evidence.

## Previous completed slice (131, REQ-16 Mission closing reserve; Schema 90)

Schema 90 adds append-only Mission reserve revisions inside the total cap. Only Kernel-created `review` and `peer_review` Tasks can consume protected calls. Ordinary work preserves reserve through admission, accepted-call accounting, Handover and provider turn clamping; closing calls consume it while still counting toward Mission total. Pending Missions may set reserve, but the first finite cap must cover usage and unspent reserve. Immutable rejection snapshots include reserve revision/state, and Settings exposes a confirmed reasoned update. `go build ./internal/kernel ./internal/control ./internal/workbench ./internal/desktop ./cmd/polis`, `npm run build`, all 90 migration hashes and `git diff --check` pass. Tests, PostgreSQL migration/runtime and provider execution were not run. See `docs/implementation/REQ16_MISSION_BUDGET_COMPOSITION.md` and the Slice 131 evidence.

## Previous completed slice (130, REQ-16 explicit Mission tool-call cap; Schema 89)

Schema 89 backfills Mission usage from durable Task counters while leaving caps unset. New Mission creation requires an explicit finite cap; Settings configures pending legacy Missions and permits only total-cap increases through a confirmed, reasoned append-only ledger. Admission and each accepted call serialize Mission → WorkerSession/Task → ProblemKey, incrementing applicable counters atomically. Immutable Mission-level denial snapshots, Handover fields and provider turn clamping use the same cap. `go build ./internal/kernel ./internal/control ./internal/workbench ./internal/desktop ./cmd/polis`, `npm run build`, all 89 migration hashes and `git diff --check` pass. Tests, PostgreSQL migration/runtime and provider execution were not run. Company/Provider/token/money accounting remain open. See `docs/implementation/REQ16_MISSION_BUDGET_COMPOSITION.md` and the Slice 130 evidence.

## Previous completed slice (129, REQ-16 Mission budget composition and lock-order audit; no migration)

The source-backed Mission cap design and lock-order rationale are recorded in `docs/implementation/REQ16_MISSION_BUDGET_COMPOSITION.md`; Slice 130 implements the cap under that decision.

## Latest completed slice (128, REQ-16 scoped provider retry observation; no migration)

Real Codex Worker usage identifies that the reconnect count covers only app-server `responseStreamDisconnected` events marked `willRetry`. The terminal record stores that scope and `unobserved_provider_retry_count: null`; retries internal to the CLI/service are not exposed or charged. `retry_visibility=limited` remains. This is conservative observability, not complete retry accounting. `go build ./internal/kernel ./internal/control ./cmd/polis`, frontend build, all Schema 88 migration hashes and `git diff --check` pass. Tests and provider execution were not run. Evidence: `evidence/development/r1-r3-implementation-validation-20261003-slice-128-provider-retry-visibility/verification.md`.

## Latest completed slice (127, REQ-16 explicit incomplete budget closeout; Schema 88)

Schema 88 adds an immutable local-owner incomplete Task closeout tied to the latest budget-caused rejection and exact Task, ProblemKey and closing-reserve snapshots. It requires a nonterminal Task with no live WorkerSession and prevents later Worker admission or Task-budget allocation. The Workbench exposes reasoned confirmation and shows the outcome. Together with Schema 86 rejections and Schema 84/87 owner allocations, exhausted work can remain blocked, receive funded recovery subject to normal Worker admission gates, or close explicitly as incomplete. Admission, allocation and closeout serialize on the Task row. Go package/command build, frontend production build and `git diff --check` pass; the frontend reports the existing >500 kB advisory. Tests and PostgreSQL migration/runtime were not run. Evidence: `evidence/development/r1-r3-implementation-validation-20261003-slice-127-task-budget-incomplete-closeout/verification.md`.

Slice 128's scoped provider retry observation is recorded above. Slices 130–131 add Mission tool-call enforcement and its closing reserve; Company cap implementation, ProviderAccount identity/limits, hidden CLI retries, token/money accounting and scenario qualification remain open.

## Objective and boundaries

Continue the user-approved Polis R1–R3 implementation from the frozen v0.4.5 design pack and the live coverage ledger at `docs/implementation/R1_R3_IMPLEMENTATION_COVERAGE.md`. Work toward the full approved plan; do not redefine completion around the slices already present. Keep the work finite and tied to the existing REQ/FT/NT/CAP/UI/WF/PP traceability.

Explicit exclusions remain office/3D, dynamic hiring/firing, arbitrary MCP compatibility, and a plugin marketplace. Do not run real employee model turns, real QQ sends, external MCP/GitHub actions, business-account flows, or production actions without separate authorization. Preserve historical R0 evidence and the original design snapshot.

## Source of truth

- GitHub repository: `https://github.com/chyinan/Polis`
- Branch: `main`
- Current code checkout used for this continuation: `/data/data/com.termux/files/home/polis` (Termux)
- Slice101 feature commit: `998f425` (`feat: add fake-only product direct messaging`), followed by review-fix commits `aa0a53e` and `6df7d54`. They add ordered Task row locks, an active-Mission `FOR SHARE` lock, and explicit rejection of an omitted `actionable` field. Read-only re-review found zero remaining findings.
- An older local checkout exists at `D:\Programs\Polis` on `master` with unrelated uncommitted/untracked files. Do not use that checkout as the current source of truth or copy its working-tree contents into `main`.

Clone/pull `main` before continuing. Read this file, `AGENTS.md`, `docs/implementation/NEXT_SLICE.md`, `docs/implementation/PROGRESS.md`, and `docs/implementation/R1_R3_IMPLEMENTATION_COVERAGE.md` first. The coverage ledger is authoritative for the finite remaining scope.

## Just-closed slice

Slice102 adds an opt-in, bounded automatic dispatcher for the fixed product `compat/emp-backend` Task. It selects at most one eligible Task every 30 seconds, rotates by Company ID after each attempt, requires an active Company/Mission, a unique ready Task with validation binding and workspace, no prior WorkerSession, no other live session for the Employee, and `wake_pending` schedule state. WorkerSession admission remains the final authority, and the dispatcher shares the Mission lifecycle lock. It never clears `paused` or `waiting_quota`.

The dispatcher is off by default and requires both `POLIS_AUTO_WORKER_DISPATCH_ENABLED=1` and the exact zero-egress Fake @7 surface. It does not dispatch Routine Tasks, retry a Task after a WorkerSession exists, or use real provider transport. The Company cursor is process-local; cross-instance fairness, global slot accounting, authoritative quota readiness/recovery, and safe production dispatch remain open. No schema migration or real Worker/model turn was used.

Slice103 adds the first REQ-14 capability-revocation status projection. The company capability catalog and Workbench show current global/employee Skill/MCP revocations, accepted/effective-for-new-dispatch flags, quiescence, recorded-use WorkerSessions, and MCP call outcomes. The projection rebuilds from durable rows in a repeatable-read transaction, needs no process-local restart state, and uses bounded detail with complete counts and truncation flags. `quiesced` requires every inventoried session to be `stopped` and zero `dispatching` MCP intents; `reconcile_required` remains live and `outcome_unknown` stays visible. Schema remains 72; no Worker stop is initiated.

The projection only inventories sessions with recorded Skill loads or MCP intents; it does not capture a bound-but-unused session at revoke time, stop a Worker, or show superseded historical revocations. Next capture the affected-session set at the revoke linearization point, then implement idempotent stop/restart reconciliation before wiring a management action. Verification: Go command build, frontend production build, and diff check pass; tests were not run. Evidence: `evidence/development/r1-r3-implementation-validation-20261001-slice-103-capability-revocation-projection/verification.md`.

Slice104 adds Schema 73 revoke-time snapshot ledgers. Both global capability revocation and employee unbind capture their affected WorkerSessions inside the guarded transaction; the snapshot stores each session's state and Skill-load count and pins exact matching MCP intent IDs plus their initial statuses. The catalog projection reads these snapshots for new revocations, so later sessions/intents after a rebind do not leak into the old revoke inventory. Pre-Schema-73 revocations retain the event-ledger fallback. This still does not stop Workers. Verification is build-only (Go command/frontend builds and diff check); no tests or PostgreSQL migration execution were run. Evidence: `evidence/development/r1-r3-implementation-validation-20261001-slice-104-revocation-session-snapshot/verification.md`.

Verification on this Termux host:

- `go test ./internal/control ./cmd/polis` passes; `go test ./internal/kernel -run '^$'` compiles the package without running tests.
- `go build ./cmd/...` passes for `android/arm64`; `GOOS=windows GOARCH=amd64 go build ./cmd/...` passes.
- The combined Kernel/control/command test run hits the Termux seccomp denial of `fchmodat2` in the existing `TestUnixParentDirectorySyncUnsupportedSentinelRemainsFatal`; the dedicated PostgreSQL suite was not run because the bundled x86_64 `.tools/pg` runtime is absent. The persistent development database was left untouched.
- `git diff --check` passes. `rtk` is unavailable in this Termux environment.

Evidence: `evidence/development/r1-r3-implementation-validation-20261001-slice-102-auto-worker-dispatch/verification.md`.

Slice101 (below) remains the prior product direct-messaging slice.

Slice101 connects direct messaging to the generic product Worker adapter through the separately versioned 12-tool `polis-product-tool-surface@7`. It provides bounded same-Mission target discovery, direct send, ordered inbox, acknowledgement, application evidence, and candidate-Artifact resolution. The Kernel locks the Mission row `FOR SHARE`, then locks source and target Tasks in stable ID order, then checks the current WorkerSession Task, explicit recipient, fixed employee roster, Mission, and Task states. The product API rejects missing or null `actionable`; `false` explicitly selects FYI. Actionable messages persist the Obligation, work signal, and schedule wake transactionally. FYIs create no Obligation or wake and advance in event order after acknowledgement.

The exact @7 manifest is `82d7b2dbc41ff3dbed56813b3b3adcfad48818fb29653bcd2debf1f280e507eb`; the aggregate schema is 3503 bytes with digest `769f7c9f4afb1c1d0ebfb43037a06d8f2ddb661bc111970c4d199f48f962fd42`. Only the exact zero-egress Fake Runtime purpose/envelope/markers accept @7. The real-provider path remains pinned to @4. Product-provider Task execution is still restricted to `compat/emp-backend`; this slice does not qualify other fixed roles or any real provider interaction. Schema remains 72; no migration was added.

Verification completed:

- `rtk bash scripts/go.sh test ./internal/codex ./internal/kernel ./internal/provider ./internal/control`
- `rtk bash scripts/r1-employee-schedule-postgres-test.sh` on a dedicated disposable PostgreSQL 18 instance through Schema 72, including send/submission and send/pause event-order checks
- `rtk bash scripts/go.sh test ./...`
- `rtk bash scripts/go.sh build ./cmd/...` for Linux amd64
- `rtk bash -lc 'GOOS=windows GOARCH=amd64 ./scripts/go.sh build ./cmd/...'`
- `rtk git diff --check` after the final handoff edit.

Evidence: `evidence/development/r1-r3-implementation-validation-20260930-slice-101-product-direct-worker/verification.md`.

## Remaining work and next move

Use the existing finite list in `R1_R3_IMPLEMENTATION_COVERAGE.md`. REQ-13 still needs authoritative quota readiness/recovery, cross-instance fairness/global slot accounting, and safe dispatch qualification; Slice102 adds only the bounded single-process Fake dispatcher. Other open software items include lifecycle safety for REQ-14/15/16/25/26/29/39; the qualified employee/runtime/continuity path for REQ-23/24/27/30–34; signing and clean Windows VM/package checks; and one bounded FT/NT/CAP/UI/WF/PP traceability reconciliation. R2 Linux host/recovery and remote-workbench qualifications, plus independent R3 content/research quality, cost, recovery, and organization-benefit evidence, remain separate qualification work.

REQ-14 dispatch mapping plus the Slice103 projection and Slice104 revoke-time snapshot are recorded in `docs/implementation/REQ14_REVOCATION_DISPATCH_MAP.md`. Next add idempotent Worker stop and restart reconciliation. Keep `effective_for_new_dispatch` separate from `quiesced`; the latter remains unavailable until each affected Worker is confirmed stopped and each in-flight intent is completed or retained as `outcome_unknown`. Keep real provider, QQ, MCP endpoint, and production qualification actions disabled unless separately authorized.

## Working conventions

- Follow `AGENTS.md`: Go/PostgreSQL 18/pgx/sqlc/goose, migrations only forward, evidence under `evidence/development/`, and RTK-prefixed shell commands.
- Preserve exact provider-surface fingerprints and old probe registries. A new model-visible tool set needs a distinct versioned surface and independent qualification.
- Use disposable PostgreSQL/file roots only. Full Go suite and both Linux/Windows command builds are appropriate after code slices; external qualification remains gated.
- There is one principal code writer. Review is read-only. Wait for reviewers instead of terminating them early.
- Slice101 through Slice104, including this current handoff and the REQ-14 dispatch map, are synchronized to GitHub `main` for cloud pickup. This is source synchronization only; it does not authorize deployment or external provider actions.

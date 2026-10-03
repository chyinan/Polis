# REQ-16 Mission budget composition decision

Updated: 2026-10-03, Slice 132

## Decision

The first outer-scope budget to enforce is a Mission-level **admitted protocol tool-call cap**. It is one separately dimensioned cap, not a claim that every provider request, CLI-internal retry, token, dollar, or tool-side effect is visible. The Codex CLI path continues to report `retry_visibility=limited`, with unobserved retry count unknown.

Each accepted tool-call usage fact must advance the Session, Task, ProblemKey, and Mission counters atomically when those scopes apply. These are projections of one accepted call; a summary must not add those projections together. A ProblemKey cap remains a child constraint and does not create or reserve Mission capacity. The Mission cap must be explicitly set by the local owner; do not derive it by summing profile defaults, Task limits, or ProblemKey limits. Existing Mission usage can be conservatively backfilled from the already-persisted Task call counters, but the cap itself must remain pending where no explicit finite Mission ceiling exists.

The Mission closing reserve is a policy inside that total cap. Only fixed, Kernel-authorized closing Task classes may consume it; a closing call still advances the same Mission usage counter and remains inside the cap. The ProblemKey reserve remains an additional child-level admission constraint and does not stand in for Mission reserve policy.

## Source basis

- `spec/design-v0.4.5/ARCHITECTURE.md` §14.1 permits caps at Group, Company, Mission, Employee, ProviderAccount and Attempt scopes, requires one uniquely identified usage fact projected into applicable scopes, and states that a child cap is not automatically parent allocation/reserved spend. It also requires checking relevant scopes in one short transaction.
- `spec/design-v0.4.5/ARCHITECTURE.md` §14.2 says partially observed CLI use cannot be presented as an exact USD hard cap; tool calls, tokens, bytes and other units remain separate.
- `spec/design-v0.4.5/ARCHITECTURE.md` §14.5 and `spec/design-v0.4.5/contracts/C-RETRY.md` §34.3 place protected closeout capacity inside the Mission total and require trusted classification.
- `spec/design-v0.4.5/contracts/C-RETRY.md` §34.2 and frozen FT-43 require a Mission outer limit to continue accruing when a new Task ID is used for the same or a different ProblemKey.
- Frozen FT-42 and §34.1 require retry ownership and bounded visibility; hidden SDK/harness retries cannot be described as a known total where the runtime cannot observe them.

## Lock-order finding

Current Worker admission reads a Task, then locks its Task row and the ProblemKey budget row (`internal/kernel/worker_state.go`, `txNewWorkerWithToolBudget`). Tool-call charging locks the WorkerSession and Task together, then locks the ProblemKey budget (`Kernel.TXConsumeToolCall` in the same file). Mission cancellation updates the Mission row and then updates its Tasks (`Kernel.TXCancelMission` in `internal/kernel/mission.go`).

Adding a Mission row lock after the current Task lock would create opposite orders between charging and cancellation. The enforcement slice must first make admission/charge and Mission lifecycle lock acquisition consistent, with a documented order of Mission → WorkerSession/Task → ProblemKey where those rows are needed. Audit every affected writer before applying that order; read-only projections can remain snapshot reads. ProblemKey-only allocation paths must not acquire Mission after holding the ProblemKey row.

## Slice 131 implementation state

Schema 90 adds a separately revisioned Mission reserve inside the explicitly configured total call cap. Each policy revision binds the current cap/revision and closing-class usage baseline; remaining reserve is the configured amount less calls by fixed Kernel-created `review` and `peer_review` Tasks since that baseline. Ordinary admission and accepted calls cannot enter that protected portion, while closing-class calls decrement the reserve and increment Mission/ProblemKey/Task/Session use atomically. The first finite cap for a pending Mission must cover used calls plus remaining reserve. Mission rejection records include reserve snapshots, and Handover/provider budget projections expose reserve remaining and revision. Evidence: `evidence/development/r1-r3-implementation-validation-20261003-slice-131-mission-closing-reserve/verification.md`.

## Previous Slice 130 implementation state

Schema 89 adds Mission usage and cap/revision columns, backfills usage by summing durable Task counters, and leaves the cap null for all existing Missions. The append-only allocation ledger only permits initial configuration or increases; immutable rejection evidence records pending/exhausted Mission denials. Workbench Mission creation requires an explicit finite cap, and Settings configures pending legacy Missions or raises a total cap with a reason, confirmation and expected revision. Kernel admission and charging acquire Mission before Session/Task and ProblemKey. Each accepted call increments all applicable counters in one transaction. Handover reports Mission usage/remaining and the provider turn limit uses the minimum remaining allowance. Hidden retries remain `limited` and are not claimed by this cap. Evidence: `evidence/development/r1-r3-implementation-validation-20261003-slice-130-mission-tool-call-budget/verification.md`.

## Remaining bounded sequence

1. Implement a Company cap for admitted protocol tool calls, with explicit owner-set limits and a conservative backfill from immutable/durable Task usage. Label this unit precisely; it does not bound provider egress, hidden retries, tokens or USD.
2. Do not claim ProviderAccount financial enforcement until a stable account identity is bound to general WorkerSessions and runtime admissions can reserve, settle and retain unknown liabilities.
3. Evaluate hidden retry, token and money accounting only where exact source observations and usage identities support them. Keep FT-42–45 and broader qualification separate from implementation evidence; they remain `not_run`.

Company and ProviderAccount caps, true USD/token accounting, unknown external liabilities, and retry visibility below the app-server observer are separate open requirements; none can be derived from this call counter.

## Qualification status

Slices 131–132 are implementation/source-audit evidence, not runtime qualification. Slice 131 Go package/command builds, frontend production build, migration hash validation and diff checks passed; Slice 132 is read-only design/source inspection. Tests, PostgreSQL migration/runtime, provider execution and live cost measurement were not run. FT-42–45 remain `not_run` in the frozen catalog.

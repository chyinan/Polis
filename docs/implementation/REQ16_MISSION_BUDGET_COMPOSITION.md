# REQ-16 Mission budget composition decision

Updated: 2026-10-03, Slice 129

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

## Bounded implementation sequence

1. Add the durable Mission call-cap/usage projection and append-only owner configuration/allocation history. New Workbench Missions must receive an explicit finite cap; a legacy Mission with no trustworthy configured cap stays pending until an owner records one. Backfill usage from Task counters without inventing missing use.
2. Reorder Worker admission, call charging, Mission lifecycle writes, and budget denials around the shared lock order. Check Mission capacity in the same transaction that accepts a call; store Mission cap/usage/revision in the rejection evidence.
3. Expose Mission cap and remaining calls in Handover and Workbench, and expose confirmed owner changes with stale-snapshot protection. Label the unit as admitted protocol tool calls and disclose that hidden retries remain unknown.
4. Add a revisioned Mission closing-reserve policy bounded by the total cap, using only the fixed Kernel-created closing classes. Keep this a later sub-slice if it cannot be safely completed with cap enforcement.

Company and ProviderAccount caps, true USD/token accounting, unknown external liabilities, and retry visibility below the app-server observer are separate open requirements; none can be derived from this call counter.

## Qualification status

This is a source and lock-order audit, not runtime qualification. No code or schema changed in Slice 129. No tests, migration runtime, provider execution, or live cost measurement were run. FT-42–45 remain `not_run` in the frozen catalog.

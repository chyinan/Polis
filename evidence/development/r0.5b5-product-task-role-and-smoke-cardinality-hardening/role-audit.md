# R0.5B5 Mission Task Role Audit

Status: offline domain audit complete. This record is for the disposable R0.5B5 test path; LIVE_1 and R0.5A–B4 evidence are unchanged.

## Existing domain distinction

The persisted `tasks.kind` discriminator is the existing domain distinction. Before B5, the Go `Task.Kind` field was a plain string; there was no typed Task role, execution class, or internal flag. B5 gives that existing discriminator the `core.TaskKind` type and adds the product-provider predicate `IsProductProviderExecutableTask(kind, owner)`. It does not add a second Task-role enum or change the Task schema.

For the current product provider adapter, a Task is provider-executable only when the persisted kind is `compat` and the assigned Employee ID is `emp-backend`. The kernel additionally requires that Task to be `ready`, in an active Mission, the unique provider-executable Task for that Mission, and without any previous WorkerSession attempt. The real adapter obtains the Task from the kernel and binds the WorkerSession to its actual Task and Employee IDs. This is a domain/control-plane rule; Workbench projection does not decide execution eligibility.

## Start-path role matrix

| Task kind | Role | Assignee | Can create a WorkerSession? | Can reserve provider capacity? | User-visible? | Lifecycle purpose |
|---|---|---|---|---|---|---|
| `bootstrap_plan` | Mission bootstrap/control planning | `emp-planning` | Yes. The generic kernel WorkerSession API accepts this kind; the shipped deterministic adapter uses that API with profile `deterministic/fake` and a local runner process. The real product provider WorkerSession API denies it. | No. It cannot pass the product-provider predicate; the Codex runtime also rejects its authorization before allowance creation. | Yes. Workbench returns it with its persisted kind, which is also used as its display title. | `TXStartMissionCommand` changes the Mission from `draft` to `active` and creates this Task in `ready`. In the real product adapter path, `TXPrepareProductTask` records the plan and marks it `completed` before creating the employee work Task. This Task transition does not independently change Mission state. |
| `compat` | Product employee execution | `emp-backend` | Yes. The generic kernel WorkerSession API permits the kind, and the real product adapter creates exactly one WorkerSession bound to this Task and Employee. The provider-only kernel method checks Mission fan-out and rejects a second attempt even if the earlier session stopped. | Yes, only through the real product provider adapter after the actual Task/WorkerSession binding is checked and authorization validates. | Yes. Workbench returns it with its persisted kind and display title. | `TXPrepareProductTask` creates it as `ready`, adds the task-scoped workspace, and freezes a `TaskValidationBinding` when the Mission has a public acceptance contract. Worker execution advances it to `working`; qualified artifact submission moves it to `candidate`. Task state progression is separate from the Mission's active state. |

## Lifecycle and evidence notes

- A normal deterministic `Start` can create a local fake WorkerSession for `bootstrap_plan`; this is not a real employee-provider turn and cannot access the product provider runtime.
- The real product `Start` path creates two Task rows: one `bootstrap_plan` and one `compat`. It completes the bootstrap before selecting the one `compat` Task for real execution. The bootstrap row is therefore permitted in total Task cardinality but excluded from provider-executable fan-out.
- The existing database uniqueness constraint on `(company_id, mission_id, kind)` prevents a second `compat` row for a Mission. B5 did not add a global one-Task constraint. The pure fan-out selector separately rejects a malformed list containing more than one provider-executable Task.
- Workbench's read store returns all Mission Tasks. The API projection already includes Task kind and `taskView` uses that kind as the Task title, so no frontend change was needed: `workbench_change = NOT_REQUIRED`.
- Mission acceptance contracts are optional for exploratory work. If a contract exists, the product Task binding is created transactionally with the Task and workspace. The B5 Start-path proof supplies a public contract and verifies one immutable binding on the `compat` Task. A future LIVE_2 eligibility decision requires that binding and checker coherence to pass.

## Revised smoke cardinality

For one bounded product smoke, count the actual execution fan-out:

- one Company and one Mission;
- exactly one Task satisfying `IsProductProviderExecutableTask`;
- exactly one assigned product Employee and one WorkerSession bound to that Task;
- at most one provider authorization, reservation, egress, and turn;
- concurrency 1, retry 0, successor 0, High 0.

Additional internal/control Tasks are allowed only when they cannot independently create a real provider WorkerSession or provider turn. B5 validates this rule offline. It does not start LIVE_2 or authorize provider traffic.

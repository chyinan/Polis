# R0.5B5 Offline Product Start-Path Proof

Status: offline-only. The harness uses a disposable PostgreSQL 18.6 cluster, the real `control.Service` and `RealProviderWorkerAdapter`, and `provider.FakeRuntime`. No live provider process, external account, or network request is used.

## Expected Start state

The happy-path test `TestRealProductStartHasOneProviderExecutableTaskAndOneBoundWorker` drives Mission creation and Start through the product command API. Its Task and execution assertions are:

- Mission state is `active` after the execution completes.
- Exactly two Mission Task rows exist: one `bootstrap_plan` and one `compat`.
- The bootstrap Task is owned by `emp-planning` and is `completed` by real product task preparation.
- Exactly one provider-executable Task exists: the `compat` Task owned by `emp-backend`, which reaches `candidate` after the fake turn's qualified artifact flow.
- Exactly one WorkerSession exists. It is stopped and bound to the `compat` Task and `emp-backend`.
- Exactly one immutable `TaskValidationBinding` exists for the `compat` Task. Its Task/Mission IDs and configuration digest match, and its frozen contract matches the public Mission acceptance contract.
- Exactly one product check passes. Its Task, Mission, binding digest, WorkerSession, and epoch match the same execution binding.
- The provider authorization recorded by the fake terminal observation matches the actual Company, Mission, Task kind/owner/ID, Employee, WorkerSession, epoch, and incarnation.
- The fake runtime observes one reserve-interface call, one process-start call, and one logical turn. Fake runtime provider egress is zero; real provider reservations and egress are zero.
- Replaying the same Start request is idempotent; a different request ID conflicts. Neither replay creates another Task, WorkerSession, or fake reservation.

The happy-path test logs the concrete disposable IDs and count snapshot in `offline-start-path-test.log` when run with `-v`.

## Failure and authorization boundaries

- A direct request to create a product-provider WorkerSession for `bootstrap_plan` returns `DENIED` and leaves WorkerSession count at zero.
- `ValidateExecutionAuthorization` rejects an `emp-planning` bootstrap Task. `CodexRuntime.Reserve` rejects it before creating its allowance file.
- A Mission seeded with an existing `compat` row cannot create a second one under the existing per-kind Mission uniqueness constraint. Product Start fails before WorkerSession creation or fake reserve/start/turn.
- The pure selector returns `CONFLICT` for an in-memory malformed Mission containing two provider-executable Tasks.
- If fake process start fails after the first fake reservation call, same-request Start replay is fenced by the prior Task WorkerSession. It makes no second fake reservation and creates no second session.
- Workbench integration coverage confirms both `bootstrap_plan` and `compat` remain visible with their kinds and owners.

The fake reserve-interface call exercises the product orchestration boundary only. It is not a real provider reservation and consumes no live Medium/High allowance. B5 did not call LIVE_2, send provider traffic, or start a successor.

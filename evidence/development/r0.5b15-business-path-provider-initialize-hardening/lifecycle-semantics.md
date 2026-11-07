# R0.5B15 lifecycle semantics

The frozen LIVE_3 state `WorkerSession=stopped`, `compat Task=working`, `Mission=active` is intentional for an in-process terminal runtime failure before any model turn or delivery.

- `RealProviderWorkerAdapter` stops and proves the WorkerSession, but does not cancel an active Mission or synthesize a business failure/Artifact.
- The Task remains `working` during the current controller incarnation because no `workspace_check`, `task_submit`, or delivery transition occurred.
- On a later kernel/controller recovery, `txResetFakeState` runs under the runtime lease and changes `working` Tasks with no live WorkerSession back to `ready` with an incremented generation; non-stopped sessions become `reconcile_required`.
- Therefore the frozen combination means “recoverable after a later separately authorized controller recovery,” not “candidate,” “completed,” or “model/business failure.”
- LIVE_3 remains immutable; this document records the current invariant and does not mutate its Mission/Task rows.

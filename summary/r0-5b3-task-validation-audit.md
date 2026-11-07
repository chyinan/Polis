# R0.5B3 Task Validation Binding Audit

## Existing authority and reusable lifecycle

- `Binding` stores company/employee/runtime incarnation/epoch/session; `checkSession` and `TXWrite` provide current session authorization and company-row serialization.
- The current Task is derived from the session in `Handover`; `worker_workspaces` is one whole-content CAS blob per Task with digest/revision. It is not a file tree.
- Product `workspace_check` currently calls an adapter-injected `CheckRunner(content, phase)`. `runner.Report` only has `Passed`, output, digest and phase. There is no Task-selected contract/runner binding.
- `worker_checks` persists session/digest/phase/passed/report. `TXCheckpoint` checks passed receipts and current workspace digest/revision; `HasQualifiedCheckpoint` checks qualified checkpoint against workspace digest/revision. Generic artifact staging/finalization does not revalidate a task validator binding inside its publication transaction.
- `TXPrepareProductTask` currently copies a hard-coded `offline-product-checker` string into the bootstrap plan, while the newly created `compat` Task has no formal validation binding. Real worker mode injects the non-empty-only `OfflineProductCheckRunner`.

## Historical isolation

R0.3A peer surfaces use `PeerBackendTools`/`PeerFrontendTools`, `TXPeerCheck`, peer contract revisions, and `TXSubmitQualifiedPeer` with exact peer-specific checker/policy bindings. These contracts and tools must remain unchanged; their patterns can inform atomic finalization but not be substituted for the generic product TaskValidationBinding.

## Storage direction selected by user

Use a dedicated task-scoped `task_validation_bindings` relation rather than overloading free-form `tasks.plan` or adding many validator fields to `tasks`. Mission creation carries an optional public acceptance contract; product Task preparation freezes it into the binding. The remaining user choice is whether missing binding allows exploration with `VALIDATION_NOT_CONFIGURED` (finalization denied by the existing qualified-checkpoint policy) or fails before Task creation.

## B3 boundaries

Keep R0.5A/B1/B2 raw evidence immutable, no provider reservation/egress, no live canary, and no R0.5B business smoke. Offline integration must use a dedicated disposable PostgreSQL database, the existing frontend, the real adapter, and a fake/local provider runtime.

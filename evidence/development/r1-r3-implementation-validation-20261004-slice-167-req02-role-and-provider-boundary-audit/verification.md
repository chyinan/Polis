# Slice 167 — REQ-02 fixed roles and direct-message/provider boundary audit

Date: 2026-10-04

## Scope

Read-only source audit of the fixed Employee roster, product-provider Task admission, direct-message tool registration and Kernel enforcement, real-provider authorization, and WorkerSession write boundary. The decision preserved the existing real-provider @4 qualification and the rule that Worker actions require a database-confirmed active WorkerSession.

## Findings

- The database constrains `employees.id` to exactly `emp-planning`, `emp-backend`, `emp-frontend`, and `emp-review`; Schema 8 maps them to the four fixed role names. See [00001_r0.sql](../../../db/migrations/00001_r0.sql) and [00008_r06_dogfood_organization.sql](../../../db/migrations/00008_r06_dogfood_organization.sql).
- Product-provider Task classification is only `compat` owned by `emp-backend`. The provider authorization validator independently requires the Task owner and Employee ID to match and `EmployeeRole == "backend"`. The product WorkerSession admission path rechecks the classification, uniqueness within the Mission, validation binding, workspace identity, schedule state, and absence of another non-stopped Employee session before persisting a `restoring` session. See [task_kind.go](../../../internal/core/task_kind.go), [task_execution.go](../../../internal/kernel/task_execution.go), [worker_state.go](../../../internal/kernel/worker_state.go), and [authorization.go](../../../internal/provider/authorization.go).
- The product direct-message registry adds five tools to the seven-tool product surface. @7 is pinned as a 12-tool surface and is accepted only when the provider mode is `fake` and its offline simulation markers and full surface manifest match. The real-provider readiness and authorization gates remain pinned to the qualified seven-tool @4 fingerprints; the adapter rejects another surface pending qualification. See [tools.go](../../../internal/codex/tools.go), [runtime.go](../../../internal/provider/runtime.go), [authorization.go](../../../internal/provider/authorization.go), and [real_provider_worker.go](../../../internal/control/real_provider_worker.go).
- The Kernel lists no more than 16 target Tasks from the same Mission and fixed roster. Send validation checks the bound sender Task is working in an active Mission, locks source and target rows, and rechecks exact target owner, same-Mission membership, and `ready`/`working` state. FYI sends persist a message; actionable requests additionally persist an obligation and wake signal. Inbox, acknowledgement, application-evidence, and candidate-backed obligation resolution are separate lifecycle steps. See [product_direct_messaging.go](../../../internal/kernel/product_direct_messaging.go) and [employee_ops.go](../../../internal/kernel/employee_ops.go).
- `checkSession` binds Company, exact WorkerSession, Employee, Task, epoch, incarnation, and provider execution mode. Kernel writes require both `worker_sessions.state='active'` and an active Mission. Admission first persists `restoring`; host/runtime activation transitions the exact session to `active` before the Worker run loop may write.

## Decision

No safe local code change follows from this audit. Enabling @7 on a real provider would bypass the exact current @4 surface authorization and require its own exact-surface provider qualification. Making Planning, Frontend, or Review product-provider executable would require role-specific product Task and validation contracts plus corresponding admission/authorization qualification; broadening the existing Backend rule alone would not be safe. REQ-02 remains partial. The local development database has no Company or WorkerSession, so this stage performed no Worker operation.

## Verification

- Source audit only; no Go or frontend tests were run.
- No WorkerSession was created, activated, stopped, or dispatched.
- No real or fake provider turn was run and no provider egress occurred.
- No code or schema changed. Documentation consistency was checked with `git diff --check`.


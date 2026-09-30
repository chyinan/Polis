# R0.5A command surface audit

Verified 2026-09-16 from the existing source and frozen design documents.

## Existing

- `internal/kernel.Kernel` owns PostgreSQL access, advisory-lock singleton control, runtime incarnation, company scope and employee/session fencing.
- `TXCreateMission` creates a draft with fixed `r0-arithmetic@1`; `TXStartMission` activates a draft and creates a bootstrap task; `TXSetPaused` flips active/paused; `TXBeginStop` and `TXConfirmStopped` provide worker stop proof.
- `Kernel.Step` is deterministic/fake-only and performs scripted claim/send/submit/resolve/verify without provider calls.
- `TXWrite` persists `(company, actor, key)` receipts and ordered company events, with same-key replay and conflict on changed fingerprints.
- `cmd/polis serve` and `internal/workbench` expose only read-only overview/activity/stream endpoints. `RealWorkbenchApi` and React Query expose only reads.

## Missing for R0.5A

- Mission title/goal persistence and a Mission create/submit HTTP command.
- Strict product start conflict semantics and a Mission cancel terminal state.
- Product command application layer and typed command receipts.
- Deterministic WorkerAdapter lifecycle binding to WorkerSession and stop proof.
- Frontend mutation methods/hooks/UI command lifecycle and post-mutation refetch.
- HTTP, integration, and browser E2E coverage for positive and negative command paths.

## Deliberate boundaries

- No provider/app-server path, QQ/MCP/GitHub, SSE implementation, replacement Workbench, or new R0.3A experiment.
- Pause/Resume remains deferred because the existing `TXSetPaused` is not a complete formal runtime pause/resume semantic.
- Existing historical changes and evidence remain untouched; only new R0.5A files and directly related source are in scope.

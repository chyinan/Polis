# R0.6 I checkpoint — Restart / Recovery UX

> Date: 2026-09-21  
> Status: PASSED targeted baseline

## Delivered

- Added `ViewMeta.recoveryState` with explicit `operational` / `recovery_required` semantics.
- Existing kernel recovery/fencing remains authoritative: restart advances incarnation, marks live sessions `reconcile_required`, and preserves companies/missions/tasks/artifacts.
- Employee projection maps `reconcile_required` to a visible warning state rather than stopped/working; the AppShell exposes recovery status in the top bar.
- Added controlled restart/recovery integration coverage using a real active WorkerSession state and a second kernel/read-store open.

## Verification evidence

- `rtk bash scripts/go.sh test ./internal/kernel ./internal/control ./internal/workbench ./cmd/polis` — passed.
- `rtk npm run typecheck` from `frontend/` — passed.
- `rtk npm test -- --run src/domain/workbench-validation.test.ts` — 11 tests passed.
- Disposable PostgreSQL 18 cluster under `/tmp/polis-r06-i-pg` — schema 11 applied; `TestRestartRecoveryProjectionShowsReconciliationRequired` passed after controlled restart; cluster stopped and removed.

## Known boundaries

- Recovery does not automatically rerun a provider or silently resume an interrupted external action.
- `reconcile_required` remains visible until a formal worker reconciliation path changes it.

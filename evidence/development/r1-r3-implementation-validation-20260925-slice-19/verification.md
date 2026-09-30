# Slice 19 — bounded preparation admission and deferred snapshot loading

Date: 2026-09-25  
Scope: bound the Windows/Node preparation executor's active and waiting work, and load the immutable CAS execution snapshot only after a preparation owns an execution slot.

## Implemented

- Preparation remains limited to two active runs and now admits at most eight queued runs. Queue exhaustion returns `environment_executor_busy`; cancellation, executor close, and slot release each release queue accounting.
- The Control executor contract passes a snapshot loader rather than an already-loaded snapshot. The Windows executor acquires queue/slot admission before it re-reads and verifies source CAS bytes, so waiting requests do not retain large snapshots.
- Regression cases cover full-queue rejection without invoking the loader, admitted snapshot loading, cancellation, concurrent preparation bounds, close/drain, and pending process cleanup ownership.

## Verification

- `scripts/go.sh test ./internal/control -count=1` passed.
- `scripts/go.sh test -race ./internal/control -count=1` passed.
- The dedicated Schema 26 PostgreSQL environment-preparation integration test passed with a fake executor and no provider egress.
- Windows amd64 Control/environment test binaries and command packages cross-compiled.
- No native AppContainer/WFP test, real npm install, project script, external registry request, model, QQ, or MCP operation ran.

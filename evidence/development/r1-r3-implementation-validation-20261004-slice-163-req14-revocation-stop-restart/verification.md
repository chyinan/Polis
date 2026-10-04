# Slice 163 — REQ-14 capability revocation stop retry after restart

Date: 2026-10-04

## Scope

Inspect whether an accepted capability revoke reaches its affected WorkerSessions and whether the exact-session stop path can resume after a process restart. Make the smallest source correction found. No Worker was stopped.

## Findings and change

- `internal/kernel/capability_governance.go:550-597` snapshots live sessions bound to the revoked capability and sessions with recorded Skill/MCP use in the same revocation transaction. The snapshot includes relevant MCP intents. The function comment records the serialization point: Worker creation and new Skill/MCP admissions cannot cross the company-row-locked transaction.
- `internal/control/capability_revocation_stop.go:24-80,89-120` starts with an immediate durable-snapshot scan, retries every 15 seconds, and processes exact company/session pairs. It serializes with in-process Mission lifecycle actions, stops the exact session and marks interrupted dispatching MCP calls `outcome_unknown`.
- `internal/kernel/recovery.go:146-153` marks every non-stopped WorkerSession `reconcile_required` during Kernel recovery. `internal/control/real_provider_worker.go:993-1019` falls back to exact host reconciliation when no in-memory owner remains.
- **Gap fixed:** `internal/kernel/worker_process_recovery.go:579-605` previously allowed only `restoring` in the Windows exact-session reconciler. If initial startup reconciliation failed, the durable revocation stopper retried the session in `reconcile_required`, but Windows rejected every retry. The reconciler now accepts either `restoring` or `reconcile_required`; it still requires the exact company/session, Windows Job Object containment and a process-tree stop proof, then `TXConfirmWindowsWorkerTreeStopped` checks the unchanged database state before marking it stopped. Linux already accepted `reconcile_required` in its exact-session path.
- `cmd/polis/main.go:357-365,596-606` runs host reconciliation during startup and starts the durable revocation stop coordinator afterward, so an unresolved revoked session can reach the exact-session retry path.

## Verification and limits

- `go build -o /data/data/com.termux/files/usr/tmp/polis-slice163-linux ./cmd/polis` passed.
- `env GOOS=windows GOARCH=amd64 go build -o /data/data/com.termux/files/usr/tmp/polis-slice163-windows.exe ./cmd/polis` passed.
- `git diff --check` passed.
- No tests, Worker start/stop, provider call, database mutation, Desktop/browser/Tauri flow or frozen CAP/FT scenario ran.
- REQ-14 remains open pending runtime stop/restart qualification and its other acceptance evidence. No Worker action occurred in this slice.

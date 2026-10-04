# Slice 164 — REQ-14 revocation inventory completeness

Date: 2026-10-04

## Scope

Audit whether the revocation Workbench can claim quiescence when historical WorkerSession inventory is incomplete. Add a durable completeness marker and make incomplete history fail closed in the API/UI.

## Finding and change

- `internal/kernel/capability_revocation_read.go:150-179,293` previously fell back to Skill/MCP usage ledgers when no session snapshot rows existed, then set `quiesced` from the fallback's live-session and dispatching-call counts. A pre-Schema-73 revoke with no recorded use could therefore appear quiesced even though the system could not know whether an unused-but-bound WorkerSession existed.
- `db/migrations/00099_capability_revocation_snapshot_completion.sql:1-22` adds an immutable `capability_revocation_snapshot_completions` receipt. `internal/kernel/capability_governance.go:553-638` writes the receipt in the same revoke transaction after the session and MCP-call snapshot, including when the complete session set is empty. Receipt metadata pins scope, capability version, Employee and session count.
- The read projection accepts a completion receipt or a nonempty exact Schema-73 snapshot as complete. A legacy zero-row revocation without either remains visible using its known usage ledger but sets `sessionInventoryComplete=false` and `quiesced=false`. The Workbench now labels that state “需复核” and displays completeness separately from stop counts.
- `db/migration_hashes.sha256` includes the Schema 99 migration hash. The local Termux development database migrated successfully from Schema 98 to 99. It still has no owner, Company or WorkerSession.

## Verification and limits

- Local Go command build passed: `go build -o /data/data/com.termux/files/usr/tmp/polis-slice164-linux ./cmd/polis`.
- Windows cross-build passed: `env GOOS=windows GOARCH=amd64 go build -o /data/data/com.termux/files/usr/tmp/polis-slice164-windows.exe ./cmd/polis`.
- Frontend production build passed: `npm run build` (`tsc -b && vite build`). Vite printed its existing large-chunk advisory.
- Local migration passed: `.runtime/dev-termux.sh migrate` reported Schema 99 successfully applied.
- `git diff --check` passed.
- No tests, Worker start/stop, provider call, capability revoke, or business-row mutation ran. Existing historical revocations with zero snapshot rows remain incomplete; safe owner-reviewed disposition and runtime stop qualification are still open. REQ-14 remains open.

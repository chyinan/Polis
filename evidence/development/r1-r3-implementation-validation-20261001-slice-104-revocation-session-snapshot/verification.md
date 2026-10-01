# Slice 104 verification — revoke-time WorkerSession snapshot

Date: 2026-10-01  
Host: Termux, Android/arm64  
Schema: 73

## Changes

- Schema 73 adds append-only `capability_revocation_sessions` and `capability_revocation_mcp_calls` ledgers.
- Global capability revocation snapshots live sessions for employees historically bound to the revoked version, plus stopped sessions with recorded use. Employee unbind snapshots that employee's live sessions and stopped sessions with matching recorded use.
- The transaction pins each session's state and Skill-load count, then records exact matching MCP intent IDs and their initial status. Both operations run under the company guard that serializes Worker creation and capability dispatch.
- New revocation projections read the immutable snapshot, so a later rebind cannot add its sessions or tool calls to an earlier revoke. Pre-Schema-73 active revocations without snapshots use the durable usage-event fallback.
- No Worker stop coordination is included in this stage.

## Verification

- `go build ./cmd/...` — passed for the local Android/arm64 host.
- `npm run build` from `frontend/` — passed (`tsc -b` and Vite production build). Vite reported the 740.88 kB minified JavaScript chunk advisory.
- `git diff --check` — passed after the final edits.
- Tests and PostgreSQL migration execution were not run. The migration SQL, snapshot concurrency behavior, and read projection therefore have not been exercised against a PostgreSQL instance in this slice.

No real Worker was started or stopped. No provider, MCP endpoint, QQ, GitHub account, or production action ran. No persistent development database was modified.

## Next step

Add idempotent WorkerSession stop coordination that consumes these snapshots and survives Control restart. Preserve the current linearization rule: new calls stay blocked at revoke commit; quiescence remains false until every snapshotted Worker is confirmed stopped and every snapshotted MCP intent is completed or retained as `outcome_unknown`.

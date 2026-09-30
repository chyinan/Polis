# Slice 38 review hardening verification

Date: 2026-09-26

## Changes covered

- Schema 32 persists the selected service ID in JobRun idempotency, storage, command receipts and Workbench history. Existing rows are backfilled as the reserved `legacy:unattributed` sentinel, which cannot collide with a policy service ID; PostgreSQL explicitly rejects a NULL service ID on new service JobRuns.
- Schema 33 enforces revoked/null-lease and non-revoked/non-null-lease parity on new endpoint observations without rewriting immutable legacy events. Revoked endpoint DTOs omit a lease timestamp.
- Endpoint evidence must match the JobRun-bound service ID and its policy-pinned endpoint/healthcheck.
- Service endpoint writes are treated as possibly committed before the transaction returns; all stop, timeout and terminal paths attempt a stable revocation even if the first write outcome is ambiguous.
- Terminal JobRun events carry a stable request ID and remain pending until persistence and endpoint revocation are confirmed. Late confirmed process exit reconciles an unknown stop. Terminal service rows can reconstruct the revoke from the latest durable endpoint when no active in-memory job exists.
- An unknown JobRun with no in-memory process after restart is reconciled only for the Windows/Node profile, after the new executor confirms it has no tracked live process; the old Job Object is kill-on-close and execution is never replayed. This path was exercised with disposable PostgreSQL and a fake executor, while native Windows tests check the executor's active/exited decision.
- Stop can cancel an accepted JobRun before an active process is registered, and Start rechecks the authoritative state before launch. The tracked AppContainer wrapper forwards fail-closed `HasExited` state.
- A root-process exit is not treated as process-tree completion: `Stop` and restart reconciliation wait for the AppContainer monitor to close the Job Object and finish cleanup before returning proof or reconciling an unknown JobRun. Legacy `legacy:unattributed` service rows no longer expose an endpoint-revocation retry that cannot be authorized.
- A native Windows regression test holds the monitor completion channel open after root exit and proves `Stop` stays unconfirmed until Job Object cleanup completes.
- Before sending HTTP health bytes, the Windows probe verifies the PID owning the server side of that exact established loopback TCP 4-tuple. Listener checks also reject exact competitors, wildcard and IPv4-mapped wildcard rows. Windows process state/handle closure is synchronized.

Microsoft documents that `SO_REUSEADDR` permits forced same-port binds with indeterminate TCP delivery, including binds by the same user. The connection-specific PID check uses `GetExtendedTcpTable`'s owner-PID all-connections table and fails closed before the HTTP request is sent. References: [Winsock address reuse guidance](https://learn.microsoft.com/en-us/windows/win32/winsock/using-so-reuseaddr-and-so-exclusiveaddruse), [GetExtendedTcpTable](https://learn.microsoft.com/en-us/windows/win32/api/iphlpapi/nf-iphlpapi-getextendedtcptable), and [MIB_TCPROW_OWNER_PID](https://learn.microsoft.com/en-us/windows/win32/api/tcpmib/ns-tcpmib-mib_tcprow_owner_pid).

## Verification

| Command / exercise | Result |
|---|---|
| `rtk proxy bash scripts/go.sh test -p 1 ./... -count=1` | PASS; full serial Go suite |
| `rtk proxy bash scripts/r1-service-endpoint-postgres-test.sh` | PASS; disposable PostgreSQL 18 migrated through Schema 33; Workbench/Control service lifecycle tests passed, including service ID integrity, lease constraint, accepted-before-registration Stop, terminal-row revoke with no in-memory active job, and unknown-stop recovery |
| `rtk proxy npm run --prefix frontend test` | PASS; 82 tests |
| `rtk proxy npm run --prefix frontend typecheck` | PASS |
| `rtk proxy npm run --prefix frontend lint` | PASS |
| `rtk proxy npm run --prefix frontend build` | PASS; existing Vite chunk advisory remains (579.16 kB main JS, over 500 kB) |
| `rtk proxy bash scripts/go.sh build ./cmd/...` | PASS; Linux build |
| `rtk proxy bash -c 'GOOS=windows GOARCH=amd64 bash scripts/go.sh build ./cmd/...'` | PASS; Windows amd64 build |
| Windows native runner test binary (`internal/runner`) | PASS; all Windows runner tests, including IPv4/IPv6 listener/connection ownership and AppContainer process-tree cleanup |
| Windows native Control test binary (`internal/control`) | PASS; all Windows Control tests, including accepted-owner checks, active/exited reconciliation and waiting for Job Object cleanup. Linux-only fixtures now skip on non-Linux hosts |
| Windows native Environment test: `TestTrackedEnvironmentProcessForwardsExitStateAndFailsClosedWithoutIt` | PASS |
| `rtk proxy git diff --check` | PASS; no whitespace errors; Git reported only the existing CRLF conversion advisory |

The initial Schema 32 PostgreSQL run exposed schema guards still pinned to 31; the guards were advanced, Schema 32 was verified, and Schema 33 was subsequently added and fully reverified. Frontend dependencies were restored from `frontend/package-lock.json` with `npm ci` after concurrent pnpm attempts conflicted with the npm-managed `node_modules`; the final npm checks passed. Native Windows Control execution originally surfaced three Linux-only fixtures in the Windows binary; they now explicitly skip outside Linux, and the full Windows Control test binary passes.

No real model turn, project service, WFP change, QQ send, external MCP/GitHub request, or business account was used. Windows tests used local loopback listeners/connections only. The terminal-row repair was exercised with no in-memory active job but without restarting the daemon. Actual prepared AppContainer service execution, full browser/clean-VM qualification, process-restart drill and WFP behavior remain unverified; the R1 environment gate remains closed.

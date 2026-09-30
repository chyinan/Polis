# Slice 38 — policy-bound service JobRun lifecycle

Date: 2026-09-26

## Implemented

- Control accepts `kind=service` only with a `serviceId` already pinned in the immutable environment policy. Service requests cannot carry a script path or argv; Control resolves the approved script path and uses an empty argument list.
- A service JobRun remains `not_ready` until the configured loopback probe returns the pinned status and response digest and the executor's `ServiceEndpointOwnerVerifier` confirms the launched process owns the listener. `StartProjectJob` returns after readiness, terminal failure, caller cancellation or the environment timeout.
- A bounded monitor renews the exact policy lease before expiration, persists owner-verified unhealthy HTTP observations without marking them ready, and requests process stop when owner proof or persistence fails. Natural exit, stop, timeout and shutdown revoke a recorded endpoint; the Workbench projects terminal JobRuns unhealthy even before the revocation event is appended.
- The Windows Node executor verifies that the PID belongs to one of its tracked, live AppContainer processes. `internal/runner.VerifyWindowsTCPListenerOwner` reads Windows IP Helper owner-PID listener tables and requires the exact configured loopback address/port and PID. Linux service Jobs remain denied because the Linux executor has no namespace-aware owner verifier yet.
- The Workbench offers start buttons only for pinned service IDs in a qualified, prepared Windows environment. The API request contains the `serviceId`; it never sends the service command or args.
- The dedicated Postgres script now runs both Workbench projection and Control lifecycle integration against one uniquely named disposable PostgreSQL 18 cluster. The Workbench service fixture is moved to a terminal state before later restart-recovery tests.

## Verification

| Command | Result |
|---|---|
| `bash scripts/r1-service-endpoint-postgres-test.sh` | PASS; disposable schema-31 PostgreSQL 18: immutable service policy selection, unpinned ID denial, `not_ready`→health-probed `ready`, repeated lease renewal, stop→`revoked`, batch unknown-stop retry recovery, Workbench expired/revoked/terminal projections |
| `bash scripts/go.sh test -p 1 ./... -count=1` | PASS; full serial Go suite |
| `bash scripts/go.sh build ./cmd/...` | PASS; Linux commands |
| `GOOS=windows GOARCH=amd64 bash scripts/go.sh build ./cmd/...` | PASS; Windows amd64 commands |
| Windows runner test binary (`TestVerifyWindowsTCPListenerOwner`) | PASS on native Windows: exact IPv4/IPv6 loopback owner accepted; wrong PID, wrong port, and non-loopback target rejected |
| `cd frontend && npm run test -- --run` | PASS; 82 tests in 12 files |
| `cd frontend && npm run typecheck` | PASS |
| `cd frontend && npm run lint` | PASS |
| `cd frontend && npm run build` | PASS; Vite reports the existing 500 kB chunk-size advisory (built JS: 578.57 kB) |
| `git diff --check` | PASS; Git printed only the existing CRLF-to-LF advisories for `AGENTS.md` and `NEXT_SLICE.md` |

Windows owner tables and row layouts follow Microsoft's [`GetExtendedTcpTable` API](https://learn.microsoft.com/windows/win32/api/iphlpapi/nf-iphlpapi-getextendedtcptable), [`MIB_TCPROW_OWNER_PID`](https://learn.microsoft.com/windows/win32/api/tcpmib/ns-tcpmib-mib_tcprow_owner_pid), and [`MIB_TCP6ROW_OWNER_PID`](https://learn.microsoft.com/en-us/windows/win32/api/tcpmib/ns-tcpmib-mib_tcp6row_owner_pid) definitions. The native test opened only local loopback fixture sockets.

## Limits

The control lifecycle is wired and tested with a fake process plus a local HTTP health server. No actual project service, qualified AppContainer service, npm install, WFP rule, loopback exemption, clean-VM install, independent browser verification or external endpoint was used. The native Windows API test proves the listener-owner query on the host, not the entire product isolation qualification. Linux service Jobs, cross-day service restoration and R2/R3 domain qualification remain open. No database migration was required.

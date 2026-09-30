# Slice 37 — service endpoint event and lease ledger

Date: 2026-09-25

## Implemented

- Added the append-only `Kernel.TXRecordServiceEndpointEvent` command. It scopes the write to a service JobRun, verifies the exact MissionInput source revision and re-reads the canonical approved environment policy before accepting the endpoint and healthcheck digest.
- Locks the same environment revision row used by approval/revocation writes, checks that the latest policy event still approves the exact policy digest, and denies ready/renewal/new-generation observations after revocation. Endpoint revocation remains allowed so a running service can be fenced.
- Enforced generation 1 for the first endpoint observation. A later generation requires the current generation to be revoked first; a revoked generation cannot transition again.
- Bound every non-revoked lease to the exact duration in the pinned probe spec and the probe timestamp. The event uses that timestamp as its event creation time, and each observation timestamp must be strictly later than the prior generation event so delayed probes cannot restore stale readiness. The endpoint event and the active JobRun readiness update commit in one transaction. RequestID replay returns the original receipt.
- Updated service readiness transitions to permit bounded same-state lease renewal and terminal revocation. The Workbench projection reports expired endpoints as unhealthy and revoked endpoint state explicitly.
- The Workbench also projects a service endpoint as unhealthy after its JobRun exits, fails, is cancelled or has unknown outcome, even if the last stored lease has not expired yet.
- Restricted generic JobRun event writes to preserve current service readiness; only the endpoint observation command can promote it to ready. Workbench DTO validation requires non-service Jobs to use `not_applicable`, endpoint-free service Jobs to use `not_ready`, checks non-revoked leases, requires endpoint data for `ready` service Jobs, and checks endpoint/readiness/terminal-state consistency.
- `ProbeServiceEndpoint` now returns bounded observation leases for owner-verified unhealthy HTTP responses. An unhealthy response remains unhealthy and never becomes ready.
- Aligned Workbench validation with Go for the one-second lease minimum, UTF-8 path byte length, and C0/C1 control characters. Service definitions persist the fixed script path and probe without caller-supplied argv.
- Added a disposable PostgreSQL 18 verification script. No schema migration was needed.

## Verification

| Command | Result |
|---|---|
| `bash scripts/go.sh test -p 1 ./... -count=1` | PASS; all Go packages, including environment, Kernel, Control and Workbench |
| `bash scripts/go.sh build ./cmd/...` | PASS; Linux command packages |
| `GOOS=windows GOARCH=amd64 bash scripts/go.sh build ./cmd/...` | PASS; Windows amd64 command packages |
| `bash scripts/r1-service-endpoint-postgres-test.sh` | PASS; disposable schema-31 PostgreSQL 18 database; rejects generic readiness bypass, wrong source, wrong lease, unpinned endpoint, stale probe, renewal/new generation after policy revocation and premature generation; verifies RequestID replay, expired lease projection, atomic readiness change, terminal revocation, and a terminal JobRun cannot retain ready projection |
| `cd frontend && npm run test -- --run` | PASS; 81 tests in 12 files |
| `cd frontend && npm run typecheck` | PASS |
| `cd frontend && npm run lint` | PASS |
| `cd frontend && npm run build` | PASS; Vite reports the existing 577 kB chunk-size advisory |
| `git diff --check` | PASS; Git printed only existing CRLF-to-LF advisories for `AGENTS.md` and `NEXT_SLICE.md` |

Focused RED checks observed the intended failures for lease minimum, UTF-8 path bytes, C1 controls, unhealthy probe lease output, readiness renewal/revocation transitions, missing Kernel command, generic readiness bypass, stale probe overwrite, lease renewal after policy revocation and terminal JobRun readiness projection. Their GREEN counterparts are included above.

A read-only final review found no unresolved issues. The reviewer confirmed policy revocation serialization, stale-probe fencing, generic readiness bypass prevention, terminal JobRun unhealthy projection, and frontend DTO consistency.

## Limits

The Control API still accepts only batch JobRuns. Service script selection/startup, periodic lease renewal, stop/shutdown revocation and native Windows/Linux process-owner verifiers remain unwired. This slice persists policy-bound endpoint observations but does not launch a service or establish live readiness. No WFP rule, loopback exemption, external endpoint, project service, model, QQ, MCP or GitHub account was used.

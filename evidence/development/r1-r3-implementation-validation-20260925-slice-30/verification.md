# Slice 30 verification — persisted R3 domain evidence workflow

Date: 2026-09-25

## Changes covered

- Schema 29 adds an append-only company-scoped domain evidence ledger and normalized evidence items. Each item references a same-company MissionInput in `usable` or `partial` state with the exact ID/revision/content digest, plus an assessment-method digest and company employee. Foreign keys bind MissionInputs and employees; parent JSON and normalized rows are checked for correspondence on readback.
- The Kernel records incomplete or structurally complete submissions with RequestID idempotency. Nil evidence lists normalize to JSON `[]`; nil/empty replay reuses the same record. Database constraints verify profile identity and evidence-array shape while hard-coding `qualification_status=not_run` and `execution_enabled=false`. Unknown profile revisions and references to absent, rejected, uploading, or hash-mismatched MissionInputs are refused. Direct SQL insertion of missing profile identity or a null evidence array is rejected by the database.
- The Workbench exposes company-scoped GET and POST endpoints and a Group Settings “领域验收” panel. It displays the distinct content/research evidence requirements, allows incomplete records, and keeps execution visibly closed. Fixture mode is read-only.
- The Desktop session-token middleware requires a session token for both the domain ledger read and evidence submission routes, including loopback tokenless browser mode.

## Verification results

| Command / evidence | Result |
|---|---|
| `rtk proxy bash scripts/r3-domain-evidence-postgres-test.sh --race` | PASS. The same database integration tests passed with Go race detection. |
| Workbench PostgreSQL integration test | PASS. The actual HTTP response contains camelCase `submission.profileId` and `evidence: []`; the front-end DTO validator accepts it. |
| `rtk proxy bash scripts/go.sh test ./internal/desktop -run TestDomainEvidence -count=1` | PASS. Both domain evidence routes are marked token-protected. |
| `rtk proxy bash scripts/go.sh test ./internal/desktop -run TestTokenlessMiddlewareRejectsDomainEvidenceEndpointsEvenOnLoopback -count=1` | PASS. Tokenless loopback requests to GET and POST receive 401. |
| `rtk proxy bash scripts/go.sh test ./... -count=1` | PASS. Database integration tests without an explicitly configured DSN were skipped; the dedicated Schema 29 database tests above were run separately. |
| `rtk proxy npm --prefix frontend test` | PASS. 73 tests. |
| `rtk proxy npm --prefix frontend run lint` | PASS. |
| `rtk proxy npm --prefix frontend run build` | PASS. Vite reported a 558.07 kB JavaScript chunk-size advisory. |
| `rtk proxy bash scripts/go.sh build ./cmd/...` | PASS for Linux. |
| `GOOS=windows GOARCH=amd64 bash scripts/go.sh build ./cmd/...` | PASS. |
| `rtk proxy git diff --check` | PASS. |

No real content/research data, provider, QQ, MCP endpoint or production database was used. The evidence screen records references and readiness only; it does not validate domain quality or create a human qualification decision. Both profiles remain `not_run` and execution-disabled pending independent domain evidence and review.

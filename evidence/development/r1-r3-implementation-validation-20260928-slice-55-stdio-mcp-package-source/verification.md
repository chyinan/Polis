# Slice 55 verification: controlled stdio MCP package source import

Date: 2026-09-28. Development schema: 44.

## Implemented

- Added the `polis-controlled-stdio-mcp@1` ZIP profile with strict JSON decoding, duplicate-key and unknown-field rejection, UTF-8 validation, bounded safe paths, immutable sorted file hashes, and an order-independent manifest digest.
- Added Schema 44 `mcp_server_package_revisions`. The DB trigger verifies each revision still matches its immutable stdio server definition; update, delete and truncate are blocked, and downgrade refuses to discard package history.
- Added server-side package import. A new import atomically creates an unverified MCP definition plus package revision. An update binds to an existing server and requires exact name/command/args match. Files go to company CAS, are read back, and are reverified before the transaction. An advisory lock serializes each company/server/revision before CAS writes; idempotent replay returns the same package, and a losing concurrency attempt does not leave its unique CAS blob.
- Added the authenticated Workbench multipart route and capability catalog projection. The form accepts an optional server ID, revision, request ID and ZIP; it sends no client digest. The React capability panel can create or update a package revision and displays its server version, digest and file count.
- Package import does not execute the command, perform runtime observation, approve the server, or bind it to an Employee.

## Verification

- `rtk bash scripts/go.sh test ./internal/capabilitysource -count=1` — PASS.
- `rtk bash scripts/r1-capability-source-postgres-test.sh` — PASS at Schema 44. Kernel/Control/Workbench package integration includes CAS readback, cross-company denial, immutable history, request replay, descriptor matching and concurrent same-revision conflict without orphan CAS content.
- `rtk bash scripts/r2-cross-backend-handover-postgres-test.sh` — PASS at Schema 44.
- `rtk bash scripts/r2-recovery-backup-postgres-test.sh` — PASS at Schema 44.
- `rtk bash scripts/r3-domain-evidence-postgres-test.sh` — PASS at Schema 44; content and research profiles remain `not_run` and execution-disabled.
- `rtk powershell.exe -NoProfile -File scripts/test-migration-hash-manifest.ps1` — PASS.
- `rtk bash scripts/go.sh build ./cmd/...` and Windows amd64 cross-build of `./cmd/...` — PASS.
- `rtk npm --prefix frontend test` — PASS, 101 tests. `npm run typecheck`, `npm run lint`, and `npm run build` — PASS. Vite reports the existing >500 kB bundle advisory.
- No local MCP command, provider model turn, QQ send, GitHub account or production operation ran.

## Remaining R1 and later-stage work

- Add the explicit runtime-observation action: revalidate approval before launch, materialize CAS source bytes under the deny-all AppContainer, confirm process-tree stop, persist the observed tool schema, and keep runtime approval/binding separate. Real provider dispatch remains denied.
- Recover an owner after parent process crash and qualify Windows AppContainer/WFP on a clean VM; code/cross-build evidence does not qualify the host.
- R2 Linux/Node restart recovery and delegated-host qualification remain open. R3 domain profiles remain unavailable until separate real quality, recovery, cost, organization-benefit and (for content operations) intervention evidence is reviewed.

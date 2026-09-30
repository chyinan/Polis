# R1–R3 continuation verification — slice 2

Date: 2026-09-24

Workspace: isolated worktree `C:\Users\chyinan\.codex\worktrees\polis-r1-r3-implementation\Polis`.

## Passed

- Dedicated PostgreSQL 18 database `polis_r0_envjobs_20260924` migrated incrementally from Schema 22 to Schema 23 with `00023_zip_mission_inputs.sql`. No reset or drop was performed.
- `TestEnvironmentPreparationPersistsPolicyAndIsolationBlocksIdempotently` passed on Schema 23. It verifies directory and root-level ZIP source bindings, server-derived package/lock hashes, policy approval, and the executor-qualification gate.
- `TestZIPInputIsDeliveredToWorkerAndReadBackPerFile` passed on Schema 23. The fake Worker context includes the README path and sentinel, excludes a binary sibling, records per-file receipt metadata, and reads it back through Workbench; provider egress is zero.
- `TestWorkbenchMissionInputUploadStoresAndListsWithoutStartingMission` passed on Schema 23. The real multipart Workbench path stores and lists both a text input and a ZIP source without starting the Mission.
- Intake ZIP tests pass for valid/partial source classification, path traversal, absolute paths, case-insensitive collisions, symlinks, nested archives, expansion limits, and the Windows `application/x-zip-compressed` MIME alias.
- R3 `internal/domainworkflow` tests pass for draft-version invalidation, independent content review, required source references, explicit inconclusive claims, pinned research dataset/method/control/risk, simulation-only execution, and accepted negative results.
- Full offline `rtk bash scripts/go.sh test ./... -count=1` passed with database DSNs unset. PostgreSQL cases are covered separately above.
- Linux `rtk bash scripts/go.sh build ./cmd/...` and Windows amd64 `GOOS=windows GOARCH=amd64 ... build ./cmd/...` passed.
- Frontend tests: 48 passed. `npm run typecheck` and `npm run lint` passed. `npm run build` passed; Vite reports the main JS chunk is about 520 KB, above its 500 KB advisory threshold.

## Remaining gates

- No Windows filesystem/network isolation is qualified; ZIP project data is validated from CAS but is not materialized or executed. `npm ci`, project commands, services and browser jobs were not run.
- No Skill script or MCP process/endpoint ran. QQ, GitHub, PDF, Linux/Node, cross-backend, multi-day recovery, backup/restore, and desktop-session browser download qualification remain open.
- R3 content and research contracts are local reference profiles only. Both remain `not_run` and execution-disabled; no domain output, user-value, quality, recovery, cost or organization claim is qualified.
- No real model turn, QQ send, external MCP call, GitHub account access, production action or business account was used.

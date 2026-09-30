# Slice 76 verification — persisted R3 research simulation

Date: 2026-09-29

## Implemented boundary

- Schema 54 adds an append-only, company-scoped simulation run ledger and verifies exact usable MissionInput source revisions, digests, media types, and size bounds at insert. Forward Schema 55 enforces numeric iteration/sample bounds and `risk_consumed_units = 2 × iterations × sample_size`.
- The Kernel binds reads to exact bounded CAS bytes, validates the deterministic simulation receipt and risk equation, persists the output digest and returns verified database readback.
- A company/request session advisory lock spans replay lookup, deterministic simulation and persistence. Identical concurrent retries return the same receipt; changed parameters conflict. The risk budget is capped at 5,120,000 sample draws.
- The authenticated Workbench selects exact current-mission input revisions and displays output as estimates only. Fixture mode denies simulation writes. Domain profile qualification remains `not_run`; execution remains disabled.
- No model, provider, external source, account, publication, trading, or network endpoint was used.

## Verification run

- `rtk bash scripts/r3-domain-evidence-postgres-test.sh` — passed on a dedicated temporary PostgreSQL 18 cluster. The script migrated from Schema 39 through Schema 55, reran the R3 domain evidence and Workbench routes, and passed research simulation persistence, exact-source binding, deterministic replay, six simultaneous identical retries returning one run, changed-request conflict, over-budget refusal, company isolation, rejection of a directly inserted underreported receipt, ledger readback, and no-qualification checks.
- `rtk bash scripts/go.sh test ./internal/control ./internal/domainworkflow ./internal/kernel ./internal/workbench -run ResearchSimulation -count=1` — passed before the disposable database run.
- `rtk bash scripts/go.sh test ./... -count=1` — passed after both review fixes.
- `rtk proxy powershell -NoProfile -Command "Set-Location frontend; npm run test -- --run"` — passed, 121 tests across 13 files.
- `rtk proxy powershell -NoProfile -Command "Set-Location frontend; npm run lint"` — passed.
- `rtk proxy powershell -NoProfile -Command "Set-Location frontend; npm run build"` — passed, including TypeScript project build and Vite production bundle. Vite emitted its existing advisory that the main minified chunk exceeds 500 kB.
- `rtk bash scripts/go.sh build ./cmd/...` — passed for Linux amd64.
- `rtk bash -c 'GOOS=windows GOARCH=amd64 bash scripts/go.sh build ./cmd/...'` — passed for Windows amd64.
- `rtk proxy powershell -NoProfile -File scripts/test-migration-hash-manifest.ps1` — passed.
- `rtk git diff --check` — passed.
- Independent Slice76 re-review confirmed both concurrency and risk-accounting findings fixed; no new scoped findings. The reviewer made no changes and ran no tests or database/external operations.

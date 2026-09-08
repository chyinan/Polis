# Polis

**Polis — A runtime for persistent autonomous AI organizations.**

Build AI organizations that outlive their models.

Polis｜长期自治 AI 组织运行时

This checkout implements one R0 kernel experiment based on Draft 0.4.5. A local CLI explicitly starts a mission; fixed planning/backend FakeWorkers collaborate through persisted messages and responsibility; a separate deterministic reviewer checks an immutable arithmetic artifact. Four employee profiles survive controller restarts. No model calls are implemented.

## Run in the prepared workspace

From Windows PowerShell:

```powershell
rtk proxy wsl -d Ubuntu-22.04 -u root -- bash /mnt/d/Programs/Polis/scripts/prepare-linux-storage.sh
rtk proxy wsl -d Ubuntu-22.04 -- bash /mnt/d/Programs/Polis/scripts/start-test-pg.sh
rtk proxy wsl -d Ubuntu-22.04 -- bash /mnt/d/Programs/Polis/scripts/test.sh
rtk proxy wsl -d Ubuntu-22.04 -- bash /mnt/d/Programs/Polis/scripts/demo.sh
```

The first command mounts this workspace's ext4 image; it does not change WSL configuration. Skip the PG start command while the dedicated instance is already running. Tests create unique `polis_r0_test_*` databases, migrate explicitly with management credentials, run with a non-superuser role, then drop only their own database. The demo likewise drops its own database and preserves its JSON evidence and small artifact file. It does not connect to business databases.

From WSL in `/mnt/d/Programs/Polis`:

```sh
POLIS_TEST_RACE=1 bash scripts/test.sh
bash scripts/go.sh test ./internal/core
bash scripts/go.sh vet ./...
bash scripts/go.sh build ./cmd/...
.tools/bin/sqlc generate
```

`go test ./...` without `POLIS_TEST_DSN` explicitly skips PG tests; this is not a PG pass. Use `scripts/test.sh` for the complete integration entry. Go's race detector needs the installed WSL gcc/libc headers.

## Concrete commands

`polis migrate` uses `POLIS_DSN` for a dedicated R0 database and is an explicit management operation. Business startup does not apply migrations. Runtime `POLIS_DSN` must identify a non-superuser role on PostgreSQL 18, and `POLIS_BLOB_ROOT` must identify this installation's private artifact root.

- `polis create COMPANY MISSION`: create a fixed roster and draft mission, without running workers.
- `polis start COMPANY MISSION`: persist a unique bootstrap task.
- `polisd --company COMPANY --steps 8`: execute up to eight fake boundaries, returning immediately when idle. Restarting it reconstitutes pending work.
- `polis status COMPANY MISSION`: return a read-only repeatable-read snapshot; does not take controller ownership or change epoch.

The CLI is a trusted local OS management interface. No worker network API, web login or general command executor exists. `polisd` deliberately exits at the bounded slice boundary. The mission retains its slot after task acceptance; full mission closing/terminal lifecycle is a later slice, not a hidden success claim.

## Evidence and limits

See `docs/implementation/PROGRESS.md`, `STARTUP_REVIEW.md` and `SLICE_TESTS.json` for actual results, versions, safety applicability and the next ticket. Raw evidence is under `evidence/development/`. `spec/design-v0.4.5/` and `spec/archives/` preserve the baseline.

Process-kill recovery is tested. Host power loss, OS old-writer isolation, real model handover, money ceilings, full RLS qualification, downloads/GC, MCP, QQ, UI and organization value are not demonstrated. The production Go binary uses PG only; Python is limited to documentation checks and development tooling.

Stop the temporary database with `rtk proxy wsl -d Ubuntu-22.04 -- bash /mnt/d/Programs/Polis/scripts/stop-test-pg.sh`. The image may stay mounted for Go caches; no daemon is installed to start it automatically.

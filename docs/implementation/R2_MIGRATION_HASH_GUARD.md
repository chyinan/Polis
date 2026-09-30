# R2 migration integrity and execution ledger

## Contract

`db/migration_hashes.sha256` pins the exact embedded Goose SQL file set. `polis migrate` verifies that every embedded migration has one sorted manifest entry and that the SHA-256 of each complete SQL file matches before it parses the database connection string or opens a connection. Missing, extra, reordered, malformed or modified migration files fail closed.

The check covers both Goose Up and Down text, so edits to historical migration files are detected. The updater reads existing pins and refuses a missing manifest, malformed or unsorted entries, renamed/deleted pinned migrations, changed bytes for pinned files, duplicate Goose versions and new versions that are not greater than the highest pinned version. It only appends hashes for new forward SQL files.

For an intentionally empty repository that is establishing its first baseline, initialization must be explicit with `-InitializeManifest`. The checked-in Polis repository already has a manifest; normal updates must not use this switch.

Add a new migration as a new numbered SQL file, review it, then regenerate the manifest:

```powershell
powershell -ExecutionPolicy Bypass -File scripts/update-migration-hash-manifest.ps1
```

Review the manifest diff together with the new migration. Existing hashes stay fixed; the updater cannot legitimize edits to a migration already pinned in a supported build. Add a new forward migration instead.

## Applied execution ledger

Schema 40 adds an append-only `migration_execution_evidence` row for every applied migration. Each row records the version and source hash, the build that recorded the evidence, result, and time. `applied_during_this_run` means Goose successfully applied that version in the current `polis migrate` invocation. `observed_preexisting` means it was already in Goose history before its ledger row was recorded; it does not claim the actual historical applying build. `polis migration-status` reads these rows.

Before Goose runs, Migrate checks existing rows against the current embedded source set and applied Goose versions, pins `search_path=public`, and serializes callers both within the process and with a PostgreSQL advisory lock. Schema 40 or later with a missing ledger is a hard integrity error. Missing tail rows may be recovered after an interrupted ledger write; a missing row before a later recorded version, mismatched hash, unknown source version or non-applied version blocks migration. Ledger additions are transactional. INSERT is limited to the table owner; UPDATE, DELETE and TRUNCATE are rejected. `polis migrate-to VERSION` supports forward staging and refuses downgrade; Schema 40 has a guarded Down migration.

The database ledger records applied migration versions. A separate filesystem attempt journal records the CLI invocation around the database operation; it does not alter Schema 40 or rewrite historical ledger evidence. `polis migrate` and `polis migrate-to VERSION` fail closed unless `POLIS_MIGRATION_ATTEMPT_ROOT` is configured or `POLIS_BLOB_ROOT` can provide a sibling `migration-attempts` directory. `scripts/go.sh` provides an isolated default under its workspace-local `GOTMPDIR` for local checks. The canonical journal path must not overlap the CAS root, including through symlinked ancestors.

Each attempt uses exclusive immutable `started` and `finished` JSON files. The records contain a random attempt ID, target version, migration manifest digest, Go build identity, timestamps, and a bounded result/failure code; they do not contain a DSN or raw database error. A start without a finish is reported as `outcome_unknown`, and unknown files/directories in the journal fail closed. On Unix the writer syncs each file and its containing directory, including the parent entry when ensuring the journal root. On Windows it syncs file content and publishes same-volume temporary files/directories using `MoveFileEx` with `MOVEFILE_WRITE_THROUGH`. `polis migration-attempt-status` reads the journal. The Windows path was cross-compiled but not run with a native Windows Go runtime.

The journal records attempt outcomes but does not create a pre-upgrade recovery backup, switch the active desktop database/CAS root, or roll back the installed binary. Those remain required R2 upgrade and recovery work.

## Verification

The updater test also covers missing manifest, backward/duplicate/zero Goose versions, uppercase extensions and explicit initial seeding. The temporary Schema 39→40 script verifies ledger backfill, current build/result, idempotent replay, evidence immutability, runtime-role insert denial, downgrade refusal, CLI status and search-path pinning. It tampers with and then removes ledger state in the disposable database to verify Migrate fails closed before any further Goose operation.

The PowerShell updater regression uses a temporary repository root. It proves modified pinned SQL, backward/duplicate/zero Goose versions, uppercase `.SQL` extensions and a missing manifest are rejected without rewriting or bootstrapping the pins; a new higher forward version is admitted while prior hashes remain unchanged.

Pure tests cover a correct manifest and changed, missing, extra, malformed and unsorted entries. An integration-level unit test tampers with the in-memory manifest and passes an invalid DSN; it verifies that `Migrate` rejects the hash drift before attempting DSN parsing or opening a database. A dedicated temporary PostgreSQL 18 migration script exercises the valid manifest through the real `polis migrate` entry point.

# R2 recovery backup and restore

## Backup package

`polis recovery-backup OUTPUT_DIRECTORY` creates a generation-named immutable directory containing a PostgreSQL custom-format dump, the full company-scoped CAS tree, a sorted SHA-256 CAS inventory, and a completion manifest. It first acquires the same exclusive PostgreSQL advisory lock used by the Polis control plane and refuses to back up while WorkerSessions, JobRuns, environment preparations, MissionInput uploads, service leases, or notification sends are active. The lock remains held through `pg_dump` and CAS capture. The database dump is taken with `pg_dump`'s consistent snapshot. The manifest records database owner/runtime role and installed extension name/version/schema metadata without credentials. CAS files are append-only/content-addressed in the current implementation; the CAS inventory is captured after the dump and includes a superset of the dump's referenced blobs. A future CAS garbage collector must synchronize with this backup protocol before deleting blobs.

The package records the source database name, PostgreSQL major, Goose schema version, creation time, file hashes and sizes. It omits database credentials and provider/QQ/GitHub/MCP secrets. The output parent and source CAS root must be separate existing directories. Backup writes to a private temporary sibling and publishes with an atomic rename only after the manifest, database dump and every CAS blob verify.

Commands:

```powershell
$env:POLIS_BACKUP_DSN = "<administrator DSN with full database read access>"
$env:POLIS_BLOB_ROOT = "<source CAS root>"
$env:POLIS_PG_DUMP_PATH = "<PostgreSQL 18 pg_dump path>"
$env:POLIS_RUNTIME_ROLE = "polis_runtime"
polis recovery-backup "<backup output parent>"
polis recovery-backup-verify "<generated polis-recovery-* directory>"
```

If `POLIS_BACKUP_DSN` is unset, `polis recovery-backup` uses `POLIS_DSN`. PostgreSQL passwords are passed to the client through a private temporary pgpass file; the client argument list contains only a generated service name.

## Restore

`polis recovery-backup-restore PACKAGE_DIRECTORY BLOB_ROOT` restores only to an already-created, empty target database. The target PostgreSQL major must match the package. The restore connection must own the target database or be a superuser, the recorded PostgreSQL extensions must be available at the recorded versions, and the configured runtime role must exist in the target cluster. Restore uses a single PostgreSQL transaction, never drops or replaces a database, never overwrites a CAS root, replays the package's exact migration version, and grants the selected runtime role bounded application DML access. Schema 40's migration execution ledger is carried inside the database dump; restore verifies its own package and CAS but does not rewrite the original migration evidence. The CAS root is copied to a new directory; an adjacent generation marker allows a verified retry after an interruption. A database comment records successful completion so retrying the same package is idempotent. A different package or an unmarked non-empty target is refused.

```powershell
$env:POLIS_RESTORE_DSN = "<administrator DSN connected to the empty target database>"
$env:POLIS_RUNTIME_ROLE = "polis_runtime"
$env:POLIS_PG_RESTORE_PATH = "<matching PostgreSQL 18 pg_restore path>"
polis recovery-backup-restore "<verified package directory>" "<new CAS root>"
```

Restore does not overwrite the active service configuration. Slice 84 adds a Tauri Desktop maintenance cutover that stages a verified generation, rejects new Workbench requests, drains admitted requests, checks active work, and switches only after candidate startup and health succeed. It retains one previous generation for explicit rollback and restores the previous pointer when candidate startup fails. The packaged Windows PostgreSQL runtime is absent from the current checkout, so this cutover remains software-verified and unqualified on the native Desktop runtime. Restore does not include provider credentials, OS isolation policy, WFP state or external service accounts. The commands do not contact providers or perform database cleanup. Cross-backend Task handover, Task workspace CAS rehydration for a fresh WorkerSession, and Linux WorkerSession cgroup v2 stop/restart recovery have offline software paths. An `outcome_unknown` project JobRun is never replayed and its mutable filesystem writes are not restored. Linux host qualification, proof that the Provider Worker cannot migrate out of its cgroup, a real multi-day run, and local sidecar update/rollback remain open.

## Verify a restored generation before cutover

`polis recovery-generation-verify PACKAGE_DIRECTORY BLOB_ROOT` is a read-only preflight for a restored target. Set `POLIS_GENERATION_DSN` to a connection string for the target database; the DSN is not accepted on the command line. The verifier checks the package, the database restore comment, the adjacent CAS restore marker, the restored schema version, and the exact CAS path/size/hash inventory. Both restore markers must match the same package generation and manifest digest. Database connection failures are returned as static messages without the DSN.

The command itself does not freeze writers or activate a target. Slice 84 calls it inside the Desktop maintenance switch after request admission is closed and existing requests drain. The prior generation is retained until candidate startup and health checks pass. Schema 60 verification and the R2 PostgreSQL round trip are recorded in `evidence/development/r1-r3-implementation-validation-20260930-slice-83-restored-generation-verification/verification.md`.

## Qualification boundary

The implementation is tested with dedicated PostgreSQL 18 temporary source/target databases and a temporary CAS root. It verifies backup denial while the Polis advisory lock is owned or a MissionInput remains uploading, a Mission and CAS-backed MissionInput across backup/restore, role/extension inventory, same-package idempotent retry, rejection of another package generation, and tamper/extra-blob refusal. It does not qualify a production database, a remote PostgreSQL deployment, OS-level secrets, host backups or a full application cutover.

# Slice 140 — installation-wide observed ProviderAccount registry

Date: 2026-10-03

## Delivered

- Schema 95 adds an immutable provider-account registry and immutable WorkerSession observation links, keyed by provider class and the existing SHA-256 locator fingerprint. Existing `available` snapshots are backfilled.
- New available snapshots create their registry entry and observation in the same guarded transaction, before provider execution. The Company lifecycle row is acquired before the global registry key; future transactions that need both scopes must preserve that order.
- `InstallationOwnerScope` is bound to the active Kernel incarnation. A read-only Workbench endpoint returns the locator fingerprint, first/last observed timestamps, and aggregate Company/session counts. The route is no-store and requires the desktop middleware to authenticate the configured management token; tokenless requests cannot access the installation path.
- This registry records provider observations only. It does not prove billing scope, establish a usage source, reserve or settle financial liability, or enable a ProviderAccount cap. The shared desktop service token is not the FT-63 first-owner initialization and individual owner-session system.

## Verification

| Check | Result |
| --- | --- |
| Go build: kernel, control, Workbench, desktop and `cmd/polis` | PASS |
| `sha256sum -c db/migration_hashes.sha256` from `db/migrations` | PASS; all 95 migrations match the manifest |
| `git diff --check` | PASS |
| Tests | Not run |
| PostgreSQL migration/runtime | Not run |
| Provider execution, workspace queries, or live usage/cost | Not run |

The PostgreSQL and external account qualifications require the relevant disposable database and owner-authorized account/workspace; this stage did not use either.

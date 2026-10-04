# Slice 219 — REQ-39 immutable workspace-tree lease binding

Schema 105 adds an all-or-none immutable workspace-tree binding to each Task takeover lease: private root ID, tree revision, canonical `polis-workspace-snapshot@1` manifest SHA-256, file count, and total bytes. The grant path builds this manifest under a root-row lock, checks its file entries and CAS bytes against the existing workspace bounds, and records the binding with the lease. Handback recomputes the binding under the same root-row lock and rejects any changed root, revision, manifest, count, or byte total. Legacy Tasks without a Schema 103 root keep null binding columns. The prior single-file `workspace.txt` baseline condition remains in force, so this slice does not yet accept multi-file trees.

## Verification

- `go build ./...` passed.
- `git diff --check` passed.
- Every SQL migration checksum in `db/migration_hashes.sha256` matched its migration source, including Schema 105.
- No tests were run. Schema 105 was not applied to any database. No takeover lease, WorkerSession, provider activity, or frozen scenario was run.

REQ-39 remains partial. The next implementation stage must expose lease-bound reads of the frozen tree, then accept and validate a bounded multi-file handback package before changing the current single-file UI path.

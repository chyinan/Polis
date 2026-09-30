# R2 ZIP input foundation

> Updated: 2026-09-24. ZIP upload, bounded text extraction, Task Worker delivery, and Node source inspection are locally verified. This does not qualify the full R2 input profile.

## Behavior

- Schema 23 admits `zip_snapshot` MissionInput revisions. The original ZIP bytes and digest remain in company CAS and Task manifests bind that exact revision.
- ZIP parsing happens in memory and never joins an entry path to a host filesystem path. Limits are 8 MiB compressed input, 512 central-directory entries, 250 files and 7 MiB expanded bytes.
- The parser rejects absolute/traversing or colliding paths, Windows-reserved names, links/special entries, nested archives, empty files, malformed data, size/CRC mismatch and over-limit archives.
- UTF-8 text entries with supported extensions can enter the existing 8-file/16 KiB each/64 KiB total Worker text context. Unsupported siblings get per-file exclusion receipts. Invalid CSV/text representations are not reclassified as supported text.
- `ProjectEnvironmentRevision` may bind to the exact ZIP MissionInput revision, including archives with a top-level project directory or files directly at the archive root (recorded as `.`); package and lockfile hashes are derived from the verified archive contents and checked again at policy approval/preparation.

## Verification

- Intake tests cover valid/partial source classification, traversal, absolute paths, case-insensitive duplicates, symlinks, nested archives and expanded-size limits.
- Windows' `application/x-zip-compressed` browser MIME alias is accepted when the `.zip` extension and archive structure validate.
- `TestZIPInputIsDeliveredToWorkerAndReadBackPerFile` passed on the dedicated PostgreSQL 18 database `polis_r0_envjobs_20260924` at Schema 23. It verifies the README sentinel/path reaches the local fake Worker context, the binary sibling is excluded, and Workbench reads the same per-file receipt. Provider egress was zero.
- `TestEnvironmentPreparationPersistsPolicyAndIsolationBlocksIdempotently` also registers a Node project from ZIP CAS and checks the stored source reference, root and derived package/lock hashes.
- Frontend tests, typecheck, lint and production build passed after adding `.zip` to the Mission input picker.

## Limits

ZIP is not a general file extraction facility. It does not install dependencies or run project files. Actual R1 project materialization remains gated on Windows filesystem/network isolation. PDF, Git, GitHub feedback, Linux/Node and full R2 qualification remain open.

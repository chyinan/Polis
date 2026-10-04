# REQ-29 authorized shared-file access audit

Updated: 2026-10-04, Slice 208. This source map leaves the frozen v0.4.5 design unchanged.

## Frozen requirements checked

Architecture section 3.1 describes REQ-29 as autonomous access to authorized workspaces and shared files, with read/write/snapshot separation and correct paths and versions. CAP-01–06 in section 44.10 cover self-discovery, path/link/special-file containment, shared versus private visibility, single-writer epochs, consistent reads, and ready/obsolete/GC snapshot handling.

## Current implementation

- `db/migrations/00002_workers.sql` still defines the compatibility workspace as one digest and revision per Task. Schema 103 (`db/migrations/00103_workspace_tree_snapshots.sql`) adds a separate logical private Task root, relative UTF-8 file records with per-file source/file revisions, and immutable snapshot file references. It does not mount or expose host paths.
- The product Worker registry in `internal/codex/tools.go` adds `mission_artifacts_list` and `mission_artifact_read` only on the separately versioned fake-only @8 surface. The existing @4 and @7 surfaces are unchanged.
- `internal/kernel/shared_mission_artifacts.go` derives scope from the active database-bound WorkerSession and owned working Task. Listing/read are limited to ready candidate/passed Artifacts from a different Task in the same Mission; content is read by Artifact ID, bounded, checked against digest and byte size, and required to be UTF-8. Reads persist immutable Artifact/version and session provenance.
- The opt-in fake-only @10 Worker surface adds `workspace_files_list`, `workspace_files_search`, `workspace_file_read`, `workspace_file_write`, `workspace_file_delete`, `workspace_snapshot`, `workspace_snapshot_read`, and `workspace_snapshot_file_read`. Real-provider authorization remains pinned to @4. The 10-tool registry is immutable and independently identified by its manifest/schema digest; this slice did not execute the Fake surface.
- `internal/kernel/workspace_tree.go` binds each tree to its Task, Mission, owner, private access class, root revision and one active WorkerSession/epoch. Relative paths reject traversal, absolute paths, ambiguous URI encodings, backslashes and control characters. Text content is bounded and checked as UTF-8. Reads/search/list are finite and append access provenance; writes/deletes use root-revision compare-and-swap. Stored objects are company CAS blobs, not host filesystem entries.
- Snapshot creation locks the tree revision and publishes a manifest as a ready `workspace_snapshot` Artifact with verdict `draft_not_accepted`; a per-file reference table retains each immutable CAS object. Direct reads require an exact Artifact ID and a current same-Mission WorkerSession. Existing deliverable and memory-target paths filter this new Artifact kind. Generic CAS collection scans company-owned tables, and startup recovery verifies each ready snapshot manifest and file object.
- `formatter.go` remains synchronized with the legacy `worker_workspaces` row so the existing single-file validation/acceptance path stays usable. An existing Task can pass its old acceptance gate only while its tree contains exactly that file; multi-file trees fail closed at that boundary.

## Remaining gap

REQ-29 is still partial. The logical tree supports only private Task-scoped UTF-8 files. It does not implement actual host mounts, OS-level symlink/special-file/TOCTOU controls, company shared documents, a group public library, or explicit snapshot obsolete/revocation policy. Snapshots are durable direct-ID draft Artifacts with retained file references; owner policy for lifecycle/retention and all applicable full CAP qualification remain open.

CAP-01–06 remain `not_run` as frozen scenarios. Slice 208 performed source/build verification only: no tests, migration application, populated WorkerSession end-to-end, Worker/provider execution or external operation occurred. The current environment has not supplied a database-confirmed active WorkerSession for qualification.

## History

Slice 150 identified the missing cross-Task Artifact read. Slice 151 implemented that bounded read path on @8, and Slice 208 added the logical private Task tree and immutable draft snapshot model on @10. This document supersedes the Slice 150 statement that no Worker tool can discover or read another Task's published Artifact; that statement was accurate only before Slice 151. See `evidence/development/r1-r3-implementation-validation-20261003-slice-151-req29-shared-artifact-read/verification.md` and `evidence/development/r1-r3-implementation-validation-20261004-slice-208-req29-workspace-tree/verification.md`.

# REQ-29 authorized shared-file access audit

Updated: 2026-10-06, Slice 264. This source map leaves the frozen v0.4.5 design unchanged.

## Frozen requirements checked

Architecture section 3.1 describes REQ-29 as autonomous access to authorized workspaces and shared files, with read/write/snapshot separation and correct paths and versions. CAP-01–06 in section 44.10 cover self-discovery, path/link/special-file containment, shared versus private visibility, single-writer epochs, consistent reads, and ready/obsolete/GC snapshot handling.

## Current implementation

- `db/migrations/00002_workers.sql` still defines the compatibility workspace as one digest and revision per Task. Schema 103 (`db/migrations/00103_workspace_tree_snapshots.sql`) adds a separate logical private Task root, relative UTF-8 file records with per-file source/file revisions, and immutable snapshot file references. It does not mount or expose host paths.
- The product Worker registry in `internal/codex/tools.go` adds `mission_artifacts_list` and `mission_artifact_read` only on the separately versioned fake-only @8 surface. The existing @4 and @7 surfaces are unchanged.
- `internal/kernel/shared_mission_artifacts.go` derives scope from the active database-bound WorkerSession and owned working Task. Listing/read are limited to ready candidate/passed Artifacts from a different Task in the same Mission; content is read by Artifact ID, bounded, checked against digest and byte size, and required to be UTF-8. Reads persist immutable Artifact/version and session provenance.
- The opt-in fake-only @10 Worker surface adds `workspace_files_list`, `workspace_files_search`, `workspace_file_read`, `workspace_file_write`, `workspace_file_delete`, `workspace_snapshot`, `workspace_snapshot_read`, and `workspace_snapshot_file_read`. Real-provider authorization remains pinned to @4. The 10-tool registry is immutable and independently identified by its manifest/schema digest; this slice did not execute the Fake surface.
- The separate fake-only @11 surface adds `workspace_snapshot_revoke`; @10 and real-provider authorization pinned to @4 remain unchanged. The @11 registry is immutable and independently identified by its manifest/schema digest; no Worker surface was executed.
- `internal/kernel/workspace_tree.go` binds each tree to its Task, Mission, owner, private access class, root revision and one active WorkerSession/epoch. Relative paths reject traversal, absolute paths, ambiguous URI encodings, backslashes and control characters. Text content is bounded and checked as UTF-8. Reads/search/list are finite and append access provenance; writes/deletes use root-revision compare-and-swap. Stored objects are company CAS blobs, not host filesystem entries.
- Snapshot creation locks the tree revision and publishes a manifest as a ready `workspace_snapshot` Artifact with verdict `draft_not_accepted`; a per-file reference table retains each immutable CAS object. Direct reads require an exact Artifact ID and a current same-Mission WorkerSession. Schema 104 adds an immutable revocation receipt requiring the current source owner session/epoch and atomically changes the Artifact to `revoked`; later manifest and file reads are denied, prior read content is not recalled, and CAS bytes are retained. Existing deliverable and memory-target paths filter this Artifact kind. Generic CAS collection scans company-owned tables, and startup recovery verifies each ready snapshot manifest and file object while skipping revoked snapshots.
- `formatter.go` remains synchronized with the legacy `worker_workspaces` row so the existing single-file validation/acceptance path stays usable. An existing Task can pass its old acceptance gate only while its tree contains exactly that file; multi-file trees fail closed at that boundary.

## Remaining gap

REQ-29 is still partial. The logical tree supports only private Task-scoped UTF-8 files. It does not implement actual host mounts, OS-level symlink/special-file/TOCTOU controls, company shared documents, or a group public library. Revocation now denies future direct-ID reads, but it does not recall prior content or reclaim CAS objects. Retention/GC policy and all applicable full CAP qualification remain open.

CAP-01–06 remain `not_run` as frozen scenarios. Slice 211 performed source/build verification only: no tests, migration application, populated WorkerSession end-to-end, Worker/provider execution or external operation occurred. The current environment has not supplied a database-confirmed active WorkerSession for qualification.

## History

Slice 150 identified the missing cross-Task Artifact read. Slice 151 implemented that bounded read path on @8, Slice 208 added the logical private Task tree and immutable draft snapshot model on @10, and Slice 211 added audited source-owner snapshot revocation on @11. This document supersedes the Slice 150 statement that no Worker tool can discover or read another Task's published Artifact; that statement was accurate only before Slice 151. See `evidence/development/r1-r3-implementation-validation-20261003-slice-151-req29-shared-artifact-read/verification.md`, `evidence/development/r1-r3-implementation-validation-20261004-slice-208-req29-workspace-tree/verification.md`, and `evidence/development/r1-r3-implementation-validation-20261005-slice-211-req29-workspace-snapshot-revocation/verification.md`.

## Local source audit (2026-10-06; based on source commit `83592318bc49b6f717d68054da7b213a1fbc63af`)

This audit checked the production Go/SQL paths and the frozen CAP-01–06 contract. It made no code or schema change. The migration source is Schema 110; this audit did not query or modify the local database.

| Area | Present source behavior | Concrete missing contract |
|---|---|---|
| Host roots and mounts | `companies.workspace_root` is saved, returned, and shown by the Workbench, but no runtime authorizes it or resolves it to an OS root. Company creation also accepts it as metadata. Linux Node preparation and native Worker launch bind fresh application-managed directories; Windows AppContainer workspaces are likewise application-managed. The Worker logical tree uses Company CAS and relative paths. Git import reads a locally available, selected commit into an immutable snapshot and does not write the source repository. | No owner-authorized root handle/bookmark, canonical host identity, binding revision, read/write capability, revoke path, or Worker mount adapter exists. A path string is not authority. Physical symlink/special-file/TOCTOU protections for a mounted user root therefore do not exist. |
| Company and Group sharing | Cross-Task published Artifact reads are bounded to another Task in the current active Mission and same Company, and record the exact digest/version and reader session. Workspace snapshot reads are exact-Artifact-ID, same-Mission reads. Neither is a Company-wide shared document root. There is no `groups` table or group membership/resource relation in the migrations. `skill_revisions.publisher_scope='group'` is still keyed by `company_id`; that enum does not create a cross-Company Group identity. | No Company shared-root ACL, Group entity/membership, cross-Company authorization join, revisioned sharing decision, or public-library visibility model exists. Employees in a Company do not gain a new root merely from their Company membership. |
| Snapshot retention | Task snapshots are immutable `draft_not_accepted` Artifacts with per-file CAS references. Revocation is owner/session/epoch audited and blocks later direct reads; already returned bytes are not recalled. The generic CAS collector defaults to dry-run and, on apply, serializes with CAS writers/company writes and deletes only digest files that no company-owned database row references. | There is no snapshot expiry/retirement policy, owner decision record, retention class, deletion/tombstone lifecycle, or reference-release operation. `workspace_snapshot_files` and Artifact history continue referencing the objects, so generic orphan collection cannot reclaim them. Snapshot creation currently caps the lifetime row count at 32 per Task; revoked/corrupt records still count, and no operation releases a slot. |

### CAP-01–06 implementation boundary

- CAP-01 has bounded private logical-tree and same-Mission published-Artifact discovery/read paths, but no discovery of an owner-authorized host root or Company/Group root.
- CAP-02 validates logical relative paths and CAS-root handling, but there is no mounted host path on which to exercise OS-level link, special-file, or replacement-race controls.
- CAP-03 distinguishes a Task-private logical tree from narrowly published same-Mission Artifacts. It has no Company shared-document visibility or Group/public-library ACL.
- CAP-04 binds private logical-tree writes to one current WorkerSession/epoch and root revision. No shared-root writer exists, so shared-root single-writer arbitration is not implemented.
- CAP-05 reads immutable CAS digests and checks byte lengths; the filesystem-root consistency/truncation behavior for a live mounted source is absent.
- CAP-06 has ready-state checks, immutable snapshot references, audited revocation, and a CAS orphan sweep. It has no obsolete/expiry transition or snapshot-reference reclamation policy; the generic collector is not a snapshot-retention engine.

### Recommended next REQ-29 slice

No host mount, Company/Group shared root, or snapshot reclamation behavior is safe to choose from source inspection alone: each needs an owner decision about which root is authorized, who shares it, whether access is read-only or writable, how revocation affects a running writer, and how long immutable snapshots remain retained. Do not make `workspace_root` executable authority or invent a default Group/retention policy.

After those decisions, the narrowest first vertical slice is a **read-only Company root binding** on one explicitly qualified host: owner selection through the OS's native directory authorization mechanism; a durable immutable binding revision and revocation event; a root-handle-based bounded walker that rejects escapes, links and non-regular entries; digest/size-pinned immutable snapshots; and an isolated fake-only read surface. Keep Group sharing, writable mounts, and garbage collection out of that slice. Validate the filesystem races on the target OS before exposing the path to any Worker. The current Termux host is not evidence for Windows or delegated Linux mount qualification.

The audit was source-only. It did not change any frozen CAP row or execution state; CAP-01–06 remain `partial` / `not_run`. See `evidence/development/r1-r3-implementation-validation-20261006-req29-local-audit/verification.md`.

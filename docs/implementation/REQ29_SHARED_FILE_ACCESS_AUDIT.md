# REQ-29 authorized shared-file access audit

Updated: 2026-10-28, Slice 333. This source map leaves the frozen v0.4.5 design unchanged.

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

REQ-29 is still partial. The logical tree supports only private Task-scoped UTF-8 files. It does not implement actual host mounts or OS-level symlink/special-file/TOCTOU controls. Slice333 selects no Company/Group shared roots and indefinite CAS retention as defaults; no native host-root path exists. Revocation denies future direct-ID reads but does not recall previously returned content. All applicable full CAP qualification remains open.

CAP-01–06 remain `not_run` as frozen scenarios. Slice 211 performed source/build verification only: no tests, migration application, populated WorkerSession end-to-end, Worker/provider execution or external operation occurred. The current environment has not supplied a database-confirmed active WorkerSession for qualification.

## Selected conservative default policy (Slice 333)

Under the user's delegated product-choice instruction, the default behavior is:

- `companies.workspace_root` remains display metadata and never grants filesystem authority by itself.
- Workspace visibility stays Company-private. There is no cross-Company or Group/public-library sharing by default.
- Work enters through an explicit owner/user file-selection upload and becomes an immutable Company CAS snapshot. Workers use their existing logical CAS workspace; no Worker writes back to the host root.
- Workspace-snapshot revocation blocks later direct-ID reads. Existing CAS references are retained; no automatic snapshot reclamation or expiry is selected.

This choice favors privacy and recoverability over convenience. It makes sharing, host-write, and retention defaults explicit without claiming a native root binding exists. Native read-only root access remains unavailable until a supported owner-selected root handle, no-follow/special-file/TOCTOU implementation, durable binding/revocation path, and target-host qualification are implemented. A plain path string is not upgraded into authority.

## History

Slice 150 identified the missing cross-Task Artifact read. Slice 151 implemented that bounded read path on @8, Slice 208 added the logical private Task tree and immutable draft snapshot model on @10, and Slice 211 added audited source-owner snapshot revocation on @11. This document supersedes the Slice 150 statement that no Worker tool can discover or read another Task's published Artifact; that statement was accurate only before Slice 151. See `evidence/development/r1-r3-implementation-validation-20261003-slice-151-req29-shared-artifact-read/verification.md`, `evidence/development/r1-r3-implementation-validation-20261004-slice-208-req29-workspace-tree/verification.md`, and `evidence/development/r1-r3-implementation-validation-20261005-slice-211-req29-workspace-snapshot-revocation/verification.md`.

## Local source audit (2026-10-06; based on source commit `83592318bc49b6f717d68054da7b213a1fbc63af`)

This audit checked the production Go/SQL paths and the frozen CAP-01–06 contract. It made no code or schema change. The migration source is Schema 110; this audit did not query or modify the local database.

| Area | Present source behavior | Concrete missing contract |
|---|---|---|
| Host roots and mounts | `companies.workspace_root` is saved, returned, and shown by the Workbench, but no runtime authorizes it or resolves it to an OS root. Company creation also accepts it as metadata. Linux Node preparation and native Worker launch bind fresh application-managed directories; Windows AppContainer workspaces are likewise application-managed. The Worker logical tree uses Company CAS and relative paths. Git import reads a locally available, selected commit into an immutable snapshot and does not write the source repository. | No owner-authorized root handle/bookmark, canonical host identity, binding revision, read/write capability, revoke path, or Worker mount adapter exists. A path string is not authority. Physical symlink/special-file/TOCTOU protections for a mounted user root therefore do not exist. |
| Company and Group sharing | Cross-Task published Artifact reads are bounded to another Task in the current active Mission and same Company, and record the exact digest/version and reader session. Workspace snapshot reads are exact-Artifact-ID, same-Mission reads. Neither is a Company-wide shared document root. Slice333 chooses Company-private visibility and deliberately adds no cross-Company/Group sharing. | No host shared-root ACL, Group entity/membership, cross-Company authorization join, or public-library visibility model is implemented; this is consistent with the selected private-only default. |
| Snapshot retention | Task snapshots are immutable `draft_not_accepted` Artifacts with per-file CAS references. Revocation is owner/session/epoch audited and blocks later direct reads; already returned bytes are not recalled. Slice333 selects indefinite retention: no automatic snapshot deletion or CAS reference release. Generic CAS collection deletes only digest files that no company-owned database row references. | There is no owner purge/reference-release lifecycle. `workspace_snapshot_files` and Artifact history continue referencing the objects, so the generic orphan collector cannot reclaim them. Snapshot creation caps the lifetime row count at 32 per Task; revoked/corrupt records still count, and once full, new snapshots fail closed. |

### CAP-01–06 implementation boundary

- CAP-01 has bounded private logical-tree and same-Mission published-Artifact discovery/read paths, but no discovery of an owner-authorized host root or Company/Group root.
- CAP-02 validates logical relative paths and CAS-root handling, but there is no mounted host path on which to exercise OS-level link, special-file, or replacement-race controls.
- CAP-03 distinguishes a Task-private logical tree from narrowly published same-Mission Artifacts. Company shared roots and Group/public-library ACLs remain intentionally unavailable under the selected private-only default.
- CAP-04 binds private logical-tree writes to one current WorkerSession/epoch and root revision. No shared-root writer exists, so shared-root single-writer arbitration is not implemented.
- CAP-05 reads immutable CAS digests and checks byte lengths; the filesystem-root consistency/truncation behavior for a live mounted source is absent.
- CAP-06 has ready-state checks, immutable snapshot references, audited revocation, and a CAS orphan sweep. It has no obsolete/expiry transition or snapshot-reference reclamation policy; the generic collector is not a snapshot-retention engine.

### Remaining REQ-29 implementation gate

The default sharing/write/retention policy is now selected above. Implementing a native root binding is still not a safe source-only change: its owner-selected root handle/bookmark, OS identity, no-follow/special-file/TOCTOU behavior, and running-reader revocation semantics need a concrete supported native adapter and host tests. Do not make `workspace_root` executable authority or add cross-Company sharing/GC under the selected default.

If a native root is later needed, the narrowest first vertical slice remains a **read-only Company root binding** on one explicitly qualified host: native owner selection, durable binding/revocation, a bounded root-handle walker, digest-pinned CAS snapshots, and an isolated fake-only read surface. Group sharing, writable mounts, and automatic garbage collection remain out of scope. The current Windows development source/build is not evidence of no-follow or TOCTOU behavior on a qualified host.

The audit was source-only. It did not change any frozen CAP row or execution state; CAP-01–06 remain `partial` / `not_run`. See `evidence/development/r1-r3-implementation-validation-20261006-req29-local-audit/verification.md`.

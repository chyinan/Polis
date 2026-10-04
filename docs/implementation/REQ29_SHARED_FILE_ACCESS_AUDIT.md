# REQ-29 authorized shared-file access audit

Updated: 2026-10-04, Slice 205. This source map leaves the frozen v0.4.5 design unchanged.

## Frozen requirements checked

Architecture section 3.1 describes REQ-29 as autonomous access to authorized workspaces and shared files, with read/write/snapshot separation and correct paths and versions. CAP-01–06 in section 44.10 cover self-discovery, path/link/special-file containment, shared versus private visibility, single-writer epochs, consistent reads, and ready/obsolete/GC snapshot handling.

## Current implementation

- `db/migrations/00002_workers.sql` defines `worker_workspaces` as one digest and revision per Task. The Worker workspace is therefore a single bounded text value, not an arbitrary directory tree with paths, links, file types, or per-file versions.
- The product Worker registry in `internal/codex/tools.go` adds `mission_artifacts_list` and `mission_artifact_read` only on the separately versioned fake-only @8 surface. The existing @4 and @7 surfaces are unchanged.
- `internal/kernel/shared_mission_artifacts.go` derives scope from the active database-bound WorkerSession and owned working Task. Listing/read are limited to ready candidate/passed Artifacts from a different Task in the same Mission; content is read by Artifact ID, bounded, checked against digest and byte size, and required to be UTF-8. Reads persist immutable Artifact/version and session provenance.
- `internal/kernel/employee_ops.go` routes those operations through the product Worker operation boundary. `internal/kernel/peer_employee_ops.go` keeps mutable writes on the current Task workspace and its digest/revision compare-and-swap path.
- Recovery snapshots remain a separate control/recovery record. They do not provide a model-visible shared-file browser or permission to read arbitrary CAS objects.

## Remaining gap

REQ-29 is still partial. The current model supports bounded text in one Task workspace plus immutable same-Mission Artifact reads; it does not model an arbitrary directory tree or a consistent snapshot of the current mutable workspace. Adding either would require an explicit path/file schema, visibility and authorization rules, snapshot consistency/retention semantics, and applicable CAP qualification. Reusing published Artifact reads for that purpose would conflate immutable outputs with mutable workspaces.

CAP-01–06 remain unqualified as frozen scenarios. Slice 151's implementation did not run tests or a populated WorkerSession end-to-end, and the current environment has not supplied a database-confirmed active WorkerSession for such execution. No Worker/provider or external action was performed for this audit.

## History

Slice 150 identified the missing cross-Task Artifact read. Slice 151 implemented that bounded read path on @8. This document supersedes the Slice 150 statement that no Worker tool can discover or read another Task's published Artifact; that statement was accurate only before Slice 151. Original implementation evidence: `evidence/development/r1-r3-implementation-validation-20261003-slice-151-req29-shared-artifact-read/verification.md`.

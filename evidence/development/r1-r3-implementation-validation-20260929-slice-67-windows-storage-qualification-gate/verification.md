# Slice 67 — Separate Windows disk-bound qualification evidence

Date: 2026-09-29  
Schema: 50 (no migration)

## Changes

- Replaced the single `windows_resource_bounds` qualification check with `windows_cpu_memory_process_bounds` and `windows_workspace_storage_ceiling`.
- A Windows host report can no longer satisfy the resource gate with CPU/memory/process evidence alone. The disk ceiling has its own explicit evidence reference and pass status.
- Linux qualification checks and the persisted qualification schema are unchanged. The Windows isolation policy remains `windows-appcontainer-node-policy@3`.
- This is a qualification evidence gate, not a storage quota implementation. The current AppContainer workspace is not confined to a dedicated capacity-limited volume, and no native disk-bound evidence was collected. The owner must not mark the separate storage check passed until a real host boundary is demonstrated.

## Verification

- Red/green unit test: `TestWindowsExecutorQualificationSeparatesDiskQuotaFromCPUAndMemoryBounds` failed against the combined check and passed after the checks were split. `ValidateEnvironmentExecutorQualificationEvidence` also rejects a report that collapses both requirements back into `windows_resource_bounds`.
- `bash scripts/go.sh test ./internal/environment -count=1` — passed.
- `bash scripts/r1-capability-source-postgres-test.sh` — passed on disposable PostgreSQL 18 through Schema 50 using reports constructed from the current required-check list.
- `rtk git diff --check` — passed.

## Qualification boundary

No native Windows storage quota or Job Object was run. The current application does not enforce a hard disk bound; `windows_workspace_storage_ceiling` only ensures the requirement is represented separately in an owner report. Fixed-port browser ingress, native WFP/registry, Node/npm and clean-VM verification remain open.

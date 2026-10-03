# REQ-26 shared-write resource binding audit

Updated: 2026-10-03, Slice 149. Scope is limited to existing code paths that can write to project or shared resources. The frozen design package is unchanged.

## C-RESOURCE contract checked

Architecture sections 42.5.1–42.5.3 require a normalized `ResourceKey` based on the actual provider/environment, external identity, target object and read/write class; alias resolution before claiming exclusivity; old-writer isolation evidence and a resource epoch before ownership transfer; and frozen candidate input plus separate bounded scratch for review. FT-66 and FT-70 are the directly relevant frozen scenarios.

## Existing write and import paths

- No `ResourceBinding` / `ResourceKey` type, `resource_bindings` table, resource ownership ledger or resource-epoch fence exists in production Go or SQL.
- `companies.workspace_root` is a non-empty stored/displayed setting. The runtime preparation path does not use it as an allowed filesystem root, so it must not be treated as a path authorization boundary or canonical resource identity.
- Durable `worker_workspaces` rows are metadata keyed by `(company_id, task_id)` and contain a digest/revision, not a shared filesystem path.
- `intake.ImportGitCommit` resolves an already-local full commit into an immutable MissionInput snapshot. Its contract rejects remote fetch, checkout, writes to the source repository and credential-helper reads. Current repository import therefore has no shared branch write to claim.
- Linux/Windows project preparation materializes verified input beneath an application-managed workspace root. Each execution receives a fresh random 128-bit workspace identity; materialization requires a new directory and creates files exclusively. These are independent local preparation workspaces, not a shared project-root writer. Linux further requires the bounded separate workspace filesystem and `deny_all` network policy; Windows uses the AppContainer workspace root.
- No production `git push`, shared deployment, or content-publish executor is present. Read-only GitHub feedback/import and per-recipient notification authorization do not provide evidence of a shared branch/deployment/publish write path.

## Remaining REQ-26 gap

The local execution model already separates attempt workspaces, but it does not satisfy ResourceKey ownership for any future shared external writer. A future writer must first identify an actual write action and target; canonicalize repository IDs plus target refs (and distinguish HTTPS/SSH aliases), bind the key to one authorized Company and resource epoch, acquire/fence that key at action admission, and refuse transfer until the old writer is confirmed stopped. Unknown aliases must require human mapping rather than optimistic independence. Review tooling must keep candidate bytes immutable and use isolated scratch.

This audit adds no inert registry. Until a concrete shared-write action is connected to an enforcing ResourceKey gate, REQ-26 remains open and no exclusivity claim is made. No external write or qualification action ran.

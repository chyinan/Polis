# REQ-29 local implementation audit

Date: 2026-10-06
Audit base: `83592318bc49b6f717d68054da7b213a1fbc63af`
Latest migration source inspected: Schema 110
Scope: host mounts, Company/Group shared roots, workspace-snapshot retention

## Result

Source review found no safe, owner-independent implementation slice for these three gaps. A filesystem root string cannot be converted into authority without an owner-selected root and explicit access policy. A Group-shared root cannot be scoped without a persisted Group and membership relation. Snapshot deletion or reference release cannot be selected without a retention decision. The detailed source map and recommended first vertical slice are in [`REQ29_SHARED_FILE_ACCESS_AUDIT.md`](../../../docs/implementation/REQ29_SHARED_FILE_ACCESS_AUDIT.md).

## Findings

- `companies.workspace_root` is persisted/displayed metadata. No Worker or environment runtime uses it as an authorized OS root. Linux/Windows worker environments use application-managed workspaces. Git import reads a local repository commit into an immutable snapshot and does not write the source repository.
- `worker_workspace_roots` is explicitly `task_private`; tree contents are Company-CAS-backed logical records, fenced by Task owner, current WorkerSession/epoch, and root revision. Same-Mission Artifact reads are a separate, narrower published-artifact path, not a Company shared document root.
- The migration set has no Group table or Company-to-Group membership/resource relation. A `group` value in Skill publisher scope is still keyed by Company and does not establish cross-Company Group identity.
- Workspace snapshot revocation blocks subsequent exact-ID reads and records the source owner's session/epoch, but does not recall content already returned. The immutable `workspace_snapshot_files` rows continue referencing CAS files. The generic `cas-collect` path can remove only unreferenced files and defaults to dry-run; it does not implement snapshot expiry or reclamation policy.
- Snapshot creation caps all snapshot Artifact rows at 32 per Task. Revoked or corrupt records still count, and no lifecycle operation releases a slot.
- CAP-01–06 therefore remain partial and `not_run`; this audit did not edit the scenario crosswalk.

## Verification

- `go build ./...` — passed.
- `git diff --check` — passed.
- Tests, database queries/migrations, Worker, provider, host mount operations, and scenarios — not run.

## Limitations

This was a source-only audit on the recorded repository base. No fresh host or database observation was taken. It does not qualify the current Android/Termux host for Linux or Windows filesystem race behavior.

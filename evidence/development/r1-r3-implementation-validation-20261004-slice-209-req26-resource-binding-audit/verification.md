# Slice 209 — REQ-26 shared-write resource binding audit refresh

## Source review

- Rechecked frozen `spec/design-v0.4.5/contracts/C-RESOURCE.md`, FT-66 and FT-70 after Schema 103 and the Slice 208 workspace-tree implementation.
- Searched production Go and SQL for `ResourceKey`, `ResourceBinding`, shared branch writes, deployment/publish executors and project-root usage.
- The new `worker_workspace_roots` and `worker_workspace_files` model remains Company/Task-private logical CAS state. Writes require the Task owner, active WorkerSession/epoch and root revision; no physical shared directory or cross-Company target is opened.
- Git import continues to snapshot an already-local commit without writing to the repository. Linux/Windows project preparation creates independent application-managed attempt workspaces. `companies.workspace_root` remains displayed configuration and is not enforced as a filesystem authorization root.
- No shared branch, deployment or content-publish writer exists to connect to a ResourceKey gate. Adding a registry without an actual action would make no write safer.

## Verification and limits

- Read-only source review only; no code or schema changed.
- No tests, database migration, Worker/provider action, external write, or frozen scenario ran.
- FT-66 and FT-70 remain `not_run`; REQ-26 remains partial. Canonical target aliasing, Company ownership, resource epoch handoff, old-writer isolation evidence, and the frozen candidate/read-only review scratch boundary remain unqualified.

# Slice 149 — REQ-26 C-RESOURCE shared-write audit

Date: 2026-10-03

## Source review

- Compared REQ-26 and Architecture sections 42.5.1–42.5.3 with production Go/SQL and the frozen FT-66/FT-70 scenario expectations.
- Confirmed no production `ResourceBinding`, `ResourceKey`, resource ownership ledger, or resource-epoch fence exists.
- Confirmed `companies.workspace_root` is persisted/displayed metadata and is not used as a runtime path authorization root.
- Confirmed `worker_workspaces` is digest/revision metadata keyed by Company+Task.
- Confirmed `intake.ImportGitCommit` accepts only locally available commits and does not fetch, checkout, write the source repo, or read credential helpers.
- Confirmed Linux/Windows preparation materializes each verified snapshot under a fresh random execution workspace, requiring a new root and exclusive file creation; Linux uses its bounded isolated workspace and `deny_all` network profile.
- No production shared branch push, deployment, or content-publish path was found. No external action was run.

## Checks and disposition

| Check | Result |
| --- | --- |
| `git diff --check` | PASS |
| Frozen catalogs or scope changed | No |
| Code/schema/tests changed or run | No |
| REQ-26 status | Remains open; a future actual shared writer needs canonical target identity, per-Company ResourceKey ownership, runtime admission fencing, and old-writer stop evidence |

This is a source audit, not a qualification result. FT-66, FT-70 and REQ-26 remain `not_run` / partial as recorded by the existing crosswalk.

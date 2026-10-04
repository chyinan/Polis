# Slice 210 — REQ-39 frozen workspace read retry

Date: 2026-10-05

## Change

The takeover panel previously marked the frozen workspace as loaded in a `finally` block, even when the read failed or the response did not match the lease. That left the still-granted lease without editable base content and without a retry action. The panel now records the loaded lease ID only after digest, revision and supported file-shape validation succeeds. A failed read presents an explicit retry that uses the same active lease and does not grant a replacement lease.

This does not extend the patch format. The current lease binds only the legacy `worker_workspaces` digest and revision; the Workbench owner read model exposes that content as one `workspace.txt`; and snapshot return is one bounded MissionInput. Schema 103's private Task file tree has no owner-facing frozen manifest bound to a takeover lease. Multi-file patch handback therefore remains open until a frozen tree manifest and bounded return representation exist.

## Verification

- `npm run build` in `frontend/`: passed. Vite transformed 1,678 modules and produced the production bundle. Existing large-chunk advisory remains for the 856.77 kB main JavaScript chunk.
- `git diff --check`: passed.
- No tests were added or run. No database migration, lease command, Worker/provider action, or frozen FT/NT/CAP/UI/WF/PP scenario ran. Frozen execution states remain `not_run`.

## Status

REQ-39 remains partial. This is local Workbench recovery behavior only; it does not qualify safe-change handback or multi-file workspaces.

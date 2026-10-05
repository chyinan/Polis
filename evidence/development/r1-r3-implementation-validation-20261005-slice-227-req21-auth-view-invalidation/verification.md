# Slice 227 — REQ-21 protected-view invalidation

## Change

Workbench API HTTP 401/403 responses now dispatch a protected-view invalidation. The App clears its React Query cache and remounts the active route, removing cached query data and component-local details/editors. A bounded per-endpoint denial latch prevents repeated invalidation loops and clears when that endpoint later succeeds. A protected EventSource that permanently closes triggers the same invalidation, and successful installation-owner logout invalidates the shared view state.

Linked this evidence from UI-09; its disposition remains `partial`, execution remains `not_run`, and the crosswalk still contains 232 scenarios, all `not_run`.

## Verification

- `npm run build` from `frontend/`: passed (TypeScript and Vite production build; existing large-chunk advisory remains).
- `git diff --check`: passed.
- No tests, authentication/logout flow, permission-revocation operation, Worker/provider activity or frozen scenario ran.

## Status

This closes the stale in-memory view handling path when an API denial or logout is observed. Full UI-09 logout, revocation and narrowed-scope qualification remains open.

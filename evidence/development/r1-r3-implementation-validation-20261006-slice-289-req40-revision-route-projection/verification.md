# Slice289 verification: REQ-40 revision-route read projection

Date: 2026-10-06

## Scope

- Added an optional Schema119-backed `revisionRoutes` read projection to the durable delivery lifecycle response.
- Route rows are Company/Delivery scoped, joined to immutable Manifest and `changes_requested` disposition rows, bounded to 16 latest routes, and fail closed on invalid state, successor Mission or Task linkage.
- Frontend types, validation and read-only Workbench rendering expose pending, successor-Mission and revision-Task route states without lifecycle side effects. Pre-Schema119 responses remain compatible with an empty optional projection.
- No route command, PostgreSQL runtime, Worker/provider, browser, external account or frozen scenario ran.

## Verification

- `bash scripts/go.sh test ./internal/workbench -run TestCanonicalDurableDeliveryManifestAllowsIncompleteAssemblingEvidence -count=1` — passed.
- `frontend/node_modules/.bin/tsc.cmd -b --pretty false` — passed.
- `frontend/node_modules/.bin/vite.cmd build` — passed.
- `git diff --check` — passed.

The pnpm wrapper remains unable to run its install preflight because `esbuild@0.28.2` build scripts are blocked by the current approval policy; direct local TypeScript/Vite binaries passed.

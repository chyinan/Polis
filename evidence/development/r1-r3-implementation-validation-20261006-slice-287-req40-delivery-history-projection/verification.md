# Slice287 verification: REQ-40 delivery revision/disposition history projection

Date: 2026-10-06

## Scope

- Extended the durable delivery lifecycle read projection with bounded Manifest revision history and UserDisposition history.
- Each historical Manifest is re-canonicalized and checked against the immutable Company/Delivery/Mission/Task/Artifact identity, stored SHA-256, artifact digest/size and revision; each historical disposition is checked against a returned Manifest revision.
- Added optional frontend types, strict history validation when present, and read-only Workbench rendering. Legacy responses remain compatible because history fields are optional.
- No delivery command, PostgreSQL runtime, Worker/provider, browser, external account or frozen scenario ran.

## Verification

- `bash scripts/go.sh test ./internal/workbench -run TestCanonicalDurableDeliveryManifestAllowsIncompleteAssemblingEvidence -count=1` — passed.
- `frontend/node_modules/.bin/tsc.cmd -b --pretty false` — passed.
- `frontend/node_modules/.bin/vite.cmd build` — passed.
- `git diff --check` — passed.

The repository pnpm wrapper could not run its install preflight because `esbuild@0.28.2` build scripts are blocked by the current pnpm approval policy; direct existing local TypeScript/Vite binaries passed.

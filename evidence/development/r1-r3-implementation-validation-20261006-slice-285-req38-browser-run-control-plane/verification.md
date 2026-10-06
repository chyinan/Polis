# Slice285 verification: REQ-38 BrowserRun control-plane foundation

Date: 2026-10-06

## Scope

- Added Schema117 immutable BrowserRun identities and append-only events. Records bind the current Company/Mission/Task/WorkerSession to a current service JobRun generation and store only bounded origin/plan/browser-environment metadata.
- Added strict HTTPS-origin canonicalization that rejects credentials, paths, query strings and fragments. Requests persist as `blocked/browser_runtime_unqualified`; no URL, browser, cookie, storage state, bearer handle, download, WebSocket or management-network access is performed.
- Added fake-only product surface `polis-product-tool-surface@18` with `browser_run` and `browser_results`. The surface inherits exact @17 Job/Lease reads while preserving fake-only authorization and manifest isolation.
- No Playwright/Chromium process, service process, native host, Provider turn, external account or frozen scenario ran.

## Verification

- `bash scripts/go.sh test ./internal/kernel -run TestCanonicalBrowserRunOriginAllowsOnlyHTTPSOrigins -count=1` — passed.
- `bash scripts/go.sh test ./internal/kernel -run TestNormalizeBrowserRunRequestBoundsMetadataAndPlan -count=1` — passed.
- `bash scripts/go.sh test ./internal/kernel -run TestEmployeeToolsBrowserRunRequiresTheIsolatedSurface -count=1` — passed.
- `bash scripts/go.sh test ./internal/codex -run TestProductEmployeeBrowserRunSurfaceAddsDefaultDeniedControlPlaneTools -count=1` — passed.
- `bash scripts/go.sh test ./internal/provider -run TestProductBrowserRunSurfaceIsFakeOnlyAndDefaultDenied -count=1` — passed.
- `bash scripts/go.sh test ./internal/control -run TestBrowserRunFakeSurfacePassesAdapterReadinessWithoutRealProvider -count=1` — passed.
- `bash scripts/go.sh test ./db -run TestEmbeddedMigrationManifestMatchesRepository -count=1` — passed.
- `bash scripts/go.sh build ./cmd/...` — passed.

PostgreSQL migration runtime, controlled browser qualification, native service/restart qualification, real Provider and frozen-scenario execution remain unrun.

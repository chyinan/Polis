# Slice286 verification: REQ-38 research/search/fetch unavailable control plane

Date: 2026-10-06

## Scope

- Added Schema118 immutable ResearchOperation identities and append-only events for bounded `search` and HTTPS `fetch` intents.
- Added fake-only product surface `polis-product-tool-surface@19` with `research_search` and `research_fetch`. Each request is bound to the current Company/Mission/Task/WorkerSession and persists `unavailable/research_backend_unavailable` with `provider_egress: 0`.
- Search/fetch input is normalized as data only; no query, URL, MCP call, HTTP request, browser navigation or external account is used.
- No external web call, Provider turn, browser, service process, external account or frozen scenario ran.

## Verification

- `bash scripts/go.sh test ./internal/kernel -run TestNormalizeResearchOperationRequestSeparatesSearchAndFetch -count=1` — passed.
- `bash scripts/go.sh test ./internal/kernel -run TestEmployeeToolsResearchOperationsRequireTheIsolatedWritableSurface -count=1` — passed.
- `bash scripts/go.sh test ./internal/codex -run TestProductEmployeeResearchSurfaceAddsExplicitUnavailableOperations -count=1` — passed.
- `bash scripts/go.sh test ./internal/provider -run TestProductResearchOperationsSurfaceIsFakeOnlyAndUnavailable -count=1` — passed.
- `bash scripts/go.sh test ./internal/control -run TestResearchOperationsFakeSurfacePassesAdapterReadinessWithoutRealProvider -count=1` — passed.
- `bash scripts/go.sh test ./db -run TestEmbeddedMigrationManifestMatchesRepository -count=1` — passed.
- `bash scripts/go.sh build ./cmd/...` — passed.

PostgreSQL migration runtime, real search/fetch backend qualification, Provider, browser, external account and frozen-scenario execution remain unrun.

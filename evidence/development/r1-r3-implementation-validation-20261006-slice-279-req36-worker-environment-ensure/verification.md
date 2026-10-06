# Slice279 verification: REQ-36 fake-only Worker environment ensure

Date: 2026-10-06

## Scope

- Added fake-only `polis-product-tool-surface@16` with `environment_status` and `environment_ensure`.
- Bound the request to the current WorkerSession, Task and Mission inside the same idempotent write transaction, including replay authorization.
- Accepted only an exact environment revision ID; no host, command, registry or network input is exposed.
- Kept qualified real-provider @4 unchanged.

## Verification

- `bash scripts/go.sh fmt ./internal/control ./internal/provider ./internal/kernel ./internal/codex` — passed.
- `bash scripts/go.sh test ./internal/codex -run TestProductEmployeeEnvironmentEnsureSurfaceExtendsEnvironmentStatusOnly` — passed.
- `bash scripts/go.sh test ./internal/provider -run TestProductEnvironmentEnsureSurfaceIsFakeOnlyAndIncludesStatus` — passed.
- `bash scripts/go.sh test ./internal/control -run TestEnvironmentEnsureFakeSurfacePassesAdapterReadinessWithoutRealProvider` — passed.
- `bash scripts/go.sh test ./internal/kernel -run TestValidateProductTaskEnvironmentEnsureScopeRequiresCurrentMissionAndTask` — passed.
- `bash scripts/go.sh test ./internal/kernel -run TestEmployeeToolsEnvironmentEnsureRequiresTheIsolatedWritableSurface` — passed.

No PostgreSQL migration/runtime, Worker turn, host executor, provider, browser, external account or frozen scenario ran. The real provider remains pinned to @4.

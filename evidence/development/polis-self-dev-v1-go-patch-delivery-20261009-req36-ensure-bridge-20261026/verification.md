# Slice 325 — REQ-36 Worker environment preparation bridge

Date: 2026-10-26
Result: `WORKER_ENSURE_BRIDGE_IMPLEMENTED_AND_OFFLINE_VERIFIED`; DependencyChange materialization and host qualification remain open.

## Implemented

- Added an explicit `POLIS_OFFLINE_ENVIRONMENT_ENSURE_ENABLED=1` serve switch for exact fake-only `polis-product-tool-surface@16`. The switch requires `POLIS_WORKER_MODE=real` with `POLIS_PROVIDER_TRANSPORT=fake`, is mutually exclusive with other offline surfaces, and is unavailable on the real Provider `@4` contract.
- Added a Control bridge from the Worker tool to the existing environment preparation executor. The Worker path first records the request through `TXRequestProductTaskEnvironmentPreparation`, which checks the active exact WorkerSession, Task/Mission binding, approved policy, source snapshot and qualified executor; Control then reuses the existing bounded preparation/recovery protocol.
- With the bridge absent, the tool fails closed. The Worker cannot select a host executor, package manager command, path, network policy, or arbitrary environment profile.
- This increment does **not** materialize an approved DependencyChange proposal into a changed `package.json`, lockfile, MissionInput, or environment revision. It only lets a Worker request preparation of an already registered and approved immutable revision.

## Verification

- Full Go tests and race suite: PASS — `go-tests.log`, `go-race.log`.
- `go vet ./...`: PASS — `go-vet.log`.
- Linux amd64 and Windows amd64 command builds: PASS — `linux-build.log`, `windows-build.log`.
- Disposable PostgreSQL 18.6 / Schema 134 integration: `TestProjectJobLifecycleLaunchesOncePersistsLogsAndReplaysWithoutExecution` PASS. Its active synthetic WorkerSession calls the new `environment_ensure` Worker bridge; an injected recording executor receives the exact immutable snapshot, and the subsequent Task JobRun start/stop checks still pass. Temporary Unix-socket-only PostgreSQL was stopped. Evidence: `postgres-runtime/20261026T083801Z-374/`.
- No npm install, package registry access, external retrieval, real Worker turn, Provider request, or host environment qualification occurred.

## Remaining REQ-36 functionality

An approved DependencyChange proposal still does not produce a derived immutable package snapshot, resolved lockfile, or successor environment revision. That requires the separate bounded registry materializer and native executor qualification described in the implementation plan.

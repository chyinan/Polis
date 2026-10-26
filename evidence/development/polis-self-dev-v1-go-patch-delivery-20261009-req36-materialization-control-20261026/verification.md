# Slice 327 — REQ-36 durable DependencyChange materialization

Date: 2026-10-26
Result: `SOURCE_FLOW_IMPLEMENTED_AND_OFFLINE_VERIFIED`; Windows host/AppContainer/WFP qualification remains open.

## Implemented

- Added Schema 135/136 immutable materialization attempt identities and append-only events. A proposal has one unresolved attempt at a time; the exact request ID replays only the recorded state, `outcome_unknown` is never rerun, and application startup changes stale `running` attempts to `outcome_unknown` under the Kernel runtime lock.
- Materialization admission rechecks exact proposal approval, base Mission/revision, current base-policy approval and current executor qualification in PostgreSQL. A policy cannot be revoked while that base has a resolver attempt in `running`.
- Added an owner-facing Workbench API and UI for listing proposals, recording exact dependency changes, approving/rejecting, and explicitly requesting materialization. No Worker tool can invoke this registry-capable operation.
- Control invokes only a configured `ProjectDependencyChangeResolver`. It validates the generated package manifest and lockfile against the exact base snapshot, approved dependency versions and registry policy; stores a derived immutable directory MissionInput; and creates a successor environment revision pinned to the original profile, toolchain and policy. The successor policy is not approved automatically.
- The Windows resolver reuses the existing bounded preparation executor and registry-only AppContainer. It stages only the generated root package manifest and runs a fixed `npm install --package-lock-only --ignore-scripts --no-audit --no-fund --no-save --workspaces=false` argument vector against exactly one proposal registry. Timeout/output bounds apply; registry egress is revoked before output reads and snapshot cleanup is confirmed. Bounded redacted logs are retained by CAS digest and linked from the terminal attempt event.
- Linux materialization remains unavailable: its approved profile is deny-all and has no qualified registry resolver.

## Verification

- Full Go suite: PASS — `go-tests.log`.
- Full Go race suite: PASS — `go-race.log`.
- `go vet ./...`: PASS — `go-vet.log`.
- Linux amd64 and Windows amd64 command builds: PASS — `linux-build.log`, `windows-build.log`.
- Frontend TypeScript project build/typecheck, ESLint, and Vitest: PASS — `frontend-typecheck.log`, `frontend-lint.log`, `frontend-tests.log` (19 files, 166 tests).
- Migration manifest hardening test: PASS — `migration-manifest-test.log`.
- Disposable PostgreSQL 18.6 / Schema 136 targeted integration: PASS — exact attempt replay, blocked concurrent attempt, policy-revocation fence, restart-to-unknown/no-replay, fake resolver output validation, immutable MissionInput/successor revision linkage, and owner proposal/materialization status projection. Evidence: `postgres-runtime/20261026T152539Z-378/`.
- The integration resolver is a recording fake. No npm process, package registry request, external network egress, real Worker, Provider request, or Windows native host operation ran.

## Qualification still required

Before real use, qualify the current Windows bounded-volume/AppContainer/WFP and proxy behavior, resolver process-tree stop/restart cleanup, and the current host/toolchain fingerprint. The native Windows resolver was cross-compiled but not executed. Keep the feature blocked until those gates pass. The default-provider `@4` surface remains unchanged.

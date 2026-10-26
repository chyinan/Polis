# Slice 324 — REQ-37 Worker project JobRun tools

Date: 2026-10-26
Result: `SOURCE_IMPLEMENTED_AND_OFFLINE_VERIFIED`; native/real-provider qualification remains open.

## Implemented

- Added exact, separate fake-only `polis-product-tool-surface@20` with `jobs_start` and `jobs_stop`. Surface manifest SHA-256: `320c584c0a76c43b3b0e072da13d5904bbaf74bcd558d1dd3e0ec5f8e5f2c281`; aggregate schema SHA-256: `30ffa168a152b094f42b16fd0cc0fee992d949f9f00c1818a88f3bf454ed8e7a` (3546 bytes).
- The existing qualified real-provider `@4` surface and historical `@15`/`@17` fingerprints are unchanged. `@20` is accepted only with `POLIS_OFFLINE_PROJECT_JOBS_ENABLED=1`, `POLIS_WORKER_MODE=real`, and `POLIS_PROVIDER_TRANSPORT=fake`; it cannot be combined with other offline extension surfaces or controlled MCP.
- Worker calls derive Company, Task, Employee and WorkerSession from the trusted Kernel `Binding`. Start supports only existing `batch` or registered `service` requests against an exact environment revision. The existing Control path repeats current Task/active-session, environment policy, executor qualification, process, timeout, output and service-owner checks.
- Stop first verifies that the exact JobRun is owned by the current Task and WorkerSession, then delegates to the existing cleanup/reconciliation state machine. Missing Control wiring, read-only sessions, wrong Task/session, malformed arguments, unavailable environment/executor, or unqualified host fail closed.

## Verification

- Full Go suite: PASS — `go-tests.log`.
- Full Go race suite: PASS — `go-race.log`.
- `go vet ./...`: PASS — `go-vet.log`.
- Linux amd64 and Windows amd64 command builds: PASS — `linux-build.log`, `windows-build.log`.
- Targeted disposable PostgreSQL 18.6 / Schema 134 integration: `TestProjectJobLifecycleLaunchesOncePersistsLogsAndReplaysWithoutExecution` PASS. It exercises the Worker `jobs_start`/`jobs_stop` bridge against the existing Control lifecycle using a synthetic sleeper WorkerSession and a recording ProjectJob executor; PostgreSQL used a temporary Unix-socket-only cluster and was stopped afterward. Evidence: `postgres-job-tool-e2e.log` and `evidence/development/polis-self-dev-v1-go-patch-delivery-20261009-req37-project-jobs-20261026-job-tool/postgres-runtime/20261026T075321Z-394/`.
- `git diff --check`: PASS.

No real Provider/model turn or network egress was performed. The synthetic sleeper is test-only; the ProjectJob executor was recorded/faked. This does not qualify Linux/Windows native process execution, WFP/AppContainer, service-generation restart/recovery, or expose `@20` to the real Provider.

## Next planned increment

Proceed to the approved REQ-36 dependency-materialization path under the fixed registry/no-lifecycle-script/resource envelope. Keep host execution disabled unless the configured executor and its qualification gates pass. The broader plan is `docs/implementation/REMAINING_PRODUCT_FEATURES_IMPLEMENTATION_PLAN.md`.

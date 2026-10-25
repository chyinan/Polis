# Current HEAD reconciliation — 2026-10-26

## Final result

`POLIS_SELF_DEV_V1_WORKSPACE_GO_JOB_PATCH_SLICE = PASSED_LOCAL_E2E`.

The complete local offline chain passed: qualified systemd host, explicit real fixed Go JobRun, durable pass receipt, replay-verified formal Patch Artifact, ready DeliveryManifest, actual Workbench diff, and installation-owner disposition in an isolated disposable PostgreSQL fixture. This slice result does not qualify the broader R1–R3 requirements, provider operations, production workspaces, or live business-worker behavior.

```ini
provider_egress = 0
real_model_worker_started = 0
automatic_apply = NO
automatic_merge = NO
automatic_deploy = NO
business_database = NOT_USED
```

The E2E uses a `deterministic/fake` WorkerSession only to seed a synthetic Task workspace; a database read confirmed it active before snapshot creation, and the test confirmed it stopped before the disposable database was destroyed. The candidate Go tests ran only inside the qualified systemd/Bubblewrap boundary.

## Repository state

- Workspace: `D:/Programs/Polis-cloud-main` only; the other checkout was not accessed.
- Branch: `codex/req40-manifest-routing`.
- Verification began from HEAD `1321c69aa9feef42e8096b439e053b6d9b6a5b80`; no reset, clean, or checkout occurred. After the user explicitly requested it, the slice changes were committed and pushed to `codex/req40-manifest-routing`.
- `evidence/development/polis-self-development-baseline-reconciliation/` remains preserved and unmodified.
- Intermediate fail-closed E2E attempts remain preserved in separate `selfdev-e2e-*` evidence directories; none reused an unknown JobRun request.

## Host qualification

Current qualification is bound to WSL boot `89e82fea-58e1-44c3-a79d-cc96ec7d8fcd`:

| Identity | Value |
|---|---|
| Supervisor SHA-256 | `4e241c70eed64ee138769b5bf191eee18ee9fa167295a3f6a3bdfa47171c7fb3` |
| Supervisor source identity | `6be9cdf873e84f7699938332fd79c8999a857a539ee23b9ce1addcc4989971ab` |
| Unit-control helper SHA-256 | `186bb99f606abbd2604480f1d2047936a7414ac97169f59977992af0853d7b89` |
| Root receipt qualification SHA-256 | `62e82a5882918d2fecacd296d5d30d6b28884c897f064f74dc43e5f712b0e9f4` |
| Probe bundle SHA-256 | `7412bd2ed5fe86d89de3d1e193b1e9d0a4ebefe1dda0af8c31ca4ca75951b7c4` |
| Toolchain tree SHA-256 | `7643befa386325ee8037168d2a6d7f3887677c2e3bd4f096c4a3917f8a78cc23` |

The root-owned, read-only toolchain stage is `/run/polis-self-dev-go-toolchain`. Hashing the original DrvFs tree took 151248ms, exceeding both the preflight and Job lifetime; hashing the root-owned ext4/tmpfs stage took 451ms. The systemd unit and API both use the staged path. The current full host probe passed CPU throttling, memory OOM/swap, PID cap, tmpfs byte/inode limits, systemd wall timeout, process-tree cleanup, Bubblewrap network isolation, and path boundaries. Full output and receipt are under `host-qualification-20261025/`.

The PKLA parent is intentionally left root-only. `SystemdExecutor` uses a fixed `pkexec --verify-installation` read-only helper command so the root helper verifies the protected `.pkla` content against the root-owned seal; no Polkit directory permission was broadened and no generic `manage-units` grant was added.

## Final E2E evidence

- Browser/Go test run: `selfdev-e2e-20261026T063547/`.
- Company: `selfdev-e2e-2dhi7brrv7hv`; Task: `5a3326c6491f81becd7793003a4c3658`.
- Real JobRun: `job-cdbb806b0e578419a30b513d6d80a742`; state `exited`, outcome `passed`, `executionKind=real`, `qualificationEligible=true`, process/cgroup cleanup confirmed.
- Formal Patch Artifact: `1edfbc093b55f7345cd14d71cacc4e26`; one replay-verified row and qualification row were confirmed inside the test.
- Durable DeliveryManifest: revision `2`, state `ready`.
- Disposition: `accepted` by the disposable installation-owner test session. The reason explicitly labels this a synthetic E2E fixture, not a production acceptance. No source repository apply, merge, or deploy occurred.
- The browser exercised `RealWorkbenchApi` through Vite and the production Workbench HTTP handler, displayed the read-only diff, submitted the disposition with CSRF, and read back the durable lifecycle. Screenshots: `go-patch-workbench-diff.png` and `go-patch-workbench-disposition.png`.
- The systemd request/result sidecars are preserved with matching SHA-256 values in `systemd-job-handoff-sha256.txt`.

The E2E PostgreSQL 18.6 harness applied Schema 134, passed its least-privilege check, used a Unix-socket-only temporary cluster, and stopped/removed that cluster. Full PG18 integration also passed Schema 134: 1941 pass events, 0 failures, 65 skips.

## Source/verification summary

- Added Schema 134 to permit machine-verified `go_patch` Artifacts without attributing a human verifier; a deferred constraint trigger still requires the exact replay-verified `go_patch_deliveries` row at commit.
- Added a conservative systemd-result fallback for a short-lived service whose `InvocationID` is cleared before polling: the handoff directory must be newly created, the report must fully validate against the JobBinding and host receipt, systemd must report successful terminal status, and the cgroup must be empty. No JobRun is started again during collection.
- Corrected owner-session browser POSTs to carry the CSRF cookie and made disposition history keys unique across Manifest revisions.
- Corrected accepted-disposition routing so an explicit acceptance does not create a revision route; `changes_requested` retains its existing Mission/backlog routing.
- Full Go tests, race tests, vet, Linux/Windows amd64 command builds, migration hash tests, frontend 164 tests, typecheck, lint, and production build passed. The Vite large-chunk advisory remains informational.
- Evidence logs are under `evidence/development/polis-self-dev-v1-go-patch-delivery-20261009-broker-20261026/` and the Schema 134 PostgreSQL runtime directories.

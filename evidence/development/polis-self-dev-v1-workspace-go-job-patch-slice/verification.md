# Polis self-development Go JobRun / Patch slice

Date: 2026-10-08

## Result

```ini
self_dev_go_job_patch_offline_slice = NOT_PASSED
source_integration = PARTIAL
host_execution_profile_qualification = BLOCKED
real_go_job_execution = NOT_RUN
patch_delivery_and_review = NOT_IMPLEMENTED
provider_surface_impact = NO_CHANGE
provider_egress = 0
real_model_worker_started = 0
live_canary = NOT_RUN
automatic_merge = NO
automatic_deploy = NO
```

This turn added offline profile, snapshot, result-classification, and patch-core rules. It did not wire a persistent Go JobRun, Patch Artifact publication, or Workbench review path. No candidate Go code ran.

## Baseline and workspace

```ini
repository = D:/Programs/Polis-cloud-main
starting_HEAD = ff34fcd7177d1807e0fd751de332f7344fbcb948
starting_branch = codex/req40-manifest-routing
starting_worktree = clean except for pre-existing untracked reconciliation evidence
reconciliation = evidence/development/polis-self-development-baseline-reconciliation/RECONCILIATION.md (preserved)
other_checkout = D:/Programs/Polis (not accessed)
```

The pre-existing reconciliation document was not edited, staged, or removed. No reset, clean, checkout, commit, or push was performed.

## Host probes

| Capability | Observation | Result |
|---|---|---|
| Linux host | WSL2, Ubuntu 22.04.5, kernel `6.6.87.2-microsoft-standard-WSL2`, amd64 | Identified; not a clean VM |
| Bubblewrap | `bwrap 0.6.1`; trusted `--unshare-net --unshare-pid` probe succeeded; only loopback appeared in the isolated network namespace | Narrow probe passed |
| Process cleanup | Trusted timeout/PID-namespace probes left no `sleep` process in the before/after check | Narrow probe passed; no candidate process was used |
| cgroup v2 | Controllers are present, but the root subtree exposes only `memory pids`; the current cgroup is not delegated/writable, and no usable child cgroup or `cgroup.kill` path was available | **Blocked**: effective CPU, memory/swap, PID limits and whole-job kill cannot be established |
| Scratch | `/tmp` is ext4, but no per-job byte or inode quota was available; workspace storage is DrvFs | **Blocked**: no hard-capped job scratch |
| Go executable | `go1.26.8 linux/amd64`; `/go/bin/go` executable SHA-256 `d9a2fa19c7ef8b57f420012c21f49f235c46f08a68c12077d9c753dbb6ccdc34` | Executable identified; full read-only toolchain tree identity/mount is not qualified |
| PostgreSQL | PostgreSQL 18 binaries and `POLIS_TEST_DSN` were unavailable; Docker Desktop daemon was unavailable | DB-backed integration not run |

Do not launch a Go JobRun until an administrator provides a delegated cgroup-v2 subtree with effective `cpu.max`, `memory.max`, `memory.swap.max`, `pids.max`, and `cgroup.kill`, plus a private scratch filesystem capped at 1 GiB and 4096 inodes. The inode cap counts filesystem objects, including files and directories, not just regular files. Verify actual values and hit both tmpfs boundaries with a trusted bounded probe; verify timeout/cancel leaves no descendants. Do not add firewall exceptions or run candidate code during these checks.

## Source changes

- `internal/workspacepath`: centralized the relative-path rule used by WorkspaceTree and Patch validation.
- `internal/gotestprofile`: added a canonical fixed fixture profile; exact argv and environment values; a Linux-only `openat` materializer that rejects unsafe entries and writes source read-only; strict immutable JobBinding serialization; and a result classifier that requires every frozen baseline test event plus affirmative host-control receipts.
- `internal/workspacepatch`: added canonical Patch bundle construction and replay checks. A bundle binds base/candidate snapshots, commit/input manifest, TaskValidationBinding, JobRun/profile/toolchain/command/test/result identities and verifies that replay produces the exact candidate tree.
- `internal/taskvalidation` and Workbench mission forms: added the fixed profile revision to contract binding and UI configuration. The existing text-only validator still returns `validator_unavailable` for this revision, so it cannot create a false Go-test pass.
- `db/migrations/00128_go_test_acceptance_contract.sql`: allows the fixed profile in persisted Mission/Task contracts; `db/migration_hashes.sha256` pins the new migration. The migration was not applied to PostgreSQL in this turn.

No changes were made to provider registries, credentials, provider egress, or real Worker admission.

## Verification

| Check | Result |
|---|---|
| Targeted Go tests: `gotestprofile`, `workspacepatch`, `workspacepath`, `taskvalidation`, `db` | Passed |
| `bash scripts/go.sh test ./...` | Passed |
| `bash scripts/go.sh test -race ./...` | Passed |
| `bash scripts/go.sh vet ./...` | Passed |
| Linux `bash scripts/go.sh build ./...` | Passed |
| Windows amd64 cross-build `GOOS=windows GOARCH=amd64 bash scripts/go.sh build ./...` | Passed |
| Frontend `npm test` | Passed, 150/150 |
| Frontend `npm run typecheck` | Passed |
| Frontend `npm run build` | Passed; Vite reported the existing large-chunk advisory |
| Frontend `npm run lint` | **Failed on four existing findings**: `no-control-regex` in `real-workbench-api.ts`, `workbench-validation.ts`, and `MissionTakeoverPanel.tsx`; unused `_kind` in `MissionTakeoverPanel.tsx`. These lines are outside the changes made for this slice. |
| PostgreSQL 18 migration/integration and real API/Workbench browser review | Not run: no PostgreSQL 18 server or test DSN |
| Restricted Go candidate execution | Not run: effective cgroup and scratch limits unavailable |

The Go test suite's configured DB tests did not provide PostgreSQL 18 integration evidence. Passing Go unit/race/vet/build checks do not qualify a candidate JobRun.

## Remaining implementation and qualification gates

1. Wire imported complete Git commit and TaskInputManifest into an initial Task-private WorkspaceTree; preserve the existing CAS/revision authorization path.
2. Persist the frozen Go JobBinding and trusted runner result on the existing JobRun lifecycle. The fixed Go profile is not yet admitted by a persistent Go runner.
3. Create a first-class Patch Artifact only from verified base/candidate snapshots and a persisted passing Go receipt; bind it through TaskValidationBinding, qualification, and DeliveryManifest policy.
4. Add the real Go API and Workbench diff/test-result review, with existing UserDisposition accept/request-changes actions and no source-repository write path.
5. Provision a disposable PostgreSQL 18/CAS test environment and run the end-to-end API/browser chain plus failure cases.
6. Qualify effective cgroup controls, scratch quotas, network isolation, and process-tree cleanup before running the trusted Go fixture. Then run the actual restricted fixture JobRun; do not substitute Node JobRun evidence.

Acceptance remains `NOT_PASSED` until these source and host gates run through the real offline chain. Any later real provider or Worker qualification remains outside this slice.

## Continuation checkpoint — host tooling and lint (2026-10-08)

This continuation preserves the earlier findings above and adds the following verified state:

```ini
current_HEAD = ff34fcd7177d1807e0fd751de332f7344fbcb948
source_integration = PARTIAL
host_execution_profile_qualification = BLOCKED
real_go_job_execution = NOT_RUN
postgres_migration_and_integration = NOT_RUN
patch_delivery_and_review = NOT_IMPLEMENTED
provider_egress = 0
real_model_worker_started = 0
live_canary = NOT_RUN
automatic_merge = NO
automatic_deploy = NO
```

The read-only `scripts/self-dev-go-job-host.sh inspect` probe now reports both the current user cgroup and the systemd `system.slice`. In this WSL session the v2 root and `system.slice` expose `memory pids`, but not `cpu`; the current user service also lacks CPU delegation, and its `cgroup.kill` file is not writable by the user. `/tmp` reports ext4 with about 1.02 TB available and about 67 million free inodes, which is host capacity, not a per-JobRun quota. The current unprepared state is therefore not qualified.

The local trusted bwrap probe passed its network-namespace, read-only input, writable temporary mount, and timeout/PID cleanup checks. The resource probe stopped before service launch because the exact systemd probe unit is not installed. No cgroup limits were inferred from configuration, and no candidate project code ran. The `prepare` entry remains plan-only. Its separate root helper requires both root and `--confirm`; it is not invoked here. No global WSL settings, sudoers, firewall, disk, or running Polis process were changed.

The per-job profile contract specifies `TasksMax=256` (the cgroup PID controller counts tasks, including processes and threads), a 120-second service lifetime, 1 GiB tmpfs size and 4096 tmpfs inodes. These remain intended settings, not effective qualification results. The probe must read the live control files and hit the actual byte/inode boundaries before the profile can be called qualified.

`scripts/self-dev-go-job-pg-test.sh` adds an isolated harness that refuses inherited Polis DSNs and root execution, uses only a temporary Unix-socket-only cluster, checks schema identity, verifies restricted runtime-role read/write and delete denial, runs the DB-backed package suites, then stops and removes only its exact temporary cluster. Its read-only `inspect` currently reports PostgreSQL 18 `initdb`, `postgres`, `pg_ctl`, `psql`, `createdb`, and share files unavailable, so it correctly reports `NOT_RUN`. The repository-supported preparation path is to place approved PostgreSQL 18 Debian packages in `.tools/downloads` and run `bash scripts/prepare-pg.sh`; the fixed shared `.runtime/linux/pg-data` cluster is not used.

Codex source was consulted as a sandbox reference. Its [Linux sandbox documentation](https://github.com/openai/codex/blob/main/codex-rs/linux-sandbox/README.md) supports the bwrap read-only filesystem, explicit writable paths, PID/network namespace, and seccomp design comparison. It does not establish this slice's cgroup CPU/memory/PID limits or tmpfs capacity/inode limits; those remain independently measured host requirements.

The four existing frontend lint findings were confirmed with the repository's exact `npm run lint` command. Three `no-control-regex` reports were replaced by a shared code-point check with regression tests; the legacy snapshot payload is now projected explicitly instead of destructuring an unused discriminator. The targeted helper tests pass (4/4) and `npm run lint` passes. Full frontend tests/typecheck/build have not yet been rerun after this narrow cleanup.

The changed host and PostgreSQL shell tools pass `bash -n`; the host CLI test passes, including the fail-closed blocked result. The targeted Go packages `gotestprofile`, `workspacepatch`, `workspacepath`, `taskvalidation`, and `db` pass. Full final integrated verification, real PostgreSQL tests, real Go JobRun, persisted Patch delivery, and Workbench API/browser review remain open.

## Continuation checkpoint — PG18, persistence wiring, and final offline checks (2026-10-08)

```ini
repository = D:/Programs/Polis-cloud-main
HEAD = ff34fcd7177d1807e0fd751de332f7344fbcb948
source_integration = PARTIAL
host_execution_profile_qualification = BLOCKED
postgres_migration_and_integration = PARTIAL
real_go_job_execution = NOT_RUN
patch_delivery_and_review = NOT_IMPLEMENTED
provider_surface_impact = NO_CHANGE
self_dev_go_job_patch_offline_slice = NOT_PASSED
provider_egress = 0
real_model_worker_started = 0
live_canary = NOT_RUN
automatic_merge = NO
automatic_deploy = NO
```

The user confirmed that PostgreSQL 18 was not already installed and authorized installation. PostgreSQL 18.6 now lives only in the ignored repository-local `.tools/pg` runtime. The installer downloaded signed Ubuntu Jammy/PGDG packages, verified the PGDG key fingerprint, and extracted them with the existing `scripts/prepare-pg.sh` path. It did not add a system apt source, install a system service, or start a shared cluster. Package and install logs are under `postgres-download/20261008T083511Z-306/`.

The latest isolated PG18 run is `postgres-runtime/20261008T094142Z-8788/`. It proves:

- a fresh Unix-socket-only PostgreSQL 18.6 cluster applied migrations through schema 129;
- the non-superuser runtime role passed the read/insert/update probe and was denied `DELETE` on the dedicated probe table;
- the filtered `TestPeerWorkspaceCheckReturnsActionableSemanticCriteria` passed against that database.

That test pattern is recorded in the evidence. It is a targeted check, not the Go JobRun or Patch delivery integration test. An earlier unfiltered `kernel/control/workbench` PG run failed because existing test fixtures did not satisfy the explicit fixed-team owner confirmation, tool-call budget, and installation worker-slot gates. One fixture path was updated to use those real owner APIs and the targeted run passed, but the full PG package suite has not been rerun and is not qualified as passing. The PG harness stops the disposable cluster and removes its exact temporary data directory after each run.

The PG run exposed a concrete correctness defect in `persistFixedTeamRoleRevisionsTX`: it compared compact Go JSON with PostgreSQL `jsonb::text`. The function now decodes the stored arrays and compares their elements. The passing targeted PG test exercises this owner-confirmation path. The behavioral test fixture also now configures a disposable owner-confirmed company, bounded company/mission budgets, and worker slots before creating its trusted `/bin/sleep` test workers.

The Go profile and WorkspaceTree changes now also include the explicit Git commit/input-manifest binding, candidate snapshot materialization rules, immutable Go JobBinding, a durable **blocked** `go_test` JobRun, read-model projection, and Workbench details. The endpoint cannot produce a successful receipt while the host is unqualified. `workspacepatch` still contains pure bundle/replay logic only. There is no trusted result writer, no `self-dev-go-job-supervisor` command in `cmd/polis/main.go`, no `bin/polis` supervisor binary, no Patch Artifact publication/read path, and no Workbench patch diff or Patch disposition chain. The current source therefore does not complete the requested delivery chain.

The read-only host `prepare` plan reported CPU quota 200%, 2 GiB memory, zero swap, 256 tasks, 120-second service lifetime, and a per-job 1 GiB/4096-inode tmpfs. Those are proposed settings, not effective qualification. The current WSL session lacks CPU-controller delegation, a writable dedicated cgroup subtree, writable `cgroup.kill`, and a hard-capped per-job scratch filesystem. The local bwrap probe passed, but the systemd resource probe unit is missing. No root helper was invoked. Do not run `apply` yet: the fixed systemd unit expects `bin/polis self-dev-go-job-supervisor`, and that command/binary does not exist. No credentials or DB DSN were copied into the unit or candidate environment.

Final offline checks on this source state:

| Check | Result |
|---|---|
| `env -u POLIS_DSN -u POLIS_TEST_DSN bash scripts/go.sh test -count=1 ./...` | PASS; DB-dependent tests were not run |
| `env -u POLIS_DSN -u POLIS_TEST_DSN bash scripts/go.sh test -race -count=1 ./...` | PASS; DB-dependent tests were not run |
| `env -u POLIS_DSN -u POLIS_TEST_DSN bash scripts/go.sh vet ./...` | PASS |
| Linux `bash scripts/go.sh build ./...` | PASS |
| Windows amd64 `GOOS=windows GOARCH=amd64 bash scripts/go.sh build ./...` | PASS |
| Frontend `npm test -- --run` | PASS, 157/157 |
| Frontend typecheck / lint | PASS / PASS |
| Frontend production build | PASS; Vite reports the existing large-chunk advisory |
| Host CLI tests and shell syntax checks | PASS |
| Migration manifest verification | PASS, 129 pinned migrations, 0 unpinned |
| `git diff --check` | PASS |

The prior reconciliation document remains unmodified and unstaged. `D:/Programs/Polis` was not accessed. No candidate project code, real Go JobRun, real Worker, provider, or external retrieval ran.

### Exact remaining boundary

The `prepare` plan now checks both `cmd/polis/main.go` for the supervisor command and `bin/polis` for the executable. It reports `apply_ready=no` and `apply_command=not_available_until_supervisor_build` while either is missing. The root helper and systemd units were not invoked.

The next source work is a trusted supervisor/JobRun result path and the formal Patch Artifact/DeliveryManifest/Workbench diff integration. The host step must wait until the supervisor command is implemented and built. After that, the owner must review the exact systemd/polkit plan and explicitly authorize the targeted root apply; then run `bash scripts/self-dev-go-job-host.sh verify`. Candidate code remains prohibited until effective CPU, memory/swap, task-count, scratch-byte/inode, network, timeout, and whole-process-tree cleanup probes pass. The PG-backed Go JobRun/Patch E2E and browser review remain unrun.

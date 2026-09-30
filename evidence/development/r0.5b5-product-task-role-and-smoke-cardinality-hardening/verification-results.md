# R0.5B5 Verification Results

Status: all requested offline verification passed. This record does not authorize a live provider run.

## Domain and control-plane tests

- `run-targeted-validation.sh` — PASS; detailed output in `targeted-validation.log`.
- Core TaskKind predicate and pure provider-task selector — PASS.
- Provider WorkerSession rejects `bootstrap_plan` — PASS; no session created.
- Existing per-Mission/per-kind uniqueness prevents a second `compat` — PASS.
- A provider Task can create only one WorkerSession, including after stop — PASS.
- TaskValidationBinding update/delete attempts — DENIED; exactly one binding remains with its original digest.
- Internal-task provider authorization — DENIED; `CodexRuntime.Reserve` creates no allowance file.
- Product Start, malformed fan-out, duplicate Start, and failed-start replay — PASS.
- Workbench projection of both Task kinds — PASS; `workbench_change = NOT_REQUIRED`.
- Independent code review — zero Critical, Important, or Minor findings in the B5 paths.

## Offline PostgreSQL Start path

The disposable PostgreSQL 18.6 cluster reported `fsync=on`, `synchronous_commit=on`, and schema version 7. The actual fake-transport Start snapshot is in `offline-start-path-test.log`; targeted integration results are also in `targeted-validation.log`.

Observed result: Mission `active`; 2 Task rows (`bootstrap_plan=completed`, `compat=candidate`); 1 provider-executable Task; 1 stopped WorkerSession for `emp-backend` bound to the `compat` Task; 1 immutable TaskValidationBinding with a passing product check. `FakeRuntime` observed one reserve-interface call, one process-start call, and one logical turn. The fake runtime reports zero provider egress; real provider reservations, real egress, live Medium, and live High are all zero.

The database cluster is a dedicated temporary fixture identified by `postgres-temp-root.txt`; it is stopped and removed after all integration checks. Its logs and migration record remain in this directory.

## Repository verification

- `bash scripts/go.sh test ./...` — PASS (`go-test.log`).
- `bash scripts/go.sh test -race ./...` — PASS (`go-race.log`).
- `bash scripts/go.sh vet ./...` — PASS (`go-vet.log`).
- Linux and Windows amd64 command builds — PASS (`go-linux-build.log`, `go-windows-build.log`).
- Frontend tests — PASS, 19 tests; typecheck, lint, and build — PASS (`frontend-*.log`).
- Bash syntax for the R0.5B5 shell scripts — PASS.
- `git diff --check` — PASS.
- No PowerShell or Python source files were changed, so their syntax checks were not applicable.

## Product surface freshness

The verbose exact-surface test output is in `surface-freshness-test.log`. It recomputed and matched the unchanged `polis-product-tool-surface@2` surface: 7 tools, manifest `2b403fc0f3c9a1513473828e66203becb7ac5202f298f917034fd95929a9f3b9`, 1470 aggregate schema bytes, and digest `8f2e1ee8ba8c8d456af9ce9839f62a854bfe7c9f5847613657ccd7f77a9da04f`.

No Codex-facing tool bytes changed. B4's provider L2 qualification remains `QUALIFIED_REUSABLE`, with its existing fingerprint `4c206409f827211cb9f7b18d37da5d096efb7f735c90eaed15e4692128d6a532`; no canary was run. B2 remains `STALE`; R0.3A surfaces remain `HISTORICAL / NOT_REUSABLE`.

## Live boundary

- LIVE_1 remains the frozen pre-Start `INCONCLUSIVE` result with zero provider authorization, reservation, egress, turn, and WorkerSession.
- LIVE_2 was not started.
- B5 sent no provider traffic and consumed zero live Medium or High turns.
- LIVE_2 eligibility is a boundary result only; a separate explicit authorization is still required to run it.

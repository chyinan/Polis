# POLIS self-development Go JobRun / Patch slice — final verification

Date: 2026-10-26
Result: `PASSED_LOCAL_E2E` (local workspace only; committed and pushed after explicit user request)

## Completed chain

The final run `selfdev-e2e-20261026T063547` exercised the real `RealWorkbenchApi`, production Go Workbench handler, disposable PostgreSQL 18.6 / Schema 134, and Windows Playwright browser:

1. The current boot-bound root receipt qualified the fixed Linux/amd64 systemd + Bubblewrap host. All resource/path/network/process-tree probes passed.
2. Workbench submitted one explicit `executeGoTest=true` request. JobRun `job-cdbb806b0e578419a30b513d6d80a742` exited 0 with both frozen tests passing, a real supervisor receipt, `qualificationEligible=true`, and cleanup confirmed.
3. The API committed replay-verified `go_patch` Artifact `1edfbc093b55f7345cd14d71cacc4e26`, with its validation qualification and DeliveryManifest revision 1.
4. The installation-owner Workbench completed the evidence-bound DeliveryManifest to revision 2 (`ready`), read the read-only diff in the browser, submitted an `accepted` disposition, and read the durable lifecycle back. This is a synthetic disposable fixture decision only; it is not acceptance of a business Artifact. Source apply, merge, and deploy did not occur.

Browser evidence: `selfdev-e2e-20261026T063547/` (`browser-complete.json`, `go-patch-workbench-diff.png`, `go-patch-workbench-disposition.png`, `systemd-job-request.json`, `systemd-job-result.json`, and matching `systemd-job-handoff-sha256.txt`). The systemd result independently records the exact JobBinding, current host qualification digest, frozen test events, and empty process-tree cleanup.

The disposable deterministic WorkerSession used only to seed the Task workspace was DB-confirmed active before snapshot creation and confirmed stopped before PostgreSQL cleanup. No real-model Worker or Provider was started.

## Host qualification identity

- Boot ID: `89e82fea-58e1-44c3-a79d-cc96ec7d8fcd`
- Supervisor SHA-256: `4e241c70eed64ee138769b5bf191eee18ee9fa167295a3f6a3bdfa47171c7fb3`
- Supervisor source identity: `6be9cdf873e84f7699938332fd79c8999a857a539ee23b9ce1addcc4989971ab`
- Unit-control helper SHA-256: `186bb99f606abbd2604480f1d2047936a7414ac97169f59977992af0853d7b89`
- Qualification SHA-256: `62e82a5882918d2fecacd296d5d30d6b28884c897f064f74dc43e5f712b0e9f4`
- Probe bundle SHA-256: `7412bd2ed5fe86d89de3d1e193b1e9d0a4ebefe1dda0af8c31ca4ca75951b7c4`
- Read-only staged toolchain tree SHA-256: `7643befa386325ee8037168d2a6d7f3887677c2e3bd4f096c4a3917f8a78cc23`

Receipt: `host-qualification-20261025/root-qualification-receipt.json`. Probe: `host-qualification-20261025/qualification-probe-root-schema134-disposition-fix.log`.

The `/run/polis-self-dev-go-toolchain` root-owned 0555 stage is required: identical tree inspection from the Windows-mounted DrvFs source took 151248ms, while the staged tree took 451ms. The systemd unit and API used the staged tree. The PKLA's root-only parent permissions were not broadened; `pkexec --verify-installation` validates the protected `.pkla` digest as root.

## Offline verification

| Check | Result | Evidence |
|---|---|---|
| Go tests, all packages | PASS | `evidence/development/polis-self-dev-v1-go-patch-delivery-20261009-broker-20261026/go-tests-schema134-accepted-route-20261026.log` |
| Go race suite, all packages | PASS | `.../go-race-schema134-accepted-route-20261026.log` |
| Go vet, all packages | PASS | `.../go-vet-schema134-accepted-route-20261026.log` |
| Linux / Windows amd64 command builds | PASS / PASS | `.../linux-build-final-20261026.log`, `.../windows-build-final-20261026.log` |
| Full PostgreSQL 18.6 integration, Schema 134 | PASS: 1941 pass events, 0 failures, 65 skips; server stopped | `evidence/development/polis-self-dev-v1-go-patch-delivery-20261009-schema134-acceptance-20261026/postgres-runtime/20261025T215354Z-36955/` |
| Migration-hash updater tests | PASS | `scripts/test-migration-hash-manifest.ps1` |
| Frontend | PASS: 164 tests, typecheck, lint, production build | final build reports the existing large-chunk advisory |
| Disposable full E2E PostgreSQL harness | PASS: Schema 134, least-privilege probe, server stopped | `evidence/development/polis-self-dev-v1-go-patch-delivery-20261009-real-e2e-20261026/postgres-runtime/20261025T223550Z-71394/` |
| `git diff --check` and host-script syntax | PASS | final local verification |

## Safeguards and scope

`provider_egress=0`; `real_model_worker_started=0`; `automatic_apply=NO`; `automatic_merge=NO`; `automatic_deploy=NO`; no business database or external account was used. The Patch remains a content-addressed Artifact and was never applied to the source repository. No commit or push was made.

Intermediate blocked/failed fixture attempts and their receipts/results remain preserved in separate `selfdev-e2e-*` directories; no unknown JobRun RequestID was retried. The final clean pass is the run identified above. This local slice does not qualify broader R1–R3 acceptance or any real-provider/production workflow.

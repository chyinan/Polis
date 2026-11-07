# R0.3A real Backend Employee — bounded attempt

## Current result

The initial attempt remains historical `NOT_STARTED` at the business execution boundary. It consumed no allowance and made no provider egress. Its passed offline preflight is preserved.

The explicitly authorized continuation also stopped before allowance/Worker/provider. Dedicated PG start/create/migration/grants/runtime connection passed, qualification/auth/tool/config revalidation passed, and the exact binding then correctly rejected `problem_key_mismatch`: the continuation script had generated `r0.3a-real-peer-collaboration-v1`, while the formal `R03AT2ProblemKey` is `r03a-real-peer-collaboration-v1`. No model turn or Polis business tool event occurred. The continuation result is bounded `INCONCLUSIVE` with `backend_real_execution=not_run`; it is not a model retry.

Evidence currently contains only:

- `evidence/development/r0.3a-real-backend-employee/luna-1/preflight.json`
- `evidence/development/r0.3a-real-backend-employee/luna-1/continuation-freshness.json`
- `evidence/development/r0.3a-real-backend-employee/luna-1/qualification-revalidation.json`
- `evidence/development/r0.3a-real-backend-employee/luna-1/runtime-connection.json`
- `evidence/development/r0.3a-real-backend-employee/luna-1/result.json`

There is no `allowance.json`, worker session, provider protocol, contract, message, obligation, checkpoint, or artifact from either attempt.

## Offline work completed

- Added v7/T21B revalidation before any business problem-key, allowance, or worker write: current fingerprint, qualification status, binary/helper/config hashes, auth identity/revision, and formal `codex.PeerBackendTools()` 11-tool surface.
- Added Windows-native Go process and environment support with redirected stdio, temporary HOME/CODEX_HOME, auth snapshot handling, proxy filtering, and Windows process-handle stop proof.
- Added the Backend-only Windows entrypoint and one-shot PowerShell orchestration with exact `emp-backend`, ProblemKey, purpose, model/profile/effort, `1 Medium / 0 High`, concurrency 1, no retry/reset, no Frontend/High.
- Fixed the dedicated WSL PostgreSQL helper to use Linux `/tmp`, explicit socket location, dynamic library paths, and in-WSL DSN construction.
- Added the v7 revalidation regression test; it was observed failing before implementation and passing after implementation.
- `bash scripts/go.sh test ./...` passed; `bash scripts/go.sh build ./cmd/...` passed; Windows cross-build and PowerShell syntax checks passed.
- The first continuation freshness record predates the guard's binding check; the guard now requires exact binding before PG, and the orchestration script's ProblemKey typo is fixed offline.

## Required continuation boundary

The migration/WSL defect and binding typo are fixed offline, but the real Backend entrypoint was not rerun after the second pre-start rejection. The current evidence path is sealed for retry purposes because it contains the continuation result. A future explicitly authorized attempt must use a fresh evidence path/attempt identity, re-run freshness with the corrected binding guard, and must not recreate/reset an allowance or start Frontend/High automatically.

The new authorized `continuation-2` used an independent evidence path and passed freshness/binding, PG migration/grants, and runtime connection. It created the one business allowance with `medium_turns=0`, then stopped before Worker/provider while initializing the disposable blob store: `os.Root` directory `Sync` at `internal/kernel/blob.go:74` returned Windows `Access is denied`. The result is `INCONCLUSIVE`/`backend_real_execution=not_run`; no model/tool turn occurred. The run is sealed and must not be retried. The disposable blob residue was later cleaned through its verified owned path; no business DB or provider side effect remains.

## Blob durability qualification

`R0.3A Blob Durability Windows Qualification` is `PASSED` with zero Medium/High/provider usage. `putBlob` now enforces file-content `Sync`, atomic rename/finalization, and parent-directory sync as separate stages. Unix keeps strong parent-directory fsync; Windows explicitly classifies only the observed parent-directory `ERROR_ACCESS_DENIED` as unsupported, reports a weaker crash-consistency contract, and still propagates file/rename/other parent-sync failures.

Regression injection tests cover file sync, rename, parent sync, stage cleanup, and traversal. Linux real filesystem smoke passed with strong parent sync. Windows native filesystem smoke passed with correct content/hash, final digest, no stage files, owned cleanup, and `parent_directory_sync_outcome=explicitly_unsupported`. Evidence: `evidence/development/r0.3a-blob-durability/qualification.json` and `evidence/development/r0.3a-blob-durability/windows-smoke-v3.json`.

The continuation-2 disposable blob residue was cleaned through the verified owned path only; no evidence or artifact was deleted. `eligible_for_backend_continuation_3 = YES`, but no Backend starts without a separate fresh evidence path and allowance authorization.

The authorized continuation-3 created a new evidence path and preserved its preflight, then stopped before PG at the freshness/blob gate. The Blob qualification itself is `PASSED`, but the loader expected flat fields while the qualification record stores `durability_contract` and `windows_native_filesystem_smoke` as nested objects. This was treated as stale/incomplete; no PG, allowance, Worker, provider, or Backend turn started. The path is sealed with only `preflight.json`; no correction-and-rerun occurred under continuation-3.

## Continuation-4 real Backend result

Continuation-4 used a fresh evidence/runtime path. Freshness/binding/blob source digest, PG schema/grants/runtime, final authorization, and the new allowance all passed. One real `gpt-5.6-luna/medium` Backend turn started for `emp-backend` and used 32 real Polis tool calls. It proposed and accepted a ContractRevision v2 that was later read back as accepted, changed its workspace through persisted writes, and sent a direct actionable `collab_send` to the `emp-frontend` task; Message/Obligation data was returned as persisted. However, every candidate `workspace_check` returned `passed=false`, `work_checkpoint` returned `POLICY_DENIED`, `artifact_submit` returned `POLICY_DENIED`, and the model continued revising until the harness returned `tool-call limit exceeded`. The process stop was confirmed, but no `turn/completed`, successful checkpoint, or frozen artifact was produced.

This is a real business `FAILED` result, not a transport failure and not an automatic retry. The DB was disposable and cleaned; evidence is preserved in `evidence/development/r0.3a-real-backend-employee/continuation-4/`, especially `result.json`, the Backend `protocol.jsonl`, and `r03a-session.json`. No Frontend, successor, High Reviewer, or additional model call was started. `eligible_for_frontend_phase=false`.

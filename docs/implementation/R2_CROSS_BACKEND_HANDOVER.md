# R2 cross-backend Task handover

## Scope

Schema 39 adds a one-use, append-only handover for a single `compat` Task between the fixed `windows-node-npm@1` and `linux-node-npm@1` profiles. It transfers only persisted, profile-neutral state. It does not copy a process, shell, package-manager cache, browser, WorkerSession, or provider conversation between hosts.

The handover snapshots the source JobRun and environment revision, source WorkerSession and runtime incarnation, Task workspace digest/revision, Task input-manifest digest, common project/package/lockfile digests, and the approved target policy and executor fingerprint. `record_sha256` covers the immutable identity and snapshot fields. Workbench reads verify that digest before returning a record.

The source runtime incarnation is provenance for the stopped WorkerSession. It identifies the Polis process and is not used as an executor-host identity. A resumed WorkerSession may share that Polis incarnation while the selected, separately qualified target environment determines the Node execution profile. The new WorkerSession ID, target revision, and persisted-state checks prevent carrying the stopped session forward.

## Creation gate

The Control API creates a record only when all of these conditions hold inside the company-scoped transaction:

- Mission is formally paused; the Task is a working backend `compat` Task.
- Every WorkerSession in the Mission is stopped.
- The source JobRun is the latest JobRun for the Task and has a known terminal state: `exited`, `failed`, or `cancelled`.
- No JobRun in the Mission is accepted, starting, running, or `outcome_unknown`.
- No Mission environment preparation is active or `outcome_unknown`; no Mission input is uploading; no unrevoked service endpoint lease remains.
- Target revision belongs to the same Mission and uses the other fixed profile. Source and target bind the same immutable MissionInput revision, project root, source digest, package digest, and lockfile digest.
- Target source bytes verify from CAS, its policy is currently approved, its exact executor fingerprint is qualified, and its latest preparation is ready.
- Current Task workspace and input manifest exist. The input manifest has passed the same integrity check as Worker input delivery.

The record is immutable and unique to its source JobRun. A rejected request leaves no record.

## Consumption gate

The first target-profile JobRun must include `handoverId`. Control rereads the record and confirms its digest, Task, source JobRun, target revision/profile, current Mission state, source WorkerSession stop/incarnation, current workspace version, and input-manifest digest. The Mission must have been resumed and a distinct active WorkerSession must exist on the target runtime. If no active session exists, JobRun creation makes a `project_job`-mode WorkerSession in the same transaction as the JobRun. This mode is reserved for the authorized local project executor; it does not start or authorize a model turn. Terminal JobRun events stop that session, and an unknown outcome moves it to `reconcile_required` until Stop is confirmed. Current target policy, qualification, and preparation gates are checked again by the normal JobRun start path. A partial unique index makes each handover single-use.

When `ResumeMission` finds an unconsumed cross-backend handover, it resumes the Mission without invoking provider readiness or `Worker.Start`. The selected target executor is checked when the JobRun starts. This keeps handover resume available without reserving or sending a model request.

JobRun creation without a handover is denied when the most recent Task JobRun used the other profile. Same-profile JobRuns keep their existing path after known terminal results. Any unresolved `outcome_unknown` JobRun for the Task blocks all new JobRuns until stop/reconciliation records it as `cancelled`; unknown is not treated as a terminal checkpoint and cannot be used as a cutover source.

## Workbench API

- `GET /api/workbench/companies/{companyId}/tasks/{taskId}/environment-handovers` lists verified records.
- `POST` to the same route accepts `sourceJobId`, `targetEnvironmentRevisionId`, and `requestId`.
- Task JobRun start accepts an optional active `sessionId` and the selected `handoverId`; when no WorkerSession is active, Control creates the project-job-only session transactionally. Its persisted JobRun projection returns the generated `sessionId` and consumed handover ID.

The UI offers only target revisions whose source, package, and lockfile digests match the selected source profile and whose policy, executor qualification, and preparation are currently ready. The server repeats every decision-relevant check.

## Qualification boundary

The PostgreSQL integration uses disposable records and a fake executor qualification/preparation event to verify the contract and database gates. It does not execute a Linux or Windows project process and does not qualify either host profile. In this workspace the Linux/Node executor remains unqualified unless separately evidenced. Real host, isolation, restart, and multi-day qualification remain open.

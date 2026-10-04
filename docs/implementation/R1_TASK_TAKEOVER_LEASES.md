# R1 Task takeover leases

## Purpose

An operator can inspect and replace a stopped Task's bounded text workspace while the Mission is paused. The old Task workspace remains immutable. A returned snapshot is stored as a company-CAS-backed MissionInput and can be carried into a successor only through the reviewed Mission change-request path.

## Lifecycle

1. Pause the Mission and stop its WorkerSessions, JobRuns, and service endpoint leases.
2. Grant one immutable lease for a Task. It binds the current Mission requirement digest plus the exact workspace digest and revision.
3. Load the workspace through the authorized Workbench API. The editor refuses a digest, revision, or file-shape mismatch. A strict UTF-8 unified patch may be imported only for the existing `workspace.txt`; every hunk must match the frozen text. The Workbench previews frozen and proposed content side by side.
4. Return a bounded UTF-8 text snapshot with the frozen digest/revision. The Kernel rechecks Mission state, requirement/input digest, writers, active lease, and workspace CAS inside the write transaction.
5. Store the snapshot as a MissionInput with immutable lease event, byte/diff summary, and optional bounded human-effort value. Release the active Task slot only with the returned event.
6. Create and review a formal Mission change request after the handback. Applying it carries the snapshot into a draft successor with `origin=human_takeover` and source Task lineage.

An operator can instead release a granted lease without a snapshot. Returned and released history is append-only; the migration refuses downgrade while lease or provenance history exists.

## Workbench API

- `GET /api/workbench/companies/{company}/missions/{mission}/takeover-leases`
- `POST /api/workbench/companies/{company}/missions/{mission}/tasks/{task}/takeover-lease`
- `POST /api/workbench/companies/{company}/missions/{mission}/takeover-leases/{lease}/snapshot`
- `POST /api/workbench/companies/{company}/missions/{mission}/takeover-leases/{lease}/release`

Every route requires the desktop session token even in tokenless loopback mode. Task workspace reads also require the token because they return content. Snapshot requests are capped by the common JSON body limit and the Kernel's 4 KiB UTF-8 content limit. No host path is accepted and the Workbench does not edit files directly.

## Qualification boundary

Schema 37 provides the append-only lease, active-slot, and event records. Dedicated temporary PostgreSQL tests cover a live-Worker denial, grant only after stop, stale base rejection, successful snapshot return, formal change application, source lineage, successor Task input delivery, and downgrade protection. Frontend request/response validation binds all records to the expected Mission and exact frozen workspace version.

This is a bounded single-text-workspace handback. The patch parser rejects other paths, multiple files, stale hunk context, invalid UTF-8, no-op hunks, and results above the existing 4 KiB handback limit; imported text is never executed. The patch does not write the old Task workspace. This does not claim general filesystem editing or automatic semantic impact analysis. Natural-language change impact remains `not_assessed`; the human snapshot does not resume execution or approve a change.

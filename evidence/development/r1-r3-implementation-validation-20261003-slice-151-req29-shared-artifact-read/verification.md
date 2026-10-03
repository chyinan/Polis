# Slice 151 — REQ-29 same-Mission shared Artifact reads

Date: 2026-10-03

## Implementation

- Added `mission_artifacts_list` and `mission_artifact_read` to a separate 14-tool, fake-only @8 product Worker surface; @4/@7 definitions and hashes remain untouched.
- The listing derives scope from the database-bound active provider WorkerSession and current owned working Task, filters to another Task in the same active Company+Mission, and returns at most 32 digest-pinned ready candidate/passed Artifacts per page.
- Each listed CAS reference is read and checked against the recorded digest and byte size. Exact-ID reads repeat the database/session scope check, cap content at 4096 bytes, verify digest and size, require UTF-8 and label returned content untrusted.
- Successful reads append the Artifact ID/digest, source and reader Task, Employee, WorkerSession and receipt to the append-only event ledger. @8 Handover exposes up to 32 read references for the current Task.
- `POLIS_OFFLINE_SHARED_ARTIFACTS_ENABLED=1` selects @8 only with `POLIS_WORKER_MODE=real` and `POLIS_PROVIDER_TRANSPORT=fake`; it is off by default. Real-provider authorization remains @4. No SQL migration, filesystem path access or external action was added.

## Checks and limits

| Check | Result |
| --- | --- |
| Go build: `./cmd/polis`, `./internal/codex`, `./internal/kernel`, `./internal/provider`, `./internal/control` | PASS |
| `git diff --check` | PASS |
| Schema/migration change | None; local database remains Schema 98 |
| Tests or populated WorkerSession E2E | Not run |
| REQ-29/CAP-01–06 | Partial; directory trees, workspace snapshot/GC races and qualification remain open |

@8 pin: manifest `4e1e75bd7f6753d5dc55b443e60a9959e7aebd953a3ee29c16a6559e821e8cc3`; 14 tools; aggregate schema 3813 bytes / `daf4523db81f860aeffdc5e7137b86fd6f4108f12592dceaa9ba74791b3b66bf`.

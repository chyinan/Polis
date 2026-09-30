# R3 local content-operations workflow

> Updated: 2026-09-29, Slice 77 / Schema 60. This is a company-scoped local evidence workflow. Profile qualification remains `not_run`; execution stays disabled.

## Workflow

- Owners explicitly authorize or revoke exact usable text/JSON MissionInput revisions. The database keeps append-only authorization events; the current event controls whether a later fact-check may cite that exact digest.
- Drafts bind an author, exact body MissionInput revision/digest, unique critical-claim IDs, and the author's mechanical-constraint declaration. A later MissionInput revision creates a new immutable draft version.
- A review requires a different company employee with the `review` role. Verified claims cite currently authorized exact source revisions; the independent human sample binds a same-company authorized plan, draft revision/digest, reviewer and selected critical claims. The deterministic contract preserves `accepted`, `inconclusive` and `rejected` distinctly.
- Simulation publication requires an accepted, current review. It writes a local `simulation` receipt that explicitly sets `externalSideEffects=false`; it never connects to or posts to a publishing platform.
- Corrections reference a simulated publication and the latest registered draft revision newer than that publication, then enter `review_required`. A correction requires a fresh accepted review linked to that correction and recorded afterward before the corrected draft can be simulated-published. Feedback is an append-only internal note tied to a simulated publication. Negative and correction-request feedback are marked `review_required`; the system does not create a Task or publish a correction automatically.

## Persistence and boundaries

Schema 56 stores source-authorization events, immutable draft versions and independent review decisions. Schema 57 prevents revoking a source unless its current state is authorized. Schema 58 stores simulation receipts, corrections and feedback. Schema 59 adds the database-side sample-plan authorization barrier before a publication receipt is inserted. Schema 60 binds a post-correction review to the exact correction and prevents a pre-correction or unrelated review from publishing the corrected draft. The Kernel re-reads MissionInput/CAS bytes, verifies company scope and digests, rechecks current authorization under the company write lock, and enforces the fixed review role.

The Workbench operates on current-mission text inputs, exposes source, draft, review, simulation, correction and feedback actions, and marks prior reviews stale after a newer draft is registered. It keeps the qualification and execution fields closed. The fixture API rejects every write. No model, external source, external publisher, account, social platform, automatic Task or notification is used.

## Verification

The disposable PostgreSQL 18 `scripts/r3-domain-evidence-postgres-test.sh` path migrates through Schema 60 and exercises source authorization/revocation, exact draft revisions, independent review/sample binding, stale-review refusal, simulation replay, no external side effect, correction-linked re-review, correction handoff to review-required, feedback state, company isolation, and no profile qualification. Evidence: `evidence/development/r1-r3-implementation-validation-20260929-slice-77-r3-content-operations/verification.md`.

This verifies software contracts only. It does not establish editorial quality, source licensing, audience benefit, platform publishing permission, or R3 domain/runtime qualification.

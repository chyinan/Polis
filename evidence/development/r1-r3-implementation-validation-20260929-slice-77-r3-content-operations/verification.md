# Slice 77 verification — local content operations

Date: 2026-09-29

## Implemented boundary

- Schema 56 stores append-only company source-authorization events, exact MissionInput draft revisions, and independent fact-check/human-sample records. The Kernel reads bounded CAS bytes and validates source/reviewer/draft bindings.
- Schema 57 refuses source revocation unless the exact source revision is currently authorized.
- Schema 58 stores local simulated-publication receipts, newer-draft corrections, and internal feedback records. Publication is simulation-only with `externalSideEffects=false`; corrections remain `review_required`; negative or correction-request feedback is recorded as review-required without creating work or sending notifications.
- Schema 59 adds a database barrier requiring the exact sample-plan revision to remain authorized when a simulated publication is recorded.
- Schema 60 makes reviews correction-aware: a corrected draft needs a fresh accepted review linked to that exact correction and ordered after it; the database rejects publication based on an earlier or unrelated review. Kernel correction recording also requires the latest registered draft revision, and the Workbench hides superseded revisions from the correction selector.
- Newer draft revisions make their prior fact-checks stale. Stale reviews cannot be simulated-published. Content/research profile qualification remains `not_run`, and execution remains disabled.
- No content was posted externally. No model, platform account, outbound notification, GitHub, MCP, or remote publisher was used.

## Verification

- `rtk bash scripts/r3-domain-evidence-postgres-test.sh` — passed on disposable PostgreSQL 18 through Schema 60. The content tests verify company-scoped source authorization and revocation, exact draft revision/digest, independent reviewer and human sample, idempotent review replay, stale review refusal, simulation receipt persistence with no external side effect, refusal of a superseded correction target, rejection of a pre-correction review, acceptance of a fresh correction-linked review, correction handoff, feedback state, readback, and no qualification change. The script's control test filter explicitly matches both `TestContentOperations` and `TestContentCorrection`.
- `rtk bash scripts/go.sh test ./... -count=1` — passed across all Go packages.
- Frontend Vitest — 123 tests passed across 13 files.
- `npm run typecheck`, `npm run lint`, and `npm run build` — passed. Vite emitted its bundle-size advisory (main minified chunk is 712.46 kB after minification).
- Linux and Windows amd64 command builds — passed.
- `scripts/test-migration-hash-manifest.ps1` and `rtk git diff --check` — passed after pinning the final Schema 60 migration body.

The first disposable migration attempt exposed a hard-coded generated-constraint name mismatch; Schema 60 now resolves the exact unique constraint by catalog definition. The first control integration run exposed an inverted assertion for `externalSideEffects`; it now asserts the intended false value. Test discovery also showed that the original script filter skipped `TestContentCorrection`, so the filter now includes it. Go's `run` invocation segfaulted in this WSL/Windows-mounted workspace; the script now builds `polis` into its disposable `/tmp` root and invokes that binary. These were local-only attempts; disposable database roots were cleaned and no external calls were made.

- Independent Slice77 review reported two P2 issues: pre-correction reviews could publish corrected drafts, and corrections could target superseded drafts. Both were fixed and the read-only re-review found no remaining functional issues. The UI's latest-draft selection was source-reviewed but has no direct component assertion in the current frontend test suite.

# R1 input, QQ dispatch and intervention UI verification

Date: 2026-09-23

## Go and PostgreSQL

- Migrated the dedicated PostgreSQL 18 test database `polis_r0_r1_resume_20260923_1732` through schema 16 on the isolated `/tmp/polis-r1-input-test-pg` cluster.
- `POLIS_TEST_DSN=... bash scripts/go.sh test -p=1 -count=1 ./...` passed. The suite ran serially because the kernel intentionally holds one database-wide advisory lock per runtime instance; parallel Go package tests contend for that lock.
- After the full-suite run, the empty evidence projection regression test was added. `bash scripts/go.sh test -count=1 ./internal/qqnotify ./internal/workbench` passed against the final source; the dedicated PostgreSQL feedback-page smoke below also read the empty-evidence projection through the live API.
- `bash scripts/go.sh vet ./...` passed.
- `bash scripts/go.sh build ./cmd/...` passed.
- `GOOS=windows GOARCH=amd64 bash scripts/go.sh build -o /tmp/polisd-resume-final-amd64.exe ./cmd/polisd` passed.
- `rtk git diff --check` passed.

## Frontend and browser

- `npm run test`: 39 tests passed across 9 files.
- `npm run typecheck`, `npm run lint`, and `npm run build` passed.
- `scripts/r1-input-browser-smoke.py` passed against the real Workbench and deterministic worker. It verified text and directory snapshot upload/readback, disabled/unqualified QQ route draft, Mission lifecycle, and `externalProviderCalls=not-run`.
- `scripts/r1-human-intervention-browser-smoke.py` passed against a seeded company-scoped intervention and pending notification intent. It verified the Feedback page displayed the pending notification, acknowledgement changed the workflow to `acknowledged` and notification to `superseded`, resolution removed the item from Attention, and no QQ send occurred. Screenshot: `human-intervention-feedback.png`; result: `feedback-browser-smoke.json`.
- A regression test first failed because an intervention with no evidence was serialized with `evidenceRefs: null`, which the frontend correctly rejected. The projection now emits an empty array; the focused Go test and browser smoke passed after the fix.

No QQ credential was present, and no real QQ, model-provider, MCP or GitHub request was made. The local backend, Vite server and dedicated PostgreSQL cluster were stopped after verification.

# R1 bounded notification retry verification

Date: 2026-09-24

## Retry and recovery behavior

- The `polis` control service starts its retry worker only when the QQ sender is explicitly enabled and injected. It scans a bounded batch of 50 notices every five seconds, including immediately on startup.
- A notice is retryable only while its HumanIntervention remains open, its outbox intent is pending, its last delivery is due in `retry_wait`, and its exact QQ route revision, target and qualification are still current.
- A retry persists a new single-use delivery attempt and consumes the existing maximum of three attempts. Ambiguous provider outcomes remain terminal and are never retried.
- Expired crash-left `sending` rows are reconciled in bounded batches to `outcome_unknown`; their intents become terminal and leave the retry queue.
- A dedicated PostgreSQL integration test returned a fake rate-limit result, persisted a due retry, closed the first control service, created a new control service, and verified the second fake send and provider receipt were persisted. Unqualified routes were also verified not to reach the sender.

## Verification

- Schema 16 full Go suite passed serially: `POLIS_TEST_DSN=... bash scripts/go.sh test -p=1 -count=1 ./...` on dedicated database `polis_r0_r1_retry_20260923_2357`.
- `bash scripts/go.sh vet ./...` passed.
- `bash scripts/go.sh build ./cmd/...` passed.
- `GOOS=windows GOARCH=amd64 bash scripts/go.sh build -o /tmp/polisd-retry-final-amd64.exe ./cmd/polisd` passed.
- `rtk git diff --check` passed.
- Frontend tests/typecheck/lint/build and the real Feedback page browser smoke are recorded in `verification-resume-2026-09-23.md`; the frontend has not changed since those runs.

No real QQ credential or provider request was used. The route remains disabled by default, and the isolated PostgreSQL cluster was stopped after the run.

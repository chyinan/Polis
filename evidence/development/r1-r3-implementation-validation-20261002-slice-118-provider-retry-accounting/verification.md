# Slice 118 — REQ-16 provider retry accounting

## Change

- The Codex transport now returns its observed tool-call event count on success and error paths.
- The Codex provider adapter carries that count, reconnect count, and same-turn recovery marker into the provider `TurnResult`.
- Immutable `provider_terminal` Worker observations persist those fields and `retry_visibility: limited` for the Codex CLI path.
- The visibility marker is required because the Worker can count protocol-visible reconnects but cannot claim to see every retry internal to the CLI/provider harness.

## Verification

- `gofmt` on the five changed Go files: passed.
- `go build ./cmd/...` on the local Android/Termux environment: passed.
- `git diff --check`: passed.
- No tests were added or run. No PostgreSQL migration/runtime or provider session was executed.

## Boundary

This records observed protocol activity; it does not create a billable cost total, guarantee a hard cap over hidden provider retries, or implement Mission/Company/Provider budget composition. Durable ProblemKey/task lineage, closing reserves, and stable exhaustion closeout/recovery remain open under REQ-16.

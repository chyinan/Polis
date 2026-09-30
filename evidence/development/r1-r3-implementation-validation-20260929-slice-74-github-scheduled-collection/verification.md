# Slice 74 verification — opt-in scheduled GitHub collection

Date: 2026-09-29
Schema: 53

## Implemented

- Schema 52 adds append-only company/source collection-policy history, durable `next_poll_at` state, and unique scheduled attempt slots. Owner policy requires an approved source with verified read permission, a rationale, and a cadence from 15 minutes to 24 hours.
- `polisd` starts the scheduler only when both `POLIS_GITHUB_READONLY_ENABLED=1` and `POLIS_GITHUB_FEEDBACK_SCHEDULER_ENABLED=1` are set and protected credential storage is available. The default remains off. A tick handles at most three sources and uses the existing fixed-host GET-only bounded poller.
- Schema 53 records an atomic dispatch claim under the same Kernel company lock used by policy/source decisions. The claim checks the company is active, the current policy is enabled, the source remains approved with verified permission, and the Kernel advisory lease/runtime incarnation is current. It marks an attempt `started` before any GET. Policy disable, source revoke or company archive that wins the lock first prevents dispatch.
- Pre-dispatch expiry becomes a known failure; only an expired `started` attempt becomes `outcome_unknown`. Same-slot replay is blocked. Expiry cleanup commits in its own transaction before company locks to preserve lock order. The reservation lease is five minutes for up to three serial, 90-second bounded polls.
- The Workbench reads policy, next poll and attempt state; its explicit company policy action and every company `/feedback` route require the desktop session token in tokenless mode. Fixture API writes remain unavailable.
- Collection only appends bounded observations to the company backlog. It does not close remote issues, create Tasks, restart Missions or invoke a model.

## Verification

- `rtk bash scripts/r2-github-feedback-backlog-postgres-test.sh` — passed on dedicated disposable PostgreSQL 18 through Schema 53. The suite covers backlog revision fencing, Workbench policy projection, policy/source approval gates, default-off behavior, a bounded fake-provider poll, same-slot non-replay, started-attempt expiry, later-slot identity, no GET after disable/revoke/archive before claim, active-company checks and no automatic Task creation.
- Kernel PostgreSQL integration closes the held advisory-lease connection while keeping the main DB pool available. Reservation and final dispatch claim reject that stale Kernel incarnation without adding an attempt or calling a provider.
- Tokenless-auth regression tests first failed for the previously ungated feedback routes, then passed after all company `/feedback` routes were covered.
- `rtk bash scripts/go.sh test ./... -count=1` — passed.
- Linux and Windows/amd64 command builds — passed (`GOOS=windows GOARCH=amd64` forwarded through `WSLENV`).
- Frontend Vitest — 119/119 passed; `rtk npm run typecheck`, `rtk npm run lint`, and `rtk npm run build` passed. Vite reported the existing large-chunk advisory.
- Migration hash manifest validation and `rtk git diff --check` passed.

No GitHub account, external endpoint, real token, QQ, MCP server, model turn, or production database was used. Linux Secret Service host qualification and live GitHub repository/permission qualification remain open. The scheduler flags were not enabled during verification.

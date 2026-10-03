# Slice 137 — general Codex auth-principal observation

Date: 2026-10-03

## Delivered

- General Codex business runtimes now inspect the configured auth file during readiness. When the existing parser can reconstruct issuer/subject identity claims, the runtime produces the same versioned auth-principal fingerprint used by the LIVE_2 path; when it cannot, the WorkerSession records `unavailable` with a bounded reason code.
- The runtime refreshes the observation before a WorkerSession is pinned. Once pinned, readiness and reservation/start compare a reconstructable principal fingerprint and fail closed if it changed.
- No credential bytes, JWT claims or credential-revision fingerprint are persisted by this path. The result is an auth-principal hint, not a validated ProviderAccount billing identity. No account financial limit is enabled.

## Verification

| Check | Result |
| --- | --- |
| `go build ./internal/provider ./internal/kernel ./internal/control ./internal/workbench ./cmd/polis` | PASS |
| `sha256sum -c ../migration_hashes.sha256` from `db/migrations/` | PASS; migrations 00001–00093 |
| `git diff --check` | PASS |
| Tests | Not run |
| PostgreSQL migration/runtime | Not run |
| Provider execution/live cost measurement | Not run |

## Remaining boundary

If identity claims are unavailable, changing between two unidentifiable accounts cannot be detected by this fingerprint and the persisted status remains explicitly unavailable. Provider-specific mapping from auth principal to billing account, account-level request reservations, settlement, unknown-liability retention, hidden retry accounting and FT-42–45 qualification remain open.

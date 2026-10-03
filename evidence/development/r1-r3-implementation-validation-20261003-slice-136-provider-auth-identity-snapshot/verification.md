# Slice 136 — provider auth identity snapshot

Date: 2026-10-03

## Delivered

- Schema 93 stores one immutable provider auth-identity snapshot per WorkerSession. The insert trigger and Kernel command require the session to be `restoring`, so the observation binds before provider execution and cannot be rewritten or removed.
- A versioned optional runtime capability distinguishes `available`, `unavailable` and `unsupported`. The Codex runtime reports an identity fingerprint only for LIVE_2, where the credential source is already captured and rechecked. Other profiles and runtimes without the capability record an explicit unsupported state.
- The real provider adapter records the snapshot before it obtains a provider reservation or starts the process. Capture or persistence failure finalizes the unstarted WorkerSession.
- The fingerprint is an auth-principal hint, not a ProviderAccount ID. It is not exposed to the model, used as a financial budget key, or treated as settled/reserved/unknown spend.

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

Only LIVE_2 currently provides an available identity fingerprint. The general Codex runtime and other provider runtimes remain unsupported/unavailable for account identity. No ProviderAccount registry, request reserve, settlement or unknown-liability ledger exists, so no ProviderAccount financial cap is enforced. Exact FT-42–45 qualification remains `not_run`.

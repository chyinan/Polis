# Slice 138 — Codex account locator snapshot

Date: 2026-10-03

## Delivered

- Added a versioned optional provider account-identity capability and immutable Schema 94 snapshot, bound once per WorkerSession while it is `restoring`.
- For Codex `auth_mode=chatgpt`, Polis fingerprints `tokens.account_id`, falling back to the `chatgpt_account_id` member of the ID token's `https://api.openai.com/auth` claim. Other auth modes are explicitly unsupported; missing or malformed identifiers are unavailable.
- Account IDs are hashed with a provider and schema domain before leaving the parser. Raw IDs and JWT claims are not persisted or exposed to the model.
- Codex rechecks the locator at WorkerSession binding and at reservation/start readiness. A changed available locator rejects the current unstarted session.
- The upstream Codex auth implementation exposes a selected account ID for token-backed ChatGPT auth and uses it as an account selector; the backend client can send it as `ChatGPT-Account-Id`. This is account-routing evidence, not proof that all billable liability shares this scope. No ProviderAccount financial cap or cost settlement was enabled. See the [Codex account-ID accessor](https://github.com/openai/codex/blob/main/codex-rs/login/src/auth/manager.rs#L3258-L3282), [managed account binding](https://github.com/openai/codex/blob/main/codex-rs/login/src/auth/manager.rs#L3718-L3778), and [backend header construction](https://github.com/openai/codex/blob/main/codex-rs/backend-client/src/client.rs#L2631-L2679), inspected on 2026-10-03.

## Verification

| Check | Result |
| --- | --- |
| `go build ./internal/codex ./internal/provider ./internal/kernel ./internal/control ./internal/workbench ./cmd/polis` | PASS |
| `sha256sum -c ../migration_hashes.sha256` from `db/migrations/` | PASS; migrations 00001–00094 |
| `git diff --check` | PASS |
| Tests | Not run |
| PostgreSQL migration/runtime | Not run |
| Provider execution/live cost measurement | Not run |

## Remaining boundary

The selected Codex account locator is not yet established as a complete billable-liability scope, especially for auth modes not supported by this extractor or provider-side retries. The ProviderAccount registry, request reservation/settlement, retained unknown liabilities, hidden retry accounting, token/money accounting and FT-42–45 qualification remain open.

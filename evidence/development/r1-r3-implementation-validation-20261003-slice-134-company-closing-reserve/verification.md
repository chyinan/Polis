# Slice 134 — Company closing reserve

Date: 2026-10-03

## Delivered

- Schema 92 adds an append-only Company closing-reserve ledger inside the explicitly configured Company protocol-tool-call total. A pending Company can configure a reserve before its first cap, while initial cap configuration must cover current usage plus the remaining reserve.
- Only fixed Kernel-created `review` and `peer_review` Tasks can spend protected calls. Ordinary admission and accepted calls preserve the reserve; closing calls consume reserve and Company total in the same accounting transaction.
- Immutable Company budget rejection evidence now captures the reserve amount, remaining reserve and reserve revision. The Workbench exposes a confirmed, reasoned owner reserve update and displays reserve state. Handover and provider turn clamping enforce effective Company availability.
- The counter covers admitted protocol tool calls only. ProviderAccount financial limits, hidden retries, token/money accounting and exact FT-42–45 qualification remain open.

## Verification

| Check | Result |
| --- | --- |
| `go build ./internal/kernel ./internal/control ./internal/workbench ./internal/desktop ./cmd/polis` | PASS |
| `npm run build` from `frontend/` | PASS; Vite reports its large-bundle advisory (810.97 kB minified JS) |
| `sha256sum -c ../migration_hashes.sha256` from `db/migrations/` | PASS; migrations 00001–00092 |
| `git diff --check` | PASS |
| Tests | Not run |
| PostgreSQL migration/runtime | Not run |
| Provider execution/live cost measurement | Not run |

## Operational boundary

Existing Companies continue to require an owner-configured finite Company cap before new Worker admissions and protocol calls. The closing reserve is carved out of that same total and does not increase it. Reserve accounting recognizes only fixed closing Task classes and cannot account for model requests without tools, hidden CLI retries, tokens, raw provider egress or USD spend. Exact FT-42–45 qualification remains `not_run`.

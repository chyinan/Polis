# Slice 269 — hidden R1 software gaps and preview cookie boundary

Date: 2026-10-06

## Findings and changes

An independent source audit found four R1 software requirements missing from the open software ledger: REQ-36 Worker-facing environment status/ensure and dependency-change handling; REQ-37 Worker job operations and consumer-bound BorrowerLease; REQ-38 persisted BrowserRun/research operations and safe browser isolation; REQ-40 durable DeliveryManifest revisions and a separate UserDisposition. The related frozen scenarios were consequently marked `implemented` by the requirement-derived crosswalk despite lacking those product paths. This slice reopens the four requirements and corrects their scenario dispositions; it does not change any frozen scenario or execution status.

The same audit confirmed that the current service preview bound `127.0.0.1` on an ephemeral port. Because cookies are not port-scoped, the installation-owner session and readable CSRF cookies could reach the preview application. The retained ingress code now uses a randomized IPv4 loopback address from `127/8`, removes the installation-owner session/CSRF cookies and CSRF header from proxied requests, and rejects upstream responses that attempt to set either owner cookie. Since those changes do not isolate a browser profile or block browser access to control-plane loopback ports, Workbench no longer exposes live-preview creation and the protected Control API refuses to issue preview sessions until those qualifications exist.

## Verification

- `go build ./cmd/polis` — passed.
- Frontend production build (`npm run build`) — passed; Vite reports the existing large-chunk advisory.
- Traceability ledger validation — 22 open software requirements, 232 unique rows, all 232 `not_run`; every scenario disposition matches its open-REQ mapping.
- `git diff --check` — passed.
- No tests were added or run. No database, migration, WorkerSession, provider, browser, or frozen scenario was used.

## Limits

The cookie isolation and proxy filtering mitigate the owner-cookie leak, and the live preview API is fail-closed. Neither change establishes an isolated browser profile, a denied browser-to-control-plane network path, or qualified Playwright/Chromium execution. BrowserRun remains unimplemented. The new REQ-36/37/38/40 software components remain open work; owner-selected dependency/borrower/delivery policy inputs and qualified browser/runtime hosts remain separate gates. Schema 109–110 were not applied; the recorded local runtime remains Schema 108.

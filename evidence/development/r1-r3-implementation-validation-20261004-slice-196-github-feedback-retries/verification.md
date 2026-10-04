# Slice 196 verification — GitHub feedback exact retries

Date: 2026-10-04

GitHub credential store/delete, internal backlog updates, source registration/probe/decision, polling, and collection-policy commands preserve a request ID for the exact same payload after an ambiguous result. Credential store identity uses a one-way SHA-256 digest of the token; the token is not copied into the pending-ID key. Existing settle handlers continue refreshing authoritative feedback projections.

Verification performed:

- `cd frontend && npm run build` — passed (TypeScript project build and Vite production build). Vite emitted its existing advisory that the generated JavaScript chunk exceeds 500 kB.
- `git diff --check` — passed.

No tests were run. No credential was stored or deleted, no repository request or poll was made, no collection policy changed, and no Worker/provider action or frozen scenario ran. No schema migration occurred. GitHub permission and collection qualification remain open.

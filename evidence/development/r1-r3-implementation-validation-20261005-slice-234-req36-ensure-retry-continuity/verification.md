# Slice 234 — REQ-36 environment preparation retry identity

## Change

The Workbench now keeps the opaque Ensure RequestID in tab-scoped `sessionStorage`, keyed by Company and environment revision. If the server accepts a preparation request but its response is lost, the owner can reload or revisit the Task page in the same tab and retry with the exact RequestID. A validated receipt removes the stored ID so a later intentional preparation can get a new identity. If session storage is unavailable, the component keeps the prior in-memory retry behavior. No source, credentials, or environment contents are persisted.

## Verification

- `npm run build` (`tsc -b` and Vite production build): passed; Vite reports the existing large-chunk advisory.
- `git diff --check`: passed.
- Backend `/healthz`: HTTP 200, status `ready`; frontend root: HTTP 200.
- Read-only PostgreSQL query: Schema 108; 0 active WorkerSessions.
- No Ensure Environment request, Worker/provider activity, tests, or frozen scenario ran.
- UI-42 and WF-08 reference this code evidence and remain `implemented` / `not_run`; all 232 frozen scenario states remain `not_run`.

REQ-36 native Windows/Linux environment execution and host qualification remain open.

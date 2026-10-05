# Slice 237 — REQ-02 owner team-coverage acknowledgment

## Change

Company creation and the existing Company directory can now persist the installation owner's acknowledgment of the exact canonical `spec/design-v0.4.5/product/TEAM_COVERAGE.json` bytes. The existing-Company review shows all seven task types, owners, independent checkers, acceptance paths, and unverified qualifications before the owner checks the acknowledgment. Submission requires the exact embedded SHA-256, an authenticated installation-owner session, and a same-origin request with valid owner CSRF. Confirmation uses its own idempotent command and appends only the acknowledgment event; it does not rewrite Company or employee settings. Creation stores the event in the same transaction as the new Company. Company summaries expose whether the latest acknowledgment matches the currently embedded matrix.

The acknowledgment payload explicitly records the decision and leaves qualification `unverified`. It does not approve execution, qualify roles or providers, or enable the execution flag. REQ-02 remains open.

Canonical matrix SHA-256: `8dcd1c20d7b0db14e77b85ff5d4379b2cd8829f24bfd70383a7c2651dedab770`.

## Verification

- `go build ./...` and `go build -o .runtime/bin/polis ./cmd/polis` passed.
- `npm run build` in `frontend/` passed (`tsc -b` and Vite production build). Vite reports the existing large-chunk advisory (886.33 kB minified JS).
- `git diff --check` passed.
- GitHub push integration published Slice236 (`56b69a51ea93b7226056bcb508912453365b9690`) and Slice237 (`576ebb7476da17e1e6bcdfdb2dd57443c601beab`) as separate commits on `main`; local and remote refs were then aligned.
- The rebuilt deterministic backend was restarted and returned `{"service":"polis_backend","status":"ready","version":"r0.7"}`; the Vite development frontend returned HTTP 200.
- Read-only local PostgreSQL transaction returned Schema 108 applied, 0 Companies, 0 Missions, and 0 active WorkerSessions; transaction ended with `ROLLBACK`.
- The seven mapped REQ-02 scenario rows link this evidence and remain `partial/not_run`; all 232 frozen scenarios remain `not_run`.
- No owner acknowledgment, tests, database writes, Worker/provider operations, or frozen scenarios ran.

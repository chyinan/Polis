# Slice 240 — fixed-role company setup UI

## Change

The New Company wizard now renders the canonical role for each fixed Employee ID as read-only and explains that the role cannot be changed during creation. The final review lists every Employee ID and role pair beside the fixed-team coverage draft, making the actual mapping visible when an installation owner confirms the matrix. Display names and model profiles remain editable. This matches Slice239's server validation and avoids presenting invalid role edits as supported setup choices.

## Verification

- `npm run build` passed (`tsc -b` and Vite production build). Vite reports the existing large-chunk advisory (886.51 kB minified JavaScript).
- `git diff --check` passed.
- The local Vite frontend returned HTTP 200 at `127.0.0.1:4173`.
- The seven REQ-02 traceability rows and UI-01 cite this implementation evidence; all eight scenario statuses remain `not_run`, and all 232 frozen scenarios remain `not_run`.
- No tests, Company creation, owner acknowledgment, database writes, Worker/provider operations, or frozen scenarios ran.

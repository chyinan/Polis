# Slice 236 — REQ-02 fixed team coverage review in company setup

## Change

The New Company wizard now imports the canonical `spec/design-v0.4.5/product/TEAM_COVERAGE.json` and displays all seven task coverage rows with their owner, eligible independent checker(s), acceptance path, and qualification status. It shows that the template remains a draft, execution is disabled, every row is unverified, and the design requires human handling before admission. The final review step says that creating a company saves organization settings and does not count as template approval or provider execution authorization.

This is a transparency improvement only. It does not record an owner decision, change the team matrix, qualify a role/provider, or change server-side Task admission. REQ-02 stays open.

Canonical matrix SHA-256: `8dcd1c20d7b0db14e77b85ff5d4379b2cd8829f24bfd70383a7c2651dedab770`.

## Verification

- `npm run build` in `frontend/` passed (`tsc -b` and Vite production build). Vite printed the existing bundle-size advisory (879.74 kB minified JS).
- `git diff --check` passed.
- Parsed the traceability JSON and confirmed its seven REQ-02 scenario rows remain `not_run`; the frozen scenario catalogs and execution statuses were not changed.
- No tests, database writes, Worker/provider operations, or frozen scenarios ran.

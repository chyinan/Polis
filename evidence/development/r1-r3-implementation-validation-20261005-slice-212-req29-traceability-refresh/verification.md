# Slice 212 — REQ-29 traceability refresh

Updated the live requirement-to-scenario crosswalk date and added the Slice 211 revocation source evidence to CAP-01–06. The crosswalk still records all 232 frozen scenarios exactly as `not_run`; CAP-01–06 remain `partial` under REQ-29, and counts and all other fields are unchanged.

## Verification

- Parsed the JSON and compared it with the prior committed crosswalk after removing only the new evidence pointer and audit date; all other fields match.
- Confirmed six REQ-29 scenario records, all partial and not_run.
- `git diff --check` passes.

No tests, code, schema, migration, database, Worker/provider activity or frozen scenario ran.

# Slice 250 — REQ-02 exact team-coverage JSON fields

## Change

The fixed team-coverage semantic validator now checks the exact canonical key set on the top-level JSON object and on each of its seven assignment objects before decoding into Go structs. This closes a parser mismatch: Go's struct decoder accepts field names case-insensitively, while the frontend reads JSON properties using exact names. A case-variant key can no longer make the server and confirmation UI interpret different values.

The validation remains a guard around the existing fixed, non-executable draft. It does not change Task admission, enable execution, qualify roles, or record an owner acknowledgment.

## Verification

- `go build ./...` passed on integrated source commit `4df990e` (published implementation commit `858e1a2919ab9a9559af57a8e024906dc43a74d3`).
- `git diff --check` passed.
- All seven REQ-02 scenario rows remain `partial/not_run`; all 232 frozen scenario rows remain `not_run`.
- No tests, database operations, Company creation/acknowledgment, Worker/provider operation, or frozen scenario ran.

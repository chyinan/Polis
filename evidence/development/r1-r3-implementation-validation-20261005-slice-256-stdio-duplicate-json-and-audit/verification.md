# Slice 256 — stdio MCP duplicate-key guard and remaining source audits

## REQ-31/34 source fix

The controlled stdio MCP decoder now recursively rejects duplicate JSON object keys before decoding a response into structs or maps. This covers the JSON-RPC envelope and nested results, including tool definitions and input schemas. Outbound stdio tool arguments also reject duplicate keys before validation and dispatch. Escaped keys are compared after JSON decoding, so equivalent spellings cannot create a last-key-wins ambiguity. Streamable HTTP parsing was not changed.

The fix is published on `main` as `e32550e1f45e0a41e2dc7ec888e404448a7bfac5`. The Go tree built successfully with `go build ./...`, and `git diff --check` passed. The 18 overlapping CAP-02/10/11/13–24/28–30 rows mapped to REQ-31/34 remain `partial/not_run`; all 232 frozen scenarios remain `not_run`.

## Additional source audits

Read-only audits checked REQ-02/13, REQ-14/15/16, and REQ-27/30/32/33. No other specific fail-open was identified for a local code change. The reviewed areas included fixed-role validation and company projections; shared dispatch cursor/claims and Fake-only admission; WorkerSession recovery, stop proof, and capability revocation; memory-revocation overlay load/write/replay and session-bound correction/revalidation; provider usage and billing projections; successor session/profile gates; Skill loading; and MCP approval, binding, schema, and endpoint rechecks.

REQ-23 closeout recovery was audited separately; its evidence is `evidence/development/r1-r3-implementation-validation-20261005-slice-256-req23-closeout-recovery-audit/verification.md`. No requirement or scenario disposition changed. Remaining owner decisions, active-session prerequisites, trusted provider/account evidence, and Windows/Linux/Desktop qualification are still open.

## Verification limits

- No tests were added or run.
- No database query/write, Worker, provider, MCP endpoint, browser qualification, or frozen scenario operation ran.
- This source audit and build do not qualify any external host/account/provider behavior.

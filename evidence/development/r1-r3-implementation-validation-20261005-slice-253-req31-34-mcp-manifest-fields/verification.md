# Slice 253 — REQ-31/34 exact stdio MCP manifest fields

## Change

The stdio MCP package manifest parser now checks that every top-level field name is an exact canonical schema key before decoding into the Go struct. Unknown fields and case variants fail closed, preventing `command`/`Command` or `entryPoint` aliases from producing inconsistent values across JSON consumers. The existing recursive duplicate-key check remains in place, and `args` remains optional.

This protects the local package intake and executable-path contract. It does not qualify Windows WFP/AppContainer isolation, an external MCP endpoint, or a product-provider session.

## Verification

- `go build ./...` passed on integrated source commit `0104612` (published implementation commit `4b8f4d1f063287a84456ac25da3e38cc052437ed`).
- `git diff --check` passed.
- The 18 mapped REQ-31/34 scenario rows remain `partial/not_run`; all 232 frozen scenario rows remain `not_run`.
- No tests, database operation, package import, Worker/provider action, or frozen scenario ran.

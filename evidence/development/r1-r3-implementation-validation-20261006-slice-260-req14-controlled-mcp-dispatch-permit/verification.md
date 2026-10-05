# Slice 260 verification — REQ-14 controlled MCP dispatch permits

Date: 2026-10-06

## Source and scope

Published implementation commit: `91bb6c25558d8709fcd7ee49f9ac137e1adfda88`.

Schema 109 adds short-lived, single-use permits for controlled stdio and Streamable HTTP MCP calls. The permit binds the action and attempt, Task, WorkerSession generation, Employee epoch, current capability-binding event, runtime qualification, target fingerprint, and canonical argument digest. A final consume transaction rechecks the current authority and atomically persists permit consumption, the immutable `dispatching` call intent, and the Company event under the same lifecycle guard used by revocation. The controlled clients require this consume callback; direct unpermitted call paths fail closed. A consumed call may remain `outcome_unknown` after interruption because local Worker stop cannot undo an endpoint action already admitted.

This implementation covers controlled MCP dispatch only. It does not add the general ActionIntent/DispatchPermit layer or shared-write ResourceKey/ResourceBinding model. Those still need an owner-approved action and canonical target/account mapping. The migration has not been applied to the local runtime database, and Windows/Linux host plus provider/endpoint qualification remain open.

## Recorded static verification

- `go build ./...` passed against the Schema 109 source before the implementation commit.
- `git diff --check` passed before the implementation commit.
- No tests were run. No database migration or write, WorkerSession activity, provider/MCP endpoint call, browser qualification, or frozen scenario was performed.
- All 232 frozen scenarios remain `not_run`; REQ-14 remains `partial` and among the 17 open software requirements.

The implementation map is `docs/implementation/REQ14_REVOCATION_DISPATCH_MAP.md`.

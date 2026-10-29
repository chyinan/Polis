# Slice 339 — bounded PDF content-operator audit

Date: 2026-10-29
Result: `OPT_IN_OPERATOR_NAME_INVENTORY_ADDED`; no rendering behavior enabled.

## Implemented

- Pinned the exact upstream `github.com/giraffesyo/pdf@v0.7.0` source (tag commit `8a26970ec43650cea9f50bed558edf9c26a10479`) under `third_party/giraffesyo-pdf`, retaining its MIT license and full upstream tests. Root `go.mod` replaces only that module path with the fork.
- Added opt-in `Options.AuditContentOperators`, exposing a sorted unique `Page.ContentOperatorNames` inventory across page streams, nested Form XObjects and visited annotation appearance streams. Inline images use the synthetic name `<inline-image>`.
- The audit retains at most 256 names, each at most 128 bytes. Malformed, stream/work-limited, overlong, or over-count traversals set `Page.ContentOperatorNamesComplete=false`. The audit stores names only, not content bytes or operands.
- The option is disabled by default. Existing extraction, current PDF @4 snapshot format, MissionInput bytes and Worker delivery behavior are unchanged. Operator presence does not assert support or renderability.
- Added `scripts/test-pdf-parser-fork.sh` so future fork changes run the nested module's test suite separately from the root module.

## Test-first and verification

- The focused test first failed to compile because the new API did not exist (red); it then passed after implementing the audit.
- Parser fork full suite: PASS — `parser-fork-tests.log`.
- Parser fork race suite: PASS — `parser-fork-race.log`.
- Module resolution: PASS — root Go module resolves `github.com/giraffesyo/pdf v0.7.0` to `third_party/giraffesyo-pdf` (`dependency-resolution.log`).
- Full Polis Go suite: PASS — `go-tests.log`.
- Full Polis race suite: PASS — `go-race.log`.
- `go vet ./...`: PASS — `go-vet.log`.
- Linux amd64 and Windows amd64 command builds: PASS — `linux-build.log`, `windows-build.log`.
- `git diff --check`: PASS.
- `POLIS_TEST_DSN` was unset; PostgreSQL integration cases requiring it were skipped. No migration, Worker, Provider, PDF render, or external runtime was used.

## Next parser step

The inventory is coverage diagnostics only. Slice340 must introduce an ordered typed display list with bounded operands/resources; any unknown, malformed, or over-budget visual operation must prevent a page from being reported as fully rendered. PDF @4 remains `unsupported` until a later renderer implements complete page semantics.

# Slice 329 — REQ-38 isolated BrowserRun/ResearchOperation surfaces

Date: 2026-10-27
Result: `FAKE_SURFACES_WIRED_FAIL_CLOSED`; no external operation was enabled.

## Implemented

- Added `POLIS_OFFLINE_BROWSER_RUN_ENABLED=1` for fake-only `@18` and `POLIS_OFFLINE_RESEARCH_OPERATIONS_ENABLED=1` for fake-only `@19`. Both require `POLIS_WORKER_MODE=real` and `POLIS_PROVIDER_TRANSPORT=fake`; offline product surfaces remain mutually exclusive.
- `POLIS_OPERATION_ADAPTERS_ENABLED=1` now requires exactly one of those isolated surfaces and the fake transport. Existing surface fingerprints and real-provider `@4` are unchanged.
- Slice328's independent authorization gate remains in force: the current Playwright and research adapters do not implement a qualified authorizer, so the Worker can only record the existing blocked/unavailable operation. Enabling these flags does not by itself authorize browser or HTTP egress.
- Source registration still pins Research origins/endpoints and credential references. The currently wired search adapter has no credential resolver; credentialed search fails closed. No credential, WFP, Browser profile or network qualification is claimed.

## Verification

- Full Go tests and race suite: PASS — `go-tests.log`, `go-race.log`.
- `go vet ./...`: PASS — `go-vet.log`.
- Linux amd64 and Windows amd64 command builds: PASS — `linux-build.log`, `windows-build.log`.
- Migration hash-manifest regression test: PASS — `migration-manifest-test.log`.
- Tests verify exact @18/@19 selection, readiness, isolation from other surfaces, and rejection of real Provider transport or adapter enablement without an operation surface.
- No browser process, HTTP request, external retrieval, credential use, Worker turn or Provider request ran.

## Remaining REQ-38 gates

Implement a durable host/profile/network qualification authorizer, protected research credential resolution, exact search endpoint/ranking qualification, and external evidence qualification. Until those exist and pass on the target host, current concrete adapters must remain non-authorizing and execution unavailable.

# Slice 328 — REQ-38 BrowserRun/ResearchOperation execution gate

Date: 2026-10-26
Result: `FAIL_CLOSED_EXECUTION_AUTHORIZATION_IMPLEMENTED`; host/provider qualification remains open.

## Implemented

- Added distinct typed `BrowserRunExecutionAuthorizer` and `ResearchOperationExecutionAuthorizer` contracts. `EmployeeTools` now invokes an executor only when the matching authorizer is present and returns success for the exact binding/request/record.
- An absent or negative authorizer leaves the durable BrowserRun request blocked or the ResearchOperation unavailable; it does not call the browser, fetch, or search adapter.
- Existing `PlaywrightOperationAdapter` and research adapters do not implement a qualified authorizer. Therefore the current `POLIS_OPERATION_ADAPTERS_ENABLED` injection cannot by itself enable external execution. The real-provider `@4` contract and fake @18/@19 surfaces are unchanged.
- This increment deliberately adds no qualification bypass, credential resolver, endpoint approval, or network authorization. A future qualified authorizer must bind the current host/profile/WFP identity and protected endpoint/credential policy before returning allow.

## Verification

- Full Go suite: PASS — `go-tests.log`.
- Full Go race suite: PASS — `go-race.log`.
- `go vet ./...`: PASS — `go-vet.log`.
- Linux amd64 and Windows amd64 command builds: PASS — `linux-build.log`, `windows-build.log`.
- Migration hash-manifest test: PASS — `migration-manifest-test.log`.
- Unit coverage verifies missing, negative and positive authorization outcomes for both operation types.
- No browser process, HTTP request, search, external retrieval, credential use, Worker turn, or Provider request ran.

## Remaining REQ-38 qualification

The concrete adapters remain blocked until a separate qualified authorizer exists for the isolated Playwright/Chromium profile, management-network/WFP boundary, exact research origin/endpoint/ranking, protected credential reference, and evidence-producing runtime. The fixed scenario/browser/provider qualification remains open.

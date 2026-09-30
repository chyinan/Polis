# Slice 36 — bounded loopback service-readiness probe foundation

Date: 2026-09-25

## Implemented

- Added `ServiceProbeSpec` and canonical SHA-256 fingerprinting. The contract pins a literal `127.0.0.1` or `::1` destination, exact port/path, expected 2xx status, exact expected response-body digest, timeout and lease duration.
- `ProbeServiceEndpoint` requires an injected process-owner verifier before the request and again before it can return `ready`. It uses one GET request to the exact loopback target, disables proxies, compression, keep-alives and redirect following, and bounds each response to 64 KiB and each request to at most five seconds.
- Probe evidence contains the process ID, endpoint, healthcheck-spec digest, response status/digest, safe reason code and (only on success) lease expiry. Response bodies and arbitrary network errors are not included.
- Input validation rejects non-loopback hosts, malformed paths including CR/LF injection, invalid digests, unsupported status codes and unbounded timeout/lease values.
- Immutable environment policies can optionally pin up to eight service definitions with a validated script path/argv and the complete healthcheck spec. Definitions are sorted before hashing; duplicate IDs/endpoints and unsafe definitions are rejected. Policies without services omit the field and retain their prior canonical representation. Workbench type validation and the environment revision panel display pinned service definitions.
- **The pinned definitions and probe are not yet consumed by `StartProjectJob`; `service_endpoint_events`, generation/lease persistence, renewal/revocation and Windows/Linux process-owner verifiers remain unwired. Control still accepts only batch Jobs, so no service Job can reach this probe or be marked ready.**
- There is no schema migration.

## Verification

| Command | Result |
|---|---|
| `bash scripts/go.sh test -race ./internal/environment -run TestProbeServiceEndpoint -count=1` | PASS; loopback fixture tests cover exact response/status/digest, timeout, oversized response, redirect refusal, non-loopback refusal, path injection, missing owner proof, and owner change before ready. |
| `bash scripts/go.sh test ./internal/environment -run TestServiceProbeSpecDigestIsCanonical -count=1` | PASS; canonical digest is deterministic and invalid specs are rejected. |
| `bash scripts/go.sh test ./internal/environment -run TestEnvironmentPolicy -count=1` | PASS; service definitions are normalized/sorted, healthcheck config changes the immutable policy digest, old empty-service policy serialization is unchanged, and unsafe IDs/commands/endpoints are rejected. |
| `cd frontend && npm run test -- --run src/domain/workbench-validation.test.ts` | PASS; 25 tests include service-policy validation. |
| `cd frontend && npm run typecheck && npm run lint && npm run build` | PASS; Workbench reads and displays pinned service contracts; production bundle retains the existing >500 kB chunk advisory. |
| `bash scripts/go.sh test -p 1 ./... -count=1` | PASS; full serial Go suite, all packages reported `ok` or no test files. |
| `bash scripts/go.sh build ./cmd/...` | PASS; Linux command packages. |
| `GOOS=windows GOARCH=amd64 bash scripts/go.sh build ./cmd/...` | PASS; Windows amd64 cross-build. |
| `.tools/go/bin/gofmt -d` on changed Go files | PASS; no formatting diff. |
| `git diff --check` | PASS. |

A focused read-only code review found no issues in loopback pinning, proxy/redirect refusal, ownership checks, timeout/body bounds or digest verification. No service process, project script, external endpoint, WFP rule, loopback exemption, model, QQ, MCP or GitHub access was used.

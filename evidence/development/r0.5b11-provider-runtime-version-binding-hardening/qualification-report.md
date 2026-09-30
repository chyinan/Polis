# R0.5B11 provider runtime version binding hardening

## Result

`r0_5b11_provider_runtime_binding_hardening = PASSED` for offline/local scope.

`eligible_for_new_v4_live_qualification = YES`; live qualification status is `STILL_REQUIRED`.

No provider reservation, provider egress, Medium turn, High turn, canary, LIVE_3, business Mission, retry or successor was started. R0.5B10 evidence remains immutable.

## Root cause

Primary classification: `PROVIDER_RUNTIME_BINDING_DEFECT`.

B10 obtained the expected version from `POLIS_PROVIDER_EXPECTED_VERSION`, falling back to the ambient `CODEX_VERSION`. That value was `0.155.0-alpha.9.2`, while the provider launch used the staged `polis-r03a` executable whose manifest, SHA256 and local `--version` identify `0.154.0-alpha.6.2`. The expected version and executable provenance were therefore separate inputs. The PATH `codex.exe` is a different unqualified 0.155 installation and was not treated as the production binary.

The repository/runtime evidence selects Case A: the already-qualified controlled production runtime is the staged 0.154 artifact. The unqualified 0.155 installation is not promoted merely because it is newer; no 0.155 controlled runtime manifest or current qualification exists.

## Old and current identity

| Field | B10 configured/observed | B11 authoritative |
| --- | --- | --- |
| semantic version | expected `0.155.0-alpha.9.2`; initialize reported `0.154.0-alpha.6.2` | `0.154.0-alpha.6.2` |
| executable | staged `C:\Users\chyinan\AppData\Local\OpenAI\Codex\controlled-runtime\polis-r03a\bffc5354119c8421\codex.exe` | same manifest-resolved path |
| executable SHA256 | `081e4de4be8e38fac6ed4d95e3b1a0b9f6d31c090ddc36e1696b349fe406f575` | same |
| helper | sibling `codex-code-mode-host.exe` | same manifest-resolved path |
| helper SHA256 | `fdf360c3a02adce2a29272357d61681bdddc2ab9828a5fa615c4528e4384a23e` | same |
| implementation | implicit | `codex-app-server` |
| protocol compatibility | implicit | `codex-app-server-stdio/initialize-thread-start@1` |
| launch mode | `windows_native_direct` | `windows_native_direct` |

The authoritative source is the existing `windows-runtime-artifact-v1` `runtime-manifest.json`, verified by `runner.LoadAndVerifyWindowsRuntimeArtifact`. `polis serve` resolves the manifest before constructing `RealProviderWorkerAdapter`; configured version, binary/helper path and optional hash assertions must match the verified manifest. `CodexRuntime.Readiness` re-verifies the same binding before process use.

## Local no-provider preflight

Evidence: `local-preflight.json` and `provider/protocol.jsonl` in this directory.

- process start: `PASS`
- executable/helper identity: exact manifest match
- initialize: `PASS`, userAgent `polis/0.154.0-alpha.6.2`
- thread/start and 7-tool registration: `PASS`
- turn/start: not sent
- provider egress: `0`
- clean stop: `PASS`, Windows process-handle proof recorded

## Surface and fingerprint impact

- `provider_surface_changed = false`
- `runtime_binding_changed = true`
- `launch_envelope_changed = false`
- `exact_surface_execution_fingerprint_changed = true`
- current surface: `polis-product-tool-surface@4`
- tool count: `7`
- manifest: `60192ba130e504ad380fd524ec02ab536a8724723098f0d3fb91ecc0f3983ea4`
- schema: `2206` bytes / `5ee61b10d249a11614d176fda5c3df86f26e0b650c1074b705dc48b4405c35cc`
- launch envelope fingerprint: `676052c13ae0380250c4fcb9dbdb3cd8794167e214e6b3766e6e38b4d68bdc51`
- current product-v4 exact-surface execution fingerprint: `1f18dd6677e3543992af97fe79fb88604980b418622e436100d0ddf76cd8314c`
- no provider L2 fingerprint was generated

## Negative regressions

Offline tests fail closed for:

- configured version `0.155.0-alpha.9.2` versus manifest/runtime `0.154.0-alpha.6.2` (`provider_runtime_identity_mismatch`)
- expected executable SHA mismatch (`provider_runtime_identity_mismatch`)
- expected helper SHA mismatch (`provider_runtime_identity_mismatch`)
- wrong helper path (`provider_runtime_identity_mismatch`)
- wrong executable selected from PATH (`provider_runtime_identity_mismatch`)
- missing executable or manifest (`provider_runtime_unavailable`)

The B10 mismatch remains a deterministic failure; the corrected exact manifest binding passes locally without provider egress.

## Validation boundary

- targeted runtime binding, provider preflight, `RealProviderWorkerAdapter`, product identity and @4 surface tests: `PASS`
- `go test ./...`: `PASS`
- `go test -race ./...`: `PASS`
- `go vet ./...`: `PASS`
- Linux `go build ./cmd/...`: `PASS`
- Windows `GOOS=windows GOARCH=amd64 go build ./cmd/...`: `PASS`
- frontend tests: `19/19 PASS`
- frontend typecheck, lint and build: `PASS`
- `git diff --check`: `PASS`

No live qualification was inferred from this offline/local result.

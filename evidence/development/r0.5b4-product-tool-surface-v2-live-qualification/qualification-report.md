# R0.5B4 product tool surface v2 live qualification

**Status:** `PASSED`

**Current surface:** `polis-product-tool-surface@2`

**Provider surface status:** `QUALIFIED`
**Eligible for a separately authorized R0.5B real-provider smoke:** `YES`

## Exact provider-visible surface

The `polis serve` production runtime registers the generic product registry through `RealProviderWorkerAdapter`. The exact current surface matched the B3-frozen identity:

- Tool count: **7**
- Manifest: `2b403fc0f3c9a1513473828e66203becb7ac5202f298f917034fd95929a9f3b9`
- Aggregate schema: **1470 bytes**
- Aggregate schema digest: `8f2e1ee8ba8c8d456af9ce9839f62a854bfe7c9f5847613657ccd7f77a9da04f`

Registered tools, in order:

1. `polis_work_current`
2. `polis_context_read`
3. `polis_workspace_read`
4. `polis_workspace_replace`
5. `polis_workspace_check`
6. `polis_work_checkpoint`
7. `polis_artifact_submit`

Full descriptions and JSON schemas are in [exact-product-surface.json](exact-product-surface.json), with the raw golden-test output in [exact-product-surface-test.txt](exact-product-surface-test.txt). The semantic checks found no formatter, pagination, frozen-probe, smoke-specific, or non-empty fake-acceptance requirements. B2 is `STALE`; R0.3A surfaces are `HISTORICAL / NOT_REUSABLE`.

## Offline L2

`product_surface_offline_L2 = PASSED` before provider access. The dedicated temporary PostgreSQL 18 database exercised TaskValidationBinding, candidate-state fencing, and the real WorkerAdapter with fake transport; provider reservations and egress were zero, and the test cluster was removed. Final offline evidence is in [offline-l2-final.json](offline-l2-final.json), with the post-canary offline rerun under [post-canary-offline](post-canary-offline/).

## Execution identity

- Model / effort: `gpt-5.6-luna / medium`
- Provider runtime: Codex `0.154.0-alpha.6.2`
- Binary SHA-256: `081e4de4be8e38fac6ed4d95e3b1a0b9f6d31c090ddc36e1696b349fe406f575`
- Helper SHA-256: `fdf360c3a02adce2a29272357d61681bdddc2ab9828a5fa615c4528e4384a23e`
- Native launch envelope fingerprint: `676052c13ae0380250c4fcb9dbdb3cd8794167e214e6b3766e6e38b4d68bdc51`
- `product_exact_surface_execution_fingerprint`: `fecc17cac69231575c13d3e2d767e31560e60ebe8dc2fb20921b925ac8d95f08`
- Transport policy: `r03a-transport-policy@1`, unchanged defaults (initialize 30 s, thread start 30 s, first output 90 s, reconnect grace 30 s, idle 90 s, total turn 600 s, stop reconciliation 5 s).

This is a newly computed fingerprint bound to surface @2 and the current runtime bytes; no B2 or R0.3A execution fingerprint was reused. The diagnostic authorization is purpose-bound and one-use. It sets one Medium turn, one reservation, one provider egress, zero business allowance, zero High, zero retries, zero expected tool calls, and forbids business objects.

## Single live diagnostic canary

| Measure | Result |
| --- | --- |
| Diagnostic reservation / provider egress | `1 / 1` |
| Registered tools / registered manifest | `7 / 2b403fc0…9f3b9` |
| First valid output | `POLIS_PRODUCT_SURFACE_V2_CANARY_OK` |
| Time to first output | `7565 ms` |
| Elapsed from reservation to stop | `11525 ms` |
| Tokens | `11060` total (`11046` input, `14` output) |
| Tool calls / reconnects | `0 / 0` |
| Turn terminal | `turn/completed: completed` |
| Clean process stop / credential snapshot removed | `PASS / YES` |
| Company / Mission / Task / WorkerSession / successor / business mutations | `0 / 0 / 0 / 0 / 0 / 0` |
| Post-canary manifest and schema | unchanged; fresh |
| Second reservation | `DENIED`; no second process or provider egress |

The protocol timestamps, initialize/thread/turn lifecycle, usage, terminal result, and stop evidence are in [provider-transport-terminal.json](provider-transport-terminal.json) and [provider/protocol.jsonl](provider/). The one-use reservation is in [diagnostic-reservation.json](diagnostic-reservation.json); the actual duplicate denial is in [duplicate-reservation-guard.json](duplicate-reservation-guard.json). The product result and fingerprint are in [product-live-l2-qualification.json](product-live-l2-qualification.json):

`product_provider_L2_fingerprint = 4c206409f827211cb9f7b18d37da5d096efb7f735c90eaed15e4692128d6a532`

## Final offline verification

- `bash scripts/go.sh test ./...` — PASS
- `bash scripts/go.sh test -race ./...` — PASS
- `bash scripts/go.sh vet ./...` — PASS
- Linux and Windows amd64 command builds — PASS
- Frontend tests — PASS (19 tests); typecheck, lint, and build — PASS
- Bash syntax and `git diff --check` — PASS
- No PowerShell or Python source files were added or changed.

No R0.5B business smoke, Company, Mission, Task, WorkerSession, High turn, retry, or successor was started. Stop after B4. The eligible business smoke remains a separately authorized next step.

# R0.5B Real Provider Product Smoke — LIVE_1

**Result:** `INCONCLUSIVE`  
**Reason:** `PRODUCT_TASK_CARDINALITY_LIMIT`  
**Provider egress:** `0`

## Outcome

The existing frontend created one draft Mission and displayed its public acceptance contract. The backend read API and Activity page showed the authoritative `mission.create` event at `company_seq=1`.

I did not click Start. The current product lifecycle would create two Task rows from one Start: a `bootstrap_plan` row in `TXStartMissionCommand`, followed by a `compat` product Task in `TXPrepareProductTask`. The Workbench read model returns all Task rows for the Mission. That exceeds the explicit `Task=1` limit, so execution stopped before Task creation, WorkerSession creation, business provider authorization, reservation, or provider egress.

## Observed state before cleanup

- Company: `r05b-live-1` (1 row; 4 logical employee identities initialized by dedicated database setup)
- Mission: `4f000865902ec201d528c8ce3a039798`, `draft`
- Product Task: none created
- Real Employee / WorkerSession: none
- TaskValidationBinding, validation receipt, checkpoint, Artifact: not created
- Provider authorization / reservation / egress / turn: `0 / 0 / 0 / 0`
- Retry / successor / High / duplicate reservation: `0 / 0 / 0 / 0`
- Orphan WorkerSession / provider process: `0 / 0`

The B4 product surface remained current and qualified: `polis-product-tool-surface@2`, 7 tools, manifest `2b403fc0f3c9a1513473828e66203becb7ac5202f298f917034fd95929a9f3b9`, schema 1470 bytes / `8f2e1ee8ba8c8d456af9ce9839f62a854bfe7c9f5847613657ccd7f77a9da04f`, exact execution fingerprint `fecc17cac69231575c13d3e2d767e31560e60ebe8dc2fb20921b925ac8d95f08`, and frozen product provider L2 fingerprint `4c206409f827211cb9f7b18d37da5d096efb7f735c90eaed15e4692128d6a532`.

## Readiness and cleanup

The dedicated PostgreSQL 18.6 database reached schema 7. The Windows `polis serve` process passed real provider runtime readiness with the B4 model, binary/helper hashes, auth source, surface identity, and default transport policy. The frontend ran in real mode with HTTP refetch; SSE remained disabled. No Codex provider process was started. The business allowance file was never created.

After evidence capture, the Polis server, frontend, PostgreSQL process, PostgreSQL temporary cluster, and Windows temporary runtime directory were stopped or removed. The durable LIVE_1 evidence is under this directory. No source implementation or frozen R0.5A/B1/B2/B3/B4 evidence was changed.

Full post-provider Go, race, vet, build, and frontend suites were not run because the provider attempt never began and the failing pre-provider gate required stopping here. The LIVE_1 JSON, PowerShell, and Bash evidence files passed syntax/parse validation.

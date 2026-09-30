# Slice 24 verification — R1 image bytes delivered to Worker

Date: 2026-09-25

## Changes covered

Validated PNG/JPEG images remain candidate entries in new immutable Task manifests, are re-read and verified from CAS, are included in a typed image list in `ModelInputContext`, appear in the input receipt, and are sent as app-server `type: image` / inline data URL items in the same `turn/start` request as the text prompt. The maximum is four images, 4 MiB each and 8 MiB total. Workbench validates and displays image refs separately from text refs. Protocol JSONL redacts encoded image bytes.

## Verification results

| Command / evidence | Result |
|---|---|
| `bash scripts/go.sh test ./internal/intake ./internal/codex ./internal/control -count=1` | PASS. |
| `bash scripts/go.sh test ./internal/workbench -run TestTaskInputManifestViewProjectsImageDeliveryReference -count=1` | PASS; receipt readback accepts and projects an image reference. |
| `bash scripts/go.sh test ./internal/codex -run TestCodexTurnStartSendsInlineImageAndKeepsImageBytesOutOfProtocolLog -count=1` | PASS with a local scripted app-server. The fake server decoded the exact PNG data URL and matched its digest; the protocol log contained the digest but not the original bytes/base64. |
| `POLIS_TEST_DSN='host=/tmp/polis-r1-image-20260925/socket port=56438 dbname=polis_r0_r1_image_20260925 user=polis_runtime' scripts/go.sh test ./internal/control -run TestProductWorkerReceivesFrozenImageBytesInTurnInput -count=1` | PASS against dedicated PostgreSQL 18 / schema 28. The fake Worker received the uploaded PNG bytes and the final CAS-bound delivery receipt contained its input ID, MIME type and content digest; `provider_egress=0`. |
| `bash scripts/go.sh test ./... -count=1` | PASS after implementation changes; database integration tests without a DSN were skipped in this full run. |
| `bash scripts/go.sh test -race ./internal/intake ./internal/codex ./internal/control ./internal/workbench -count=1` | PASS. |
| `bash scripts/go.sh build ./cmd/...` / `GOOS=windows GOARCH=amd64 bash scripts/go.sh build ./cmd/...` | PASS. |
| Frontend `npm test`, `npm run typecheck`, `npm run lint`, `npm run build` | PASS; 63 tests. The build retains the existing >500 kB chunk advisory. |
| `git diff --check` | PASS after final changes. |

The app-server JSON schema was generated from the installed Codex CLI 0.151.0 into a dedicated temporary directory, then deleted. The product manifest's 0.154 binary was not run. The Worker and app-server were fakes; no real model turn or provider egress occurred, so image vision behavior with the pinned production runtime remains unqualified. The dedicated PostgreSQL cluster was stopped and its exact temporary directory removed.

# R1 Task text input delivery verification

Date: 2026-09-24

## Behavior

- Product Task creation freezes exact Mission input IDs, revisions and content digests in immutable schema-15 `task_input_manifests`.
- Before provider reservation/turn, the product Worker reads eligible text uploads from company-scoped CAS by those frozen references and verifies the stored bytes against the digest and size.
- The initial turn prompt contains included text, display names, input IDs/revisions, manifest digest and an explicit untrusted-input boundary. The context limit is 8 files, 16 KiB per file and 64 KiB total.
- Images remain partial, compressed directory snapshots are not unpacked into provider context, Git snapshot import is absent, and over-limit/unrepresented entries are listed as excluded from the turn prompt.
- Schema 17 stores immutable per-session prepared/final delivery evidence with manifest digest, payload digest, input references, provider egress count and outcome. Workbench derives the effective delivery status separately from the manifest snapshot's original `not_delivered` state.

## Verification

- The first offline Worker regression run failed because the captured turn prompt omitted the uploaded sentinel. After implementation, `TestProductWorkerReceivesFrozenTextInputsInTurnPrompt` passed and verified the sentinel and manifest digest in the prompt.
- Dedicated PostgreSQL tests verified frozen-revision/CAS reads, partial image exclusion, a `local_context_loaded` receipt with provider egress=0, and Workbench delivery-status readback.
- Full Go suite passed serially on schema 17 database `polis_r0_r1_taskinput_20260924_002b`: `POLIS_TEST_DSN=... bash scripts/go.sh test -p=1 -count=1 ./...`.
- Frontend tests (40), typecheck, lint and production build passed.
- `bash scripts/go.sh vet ./...`, `bash scripts/go.sh build ./cmd/...`, Windows amd64 `polisd` cross-build and `rtk git diff --check` passed.

No real model-provider request was run. The fake runtime verifies local prompt construction and reports no provider egress; real-provider use remains separately gated and unqualified.

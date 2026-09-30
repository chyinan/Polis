# Slice 100 verification — bidirectional peer messages

Date: 2026-09-30

The fixed Backend/Frontend peer path now supports direct messages in both directions. A bound sender must submit its WorkerSession-bound Task, which must be working and owned by that Employee; the explicit recipient/task pair must be the other ready or working peer Task in the same Company and Mission; and the message must reference the accepted ContractRevision. Cross-Mission targets and finalized recipient Tasks are rejected. Actionable sends transactionally persist the message, Obligation, WorkSignal and employee wake. FYI sends are readable and acknowledgeable without an Obligation or wake; repeated reads advance through pending FYIs in send-event order after each acknowledgement. The recipient can apply an actionable request to its own workspace and resolve it with its own candidate Artifact; Apply and resolve both bind to the WorkerSession Task.

| Verification | Result |
| --- | --- |
| RED: `bash scripts/go.sh test ./internal/codex -run '^TestPeerFrontendSurfaceExposesExplicitDirectSendTarget$' -count=1` | Failed because the original frontend peer registry had no direct-send tool |
| RED: historical default-surface guard | Failed when direct-send was initially added to the default registry; the new tool now lives in the explicit `PeerFrontendToolsWithDirectMessaging` extension |
| RED: dedicated PostgreSQL peer flow | Failed at frontend `collab_send` with `POLICY_DENIED` under the backend-only sender restriction |
| RED: cross-Mission and finalized-target checks | Both tests initially failed because the Kernel accepted those targets |
| RED: source Task/session binding | Failed because a mismatched source reached input validation instead of being denied |
| RED: resolve Task/session binding | Failed because one session resolved another Task's candidate |
| RED: FYI inbox readback | Failed because non-actionable messages were excluded from the peer inbox |
| RED: FYI inbox advancement | Failed because an acknowledged message stayed at the top of the inbox |
| `bash scripts/r1-employee-schedule-postgres-test.sh` | PASS; disposable PostgreSQL 18, schema 60→72, including bidirectional action/FYI flows, ordered FYI inbox advancement, cross-Mission/finalized-target denials, and session-bound apply/resolve |
| `bash scripts/go.sh test ./...` | PASS; all Go packages |
| `bash scripts/go.sh build ./cmd/...` | PASS; Linux amd64 commands |
| `GOOS=windows GOARCH=amd64 bash scripts/go.sh build ./cmd/...` | PASS; Windows amd64 commands |
| `git diff --check` | PASS |

Independent read-only re-review after the session-binding fixes found zero remaining actionable issues.

The explicit `PeerFrontendToolsWithDirectMessaging` extension carries the new tool. The default `PeerFrontendTools` and historical R0.3A probe/frontend L2 paths retain the old registry. No migration, real model/provider, QQ send, external MCP/GitHub call, or production system was used. REQ-02 remains partial: the current R1 product Worker surface and the rest of the fixed employee roles still need the corresponding communication path and qualification.

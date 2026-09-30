# R0.3A-T5 tool surface and native request qualification

Known-good R0.2 Medium tools: 7; failing R0.3A Backend tools: 11; exact additions: polis_contract_propose, polis_contract_accept, polis_contract_read, polis_collab_send.

All results are local-only; provider compatibility is explicitly unproven.

## Exact new tools
- polis_contract_propose: schema=76d4b529294c78d6247848e3d195afa2b61937c11699f535a16efc7e9c4a188f, bytes=183, handler=PeerEmployeeTools.call->TXProposePeerContract, policy=peer_backend, ordinal=5.
- polis_contract_accept: schema=9d84956a23db52364349792ee916714bba6b5fd8087fa158d7fc86056199ad46, bytes=120, handler=PeerEmployeeTools.call->TXAcceptPeerContract, policy=peer_backend, ordinal=6.
- polis_contract_read: schema=9d84956a23db52364349792ee916714bba6b5fd8087fa158d7fc86056199ad46, bytes=120, handler=PeerEmployeeTools.call->PeerContractRead, policy=peer_backend, ordinal=7.
- polis_collab_send: schema=3506df54159d11c3f0a061deeea8662fbdc0276476517bfa1692b1ea33422a98, bytes=270, handler=PeerEmployeeTools.call->TXPeerSend, policy=peer_backend, ordinal=8.

## Intersection
- polis_work_current: schema_same=True, description_same=False, handler_changed=True, ordinal=1->1.
- polis_context_read: schema_same=True, description_same=False, handler_changed=True, ordinal=2->2.
- polis_workspace_read: schema_same=True, description_same=False, handler_changed=True, ordinal=3->3.
- polis_workspace_replace: schema_same=False, description_same=False, handler_changed=True, ordinal=4->4.
- polis_workspace_check: schema_same=True, description_same=False, handler_changed=True, ordinal=5->9.
- polis_work_checkpoint: schema_same=False, description_same=False, handler_changed=True, ordinal=6->10.
- polis_artifact_submit: schema_same=True, description_same=False, handler_changed=True, ordinal=7->11.

## Subsets
All 16 surface combinations 脳 3 orderings locally valid: True. Failing rows: 0.

## Synthetic request padding
- 3072 bytes: serialized=6639, round_trip_stable=True, truncated=False.
- 4096 bytes: serialized=7663, round_trip_stable=True, truncated=False.
- 8192 bytes: serialized=11759, round_trip_stable=True, truncated=False.
- 16384 bytes: serialized=19951, round_trip_stable=True, truncated=False.

## Findings classification

`confirmed_same`:

- R0.2 Medium and failing Backend use Codex/app-server `0.151.0`, `gpt-5.6-luna/medium`, `/work`, and read-only sandbox.
- The recorded binary SHA for R0.2 and the first R0.3A preflight is identical.
- Canonical serialization round-trips are stable for A/B/C/D and all 16 subsets/orderings.

`confirmed_different`:

- The 7-tool intersection has description changes and `EmployeeTools.call` -> `PeerEmployeeTools.call` binding changes for all seven tools.
- `workspace_replace` and `work_checkpoint` also have schema canonical digest changes because required-field ordering differs.
- The four exact additions are `contract_propose`, `contract_accept`, `contract_read`, and `collab_send`.
- Tool count, tool schema digest/bytes, developer instruction bytes, and thread/start request bytes differ as recorded in the manifest.

`locally_valid_but_provider_compatibility_unknown`:

- All 11 schemas pass local structural checks.
- All 11 Backend handlers were invoked once through the real `PeerEmployeeTools` PG/FakeWorker path.
- All 16 subsets x canonical/reversed/alternate orderings pass local serialization, uniqueness, schema, handler-binding, and round-trip checks.
- Synthetic 3 KiB, 4 KiB, 8 KiB, and 16 KiB padded requests round-trip without local truncation.
- These results do not prove app-server/provider compatibility.

`not_recorded`:

- Per-turn proxy implementation/configuration, failing-run code-mode-host SHA, callback endpoint readiness, process tree, and employee binding readiness.

## T3 erratum

The historical T3 summary said four reconnect events. The raw T3 protocol contains five `responseStreamDisconnected` records. The raw protocol count `5` is authoritative for future analysis; the old summary remains unchanged historical evidence.

No local causal bug was found in schema validation, registration, ordering, serialization, policy binding, or fake callback dispatch. Therefore `11_tool_provider_causality = UNPROVEN`.

The smallest next diagnostic path remains a pure-local exact tool-binding/request comparison. If a real canary is later authorized, it should be a fresh no-business-side-effect Medium with minimal or no dynamic tools; do not resume any sealed allowance.

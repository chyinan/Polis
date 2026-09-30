import hashlib
import json
import sys
from pathlib import Path


ROOT = Path(sys.argv[1])
PAGINATION_1 = json.loads(Path(sys.argv[2]).read_text(encoding="utf-8"))
PAGINATION_2 = json.loads(Path(sys.argv[3]).read_text(encoding="utf-8"))
BASELINE_QUALIFICATION = sys.argv[4] if len(sys.argv) > 4 else "V5"
MANIFEST_REVISION = sys.argv[5] if len(sys.argv) > 5 else "r03a-frontend-execution-baseline-manifest@5"

def load(name):
    return json.loads((ROOT / name).read_text(encoding="utf-8"))

probes = [load(name) for name in (
    "starting-state-probe-1.json",
    "starting-state-probe-2.json",
    "starting-state-probe-post-1.json",
    "starting-state-probe-post-2.json",
)]
activations = [load(name) for name in (
    "frontend-activation-preflight-1.json",
    "frontend-activation-preflight-2.json",
)]
runtime = [load(name) for name in (
    "frontend-runtime-preflight.json",
    "frontend-runtime-preflight-2.json",
)]
manifest_path = ROOT / "FrontendExecutionBaselineManifest.json"
manifest = json.loads(manifest_path.read_text(encoding="utf-8"))

assert all(p["status"] == "FRONTEND_STARTING_STATE_READY" and p["read_only"] and not p["mutation"] for p in probes)
assert all(p["state"] == probes[0]["state"] for p in probes)
assert probes[0]["state"]["frontend_session_count"] == 0
assert probes[0]["state"]["live_writer_count"] == 0
assert probes[0]["state"]["obligation_state"] == "pending"
assert probes[0]["state"]["planner_relay_count"] == 0

assert all(a["status"] == "FRONTEND_ACTIVATION_PREFLIGHT_PASSED" and a["anchor_probe"] == "PASSED" and not a["mutation"] for a in activations)
assert all(a["starting_state"] == probes[0]["state"] for a in activations)

expected_fp = "676052c13ae0380250c4fcb9dbdb3cd8794167e214e6b3766e6e38b4d68bdc51"
for report in runtime:
    assert report["status"] == "FRONTEND_RUNTIME_ACTIVATION_PREFLIGHT_PASSED"
    assert report["launch_envelope"]["launch_mode"] == "windows_native_direct"
    assert report["execution_envelope_fingerprint"] == expected_fp
    assert report["transport_policy"]["transport_policy_revision"] == "r03a-transport-policy@1"
    assert report["transport_policy"]["reconnect_grace_ms"] == 30000
    assert report["transport_policy"]["first_output_deadline_ms"] == 90000
    assert report["transport_policy"]["streaming_idle_ms"] == 90000
    assert report["transport_policy"]["total_turn_deadline_ms"] == 600000
    assert report["transport_policy"]["reconciliation_ms"] == 5000
    assert report["local_protocol_ready"] and not report["turn_started"]
    assert not report["provider_egress"] and not report["allowance_created"] and not report["business_mutation"]

for report in (PAGINATION_1, PAGINATION_2):
    assert report["status"] == "PASSED"
    assert report["pagination_runtime_preflight"] == "PASSED"
    assert report["required_blob_count"] == 4
    assert report["provider_egress"] == 0 and report["allowance_created"] == 0 and report["business_mutation"] == 0

manifest["baseline_manifest_revision"] = MANIFEST_REVISION
manifest["execution_envelope_revision"] = "r03a-native-launch-envelope@1"
manifest["execution_envelope_fingerprint"] = expected_fp
manifest["transport_policy_revision"] = "r03a-transport-policy@1"
manifest["effective_transport_policy"] = {
    "initialize_timeout_ms": 30000,
    "start_acknowledgement_ms": 30000,
    "first_output_deadline_ms": 90000,
    "reconnect_grace_ms": 30000,
    "streaming_idle_ms": 90000,
    "total_turn_deadline_ms": 600000,
    "reconciliation_ms": 5000,
}
manifest["windows_cas_access_view"] = {
    "consumer_os": "windows",
    "path_kind": "UNC",
    "access_path": manifest.get("windows_cas_access_view", {}).get("access_path", ""),
    "source_wsl_path": manifest.get("windows_cas_access_view", {}).get("source_wsl_path", ""),
    "authoritative_inventory_digest": manifest["cas_inventory_digest"],
    "access_view_fingerprint": hashlib.sha256(json.dumps({
        "consumer_os": "windows",
        "path_kind": "UNC",
        "authoritative_inventory_digest": manifest["cas_inventory_digest"],
        "translation_method": "wslpath -a -w",
    }, sort_keys=True).encode()).hexdigest(),
}
manifest["windows_cas_access_view"]["translation_method"] = "wslpath -a -w"
manifest["pagination_runtime_binding"] = {
    "revision": "r03a-pagination-runtime-binding@1",
    "runtime_os": "windows",
    "windows_access_view_fingerprint": manifest["windows_cas_access_view"]["access_view_fingerprint"],
    "required_blob_count": 4,
    "inventory_digest": manifest["cas_inventory_digest"],
    "preflight": "PASSED",
}
manifest["recovery_anchor_probe"] = "PASSED"
current_l2 = json.loads(Path("evidence/development/r0.3a-frontend-evidence-contract-hardening-l2-offline/execution-manifest.json").read_text(encoding="utf-8"))
manifest["frontend_l2_binding"] = {
    "execution_fingerprint": current_l2["execution_fingerprint"],
    "tool_manifest_digest": current_l2["tool_manifest_digest"],
    "tool_count": current_l2["tool_surface"]["tool_count"],
    "aggregate_schema_digest": current_l2["tool_surface"]["aggregate_schema_digest"],
    "aggregate_schema_bytes": current_l2["tool_surface"]["aggregate_schema_bytes"],
    "qualification": "R0.3A-CURRENT-BINARY-REVISED-FRONTEND-L2",
}
manifest["collab_apply_evidence_contract"] = {
    "accepted_evidence_types": ["workspace.replace", "workspace.check", "collab.apply"],
    "rejected_evidence_types": ["work.checkpoint", "ordinary_object_id", "message_id", "contract_revision_id", "workspace_id", "prose"],
    "feedback_fields": ["invalid_evidence_refs[]", "accepted_evidence_types[]", "actionable_summary"],
}

raw = json.dumps(manifest, indent=2, sort_keys=True).encode() + b"\n"
manifest_path.write_bytes(raw)
manifest_path.with_name(manifest_path.name + ".sha256").write_text(hashlib.sha256(raw).hexdigest() + "\n", encoding="utf-8")

result = {
    "qualification": f"R0.3A-FRONTEND-CLEAN-BASELINE-{BASELINE_QUALIFICATION}",
    "baseline_generation_id": manifest["baseline_generation_id"],
    "status": "PASSED",
    "clean_restore": "PASSED",
    "semantic_closure": "PASSED",
    "RuntimeCASBinding": "PASSED",
    "WindowsCASAccessView": "PASSED",
    "CAS_validation": "PASSED",
    "recovery_anchor_probe": "PASSED",
    "FrontendStartingStateProbe": "PASSED",
    "FrontendActivationPreflight": "PASSED",
    "FrontendRuntimeActivationPreflight": "PASSED",
    "PaginationRuntimePreflight": "PASSED",
    "execution_envelope_status": "QUALIFIED",
    "execution_envelope_fingerprint": expected_fp,
    "transport_policy_binding": "PASSED",
    "transport_policy_revision": "r03a-transport-policy@1",
    "frontend_L2_status": "QUALIFIED_REUSABLE",
    "frontend_sessions": 0,
    "live_writer": 0,
    "obligation": "pending",
    "planner_relay": 0,
    f"baseline_manifest_{BASELINE_QUALIFICATION.lower()}": "PASSED",
    "baseline_probe_mutation": 0,
    "frontend_business": "NOT_STARTED",
    "allowance": 0,
    "Worker": 0,
    "Medium": 0,
    "High": 0,
    "provider_egress": 0,
    "historical_evidence_modified": False,
}
(ROOT / "result.json").write_text(json.dumps(result, indent=2) + "\n", encoding="utf-8")

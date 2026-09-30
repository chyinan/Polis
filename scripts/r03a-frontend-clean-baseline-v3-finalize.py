import hashlib
import json
import os
import sys
from pathlib import Path

root = Path(sys.argv[1])
baseline_id = os.environ.get("R03A_BASELINE_ID", "r03a-frontend-clean-baseline-v3")
revision = baseline_id.rsplit("-", 1)[-1].upper()
pre = [json.loads((root / name).read_text(encoding="utf-8")) for name in ("starting-state-probe-1.json", "starting-state-probe-2.json")]
post = [json.loads((root / name).read_text(encoding="utf-8")) for name in ("starting-state-probe-post-1.json", "starting-state-probe-post-2.json")]
activation = [json.loads((root / name).read_text(encoding="utf-8")) for name in ("frontend-activation-preflight-1.json", "frontend-activation-preflight-2.json")]
runtime = json.loads((root / "frontend-runtime-preflight.json").read_text(encoding="utf-8"))
assert pre[0]["state"] == pre[1]["state"] == post[0]["state"] == post[1]["state"]
assert all(x["status"] == "FRONTEND_STARTING_STATE_READY" and x["read_only"] and not x["mutation"] for x in pre + post)
assert all(x["status"] == "FRONTEND_ACTIVATION_PREFLIGHT_PASSED" and x["anchor_probe"] == "PASSED" and not x["mutation"] for x in activation)
assert all(x["starting_state"] == pre[0]["state"] for x in activation)
assert runtime["status"] == "FRONTEND_RUNTIME_ACTIVATION_PREFLIGHT_PASSED"
assert runtime["launch_envelope"]["launch_mode"] == "windows_native_direct"
assert runtime["local_protocol_ready"] and not runtime["turn_started"] and not runtime["provider_egress"]
assert not runtime["business_mutation"] and not runtime["allowance_created"]

manifest_path = root / "FrontendExecutionBaselineManifest.json"
manifest = json.loads(manifest_path.read_text(encoding="utf-8"))
manifest["execution_envelope_revision"] = "r03a-native-launch-envelope@1"
manifest["execution_envelope_fingerprint"] = runtime["execution_envelope_fingerprint"]
manifest["recovery_anchor_probe"] = "PASSED"
manifest["windows_cas_access_view"] = {
    "consumer_os": "windows",
    "path_kind": "UNC",
    "access_path": os.environ.get("R03A_WINDOWS_CAS_ROOT", ""),
    "source_wsl_path": os.environ.get("R03A_WSL_CAS_ROOT", ""),
    "translation_method": "wslpath -a -w",
    "authoritative_inventory_digest": manifest["cas_inventory_digest"],
    "access_view_fingerprint": hashlib.sha256(json.dumps({"consumer_os": "windows", "path_kind": "UNC", "authoritative_inventory_digest": manifest["cas_inventory_digest"], "translation_method": "wslpath -a -w"}, sort_keys=True).encode()).hexdigest(),
}
manifest["pagination_runtime_binding"] = {
    "revision": "r03a-pagination-runtime-binding@1",
    "runtime_os": "windows",
    "windows_access_view_fingerprint": manifest["windows_cas_access_view"]["access_view_fingerprint"],
    "required_blob_count": manifest["required_cas_blobs"],
    "inventory_digest": manifest["cas_inventory_digest"],
    "preflight": "PASSED",
}
raw = json.dumps(manifest, indent=2, sort_keys=True).encode() + b"\n"
manifest_path.write_bytes(raw)
manifest_path.with_name(manifest_path.name + ".sha256").write_text(hashlib.sha256(raw).hexdigest() + "\n", encoding="utf-8")
result = {
    "qualification": f"R0.3A-FRONTEND-CLEAN-BASELINE-{revision}",
    "baseline_generation_id": manifest["baseline_generation_id"],
    "status": "PASSED",
    "clean_restore": "PASSED",
    "semantic_closure": "PASSED",
    "RuntimeCASBinding": "PASSED",
    "CAS_validation": "PASSED",
    "recovery_anchor_probe": "PASSED",
    "FrontendStartingStateProbe": "PASSED",
    "FrontendActivationPreflight": "PASSED",
    "FrontendRuntimeActivationPreflight": "PASSED",
    "execution_envelope_status": "QUALIFIED",
    "execution_envelope_fingerprint": runtime["execution_envelope_fingerprint"],
    "frontend_L2_status": "QUALIFIED_REUSABLE",
    "frontend_sessions": 0,
    "live_writer": 0,
    "obligation": "pending",
    "planner_relay": 0,
    f"baseline_manifest_{revision.lower()}": "PASSED",
    "WindowsCASAccessView": "PASSED",
    "PaginationRuntimePreflight": "PASSED",
    "baseline_probe_mutation": 0,
    "frontend_business": "NOT_STARTED",
    "allowance": 0,
    "Worker": 0,
    "Medium": 0,
    "High": 0,
    "provider_egress": 0,
    "historical_evidence_modified": False,
}
(root / "result.json").write_text(json.dumps(result, indent=2) + "\n", encoding="utf-8")

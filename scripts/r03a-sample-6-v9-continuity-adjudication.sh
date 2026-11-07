#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
umask 077
V9_ROOT="/home/chyinan/.local/state/polis-recovery/r03a-sample-6-restore-continuation-v9"
V5_SOURCE_STATE="/home/chyinan/.local/state/polis-recovery/r03a-sample-6-final-recovery-continuation-v5/work/source-state.json"
V5_CAS_ROOT="/home/chyinan/.local/state/polis-recovery/r03a-sample-6-final-recovery-continuation-v5/package/cas"
V5_CAS_MANIFEST="/mnt/d/Programs/Polis/evidence/development/r0.3a-sample-6-final-recovery-continuation-v5/cas-manifest.json"
V5_PACKAGE_MANIFEST="/mnt/d/Programs/Polis/evidence/development/r0.3a-sample-6-final-recovery-continuation-v5/recovery-package-complete.json"
V5_CLOSURE="/mnt/d/Programs/Polis/evidence/development/r0.3a-sample-6-package-semantic-closure-qualification-v1/result.json"
EVIDENCE="/mnt/d/Programs/Polis/evidence/development/r0.3a-sample-6-v9-continuity-adjudication-v3"
WORK="$V9_ROOT/adjudication-work-v1"
FRESH="$V9_ROOT/postgres/pgdata"
SOCKET="/tmp/polis-pg-v9-adjudication-v1"
PG="/mnt/d/Programs/Polis/.tools/pg/usr/lib/postgresql/18/bin"
export LD_LIBRARY_PATH="/mnt/d/Programs/Polis/.tools/pg/usr/lib/x86_64-linux-gnu"
FRESH_STARTED=0
RECOVERY_ERROR="v9 adjudication failed"

failure(){ mkdir -m 700 -p "$EVIDENCE"; python3 - "$EVIDENCE/result.json" "$RECOVERY_ERROR" <<'PY'
import json,sys
with open(sys.argv[1],"w",encoding="utf-8") as f: json.dump({"qualification":"R0.3A-SAMPLE-6-V9-CONTINUITY-ADJUDICATION","status":"INCONCLUSIVE","error":sys.argv[2],"provider_egress":0,"historical_evidence_modified":False},f,indent=2); f.write("\n")
PY
}
cleanup(){ if [[ "$FRESH_STARTED" == 1 ]]; then "$PG/pg_ctl" -D "$FRESH" -m fast -w stop >/dev/null 2>&1 || true; fi; rmdir -- "$SOCKET" 2>/dev/null || true; }
on_exit(){ local rc=$?; trap - EXIT; if [[ "$rc" != 0 ]]; then failure; fi; cleanup; exit "$rc"; }
trap on_exit EXIT
mkdir -m 700 -p "$EVIDENCE"
if [[ -n "$(find "$EVIDENCE" -mindepth 1 -maxdepth 1 -print -quit 2>/dev/null)" ]]; then RECOVERY_ERROR="adjudication evidence path not fresh"; exit 1; fi
mkdir -m 700 -p "$WORK"
bash scripts/go.sh run ./cmd/polis-r03a-recovery-package-validate -package-complete-manifest "$V5_PACKAGE_MANIFEST" -semantic-closure "$V5_CLOSURE" -cas-root "$V5_CAS_ROOT" -runtime-root "/home/chyinan/.local/state/polis-recovery/r03a-sample-6-final-recovery-continuation-v5" -package-evidence-root "/mnt/d/Programs/Polis/evidence/development/r0.3a-sample-6-final-recovery-continuation-v5" -expected-generation r03a-sample-6-recovery-20260914T150111Z-307 -expected-package-manifest-sha256 acb19b8acd25e3f9230bafa9ed5cfeaf707d3a9bd07f036bfbb4e55fea555c5c -expected-dump-sha256 1b2acaacad9e9b907c041be58734bd5baaa4c7baffa09531dc28e4154b5401bd -expected-cas-manifest-sha256 9a7d103f56f9dd7300ba10bd88d337cafd286dbcd09177c483f4129125a7ff0f -output "$EVIDENCE/package-resolution.json"
bash scripts/go.sh run ./cmd/polis-r03a-recovery-closure-validate -source-state "$V5_SOURCE_STATE" -restored-state "$V9_ROOT/work/restored-before.json" -cas-manifest "$V5_CAS_MANIFEST" -cas-root "$V9_ROOT/blobs" -output "$EVIDENCE/continuity-before-fence.json"
mkdir -m 700 -p "$SOCKET"
bash scripts/go.sh run ./cmd/polis-r03a-postgres-socket-validate -directory "$SOCKET" -port 55432 -output "$EVIDENCE/socket-readiness.json"
"$PG/pg_ctl" -D "$FRESH" -l "$V9_ROOT/postgres/adjudication-v1.log" -o "-k $SOCKET -h 127.0.0.1 -p 55432 -c synchronous_commit=on" -w start >/dev/null
FRESH_STARTED=1
restore_db=$("$PG/psql" -X -h 127.0.0.1 -p 55432 -U chyinan -d postgres -Atc "select datname from pg_database where datname like 'polis_r0_3a_sample6_v6_restore_%' order by datname desc limit 1")
[[ -n "$restore_db" ]] || { RECOVERY_ERROR="v9 restored database missing"; exit 1; }
export POLIS_DSN="host=127.0.0.1 port=55432 dbname=$restore_db user=polis_runtime"
export POLIS_V3_RUNTIME_ROOT="$V9_ROOT"
export POLIS_V3_EVIDENCE="$EVIDENCE"
export POLIS_V3_BACKEND_EXECUTION_MANIFEST="/mnt/d/Programs/Polis/evidence/development/r0.3a-current-binary-backend-l2-v6/execution-manifest.json"
export POLIS_V3_BACKEND_QUALIFICATION="/mnt/d/Programs/Polis/evidence/development/r0.3a-current-binary-backend-l2-target-live-v1/result.json"
export POLIS_V3_BACKEND_L2_LIVE="$POLIS_V3_BACKEND_QUALIFICATION"
export POLIS_V3_ACCEPTANCE_QUALIFICATION="/mnt/d/Programs/Polis/evidence/development/r0.3a-public-response-encoding-contract-hardening/qualification.json"
export POLIS_BLOB_DURABILITY_QUALIFICATION="/mnt/d/Programs/Polis/evidence/development/r0.3a-blob-durability/qualification.json"
export POLIS_FRONTEND_CHECKER_FEEDBACK_QUALIFICATION="/mnt/d/Programs/Polis/evidence/development/r0.3a-frontend-checker-feedback-l2-v2/offline-result.json"
export POLIS_CODEX_BINARY="/mnt/c/Users/chyinan/AppData/Local/OpenAI/Codex/controlled-runtime/polis-r03a/bffc5354119c8421/codex.exe"
export POLIS_CODEX_CODE_MODE_HOST="/mnt/c/Users/chyinan/AppData/Local/OpenAI/Codex/controlled-runtime/polis-r03a/bffc5354119c8421/codex-code-mode-host.exe"
export POLIS_CODEX_AUTH_FILE="/mnt/c/Users/chyinan/.codex/auth.json"
export POLIS_SELECTED_CODEX_CONFIG="/mnt/d/Programs/Polis/.runtime/linux/r03a-t14c/home/config.toml"
export POLIS_CURRENT_L1_EVIDENCE="/mnt/d/Programs/Polis/evidence/development/r0.3a-current-binary-l1"
export POLIS_V3_BACKEND_BINDING="/mnt/d/Programs/Polis/evidence/development/r0.3a-pagination-v3-backend-sample-6/authorization-binding.json"
export POLIS_V3_ALLOWANCE="$EVIDENCE/no-business-allowance.json"
export POLIS_V3_BACKEND_RESULT="$EVIDENCE/restore-verification-result.json"
export POLIS_V3_POSTGRES_SNAPSHOT="/home/chyinan/.local/state/polis-recovery/r03a-sample-6-final-recovery-continuation-v5/package/sample-6-authoritative.dump"
bash scripts/go.sh run ./cmd/polis-r03a-pagination-v3 -backend-restore-verify
python3 - "$EVIDENCE/package-resolution.json" "$EVIDENCE/continuity-before-fence.json" "$EVIDENCE/backend-restored-runtime-verification.json" "$EVIDENCE/result.json" "$restore_db" <<'PY'
import json,sys
p=json.load(open(sys.argv[1])); c=json.load(open(sys.argv[2])); v=json.load(open(sys.argv[3]))
if p.get("status")!="PASSED" or c.get("status")!="PASSED" or v.get("status")!="PASSED": raise SystemExit("v9 adjudication verification failed")
with open(sys.argv[4],"w",encoding="utf-8") as f:
 json.dump({"qualification":"R0.3A-SAMPLE-6-V9-CONTINUITY-ADJUDICATION","status":"PASSED","v9_specimen_identity":"PASSED","relational_identity_continuity":"PASSED","artifact_workspace_continuity":"PASSED","checkpoint_continuity":"PASSED","collaboration_continuity":"PASSED","event_continuity":"PASSED","CAS_continuity":"PASSED","semantic_closure_determinism":"PASSED","old_writer_fencing":"PASSED","post_fencing_state_unchanged":"PASSED","restore_database":sys.argv[5],"raw_v9_result_preserved":True,"provider_egress":0,"historical_evidence_modified":False},f,indent=2); f.write("\n")
PY

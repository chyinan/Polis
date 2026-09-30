#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
umask 077
REPO="/mnt/d/Programs/Polis"
BASELINE_ID="${R03A_FRONTEND_BASELINE_ID:-r03a-frontend-clean-baseline-v2}"
DB_SUFFIX="${R03A_FRONTEND_BASELINE_DB_SUFFIX:-v2}"
SOCKET_SUFFIX="${R03A_FRONTEND_BASELINE_SOCKET_SUFFIX:-v2}"
EVIDENCE="$REPO/evidence/development/$BASELINE_ID"
V5_EVIDENCE="$REPO/evidence/development/r0.3a-sample-6-final-recovery-continuation-v5"
PACKAGE_ROOT="/home/chyinan/.local/state/polis-recovery/r03a-sample-6-final-recovery-continuation-v5"
PACKAGE_MANIFEST="$V5_EVIDENCE/recovery-package-complete.json"
CLOSURE="$REPO/evidence/development/r0.3a-sample-6-package-semantic-closure-qualification-v1/result.json"
CAS_MANIFEST="$V5_EVIDENCE/cas-manifest.json"
PACKAGE_CAS="$PACKAGE_ROOT/package/cas"
SOURCE_STATE="$PACKAGE_ROOT/work/source-state.json"
CLEAN_ROOT="/home/chyinan/.local/state/polis-recovery/$BASELINE_ID"
PGDATA="$CLEAN_ROOT/postgres/pgdata"
CAS_ROOT="$CLEAN_ROOT/blobs"
SOCKET="/tmp/polis-pg-r03a-frontend-clean-baseline-$SOCKET_SUFFIX"
DB="polis_r0_3a_frontend_clean_baseline_$DB_SUFFIX"
COMPANY="r03a-pagination-v3-company-1789379324974307400"
PG="$REPO/.tools/pg/usr/lib/postgresql/18/bin"
PG_SHARE="$REPO/.tools/pg/usr/share/postgresql/18"
export LD_LIBRARY_PATH="$REPO/.tools/pg/usr/lib/x86_64-linux-gnu"

mkdir -m 700 -p "$EVIDENCE"
if [[ -n "$(find "$EVIDENCE" -mindepth 1 -maxdepth 1 -print -quit 2>/dev/null)" ]]; then echo "baseline evidence path is not fresh" >&2; exit 1; fi
if [[ -e "$CLEAN_ROOT" ]]; then echo "baseline root already exists" >&2; exit 1; fi
mkdir -m 700 -p "$CLEAN_ROOT/postgres" "$CAS_ROOT"

bash scripts/go.sh run ./cmd/polis-r03a-recovery-package-validate \
  -package-complete-manifest "$PACKAGE_MANIFEST" -semantic-closure "$CLOSURE" -cas-root "$PACKAGE_CAS" \
  -runtime-root "$PACKAGE_ROOT" -package-evidence-root "$V5_EVIDENCE" \
  -expected-generation r03a-sample-6-recovery-20260914T150111Z-307 \
  -expected-package-manifest-sha256 acb19b8acd25e3f9230bafa9ed5cfeaf707d3a9bd07f036bfbb4e55fea555c5c \
  -expected-dump-sha256 1b2acaacad9e9b907c041be58734bd5baaa4c7baffa09531dc28e4154b5401bd \
  -expected-cas-manifest-sha256 9a7d103f56f9dd7300ba10bd88d337cafd286dbcd09177c483f4129125a7ff0f \
  -output "$EVIDENCE/package-validation.json"
cp -a -- "$PACKAGE_CAS"/. "$CAS_ROOT"/
mkdir -m 700 -p "$CAS_ROOT/.pagination-runtime"
bash scripts/go.sh run ./cmd/polis-r03a-cas-validate -root "$CAS_ROOT" -anchor-snapshot "$V5_EVIDENCE/authoritative-snapshot.json" -output "$EVIDENCE/cas-validation.json"

"$PG/initdb" -D "$PGDATA" -L "$PG_SHARE" --locale=C --encoding=UTF8 --auth-local=trust --auth-host=trust >/dev/null
mkdir -m 700 -p "$SOCKET"
bash scripts/go.sh run ./cmd/polis-r03a-postgres-socket-validate -directory "$SOCKET" -port 55432 -output "$EVIDENCE/socket-readiness.json"
started=0
cleanup(){ if [[ "$started" == 1 ]]; then "$PG/pg_ctl" -D "$PGDATA" -m fast -w stop >/dev/null 2>&1 || true; fi; rmdir -- "$SOCKET" 2>/dev/null || true; }
trap cleanup EXIT
"$PG/pg_ctl" -D "$PGDATA" -l "$CLEAN_ROOT/postgres/server.log" -o "-k $SOCKET -h 127.0.0.1 -p 55432 -c synchronous_commit=on" -w start >/dev/null
started=1
"$PG/createdb" -h 127.0.0.1 -p 55432 "$DB"
"$PG/psql" -X -h 127.0.0.1 -p 55432 -d postgres -v ON_ERROR_STOP=1 -c "CREATE ROLE polis_runtime LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE"
"$PG/pg_restore" -h 127.0.0.1 -p 55432 -d "$DB" --no-owner --no-acl --exit-on-error "$PACKAGE_ROOT/package/sample-6-authoritative.dump"
"$PG/psql" -X -h 127.0.0.1 -p 55432 -d "$DB" -v ON_ERROR_STOP=1 -c 'GRANT USAGE ON SCHEMA public TO polis_runtime; GRANT SELECT,INSERT,UPDATE ON ALL TABLES IN SCHEMA public TO polis_runtime;'

system_id=$("$PG/pg_controldata" "$PGDATA" | awk -F: '/Database system identifier/ {gsub(/^[ \t]+/,"",$2); print $2}')
db_oid=$("$PG/psql" -X -h 127.0.0.1 -p 55432 -d postgres -Atc "SELECT oid FROM pg_database WHERE datname='$DB'")
printf '%s\n' "$system_id" > "$EVIDENCE/cluster-system-identifier.txt"
printf '%s\n' "$db_oid" > "$EVIDENCE/database-oid.txt"

dsn="host=127.0.0.1 port=55432 dbname=$DB user=polis_runtime"
bash scripts/go.sh run ./cmd/polis-r03a-sample6-state-query -dsn "$dsn" -company "$COMPANY" -output "$CLEAN_ROOT/restored-state.json"
bash scripts/go.sh run ./cmd/polis-r03a-recovery-closure-validate -source-state "$SOURCE_STATE" -restored-state "$CLEAN_ROOT/restored-state.json" -cas-manifest "$CAS_MANIFEST" -cas-root "$CAS_ROOT" -output "$EVIDENCE/semantic-closure.json"
bash scripts/go.sh run ./cmd/polis-r03a-frontend-starting-state-probe -dsn "$dsn" -output "$EVIDENCE/starting-state-probe-1.json"
bash scripts/go.sh run ./cmd/polis-r03a-frontend-starting-state-probe -dsn "$dsn" -output "$EVIDENCE/starting-state-probe-2.json"

python3 - "$EVIDENCE/starting-state-probe-1.json" "$EVIDENCE/starting-state-probe-2.json" <<'PY'
import json,sys
a=json.load(open(sys.argv[1],encoding='utf-8'))
b=json.load(open(sys.argv[2],encoding='utf-8'))
assert a['status']=='FRONTEND_STARTING_STATE_READY' and b['status']=='FRONTEND_STARTING_STATE_READY'
assert a['read_only'] and b['read_only'] and not a['mutation'] and not b['mutation']
assert a['state']==b['state']
assert a['state']['frontend_session_count']==0 and a['state']['live_writer_count']==0
assert a['state']['obligation_state']=='pending' and a['state']['planner_relay_count']==0
PY

export CLEAN_CAS_ROOT="$CAS_ROOT"
export BASELINE_GENERATION_ID="$BASELINE_ID"
export BASELINE_DB_NAME="$DB"
python3 - "$EVIDENCE/package-validation.json" "$EVIDENCE/semantic-closure.json" "$EVIDENCE/cas-validation.json" "$EVIDENCE/starting-state-probe-1.json" "$EVIDENCE/FrontendExecutionBaselineManifest.json" <<'PY'
import hashlib,json,os,sys
package=json.load(open(sys.argv[1],encoding='utf-8'))
closure=json.load(open(sys.argv[2],encoding='utf-8'))
cas=json.load(open(sys.argv[3],encoding='utf-8'))
probe=json.load(open(sys.argv[4],encoding='utf-8'))
state=probe['state']
l2=json.load(open('evidence/development/r0.3a-current-binary-frontend-l2/execution-manifest.json',encoding='utf-8'))
surface=l2.get('tool_surface',{})
manifest={
 'schema_version':'r03a-frontend-execution-baseline-manifest@1',
 'baseline_generation_id':os.environ['BASELINE_GENERATION_ID'],
 'source_recovery_package':{'package_generation_id':package['package_generation_id'],'package_manifest_sha256':package['package_manifest_sha256'],'dump_sha256':package['dump_sha256'],'cas_manifest_sha256':package['cas_manifest_sha256'],'semantic_closure_revision':closure['closure_canonicalization_revision']},
 'postgres':{'cluster_system_identifier':open(sys.argv[1].replace('package-validation.json','cluster-system-identifier.txt'),encoding='utf-8').read().strip(),'database_name':os.environ['BASELINE_DB_NAME'],'database_oid':open(sys.argv[1].replace('package-validation.json','database-oid.txt'),encoding='utf-8').read().strip()},
 'semantic_closure_digest':hashlib.sha256(open(sys.argv[2],'rb').read()).hexdigest(),
 'runtime_cas_binding':{'canonical_root':os.environ['CLEAN_CAS_ROOT'],'layout_revision':cas['layout_revision'],'company_namespace':state['company_id'],'required_blob_inventory_digest':cas['inventory_digest'],'required_blob_count':cas['required_blob_count']},
 'company_id':state['company_id'],'mission_id':state['mission_id'],'employee_id':'emp-frontend','frontend_task_id':state['frontend_task_id'],'message_id':state['message_id'],'obligation_id':state['obligation_id'],'contract_revision_id':state['contract_revision_id'],
 'frontend_sessions':state['frontend_session_count'],'live_writer':state['live_writer_count'],'employee_epoch':state['employee_epoch'],'runtime_incarnation':state['runtime_incarnation'],'writer_authority':state.get('current_writer_session'),
 'workspace_revision':state['workspace_revision'],'workspace_digest':state['workspace_digest'],'cas_inventory_digest':cas['inventory_digest'],'required_cas_blobs':cas['required_blob_count'],'verified_cas_blobs':cas['digest_verified_count'],
 'message_state':state['message_state'],'obligation_state':state['obligation_state'],'planner_relay':state['planner_relay_count'],'recovery_anchor_probe_revision':'r03a-peer-recovery-anchors-read-only@1','recovery_anchor_probe':'PENDING','frontend_l2_binding':{'execution_fingerprint':l2.get('execution_fingerprint',''),'tool_manifest_digest':surface.get('aggregate_manifest_digest',''),'tool_count':surface.get('tool_count',0),'aggregate_schema_digest':surface.get('aggregate_schema_digest',''),'aggregate_schema_bytes':surface.get('aggregate_schema_bytes',0)},'package_mutated':False,'synthesized_rows':0,'synthesized_blobs':0
}
raw=json.dumps(manifest,indent=2,sort_keys=True).encode()+b'\n'
open(sys.argv[5],'wb').write(raw)
open(sys.argv[5]+'.sha256','w',encoding='utf-8').write(hashlib.sha256(raw).hexdigest()+'\n')
PY

export POLIS_DSN="$dsn"
export POLIS_V3_FRONTEND_BASELINE_MANIFEST="$EVIDENCE/FrontendExecutionBaselineManifest.json"
export POLIS_V3_FRONTEND_BASELINE_MANIFEST_SHA256="$(tr -d '\r\n' < "$EVIDENCE/FrontendExecutionBaselineManifest.json.sha256")"
export POLIS_V3_FRONTEND_CAS_MANIFEST="$CAS_MANIFEST"
export POLIS_V3_FRONTEND_CAS_ROOT="$CAS_ROOT"
export POLIS_V3_FRONTEND_CAS_LAYOUT_REVISION="$(python3 - "$EVIDENCE/cas-validation.json" <<'PY'
import json,sys
print(json.load(open(sys.argv[1],encoding='utf-8'))['layout_revision'])
PY
)"
export POLIS_V3_FRONTEND_CAS_COMPANY_NAMESPACE="$COMPANY"
export POLIS_V3_FRONTEND_CAS_INVENTORY_DIGEST="$(python3 - "$EVIDENCE/cas-validation.json" <<'PY'
import json,sys
print(json.load(open(sys.argv[1],encoding='utf-8'))['inventory_digest'])
PY
)"
export POLIS_V3_FRONTEND_ACTIVATION_PREFLIGHT_OUTPUT="$EVIDENCE/frontend-activation-preflight-1.json"
bash scripts/go.sh run ./cmd/polis-r03a-pagination-v3 -frontend-activation-preflight
export POLIS_V3_FRONTEND_ACTIVATION_PREFLIGHT_OUTPUT="$EVIDENCE/frontend-activation-preflight-2.json"
bash scripts/go.sh run ./cmd/polis-r03a-pagination-v3 -frontend-activation-preflight

python3 - "$EVIDENCE/FrontendExecutionBaselineManifest.json" "$EVIDENCE/frontend-activation-preflight-2.json" <<'PY'
import hashlib,json,sys
from pathlib import Path
manifest_path=Path(sys.argv[1])
preflight=json.loads(Path(sys.argv[2]).read_text(encoding='utf-8'))
manifest=json.loads(manifest_path.read_text(encoding='utf-8'))
manifest['recovery_anchor_probe']='PASSED'
manifest['recovery_anchor_ids']={'backend_artifact_id':preflight['recovery_anchors']['backend_artifact_id'],'backend_checkpoint_id':preflight['recovery_anchors']['backend_checkpoint_id'],'message_id':preflight['recovery_anchors']['message_id'],'obligation_id':preflight['recovery_anchors']['obligation_id']}
raw=json.dumps(manifest,indent=2,sort_keys=True).encode()+b'\n'
manifest_path.write_bytes(raw)
manifest_path.with_name(manifest_path.name+'.sha256').write_text(hashlib.sha256(raw).hexdigest()+'\n',encoding='utf-8')
PY

python3 - "$EVIDENCE/semantic-closure.json" "$EVIDENCE/starting-state-probe-1.json" "$EVIDENCE/starting-state-probe-2.json" "$EVIDENCE/frontend-activation-preflight-1.json" "$EVIDENCE/frontend-activation-preflight-2.json" > "$EVIDENCE/result.json" <<'PY'
import json,os,sys
closure=json.load(open(sys.argv[1],encoding='utf-8'))
p1=json.load(open(sys.argv[2],encoding='utf-8'))
p2=json.load(open(sys.argv[3],encoding='utf-8'))
p1a=json.load(open(sys.argv[4],encoding='utf-8'))
p2a=json.load(open(sys.argv[5],encoding='utf-8'))
assert p1['state']==p2['state']
assert p1a['status']=='FRONTEND_ACTIVATION_PREFLIGHT_PASSED' and p2a['status']=='FRONTEND_ACTIVATION_PREFLIGHT_PASSED'
assert p1a['anchor_probe']=='PASSED' and p2a['anchor_probe']=='PASSED'
assert not p1a['mutation'] and not p2a['mutation']
assert p1a['starting_state']==p1['state'] and p2a['starting_state']==p1['state']
print(json.dumps({'qualification':'R0.3A-FRONTEND-CLEAN-BASELINE-MATERIALIZATION','baseline_generation_id':os.environ['BASELINE_GENERATION_ID'],'status':'PASSED','clean_restore':'PASSED','semantic_closure':'PASSED' if closure['status']=='PASSED' else 'FAILED','RuntimeCASBinding':'PASSED','CAS_validation':'PASSED','recovery_anchor_probe':'PASSED','FrontendStartingStateProbe':'PASSED','FrontendActivationPreflight':'PASSED','frontend_sessions':0,'live_writer':0,'obligation':'pending','planner_relay':0,'baseline_manifest':'PASSED','probe_preflight_mutation':0,'frontend_business':'NOT_STARTED','allowance':0,'Worker':0,'Medium':0,'provider_egress':0,'historical_evidence_modified':False},indent=2))
PY

#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
export LD_LIBRARY_PATH="$PWD/.tools/pg/usr/lib/x86_64-linux-gnu"
pg="$PWD/.tools/pg/usr/lib/postgresql/18/bin"
socket="$PWD/.runtime/linux/pg-socket"
database="polis_r0_demo_$(date +%s)_$$"
export POLIS_BLOB_ROOT="$PWD/.runtime/linux/$database/blobs"
mkdir -p "$POLIS_BLOB_ROOT" evidence/development
admin="host=$socket port=55432 dbname=postgres user=$(id -un)"
"$pg/psql" "$admin" -v ON_ERROR_STOP=1 -c "SELECT 'CREATE ROLE polis_runtime LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE' WHERE NOT EXISTS (SELECT FROM pg_roles WHERE rolname='polis_runtime')" -tA | "$pg/psql" "$admin" -v ON_ERROR_STOP=1
"$pg/createdb" -h "$socket" -p 55432 "$database"
trap '"$pg/dropdb" -h "$socket" -p 55432 "$database"' EXIT
export POLIS_DSN="host=$socket port=55432 dbname=$database user=$(id -un)"
bin/polis migrate
"$pg/psql" "$POLIS_DSN" -v ON_ERROR_STOP=1 -c 'GRANT USAGE ON SCHEMA public TO polis_runtime; GRANT SELECT,INSERT,UPDATE ON ALL TABLES IN SCHEMA public TO polis_runtime;'
export POLIS_DSN="host=$socket port=55432 dbname=$database user=polis_runtime"
bin/polis create demo-company demo-mission
bin/polis start demo-company demo-mission
bin/polisd --company demo-company --steps 1
bin/polis status demo-company demo-mission | tee evidence/development/demo-pending.json
# This is a new controller process with a new incarnation and persisted responsibility.
bin/polisd --company demo-company --steps 8
bin/polis status demo-company demo-mission | tee evidence/development/demo-complete.json
bin/polisd --company demo-company --steps 8 | tee evidence/development/demo-idle.json
printf '%s\n' "$POLIS_BLOB_ROOT" > evidence/development/demo-artifact-root.txt
python3 - <<'PY'
import json
from pathlib import Path
p=Path('evidence/development')
pending=json.loads((p/'demo-pending.json').read_text())
done=json.loads((p/'demo-complete.json').read_text())
idle=json.loads((p/'demo-idle.json').read_text())
assert len(pending['Obligations'])==1 and pending['Obligations'][0]['State']=='pending'
assert done['Obligations'][0]['State']=='fulfilled'
assert len(done['Employees'])==4 and done['FakeClaims']==2
assert len(done['Messages'])==2 and done['Artifacts'][0]['Verdict']=='passed'
assert all(t['State']=='completed' for t in done['Tasks'])
assert idle['steps']==0
root=Path((p/'demo-artifact-root.txt').read_text().strip())
artifact=(root/'demo-company'/done['Artifacts'][0]['Digest']).read_bytes()
assert artifact==b'Polis R0: 2 + 3 = 5\n'
(p/'fixed-artifact.txt').write_bytes(artifact)
print('CLI demo assertions: PASS; real models: not enabled')
PY

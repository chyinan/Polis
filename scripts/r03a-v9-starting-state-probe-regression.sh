#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
V9_ROOT="/home/chyinan/.local/state/polis-recovery/r03a-sample-6-restore-continuation-v9"
PGDATA="$V9_ROOT/postgres/pgdata"
SOCKET="/tmp/polis-pg-r03a-v9-starting-state-probe"
PG="/mnt/d/Programs/Polis/.tools/pg/usr/lib/postgresql/18/bin"
export LD_LIBRARY_PATH="/mnt/d/Programs/Polis/.tools/pg/usr/lib/x86_64-linux-gnu"
OUT="/mnt/d/Programs/Polis/evidence/development/r0.3a-v9-frontend-session-provenance-forensics"
DB="polis_r0_3a_sample6_v6_restore_20260914164615_305"
started=0
cleanup(){ if [[ "$started" == 1 ]]; then "$PG/pg_ctl" -D "$PGDATA" -m fast -w stop >/dev/null 2>&1 || true; fi; rmdir -- "$SOCKET" 2>/dev/null || true; }
trap cleanup EXIT
mkdir -m 700 -p "$SOCKET"
"$PG/pg_ctl" -D "$PGDATA" -l "$V9_ROOT/postgres/starting-state-probe.log" -o "-k $SOCKET -h 127.0.0.1 -p 55432 -c synchronous_commit=on" -w start >/dev/null
started=1
dsn="host=127.0.0.1 port=55432 dbname=$DB user=polis_runtime"
set +e
bash scripts/go.sh run ./cmd/polis-r03a-frontend-starting-state-probe -dsn "$dsn" -output "$OUT/probe-regression.json"
probe_status=$?
set -e
[[ "$probe_status" -ne 0 ]] || { echo "contaminated v9 unexpectedly accepted" >&2; exit 1; }
set +e
bash scripts/go.sh run ./cmd/polis-r03a-frontend-starting-state-probe -dsn "$dsn" -output "$OUT/probe-regression-after.json"
probe_status_after=$?
set -e
[[ "$probe_status_after" -ne 0 ]] || { echo "contaminated v9 unexpectedly accepted on second read" >&2; exit 1; }
python3 - "$OUT/probe-regression.json" "$OUT/probe-regression-after.json" <<'PY'
import json,sys
before=json.load(open(sys.argv[1],encoding='utf-8'))
after=json.load(open(sys.argv[2],encoding='utf-8'))
for record in (before,after):
    assert record["status"] == "FRONTEND_STARTING_STATE_MISMATCH"
    assert record["read_only"] is True
    assert record["mutation"] is False
    assert "frontend_sessions_expected_zero" in record["reasons"]
assert before["state"] == after["state"]
PY
echo 'v9-starting-state-probe-regression=PASS'

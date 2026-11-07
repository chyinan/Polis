#!/usr/bin/env bash
set -euo pipefail
cd /mnt/d/Programs/Polis
POLIS_WORKBENCH_ADDR=127.0.0.1:8081 POLIS_DSN="host=/tmp/polis-r06-browser-pg/sock port=55441 dbname=polis_r0_r06_browser user=polis_runtime" POLIS_BLOB_ROOT=/tmp/polis-r06-browser-cas bash scripts/go.sh run ./cmd/polis serve

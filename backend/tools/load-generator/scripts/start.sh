#!/bin/sh
set -eu

cd /xk6

echo "k6 REST API on :6565; the run orchestrator scales VUs from 0"
echo "PODS_PER_VU=${PODS_PER_VU:-1} MAX_VUS=${MAX_VUS:-600} DURATION=${DURATION:-2h} TESTID=${TESTID:-dev}"

# shellcheck disable=SC2086 # the remote-write flag is intentionally word-split
exec ./k6 run --address 0.0.0.0:6565 \
    ${K6_PROMETHEUS_RW_SERVER_URL:+-o experimental-prometheus-rw} \
    --summary-mode=full scripts/scenario.js

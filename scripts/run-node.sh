#!/usr/bin/env bash
# Usage: ./scripts/run-node.sh <node-num> [--bootstrap]
set -euo pipefail
n="${1:?node num required}"
shift || true
raft_port=$((7000 + n))
http_port=$((8000 + n))
data="./data/node${n}"
mkdir -p "$data"
exec go run . \
  --id "node${n}" \
  --raft "127.0.0.1:${raft_port}" \
  --http "127.0.0.1:${http_port}" \
  --data "$data" \
  "$@"

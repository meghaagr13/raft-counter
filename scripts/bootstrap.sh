#!/usr/bin/env bash
set -euo pipefail
rm -rf data
mkdir -p data logs
cleanup() { jobs -p | xargs -r kill 2>/dev/null || true; }
trap cleanup EXIT
./scripts/run-node.sh 1 --bootstrap > logs/node1.log 2>&1 &
sleep 2
./scripts/run-node.sh 2 > logs/node2.log 2>&1 &
./scripts/run-node.sh 3 > logs/node3.log 2>&1 &
sleep 2
curl -sf -X POST "http://127.0.0.1:8001/join?id=node2&addr=127.0.0.1:7002" && echo
curl -sf -X POST "http://127.0.0.1:8001/join?id=node3&addr=127.0.0.1:7003" && echo
echo "Cluster up. Tailing logs (Ctrl-C to stop)..."
tail -F logs/node1.log logs/node2.log logs/node3.log

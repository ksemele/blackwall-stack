#!/usr/bin/env bash
#
# blackwall-scan host runner — build the image (if needed) and scan.
# Runs on your laptop. Mounts ./targets and ./output.
#
# Usage:
#   ./run.sh                 # top-1000 ports, no nuclei
#   RUN_NUCLEI=1 ./run.sh    # + exposure/misconfig/takeover checks
#   PORTS="-p -" ./run.sh    # full 65535 TCP (slow)
#   ./run.sh --build         # force rebuild image
#
set -euo pipefail
cd "$(dirname "$0")"

IMAGE="blackwall-scan:latest"
mkdir -p output

if [[ "${1:-}" == "--build" ]] || ! docker image inspect "$IMAGE" >/dev/null 2>&1; then
  echo "[run] building $IMAGE ..."
  docker build -t "$IMAGE" .
fi

if [[ ! -f targets/targets.txt ]]; then
  echo "[run] no targets/targets.txt — copy targets/targets.txt.example and fill it in" >&2
  exit 1
fi

echo "[run] scanning targets from ./targets/targets.txt ..."
docker run --rm \
  -v "$(pwd)/targets:/targets:ro" \
  -v "$(pwd)/output:/output" \
  -e PORTS="${PORTS:--top-ports 1000}" \
  -e RUN_NUCLEI="${RUN_NUCLEI:-0}" \
  -e NUCLEI_TAGS="${NUCLEI_TAGS:-exposure,misconfig,takeover,default-login}" \
  -e RATE="${RATE:-1000}" \
  "$IMAGE"

echo "[run] results in ./output/"

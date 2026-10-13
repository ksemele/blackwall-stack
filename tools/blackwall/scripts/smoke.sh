#!/usr/bin/env bash
# blackwall smoke test: --once against the docker test env.
#
# Brings up the docker test env (or assumes it is already up), waits for
# the Wazuh manager API to respond, runs blackwall --once with a tmp
# config pointing at the manager, greps for an Active agent.

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/../../.." && pwd)"
COMPOSE_DIR="${REPO_ROOT}/tests/docker"
MANAGER_HOST="${BW_SMOKE_HOST:-172.28.0.10}"

cleanup_cfg=""
trap '[[ -n "$cleanup_cfg" ]] && rm -rf "$cleanup_cfg"' EXIT

echo "[smoke] checking docker compose state in ${COMPOSE_DIR}"
if ! docker compose -f "${COMPOSE_DIR}/docker-compose.yml" ps --services --filter status=running | grep -q .; then
    echo "[smoke] starting compose"
    docker compose -f "${COMPOSE_DIR}/docker-compose.yml" up -d
fi

echo "[smoke] waiting for Wazuh API on ${MANAGER_HOST}:55000"
for i in {1..60}; do
    if curl -sk -o /dev/null -w "%{http_code}" "https://${MANAGER_HOST}:55000/" | grep -qE '^(200|401)$'; then
        echo "[smoke] API responding"
        break
    fi
    sleep 2
done

cleanup_cfg="$(mktemp -d)"
cfg="${cleanup_cfg}/config.yaml"
export BW_SMOKE_PASSWORD="${BW_SMOKE_PASSWORD:-wazuh}"

cat >"${cfg}" <<YAML
current: smoke
select_context_on_start: false
contexts:
  smoke:
    description: "smoke target"
    wazuh:
      server: ${MANAGER_HOST}
      port: 55000
      user: wazuh
      password: \${BW_SMOKE_PASSWORD}
      tls:
        insecure_skip_verify: true
ui:
  refresh_interval: 3s
logging:
  level: warn
  file: ${cleanup_cfg}/blackwall.log
YAML

cd "${REPO_ROOT}/tools/blackwall"
go build -o "${cleanup_cfg}/blackwall" ./cmd/blackwall

echo "[smoke] running blackwall --once"
out="$("${cleanup_cfg}/blackwall" --config "${cfg}" --once)"
echo "$out"

if ! echo "$out" | grep -q "active"; then
    echo "[smoke] FAIL: no active agent in output" >&2
    exit 1
fi
echo "[smoke] OK"

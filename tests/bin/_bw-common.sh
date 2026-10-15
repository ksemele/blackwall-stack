# shellcheck shell=bash
#
# Shared variables and helpers for bw-lab / bw-check.
# Sourced, never executed directly.

BW_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
BW_PROFILE="${BW_PROFILE:-ubuntu2204}"
BW_COMPOSE="$BW_ROOT/tests/docker/docker-compose.yml"
BW_SSH_CONFIG="$BW_ROOT/tests/docker/ssh_config"
BW_INVENTORY="$BW_ROOT/tests/inventory/hosts.yml"

# Root password inside the test containers. localhost only — see tests/README.md.
BW_SSH_PASSWORD="${BW_SSH_PASSWORD:-blackwall}"

# Web endpoints of the test environment.
BW_GRAFANA_URL="http://localhost:3000"
BW_GRAFANA_AUTH="admin:blackwall"
BW_PROM_URL="http://localhost:9090"
BW_DASHBOARD_URL="https://localhost:5601"
BW_INDEXER_URL="https://127.0.0.1:9200"
BW_INDEXER_AUTH="admin:Blackwall1!"
# Wazuh manager REST API (wazuh_manager_api_password in tests/inventory/group_vars).
BW_API_AUTH="wazuh:wazuh"

if [[ -t 1 ]]; then
  BW_RED=$'\033[31m'; BW_GREEN=$'\033[32m'; BW_YELLOW=$'\033[33m'
  BW_BOLD=$'\033[1m'; BW_DIM=$'\033[2m'; BW_RESET=$'\033[0m'
else
  BW_RED=''; BW_GREEN=''; BW_YELLOW=''; BW_BOLD=''; BW_DIM=''; BW_RESET=''
fi

bw_info() { printf '%s==>%s %s\n' "$BW_BOLD" "$BW_RESET" "$*"; }
bw_die()  { printf '%serror:%s %s\n' "$BW_RED" "$BW_RESET" "$*" >&2; exit 1; }

bw_require_docker() {
  command -v docker >/dev/null 2>&1 || bw_die "docker not found in PATH"
  docker info >/dev/null 2>&1 \
    || bw_die "docker daemon is not responding — start OrbStack or Docker Desktop"
}

bw_require_ansible() {
  command -v ansible-playbook >/dev/null 2>&1 \
    || bw_die "ansible-playbook not found in PATH (see tests/README.md, controller prerequisites)"
}

bw_compose() {
  docker compose -f "$BW_COMPOSE" --profile "$BW_PROFILE" "$@"
}

# bw_ansible <module-dir> <playbook-relative-path> [extra args...]
#
# Runs from the module directory: each module has its own ansible.cfg with
# roles_path, and roles are not found without it.
bw_ansible() {
  local module="$1" playbook="$2"
  shift 2
  ( cd "$BW_ROOT/$module" && ansible-playbook -i "$BW_INVENTORY" "$playbook" "$@" )
}

# bw_ansible_adhoc <module-dir> <pattern> [extra args...]
bw_ansible_adhoc() {
  local module="$1" pattern="$2"
  shift 2
  ( cd "$BW_ROOT/$module" && ansible "$pattern" -i "$BW_INVENTORY" "$@" )
}

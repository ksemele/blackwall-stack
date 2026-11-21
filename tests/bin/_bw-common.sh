# shellcheck shell=bash
#
# Shared variables and helpers for bw-lab / bw-check.
# Sourced, never executed directly.

BW_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
BW_PROFILE="${BW_PROFILE:-ubuntu2204}"
BW_COMPOSE="$BW_ROOT/tests/docker/docker-compose.yml"
BW_SSH_CONFIG="${BW_SSH_CONFIG:-$BW_ROOT/tests/docker/ssh_config}"
BW_INVENTORY="$BW_ROOT/tests/inventory/hosts.yml"

# Root password inside the test containers. localhost only — see tests/README.md.
BW_SSH_PASSWORD="${BW_SSH_PASSWORD:-blackwall}"

# Host ports of the monitoring containers. Exported so that bw_compose passes
# the same values to docker compose, which interpolates them into the published
# ports; unset means the compose defaults (9090 / 3000). Set them when those
# ports are taken on the host — e.g. BW_LAB_PROM_PORT=19090 BW_LAB_GRAFANA_PORT=13000.
export BW_LAB_PROM_PORT="${BW_LAB_PROM_PORT:-9090}"
export BW_LAB_GRAFANA_PORT="${BW_LAB_GRAFANA_PORT:-3000}"

# cgroup namespace of the systemd nodes, interpolated into the compose anchor.
# Exported for the same reason as the two ports. Default `host` (macOS container
# runtimes); Linux + rootless podman needs `private` — see tests/README.md.
export BW_LAB_CGROUP="${BW_LAB_CGROUP:-host}"

# Web endpoints of the test environment.
BW_GRAFANA_URL="http://localhost:${BW_LAB_GRAFANA_PORT}"
BW_GRAFANA_AUTH="admin:blackwall"
BW_PROM_URL="http://localhost:${BW_LAB_PROM_PORT}"
BW_DASHBOARD_URL="https://localhost:5601"
BW_INDEXER_URL="https://127.0.0.1:9200"
BW_INDEXER_AUTH="admin:Blackwall1!"
# Wazuh manager REST API (wazuh_manager_api_user/password in group_vars).
BW_API_AUTH="wazuh:wazuh"

# --- SSO lab (bw-lab up --sso) -------------------------------------------------
# The IdP is compose profile `sso`: a plain `bw-lab up` never starts it.
# BW_SSO also drives bw_sso_enabled in tests/inventory/group_vars, so the
# ansible roles provision the OIDC parts only for an SSO lab — both are exported
# for that reason.
BW_SSO="${BW_SSO:-0}"
# Loopback port for the IdP. 8080/9090/3000 are taken on the shared lab host.
BW_LAB_IDP_PORT="${BW_LAB_IDP_PORT:-18080}"
export BW_SSO BW_LAB_IDP_PORT

# Issuer as blackwall on the host sees it, and as the indexer sees it inside the
# lab network. Keycloak publishes one issuer (the host one) for both — see
# tests/docker/keycloak/README.md.
BW_IDP_URL="http://127.0.0.1:${BW_LAB_IDP_PORT}"
BW_IDP_NETWORK_URL="http://black-idp-00:8080"
BW_IDP_REALM="blackwall-lab"
BW_IDP_DISCOVERY="${BW_IDP_URL}/realms/${BW_IDP_REALM}/.well-known/openid-configuration"
# Clients from tests/docker/keycloak/blackwall-lab-realm.json. `blackwall-cli` is public
# (device code / auth code + PKCE) and is the client blackwall itself uses;
# blackwall-test is confidential with direct access grants so bw-check can get
# a token without a browser.
BW_IDP_CLIENT="blackwall-cli"
BW_IDP_TEST_CLIENT="blackwall-test"
BW_IDP_TEST_CLIENT_SECRET="blackwall-lab-test-secret"   # lab-only
# Lab users and their groups: alice in analysts, bob in read-only.
BW_IDP_ANALYST_USER="alice"
BW_IDP_ANALYST_PASSWORD="blackwall-lab"                 # lab-only
BW_IDP_OUTSIDER_USER="bob"
BW_IDP_OUTSIDER_PASSWORD="blackwall-lab"                # lab-only
BW_IDP_ANALYST_GROUP="analysts"
BW_IDP_OUTSIDER_GROUP="read-only"
# Issuer as both the host and the lab network see it (the IdP's --hostname, see
# tests/docker/keycloak/README.md); blackwall's auth.issuer and the indexer's
# openid_connect_url must agree with this.
BW_IDP_ISSUER="${BW_IDP_URL}/realms/${BW_IDP_REALM}"
# Names provisioned by the ansible SSO tasks (group_vars/wazuh_managers.yml).
BW_INDEXER_OIDC_AUTH_DOMAIN="openid_auth_domain"
BW_INDEXER_OIDC_ROLE="blackwall_read"
BW_API_URL="https://127.0.0.1:55000"
BW_API_RUN_AS_USER="svc-blackwall"
BW_API_RUN_AS_PASSWORD="Blackwall-Lab1!"        # lab-only
BW_API_RUN_AS_RULE="blackwall_groups_analysts"

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
  if [[ "$BW_SSO" == "1" ]]; then
    docker compose -f "$BW_COMPOSE" --profile "$BW_PROFILE" --profile sso "$@"
  else
    docker compose -f "$BW_COMPOSE" --profile "$BW_PROFILE" "$@"
  fi
}

# bw_compose with every profile, so `down` also removes an IdP that was started
# from another shell — BW_SSO lives in the shell that set it, not in compose.
bw_compose_all_profiles() {
  docker compose -f "$BW_COMPOSE" --profile "$BW_PROFILE" --profile sso "$@"
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

# --- RHEL VM (Lima): a real kernel for what containers cannot test (auditd) ---
# Lima writes the instance's ssh config to ~/.lima/<name>/ssh.config, host
# alias lima-<name>; tests/inventory/vm.yml points ansible at it.
BW_VM="black-vm-05"   # also named in tests/inventory/vm.yml
BW_VM_TEMPLATE="${BW_VM_TEMPLATE:-template://rocky-9}"
BW_VM_INVENTORY="$BW_ROOT/tests/inventory/vm.yml"

bw_require_lima() {
  command -v limactl >/dev/null 2>&1 \
    || bw_die "limactl not found in PATH (brew install lima, see tests/README.md)"
}

# Prints Running, Stopped, or nothing when the instance does not exist.
bw_vm_status() {
  limactl list --format '{{.Status}}' "$BW_VM" 2>/dev/null || true
}

# bw_vm_ansible <module-dir> <playbook-relative-path> [extra args...]
bw_vm_ansible() {
  local module="$1" playbook="$2"
  shift 2
  ( cd "$BW_ROOT/$module" && ansible-playbook -i "$BW_VM_INVENTORY" "$playbook" "$@" )
}

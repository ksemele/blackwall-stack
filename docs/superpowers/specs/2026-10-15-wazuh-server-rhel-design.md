# Wazuh server stack on the RHEL family (lab sentinel) — design

Date: 2026-10-15
Module: blackwall-eye
Status: approved in chat

## 1. Goal

Sub-project B of "Eye on RHEL". The prod host (RHEL family, podman, nginx in a
container on 443, `nftables` + `cf-wall` roles) will later become the Wazuh
manager. Nothing is deployed to prod now. In the lab, a new node
`black-sentinel-03` (Rocky 9) reproduces that host and runs the full server
stack (indexer, manager + filebeat, dashboard) from the same roles, so that:

- the server roles work on the RedHat family;
- the stack coexists with podman, nginx on 443 and our firewall;
- all management endpoints are reachable only on `127.0.0.1`;
- we know its memory/CPU footprint (sizing for prod).

The existing Ubuntu stack on `black-master-00` and its agents are unchanged.

## 2. Design

### 2.1 Lab node and podman fixture

- Compose service `black-sentinel-03`, Rocky 9 image (same as `black-rocky-02`),
  `172.28.0.16`, SSH on host port 2204, no other published ports.
- Inventory group `sentinel` (not `wazuh_managers` — it is a separate stack,
  and every existing check keeps meaning "the main lab stack").
- Lab fixture playbook `tests/triggers/sentinel-podman.yml`: podman installed,
  an nginx container published on host port 443. If nested podman cannot run
  inside the lab container, the fallback (ruled in the plan's Task 1) is
  documented, and `nft_podman` is not set on the node.

### 2.2 Server roles on RedHat

- `wazuh-indexer`, `wazuh-manager` (incl. filebeat), `wazuh-dashboard`: dnf
  installs with `wazuh_pkg_suffix` and `disable_excludes: wazuh`; Debian path
  unchanged (apt + hold). Package names/versions that differ by family (e.g.
  filebeat release suffix) become family-aware variables.
- dpkg-lock waits only on Debian (already done in `wazuh-repo`; remove the
  copies in the server roles).

### 2.3 Bindings (variables, lab default unchanged)

| Endpoint | Variable | `black-master-00` | `black-sentinel-03` |
|---|---|---|---|
| indexer | `wazuh_indexer_network_host` | 172.28.0.10 | 127.0.0.1 |
| dashboard | `wazuh_dashboard_listen_host` / `_port` | 0.0.0.0 / 443 | 127.0.0.1 / 5601 |
| API | `wazuh_api_host` (new) | 0.0.0.0 | 127.0.0.1 |
| dashboard -> indexer / API | `wazuh_indexer_url`, `wazuh_manager_api_url` | 172.28.0.10 | 127.0.0.1 |
| agents enrollment/events | 1514/1515 | open | nftables: only `wazuh_agent_sources` |

### 2.4 Certificates per stack

`wazuh-certificates.yml` generates one bundle per server host instead of one
`run_once` bundle: node name = `inventory_hostname`, IP =
`wazuh_cert_ip` (default `ansible_host`-independent var; sentinel `127.0.0.1`).
`wazuh_certs_local_dir` stays `tests/.wazuh-certs` for `black-master-00` (no
regeneration of the running stack) and is `tests/.wazuh-certs/<host>` for the
sentinel.

### 2.5 Resources

- `wazuh_indexer_heap` (JVM `-Xms/-Xmx`), default = current behaviour,
  sentinel `1g`.
- After deploy, record RSS of indexer/manager/dashboard/filebeat and the
  container's total memory (`docker stats`) in `blackwall-eye/STATUS.md`.

### 2.6 Playbook and checks

- `ansible/playbooks/sentinel.yml`: certificates for the host, then
  `nftables` (+`nft_podman` when the fixture runs podman), indexer, manager,
  dashboard, `cf-wall`.
- `bw-lab provision sentinel` = fixture + `sentinel.yml`; opt-in (not in
  `all`).
- `verify-sentinel.yml` / `bw-check sentinel`: services active; indexer, API,
  dashboard answer on 127.0.0.1 and are NOT reachable on 172.28.0.16; agent
  000 active; nginx answers on 443; podman's tables survive
  `systemctl reload nftables`; Wazuh packages pinned.

## 3. Out of scope

Prod deployment; dashboard exposure beyond SSH tunnel; clustering; TUI context
for the sentinel; re-pointing lab agents to the sentinel.

# Test-env lab tooling — design

- **Date:** 2026-08-31
- **Status:** approved
- **Modules:** `tests/`, `blackwall-eye`, `blackwall-voice`

## Problem

Provide a reproducible way to bring the Blackwall test environment up and down
and check its state with a single command, instead of a pile of manual
`ansible`/`docker` calls from `tests/README.md`.

Three scenarios must be covered:

1. Start and tear down the test containers.
2. Connect to them and check: host reachability, package versions,
   applied nftables, open ports.
3. Bring up local Wazuh + Prometheus + Grafana and confirm that agents are
   registered and metrics are arriving.

## Decisions (made by the owner on 2026-08-31)

| Question | Decision |
|---|---|
| Shape of the "two binaries" | bash scripts in `tests/bin/` that call ansible playbooks |
| Prometheus / Grafana | plain lab compose services, not ansible roles |
| nftables | add a parametrised ansible role |
| Existing `tools/blackwall` (TUI) | left untouched |

## Topology after the change

Profile `ubuntu2204`, network `172.28.0.0/24`:

| Container | Role | IP | Host port |
|---|---|---|---|
| `black-master-00` | wazuh manager + indexer + dashboard | 172.28.0.10 | 2200 (ssh), 5601 (dashboard) |
| `black-host-01` | wazuh agent + auditd | 172.28.0.11 | 2201 (ssh) |
| `black-dead-98` | disconnected fixture | 172.28.0.98 | 2202 (ssh) |
| `black-metrics-00` | **new** — Prometheus | 172.28.0.12 | 9090 |
| `black-grafana-00` | **new** — Grafana | 172.28.0.13 | 3000 |

`black-metrics-00` and `black-grafana-00` are plain containers (not privileged,
no systemd), in the same `ubuntu2204` profile so that `--profile` stays the
single knob for selecting the environment.

## Components

### 1. Prometheus + Grafana compose services

Config files live in the repo and are mounted read-only — no manual steps
inside the containers.

```
tests/docker/
├── prometheus/
│   └── prometheus.yml
└── grafana/
    └── provisioning/
        ├── datasources/prometheus.yml
        └── dashboards/
            ├── dashboards.yml            # provider
            └── blackwall-hosts.json      # dashboard as code
```

`prometheus.yml` scrapes:

- job `blackwall-hosts` — `172.28.0.10:9100`, `172.28.0.11:9100`, `172.28.0.98:9100`
  (node_exporter), with `labels: {env: test}`
- job `prometheus` — itself (`localhost:9090`), so that `bw-check metrics`
  always has at least one target known to be alive

Grafana: anonymous access off, `admin / blackwall`, the Prometheus datasource
(`http://black-metrics-00:9090`) is set up by provisioning.

Images are pinned by version (config as code, reproducibility):
`prom/prometheus:v2.55.1`, `grafana/grafana:11.4.0` — the exact tags are
confirmed during implementation; if a tag is unavailable, the closest stable
one is taken and recorded in this file.

Wazuh does **not** expose Prometheus metrics natively; an exporter for it is out
of scope. "Metrics are arriving" is checked via node_exporter. Agent
registration is checked separately, via the Wazuh API / `agent_control`.

### 2. Ansible role `node-exporter` (blackwall-voice)

`blackwall-voice/ansible/roles/node-exporter/`

- installs `prometheus-node-exporter` from apt; if the package is unavailable on
  the chosen mirror — falls back to the GitHub tarball (`node_exporter_install_method: apt|tarball`)
- systemd unit enabled + started, listens on `:9100` on all interfaces
- variables: `node_exporter_version`, `node_exporter_listen_address`,
  `node_exporter_install_method`

Playbooks:

- `blackwall-voice/ansible/playbooks/metrics.yml` — rollout to the whole `all` group
- `blackwall-voice/ansible/playbooks/verify-metrics.yml` — service active,
  `curl localhost:9100/metrics` returns `node_` metrics

The role lives in Voice: Voice is the observability and dashboards layer, Eye is detection.

### 3. Ansible role `nftables` (blackwall-eye)

`blackwall-eye/ansible/roles/nftables/`

- installs the `nftables` package
- renders `/etc/nftables.d/blackwall-base.nft` from a template
- a systemd drop-in includes `/etc/nftables.d/*.nft`, the `nftables` service is enabled
- **before applying** runs `nft -c -f <file>` (dry run); on a syntax error the
  task fails and the ruleset is not applied

Variables:

| Variable | Default | Meaning |
|---|---|---|
| `nft_enabled` | `true` | disable the role on a host entirely |
| `nft_wan_if` | `eth0` | external interface |
| `nft_public_tcp` | `[22]` | TCP ports open to everyone |
| `nft_restricted` | `[]` | list of `{port, sources[]}` — port reachable only from these subnets |
| `nft_log_drops` | `true` | `limit rate 5/minute log prefix ...` on drops |

A non-optional skeleton in the template (otherwise the lab locks itself out):
`iif lo accept`, `ct state established,related accept`, `ct state invalid drop`,
all of ICMP/ICMPv6, `policy drop` only on `input`.

Lab rules (in `group_vars`):

| Group | `nft_public_tcp` |
|---|---|
| `wazuh_managers` | 22, 443, 1514, 1515, 55000, 9100 |
| `wazuh_agents` | 22, 9100 |

Playbooks:

- `blackwall-eye/ansible/playbooks/nftables.yml`
- `blackwall-eye/ansible/playbooks/verify-nftables.yml` — `nft list ruleset`,
  asserts that every port from `nft_public_tcp` is present and that `policy drop`
  is set on input

The prod configs `hosts/example.com/nftables/*.nft` are **not touched** by this
work — they are specific to podman/DNAT/Cloudflare. Porting them into the role
is a separate task, recorded as a follow-up in `blackwall-eye/STATUS.md`.

**Known risk.** nftables inside Docker requires the VM kernel (OrbStack) to
provide `nf_tables`, and iptables-legacy/nft must not conflict in the image.
This is checked with a spike at the start of implementation. If it doesn't
work, the role stays (prod hosts need it), and `bw-check nft` reports `SKIP`
with an explanation instead of `FAIL` in the lab.

### 4. `tests/bin/bw-lab` — environment lifecycle

A thin bash wrapper over `docker compose` and the ansible playbooks. All paths
are resolved from the repo root, so the script can be called from any directory.

```
bw-lab up [--profile <p>] [--no-provision]   compose up --build → wait for SSH → provision
bw-lab down [--keep-data]                    compose down (-v unless --keep-data)
bw-lab reset                                 down -v + wipe tests/.wazuh-certs + up
bw-lab status                                containers, published ports, Prometheus/Grafana health
bw-lab provision <target>                    ansible only: eye|voice|nftables|metrics|all
bw-lab ssh <host> [cmd...]                   ssh -F tests/docker/ssh_config
bw-lab logs <service> [-f]                   docker compose logs
bw-lab open <dashboard|grafana|prometheus>   open the URL in a browser
```

`bw-lab up` sequence:

1. `docker compose --profile <p> up -d --build`
2. `ansible all -m wait_for_connection -T 60` (systemd nodes only)
3. `blackwall-eye`: `all.yml` (certificates → security-node → monitored-host)
4. `blackwall-eye`: `nftables.yml`
5. `blackwall-voice`: `metrics.yml`

`--no-provision` stops after step 1 — for when you want to run provisioning by
hand, piece by piece.

The default profile is `ubuntu2204`; override with `--profile` or the
`BW_PROFILE` environment variable.

### 5. `tests/bin/bw-check` — checks

```
bw-check all                 every check in a row
bw-check hosts               docker ps + ansible ping/wait_for_connection
bw-check versions            package versions on all hosts
bw-check nft                 applied ruleset + assert expected ports
bw-check ports               ss -tlnp inside containers + published ports from the host
bw-check wazuh               agent registration + REST API
bw-check metrics             Prometheus targets + query; Grafana health + datasource
bw-check voice               telegram-alerter
```

Split of responsibilities:

- **facts about hosts** (versions, nft, ports) — new ansible playbooks with `assert`,
  the script only calls them and passes the exit code through
- **HTTP endpoints** (Prometheus, Grafana, Wazuh dashboard) — `curl` straight
  from the host, no ansible needed

Packages checked by `versions`: `wazuh-manager`, `wazuh-indexer`,
`wazuh-dashboard`, `wazuh-agent`, `filebeat`, `auditd`, `nftables`,
`prometheus-node-exporter`. A package missing on a host where it isn't expected
is not an error; expectations are set per inventory group.

`metrics` checks:

- `GET /api/v1/targets` — all targets of job `blackwall-hosts` are `up`
- `GET /api/v1/query?query=node_load1` — non-empty `result`
- Grafana `GET /api/health` → `"database": "ok"`
- Grafana `GET /api/datasources` → a `Prometheus` datasource exists

Output: one line per check prefixed `PASS` / `FAIL` / `SKIP`, a final summary,
non-zero exit code on any `FAIL` — usable in CI.

### 6. New verify playbooks

| File | Checks |
|---|---|
| `blackwall-eye/ansible/playbooks/verify-versions.yml` | `dpkg-query` over the package list, assert per group |
| `blackwall-eye/ansible/playbooks/verify-nftables.yml` | `nft list ruleset`, assert ports and policy |
| `blackwall-eye/ansible/playbooks/verify-ports.yml` | `ss -tlnp`, assert expected listeners |
| `blackwall-voice/ansible/playbooks/verify-metrics.yml` | node_exporter active + serves metrics |

The existing `verify-agents.yml`, `verify-api.yml`, `verify-auditd.yml`,
`verify-voice.yml` are reused as-is.

## Documentation

- `tests/README.md` — restructured: `bw-lab`/`bw-check` scenarios on top, the
  existing detailed cookbook of manual commands stays below as a
  "manual mode / debugging" section (useful when diagnosing breakage)
- `blackwall-eye/README.md` — the `nftables` role, new verify playbooks
- `blackwall-voice/README.md` — the `node-exporter` role, Prometheus/Grafana in the lab
- root `README.md` — mention of `tests/bin/`
- `blackwall-eye/STATUS.md`, `blackwall-voice/STATUS.md` — state and follow-ups

## Explicitly out of scope

- A Wazuh metrics exporter for Prometheus
- Porting `hosts/example.com/nftables/*.nft` into the new role
- `debian12` / `centos9` profiles
- Grafana alerts and Alertmanager
- Unit tests for the bash scripts (bats) — verified by a real lab run instead

## Definition of done

From scratch, on a clean machine with Docker running:

```bash
tests/bin/bw-lab reset
tests/bin/bw-check all
```

must finish with a zero exit code and all `PASS` (a `SKIP` for `nft` is
acceptable if the container kernel lacks nf_tables), and
`http://localhost:3000` must show a dashboard with live metrics from three hosts.

# Blackwall

> *"The Blackwall is a military-grade AI construct — an autonomous guardian that adapts,
> learns, and strikes back."*

Blackwall is an open-source modular security platform for Linux infrastructure.
It combines battle-tested open-source components into a layered defensive system,
deployable via Infrastructure-as-Code with no vendor lock-in.

## Modules

| Module | Purpose | Stage |
|---|---|---|
| [blackwall-eye](./blackwall-eye/) | Host monitoring core | 1 — Active |
| [blackwall-voice](./blackwall-voice/) | Alerts & dashboards | 2 — Active |
| blackwall-brain | AI analysis agents | 3 — In progress |
| blackwall-trap | Honeypots | 4 — Planned |
| blackwall-fang | Active countermeasures (research) | 5 — Planned |
| [blackwall-scan](./blackwall-scan/) | External attack surface scanning (CESM)¹ | 1 — Active |

¹ Separate track from the defense chain (eye→voice→brain→trap→fang); see [blackwall-scan/STATUS.md](./blackwall-scan/STATUS.md).

## Getting Started

Each module is independently deployable. A shared Docker test environment lives in `tests/`:

| Container | Role |
|---|---|
| `black-master-00` | Wazuh manager + indexer + dashboard |
| `black-host-01` | Wazuh agent (target for other module roles) |
| `black-dead-98` | agent fixture, stopped on purpose |
| `black-rocky-02` | Rocky Linux 9, firewall roles on the RHEL family |
| `black-metrics-00` | Prometheus |
| `black-grafana-00` | Grafana |
| `black-aptcache-00` | apt cache (lab only): Wazuh packages are downloaded once |

### 1. Start the test environment

Two scripts drive it — [`tests/bin/bw-lab`](./tests/bin/bw-lab) for lifecycle and
[`tests/bin/bw-check`](./tests/bin/bw-check) for verification:

```bash
# From anywhere in the repo
tests/bin/bw-lab up        # containers + Wazuh + node_exporter + nftables
tests/bin/bw-check all     # hosts, packages, firewall, ports, agents, metrics
```

| Service | URL | Credentials |
|---|---|---|
| Wazuh dashboard | https://localhost:5601 | `admin` / `Blackwall1!` |
| Grafana | http://localhost:3000 | `admin` / `blackwall` |
| Prometheus | http://localhost:9090 | none |
| Wazuh API (`blackwall --context lab`) | https://127.0.0.1:55000 | `wazuh` / `wazuh` |
| Wazuh indexer (blackwall :alerts) | https://127.0.0.1:9200 | `admin` / `Blackwall1!` |

The nodes are containers: kernel audit is unavailable, so auditd rules are
deployed but never fire in the lab (see [container limitations](./tests/README.md#container-limitations)).

Certificates are self-signed; accept the browser warning. Tear down with
`tests/bin/bw-lab down`, or start from scratch with `tests/bin/bw-lab reset`.

### 2. Watch it in the TUI

[`tools/blackwall`](./tools/blackwall/) is a k9s-style terminal UI: agents,
the `:alerts` feed and per-agent Sessions.

```bash
cd tools/blackwall && go install ./cmd/blackwall   # binary lands in ~/go/bin
blackwall                          # first run writes ~/.config/blackwall/config.yaml (with a lab context) and exits
blackwall --context lab --once     # agents table, then exit
blackwall --context lab            # interactive TUI
```

See [tools/blackwall/README.md](./tools/blackwall/README.md) for config and keys.

### 3. Point agents at a real Wazuh master

Write your own inventory (`inventory/prod.yml`) with the target hosts and run:

```bash
ansible-playbook -i inventory/prod.yml ansible/playbooks/monitored-host.yml \
  -e wazuh_manager_host=wazuh.prod.example.com \
  -e wazuh_enrollment_password=<secret>
```

See [tests/README.md](./tests/README.md) for the full test-environment docs, verification cookbook, and override mechanics.
See each module's `README.md` for module-specific deploy and verify commands.

## Development

This project is developed using Claude Code with a skill orchestrator.
Run `/blackwall` (Claude Code slash command) in Claude Code to see module status and continue development.

## License

See [LICENSE](./LICENSE).

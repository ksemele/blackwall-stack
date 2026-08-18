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

Each module is independently deployable. A shared 2-container Docker test environment lives in `tests/`:

| Container | Role |
|---|---|
| `black-master-00` | Wazuh manager + indexer + dashboard |
| `black-host-01`   | Wazuh agent (target for other module roles) |

### 1. Start the test environment

```bash
# From repo root
docker compose -f tests/docker/docker-compose.yml --profile ubuntu2204 up -d --build
cd blackwall-eye
ansible all -m wait_for_connection -T 60
ansible-playbook ansible/playbooks/all.yml
```

Then open the dashboard: **https://localhost:5601** — login `admin / Blackwall1!` (self-signed cert; accept the browser warning).

Tear down:

```bash
docker compose -f tests/docker/docker-compose.yml --profile ubuntu2204 down -v
```

### 2. Point agents at a real Wazuh master

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

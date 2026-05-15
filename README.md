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
| blackwall-brain | AI analysis agents | 3 — Planned |
| blackwall-trap | Honeypots | 4 — Planned |
| blackwall-fang | Active countermeasures (research) | 5 — Planned |

## Getting Started

Each module is independently deployable. Shared Docker test environment lives in `tests/`.

### 1. Start the test environment

Run from repo root:

```bash
docker compose -f tests/docker/docker-compose.yml --profile ubuntu2204 up -d --build
```

### 2. Deploy a module

```bash
cd blackwall-eye
ansible all -m wait_for_connection   # wait for SSH
ansible-playbook ansible/playbooks/all.yml
```

See [tests/README.md](./tests/README.md) for full test environment docs.
See each module's `README.md` for module-specific deploy and verify commands.

## Development

This project is developed using Claude Code with a skill orchestrator.
Run `/blackwall` (Claude Code slash command) in Claude Code to see module status and continue development.

## License

See [LICENSE](./LICENSE).

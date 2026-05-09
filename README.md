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
| blackwall-voice | Alerts & dashboards | 2 — Planned |
| blackwall-brain | AI analysis agents | 3 — Planned |
| blackwall-trap | Honeypots | 4 — Planned |
| blackwall-fang | Active countermeasures (research) | 5 — Planned |

## Getting Started

Each module is independently deployable. Start with `blackwall-eye`:

```bash
cd blackwall-eye
# See blackwall-eye/docs/ARCHITECTURE.md for deployment guide
```

## Development

This project is developed using Claude Code with a skill orchestrator.
Run `/blackwall` (Claude Code slash command) in Claude Code to see module status and continue development.

## License

See [blackwall-eye/LICENSE](./blackwall-eye/LICENSE).

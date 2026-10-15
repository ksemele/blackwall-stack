# Blackwall

Blackwall is a modular security platform — a suite of "combat AIs" providing layered,
autonomous defense of systems. Inspired by the Blackwall AI from Cyberpunk 2077.

## Vision

Not just monitoring: perceive → alert → analyze → deceive → strike back.
Each module is an independent, deployable unit with a single security responsibility.

## Modules

| Directory | Codename | Purpose | Stage |
|---|---|---|---|
| `blackwall-eye/` | Eye | Host monitoring core (Wazuh, auditd, Suricata, Zeek) | 1 |
| `blackwall-voice/` | Voice | Alerts, dashboards, notification layer | 2 |
| `blackwall-brain/` | Brain | AI agents for automated log/event analysis | 3 |
| `blackwall-trap/` | Trap | Honeypots and canary infrastructure | 4 |
| `blackwall-fang/` | Fang | Active countermeasures: automatic blocking (production); counter-attack (research only) | 5 |
| `blackwall-scan/` | Scan | External attack surface self-assessment (CESM) — outside-in recon of own perimeter | 1* |

\* separate track (perimeter recon), not part of the defense chain.

## Roadmap

| Stage | Module | Delivers |
|---|---|---|
| 1 | Eye | Wazuh + auditd on 1–3 hosts, first alerts firing |
| 2 | Voice | Standardized alerts, dashboards, Telegram/Slack/webhook |
| 3 | Brain | AI agents for anomaly explanation and threat classification |
| 4 | Trap | Decoy services, fake credentials, canary tokens |
| 5 | Fang | Counterattack research tooling |

## Agent Rules

- **Before touching any module:** read its `STATUS.md` to understand current state. If `STATUS.md` doesn't exist, the module hasn't been started — create it with `status: pending` before doing anything else.
- **Naming:** keep the Blackwall/Cyberpunk thematic naming (eye, voice, brain, trap, fang).
- **Isolation:** each module must remain independently deployable.
- **Config as code:** no manual steps that can't be reproduced from version-controlled files.
- **Signal over noise:** high-confidence detections before broad coverage.
- **Keep docs in sync:** when adding or changing playbooks, roles, inventory files, CLI commands, or tooling — update the relevant README and any scripts that reference those commands in the same commit. Stale docs are bugs.

## Orchestrator

Run `/blackwall` (Claude Code slash command) to see module status and dispatch development work.

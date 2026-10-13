# Blackwall Orchestrator Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Create the project scaffold: root CLAUDE.md/README.md, the `/blackwall` orchestrator skill, and the `blackwall-eye` module context + initial state file.

**Architecture:** Flat skill orchestrator reads `STATUS.md` from each module directory, shows a status summary, waits for user input, then dispatches a subagent with the module's CLAUDE.md and STATUS.md as context. Subagents write state; orchestrator only reads.

**Tech Stack:** Markdown, YAML front-matter, Claude Code slash commands (`.claude/commands/`).

---

### Task 1: Root `CLAUDE.md`

**Files:**
- Create: `CLAUDE.md`

- [ ] **Step 1: Create the file**

```markdown
# Blackwall

Blackwall is a modular security platform — a suite of "combat AIs" providing layered,
autonomous defense of systems. Inspired by the Blackwall AI from Cyberpunk 2077.

## Vision

Not just monitoring: perceive → alert → analyze → deceive → strike back.
Each module is an independent, deployable unit with a single security responsibility.

## Modules

| Directory | Codename | Purpose | Status |
|---|---|---|---|
| `blackwall-eye/` | Eye | Host monitoring core (Wazuh, auditd, Suricata, Zeek) | Stage 1 |
| `blackwall-voice/` | Voice | Alerts, dashboards, notification layer | Planned |
| `blackwall-brain/` | Brain | AI agents for automated log/event analysis | Planned |
| `blackwall-trap/` | Trap | Honeypots and canary infrastructure | Planned |
| `blackwall-fang/` | Fang | Active countermeasures (research context only) | Planned |

## Roadmap

| Stage | Module | Delivers |
|---|---|---|
| 1 | Eye | Wazuh + auditd on 1–3 hosts, first alerts firing |
| 2 | Voice | Standardized alerts, dashboards, Telegram/Slack/webhook |
| 3 | Brain | AI agents for anomaly explanation and threat classification |
| 4 | Trap | Decoy services, fake credentials, canary tokens |
| 5 | Fang | Counterattack research tooling |

## Agent Rules

- **Before touching any module:** read its `STATUS.md` to understand current state.
- **Naming:** keep the Blackwall/Cyberpunk thematic naming (eye, voice, brain, trap, fang).
- **Isolation:** each module must remain independently deployable.
- **Config as code:** no manual steps that can't be reproduced from version-controlled files.
- **Signal over noise:** high-confidence detections before broad coverage.

## Orchestrator

Run `/blackwall` to see module status and dispatch development work.
```

- [ ] **Step 2: Verify required sections exist**

Open `CLAUDE.md` and confirm these headings are present:
- `## Modules` (table with all 5 modules)
- `## Roadmap`
- `## Agent Rules` (includes "read STATUS.md" rule)
- `## Orchestrator`

- [ ] **Step 3: Commit**

```bash
git add CLAUDE.md
git commit -m "docs: add root CLAUDE.md with project vision and agent rules"
```

---

### Task 2: Root `README.md`

**Files:**
- Create: `README.md`

- [ ] **Step 1: Create the file**

```markdown
# Blackwall

> *"The Blackwall is a military-grade AI construct — an autonomous guardian that adapts,
> learns, and strikes back."*

Blackwall is an open-source modular security platform for Linux infrastructure.
It combines battle-tested open-source components into a layered defensive system,
deployable via Infrastructure-as-Code with no vendor lock-in.

## Modules

| Module | Purpose | Stage |
|---|---|---|
| [blackwall-eye](./blackwall-eye/) | Host monitoring core | Active |
| blackwall-voice | Alerts & dashboards | Planned |
| blackwall-brain | AI analysis agents | Planned |
| blackwall-trap | Honeypots | Planned |
| blackwall-fang | Active countermeasures (research) | Planned |

## Getting Started

Each module is independently deployable. Start with `blackwall-eye`:

```bash
cd blackwall-eye
# See blackwall-eye/docs/ARCHITECTURE.md for deployment guide
```

## Development

This project is developed using Claude Code with a skill orchestrator.
Run `/blackwall` in Claude Code to see module status and continue development.

## License

See [blackwall-eye/LICENSE](./blackwall-eye/LICENSE).
```

- [ ] **Step 2: Verify**

Open `README.md` and confirm:
- Module table present with all 5 entries
- Getting started section points to `blackwall-eye`
- `/blackwall` orchestrator is mentioned

- [ ] **Step 3: Commit**

```bash
git add README.md
git commit -m "docs: add root README with module overview and getting started"
```

---

### Task 3: Orchestrator skill `.claude/commands/blackwall.md`

**Files:**
- Create: `.claude/commands/blackwall.md`

- [ ] **Step 1: Create the commands directory**

```bash
mkdir -p .claude/commands
```

- [ ] **Step 2: Create the skill file**

```markdown
# /blackwall — Blackwall Orchestrator

You are the Blackwall orchestrator. Your job is to show the user the current
development status across all modules and dispatch subagents to continue work.

## Step 1: Discover modules

Scan the root directory for subdirectories that contain a `STATUS.md` file.
Each such directory is a Blackwall module. Read every `STATUS.md` found.

## Step 2: Display status summary

Parse the YAML front-matter from each `STATUS.md` and render this table:

```
BLACKWALL — статус модулей
──────────────────────────
[<status>] <module>    Stage <stage> | <next_task>
...
```

Status badge format:
- `pending`     → `[·····]`
- `in_progress` → `[>>>>·]`
- `blocked`     → `[BLOCK]`
- `done`        → `[DONE·]`

Example:
```
BLACKWALL — статус модулей
──────────────────────────
[>>>>·] blackwall-eye    Stage 1 | Следующее: Ansible роль wazuh-agent
[·····] blackwall-voice  Не начат
```

## Step 3: Present options and wait

After the status table, show numbered options and wait for the user's choice.
Do NOT auto-select. Do NOT proceed without user input.

```
Что делаем?
  1. Продолжить <in_progress-module> — <next_task>
  2. Начать <pending-module>
  3. Показать детали модуля [имя]
  4. Ничего, только статус
```

Only show option 1 for modules with status `in_progress`.
Only show option 2 for modules with status `pending`.
If multiple modules are `in_progress`, list each as a separate option.

## Step 4: Dispatch subagent (if action chosen)

When the user selects an action (not option 4), spawn a subagent using the Agent tool.

Pass to the subagent:
- Full content of `<module>/CLAUDE.md`
- Full content of `<module>/STATUS.md`
- The task: content of `next_task` field from STATUS.md, or the user's custom instruction

Include this instruction to the subagent:

> You are a Blackwall module subagent working on <module>.
> The project context is in the CLAUDE.md above.
> The current module state is in STATUS.md above.
> Your task: <task>
>
> When done:
> 1. Update `<module>/STATUS.md`: set `last_task` to what you completed,
>    set `next_task` to what should happen next, update `last_updated`,
>    update `status` if appropriate.
> 2. Return a brief report (3-5 lines):
>    ✓ Выполнено: <what was done>
>    → Следующее: <next_task>
>    ⚠ Вопросы: <open questions, if any>

## Step 5: Show report and return to idle

Display the subagent's report to the user. Do not auto-dispatch another subagent.
The user decides what happens next.
```

- [ ] **Step 3: Verify the skill file contains all required sections**

Open `.claude/commands/blackwall.md` and confirm:
- Step 1: module discovery by scanning for `STATUS.md`
- Step 2: status table with badge format defined
- Step 3: option list with explicit "wait for user" instruction
- Step 4: subagent dispatch with full instruction template
- Step 5: show report and stop

- [ ] **Step 4: Commit**

```bash
git add .claude/commands/blackwall.md
git commit -m "feat: add /blackwall orchestrator skill"
```

---

### Task 4: `blackwall-eye/CLAUDE.md`

**Files:**
- Create: `blackwall-eye/CLAUDE.md`

- [ ] **Step 1: Create the file**

```markdown
# blackwall-eye — Module Context

**Codename:** Eye
**Purpose:** Host monitoring core. First line of perception.
**Scope:** Collects, normalizes, and correlates host-level security events.
Does NOT handle alerting UI, AI analysis, honeypots, or active response —
those belong to later modules.

## Stack

| Component | Role |
|---|---|
| Wazuh Manager | Central event collector, rule engine, HIDS, FIM, alert hub |
| Wazuh Agent | Per-host agent: log collection, FIM, auditd integration |
| auditd | Kernel-level syscall audit (execve, sudo, file changes) |
| Suricata | Network IDS/IPS, signature-based detection |
| Zeek | Network telemetry: structured logs per protocol |
| OpenSearch | Long-term storage, anomaly detection (Stage 4) |

Deployment: Ansible IaC. No manual UI steps.
Target OS: Ubuntu 22.04 LTS / Debian 12 (primary), Rocky Linux 9 (secondary).

## Stage Breakdown

| Stage | Delivers |
|---|---|
| 1 | Wazuh manager + agent + auditd on 1–3 hosts. First alerts firing. |
| 2 | Enriched detections: GeoIP, time-based rules, correlation. |
| 3 | Suricata + Zeek network layer, host+network correlation. |
| 4 | OpenSearch Anomaly Detection baselines. |

## Design Principles

- **Signal over noise.** High-confidence detections before broad coverage.
- **Config as code.** Every component deployed via Ansible. No manual steps.
- **Minimal footprint.** Agents must not become a resource problem on monitored hosts.
- **Independent stages.** Each stage is deployable and testable on its own.

## Repository Layout (planned)

```
blackwall-eye/
├── ansible/
│   ├── roles/
│   │   ├── wazuh-manager/
│   │   ├── wazuh-agent/
│   │   ├── suricata/
│   │   └── zeek/
│   ├── playbooks/
│   │   ├── site.yml
│   │   ├── security-node.yml
│   │   └── monitored-host.yml
│   └── inventory/
│       └── example.yml
├── rules/
│   ├── wazuh/
│   └── suricata/
├── config/
│   ├── auditd/
│   ├── wazuh/
│   └── zeek/
├── docs/
│   ├── ARCHITECTURE.md
│   └── stages/
└── tests/
    └── molecule/
```

## Agent Rules

- Read `STATUS.md` before starting any work to understand current progress.
- Update `STATUS.md` after completing each task.
- Follow Ansible role structure — one role per component.
- Test Ansible roles with Molecule before marking a task complete.
- Commit after each working increment.
```

- [ ] **Step 2: Verify**

Open `blackwall-eye/CLAUDE.md` and confirm:
- Stack table with all 6 components
- Stage breakdown table (4 stages)
- Repository layout matches the spec
- Agent rules include "read STATUS.md" and "update STATUS.md"

- [ ] **Step 3: Commit**

```bash
git add blackwall-eye/CLAUDE.md
git commit -m "docs: add blackwall-eye module CLAUDE.md for subagent context"
```

---

### Task 5: `blackwall-eye/STATUS.md` (initial state)

**Files:**
- Create: `blackwall-eye/STATUS.md`

- [ ] **Step 1: Create the file**

```markdown
---
module: blackwall-eye
status: in_progress
stage: 1
last_updated: 2026-05-09
last_task: "Создан ARCHITECTURE.md и базовая структура документации"
next_task: "Создать Ansible роль wazuh-manager: установка пакетов, базовый ossec.conf, systemd unit"
blockers: []
---

## Что сделано

- `docs/ARCHITECTURE.md` — полная архитектура, компоненты, detection coverage, deployment model
- Структура репозитория спроектирована (см. ARCHITECTURE.md → Repository Layout)

## Открытые вопросы

- Версия Wazuh: зафиксировать на 4.x (current stable) или конкретный minor?
- Molecule driver: docker или vagrant для тестов Ansible-ролей?

## Решения

- Target OS: Ubuntu 22.04 LTS / Debian 12 (primary), Rocky Linux 9 (secondary)
- Deployment: Ansible only, без Terraform на первом этапе
- Alerting: Telegram/Slack/webhook (реализуется в blackwall-voice, не здесь)
```

- [ ] **Step 2: Verify YAML front-matter is valid**

Check that the front-matter block:
- Opens and closes with `---`
- Contains all required fields: `module`, `status`, `stage`, `last_updated`, `last_task`, `next_task`, `blockers`
- `status` value is one of: `pending`, `in_progress`, `blocked`, `done`

- [ ] **Step 3: Commit**

```bash
git add blackwall-eye/STATUS.md
git commit -m "feat: add blackwall-eye initial STATUS.md"
```

---

### Task 6: Smoke test — run `/blackwall`

- [ ] **Step 1: Open Claude Code in the repo root**

```bash
cd /path/to/blackwall
# open Claude Code CLI
```

- [ ] **Step 2: Run the orchestrator**

Type `/blackwall` and verify the output:
- Status table appears with `blackwall-eye` shown as `[>>>>·]` (in_progress)
- `next_task` shown correctly: "Создать Ansible роль wazuh-manager..."
- Options list appears with option 1 to continue `blackwall-eye`
- Orchestrator waits — does NOT auto-proceed

- [ ] **Step 3: Select option 4 (status only)**

Verify: no subagent is dispatched, conversation ends cleanly.

- [ ] **Step 4: Final commit**

```bash
git add docs/superpowers/
git commit -m "docs: add orchestrator design spec and implementation plan"
```

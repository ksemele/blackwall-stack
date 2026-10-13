# Blackwall Orchestrator Design

**Date:** 2026-05-09
**Status:** Approved
**Scope:** Root README/CLAUDE.md files, skill orchestrator, subagent contract, module state format

---

## Context

Blackwall is a multi-module security platform. The root repo contains independent modules
(`blackwall-eye`, `blackwall-voice`, etc.), each developed in stages. Development is driven
by Claude Code agents. This spec defines how agents orient themselves in the project and how
development work is coordinated across modules.

---

## Architecture

### File Structure

```
blackwall/
├── CLAUDE.md                       ← project-wide agent context
├── README.md                       ← human-readable project overview
├── .claude/
│   └── commands/
│       └── blackwall.md            ← orchestrator skill (/blackwall)
├── blackwall-eye/
│   ├── CLAUDE.md                   ← module-level agent context
│   ├── STATUS.md                   ← module state file
│   └── docs/
│       └── ARCHITECTURE.md
└── blackwall-voice/                ← (future module, same structure)
    ├── CLAUDE.md
    └── STATUS.md
```

### Components

**Root `CLAUDE.md`** — loaded into every agent's context automatically. Contains:
- Project vision and thematic naming conventions
- 5-stage roadmap summary
- Module directory index
- Instructions: always read module `STATUS.md` before touching a module

**Module `CLAUDE.md`** — loaded when an agent works inside a module directory. Contains:
- Module purpose and scope
- Stack/components specific to this module
- Design principles relevant to this module
- Pointer to `STATUS.md` as source of truth for current progress

**`/blackwall` skill** — orchestrator, invoked via `/blackwall` in Claude Code. Flat skill
(single file). Handles status display and subagent dispatch.

**`STATUS.md`** — state file per module. Written by subagents, read by orchestrator.
Subagents always update it after completing a task. Orchestrator never writes to it.

---

## Module State Format (`STATUS.md`)

YAML front-matter for machine-readable fields, markdown body for human/agent-readable detail.

```markdown
---
module: blackwall-eye
status: in_progress          # pending | in_progress | blocked | done
stage: 1
last_updated: 2026-05-09
last_task: "Description of the last completed task"
next_task: "Description of the next task"
blockers: []
---

## Done

- ...

## Open questions

- ...

## Decisions

- ...
```

**Status values:**
- `pending` — module not started
- `in_progress` — active development
- `blocked` — cannot proceed, see `blockers` field
- `done` — module complete for current stage

---

## Orchestrator Behavior (`/blackwall`)

### Step 1: Collect status

Read `STATUS.md` from every module directory. Render summary:

```
BLACKWALL — module status
──────────────────────────
[in_progress] blackwall-eye    Stage 1 | Next: wazuh-agent Ansible role
[pending]     blackwall-voice  Not started
```

Discovery: scan root directory for subdirectories containing `STATUS.md`.

### Step 2: Present options and wait

```
What next?
  1. Continue <module> (next task: <next_task>)
  2. Start <pending-module>
  3. Show module details [name]
  4. Nothing, status only
```

Orchestrator does NOT auto-select. Always waits for user input.

### Step 3: Dispatch subagent

When user selects an action, spawn Agent with:
- Full content of `<module>/CLAUDE.md`
- Full content of `<module>/STATUS.md`
- Explicit task description (from `next_task` or user input)
- Instruction: update `STATUS.md` on completion and return a brief report

### Subagent report format

3-5 lines returned to orchestrator:
```
✓ Done: <what was done>
→ Next: <next_task>
⚠ Questions: <if any>
```

Orchestrator displays the report to user and returns to idle.

---

## CLAUDE.md Content Guidelines

**Root `CLAUDE.md`:**
- Vision: "combat AI" security platform, Cyberpunk Blackwall theme
- Module index with one-line purpose per module
- 5-stage roadmap
- Rule: always check `STATUS.md` before starting work on a module

**Module `CLAUDE.md`:**
- Module name, codename, purpose
- Scope: what this module does and does NOT do
- Stack and key components
- Stage breakdown (what stage N delivers)
- Development principles specific to this module

---

## Design Decisions

**Orchestrator does not write state.** Only subagents write `STATUS.md`. This prevents
conflicts and keeps state ownership clear.

**Discovery by convention.** Orchestrator finds modules by scanning for directories
containing `STATUS.md`. Adding a new module requires only creating the directory and
an initial `STATUS.md` with `status: pending`.

**Flat skill for now.** Single orchestrator skill handles all modules. When modules grow
to 3+, consider migrating to per-module skills (each module gets its own skill with
module-specific commands). Tracked as TODO.

---

## Open Questions

None — scope is fully defined.

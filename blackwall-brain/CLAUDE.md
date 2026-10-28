# blackwall-brain — Brain

**Role:** AI analysis layer.
Brain receives Wazuh alerts that pass the severity threshold, asks a model (Claude by default) to classify
and explain them, then forwards enriched analysis to Telegram.
It does NOT route alerts to humans (that is Voice) and does NOT perform detection (that is Eye).

---

## Purpose

Eye sees. Voice speaks. Brain thinks. When a high-severity alert fires, Brain interrogates it:
What kind of attack is this? How confident are we? What should the operator do next?
One model call per alert — a validated structured verdict, actionable signal.

---

## Brain v2 (current)

Design: `../docs/superpowers/specs/2026-10-28-brain-v2-eval-design.md`.
Step 1 (this code): verdict, model interface, message, container, intake.
Step 2: eval set and harness. Step 3: context sources (mode 1).

- Package `src/brain/`: `verdict` (schema, parsing), `models/` (interface +
  `anthropic_model`, `openai_compat` adapters), `analysis` (prompt,
  `PROMPT_VERSION`, retries), `message` (Telegram HTML), `indexer` + `state`
  (polling intake), `telegram`, `config` (BRAIN_* env), `main` (loop).
- Container built on the target by role `brain-analyst` (Containerfile,
  hash-pinned `requirements.lock`), run by `brain-analyst.service`
  (`podman run --network host`), config in `/etc/brain-analyst/brain.env`,
  state in `/var/lib/brain-analyst/state.json`, logs in journald.
- Intake polls `wazuh-alerts-*` in the indexer; no file tailing.

## Design Principles

- **No vendor lock-in:** only `models/anthropic_model.py` imports `anthropic`
  and only `models/openai_compat.py` imports `openai` (a test enforces it).
- **Read-only:** indexer user can only search `wazuh-alerts-*`.
- **Validate, don't trust:** every model answer is checked against the verdict
  schema; invalid → one retry → fallback message.
- **Config as code:** all settings are Ansible vars rendered into `brain.env`.
- **Fail loud:** model failure → fallback message with the reason; Telegram
  failure → retried, then logged at ERROR as dropped.
- **Signal over noise:** `analysis_threshold` default 7 (higher than Voice's 5).

## Repository Layout

```
blackwall-brain/
├── CLAUDE.md, README.md, STATUS.md
├── pyproject.toml, requirements.lock, Containerfile, .containerignore
├── src/brain/                 # the package (see above)
├── tests/                     # pytest, no network
└── ansible/
    ├── ansible.cfg
    ├── playbooks/brain.yml, verify-brain.yml
    └── roles/brain-analyst/   # podman, image build, indexer RO user, env, unit
```

Key variables: see README.md (Configure).

## Inventory

Brain shares the inventory with blackwall-eye and blackwall-voice. The `ansible.cfg` points to:

```
inventory = ../tests/inventory/hosts.yml
```

Do NOT duplicate inventory files. Brain deploys to `wazuh_managers` group (same as Voice: Voice checks Brain's unit on its own host).

---

## Agent Rules

1. **Read STATUS.md before starting any work.**
2. **Update STATUS.md after completing each task** — `last_task`, `next_task`, `last_updated`.
3. **TDD:** `.venv/bin/pytest` in this directory; lab checks via `tests/bin/bw-check brain`.
4. **Keep Brain independently deployable** — no imports or hard path dependencies on other modules.
5. **Secrets** never in the repo; lab values come from `.envrc`.
6. **Keep docs in sync** — if playbooks, roles, env vars or CLI commands change, update README.md in the same commit.
7. **Never drop an alert silently** — model failure sends the fallback; a Telegram failure after retries is logged at ERROR. An alert is marked processed after handling (the cursor would skip it anyway).
8. **Bump `PROMPT_VERSION`** in `analysis.py` whenever the prompt or the verdict schema changes.

# blackwall-brain — Brain

AI analysis layer for the Blackwall security platform. Polls new Wazuh alerts
from the indexer, asks a model for a structured verdict and sends a short,
readable message to Telegram: severity, host, verdict, one line on what
happened, what to do, and links.

Design: [Brain v2 spec](../docs/superpowers/specs/2026-10-28-brain-v2-eval-design.md).

## How it works

- A Python package (`src/brain/`) in its own container, run on the host by a
  systemd unit (`podman run`). The `brain-analyst` role builds the image on
  the target from this directory.
- Intake: every `poll_interval` seconds it queries `wazuh-alerts-*` in the
  Wazuh indexer for alerts with `rule.level >= analysis_threshold`, newer than
  a cursor, with a short overlap for late-indexed alerts and dedup by indexer
  `_id`. After downtime it catches up at most `brain_max_backfill` seconds.
  Read-only indexer user; it does not need to run on the manager.
- Verdict: one JSON object per alert, validated (`verdict`, `severity`,
  `category`, `summary`, `action`, `confidence`, `needs`). An invalid answer is
  retried once; a failed call three times; then the fallback message.
- Models: no vendor lock-in. `anthropic` (Claude, default) or `openai_compat`
  (Ollama, vLLM, llama.cpp server, LM Studio, OpenRouter, ...). Only the two
  adapters in `src/brain/models/` know a vendor's format.

## Message

```
🔴 HIGH · black-host-01 · true positive
┌─────────────────────────────────────────────────────────────── copy ┐
│ New sudoers drop-in outside config management                       │
│ Category: privilege_escalation · confidence: medium                 │
│ Action: Check who wrote /etc/sudoers.d/90-app and revert it if unknown│
└─────────────────────────────────────────────────────────────────────┘
Rule 100120 (level 10): Sudoers file changed · 2026-10-28 14:02 UTC
🔗 Alert in dashboard · Runbook
```

- The model's answer (summary, category, action, `Missing:`) is one `<pre>`
  block, so Telegram shows a copy button for pasting it into a reply.
- 🟥 critical, 🔴 high, 🟠 medium, 🟢 low. **🟡 "needs a look"** when the
  model is unsure, whatever the severity: an unsure verdict never reads as calm.
- `Missing:` lists what the model lacked (`needs`), when it says so.
- "Alert in dashboard" opens that one alert in the Wazuh dashboard
  (`brain_dashboard_url`). Per-rule links (runbooks, wiki) come from
  `brain_rule_links`.
- If the model fails, the alert still goes out, marked
  `AI analysis unavailable: <reason>`, with severity from `rule.level`
  (0–6 low, 7–9 medium, 10–12 high, 13+ critical). If Telegram fails after
  3 attempts the alert is logged at ERROR as dropped.

## Configure

| Variable | Default | Purpose |
|---|---|---|
| `brain_enabled` | `true` | `false` stops and disables the service (image, config, state stay) |
| `analysis_threshold` | `7` | analyse alerts with `rule.level` ≥ this |
| `brain_ignore_rule_ids` | `["99999"]` | `rule.id`s never analysed (Voice's verify rule) |
| `poll_interval` / `brain_max_backfill` | `5` / `3600` | seconds between polls / max catch-up after downtime |
| `brain_indexer_url` | `https://127.0.0.1:9200` | Wazuh indexer |
| `brain_indexer_user` / `brain_indexer_password` | `brain-ro` / — | read-only indexer user |
| `brain_indexer_verify_tls` / `brain_indexer_ca_file` | `true` / `""` | TLS check; CA bundle on the host |
| `brain_indexer_admin_user` / `_password` | `""` | if set, the role creates the read-only user (role `brain_read`: search on `wazuh-alerts-*` only) |
| `brain_model_backend` | `anthropic` | or `openai_compat` |
| `brain_model` / `brain_model_api_key` | `claude_model` / `claude_api_key` | model and key |
| `brain_model_base_url` | `""` | required for `openai_compat`, e.g. `http://127.0.0.1:11434/v1` |
| `brain_model_options` | `{effort: low, workspace_id: ""}` | anthropic: `effort`, `workspace_id`; openai_compat: `json_mode` (`schema`, `object`, `none`) |
| `brain_model_timeout` / `brain_model_max_tokens` | `30` / `2048` | per request; thinking counts against max tokens |
| `telegram_bot_token` / `telegram_chat_id` | — | same bot as Voice |
| `brain_dashboard_url` | `""` | Wazuh dashboard base URL for the alert link |
| `brain_rule_links` | `{}` | `{"<rule id>" or "*": [{title, url}]}` |

Without `brain_indexer_admin_user`, create the indexer user yourself: an
internal user with a role that allows `read` and `indices:data/read/search`
on `wazuh-alerts-*` only.

Secrets: in the lab from `.envrc` (see
[tests/README.md](../tests/README.md#secrets-telegram-claude-api)); elsewhere
from your own secret store. They end up in `/etc/brain-analyst/brain.env`
(root, 0600) on the host.

A local model instead of Claude:

```yaml
brain_model_backend: openai_compat
brain_model: qwen3:8b
brain_model_base_url: http://127.0.0.1:11434/v1
brain_model_api_key: ""
```

## One message per alert

Brain's Telegram message replaces Voice's for alerts at or above
`analysis_threshold`: Voice is capped with `voice_max_level = analysis_threshold - 1`
(see [blackwall-voice/README.md](../blackwall-voice/README.md#one-message-per-alert-with-brain)).
Voice reads the threshold from `/etc/brain-analyst/brain.env` and checks that
the `brain-analyst` unit is active, so both roles must run on the same host.

## Deploy, turn on/off

```bash
cd blackwall-brain
ansible-playbook ansible/playbooks/brain.yml                          # deploy / on
ansible-playbook ansible/playbooks/brain.yml -e brain_enabled=false   # off
```

In the lab: `tests/bin/bw-lab provision brain` (run through `direnv exec .`;
`-e brain_enabled=false` to turn it off, Voice is re-deployed to match).
Deploying over v1 removes the old script, its log and state.

Logs: `journalctl -u brain-analyst` (one line per alert: verdict, severity,
tokens, latency, model, prompt version).

## Verify

```bash
tests/bin/bw-check brain
```

Checks the image, then injects a synthetic level-10 alert (rule 99998, ignored
by Voice) into `alerts.json`; it reaches the indexer through filebeat. With
credentials Brain must deliver it to Telegram within 120 s; without, it must
at least pick it up.

## Eval

`eval/` holds an open, labelled set of lab alerts and a harness that scores
Brain's verdicts per field, with cost and latency, for any backend:
`.venv/bin/python -m brain.eval.run --max-cost 1`. Results so far are in
`STATUS.md`; how to run, score and add scenarios: [eval/README.md](eval/README.md).

## Develop

```bash
cd blackwall-brain
python3.12 -m venv .venv && .venv/bin/pip install -e '.[dev]'
.venv/bin/pytest
```

`requirements.lock` pins runtime dependencies with hashes; regenerate it after
changing `pyproject.toml`:
`.venv/bin/pip install pip-tools && .venv/bin/pip-compile --generate-hashes --strip-extras -o requirements.lock pyproject.toml`.

## Docs

- `CLAUDE.md` — module context, design decisions, agent rules
- `STATUS.md` — current implementation state

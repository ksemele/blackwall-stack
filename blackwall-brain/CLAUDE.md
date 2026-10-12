# blackwall-brain — Brain

**Role:** AI analysis layer.
Brain receives Wazuh alerts that pass the severity threshold, calls the Claude API to classify
and explain them, then forwards enriched analysis to Telegram.
It does NOT route alerts to humans (that is Voice) and does NOT perform detection (that is Eye).

---

## Purpose

Eye sees. Voice speaks. Brain thinks. When a high-severity alert fires, Brain interrogates it:
What kind of attack is this? How confident are we? What should the operator do next?
One Claude API call per alert — structured output, actionable signal.

---

## Stage 3 Scope

### 1. Brain Analyst (primary deliverable)

A Python script deployed as a systemd service on the manager host that:

- Reads from `/var/ossec/logs/alerts/alerts.json` (tail-follows the live log, inode-rotation-aware)
- Filters by severity: only analyzes alerts with `rule.level >= analysis_threshold` (default 7)
- Calls Claude API with a structured prompt containing alert context (host, level, description, MITRE, full_log)
- Receives structured AI classification: threat category, confidence, recommended action
- Formats a "Brain analysis" Telegram message and sends via same bot as Voice
- Deduplicates: tracks processed `id` values to avoid re-analyzing on restart (state file)
- On Claude API failure: logs error and skips the alert (does NOT write ID to state — allows retry)
- Restarts automatically on failure (systemd `Restart=on-failure`)

---

## Design Principles

- **Zero pip dependencies:** uses only Python 3 stdlib (`urllib.request`, `json`, `logging`, `os`, `time`)
- **Config as code:** all settings live in Ansible vars and templates; no manual edits on hosts
- **Fail open on Claude errors:** if Claude API fails, alert is not suppressed — it remains eligible for retry
- **Signal over noise:** `analysis_threshold` default 7 (higher than Voice's 5) — only high-confidence events get AI analysis
- **Resilient:** systemd `Restart=on-failure`; script catches all errors and sleeps, never crashes

---

## Stack

| Component | Detail |
|---|---|
| Language | Python 3 stdlib only (no external deps) |
| Alert source | `/var/ossec/logs/alerts/alerts.json` (line-buffered JSON, one event per line) |
| AI backend | Claude API (`https://api.anthropic.com/v1/messages`) via `urllib.request` |
| Notification sink | Telegram Bot API (same bot as Voice) |
| Secrets | `claude_api_key`, `telegram_bot_token`, `telegram_chat_id` — passed as Ansible vars (vault-encrypted in production) |
| Service manager | systemd on manager host |
| Deployment | Ansible role `brain-analyst`, playbook `brain.yml` |
| Target host group | `wazuh_managers` (shared inventory at `../tests/inventory/hosts.yml`) |

---

## Repository Layout

```
blackwall-brain/
├── CLAUDE.md                         # this file
├── README.md                         # operator quick-start
├── STATUS.md                         # current module state
└── ansible/
    ├── ansible.cfg                   # roles_path, inventory pointer → shared tests inventory
    ├── playbooks/
    │   ├── brain.yml                 # deploy brain-analyst to wazuh_managers
    │   └── verify-brain.yml          # check service running, inject test alert, verify pickup
    └── roles/
        └── brain-analyst/
            ├── defaults/
            │   └── main.yml          # analysis_threshold, claude_model, state_file path, etc.
            ├── tasks/
            │   └── main.yml          # create dirs, deploy script + unit, enable service
            ├── handlers/
            │   └── main.yml          # Reload systemd daemon, Restart brain-analyst service
            └── templates/
                ├── brain.py.j2       # the analyst script (Jinja2 → Python)
                └── brain.service.j2  # systemd unit file
```

---

## Key Variables (defaults)

| Variable | Default | Description |
|---|---|---|
| `analysis_threshold` | `7` | Minimum `rule.level` to trigger AI analysis |
| `poll_interval` | `5` | Seconds between log tail reads |
| `claude_api_key` | `""` | Anthropic API key — must be set |
| `claude_model` | `claude-haiku-5-5` | Claude model for analysis |
| `max_tokens` | `300` | Max tokens in Claude response |
| `brain_state_file` | `/var/lib/brain-analyst/sent_ids.txt` | Dedup state |
| `brain_log_file` | `/var/log/brain-analyst/analyst.log` | Service log |
| `brain_alert_log` | `/var/ossec/logs/alerts/alerts.json` | Alert source |
| `telegram_bot_token` | `""` | Same bot as Voice — must be set |
| `telegram_chat_id` | `""` | Same chat as Voice — must be set |

---

## Inventory

Brain shares the inventory with blackwall-eye and blackwall-voice. The `ansible.cfg` points to:

```
inventory = ../tests/inventory/hosts.yml
```

Do NOT duplicate inventory files. Brain deploys to `wazuh_managers` group (same as Voice).

---

## Claude API Response Format

Brain instructs Claude to respond in a single structured line:

```
CATEGORY: <threat_category> | CONFIDENCE: <high/medium/low> | ACTION: <recommended_action>
```

Valid categories: `lateral_movement`, `privilege_escalation`, `persistence`, `credential_access`,
`defense_evasion`, `discovery`, `collection`, `exfiltration`, `command_and_control`, `impact`,
`false_positive`, `unknown`.

---

## Agent Rules

1. **Read STATUS.md before starting any work.** If status is `pending`, nothing has been implemented.
2. **Update STATUS.md after completing each task** — `last_task`, `next_task`, `last_updated`.
3. **Test against the shared Docker test environment** defined in `tests/inventory/hosts.yml`.
4. **Keep Brain independently deployable** — no imports or hard path dependencies on other modules.
5. **Secrets via Ansible vault** — never commit `claude_api_key` or `telegram_bot_token` in plaintext.
6. **Keep docs in sync** — if playbooks, roles, or CLI commands change, update README.md in the same commit.
7. **Do not suppress alerts on Claude failure** — only write to state file after both Claude + Telegram succeed.

---

## Testing Checklist

- [ ] `ansible-playbook ansible/playbooks/brain.yml` completes without errors on test host
- [ ] `systemctl is-active brain-analyst` returns `active` on manager host
- [ ] Injecting a level-9+ alert into `alerts.json` triggers brain-analyst to attempt Claude analysis
- [ ] With real API key: Claude response appears in log and Telegram message is sent
- [ ] With placeholder API key: Claude failure is logged gracefully, alert ID not written to state file
- [ ] Restarting the service does not re-analyze already-processed alerts (dedup state persists)
- [ ] Alerts below `analysis_threshold` are silently dropped
